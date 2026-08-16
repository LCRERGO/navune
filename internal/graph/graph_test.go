package graph

import (
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func goFile(path, pkg, importPath string, imports []string) *lang.FileResult {
	return &lang.FileResult{
		Path:       path,
		Package:    pkg,
		ImportPath: importPath,
		Imports:    imports,
	}
}

func TestThreePackageCycle(t *testing.T) {
	files := []*lang.FileResult{
		goFile("a/a.go", "a", "m/a", []string{"m/b"}),
		goFile("b/b.go", "b", "m/b", []string{"m/c"}),
		goFile("c/c.go", "c", "m/c", []string{"m/a"}),
	}
	g := Build(files)
	if len(g.Cycles) != 1 {
		t.Fatalf("want 1 cycle, got %d", len(g.Cycles))
	}
	if len(g.Cycles[0].Members) != 3 {
		t.Fatalf("want 3 members in cycle, got %d", len(g.Cycles[0].Members))
	}
	for _, n := range g.Nodes {
		if !n.InCycle {
			t.Errorf("%s should be in cycle", n.File.Path)
		}
		if n.Ca != 1 || n.Ce != 1 {
			t.Errorf("%s want Ca=1 Ce=1, got Ca=%d Ce=%d", n.File.Path, n.Ca, n.Ce)
		}
	}
}

func TestExternalImportIgnored(t *testing.T) {
	files := []*lang.FileResult{
		goFile("a/a.go", "a", "m/a", []string{"net/http", "m/b"}),
		goFile("b/b.go", "b", "m/b", nil),
	}
	g := Build(files)
	if len(g.Cycles) != 0 {
		t.Fatalf("want 0 cycles, got %d", len(g.Cycles))
	}
	a := g.Nodes[0]
	if a.Ce != 1 { // only m/b is internal
		t.Errorf("Ce want 1 (internal only), got %d", a.Ce)
	}
	if a.Instability != 1 {
		t.Errorf("instability of a (Ce=1,Ca=0) want 1, got %v", a.Instability)
	}
}

func TestPackageFlattening(t *testing.T) {
	// a imports package pkg with two files -> a depends on both files.
	files := []*lang.FileResult{
		goFile("a/a.go", "a", "m/a", []string{"m/p"}),
		goFile("p/p1.go", "p", "m/p", nil),
		goFile("p/p2.go", "p", "m/p", nil),
	}
	g := Build(files)
	a := g.Nodes[0]
	if a.Ce != 2 {
		t.Errorf("file in package a importing m/p flattens to 2 edges, got %d", a.Ce)
	}
	for _, n := range g.Nodes[1:] {
		if n.Ca != 1 {
			t.Errorf("p/%s want Ca=1, got %d", n.File.Path, n.Ca)
		}
	}
}

func TestNoSelfEdge(t *testing.T) {
	files := []*lang.FileResult{
		goFile("a/a.go", "a", "m/a", []string{"m/a"}), // impossible but guard
	}
	g := Build(files)
	if len(g.Cycles) != 0 {
		t.Fatalf("self import must not create a cycle, got %d", len(g.Cycles))
	}
}

func TestAcyclicTwoFiles(t *testing.T) {
	files := []*lang.FileResult{
		goFile("a/a.go", "a", "m/a", []string{"m/b"}),
		goFile("b/b.go", "b", "m/b", nil),
	}
	g := Build(files)
	if len(g.Cycles) != 0 {
		t.Fatalf("want 0 cycles, got %d", len(g.Cycles))
	}
	b := g.Nodes[1]
	if b.Ca != 1 || b.Ce != 0 {
		t.Errorf("b want Ca=1 Ce=0 got Ca=%d Ce=%d", b.Ca, b.Ce)
	}
	if b.Instability != 0 {
		t.Errorf("b instability want 0, got %v", b.Instability)
	}
}
