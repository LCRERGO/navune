// Package golang adapts the standard Go toolchain (go/parser) to Navune's
// uniform element model. Pure Go — always compiled in (ADR 0007).
package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"github.com/lcr/navune/internal/lang"
)

// Parser implements lang.Parser for the Go language.
type Parser struct{}

// New returns a Go parser adapter.
func New() *Parser { return &Parser{} }

// Lang implements lang.Parser.
func (*Parser) Lang() lang.Lang { return lang.Go }

// Extensions implements lang.Parser.
func (*Parser) Extensions() []string { return []string{".go"} }

// Parse parses a single Go source file.
func (*Parser) Parse(path string, src []byte) (*lang.FileResult, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	res := &lang.FileResult{
		Path:       path,
		Lang:       lang.Go,
		Class:      lang.Prod,
		TotalLines: countLines(src),
	}

	// generated-code marker: "// Code generated ... DO NOT EDIT." comment
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// Code generated") && strings.Contains(c.Text, "DO NOT EDIT") {
				res.MarkGenerated()
			}
		}
	}

	tf := fset.File(file.Pos())
	res.Package = file.Name.Name

	codeLines := scanCodeLines(tf, src)
	res.PhysicalSLOC = len(codeLines)
	res.CommentLines = countCommentLines(file.Comments, fset)

	collectDecls(file, res, fset, codeLines)

	// Token stream (comments/whitespace stripped, literals normalized).
	res.Tokens = tokenize(tf, src)

	return res, nil
}

func countLines(src []byte) int {
	n := bytes.Count(src, []byte{'\n'})
	if n == 0 && len(src) > 0 {
		return 1
	}
	if len(src) > 0 && src[len(src)-1] != '\n' {
		n++
	}
	return n
}

func collectDecls(file *ast.File, res *lang.FileResult, fset *token.FileSet, codeLines map[int]bool) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			f := goFunc(d, fset, codeLines)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				if t, ok := recvType(d.Recv.List[0].Type); ok {
					f.Enclosing = t
				}
			}
			res.Functions = append(res.Functions, f)
			if d.Body != nil {
				res.Functions = append(res.Functions, collectAnonFuncs(d.Body, f.Name, fset, codeLines)...)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					td := lang.TypeDecl{Name: ts.Name.Name}
					if iface, ok := ts.Type.(*ast.InterfaceType); ok && len(iface.Methods.List) > 0 {
						td.Abstract = true
					}
					res.Types = append(res.Types, td)
				}
			}
		}
	}

	// Logical LOC: statements + top-level declarations, counted via a walk.
	res.LogicalLOC = countStatements(file)
	// imports
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != "" {
			res.Imports = append(res.Imports, p)
		}
	}
}

func recvType(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, true
	case *ast.StarExpr:
		return recvType(t.X)
	case *ast.IndexExpr: // generic receiver
		return recvType(t.X)
	case *ast.IndexListExpr:
		return recvType(t.X)
	}
	return "", false
}

func goFunc(d *ast.FuncDecl, fset *token.FileSet, codeLines map[int]bool) lang.Function {
	f := lang.Function{
		Name:       d.Name.Name,
		Complexity: 1,
		StartLine:  fset.Position(d.Pos()).Line,
		EndLine:    fset.Position(d.End()).Line,
		Params:     countParams(d.Type.Params),
	}
	if d.Body != nil {
		f.Complexity += bodyComplexity(d.Body)
		f.Cognitive, f.Nesting, f.BoolMax, f.Calls = bodyCognitive(d.Body, d.Name.Name)
	}
	f.Length = countCodeLinesIn(codeLines, f.StartLine, f.EndLine)
	return f
}

// collectAnonFuncs emits every func literal under root as a first-class
// function (ADR 0015). Nested literals are emitted too.
func collectAnonFuncs(root ast.Node, enclosing string, fset *token.FileSet, codeLines map[int]bool) []lang.Function {
	var out []lang.Function
	ast.Inspect(root, func(n ast.Node) bool {
		fl, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		start := fset.Position(fl.Pos()).Line
		end := fset.Position(fl.End()).Line
		f := lang.Function{
			Name:       fmt.Sprintf("<anonymous@%d>", start),
			Enclosing:  enclosing,
			Anonymous:  true,
			Complexity: 1,
			StartLine:  start,
			EndLine:    end,
			Params:     countParams(fl.Type.Params),
		}
		if fl.Body != nil {
			f.Complexity += bodyComplexity(fl.Body)
			f.Cognitive, f.Nesting, f.BoolMax, f.Calls = bodyCognitive(fl.Body, "")
		}
		f.Length = countCodeLinesIn(codeLines, start, end)
		out = append(out, f)
		return true
	})
	return out
}

func countParams(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
		} else {
			n += len(f.Names)
		}
	}
	return n
}

func countCodeLinesIn(lines map[int]bool, start, end int) int {
	n := 0
	for l := start; l <= end; l++ {
		if lines[l] {
			n++
		}
	}
	return n
}

// bodyComplexity returns the cyclomatic contribution of a function body beyond
// the base of 1: if/for/range/case nodes and && || binary expressions.
// Nested func literals are skipped so a closure does not inflate its
// enclosing function's complexity (closures are not separately budgeted in v1).
func bodyComplexity(body *ast.BlockStmt) int {
	n := 0
	ast.Inspect(body, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.FuncLit:
			return false // do not descend into closures
		case *ast.IfStmt:
			n++
		case *ast.ForStmt:
			n++
		case *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			n++
		case *ast.CommClause:
			n++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}

// countStatements counts every statement node plus every top-level declaration
// node in the file: Navune's definition of logical SLOC.
func countStatements(f *ast.File) int {
	n := 0
	ast.Inspect(f, func(node ast.Node) bool {
		switch node.(type) {
		case ast.Stmt, ast.Decl:
			n++
		}
		return true
	})
	return n
}

// cog accumulates cognitive complexity and nesting while walking statements.
type cog struct {
	nesting int
	maxNest int
	score   int
}

func (c *cog) enter() {
	c.nesting++
	if c.nesting > c.maxNest {
		c.maxNest = c.nesting
	}
}

func (c *cog) exit() { c.nesting-- }

func (c *cog) block(b *ast.BlockStmt) {
	if b == nil {
		return
	}
	for _, s := range b.List {
		c.stmt(s)
	}
}

// bodyCognitive computes the SonarSource cognitive complexity of a function
// body (ADR 0015), its maximum control-structure nesting, and its direct
// callees. Nested func literals are separate function units and are skipped.
func bodyCognitive(body *ast.BlockStmt, self string) (score, maxNesting, boolMax int, calls []string) {
	c := &cog{}
	c.block(body)
	calls, recursion, logical, bm := scanCallsAndLogical(body, self)
	c.score += logical + recursion
	return c.score, c.maxNest, bm, calls
}

func (c *cog) stmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.IfStmt:
		c.score += 1 + c.nesting
		c.enter()
		c.block(n.Body)
		c.exit()
		c.elsePart(n.Else)
	case *ast.ForStmt:
		c.score += 1 + c.nesting
		c.enter()
		c.block(n.Body)
		c.exit()
	case *ast.RangeStmt:
		c.score += 1 + c.nesting
		c.enter()
		c.block(n.Body)
		c.exit()
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		c.score += 1 + c.nesting
		c.enter()
		switch x := n.(type) {
		case *ast.SwitchStmt:
			c.block(x.Body)
		case *ast.TypeSwitchStmt:
			c.block(x.Body)
		case *ast.SelectStmt:
			c.block(x.Body)
		}
		c.exit()
	case *ast.BlockStmt:
		c.block(n)
	case *ast.LabeledStmt:
		c.stmt(n.Stmt)
	case *ast.CaseClause:
		for _, s := range n.Body {
			c.stmt(s)
		}
	case *ast.CommClause:
		for _, s := range n.Body {
			c.stmt(s)
		}
	case *ast.BranchStmt:
		if (n.Tok == token.BREAK || n.Tok == token.CONTINUE) && n.Label != nil {
			c.score++
		}
	}
}

// elsePart handles an if's else branch: an else-if chain adds increments with
// no nesting penalty, and a plain else adds one increment.
func (c *cog) elsePart(s ast.Stmt) {
	switch e := s.(type) {
	case nil:
		return
	case *ast.IfStmt:
		c.score++
		c.enter()
		c.block(e.Body)
		c.exit()
		c.elsePart(e.Else)
	case *ast.BlockStmt:
		c.score++
		c.enter()
		c.block(e)
		c.exit()
	default:
		c.stmt(s)
	}
}

// scanCallsAndLogical counts direct callees, direct self-calls (recursion),
// and sequences of like logical operators (SonarSource model).
func scanCallsAndLogical(body *ast.BlockStmt, self string) (calls []string, recursion, logical, boolMax int) {
	var stack []token.Token
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		inherited := token.ILLEGAL
		if len(stack) > 0 {
			inherited = stack[len(stack)-1]
		}
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				if x.Op != inherited {
					logical++
					if c := logicalOperands(x); c > boolMax {
						boolMax = c
					}
				}
				stack = append(stack, x.Op)
				return true
			}
			stack = append(stack, token.ILLEGAL)
		case *ast.CallExpr:
			if name := calleeName(x.Fun); name != "" {
				calls = append(calls, name)
				if self != "" && name == self {
					recursion++
				}
			}
			stack = append(stack, token.ILLEGAL)
		default:
			stack = append(stack, token.ILLEGAL)
		}
		return true
	})
	return calls, recursion, logical, boolMax
}

// logicalOperands counts the leaf conditions in a logical expression tree.
func logicalOperands(e ast.Expr) int {
	if be, ok := e.(*ast.BinaryExpr); ok && (be.Op == token.LAND || be.Op == token.LOR) {
		return logicalOperands(be.X) + logicalOperands(be.Y)
	}
	return 1
}

func calleeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.IndexExpr:
		return calleeName(x.X)
	case *ast.IndexListExpr:
		return calleeName(x.X)
	}
	return ""
}

// scanCodeLines returns the set of lines that contain at least one non-comment
// token.
func scanCodeLines(tf *token.File, src []byte) map[int]bool {
	var s scanner.Scanner
	s.Init(tf, src, nil, scanner.ScanComments)
	lines := map[int]bool{}
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			continue
		}
		lines[tf.Position(pos).Line] = true
	}
	return lines
}

// countCommentLines counts significant comment lines: lines whose comment text
// contains at least one letter or digit. Empty and decorative comment lines are
// excluded; commented-out code counts (ADR 0015).
func countCommentLines(groups []*ast.CommentGroup, fset *token.FileSet) int {
	seen := map[int]bool{}
	for _, cg := range groups {
		for _, c := range cg.List {
			start := fset.Position(c.Slash).Line
			lines := strings.Split(c.Text, "\n")
			for i, line := range lines {
				if isSignificantComment(line) {
					seen[start+i] = true
				}
			}
		}
	}
	return len(seen)
}

func isSignificantComment(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// tokenize produces Navune's normalized token stream for the duplication
// engine: comments and whitespace removed, literals normalized.
func tokenize(tf *token.File, src []byte) []lang.Token {
	var s scanner.Scanner
	s.Init(tf, src, nil, scanner.ScanComments)
	toks := make([]lang.Token, 0, 256)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			continue
		}
		text := ""
		switch {
		case tok == token.IDENT:
			text = lit
		case tok.IsKeyword():
			text = lit
		case tok == token.STRING:
			text = "STR"
		case tok == token.CHAR:
			text = "CHAR"
		case tok == token.INT || tok == token.FLOAT || tok == token.IMAG:
			text = "NUM"
		default:
			text = tok.String()
		}
		k := lang.TokKeyword
		if tok == token.IDENT {
			k = lang.TokIdent
		} else if tok == token.STRING {
			k = lang.TokString
		} else if tok == token.CHAR {
			k = lang.TokChar
		} else if tok == token.INT || tok == token.FLOAT || tok == token.IMAG {
			k = lang.TokNumber
		}
		toks = append(toks, lang.Token{Kind: k, Text: text, Line: tf.Position(pos).Line})
	}
	return toks
}
