//go:build evolution

package evolution

import "testing"

// complexity evolution: a single function grows decision points over time.
// worst complexity must rise 1 -> 2 -> 3 in every supported language.

func complexityStepsGo() []step {
	body0 := `package main

func f() int {
	return 0
}
`
	body1 := `package main

func f(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}
`
	body2 := `package main

func f(x int, y bool) int {
	if x > 0 && y {
		return 1
	}
	return 0
}
`
	return []step{
		{"no-decisions", map[string]string{"main.go": body0}},
		{"if", map[string]string{"main.go": body1}},
		{"if-and", map[string]string{"main.go": body2}},
	}
}

func complexityStepsTS() []step {
	return []step{
		{"no-decisions", map[string]string{"main.ts": "export function f(): number { return 0; }\n"}},
		{"if", map[string]string{"main.ts": "export function f(x: number): number { if (x > 0) { return 1; } return 0; }\n"}},
		{"if-and", map[string]string{"main.ts": "export function f(x: number, y: boolean): number { if (x > 0 && y) { return 1; } return 0; }\n"}},
	}
}

func complexityStepsPy() []step {
	return []step{
		{"no-decisions", map[string]string{"main.py": "def f():\n    return 0\n"}},
		{"if", map[string]string{"main.py": "def f(x):\n    if x > 0:\n        return 1\n    return 0\n"}},
		{"if-and", map[string]string{"main.py": "def f(x, y):\n    if x > 0 and y:\n        return 1\n    return 0\n"}},
	}
}

func TestComplexityRises(t *testing.T) {
	cases := []struct {
		name  string
		steps []step
	}{
		{"go", complexityStepsGo()},
		{"ts", complexityStepsTS()},
		{"py", complexityStepsPy()},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			reports := evolve(t, defaults(), c.steps)
			worst := make([]int, len(reports))
			fns := make([]int, len(reports))
			for i, r := range reports {
				worst[i] = r.Summary.WorstComplexity
				fns[i] = r.Summary.Functions
			}
			increasing(t, "worst_complexity", worst, []int{1, 2, 3})
			for i, n := range fns {
				if n != 1 {
					t.Errorf("step %d: want exactly 1 function, got %d", i, n)
				}
			}
		})
	}
}
