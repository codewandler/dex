package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"dex/internal/config"
	"dex/internal/gitlab"
	"dex/internal/models"
	"dex/internal/output"

	"github.com/spf13/cobra"
	gogitlab "github.com/xanzy/go-gitlab"
)

const maxConcurrentProjects = 10

var gitlabCmd = &cobra.Command{
	Use:     "gl",
	Aliases: []string{"gitlab"},
	Short:   "GitLab activity and management",
	Long:    `Commands for interacting with GitLab repositories and activity.`,
}

var gitlabActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Fetch and display GitLab activity",
	Long: `Fetch recent activity from GitLab including commits, merge requests, and tags.

Examples:
  dex gitlab activity                    # Last 14 days (default)
  dex gitlab activity --since 7d         # Last 7 days
  dex gitlab activity --since 4h         # Last 4 hours
  dex gitlab activity --since 30m        # Last 30 minutes`,
	Run: func(cmd *cobra.Command, args []string) {
		sinceStr, _ := cmd.Flags().GetString("since")
		duration := parseDuration(sinceStr)

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Use parsed duration or fall back to config default
		if duration == 0 {
			duration = time.Duration(cfg.ActivityDays) * 24 * time.Hour
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		since := time.Now().Add(-duration)

		fmt.Printf("Fetching projects with activity since %s...\n", formatSinceTime(since, duration))

		projects, err := client.GetActiveProjects(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch projects: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Found %d projects with recent activity, fetching details...\n", len(projects))

		activities := fetchProjectActivitiesConcurrently(client, projects, since)

		fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")

		output.PrintHeaderDuration(duration)

		if len(activities) == 0 {
			output.PrintNoActivity()
			return
		}

		for _, activity := range activities {
			output.PrintProject(activity)
		}

		summary := models.CalculateSummary(activities)
		output.PrintSummary(summary)
	},
}

var gitlabIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index all accessible GitLab projects",
	Long: `Scan and cache metadata for all GitLab projects you have access to.

The index is stored at ~/.config/dex/gitlab-index.json and includes:
- Project info (name, path, description, visibility)
- Languages breakdown
- Top 5 contributors with commit stats

Examples:
  dex gitlab index           # Index if cache is older than 24h
  dex gitlab index --force   # Force re-index regardless of cache age`,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Check if index is fresh (< 24h old)
		if !force {
			idx, err := gitlab.LoadIndex()
			if err == nil && !idx.LastFullIndexAt.IsZero() {
				age := time.Since(idx.LastFullIndexAt)
				if age < 24*time.Hour {
					fmt.Printf("Index is fresh (%s old, %d projects). Use --force to re-index.\n",
						formatIndexAge(age), len(idx.Projects))
					return
				}
			}
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Indexing GitLab projects...")

		idx, err := client.IndexAllProjects(cfg.GitLabURL, func(completed, total int) {
			fmt.Printf("\r  Indexed %d/%d projects...", completed, total)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to index projects: %v\n", err)
			os.Exit(1)
		}

		fmt.Print("\r" + strings.Repeat(" ", 40) + "\r")

		if err := gitlab.SaveIndex(idx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save index: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Indexed %d projects. Saved to ~/.config/dex/gitlab-index.json\n", len(idx.Projects))
	},
}

var gitlabProjCmd = &cobra.Command{
	Use:   "proj",
	Short: "Project management commands",
	Long:  `Commands for listing and managing GitLab projects.`,
}

var gitlabCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit commands",
	Long:  `Commands for viewing GitLab commits.`,
}

var gitlabMRCmd = &cobra.Command{
	Use:     "mr",
	Aliases: []string{"merge-request"},
	Short:   "Merge request commands",
	Long:    `Commands for listing and managing GitLab merge requests.`,
}

var gitlabMRLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List merge requests",
	Long: `List merge requests with configurable filters.

State options:
  opened  - Open merge requests (default)
  merged  - Merged merge requests
  closed  - Closed merge requests
  all     - All merge requests

Scope options:
  all            - All visible MRs (default)
  created_by_me  - MRs you created
  assigned_to_me - MRs assigned to you

Examples:
  dex gl mr ls                          # List open MRs
  dex gl mr ls --state merged           # List merged MRs
  dex gl mr ls --scope created_by_me    # MRs you created
  dex gl mr ls --state all -n 50        # All MRs, limit 50`,
	Run: func(cmd *cobra.Command, args []string) {
		state, _ := cmd.Flags().GetString("state")
		scope, _ := cmd.Flags().GetString("scope")
		limit, _ := cmd.Flags().GetInt("limit")
		includeWIP, _ := cmd.Flags().GetBool("include-wip")
		conflictsOnly, _ := cmd.Flags().GetBool("conflicts-only")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		mrs, err := client.ListMergeRequests(gitlab.ListMergeRequestsOptions{
			State:         state,
			Scope:         scope,
			Limit:         limit,
			IncludeWIP:    includeWIP,
			ConflictsOnly: conflictsOnly,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list merge requests: %v\n", err)
			os.Exit(1)
		}

		output.PrintMergeRequestList(mrs)
	},
}

var gitlabMRShowCmd = &cobra.Command{
	Use:   "show <project!iid>",
	Short: "Show merge request details",
	Long: `Display detailed information about a specific merge request.

Use the canonical reference format: project!iid

Examples:
  dex gl mr show sre/helmchart-prod-configs!2903
  dex gl mr show group/project!456
  dex gl mr show group/project!456 --show-diff   # Include file diffs`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		showDiff, _ := cmd.Flags().GetBool("show-diff")

		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		mr, err := client.GetMergeRequest(projectID, mrIID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get merge request: %v\n", err)
			os.Exit(1)
		}

		// Fetch commits
		commits, err := client.GetMergeRequestCommits(projectID, mrIID)
		if err == nil {
			mr.Commits = commits
		}

		// Fetch file changes (with diff if requested)
		files, err := client.GetMergeRequestChanges(projectID, mrIID, showDiff)
		if err == nil {
			mr.Files = files
		}

		// Fetch discussions (threaded comments)
		discussions, err := client.GetMergeRequestDiscussions(projectID, mrIID)
		if err == nil {
			mr.Discussions = discussions
		}

		output.PrintMergeRequestDetails(mr)
	},
}

var gitlabMROpenCmd = &cobra.Command{
	Use:   "open <project!iid>",
	Short: "Open merge request in browser",
	Long: `Open a merge request in the default web browser.

Use the canonical reference format: project!iid

Examples:
  dex gl mr open sre/helmchart-prod-configs!2903
  dex gl mr open group/project!456`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		mr, err := client.GetMergeRequest(projectID, mrIID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get merge request: %v\n", err)
			os.Exit(1)
		}

		if err := openBrowser(mr.WebURL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open browser: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Opening %s\n", mr.WebURL)
	},
}

var gitlabMRCommentCmd = &cobra.Command{
	Use:   "comment <project!iid> <message>",
	Short: "Add a comment to a merge request",
	Long: `Add a comment/note to a merge request.

Use the canonical reference format: project!iid

The message can be provided as an argument or via stdin (use - as message).

Comment types:
  - Regular comment: just provide the message
  - Reply to thread: use --reply-to <discussion-id>
  - Inline comment: use --file and --line flags

Examples:
  dex gl mr comment sre/helm!2903 "LGTM, approved!"
  dex gl mr comment group/project!456 "Please address the review comments"
  echo "Comment from stdin" | dex gl mr comment group/project!456 -

  # Reply to an existing discussion thread
  dex gl mr comment project!123 "Done, fixed!" --reply-to abc12345

  # Add inline comment on a file/line
  dex gl mr comment project!123 "Use a constant here" --file src/main.go --line 42`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		replyTo, _ := cmd.Flags().GetString("reply-to")
		filePath, _ := cmd.Flags().GetString("file")
		lineNum, _ := cmd.Flags().GetInt("line")

		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		message := args[1]

		// Read from stdin if message is "-"
		if message == "-" {
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read from stdin: %v\n", err)
				os.Exit(1)
			}
			message = strings.TrimSpace(string(data))
		}

		if message == "" {
			fmt.Fprintf(os.Stderr, "Comment message cannot be empty\n")
			os.Exit(1)
		}

		// Validate flag combinations
		if replyTo != "" && (filePath != "" || lineNum > 0) {
			fmt.Fprintf(os.Stderr, "Cannot use --reply-to with --file/--line\n")
			os.Exit(1)
		}
		if (filePath != "" && lineNum == 0) || (filePath == "" && lineNum > 0) {
			fmt.Fprintf(os.Stderr, "Both --file and --line are required for inline comments\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		// Determine which type of comment to create
		if replyTo != "" {
			// Reply to existing discussion thread
			if err := client.AddMergeRequestDiscussionReply(projectID, mrIID, replyTo, message); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add reply: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Reply added to discussion %s on %s!%d\n", replyTo, projectID, mrIID)
		} else if filePath != "" && lineNum > 0 {
			// Create inline comment
			opts := gitlab.InlineCommentOptions{
				Body:    message,
				NewPath: filePath,
				OldPath: filePath,
				NewLine: lineNum,
			}
			if err := client.CreateMergeRequestInlineComment(projectID, mrIID, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add inline comment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Inline comment added to %s:%d on %s!%d\n", filePath, lineNum, projectID, mrIID)
		} else {
			// Regular comment
			if err := client.CreateMergeRequestNote(projectID, mrIID, message); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add comment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Comment added to %s!%d\n", projectID, mrIID)
		}
	},
}

var gitlabMRCloseCmd = &cobra.Command{
	Use:   "close <project!iid>",
	Short: "Close a merge request",
	Long: `Close an open merge request.

Use the canonical reference format: project!iid

Examples:
  dex gl mr close sre/helmchart-prod-configs!2903
  dex gl mr close group/project!456`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		if err := client.CloseMergeRequest(projectID, mrIID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close merge request: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Closed %s!%d\n", projectID, mrIID)
	},
}

var gitlabMRApproveCmd = &cobra.Command{
	Use:   "approve <project!iid>",
	Short: "Approve a merge request",
	Long: `Approve a merge request.

Use the canonical reference format: project!iid

Examples:
  dex gl mr approve sre/helmchart-prod-configs!2903
  dex gl mr approve group/project!456`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		if err := client.ApproveMergeRequest(projectID, mrIID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to approve merge request: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Approved %s!%d\n", projectID, mrIID)
	},
}

var gitlabMRMergeCmd = &cobra.Command{
	Use:   "merge <project!iid>",
	Short: "Merge a merge request",
	Long: `Merge (accept) a merge request.

Use the canonical reference format: project!iid

Examples:
  dex gl mr merge sre/helmchart-prod-configs!2903
  dex gl mr merge group/project!456 --squash
  dex gl mr merge group/project!456 --when-pipeline-succeeds
  dex gl mr merge group/project!456 --remove-source-branch`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		squash, _ := cmd.Flags().GetBool("squash")
		removeSource, _ := cmd.Flags().GetBool("remove-source-branch")
		whenPipeline, _ := cmd.Flags().GetBool("when-pipeline-succeeds")
		message, _ := cmd.Flags().GetString("message")

		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		if err := client.MergeMergeRequest(projectID, mrIID, gitlab.MergeMergeRequestOptions{
			Squash:                    squash,
			RemoveSourceBranch:        removeSource,
			MergeWhenPipelineSucceeds: whenPipeline,
			MergeCommitMessage:        message,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to merge: %v\n", err)
			os.Exit(1)
		}

		if whenPipeline {
			fmt.Printf("Set %s!%d to merge when pipeline succeeds\n", projectID, mrIID)
		} else {
			fmt.Printf("Merged %s!%d\n", projectID, mrIID)
		}
	},
}

var gitlabMRCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new merge request",
	Long: `Create a new merge request from the current branch.

By default, detects the current git branch as source and the project from
the git remote. Target branch defaults to 'main'.

Examples:
  dex gl mr create "Add user authentication"
  dex gl mr create "Fix login bug" --target develop
  dex gl mr create "WIP: New feature" --draft
  dex gl mr create "Refactor API" --description "Detailed description here"
  dex gl mr create "Feature" --project group/project --source feature-branch`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		title := args[0]
		target, _ := cmd.Flags().GetString("target")
		source, _ := cmd.Flags().GetString("source")
		project, _ := cmd.Flags().GetString("project")
		description, _ := cmd.Flags().GetString("description")
		draft, _ := cmd.Flags().GetBool("draft")
		removeSource, _ := cmd.Flags().GetBool("remove-source-branch")
		squash, _ := cmd.Flags().GetBool("squash")

		// Auto-detect source branch if not provided
		if source == "" {
			branch, err := getCurrentGitBranch()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to detect current branch: %v\n", err)
				fmt.Fprintf(os.Stderr, "Use --source to specify the source branch\n")
				os.Exit(1)
			}
			source = branch
		}

		// Auto-detect project if not provided
		if project == "" {
			proj, err := getGitLabProjectFromRemote()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to detect project from git remote: %v\n", err)
				fmt.Fprintf(os.Stderr, "Use --project to specify the project path\n")
				os.Exit(1)
			}
			project = proj
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		mr, err := client.CreateMergeRequest(project, gitlab.CreateMergeRequestOptions{
			Title:              title,
			Description:        description,
			SourceBranch:       source,
			TargetBranch:       target,
			Draft:              draft,
			RemoveSourceBranch: removeSource,
			Squash:             squash,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create merge request: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created %s!%d: %s\n", project, mr.IID, mr.Title)
		fmt.Printf("  %s\n", mr.WebURL)
	},
}

var gitlabMRReactCmd = &cobra.Command{
	Use:   "react <project!iid> <emoji>",
	Short: "Add a reaction to a merge request or comment",
	Long: `Add an emoji reaction to a merge request or a specific comment.

Use the canonical reference format: project!iid

By default, the reaction is added to the merge request itself.
Use --note to react to a specific comment instead.

Common emojis: thumbsup, thumbsdown, heart, tada, smile, rocket, eyes

Examples:
  dex gl mr react project!123 thumbsup
  dex gl mr react project!123 heart
  dex gl mr react project!123 thumbsup --note 456`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		noteID, _ := cmd.Flags().GetInt("note")

		projectID, mrIID, err := parseMRReference(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid MR reference: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use format: project!iid (e.g., group/project!123)\n")
			os.Exit(1)
		}

		emoji := args[1]
		// Remove colons if user included them (e.g., :thumbsup:)
		emoji = strings.Trim(emoji, ":")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		if noteID > 0 {
			// React to a specific note/comment
			if err := client.CreateMergeRequestNoteReaction(projectID, mrIID, noteID, emoji); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add reaction: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Added :%s: to note %d on %s!%d\n", emoji, noteID, projectID, mrIID)
		} else {
			// React to the MR itself
			if err := client.CreateMergeRequestReaction(projectID, mrIID, emoji); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add reaction: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Added :%s: to %s!%d\n", emoji, projectID, mrIID)
		}
	},
}

var gitlabCommitShowCmd = &cobra.Command{
	Use:   "show <project> <sha>",
	Short: "Show commit details",
	Long: `Display detailed information about a specific commit.

The commit SHA can be a full or short hash.

Examples:
  dex gl commit show 742 95a1e625
  dex gl commit show sre/mysql-mcp-wrapper 95a1e625
  dex gl commit show mygroup/myproject abc123def`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProjectNames,
	Run: func(cmd *cobra.Command, args []string) {
		projectID := args[0]
		sha := args[1]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		commit, err := client.GetCommit(projectID, sha)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get commit: %v\n", err)
			os.Exit(1)
		}

		output.PrintCommitDetails(commit)
	},
}

var gitlabProjLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List GitLab projects",
	Long: `List GitLab projects with configurable sorting and limits.

Uses the local index for instant results. Run 'dex gl index' first to populate.

Sort options:
  created    - Sort by creation date
  activity   - Sort by last activity (default)
  name       - Sort by project name
  path       - Sort by project path

Examples:
  dex gl proj ls                      # List 20 projects by last activity
  dex gl proj ls -n 50                # List 50 projects
  dex gl proj ls --sort name          # Sort by name ascending
  dex gl proj ls --sort created -d    # Sort by creation date descending
  dex gl proj ls --no-cache           # Fetch from API instead of index`,
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		sortField, _ := cmd.Flags().GetString("sort")
		desc, _ := cmd.Flags().GetBool("desc")
		noCache, _ := cmd.Flags().GetBool("no-cache")

		// Map sort field names to API values
		orderBy := "last_activity_at"
		switch sortField {
		case "created":
			orderBy = "created_at"
		case "activity":
			orderBy = "last_activity_at"
		case "name":
			orderBy = "name"
		case "path":
			orderBy = "path"
		}

		// Default sort direction based on field
		sortDir := "asc"
		if sortField == "created" || sortField == "activity" || sortField == "" {
			sortDir = "desc" // Dates default to descending (newest first)
		}
		if desc {
			sortDir = "desc"
		}

		// Try index first unless --no-cache
		if !noCache {
			idx, err := gitlab.LoadIndex()
			if err == nil && len(idx.Projects) > 0 {
				projects := idx.ListProjects(orderBy, sortDir, limit)
				output.PrintProjectListFromIndex(projects, idx.LastFullIndexAt)
				return
			}
		}

		// Fall back to API
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		projects, err := client.ListProjects(gitlab.ListProjectsOptions{
			Limit:   limit,
			OrderBy: orderBy,
			Sort:    sortDir,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list projects: %v\n", err)
			os.Exit(1)
		}

		output.PrintProjectList(projects)
	},
}

var gitlabShowCmd = &cobra.Command{
	Use:   "show <id|path>",
	Short: "Show project details",
	Long: `Display detailed information about a GitLab project.

Looks up in local cache first, falls back to API if not found.
When fetched from API, the project is added to the cache.

Examples:
  dex gl proj show 123                    # By project ID
  dex gl proj show group/project          # By path
  dex gl proj show group/sub/project      # Nested groups
  dex gl proj show myproject --no-cache   # Always fetch from API`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectNames,
	Run: func(cmd *cobra.Command, args []string) {
		noCache, _ := cmd.Flags().GetBool("no-cache")
		idOrPath := args[0]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		var pm *models.ProjectMetadata

		// Try cache first unless --no-cache
		if !noCache {
			idx, err := gitlab.LoadIndex()
			if err == nil {
				pm = idx.FindProject(idOrPath)
			}
		}

		// Fetch from API if not in cache or --no-cache
		if pm == nil {
			client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
				os.Exit(1)
			}

			pm, err = client.GetProjectMetadata(idOrPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Project not found: %v\n", err)
				os.Exit(1)
			}

			// Add to cache unless --no-cache
			if !noCache {
				idx, _ := gitlab.LoadIndex()
				if idx != nil {
					idx.UpsertProject(*pm)
					gitlab.SaveIndex(idx)
				}
			}
		}

		output.PrintProjectDetails(pm)
	},
}

// completeProjectNames provides shell completion for project names from the index
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	idx, err := gitlab.LoadIndex()
	if err != nil || len(idx.Projects) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	toCompleteLower := strings.ToLower(toComplete)

	for _, p := range idx.Projects {
		// Match against path (most common use case)
		if strings.Contains(strings.ToLower(p.PathWithNS), toCompleteLower) {
			completions = append(completions, p.PathWithNS+"\t"+p.Name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func formatIndexAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// getCurrentGitBranch returns the current git branch name
func getCurrentGitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitLabProjectFromRemote extracts the GitLab project path from the git remote
func getGitLabProjectFromRemote() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(string(output))

	// Handle SSH format: git@gitlab.example.com:group/project.git
	if strings.HasPrefix(url, "git@") {
		// Remove git@ prefix and find the : separator
		url = strings.TrimPrefix(url, "git@")
		colonIdx := strings.Index(url, ":")
		if colonIdx == -1 {
			return "", fmt.Errorf("invalid SSH remote format")
		}
		url = url[colonIdx+1:]
	} else if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// Handle HTTPS format: https://gitlab.example.com/group/project.git
		// Remove the protocol and host
		parts := strings.SplitN(url, "/", 4)
		if len(parts) < 4 {
			return "", fmt.Errorf("invalid HTTPS remote format")
		}
		url = parts[3]
	}

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	return url, nil
}

// parseMRReference parses a merge request reference like "group/project!123"
// Returns the project path and MR IID
func parseMRReference(ref string) (string, int, error) {
	// Find the last ! which separates project from IID
	idx := strings.LastIndex(ref, "!")
	if idx == -1 {
		return "", 0, fmt.Errorf("missing '!' separator")
	}

	project := ref[:idx]
	iidStr := ref[idx+1:]

	if project == "" {
		return "", 0, fmt.Errorf("empty project path")
	}

	iid, err := strconv.Atoi(iidStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid IID: %s", iidStr)
	}

	return project, iid, nil
}

func init() {
	gitlabCmd.AddCommand(gitlabActivityCmd)
	gitlabCmd.AddCommand(gitlabIndexCmd)
	gitlabCmd.AddCommand(gitlabProjCmd)
	gitlabCmd.AddCommand(gitlabCommitCmd)
	gitlabCmd.AddCommand(gitlabMRCmd)

	gitlabProjCmd.AddCommand(gitlabProjLsCmd)
	gitlabProjCmd.AddCommand(gitlabShowCmd)

	gitlabCommitCmd.AddCommand(gitlabCommitShowCmd)

	gitlabMRCmd.AddCommand(gitlabMRLsCmd)
	gitlabMRCmd.AddCommand(gitlabMRShowCmd)
	gitlabMRCmd.AddCommand(gitlabMROpenCmd)
	gitlabMRCmd.AddCommand(gitlabMRCommentCmd)
	gitlabMRCmd.AddCommand(gitlabMRReactCmd)
	gitlabMRCmd.AddCommand(gitlabMRCloseCmd)
	gitlabMRCmd.AddCommand(gitlabMRApproveCmd)
	gitlabMRCmd.AddCommand(gitlabMRMergeCmd)
	gitlabMRCmd.AddCommand(gitlabMRCreateCmd)

	gitlabActivityCmd.Flags().StringP("since", "s", "14d", "Time period to look back (e.g., 4h, 30m, 7d)")
	gitlabIndexCmd.Flags().BoolP("force", "f", false, "Force re-index even if cache is fresh")
	gitlabShowCmd.Flags().Bool("no-cache", false, "Always fetch from API, don't use cache")

	gitlabProjLsCmd.Flags().IntP("limit", "n", 20, "Number of projects to list (0 = all)")
	gitlabProjLsCmd.Flags().StringP("sort", "s", "activity", "Sort by: created, activity, name, path")
	gitlabProjLsCmd.Flags().BoolP("desc", "d", false, "Sort descending (default for dates, ascending for names)")
	gitlabProjLsCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")

	gitlabMRLsCmd.Flags().StringP("state", "s", "opened", "MR state: opened, merged, closed, all")
	gitlabMRLsCmd.Flags().String("scope", "all", "Scope: all, created_by_me, assigned_to_me")
	gitlabMRLsCmd.Flags().IntP("limit", "n", 20, "Number of MRs to list")
	gitlabMRLsCmd.Flags().Bool("include-wip", false, "Include WIP/draft MRs (excluded by default)")
	gitlabMRLsCmd.Flags().Bool("conflicts-only", false, "Only show MRs with merge conflicts")

	gitlabMRShowCmd.Flags().Bool("show-diff", false, "Show file diffs")

	gitlabMRCommentCmd.Flags().String("reply-to", "", "Reply to an existing discussion thread (discussion ID)")
	gitlabMRCommentCmd.Flags().String("file", "", "File path for inline comment")
	gitlabMRCommentCmd.Flags().Int("line", 0, "Line number for inline comment")

	gitlabMRReactCmd.Flags().Int("note", 0, "Note ID to react to (instead of MR)")

	gitlabMRMergeCmd.Flags().Bool("squash", false, "Squash commits on merge")
	gitlabMRMergeCmd.Flags().Bool("remove-source-branch", false, "Remove source branch after merge")
	gitlabMRMergeCmd.Flags().Bool("when-pipeline-succeeds", false, "Merge when pipeline succeeds")
	gitlabMRMergeCmd.Flags().StringP("message", "m", "", "Custom merge commit message")

	gitlabMRCreateCmd.Flags().StringP("target", "t", "main", "Target branch")
	gitlabMRCreateCmd.Flags().StringP("source", "s", "", "Source branch (default: current branch)")
	gitlabMRCreateCmd.Flags().StringP("project", "p", "", "Project path (default: from git remote)")
	gitlabMRCreateCmd.Flags().StringP("description", "d", "", "MR description")
	gitlabMRCreateCmd.Flags().Bool("draft", false, "Create as draft/WIP")
	gitlabMRCreateCmd.Flags().Bool("remove-source-branch", false, "Remove source branch after merge")
	gitlabMRCreateCmd.Flags().Bool("squash", false, "Squash commits on merge")
}

// parseDuration parses a duration string like "30m", "4h", "7d" and returns time.Duration
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	// Handle days (not supported by time.ParseDuration)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}

	// Try standard duration parsing (handles h, m, s)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	// Try parsing as plain number (assume days for backwards compatibility)
	if days, err := strconv.Atoi(s); err == nil && days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}

	return 0
}

// formatSinceTime returns a human-readable description of the time range
func formatSinceTime(since time.Time, duration time.Duration) string {
	if duration < time.Hour {
		return fmt.Sprintf("%s (%d minutes ago)", since.Format("15:04"), int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%s (%d hours ago)", since.Format("15:04"), int(duration.Hours()))
	}
	return since.Format("2006-01-02")
}

// fetchProjectActivitiesConcurrently fetches activity for multiple projects in parallel
func fetchProjectActivitiesConcurrently(client *gitlab.Client, projects []*gogitlab.Project, since time.Time) []models.ProjectActivity {
	type result struct {
		activity models.ProjectActivity
		hasData  bool
	}

	results := make(chan result, len(projects))
	semaphore := make(chan struct{}, maxConcurrentProjects)

	var wg sync.WaitGroup

	for _, project := range projects {
		wg.Add(1)
		go func(p *gogitlab.Project) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			activity := models.ProjectActivity{
				ProjectID:   p.ID,
				ProjectName: p.Name,
				ProjectPath: p.PathWithNamespace,
				WebURL:      p.WebURL,
			}

			commits, err := client.GetCommits(p.ID, since)
			if err == nil {
				activity.Commits = commits
			}

			mrs, err := client.GetMergeRequests(p.ID, since)
			if err == nil {
				activity.MergeRequests = mrs
			}

			tags, err := client.GetTags(p.ID, since)
			if err == nil {
				activity.Tags = tags
			}

			results <- result{activity: activity, hasData: activity.HasActivity()}
		}(project)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var activities []models.ProjectActivity
	completed := 0
	total := len(projects)

	for r := range results {
		completed++
		fmt.Printf("\r  Fetched %d/%d projects...", completed, total)
		if r.hasData {
			activities = append(activities, r.activity)
		}
	}

	return activities
}
