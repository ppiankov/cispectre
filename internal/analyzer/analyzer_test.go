package analyzer

import (
	"testing"
	"time"

	"github.com/ppiankov/cispectre/internal/github"
)

func findByType(findings []Finding, t FindingType) *Finding {
	for i := range findings {
		if findings[i].Type == t {
			return &findings[i]
		}
	}
	return nil
}

func TestWorkflowIdle(t *testing.T) {
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: ".github/workflows/ci.yml"}},
		Runs:      []github.Run{}, // no runs
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	f := findByType(findings, WorkflowIdle)
	if f == nil {
		t.Fatal("expected WORKFLOW_IDLE finding")
	}
	if f.Resource != "CI" {
		t.Errorf("resource = %q, want CI", f.Resource)
	}
}

func TestWorkflowNotIdle(t *testing.T) {
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: ".github/workflows/ci.yml"}},
		Runs: []github.Run{{
			ID:         1,
			WorkflowID: 1,
			Status:     "completed",
			CreatedAt:  time.Now().Add(-24 * time.Hour),
			UpdatedAt:  time.Now().Add(-23 * time.Hour),
		}},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowIdle); f != nil {
		t.Error("unexpected WORKFLOW_IDLE finding for active workflow")
	}
}

func TestDuplicateTrigger_mapForm(t *testing.T) {
	yamlContent := `name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowDuplicateTrigger); f == nil {
		t.Fatal("expected WORKFLOW_DUPLICATE_TRIGGER finding")
	}
}

func TestDuplicateTrigger_listForm(t *testing.T) {
	yamlContent := `name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowDuplicateTrigger); f == nil {
		t.Fatal("expected WORKFLOW_DUPLICATE_TRIGGER finding for list form")
	}
}

func TestNoDuplicateTrigger(t *testing.T) {
	yamlContent := `name: CI
on:
  push:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowDuplicateTrigger); f != nil {
		t.Error("unexpected WORKFLOW_DUPLICATE_TRIGGER finding for push-only workflow")
	}
}

func TestMissingCache(t *testing.T) {
	yamlContent := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test ./...
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowNoCache); f == nil {
		t.Fatal("expected WORKFLOW_NO_CACHE finding")
	}
}

func TestHasCache(t *testing.T) {
	yamlContent := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/cache@v3
        with:
          path: ~/.cache/go
          key: go-cache
      - run: go test ./...
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowNoCache); f != nil {
		t.Error("unexpected WORKFLOW_NO_CACHE finding when cache is present")
	}
}

func TestHighBurn(t *testing.T) {
	now := time.Now()
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Runs: []github.Run{
			{ID: 1, WorkflowID: 1, Status: "completed", DurationSecs: 600, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-23 * time.Hour)},
			{ID: 2, WorkflowID: 1, Status: "completed", DurationSecs: 600, CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-47 * time.Hour)},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
		MinCost:  0,
	}
	findings := Analyze(input)
	f := findByType(findings, WorkflowHighBurn)
	if f == nil {
		t.Fatal("expected WORKFLOW_HIGH_BURN finding")
	}
	if f.EstimatedMonthlyCost <= 0 {
		t.Errorf("expected positive cost, got %f", f.EstimatedMonthlyCost)
	}
}

func TestHighBurn_belowThreshold(t *testing.T) {
	now := time.Now()
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Runs: []github.Run{
			{ID: 1, WorkflowID: 1, Status: "completed", DurationSecs: 60, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-23 * time.Hour)},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
		MinCost:  999, // very high threshold
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowHighBurn); f != nil {
		t.Error("unexpected WORKFLOW_HIGH_BURN finding below threshold")
	}
}

func TestNoConcurrency(t *testing.T) {
	yamlContent := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowNoConcurrency); f == nil {
		t.Fatal("expected WORKFLOW_NO_CONCURRENCY finding")
	}
}

func TestHasConcurrency(t *testing.T) {
	yamlContent := `name: CI
on: push
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI", Path: "ci.yml"}},
		Contents:  []github.WorkflowContent{{Path: "ci.yml", Content: []byte(yamlContent)}},
		Jobs:      map[int64][]github.Job{},
		IdleDays:  90,
	}
	findings := Analyze(input)
	if f := findByType(findings, WorkflowNoConcurrency); f != nil {
		t.Error("unexpected WORKFLOW_NO_CONCURRENCY finding when concurrency is configured")
	}
}

func TestOverprovisionedRunner(t *testing.T) {
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI"}},
		Runs: []github.Run{
			{ID: 1, WorkflowID: 1, Status: "completed"},
			{ID: 2, WorkflowID: 1, Status: "completed"},
		},
		Jobs: map[int64][]github.Job{
			1: {{ID: 10, RunID: 1, Labels: []string{"ubuntu-latest"}, DurationSecs: 30, Conclusion: "success"}},
			2: {{ID: 11, RunID: 2, Labels: []string{"ubuntu-latest"}, DurationSecs: 20, Conclusion: "success"}},
		},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, RunnerOverprovisioned); f == nil {
		t.Fatal("expected RUNNER_OVERPROVISIONED finding for sub-minute ubuntu-latest jobs")
	}
}

func TestNotOverprovisioned(t *testing.T) {
	input := Input{
		Workflows: []github.Workflow{{ID: 1, Name: "CI"}},
		Runs: []github.Run{
			{ID: 1, WorkflowID: 1, Status: "completed"},
		},
		Jobs: map[int64][]github.Job{
			1: {{ID: 10, RunID: 1, Labels: []string{"ubuntu-latest"}, DurationSecs: 120, Conclusion: "success"}},
		},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, RunnerOverprovisioned); f != nil {
		t.Error("unexpected RUNNER_OVERPROVISIONED finding for long-running job")
	}
}

func TestIdleRunner(t *testing.T) {
	input := Input{
		Runners: []github.Runner{
			{ID: 1, Name: "build-01", OS: "Linux", Status: "offline", Labels: []string{"self-hosted"}},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, RunnerIdle); f == nil {
		t.Fatal("expected RUNNER_IDLE finding for offline runner")
	}
}

func TestOnlineRunner(t *testing.T) {
	input := Input{
		Runners: []github.Runner{
			{ID: 1, Name: "build-01", OS: "Linux", Status: "online", Labels: []string{"self-hosted"}},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, RunnerIdle); f != nil {
		t.Error("unexpected RUNNER_IDLE finding for online runner")
	}
}

func TestArtifactBloat_singleLarge(t *testing.T) {
	input := Input{
		Artifacts: []github.Artifact{
			{ID: 1, Name: "big-build", SizeBytes: 200 << 20, Expired: false}, // 200MB
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, ArtifactBloat); f == nil {
		t.Fatal("expected ARTIFACT_BLOAT finding for 200MB artifact")
	}
}

func TestArtifactBloat_totalExceedsThreshold(t *testing.T) {
	input := Input{
		Artifacts: []github.Artifact{
			{ID: 1, Name: "a", SizeBytes: 600 << 20, Expired: false},
			{ID: 2, Name: "b", SizeBytes: 600 << 20, Expired: false},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	// Should have 2 single-artifact findings + 1 total finding
	count := 0
	for _, f := range findings {
		if f.Type == ArtifactBloat {
			count++
		}
	}
	if count < 3 {
		t.Errorf("expected at least 3 ARTIFACT_BLOAT findings (2 single + 1 total), got %d", count)
	}
}

func TestArtifactBloat_expiredSkipped(t *testing.T) {
	input := Input{
		Artifacts: []github.Artifact{
			{ID: 1, Name: "old", SizeBytes: 200 << 20, Expired: true},
		},
		Jobs:     map[int64][]github.Job{},
		IdleDays: 90,
	}
	findings := Analyze(input)
	if f := findByType(findings, ArtifactBloat); f != nil {
		t.Error("unexpected ARTIFACT_BLOAT finding for expired artifact")
	}
}

func TestAnalyze_integration(t *testing.T) {
	now := time.Now()
	yamlContent := `name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test ./...
`
	input := Input{
		Owner: "myorg",
		Repo:  "myrepo",
		Workflows: []github.Workflow{
			{ID: 1, Name: "CI", Path: "ci.yml"},
			{ID: 2, Name: "Unused", Path: "unused.yml"},
		},
		Contents: []github.WorkflowContent{
			{Path: "ci.yml", Content: []byte(yamlContent)},
		},
		Runs: []github.Run{
			{ID: 10, WorkflowID: 1, Status: "completed", DurationSecs: 300,
				CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-23 * time.Hour)},
		},
		Jobs: map[int64][]github.Job{
			10: {{ID: 100, RunID: 10, Labels: []string{"ubuntu-latest"}, DurationSecs: 30}},
		},
		Runners: []github.Runner{
			{ID: 1, Name: "stale-runner", Status: "offline", Labels: []string{"self-hosted"}},
		},
		Artifacts: []github.Artifact{
			{ID: 1, Name: "big-artifact", SizeBytes: 200 << 20, Expired: false},
		},
		IdleDays: 90,
		MinCost:  0,
	}

	findings := Analyze(input)

	expectedTypes := map[FindingType]bool{
		WorkflowIdle:             true, // "Unused" has no runs
		WorkflowDuplicateTrigger: true, // CI has push+PR
		WorkflowNoCache:          true, // CI has no cache
		WorkflowNoConcurrency:    true, // CI has no concurrency
		RunnerOverprovisioned:    true, // ubuntu-latest avg 30s
		RunnerIdle:               true, // offline runner
		ArtifactBloat:            true, // 200MB artifact
	}

	found := make(map[FindingType]bool)
	for _, f := range findings {
		found[f.Type] = true
	}

	for expected := range expectedTypes {
		if !found[expected] {
			t.Errorf("missing expected finding type: %s", expected)
		}
	}
}

func TestRunnerRate(t *testing.T) {
	tests := []struct {
		labels []string
		want   float64
	}{
		{[]string{"ubuntu-latest"}, RateLinux},
		{[]string{"windows-latest"}, RateWindows},
		{[]string{"macos-latest"}, RateMacOS},
		{[]string{"self-hosted", "linux"}, RateLinux},
		{[]string{"self-hosted", "macOS"}, RateMacOS},
		{nil, RateLinux},
	}
	for _, tt := range tests {
		got := runnerRate(tt.labels)
		if got != tt.want {
			t.Errorf("runnerRate(%v) = %f, want %f", tt.labels, got, tt.want)
		}
	}
}
