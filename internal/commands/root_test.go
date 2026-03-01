package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestVersionOutput(t *testing.T) {
	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2025-01-01T00:00:00Z"

	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("expected version in output, got: %s", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("expected commit in output, got: %s", out)
	}
	if !strings.Contains(out, "2025-01-01T00:00:00Z") {
		t.Errorf("expected date in output, got: %s", out)
	}
}

func TestScanHelpShowsFlags(t *testing.T) {
	out, err := executeCommand("scan", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFlags := []string{"--repo", "--org", "--format", "--idle-days", "--min-cost", "--token"}
	for _, flag := range expectedFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("scan --help missing flag %s, got: %s", flag, out)
		}
	}
}

func TestScanRequiresRepoOrOrg(t *testing.T) {
	_, err := executeCommand("scan")
	if err == nil {
		t.Fatal("expected error when neither --repo nor --org is set")
	}
	if !strings.Contains(err.Error(), "--repo or --org") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScanRejectsRepoAndOrg(t *testing.T) {
	_, err := executeCommand("scan", "--repo", "owner/repo", "--org", "myorg")
	if err == nil {
		t.Fatal("expected error when both --repo and --org are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScanRequiresToken(t *testing.T) {
	// Ensure GITHUB_TOKEN is not set for this test
	orig := os.Getenv("GITHUB_TOKEN")
	t.Setenv("GITHUB_TOKEN", "")
	t.Cleanup(func() { t.Setenv("GITHUB_TOKEN", orig) })

	_, err := executeCommand("scan", "--repo", "owner/repo")
	if err == nil {
		t.Fatal("expected error when no token is set")
	}
	if !strings.Contains(err.Error(), "token required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScanRejectsInvalidRepoFormat(t *testing.T) {
	_, err := executeCommand("scan", "--repo", "noslash", "--token", "fake")
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
	if !strings.Contains(err.Error(), "expected owner/repo") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err := executeCommand("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, ".cispectre.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !strings.Contains(string(data), "idle_days") {
		t.Error("config file missing idle_days field")
	}
}

func TestInitFailsIfExists(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_ = os.WriteFile(filepath.Join(dir, ".cispectre.yaml"), []byte("existing"), 0o644)

	_, err := executeCommand("init")
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}
