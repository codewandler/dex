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
		status, _ := cmd.Flags().GetString("status")

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Build JQL query
		jql := "assignee = currentUser()"
		if status != "" {
			jql += fmt.Sprintf(" AND status = '%s'", status)
		} else {
			jql += " AND status != Done"
		}
		jql += " ORDER BY updated DESC"

		result, err := client.SearchIssues(ctx, jql, limit)
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

var jiraDeleteCmd = &cobra.Command{
	Use:   "delete <ISSUE-KEY> [ISSUE-KEY...]",
	Short: "Delete one or more Jira issues",
	Long: `Delete Jira issues by key. Supports deleting multiple issues at once.

Examples:
  dex jira delete DEV-123
  dex jira delete DEV-400 DEV-401 DEV-402`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		for _, key := range args {
			if err := client.DeleteIssue(ctx, key, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", key, err)
				continue
			}
			fmt.Printf("Deleted %s\n", key)
		}
	},
}

var jiraLinkCmd = &cobra.Command{
	Use:   "link <ISSUE-KEY> <ISSUE-KEY> [ISSUE-KEY...]",
	Short: "Link issues together",
	Long: `Create links between Jira issues.

The first issue is the source, subsequent issues are linked to it.
Default link type is "Relates" (symmetric relationship).

Common link types:
  Relates     - Generic relationship (symmetric)
  Blocks      - First issue blocks the others
  Cloners     - First issue clones the others
  Duplicate   - First issue duplicates the others

Use --list-types to see all available link types in your Jira instance.

Examples:
  dex jira link DEV-123 DEV-456                    # Link two issues (Relates)
  dex jira link DEV-123 DEV-456 DEV-789            # Link multiple issues to DEV-123
  dex jira link DEV-123 DEV-456 -t Blocks          # DEV-123 blocks DEV-456
  dex jira link DEV-123 DEV-456 -t Duplicate       # DEV-123 duplicates DEV-456
  dex jira link --list-types                       # Show available link types`,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		listTypes, _ := cmd.Flags().GetBool("list-types")
		linkType, _ := cmd.Flags().GetString("type")

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// List available link types
		if listTypes {
			types, err := client.ListLinkTypes(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%-15s %-25s %s\n", "NAME", "OUTWARD", "INWARD")
			fmt.Println("─────────────────────────────────────────────────────────────")
			for _, t := range types {
				fmt.Printf("%-15s %-25s %s\n", t.Name, t.Outward, t.Inward)
			}
			return
		}

		// Require at least 2 issues for linking
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: at least two issue keys required\n")
			os.Exit(1)
		}

		sourceIssue := args[0]
		targetIssues := args[1:]

		for _, target := range targetIssues {
			req := jira.LinkIssuesRequest{
				InwardIssue:  sourceIssue,
				OutwardIssue: target,
				LinkType:     linkType,
			}
			if err := client.LinkIssues(ctx, req); err != nil {
				fmt.Fprintf(os.Stderr, "Error linking %s -> %s: %v\n", sourceIssue, target, err)
				continue
			}
			fmt.Printf("Linked %s -> %s (%s)\n", sourceIssue, target, linkType)
		}
	},
}

var jiraCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Jira issue",
	Long: `Create a new Jira issue with specified fields.

Examples:
  dex jira create -p DEV -t Task -s "Update API documentation"
  dex jira create -p DEV -t Bug -s "Login fails" -d "Users report 500 error on login"
  dex jira create -p TEL -t Story -s "Add dark mode" -l ui,enhancement
  dex jira create -p DEV -t Task -s "Fix tests" -a user@example.com --priority High
  dex jira create -p DEV -t Sub-task -s "Write unit tests" --parent DEV-123`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		project, _ := cmd.Flags().GetString("project")
		issueType, _ := cmd.Flags().GetString("type")
		summary, _ := cmd.Flags().GetString("summary")
		description, _ := cmd.Flags().GetString("description")
		labelsStr, _ := cmd.Flags().GetString("labels")
		assignee, _ := cmd.Flags().GetString("assignee")
		priority, _ := cmd.Flags().GetString("priority")
		parent, _ := cmd.Flags().GetString("parent")

		// Validate required fields
		if project == "" || issueType == "" || summary == "" {
			fmt.Fprintf(os.Stderr, "Error: --project, --type, and --summary are required\n")
			os.Exit(1)
		}

		// Parse labels
		var labels []string
		if labelsStr != "" {
			for _, l := range strings.Split(labelsStr, ",") {
				l = strings.TrimSpace(l)
				if l != "" {
					labels = append(labels, l)
				}
			}
		}

		client, err := jira.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		req := jira.CreateIssueRequest{
			ProjectKey:  project,
			IssueType:   issueType,
			Summary:     summary,
			Description: description,
			Labels:      labels,
			Assignee:    assignee,
			Priority:    priority,
			Parent:      parent,
		}

		issue, err := client.CreateIssue(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating issue: %v\n", err)
			os.Exit(1)
		}

		// Print the created issue
		siteURL := client.GetSiteURL()
		fmt.Printf("Created %s: %s\n", issue.Key, issue.Fields.Summary)
		if siteURL != "" {
			fmt.Printf("URL: %s/browse/%s\n", siteURL, issue.Key)
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
	jiraCmd.AddCommand(jiraCreateCmd)
	jiraCmd.AddCommand(jiraDeleteCmd)
	jiraCmd.AddCommand(jiraLinkCmd)

	jiraSearchCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().StringP("status", "s", "", "Filter by status (e.g., 'In Progress', 'Review')")
	jiraProjectsCmd.Flags().BoolP("keys", "k", false, "Output only project keys (one per line)")
	jiraProjectsCmd.Flags().BoolP("archived", "a", false, "Include archived projects")

	// Create command flags
	jiraCreateCmd.Flags().StringP("project", "p", "", "Project key (e.g., DEV, TEL)")
	jiraCreateCmd.Flags().StringP("type", "t", "", "Issue type (Task, Bug, Story, Sub-task)")
	jiraCreateCmd.Flags().StringP("summary", "s", "", "Issue summary/title")
	jiraCreateCmd.Flags().StringP("description", "d", "", "Issue description (plain text)")
	jiraCreateCmd.Flags().StringP("labels", "l", "", "Comma-separated labels")
	jiraCreateCmd.Flags().StringP("assignee", "a", "", "Assignee (email or account ID)")
	jiraCreateCmd.Flags().String("priority", "", "Priority (Lowest, Low, Medium, High, Highest)")
	jiraCreateCmd.Flags().String("parent", "", "Parent issue key for subtasks (e.g., DEV-123)")
	jiraCreateCmd.MarkFlagRequired("project")
	jiraCreateCmd.MarkFlagRequired("type")
	jiraCreateCmd.MarkFlagRequired("summary")

	// Link command flags
	jiraLinkCmd.Flags().StringP("type", "t", "Relates", "Link type (Relates, Blocks, Duplicate, etc.)")
	jiraLinkCmd.Flags().Bool("list-types", false, "List available link types")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
