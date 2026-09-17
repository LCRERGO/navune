package report

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lcr/navune/internal/analysis"
)

// renderYAML serializes the report to the versioned YAML contract. It mirrors
// the JSON schema exactly (same struct field names via the yaml tags, ADR 0008
// revision); the two formats are interchangeable machine contracts.
func renderYAML(r *analysis.Report) (string, error) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(r); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	// Match the JSON renderer: no trailing newline (the CLI's Println adds one).
	return strings.TrimRight(b.String(), "\n"), nil
}
