package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Workflow represents a GitHub Actions workflow.
type Workflow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	State string `json:"state"`
}

// WorkflowContent holds the decoded YAML source of a workflow file.
type WorkflowContent struct {
	Path    string
	Content []byte
}

// ListWorkflows returns all workflows for the given owner/repo.
func (c *Client) ListWorkflows(ctx context.Context, owner, repo string) ([]Workflow, error) {
	url := c.baseURL + "/repos/" + owner + "/" + repo + "/actions/workflows"
	var result []Workflow

	err := c.paginate(ctx, url, func(raw []byte) error {
		var page struct {
			Workflows []Workflow `json:"workflows"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decoding workflows page: %w", err)
		}
		result = append(result, page.Workflows...)
		return nil
	})
	return result, err
}

// GetWorkflowContent fetches the YAML source for a workflow file.
func (c *Client) GetWorkflowContent(ctx context.Context, owner, repo, path string) (WorkflowContent, error) {
	url := c.baseURL + "/repos/" + owner + "/" + repo + "/contents/" + path

	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if _, err := c.get(ctx, url, &resp); err != nil {
		return WorkflowContent{}, err
	}

	if resp.Encoding != "base64" {
		return WorkflowContent{}, fmt.Errorf("unsupported encoding %q for %s", resp.Encoding, path)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return WorkflowContent{}, fmt.Errorf("decoding base64 content for %s: %w", path, err)
	}

	return WorkflowContent{Path: path, Content: decoded}, nil
}
