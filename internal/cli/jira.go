package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/jira"

	"github.com/spf13/cobra"
)

var jiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Jira issue management",
	Long:  `Commands for interacting with Jira issues via OAuth.`,
}

var jiraAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Jira (opens browser)",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := client.EnsureAuth(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✓ Authentication successful! Token saved.")
	},
}

var jiraViewCmd = &cobra.Command{
	Use:   "view <ISSUE-KEY>",
	Short: "View a single issue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		issue, err := client.GetIssue(ctx, args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(jira.FormatIssue(issue))
	},
}

var jiraSearchCmd = &cobra.Command{
	Use:   "search [JQL]",
	Short: "Search issues with JQL query",
	Long: `Search for issues using JQL (Jira Query Language).

Examples:
  bf jira search "project = TEL"
  bf jira search "assignee = currentUser() AND status != Done"
  bf jira search "updated >= -7d ORDER BY updated DESC"`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		jql := "updated >= -7d ORDER BY updated DESC"
		if len(args) > 0 {
			jql = strings.Join(args, " ")
		}

		limit, _ := cmd.Flags().GetInt("limit")

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		result, err := client.SearchIssues(ctx, jql, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(result.Issues) == 0 {
			fmt.Println("No issues found.")
			return
		}

		fmt.Printf("Found %d issues:\n\n", len(result.Issues))
		for _, issue := range result.Issues {
			assignee := "Unassigned"
			if issue.Fields.Assignee != nil {
				assignee = issue.Fields.Assignee.DisplayName
			}
			fmt.Printf("%-12s %-12s %-15s %s\n",
				issue.Key,
				issue.Fields.Status.Name,
				truncate(assignee, 15),
				truncate(issue.Fields.Summary, 50),
			)
		}
	},
}

var jiraMyCmd = &cobra.Command{
	Use:   "my",
	Short: "Show issues assigned to me",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		limit, _ := cmd.Flags().GetInt("limit")

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		result, err := client.GetMyIssues(ctx, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(result.Issues) == 0 {
			fmt.Println("No issues assigned to you.")
			return
		}

		fmt.Printf("Your issues (%d):\n\n", len(result.Issues))
		for _, issue := range result.Issues {
			fmt.Printf("%-12s %-12s %s\n",
				issue.Key,
				issue.Fields.Status.Name,
				truncate(issue.Fields.Summary, 60),
			)
		}
	},
}

var jiraLookupCmd = &cobra.Command{
	Use:   "lookup <KEY> [KEY...]",
	Short: "Look up multiple issue keys",
	Long:  `Quickly look up multiple issues by key. Useful for enriching summaries with Jira context.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		for _, key := range args {
			issue, err := client.GetIssue(ctx, key)
			if err != nil {
				fmt.Printf("%-12s ERROR: %v\n", key, err)
				continue
			}
			fmt.Printf("%-12s %s\n", issue.Key, issue.Fields.Summary)
		}
	},
}

func init() {
	jiraCmd.AddCommand(jiraAuthCmd)
	jiraCmd.AddCommand(jiraViewCmd)
	jiraCmd.AddCommand(jiraSearchCmd)
	jiraCmd.AddCommand(jiraMyCmd)
	jiraCmd.AddCommand(jiraLookupCmd)

	jiraSearchCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
