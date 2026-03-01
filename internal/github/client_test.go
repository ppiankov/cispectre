package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New("test-token", srv.URL)
}

// --- Core client tests ---

func TestGet_authAndAcceptHeaders(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q, want application/vnd.github+json", got)
		}
		_, _ = fmt.Fprint(w, `{}`)
	})

	var dst map[string]any
	_, err := client.get(context.Background(), client.baseURL+"/test", &dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet_rateLimitBackoff(t *testing.T) {
	calls := 0
	resetTime := time.Now().Add(2 * time.Second).Unix()
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `{"message":"rate limit"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	})

	start := time.Now()
	var dst map[string]any
	_, err := client.get(context.Background(), client.baseURL+"/test", &dst)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("elapsed = %v, expected backoff", elapsed)
	}
}

func TestGet_rateLimitBackoff_contextCancelled(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(10*time.Second).Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"message":"rate limit"}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var dst map[string]any
	_, err := client.get(ctx, client.baseURL+"/test", &dst)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestGet_403_notRateLimit(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "5")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"message":"forbidden"}`)
	})

	var dst map[string]any
	_, err := client.get(context.Background(), client.baseURL+"/test", &dst)
	if err == nil {
		t.Fatal("expected error for non-rate-limit 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestGet_nonOKStatus(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})

	var dst map[string]any
	_, err := client.get(context.Background(), client.baseURL+"/test", &dst)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

// --- Link header tests ---

func TestNextPage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"with next", `<https://api.github.com/repos?page=2>; rel="next", <https://api.github.com/repos?page=5>; rel="last"`, "https://api.github.com/repos?page=2"},
		{"last page", `<https://api.github.com/repos?page=1>; rel="first", <https://api.github.com/repos?page=1>; rel="prev"`, ""},
		{"empty", "", ""},
		{"malformed", `broken; rel="next"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextPage(tt.header)
			if got != tt.want {
				t.Errorf("nextPage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Pagination tests ---

func TestListWorkflows_pagination(t *testing.T) {
	page := 0
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		page++
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/workflows") && page == 1:
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/o/r/actions/workflows?page=2>; rel="next"`, "http://"+r.Host))
			_, _ = fmt.Fprint(w, `{"workflows":[{"id":1,"name":"CI","path":".github/workflows/ci.yml","state":"active"}]}`)
		default:
			_, _ = fmt.Fprint(w, `{"workflows":[{"id":2,"name":"Deploy","path":".github/workflows/deploy.yml","state":"active"}]}`)
		}
	})

	wfs, err := client.ListWorkflows(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wfs) != 2 {
		t.Fatalf("got %d workflows, want 2", len(wfs))
	}
	if wfs[0].Name != "CI" || wfs[1].Name != "Deploy" {
		t.Errorf("unexpected workflow names: %v", wfs)
	}
}

// --- Workflow content tests ---

func TestGetWorkflowContent_base64Decode(t *testing.T) {
	yamlContent := "name: CI\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(yamlContent))

	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"content":"%s\n","encoding":"base64"}`, encoded)
	})

	wc, err := client.GetWorkflowContent(context.Background(), "o", "r", ".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(wc.Content) != yamlContent {
		t.Errorf("content = %q, want %q", string(wc.Content), yamlContent)
	}
	if wc.Path != ".github/workflows/ci.yml" {
		t.Errorf("path = %q, want .github/workflows/ci.yml", wc.Path)
	}
}

func TestGetWorkflowContent_nonBase64Encoding(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"content":"raw stuff","encoding":"none"}`)
	})

	_, err := client.GetWorkflowContent(context.Background(), "o", "r", "path")
	if err == nil {
		t.Fatal("expected error for non-base64 encoding")
	}
	if !strings.Contains(err.Error(), "unsupported encoding") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Runs tests ---

func TestListRuns_sinceFilter(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		created := r.URL.Query().Get("created")
		if !strings.HasPrefix(created, ">=") {
			t.Errorf("expected created filter with >=, got %q", created)
		}
		_, _ = fmt.Fprint(w, `{"workflow_runs":[]}`)
	})

	since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := client.ListRuns(context.Background(), "o", "r", since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRuns_durationComputed(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"workflow_runs":[
			{"id":1,"workflow_id":10,"status":"completed","conclusion":"success","event":"push","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:05:00Z"},
			{"id":2,"workflow_id":10,"status":"in_progress","conclusion":"","event":"push","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:03:00Z"}
		]}`)
	})

	runs, err := client.ListRuns(context.Background(), "o", "r", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(runs))
	}
	if runs[0].DurationSecs != 300 {
		t.Errorf("completed run duration = %d, want 300", runs[0].DurationSecs)
	}
	if runs[1].DurationSecs != 0 {
		t.Errorf("in-progress run duration = %d, want 0", runs[1].DurationSecs)
	}
}

func TestListRuns_noSinceFilter(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		created := r.URL.Query().Get("created")
		if created != "" {
			t.Errorf("expected no created filter, got %q", created)
		}
		_, _ = fmt.Fprint(w, `{"workflow_runs":[]}`)
	})

	_, err := client.ListRuns(context.Background(), "o", "r", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Jobs tests ---

func TestListRunJobs_labelsFlattened(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"jobs":[{
			"id":100,"run_id":1,"name":"build","runner_name":"my-runner",
			"conclusion":"success",
			"started_at":"2025-01-01T00:00:00Z","completed_at":"2025-01-01T00:02:00Z",
			"labels":[{"name":"self-hosted"},{"name":"linux"},{"name":"x64"}]
		}]}`)
	})

	jobs, err := client.ListRunJobs(context.Background(), "o", "r", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	j := jobs[0]
	if j.Name != "build" {
		t.Errorf("name = %q, want build", j.Name)
	}
	if j.DurationSecs != 120 {
		t.Errorf("duration = %d, want 120", j.DurationSecs)
	}
	if len(j.Labels) != 3 || j.Labels[0] != "self-hosted" {
		t.Errorf("labels = %v, want [self-hosted linux x64]", j.Labels)
	}
}

// --- Runners tests ---

func TestListRunners(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"runners":[{
			"id":1,"name":"runner-1","os":"Linux","status":"online","busy":true,
			"labels":[{"name":"self-hosted"},{"name":"linux"}]
		}]}`)
	})

	runners, err := client.ListRunners(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("got %d runners, want 1", len(runners))
	}
	r := runners[0]
	if r.Name != "runner-1" || r.OS != "Linux" || r.Status != "online" || !r.Busy {
		t.Errorf("unexpected runner fields: %+v", r)
	}
	if len(r.Labels) != 2 || r.Labels[0] != "self-hosted" {
		t.Errorf("labels = %v, want [self-hosted linux]", r.Labels)
	}
}

// --- Artifacts tests ---

func TestListArtifacts(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"artifacts":[
			{"id":1,"name":"build-output","size_in_bytes":1048576,"expired":false,"created_at":"2025-01-01T00:00:00Z","expires_at":"2025-02-01T00:00:00Z"},
			{"id":2,"name":"old-logs","size_in_bytes":524288,"expired":true,"created_at":"2024-01-01T00:00:00Z","expires_at":"2024-02-01T00:00:00Z"}
		]}`)
	})

	artifacts, err := client.ListArtifacts(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(artifacts))
	}
	if artifacts[0].SizeBytes != 1048576 {
		t.Errorf("size = %d, want 1048576", artifacts[0].SizeBytes)
	}
	if artifacts[1].Expired != true {
		t.Error("expected second artifact to be expired")
	}
}

// --- Org repos tests ---

func TestListOrgRepos_pagination(t *testing.T) {
	page := 0
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		page++
		switch {
		case strings.HasSuffix(r.URL.Path, "/repos") && page == 1:
			w.Header().Set("Link", fmt.Sprintf(`<%s/orgs/myorg/repos?page=2>; rel="next"`, "http://"+r.Host))
			_, _ = fmt.Fprint(w, `[{"name":"repo-a","owner":{"login":"myorg"}}]`)
		default:
			_, _ = fmt.Fprint(w, `[{"name":"repo-b","owner":{"login":"myorg"}}]`)
		}
	})

	repos, err := client.ListOrgRepos(context.Background(), "myorg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
	if repos[0].Owner != "myorg" || repos[0].Name != "repo-a" {
		t.Errorf("unexpected first repo: %+v", repos[0])
	}
	if repos[1].Name != "repo-b" {
		t.Errorf("unexpected second repo: %+v", repos[1])
	}
}
