package report

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/config"
)

func fixtureReport(t *testing.T) *analysis.Report {
	t.Helper()
	r, err := analysis.Analyze(filepath.Join("..", "..", "test", "fixture"), config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRenderJSONValid(t *testing.T) {
	r := fixtureReport(t)
	out, err := Render(r, JSON)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("json must unmarshal: %v", err)
	}
	if decoded["schema_version"].(float64) != 1 {
		t.Error("schema_version must be 1")
	}
	if _, ok := decoded["summary"]; !ok {
		t.Error("summary key missing")
	}
	if _, ok := decoded["files"]; !ok {
		t.Error("files key missing")
	}
	if _, ok := decoded["cycles"]; !ok {
		t.Error("cycles key missing")
	}
	if _, ok := decoded["budgets"]; !ok {
		t.Error("budgets key missing")
	}
}

func TestRenderTextContainsHeadings(t *testing.T) {
	r := fixtureReport(t)
	out, err := Render(r, Text)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Files:", "Complexity:", "Quality gate", "Composite index", "Exit code:"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output should contain %q", want)
		}
	}
}

func TestRenderMermaidContainsGraph(t *testing.T) {
	r := fixtureReport(t)
	out, err := Render(r, Mermaid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "flowchart LR") {
		t.Errorf("mermaid should start with flowchart LR:\n%s", out)
	}
	if !strings.Contains(out, "subgraph cycle0") {
		t.Errorf("mermaid should contain cycle subgraph:\n%s", out)
	}
}

func TestParseFormat(t *testing.T) {
	for _, ok := range []string{"text", "json", "mermaid"} {
		if _, err := ParseFormat(ok); err != nil {
			t.Errorf("%s should parse", ok)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Error("xml must be rejected")
	}
}
