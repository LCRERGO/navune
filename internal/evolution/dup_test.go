//go:build evolution

package evolution

import "testing"

// duplication evolution: a helper function is copied verbatim into a second
// file; duplicated tokens must go from 0 to >0 in every language.

// dupBody is an identical helper body pasted into two files. It must exceed
// dup.DefaultMinTokens (20 normalized tokens).
const dupGo = `package main

func Compute(x int, vals []int) int {
	total := 0
	for _, v := range vals {
		if v > 0 && v%2 == 0 {
			total = total + v + x
		} else if v < 0 {
			total = total - v
		}
	}
	return total
}
`

func dupStepsGo() []step {
	return []step{
		{"one-copy", map[string]string{"a.go": dupGo}},
		{"copy-paste", map[string]string{"a.go": dupGo, "b.go": dupGo}},
	}
}

const dupTS = `export function compute(x: number, vals: number[]): number {
  let total = 0;
  for (const v of vals) {
    if (v > 0 && v % 2 === 0) {
      total = total + v + x;
    } else if (v < 0) {
      total = total - v;
    }
  }
  return total;
}
`

func dupStepsTS() []step {
	return []step{
		{"one-copy", map[string]string{"a.ts": dupTS}},
		{"copy-paste", map[string]string{"a.ts": dupTS, "b.ts": dupTS}},
	}
}

const dupPy = `def compute(x, vals):
    total = 0
    for v in vals:
        if v > 0 and v % 2 == 0:
            total = total + v + x
        elif v < 0:
            total = total - v
    return total
`

func dupStepsPy() []step {
	return []step{
		{"one-copy", map[string]string{"a.py": dupPy}},
		{"copy-paste", map[string]string{"a.py": dupPy, "b.py": dupPy}},
	}
}

func TestDuplicationAppears(t *testing.T) {
	cases := []struct {
		name  string
		steps []step
	}{
		{"go", dupStepsGo()},
		{"ts", dupStepsTS()},
		{"py", dupStepsPy()},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			reports := evolve(t, defaults(), c.steps)
			if reports[0].Summary.DupBlocks != 0 || reports[0].Summary.DupTokens != 0 {
				t.Errorf("step 0: want no duplication, got blocks=%d tokens=%d",
					reports[0].Summary.DupBlocks, reports[0].Summary.DupTokens)
			}
			if reports[1].Summary.DupBlocks < 1 {
				t.Errorf("step 1: expected duplication blocks, got %d", reports[1].Summary.DupBlocks)
			}
			if reports[1].Summary.DupTokens <= 0 {
				t.Errorf("step 1: expected duplicated tokens > 0, got %d", reports[1].Summary.DupTokens)
			}
		})
	}
}
