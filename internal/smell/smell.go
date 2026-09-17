// Package smell implements Navune's deterministic structural smell layer
// (ADR 0016): threshold rules over the measures of ADR 0015. Every finding is
// reproducible from a reported number; there is no pattern database.
package smell

import (
	"sort"

	"github.com/lcr/navune/internal/lang"
)

// Severity is the MQR severity scale.
type Severity string

const (
	Blocker Severity = "blocker"
	High    Severity = "high"
	Medium  Severity = "medium"
	Low     Severity = "low"
	Info    Severity = "info"
)

// Rule describes one smell rule and its per-language default thresholds.
type Rule struct {
	ID       string
	Target   string // function | file | expression | type
	Severity Severity
	Defaults map[lang.Lang]float64
}

func all(v float64) map[lang.Lang]float64 {
	m := map[lang.Lang]float64{}
	for _, l := range lang.Languages() {
		m[l] = v
	}
	return m
}

func with(base float64, over map[lang.Lang]float64) map[lang.Lang]float64 {
	m := all(base)
	for k, v := range over {
		m[k] = v
	}
	return m
}

// Catalog is the ordered rule catalog (ADR 0016). All rules are enabled by
// default; none gate unless a budget is configured.
var Catalog = []Rule{
	{ID: "cognitive-complexity", Target: "function", Severity: High, Defaults: all(15)},
	{ID: "cyclomatic-complexity", Target: "function", Severity: Medium, Defaults: all(10)},
	{ID: "function-length", Target: "function", Severity: High, Defaults: with(60, map[lang.Lang]float64{lang.Python: 50})},
	{ID: "nesting-depth", Target: "function", Severity: Medium, Defaults: with(4, map[lang.Lang]float64{lang.TypeScript: 3, lang.JavaScript: 3})},
	{ID: "parameter-count", Target: "function", Severity: Medium, Defaults: all(7)},
	{ID: "file-length", Target: "file", Severity: Low, Defaults: all(750)},
	{ID: "duplicated-file", Target: "file", Severity: Medium, Defaults: all(10)},
	{ID: "comment-density", Target: "file", Severity: Low, Defaults: all(25)},
	{ID: "boolean-complexity", Target: "expression", Severity: Medium, Defaults: all(3)},
	{ID: "too-many-methods", Target: "type", Severity: Low, Defaults: all(35)},
}

// CommentFloor is the minimum physical SLOC a file needs before comment-density
// is evaluated (ADR 0016).
const CommentFloor = 50

// RuleIDs returns the catalog rule ids.
func RuleIDs() []string {
	ids := make([]string, 0, len(Catalog))
	for _, r := range Catalog {
		ids = append(ids, r.ID)
	}
	return ids
}

// RuleByID returns the rule metadata for id.
func RuleByID(id string) (Rule, bool) {
	for _, r := range Catalog {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}

// RuleOverride is a partial rule configuration from navune.yaml. Nil fields
// inherit from the next level down (language -> global -> built-in default).
type RuleOverride struct {
	Enabled   *bool    `yaml:"enabled"`
	Threshold *float64 `yaml:"threshold"`
	Severity  string   `yaml:"severity"`
}

// RuleConfig is a fully resolved rule configuration.
type RuleConfig struct {
	Enabled   bool
	Threshold float64
	Severity  Severity
}

// Settings maps each language to its resolved rule configurations.
type Settings map[lang.Lang]map[string]RuleConfig

// Resolve merges global overrides, then per-language overrides, on top of the
// built-in per-language defaults (ADR 0016).
func Resolve(global map[string]RuleOverride, perLang map[string]map[string]RuleOverride) Settings {
	// normalize per-language keys to canonical Langs
	langOverrides := map[lang.Lang]map[string]RuleOverride{}
	for name, rules := range perLang {
		l, ok := lang.Canonical(name)
		if !ok {
			continue
		}
		langOverrides[l] = rules
	}

	out := Settings{}
	for _, l := range lang.Languages() {
		resolved := map[string]RuleConfig{}
		for _, r := range Catalog {
			rc := RuleConfig{Enabled: true, Threshold: r.Defaults[l], Severity: r.Severity}
			if o, ok := global[r.ID]; ok {
				apply(&rc, o)
			}
			if o, ok := langOverrides[l][r.ID]; ok {
				apply(&rc, o)
			}
			resolved[r.ID] = rc
		}
		out[l] = resolved
	}
	return out
}

func apply(rc *RuleConfig, o RuleOverride) {
	if o.Enabled != nil {
		rc.Enabled = *o.Enabled
	}
	if o.Threshold != nil {
		rc.Threshold = *o.Threshold
	}
	if o.Severity != "" {
		if s, ok := ParseSeverity(o.Severity); ok {
			rc.Severity = s
		}
	}
}

// ParseSeverity validates a severity string.
func ParseSeverity(s string) (Severity, bool) {
	switch Severity(s) {
	case Blocker, High, Medium, Low, Info:
		return Severity(s), true
	}
	return "", false
}

// File couples a parsed file with its duplicated-line count (computed by the
// duplication engine).
type File struct {
	Result          *lang.FileResult
	DuplicatedLines int
}

// Issue is one located smell finding.
type Issue struct {
	Rule      string  `json:"rule" yaml:"rule"`
	Severity  string  `json:"severity" yaml:"severity"`
	File      string  `json:"file" yaml:"file"`
	Line      int     `json:"line" yaml:"line"`
	Function  string  `json:"function,omitempty" yaml:"function,omitempty"`
	Value     float64 `json:"value" yaml:"value"`
	Threshold float64 `json:"threshold" yaml:"threshold"`
}

// Evaluate runs every enabled rule over the production files and returns
// issues sorted deterministically by (file, line, rule).
func Evaluate(files []File, s Settings) []Issue {
	var issues []Issue
	for _, f := range files {
		cfg := s[f.Result.Lang]
		if cfg == nil {
			continue
		}
		issues = append(issues, evaluateFile(f, cfg)...)
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].File != issues[j].File {
			return issues[i].File < issues[j].File
		}
		if issues[i].Line != issues[j].Line {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Rule < issues[j].Rule
	})
	return issues
}

func evaluateFile(f File, cfg map[string]RuleConfig) []Issue {
	fr := f.Result
	var issues []Issue
	add := func(rule string, rc RuleConfig, line int, fn string, val float64) {
		issues = append(issues, Issue{
			Rule:      rule,
			Severity:  string(rc.Severity),
			File:      fr.Path,
			Line:      line,
			Function:  fn,
			Value:     round2(val),
			Threshold: rc.Threshold,
		})
	}

	if rc := cfg["file-length"]; rc.Enabled && float64(fr.PhysicalSLOC) > rc.Threshold {
		add("file-length", rc, 1, "", float64(fr.PhysicalSLOC))
	}
	if rc := cfg["duplicated-file"]; rc.Enabled && fr.PhysicalSLOC > 0 {
		pct := 100 * float64(f.DuplicatedLines) / float64(fr.PhysicalSLOC)
		if pct > rc.Threshold {
			add("duplicated-file", rc, 1, "", pct)
		}
	}
	if rc := cfg["comment-density"]; rc.Enabled && fr.PhysicalSLOC >= CommentFloor {
		if pct := CommentPct(fr); pct < rc.Threshold {
			add("comment-density", rc, 1, "", pct)
		}
	}

	for _, fn := range fr.Functions {
		if rc := cfg["cognitive-complexity"]; rc.Enabled && float64(fn.Cognitive) > rc.Threshold {
			add("cognitive-complexity", rc, fn.StartLine, fn.Name, float64(fn.Cognitive))
		}
		if rc := cfg["cyclomatic-complexity"]; rc.Enabled && float64(fn.Complexity) > rc.Threshold {
			add("cyclomatic-complexity", rc, fn.StartLine, fn.Name, float64(fn.Complexity))
		}
		if rc := cfg["function-length"]; rc.Enabled && float64(fn.Length) > rc.Threshold {
			add("function-length", rc, fn.StartLine, fn.Name, float64(fn.Length))
		}
		if rc := cfg["nesting-depth"]; rc.Enabled && float64(fn.Nesting) > rc.Threshold {
			add("nesting-depth", rc, fn.StartLine, fn.Name, float64(fn.Nesting))
		}
		if rc := cfg["parameter-count"]; rc.Enabled && float64(fn.Params) > rc.Threshold {
			add("parameter-count", rc, fn.StartLine, fn.Name, float64(fn.Params))
		}
		if rc := cfg["boolean-complexity"]; rc.Enabled && float64(fn.BoolMax) > rc.Threshold {
			add("boolean-complexity", rc, fn.StartLine, fn.Name, float64(fn.BoolMax))
		}
	}

	if rc := cfg["too-many-methods"]; rc.Enabled {
		counts := map[string]int{}
		first := map[string]int{}
		for _, fn := range fr.Functions {
			if fn.Enclosing == "" || fn.Anonymous {
				continue
			}
			counts[fn.Enclosing]++
			if first[fn.Enclosing] == 0 {
				first[fn.Enclosing] = fn.StartLine
			}
		}
		for name, n := range counts {
			if float64(n) > rc.Threshold {
				add("too-many-methods", rc, first[name], name, float64(n))
			}
		}
	}
	return issues
}

// CommentPct computes comment density: comment lines over code + comment lines.
func CommentPct(fr *lang.FileResult) float64 {
	denom := fr.PhysicalSLOC + fr.CommentLines
	if denom == 0 {
		return 0
	}
	return 100 * float64(fr.CommentLines) / float64(denom)
}

func round2(x float64) float64 {
	return float64(int(x*100+0.5)) / 100
}
