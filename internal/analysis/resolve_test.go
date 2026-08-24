package analysis

import (
	"reflect"
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func scriptFile(path string, l lang.Lang, imports []string) *lang.FileResult {
	return &lang.FileResult{Path: path, Lang: l, Imports: imports}
}

func TestResolveTSDepsRelative(t *testing.T) {
	files := []*lang.FileResult{
		scriptFile("src/a.ts", lang.TypeScript, []string{"./b", "./missing", "react"}),
		scriptFile("src/b.ts", lang.TypeScript, nil),
		scriptFile("src/sub/index.ts", lang.TypeScript, nil),
	}
	resolveScriptDeps(files)
	// a.ts depends on b.ts via ./b; "react" and missing stay external
	if got := files[0].Deps; !reflect.DeepEqual(got, []string{"src/b.ts"}) {
		t.Errorf("a.ts deps = %v, want [src/b.ts]", got)
	}
}

func TestResolveTSDepsIndex(t *testing.T) {
	files := []*lang.FileResult{
		scriptFile("a/app.ts", lang.TypeScript, []string{"./lib"}),
		scriptFile("a/lib/index.ts", lang.TypeScript, nil),
	}
	resolveScriptDeps(files)
	if got := files[0].Deps; !reflect.DeepEqual(got, []string{"a/lib/index.ts"}) {
		t.Errorf("deps = %v, want [a/lib/index.ts]", got)
	}
}

func TestResolvePyDepsAbsoluteAndRelative(t *testing.T) {
	files := []*lang.FileResult{
		scriptFile("pkg/a/a.py", lang.Python, []string{"os", "pkg.b", "..b", "..c"}),
		scriptFile("pkg/b.py", lang.Python, nil),
		// package c with __init__; ..c from pkg/a resolves to pkg/c/__init__.py
		scriptFile("pkg/c/__init__.py", lang.Python, nil),
	}
	resolveScriptDeps(files)
	// pkg.b & ..b -> pkg/b.py (deduped); ..c -> pkg/c/__init__.py; os external
	want := []string{"pkg/b.py", "pkg/c/__init__.py"}
	if got := files[0].Deps; !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v, want %v", got, want)
	}
}

func TestResolveCrossLangIgnored(t *testing.T) {
	// go files should be untouched by script resolution
	f := &lang.FileResult{Path: "a.go", Lang: lang.Go, Imports: []string{"m/b"}}
	resolveScriptDeps([]*lang.FileResult{f})
	if len(f.Deps) != 0 {
		t.Errorf("go file should not get script deps, got %v", f.Deps)
	}
}
