package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dex",
	Short: "The engineer's CLI",
	Long: `dex - the engineer's swiss army knife.

Unified access to your engineering tools:
  - GitLab (activity, repos)
  - Jira (issues, search)
  - Slack (messaging)
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
		fmt.Println(getVersion())
	},
}

func getVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown)"
	}
	return info.Main.Version
}

func init() {
	rootCmd.AddCommand(jiraCmd)
	rootCmd.AddCommand(gitlabCmd)
	rootCmd.AddCommand(slackCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(skillCmd)
}
