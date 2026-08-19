package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/gate"
)

func renderText(r *analysis.Report) string {
	var b strings.Builder
	s := r.Summary

	title := fmt.Sprintf("Navune — %s", r.Root)
	if r.InModule {
		title += fmt.Sprintf("  [module %s]", r.ModulePath)
	}
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", displayWidth(title)))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "Files:  %d production · %d test · %d generated skipped · %d dirs excluded\n",
		s.ProdFiles, s.TestFiles, s.GeneratedFiles, s.SkippedDirs)
	fmt.Fprintf(&b, "Size:   %d physical SLOC · %d logical LOC\n", s.PhysicalSLOC, s.LogicalLOC)
	fmt.Fprintf(&b, "Types:  %d (%d abstract)\n", s.Types, s.AbstractTypes)
	if s.Functions > 0 {
		fmt.Fprintf(&b, "Complexity:  avg %.2f / function · worst %d (%s)\n",
			s.AvgComplexity, s.WorstComplexity, s.WorstFunction)
	} else {
		b.WriteString("Complexity: no functions\n")
	}
	fmt.Fprintf(&b, "Duplication: %d / %d tokens (%.2f%%) · %d blocks\n",
		s.DupTokens, s.TotalTokens, s.DupPct, s.DupBlocks)
	fmt.Fprintf(&b, "Cycles:      %d component(s) · %d/%d files in cycle (%.2f%%) · largest %d\n",
		s.Cycles, s.FilesInCycle, s.ProdFiles, s.InCyclePct, s.MaxCycleMembers)

	writeCycles(&b, r)
	writeFileTable(&b, r)

	b.WriteString("\nQuality gate (budgets):\n")
	if len(r.Budgets) == 0 {
		b.WriteString("  (none configured)\n")
	}
	for _, br := range r.Budgets {
		status := "ok"
		if br.Breached {
			status = "BREACHED"
		}
		m, ok := gate.MetricByKey[br.Key]
		prec := 0
		if ok {
			prec = m.Precision
		}
		fmt.Fprintf(&b, "  [%s] %-26s %12s / %-10s  %s\n",
			padTier(br.Tier), br.Key, formatNum(br.Value, prec), formatNum(br.Limit, prec), status)
	}
	fmt.Fprintf(&b, "\nComposite index: %.1f / 100\n", r.Composite)
	fmt.Fprintf(&b, "Exit code: %d (%s)\n", r.ExitCode, exitWord(r.ExitCode))
	return b.String()
}

// writeCycles prints each cyclic component.
func writeCycles(b *strings.Builder, r *analysis.Report) {
	if len(r.Cycles) == 0 {
		return
	}
	b.WriteString("\nCyclic dependencies (structural debt):\n")
	for _, c := range r.Cycles {
		fmt.Fprintf(b, "  cycle-%d (%d files): %s\n", c.Index, c.Size, strings.Join(c.Members, " -> "))
	}
}

// writeFileTable lists production files with headline metrics.
func writeFileTable(b *strings.Builder, r *analysis.Report) {
	type row struct {
		path    string
		sloc    int
		worst   int
		dupPct  float64
		inCycle bool
	}
	var rows []row
	for _, f := range r.Files {
		if f.Class != "prod" {
			continue
		}
		rows = append(rows, row{f.Path, f.PhysicalSLOC, f.WorstComplex, f.DupPct, f.InCycle})
	}
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })

	b.WriteString("\nProduction files:\n")
	fmt.Fprintf(b, "  %-52s %7s %6s %7s %6s\n", "file", "SLOC", "worst", "dup%", "cycle")
	for _, rw := range rows {
		cyc := ""
		if rw.inCycle {
			cyc = "y"
		}
		fmt.Fprintf(b, "  %-52s %7d %6d %6.2f%% %6s\n", rw.path, rw.sloc, rw.worst, rw.dupPct, cyc)
	}
}
