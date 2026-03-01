package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ppiankov/cispectre/internal/analyzer"
)

var testFindings = []analyzer.Finding{
	{Type: analyzer.WorkflowHighBurn, Severity: analyzer.SeverityHigh, Resource: "CI", Message: `workflow "CI" estimated at $12.50/month`, EstimatedMonthlyCost: 12.50},
	{Type: analyzer.WorkflowIdle, Severity: analyzer.SeverityMedium, Resource: "Deploy", Message: `workflow "Deploy" has had no runs in the last 90 days`},
	{Type: analyzer.WorkflowNoCache, Severity: analyzer.SeverityLow, Resource: "Lint", Message: `workflow "Lint" does not use actions/cache`},
}

func testReport() Report {
	return Report{
		Owner:      "myorg",
		Repo:       "myrepo",
		TargetType: "github-repo",
		Version:    "0.1.0",
		Findings:   testFindings,
	}
}

func TestSpectreHub_schema(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if env["schema"] != "spectre/v1" {
		t.Errorf("schema = %v, want spectre/v1", env["schema"])
	}
}

func TestSpectreHub_tool(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Tool spectreTool `json:"tool"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Tool.Name != "cispectre" {
		t.Errorf("tool.name = %q, want cispectre", env.Tool.Name)
	}
	if env.Tool.Version != "0.1.0" {
		t.Errorf("tool.version = %q, want 0.1.0", env.Tool.Version)
	}
}

func TestSpectreHub_target(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Target spectreTarget `json:"target"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Target.Type != "github-repo" {
		t.Errorf("target.type = %q, want github-repo", env.Target.Type)
	}
	if env.Target.Name != "myorg/myrepo" {
		t.Errorf("target.name = %q, want myorg/myrepo", env.Target.Name)
	}
}

func TestSpectreHub_targetOrg(t *testing.T) {
	r := testReport()
	r.TargetType = "github-org"

	var buf bytes.Buffer
	if err := Write(&buf, r, "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Target spectreTarget `json:"target"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Target.Name != "myorg" {
		t.Errorf("target.name = %q, want myorg", env.Target.Name)
	}
}

func TestSpectreHub_summary(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Summary spectreSummary `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Summary.Total != 3 {
		t.Errorf("summary.total = %d, want 3", env.Summary.Total)
	}
	if env.Summary.BySeverity["high"] != 1 {
		t.Errorf("summary.by_severity.high = %d, want 1", env.Summary.BySeverity["high"])
	}
	if env.Summary.BySeverity["medium"] != 1 {
		t.Errorf("summary.by_severity.medium = %d, want 1", env.Summary.BySeverity["medium"])
	}
	if env.Summary.BySeverity["low"] != 1 {
		t.Errorf("summary.by_severity.low = %d, want 1", env.Summary.BySeverity["low"])
	}
	if env.Summary.EstimatedMonthlyCost != 12.50 {
		t.Errorf("summary.estimated_monthly_cost = %f, want 12.50", env.Summary.EstimatedMonthlyCost)
	}
}

func TestSpectreHub_findingsIncludeCost(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "spectrehub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Findings []spectreFinding `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(env.Findings) != 3 {
		t.Fatalf("got %d findings, want 3", len(env.Findings))
	}
	if env.Findings[0].EstimatedMonthlyCost != 12.50 {
		t.Errorf("finding[0].estimated_monthly_cost = %f, want 12.50", env.Findings[0].EstimatedMonthlyCost)
	}
}

func TestJSON_validOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var findings []analyzer.Finding
	if err := json.Unmarshal(buf.Bytes(), &findings); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
	if len(findings) != 3 {
		t.Errorf("got %d findings, want 3", len(findings))
	}
}

func TestText_containsSeverityAndResource(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[HIGH]") {
		t.Error("text output missing [HIGH]")
	}
	if !strings.Contains(out, "[MEDIUM]") {
		t.Error("text output missing [MEDIUM]")
	}
	if !strings.Contains(out, "[LOW]") {
		t.Error("text output missing [LOW]")
	}
	if !strings.Contains(out, "CI") {
		t.Error("text output missing resource CI")
	}
	if !strings.Contains(out, "Deploy") {
		t.Error("text output missing resource Deploy")
	}
}

func TestText_summaryLine(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testReport(), "text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "3 findings") {
		t.Error("text output missing summary count")
	}
	if !strings.Contains(out, "$12.50/month") {
		t.Error("text output missing cost in summary")
	}
}

func TestText_noFindings(t *testing.T) {
	r := testReport()
	r.Findings = nil

	var buf bytes.Buffer
	if err := Write(&buf, r, "text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No findings") {
		t.Error("expected 'No findings' message for empty report")
	}
}

func TestWrite_unknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, testReport(), "xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("unexpected error: %v", err)
	}
}
