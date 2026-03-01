package report

import (
	"encoding/json"
	"io"
	"time"

	"github.com/ppiankov/cispectre/internal/analyzer"
)

type spectreEnvelope struct {
	Schema    string           `json:"schema"`
	Tool      spectreTool      `json:"tool"`
	Timestamp string           `json:"timestamp"`
	Target    spectreTarget    `json:"target"`
	Findings  []spectreFinding `json:"findings"`
	Summary   spectreSummary   `json:"summary"`
}

type spectreTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type spectreTarget struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type spectreFinding struct {
	Type                 string  `json:"type"`
	Severity             string  `json:"severity"`
	Resource             string  `json:"resource"`
	Message              string  `json:"message"`
	EstimatedMonthlyCost float64 `json:"estimated_monthly_cost"`
}

type spectreSummary struct {
	Total                int            `json:"total"`
	BySeverity           map[string]int `json:"by_severity"`
	EstimatedMonthlyCost float64        `json:"estimated_monthly_cost"`
}

func buildSummary(findings []analyzer.Finding) spectreSummary {
	s := spectreSummary{
		Total:      len(findings),
		BySeverity: map[string]int{"high": 0, "medium": 0, "low": 0},
	}
	for _, f := range findings {
		s.BySeverity[string(f.Severity)]++
		s.EstimatedMonthlyCost += f.EstimatedMonthlyCost
	}
	return s
}

func writeSpectreHub(w io.Writer, r Report) error {
	targetName := r.Owner + "/" + r.Repo
	if r.TargetType == "github-org" {
		targetName = r.Owner
	}

	sFindings := make([]spectreFinding, len(r.Findings))
	for i, f := range r.Findings {
		sFindings[i] = spectreFinding{
			Type:                 string(f.Type),
			Severity:             string(f.Severity),
			Resource:             f.Resource,
			Message:              f.Message,
			EstimatedMonthlyCost: f.EstimatedMonthlyCost,
		}
	}

	env := spectreEnvelope{
		Schema:    "spectre/v1",
		Tool:      spectreTool{Name: "cispectre", Version: r.Version},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Target:    spectreTarget{Type: r.TargetType, Name: targetName},
		Findings:  sFindings,
		Summary:   buildSummary(r.Findings),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
