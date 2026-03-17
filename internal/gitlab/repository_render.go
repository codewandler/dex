package gitlab

import (
	"fmt"
	"strings"

	"github.com/codewandler/dex/internal/render"
	"github.com/fatih/color"
)

var (
	glAddColor  = color.New(color.FgGreen)
	glDelColor  = color.New(color.FgRed)
	glHunkColor = color.New(color.FgCyan)
)

// ── FileResult ────────────────────────────────────────────────────────────────

func (r *FileResult) RenderText(mode render.Mode) string {
	// Normal mode: print raw content
	// Compact mode: print content with a brief metadata header
	if mode == render.ModeCompact {
		var sb strings.Builder
		glDimColor.Fprintf(&sb, "# %s @ %s  (%d bytes, commit %s)\n",
			r.FilePath, r.Ref, r.Size, shortID(r.LastCommitID))
		sb.WriteString(r.Content)
		if !strings.HasSuffix(r.Content, "\n") {
			sb.WriteByte('\n')
		}
		return sb.String()
	}
	// Normal: just raw content, no decoration — most useful for piping
	out := r.Content
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

// ── FileMetaResult ────────────────────────────────────────────────────────────

func (r *FileMetaResult) RenderText(mode render.Mode) string {
	var sb strings.Builder

	if mode == render.ModeCompact {
		glCommitColor.Fprintf(&sb, "%s", r.FilePath)
		glDimColor.Fprintf(&sb, "  @ %s  %d bytes  commit %s\n",
			r.Ref, r.Size, shortID(r.LastCommitID))
		return sb.String()
	}

	line := strings.Repeat("─", 60)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glProjectColor.Fprintf(&sb, "  %s\n", r.FilePath)
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	glPrintField(&sb, "Ref", r.Ref)
	glPrintField(&sb, "Size", fmt.Sprintf("%d bytes", r.Size))
	glPrintField(&sb, "Encoding", r.Encoding)
	glPrintField(&sb, "Blob ID", r.BlobID)
	glPrintField(&sb, "Last Commit", r.LastCommitID)
	glPrintField(&sb, "SHA256", r.SHA256)
	fmt.Fprintln(&sb)
	return sb.String()
}

// ── FileBlameResult ───────────────────────────────────────────────────────────

func (r *FileBlameResult) RenderText(mode render.Mode) string {
	if len(r.Ranges) == 0 {
		return glDimColor.Sprint("No blame data found.\n")
	}

	var sb strings.Builder
	lineNum := 1

	for _, br := range r.Ranges {
		commitInfo := fmt.Sprintf("%s (%s)", br.CommitShortID, br.AuthorName)
		if mode != render.ModeCompact && !br.AuthoredDate.IsZero() {
			commitInfo += fmt.Sprintf(" %s", br.AuthoredDate.Format("2006-01-02"))
		}
		for i, line := range br.Lines {
			if i == 0 {
				glCommitColor.Fprintf(&sb, "%-30s ", commitInfo)
			} else {
				fmt.Fprintf(&sb, "%-30s ", strings.Repeat(" ", len(commitInfo)))
			}
			glDimColor.Fprintf(&sb, "%4d ", lineNum)
			fmt.Fprintln(&sb, line)
			lineNum++
		}
	}
	return sb.String()
}

// ── TreeResult ────────────────────────────────────────────────────────────────

func (r *TreeResult) RenderText(mode render.Mode) string {
	if len(r.Nodes) == 0 {
		return glDimColor.Sprint("No files found.\n")
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, n := range r.Nodes {
			fmt.Fprintln(&sb, n.Path)
		}
		return sb.String()
	}

	ref := r.Ref
	if ref == "" {
		ref = "HEAD"
	}
	path := r.Path
	if path == "" {
		path = "/"
	}

	fmt.Fprintln(&sb)
	glHeaderColor.Fprintf(&sb, "  %s @ %s  (%d entries)\n", path, ref, r.Total)
	fmt.Fprintln(&sb)

	for _, n := range r.Nodes {
		switch n.Type {
		case "tree":
			glSectionColor.Fprintf(&sb, "  📁 %s/\n", n.Path)
		default:
			glValueColor.Fprintf(&sb, "  📄 %s\n", n.Path)
		}
	}
	fmt.Fprintln(&sb)
	return sb.String()
}

// ── CompareResult ─────────────────────────────────────────────────────────────

func (r *CompareResult) RenderText(mode render.Mode) string {
	var sb strings.Builder

	// Tally additions/deletions
	additions, deletions := 0, 0
	for _, d := range r.Diffs {
		additions += d.Additions
		deletions += d.Deletions
	}

	dotStr := "..."
	if r.Straight {
		dotStr = ".."
	}
	scopeStr := ""
	if r.Path != "" {
		scopeStr = fmt.Sprintf("  [%s]", r.Path)
	}
	title := fmt.Sprintf("%s%s%s%s  (%d commits, %d files changed, +%d/-%d)",
		r.From, dotStr, r.To, scopeStr, len(r.Commits), len(r.Diffs), additions, deletions)

	line := strings.Repeat("═", 80)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glProjectColor.Fprintf(&sb, "  %s\n", title)
	glHeaderColor.Fprintln(&sb, line)

	if r.Timeout {
		fmt.Fprintln(&sb)
		glMRClosedColor.Fprint(&sb, "  ⚠ Compare timeout: diff may be incomplete\n")
	}

	// Commits section (always shown, capped at 20 without --path)
	if len(r.Commits) > 0 {
		fmt.Fprintln(&sb)
		cap := len(r.Commits)
		capped := r.Path == "" && cap > 20
		if capped {
			cap = 20
		}
		glSectionColor.Fprintf(&sb, "  Commits (%d):\n", len(r.Commits))
		for _, c := range r.Commits[:cap] {
			glCommitColor.Fprintf(&sb, "    %s ", c.ShortID)
			fmt.Fprintf(&sb, "%-55s ", glTruncate(c.Title, 55))
			glDimColor.Fprintf(&sb, "(%s)\n", c.AuthorName)
		}
		if capped {
			glDimColor.Fprintf(&sb, "    … and %d more commits\n", len(r.Commits)-20)
		}
	}

	// Files changed — always shown
	if len(r.Diffs) > 0 {
		fmt.Fprintln(&sb)
		glSectionColor.Fprintf(&sb, "  Files Changed (%d):\n", len(r.Diffs))
		for _, d := range r.Diffs {
			renderRepoDiff(&sb, d)
		}
	}

	// Diff content — only when scoped to a path
	if r.Path != "" && len(r.Diffs) > 0 && mode != render.ModeCompact {
		fmt.Fprintln(&sb)
		glSectionColor.Fprint(&sb, "  Diff:\n")
		fmt.Fprintln(&sb)
		for _, d := range r.Diffs {
			renderDiffContent(&sb, d)
		}
	}

	// Hint when no --path was given
	if r.Path == "" && len(r.Diffs) > 0 {
		fmt.Fprintln(&sb)
		glDimColor.Fprintf(&sb, "  ℹ  Use --path <file|dir> to show diff content.\n")
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

// renderRepoDiff renders a single file entry in the file-changed summary.
func renderRepoDiff(sb *strings.Builder, d RepoDiff) {
	var prefix string
	var path string
	switch {
	case d.IsNew:
		prefix = glAddColor.Sprint("  A  ")
		path = d.NewPath
	case d.IsDeleted:
		prefix = glDelColor.Sprint("  D  ")
		path = d.OldPath
	case d.IsRenamed:
		prefix = glSectionColor.Sprint("  R  ")
		path = fmt.Sprintf("%s → %s", d.OldPath, d.NewPath)
	default:
		prefix = glValueColor.Sprint("  M  ")
		path = d.NewPath
	}
	fmt.Fprintf(sb, "    %s%s", prefix, path)
	if d.Additions > 0 || d.Deletions > 0 {
		glDimColor.Fprintf(sb, " (+%d/-%d)", d.Additions, d.Deletions)
	}
	fmt.Fprintln(sb)
}

// renderDiffContent renders the unified diff lines for a single file.
func renderDiffContent(sb *strings.Builder, d RepoDiff) {
	if d.Diff == "" {
		return
	}
	// File header
	glLabelColor.Fprintf(sb, "  --- a/%s\n", d.OldPath)
	glLabelColor.Fprintf(sb, "  +++ b/%s\n", d.NewPath)
	for _, line := range strings.Split(d.Diff, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "@@"):
			glHunkColor.Fprintf(sb, "  %s\n", line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			glAddColor.Fprintf(sb, "  %s\n", line)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			glDelColor.Fprintf(sb, "  %s\n", line)
		default:
			fmt.Fprintf(sb, "  %s\n", line)
		}
	}
}

// ── BlobSearchResult ──────────────────────────────────────────────────────────

func (r *BlobSearchResult) RenderText(mode render.Mode) string {
	if len(r.Matches) == 0 {
		return glDimColor.Sprintf("No results for %q\n", r.Query)
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, m := range r.Matches {
			glCommitColor.Fprintf(&sb, "%s", m.Path)
			if m.Ref != "" {
				glDimColor.Fprintf(&sb, " @%s", m.Ref)
			}
			if m.StartLine > 0 {
				glDimColor.Fprintf(&sb, ":%d", m.StartLine)
			}
			fmt.Fprintf(&sb, "  %s\n", glTruncate(strings.TrimSpace(m.Data), 80))
		}
		return sb.String()
	}

	line := strings.Repeat("═", 80)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glHeaderColor.Fprintf(&sb, "  Blob search: %q  (%d results)\n", r.Query, r.Total)
	glHeaderColor.Fprintln(&sb, line)

	for _, m := range r.Matches {
		fmt.Fprintln(&sb)
		glProjectColor.Fprintf(&sb, "  %s", m.Path)
		if m.Ref != "" {
			glDimColor.Fprintf(&sb, " @ %s", m.Ref)
		}
		if m.StartLine > 0 {
			glDimColor.Fprintf(&sb, "  line %d", m.StartLine)
		}
		fmt.Fprintln(&sb)
		for _, l := range strings.Split(strings.TrimSpace(m.Data), "\n") {
			glDimColor.Fprintf(&sb, "    %s\n", l)
		}
	}
	fmt.Fprintln(&sb)
	return sb.String()
}

// shortID returns the first 8 chars of a commit SHA (or the full string if shorter).
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
