package gitlab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/render"
	gogitlab "github.com/xanzy/go-gitlab"
)

// ── Data types ────────────────────────────────────────────────────────────────

// SnippetFile represents a single file within a snippet.
type SnippetFile struct {
	Path   string `json:"path"`
	RawURL string `json:"raw_url"`
}

// Snippet represents a GitLab personal snippet.
type Snippet struct {
	ID          int           `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Visibility  string        `json:"visibility"`
	FileName    string        `json:"file_name,omitempty"` // legacy single-file field
	Author      string        `json:"author"`
	WebURL      string        `json:"web_url"`
	RawURL      string        `json:"raw_url,omitempty"`
	Files       []SnippetFile `json:"files,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// SnippetDetail wraps a Snippet with its raw file content for display.
type SnippetDetail struct {
	Snippet
	Content string `json:"content,omitempty"`
}

// SnippetList is a slice of Snippets with structured render support.
type SnippetList struct {
	Snippets []Snippet `json:"snippets"`
}

// CreateSnippetFileInput is a file to include when creating a snippet.
type CreateSnippetFileInput struct {
	FilePath string
	Content  string
}

// CreateSnippetInput holds options for creating a new snippet.
type CreateSnippetInput struct {
	Title       string
	Description string
	Visibility  string // public, internal, private
	Files       []CreateSnippetFileInput
}

// ── render.Renderable implementations ────────────────────────────────────────

// RenderText implements render.Renderable on Snippet.
// ModeCompact: single line. ModeNormal: multi-line detail block.
func (s *Snippet) RenderText(mode render.Mode) string {
	if mode == render.ModeCompact {
		return fmt.Sprintf("#%-6d %-9s %-40s  %s\n",
			s.ID,
			"["+s.Visibility+"]",
			truncateSnippet(s.Title, 40),
			s.Author,
		)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n", s.ID, s.Title)
	fmt.Fprintf(&b, "  Visibility: %s\n", s.Visibility)
	fmt.Fprintf(&b, "  Author:     %s\n", s.Author)
	fmt.Fprintf(&b, "  URL:        %s\n", s.WebURL)
	fmt.Fprintf(&b, "  Created:    %s\n", s.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "  Updated:    %s\n", s.UpdatedAt.Format("2006-01-02 15:04"))
	if s.Description != "" {
		fmt.Fprintf(&b, "  Desc:       %s\n", s.Description)
	}
	return b.String()
}

// RenderText implements render.Renderable on SnippetDetail.
// Adds file listing and content to the base Snippet rendering.
func (sd *SnippetDetail) RenderText(mode render.Mode) string {
	var b strings.Builder
	b.WriteString(sd.Snippet.RenderText(mode))
	if mode == render.ModeCompact {
		return b.String()
	}
	// Files
	if len(sd.Files) > 0 {
		b.WriteString("  Files:\n")
		for _, f := range sd.Files {
			fmt.Fprintf(&b, "    • %s\n", f.Path)
		}
	} else if sd.FileName != "" {
		fmt.Fprintf(&b, "  Files:\n    • %s\n", sd.FileName)
	}
	// Content
	if sd.Content != "" {
		b.WriteString("\n")
		lines := strings.Split(strings.TrimRight(sd.Content, "\n"), "\n")
		for i, l := range lines {
			fmt.Fprintf(&b, "  %4d  %s\n", i+1, l)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// MarshalJSON on SnippetDetail includes the content field cleanly.
func (sd *SnippetDetail) MarshalJSON() ([]byte, error) {
	type flat struct {
		ID          int           `json:"id"`
		Title       string        `json:"title"`
		Description string        `json:"description,omitempty"`
		Visibility  string        `json:"visibility"`
		Author      string        `json:"author"`
		WebURL      string        `json:"web_url"`
		Files       []SnippetFile `json:"files,omitempty"`
		CreatedAt   time.Time     `json:"created_at"`
		UpdatedAt   time.Time     `json:"updated_at"`
		Content     string        `json:"content,omitempty"`
	}
	return json.Marshal(flat{
		ID:          sd.ID,
		Title:       sd.Title,
		Description: sd.Description,
		Visibility:  sd.Visibility,
		Author:      sd.Author,
		WebURL:      sd.WebURL,
		Files:       sd.Files,
		CreatedAt:   sd.CreatedAt,
		UpdatedAt:   sd.UpdatedAt,
		Content:     sd.Content,
	})
}

// RenderText implements render.Renderable on SnippetList.
func (sl *SnippetList) RenderText(mode render.Mode) string {
	if len(sl.Snippets) == 0 {
		return "No snippets found.\n"
	}
	var b strings.Builder
	if mode == render.ModeNormal {
		fmt.Fprintf(&b, "Snippets (%d):\n\n", len(sl.Snippets))
	}
	for i := range sl.Snippets {
		b.WriteString(sl.Snippets[i].RenderText(mode))
		if mode == render.ModeNormal {
			b.WriteString("\n")
		}
	}
	if mode == render.ModeNormal {
		fmt.Fprintf(&b, "%d snippets\n", len(sl.Snippets))
	}
	return b.String()
}

// MarshalJSON on SnippetList produces {snippets:[...], total:N}.
func (sl *SnippetList) MarshalJSON() ([]byte, error) {
	type out struct {
		Snippets []Snippet `json:"snippets"`
		Total    int       `json:"total"`
	}
	return json.Marshal(out{Snippets: sl.Snippets, Total: len(sl.Snippets)})
}

// ── Client methods ────────────────────────────────────────────────────────────

// ListSnippets returns the current user's personal snippets.
func (c *Client) ListSnippets(limit int) (*SnippetList, error) {
	if limit <= 0 {
		limit = 20
	}

	opts := &gogitlab.ListSnippetsOptions{
		Page:    1,
		PerPage: min(limit, 100),
	}

	var snippets []Snippet
	for {
		apiSnippets, resp, err := c.gl.Snippets.ListSnippets(opts)
		if err != nil {
			return nil, err
		}

		for _, s := range apiSnippets {
			snippets = append(snippets, mapSnippet(s))
			if len(snippets) >= limit {
				return &SnippetList{Snippets: snippets}, nil
			}
		}

		if resp.NextPage == 0 || len(snippets) >= limit {
			break
		}
		opts.Page = resp.NextPage
	}

	return &SnippetList{Snippets: snippets}, nil
}

// GetSnippet fetches a single snippet with its raw content.
func (c *Client) GetSnippet(id int, fetchContent bool) (*SnippetDetail, error) {
	s, _, err := c.gl.Snippets.GetSnippet(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get snippet %d: %w", id, err)
	}

	detail := &SnippetDetail{Snippet: mapSnippet(s)}

	if fetchContent {
		data, _, err := c.gl.Snippets.SnippetContent(id)
		if err != nil {
			// non-fatal: return what we have
			return detail, nil
		}
		detail.Content = string(data)
	}

	return detail, nil
}

// CreateSnippet creates a new personal snippet.
func (c *Client) CreateSnippet(opts CreateSnippetInput) (*Snippet, error) {
	if opts.Title == "" {
		return nil, fmt.Errorf("snippet title is required")
	}
	if len(opts.Files) == 0 {
		return nil, fmt.Errorf("at least one file is required")
	}
	if opts.Visibility == "" {
		opts.Visibility = "private"
	}

	visibility := gogitlab.VisibilityValue(opts.Visibility)
	files := make([]*gogitlab.CreateSnippetFileOptions, 0, len(opts.Files))
	for _, f := range opts.Files {
		fp := f.FilePath
		content := f.Content
		files = append(files, &gogitlab.CreateSnippetFileOptions{
			FilePath: &fp,
			Content:  &content,
		})
	}

	createOpts := &gogitlab.CreateSnippetOptions{
		Title:      gogitlab.Ptr(opts.Title),
		Visibility: &visibility,
		Files:      &files,
	}
	if opts.Description != "" {
		createOpts.Description = gogitlab.Ptr(opts.Description)
	}

	s, _, err := c.gl.Snippets.CreateSnippet(createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create snippet: %w", err)
	}

	result := mapSnippet(s)
	return &result, nil
}

// DeleteSnippet deletes a personal snippet by ID.
func (c *Client) DeleteSnippet(id int) error {
	_, err := c.gl.Snippets.DeleteSnippet(id)
	if err != nil {
		return fmt.Errorf("failed to delete snippet %d: %w", id, err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mapSnippet(s *gogitlab.Snippet) Snippet {
	snip := Snippet{
		ID:          s.ID,
		Title:       s.Title,
		Description: s.Description,
		Visibility:  s.Visibility,
		FileName:    s.FileName,
		WebURL:      s.WebURL,
		RawURL:      s.RawURL,
		Author:      s.Author.Username,
	}
	if s.CreatedAt != nil {
		snip.CreatedAt = *s.CreatedAt
	}
	if s.UpdatedAt != nil {
		snip.UpdatedAt = *s.UpdatedAt
	}
	for _, f := range s.Files {
		snip.Files = append(snip.Files, SnippetFile{
			Path:   f.Path,
			RawURL: f.RawURL,
		})
	}
	return snip
}

func truncateSnippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
