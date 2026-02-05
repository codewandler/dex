package cli

import (
	"fmt"

	"github.com/codewandler/dex/internal/gh"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:     "gh",
	Aliases: []string{"github"},
	Short:   "GitHub operations via gh CLI",
	Long:    `Wrapper around the gh CLI for GitHub operations.`,
}

var ghCloneCmd = &cobra.Command{
	Use:   "clone <repo> [dest]",
	Short: "Clone a GitHub repository",
	Long: `Clone a GitHub repository using the gh CLI.

The repo can be specified as:
  - Full URL: https://github.com/owner/repo
  - Short form: owner/repo

If dest is not provided, clones to the repo name in the current directory.

Examples:
  dex gh clone owner/repo
  dex gh clone https://github.com/owner/repo
  dex gh clone owner/repo ./my-local-dir`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'gh auth login' first")
		}

		repoURL := args[0]
		dest := ""
		if len(args) > 1 {
			dest = args[1]
		}

		fmt.Printf("Cloning %s...\n", repoURL)
		if err := client.Clone(repoURL, dest); err != nil {
			return err
		}

		fmt.Println("Clone complete!")
		return nil
	},
}

var ghTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test GitHub CLI authentication",
	Long: `Test that the gh CLI is installed and authenticated.

Examples:
  dex gh test`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsInstalled() {
			return fmt.Errorf("gh CLI is not installed. Install from https://cli.github.com/")
		}

		status, err := client.GetAuthStatus()
		if err != nil {
			return fmt.Errorf("gh CLI is not authenticated. Run 'gh auth login' to authenticate")
		}

		fmt.Printf("Authenticated as @%s\n", status.Username)
		return nil
	},
}

func init() {
	ghCmd.AddCommand(ghCloneCmd)
	ghCmd.AddCommand(ghTestCmd)
	rootCmd.AddCommand(ghCmd)
}
