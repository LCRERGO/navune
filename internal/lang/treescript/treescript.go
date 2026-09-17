// Package treescript provides tree-sitter based adapters for JavaScript and
// TypeScript. It maps the grammar's concrete syntax tree onto Navune's uniform
// element model (file -> type/function, complexity, imports, tokens) using the
// official tree-sitter Go bindings. These require cgo (ADR 0007 revision).
package treescript

import (
	"fmt"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/lcr/navune/internal/lang"
)

// Parser adapts JavaScript (.js/.jsx/.mjs/.cjs) and TypeScript (.ts/.tsx).
type Parser struct {
	lang lang.Lang
}

// NewJS returns a parser for plain JavaScript/JSX.
func NewJS() *Parser { return &Parser{lang: lang.JavaScript} }

// NewTS returns a parser for TypeScript (and TSX).
func NewTS() *Parser { return &Parser{lang: lang.TypeScript} }

// Lang implements lang.Parser.
func (p *Parser) Lang() lang.Lang { return p.lang }

// Extensions implements lang.Parser.
func (p *Parser) Extensions() []string {
	if p.lang == lang.TypeScript {
		return []string{".ts", ".tsx"}
	}
	return []string{".js", ".jsx", ".mjs", ".cjs"}
}

func (p *Parser) grammarFor(path string) *sitter.Language {
	if p.lang == lang.TypeScript {
		if strings.HasSuffix(path, ".tsx") {
			return sitter.NewLanguage(tsts.LanguageTSX())
		}
		return sitter.NewLanguage(tsts.LanguageTypescript())
	}
	return sitter.NewLanguage(tsjs.Language())
}

// Parse parses a single JS/TS source file into the element model.
func (p *Parser) Parse(path string, src []byte) (*lang.FileResult, error) {
	par := sitter.NewParser()
	if err := par.SetLanguage(p.grammarFor(path)); err != nil {
		return nil, err
	}
	tree := par.Parse(src, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("parse %s: no syntax tree", path)
	}
	res := &lang.FileResult{
		Path:       path,
		Lang:       p.lang,
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

// generatedMarker mirrors the Go convention: a leading comment containing
// "Code generated" and "DO NOT EDIT".
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

// nameKinds are the leaf kinds that hold an identifier's text.
var nameKinds = map[string]bool{
	"identifier":                  true,
	"property_identifier":         true,
	"private_property_identifier": true,
	"type_identifier":             true,
	"statement_identifier":        true,
}

// functionKinds become Function units.
var functionKinds = map[string]bool{
	"function_declaration":           true,
	"generator_function_declaration": true,
	"method_definition":              true,
}

// closureKinds are anonymous function nodes whose decision points are not
// attributed to an enclosing named function (matching the Go adapter).
var closureKinds = map[string]bool{
	"arrow_function":      true,
	"function_expression": true,
	"generator_function":  true,
}

var typeKinds = map[string]bool{
	"class_declaration":          true,
	"abstract_class_declaration": true,
	"interface_declaration":      true,
	"type_alias_declaration":     true,
	"enum_declaration":           true,
}

var abstractKinds = map[string]bool{
	"interface_declaration":      true,
	"abstract_class_declaration": true,
}

// decisionKinds add +1 cyclomatic complexity each.
var decisionKinds = map[string]bool{
	"if_statement":       true,
	"for_statement":      true,
	"for_in_statement":   true,
	"for_of_statement":   true,
	"while_statement":    true,
	"do_statement":       true,
	"switch_case":        true,
	"switch_default":     true,
	"catch_clause":       true,
	"ternary_expression": true,
}

var commentKinds = map[string]bool{
	"comment":        true,
	"html_comment":   true,
	"hash_bang_line": true,
}

// stringLike leaves are emitted as one normalized STR token; tree-sitter puts
// string content in children (string_fragment/template chars).
var stringLike = map[string]bool{
	"string":          true,
	"template_string": true,
}

func isLogicalNode(kind string) bool {
	if strings.HasSuffix(kind, "_statement") {
		return true
	}
	switch kind {
	case "import_statement", "export_statement", "function_declaration",
		"generator_function_declaration", "method_definition",
		"class_declaration", "abstract_class_declaration",
		"interface_declaration", "type_alias_declaration", "enum_declaration",
		"lexical_declaration", "variable_declaration", "using_declaration",
		"jsx_element", "jsx_self_closing_element":
		return true
	}
	return false
}

// walker traverses the concrete syntax tree and builds the element model.
type walker struct {
	res      *lang.FileResult
	src      []byte
	lines    map[int]bool
	comments map[int]bool
	logical  int
	funcs    []*lang.Function // stable pointers; flattened into res at end
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

// walkNode traverses n with an optional current function (fn) and the name of
// the enclosing type (typeName, for method attribution).
func (w *walker) walkNode(n *sitter.Node, fn *lang.Function, typeName string) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}

	// literals become single normalized tokens
	if stringLike[kind] {
		w.emitToken(lang.Token{Kind: lang.TokString, Text: "STR", Line: int(n.StartPosition().Row) + 1})
		return
	}
	if kind == "number" || kind == "regex" {
		w.emitToken(lang.Token{Kind: lang.TokNumber, Text: "NUM", Line: int(n.StartPosition().Row) + 1})
		return
	}

	if typeKinds[kind] {
		name := w.nameOf(n)
		if name != "" {
			w.res.Types = append(w.res.Types, lang.TypeDecl{Name: name, Abstract: abstractKinds[kind]})
		}
		nextType := name
		if nextType == "" {
			nextType = typeName
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			w.walkNode(n.NamedChild(i), fn, nextType)
		}
		return
	}

	if functionKinds[kind] || closureKinds[kind] {
		w.addFunction(n, fn, typeName, functionKinds[kind])
		return
	}

	// complexity contribution
	if fn != nil {
		if decisionKinds[kind] {
			fn.Complexity++
		}
		if kind == "binary_expression" && hasChildToken(n, "&&", "||") {
			fn.Complexity++
		}
	}

	if kind == "import_statement" || kind == "export_statement" {
		if s := w.sourceOf(n); s != "" {
			w.res.Imports = append(w.res.Imports, s)
		}
	}
	if isLogicalNode(kind) {
		w.logical++
	}
	w.children(n, fn, typeName)
}

// addFunction emits a named or anonymous function as a first-class unit
// (ADR 0015), computes its extended measures, and recurses into its children.
func (w *walker) addFunction(n *sitter.Node, parent *lang.Function, typeName string, named bool) {
	start := int(n.StartPosition().Row) + 1
	name := w.nameOf(n)
	enclosing := ""
	if !named || name == "" {
		name = fmt.Sprintf("<anonymous@%d>", start)
	} else if n.Kind() == "method_definition" {
		enclosing = typeName
	}
	f := &lang.Function{
		Name:       name,
		Enclosing:  enclosing,
		Anonymous:  !named,
		Complexity: 1,
		StartLine:  start,
		EndLine:    int(n.EndPosition().Row) + 1,
		Params:     w.paramsOf(n),
	}
	self := ""
	if named && n.Kind() != "method_definition" {
		self = name
	} else if n.Kind() == "method_definition" {
		self = name
	}
	f.Cognitive, f.Nesting, f.BoolMax, f.Calls = w.analyzeBody(n, self)
	w.funcs = append(w.funcs, f)
	w.children(n, f, typeName)
}

func (w *walker) paramsOf(n *sitter.Node) int {
	p := n.ChildByFieldName("parameters")
	if p == nil {
		return 0
	}
	switch p.Kind() {
	case "identifier", "required_parameter", "optional_parameter":
		return 1
	}
	cnt := 0
	for i := uint(0); i < p.NamedChildCount(); i++ {
		if p.NamedChild(i).Kind() == "comment" {
			continue
		}
		cnt++
	}
	return cnt
}

// analyzeBody computes cognitive complexity, maximum control-structure nesting,
// and direct callees for a function node (ADR 0015).
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

// cogWalk computes the SonarSource cognitive model over a function body.
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
	if functionKinds[kind] || closureKinds[kind] {
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
	case "for_statement", "for_in_statement", "while_statement", "do_statement":
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
	case "ternary_expression":
		c.cognitive += 1 + c.depth
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c.nest(n.NamedChild(i))
		}
		return
	case "binary_expression":
		op := c.w.operatorText(n)
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
		c.cognitive++ // else-if: increment without nesting penalty
		c.nest(stmt.ChildByFieldName("consequence"))
		if a := stmt.ChildByFieldName("alternative"); a != nil {
			c.elseClause(a)
		}
		c.node(stmt.ChildByFieldName("condition"), "")
		return
	}
	c.cognitive++ // plain else
	c.nest(stmt)
}

func (w *walker) operatorText(n *sitter.Node) string {
	if op := n.ChildByFieldName("operator"); op != nil {
		return w.text(op)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if !ch.IsNamed() && ch.ChildCount() == 0 {
			switch ch.Kind() {
			case "&&", "||":
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
	case "identifier", "property_identifier":
		return w.text(fn)
	case "member_expression":
		if prop := fn.ChildByFieldName("property"); prop != nil {
			return w.text(prop)
		}
	}
	return ""
}

// boolOperands counts the leaf conditions in a logical expression tree.
func (c *cogWalk) boolOperands(n *sitter.Node) int {
	if n == nil || n.Kind() != "binary_expression" {
		return 1
	}
	op := c.w.operatorText(n)
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

// children recurses over all children: named nodes are walked structurally;
// unnamed leaves (keywords, operators, punctuation) become tokens so the
// token stream carries the same operator/statement detail as the Go adapter.
func (w *walker) children(n *sitter.Node, fn *lang.Function, typeName string) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.ChildCount() == 0 {
			w.emitLeaf(c, fn)
			continue
		}
		w.walkNode(c, fn, typeName)
	}
}

// emitLeaf emits a leaf token (identifier, keyword, punctuation, literal). The
// fn is unused: leaf content never changes complexity (string/number/comments
// are handled upstream).
func (w *walker) emitLeaf(n *sitter.Node, _ *lang.Function) {
	kind := n.Kind()
	if commentKinds[kind] {
		w.recordComment(n)
		return
	}
	if stringLike[kind] {
		return
	}
	line := int(n.StartPosition().Row) + 1
	if kind == "number" || kind == "regex" {
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

// nameOf finds the identifier under a function/type declaration node.
func (w *walker) nameOf(n *sitter.Node) string {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if nameKinds[c.Kind()] {
			return w.text(c)
		}
		if c.Kind() == "class_body" {
			continue
		}
	}
	return ""
}

// sourceOf returns the module specifier (unquoted) from an import/export
// statement: the trailing string child.
func (w *walker) sourceOf(n *sitter.Node) string {
	var src *sitter.Node
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c.Kind() == "string" {
			src = c
		}
	}
	if src == nil {
		return ""
	}
	return unquote(w.text(src))
}

func hasChildToken(n *sitter.Node, kinds ...string) bool {
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
