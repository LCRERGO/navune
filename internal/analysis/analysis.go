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
	"github.com/lcr/navune/internal/lang/cfamily"
	"github.com/lcr/navune/internal/lang/golang"
	"github.com/lcr/navune/internal/lang/python"
	"github.com/lcr/navune/internal/lang/treescript"
	"github.com/lcr/navune/internal/smell"
)

// FileReport is one file's row in the report (production or test).
type FileReport struct {
	Path              string           `json:"path"`
	Lang              string           `json:"lang"`
	Class             string           `json:"class"` // "prod" | "test"
	Package           string           `json:"package"`
	ImportPath        string           `json:"import_path,omitempty"`
	TotalLines        int              `json:"total_lines"`
	PhysicalSLOC      int              `json:"physical_sloc"`
	LogicalLOC        int              `json:"logical_loc"`
	CommentLines      int              `json:"comment_lines"`
	CommentPct        float64          `json:"comment_pct"`
	Functions         int              `json:"functions"`
	AvgComplexity     float64          `json:"avg_complexity"`
	WorstComplex      int              `json:"worst_complexity"`
	WorstFunc         string           `json:"worst_function,omitempty"`
	AvgCognitive      float64          `json:"avg_cognitive_complexity"`
	WorstCognitive    int              `json:"cognitive_complexity"`
	MaxFunctionLength int              `json:"max_function_length"`
	MaxNesting        int              `json:"max_nesting"`
	MaxParams         int              `json:"max_params"`
	Types             int              `json:"types"`
	AbstractTypes     int              `json:"abstract_types"`
	Ca                int              `json:"ca,omitempty"`
	Ce                int              `json:"ce,omitempty"`
	Instability       float64          `json:"instability,omitempty"`
	Abstractness      float64          `json:"abstractness,omitempty"`
	Distance          float64          `json:"distance,omitempty"`
	InCycle           bool             `json:"in_cycle"`
	CycleName         string           `json:"cycle,omitempty"`
	DupTokens         int              `json:"dup_tokens"`
	DupPct            float64          `json:"dup_pct"`
	DuplicatedLines   int              `json:"duplicated_lines"`
	FunctionsDetail   []FunctionReport `json:"functions_detail,omitempty"`
}

// FunctionReport is per-function detail emitted only with --verbose.
type FunctionReport struct {
	Name       string `json:"name"`
	Enclosing  string `json:"enclosing,omitempty"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Complexity int    `json:"complexity"`
	Cognitive  int    `json:"cognitive_complexity"`
	Length     int    `json:"length"`
	Nesting    int    `json:"nesting"`
	Params     int    `json:"params"`
	Anonymous  bool   `json:"anonymous,omitempty"`
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
	ProdFiles         int                        `json:"prod_files"`
	TestFiles         int                        `json:"test_files"`
	SkippedDirs       int                        `json:"skipped_dirs"`
	GeneratedFiles    int                        `json:"generated_files"`
	PhysicalSLOC      int                        `json:"physical_sloc"`
	LogicalLOC        int                        `json:"logical_loc"`
	CommentLines      int                        `json:"comment_lines"`
	CommentPct        float64                    `json:"comment_pct"`
	Functions         int                        `json:"functions"`
	AvgComplexity     float64                    `json:"avg_complexity"`
	WorstComplexity   int                        `json:"worst_complexity"`
	WorstFunction     string                     `json:"worst_function,omitempty"`
	AvgCognitive      float64                    `json:"avg_cognitive_complexity"`
	WorstCognitive    int                        `json:"worst_cognitive_complexity"`
	MaxFunctionLength int                        `json:"max_function_length"`
	MaxNesting        int                        `json:"max_nesting"`
	MaxParams         int                        `json:"max_params"`
	Types             int                        `json:"types"`
	AbstractTypes     int                        `json:"abstract_types"`
	TotalTokens       int                        `json:"total_tokens"`
	DupTokens         int                        `json:"dup_tokens"`
	DupPct            float64                    `json:"dup_pct"`
	DupBlocks         int                        `json:"dup_blocks"`
	DuplicatedLines   int                        `json:"duplicated_lines"`
	IssueCount        int                        `json:"issue_count"`
	Cycles            int                        `json:"cycles"`
	FilesInCycle      int                        `json:"files_in_cycle"`
	MaxCycleMembers   int                        `json:"max_cycle_members"`
	InCyclePct        float64                    `json:"in_cycle_pct"`
	ByLanguage        map[string]LanguageSummary `json:"by_language,omitempty"`
}

// LanguageSummary repeats the size, complexity, comment, duplication, and
// issue aggregates for one language. Cycle and coupling metrics are excluded:
// they are inherently cross-language (ADR 0016).
type LanguageSummary struct {
	ProdFiles         int     `json:"prod_files"`
	PhysicalSLOC      int     `json:"physical_sloc"`
	LogicalLOC        int     `json:"logical_loc"`
	CommentLines      int     `json:"comment_lines"`
	CommentPct        float64 `json:"comment_pct"`
	Functions         int     `json:"functions"`
	AvgComplexity     float64 `json:"avg_complexity"`
	WorstComplexity   int     `json:"worst_complexity"`
	AvgCognitive      float64 `json:"avg_cognitive_complexity"`
	WorstCognitive    int     `json:"worst_cognitive_complexity"`
	MaxFunctionLength int     `json:"max_function_length"`
	MaxNesting        int     `json:"max_nesting"`
	MaxParams         int     `json:"max_params"`
	Types             int     `json:"types"`
	AbstractTypes     int     `json:"abstract_types"`
	TotalTokens       int     `json:"total_tokens"`
	DupTokens         int     `json:"dup_tokens"`
	DupPct            float64 `json:"dup_pct"`
	DuplicatedLines   int     `json:"duplicated_lines"`
	IssueCount        int     `json:"issue_count"`
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
	Issues        []smell.Issue       `json:"issues"`
	Budgets       []gate.BudgetResult `json:"budgets"`
	Composite     float64             `json:"composite"`
	ExitCode      int                 `json:"exit_code"`

	// Verbose controls whether per-function detail is populated.
	Verbose bool `json:"-"`

	// InternalEdges carries internal file->file dependencies for the Mermaid
	// renderer only; it is excluded from the JSON schema.
	InternalEdges [][2]string `json:"-"`
}

// Options controls optional analysis behavior.
type Options struct {
	Verbose bool
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
	case lang.C:
		return cfamily.NewC()
	case lang.CPP:
		return cfamily.NewCPP()
	}
	return nil
}

// Analyze runs the full pipeline against root with the given config.
func Analyze(root string, cfg *config.Config) (*Report, error) {
	return AnalyzeOptions(root, cfg, Options{})
}

// AnalyzeOptions runs the pipeline with optional behavior (e.g. verbose).
func AnalyzeOptions(root string, cfg *config.Config, opts Options) (*Report, error) {
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
		Verbose:       opts.Verbose,
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

	resolveDeps(prodFiles, cfg, root)
	g := graph.Build(prodFiles)
	dupRes := dupEngine().Detect(tokenSets(prodFiles))
	sortGraph(g)

	issues := smell.Evaluate(smellFiles(prodFiles, dupRes), cfg.SmellSettings())
	if issues == nil {
		issues = []smell.Issue{}
	}
	r.Issues = issues

	r.Summary.TestFiles = len(testFiles)
	aggregateSummary(r, prodFiles, dupRes)
	r.Summary.IssueCount = len(issues)
	r.Summary.ByLanguage = aggregateLanguages(prodFiles, dupRes, issues)
	aggregateCycles(r, g)
	r.Files = buildFileRows(prodFiles, g, dupRes, opts.Verbose)
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

	verdict := gate.Evaluate(cfg, measured(r.Summary, issues))
	verdict.EvaluateLanguages(cfg, measuredLanguages(prodFiles, dupRes, issues))
	r.Budgets = verdict.Budgets
	r.Composite = verdict.Score
	r.ExitCode = verdict.ExitCode
	return r, nil
}

// smellFiles pairs production files with their duplicated-line counts.
func smellFiles(prod []*lang.FileResult, d *dup.Result) []smell.File {
	out := make([]smell.File, 0, len(prod))
	for i, f := range prod {
		out = append(out, smell.File{Result: f, DuplicatedLines: d.DupLinesByFile[i]})
	}
	return out
}

func dupEngine() *dup.Engine { return dup.New() }

func tokenSets(files []*lang.FileResult) [][]lang.Token {
	sets := make([][]lang.Token, 0, len(files))
	for _, f := range files {
		sets = append(sets, f.Tokens)
	}
	return sets
}

func measured(s Summary, issues []smell.Issue) map[string]float64 {
	m := map[string]float64{
		"avg_complexity":             s.AvgComplexity,
		"worst_complexity":           float64(s.WorstComplexity),
		"max_duplication_pct":        s.DupPct,
		"max_cycle_members":          float64(s.MaxCycleMembers),
		"max_in_cycle_pct":           s.InCyclePct,
		"avg_cognitive_complexity":   s.AvgCognitive,
		"worst_cognitive_complexity": float64(s.WorstCognitive),
		"max_function_length":        float64(s.MaxFunctionLength),
		"max_nesting":                float64(s.MaxNesting),
		"max_params":                 float64(s.MaxParams),
		"max_duplicated_lines_pct":   dupLinesPct(s.DuplicatedLines, s.PhysicalSLOC),
		"max_issues":                 float64(len(issues)),
	}
	for sev, n := range severityCounts(issues) {
		m["max_"+sev+"_issues"] = float64(n)
	}
	return m
}

// measuredLanguages builds the per-language metric maps used for per-language
// budgets (ADR 0016). The composite is unaffected.
func measuredLanguages(prod []*lang.FileResult, d *dup.Result, issues []smell.Issue) map[lang.Lang]map[string]float64 {
	summaries := aggregateLanguages(prod, d, issues)
	out := map[lang.Lang]map[string]float64{}
	for name, ls := range summaries {
		l, _ := lang.Canonical(name)
		m := map[string]float64{
			"avg_complexity":             ls.AvgComplexity,
			"worst_complexity":           float64(ls.WorstComplexity),
			"max_duplication_pct":        ls.DupPct,
			"avg_cognitive_complexity":   ls.AvgCognitive,
			"worst_cognitive_complexity": float64(ls.WorstCognitive),
			"max_function_length":        float64(ls.MaxFunctionLength),
			"max_nesting":                float64(ls.MaxNesting),
			"max_params":                 float64(ls.MaxParams),
			"max_duplicated_lines_pct":   dupLinesPct(ls.DuplicatedLines, ls.PhysicalSLOC),
			"max_issues":                 float64(ls.IssueCount),
		}
		out[l] = m
	}
	return out
}

func severityCounts(issues []smell.Issue) map[string]int {
	m := map[string]int{}
	for _, iss := range issues {
		m[iss.Severity]++
	}
	return m
}

func dupLinesPct(dupLines, sloc int) float64 {
	if sloc == 0 {
		return 0
	}
	return round2(100 * float64(dupLines) / float64(sloc))
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

	var totalComp, totalFuncs, totalCog, types, abstract int
	worst := 0
	worstLoc := ""
	worstCog := 0
	maxLen, maxNest, maxParams := 0, 0, 0
	for _, f := range prod {
		s.PhysicalSLOC += f.PhysicalSLOC
		s.LogicalLOC += f.LogicalLOC
		s.CommentLines += f.CommentLines
		for _, t := range f.Types {
			types++
			if t.Abstract {
				abstract++
			}
		}
		for _, fn := range f.Functions {
			totalFuncs++
			totalComp += fn.Complexity
			totalCog += fn.Cognitive
			if fn.Complexity > worst {
				worst = fn.Complexity
				worstLoc = fmt.Sprintf("%s: %s", f.Path, fn.Name)
			}
			if fn.Cognitive > worstCog {
				worstCog = fn.Cognitive
			}
			if fn.Length > maxLen {
				maxLen = fn.Length
			}
			if fn.Nesting > maxNest {
				maxNest = fn.Nesting
			}
			if fn.Params > maxParams {
				maxParams = fn.Params
			}
		}
		s.TotalTokens += len(f.Tokens)
	}
	s.Functions = totalFuncs
	s.Types = types
	s.AbstractTypes = abstract
	if totalFuncs > 0 {
		s.AvgComplexity = round1(float64(totalComp) / float64(totalFuncs))
		s.AvgCognitive = round1(float64(totalCog) / float64(totalFuncs))
	}
	s.WorstComplexity = worst
	s.WorstFunction = worstLoc
	s.WorstCognitive = worstCog
	s.MaxFunctionLength = maxLen
	s.MaxNesting = maxNest
	s.MaxParams = maxParams
	s.CommentPct = commentPct(s.CommentLines, s.PhysicalSLOC)
	s.DupTokens = d.DupTokens
	s.DupBlocks = d.Blocks
	for _, n := range d.DupLinesByFile {
		s.DuplicatedLines += n
	}
	if s.TotalTokens > 0 {
		s.DupPct = round1(100 * float64(s.DupTokens) / float64(s.TotalTokens))
	}
}

// aggregateLanguages rolls the production files up per language (ADR 0016).
func aggregateLanguages(prod []*lang.FileResult, d *dup.Result, issues []smell.Issue) map[string]LanguageSummary {
	type acc struct {
		ls                        LanguageSummary
		comp, cog, funcs          int
		worstComp, worstCog       int
		maxLen, maxNest, maxParam int
	}
	byLang := map[lang.Lang]*acc{}
	langOfPath := map[string]lang.Lang{}
	for i, f := range prod {
		a := byLang[f.Lang]
		if a == nil {
			a = &acc{}
			byLang[f.Lang] = a
		}
		langOfPath[f.Path] = f.Lang
		a.ls.ProdFiles++
		a.ls.PhysicalSLOC += f.PhysicalSLOC
		a.ls.LogicalLOC += f.LogicalLOC
		a.ls.CommentLines += f.CommentLines
		a.ls.TotalTokens += len(f.Tokens)
		a.ls.DupTokens += d.ByFile[i]
		a.ls.DuplicatedLines += d.DupLinesByFile[i]
		for _, t := range f.Types {
			a.ls.Types++
			if t.Abstract {
				a.ls.AbstractTypes++
			}
		}
		for _, fn := range f.Functions {
			a.funcs++
			a.comp += fn.Complexity
			a.cog += fn.Cognitive
			if fn.Complexity > a.worstComp {
				a.worstComp = fn.Complexity
			}
			if fn.Cognitive > a.worstCog {
				a.worstCog = fn.Cognitive
			}
			if fn.Length > a.maxLen {
				a.maxLen = fn.Length
			}
			if fn.Nesting > a.maxNest {
				a.maxNest = fn.Nesting
			}
			if fn.Params > a.maxParam {
				a.maxParam = fn.Params
			}
		}
	}
	for _, iss := range issues {
		if l, ok := langOfPath[iss.File]; ok {
			byLang[l].ls.IssueCount++
		}
	}
	out := map[string]LanguageSummary{}
	for l, a := range byLang {
		ls := a.ls
		ls.Functions = a.funcs
		if a.funcs > 0 {
			ls.AvgComplexity = round1(float64(a.comp) / float64(a.funcs))
			ls.AvgCognitive = round1(float64(a.cog) / float64(a.funcs))
		}
		ls.WorstComplexity = a.worstComp
		ls.WorstCognitive = a.worstCog
		ls.MaxFunctionLength = a.maxLen
		ls.MaxNesting = a.maxNest
		ls.MaxParams = a.maxParam
		ls.CommentPct = commentPct(ls.CommentLines, ls.PhysicalSLOC)
		if ls.TotalTokens > 0 {
			ls.DupPct = round1(100 * float64(ls.DupTokens) / float64(ls.TotalTokens))
		}
		out[string(l)] = ls
	}
	return out
}

func commentPct(commentLines, sloc int) float64 {
	denom := sloc + commentLines
	if denom == 0 {
		return 0
	}
	return round1(100 * float64(commentLines) / float64(denom))
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

func buildFileRows(prod []*lang.FileResult, g *graph.Graph, d *dup.Result, verbose bool) []FileReport {
	byPath := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byPath[n.File.Path] = n
	}
	rows := make([]FileReport, 0, len(prod))
	for i, f := range prod {
		rows = append(rows, buildFileReport(f, byPath[f.Path], d.ByFile[i], d.DupLinesByFile[i], verbose))
	}
	return rows
}

func buildTestRows(test []*lang.FileResult) []FileReport {
	rows := make([]FileReport, 0, len(test))
	for _, f := range test {
		rows = append(rows, buildFileReport(f, nil, 0, 0, false))
	}
	return rows
}

func buildFileReport(fr *lang.FileResult, n *graph.Node, dupTokens, dupLines int, verbose bool) FileReport {
	row := FileReport{
		Path:            fr.Path,
		Lang:            string(fr.Lang),
		Class:           fr.Class.String(),
		Package:         fr.Package,
		ImportPath:      fr.ImportPath,
		TotalLines:      fr.TotalLines,
		PhysicalSLOC:    fr.PhysicalSLOC,
		LogicalLOC:      fr.LogicalLOC,
		CommentLines:    fr.CommentLines,
		CommentPct:      commentPct(fr.CommentLines, fr.PhysicalSLOC),
		Functions:       len(fr.Functions),
		DuplicatedLines: dupLines,
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
		sum, cog := 0, 0
		worst, worstCog := -1, -1
		worstName := ""
		for _, f := range fr.Functions {
			sum += f.Complexity
			cog += f.Cognitive
			if f.Complexity > worst {
				worst = f.Complexity
				worstName = f.Name
			}
			if f.Cognitive > worstCog {
				worstCog = f.Cognitive
			}
			if f.Length > row.MaxFunctionLength {
				row.MaxFunctionLength = f.Length
			}
			if f.Nesting > row.MaxNesting {
				row.MaxNesting = f.Nesting
			}
			if f.Params > row.MaxParams {
				row.MaxParams = f.Params
			}
			if verbose {
				row.FunctionsDetail = append(row.FunctionsDetail, FunctionReport{
					Name:       f.Name,
					Enclosing:  f.Enclosing,
					StartLine:  f.StartLine,
					EndLine:    f.EndLine,
					Complexity: f.Complexity,
					Cognitive:  f.Cognitive,
					Length:     f.Length,
					Nesting:    f.Nesting,
					Params:     f.Params,
					Anonymous:  f.Anonymous,
				})
			}
		}
		row.AvgComplexity = round2(float64(sum) / float64(len(fr.Functions)))
		row.WorstComplex = worst
		row.WorstFunc = worstName
		row.AvgCognitive = round2(float64(cog) / float64(len(fr.Functions)))
		row.WorstCognitive = worstCog
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
