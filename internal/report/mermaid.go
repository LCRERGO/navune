package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lcr/navune/internal/analysis"
)

// renderMermaid exports the internal file dependency graph as a Mermaid
// flowchart (ADR 0008). Files that participate in a cycle are grouped into
// per-cycle subgraphs so structural debt is visible. Only production files and
// internal edges are shown (ADR 0006, ADR 0010).
func renderMermaid(r *analysis.Report) (string, error) {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	cycleOf := map[string]int{}
	cycleMembers := map[int][]string{}
	for _, c := range r.Cycles {
		cycleMembers[c.Index] = c.Members
		for _, m := range c.Members {
			cycleOf[m] = c.Index
		}
	}

	nodeID := func(p string) string {
		return "f_" + sanitize(p)
	}
	label := func(p string) string {
		return shortPath(p)
	}

	// all referenced paths (nodes + edge endpoints)
	paths := map[string]bool{}
	for _, f := range r.Files {
		if f.Class == "prod" {
			paths[f.Path] = true
		}
	}
	for _, e := range r.InternalEdges {
		paths[e[0]] = true
		paths[e[1]] = true
	}

	var sorted []string
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	// nodes grouped into cycles as subgraphs
	var nonCycle []string
	emittedCycle := map[int]bool{}
	for _, p := range sorted {
		if ci, ok := cycleOf[p]; ok {
			if !emittedCycle[ci] {
				emittedCycle[ci] = true
				// find members sorted
				members := cycleMembers[ci]
				sort.Strings(members)
				fmt.Fprintf(&b, "  subgraph cycle%d[\"cycle-%d · %d files\"]\n", ci, ci, len(members))
				for _, m := range members {
					fmt.Fprintf(&b, "    %s[\"%s\"]\n", nodeID(m), label(m))
				}
				b.WriteString("  end\n")
			}
			continue
		}
		nonCycle = append(nonCycle, p)
	}
	for _, p := range nonCycle {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", nodeID(p), label(p))
	}

	// edges
	edgeSeen := map[[2]string]bool{}
	for _, e := range r.InternalEdges {
		from, to := e[0], e[1]
		if from == to {
			continue
		}
		key := [2]string{from, to}
		if edgeSeen[key] {
			continue
		}
		edgeSeen[key] = true
		fmt.Fprintf(&b, "  %s --> %s\n", nodeID(from), nodeID(to))
	}

	return b.String(), nil
}

// shortPath renders just the base file name plus enough of the directory to
// keep node labels readable.
func shortPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) < 3 {
		return p
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}
