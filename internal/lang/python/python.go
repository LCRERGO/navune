// Package python provides a tree-sitter based adapter for Python. It maps the
// grammar's concrete syntax tree onto Navune's uniform element model. Requires
// cgo (ADR 0007 revision).
package python

import (
	"fmt"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"

	"github.com/lcr/navune/internal/lang"
)

// Parser adapts Python source files.
type Parser struct{}

// New returns a Python parser adapter.
func New() *Parser { return &Parser{} }

// Lang implements lang.Parser.
func (*Parser) Lang() lang.Lang { return lang.Python }

// Extensions implements lang.Parser.
func (*Parser) Extensions() []string { return []string{".py"} }

// Parse parses a single Python source file into the element model.
func (*Parser) Parse(path string, src []byte) (*lang.FileResult, error) {
	par := sitter.NewParser()
	if err := par.SetLanguage(sitter.NewLanguage(tspy.Language())); err != nil {
		return nil, err
	}
	tree := par.Parse(src, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("parse %s: no syntax tree", path)
	}
	res := &lang.FileResult{
		Path:       path,
		Lang:       lang.Python,
		Class:      lang.Prod,
		TotalLines: countLines(src),
	}
	if generatedMarker(tree.RootNode(), src) {
		res.MarkGenerated()
	}
	w := &walker{res: res, src: src}
	w.walk(tree.RootNode())
	res.PhysicalSLOC = len(w.lines)
	res.LogicalLOC = w.logical
	return res, nil
}

// generatedMarker mirrors the Go convention (codegen tools emit a header
// comment; Python tools commonly use "# Code generated ... DO NOT EDIT.").
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

// nameKinds hold identifier text.
var nameKinds = map[string]bool{"identifier": true}

// funcKinds that become Function units.
var funcKinds = map[string]bool{
	"function_definition": true,
}

// typeKinds become TypeDecl units. Python has no language-level abstract
// type, so Abstract is always false (ABCs are duck-typed conventions).
var typeKinds = map[string]bool{"class_definition": true}

// decisionKinds add +1 cyclomatic complexity each. elif/except/case branches
// are decision points; boolean and/or and ternary each add one.
var decisionKinds = map[string]bool{
	"if_statement":           true,
	"elif_clause":            true,
	"for_statement":          true,
	"while_statement":        true,
	"except_clause":          true,
	"case_clause":            true,
	"conditional_expression": true, // x if c else y
}

// operatorLeaves inside boolean_operator that add complexity.
var logicalOps = map[string]bool{"and": true, "or": true}

var commentKinds = map[string]bool{"comment": true}

// stringNodes are emitted as a single STR token; their children
// (string_start/content/end, interpolation) are not descended into.
var stringNodes = map[string]bool{"string": true}

// number leaf kinds are normalized to NUM.
var numberKinds = map[string]bool{"integer": true, "float": true}

func isLogicalNode(kind string) bool {
	if strings.HasSuffix(kind, "_statement") {
		return true
	}
	switch kind {
	case "import_statement", "import_from_statement", "future_import_statement",
		"function_definition", "class_definition",
		"expression_list", "exec_statement":
		return true
	}
	return false
}

type walker struct {
	res      *lang.FileResult
	src      []byte
	lines    map[int]bool
	comments map[int]bool
	logical  int
	funcs    []*lang.Function
}

func (w *walker) walk(root *sitter.Node) {
	w.lines = map[int]bool{}
	w.comments = map[int]bool{}
	w.walkNode(root, nil, "", false)
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

// walkNode traverses n. fn is the function currently being attributed
// complexity, typeName the nearest enclosing class name, and inFunc reports
// whether we are already lexically inside any function or lambda (used so only
// direct methods of a class get an Enclosing type).
func (w *walker) walkNode(n *sitter.Node, fn *lang.Function, typeName string, inFunc bool) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}
	if stringNodes[kind] {
		w.emitToken(lang.Token{Kind: lang.TokString, Text: "STR", Line: int(n.StartPosition().Row) + 1})
		return
	}
	if typeKinds[kind] {
		name := w.nameOf(n)
		if name != "" {
			w.res.Types = append(w.res.Types, lang.TypeDecl{Name: name})
		}
		next := name
		if next == "" {
			next = typeName
		}
		w.children(n, fn, next, inFunc)
		return
	}
	if funcKinds[kind] {
		w.addFunction(n, fn, typeName, inFunc, false)
		return
	}
	// lambda is an anonymous function unit (ADR 0015)
	if kind == "lambda" {
		w.addFunction(n, fn, typeName, inFunc, true)
		return
	}

	if fn != nil {
		if decisionKinds[kind] {
			fn.Complexity++
		}
		if kind == "boolean_operator" && hasLeafToken(n, logicalOps) {
			fn.Complexity++
		}
	}

	if kind == "import_statement" || kind == "import_from_statement" || kind == "future_import_statement" {
		w.collectImports(n)
	}
	if isLogicalNode(kind) {
		w.logical++
	}
	w.children(n, fn, typeName, inFunc)
}

// addFunction emits a named function or lambda as a first-class unit and
// computes its extended measures (ADR 0015).
func (w *walker) addFunction(n *sitter.Node, fn *lang.Function, typeName string, inFunc, isLambda bool) {
	start := int(n.StartPosition().Row) + 1
	name := w.nameOf(n)
	enclosing := ""
	if isLambda {
		name = fmt.Sprintf("<anonymous@%d>", start)
		if fn != nil {
			enclosing = fn.Name
		} else {
			enclosing = typeName
		}
	} else if typeName != "" && !inFunc {
		enclosing = typeName
	}
	f := &lang.Function{
		Name:       name,
		Enclosing:  enclosing,
		Anonymous:  isLambda,
		Complexity: 1,
		StartLine:  start,
		EndLine:    int(n.EndPosition().Row) + 1,
		Params:     w.paramsOf(n, !isLambda && enclosing != ""),
	}
	self := ""
	if !isLambda {
		self = name
	}
	f.Cognitive, f.Nesting, f.BoolMax, f.Calls = w.analyzeBody(n, self)
	w.funcs = append(w.funcs, f)
	w.children(n, f, typeName, true)
}

func (w *walker) paramsOf(n *sitter.Node, isMethod bool) int {
	p := n.ChildByFieldName("parameters")
	if p == nil {
		return 0
	}
	cnt := 0
	first := ""
	for i := uint(0); i < p.NamedChildCount(); i++ {
		ch := p.NamedChild(i)
		if ch.Kind() == "comment" {
			continue
		}
		if first == "" {
			first = w.paramName(ch)
		}
		cnt++
	}
	if isMethod && (first == "self" || first == "cls") && cnt > 0 {
		cnt--
	}
	return cnt
}

func (w *walker) paramName(ch *sitter.Node) string {
	if ch.Kind() == "identifier" {
		return w.text(ch)
	}
	for i := uint(0); i < ch.NamedChildCount(); i++ {
		if ch.NamedChild(i).Kind() == "identifier" {
			return w.text(ch.NamedChild(i))
		}
	}
	return ""
}

// analyzeBody computes cognitive complexity, maximum control-structure nesting,
// and direct callees for a function or lambda node (ADR 0015).
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
	if funcKinds[kind] || kind == "lambda" {
		return // separate function unit
	}
	switch kind {
	case "if_statement":
		c.cognitive += 1 + c.depth
		c.nest(n.ChildByFieldName("consequence"))
		c.node(n.ChildByFieldName("condition"), "")
		c.alternative(n.ChildByFieldName("alternative"))
		return
	case "for_statement", "while_statement", "except_clause", "match_statement":
		c.cognitive += 1 + c.depth
		body := n.ChildByFieldName("body")
		c.nest(body)
		c.walkOthers(n, body)
		return
	case "conditional_expression":
		c.cognitive += 1 + c.depth
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c.nest(n.NamedChild(i))
		}
		return
	case "boolean_operator":
		op := c.w.operatorText(n)
		if op == "and" || op == "or" {
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
	case "call":
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

func (c *cogWalk) alternative(alt *sitter.Node) {
	if alt == nil {
		return
	}
	switch alt.Kind() {
	case "elif_clause":
		c.cognitive++ // else-if: no nesting penalty
		c.nest(alt.ChildByFieldName("consequence"))
		c.node(alt.ChildByFieldName("condition"), "")
		c.alternative(alt.ChildByFieldName("alternative"))
	case "else_clause":
		c.cognitive++
		c.nest(alt.ChildByFieldName("body"))
	default:
		c.nest(alt)
	}
}

func (w *walker) operatorText(n *sitter.Node) string {
	if op := n.ChildByFieldName("operator"); op != nil {
		return w.text(op)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if !ch.IsNamed() && ch.ChildCount() == 0 {
			if ch.Kind() == "and" || ch.Kind() == "or" {
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
	case "identifier":
		return w.text(fn)
	case "attribute":
		if attr := fn.ChildByFieldName("attribute"); attr != nil {
			return w.text(attr)
		}
	}
	return ""
}

// boolOperands counts the leaf conditions in a logical expression tree.
func (c *cogWalk) boolOperands(n *sitter.Node) int {
	if n == nil || n.Kind() != "boolean_operator" {
		return 1
	}
	op := c.w.operatorText(n)
	if op != "and" && op != "or" {
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

// recordComment counts significant comment lines (ADR 0015).
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

func (w *walker) children(n *sitter.Node, fn *lang.Function, typeName string, inFunc bool) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.ChildCount() == 0 {
			w.emitLeaf(c)
			continue
		}
		w.walkNode(c, fn, typeName, inFunc)
	}
}

func (w *walker) emitLeaf(n *sitter.Node) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}
	if stringNodes[kind] {
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

func (w *walker) emitToken(t lang.Token) {
	w.res.Tokens = append(w.res.Tokens, t)
	w.lines[t.Line] = true
}

func (w *walker) text(n *sitter.Node) string {
	return string(w.src[n.StartByte():n.EndByte()])
}

func (w *walker) nameOf(n *sitter.Node) string {
	// function_definition: def <identifier>; class_definition: class <identifier>
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if nameKinds[c.Kind()] {
			return w.text(c)
		}
	}
	return ""
}

// collectImports appends module references from an import statement.
func (w *walker) collectImports(n *sitter.Node) {
	switch n.Kind() {
	case "import_statement", "future_import_statement":
		// `import a.b`, `import a.b as p`: collect every dotted module name.
		var collect func(m *sitter.Node)
		collect = func(m *sitter.Node) {
			if m.Kind() == "dotted_name" {
				w.res.Imports = append(w.res.Imports, w.text(m))
				return
			}
			for i := uint(0); i < m.ChildCount(); i++ {
				collect(m.Child(i))
			}
		}
		collect(n)
	case "import_from_statement":
		w.collectFromImport(n)
	}
}

func (w *walker) collectFromImport(n *sitter.Node) {
	// grammar: from <relative_import?> import <names>
	// relative_import text is like "..", "..pkg", ".x" (dots + optional module)
	// and is nested; absolute form has a bare dotted_name before `import`.
	dots := ""
	mod := ""
	var imported []string
	seenImport := false
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "relative_import":
			txt := w.text(c)
			dots = txt[:len(txt)-len(strings.TrimLeft(txt, "."))]
			mod = strings.TrimLeft(txt, ".")
		case "import":
			seenImport = true
		case "dotted_name":
			if !seenImport && mod == "" {
				mod = w.text(c)
			} else if seenImport {
				imported = append(imported, w.text(c))
			}
		case "aliased_import":
			if seenImport {
				for j := uint(0); j < c.NamedChildCount(); j++ {
					cc := c.NamedChild(j)
					if cc.Kind() == "dotted_name" {
						imported = append(imported, w.text(cc))
						break
					}
				}
			}
		}
	}
	prefix := dots
	if mod == "" && dots != "" {
		// "from . import util" -> depend on a sibling module "util"
		if len(imported) > 0 {
			w.res.Imports = append(w.res.Imports, prefix+imported[0])
		}
		return
	}
	if mod == "" {
		return
	}
	w.res.Imports = append(w.res.Imports, prefix+mod)
	// `from x import y`: y may be a submodule (x.y); resolver tries it first.
	if len(imported) > 0 {
		sub := prefix + mod + "." + imported[0]
		if sub != prefix+mod {
			w.res.Imports = append(w.res.Imports, sub)
		}
	}
}

func hasLeafToken(n *sitter.Node, kinds map[string]bool) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.ChildCount() == 0 && kinds[c.Kind()] {
			return true
		}
	}
	return false
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
