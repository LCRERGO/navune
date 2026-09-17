//go:build evolution

package evolution

import (
	"fmt"
	"strings"
)

// Rust step generators (ADR 0019).

func complexityStepsRust() []step {
	return []step{
		{"no-decisions", map[string]string{"main.rs": "pub fn f() -> i32 {\n    0\n}\n"}},
		{"if", map[string]string{"main.rs": "pub fn f(x: i32) -> i32 {\n    if x > 0 {\n        1\n    } else {\n        0\n    }\n}\n"}},
		{"if-and", map[string]string{"main.rs": "pub fn f(x: i32, y: bool) -> i32 {\n    if x > 0 && y {\n        1\n    } else {\n        0\n    }\n}\n"}},
	}
}

func cycleStepsRust() []step {
	step0 := map[string]string{
		"a.rs": "use crate::b::f;\n\npub fn a() -> i32 {\n    f()\n}\n",
		"b.rs": "pub fn f() -> i32 {\n    1\n}\n",
	}
	step1 := map[string]string{
		"a.rs": "use crate::b::f;\n\npub fn a() -> i32 {\n    f()\n}\n",
		"b.rs": "use crate::a::a;\n\npub fn f() -> i32 {\n    a()\n}\n",
	}
	return []step{
		{"one-way", step0},
		{"reverse-import", step1},
	}
}

const dupRust = `pub fn compute(x: i32, vals: &[i32]) -> i32 {
    let mut total = 0;
    for v in vals {
        if *v > 0 && *v % 2 == 0 {
            total = total + *v + x;
        } else if *v < 0 {
            total = total - *v;
        }
    }
    total
}
`

func dupStepsRust() []step {
	return []step{
		{"one-copy", map[string]string{"a.rs": dupRust}},
		{"copy-paste", map[string]string{"a.rs": dupRust, "b.rs": dupRust}},
	}
}

func rustIfs(n int) string {
	var b strings.Builder
	b.WriteString("pub fn f(x: i32) -> i32 {\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "    if x == %d { return %d; }\n", i, i)
	}
	b.WriteString("    0\n}\n")
	return b.String()
}
