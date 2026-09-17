// Package cfamily provides a shared tree-sitter adapter for C and C++. It maps
// the grammar's concrete syntax tree onto Navune's uniform element model.
// Requires cgo (ADR 0017).
package cfamily

import (
	"fmt"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsc "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tscpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"

	"github.com/lcr/navune/internal/lang"
)

// Parser adapts C (.c) or C++ (.cc/.cpp/.cxx/.c++/.h/.hpp/.hh/.hxx). A .h file
// is content-sniffed to choose the grammar (ADR 0017).
type Parser struct {
	lang lang.Lang
}

// NewC returns a parser for C.
func NewC() *Parser { return &Parser{lang: lang.C} }

// NewCPP returns a parser for C++ (including ambiguous headers).
func NewCPP() *Parser { return &Parser{lang: lang.CPP} }

// Lang implements lang.Parser.
func (p *Parser) Lang() lang.Lang { return p.lang }

// Extensions implements lang.Parser.
func (p *Parser) Extensions() []string {
	if p.lang == lang.C {
		return []string{".c"}
	}
	return []string{".cc", ".cpp", ".cxx", ".c++", ".h", ".hpp", ".hh", ".hxx"}
}

// grammarFor selects the grammar and resolved language for a file. A .h file is
// sniffed: C++ markers win, otherwise the default is C++ (ADR 0017).
func (p *Parser) grammarFor(src []byte, path string) (*sitter.Language, lang.Lang) {
	if p.lang == lang.C {
		return sitter.NewLanguage(tsc.Language()), lang.C
	}
	if strings.HasSuffix(path, ".h") && !hasCppMarkers(src) {
		return sitter.NewLanguage(tsc.Language()), lang.C
	}
	return sitter.NewLanguage(tscpp.Language()), lang.CPP
}

// Parse parses a single C/C++ source file into the element model.
func (p *Parser) Parse(path string, src []byte) (*lang.FileResult, error) {
	grammar, resolved := p.grammarFor(src, path)
	par := sitter.NewParser()
	if err := par.SetLanguage(grammar); err != nil {
		return nil, err
	}
	tree := par.Parse(src, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("parse %s: no syntax tree", path)
	}
	res := &lang.FileResult{
		Path:       path,
		Lang:       resolved,
		Class:      lang.Prod,
		TotalLines: countLines(src),
	}
	if generatedMarker(tree.RootNode(), src) {
		res.MarkGenerated()
	}
	w := &walker{res: res, src: src, cpp: resolved == lang.CPP}
	w.walk(tree.RootNode())
	res.PhysicalSLOC = len(w.lines)
	res.LogicalLOC = w.logical
	return res, nil
}

// hasCppMarkers reports whether comment/string-stripped source contains a C++
// marker. `extern "C"` is checked on the raw source because stripping removes
// its string literal.
func hasCppMarkers(src []byte) bool {
	if strings.Contains(string(src), "extern \"C\"") {
		return true
	}
	s := stripCommentsAndLiterals(string(src))
	if hasWord(s, "class") || hasWord(s, "template") || hasWord(s, "namespace") {
		return true
	}
	return strings.Contains(s, "::")
}

func hasWord(s, word string) bool {
	idx := 0
	for {
		i := strings.Index(s[idx:], word)
		if i < 0 {
			return false
		}
		i += idx
		before := i == 0 || !isIdentChar(rune(s[i-1]))
		after := i+len(word) >= len(s) || !isIdentChar(rune(s[i+len(word)]))
		if before && after {
			return true
		}
		idx = i + 1
	}
}

func isIdentChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// stripCommentsAndLiterals removes // line comments, /* block */ comments, and
// string/char literals so sniffing does not misfire on their contents.
func stripCommentsAndLiterals(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i += 2
		case c == '"' || c == '\'':
			quote := c
			i++
			for i < len(s) {
				if s[i] == '\\' {
					i += 2
					continue
				}
				if s[i] == quote {
					i++
					break
				}
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// generatedMarker mirrors the Go convention in C/C++ comments.
func generatedMarker(root *sitter.Node, src []byte) bool {
	var scan func(n *sitter.Node) bool
	scan = func(n *sitter.Node) bool {
		if n.Kind() == "comment" && n.ChildCount() == 0 {
			text := string(src[n.StartByte():n.EndByte()])
			if strings.Contains(text, "Code generated") && strings.Contains(text, "DO NOT EDIT") {
				return true
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if scan(n.NamedChild(i)) {
				return true
			}
		}
		return false
	}
	return scan(root)
}

var nameKinds = map[string]bool{
	"identifier":       true,
	"field_identifier": true,
	"type_identifier":  true,
}

var commentKinds = map[string]bool{"comment": true}

var stringLike = map[string]bool{
	"string_literal":     true,
	"raw_string_literal": true,
	"char_literal":       true,
	"system_lib_string":  true,
}

var numberKinds = map[string]bool{"number_literal": true}

// decisionKinds add +1 cyclomatic complexity each (ADR 0017, Sonar C/C++).
var decisionKinds = map[string]bool{
	"if_statement":           true,
	"for_statement":          true,
	"for_range_loop":         true,
	"while_statement":        true,
	"do_statement":           true,
	"case_statement":         true,
	"catch_clause":           true,
	"conditional_expression": true,
	"lambda_expression":      true,
}

func isLogicalNode(kind string) bool {
	if strings.HasSuffix(kind, "_statement") {
		return true
	}
	switch kind {
	case "function_definition", "class_specifier", "struct_specifier",
		"declaration", "preproc_include", "preproc_def", "preproc_function_def",
		"namespace_definition", "template_declaration":
		return true
	}
	return false
}

type walker struct {
	res      *lang.FileResult
	src      []byte
	cpp      bool
	lines    map[int]bool
	comments map[int]bool
	logical  int
	funcs    []*lang.Function
}

func (w *walker) walk(root *sitter.Node) {
	w.lines = map[int]bool{}
	w.comments = map[int]bool{}
	w.walkNode(root, nil, "")
	for _, f := range w.funcs {
		f.Length = w.codeLinesBetween(f.StartLine, f.EndLine)
		w.res.Functions = append(w.res.Functions, *f)
	}
	w.res.CommentLines = len(w.comments)
}

func (w *walker) codeLinesBetween(start, end int) int {
	n := 0
	for l := start; l <= end; l++ {
		if w.lines[l] {
			n++
		}
	}
	return n
}

func (w *walker) walkNode(n *sitter.Node, fn *lang.Function, typeName string) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}
	if stringLike[kind] {
		w.emitToken(lang.Token{Kind: lang.TokString, Text: "STR", Line: int(n.StartPosition().Row) + 1})
		return
	}
	if numberKinds[kind] {
		w.emitToken(lang.Token{Kind: lang.TokNumber, Text: "NUM", Line: int(n.StartPosition().Row) + 1})
		return
	}

	if w.cpp && (kind == "class_specifier" || kind == "struct_specifier") {
		name := w.nameOf(n)
		if name != "" {
			w.res.Types = append(w.res.Types, lang.TypeDecl{Name: name, Abstract: w.isAbstractClass(n)})
		}
		next := name
		if next == "" {
			next = typeName
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			w.walkNode(n.NamedChild(i), fn, next)
		}
		return
	}

	if kind == "function_definition" {
		w.addFunction(n, typeName, false)
		return
	}
	if w.cpp && kind == "lambda_expression" {
		w.addFunction(n, typeName, true)
		return
	}

	if kind == "preproc_include" {
		if p := w.includePath(n); p != "" {
			w.res.Imports = append(w.res.Imports, p)
		}
	}

	if fn != nil {
		if decisionKinds[kind] {
			fn.Complexity++
		}
		if kind == "binary_expression" && hasLeafToken(n, "&&", "||") {
			fn.Complexity++
		}
	}
	if isLogicalNode(kind) {
		w.logical++
	}
	w.children(n, fn, typeName)
}

func (w *walker) addFunction(n *sitter.Node, typeName string, isLambda bool) {
	start := int(n.StartPosition().Row) + 1
	name, qualifier := w.funcName(n)
	if isLambda || name == "" {
		name = fmt.Sprintf("<anonymous@%d>", start)
	}
	enclosing := typeName
	if qualifier != "" {
		enclosing = qualifier
	}
	f := &lang.Function{
		Name:       name,
		Enclosing:  enclosing,
		Anonymous:  isLambda,
		Complexity: 1,
		StartLine:  start,
		EndLine:    int(n.EndPosition().Row) + 1,
		Params:     w.paramsOf(n),
	}
	self := ""
	if !isLambda && qualifier == "" {
		self = name
	}
	f.Cognitive, f.Nesting, f.BoolMax, f.Calls = w.analyzeBody(n, self)
	w.funcs = append(w.funcs, f)
	w.children(n, f, typeName)
}

// funcName extracts the declared name and, for qualified definitions
// (Foo::bar), the qualifier.
func (w *walker) funcName(n *sitter.Node) (name, qualifier string) {
	d := n.ChildByFieldName("declarator")
	return w.declaratorName(d)
}

func (w *walker) declaratorName(d *sitter.Node) (name, qualifier string) {
	if d == nil {
		return "", ""
	}
	switch d.Kind() {
	case "identifier", "field_identifier", "type_identifier", "destructor_name", "operator_name":
		return w.text(d), ""
	case "qualified_identifier":
		scope := d.ChildByFieldName("scope")
		nm := d.ChildByFieldName("name")
		q := ""
		if scope != nil {
			q = w.text(scope)
		}
		if nm != nil {
			return w.text(nm), q
		}
	}
	if id := d.ChildByFieldName("declarator"); id != nil {
		if name, q := w.declaratorName(id); name != "" {
			return name, q
		}
	}
	for i := uint(0); i < d.NamedChildCount(); i++ {
		if name, q := w.declaratorName(d.NamedChild(i)); name != "" {
			return name, q
		}
	}
	return "", ""
}

func (w *walker) nameOf(n *sitter.Node) string {
	if nm := n.ChildByFieldName("name"); nm != nil {
		return w.text(nm)
	}
	return ""
}

// isAbstractClass reports whether a C++ class has at least one pure virtual
// method (ADR 0017). The grammar renders `= 0` as a field_declaration's
// default_value; some contexts use a pure_virtual_clause node.
func (w *walker) isAbstractClass(n *sitter.Node) bool {
	body := n.ChildByFieldName("body")
	if body == nil {
		return false
	}
	return w.hasPureVirtual(body)
}

func (w *walker) hasPureVirtual(n *sitter.Node) bool {
	if n.Kind() == "pure_virtual_clause" {
		return true
	}
	if n.Kind() == "field_declaration" {
		if dv := n.ChildByFieldName("default_value"); dv != nil && strings.TrimSpace(w.text(dv)) == "0" {
			if containsToken(n, "virtual") {
				return true
			}
		}
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if w.hasPureVirtual(n.NamedChild(i)) {
			return true
		}
	}
	return false
}

func containsToken(n *sitter.Node, kind string) bool {
	if n.Kind() == kind {
		return true
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if containsToken(n.Child(i), kind) {
			return true
		}
	}
	return false
}

func (w *walker) paramsOf(n *sitter.Node) int {
	// function_definition: parameter_list is nested under the declarator.
	pl := findKind(n, "parameter_list")
	if pl == nil {
		return 0
	}
	cnt := 0
	onlyVoid := false
	for i := uint(0); i < pl.NamedChildCount(); i++ {
		ch := pl.NamedChild(i)
		if ch.Kind() == "comment" {
			continue
		}
		cnt++
		if strings.TrimSpace(w.text(ch)) == "void" {
			onlyVoid = true
		}
	}
	if cnt == 1 && onlyVoid {
		return 0
	}
	return cnt
}

func findKind(n *sitter.Node, kind string) *sitter.Node {
	if n.Kind() == kind {
		return n
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if found := findKind(n.NamedChild(i), kind); found != nil {
			return found
		}
	}
	return nil
}

func (w *walker) includePath(n *sitter.Node) string {
	p := n.ChildByFieldName("path")
	if p == nil {
		return ""
	}
	switch p.Kind() {
	case "string_literal":
		return unquote(w.text(p))
	case "system_lib_string":
		return w.text(p) // keeps the angle brackets as an external marker
	}
	return w.text(p)
}

// analyzeBody computes cognitive complexity, nesting, boolean-condition max,
// and direct callees (ADR 0015).
func (w *walker) analyzeBody(n *sitter.Node, self string) (cognitive, nesting, boolMax int, calls []string) {
	body := n.ChildByFieldName("body")
	if body == nil {
		return 0, 0, 0, nil
	}
	c := &cogWalk{w: w, self: self}
	c.node(body, "")
	c.cognitive += c.logical + c.recursion
	return c.cognitive, c.maxDepth, c.boolMax, c.calls
}

type cogWalk struct {
	w         *walker
	cognitive int
	depth     int
	maxDepth  int
	boolMax   int
	self      string
	calls     []string
	recursion int
	logical   int
}

func (c *cogWalk) nest(n *sitter.Node) {
	if n == nil {
		return
	}
	c.depth++
	if c.depth > c.maxDepth {
		c.maxDepth = c.depth
	}
	c.node(n, "")
	c.depth--
}

func sameNode(a, b *sitter.Node) bool {
	return a != nil && b != nil && a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte()
}

func (c *cogWalk) walkChildren(n *sitter.Node, inh string) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c.node(n.NamedChild(i), inh)
	}
}

func (c *cogWalk) walkOthers(n, body *sitter.Node) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		ch := n.NamedChild(i)
		if sameNode(ch, body) {
			continue
		}
		c.node(ch, "")
	}
}

func (c *cogWalk) node(n *sitter.Node, inh string) {
	if n == nil {
		return
	}
	kind := n.Kind()
	if kind == "function_definition" || kind == "lambda_expression" {
		return // separate function unit
	}
	switch kind {
	case "if_statement":
		c.cognitive += 1 + c.depth
		c.nest(n.ChildByFieldName("consequence"))
		if alt := n.ChildByFieldName("alternative"); alt != nil {
			c.elseClause(alt)
		}
		c.node(n.ChildByFieldName("condition"), "")
		return
	case "for_statement", "for_range_loop", "while_statement", "do_statement":
		c.cognitive += 1 + c.depth
		body := n.ChildByFieldName("body")
		c.nest(body)
		c.walkOthers(n, body)
		return
	case "switch_statement":
		c.cognitive += 1 + c.depth
		body := n.ChildByFieldName("body")
		c.nest(body)
		c.walkOthers(n, body)
		return
	case "catch_clause":
		c.cognitive += 1 + c.depth
		body := n.ChildByFieldName("body")
		c.nest(body)
		c.walkOthers(n, body)
		return
	case "goto_statement":
		c.cognitive++
		c.walkChildren(n, "")
		return
	case "conditional_expression":
		c.cognitive += 1 + c.depth
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c.nest(n.NamedChild(i))
		}
		return
	case "binary_expression":
		op := c.w.binaryOp(n)
		if op == "&&" || op == "||" {
			if op != inh {
				c.logical++
				if cnt := c.boolOperands(n); cnt > c.boolMax {
					c.boolMax = cnt
				}
			}
			inh = op
		} else {
			inh = ""
		}
		c.walkChildren(n, inh)
		return
	case "call_expression":
		if name := c.w.calleeName(n); name != "" {
			c.calls = append(c.calls, name)
			if c.self != "" && name == c.self {
				c.recursion++
			}
		}
		c.walkChildren(n, "")
		return
	}
	c.walkChildren(n, inh)
}

func (c *cogWalk) elseClause(alt *sitter.Node) {
	if alt.NamedChildCount() == 0 {
		return
	}
	stmt := alt.NamedChild(0)
	if stmt.Kind() == "if_statement" {
		c.cognitive++
		c.nest(stmt.ChildByFieldName("consequence"))
		if a := stmt.ChildByFieldName("alternative"); a != nil {
			c.elseClause(a)
		}
		c.node(stmt.ChildByFieldName("condition"), "")
		return
	}
	c.cognitive++
	c.nest(stmt)
}

func (w *walker) binaryOp(n *sitter.Node) string {
	for i := uint(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if !ch.IsNamed() && ch.ChildCount() == 0 {
			if ch.Kind() == "&&" || ch.Kind() == "||" {
				return ch.Kind()
			}
		}
	}
	return ""
}

func (w *walker) calleeName(n *sitter.Node) string {
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	switch fn.Kind() {
	case "identifier", "field_identifier":
		return w.text(fn)
	case "field_expression":
		if f := fn.ChildByFieldName("field"); f != nil {
			return w.text(f)
		}
	}
	return ""
}

func (c *cogWalk) boolOperands(n *sitter.Node) int {
	if n == nil || n.Kind() != "binary_expression" {
		return 1
	}
	op := c.w.binaryOp(n)
	if op != "&&" && op != "||" {
		return 1
	}
	left := n.ChildByFieldName("left")
	right := n.ChildByFieldName("right")
	if left == nil && n.NamedChildCount() > 0 {
		left = n.NamedChild(0)
	}
	if right == nil && n.NamedChildCount() > 1 {
		right = n.NamedChild(1)
	}
	return c.boolOperands(left) + c.boolOperands(right)
}

func (w *walker) recordComment(n *sitter.Node) {
	start := int(n.StartPosition().Row) + 1
	for i, line := range strings.Split(w.text(n), "\n") {
		if isSignificantComment(line) {
			w.comments[start+i] = true
		}
	}
}

func isSignificantComment(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func (w *walker) children(n *sitter.Node, fn *lang.Function, typeName string) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.ChildCount() == 0 {
			w.emitLeaf(c)
			continue
		}
		w.walkNode(c, fn, typeName)
	}
}

func (w *walker) emitLeaf(n *sitter.Node) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}
	if stringLike[kind] {
		return
	}
	line := int(n.StartPosition().Row) + 1
	if numberKinds[kind] {
		w.emitToken(lang.Token{Kind: lang.TokNumber, Text: "NUM", Line: line})
		return
	}
	t := lang.Token{Line: line}
	if nameKinds[kind] {
		t.Kind = lang.TokIdent
		t.Text = w.text(n)
	} else {
		t.Kind = lang.TokKeyword
		t.Text = w.text(n)
	}
	w.emitToken(t)
}

func (w *walker) text(n *sitter.Node) string {
	return string(w.src[n.StartByte():n.EndByte()])
}

func (w *walker) emitToken(t lang.Token) {
	w.res.Tokens = append(w.res.Tokens, t)
	w.lines[t.Line] = true
}

func hasLeafToken(n *sitter.Node, kinds ...string) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() && c.ChildCount() == 0 {
			for _, k := range kinds {
				if c.Kind() == k {
					return true
				}
			}
		}
	}
	return false
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	n := strings.Count(string(src), "\n")
	if src[len(src)-1] != '\n' {
		n++
	}
	return n
}
