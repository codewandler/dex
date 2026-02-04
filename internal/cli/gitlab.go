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
	Use:   "gitlab",
	Short: "GitLab activity and management",
	Long:  `Commands for interacting with GitLab repositories and activity.`,
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

var gitlabShowCmd = &cobra.Command{
	Use:   "show <id|path>",
	Short: "Show project details",
	Long: `Display detailed information about a GitLab project.

Looks up in local cache first, falls back to API if not found.
When fetched from API, the project is added to the cache.

Examples:
  dex gitlab show 123                    # By project ID
  dex gitlab show group/project          # By path
  dex gitlab show group/sub/project      # Nested groups
  dex gitlab show myproject --no-cache   # Always fetch from API`,
	Args: cobra.ExactArgs(1),
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
	gitlabCmd.AddCommand(gitlabShowCmd)

	gitlabActivityCmd.Flags().StringP("since", "s", "14d", "Time period to look back (e.g., 4h, 30m, 7d)")
	gitlabIndexCmd.Flags().BoolP("force", "f", false, "Force re-index even if cache is fresh")
	gitlabShowCmd.Flags().Bool("no-cache", false, "Always fetch from API, don't use cache")
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
