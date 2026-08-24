package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/lang"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, content := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestWalkCollectsGoAndSkipsVendor(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.go":              "package a",
		"sub/b.go":          "package b",
		"vendor/v/v.go":     "package v",
		"node_modules/x.go": "package x",
		"README.md":         "not code",
		"testdata/t.go":     "package t",
	})
	res, err := Walk(dir, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range res.Files {
		if f.Lang != lang.Go {
			t.Errorf("expected .go file, got lang %s", f.Lang)
		}
		rels = append(rels, f.RelPath)
	}
	joined := "a.go sub/b.go"
	if len(rels) != 2 || !contains(rels, "a.go") || !contains(rels, "sub/b.go") {
		t.Errorf("unexpected file set: %v", rels)
	}
	_ = joined
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestIsTestFile(t *testing.T) {
	cases := []struct {
		path string
		lang lang.Lang
		want bool
	}{
		{"foo_test.go", lang.Go, true},
		{"foo.go", lang.Go, false},
		{"internal/bar/bar_test.go", lang.Go, true},
		{"a.test.ts", lang.TypeScript, true},
		{"a.spec.tsx", lang.TypeScript, true},
		{"a.ts", lang.TypeScript, false},
		{"b.test.js", lang.JavaScript, true},
		{"b.js", lang.JavaScript, false},
		{"test_foo.py", lang.Python, true},
		{"foo_test.py", lang.Python, true},
		{"foo.py", lang.Python, false},
	}
	for _, c := range cases {
		f := File{RelPath: c.path, Lang: c.lang}
		if got := IsTestFile(f); got != c.want {
			t.Errorf("IsTestFile(%q, %s) = %v, want %v", c.path, c.lang, got, c.want)
		}
	}
}

func TestFindModuleUpward(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        "module example.com/demo\n",
		"pkg/deep/x.go": "package x",
	})
	mod, err := FindModule(filepath.Join(dir, "pkg", "deep"))
	if err != nil {
		t.Fatal(err)
	}
	if !mod.Present || mod.Path != "example.com/demo" {
		t.Errorf("want module example.com/demo, got %+v", mod)
	}
}

func TestImportPathWithinModule(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":   "module example.com/demo\n",
		"pkg/x.go": "package x",
	})
	mod, err := FindModule(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := File{Path: filepath.Join(dir, "pkg", "x.go"), Lang: lang.Go}
	if got := ImportPath(mod, f); got != "example.com/demo/pkg" {
		t.Errorf("import path want example.com/demo/pkg, got %q", got)
	}
	rootF := File{Path: filepath.Join(dir, "main.go"), Lang: lang.Go}
	if got := ImportPath(mod, rootF); got != "example.com/demo" {
		t.Errorf("root import path want example.com/demo, got %q", got)
	}
}
