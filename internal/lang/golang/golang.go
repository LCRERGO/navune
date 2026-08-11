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

	collectDecls(file, res, fset)

	// Physical SLOC: distinct lines that carry at least one code token.
	// We rescan without comments so pure-comment lines contribute nothing.
	res.PhysicalSLOC = scanPhysicalSLOC(tf, src)

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

func collectDecls(file *ast.File, res *lang.FileResult, fset *token.FileSet) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			f := goFunc(d, fset)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				if t, ok := recvType(d.Recv.List[0].Type); ok {
					f.Enclosing = t
				}
			}
			res.Functions = append(res.Functions, f)
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

func goFunc(d *ast.FuncDecl, fset *token.FileSet) lang.Function {
	f := lang.Function{
		Name:       d.Name.Name,
		Complexity: 1,
		StartLine:  fset.Position(d.Pos()).Line,
		EndLine:    fset.Position(d.End()).Line,
	}
	if d.Body != nil {
		f.Complexity += bodyComplexity(d.Body)
	}
	return f
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

// scanPhysicalSLOC counts the number of distinct source lines that contain at
// least one non-comment token.
func scanPhysicalSLOC(tf *token.File, src []byte) int {
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
		p := tf.Position(pos)
		lines[p.Line] = true
	}
	return len(lines)
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
