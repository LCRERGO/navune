//go:build evolution

package evolution

import (
	"fmt"
	"strings"
)

// Java step generators (ADR 0019).

func complexityStepsJava() []step {
	return []step{
		{"no-decisions", map[string]string{"Main.java": "public class Main {\n    public int f() {\n        return 0;\n    }\n}\n"}},
		{"if", map[string]string{"Main.java": "public class Main {\n    public int f(int x) {\n        if (x > 0) {\n            return 1;\n        }\n        return 0;\n    }\n}\n"}},
		{"if-and", map[string]string{"Main.java": "public class Main {\n    public int f(int x, boolean y) {\n        if (x > 0 && y) {\n            return 1;\n        }\n        return 0;\n    }\n}\n"}},
	}
}

func cycleStepsJava() []step {
	step0 := map[string]string{
		"a/A.java": "package a;\n\nimport b.B;\n\npublic class A {\n    public int use() {\n        return B.b();\n    }\n}\n",
		"b/B.java": "package b;\n\npublic class B {\n    public static int b() {\n        return 1;\n    }\n}\n",
	}
	step1 := map[string]string{
		"a/A.java": "package a;\n\nimport b.B;\n\npublic class A {\n    public int use() {\n        return B.b();\n    }\n}\n",
		"b/B.java": "package b;\n\nimport a.A;\n\npublic class B {\n    public static int b() {\n        return new A().use();\n    }\n}\n",
	}
	return []step{
		{"one-way", step0},
		{"reverse-import", step1},
	}
}

const dupJava = `package fixture;

public class Compute {
    public int compute(int x, int[] vals) {
        int total = 0;
        for (int v : vals) {
            if (v > 0 && v % 2 == 0) {
                total = total + v + x;
            } else if (v < 0) {
                total = total - v;
            }
        }
        return total;
    }
}
`

func dupStepsJava() []step {
	return []step{
		{"one-copy", map[string]string{"a/Compute.java": dupJava}},
		{"copy-paste", map[string]string{"a/Compute.java": dupJava, "b/Compute.java": dupJava}},
	}
}

func javaIfs(n int) string {
	var b strings.Builder
	b.WriteString("public class Main {\n    public int f(int x) {\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "        if (x == %d) { return %d; }\n", i, i)
	}
	b.WriteString("        return 0;\n    }\n}\n")
	return b.String()
}
