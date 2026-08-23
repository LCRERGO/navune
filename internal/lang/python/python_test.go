package python

import (
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func TestParseBasics(t *testing.T) {
	src := `import os
from . import util
from a.b import c

class Base:
    pass

def f(a, b=1):
    if a and b:
        try:
            return 1 if a else 2
        except ValueError:
            return 0
    while a:
        a -= 1
    return a
`
	res, err := New().Parse("m.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if res.Lang != lang.Python {
		t.Errorf("lang = %s", res.Lang)
	}
	if len(res.Types) != 1 || res.Types[0].Name != "Base" {
		t.Errorf("types = %+v", res.Types)
	}
	if len(res.Functions) != 1 || res.Functions[0].Name != "f" {
		t.Fatalf("functions = %+v", res.Functions)
	}
	f := res.Functions[0]
	// base 1 + if(1) + and(1) + try/except(1) + conditional_expression(1) + while(1) = 6
	if f.Complexity != 6 {
		t.Errorf("f complexity = %d (want 6)", f.Complexity)
	}
	if len(res.Imports) != 4 {
		t.Errorf("imports = %v", res.Imports)
	}
	if res.Imports[0] != "os" || res.Imports[1] != ".util" || res.Imports[2] != "a.b" || res.Imports[3] != "a.b.c" {
		t.Errorf("imports = %v", res.Imports)
	}
}

func TestClassMethodEnclosing(t *testing.T) {
	src := `class Greeter:
    def greet(self, name):
        if name:
            return "hi " + name
        return "hi"
`
	res, err := New().Parse("g.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Functions) != 1 {
		t.Fatalf("functions = %+v", res.Functions)
	}
	if res.Functions[0].Enclosing != "Greeter" {
		t.Errorf("method enclosing = %q, want Greeter", res.Functions[0].Enclosing)
	}
	if res.Functions[0].Complexity != 2 {
		t.Errorf("greet complexity = %d, want 2", res.Functions[0].Complexity)
	}
}

func TestNestedDefIsOwnFunction(t *testing.T) {
	src := `def outer():
    def inner(x):
        if x:
            return 1
        return 0
    return inner
`
	res, err := New().Parse("n.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Functions) != 2 {
		t.Fatalf("want 2 functions (outer + inner), got %d", len(res.Functions))
	}
	if res.Functions[0].Complexity != 1 {
		t.Errorf("outer complexity should be 1 (inner not attributed), got %d", res.Functions[0].Complexity)
	}
	if res.Functions[1].Name != "inner" || res.Functions[1].Complexity != 2 {
		t.Errorf("inner = %+v (want complexity 2)", res.Functions[1])
	}
	if res.Functions[1].Enclosing != "" {
		t.Errorf("nested def should not inherit class/enclosing, got %q", res.Functions[1].Enclosing)
	}
}

func TestTokensNormalize(t *testing.T) {
	src := `def f():
    s = "hello"
    n = 42
    return s
`
	res, err := New().Parse("t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var str, num int
	for _, tok := range res.Tokens {
		switch tok.Kind {
		case lang.TokString:
			str++
			if tok.Text != "STR" {
				t.Errorf("string = %q", tok.Text)
			}
		case lang.TokNumber:
			num++
			if tok.Text != "NUM" {
				t.Errorf("number = %q", tok.Text)
			}
		}
	}
	if str != 1 || num != 1 {
		t.Errorf("want 1 STR and 1 NUM, got %d and %d", str, num)
	}
	if res.PhysicalSLOC == 0 {
		t.Error("physical sloc should be > 0")
	}
}
