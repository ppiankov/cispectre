package github

import (
	"context"
	"encoding/json"
	"fmt"
)

// Repo represents a GitHub repository.
type Repo struct {
	Owner string `json:"-"`
	Name  string `json:"name"`
}

// repoJSON is the raw GitHub API shape for a repository.
type repoJSON struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// ListOrgRepos returns all repositories in an organization.
func (c *Client) ListOrgRepos(ctx context.Context, org string) ([]Repo, error) {
	url := c.baseURL + "/orgs/" + org + "/repos"
	var result []Repo

	err := c.paginate(ctx, url, func(raw []byte) error {
		var page []repoJSON
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding repos page: %w", err)
		}
		for _, r := range page {
			result = append(result, Repo{Owner: r.Owner.Login, Name: r.Name})
		}
		return nil
	})
	return result, err
}
