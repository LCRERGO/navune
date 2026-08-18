package analysis

import (
	"path/filepath"
	"testing"

	"github.com/lcr/navune/internal/config"
)

func TestAnalyzeFixture(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixture")
	cfg := config.Defaults()
	r, err := Analyze(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != 1 {
		t.Errorf("want schema_version 1, got %d", r.SchemaVersion)
	}
	s := r.Summary
	if s.ProdFiles != 5 {
		t.Errorf("want 5 prod files, got %d", s.ProdFiles)
	}
	if s.TestFiles != 1 {
		t.Errorf("want 1 test file, got %d", s.TestFiles)
	}
	if s.GeneratedFiles != 1 {
		t.Errorf("want 1 generated file skipped, got %d", s.GeneratedFiles)
	}
	if s.SkippedDirs < 1 {
		t.Errorf("vendor dir should be excluded, skipped=%d", s.SkippedDirs)
	}
	// cycle between graph/alpha, graph/beta, graph/gamma
	if s.Cycles != 1 {
		t.Errorf("want 1 cycle, got %d", s.Cycles)
	}
	if s.MaxCycleMembers != 3 {
		t.Errorf("want max cycle members 3, got %d", s.MaxCycleMembers)
	}
	// worst function Helper in alpha has complexity 5
	if s.WorstComplexity != 5 {
		t.Errorf("want worst complexity 5, got %d", s.WorstComplexity)
	}
	// duplication detected (magnitude funcs duplicated)
	if s.DupBlocks < 1 {
		t.Errorf("expected at least 1 duplication block, got %d", s.DupBlocks)
	}
	if r.InModule != true {
		t.Error("fixture is inside a go module")
	}
	if s.FilesInCycle != 3 {
		t.Errorf("want 3 files in cycle, got %d", s.FilesInCycle)
	}
}

func TestFilesRelativePaths(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixture")
	r, err := Analyze(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Files {
		if filepath.IsAbs(f.Path) {
			t.Errorf("file path should be relative to analysis root, got %q", f.Path)
		}
	}
}
