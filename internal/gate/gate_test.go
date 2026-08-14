package gate

import (
	"math"
	"testing"

	"github.com/lcr/navune/internal/config"
)

func TestValidateUnknownMetric(t *testing.T) {
	cfg := config.Defaults()
	cfg.Budgets["not_a_metric"] = config.Budget{Limit: 1, Tier: "error"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unknown budget metric")
	}
}

func TestExitBreachOnErrorTier(t *testing.T) {
	cfg := config.Defaults()
	cfg.Budgets["worst_complexity"] = config.Budget{Limit: 3, Tier: "error"}
	v := Evaluate(cfg, map[string]float64{
		"avg_complexity":      5,
		"worst_complexity":    9, // > 3, error tier
		"max_duplication_pct": 1,
		"max_cycle_members":   1,
		"max_in_cycle_pct":    1,
	})
	if v.ExitCode != ExitBreach {
		t.Errorf("want ExitBreach, got %d", v.ExitCode)
	}
	if len(v.Breaches) != 1 {
		t.Errorf("want 1 breach, got %d", len(v.Breaches))
	}
}

func TestWarnOnlyDoesNotFail(t *testing.T) {
	cfg := config.Defaults()
	cfg.Budgets["max_duplication_pct"] = config.Budget{Limit: 1, Tier: "warn"}
	v := Evaluate(cfg, map[string]float64{
		"avg_complexity":      1,
		"worst_complexity":    1,
		"max_duplication_pct": 50, // breach but warn tier
		"max_cycle_members":   1,
		"max_in_cycle_pct":    1,
	})
	if v.ExitCode != ExitPass {
		t.Errorf("warn breach must not fail: got exit %d", v.ExitCode)
	}
}

func TestCompositeFormula(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Budgets: map[string]config.Budget{
			"worst_complexity": {Limit: 100, Tier: "error"},
		},
		Weights: map[string]float64{
			"worst_complexity": 100,
		},
	}
	// value == limit/2 -> score 50
	v := Evaluate(cfg, map[string]float64{"worst_complexity": 50})
	if math.Abs(v.Score-50) > 1e-6 {
		t.Errorf("want composite 50, got %v", v.Score)
	}
	// value == limit -> score 0
	v2 := Evaluate(cfg, map[string]float64{"worst_complexity": 100})
	if v2.Score != 0 {
		t.Errorf("want composite 0 at limit, got %v", v2.Score)
	}
	// value == 0 -> score 100
	v3 := Evaluate(cfg, map[string]float64{"worst_complexity": 0})
	if v3.Score != 100 {
		t.Errorf("want composite 100 at zero, got %v", v3.Score)
	}
}
