//go:build evolution

package evolution

import (
	"testing"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/gate"
)

// quality-gate evolution: with an aggressive worst_complexity budget, growing
// complexity must flip the exit code from PASS to BREACH mid-evolution.

func TestBudgetBreachFlipsExitCode(t *testing.T) {
	cfg := config.Defaults()
	cfg.Weights = map[string]float64{}
	// worst_complexity == 2 allowed; anything > 2 is an error-tier breach
	cfg.Budgets["worst_complexity"] = config.Budget{Limit: 2, Tier: "error"}

	reports := evolve(t, cfg, complexityStepsGo()) // worst: 1 -> 2 -> 3
	codes := make([]int, len(reports))
	for i, r := range reports {
		codes[i] = r.ExitCode
	}
	if codes[0] != gate.ExitPass || codes[1] != gate.ExitPass {
		t.Errorf("want PASS at worst 1 and 2, got exit codes %v", codes)
	}
	if codes[2] != gate.ExitBreach {
		t.Errorf("want BREACH when worst complexity 3 > budget 2, got exit code %d", codes[2])
	}
	// breach must be reflected in the budget results too
	for _, br := range reports[2].Budgets {
		if br.Key == "worst_complexity" && !br.Breached {
			t.Errorf("worst_complexity budget should be breached at final step")
		}
	}
}

// TestBudgetsIgnoredUnderLimit guards the "no false breach at the boundary"
// semantics (value == limit is not a breach).
func TestBudgetsIgnoredUnderLimit(t *testing.T) {
	cfg := config.Defaults()
	cfg.Weights = map[string]float64{}
	cfg.Budgets["worst_complexity"] = config.Budget{Limit: 3, Tier: "error"}
	reports := evolve(t, cfg, complexityStepsGo()) // worst: 1 -> 2 -> 3
	if reports[2].ExitCode != gate.ExitPass {
		t.Errorf("worst == limit must not breach: exit %d", reports[2].ExitCode)
	}
}
