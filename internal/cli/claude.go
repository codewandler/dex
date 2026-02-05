package cli

import (
	"fmt"
	"os"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/statusline"
	"github.com/spf13/cobra"
)

var claudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Claude Code integrations",
	Long:  `Commands for integrating with Claude Code.`,
}

var claudeStatuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Output status line for Claude Code",
	Long: `Generate a status line for Claude Code's terminal UI.

This command fetches data from configured integrations (Kubernetes, GitLab,
GitHub, Jira, Slack) and outputs a formatted status line.

Configure the format and segments in ~/.dex/config.json under "status_line".

Example configuration:
  {
    "status_line": {
      "format": "{{if .K8s}}☸ {{.K8s}}{{end}}{{if .Jira}} | {{.Jira}}{{end}}",
      "segments": {
        "k8s": {
          "enabled": true,
          "format": "{{.Context}}/{{.Namespace}}",
          "cache_ttl": "30s"
        },
        "jira": {
          "enabled": true,
          "format": "{{.Open}} open",
          "cache_ttl": "2m"
        }
      }
    }
  }

To use with Claude Code, add to ~/.claude/settings.json:
  {
    "statusLine": {
      "type": "command",
      "command": "dex claude statusline"
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			// Don't fail on config errors, use defaults
			cfg = &config.Config{}
		}

		output, err := statusline.Run(cmd.Context(), cfg)
		if err != nil {
			// On error, output empty line (don't break Claude's UI)
			fmt.Fprintln(os.Stderr, "statusline error:", err)
			return nil
		}

		fmt.Println(output)
		return nil
	},
}

func init() {
	claudeCmd.AddCommand(claudeStatuslineCmd)
}
