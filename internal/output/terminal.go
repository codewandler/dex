package output

import (
	"fmt"
	"strings"
	"time"

	"dev-activity/internal/models"

	"github.com/fatih/color"
	gitlab "github.com/xanzy/go-gitlab"
)

var (
	headerColor  = color.New(color.FgCyan, color.Bold)
	projectColor = color.New(color.FgYellow, color.Bold)
	sectionColor = color.New(color.FgGreen)
	commitColor  = color.New(color.FgWhite)
	mrOpenColor  = color.New(color.FgBlue)
	mrMergedColor = color.New(color.FgGreen)
	mrClosedColor = color.New(color.FgRed)
	tagColor     = color.New(color.FgMagenta)
	dimColor     = color.New(color.FgHiBlack)
)

func PrintHeader(days int) {
	line := strings.Repeat("═", 60)
	fmt.Println()
	headerColor.Println(line)
	headerColor.Printf("  GitLab Activity Report (Last %d days)\n", days)
	headerColor.Println(line)
	fmt.Println()
}

func PrintHeaderDuration(d time.Duration) {
	line := strings.Repeat("═", 60)
	fmt.Println()
	headerColor.Println(line)
	headerColor.Printf("  GitLab Activity Report (%s)\n", formatDuration(d))
	headerColor.Println(line)
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "Last 1 minute"
		}
		return fmt.Sprintf("Last %d minutes", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "Last 1 hour"
		}
		return fmt.Sprintf("Last %d hours", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "Last 1 day"
	}
	return fmt.Sprintf("Last %d days", days)
}

func PrintProject(activity models.ProjectActivity) {
	projectColor.Printf("📁 %s\n", activity.ProjectPath)

	if len(activity.Commits) > 0 {
		sectionColor.Printf("   Commits (%d):\n", len(activity.Commits))
		for _, c := range activity.Commits {
			title := truncate(c.Title, 50)
			ago := timeAgo(c.CreatedAt)
			commitColor.Printf("     • %s - %s ", c.ShortID, title)
			dimColor.Printf("(%s, %s)\n", c.AuthorName, ago)
		}
	}

	if len(activity.MergeRequests) > 0 {
		sectionColor.Printf("   Merge Requests (%d):\n", len(activity.MergeRequests))
		for _, mr := range activity.MergeRequests {
			title := truncate(mr.Title, 45)
			stateStr := formatMRState(mr.State)
			fmt.Printf("     • !%d %s - %s\n", mr.IID, stateStr, title)
		}
	}

	if len(activity.Tags) > 0 {
		sectionColor.Printf("   Tags (%d):\n", len(activity.Tags))
		for _, t := range activity.Tags {
			ago := timeAgo(t.CreatedAt)
			tagColor.Printf("     • %s ", t.Name)
			dimColor.Printf("(created %s)\n", ago)
		}
	}

	fmt.Println()
}

func PrintSummary(summary models.ActivitySummary) {
	line := strings.Repeat("═", 60)
	headerColor.Println(line)
	headerColor.Printf("  Summary: %d commits, %d MRs, %d tags across %d projects\n",
		summary.TotalCommits,
		summary.TotalMergeRequests,
		summary.TotalTags,
		summary.TotalProjects,
	)
	headerColor.Println(line)
	fmt.Println()
}

func PrintNoActivity() {
	dimColor.Println("  No activity found in the specified time period.")
	fmt.Println()
}

func formatMRState(state string) string {
	switch state {
	case "merged":
		return mrMergedColor.Sprint("[MERGED]")
	case "opened":
		return mrOpenColor.Sprint("[OPEN]")
	case "closed":
		return mrClosedColor.Sprint("[CLOSED]")
	default:
		return fmt.Sprintf("[%s]", strings.ToUpper(state))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	duration := time.Since(t)
	hours := duration.Hours()

	if hours < 1 {
		return "just now"
	}
	if hours < 24 {
		h := int(hours)
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	}

	days := int(hours / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

var (
	labelColor = color.New(color.FgCyan)
	valueColor = color.New(color.FgWhite)
	langColor  = color.New(color.FgYellow)
)

func PrintProjectDetails(p *models.ProjectMetadata) {
	line := strings.Repeat("═", 60)
	fmt.Println()
	headerColor.Println(line)
	projectColor.Printf("  %s\n", p.PathWithNS)
	headerColor.Println(line)
	fmt.Println()

	// Basic info
	printField("ID", fmt.Sprintf("%d", p.ID))
	printField("Name", p.Name)
	printField("URL", p.WebURL)
	if p.Description != "" {
		printField("Description", truncate(p.Description, 60))
	}
	printField("Default Branch", p.DefaultBranch)
	printField("Visibility", p.Visibility)

	if len(p.Topics) > 0 {
		printField("Topics", strings.Join(p.Topics, ", "))
	}

	printField("Stars", fmt.Sprintf("%d", p.StarCount))
	printField("Forks", fmt.Sprintf("%d", p.ForksCount))
	fmt.Println()

	// Languages
	if len(p.Languages) > 0 {
		sectionColor.Println("  Languages:")
		printLanguages(p.Languages)
		fmt.Println()
	}

	// Contributors
	if len(p.TopContributors) > 0 {
		sectionColor.Println("  Top Contributors:")
		for _, c := range p.TopContributors {
			fmt.Printf("    • %-25s ", truncate(c.Name, 25))
			dimColor.Printf("%4d commits", c.Commits)
			if c.Additions > 0 || c.Deletions > 0 {
				dimColor.Printf(" (+%d/-%d)", c.Additions, c.Deletions)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Timestamps
	printField("Last Activity", formatTimestamp(p.LastActivityAt))
	printField("Indexed At", formatTimestamp(p.IndexedAt))
	fmt.Println()
}

func printField(label, value string) {
	labelColor.Printf("  %-16s ", label+":")
	valueColor.Println(value)
}

func printLanguages(langs map[string]float32) {
	// Sort languages by percentage
	type langPct struct {
		name string
		pct  float32
	}
	var sorted []langPct
	for name, pct := range langs {
		sorted = append(sorted, langPct{name, pct})
	}
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].pct > sorted[i].pct {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for _, l := range sorted {
		barLen := int(l.pct / 5) // 20 chars for 100%
		if barLen < 1 && l.pct > 0 {
			barLen = 1
		}
		bar := strings.Repeat("█", barLen)
		langColor.Printf("    %-12s ", l.name)
		sectionColor.Printf("%-20s", bar)
		dimColor.Printf(" %5.1f%%\n", l.pct)
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04"), timeAgo(t))
}

// PrintProjectListFromIndex prints projects from the local index
func PrintProjectListFromIndex(projects []models.ProjectMetadata, indexedAt time.Time) {
	if len(projects) == 0 {
		dimColor.Println("No projects in index. Run 'dex gitlab index' first.")
		return
	}

	line := strings.Repeat("═", 80)
	fmt.Println()
	headerColor.Println(line)
	headerColor.Printf("  GitLab Projects (%d)", len(projects))
	dimColor.Printf("  [indexed %s]\n", timeAgo(indexedAt))
	headerColor.Println(line)
	fmt.Println()

	// Print header row
	fmt.Printf("  %-6s  %-40s  %-12s  %s\n",
		"ID", "PATH", "VISIBILITY", "LAST ACTIVITY")
	fmt.Printf("  %s\n", strings.Repeat("─", 76))

	for _, p := range projects {
		path := truncate(p.PathWithNS, 40)
		visStr := formatVisibility(p.Visibility)

		projectColor.Printf("  %-6d  ", p.ID)
		fmt.Printf("%-40s  %s  ", path, visStr)
		dimColor.Printf("%s\n", timeAgo(p.LastActivityAt))
	}

	fmt.Println()
}

// PrintProjectList prints a list of GitLab projects from the API
func PrintProjectList(projects []*gitlab.Project) {
	if len(projects) == 0 {
		dimColor.Println("No projects found.")
		return
	}

	line := strings.Repeat("═", 80)
	fmt.Println()
	headerColor.Println(line)
	headerColor.Printf("  GitLab Projects (%d)\n", len(projects))
	headerColor.Println(line)
	fmt.Println()

	// Print header row
	fmt.Printf("  %-6s  %-40s  %-12s  %s\n",
		"ID", "PATH", "VISIBILITY", "LAST ACTIVITY")
	fmt.Printf("  %s\n", strings.Repeat("─", 76))

	for _, p := range projects {
		path := truncate(p.PathWithNamespace, 40)
		visStr := formatVisibility(string(p.Visibility))

		var lastActivity string
		if p.LastActivityAt != nil {
			lastActivity = timeAgo(*p.LastActivityAt)
		} else {
			lastActivity = "unknown"
		}

		projectColor.Printf("  %-6d  ", p.ID)
		fmt.Printf("%-40s  %s  ", path, visStr)
		dimColor.Printf("%s\n", lastActivity)
	}

	fmt.Println()
}

func formatVisibility(visibility string) string {
	switch visibility {
	case "public":
		return color.GreenString("%-12s", "public")
	case "internal":
		return color.YellowString("%-12s", "internal")
	case "private":
		return color.RedString("%-12s", "private")
	default:
		return fmt.Sprintf("%-12s", visibility)
	}
}

// PrintMergeRequestList prints a list of merge requests
func PrintMergeRequestList(mrs []models.MergeRequestDetail) {
	if len(mrs) == 0 {
		dimColor.Println("No merge requests found.")
		return
	}

	line := strings.Repeat("═", 90)
	fmt.Println()
	headerColor.Println(line)
	headerColor.Printf("  Merge Requests (%d)\n", len(mrs))
	headerColor.Println(line)
	fmt.Println()

	for _, mr := range mrs {
		// First line: state + project path + MR IID + title
		stateStr := formatMRState(mr.State)
		if mr.Draft {
			stateStr = dimColor.Sprint("[DRAFT]") + " " + stateStr
		}

		// Extract just the key part (e.g., "group/project!123")
		projectColor.Printf("  %s ", stateStr)
		fmt.Printf("%s\n", truncate(mr.Title, 70))

		// Second line: project, branches, author, time
		dimColor.Printf("    %s  ", mr.ProjectPath)
		sectionColor.Printf("%s → %s", mr.SourceBranch, mr.TargetBranch)
		dimColor.Printf("  by %s  %s\n", mr.Author, timeAgo(mr.UpdatedAt))

		// Third line: conflicts or merge status if relevant
		if mr.HasConflicts {
			mrClosedColor.Printf("    ⚠ Has conflicts\n")
		}

		fmt.Println()
	}
}

// PrintCommitDetails displays full commit information
func PrintCommitDetails(c *models.CommitDetail) {
	line := strings.Repeat("═", 60)
	fmt.Println()
	headerColor.Println(line)
	commitColor.Printf("  Commit %s\n", c.ShortID)
	headerColor.Println(line)
	fmt.Println()

	// Basic info
	printField("SHA", c.ID)
	printField("Author", fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail))
	if c.CommitterName != c.AuthorName {
		printField("Committer", fmt.Sprintf("%s <%s>", c.CommitterName, c.CommitterEmail))
	}
	printField("Date", formatTimestamp(c.CreatedAt))
	if c.WebURL != "" {
		printField("URL", c.WebURL)
	}
	fmt.Println()

	// Stats
	if c.Stats.Total > 0 {
		statsStr := fmt.Sprintf("+%d/-%d (%d total)", c.Stats.Additions, c.Stats.Deletions, c.Stats.Total)
		printField("Changes", statsStr)
		fmt.Println()
	}

	// Title and message
	sectionColor.Println("  Message:")
	fmt.Println()

	// Print the full message, indented
	lines := strings.Split(strings.TrimSpace(c.Message), "\n")
	for _, line := range lines {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
}
