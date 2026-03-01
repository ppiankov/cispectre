package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const configFileName = ".cispectre.yaml"

const sampleConfig = `# cispectre configuration
# See: https://github.com/ppiankov/cispectre

# Days without runs before a workflow is flagged as idle
# idle_days: 90

# Minimum estimated monthly cost to include in report (USD)
# min_cost: 0

# Output format: text, json, spectrehub
# format: text

# GitHub API token (or set GITHUB_TOKEN env)
# token: ""
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a sample .cispectre.yaml config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := os.Stat(configFileName); err == nil {
				return errors.New(configFileName + " already exists")
			}

			if err := os.WriteFile(configFileName, []byte(sampleConfig), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", configFileName, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", configFileName)
			return nil
		},
	}
}
