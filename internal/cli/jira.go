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
			RenderError(err)
		}

		if err := client.EnsureAuth(ctx); err != nil {
			RenderError(fmt.Errorf("authentication failed: %w", err))
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
			RenderError(err)
		}

		issue, err := client.GetIssue(ctx, args[0])
		if err != nil {
			RenderError(err)
		}

		Render(issue)
	},
}

var jiraSearchCmd = &cobra.Command{
	Use:   "search [JQL]",
	Short: "Search issues with JQL query",
	Long: `Search for issues using JQL (Jira Query Language).

Examples:
  dex jira search "project = TEL"
  dex jira search "assignee = currentUser() AND status != Done"
  dex jira search "updated >= -7d ORDER BY updated DESC"`,
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
			RenderError(err)
		}

		result, err := client.SearchIssues(ctx, jql, limit)
		if err != nil {
			RenderError(err)
		}

		Render(result)
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
			RenderError(err)
		}

		jql := "assignee = currentUser()"
		if status != "" {
			jql += fmt.Sprintf(" AND status = '%s'", status)
		} else {
			jql += " AND status != Done"
		}
		jql += " ORDER BY updated DESC"

		result, err := client.SearchIssues(ctx, jql, limit)
		if err != nil {
			RenderError(err)
		}

		// Wrap in a type that has a "my issues" flavoured text header
		Render(&jira.MyIssueResult{SearchResult: result})
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
			RenderError(err)
		}

		// TODO(#24): add structured output
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
			RenderError(err)
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
			RenderError(err)
		}

		if listTypes {
			types, err := client.ListLinkTypes(ctx)
			if err != nil {
				RenderError(err)
			}
			// TODO(#24): add structured output
			fmt.Printf("%-15s %-25s %s\n", "NAME", "OUTWARD", "INWARD")
			fmt.Println("─────────────────────────────────────────────────────────────")
			for _, t := range types {
				fmt.Printf("%-15s %-25s %s\n", t.Name, t.Outward, t.Inward)
			}
			return
		}

		if len(args) < 2 {
			RenderError(fmt.Errorf("at least two issue keys required"))
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

var jiraUnlinkCmd = &cobra.Command{
	Use:   "unlink <ISSUE-KEY> <TARGET-KEY> [TARGET-KEY...]",
	Short: "Remove links between issues",
	Long: `Remove links between Jira issues.

The first issue is the source, subsequent issues are targets whose links will be removed.
Use --type to filter by link type when multiple links exist between the same issues.

Examples:
  dex jira unlink DEV-123 DEV-456                    # Remove link between two issues
  dex jira unlink DEV-123 DEV-456 DEV-789            # Remove multiple links from DEV-123
  dex jira unlink DEV-123 DEV-456 -t Blocks          # Remove only "Blocks" link`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		linkType, _ := cmd.Flags().GetString("type")

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		sourceIssue := args[0]
		targetIssues := args[1:]

		for _, target := range targetIssues {
			if err := client.UnlinkIssues(ctx, sourceIssue, target, linkType); err != nil {
				fmt.Fprintf(os.Stderr, "Error unlinking %s -> %s: %v\n", sourceIssue, target, err)
				continue
			}
			fmt.Printf("Unlinked %s -> %s\n", sourceIssue, target)
		}
	},
}

var jiraUpdateCmd = &cobra.Command{
	Use:   "update <ISSUE-KEY>",
	Short: "Update issue fields",
	Long: `Update fields on a Jira issue.

Examples:
  dex jira update DEV-123 --summary "New title"
  dex jira update DEV-123 --assignee user@example.com
  dex jira update DEV-123 --assignee ""                  # Unassign
  dex jira update DEV-123 --priority High
  dex jira update DEV-123 --add-label urgent
  dex jira update DEV-123 --remove-label backlog
  dex jira update DEV-123 --description "New description"
  dex jira update DEV-123 --parent DEV-100                    # Set parent
  dex jira update DEV-123 --parent ""                         # Clear parent
  dex jira update DEV-123 --assignee me@example.com --priority High --add-label urgent`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		issueKey := args[0]

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		req := jira.UpdateIssueRequest{}
		hasUpdates := false

		if cmd.Flags().Changed("summary") {
			v, _ := cmd.Flags().GetString("summary")
			req.Summary = &v
			hasUpdates = true
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = &v
			hasUpdates = true
		}
		if cmd.Flags().Changed("assignee") {
			v, _ := cmd.Flags().GetString("assignee")
			req.Assignee = &v
			hasUpdates = true
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			req.Priority = &v
			hasUpdates = true
		}
		if cmd.Flags().Changed("add-label") {
			v, _ := cmd.Flags().GetStringSlice("add-label")
			req.AddLabels = v
			hasUpdates = true
		}
		if cmd.Flags().Changed("remove-label") {
			v, _ := cmd.Flags().GetStringSlice("remove-label")
			req.RemoveLabels = v
			hasUpdates = true
		}
		if cmd.Flags().Changed("parent") {
			v, _ := cmd.Flags().GetString("parent")
			req.Parent = &v
			hasUpdates = true
		}

		if !hasUpdates {
			RenderError(fmt.Errorf("no updates specified"))
		}

		if err := client.UpdateIssue(ctx, issueKey, req); err != nil {
			RenderError(err)
		}

		fmt.Printf("Updated %s\n", issueKey)
	},
}

var jiraTransitionCmd = &cobra.Command{
	Use:   "transition <ISSUE-KEY> [STATUS]",
	Short: "Transition issue to a new status",
	Long: `Move an issue through its workflow to a new status.

Use --list to see available transitions for an issue.

Examples:
  dex jira transition DEV-123 --list           # Show available transitions
  dex jira transition DEV-123 "In Progress"    # Move to In Progress
  dex jira transition DEV-123 Done             # Move to Done
  dex jira transition DEV-123 Review           # Move to Review`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		issueKey := args[0]
		listTransitions, _ := cmd.Flags().GetBool("list")

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		if listTransitions {
			transitions, err := client.ListTransitions(ctx, issueKey)
			if err != nil {
				RenderError(err)
			}
			Render(&jira.TransitionList{IssueKey: issueKey, Transitions: transitions})
			return
		}

		if len(args) < 2 {
			RenderError(fmt.Errorf("status argument required (or use --list)"))
		}

		targetStatus := args[1]
		if err := client.TransitionIssue(ctx, issueKey, targetStatus); err != nil {
			RenderError(err)
		}

		issue, err := client.GetIssue(ctx, issueKey)
		if err != nil {
			fmt.Printf("Transitioned %s\n", issueKey)
			return
		}
		fmt.Printf("Transitioned %s → %s\n", issueKey, issue.Fields.Status.Name)
	},
}

var jiraCommentCmd = &cobra.Command{
	Use:   "comment <ISSUE-KEY> <MESSAGE>",
	Short: "Add a comment to an issue",
	Long: `Add a comment to a Jira issue.

The message can be provided as an argument or via --body flag for longer text.
Supports markdown formatting (headings, lists, code blocks, links, etc.)
which is automatically converted to Jira's format.

Examples:
  dex jira comment DEV-123 "Working on this now"
  dex jira comment DEV-123 --body "## Status Update

- Fixed the auth bug
- Still need to add tests

See DEV-456 for context"`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		issueKey := args[0]
		body, _ := cmd.Flags().GetString("body")

		if len(args) > 1 {
			body = args[1]
		}
		if body == "" {
			RenderError(fmt.Errorf("comment message required"))
		}

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		comment, err := client.AddComment(ctx, issueKey, body)
		if err != nil {
			RenderError(err)
		}

		fmt.Printf("Added comment to %s (id: %s)\n", issueKey, comment.ID)
	},
}

var jiraCommentDeleteCmd = &cobra.Command{
	Use:   "comment-delete <ISSUE-KEY> <COMMENT-ID>",
	Short: "Delete a comment from an issue",
	Long: `Delete a comment from a Jira issue.

Examples:
  dex jira comment-delete DEV-123 10042`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		issueKey := args[0]
		commentID := args[1]

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		if err := client.DeleteComment(ctx, issueKey, commentID); err != nil {
			RenderError(err)
		}

		fmt.Printf("Deleted comment %s from %s\n", commentID, issueKey)
	},
}

var jiraCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Jira issue",
	Long: `Create a new Jira issue with specified fields.

Descriptions support markdown formatting (headings, lists, code blocks, links, etc.)
which is automatically converted to Jira's format. Issue keys like DEV-123 in the
description are auto-linked.

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

		if project == "" || issueType == "" || summary == "" {
			RenderError(fmt.Errorf("--project, --type, and --summary are required"))
		}

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
			RenderError(err)
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
			RenderError(fmt.Errorf("creating issue: %w", err))
		}

		siteURL := client.GetSiteURL()
		fmt.Printf("Created %s: %s\n", issue.Key, issue.Fields.Summary)
		if siteURL != "" {
			fmt.Printf("URL: %s/browse/%s\n", siteURL, issue.Key)
		}
	},
}

var jiraProjectCmd = &cobra.Command{
	Use:   "project <PROJECT-KEY>",
	Short: "Show detailed information about a project",
	Long: `Display detailed information about a Jira project including
its issue types, components, and workflow statuses.

Examples:
  dex jira project DEV
  dex jira project TEL`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		projectKey := strings.ToUpper(args[0])
		transitionsOnly, _ := cmd.Flags().GetBool("transitions")

		client, err := jira.NewClient()
		if err != nil {
			RenderError(err)
		}

		project, err := client.GetProject(ctx, projectKey)
		if err != nil {
			RenderError(err)
		}

		statuses, _ := client.GetProjectStatuses(ctx, projectKey) // non-fatal

		siteURL := client.GetSiteURL()
		if siteURL != "" {
			project.URL = fmt.Sprintf("%s/jira/software/projects/%s/boards", siteURL, project.Key)
		}

		if transitionsOnly {
			Render(&jira.WorkflowStatuses{ProjectKey: project.Key, Statuses: statuses})
			return
		}

		Render(&jira.ProjectWithStatuses{Project: project, Statuses: statuses, SiteURL: siteURL})
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
			RenderError(err)
		}

		projects, err := client.ListProjects(ctx)
		if err != nil {
			RenderError(err)
		}

		var filtered []jira.Project
		for _, p := range projects {
			isArchived := strings.HasPrefix(strings.ToLower(p.Name), "z[archive")
			if showArchived || !isArchived {
				filtered = append(filtered, p)
			}
		}

		siteURL := client.GetSiteURL()

		// --keys with plain text: legacy one-key-per-line output
		if keysOnly && (outputFormat == "text" || outputFormat == "") {
			for _, p := range filtered {
				fmt.Println(p.Key)
			}
			return
		}

		Render(&jira.ProjectList{Projects: filtered, SiteURL: siteURL})
	},
}

func init() {
	jiraCmd.AddCommand(jiraAuthCmd)
	jiraCmd.AddCommand(jiraViewCmd)
	jiraCmd.AddCommand(jiraSearchCmd)
	jiraCmd.AddCommand(jiraMyCmd)
	jiraCmd.AddCommand(jiraLookupCmd)
	jiraCmd.AddCommand(jiraProjectCmd)
	jiraCmd.AddCommand(jiraProjectsCmd)
	jiraCmd.AddCommand(jiraCreateCmd)
	jiraCmd.AddCommand(jiraDeleteCmd)
	jiraCmd.AddCommand(jiraLinkCmd)
	jiraCmd.AddCommand(jiraUnlinkCmd)
	jiraCmd.AddCommand(jiraUpdateCmd)
	jiraCmd.AddCommand(jiraTransitionCmd)
	jiraCmd.AddCommand(jiraCommentCmd)
	jiraCmd.AddCommand(jiraCommentDeleteCmd)

	jiraSearchCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	jiraMyCmd.Flags().StringP("status", "s", "", "Filter by status (e.g., 'In Progress', 'Review')")
	jiraProjectCmd.Flags().BoolP("transitions", "t", false, "Only show workflow statuses/transitions")

	jiraProjectsCmd.Flags().BoolP("keys", "k", false, "Output only project keys (one per line)")
	jiraProjectsCmd.Flags().BoolP("archived", "a", false, "Include archived projects")

	jiraCreateCmd.Flags().StringP("project", "p", "", "Project key (e.g., DEV, TEL)")
	jiraCreateCmd.Flags().StringP("type", "t", "", "Issue type (Task, Bug, Story, Sub-task)")
	jiraCreateCmd.Flags().StringP("summary", "s", "", "Issue summary/title")
	jiraCreateCmd.Flags().StringP("description", "d", "", "Issue description (markdown)")
	jiraCreateCmd.Flags().StringP("labels", "l", "", "Comma-separated labels")
	jiraCreateCmd.Flags().StringP("assignee", "a", "", "Assignee (email or account ID)")
	jiraCreateCmd.Flags().String("priority", "", "Priority (Lowest, Low, Medium, High, Highest)")
	jiraCreateCmd.Flags().String("parent", "", "Parent issue key for subtasks (e.g., DEV-123)")
	jiraCreateCmd.MarkFlagRequired("project")
	jiraCreateCmd.MarkFlagRequired("type")
	jiraCreateCmd.MarkFlagRequired("summary")

	jiraLinkCmd.Flags().StringP("type", "t", "Relates", "Link type (Relates, Blocks, Duplicate, etc.)")
	jiraLinkCmd.Flags().Bool("list-types", false, "List available link types")

	jiraUnlinkCmd.Flags().StringP("type", "t", "", "Filter by link type (e.g., Blocks, Duplicate)")

	jiraUpdateCmd.Flags().StringP("summary", "s", "", "New summary/title")
	jiraUpdateCmd.Flags().StringP("description", "d", "", "New description (markdown)")
	jiraUpdateCmd.Flags().StringP("assignee", "a", "", "New assignee (email or account ID, empty to unassign)")
	jiraUpdateCmd.Flags().StringP("priority", "p", "", "New priority (Lowest, Low, Medium, High, Highest)")
	jiraUpdateCmd.Flags().StringSlice("add-label", nil, "Labels to add (can specify multiple)")
	jiraUpdateCmd.Flags().StringSlice("remove-label", nil, "Labels to remove (can specify multiple)")
	jiraUpdateCmd.Flags().String("parent", "", "Parent issue key (e.g., DEV-123, empty to clear)")

	jiraTransitionCmd.Flags().BoolP("list", "l", false, "List available transitions")

	jiraCommentCmd.Flags().StringP("body", "b", "", "Comment body in markdown (alternative to positional argument)")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
