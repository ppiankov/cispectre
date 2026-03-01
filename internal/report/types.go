package report

import (
	"fmt"
	"io"

	"github.com/ppiankov/cispectre/internal/analyzer"
)

// Report holds the data needed to produce any output format.
type Report struct {
	Owner      string
	Repo       string
	TargetType string // "github-repo" or "github-org"
	Version    string
	Findings   []analyzer.Finding
}

// Write renders the report in the given format to w.
func Write(w io.Writer, r Report, format string) error {
	switch format {
	case "text":
		return writeText(w, r)
	case "json":
		return writeJSON(w, r)
	case "spectrehub":
		return writeSpectreHub(w, r)
	default:
		return fmt.Errorf("unknown format %q (expected text, json, or spectrehub)", format)
	}
}
