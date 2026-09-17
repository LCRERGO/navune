//go:build evolution

package evolution

import (
	"fmt"
	"strings"
	"testing"
)

// smell evolution: growing a function past the cyclomatic/cognitive thresholds
// must raise the smell issue count from 0 to >0 (ADR 0016).

func ifsSource(lang string, n int) string {
	switch lang {
	case "go":
		var b strings.Builder
		b.WriteString("package main\n\nfunc f(x int) int {\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "\tif x == %d {\n\t\treturn %d\n\t}\n", i, i)
		}
		b.WriteString("\treturn 0\n}\n")
		return b.String()
	case "ts":
		var b strings.Builder
		b.WriteString("export function f(x: number): number {\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "  if (x === %d) { return %d; }\n", i, i)
		}
		b.WriteString("  return 0;\n}\n")
		return b.String()
	case "py":
		var b strings.Builder
		b.WriteString("def f(x):\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "    if x == %d:\n        return %d\n", i, i)
		}
		b.WriteString("    return 0\n")
		return b.String()
	case "c":
		var b strings.Builder
		b.WriteString("int f(int x) {\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "    if (x == %d) { return %d; }\n", i, i)
		}
		b.WriteString("    return 0;\n}\n")
		return b.String()
	case "cpp":
		var b strings.Builder
		b.WriteString("int f(int x) {\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "    if (x == %d) { return %d; }\n", i, i)
		}
		b.WriteString("    return 0;\n}\n")
		return b.String()
	case "java":
		return javaIfs(n)
	case "rust":
		return rustIfs(n)
	}
	return ""
}

func smellSteps(lang, file string) []step {
	return []step{
		{"under-threshold", map[string]string{file: ifsSource(lang, 1)}},
		{"over-threshold", map[string]string{file: ifsSource(lang, 16)}},
	}
}

func TestSmellCountRises(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"go", "main.go"},
		{"ts", "main.ts"},
		{"py", "main.py"},
		{"c", "main.c"},
		{"cpp", "main.cpp"},
		{"java", "Main.java"},
		{"rust", "main.rs"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			reports := evolve(t, defaults(), smellSteps(c.name, c.file))
			if got := reports[0].Summary.IssueCount; got != 0 {
				t.Errorf("step 0: want 0 issues, got %d", got)
			}
			if got := reports[1].Summary.IssueCount; got <= 0 {
				t.Errorf("step 1: want issues, got %d", got)
			}
		})
	}
}
