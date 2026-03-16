package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/render"
	"github.com/fatih/color"
	gogitlab "github.com/xanzy/go-gitlab"
)

// ── Color palette (mirrors internal/output/terminal.go) ──────────────────────

var (
	glHeaderColor   = color.New(color.FgCyan, color.Bold)
	glProjectColor  = color.New(color.FgYellow, color.Bold)
	glSectionColor  = color.New(color.FgGreen)
	glCommitColor   = color.New(color.FgWhite)
	glMROpenColor   = color.New(color.FgBlue)
	glMRMergedColor = color.New(color.FgGreen)
	glMRClosedColor = color.New(color.FgRed)
	glDimColor      = color.New(color.FgHiBlack)
	glLabelColor    = color.New(color.FgCyan)
	glValueColor    = color.New(color.FgWhite)
	glLangColor     = color.New(color.FgYellow)
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func glTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func glTimeAgo(t time.Time) string {
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

func glFormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04"), glTimeAgo(t))
}

func glPrintField(sb *strings.Builder, label, value string) {
	glLabelColor.Fprintf(sb, "  %-16s ", label+":")
	glValueColor.Fprintln(sb, value)
}

func glFormatMRState(state string) string {
	switch state {
	case "merged":
		return glMRMergedColor.Sprint("[MERGED]")
	case "opened":
		return glMROpenColor.Sprint("[OPEN]")
	case "closed":
		return glMRClosedColor.Sprint("[CLOSED]")
	default:
		return fmt.Sprintf("[%s]", strings.ToUpper(state))
	}
}

func glFormatPipelineStatus(status string) string {
	switch status {
	case "success", "passed":
		return glMRMergedColor.Sprintf("%-10s", status)
	case "failed":
		return glMRClosedColor.Sprintf("%-10s", status)
	case "running":
		return glMROpenColor.Sprintf("%-10s", status)
	case "pending":
		return glProjectColor.Sprintf("%-10s", status)
	case "canceled", "skipped":
		return glDimColor.Sprintf("%-10s", status)
	case "manual":
		return glLabelColor.Sprintf("%-10s", status)
	case "created":
		return glCommitColor.Sprintf("%-10s", status)
	default:
		return fmt.Sprintf("%-10s", status)
	}
}

func glFormatDurationSecs(seconds int) string {
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

func glFormatVisibility(visibility string) string {
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

func glHyperlink(url, text string) string {
	if url == "" {
		return text
	}
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// ── MRListResult ─────────────────────────────────────────────────────────────

// MRListResult holds a list of merge requests for display.
type MRListResult struct {
	MRs   []MergeRequestDetail `json:"merge_requests"`
	Total int                  `json:"total"`
}

func (r *MRListResult) RenderText(mode render.Mode) string {
	if len(r.MRs) == 0 {
		return glDimColor.Sprint("No merge requests found.\n")
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, mr := range r.MRs {
			state := glFormatMRState(mr.State)
			draft := ""
			if mr.Draft {
				draft = glDimColor.Sprint("[DRAFT] ")
			}
			conflicts := ""
			if mr.HasConflicts {
				conflicts = glMRClosedColor.Sprint(" ⚠")
			}
			fmt.Fprintf(&sb, "%s%s  %-20s  %s%s\n",
				draft, state,
				glTruncate(mr.ProjectPath+"!"+fmt.Sprint(mr.IID), 20),
				glTruncate(mr.Title, 60),
				conflicts,
			)
		}
		return sb.String()
	}

	// Normal mode
	line := strings.Repeat("═", 90)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glHeaderColor.Fprintf(&sb, "  Merge Requests (%d)\n", len(r.MRs))
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	for _, mr := range r.MRs {
		stateStr := glFormatMRState(mr.State)
		if mr.Draft {
			stateStr = glDimColor.Sprint("[DRAFT]") + " " + stateStr
		}
		glProjectColor.Fprintf(&sb, "  %s ", stateStr)
		fmt.Fprintf(&sb, "%s\n", glTruncate(mr.Title, 70))

		refLink := glHyperlink(mr.WebURL, mr.ProjectPath)
		fmt.Fprintf(&sb, "    %s  ", refLink)
		glSectionColor.Fprintf(&sb, "%s → %s", mr.SourceBranch, mr.TargetBranch)
		glDimColor.Fprintf(&sb, "  by %s  %s\n", mr.Author, glTimeAgo(mr.UpdatedAt))

		if mr.HasConflicts {
			glMRClosedColor.Fprint(&sb, "    ⚠ Has conflicts\n")
		}
		fmt.Fprintln(&sb)
	}

	return sb.String()
}

// ── MRDetailResult ────────────────────────────────────────────────────────────

// MRDetailResult holds full MR information for display.
type MRDetailResult struct {
	MergeRequestDetail
}

func (r *MRDetailResult) RenderText(mode render.Mode) string {
	mr := &r.MergeRequestDetail
	var sb strings.Builder

	line := strings.Repeat("═", 70)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)

	stateStr := glFormatMRState(mr.State)
	if mr.Draft {
		stateStr = glDimColor.Sprint("[DRAFT]") + " " + stateStr
	}
	glProjectColor.Fprintf(&sb, "  %s %s\n", stateStr, mr.Title)
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	glPrintField(&sb, "Reference", mr.ProjectPath)
	glPrintField(&sb, "URL", mr.WebURL)
	glPrintField(&sb, "Author", mr.Author)
	glPrintField(&sb, "Branches", fmt.Sprintf("%s → %s", mr.SourceBranch, mr.TargetBranch))
	glPrintField(&sb, "Created", glFormatTimestamp(mr.CreatedAt))
	glPrintField(&sb, "Updated", glFormatTimestamp(mr.UpdatedAt))

	if mr.MergedAt != nil {
		glPrintField(&sb, "Merged", glFormatTimestamp(*mr.MergedAt))
		if mr.MergedBy != "" {
			glPrintField(&sb, "Merged By", mr.MergedBy)
		}
	}
	if len(mr.Assignees) > 0 {
		glPrintField(&sb, "Assignees", strings.Join(mr.Assignees, ", "))
	}
	if len(mr.Reviewers) > 0 {
		glPrintField(&sb, "Reviewers", strings.Join(mr.Reviewers, ", "))
	}
	if len(mr.Labels) > 0 {
		glPrintField(&sb, "Labels", strings.Join(mr.Labels, ", "))
	}
	if mr.Changes.Files > 0 {
		glPrintField(&sb, "Changes", fmt.Sprintf("%d files", mr.Changes.Files))
	}
	if mr.HasConflicts {
		fmt.Fprintln(&sb)
		glMRClosedColor.Fprint(&sb, "  ⚠ This merge request has conflicts that must be resolved\n")
	}

	if mode == render.ModeCompact {
		// Compact: header only + counts
		if len(mr.Commits) > 0 {
			glPrintField(&sb, "Commits", fmt.Sprintf("%d", len(mr.Commits)))
		}
		if len(mr.Files) > 0 {
			glPrintField(&sb, "Files changed", fmt.Sprintf("%d", len(mr.Files)))
		}
		threadCount := 0
		for _, d := range mr.Discussions {
			if !d.IndividualNote {
				threadCount++
			}
		}
		if len(mr.Discussions) > 0 || len(mr.Notes) > 0 {
			glPrintField(&sb, "Discussions", fmt.Sprintf("%d", len(mr.Discussions)))
		}
		_ = threadCount
		fmt.Fprintln(&sb)
		return sb.String()
	}

	// Normal mode: full detail
	if mr.Description != "" {
		fmt.Fprintln(&sb)
		glSectionColor.Fprint(&sb, "  Description:\n")
		fmt.Fprintln(&sb)
		for _, l := range strings.Split(strings.TrimSpace(mr.Description), "\n") {
			fmt.Fprintf(&sb, "    %s\n", l)
		}
	}

	if len(mr.Commits) > 0 {
		fmt.Fprintln(&sb)
		glSectionColor.Fprintf(&sb, "  Commits (%d):\n", len(mr.Commits))
		for _, c := range mr.Commits {
			glCommitColor.Fprintf(&sb, "    %s ", c.ShortID)
			fmt.Fprintf(&sb, "%s ", glTruncate(c.Title, 50))
			glDimColor.Fprintf(&sb, "(%s)\n", c.Author)
		}
	}

	if len(mr.Files) > 0 {
		fmt.Fprintln(&sb)
		glSectionColor.Fprintf(&sb, "  Files Changed (%d):\n", len(mr.Files))
		for _, f := range mr.Files {
			renderMRFile(&sb, f)
		}
	}

	if len(mr.Discussions) > 0 {
		renderMRDiscussions(&sb, mr.Discussions)
	} else if len(mr.Notes) > 0 {
		var userNotes []MRNote
		for _, n := range mr.Notes {
			if !n.System {
				userNotes = append(userNotes, n)
			}
		}
		if len(userNotes) > 0 {
			fmt.Fprintln(&sb)
			glSectionColor.Fprintf(&sb, "  Comments (%d):\n", len(userNotes))
			for _, n := range userNotes {
				fmt.Fprintln(&sb)
				glLabelColor.Fprintf(&sb, "    %s ", n.Author)
				glDimColor.Fprintf(&sb, "(%s):\n", glTimeAgo(n.CreatedAt))
				for _, l := range strings.Split(strings.TrimSpace(n.Body), "\n") {
					fmt.Fprintf(&sb, "    %s\n", l)
				}
			}
		}
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

func renderMRFile(sb *strings.Builder, f MRFile) {
	prefix := " "
	if f.IsNew {
		prefix = glMRMergedColor.Sprint("+")
	} else if f.IsDeleted {
		prefix = glMRClosedColor.Sprint("-")
	} else if f.IsRenamed {
		prefix = glProjectColor.Sprint("→")
	}
	path := f.NewPath
	if f.IsRenamed && f.OldPath != f.NewPath {
		path = fmt.Sprintf("%s → %s", f.OldPath, f.NewPath)
	}
	fmt.Fprintf(sb, "    %s %s", prefix, path)
	if f.Additions > 0 || f.Deletions > 0 {
		glDimColor.Fprintf(sb, " (+%d/-%d)", f.Additions, f.Deletions)
	}
	fmt.Fprintln(sb)
}

func renderMRDiscussions(sb *strings.Builder, discussions []MRDiscussion) {
	var nonSystem []MRDiscussion
	for _, d := range discussions {
		hasUserNote := false
		for _, n := range d.Notes {
			if !n.System {
				hasUserNote = true
				break
			}
		}
		if hasUserNote {
			nonSystem = append(nonSystem, d)
		}
	}
	if len(nonSystem) == 0 {
		return
	}

	fmt.Fprintln(sb)
	glSectionColor.Fprintf(sb, "  Discussions (%d):\n", len(nonSystem))

	for _, d := range nonSystem {
		fmt.Fprintln(sb)
		for i, n := range d.Notes {
			if n.System {
				continue
			}
			if i == 0 {
				glLabelColor.Fprintf(sb, "  [%s] %s ", glTruncate(d.ID, 8), n.Author)
			} else {
				glDimColor.Fprint(sb, "    ↳ ")
				glLabelColor.Fprintf(sb, "%s ", n.Author)
			}
			glDimColor.Fprintf(sb, "(%s)", glTimeAgo(n.CreatedAt))
			if n.Resolvable {
				if n.Resolved {
					glMRMergedColor.Fprint(sb, " ✓")
				} else {
					glMRClosedColor.Fprint(sb, " ○")
				}
			}
			if n.Position != nil {
				glDimColor.Fprintf(sb, " [%s:%d]", n.Position.NewPath, n.Position.NewLine)
			}
			fmt.Fprintln(sb)
			for _, l := range strings.Split(strings.TrimSpace(n.Body), "\n") {
				if i == 0 {
					fmt.Fprintf(sb, "    %s\n", l)
				} else {
					fmt.Fprintf(sb, "      %s\n", l)
				}
			}
		}
	}
}

// ── PipelineListResult ────────────────────────────────────────────────────────

// PipelineListResult holds a list of pipelines for display.
type PipelineListResult struct {
	Pipelines []PipelineSummary `json:"pipelines"`
	Total     int               `json:"total"`
}

func (r *PipelineListResult) RenderText(mode render.Mode) string {
	if len(r.Pipelines) == 0 {
		return glDimColor.Sprint("No pipelines found.\n")
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, p := range r.Pipelines {
			sha := p.SHA
			if len(sha) > 8 {
				sha = sha[:8]
			}
			status := glFormatPipelineStatus(p.Status)
			fmt.Fprintf(&sb, "%-8d  %s  %-30s  ", p.ID, status, glTruncate(p.Ref, 30))
			glDimColor.Fprintf(&sb, "%s  %s\n", sha, glTimeAgo(p.CreatedAt))
		}
		return sb.String()
	}

	line := strings.Repeat("═", 90)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glHeaderColor.Fprintf(&sb, "  Pipelines (%d)\n", len(r.Pipelines))
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	fmt.Fprintf(&sb, "  %-8s  %-10s  %-20s  %-8s  %-14s  %s\n",
		"ID", "STATUS", "REF", "SHA", "SOURCE", "CREATED")
	fmt.Fprintf(&sb, "  %s\n", strings.Repeat("─", 86))

	for _, p := range r.Pipelines {
		sha := p.SHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		ref := glTruncate(p.Ref, 20)
		source := glTruncate(p.Source, 14)
		status := glFormatPipelineStatus(p.Status)

		fmt.Fprintf(&sb, "  %-8d  %s  %-20s  ", p.ID, status, ref)
		glDimColor.Fprintf(&sb, "%-8s  ", sha)
		fmt.Fprintf(&sb, "%-14s  ", source)
		glDimColor.Fprintf(&sb, "%s\n", glTimeAgo(p.CreatedAt))
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

// ── PipelineDetailResult ──────────────────────────────────────────────────────

// PipelineDetailResult holds full pipeline information for display.
type PipelineDetailResult struct {
	PipelineDetail
}

func (r *PipelineDetailResult) RenderText(mode render.Mode) string {
	p := &r.PipelineDetail
	var sb strings.Builder

	line := strings.Repeat("═", 70)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	statusStr := glFormatPipelineStatus(p.Status)
	glProjectColor.Fprintf(&sb, "  Pipeline #%d  %s\n", p.ID, statusStr)
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	glPrintField(&sb, "ID", fmt.Sprintf("%d", p.ID))
	glPrintField(&sb, "Status", p.Status)
	glPrintField(&sb, "Ref", p.Ref)
	if p.Tag {
		glPrintField(&sb, "Tag", "yes")
	}
	glPrintField(&sb, "SHA", p.SHA)
	glPrintField(&sb, "Source", p.Source)
	if p.User != "" {
		glPrintField(&sb, "User", p.User)
	}
	glPrintField(&sb, "URL", p.WebURL)
	glPrintField(&sb, "Created", glFormatTimestamp(p.CreatedAt))
	if p.StartedAt != nil {
		glPrintField(&sb, "Started", glFormatTimestamp(*p.StartedAt))
	}
	if p.FinishedAt != nil {
		glPrintField(&sb, "Finished", glFormatTimestamp(*p.FinishedAt))
	}
	if p.Duration > 0 {
		glPrintField(&sb, "Duration", glFormatDurationSecs(p.Duration))
	}
	if p.QueuedDuration > 0 {
		glPrintField(&sb, "Queued", glFormatDurationSecs(p.QueuedDuration))
	}
	if p.Coverage != "" {
		glPrintField(&sb, "Coverage", p.Coverage+"%")
	}
	if p.YamlErrors != "" {
		fmt.Fprintln(&sb)
		glMRClosedColor.Fprintf(&sb, "  YAML Errors: %s\n", p.YamlErrors)
	}

	if mode == render.ModeCompact {
		if len(p.Jobs) > 0 {
			// Count by status
			statusCounts := map[string]int{}
			for _, j := range p.Jobs {
				statusCounts[j.Status]++
			}
			parts := []string{fmt.Sprintf("%d total", len(p.Jobs))}
			for _, s := range []string{"failed", "running", "success", "canceled", "skipped", "manual"} {
				if n := statusCounts[s]; n > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", n, s))
				}
			}
			glPrintField(&sb, "Jobs", strings.Join(parts, ", "))
		}
		fmt.Fprintln(&sb)
		return sb.String()
	}

	// Normal mode: full job list
	if len(p.Jobs) > 0 {
		fmt.Fprintln(&sb)
		jobsResult := &PipelineJobsResult{Jobs: p.Jobs, Total: len(p.Jobs)}
		sb.WriteString(jobsResult.RenderText(mode))
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

// ── PipelineJobsResult ────────────────────────────────────────────────────────

// PipelineJobsResult holds a list of pipeline jobs for display.
type PipelineJobsResult struct {
	PipelineID int          `json:"pipeline_id,omitempty"`
	Jobs       []PipelineJob `json:"jobs"`
	Total      int          `json:"total"`
}

func (r *PipelineJobsResult) RenderText(mode render.Mode) string {
	if len(r.Jobs) == 0 {
		return glDimColor.Sprint("  No jobs found.\n")
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, j := range r.Jobs {
			status := glFormatPipelineStatus(j.Status)
			dur := glFormatDurationSecs(int(j.Duration))
			fmt.Fprintf(&sb, "  %s  %-8s  %-30s  %s", status, glTruncate(j.Stage, 8), glTruncate(j.Name, 30), dur)
			if j.FailureReason != "" {
				glMRClosedColor.Fprintf(&sb, "  (%s)", j.FailureReason)
			}
			fmt.Fprintln(&sb)
		}
		return sb.String()
	}

	// Group jobs by stage
	type stageJobs struct {
		name string
		jobs []PipelineJob
	}
	var stages []stageJobs
	stageIndex := make(map[string]int)

	for _, j := range r.Jobs {
		idx, exists := stageIndex[j.Stage]
		if !exists {
			idx = len(stages)
			stageIndex[j.Stage] = idx
			stages = append(stages, stageJobs{name: j.Stage})
		}
		stages[idx].jobs = append(stages[idx].jobs, j)
	}

	glSectionColor.Fprintf(&sb, "  Jobs (%d):\n", len(r.Jobs))

	for _, stage := range stages {
		fmt.Fprintln(&sb)
		glLabelColor.Fprintf(&sb, "    Stage: %s\n", stage.name)

		for _, j := range stage.jobs {
			status := glFormatPipelineStatus(j.Status)
			duration := glFormatDurationSecs(int(j.Duration))

			fmt.Fprintf(&sb, "      %s  %-30s  ", status, glTruncate(j.Name, 30))
			glDimColor.Fprint(&sb, duration)
			if j.FailureReason != "" {
				glMRClosedColor.Fprintf(&sb, "  (%s)", j.FailureReason)
			}
			if j.AllowFailure {
				glDimColor.Fprint(&sb, "  [allowed to fail]")
			}
			fmt.Fprintln(&sb)
		}
	}

	return sb.String()
}

// ── CommitListResult ──────────────────────────────────────────────────────────

// CommitListResult holds a list of commits for display.
type CommitListResult struct {
	Commits []Commit `json:"commits"`
	Total   int      `json:"total"`
}

func (r *CommitListResult) RenderText(mode render.Mode) string {
	if len(r.Commits) == 0 {
		return glDimColor.Sprint("No commits found.\n")
	}

	var sb strings.Builder

	if mode == render.ModeCompact {
		for _, c := range r.Commits {
			glCommitColor.Fprintf(&sb, "%s ", c.ShortID)
			fmt.Fprintf(&sb, "%-60s  ", glTruncate(c.Title, 60))
			glDimColor.Fprintf(&sb, "%s  %s\n", c.AuthorName, glTimeAgo(c.CreatedAt))
		}
		return sb.String()
	}

	fmt.Fprintln(&sb)
	glSectionColor.Fprintf(&sb, "  Commits (%d):\n", len(r.Commits))
	fmt.Fprintln(&sb)

	for _, c := range r.Commits {
		title := glTruncate(c.Title, 60)
		ago := glTimeAgo(c.CreatedAt)
		glCommitColor.Fprintf(&sb, "  %s ", c.ShortID)
		fmt.Fprintf(&sb, "%s ", title)
		glDimColor.Fprintf(&sb, "(%s, %s)\n", c.AuthorName, ago)
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

// ── CommitDetailResult ────────────────────────────────────────────────────────

// CommitDetailResult holds full commit information for display.
type CommitDetailResult struct {
	CommitDetail
}

func (r *CommitDetailResult) RenderText(mode render.Mode) string {
	c := &r.CommitDetail
	var sb strings.Builder

	line := strings.Repeat("═", 60)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glCommitColor.Fprintf(&sb, "  Commit %s\n", c.ShortID)
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	glPrintField(&sb, "SHA", c.ID)
	glPrintField(&sb, "Author", fmt.Sprintf("%s <%s>", c.AuthorName, c.AuthorEmail))
	if c.CommitterName != c.AuthorName {
		glPrintField(&sb, "Committer", fmt.Sprintf("%s <%s>", c.CommitterName, c.CommitterEmail))
	}
	glPrintField(&sb, "Date", glFormatTimestamp(c.CreatedAt))
	if c.WebURL != "" {
		glPrintField(&sb, "URL", c.WebURL)
	}

	if c.Stats.Total > 0 {
		fmt.Fprintln(&sb)
		statsStr := fmt.Sprintf("+%d/-%d (%d total)", c.Stats.Additions, c.Stats.Deletions, c.Stats.Total)
		glPrintField(&sb, "Changes", statsStr)
	}

	if mode == render.ModeCompact {
		// Compact: title only, no full message body
		fmt.Fprintln(&sb)
		glSectionColor.Fprint(&sb, "  Title:\n")
		fmt.Fprintf(&sb, "    %s\n", c.Title)
		fmt.Fprintln(&sb)
		return sb.String()
	}

	fmt.Fprintln(&sb)
	glSectionColor.Fprint(&sb, "  Message:\n")
	fmt.Fprintln(&sb)
	for _, l := range strings.Split(strings.TrimSpace(c.Message), "\n") {
		fmt.Fprintf(&sb, "    %s\n", l)
	}
	fmt.Fprintln(&sb)
	return sb.String()
}

// ── ProjectListResult ─────────────────────────────────────────────────────────

// ProjectListResult holds a list of projects for display.
// IndexedAt is non-nil when results come from the local index.
type ProjectListResult struct {
	Projects  []ProjectMetadata `json:"projects"`
	Total     int               `json:"total"`
	IndexedAt *time.Time        `json:"indexed_at,omitempty"`
}

// NewProjectListFromIndex builds a ProjectListResult from the local index.
func NewProjectListFromIndex(projects []ProjectMetadata, indexedAt time.Time) *ProjectListResult {
	return &ProjectListResult{
		Projects:  projects,
		Total:     len(projects),
		IndexedAt: &indexedAt,
	}
}

// NewProjectListFromAPI builds a ProjectListResult from API projects.
func NewProjectListFromAPI(projects []*gogitlab.Project) *ProjectListResult {
	var ps []ProjectMetadata
	for _, p := range projects {
		pm := ProjectMetadata{
			ID:          p.ID,
			Name:        p.Name,
			PathWithNS:  p.PathWithNamespace,
			Description: p.Description,
			WebURL:      p.WebURL,
			Visibility:  string(p.Visibility),
		}
		if p.LastActivityAt != nil {
			pm.LastActivityAt = *p.LastActivityAt
		}
		ps = append(ps, pm)
	}
	return &ProjectListResult{
		Projects: ps,
		Total:    len(ps),
	}
}

func (r *ProjectListResult) RenderText(mode render.Mode) string {
	if len(r.Projects) == 0 {
		if r.IndexedAt != nil {
			return glDimColor.Sprint("No projects in index. Run 'dex gitlab index' first.\n")
		}
		return glDimColor.Sprint("No projects found.\n")
	}

	var sb strings.Builder
	line := strings.Repeat("═", 80)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)

	if r.IndexedAt != nil {
		glHeaderColor.Fprintf(&sb, "  GitLab Projects (%d)", len(r.Projects))
		glDimColor.Fprintf(&sb, "  [indexed %s]\n", glTimeAgo(*r.IndexedAt))
	} else {
		glHeaderColor.Fprintf(&sb, "  GitLab Projects (%d)\n", len(r.Projects))
	}
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	if mode == render.ModeCompact {
		for _, p := range r.Projects {
			path := glTruncate(p.PathWithNS, 50)
			vis := glFormatVisibility(p.Visibility)
			glProjectColor.Fprintf(&sb, "  %-6d  ", p.ID)
			fmt.Fprintf(&sb, "%-50s  %s  ", path, vis)
			glDimColor.Fprintf(&sb, "%s\n", glTimeAgo(p.LastActivityAt))
		}
		return sb.String()
	}

	fmt.Fprintf(&sb, "  %-6s  %-40s  %-12s  %s\n",
		"ID", "PATH", "VISIBILITY", "LAST ACTIVITY")
	fmt.Fprintf(&sb, "  %s\n", strings.Repeat("─", 76))

	for _, p := range r.Projects {
		path := glTruncate(p.PathWithNS, 40)
		vis := glFormatVisibility(p.Visibility)
		glProjectColor.Fprintf(&sb, "  %-6d  ", p.ID)
		fmt.Fprintf(&sb, "%-40s  %s  ", path, vis)
		glDimColor.Fprintf(&sb, "%s\n", glTimeAgo(p.LastActivityAt))
	}

	fmt.Fprintln(&sb)
	return sb.String()
}

// ── ProjectDetailResult ───────────────────────────────────────────────────────

// ProjectDetailResult holds full project information for display.
type ProjectDetailResult struct {
	ProjectMetadata
}

func (r *ProjectDetailResult) RenderText(mode render.Mode) string {
	p := &r.ProjectMetadata
	var sb strings.Builder

	line := strings.Repeat("═", 60)
	fmt.Fprintln(&sb)
	glHeaderColor.Fprintln(&sb, line)
	glProjectColor.Fprintf(&sb, "  %s\n", p.PathWithNS)
	glHeaderColor.Fprintln(&sb, line)
	fmt.Fprintln(&sb)

	glPrintField(&sb, "ID", fmt.Sprintf("%d", p.ID))
	glPrintField(&sb, "Name", p.Name)
	glPrintField(&sb, "URL", p.WebURL)
	if p.Description != "" {
		glPrintField(&sb, "Description", glTruncate(p.Description, 60))
	}
	glPrintField(&sb, "Default Branch", p.DefaultBranch)
	glPrintField(&sb, "Visibility", p.Visibility)
	if len(p.Topics) > 0 {
		glPrintField(&sb, "Topics", strings.Join(p.Topics, ", "))
	}
	glPrintField(&sb, "Stars", fmt.Sprintf("%d", p.StarCount))
	glPrintField(&sb, "Forks", fmt.Sprintf("%d", p.ForksCount))

	if mode == render.ModeCompact {
		if len(p.Languages) > 0 {
			glPrintField(&sb, "Languages", fmt.Sprintf("%d", len(p.Languages)))
		}
		if len(p.TopContributors) > 0 {
			glPrintField(&sb, "Contributors", fmt.Sprintf("%d", len(p.TopContributors)))
		}
		glPrintField(&sb, "Last Activity", glFormatTimestamp(p.LastActivityAt))
		glPrintField(&sb, "Indexed At", glFormatTimestamp(p.IndexedAt))
		fmt.Fprintln(&sb)
		return sb.String()
	}

	fmt.Fprintln(&sb)

	if len(p.Languages) > 0 {
		glSectionColor.Fprint(&sb, "  Languages:\n")
		renderLanguages(&sb, p.Languages)
		fmt.Fprintln(&sb)
	}

	if len(p.TopContributors) > 0 {
		glSectionColor.Fprint(&sb, "  Top Contributors:\n")
		for _, c := range p.TopContributors {
			fmt.Fprintf(&sb, "    • %-25s ", glTruncate(c.Name, 25))
			glDimColor.Fprintf(&sb, "%4d commits", c.Commits)
			if c.Additions > 0 || c.Deletions > 0 {
				glDimColor.Fprintf(&sb, " (+%d/-%d)", c.Additions, c.Deletions)
			}
			fmt.Fprintln(&sb)
		}
		fmt.Fprintln(&sb)
	}

	glPrintField(&sb, "Last Activity", glFormatTimestamp(p.LastActivityAt))
	glPrintField(&sb, "Indexed At", glFormatTimestamp(p.IndexedAt))
	fmt.Fprintln(&sb)
	return sb.String()
}

func renderLanguages(sb *strings.Builder, langs map[string]float32) {
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
		barLen := int(l.pct / 5)
		if barLen < 1 && l.pct > 0 {
			barLen = 1
		}
		bar := strings.Repeat("█", barLen)
		glLangColor.Fprintf(sb, "    %-12s ", l.name)
		glSectionColor.Fprintf(sb, "%-20s", bar)
		glDimColor.Fprintf(sb, " %5.1f%%\n", l.pct)
	}
}
