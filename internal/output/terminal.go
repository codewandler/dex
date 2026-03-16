package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	gl "github.com/codewandler/dex/internal/gitlab"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
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

func PrintProject(activity gl.ProjectActivity) {
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

func PrintSummary(summary gl.ActivitySummary) {
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

// PrintMergeRequestDetails displays full MR information

// PrintMergeRequestDiscussions displays discussion threads with IDs for easy reference

// PrintParsedDiff displays a diff with explicit line number columns
func PrintParsedDiff(filePath string, diff *gl.ParsedDiff) {
	addColor := color.New(color.FgGreen)
	delColor := color.New(color.FgRed)
	ctxColor := color.New(color.FgWhite)
	hdrColor := color.New(color.FgCyan)

	// Header
	fmt.Printf("%s\n\n", filePath)
	hdrColor.Printf("  %5s  %5s  %-4s  %s\n", "new", "old", "type", "content")
	hdrColor.Printf("  %5s  %5s  %-4s  %s\n", "---", "---", "----", strings.Repeat("-", 50))

	for _, line := range diff.Lines {
		// Format line numbers
		newNum := "    -"
		oldNum := "    -"
		if line.NewLine > 0 {
			newNum = fmt.Sprintf("%5d", line.NewLine)
		}
		if line.OldLine > 0 {
			oldNum = fmt.Sprintf("%5d", line.OldLine)
		}

		// Determine type label and color
		var typeLabel string
		var printFn func(format string, a ...interface{})

		switch line.Type {
		case gl.LineAdded:
			typeLabel = "add"
			printFn = func(format string, a ...interface{}) {
				addColor.Printf(format, a...)
			}
		case gl.LineDeleted:
			typeLabel = "del"
			printFn = func(format string, a ...interface{}) {
				delColor.Printf(format, a...)
			}
		default:
			typeLabel = "ctx"
			printFn = func(format string, a ...interface{}) {
				ctxColor.Printf(format, a...)
			}
		}

		printFn("  %s  %s  %-4s  %s\n", newNum, oldNum, typeLabel, line.Content)
	}

	fmt.Println()
}

// PrintLineWithContext displays a specific line with surrounding context
func PrintLineWithContext(filePath string, diff *gl.ParsedDiff, lineNum int, contextLines int) {
	target, before, after := diff.GetLineWithContext(lineNum, contextLines)

	if target == nil {
		fmt.Printf("Line %d not found in diff for %s\n", lineNum, filePath)
		fmt.Println("\nAvailable line ranges in this diff:")
		printLineRanges(diff)
		return
	}

	hdrColor := color.New(color.FgCyan)
	addColor := color.New(color.FgGreen)
	delColor := color.New(color.FgRed)
	ctxColor := color.New(color.FgWhite)
	highlightColor := color.New(color.BgYellow, color.FgBlack)

	fmt.Printf("%s:%d\n\n", filePath, lineNum)

	// Print context before
	for _, line := range before {
		printDiffLineWithHighlight(&line, false, addColor, delColor, ctxColor, highlightColor)
	}

	// Print target line (highlighted)
	printDiffLineWithHighlight(target, true, addColor, delColor, ctxColor, highlightColor)

	// Print context after
	for _, line := range after {
		printDiffLineWithHighlight(&line, false, addColor, delColor, ctxColor, highlightColor)
	}

	// Print line info summary
	fmt.Println()
	hdrColor.Println("Line Info:")
	fmt.Printf("  Type:     %s\n", target.Type.String())
	fmt.Printf("  New line: %d\n", target.NewLine)
	fmt.Printf("  Old line: %d\n", target.OldLine)

	// Explain what this means for inline comments
	fmt.Println()
	hdrColor.Println("For inline comments:")
	switch target.Type {
	case gl.LineAdded:
		fmt.Printf("  Use --line %d (added line, only has new_line)\n", target.NewLine)
	case gl.LineDeleted:
		fmt.Printf("  Cannot comment on deleted lines (line only exists in old version)\n")
	case gl.LineContext:
		fmt.Printf("  Use --line %d (context line, has both old_line=%d and new_line=%d)\n",
			target.NewLine, target.OldLine, target.NewLine)
	}
}

// printDiffLineWithHighlight prints a single diff line with optional highlighting
func printDiffLineWithHighlight(line *gl.DiffLine, highlight bool, addColor, delColor, ctxColor, highlightColor *color.Color) {
	// Format line numbers
	newNum := "    -"
	oldNum := "    -"
	if line.NewLine > 0 {
		newNum = fmt.Sprintf("%5d", line.NewLine)
	}
	if line.OldLine > 0 {
		oldNum = fmt.Sprintf("%5d", line.OldLine)
	}

	prefix := " "
	var printFn func(format string, a ...interface{})

	switch line.Type {
	case gl.LineAdded:
		prefix = "+"
		printFn = func(format string, a ...interface{}) { addColor.Printf(format, a...) }
	case gl.LineDeleted:
		prefix = "-"
		printFn = func(format string, a ...interface{}) { delColor.Printf(format, a...) }
	default:
		printFn = func(format string, a ...interface{}) { ctxColor.Printf(format, a...) }
	}

	if highlight {
		highlightColor.Printf("→ %s  %s  %s  %s %s\n", newNum, oldNum, line.Type.String(), prefix, line.Content)
	} else {
		printFn("  %s  %s  %s  %s %s\n", newNum, oldNum, line.Type.String(), prefix, line.Content)
	}
}

// printLineRanges shows what line ranges are available in the diff
func printLineRanges(diff *gl.ParsedDiff) {
	if len(diff.Lines) == 0 {
		fmt.Println("  (no lines in diff)")
		return
	}

	// Find continuous ranges
	type lineRange struct {
		start, end int
	}
	var ranges []lineRange
	var current *lineRange

	for _, line := range diff.Lines {
		if line.NewLine == 0 {
			continue // Skip deleted lines (no new line number)
		}
		if current == nil {
			current = &lineRange{start: line.NewLine, end: line.NewLine}
		} else if line.NewLine == current.end+1 {
			current.end = line.NewLine
		} else {
			ranges = append(ranges, *current)
			current = &lineRange{start: line.NewLine, end: line.NewLine}
		}
	}
	if current != nil {
		ranges = append(ranges, *current)
	}

	for _, r := range ranges {
		if r.start == r.end {
			fmt.Printf("  Line %d\n", r.start)
		} else {
			fmt.Printf("  Lines %d-%d\n", r.start, r.end)
		}
	}
}

// PrintInlineCommentDryRun previews where an inline comment will land
func PrintInlineCommentDryRun(client *gl.Client, projectID string, mrIID int, filePath string, lineNum int, message string) {
	hdrColor := color.New(color.FgCyan, color.Bold)
	warnColor := color.New(color.FgYellow)
	errColor := color.New(color.FgRed)
	okColor := color.New(color.FgGreen)

	fmt.Println()
	hdrColor.Println("Dry Run: Inline Comment Preview")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()

	// Try to get the parsed diff for the file
	diff, err := client.GetParsedDiffForFile(projectID, mrIID, filePath)
	if err != nil {
		errColor.Printf("✗ File not found in diff: %s\n", filePath)
		fmt.Println()
		fmt.Println("The file must be changed in this MR to add inline comments.")

		// List available files
		files, err := client.GetMergeRequestChanges(projectID, mrIID, false)
		if err == nil && len(files) > 0 {
			fmt.Println()
			fmt.Println("Changed files in this MR:")
			for _, f := range files {
				fmt.Printf("  • %s\n", f.NewPath)
			}
		}
		os.Exit(1)
	}

	// Look for the line in the diff
	line, found := diff.FindLineByNew(lineNum)
	if !found {
		errColor.Printf("✗ Line %d is not in the diff for %s\n", lineNum, filePath)
		fmt.Println()
		fmt.Println("You can only comment on lines that appear in the diff.")
		fmt.Println()
		printLineRanges(diff)
		os.Exit(1)
	}

	// Found the line - show what it looks like
	okColor.Printf("✓ Line %d found in diff\n", lineNum)
	fmt.Println()

	fmt.Printf("  File:     %s\n", filePath)
	fmt.Printf("  Line:     %d\n", lineNum)
	fmt.Printf("  Type:     %s\n", line.Type.String())

	if line.OldLine > 0 {
		fmt.Printf("  Old line: %d\n", line.OldLine)
	}
	fmt.Println()

	// Show the target line with context
	fmt.Println("Target line:")
	target, before, after := diff.GetLineWithContext(lineNum, 2)
	if target != nil {
		addColor := color.New(color.FgGreen)
		delColor := color.New(color.FgRed)
		ctxColor := color.New(color.FgWhite)
		highlightColor := color.New(color.BgYellow, color.FgBlack)

		for _, l := range before {
			printDiffLineWithHighlight(&l, false, addColor, delColor, ctxColor, highlightColor)
		}
		printDiffLineWithHighlight(target, true, addColor, delColor, ctxColor, highlightColor)
		for _, l := range after {
			printDiffLineWithHighlight(&l, false, addColor, delColor, ctxColor, highlightColor)
		}
	}
	fmt.Println()

	// Check for potential issues
	if line.Type == gl.LineDeleted {
		warnColor.Println("⚠ Warning: This is a deleted line. Comments on deleted lines")
		warnColor.Println("  may not display as expected in the GitLab UI.")
		fmt.Println()
	}

	if line.Content == "" {
		warnColor.Println("⚠ Warning: This is an empty line. Consider commenting on a")
		warnColor.Println("  nearby line with actual content.")
		fmt.Println()
	}

	// Show the message preview
	hdrColor.Println("Comment message:")
	fmt.Printf("  %s\n", message)
	fmt.Println()

	okColor.Println("Run without --dry-run to post this comment.")
}

// PrintInlineCommentError provides a helpful error message when inline comment fails
func PrintInlineCommentError(client *gl.Client, projectID string, mrIID int, filePath string, lineNum int, err error) {
	errColor := color.New(color.FgRed)
	hdrColor := color.New(color.FgCyan)

	errColor.Fprintf(os.Stderr, "Failed to add inline comment: %v\n", err)
	fmt.Fprintln(os.Stderr)

	// Try to diagnose the issue
	diff, diffErr := client.GetParsedDiffForFile(projectID, mrIID, filePath)
	if diffErr != nil {
		hdrColor.Fprintln(os.Stderr, "Possible cause: File not found in diff")
		fmt.Fprintf(os.Stderr, "The file '%s' does not appear in the MR changes.\n", filePath)
		fmt.Fprintln(os.Stderr)

		// List available files
		files, listErr := client.GetMergeRequestChanges(projectID, mrIID, false)
		if listErr == nil && len(files) > 0 {
			fmt.Fprintln(os.Stderr, "Available files:")
			for _, f := range files {
				fmt.Fprintf(os.Stderr, "  • %s\n", f.NewPath)
			}
		}
		return
	}

	line, found := diff.FindLineByNew(lineNum)
	if !found {
		hdrColor.Fprintln(os.Stderr, "Possible cause: Line not in diff")
		fmt.Fprintf(os.Stderr, "Line %d is not part of the diff for '%s'.\n", lineNum, filePath)
		fmt.Fprintln(os.Stderr, "You can only comment on lines that appear in the diff.")
		fmt.Fprintln(os.Stderr)

		fmt.Fprintln(os.Stderr, "Available line ranges:")
		printLineRangesToStderr(diff)
		return
	}

	if line.Type == gl.LineDeleted {
		hdrColor.Fprintln(os.Stderr, "Possible cause: Commenting on deleted line")
		fmt.Fprintln(os.Stderr, "Line", lineNum, "is a deleted line, which may require special handling.")
		fmt.Fprintln(os.Stderr)
	}

	// Generic advice
	hdrColor.Fprintln(os.Stderr, "Troubleshooting:")
	fmt.Fprintln(os.Stderr, "  1. Use 'dex gl mr comment ... --dry-run' to preview the comment location")
	fmt.Fprintln(os.Stderr, "  2. Use 'dex gl mr diff ... -f <file> -l <line>' to inspect the line")
	fmt.Fprintln(os.Stderr, "  3. Ensure the line number is from the 'new' version (right side of diff)")
}

// printLineRangesToStderr is like printLineRanges but writes to stderr
func printLineRangesToStderr(diff *gl.ParsedDiff) {
	if len(diff.Lines) == 0 {
		fmt.Fprintln(os.Stderr, "  (no lines in diff)")
		return
	}

	type lineRange struct {
		start, end int
	}
	var ranges []lineRange
	var current *lineRange

	for _, line := range diff.Lines {
		if line.NewLine == 0 {
			continue
		}
		if current == nil {
			current = &lineRange{start: line.NewLine, end: line.NewLine}
		} else if line.NewLine == current.end+1 {
			current.end = line.NewLine
		} else {
			ranges = append(ranges, *current)
			current = &lineRange{start: line.NewLine, end: line.NewLine}
		}
	}
	if current != nil {
		ranges = append(ranges, *current)
	}

	for _, r := range ranges {
		if r.start == r.end {
			fmt.Fprintf(os.Stderr, "  Line %d\n", r.start)
		} else {
			fmt.Fprintf(os.Stderr, "  Lines %d-%d\n", r.start, r.end)
		}
	}
}

// PrintSearchResults displays lines matching a search pattern
func PrintSearchResults(filePath string, diff *gl.ParsedDiff, pattern string) {
	matches, err := diff.SearchLines(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid regex pattern: %v\n", err)
		os.Exit(1)
	}

	if len(matches) == 0 {
		fmt.Printf("No lines matching %q in %s\n", pattern, filePath)
		return
	}

	hdrColor := color.New(color.FgCyan)
	addColor := color.New(color.FgGreen)
	delColor := color.New(color.FgRed)
	ctxColor := color.New(color.FgWhite)

	fmt.Printf("%s: %d matches for %q\n\n", filePath, len(matches), pattern)
	hdrColor.Printf("  %5s  %5s  %-4s  %s\n", "new", "old", "type", "content")
	hdrColor.Printf("  %5s  %5s  %-4s  %s\n", "---", "---", "----", strings.Repeat("-", 50))

	for _, line := range matches {
		newNum := "    -"
		oldNum := "    -"
		if line.NewLine > 0 {
			newNum = fmt.Sprintf("%5d", line.NewLine)
		}
		if line.OldLine > 0 {
			oldNum = fmt.Sprintf("%5d", line.OldLine)
		}

		var printFn func(format string, a ...interface{})
		switch line.Type {
		case gl.LineAdded:
			printFn = func(format string, a ...interface{}) { addColor.Printf(format, a...) }
		case gl.LineDeleted:
			printFn = func(format string, a ...interface{}) { delColor.Printf(format, a...) }
		default:
			printFn = func(format string, a ...interface{}) { ctxColor.Printf(format, a...) }
		}

		printFn("  %s  %s  %-4s  %s\n", newNum, oldNum, line.Type.String(), line.Content)
	}
	fmt.Println()
}

// formatPipelineStatus returns a color-coded status string for pipelines/jobs
func formatPipelineStatus(status string) string {
	switch status {
	case "success", "passed":
		return mrMergedColor.Sprintf("%-10s", status)
	case "failed":
		return mrClosedColor.Sprintf("%-10s", status)
	case "running":
		return mrOpenColor.Sprintf("%-10s", status)
	case "pending":
		return projectColor.Sprintf("%-10s", status)
	case "canceled", "skipped":
		return dimColor.Sprintf("%-10s", status)
	case "manual":
		return labelColor.Sprintf("%-10s", status)
	case "created":
		return commitColor.Sprintf("%-10s", status)
	default:
		return fmt.Sprintf("%-10s", status)
	}
}

// formatDurationSecs formats a duration given in seconds as a human-readable string
func formatDurationSecs(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60)
}

// formatDurationSecsFloat formats a float64 duration in seconds
func formatDurationSecsFloat(seconds float64) string {
	return formatDurationSecs(int(seconds))
}





