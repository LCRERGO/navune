package analysis

import (
	"path/filepath"
	"testing"

	"github.com/lcr/navune/internal/config"
)

func TestAnalyzeTSFixture(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixture-ts")
	r, err := Analyze(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary
	if s.ProdFiles != 3 {
		t.Errorf("want 3 ts prod files, got %d", s.ProdFiles)
	}
	if s.TestFiles != 1 {
		t.Errorf("want 1 ts test file, got %d", s.TestFiles)
	}
	if s.Cycles != 1 || s.MaxCycleMembers != 3 {
		t.Errorf("want 1 3-file cycle, got cycles=%d max=%d", s.Cycles, s.MaxCycleMembers)
	}
	// interface Config is abstract in fixture-ts/src/gamma/c.ts
	foundAbstract := false
	for _, f := range r.Files {
		if f.Class == "prod" && f.AbstractTypes > 0 {
			foundAbstract = true
		}
	}
	if !foundAbstract {
		t.Error("TS interface should be counted as an abstract type")
	}
	// node_modules must be excluded
	for _, f := range r.Files {
		if containsSub(f.Path, "node_modules") {
			t.Errorf("node_modules file analyzed: %s", f.Path)
		}
	}
}

func TestAnalyzePyFixture(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixture-py")
	r, err := Analyze(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary
	if s.ProdFiles != 3 {
		t.Errorf("want 3 py prod files, got %d", s.ProdFiles)
	}
	if s.TestFiles != 1 {
		t.Errorf("want 1 py test file, got %d", s.TestFiles)
	}
	if s.Cycles != 1 || s.MaxCycleMembers != 3 {
		t.Errorf("want 1 3-file cycle, got cycles=%d max=%d", s.Cycles, s.MaxCycleMembers)
	}
	// .venv must be excluded
	for _, f := range r.Files {
		if containsSub(f.Path, ".venv") {
			t.Errorf(".venv file analyzed: %s", f.Path)
		}
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
