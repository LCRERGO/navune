//go:build evolution

package evolution

// C and C++ step generators (ADR 0017). C headers with no C++ markers sniff as
// C; .hpp is always C++.

func complexityStepsC() []step {
	return []step{
		{"no-decisions", map[string]string{"main.c": "int f(void) { return 0; }\n"}},
		{"if", map[string]string{"main.c": "int f(int x) { if (x > 0) { return 1; } return 0; }\n"}},
		{"if-and", map[string]string{"main.c": "int f(int x, int y) { if (x > 0 && y) { return 1; } return 0; }\n"}},
	}
}

func complexityStepsCPP() []step {
	return []step{
		{"no-decisions", map[string]string{"main.cpp": "int f() { return 0; }\n"}},
		{"if", map[string]string{"main.cpp": "int f(int x) { if (x > 0) { return 1; } return 0; }\n"}},
		{"if-and", map[string]string{"main.cpp": "int f(int x, bool y) { if (x > 0 && y) { return 1; } return 0; }\n"}},
	}
}

func cycleStepsC() []step {
	return []step{
		{"one-way", map[string]string{
			"a.h": "#include \"b.h\"\nint a(void);\n",
			"b.h": "int b(void);\n",
		}},
		{"reverse-include", map[string]string{
			"a.h": "#include \"b.h\"\nint a(void);\n",
			"b.h": "#include \"a.h\"\nint b(void);\n",
		}},
	}
}

func cycleStepsCPP() []step {
	return []step{
		{"one-way", map[string]string{
			"a.hpp": "#include \"b.hpp\"\nclass A {};\n",
			"b.hpp": "class B {};\n",
		}},
		{"reverse-include", map[string]string{
			"a.hpp": "#include \"b.hpp\"\nclass A {};\n",
			"b.hpp": "#include \"a.hpp\"\nclass B {};\n",
		}},
	}
}

const dupC = `int compute(int x, int *vals, int n) {
    int total = 0;
    for (int i = 0; i < n; i++) {
        if (vals[i] > 0 && vals[i] % 2 == 0) {
            total = total + vals[i] + x;
        } else if (vals[i] < 0) {
            total = total - vals[i];
        }
    }
    return total;
}
`

func dupStepsC() []step {
	return []step{
		{"one-copy", map[string]string{"a.c": dupC}},
		{"copy-paste", map[string]string{"a.c": dupC, "b.c": dupC}},
	}
}

const dupCPP = `int compute(int x, int *vals, int n) {
    int total = 0;
    for (int i = 0; i < n; i++) {
        if (vals[i] > 0 && vals[i] % 2 == 0) {
            total = total + vals[i] + x;
        } else if (vals[i] < 0) {
            total = total - vals[i];
        }
    }
    return total;
}
`

func dupStepsCPP() []step {
	return []step{
		{"one-copy", map[string]string{"a.cpp": dupCPP}},
		{"copy-paste", map[string]string{"a.cpp": dupCPP, "b.cpp": dupCPP}},
	}
}
