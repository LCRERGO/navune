// Package lang defines the uniform element model every language adapter
// produces (ADR 0006: file is the unit of analysis) and the parser interface.
package lang

// Lang identifies an analyzed language.
type Lang string

const (
	Go         Lang = "go"
	TypeScript Lang = "ts"
	JavaScript Lang = "js"
	Python     Lang = "py"
)

// Class is how a file is treated for metrics and gates (ADR 0011).
type Class int

const (
	Prod Class = iota // production code: participates in all metrics and budgets
	Test              // test code: reported in a separate tests namespace, excluded from budgets/graph
)

func (c Class) String() string {
	if c == Test {
		return "test"
	}
	return "prod"
}

// TokenKind is the coarse lexical class of a token, used for duplication.
type TokenKind int

const (
	TokIdent     TokenKind = iota // identifier, kept verbatim
	TokKeyword                    // keyword / operator / punctuation, kept verbatim
	TokString                     // string literal, normalized
	TokChar                       // character literal, normalized
	TokNumber                     // numeric literal, normalized
	TokRawString                  // back-quoted raw string, normalized
)

// Token is one normalized source token. Comments and whitespace never reach
// this stream. String/char/number literals are normalized to a fixed spelling
// so that copy-paste with changed constants is still detected; identifiers and
// keywords are kept verbatim. This is the language-agnostic input to the
// duplication engine.
type Token struct {
	Kind TokenKind
	Text string
	Line int // 1-based physical line the token starts on
}

// Function is one function/method/literal of the second+third layer.
type Function struct {
	Name       string
	Enclosing  string // enclosing type name for methods, "" otherwise
	StartLine  int
	EndLine    int
	Complexity int // cyclomatic complexity (>= 1)
}

// TypeDecl is a named type at the type layer. Abstract = interface in Go.
type TypeDecl struct {
	Name     string
	Abstract bool
}

// FileResult is the per-file analysis output of a language adapter.
type FileResult struct {
	Path       string
	ImportPath string // import path of the file's own package ("" if unknown)
	Lang       Lang
	Class      Class
	Package    string // human display of the package clause name

	TotalLines   int // physical lines including blanks & comments
	PhysicalSLOC int // lines containing at least one code token
	LogicalLOC   int // statement/declaration count

	Functions []Function
	Types     []TypeDecl
	Imports   []string // raw import paths / module keys
	Deps      []string // resolved internal dependency target paths (scripts)

	Tokens []Token

	generated bool
}

// IsGenerated reports whether the file carries a "Code generated ... DO NOT
// EDIT." marker. Generated files are skipped entirely (ADR 0011).
func (f *FileResult) IsGenerated() bool { return f.generated }

// MarkGenerated is used by adapters during parsing.
func (f *FileResult) MarkGenerated() { f.generated = true }

// Parser turns a single source file into the uniform element model plus the
// normalized token stream.
type Parser interface {
	Parse(path string, src []byte) (*FileResult, error)
	Lang() Lang
	Extensions() []string
}
