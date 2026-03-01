package analyzer

import "github.com/ppiankov/cispectre/internal/github"

// Severity indicates the urgency of a finding.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// FindingType identifies a specific waste pattern.
type FindingType string

const (
	WorkflowIdle             FindingType = "WORKFLOW_IDLE"
	WorkflowDuplicateTrigger FindingType = "WORKFLOW_DUPLICATE_TRIGGER"
	WorkflowNoCache          FindingType = "WORKFLOW_NO_CACHE"
	WorkflowHighBurn         FindingType = "WORKFLOW_HIGH_BURN"
	WorkflowNoConcurrency    FindingType = "WORKFLOW_NO_CONCURRENCY"
	RunnerOverprovisioned    FindingType = "RUNNER_OVERPROVISIONED"
	RunnerIdle               FindingType = "RUNNER_IDLE"
	ArtifactBloat            FindingType = "ARTIFACT_BLOAT"
)

// Finding represents a single waste detection result.
type Finding struct {
	Type                 FindingType `json:"type"`
	Severity             Severity    `json:"severity"`
	Resource             string      `json:"resource"`
	Message              string      `json:"message"`
	EstimatedMonthlyCost float64     `json:"estimated_monthly_cost"`
}

// Input holds all pre-fetched GitHub data for analysis.
type Input struct {
	Owner     string
	Repo      string
	Workflows []github.Workflow
	Contents  []github.WorkflowContent
	Runs      []github.Run
	Jobs      map[int64][]github.Job // runID → jobs
	Runners   []github.Runner
	Artifacts []github.Artifact
	IdleDays  int
	MinCost   float64
}
