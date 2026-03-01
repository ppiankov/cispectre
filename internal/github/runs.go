package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Run represents a single workflow run.
type Run struct {
	ID           int64     `json:"id"`
	WorkflowID   int64     `json:"workflow_id"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Event        string    `json:"event"`
	DurationSecs int64     `json:"-"`
}

// Job represents a single job within a workflow run.
type Job struct {
	ID           int64    `json:"-"`
	RunID        int64    `json:"-"`
	Name         string   `json:"-"`
	RunnerName   string   `json:"-"`
	Labels       []string `json:"-"`
	DurationSecs int64    `json:"-"`
	Conclusion   string   `json:"-"`
}

// jobJSON is the raw GitHub API shape for a job.
type jobJSON struct {
	ID          int64     `json:"id"`
	RunID       int64     `json:"run_id"`
	Name        string    `json:"name"`
	RunnerName  string    `json:"runner_name"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (j jobJSON) toJob() Job {
	var dur int64
	if !j.CompletedAt.IsZero() && !j.StartedAt.IsZero() {
		dur = int64(j.CompletedAt.Sub(j.StartedAt).Seconds())
	}
	labels := make([]string, len(j.Labels))
	for i, l := range j.Labels {
		labels[i] = l.Name
	}
	return Job{
		ID:           j.ID,
		RunID:        j.RunID,
		Name:         j.Name,
		RunnerName:   j.RunnerName,
		Labels:       labels,
		DurationSecs: dur,
		Conclusion:   j.Conclusion,
	}
}

// ListRuns returns all runs for the given repo. When since is non-zero,
// only runs created on or after that time are returned.
func (c *Client) ListRuns(ctx context.Context, owner, repo string, since time.Time) ([]Run, error) {
	u, err := url.Parse(c.baseURL + "/repos/" + owner + "/" + repo + "/actions/runs")
	if err != nil {
		return nil, fmt.Errorf("parsing runs url: %w", err)
	}
	if !since.IsZero() {
		q := u.Query()
		q.Set("created", ">="+since.UTC().Format(time.RFC3339))
		u.RawQuery = q.Encode()
	}

	var result []Run
	err = c.paginate(ctx, u.String(), func(raw []byte) error {
		var page struct {
			Runs []Run `json:"workflow_runs"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding runs page: %w", err)
		}
		for i := range page.Runs {
			r := &page.Runs[i]
			if r.Status == "completed" && !r.UpdatedAt.IsZero() && !r.CreatedAt.IsZero() {
				r.DurationSecs = int64(r.UpdatedAt.Sub(r.CreatedAt).Seconds())
			}
		}
		result = append(result, page.Runs...)
		return nil
	})
	return result, err
}

// ListRunJobs returns all jobs for a single workflow run.
func (c *Client) ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]Job, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/jobs", c.baseURL, owner, repo, runID)
	var result []Job

	err := c.paginate(ctx, url, func(raw []byte) error {
		var page struct {
			Jobs []jobJSON `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding jobs page: %w", err)
		}
		for _, j := range page.Jobs {
			result = append(result, j.toJob())
		}
		return nil
	})
	return result, err
}
