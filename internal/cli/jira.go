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

var jiraProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all accessible Jira projects",
	Long: `List all Jira projects you have access to.

Shows project keys (e.g., DEV, TEL) which are used as prefixes for issue keys.
Archived projects (names starting with "z[archive]") are hidden by default.

Examples:
  dex jira projects
  dex jira projects --keys
  dex jira projects --archived`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		keysOnly, _ := cmd.Flags().GetBool("keys")
		showArchived, _ := cmd.Flags().GetBool("archived")

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		projects, err := client.ListProjects(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return
		}

		// Filter out archived projects unless --archived flag is set
		var filtered []jira.Project
		for _, p := range projects {
			isArchived := strings.HasPrefix(strings.ToLower(p.Name), "z[archive")
			if showArchived || !isArchived {
				filtered = append(filtered, p)
			}
		}

		if len(filtered) == 0 {
			fmt.Println("No projects found.")
			return
		}

		// Get site URL for clickable links
		siteURL := client.GetSiteURL()

		if keysOnly {
			for _, p := range filtered {
				fmt.Println(p.Key)
			}
			return
		}

		fmt.Printf("%-10s %-40s %s\n", "KEY", "NAME", "TYPE")
		fmt.Println("────────────────────────────────────────────────────────────────")
		for _, p := range filtered {
			keyDisplay := fmt.Sprintf("%-10s", p.Key) // Pad first
			if siteURL != "" {
				// Wrap padded key in hyperlink with magenta color
				keyDisplay = fmt.Sprintf("\033]8;;%s/browse/%s\033\\\033[35m%s\033[0m\033]8;;\033\\", siteURL, p.Key, keyDisplay)
			}
			fmt.Printf("%s %-40s %s\n", keyDisplay, truncate(p.Name, 40), p.ProjectType)
		}
		fmt.Printf("\n%d projects\n", len(filtered))
	},
}

func init() {
	jiraCmd.AddCommand(jiraAuthCmd)
	jiraCmd.AddCommand(jiraViewCmd)
	jiraCmd.AddCommand(jiraSearchCmd)
	jiraCmd.AddCommand(jiraMyCmd)
	jiraCmd.AddCommand(jiraLookupCmd)
	jiraCmd.AddCommand(jiraProjectsCmd)

	jiraSearchCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraProjectsCmd.Flags().BoolP("keys", "k", false, "Output only project keys (one per line)")
	jiraProjectsCmd.Flags().BoolP("archived", "a", false, "Include archived projects")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
