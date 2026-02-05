package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string (called from main)
func SetVersion(v string) {
	version = v
}

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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(jiraCmd)
	rootCmd.AddCommand(gitlabCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(skillCmd)
}
