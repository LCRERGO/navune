// Package discover walks an analysis root, applies exclusion and
// production/test classification, and locates the enclosing Go module.
package discover

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/lang"
)

// File is a candidate source file to hand to a language adapter.
type File struct {
	Path    string // absolute path
	RelPath string // path relative to the analysis root (forward slashes)
	Lang    lang.Lang
}

// Module describes the Go module an analysis root belongs to, if any.
type Module struct {
	Path    string // import path from the module line
	Dir     string // absolute directory containing go.mod
	Present bool
}

// Results of a directory walk.
type Results struct {
	Files       []File
	SkippedDirs []string
}

// extLang maps a file extension to a language.
var extLang = map[string]lang.Lang{
	".go":  lang.Go,
	".ts":  lang.TypeScript,
	".tsx": lang.TypeScript,
	".js":  lang.JavaScript,
	".jsx": lang.JavaScript,
	".mjs": lang.JavaScript,
	".cjs": lang.JavaScript,
	".py":  lang.Python,
}

// Walk collects analyzable source files under root, honouring config
// exclusions. Test detection is language-aware at this layer by file name;
// the "generated" marker is detected later during parsing (ADR 0011).
func Walk(root string, cfg *config.Config) (*Results, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		abs = filepath.Dir(abs)
	}

	res := &Results{}
	err = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Nested Go modules (a directory with its own go.mod below the
			// analysis root) are their own codebase, mirroring `go list ./...`;
			// do not descend into them.
			if p != abs && hasGoMod(p) {
				res.SkippedDirs = append(res.SkippedDirs, p)
				return filepath.SkipDir
			}
			if p != abs && cfg.IsExcludedDir(d.Name()) {
				res.SkippedDirs = append(res.SkippedDirs, p)
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(p)
		pl, ok := extLang[ext]
		if !ok {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return err
		}
		res.Files = append(res.Files, File{
			Path:    p,
			RelPath: filepath.ToSlash(rel),
			Lang:    pl,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].RelPath < res.Files[j].RelPath })
	return res, nil
}

func hasGoMod(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return true
	}
	return false
}

// IsTestFile applies language-aware test classification by filename.
func IsTestFile(f File) bool {
	base := f.RelPath
	switch f.Lang {
	case lang.Go:
		return strings.HasSuffix(base, "_test.go")
	case lang.Python:
		base := filepath.Base(base)
		return strings.HasSuffix(base, "_test.py") || strings.HasPrefix(base, "test_")
	case lang.TypeScript, lang.JavaScript:
		for _, marker := range []string{".test.", ".spec."} {
			idx := strings.Index(base, marker)
			if idx > 0 {
				return true
			}
		}
	}
	return false
}

// FindModule walks upward from dir (toward filesystem root) looking for the
// nearest go.mod and returns the module it declares.
func FindModule(dir string) (*Module, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		gm := filepath.Join(abs, "go.mod")
		if b, err := os.ReadFile(gm); err == nil {
			path, ok := parseModuleLine(b)
			if !ok {
				return &Module{Dir: abs, Present: true}, nil
			}
			return &Module{Path: path, Dir: abs, Present: true}, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return &Module{}, nil
		}
		abs = parent
	}
}

func parseModuleLine(b []byte) (string, bool) {
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), true
		}
	}
	return "", false
}

// ImportPath returns the import path of the package containing a file,
// given the enclosing module. Files outside the module (or with no module)
// have no import path (""), so their imports are all external.
func ImportPath(mod *Module, f File) string {
	if mod == nil || !mod.Present {
		return ""
	}
	absFile, err := filepath.Abs(f.Path)
	if err != nil {
		return ""
	}
	modDir, err := filepath.Abs(mod.Dir)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(modDir, filepath.Dir(absFile))
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		if mod.Path == "" {
			return ""
		}
		return mod.Path
	}
	if mod.Path == "" {
		return rel
	}
	return mod.Path + "/" + rel
}
