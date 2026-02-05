package cli

import (
	"fmt"
	"strings"

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

var ghAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with GitHub",
	Long: `Authenticate with GitHub using the gh CLI.

This runs 'gh auth login' interactively to set up authentication.

Examples:
  dex gh auth`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsInstalled() {
			return fmt.Errorf("gh CLI is not installed. Install from https://cli.github.com/")
		}

		return client.Login()
	},
}

// Issue commands
var ghIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage GitHub issues",
	Long:  `Create, list, and view GitHub issues.`,
}

var ghIssueListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issues in a repository",
	Long: `List issues in a GitHub repository.

By default, lists open issues in the current repository.

Examples:
  dex gh issue list
  dex gh issue ls
  dex gh issue list --state closed
  dex gh issue list --label bug
  dex gh issue list --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		state, _ := cmd.Flags().GetString("state")
		label, _ := cmd.Flags().GetString("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		limit, _ := cmd.Flags().GetInt("limit")
		repo, _ := cmd.Flags().GetString("repo")

		issues, err := client.IssueList(gh.IssueListOptions{
			State:    state,
			Label:    label,
			Assignee: assignee,
			Limit:    limit,
			Repo:     repo,
		})
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			fmt.Println("No issues found")
			return nil
		}

		for _, issue := range issues {
			labels := ""
			if len(issue.Labels) > 0 {
				labels = fmt.Sprintf(" [%s]", issue.Labels[0])
				if len(issue.Labels) > 1 {
					labels = fmt.Sprintf(" [%s +%d]", issue.Labels[0], len(issue.Labels)-1)
				}
			}
			fmt.Printf("#%-4d %s%s\n", issue.Number, issue.Title, labels)
		}

		return nil
	},
}

var ghIssueViewCmd = &cobra.Command{
	Use:   "view <number>",
	Short: "View a specific issue",
	Long: `View details of a GitHub issue.

Examples:
  dex gh issue view 123
  dex gh issue view 123 --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")

		issue, err := client.IssueView(number, repo)
		if err != nil {
			return err
		}

		fmt.Printf("#%d %s\n", issue.Number, issue.Title)
		fmt.Printf("State: %s | Author: @%s | Created: %s\n", issue.State, issue.Author, issue.CreatedAt[:10])

		if len(issue.Labels) > 0 {
			fmt.Printf("Labels: %s\n", joinStrings(issue.Labels))
		}
		if len(issue.Assignees) > 0 {
			fmt.Printf("Assignees: %s\n", joinStrings(issue.Assignees))
		}

		fmt.Printf("URL: %s\n", issue.URL)

		if issue.Body != "" {
			fmt.Printf("\n%s\n", issue.Body)
		}

		return nil
	},
}

var ghIssueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	Long: `Create a new GitHub issue.

Examples:
  dex gh issue create --title "Bug report" --body "Description here"
  dex gh issue create -t "Feature request" -b "Details" --label enhancement
  dex gh issue create --title "Issue" --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		labels, _ := cmd.Flags().GetStringSlice("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		repo, _ := cmd.Flags().GetString("repo")

		if title == "" {
			return fmt.Errorf("--title is required")
		}

		issue, err := client.IssueCreate(gh.IssueCreateOptions{
			Title:    title,
			Body:     body,
			Labels:   labels,
			Assignee: assignee,
			Repo:     repo,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created issue #%d: %s\n", issue.Number, issue.URL)
		return nil
	},
}

var ghIssueCloseCmd = &cobra.Command{
	Use:   "close <number>",
	Short: "Close an issue",
	Long: `Close a GitHub issue.

Examples:
  dex gh issue close 123
  dex gh issue close 123 --comment "Fixed in PR #456"
  dex gh issue close 123 --reason "not planned"
  dex gh issue close 123 --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		comment, _ := cmd.Flags().GetString("comment")
		reason, _ := cmd.Flags().GetString("reason")
		repo, _ := cmd.Flags().GetString("repo")

		if err := client.IssueClose(gh.IssueCloseOptions{
			Number:  number,
			Comment: comment,
			Reason:  reason,
			Repo:    repo,
		}); err != nil {
			return err
		}

		fmt.Printf("Closed issue #%d\n", number)
		return nil
	},
}

func joinStrings(s []string) string {
	return strings.Join(s, ", ")
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
	// Issue list flags
	ghIssueListCmd.Flags().StringP("state", "s", "", "Filter by state: open, closed, all (default: open)")
	ghIssueListCmd.Flags().StringP("label", "l", "", "Filter by label")
	ghIssueListCmd.Flags().StringP("assignee", "a", "", "Filter by assignee")
	ghIssueListCmd.Flags().IntP("limit", "L", 30, "Maximum number of issues to fetch")
	ghIssueListCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue view flags
	ghIssueViewCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue create flags
	ghIssueCreateCmd.Flags().StringP("title", "t", "", "Issue title (required)")
	ghIssueCreateCmd.Flags().StringP("body", "b", "", "Issue body")
	ghIssueCreateCmd.Flags().StringSliceP("label", "l", nil, "Labels to add")
	ghIssueCreateCmd.Flags().StringP("assignee", "a", "", "Assignee")
	ghIssueCreateCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue close flags
	ghIssueCloseCmd.Flags().StringP("comment", "c", "", "Leave a closing comment")
	ghIssueCloseCmd.Flags().StringP("reason", "r", "", "Reason for closing: completed, not planned")
	ghIssueCloseCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Add issue subcommands
	ghIssueCmd.AddCommand(ghIssueCloseCmd)
	ghIssueCmd.AddCommand(ghIssueCreateCmd)
	ghIssueCmd.AddCommand(ghIssueListCmd)
	ghIssueCmd.AddCommand(ghIssueViewCmd)

	ghCmd.AddCommand(ghAuthCmd)
	ghCmd.AddCommand(ghCloneCmd)
	ghCmd.AddCommand(ghIssueCmd)
	ghCmd.AddCommand(ghTestCmd)
	rootCmd.AddCommand(ghCmd)
}
