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

	"github.com/lcr/navune/internal/lang"
	"github.com/lcr/navune/internal/smell"
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

	// Smells configure the structural smell layer (ADR 0016).
	Smells map[string]smell.RuleOverride `yaml:"smells"`
	// LanguageSmells override rule fields per language.
	LanguageSmells map[string]map[string]smell.RuleOverride `yaml:"language_smells"`
	// LanguageBudgets add per-language limits evaluated alongside the global
	// budgets.
	LanguageBudgets map[string]map[string]Budget `yaml:"language_budgets"`
	// IncludePaths are C/C++ #include search directories, relative to this
	// config file (ADR 0017).
	IncludePaths []string `yaml:"include_paths"`

	dir string // directory of the loaded config file, for relative resolution
}

// IncludeDirs returns the include search directories as absolute paths.
func (c *Config) IncludeDirs() []string {
	out := make([]string, 0, len(c.IncludePaths))
	for _, p := range c.IncludePaths {
		if filepath.IsAbs(p) {
			out = append(out, filepath.Clean(p))
			continue
		}
		base := c.dir
		if base == "" {
			base = "."
		}
		out = append(out, filepath.Clean(filepath.Join(base, p)))
	}
	return out
}

// SmellSettings resolves the built-in per-language defaults with the config's
// global and per-language overrides.
func (c *Config) SmellSettings() smell.Settings {
	return smell.Resolve(c.Smells, c.LanguageSmells)
}

// LanguageBudget returns the configured per-language budgets keyed by canonical
// language.
func (c *Config) LanguageBudget() map[lang.Lang]map[string]Budget {
	out := map[lang.Lang]map[string]Budget{}
	for name, b := range c.LanguageBudgets {
		l, ok := lang.Canonical(name)
		if !ok {
			continue
		}
		out[l] = b
	}
	return out
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
	if abs, err := filepath.Abs(path); err == nil {
		cfg.dir = filepath.Dir(abs)
	}
	return cfg, nil
}

// Defaults returns the built-in configuration. Budgets are the quality gate
// that ships with the binary; weights define the composite blend.
func Defaults() *Config {
	return &Config{
		Version: 1,
		Exclude: []string{".git", "vendor", "node_modules", "testdata", "__pycache__", ".venv", "venv", "dist", "build", "target", ".gradle", "out"},
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
# node_modules testdata __pycache__ .venv venv dist build target .gradle out).
# Generated code (e.g. "// Code generated ... DO NOT EDIT.") is always skipped.
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

# Structural smell rules (ADR 0016). All are enabled by default; none gate
# unless a matching budget is configured. Each entry accepts enabled,
# threshold and severity (blocker|high|medium|low|info).
smells:
  cognitive-complexity:  { enabled: true, threshold: 15, severity: high }
  cyclomatic-complexity: { enabled: true, threshold: 10, severity: medium }
  function-length:       { enabled: true, threshold: 60, severity: high }
  nesting-depth:         { enabled: true, threshold: 4,  severity: medium }
  parameter-count:       { enabled: true, threshold: 7,  severity: medium }
  file-length:           { enabled: true, threshold: 750, severity: low }
  duplicated-file:       { enabled: true, threshold: 10, severity: medium }
  comment-density:       { enabled: true, threshold: 25, severity: low }
  boolean-complexity:    { enabled: true, threshold: 3,  severity: medium }
  too-many-methods:      { enabled: true, threshold: 35, severity: low }

# Per-language threshold overrides (full rule objects; fields not set inherit
# from the built-in per-language default, then the global smells entry).
language_smells:
  python:
    function-length: { threshold: 50 }
  typescript:
    nesting-depth:   { threshold: 3 }

# Per-language budgets, evaluated over that language's production files in
# addition to the global budgets above. The composite uses global values only.
language_budgets:
  python:
    avg_complexity: { limit: 8, tier: warn }

# C/C++ #include search directories, relative to this config file (ADR 0017).
include_paths:
  - include
`
}
