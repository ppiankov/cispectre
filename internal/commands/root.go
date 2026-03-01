package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// Version info set from main via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// NewRootCmd builds and returns the CLI command tree.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "cispectre",
		Short:         "GitHub Actions waste auditor",
		Long:          "cispectre scans GitHub Actions workflows to find idle pipelines, duplicate triggers, missing caches, and cost overruns.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := rootCmd.PersistentFlags()
	flags.String("format", "text", "output format: text, json, spectrehub")
	flags.String("token", "", "GitHub API token (default: GITHUB_TOKEN env)")
	flags.String("repo", "", "target repository (owner/repo)")
	flags.String("org", "", "target organization")
	flags.Int("idle-days", 90, "days without runs to flag as idle")
	flags.Float64("min-cost", 0, "minimum monthly cost to report")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newScanCmd())
	rootCmd.AddCommand(newInitCmd())

	return rootCmd
}

// ResolveToken returns the token from the flag or GITHUB_TOKEN env.
func ResolveToken(cmd *cobra.Command) string {
	token, _ := cmd.Flags().GetString("token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return token
}

// Execute builds the command tree and runs it.
func Execute() error {
	return NewRootCmd().Execute()
}
