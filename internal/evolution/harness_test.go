//go:build evolution

// Package evolution contains fixture-evolution tests (ADR 0013 revision):
// staged, snapshot-based mutations of tiny multi-language codebases whose
// metrics must move in a controlled, reviewable direction. These are opt-in
// (build tag evolution) and not part of the default test suite.
//
//	make evolve   # go test -tags evolution ./internal/evolution/...
package evolution

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/config"
)

// step is one point-in-time snapshot of the evolving codebase.
type step struct {
	name  string
	files map[string]string // rel path -> full source
}

// Report wraps the analysis results of every step of one evolution.
type Report struct {
	Step string
	*analysis.Report
}

// snapshot writes one step to a fresh temp dir and analyzes it.
func snapshot(t *testing.T, cfg *config.Config, s step) *analysis.Report {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(s.files))
	for p := range s.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s.files[p]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := analysis.Analyze(dir, cfg)
	if err != nil {
		t.Fatalf("analyze step %q: %v", s.name, err)
	}
	return r
}

// evolve runs every step in order and returns the per-step reports.
func evolve(t *testing.T, cfg *config.Config, steps []step) []Report {
	t.Helper()
	outs := make([]Report, 0, len(steps))
	for _, s := range steps {
		r := snapshot(t, cfg, s)
		outs = append(outs, Report{Step: s.name, Report: r})
	}
	return outs
}

// increasing asserts that values strictly increase across consecutive reports
// (or are all equal when first==last) and match the expected sequence.
func increasing(t *testing.T, label string, got []int, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d values %v, want %d values %v", label, len(got), got, len(want), want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("%s step %d: value = %d, want %d", label, i, got[i], w)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Errorf("%s must increase at step %d: %v", label, i, got)
		}
	}
}

func defaults() *config.Config {
	cfg := config.Defaults()
	cfg.Weights = map[string]float64{} // composite irrelevant in these tests
	return cfg
}
