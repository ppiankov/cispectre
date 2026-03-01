package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Artifact represents a stored workflow artifact.
type Artifact struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SizeBytes int64     `json:"size_in_bytes"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Expired   bool      `json:"expired"`
}

// ListArtifacts returns all artifacts for the given repo.
func (c *Client) ListArtifacts(ctx context.Context, owner, repo string) ([]Artifact, error) {
	url := c.baseURL + "/repos/" + owner + "/" + repo + "/actions/artifacts"
	var result []Artifact

	err := c.paginate(ctx, url, func(raw []byte) error {
		var page struct {
			Artifacts []Artifact `json:"artifacts"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding artifacts page: %w", err)
		}
		result = append(result, page.Artifacts...)
		return nil
	})
	return result, err
}
