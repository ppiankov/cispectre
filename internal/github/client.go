package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxBackoff     = 60 * time.Second
	defaultTimeout = 30 * time.Second
)

// Client is the GitHub REST API client.
type Client struct {
	http    *http.Client
	token   string
	baseURL string
}

// New returns a Client authenticated with token.
// baseURL is normally "https://api.github.com" but can be overridden for tests.
func New(token, baseURL string) *Client {
	return &Client{
		http:    &http.Client{Timeout: defaultTimeout},
		token:   token,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// get performs an authenticated GET, decodes JSON into dst, and returns the
// next-page URL from the Link header (empty if last page).
// On rate-limit 403 it backs off and retries.
func (c *Client) get(ctx context.Context, url string, dst any) (string, error) {
	backoff := time.Second

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.http.Do(req)
		if err != nil {
			return "", fmt.Errorf("executing request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			wait := backoffDuration(resp.Header.Get("X-RateLimit-Reset"), backoff)
			if err := backoffSleep(ctx, wait); err != nil {
				return "", err
			}
			backoff = nextBackoff(backoff)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("github api %s: %d %s", url, resp.StatusCode, string(body))
		}

		if err := json.Unmarshal(body, dst); err != nil {
			return "", fmt.Errorf("decoding response: %w", err)
		}

		return nextPage(resp.Header.Get("Link")), nil
	}
}

// paginate calls get repeatedly, passing each page's raw JSON to collect.
func (c *Client) paginate(ctx context.Context, url string, collect func([]byte) error) error {
	for url != "" {
		var raw json.RawMessage
		next, err := c.get(ctx, url, &raw)
		if err != nil {
			return err
		}
		if err := collect(raw); err != nil {
			return err
		}
		url = next
	}
	return nil
}

// nextPage parses an RFC 5988 Link header and returns the URL for rel="next".
func nextPage(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

// backoffDuration computes how long to wait from the X-RateLimit-Reset header.
// Falls back to fallback if the header is missing or unparseable.
func backoffDuration(resetHeader string, fallback time.Duration) time.Duration {
	if resetHeader == "" {
		return fallback
	}
	resetUnix, err := strconv.ParseInt(resetHeader, 10, 64)
	if err != nil {
		return fallback
	}
	wait := time.Until(time.Unix(resetUnix, 0))
	if wait <= 0 {
		return time.Second
	}
	return wait
}

// nextBackoff doubles the backoff, capped at maxBackoff.
func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// backoffSleep sleeps for d or until ctx is done.
func backoffSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
