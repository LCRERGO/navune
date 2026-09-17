package analysis

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/lang"
)

// resolveDeps computes, for every production file, the set of internal files
// it depends on (ADR 0010: internal edges only). Resolution is best-effort: a
// specifier is internal only if it maps to a file that was actually analyzed.
//
// TS/JS: only relative specifiers (./ ../) resolve; bare package specifiers
// (e.g. "react") are external. Candidates honour extension omission and
// index files.
//
// Python: dotted imports (os.path, a.b) resolve under the analysis root as
// a/b.py or a/b/__init__.py; relative imports (., ..) resolve against the
// importing file's directory. Anything else is external.
//
// C/C++: quoted #include paths resolve relative to the including file, then
// against the configured include_paths; angle includes only against
// include_paths (ADR 0017).
func resolveDeps(files []*lang.FileResult, cfg *config.Config, root string) {
	has := map[string]bool{}
	for _, f := range files {
		has[f.Path] = true
	}
	includeDirs := includeDirsRelToRoot(cfg, root)
	for _, f := range files {
		switch f.Lang {
		case lang.TypeScript, lang.JavaScript:
			f.Deps = resolveTSDeps(f, has)
		case lang.Python:
			f.Deps = resolvePyDeps(f, has)
		case lang.C, lang.CPP:
			f.Deps = resolveCDeps(f, has, includeDirs)
		}
	}
}

// includeDirsRelToRoot converts configured include directories to paths
// relative to the analysis root; directories outside the root cannot resolve
// to analyzed files and are dropped.
func includeDirsRelToRoot(cfg *config.Config, root string) []string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range cfg.IncludeDirs() {
		rel, err := filepath.Rel(absRoot, d)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") {
			continue
		}
		if rel == "." {
			rel = ""
		}
		out = append(out, rel)
	}
	return out
}

// resolveCDeps resolves C/C++ #include directives. Angle includes are stored
// wrapped in <...> by the adapter; quoted includes are bare.
func resolveCDeps(f *lang.FileResult, has map[string]bool, includeDirs []string) []string {
	dir := path.Dir(f.Path)
	if dir == "." {
		dir = ""
	}
	var deps []string
	seen := map[string]bool{}
	for _, imp := range f.Imports {
		angle := strings.HasPrefix(imp, "<") && strings.HasSuffix(imp, ">")
		inc := strings.TrimSuffix(strings.TrimPrefix(imp, "<"), ">")
		var bases []string
		if !angle {
			bases = append(bases, path.Join(dir, inc))
		}
		for _, id := range includeDirs {
			bases = append(bases, path.Join(id, inc))
		}
		for _, b := range bases {
			b = path.Clean(b)
			if b == "." || b == "" {
				continue
			}
			if has[b] && !seen[b] {
				seen[b] = true
				deps = append(deps, b)
				break
			}
		}
	}
	return deps
}

func resolveTSDeps(f *lang.FileResult, has map[string]bool) []string {
	dir := path.Dir(f.Path)
	if dir == "." {
		dir = ""
	}
	var deps []string
	seen := map[string]bool{}
	for _, imp := range f.Imports {
		if !strings.HasPrefix(imp, "./") && !strings.HasPrefix(imp, "../") {
			continue // bare package -> external
		}
		base := path.Join(dir, imp)
		for _, cand := range tsCandidates(base) {
			if has[cand] && !seen[cand] {
				seen[cand] = true
				deps = append(deps, cand)
				break
			}
		}
	}
	return deps
}

// tsCandidates lists file paths worth trying for a resolved TS/JS import,
// preferring source over index and same-language over cross-language.
func tsCandidates(base string) []string {
	exts := []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	var out []string
	// as-is (may already carry an extension)
	out = append(out, base)
	for _, e := range exts {
		if strings.HasSuffix(base, e) {
			continue
		}
		out = append(out, base+e)
	}
	for _, e := range exts {
		out = append(out, base+"/index"+e)
	}
	return out
}

func resolvePyDeps(f *lang.FileResult, has map[string]bool) []string {
	dir := path.Dir(f.Path)
	if dir == "." {
		dir = ""
	}
	var deps []string
	seen := map[string]bool{}
	for _, imp := range f.Imports {
		for _, target := range pyCandidates(dir, imp) {
			if target != "" && has[target] && !seen[target] {
				seen[target] = true
				deps = append(deps, target)
				break
			}
		}
	}
	return deps
}

// pyCandidates converts a Python import module reference into candidate rel
// paths of analyzable files, in preference order ("" means no candidates).
// Absolute dotted imports (a.b) resolve under the analysis root as a/b.py or
// a/b/__init__.py. Relative imports (., .., ...) resolve against the importing
// file's package directory.
func pyCandidates(importingDir, module string) []string {
	dots := 0
	for dots < len(module) && module[dots] == '.' {
		dots++
	}
	rest := module[dots:]

	base := ""
	switch {
	case dots == 0:
		base = rest // analysis root
	case dots == 1:
		base = path.Join(importingDir, rest)
	default:
		up := dots - 1
		cur := importingDir
		for i := 0; i < up && cur != ""; i++ {
			cur = path.Dir(cur)
			if cur == "." {
				cur = ""
			}
		}
		base = path.Join(cur, rest)
	}
	if base == "" {
		return nil
	}
	// module references are dot-separated (a.b.c); a component may itself be a
	// dotted file name in exotic layouts, which we do not chase.
	p := strings.ReplaceAll(base, ".", "/")
	if p == "" {
		return nil
	}
	var cands []string
	if !strings.HasSuffix(p, ".py") {
		cands = append(cands, p+".py", p+"/__init__.py")
	} else {
		cands = append(cands, p)
	}
	return cands
}
