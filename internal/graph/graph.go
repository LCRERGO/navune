// Package graph builds the internal dependency graph between production
// files (ADR 0006: file is the unit), finds cyclic dependencies (strongly
// connected components), and computes Martin's coupling metrics Ca/Ce,
// instability, abstractness and distance from the main sequence
// (ADR 0010: internal edges only).
package graph

import (
	"sort"

	"github.com/lcr/navune/internal/lang"
)

// Node is one production file in the dependency graph.
type Node struct {
	File        *lang.FileResult
	Index       int
	Ca          int     // afferent coupling: distinct internal files depending on this one
	Ce          int     // efferent coupling: distinct internal files this one depends on
	Instability float64 // Ce / (Ca + Ce); 0 stable, 1 unstable
	Abstract    float64 // fraction of abstract types (Go interfaces)
	Distance    float64 // |A + I - 1|
	InCycle     bool
	CycleIndex  int // -1 when not in a cycle
}

// Cycle is a strongly connected component of size > 1.
type Cycle struct {
	Index   int
	Members []*Node
}

// Graph is the internal file-level dependency graph of a codebase.
type Graph struct {
	Nodes   []*Node
	byPath  map[string]int
	succ    []map[int]struct{}
	pred    []map[int]struct{}
	Cycles  []*Cycle
	cycleID []int // per-node index into Cycles, -1 if none
}

// Build constructs the graph from production files only. A file F imports
// path P; if P resolves to analyzed production files (internal), F gains an
// edge to every one of those files (package-level imports flattened to file
// granularity, ADR 0006). External and stdlib imports never create edges
// (ADR 0010).
func Build(files []*lang.FileResult) *Graph {
	g := &Graph{byPath: map[string]int{}}
	for i, f := range files {
		n := &Node{File: f, Index: i, CycleIndex: -1}
		g.Nodes = append(g.Nodes, n)
		g.byPath[f.Path] = i
	}

	// package import path -> production files in that package
	pkgFiles := map[string][]int{}
	for i, n := range g.Nodes {
		if n.File.ImportPath == "" {
			continue
		}
		pkgFiles[n.File.ImportPath] = append(pkgFiles[n.File.ImportPath], i)
	}

	g.succ = make([]map[int]struct{}, len(g.Nodes))
	g.pred = make([]map[int]struct{}, len(g.Nodes))
	for i := range g.Nodes {
		g.succ[i] = map[int]struct{}{}
		g.pred[i] = map[int]struct{}{}
	}

	for _, n := range g.Nodes {
		self := n.Index
		for _, imp := range n.File.Imports {
			if imp == n.File.ImportPath {
				continue // a package cannot import itself
			}
			targets, ok := pkgFiles[imp]
			if !ok {
				continue // external / not analyzed
			}
			for _, t := range targets {
				if t == self {
					continue
				}
				g.succ[self][t] = struct{}{}
				g.pred[t][self] = struct{}{}
			}
		}
	}

	for _, n := range g.Nodes {
		n.Ce = len(g.succ[n.Index])
		n.Ca = len(g.pred[n.Index])
		sum := n.Ca + n.Ce
		if sum > 0 {
			n.Instability = float64(n.Ce) / float64(sum)
		}
		types := 0
		abstract := 0
		for _, t := range n.File.Types {
			types++
			if t.Abstract {
				abstract++
			}
		}
		if types > 0 {
			n.Abstract = float64(abstract) / float64(types)
		}
		d := n.Abstract + n.Instability - 1
		if d < 0 {
			d = -d
		}
		n.Distance = d
	}

	g.findCycles()
	g.computeCycleFlags()
	return g
}

// findCycles runs Tarjan's SCC algorithm and records components of size > 1.
func (g *Graph) findCycles() {
	n := len(g.Nodes)
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	var stack []int
	idx := 0
	g.cycleID = make([]int, n)
	for i := range g.cycleID {
		g.cycleID[i] = -1
	}

	var strongconnect func(v int)
	strongconnect = func(v int) {
		index[v] = idx
		low[v] = idx
		idx++
		stack = append(stack, v)
		onStack[v] = true

		for w := range g.succ[v] {
			if index[w] == 0 {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] {
				if index[w] < low[v] {
					low[v] = index[w]
				}
			}
		}

		if low[v] == index[v] {
			// v is the root of an SCC
			comp := []int{}
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 {
				cyc := &Cycle{Index: len(g.Cycles), Members: []*Node{}}
				sort.Ints(comp)
				for _, w := range comp {
					cyc.Members = append(cyc.Members, g.Nodes[w])
					g.cycleID[w] = cyc.Index
				}
				g.Cycles = append(g.Cycles, cyc)
			}
		}
	}

	for v := 0; v < n; v++ {
		if index[v] == 0 {
			strongconnect(v)
		}
	}
}

func (g *Graph) computeCycleFlags() {
	for _, c := range g.Cycles {
		for _, m := range c.Members {
			m.InCycle = true
			m.CycleIndex = c.Index
		}
	}
}

// FilesInCycles returns the sorted list of distinct paths in any cycle.
func (g *Graph) FilesInCycles() []string {
	var paths []string
	for _, c := range g.Cycles {
		for _, m := range c.Members {
			paths = append(paths, m.File.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

// Successors returns the indices of files the given node depends on.
func (g *Graph) Successors(i int) map[int]struct{} {
	if i < 0 || i >= len(g.succ) {
		return nil
	}
	return g.succ[i]
}
