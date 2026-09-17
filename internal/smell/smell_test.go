package smell

import (
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func boolp(b bool) *bool      { return &b }
func f64p(f float64) *float64 { return &f }

func TestResolvePrecedence(t *testing.T) {
	s := Resolve(
		map[string]RuleOverride{
			"function-length": {Threshold: f64p(70), Severity: "high"},
		},
		map[string]map[string]RuleOverride{
			"python": {"function-length": {Threshold: f64p(40)}},
		},
	)
	if got := s[lang.Go]["function-length"]; got.Threshold != 70 || got.Severity != High || !got.Enabled {
		t.Errorf("go function-length = %+v, want threshold 70/high/enabled", got)
	}
	py := s[lang.Python]["function-length"]
	if py.Threshold != 40 {
		t.Errorf("python function-length threshold = %v, want 40 (language wins)", py.Threshold)
	}
	if py.Severity != High {
		t.Errorf("python severity = %v, want inherited high", py.Severity)
	}
	// built-in defaults survive where nothing is configured
	if got := s[lang.TypeScript]["nesting-depth"].Threshold; got != 3 {
		t.Errorf("ts nesting-depth = %v, want built-in 3", got)
	}
	if got := s[lang.Go]["nesting-depth"].Threshold; got != 4 {
		t.Errorf("go nesting-depth = %v, want built-in 4", got)
	}
}

func TestResolveDisable(t *testing.T) {
	s := Resolve(map[string]RuleOverride{
		"parameter-count": {Enabled: boolp(false)},
	}, nil)
	if s[lang.Go]["parameter-count"].Enabled {
		t.Error("parameter-count should be disabled")
	}
}

func TestEvaluate(t *testing.T) {
	fr := &lang.FileResult{
		Path:         "a.go",
		Lang:         lang.Go,
		PhysicalSLOC: 100,
		Functions: []lang.Function{
			{Name: "big", StartLine: 10, Cognitive: 20, Complexity: 5, Params: 9},
		},
	}
	issues := Evaluate([]File{{Result: fr}}, Resolve(nil, nil))
	rules := map[string]bool{}
	for _, i := range issues {
		rules[i.Rule] = true
	}
	if !rules["cognitive-complexity"] {
		t.Error("expected cognitive-complexity issue (20 > 15)")
	}
	if !rules["parameter-count"] {
		t.Error("expected parameter-count issue (9 > 7)")
	}
	if rules["cyclomatic-complexity"] {
		t.Error("cyclomatic 5 should not breach 10")
	}
	// sorted by (file, line, rule)
	for i := 1; i < len(issues); i++ {
		if issues[i-1].File > issues[i].File {
			t.Error("issues not sorted by file")
		}
	}
}

func TestCommentDensityFloor(t *testing.T) {
	small := &lang.FileResult{Path: "s.go", Lang: lang.Go, PhysicalSLOC: 10}
	if issues := Evaluate([]File{{Result: small}}, Resolve(nil, nil)); len(issues) != 0 {
		t.Errorf("files under the comment floor should not be flagged, got %v", issues)
	}
	large := &lang.FileResult{Path: "l.go", Lang: lang.Go, PhysicalSLOC: 100, CommentLines: 0}
	found := false
	for _, i := range Evaluate([]File{{Result: large}}, Resolve(nil, nil)) {
		if i.Rule == "comment-density" {
			found = true
		}
	}
	if !found {
		t.Error("expected comment-density issue for a large comment-free file")
	}
}
