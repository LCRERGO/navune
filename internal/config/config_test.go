package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsSane(t *testing.T) {
	cfg := Defaults()
	if cfg.Version != 1 {
		t.Errorf("want version 1, got %d", cfg.Version)
	}
	for _, k := range []string{"avg_complexity", "worst_complexity", "max_duplication_pct", "max_cycle_members", "max_in_cycle_pct"} {
		if _, ok := cfg.Budgets[k]; !ok {
			t.Errorf("missing default budget %s", k)
		}
	}
	if cfg.WeightSum() == 0 {
		t.Error("weights must not sum to zero")
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Budgets) == 0 {
		t.Error("missing file must still produce default budgets")
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigName)
	if err := os.WriteFile(p, []byte(Template()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 {
		t.Errorf("template version want 1, got %d", cfg.Version)
	}
	if len(cfg.Budgets) == 0 {
		t.Error("template should parse budgets")
	}
}

func TestDiscoverUpward(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ConfigName)); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(dir, ConfigName), []byte(Template()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Discover(sub)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ConfigName)
	if got != want {
		t.Errorf("want discovered config %s, got %s", want, got)
	}
}

func TestIsExcludedDir(t *testing.T) {
	cfg := Defaults()
	for _, d := range []string{".git", "vendor", "node_modules"} {
		if !cfg.IsExcludedDir(d) {
			t.Errorf("expected %s to be excluded by default", d)
		}
	}
	if cfg.IsExcludedDir("internal") {
		t.Error("internal should not be excluded by default")
	}
}
