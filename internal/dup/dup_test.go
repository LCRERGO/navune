package dup

import (
	"testing"

	"github.com/lcr/navune/internal/lang"
)

func tokens(texts ...string) []lang.Token {
	var out []lang.Token
	for i, t := range texts {
		out = append(out, lang.Token{Kind: lang.TokIdent, Text: t, Line: i + 1})
	}
	return out
}

func TestNoDuplication(t *testing.T) {
	e := &Engine{MinTokens: 3}
	res := e.Detect([][]lang.Token{
		tokens("a", "b", "c", "d", "e", "f", "g"),
		tokens("x", "y", "z"),
	})
	if res.DupTokens != 0 {
		t.Fatalf("want 0 dup tokens, got %d", res.DupTokens)
	}
}

func TestCrossFileDuplication(t *testing.T) {
	e := &Engine{MinTokens: 3}
	// tail of file A repeated verbatim in file B
	res := e.Detect([][]lang.Token{
		tokens("alpha", "a", "b", "c", "d", "e", "f"),
		tokens("beta", "a", "b", "c", "d", "e", "f"),
	})
	// duplicated block a..f = 6 tokens in each file
	if res.ByFile[0] != 6 || res.ByFile[1] != 6 {
		t.Fatalf("want 6 dup tokens per file, got %v", res.ByFile)
	}
	if res.DupTokens != 12 {
		t.Fatalf("want 12 total dup tokens, got %d", res.DupTokens)
	}
	if res.Blocks != 1 {
		t.Fatalf("want 1 block, got %d", res.Blocks)
	}
}

func TestMinTokensThreshold(t *testing.T) {
	e := &Engine{MinTokens: 5}
	res := e.Detect([][]lang.Token{
		tokens("a", "b", "c", "d"), // only 4 matching tokens < 5
		tokens("a", "b", "c", "d", "z"),
	})
	if res.DupTokens != 0 {
		t.Fatalf("want 0 dup tokens below threshold, got %d", res.DupTokens)
	}
}

func TestIntraFileDuplication(t *testing.T) {
	e := &Engine{MinTokens: 3}
	// same content appears twice within one file
	res := e.Detect([][]lang.Token{
		tokens("a", "b", "c", "d", "a", "b", "c", "d"),
	})
	if res.DupTokens != 8 {
		t.Fatalf("intra-file: want 8 dup tokens (both copies), got %d", res.DupTokens)
	}
}

func TestDefaultMinTokens(t *testing.T) {
	if New().MinTokens != DefaultMinTokens {
		t.Fatalf("want default %d", DefaultMinTokens)
	}
}
