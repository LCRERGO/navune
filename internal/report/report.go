// Package report renders the analysis.Report (ADR 0008) as text, versioned
// JSON, or Mermaid graph export.
package report

import (
	"fmt"
	"strings"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/gate"
)

// Format enumerates output formats.
type Format string

const (
	Text    Format = "text"
	JSON    Format = "json"
	Mermaid Format = "mermaid"
)

// ParseFormat validates a --format flag value.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case Text, JSON, Mermaid:
		return Format(s), nil
	}
	return "", fmt.Errorf("unknown format %q (want text, json or mermaid)", s)
}

// Render emits the report in the requested format.
func Render(r *analysis.Report, f Format) (string, error) {
	switch f {
	case Text:
		return renderText(r), nil
	case JSON:
		return renderJSON(r)
	case Mermaid:
		return renderMermaid(r)
	}
	return "", fmt.Errorf("unknown format %q", f)
}

// exitWord maps an exit code to its human meaning.
func exitWord(code int) string {
	switch code {
	case gate.ExitPass:
		return "PASS"
	case gate.ExitBreach:
		return "BREACH"
	case gate.ExitUsage:
		return "USAGE ERROR"
	}
	return "ERROR"
}

func padTier(t string) string {
	if t == "" {
		t = "warn"
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

func displayWidth(s string) int { return len(s) }

func formatNum(v float64, prec int) string {
	if prec == 0 {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.*f", prec, v)
}
