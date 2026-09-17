// Package gate turns measured codebase metrics into a verdict: per-metric
// budget breaches with severity, an exit code, and the transparent composite
// 0-100 index (ADR 0009). The composite is a weighted blend of per-metric
// distance-from-budget scores, never an invented industry formula.
package gate

import (
	"fmt"
	"math"
	"sort"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/lang"
	"github.com/lcr/navune/internal/smell"
)

// Exit codes (ADR 0012).
const (
	ExitPass     = 0 // all error-tier budgets satisfied
	ExitBreach   = 1 // at least one error-tier budget breached
	ExitUsage    = 2 // usage or configuration error
	ExitInternal = 3
)

// Metric describes one budgetable quality metric. All metrics are
// "smaller is better": value > limit means a breach.
type Metric struct {
	Key       string
	Label     string
	Unit      string
	Precision int
}

// Registry lists every budgetable metric Navune can measure. The first five
// ship with default budgets; the rest are opt-in (ADR 0016).
var Registry = []Metric{
	{"avg_complexity", "Average cyclomatic complexity (per function)", "", 2},
	{"worst_complexity", "Worst cyclomatic complexity (single function)", "", 0},
	{"max_duplication_pct", "Duplicated tokens, % of all tokens", "%", 2},
	{"max_cycle_members", "Largest dependency cycle (files)", "files", 0},
	{"max_in_cycle_pct", "Files that sit in a dependency cycle", "%", 2},
	{"avg_cognitive_complexity", "Average cognitive complexity (per function)", "", 2},
	{"worst_cognitive_complexity", "Worst cognitive complexity (single function)", "", 0},
	{"max_function_length", "Longest function (non-comment lines)", "lines", 0},
	{"max_nesting", "Deepest control-structure nesting", "", 0},
	{"max_params", "Largest parameter count", "", 0},
	{"max_duplicated_lines_pct", "Duplicated lines, % of physical lines", "%", 2},
	{"max_issues", "Total structural smell issues", "issues", 0},
	{"max_blocker_issues", "Blocker-severity issues", "issues", 0},
	{"max_high_issues", "High-severity issues", "issues", 0},
	{"max_medium_issues", "Medium-severity issues", "issues", 0},
	{"max_low_issues", "Low-severity issues", "issues", 0},
	{"max_info_issues", "Info-severity issues", "issues", 0},
}

// MetricByKey indexes Registry.
var MetricByKey = map[string]Metric{}

func init() {
	for _, m := range Registry {
		MetricByKey[m.Key] = m
	}
}

// BudgetResult is the evaluation of one configured budget. Language is "" for
// global budgets and the canonical language key for per-language budgets.
type BudgetResult struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Language string  `json:"language,omitempty"`
	Limit    float64 `json:"limit"`
	Value    float64 `json:"value"`
	Tier     string  `json:"tier"`
	Breached bool    `json:"breached"`
}

// Verdict is the outcome of evaluating all budgets against measured values.
type Verdict struct {
	Budgets  []BudgetResult `json:"budgets"`
	Breaches []BudgetResult `json:"breaches"`
	Score    float64        `json:"composite_score"` // 0-100, higher is better
	ExitCode int            `json:"exit_code"`
}

// Validate ensures every budget/weight key in the config is known, and that
// smell rules, severities, and language keys are valid.
func Validate(cfg *config.Config) error {
	for _, k := range cfg.BudgetKeys() {
		if _, ok := MetricByKey[k]; !ok {
			return fmt.Errorf("unknown budget metric %q (known: %v)", k, metricKeys())
		}
	}
	for _, k := range cfg.WeightKeys() {
		if _, ok := MetricByKey[k]; !ok {
			return fmt.Errorf("unknown weight metric %q (known: %v)", k, metricKeys())
		}
	}
	if err := validateSmells(cfg); err != nil {
		return err
	}
	for name, budgets := range cfg.LanguageBudgets {
		if _, ok := lang.Canonical(name); !ok {
			return fmt.Errorf("unknown language %q in language_budgets", name)
		}
		for k := range budgets {
			if _, ok := MetricByKey[k]; !ok {
				return fmt.Errorf("unknown budget metric %q in language_budgets.%s (known: %v)", k, name, metricKeys())
			}
		}
	}
	return nil
}

func validateSmells(cfg *config.Config) error {
	check := func(where string, rules map[string]smell.RuleOverride) error {
		for id, o := range rules {
			if _, ok := smell.RuleByID(id); !ok {
				return fmt.Errorf("unknown smell rule %q in %s (known: %v)", id, where, smell.RuleIDs())
			}
			if o.Severity != "" {
				if _, ok := smell.ParseSeverity(o.Severity); !ok {
					return fmt.Errorf("invalid severity %q for smell rule %q in %s", o.Severity, id, where)
				}
			}
		}
		return nil
	}
	if err := check("smells", cfg.Smells); err != nil {
		return err
	}
	for name, rules := range cfg.LanguageSmells {
		if _, ok := lang.Canonical(name); !ok {
			return fmt.Errorf("unknown language %q in language_smells", name)
		}
		if err := check("language_smells."+name, rules); err != nil {
			return err
		}
	}
	return nil
}

func metricKeys() []string {
	ks := make([]string, 0, len(Registry))
	for _, m := range Registry {
		ks = append(ks, m.Key)
	}
	return ks
}

// Evaluate checks measured values against cfg budgets and computes the
// composite index.
//
// Composite: for each metric with a budget limit, metric_score =
// clamp(100 * (1 - value/limit), 0, 100) (value == limit scores 0, value == 0
// scores 100). The composite is the weighted mean of metric_scores using the
// config weights (unweighted metrics ignored). Budgets decide the exit code:
// a breach of any error-tier budget sets it to ExitBreach; warn-tier breaches
// are reported but do not fail the run.
func Evaluate(cfg *config.Config, measured map[string]float64) *Verdict {
	v := &Verdict{Score: 100, ExitCode: ExitPass}
	keys := cfg.BudgetKeys()
	totalWeight := 0.0
	weighted := 0.0
	weights := cfg.Weights
	if len(keys) > 0 {
		for _, k := range keys {
			if w, ok := weights[k]; ok {
				totalWeight += w
			}
		}
	}

	for _, k := range keys {
		b := cfg.Budgets[k]
		value := measured[k]
		br := BudgetResult{
			Key:   k,
			Label: MetricByKey[k].Label,
			Limit: b.Limit,
			Value: value,
			Tier:  b.Tier,
		}
		if value > b.Limit {
			br.Breached = true
			v.Breaches = append(v.Breaches, br)
			if b.Tier == "error" {
				v.ExitCode = ExitBreach
			}
		}
		v.Budgets = append(v.Budgets, br)

		if w, ok := weights[k]; ok && b.Limit > 0 {
			score := 100 * (1 - value/b.Limit)
			score = math.Max(0, math.Min(100, score))
			weighted += w * score
		}
	}

	if totalWeight > 0 {
		v.Score = weighted / totalWeight
	}
	sortBudgets(v.Budgets)
	return v
}

// EvaluateLanguages appends per-language budget results (evaluated over that
// language's production files) to the verdict and applies their error-tier
// breaches to the exit code. The composite is unaffected (ADR 0016).
func (v *Verdict) EvaluateLanguages(cfg *config.Config, measured map[lang.Lang]map[string]float64) {
	langs := cfg.LanguageBudget()
	names := make([]string, 0, len(langs))
	for l := range langs {
		names = append(names, string(l))
	}
	sort.Strings(names)
	for _, name := range names {
		l, _ := lang.Canonical(name)
		budgets := langs[l]
		keys := make([]string, 0, len(budgets))
		for k := range budgets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b := budgets[k]
			value := measured[l][k]
			br := BudgetResult{
				Key:      k,
				Label:    MetricByKey[k].Label,
				Language: string(l),
				Limit:    b.Limit,
				Value:    value,
				Tier:     b.Tier,
			}
			if value > b.Limit {
				br.Breached = true
				v.Breaches = append(v.Breaches, br)
				if b.Tier == "error" {
					v.ExitCode = ExitBreach
				}
			}
			v.Budgets = append(v.Budgets, br)
		}
	}
	sortBudgets(v.Budgets)
}

func sortBudgets(b []BudgetResult) {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Language != b[j].Language {
			return b[i].Language < b[j].Language
		}
		return b[i].Key < b[j].Key
	})
}
