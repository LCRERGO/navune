// Package treescript provides tree-sitter based adapters for JavaScript and
// TypeScript. It maps the grammar's concrete syntax tree onto Navune's uniform
// element model (file -> type/function, complexity, imports, tokens) using the
// official tree-sitter Go bindings. These require cgo (ADR 0007 revision).
package treescript

import (
	"fmt"
	"strings"

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

// fnCtx is an open function we are currently attributing complexity to, or nil
// inside a closure where decisions are not attributed anywhere.
type walker struct {
	res     *lang.FileResult
	src     []byte
	lines   map[int]bool
	logical int
	funcs   []*lang.Function // stable pointers; flatttened into res at end
}

func (w *walker) walk(root *sitter.Node) {
	w.lines = map[int]bool{}
	w.walkNode(root, nil, "")
	for _, f := range w.funcs {
		w.res.Functions = append(w.res.Functions, *f)
	}
}

// walkNode traverses n with an optional current function (fn) and the name of
// the enclosing type (typeName, for method attribution).
func (w *walker) walkNode(n *sitter.Node, fn *lang.Function, typeName string) {
	kind := n.Kind()
	if commentKinds[kind] {
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

	if functionKinds[kind] {
		name := w.nameOf(n)
		enclosing := ""
		if kind == "method_definition" {
			enclosing = typeName
		}
		f := &lang.Function{
			Name:       name,
			Enclosing:  enclosing,
			Complexity: 1,
			StartLine:  int(n.StartPosition().Row) + 1,
			EndLine:    int(n.EndPosition().Row) + 1,
		}
		w.funcs = append(w.funcs, f)
		w.children(n, f, typeName)
		return
	}

	if closureKinds[kind] {
		// do not attribute inner decisions to the enclosing function; still
		// collect tokens/statements/imports/classes inside the closure.
		w.children(n, nil, typeName)
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
	if commentKinds[kind] || stringLike[kind] {
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
