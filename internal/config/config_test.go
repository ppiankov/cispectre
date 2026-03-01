package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadFrom_file(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cispectre.yaml")
	content := `idle_days: 30
min_cost: 5.5
format: json
token: ghp_test123
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	if cfg.IdleDays != 30 {
		t.Errorf("idle_days = %d, want 30", cfg.IdleDays)
	}
	if cfg.MinCost != 5.5 {
		t.Errorf("min_cost = %f, want 5.5", cfg.MinCost)
	}
	if cfg.Format != "json" {
		t.Errorf("format = %q, want json", cfg.Format)
	}
	if cfg.Token != "ghp_test123" {
		t.Errorf("token = %q, want ghp_test123", cfg.Token)
	}
}

func TestLoadFrom_missingFile(t *testing.T) {
	cfg := LoadFrom("/nonexistent/path/.cispectre.yaml")

	if cfg.IdleDays != 0 {
		t.Errorf("idle_days = %d, want 0", cfg.IdleDays)
	}
	if cfg.Format != "" {
		t.Errorf("format = %q, want empty", cfg.Format)
	}
}

func TestLoad_cwdOverridesHome(t *testing.T) {
	// Write a "home" config
	homeDir := t.TempDir()
	homePath := filepath.Join(homeDir, ".cispectre.yaml")
	_ = os.WriteFile(homePath, []byte("idle_days: 60\nformat: text\n"), 0o644)

	// Write a "cwd" config that overrides idle_days but not format
	cwdDir := t.TempDir()
	cwdPath := filepath.Join(cwdDir, ".cispectre.yaml")
	_ = os.WriteFile(cwdPath, []byte("idle_days: 14\n"), 0o644)

	origDir, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Load from both files via LoadFrom to simulate the merge
	var cfg Config
	loadFile(homePath, &cfg)
	loadFile(cwdPath, &cfg)

	if cfg.IdleDays != 14 {
		t.Errorf("idle_days = %d, want 14 (cwd override)", cfg.IdleDays)
	}
	if cfg.Format != "text" {
		t.Errorf("format = %q, want text (from home)", cfg.Format)
	}
}

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "text", "")
	cmd.Flags().String("token", "", "")
	cmd.Flags().Int("idle-days", 90, "")
	cmd.Flags().Float64("min-cost", 0, "")
	return cmd
}

func TestApply_setsUnchangedFlags(t *testing.T) {
	cmd := newTestCmd()

	cfg := Config{
		IdleDays: 30,
		MinCost:  5.0,
		Format:   "json",
		Token:    "mytoken",
	}
	cfg.Apply(cmd)

	format, _ := cmd.Flags().GetString("format")
	if format != "json" {
		t.Errorf("format = %q, want json", format)
	}
	token, _ := cmd.Flags().GetString("token")
	if token != "mytoken" {
		t.Errorf("token = %q, want mytoken", token)
	}
	idleDays, _ := cmd.Flags().GetInt("idle-days")
	if idleDays != 30 {
		t.Errorf("idle-days = %d, want 30", idleDays)
	}
	minCost, _ := cmd.Flags().GetFloat64("min-cost")
	if minCost != 5.0 {
		t.Errorf("min-cost = %f, want 5.0", minCost)
	}
}

func TestApply_flagOverridesConfig(t *testing.T) {
	cmd := newTestCmd()
	// Simulate user passing --format=spectrehub on CLI
	_ = cmd.Flags().Set("format", "spectrehub")

	cfg := Config{
		Format: "json", // config wants json, but CLI wins
	}
	cfg.Apply(cmd)

	format, _ := cmd.Flags().GetString("format")
	if format != "spectrehub" {
		t.Errorf("format = %q, want spectrehub (CLI override)", format)
	}
}

func TestApply_zeroValuesNotApplied(t *testing.T) {
	cmd := newTestCmd()

	cfg := Config{} // all zero values
	cfg.Apply(cmd)

	// Defaults should remain
	format, _ := cmd.Flags().GetString("format")
	if format != "text" {
		t.Errorf("format = %q, want text (default)", format)
	}
	idleDays, _ := cmd.Flags().GetInt("idle-days")
	if idleDays != 90 {
		t.Errorf("idle-days = %d, want 90 (default)", idleDays)
	}
}
