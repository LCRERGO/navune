package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPackagedArtifactsCoverCLISurface guards against the hand-rolled CLI
// (main.go) drifting away from the committed bash completion and man page
// (ADR 0018). The token list below is the contract: when the CLI surface
// changes, update it, the completion, the man page, and the help text together.
func TestPackagedArtifactsCoverCLISurface(t *testing.T) {
	root := filepath.Join("..", "..")
	completion := readArtifact(t, filepath.Join(root, "completions", "navune.bash"))
	manpage := strings.ReplaceAll(
		readArtifact(t, filepath.Join(root, "man", "navune.1")), `\-`, "-")

	tokens := []string{
		// Commands.
		"analyze", "init", "version", "help",
		// Global and analyze flags.
		"--config", "--format", "--out", "--verbose",
		"-v", "-h", "--help", "-V", "--version",
		// --format values.
		"text", "json", "yaml", "mermaid",
	}

	for _, tok := range tokens {
		if !containsToken(completion, tok) {
			t.Errorf("completions/navune.bash does not mention %q", tok)
		}
		if !containsToken(manpage, tok) {
			t.Errorf("man/navune.1 does not mention %q", tok)
		}
	}
}

func readArtifact(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// wordChar matches characters that may be part of a flag or command token.
var wordChar = regexp.MustCompile(`[A-Za-z0-9_-]`)

// containsToken reports whether s mentions token as a whole word, so that "-v"
// does not spuriously match inside "--verbose".
func containsToken(s, token string) bool {
	for i := 0; i+len(token) <= len(s); {
		j := strings.Index(s[i:], token)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(token)
		beforeOK := start == 0 || !wordChar.MatchString(s[start-1:start])
		afterOK := end == len(s) || !wordChar.MatchString(s[end:end+1])
		if beforeOK && afterOK {
			return true
		}
		i = start + 1
	}
	return false
}
