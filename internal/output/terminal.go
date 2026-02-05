package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	gl "github.com/codewandler/dex/internal/gitlab"
	"github.com/codewandler/dex/internal/models"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	gitlab "github.com/xanzy/go-gitlab"
	"golang.org/x/term"
)

var (
	headerColor   = color.New(color.FgCyan, color.Bold)
	projectColor  = color.New(color.FgYellow, color.Bold)
	sectionColor  = color.New(color.FgGreen)
	commitColor   = color.New(color.FgWhite)
	mrOpenColor   = color.New(color.FgBlue)
	mrMergedColor = color.New(color.FgGreen)
	mrClosedColor = color.New(color.FgRed)
	tagColor      = color.New(color.FgMagenta)
	dimColor      = color.New(color.FgHiBlack)
	linkColor     = color.New(color.FgCyan, color.Underline)

	// Markdown renderer for comments
	mdRenderer, _ = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
)

// hyperlink creates a clickable terminal hyperlink using OSC 8 escape sequence
// Uses BEL (\a) as string terminator for wider terminal compatibility
// Styled with underline and cyan color to indicate it's clickable
func hyperlink(url, text string) string {
	styledText := linkColor.Sprint(text)
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return styledText
	}
	return fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07", url, styledText)
}

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

// renderMarkdown renders markdown text for terminal display
func renderMarkdown(text string) string {
	if mdRenderer == nil {
		return text
	}
	rendered, err := mdRenderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(rendered)
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
		// First line: state + title
		stateStr := formatMRState(mr.State)
		if mr.Draft {
			stateStr = dimColor.Sprint("[DRAFT]") + " " + stateStr
		}

		projectColor.Printf("  %s ", stateStr)
		fmt.Printf("%s\n", truncate(mr.Title, 70))

		// Second line: clickable project reference, branches, author, time
		refLink := hyperlink(mr.WebURL, mr.ProjectPath)
		fmt.Printf("    %s  ", refLink)
		sectionColor.Printf("%s → %s", mr.SourceBranch, mr.TargetBranch)
		dimColor.Printf("  by %s  %s\n", mr.Author, timeAgo(mr.UpdatedAt))

		// Third line: conflicts or merge status if relevant
		if mr.HasConflicts {
			mrClosedColor.Printf("    ⚠ Has conflicts\n")
		}

		fmt.Println()
	}
}

// PrintMergeRequestDetails displays full MR information
func PrintMergeRequestDetails(mr *models.MergeRequestDetail) {
	line := strings.Repeat("═", 70)
	fmt.Println()
	headerColor.Println(line)

	// Title with state
	stateStr := formatMRState(mr.State)
	if mr.Draft {
		stateStr = dimColor.Sprint("[DRAFT]") + " " + stateStr
	}
	projectColor.Printf("  %s %s\n", stateStr, mr.Title)
	headerColor.Println(line)
	fmt.Println()

	// Basic info
	printField("Reference", mr.ProjectPath)
	printField("URL", mr.WebURL)
	printField("Author", mr.Author)
	printField("Branches", fmt.Sprintf("%s → %s", mr.SourceBranch, mr.TargetBranch))
	printField("Created", formatTimestamp(mr.CreatedAt))
	printField("Updated", formatTimestamp(mr.UpdatedAt))

	if mr.MergedAt != nil {
		printField("Merged", formatTimestamp(*mr.MergedAt))
		if mr.MergedBy != "" {
			printField("Merged By", mr.MergedBy)
		}
	}

	// Assignees
	if len(mr.Assignees) > 0 {
		printField("Assignees", strings.Join(mr.Assignees, ", "))
	}

	// Reviewers
	if len(mr.Reviewers) > 0 {
		printField("Reviewers", strings.Join(mr.Reviewers, ", "))
	}

	// Labels
	if len(mr.Labels) > 0 {
		printField("Labels", strings.Join(mr.Labels, ", "))
	}

	// Changes
	if mr.Changes.Files > 0 {
		printField("Changes", fmt.Sprintf("%d files", mr.Changes.Files))
	}

	// Conflicts warning
	if mr.HasConflicts {
		fmt.Println()
		mrClosedColor.Printf("  ⚠ This merge request has conflicts that must be resolved\n")
	}

	// Description
	if mr.Description != "" {
		fmt.Println()
		sectionColor.Println("  Description:")
		fmt.Println()
		rendered := renderMarkdown(mr.Description)
		for _, line := range strings.Split(rendered, "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	// Commits
	if len(mr.Commits) > 0 {
		fmt.Println()
		sectionColor.Printf("  Commits (%d):\n", len(mr.Commits))
		for _, c := range mr.Commits {
			commitColor.Printf("    %s ", c.ShortID)
			fmt.Printf("%s ", truncate(c.Title, 50))
			dimColor.Printf("(%s)\n", c.Author)
		}
	}

	// Files
	if len(mr.Files) > 0 {
		fmt.Println()
		sectionColor.Printf("  Files Changed (%d):\n", len(mr.Files))
		for _, f := range mr.Files {
			printMRFile(f)
		}
	}

	// Discussions/Comments (prefer threaded discussions if available)
	if len(mr.Discussions) > 0 {
		PrintMergeRequestDiscussions(mr.Discussions)
	} else if len(mr.Notes) > 0 {
		// Fall back to flat notes for backwards compatibility
		var userNotes []models.MRNote
		for _, n := range mr.Notes {
			if !n.System {
				userNotes = append(userNotes, n)
			}
		}
		if len(userNotes) > 0 {
			fmt.Println()
			sectionColor.Printf("  Comments (%d):\n", len(userNotes))
			for _, n := range userNotes {
				fmt.Println()
				labelColor.Printf("    %s ", n.Author)
				dimColor.Printf("(%s):\n", timeAgo(n.CreatedAt))
				// Render markdown
				rendered := renderMarkdown(n.Body)
				// Indent each line
				for _, line := range strings.Split(rendered, "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	}

	fmt.Println()
}

// PrintMergeRequestDiscussions displays discussion threads with IDs for easy reference
func PrintMergeRequestDiscussions(discussions []models.MRDiscussion) {
	// Count non-system discussions
	var userDiscussions []models.MRDiscussion
	for _, d := range discussions {
		hasUserNotes := false
		for _, n := range d.Notes {
			if !n.System {
				hasUserNotes = true
				break
			}
		}
		if hasUserNotes {
			userDiscussions = append(userDiscussions, d)
		}
	}

	if len(userDiscussions) == 0 {
		return
	}

	fmt.Println()
	sectionColor.Printf("  Comments (%d):\n", len(userDiscussions))

	for _, d := range userDiscussions {
		fmt.Println()
		for i, n := range d.Notes {
			if n.System {
				continue
			}

			// Build the prefix showing discussion ID, note ID, and optional position
			var prefix string
			if i == 0 {
				// First note in thread - show full discussion ID (needed for replies)
				prefix = dimColor.Sprintf("[d:%s] ", d.ID)
			} else {
				// Reply - show thread indent
				prefix = "    └─ "
			}

			// Note ID
			noteIDStr := dimColor.Sprintf("[n:%d] ", n.ID)

			// Position info for inline comments
			posStr := ""
			if n.Position != nil {
				path := n.Position.NewPath
				if path == "" {
					path = n.Position.OldPath
				}
				line := n.Position.NewLine
				if line == 0 {
					line = n.Position.OldLine
				}
				if path != "" && line > 0 {
					posStr = sectionColor.Sprintf("[%s:%d] ", path, line)
				}
			}

			// Resolved indicator
			resolvedStr := ""
			if n.Resolvable && n.Resolved {
				resolvedStr = mrMergedColor.Sprint("✓ ")
			}

			// Author and time
			authorTime := fmt.Sprintf("%s (%s):", n.Author, timeAgo(n.CreatedAt))

			// Print header line
			if i == 0 {
				fmt.Printf("    %s%s%s%s", prefix, noteIDStr, posStr, resolvedStr)
				labelColor.Printf("%s\n", authorTime)
			} else {
				fmt.Printf("    %s%s%s", prefix, noteIDStr, resolvedStr)
				labelColor.Printf("%s\n", authorTime)
			}

			// Render and print body
			rendered := renderMarkdown(n.Body)
			indent := "    "
			if i > 0 {
				indent = "       " // extra indent for replies
			}
			for _, line := range strings.Split(rendered, "\n") {
				fmt.Printf("%s%s\n", indent, line)
			}
		}
	}
}

// printMRFile displays a single file change with optional diff
func printMRFile(f models.MRFile) {
	var prefix, path string
	switch {
	case f.IsNew:
		prefix = color.GreenString("A")
		path = f.NewPath
	case f.IsDeleted:
		prefix = color.RedString("D")
		path = f.OldPath
	case f.IsRenamed:
		prefix = color.YellowString("R")
		path = fmt.Sprintf("%s → %s", f.OldPath, f.NewPath)
	default:
		prefix = color.CyanString("M")
		path = f.NewPath
	}

	fmt.Printf("    %s %s\n", prefix, path)

	// Show diff if available
	if f.Diff != "" {
		printDiff(f.Diff)
	}
}

// printDiff renders a unified diff with syntax highlighting and line numbers
func printDiff(diff string) {
	addColor := color.New(color.FgGreen)
	delColor := color.New(color.FgRed)
	hunkColor := color.New(color.FgCyan)
	lineNumColor := color.New(color.FgHiBlack)

	// Collect hunk headers from original diff
	var hunkHeaders []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			hunkHeaders = append(hunkHeaders, line)
		}
	}

	parsed := gl.ParseUnifiedDiff(diff)

	// Track which hunk we're in by detecting line number jumps
	hunkIdx := 0
	lastNewLine := 0
	printedFirstHunk := false

	for _, line := range parsed.Lines {
		// Detect new hunk: line number jumped significantly or first line
		currentNewLine := line.NewLine
		if line.Type == gl.LineDeleted {
			currentNewLine = line.OldLine // Use old line for deleted lines
		}

		if !printedFirstHunk || (lastNewLine > 0 && currentNewLine > 0 && currentNewLine > lastNewLine+1) {
			// Print hunk header if we have one
			if hunkIdx < len(hunkHeaders) {
				hunkColor.Printf("      %s\n", hunkHeaders[hunkIdx])
				hunkIdx++
			}
			printedFirstHunk = true
		}

		if line.NewLine > 0 {
			lastNewLine = line.NewLine
		}

		// Format: "  123 +content" or "      -content" (deleted lines have no new line number)
		var lineNum string
		if line.NewLine > 0 {
			lineNum = fmt.Sprintf("%4d", line.NewLine)
		} else {
			lineNum = "    "
		}

		prefix := " "
		content := line.Content
		printFn := fmt.Printf

		switch line.Type {
		case gl.LineAdded:
			prefix = "+"
			printFn = func(format string, a ...interface{}) (int, error) {
				addColor.Printf(format, a...)
				return 0, nil
			}
		case gl.LineDeleted:
			prefix = "-"
			printFn = func(format string, a ...interface{}) (int, error) {
				delColor.Printf(format, a...)
				return 0, nil
			}
		}

		lineNumColor.Printf("      %s ", lineNum)
		printFn("%s %s\n", prefix, content)
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
