// Package config loads navune.yaml, applies defaults, and defines the budget
// and weight model that gates and the composite index are derived from
// (ADR 0009).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigName = "navune.yaml"

// Budget is a per-metric limit. Tier is "error" or "warn".
// All budgets are upper bounds: smaller measured values are better.
type Budget struct {
	Limit float64 `yaml:"limit"`
	Tier  string  `yaml:"tier"`
}

// Config is the schema of navune.yaml.
type Config struct {
	Version int                `yaml:"version"`
	Exclude []string           `yaml:"exclude"`
	Budgets map[string]Budget  `yaml:"budgets"`
	Weights map[string]float64 `yaml:"weights"`
}

// Load reads, validates, and fills defaults for the given file.
// A missing file is not an error: it returns the default configuration.
func Load(path string) (*Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported navune config version %d (want 1)", cfg.Version)
	}
	return cfg, nil
}

// Defaults returns the built-in configuration. Budgets are the quality gate
// that ships with the binary; weights define the composite blend.
func Defaults() *Config {
	return &Config{
		Version: 1,
		Exclude: []string{".git", "vendor", "node_modules", "testdata"},
		Budgets: map[string]Budget{
			"avg_complexity":      {Limit: 10, Tier: "error"},
			"worst_complexity":    {Limit: 50, Tier: "error"},
			"max_duplication_pct": {Limit: 5, Tier: "warn"},
			"max_cycle_members":   {Limit: 8, Tier: "error"},
			"max_in_cycle_pct":    {Limit: 10, Tier: "warn"},
		},
		Weights: map[string]float64{
			"avg_complexity":      25,
			"worst_complexity":    15,
			"max_duplication_pct": 20,
			"max_cycle_members":   20,
			"max_in_cycle_pct":    20,
		},
	}
}

// Discover walks upward from dir looking for a navune.yaml file.
func Discover(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(abs, ConfigName)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", nil
		}
		abs = parent
	}
}

// BudgetKeys returns sorted budget metric keys.
func (c *Config) BudgetKeys() []string {
	ks := make([]string, 0, len(c.Budgets))
	for k := range c.Budgets {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// WeightKeys returns sorted weight metric keys.
func (c *Config) WeightKeys() []string {
	ks := make([]string, 0, len(c.Weights))
	for k := range c.Weights {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// WeightSum returns the total weight across all metrics.
func (c *Config) WeightSum() float64 {
	s := 0.0
	for _, w := range c.Weights {
		s += w
	}
	return s
}

// IsExcludedDir reports whether a directory basename is excluded.
func (c *Config) IsExcludedDir(name string) bool {
	for _, pat := range c.Exclude {
		if strings.ContainsAny(pat, "*?[") {
			if ok, _ := filepath.Match(pat, name); ok {
				return true
			}
			continue
		}
		if name == pat || strings.HasPrefix(name, pat+"/") {
			return true
		}
	}
	return false
}

// Template renders a commented example navune.yaml for `navune init`.
func Template() string {
	return `# Navune configuration (navune.yaml)
# Budgets are quality-gate limits: smaller measured values are better.
# A breach of an "error"-tier budget makes navune exit 1. Weights blend the
# per-metric distance-from-budget scores into the composite 0-100 index.
version: 1

# Additional exclusion globs/directory names (built-ins: .git vendor
# node_modules testdata). Generated code (e.g. "// Code generated ... DO NOT
# EDIT.") is always skipped.
exclude:
  - "legacy"
  - "**/migrations/**"

budgets:
  avg_complexity:        { limit: 10, tier: error }   # avg cyclomatic complexity per function
  worst_complexity:      { limit: 50, tier: error }   # worst single function complexity
  max_duplication_pct:   { limit: 5,  tier: warn  }   # duplicated tokens, % of all tokens
  max_cycle_members:     { limit: 8,  tier: error }   # largest dependency cycle (SCC)
  max_in_cycle_pct:      { limit: 10, tier: warn  }   # % of files that sit in a cycle

weights:
  avg_complexity:        25
  worst_complexity:      15
  max_duplication_pct:   20
  max_cycle_members:     20
  max_in_cycle_pct:      20
`
}
