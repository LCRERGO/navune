//go:build evolution

package evolution

import "testing"

// cycle evolution: two units start with a one-way dependency; adding the
// reverse import must create exactly one 2-member cycle in every language.

func cycleStepsGo() []step {
	gomod := "module example.com/evo\n\ngo 1.27\n"
	step0 := map[string]string{
		"go.mod": gomod,
		"a/a.go": "package a\n\nimport \"example.com/evo/b\"\n\nfunc Use() int { return b.B() }\n",
		"b/b.go": "package b\n\nfunc B() int { return 1 }\n",
	}
	step1 := map[string]string{
		"go.mod": gomod,
		"a/a.go": "package a\n\nimport \"example.com/evo/b\"\n\nfunc Use() int { return b.B() }\n",
		"b/b.go": "package b\n\nimport \"example.com/evo/a\"\n\nfunc B() int { a.Use(); return 1 }\n",
	}
	return []step{
		{"one-way", step0},
		{"reverse-import", step1},
	}
}

func cycleStepsTS() []step {
	step0 := map[string]string{
		"a.ts": "import { b } from \"./b\";\nexport function a(): number { return b(); }\n",
		"b.ts": "export function b(): number { return 1; }\n",
	}
	step1 := map[string]string{
		"a.ts": "import { b } from \"./b\";\nexport function a(): number { return b(); }\n",
		"b.ts": "import { a } from \"./a\";\nexport function b(): number { a(); return 1; }\n",
	}
	return []step{
		{"one-way", step0},
		{"reverse-import", step1},
	}
}

func cycleStepsPy() []step {
	step0 := map[string]string{
		"a.py": "import b\n\ndef a():\n    return b.f()\n",
		"b.py": "def f():\n    return 1\n",
	}
	step1 := map[string]string{
		"a.py": "import b\n\ndef a():\n    return b.f()\n",
		"b.py": "import a\n\ndef f():\n    a.a()\n    return 1\n",
	}
	return []step{
		{"one-way", step0},
		{"reverse-import", step1},
	}
}

func TestCycleAppears(t *testing.T) {
	cases := []struct {
		name  string
		steps []step
	}{
		{"go", cycleStepsGo()},
		{"ts", cycleStepsTS()},
		{"py", cycleStepsPy()},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			reports := evolve(t, defaults(), c.steps)
			cycles := make([]int, len(reports))
			maxMembers := make([]int, len(reports))
			for i, r := range reports {
				cycles[i] = r.Summary.Cycles
				maxMembers[i] = r.Summary.MaxCycleMembers
			}
			// one-way: 0 cycles; after reverse import: exactly 1 cycle of 2
			if cycles[0] != 0 {
				t.Errorf("step 0: want 0 cycles, got %d", cycles[0])
			}
			if cycles[1] != 1 || maxMembers[1] != 2 {
				t.Errorf("step 1: want 1 cycle of 2 members, got cycles=%d members=%d", cycles[1], maxMembers[1])
			}
		})
	}
}
