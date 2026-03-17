package gitlab

import (
	"fmt"
	"strings"
	"time"

	gogitlab "github.com/xanzy/go-gitlab"
)

// ListMergeRequestsOptions configures the MR list query
type ListMergeRequestsOptions struct {
	State         string // opened, closed, merged, all
	Scope         string // created_by_me, assigned_to_me, all
	Limit         int
	OrderBy       string // created_at, updated_at
	Sort          string // asc, desc
	ProjectID     string // optional - filter to specific project
	IncludeWIP    bool   // include WIP/draft MRs (excluded by default)
	ConflictsOnly bool   // only show MRs with conflicts
}

func (c *Client) GetMergeRequests(projectID int, since time.Time) ([]MergeRequest, error) {
	var allMRs []MergeRequest

	opts := &gogitlab.ListProjectMergeRequestsOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		UpdatedAfter: gogitlab.Ptr(since),
		Scope:        gogitlab.Ptr("all"),
	}

	for {
		mrs, resp, err := c.gl.MergeRequests.ListProjectMergeRequests(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, m := range mrs {
			mr := MergeRequest{
				IID:    m.IID,
				Title:  m.Title,
				State:  m.State,
				WebURL: m.WebURL,
			}
			if m.Author != nil {
				mr.Author = m.Author.Username
			}
			if m.CreatedAt != nil {
				mr.CreatedAt = *m.CreatedAt
			}
			if m.UpdatedAt != nil {
				mr.UpdatedAt = *m.UpdatedAt
			}
			allMRs = append(allMRs, mr)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allMRs, nil
}

// ListMergeRequests fetches merge requests based on options
func (c *Client) ListMergeRequests(opts ListMergeRequestsOptions) ([]MergeRequestDetail, error) {
	var allMRs []MergeRequestDetail

	// Default values
	if opts.Limit == 0 {
		opts.Limit = 20
	}
	if opts.State == "" {
		opts.State = "opened"
	}
	if opts.Scope == "" {
		opts.Scope = "all"
	}
	if opts.OrderBy == "" {
		opts.OrderBy = "updated_at"
	}
	if opts.Sort == "" {
		opts.Sort = "desc"
	}

	listOpts := &gogitlab.ListMergeRequestsOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: min(opts.Limit, 100),
			Page:    1,
		},
		State:   gogitlab.Ptr(opts.State),
		Scope:   gogitlab.Ptr(opts.Scope),
		OrderBy: gogitlab.Ptr(opts.OrderBy),
		Sort:    gogitlab.Ptr(opts.Sort),
	}

	// Exclude WIP/drafts by default
	if !opts.IncludeWIP {
		listOpts.WIP = gogitlab.Ptr("no")
	}

	for {
		mrs, resp, err := c.gl.MergeRequests.ListMergeRequests(listOpts)
		if err != nil {
			return nil, err
		}

		for _, m := range mrs {
			// Skip non-conflicting MRs if conflicts-only filter is set
			if opts.ConflictsOnly && !m.HasConflicts {
				continue
			}

			mr := MergeRequestDetail{
				IID:           m.IID,
				Title:         m.Title,
				State:         m.State,
				WebURL:        m.WebURL,
				SourceBranch:  m.SourceBranch,
				TargetBranch:  m.TargetBranch,
				Draft:         m.Draft,
				MergeStatus:   m.MergeStatus,
				HasConflicts:  m.HasConflicts,
			}
			if m.Author != nil {
				mr.Author = m.Author.Username
			}
			if m.CreatedAt != nil {
				mr.CreatedAt = *m.CreatedAt
			}
			if m.UpdatedAt != nil {
				mr.UpdatedAt = *m.UpdatedAt
			}
			if m.MergedAt != nil {
				mr.MergedAt = m.MergedAt
			}
			// Extract project path from web URL or references
			if m.References != nil {
				mr.ProjectPath = m.References.Full
			}
			allMRs = append(allMRs, mr)

			if len(allMRs) >= opts.Limit {
				return allMRs, nil
			}
		}

		if resp.NextPage == 0 || len(allMRs) >= opts.Limit {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return allMRs, nil
}

// GetMergeRequest fetches a single merge request with full details
func (c *Client) GetMergeRequest(projectID interface{}, mrIID int) (*MergeRequestDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	m, _, err := c.gl.MergeRequests.GetMergeRequest(pid, mrIID, nil)
	if err != nil {
		return nil, err
	}

	mr := &MergeRequestDetail{
		IID:          m.IID,
		Title:        m.Title,
		Description:  m.Description,
		State:        m.State,
		WebURL:       m.WebURL,
		SourceBranch: m.SourceBranch,
		TargetBranch: m.TargetBranch,
		Draft:        m.Draft,
		MergeStatus:  m.MergeStatus,
		HasConflicts: m.HasConflicts,
	}

	if m.Author != nil {
		mr.Author = m.Author.Username
	}
	if m.CreatedAt != nil {
		mr.CreatedAt = *m.CreatedAt
	}
	if m.UpdatedAt != nil {
		mr.UpdatedAt = *m.UpdatedAt
	}
	if m.MergedAt != nil {
		mr.MergedAt = m.MergedAt
	}
	if m.MergedBy != nil {
		mr.MergedBy = m.MergedBy.Username
	}
	if m.References != nil {
		mr.ProjectPath = m.References.Full
	}

	// Labels
	for _, label := range m.Labels {
		mr.Labels = append(mr.Labels, label)
	}

	// Assignees
	if m.Assignees != nil {
		for _, a := range m.Assignees {
			mr.Assignees = append(mr.Assignees, a.Username)
		}
	}

	// Reviewers
	if m.Reviewers != nil {
		for _, r := range m.Reviewers {
			mr.Reviewers = append(mr.Reviewers, r.Username)
		}
	}

	// Changes stats - ChangesCount is a string in the API
	if m.ChangesCount != "" {
		// Parse changes count
		var count int
		fmt.Sscanf(m.ChangesCount, "%d", &count)
		mr.Changes.Files = count
	}

	return mr, nil
}

// GetMergeRequestCommits fetches commits associated with a merge request
func (c *Client) GetMergeRequestCommits(projectID any, mrIID int) ([]MRCommit, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	commits, _, err := c.gl.MergeRequests.GetMergeRequestCommits(pid, mrIID, nil)
	if err != nil {
		return nil, err
	}

	var result []MRCommit
	for _, commit := range commits {
		mc := MRCommit{
			ShortID: commit.ShortID,
			Title:   commit.Title,
		}
		if commit.AuthorName != "" {
			mc.Author = commit.AuthorName
		}
		if commit.CreatedAt != nil {
			mc.CreatedAt = *commit.CreatedAt
		}
		result = append(result, mc)
	}

	return result, nil
}

// GetMergeRequestChanges fetches the list of files changed in a merge request
func (c *Client) GetMergeRequestChanges(projectID any, mrIID int, includeDiff bool) ([]MRFile, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.ListMergeRequestDiffsOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var files []MRFile
	for {
		diffs, resp, err := c.gl.MergeRequests.ListMergeRequestDiffs(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, diff := range diffs {
			f := MRFile{
				OldPath:   diff.OldPath,
				NewPath:   diff.NewPath,
				IsNew:     diff.NewFile,
				IsDeleted: diff.DeletedFile,
				IsRenamed: diff.RenamedFile,
			}
			if includeDiff {
				f.Diff = diff.Diff
			}
			files = append(files, f)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return files, nil
}

// CreateMergeRequestNote adds a comment/note to a merge request
func (c *Client) CreateMergeRequestNote(projectID any, mrIID int, body string) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.CreateMergeRequestNoteOptions{
		Body: gogitlab.Ptr(body),
	}

	_, _, err = c.gl.Notes.CreateMergeRequestNote(pid, mrIID, opts)
	return err
}

// GetMergeRequestNotes fetches all notes/comments on a merge request
func (c *Client) GetMergeRequestNotes(projectID any, mrIID int) ([]MRNote, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.ListMergeRequestNotesOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		Sort:    gogitlab.Ptr("asc"),
		OrderBy: gogitlab.Ptr("created_at"),
	}

	var notes []MRNote
	for {
		apiNotes, resp, err := c.gl.Notes.ListMergeRequestNotes(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, n := range apiNotes {
			note := MRNote{
				ID:     n.ID,
				Body:   n.Body,
				System: n.System,
				Author: n.Author.Username,
			}
			if n.CreatedAt != nil {
				note.CreatedAt = *n.CreatedAt
			}
			notes = append(notes, note)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return notes, nil
}

// GetMergeRequestDiscussions fetches all discussions/threads on a merge request
func (c *Client) GetMergeRequestDiscussions(projectID any, mrIID int) ([]MRDiscussion, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.ListMergeRequestDiscussionsOptions{
		PerPage: 100,
		Page:    1,
	}

	var discussions []MRDiscussion
	for {
		apiDiscussions, resp, err := c.gl.Discussions.ListMergeRequestDiscussions(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, d := range apiDiscussions {
			disc := MRDiscussion{
				ID:             d.ID,
				IndividualNote: d.IndividualNote,
			}
			for _, n := range d.Notes {
				note := MRNote{
					ID:           n.ID,
					DiscussionID: d.ID,
					Body:         n.Body,
					System:       n.System,
					Resolvable:   n.Resolvable,
					Resolved:     n.Resolved,
					Author:       n.Author.Username,
				}
				if n.CreatedAt != nil {
					note.CreatedAt = *n.CreatedAt
				}
				if n.Position != nil {
					note.Position = &NotePosition{
						NewPath: n.Position.NewPath,
						OldPath: n.Position.OldPath,
						NewLine: n.Position.NewLine,
						OldLine: n.Position.OldLine,
					}
				}
				disc.Notes = append(disc.Notes, note)
			}
			discussions = append(discussions, disc)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return discussions, nil
}

// AddMergeRequestDiscussionReply adds a reply to an existing discussion thread
func (c *Client) AddMergeRequestDiscussionReply(projectID any, mrIID int, discussionID, body string) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.AddMergeRequestDiscussionNoteOptions{
		Body: gogitlab.Ptr(body),
	}

	_, _, err = c.gl.Discussions.AddMergeRequestDiscussionNote(pid, mrIID, discussionID, opts)
	return err
}

// InlineCommentOptions contains options for creating an inline/diff comment
type InlineCommentOptions struct {
	Body    string
	NewPath string
	OldPath string
	NewLine int
	OldLine int
}

// CreateMergeRequestInlineComment creates an inline comment on a specific file/line
func (c *Client) CreateMergeRequestInlineComment(projectID any, mrIID int, opts InlineCommentOptions) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	// Auto-detect old_line/new_line mapping if only new_line is provided
	// This handles context lines which require BOTH old_line and new_line
	if opts.NewLine > 0 && opts.OldLine == 0 {
		parsedDiff, err := c.GetParsedDiffForFile(projectID, mrIID, opts.NewPath)
		if err == nil {
			if line, found := parsedDiff.FindLineByNew(opts.NewLine); found {
				// For context lines, set the old_line
				// For added lines, OldLine will be 0 (which is correct)
				opts.OldLine = line.OldLine
			}
		}
		// If parsing fails, continue with original behavior (works for added lines)
	}

	// Get the diff version info
	diffVersion, err := c.GetMergeRequestDiffVersions(pid, mrIID)
	if err != nil {
		return fmt.Errorf("failed to get diff versions: %w", err)
	}

	position := &gogitlab.PositionOptions{
		BaseSHA:      gogitlab.Ptr(diffVersion.BaseCommitSHA),
		StartSHA:     gogitlab.Ptr(diffVersion.StartCommitSHA),
		HeadSHA:      gogitlab.Ptr(diffVersion.HeadCommitSHA),
		PositionType: gogitlab.Ptr("text"),
		NewPath:      gogitlab.Ptr(opts.NewPath),
		OldPath:      gogitlab.Ptr(opts.OldPath),
	}

	// Set line numbers based on what was provided
	if opts.NewLine > 0 {
		position.NewLine = gogitlab.Ptr(opts.NewLine)
	}
	if opts.OldLine > 0 {
		position.OldLine = gogitlab.Ptr(opts.OldLine)
	}

	createOpts := &gogitlab.CreateMergeRequestDiscussionOptions{
		Body:     gogitlab.Ptr(opts.Body),
		Position: position,
	}

	_, _, err = c.gl.Discussions.CreateMergeRequestDiscussion(pid, mrIID, createOpts)
	return err
}

// GetMergeRequestDiffVersions fetches the diff version info needed for inline comments
func (c *Client) GetMergeRequestDiffVersions(projectID any, mrIID int) (*MRDiffVersion, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	versions, _, err := c.gl.MergeRequests.GetMergeRequestDiffVersions(pid, mrIID, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no diff versions found")
	}

	// Use the latest (first) version
	v := versions[0]
	return &MRDiffVersion{
		HeadCommitSHA:  v.HeadCommitSHA,
		BaseCommitSHA:  v.BaseCommitSHA,
		StartCommitSHA: v.StartCommitSHA,
	}, nil
}

// GetParsedDiffForFile fetches and parses the diff for a specific file in an MR
func (c *Client) GetParsedDiffForFile(projectID any, mrIID int, filePath string) (*ParsedDiff, error) {
	files, err := c.GetMergeRequestChanges(projectID, mrIID, true)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.NewPath == filePath || f.OldPath == filePath {
			parsed := ParseUnifiedDiff(f.Diff)
			parsed.OldPath = f.OldPath
			parsed.NewPath = f.NewPath
			return parsed, nil
		}
	}

	return nil, fmt.Errorf("file %q not found in merge request diff", filePath)
}

// CreateMergeRequestReaction adds an emoji reaction to a merge request
func (c *Client) CreateMergeRequestReaction(projectID any, mrIID int, emoji string) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.CreateAwardEmojiOptions{
		Name: emoji,
	}

	_, _, err = c.gl.AwardEmoji.CreateMergeRequestAwardEmoji(pid, mrIID, opts)
	return err
}

// CreateMergeRequestNoteReaction adds an emoji reaction to a note/comment on a merge request
func (c *Client) CreateMergeRequestNoteReaction(projectID any, mrIID int, noteID int, emoji string) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.CreateAwardEmojiOptions{
		Name: emoji,
	}

	_, _, err = c.gl.AwardEmoji.CreateMergeRequestAwardEmojiOnNote(pid, mrIID, noteID, opts)
	return err
}

// CloseMergeRequest closes a merge request
func (c *Client) CloseMergeRequest(projectID any, mrIID int) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.UpdateMergeRequestOptions{
		StateEvent: gogitlab.Ptr("close"),
	}

	_, _, err = c.gl.MergeRequests.UpdateMergeRequest(pid, mrIID, opts)
	return err
}

// ReopenMergeRequest reopens a closed merge request
func (c *Client) ReopenMergeRequest(projectID any, mrIID int) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gogitlab.UpdateMergeRequestOptions{
		StateEvent: gogitlab.Ptr("reopen"),
	}

	_, _, err = c.gl.MergeRequests.UpdateMergeRequest(pid, mrIID, opts)
	return err
}

// ApproveMergeRequest approves a merge request
func (c *Client) ApproveMergeRequest(projectID any, mrIID int) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	_, _, err = c.gl.MergeRequestApprovals.ApproveMergeRequest(pid, mrIID, nil)
	return err
}

// MergeMergeRequestOptions contains options for merging a merge request
type MergeMergeRequestOptions struct {
	Squash                    bool
	RemoveSourceBranch        bool
	MergeWhenPipelineSucceeds bool
	MergeCommitMessage        string
	SquashCommitMessage       string
}

// MergeMergeRequest merges (accepts) a merge request
func (c *Client) MergeMergeRequest(projectID any, mrIID int, opts MergeMergeRequestOptions) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	acceptOpts := &gogitlab.AcceptMergeRequestOptions{}

	if opts.Squash {
		acceptOpts.Squash = gogitlab.Ptr(true)
	}
	if opts.RemoveSourceBranch {
		acceptOpts.ShouldRemoveSourceBranch = gogitlab.Ptr(true)
	}
	if opts.MergeWhenPipelineSucceeds {
		acceptOpts.MergeWhenPipelineSucceeds = gogitlab.Ptr(true)
	}
	if opts.MergeCommitMessage != "" {
		acceptOpts.MergeCommitMessage = gogitlab.Ptr(opts.MergeCommitMessage)
	}
	if opts.SquashCommitMessage != "" {
		acceptOpts.SquashCommitMessage = gogitlab.Ptr(opts.SquashCommitMessage)
	}

	_, _, err = c.gl.MergeRequests.AcceptMergeRequest(pid, mrIID, acceptOpts)
	return err
}

// EditMergeRequestOptions contains options for editing (updating) a merge request
type EditMergeRequestOptions struct {
	Title              *string  // New title (nil = no change)
	Description        *string  // New description (nil = no change)
	TargetBranch       *string  // New target branch (nil = no change)
	AddLabels          []string // Labels to add
	RemoveLabels       []string // Labels to remove
	Draft              *bool    // Set draft status (nil = no change)
	Squash             *bool    // Set squash setting (nil = no change)
	RemoveSourceBranch *bool    // Set remove source branch setting (nil = no change)
}

// EditMergeRequest updates a merge request and returns the updated details
func (c *Client) EditMergeRequest(projectID any, mrIID int, opts EditMergeRequestOptions) (*MergeRequestDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	updateOpts := &gogitlab.UpdateMergeRequestOptions{}

	if opts.Title != nil {
		updateOpts.Title = opts.Title
	}
	if opts.Description != nil {
		updateOpts.Description = opts.Description
	}
	if opts.TargetBranch != nil {
		updateOpts.TargetBranch = opts.TargetBranch
	}
	if opts.Squash != nil {
		updateOpts.Squash = opts.Squash
	}
	if opts.RemoveSourceBranch != nil {
		updateOpts.RemoveSourceBranch = opts.RemoveSourceBranch
	}
	if len(opts.AddLabels) > 0 {
		labels := gogitlab.LabelOptions(opts.AddLabels)
		updateOpts.AddLabels = &labels
	}
	if len(opts.RemoveLabels) > 0 {
		labels := gogitlab.LabelOptions(opts.RemoveLabels)
		updateOpts.RemoveLabels = &labels
	}
	// Handle draft toggle: prefix/strip "Draft: " from title
	if opts.Draft != nil {
		// Fetch current title if we don't have a new one
		currentTitle := ""
		if opts.Title != nil {
			currentTitle = *opts.Title
		} else {
			mr, _, err := c.gl.MergeRequests.GetMergeRequest(pid, mrIID, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch current MR: %w", err)
			}
			currentTitle = mr.Title
		}

		const draftPrefix = "Draft: "
		isDraft := strings.HasPrefix(currentTitle, draftPrefix) ||
			strings.HasPrefix(currentTitle, "WIP: ") ||
			strings.HasPrefix(currentTitle, "draft: ")

		if *opts.Draft && !isDraft {
			currentTitle = draftPrefix + currentTitle
			updateOpts.Title = &currentTitle
		} else if !*opts.Draft && isDraft {
			for _, prefix := range []string{"Draft: ", "draft: ", "WIP: "} {
				currentTitle = strings.TrimPrefix(currentTitle, prefix)
			}
			updateOpts.Title = &currentTitle
		}
	}

	m, _, err := c.gl.MergeRequests.UpdateMergeRequest(pid, mrIID, updateOpts)
	if err != nil {
		return nil, err
	}

	result := &MergeRequestDetail{
		IID:          m.IID,
		Title:        m.Title,
		Description:  m.Description,
		State:        m.State,
		WebURL:       m.WebURL,
		SourceBranch: m.SourceBranch,
		TargetBranch: m.TargetBranch,
		Draft:        m.Draft,
		MergeStatus:  m.MergeStatus,
		HasConflicts: m.HasConflicts,
	}
	if m.Author != nil {
		result.Author = m.Author.Username
	}
	if m.CreatedAt != nil {
		result.CreatedAt = *m.CreatedAt
	}
	if m.UpdatedAt != nil {
		result.UpdatedAt = *m.UpdatedAt
	}
	if m.MergedAt != nil {
		result.MergedAt = m.MergedAt
	}
	if m.References != nil {
		result.ProjectPath = m.References.Full
	}
	for _, l := range m.Labels {
		result.Labels = append(result.Labels, l)
	}
	if m.Assignees != nil {
		for _, a := range m.Assignees {
			result.Assignees = append(result.Assignees, a.Username)
		}
	}
	if m.Reviewers != nil {
		for _, r := range m.Reviewers {
			result.Reviewers = append(result.Reviewers, r.Username)
		}
	}

	return result, nil
}

// CreateMergeRequestOptions contains options for creating a merge request
type CreateMergeRequestOptions struct {
	Title              string
	Description        string
	SourceBranch       string
	TargetBranch       string
	Draft              bool
	RemoveSourceBranch bool
	Squash             bool
}

// CreateMergeRequest creates a new merge request and returns its details
func (c *Client) CreateMergeRequest(projectID any, opts CreateMergeRequestOptions) (*MergeRequestDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	createOpts := &gogitlab.CreateMergeRequestOptions{
		Title:              gogitlab.Ptr(opts.Title),
		SourceBranch:       gogitlab.Ptr(opts.SourceBranch),
		TargetBranch:       gogitlab.Ptr(opts.TargetBranch),
		RemoveSourceBranch: gogitlab.Ptr(opts.RemoveSourceBranch),
		Squash:             gogitlab.Ptr(opts.Squash),
	}

	if opts.Description != "" {
		createOpts.Description = gogitlab.Ptr(opts.Description)
	}

	// Handle draft status by prefixing title
	if opts.Draft {
		createOpts.Title = gogitlab.Ptr("Draft: " + opts.Title)
	}

	mr, _, err := c.gl.MergeRequests.CreateMergeRequest(pid, createOpts)
	if err != nil {
		return nil, err
	}

	result := &MergeRequestDetail{
		IID:          mr.IID,
		Title:        mr.Title,
		State:        mr.State,
		WebURL:       mr.WebURL,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Draft:        mr.Draft,
	}

	if mr.Author != nil {
		result.Author = mr.Author.Username
	}

	return result, nil
}
