package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ppiankov/cispectre/internal/analyzer"
	"github.com/ppiankov/cispectre/internal/config"
	"github.com/ppiankov/cispectre/internal/github"
	"github.com/ppiankov/cispectre/internal/report"
	"github.com/spf13/cobra"
)

const (
	githubBaseURL   = "https://api.github.com"
	maxJobFetchRuns = 20
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan GitHub Actions for waste",
		Long:  "Scan a repository or organization for idle workflows, duplicate triggers, missing caches, and cost overruns.",
		RunE:  runScan,
	}
}

func runScan(cmd *cobra.Command, _ []string) error {
	cfg := config.Load()
	cfg.Apply(cmd)

	repoFlag, _ := cmd.Flags().GetString("repo")
	orgFlag, _ := cmd.Flags().GetString("org")

	if repoFlag == "" && orgFlag == "" {
		return errors.New("either --repo or --org is required")
	}
	if repoFlag != "" && orgFlag != "" {
		return errors.New("--repo and --org are mutually exclusive")
	}

	token := ResolveToken(cmd)
	if token == "" {
		return errors.New("GitHub token required: set --token or GITHUB_TOKEN env")
	}

	format, _ := cmd.Flags().GetString("format")
	idleDays, _ := cmd.Flags().GetInt("idle-days")
	minCost, _ := cmd.Flags().GetFloat64("min-cost")

	client := github.New(token, githubBaseURL)
	ctx := cmd.Context()

	var allFindings []analyzer.Finding
	var targetType, targetOwner, targetRepo string

	if repoFlag != "" {
		owner, repo, err := splitRepo(repoFlag)
		if err != nil {
			return err
		}
		targetType = "github-repo"
		targetOwner = owner
		targetRepo = repo

		findings, err := scanRepo(ctx, client, owner, repo, idleDays, minCost)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", repoFlag, err)
		}
		allFindings = findings
	} else {
		targetType = "github-org"
		targetOwner = orgFlag

		repos, err := client.ListOrgRepos(ctx, orgFlag)
		if err != nil {
			return fmt.Errorf("listing repos for org %s: %w", orgFlag, err)
		}

		for _, r := range repos {
			findings, err := scanRepo(ctx, client, r.Owner, r.Name, idleDays, minCost)
			if err != nil {
				return fmt.Errorf("scanning %s/%s: %w", r.Owner, r.Name, err)
			}
			allFindings = append(allFindings, findings...)
		}
	}

	rpt := report.Report{
		Owner:      targetOwner,
		Repo:       targetRepo,
		TargetType: targetType,
		Version:    Version,
		Findings:   allFindings,
	}

	return report.Write(cmd.OutOrStdout(), rpt, format)
}

func scanRepo(ctx context.Context, client *github.Client, owner, repo string, idleDays int, minCost float64) ([]analyzer.Finding, error) {
	workflows, err := client.ListWorkflows(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}

	var contents []github.WorkflowContent
	for _, wf := range workflows {
		wc, err := client.GetWorkflowContent(ctx, owner, repo, wf.Path)
		if err != nil {
			continue // non-fatal: workflow file may be deleted
		}
		contents = append(contents, wc)
	}

	since := time.Now().AddDate(0, 0, -idleDays)
	runs, err := client.ListRuns(ctx, owner, repo, since)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}

	jobs := make(map[int64][]github.Job)
	fetched := 0
	for _, r := range runs {
		if r.Status != "completed" || fetched >= maxJobFetchRuns {
			continue
		}
		runJobs, err := client.ListRunJobs(ctx, owner, repo, r.ID)
		if err != nil {
			continue // non-fatal
		}
		jobs[r.ID] = runJobs
		fetched++
	}

	runners, err := client.ListRunners(ctx, owner, repo)
	if err != nil {
		runners = nil // non-fatal: may not have permission
	}

	artifacts, err := client.ListArtifacts(ctx, owner, repo)
	if err != nil {
		artifacts = nil // non-fatal
	}

	input := analyzer.Input{
		Owner:     owner,
		Repo:      repo,
		Workflows: workflows,
		Contents:  contents,
		Runs:      runs,
		Jobs:      jobs,
		Runners:   runners,
		Artifacts: artifacts,
		IdleDays:  idleDays,
		MinCost:   minCost,
	}

	return analyzer.Analyze(input), nil
}

func splitRepo(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q: expected owner/repo", s)
	}
	return parts[0], parts[1], nil
}
