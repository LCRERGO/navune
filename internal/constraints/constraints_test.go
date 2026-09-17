package constraints

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/gate"
	"github.com/lcr/navune/internal/lang"
	"github.com/lcr/navune/internal/report"
	"github.com/lcr/navune/internal/smell"
)

// repoRoot locates this repository from the test file's own path.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate constraints test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

var (
	selfOnce sync.Once
	selfRep  *analysis.Report
	selfErr  error
)

// self analyzes Navune's own repository once per test binary. Verbose detail is
// enabled so the anonymous-function invariant can be checked.
func self(t *testing.T) *analysis.Report {
	t.Helper()
	selfOnce.Do(func() {
		selfRep, selfErr = analysis.AnalyzeOptions(repoRoot(t), config.Defaults(), analysis.Options{Verbose: true})
	})
	if selfErr != nil {
		t.Fatalf("self-analysis failed: %v", selfErr)
	}
	return selfRep
}

// --- helpers -----------------------------------------------------------------

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

// without returns base minus the named optional keys.
func without(base map[string]bool, drop ...string) map[string]bool {
	m := make(map[string]bool, len(base))
	for k := range base {
		m[k] = true
	}
	for _, d := range drop {
		delete(m, d)
	}
	return m
}

func keys(m map[string]json.RawMessage) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func decodeObj(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode object: %v", err)
	}
	return m
}

func decodeArr(t *testing.T, raw json.RawMessage) []map[string]json.RawMessage {
	t.Helper()
	var a []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("decode array: %v", err)
	}
	return a
}

// checkKeys enforces the frozen schema: every rendered key must be expected
// (no silent additions/renames) and every required key must be present.
func checkKeys(t *testing.T, label string, got, expected, required map[string]bool) {
	t.Helper()
	for k := range got {
		if !expected[k] {
			t.Errorf("%s: unexpected key %q (schema is frozen; update this constraint deliberately)", label, k)
		}
	}
	for k := range required {
		if !got[k] {
			t.Errorf("%s: missing required key %q", label, k)
		}
	}
}

// --- invariants --------------------------------------------------------------

// TestDeterminismByteStable asserts the whole pipeline is byte-stable: two
// analyses of the same tree render identical output in every format.
func TestDeterminismByteStable(t *testing.T) {
	root := repoRoot(t)
	cfg := config.Defaults()
	opts := analysis.Options{Verbose: true}
	a, err := analysis.AnalyzeOptions(root, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := analysis.AnalyzeOptions(root, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []report.Format{report.Text, report.JSON, report.YAML, report.Mermaid} {
		oa, err := report.Render(a, f)
		if err != nil {
			t.Fatalf("render %s: %v", f, err)
		}
		ob, err := report.Render(b, f)
		if err != nil {
			t.Fatalf("render %s: %v", f, err)
		}
		if oa != ob {
			t.Errorf("format %s is not byte-stable across runs", f)
		}
	}
}

// TestSchemaContractLock pins the JSON schema (schema_version 1, additive-only).
func TestSchemaContractLock(t *testing.T) {
	r := self(t)
	raw, err := report.Render(r, report.JSON)
	if err != nil {
		t.Fatal(err)
	}
	top := decodeObj(t, []byte(raw))
	checkKeys(t, "top-level", keys(top), topExpected, topRequired)

	summary := decodeObj(t, top["summary"])
	checkKeys(t, "summary", keys(summary), summaryExpected, summaryRequired)

	files := decodeArr(t, top["files"])
	if len(files) == 0 {
		t.Fatal("expected analyzed files")
	}
	for i, f := range files {
		label := fmt.Sprintf("files[%d]", i)
		checkKeys(t, label, keys(f), fileExpected, fileRequired)
		if fd, ok := f["functions_detail"]; ok {
			for j, fn := range decodeArr(t, fd) {
				checkKeys(t, fmt.Sprintf("%s.functions_detail[%d]", label, j), keys(fn), funcExpected, funcRequired)
			}
		}
	}
	for i, c := range decodeArr(t, top["cycles"]) {
		checkKeys(t, fmt.Sprintf("cycles[%d]", i), keys(c), cycleExpected, cycleExpected)
	}
	for i, b := range decodeArr(t, top["budgets"]) {
		checkKeys(t, fmt.Sprintf("budgets[%d]", i), keys(b), budgetExpected, budgetRequired)
	}
	for i, iss := range decodeArr(t, top["issues"]) {
		checkKeys(t, fmt.Sprintf("issues[%d]", i), keys(iss), issueExpected, issueRequired)
	}
	if bl, ok := summary["by_language"]; ok {
		byLang := decodeObj(t, bl)
		if len(byLang) == 0 {
			t.Error("by_language is present but empty")
		}
		for name, obj := range byLang {
			checkKeys(t, "by_language."+name, keys(decodeObj(t, obj)), langExpected, langExpected)
		}
	}
}

// TestOrderingStable asserts every collection the contract promises to sort.
func TestOrderingStable(t *testing.T) {
	r := self(t)
	for i := 1; i < len(r.Files); i++ {
		if r.Files[i-1].Path > r.Files[i].Path {
			t.Errorf("files not sorted: %q before %q", r.Files[i-1].Path, r.Files[i].Path)
		}
	}
	for i := 1; i < len(r.Cycles); i++ {
		if cycleFirst(r.Cycles[i-1]) > cycleFirst(r.Cycles[i]) {
			t.Errorf("cycles not sorted by first member")
		}
	}
	for i := 1; i < len(r.Budgets); i++ {
		a, b := r.Budgets[i-1], r.Budgets[i]
		if a.Language > b.Language || (a.Language == b.Language && a.Key > b.Key) {
			t.Errorf("budgets not sorted by (language,key): %s/%s before %s/%s", a.Language, a.Key, b.Language, b.Key)
		}
	}
	for i := 1; i < len(r.Issues); i++ {
		if issueLess(r.Issues[i], r.Issues[i-1]) {
			t.Errorf("issues not sorted by (file,line,rule): %+v before %+v", r.Issues[i], r.Issues[i-1])
		}
	}
}

func cycleFirst(c analysis.CycleReport) string {
	if len(c.Members) == 0 {
		return ""
	}
	return c.Members[0]
}

func issueLess(a, b smell.Issue) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Rule < b.Rule
}

// TestTestNamespaceExcluded asserts test files never leak into production math.
func TestTestNamespaceExcluded(t *testing.T) {
	r := self(t)
	prod, test := map[string]bool{}, map[string]bool{}
	for _, f := range r.Files {
		switch f.Class {
		case "prod":
			prod[f.Path] = true
		case "test":
			test[f.Path] = true
		default:
			t.Errorf("unknown file class %q", f.Class)
		}
	}
	if len(test) == 0 {
		t.Fatal("expected test files in Navune's own tree")
	}
	if r.Summary.TestFiles != len(test) {
		t.Errorf("summary.test_files = %d, want %d", r.Summary.TestFiles, len(test))
	}
	if r.Summary.ProdFiles != len(prod) {
		t.Errorf("summary.prod_files = %d, want %d", r.Summary.ProdFiles, len(prod))
	}
	for _, f := range r.Files {
		if f.Class != "test" {
			continue
		}
		if f.Ca != 0 || f.Ce != 0 || f.InCycle || f.DupTokens != 0 || len(f.FunctionsDetail) != 0 {
			t.Errorf("test file %s carries production-only data: ca=%d ce=%d cycle=%v dup=%d detail=%d",
				f.Path, f.Ca, f.Ce, f.InCycle, f.DupTokens, len(f.FunctionsDetail))
		}
	}
	for _, e := range r.InternalEdges {
		if test[e[0]] || test[e[1]] {
			t.Errorf("internal edge touches a test file: %v", e)
		}
	}
	for _, iss := range r.Issues {
		if test[iss.File] {
			t.Errorf("smell issue on a test file: %+v", iss)
		}
	}
}

// TestInternalEdgesOnly asserts edges are internal and Ca/Ce match degree.
func TestInternalEdgesOnly(t *testing.T) {
	r := self(t)
	prod := map[string]bool{}
	for _, f := range r.Files {
		if f.Class == "prod" {
			prod[f.Path] = true
		}
	}
	outdeg, indeg := map[string]int{}, map[string]int{}
	for _, e := range r.InternalEdges {
		if !prod[e[0]] || !prod[e[1]] {
			t.Errorf("edge %v is not between analyzed production files", e)
		}
		outdeg[e[0]]++
		indeg[e[1]]++
	}
	for _, f := range r.Files {
		if f.Class != "prod" {
			continue
		}
		if f.Ce != outdeg[f.Path] {
			t.Errorf("%s: ce = %d, want out-degree %d", f.Path, f.Ce, outdeg[f.Path])
		}
		if f.Ca != indeg[f.Path] {
			t.Errorf("%s: ca = %d, want in-degree %d", f.Path, f.Ca, indeg[f.Path])
		}
	}
}

// TestCanonicalLanguageKeys asserts canonical output keys and alias resolution.
func TestCanonicalLanguageKeys(t *testing.T) {
	r := self(t)
	canonical := map[string]bool{}
	for _, l := range lang.Languages() {
		canonical[string(l)] = true
	}
	if len(r.Summary.ByLanguage) == 0 {
		t.Fatal("by_language is empty")
	}
	for k := range r.Summary.ByLanguage {
		if !canonical[k] {
			t.Errorf("by_language key %q is not a canonical language key", k)
		}
	}
	if _, ok := r.Summary.ByLanguage[string(lang.Go)]; !ok {
		t.Errorf("Navune is written in Go; by_language is missing %q", lang.Go)
	}
	aliases := map[string]lang.Lang{
		"go": lang.Go, "typescript": lang.TypeScript, "ts": lang.TypeScript,
		"javascript": lang.JavaScript, "js": lang.JavaScript,
		"python": lang.Python, "py": lang.Python,
		"c": lang.C, "cpp": lang.CPP, "c++": lang.CPP, "cxx": lang.CPP,
		"java": lang.Java, "rust": lang.Rust, "rs": lang.Rust,
	}
	for in, want := range aliases {
		got, ok := lang.Canonical(in)
		if !ok || got != want {
			t.Errorf("lang.Canonical(%q) = (%q,%v), want %q", in, got, ok, want)
		}
	}
}

// TestBudgetContract asserts the gate's documented shape and semantics.
func TestBudgetContract(t *testing.T) {
	def := config.Defaults()
	want := []string{"avg_complexity", "worst_complexity", "max_duplication_pct", "max_cycle_members", "max_in_cycle_pct"}
	if len(def.Budgets) != len(want) {
		t.Errorf("defaults ship %d budgets, want exactly %d", len(def.Budgets), len(want))
	}
	for _, k := range want {
		if _, ok := def.Budgets[k]; !ok {
			t.Errorf("default budgets missing %q", k)
		}
	}
	if _, ok := def.Budgets["comment-density"]; ok {
		t.Error("comment-density must not be a budget key (it is a lower-bound smell rule)")
	}

	r := self(t)
	if len(r.Budgets) != len(want) {
		t.Errorf("report has %d budgets, want the %d defaults (no per-language budgets configured)", len(r.Budgets), len(want))
	}
	for _, b := range r.Budgets {
		if b.Language != "" {
			t.Errorf("unexpected per-language budget %s/%s with default config", b.Language, b.Key)
		}
		if b.Breached != (b.Value > b.Limit) {
			t.Errorf("budget %s: breached=%v but value=%v limit=%v (budgets are upper-bound-only)", b.Key, b.Breached, b.Value, b.Limit)
		}
	}
	if r.Composite < 0 || r.Composite > 100 {
		t.Errorf("composite %.2f outside [0,100]", r.Composite)
	}
	breached := false
	for _, b := range r.Budgets {
		if b.Breached && b.Tier == "error" {
			breached = true
		}
	}
	wantExit := gate.ExitPass
	if breached {
		wantExit = gate.ExitBreach
	}
	if r.ExitCode != wantExit {
		t.Errorf("exit code %d inconsistent with error-tier breaches (want %d)", r.ExitCode, wantExit)
	}
}

// TestIssueContract asserts the smell/issue invariants.
func TestIssueContract(t *testing.T) {
	r := self(t)
	rules := map[string]bool{}
	for _, id := range smell.RuleIDs() {
		rules[id] = true
	}
	severities := set("blocker", "high", "medium", "low", "info")
	test := map[string]bool{}
	for _, f := range r.Files {
		if f.Class == "test" {
			test[f.Path] = true
		}
	}
	for _, iss := range r.Issues {
		if !rules[iss.Rule] {
			t.Errorf("issue references unknown rule %q", iss.Rule)
		}
		if !severities[iss.Severity] {
			t.Errorf("issue %s has non-MQR severity %q", iss.Rule, iss.Severity)
		}
		if test[iss.File] {
			t.Errorf("issue on a test file: %+v", iss)
		}
		if iss.Line < 1 {
			t.Errorf("issue %s has invalid line %d", iss.Rule, iss.Line)
		}
		if iss.Threshold <= 0 {
			t.Errorf("issue %s has non-positive threshold %v", iss.Rule, iss.Threshold)
		}
	}
}

// TestByLanguageShape asserts by_language excludes cross-language metrics.
func TestByLanguageShape(t *testing.T) {
	r := self(t)
	raw, err := report.Render(r, report.JSON)
	if err != nil {
		t.Fatal(err)
	}
	top := decodeObj(t, []byte(raw))
	summary := decodeObj(t, top["summary"])
	bl, ok := summary["by_language"]
	if !ok {
		t.Fatal("summary.by_language missing")
	}
	byLang := decodeObj(t, bl)
	if len(byLang) == 0 {
		t.Fatal("summary.by_language is empty")
	}
	for name, obj := range byLang {
		checkKeys(t, "by_language."+name, keys(decodeObj(t, obj)), langExpected, langExpected)
	}
	// Cycle/coupling keys must never appear in a per-language summary.
	for _, banned := range []string{"cycles", "files_in_cycle", "max_cycle_members", "in_cycle_pct", "ca", "ce", "instability", "abstractness", "distance"} {
		if langExpected[banned] {
			t.Errorf("by_language expected set wrongly includes cross-language key %q", banned)
		}
	}
}

// TestAnonymousFunctionsMeasured asserts anonymous functions are first-class
// and carry measures.
func TestAnonymousFunctionsMeasured(t *testing.T) {
	r := self(t)
	anon, measured := 0, 0
	for _, f := range r.Files {
		for _, fn := range f.FunctionsDetail {
			if !fn.Anonymous {
				continue
			}
			anon++
			if !strings.HasPrefix(fn.Name, "<anonymous@") {
				t.Errorf("anonymous function name %q does not use the <anonymous@Lstart> form", fn.Name)
			}
			if fn.Complexity >= 1 && fn.Length > 0 {
				measured++
			}
		}
	}
	if anon == 0 {
		t.Fatal("expected anonymous functions in Navune's own tree")
	}
	if measured == 0 {
		t.Error("anonymous functions carry no measures")
	}
}

// --- expected schema field sets ---------------------------------------------

var (
	topExpected = set("schema_version", "tool", "root", "module_path", "in_module",
		"summary", "files", "cycles", "issues", "budgets", "composite", "exit_code")
	topRequired = without(topExpected, "module_path")

	summaryExpected = set("prod_files", "test_files", "skipped_dirs", "generated_files",
		"physical_sloc", "logical_loc", "comment_lines", "comment_pct", "functions",
		"avg_complexity", "worst_complexity", "worst_function", "avg_cognitive_complexity",
		"worst_cognitive_complexity", "max_function_length", "max_nesting", "max_params",
		"types", "abstract_types", "total_tokens", "dup_tokens", "dup_pct", "dup_blocks",
		"duplicated_lines", "issue_count", "cycles", "files_in_cycle", "max_cycle_members",
		"in_cycle_pct", "by_language")
	summaryRequired = without(summaryExpected, "worst_function", "by_language")

	fileExpected = set("path", "lang", "class", "package", "import_path", "total_lines",
		"physical_sloc", "logical_loc", "comment_lines", "comment_pct", "functions",
		"avg_complexity", "worst_complexity", "worst_function", "avg_cognitive_complexity",
		"cognitive_complexity", "max_function_length", "max_nesting", "max_params",
		"types", "abstract_types", "ca", "ce", "instability", "abstractness", "distance",
		"in_cycle", "cycle", "dup_tokens", "dup_pct", "duplicated_lines", "functions_detail")
	fileRequired = without(fileExpected, "import_path", "worst_function", "ca", "ce",
		"instability", "abstractness", "distance", "cycle", "functions_detail")

	funcExpected = set("name", "enclosing", "start_line", "end_line", "complexity",
		"cognitive_complexity", "length", "nesting", "params", "anonymous")
	funcRequired = without(funcExpected, "enclosing", "anonymous")

	cycleExpected = set("index", "size", "name", "members")

	budgetExpected = set("key", "label", "language", "limit", "value", "tier", "breached")
	budgetRequired = without(budgetExpected, "language")

	issueExpected = set("rule", "severity", "file", "line", "function", "value", "threshold")
	issueRequired = without(issueExpected, "function")

	langExpected = set("prod_files", "physical_sloc", "logical_loc", "comment_lines",
		"comment_pct", "functions", "avg_complexity", "worst_complexity",
		"avg_cognitive_complexity", "worst_cognitive_complexity", "max_function_length",
		"max_nesting", "max_params", "types", "abstract_types", "total_tokens",
		"dup_tokens", "dup_pct", "duplicated_lines", "issue_count")
)
