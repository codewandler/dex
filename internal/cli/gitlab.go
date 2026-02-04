package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"dev-activity/internal/config"
	"dev-activity/internal/gitlab"
	"dev-activity/internal/models"
	"dev-activity/internal/output"

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
			State: state,
			Scope: scope,
			Limit: limit,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list merge requests: %v\n", err)
			os.Exit(1)
		}

		output.PrintMergeRequestList(mrs)
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
