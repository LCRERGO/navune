package report

import (
	"encoding/json"

	"github.com/lcr/navune/internal/analysis"
)

// renderJSON serializes the report to the versioned JSON contract. The Go
// structs in analysis.Report define the schema; field json tags are part of
// the stable contract (ADR 0008) and must not be renamed casually.
func renderJSON(r *analysis.Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
