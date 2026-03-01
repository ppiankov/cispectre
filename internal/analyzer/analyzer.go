package analyzer

import (
	"fmt"
	"strings"

	"github.com/ppiankov/cispectre/internal/github"
	"gopkg.in/yaml.v3"
)

// workflowYAML is the subset of a GitHub Actions workflow YAML we parse.
type workflowYAML struct {
	On          any            `yaml:"on"`
	Concurrency any            `yaml:"concurrency"`
	Jobs        map[string]any `yaml:"jobs"`
}

// Analyze runs all checks against the input and returns findings.
func Analyze(input Input) []Finding {
	var findings []Finding

	contentByPath := make(map[string]github.WorkflowContent, len(input.Contents))
	for _, c := range input.Contents {
		contentByPath[c.Path] = c
	}

	// Group runs by workflow ID
	runsByWorkflow := make(map[int64][]github.Run)
	for _, r := range input.Runs {
		runsByWorkflow[r.WorkflowID] = append(runsByWorkflow[r.WorkflowID], r)
	}

	months := float64(input.IdleDays) / 30.0
	if months <= 0 {
		months = 3.0
	}

	for _, wf := range input.Workflows {
		wfRuns := runsByWorkflow[wf.ID]

		findings = append(findings, checkIdleWorkflow(wf, wfRuns, input.IdleDays)...)

		if content, ok := contentByPath[wf.Path]; ok {
			findings = append(findings, checkDuplicateTriggers(wf, content)...)
			findings = append(findings, checkMissingCache(wf, content)...)
			findings = append(findings, checkNoConcurrency(wf, content)...)
		}

		findings = append(findings, checkHighBurn(wf, wfRuns, input.Jobs, months, input.MinCost)...)
	}

	findings = append(findings, checkOverprovisionedRunners(input.Runs, input.Jobs)...)
	findings = append(findings, checkIdleRunners(input.Runners)...)
	findings = append(findings, checkArtifactBloat(input.Artifacts)...)

	return findings
}

func checkIdleWorkflow(wf github.Workflow, runs []github.Run, idleDays int) []Finding {
	if len(runs) > 0 {
		return nil
	}
	return []Finding{{
		Type:     WorkflowIdle,
		Severity: SeverityMedium,
		Resource: wf.Name,
		Message:  fmt.Sprintf("workflow %q has had no runs in the last %d days", wf.Name, idleDays),
	}}
}

func checkDuplicateTriggers(wf github.Workflow, content github.WorkflowContent) []Finding {
	var parsed workflowYAML
	if err := yaml.Unmarshal(content.Content, &parsed); err != nil {
		return nil
	}

	hasPush, hasPR := false, false

	switch on := parsed.On.(type) {
	case string:
		// on: push — single trigger, never duplicate
		return nil
	case []any:
		for _, v := range on {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if s == "push" {
				hasPush = true
			}
			if s == "pull_request" {
				hasPR = true
			}
		}
	case map[string]any:
		_, hasPush = on["push"]
		_, hasPR = on["pull_request"]
	}

	if hasPush && hasPR {
		return []Finding{{
			Type:     WorkflowDuplicateTrigger,
			Severity: SeverityMedium,
			Resource: wf.Name,
			Message:  fmt.Sprintf("workflow %q triggers on both push and pull_request, causing duplicate runs", wf.Name),
		}}
	}
	return nil
}

func checkMissingCache(wf github.Workflow, content github.WorkflowContent) []Finding {
	var parsed workflowYAML
	if err := yaml.Unmarshal(content.Content, &parsed); err != nil {
		return nil
	}

	if hasCache(parsed.Jobs) {
		return nil
	}

	return []Finding{{
		Type:     WorkflowNoCache,
		Severity: SeverityLow,
		Resource: wf.Name,
		Message:  fmt.Sprintf("workflow %q does not use actions/cache", wf.Name),
	}}
}

// hasCache recursively searches job definitions for actions/cache usage.
func hasCache(jobs map[string]any) bool {
	for _, job := range jobs {
		jobMap, ok := job.(map[string]any)
		if !ok {
			continue
		}
		steps, ok := jobMap["steps"].([]any)
		if !ok {
			continue
		}
		for _, step := range steps {
			stepMap, ok := step.(map[string]any)
			if !ok {
				continue
			}
			uses, ok := stepMap["uses"].(string)
			if ok && strings.Contains(uses, "actions/cache") {
				return true
			}
		}
	}
	return false
}

func checkHighBurn(wf github.Workflow, runs []github.Run, jobs map[int64][]github.Job, months, minCost float64) []Finding {
	cost := estimateMonthlyCost(runs, jobs, months)
	if cost <= minCost {
		return nil
	}
	return []Finding{{
		Type:                 WorkflowHighBurn,
		Severity:             SeverityHigh,
		Resource:             wf.Name,
		Message:              fmt.Sprintf("workflow %q estimated at $%.2f/month", wf.Name, cost),
		EstimatedMonthlyCost: cost,
	}}
}

func checkNoConcurrency(wf github.Workflow, content github.WorkflowContent) []Finding {
	var parsed workflowYAML
	if err := yaml.Unmarshal(content.Content, &parsed); err != nil {
		return nil
	}

	if parsed.Concurrency != nil {
		return nil
	}

	return []Finding{{
		Type:     WorkflowNoConcurrency,
		Severity: SeverityLow,
		Resource: wf.Name,
		Message:  fmt.Sprintf("workflow %q has no concurrency control configured", wf.Name),
	}}
}

func checkOverprovisionedRunners(runs []github.Run, jobs map[int64][]github.Job) []Finding {
	// Group jobs by runner label set, track durations
	type stats struct {
		totalSecs int64
		count     int
	}
	runnerStats := make(map[string]*stats)

	for _, r := range runs {
		for _, j := range jobs[r.ID] {
			key := strings.Join(j.Labels, ",")
			s, ok := runnerStats[key]
			if !ok {
				s = &stats{}
				runnerStats[key] = s
			}
			s.totalSecs += j.DurationSecs
			s.count++
		}
	}

	var findings []Finding
	for labels, s := range runnerStats {
		if s.count == 0 {
			continue
		}
		avgSecs := s.totalSecs / int64(s.count)
		if avgSecs < 60 && strings.Contains(labels, "ubuntu-latest") {
			findings = append(findings, Finding{
				Type:     RunnerOverprovisioned,
				Severity: SeverityLow,
				Resource: labels,
				Message:  fmt.Sprintf("jobs on %q average %ds — consider a smaller runner", labels, avgSecs),
			})
		}
	}
	return findings
}

func checkIdleRunners(runners []github.Runner) []Finding {
	var findings []Finding
	for _, r := range runners {
		if r.Status == "offline" {
			findings = append(findings, Finding{
				Type:     RunnerIdle,
				Severity: SeverityMedium,
				Resource: r.Name,
				Message:  fmt.Sprintf("self-hosted runner %q is offline", r.Name),
			})
		}
	}
	return findings
}

const (
	artifactTotalThreshold  = 1 << 30   // 1 GB
	artifactSingleThreshold = 100 << 20 // 100 MB
)

func checkArtifactBloat(artifacts []github.Artifact) []Finding {
	var findings []Finding
	var totalSize int64

	for _, a := range artifacts {
		if a.Expired {
			continue
		}
		totalSize += a.SizeBytes
		if a.SizeBytes > artifactSingleThreshold {
			findings = append(findings, Finding{
				Type:     ArtifactBloat,
				Severity: SeverityMedium,
				Resource: a.Name,
				Message:  fmt.Sprintf("artifact %q is %dMB", a.Name, a.SizeBytes/(1<<20)),
			})
		}
	}

	if totalSize > artifactTotalThreshold {
		findings = append(findings, Finding{
			Type:     ArtifactBloat,
			Severity: SeverityHigh,
			Resource: "all artifacts",
			Message:  fmt.Sprintf("total artifact storage is %dMB", totalSize/(1<<20)),
		})
	}

	return findings
}
