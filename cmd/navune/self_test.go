// Package main_test is the always-on self-analysis smoke test (ADR 0013):
// Navune analyzes its own source tree on every test run as a real-world
// exercise of the full pipeline.
package main_test

import (
	"path/filepath"
	"testing"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/config"
)

func TestSelfAnalysis(t *testing.T) {
	root := filepath.Join("..", "..") // repo root
	cfg := config.Defaults()
	// our own budgets are intentionally generous; the point is that the
	// pipeline completes on a real, non-trivial codebase and every metric is
	// produced, not that the gate passes.
	r, err := analysis.Analyze(root, cfg)
	if err != nil {
		t.Fatalf("self-analysis failed: %v", err)
	}
	if r.SchemaVersion != 1 {
		t.Errorf("schema version want 1, got %d", r.SchemaVersion)
	}
	if r.Summary.ProdFiles < 10 {
		t.Errorf("self-analysis should find many files, got %d", r.Summary.ProdFiles)
	}
	if r.Summary.Functions == 0 {
		t.Error("self-analysis should find functions")
	}
	if r.Summary.PhysicalSLOC == 0 {
		t.Error("self-analysis should count physical SLOC")
	}
	if len(r.Files) == 0 {
		t.Error("self-analysis should produce file rows")
	}
	if len(r.Budgets) == 0 {
		t.Error("default config has budgets")
	}
	if r.ModulePath == "" {
		t.Errorf("self root should be in a module, got %q", r.ModulePath)
	}
	for _, f := range r.Files {
		if filepath.IsAbs(f.Path) {
			t.Errorf("self-analysis paths must be relative, got %q", f.Path)
		}
	}
}
