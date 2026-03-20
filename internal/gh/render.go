package gh

import (
	"fmt"
	"strings"

	"github.com/codewandler/dex/internal/render"
)

// ── IssueListResult ──────────────────────────────────────────────────────────

// RenderText implements render.Renderable on IssueListResult.
// ModeNormal: header with total count, then one detailed line per issue,
//
//	followed by a "next page" hint when there are more results.
//
// ModeCompact: one compact line per issue, no header (good for piping/agents).
func (r *IssueListResult) RenderText(mode render.Mode) string {
	var b strings.Builder

	if len(r.Issues) == 0 {
		return "No issues found.\n"
	}

	if mode == render.ModeNormal {
		if r.TotalCount > 0 {
			fmt.Fprintf(&b, "Issues (%d total, showing %d):\n\n", r.TotalCount, len(r.Issues))
		} else {
			fmt.Fprintf(&b, "Issues (%d):\n\n", len(r.Issues))
		}
	}

	for _, issue := range r.Issues {
		date := ""
		if len(issue.CreatedAt) >= 10 {
			date = issue.CreatedAt[:10]
		}
		if mode == render.ModeCompact {
			title := issue.Title
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			fmt.Fprintf(&b, "#%-5d %s\n", issue.Number, title)
		} else {
			state := strings.ToLower(issue.State)
			labelStr := ""
			if len(issue.Labels) > 0 {
				labelStr = "  [" + strings.Join(issue.Labels, ", ") + "]"
			}
			fmt.Fprintf(&b, "#%-5d %-6s  %s  @%-20s  %s%s\n",
				issue.Number, state, date, issue.Author, issue.Title, labelStr)
		}
	}

	if r.HasMore && r.NextCursor != "" && mode == render.ModeNormal {
		fmt.Fprintf(&b, "\nMore results available. Next page: --after %s\n", r.NextCursor)
	}

	return b.String()
}

// ── IssueResult ──────────────────────────────────────────────────────────────

// IssueResult wraps a single Issue for Renderable output.
type IssueResult struct {
	*Issue
}

// RenderText implements render.Renderable on IssueResult.
// ModeNormal: full multi-line detail view.
// ModeCompact: single summary line.
func (r *IssueResult) RenderText(mode render.Mode) string {
	if r.Issue == nil {
		return "Issue not found.\n"
	}

	if mode == render.ModeCompact {
		return fmt.Sprintf("#%d [%s] %s (@%s)\n",
			r.Number, strings.ToLower(r.State), r.Title, r.Author)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "#%d %s\n", r.Number, r.Title)

	date := r.CreatedAt
	if len(date) >= 10 {
		date = date[:10]
	}
	fmt.Fprintf(&b, "State: %s | Author: @%s | Created: %s\n",
		strings.ToLower(r.State), r.Author, date)

	if len(r.Labels) > 0 {
		fmt.Fprintf(&b, "Labels: %s\n", strings.Join(r.Labels, ", "))
	}
	if len(r.Assignees) > 0 {
		fmt.Fprintf(&b, "Assignees: %s\n", strings.Join(r.Assignees, ", "))
	}
	if r.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", r.URL)
	}
	if r.Body != "" {
		fmt.Fprintf(&b, "\n%s\n", r.Body)
	}

	return b.String()
}

// ── ReleaseListResult ────────────────────────────────────────────────────────

// ReleaseListResult wraps a slice of releases for Renderable output.
type ReleaseListResult struct {
	Releases []Release `json:"releases"`
}

// RenderText implements render.Renderable on ReleaseListResult.
// ModeNormal: table with tag, date, name, and status flags.
// ModeCompact: one line per release — tag, date, name only.
func (r *ReleaseListResult) RenderText(mode render.Mode) string {
	if len(r.Releases) == 0 {
		return "No releases found.\n"
	}

	var b strings.Builder
	for _, rel := range r.Releases {
		name := rel.Name
		if name == "" {
			name = rel.TagName
		}
		date := ""
		if len(rel.PublishedAt) >= 10 {
			date = rel.PublishedAt[:10]
		}

		if mode == render.ModeCompact {
			fmt.Fprintf(&b, "%-12s  %s  %s\n", rel.TagName, date, name)
		} else {
			flags := ""
			if rel.IsLatest {
				flags = " (latest)"
			} else if rel.IsDraft {
				flags = " (draft)"
			} else if rel.IsPrerelease {
				flags = " (prerelease)"
			}
			fmt.Fprintf(&b, "%-12s  %s  %s%s\n", rel.TagName, date, name, flags)
		}
	}

	return b.String()
}

// ── ReleaseResult ─────────────────────────────────────────────────────────────

// ReleaseResult wraps a single Release for Renderable output.
type ReleaseResult struct {
	*Release
}

// RenderText implements render.Renderable on ReleaseResult.
// ModeNormal: full multi-line detail view.
// ModeCompact: single summary line.
func (r *ReleaseResult) RenderText(mode render.Mode) string {
	if r.Release == nil {
		return "Release not found.\n"
	}

	name := r.Name
	if name == "" {
		name = r.TagName
	}
	date := r.PublishedAt
	if len(date) >= 10 {
		date = date[:10]
	}

	if mode == render.ModeCompact {
		status := "published"
		if r.IsDraft {
			status = "draft"
		} else if r.IsPrerelease {
			status = "prerelease"
		}
		return fmt.Sprintf("%s  %s  %s  %s\n", r.TagName, status, date, r.URL)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s - %s\n", r.TagName, name)

	status := "published"
	if r.IsDraft {
		status = "draft"
	} else if r.IsPrerelease {
		status = "prerelease"
	}
	fmt.Fprintf(&b, "Status: %s | Author: @%s | Published: %s\n", status, r.Author, date)
	fmt.Fprintf(&b, "URL: %s\n", r.URL)
	if r.Body != "" {
		fmt.Fprintf(&b, "\n%s\n", r.Body)
	}

	return b.String()
}

// ── LabelListResult ──────────────────────────────────────────────────────────

// LabelListResult wraps a slice of labels for Renderable output.
type LabelListResult struct {
	Labels []Label `json:"labels"`
}

// RenderText implements render.Renderable on LabelListResult.
// ModeNormal: name, color, and description table.
// ModeCompact: label names only, one per line.
func (r *LabelListResult) RenderText(mode render.Mode) string {
	if len(r.Labels) == 0 {
		return "No labels found.\n"
	}

	var b strings.Builder
	for _, label := range r.Labels {
		if mode == render.ModeCompact {
			fmt.Fprintf(&b, "%s\n", label.Name)
		} else {
			desc := ""
			if label.Description != "" {
				desc = fmt.Sprintf(" - %s", label.Description)
			}
			fmt.Fprintf(&b, "%-30s #%s%s\n", label.Name, label.Color, desc)
		}
	}

	return b.String()
}

