package cfamily

import (
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func TestParseC(t *testing.T) {
	src := `#include "dep.h"
#include <stdio.h>

// a significant comment
int f(int a, int b) {
    if (a && b) {
        return 1;
    }
    return 0;
}
`
	res, err := NewC().Parse("x.c", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if res.Lang != lang.C {
		t.Errorf("lang = %s", res.Lang)
	}
	if len(res.Functions) != 1 {
		t.Fatalf("want 1 function, got %d", len(res.Functions))
	}
	f := res.Functions[0]
	if f.Name != "f" {
		t.Errorf("name = %q", f.Name)
	}
	if f.Complexity != 3 { // base + if + &&
		t.Errorf("complexity = %d (want 3)", f.Complexity)
	}
	if f.Params != 2 {
		t.Errorf("params = %d (want 2)", f.Params)
	}
	if res.CommentLines != 1 {
		t.Errorf("comment lines = %d (want 1)", res.CommentLines)
	}
	if len(res.Imports) != 2 || res.Imports[0] != "dep.h" || res.Imports[1] != "<stdio.h>" {
		t.Errorf("imports = %v", res.Imports)
	}
	if len(res.Types) != 0 {
		t.Errorf("C has no type layer, got %v", res.Types)
	}
}

func TestParseCPP(t *testing.T) {
	src := `#include <vector>

class Base {
public:
    virtual void run() = 0;
};

class Impl : public Base {
public:
    void run() override {
        for (int i = 0; i < 10; i++) {
        }
    }
};

int add(int a, int b) { return a + b; }
`
	res, err := NewCPP().Parse("x.cpp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if res.Lang != lang.CPP {
		t.Errorf("lang = %s", res.Lang)
	}
	if len(res.Types) != 2 {
		t.Fatalf("want 2 types, got %+v", res.Types)
	}
	if !res.Types[0].Abstract {
		t.Errorf("Base should be abstract (pure virtual), got %+v", res.Types[0])
	}
	if res.Types[1].Abstract {
		t.Errorf("Impl should not be abstract, got %+v", res.Types[1])
	}
	byName := map[string]lang.Function{}
	for _, f := range res.Functions {
		byName[f.Name] = f
	}
	run, ok := byName["run"]
	if !ok {
		t.Fatalf("missing run; functions = %+v", res.Functions)
	}
	if run.Enclosing != "Impl" {
		t.Errorf("run enclosing = %q (want Impl)", run.Enclosing)
	}
	if run.Complexity != 2 { // base + for
		t.Errorf("run complexity = %d (want 2)", run.Complexity)
	}
	if run.Nesting != 1 {
		t.Errorf("run nesting = %d (want 1)", run.Nesting)
	}
}

func TestHeaderSniffing(t *testing.T) {
	cSrc := `#include "a.h"
int f(void) { return 0; }
`
	res, err := NewCPP().Parse("x.h", []byte(cSrc))
	if err != nil {
		t.Fatal(err)
	}
	if res.Lang != lang.C {
		t.Errorf("plain C header should sniff as C, got %s", res.Lang)
	}

	cppSrc := `#include "a.h"
namespace n { class C {}; }
`
	res2, err := NewCPP().Parse("y.h", []byte(cppSrc))
	if err != nil {
		t.Fatal(err)
	}
	if res2.Lang != lang.CPP {
		t.Errorf("C++ header should sniff as C++, got %s", res2.Lang)
	}

	externC := "#ifdef __cplusplus\nextern \"C\" {\n#endif\nint f(void);\n"
	res3, err := NewCPP().Parse("z.h", []byte(externC))
	if err != nil {
		t.Fatal(err)
	}
	if res3.Lang != lang.CPP {
		t.Errorf("extern \"C\" header should sniff as C++, got %s", res3.Lang)
	}
}

func TestCppLambda(t *testing.T) {
	src := `int f() {
    auto g = [](int x) { return x + 1; };
    return g(1);
}
`
	res, err := NewCPP().Parse("x.cpp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Functions) != 2 {
		t.Fatalf("want 2 functions (f + lambda), got %d", len(res.Functions))
	}
	lambda := res.Functions[1]
	if !lambda.Anonymous {
		t.Errorf("expected anonymous lambda, got %q", lambda.Name)
	}
	if lambda.Params != 1 {
		t.Errorf("lambda params = %d (want 1)", lambda.Params)
	}
}
