package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dex",
	Short: "The engineer's CLI",
	Long: `dex - the engineer's swiss army knife.

Unified access to your engineering tools:
  - GitLab (activity, repos)
  - Jira (issues, search)
  - Slack (coming soon)
  - Loki, Grafana (coming soon)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(jiraCmd)
	rootCmd.AddCommand(gitlabCmd)
}
