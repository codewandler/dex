package jira

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/dex/internal/render"
)

// RenderText implements render.Renderable on Issue.
// ModeNormal reproduces the existing FormatIssue output exactly.
// ModeCompact prints a single line: key  status  assignee  summary.
func (i *Issue) RenderText(mode render.Mode) string {
	if mode == render.ModeCompact {
		assignee := "Unassigned"
		if i.Fields.Assignee != nil {
			assignee = i.Fields.Assignee.DisplayName
		}
		return fmt.Sprintf("%-12s %-12s %-15s %s\n",
			i.Key,
			i.Fields.Status.Name,
			truncateJira(assignee, 15),
			truncateJira(i.Fields.Summary, 50),
		)
	}
	return FormatIssue(i)
}

// RenderText implements render.Renderable on SearchResult.
// ModeNormal prints a header + compact row per issue.
// ModeCompact prints only the compact rows (no header).
func (s *SearchResult) RenderText(mode render.Mode) string {
	var b strings.Builder
	if len(s.Issues) == 0 {
		return "No issues found.\n"
	}
	if mode == render.ModeNormal {
		fmt.Fprintf(&b, "Found %d issues:\n\n", len(s.Issues))
	}
	for i := range s.Issues {
		b.WriteString(s.Issues[i].RenderText(render.ModeCompact))
	}
	return b.String()
}

// MarshalJSON on SearchResult produces a clean {total, issues:[...]} shape.
func (s *SearchResult) MarshalJSON() ([]byte, error) {
	type out struct {
		Total  int     `json:"total"`
		Issues []Issue `json:"issues"`
	}
	return json.Marshal(out{Total: len(s.Issues), Issues: s.Issues})
}

// RenderText implements render.Renderable on Project.
// ModeNormal reproduces the existing jiraProjectCmd text output exactly.
// ModeCompact prints a single line: key: name (type).
func (p *Project) RenderText(mode render.Mode) string {
	var b strings.Builder
	if mode == render.ModeCompact {
		fmt.Fprintf(&b, "%s: %s (%s)\n", p.Key, p.Name, p.ProjectType)
		return b.String()
	}
	fmt.Fprintf(&b, "%s: %s\n", p.Key, p.Name)
	style := p.Style
	if style == "" {
		style = "classic"
	}
	fmt.Fprintf(&b, "  Type:        %s (%s)\n", p.ProjectType, style)
	if p.Lead != nil {
		fmt.Fprintf(&b, "  Lead:        %s\n", p.Lead.DisplayName)
	}
	if p.ProjectCategory != nil {
		fmt.Fprintf(&b, "  Category:    %s\n", p.ProjectCategory.Name)
	}
	if p.Description != "" {
		fmt.Fprintf(&b, "  Description: %s\n", p.Description)
	}
	if p.URL != "" {
		fmt.Fprintf(&b, "  URL:         %s\n", p.URL)
	}
	if len(p.IssueTypes) > 0 {
		b.WriteString("\nIssue Types:\n")
		for _, it := range p.IssueTypes {
			fmt.Fprintf(&b, "  • %s\n", it.Name)
		}
	}
	if len(p.Components) > 0 {
		b.WriteString("\nComponents:\n")
		for _, c := range p.Components {
			fmt.Fprintf(&b, "  • %s\n", c.Name)
		}
	}
	return b.String()
}

// ProjectWithStatuses combines a Project with its workflow statuses so
// the CLI can call Render() on a single value.
// It implements json.Marshaler to produce a flat, user-friendly JSON shape.
type ProjectWithStatuses struct {
	Project  *Project
	Statuses []IssueTypeWithStatus
	SiteURL  string
}

// MarshalJSON produces a flat JSON object instead of the nested struct fields.
func (ps *ProjectWithStatuses) MarshalJSON() ([]byte, error) {
	type workflowGroup struct {
		IssueTypes []string `json:"issue_types"`
		Statuses   []string `json:"statuses"`
	}

	type flat struct {
		Key         string          `json:"key"`
		Name        string          `json:"name"`
		Type        string          `json:"type"`
		Style       string          `json:"style,omitempty"`
		Lead        string          `json:"lead,omitempty"`
		Category    string          `json:"category,omitempty"`
		Description string          `json:"description,omitempty"`
		URL         string          `json:"url,omitempty"`
		IssueTypes  []string        `json:"issue_types"`
		Components  []string        `json:"components"`
		Workflows   []workflowGroup `json:"workflows"`
	}

	f := flat{
		Key:         ps.Project.Key,
		Name:        ps.Project.Name,
		Type:        ps.Project.ProjectType,
		Style:       ps.Project.Style,
		Description: ps.Project.Description,
		URL:         ps.Project.URL,
	}
	if ps.Project.Lead != nil {
		f.Lead = ps.Project.Lead.DisplayName
	}
	if ps.Project.ProjectCategory != nil {
		f.Category = ps.Project.ProjectCategory.Name
	}
	for _, it := range ps.Project.IssueTypes {
		f.IssueTypes = append(f.IssueTypes, it.Name)
	}
	for _, c := range ps.Project.Components {
		f.Components = append(f.Components, c.Name)
	}

	type group struct {
		issueTypes []string
		statuses   []string
	}
	var groups []group
outer:
	for _, it := range ps.Statuses {
		names := make([]string, len(it.Statuses))
		for i, s := range it.Statuses {
			names[i] = s.Name
		}
		for gi := range groups {
			if fmt.Sprint(groups[gi].statuses) == fmt.Sprint(names) {
				groups[gi].issueTypes = append(groups[gi].issueTypes, it.Name)
				continue outer
			}
		}
		groups = append(groups, group{issueTypes: []string{it.Name}, statuses: names})
	}
	for _, g := range groups {
		f.Workflows = append(f.Workflows, workflowGroup{IssueTypes: g.issueTypes, Statuses: g.statuses})
	}

	return json.Marshal(f)
}

// RenderText implements render.Renderable on ProjectWithStatuses.
func (ps *ProjectWithStatuses) RenderText(mode render.Mode) string {
	var b strings.Builder
	b.WriteString(ps.Project.RenderText(mode))
	if mode == render.ModeCompact {
		return b.String()
	}
	if len(ps.Statuses) > 0 {
		b.WriteString("\nWorkflow Statuses:\n")
		b.WriteString(renderWorkflowStatuses(ps.Statuses))
	}
	return b.String()
}

// ProjectList is a slice of Projects with a RenderText implementation.
// It implements json.Marshaler to produce a clean flat shape.
type ProjectList struct {
	Projects []Project
	SiteURL  string
}

// MarshalJSON produces a flat JSON array of project summaries.
func (pl *ProjectList) MarshalJSON() ([]byte, error) {
	type projectSummary struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Type string `json:"type"`
		URL  string `json:"url,omitempty"`
	}
	type out struct {
		Projects []projectSummary `json:"projects"`
		Total    int              `json:"total"`
	}
	o := out{}
	for _, p := range pl.Projects {
		ps := projectSummary{Key: p.Key, Name: p.Name, Type: p.ProjectType}
		if pl.SiteURL != "" {
			ps.URL = fmt.Sprintf("%s/browse/%s", pl.SiteURL, p.Key)
		}
		o.Projects = append(o.Projects, ps)
	}
	o.Total = len(o.Projects)
	return json.Marshal(o)
}

// RenderText implements render.Renderable on ProjectList.
func (pl *ProjectList) RenderText(mode render.Mode) string {
	var b strings.Builder
	if len(pl.Projects) == 0 {
		return "No projects found.\n"
	}
	if mode == render.ModeCompact {
		for _, p := range pl.Projects {
			fmt.Fprintf(&b, "%s\n", p.Key)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "%-10s %-40s %s\n", "KEY", "NAME", "TYPE")
	b.WriteString("────────────────────────────────────────────────────────────────\n")
	for _, p := range pl.Projects {
		keyDisplay := fmt.Sprintf("%-10s", p.Key)
		if pl.SiteURL != "" {
			keyDisplay = fmt.Sprintf("\033]8;;%s/browse/%s\033\\\033[35m%s\033[0m\033]8;;\033\\",
				pl.SiteURL, p.Key, keyDisplay)
		}
		fmt.Fprintf(&b, "%s %-40s %s\n", keyDisplay, truncateJira(p.Name, 40), p.ProjectType)
	}
	fmt.Fprintf(&b, "\n%d projects\n", len(pl.Projects))
	return b.String()
}

// TransitionList is a slice of Transitions with a RenderText implementation.
// It implements json.Marshaler for a clean flat shape.
type TransitionList struct {
	IssueKey    string
	Transitions []Transition
}

// MarshalJSON produces a flat shape with snake_case fields.
func (tl *TransitionList) MarshalJSON() ([]byte, error) {
	type td struct {
		Name   string `json:"name"`
		ToName string `json:"to_name"`
	}
	type out struct {
		IssueKey    string `json:"issue_key"`
		Transitions []td   `json:"transitions"`
	}
	o := out{IssueKey: tl.IssueKey}
	for _, t := range tl.Transitions {
		o.Transitions = append(o.Transitions, td{Name: t.Name, ToName: t.To.Name})
	}
	return json.Marshal(o)
}

// RenderText implements render.Renderable on TransitionList.
func (tl *TransitionList) RenderText(mode render.Mode) string {
	var b strings.Builder
	if len(tl.Transitions) == 0 {
		return "No transitions available\n"
	}
	if mode == render.ModeCompact {
		for _, t := range tl.Transitions {
			fmt.Fprintf(&b, "%s → %s\n", t.Name, t.To.Name)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "Available transitions for %s:\n", tl.IssueKey)
	for _, t := range tl.Transitions {
		fmt.Fprintf(&b, "  • %s → %s\n", t.Name, t.To.Name)
	}
	return b.String()
}

// WorkflowStatuses is a slice of IssueTypeWithStatus with a RenderText implementation.
// It implements json.Marshaler for a clean flat shape.
type WorkflowStatuses struct {
	ProjectKey string
	Statuses   []IssueTypeWithStatus
}

// MarshalJSON produces a flat shape with grouped workflows.
func (w *WorkflowStatuses) MarshalJSON() ([]byte, error) {
	type wg struct {
		IssueTypes []string `json:"issue_types"`
		Statuses   []string `json:"statuses"`
	}
	type out struct {
		ProjectKey string `json:"project_key"`
		Workflows  []wg   `json:"workflows"`
	}
	o := out{ProjectKey: w.ProjectKey}
	type group struct {
		issueTypes []string
		statuses   []string
	}
	var groups []group
outer:
	for _, it := range w.Statuses {
		names := make([]string, len(it.Statuses))
		for i, s := range it.Statuses {
			names[i] = s.Name
		}
		for gi := range groups {
			if fmt.Sprint(groups[gi].statuses) == fmt.Sprint(names) {
				groups[gi].issueTypes = append(groups[gi].issueTypes, it.Name)
				continue outer
			}
		}
		groups = append(groups, group{issueTypes: []string{it.Name}, statuses: names})
	}
	for _, g := range groups {
		o.Workflows = append(o.Workflows, wg{IssueTypes: g.issueTypes, Statuses: g.statuses})
	}
	return json.Marshal(o)
}

// RenderText implements render.Renderable on WorkflowStatuses.
func (w *WorkflowStatuses) RenderText(mode render.Mode) string {
	var b strings.Builder
	if len(w.Statuses) == 0 {
		return "No workflow statuses found.\n"
	}
	if mode == render.ModeNormal {
		fmt.Fprintf(&b, "Workflow Statuses for %s:\n", w.ProjectKey)
	}
	b.WriteString(renderWorkflowStatuses(w.Statuses))
	return b.String()
}

// renderWorkflowStatuses formats []IssueTypeWithStatus grouped by shared status sets.
func renderWorkflowStatuses(statuses []IssueTypeWithStatus) string {
	type group struct {
		issueTypes []string
		statuses   []string
	}
	var groups []group
outer:
	for _, it := range statuses {
		names := make([]string, len(it.Statuses))
		for i, s := range it.Statuses {
			names[i] = s.Name
		}
		for gi := range groups {
			if fmt.Sprint(groups[gi].statuses) == fmt.Sprint(names) {
				groups[gi].issueTypes = append(groups[gi].issueTypes, it.Name)
				continue outer
			}
		}
		groups = append(groups, group{issueTypes: []string{it.Name}, statuses: names})
	}
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, "  %s\n", strings.Join(g.issueTypes, " / "))
		for _, s := range g.statuses {
			fmt.Fprintf(&b, "    • %s\n", s)
		}
	}
	return b.String()
}

// truncateJira truncates s to maxLen bytes, appending "..." if truncated.
func truncateJira(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// MyIssueResult wraps SearchResult to give `dex jira my` a distinct text header.
// It implements json.Marshaler for a clean flat shape.
type MyIssueResult struct {
	*SearchResult
}

// MarshalJSON produces a clean flat shape: {total, issues:[...]}.
func (m *MyIssueResult) MarshalJSON() ([]byte, error) {
	type out struct {
		Total  int     `json:"total"`
		Issues []Issue `json:"issues"`
	}
	return json.Marshal(out{Total: len(m.Issues), Issues: m.Issues})
}

// RenderText implements render.Renderable on MyIssueResult.
// ModeNormal prints "Your issues (N):" followed by compact rows.
// ModeCompact prints only the compact rows.
func (m *MyIssueResult) RenderText(mode render.Mode) string {
	var b strings.Builder
	if len(m.Issues) == 0 {
		return "No issues assigned to you.\n"
	}
	if mode == render.ModeNormal {
		fmt.Fprintf(&b, "Your issues (%d):\n\n", len(m.Issues))
	}
	for i := range m.Issues {
		b.WriteString(m.Issues[i].RenderText(render.ModeCompact))
	}
	return b.String()
}
