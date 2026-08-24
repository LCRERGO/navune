// Package analysis orchestrates the full pipeline: discover files, parse each
// with its language adapter, resolve imports, compute duplication, cycles and
// coupling, aggregate summary metrics, and evaluate quality gates. It owns the
// stable Report model that text/JSON/Mermaid renderers consume.
package analysis

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/discover"
	"github.com/lcr/navune/internal/dup"
	"github.com/lcr/navune/internal/gate"
	"github.com/lcr/navune/internal/graph"
	"github.com/lcr/navune/internal/lang"
	"github.com/lcr/navune/internal/lang/golang"
	"github.com/lcr/navune/internal/lang/python"
	"github.com/lcr/navune/internal/lang/treescript"
)

// FileReport is one file's row in the report (production or test).
type FileReport struct {
	Path          string  `json:"path"`
	Lang          string  `json:"lang"`
	Class         string  `json:"class"` // "prod" | "test"
	Package       string  `json:"package"`
	ImportPath    string  `json:"import_path,omitempty"`
	TotalLines    int     `json:"total_lines"`
	PhysicalSLOC  int     `json:"physical_sloc"`
	LogicalLOC    int     `json:"logical_loc"`
	Functions     int     `json:"functions"`
	AvgComplexity float64 `json:"avg_complexity"`
	WorstComplex  int     `json:"worst_complexity"`
	WorstFunc     string  `json:"worst_function,omitempty"`
	Types         int     `json:"types"`
	AbstractTypes int     `json:"abstract_types"`
	Ca            int     `json:"ca,omitempty"`
	Ce            int     `json:"ce,omitempty"`
	Instability   float64 `json:"instability,omitempty"`
	Abstractness  float64 `json:"abstractness,omitempty"`
	Distance      float64 `json:"distance,omitempty"`
	InCycle       bool    `json:"in_cycle"`
	CycleName     string  `json:"cycle,omitempty"`
	DupTokens     int     `json:"dup_tokens"`
	DupPct        float64 `json:"dup_pct"`
}

// CycleReport describes one strongly connected component.
type CycleReport struct {
	Index   int      `json:"index"`
	Size    int      `json:"size"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// Summary is the codebase-level aggregation (production code only).
type Summary struct {
	ProdFiles       int     `json:"prod_files"`
	TestFiles       int     `json:"test_files"`
	SkippedDirs     int     `json:"skipped_dirs"`
	GeneratedFiles  int     `json:"generated_files"`
	PhysicalSLOC    int     `json:"physical_sloc"`
	LogicalLOC      int     `json:"logical_loc"`
	Functions       int     `json:"functions"`
	AvgComplexity   float64 `json:"avg_complexity"`
	WorstComplexity int     `json:"worst_complexity"`
	WorstFunction   string  `json:"worst_function,omitempty"`
	Types           int     `json:"types"`
	AbstractTypes   int     `json:"abstract_types"`
	TotalTokens     int     `json:"total_tokens"`
	DupTokens       int     `json:"dup_tokens"`
	DupPct          float64 `json:"dup_pct"`
	DupBlocks       int     `json:"dup_blocks"`
	Cycles          int     `json:"cycles"`
	FilesInCycle    int     `json:"files_in_cycle"`
	MaxCycleMembers int     `json:"max_cycle_members"`
	InCyclePct      float64 `json:"in_cycle_pct"`
}

// Report is Navune's stable output contract (schema_version 1).
type Report struct {
	SchemaVersion int                 `json:"schema_version"`
	Tool          string              `json:"tool"`
	Root          string              `json:"root"`
	ModulePath    string              `json:"module_path,omitempty"`
	InModule      bool                `json:"in_module"`
	Summary       Summary             `json:"summary"`
	Files         []FileReport        `json:"files"`
	Cycles        []CycleReport       `json:"cycles"`
	Budgets       []gate.BudgetResult `json:"budgets"`
	Composite     float64             `json:"composite"`
	ExitCode      int                 `json:"exit_code"`

	// InternalEdges carries internal file->file dependencies for the Mermaid
	// renderer only; it is excluded from the JSON schema.
	InternalEdges [][2]string `json:"-"`
}

func parserFor(langID lang.Lang) lang.Parser {
	switch langID {
	case lang.Go:
		return golang.New()
	case lang.TypeScript:
		return treescript.NewTS()
	case lang.JavaScript:
		return treescript.NewJS()
	case lang.Python:
		return python.New()
	}
	return nil
}

// Analyze runs the full pipeline against root with the given config.
func Analyze(root string, cfg *config.Config) (*Report, error) {
	if err := gate.Validate(cfg); err != nil {
		return nil, err
	}
	mod, err := discover.FindModule(root)
	if err != nil {
		return nil, err
	}
	walk, err := discover.Walk(root, cfg)
	if err != nil {
		return nil, err
	}

	r := &Report{
		SchemaVersion: 1,
		Tool:          "navune",
		Root:          filepath.ToSlash(root),
	}
	if mod != nil {
		r.ModulePath = mod.Path
		r.InModule = mod.Present
	}
	r.Summary.SkippedDirs = len(walk.SkippedDirs)

	// Parse every candidate file concurrently.
	type parsed struct {
		fr  *lang.FileResult
		err error
	}
	results := make([]parsed, len(walk.Files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, f := range walk.Files {
		wg.Add(1)
		go func(i int, f discover.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := parserFor(f.Lang)
			if p == nil {
				results[i] = parsed{err: fmt.Errorf("no adapter for %s", f.Lang)}
				return
			}
			src, err := os.ReadFile(f.Path)
			if err != nil {
				results[i] = parsed{err: err}
				return
			}
			fr, err := p.Parse(f.Path, src)
			if err != nil {
				results[i] = parsed{err: err}
				return
			}
			if discover.IsTestFile(f) {
				fr.Class = lang.Test
			}
			fr.ImportPath = discover.ImportPath(mod, f)
			fr.Path = f.RelPath // report paths relative to the analysis root
			results[i] = parsed{fr: fr}
		}(i, f)
	}
	wg.Wait()

	var errs []string
	var prodFiles, testFiles []*lang.FileResult
	for _, p := range results {
		if p.err != nil {
			errs = append(errs, p.err.Error())
			continue
		}
		if p.fr.IsGenerated() {
			r.Summary.GeneratedFiles++
			continue
		}
		if p.fr.Class == lang.Test {
			testFiles = append(testFiles, p.fr)
		} else {
			prodFiles = append(prodFiles, p.fr)
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return nil, fmt.Errorf("%d file(s) could not be analyzed:\n  %s", len(errs), strings.Join(errs, "\n  "))
	}

	resolveScriptDeps(prodFiles)
	g := graph.Build(prodFiles)
	dupRes := dupEngine().Detect(tokenSets(prodFiles))
	sortGraph(g)

	r.Summary.TestFiles = len(testFiles)
	aggregateSummary(r, prodFiles, dupRes)
	aggregateCycles(r, g)
	r.Files = buildFileRows(prodFiles, g, dupRes)
	r.Files = append(r.Files, buildTestRows(testFiles)...)
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })

	// internal edges for the mermaid renderer
	for i := range g.Nodes {
		for j := range g.Successors(i) {
			r.InternalEdges = append(r.InternalEdges, [2]string{g.Nodes[i].File.Path, g.Nodes[j].File.Path})
		}
	}
	sort.Slice(r.InternalEdges, func(a, b int) bool {
		if r.InternalEdges[a][0] != r.InternalEdges[b][0] {
			return r.InternalEdges[a][0] < r.InternalEdges[b][0]
		}
		return r.InternalEdges[a][1] < r.InternalEdges[b][1]
	})

	verdict := gate.Evaluate(cfg, measured(r.Summary))
	r.Budgets = verdict.Budgets
	r.Composite = verdict.Score
	r.ExitCode = verdict.ExitCode
	return r, nil
}

func dupEngine() *dup.Engine { return dup.New() }

func tokenSets(files []*lang.FileResult) [][]lang.Token {
	sets := make([][]lang.Token, 0, len(files))
	for _, f := range files {
		sets = append(sets, f.Tokens)
	}
	return sets
}

func measured(s Summary) map[string]float64 {
	return map[string]float64{
		"avg_complexity":      s.AvgComplexity,
		"worst_complexity":    float64(s.WorstComplexity),
		"max_duplication_pct": s.DupPct,
		"max_cycle_members":   float64(s.MaxCycleMembers),
		"max_in_cycle_pct":    s.InCyclePct,
	}
}

func sortGraph(g *graph.Graph) {
	for _, c := range g.Cycles {
		sort.Slice(c.Members, func(i, j int) bool { return c.Members[i].File.Path < c.Members[j].File.Path })
	}
	sort.Slice(g.Cycles, func(i, j int) bool {
		if len(g.Cycles[i].Members) == 0 {
			return false
		}
		if len(g.Cycles[j].Members) == 0 {
			return true
		}
		return g.Cycles[i].Members[0].File.Path < g.Cycles[j].Members[0].File.Path
	})
}

func aggregateSummary(r *Report, prod []*lang.FileResult, d *dup.Result) {
	s := &r.Summary
	s.ProdFiles = len(prod)

	var totalComp, totalFuncs, types, abstract int
	worst := 0
	worstLoc := ""
	for _, f := range prod {
		s.PhysicalSLOC += f.PhysicalSLOC
		s.LogicalLOC += f.LogicalLOC
		for _, t := range f.Types {
			types++
			if t.Abstract {
				abstract++
			}
		}
		for _, fn := range f.Functions {
			totalFuncs++
			totalComp += fn.Complexity
			if fn.Complexity > worst {
				worst = fn.Complexity
				worstLoc = fmt.Sprintf("%s: %s", f.Path, fn.Name)
			}
		}
		s.TotalTokens += len(f.Tokens)
	}
	s.Functions = totalFuncs
	s.Types = types
	s.AbstractTypes = abstract
	if totalFuncs > 0 {
		s.AvgComplexity = round1(float64(totalComp) / float64(totalFuncs))
	}
	s.WorstComplexity = worst
	s.WorstFunction = worstLoc
	s.DupTokens = d.DupTokens
	s.DupBlocks = d.Blocks
	if s.TotalTokens > 0 {
		s.DupPct = round1(100 * float64(s.DupTokens) / float64(s.TotalTokens))
	}
}

func aggregateCycles(r *Report, g *graph.Graph) {
	s := &r.Summary
	for _, c := range g.Cycles {
		cr := CycleReport{
			Index:   c.Index,
			Size:    len(c.Members),
			Name:    "",
			Members: make([]string, 0, len(c.Members)),
		}
		for _, m := range c.Members {
			cr.Members = append(cr.Members, m.File.Path)
		}
		if len(cr.Members) > 0 {
			cr.Name = cr.Members[0]
		}
		r.Cycles = append(r.Cycles, cr)
		if len(c.Members) > s.MaxCycleMembers {
			s.MaxCycleMembers = len(c.Members)
		}
	}
	s.Cycles = len(r.Cycles)
	s.FilesInCycle = len(g.FilesInCycles())
	if s.ProdFiles > 0 {
		s.InCyclePct = round1(100 * float64(s.FilesInCycle) / float64(s.ProdFiles))
	}
}

func buildFileRows(prod []*lang.FileResult, g *graph.Graph, d *dup.Result) []FileReport {
	byPath := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byPath[n.File.Path] = n
	}
	rows := make([]FileReport, 0, len(prod))
	for i, f := range prod {
		rows = append(rows, buildFileReport(f, byPath[f.Path], d.ByFile[i]))
	}
	return rows
}

func buildTestRows(test []*lang.FileResult) []FileReport {
	rows := make([]FileReport, 0, len(test))
	for _, f := range test {
		rows = append(rows, buildFileReport(f, nil, 0))
	}
	return rows
}

func buildFileReport(fr *lang.FileResult, n *graph.Node, dupTokens int) FileReport {
	row := FileReport{
		Path:         fr.Path,
		Lang:         string(fr.Lang),
		Class:        fr.Class.String(),
		Package:      fr.Package,
		ImportPath:   fr.ImportPath,
		TotalLines:   fr.TotalLines,
		PhysicalSLOC: fr.PhysicalSLOC,
		LogicalLOC:   fr.LogicalLOC,
		Functions:    len(fr.Functions),
	}
	if n != nil {
		row.Ca = n.Ca
		row.Ce = n.Ce
		row.Instability = round3(n.Instability)
		row.Abstractness = round3(n.Abstract)
		row.Distance = round3(n.Distance)
		row.InCycle = n.InCycle
		if n.InCycle {
			row.CycleName = fmt.Sprintf("cycle-%d", n.CycleIndex)
		}
	}
	for _, t := range fr.Types {
		row.Types++
		if t.Abstract {
			row.AbstractTypes++
		}
	}
	if len(fr.Functions) > 0 {
		sum := 0
		worst := -1
		worstName := ""
		for _, f := range fr.Functions {
			sum += f.Complexity
			if f.Complexity > worst {
				worst = f.Complexity
				worstName = f.Name
			}
		}
		row.AvgComplexity = round2(float64(sum) / float64(len(fr.Functions)))
		row.WorstComplex = worst
		row.WorstFunc = worstName
	}
	if len(fr.Tokens) > 0 {
		row.DupTokens = dupTokens
		row.DupPct = round2(100 * float64(dupTokens) / float64(len(fr.Tokens)))
	}
	return row
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }
func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
