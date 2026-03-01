package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan GitHub Actions for waste",
		Long:  "Scan a repository or organization for idle workflows, duplicate triggers, missing caches, and cost overruns.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, _ := cmd.Flags().GetString("repo")
			org, _ := cmd.Flags().GetString("org")

			if repo == "" && org == "" {
				return errors.New("either --repo or --org is required")
			}
			if repo != "" && org != "" {
				return errors.New("--repo and --org are mutually exclusive")
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "scan not yet implemented")
			return nil
		},
	}
}
