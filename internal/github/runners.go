package github

import (
	"context"
	"encoding/json"
	"fmt"
)

// Runner represents a self-hosted runner registered to a repo.
type Runner struct {
	ID     int64    `json:"-"`
	Name   string   `json:"-"`
	OS     string   `json:"-"`
	Status string   `json:"-"`
	Busy   bool     `json:"-"`
	Labels []string `json:"-"`
}

// runnerJSON is the raw GitHub API shape for a runner.
type runnerJSON struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	OS     string `json:"os"`
	Status string `json:"status"`
	Busy   bool   `json:"busy"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (r runnerJSON) toRunner() Runner {
	labels := make([]string, len(r.Labels))
	for i, l := range r.Labels {
		labels[i] = l.Name
	}
	return Runner{
		ID:     r.ID,
		Name:   r.Name,
		OS:     r.OS,
		Status: r.Status,
		Busy:   r.Busy,
		Labels: labels,
	}
}

// ListRunners returns all self-hosted runners for the given repo.
func (c *Client) ListRunners(ctx context.Context, owner, repo string) ([]Runner, error) {
	url := c.baseURL + "/repos/" + owner + "/" + repo + "/actions/runners"
	var result []Runner

	err := c.paginate(ctx, url, func(raw []byte) error {
		var page struct {
			Runners []runnerJSON `json:"runners"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding runners page: %w", err)
		}
		for _, r := range page.Runners {
			result = append(result, r.toRunner())
		}
		return nil
	})
	return result, err
}
