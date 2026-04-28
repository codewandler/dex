package cli

import (
	"fmt"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/render"
	"github.com/codewandler/dex/internal/slack"
	"github.com/spf13/cobra"
)

// ── file subcommand group ────────────────────────────────────────────────────

var slackFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Manage Slack files (list, info, download, delete)",
}

var slackFileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List files visible to the current token identity",
	Long: `List files uploaded to Slack, optionally filtered by channel.

Uses the user token if available (preferred); falls back to bot.

Examples:
  dex slack file list
  dex slack file list --channel dev-team
  dex slack file list --count 50
  dex slack file list -o compact
  dex slack file list -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		channelArg, _ := cmd.Flags().GetString("channel")
		count, _ := cmd.Flags().GetInt("count")
		compact, _ := cmd.Flags().GetBool("compact")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if err := cfg.RequireSlack(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			return fmt.Errorf("failed to create Slack client: %w", err)
		}

		channelID := ""
		if channelArg != "" {
			channelID = slack.ResolveChannel(channelArg)
		}

		files, err := client.ListFiles(channelID, count)
		if err != nil {
			return err
		}

		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(&slack.FileListResult{Files: files}, mode)
		return nil
	},
}

var slackFileInfoCmd = &cobra.Command{
	Use:   "info <file-id>",
	Short: "Show metadata for a file",
	Long: `Show detailed metadata for a Slack file by its ID.

Examples:
  dex slack file info F0AMV0SKKED
  dex slack file info F0AMV0SKKED -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if err := cfg.RequireSlack(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			return fmt.Errorf("failed to create Slack client: %w", err)
		}

		fi, err := client.GetFileInfo(fileID)
		if err != nil {
			return err
		}
		Render(&slack.FileInfoResult{File: *fi})
		return nil
	},
}

var slackFileDownloadCmd = &cobra.Command{
	Use:   "download <file-id>",
	Short: "Download a file to disk",
	Long: `Download a Slack file by its ID to a local path.

If --output is a directory, the file's original name is used inside it.
If --output is omitted, the file is saved to the current directory.

Examples:
  dex slack file download F0AMV0SKKED
  dex slack file download F0AMV0SKKED --output ~/Downloads/
  dex slack file download F0AMV0SKKED --output report.pdf`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]
		outPath, _ := cmd.Flags().GetString("output")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if err := cfg.RequireSlack(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			return fmt.Errorf("failed to create Slack client: %w", err)
		}

		dest, err := client.DownloadFile(fileID, outPath)
		if err != nil {
			return err
		}
		fmt.Printf("Downloaded to %s\n", dest)
		return nil
	},
}

var slackFileDeleteCmd = &cobra.Command{
	Use:   "delete <file-id>",
	Short: "Delete a file",
	Long: `Delete a Slack file by its ID.

Use --as to specify the token identity that owns the file.
Bot-uploaded files require --as bot (default).
User-uploaded files require --as user.

Examples:
  dex slack file delete F0AMV0SKKED
  dex slack file delete F0AMKLSE8E8 --as user`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]
		deleteAs, _ := cmd.Flags().GetString("as")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if err := cfg.RequireSlack(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		client, err := slackClientFor(cfg, deleteAs)
		if err != nil {
			return err
		}

		if err := client.DeleteFile(fileID); err != nil {
			return err
		}
		fmt.Printf("File %s deleted.\n", fileID)
		return nil
	},
}

// slackDownloadCmd is a top-level shortcut for `dex slack file download`.
// It accepts `dex slack download <file-id> [output-path]` for convenience,
// especially useful when file IDs come from thread/search output.
var slackDownloadCmd = &cobra.Command{
	Use:   "download <file-id> [output-path]",
	Short: "Download a Slack file (shortcut for 'slack file download')",
	Long: `Download a Slack file by its ID to a local path.

This is a convenience shortcut for 'dex slack file download'.
File IDs are shown in thread and search output when messages have attachments.

If output-path is a directory, the file's original name is used inside it.
If output-path is omitted, the file is saved to the current directory.

Examples:
  dex slack download F0123456789
  dex slack download F0123456789 ./screenshots/
  dex slack download F0123456789 report.pdf`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]
		outPath := ""
		if len(args) > 1 {
			outPath = args[1]
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if err := cfg.RequireSlack(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			return fmt.Errorf("failed to create Slack client: %w", err)
		}

		dest, err := client.DownloadFile(fileID, outPath)
		if err != nil {
			return err
		}
		fmt.Printf("Downloaded to %s\n", dest)
		return nil
	},
}

func initSlackFileFlags() {
	slackFileListCmd.Flags().StringP("channel", "C", "", "Filter by channel name or ID")
	slackFileListCmd.Flags().IntP("count", "n", 20, "Number of files to return")
	slackFileListCmd.Flags().Bool("compact", false, "Compact table view (one line per file)")
	slackFileDownloadCmd.Flags().String("output", "", "Output path (file or directory)")
	slackFileDeleteCmd.Flags().String("as", "bot", "Act as 'bot' (default) or 'user'")
}
