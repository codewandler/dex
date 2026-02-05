package gitlab

import (
	"fmt"
	"time"

	"github.com/codewandler/dex/internal/models"

	"github.com/xanzy/go-gitlab"
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

func (c *Client) GetMergeRequests(projectID int, since time.Time) ([]models.MergeRequest, error) {
	var allMRs []models.MergeRequest

	opts := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		UpdatedAfter: gitlab.Ptr(since),
		Scope:        gitlab.Ptr("all"),
	}

	for {
		mrs, resp, err := c.gl.MergeRequests.ListProjectMergeRequests(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, m := range mrs {
			mr := models.MergeRequest{
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
func (c *Client) ListMergeRequests(opts ListMergeRequestsOptions) ([]models.MergeRequestDetail, error) {
	var allMRs []models.MergeRequestDetail

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

	listOpts := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: min(opts.Limit, 100),
			Page:    1,
		},
		State:   gitlab.Ptr(opts.State),
		Scope:   gitlab.Ptr(opts.Scope),
		OrderBy: gitlab.Ptr(opts.OrderBy),
		Sort:    gitlab.Ptr(opts.Sort),
	}

	// Exclude WIP/drafts by default
	if !opts.IncludeWIP {
		listOpts.WIP = gitlab.Ptr("no")
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

			mr := models.MergeRequestDetail{
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
func (c *Client) GetMergeRequest(projectID interface{}, mrIID int) (*models.MergeRequestDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	m, _, err := c.gl.MergeRequests.GetMergeRequest(pid, mrIID, nil)
	if err != nil {
		return nil, err
	}

	mr := &models.MergeRequestDetail{
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
func (c *Client) GetMergeRequestCommits(projectID any, mrIID int) ([]models.MRCommit, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	commits, _, err := c.gl.MergeRequests.GetMergeRequestCommits(pid, mrIID, nil)
	if err != nil {
		return nil, err
	}

	var result []models.MRCommit
	for _, commit := range commits {
		mc := models.MRCommit{
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
func (c *Client) GetMergeRequestChanges(projectID any, mrIID int, includeDiff bool) ([]models.MRFile, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gitlab.ListMergeRequestDiffsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var files []models.MRFile
	for {
		diffs, resp, err := c.gl.MergeRequests.ListMergeRequestDiffs(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, diff := range diffs {
			f := models.MRFile{
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

	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.Ptr(body),
	}

	_, _, err = c.gl.Notes.CreateMergeRequestNote(pid, mrIID, opts)
	return err
}

// GetMergeRequestNotes fetches all notes/comments on a merge request
func (c *Client) GetMergeRequestNotes(projectID any, mrIID int) ([]models.MRNote, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		Sort:    gitlab.Ptr("asc"),
		OrderBy: gitlab.Ptr("created_at"),
	}

	var notes []models.MRNote
	for {
		apiNotes, resp, err := c.gl.Notes.ListMergeRequestNotes(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, n := range apiNotes {
			note := models.MRNote{
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
func (c *Client) GetMergeRequestDiscussions(projectID any, mrIID int) ([]models.MRDiscussion, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gitlab.ListMergeRequestDiscussionsOptions{
		PerPage: 100,
		Page:    1,
	}

	var discussions []models.MRDiscussion
	for {
		apiDiscussions, resp, err := c.gl.Discussions.ListMergeRequestDiscussions(pid, mrIID, opts)
		if err != nil {
			return nil, err
		}

		for _, d := range apiDiscussions {
			disc := models.MRDiscussion{
				ID:             d.ID,
				IndividualNote: d.IndividualNote,
			}
			for _, n := range d.Notes {
				note := models.MRNote{
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
					note.Position = &models.NotePosition{
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

	opts := &gitlab.AddMergeRequestDiscussionNoteOptions{
		Body: gitlab.Ptr(body),
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

	// Get the diff version info first
	diffVersion, err := c.GetMergeRequestDiffVersions(pid, mrIID)
	if err != nil {
		return fmt.Errorf("failed to get diff versions: %w", err)
	}

	position := &gitlab.PositionOptions{
		BaseSHA:      gitlab.Ptr(diffVersion.BaseCommitSHA),
		StartSHA:     gitlab.Ptr(diffVersion.StartCommitSHA),
		HeadSHA:      gitlab.Ptr(diffVersion.HeadCommitSHA),
		PositionType: gitlab.Ptr("text"),
		NewPath:      gitlab.Ptr(opts.NewPath),
		OldPath:      gitlab.Ptr(opts.OldPath),
	}

	// Set line numbers based on what was provided
	if opts.NewLine > 0 {
		position.NewLine = gitlab.Ptr(opts.NewLine)
	}
	if opts.OldLine > 0 {
		position.OldLine = gitlab.Ptr(opts.OldLine)
	}

	createOpts := &gitlab.CreateMergeRequestDiscussionOptions{
		Body:     gitlab.Ptr(opts.Body),
		Position: position,
	}

	_, _, err = c.gl.Discussions.CreateMergeRequestDiscussion(pid, mrIID, createOpts)
	return err
}

// GetMergeRequestDiffVersions fetches the diff version info needed for inline comments
func (c *Client) GetMergeRequestDiffVersions(projectID any, mrIID int) (*models.MRDiffVersion, error) {
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
	return &models.MRDiffVersion{
		HeadCommitSHA:  v.HeadCommitSHA,
		BaseCommitSHA:  v.BaseCommitSHA,
		StartCommitSHA: v.StartCommitSHA,
	}, nil
}

// CreateMergeRequestReaction adds an emoji reaction to a merge request
func (c *Client) CreateMergeRequestReaction(projectID any, mrIID int, emoji string) error {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return err
	}

	opts := &gitlab.CreateAwardEmojiOptions{
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

	opts := &gitlab.CreateAwardEmojiOptions{
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

	opts := &gitlab.UpdateMergeRequestOptions{
		StateEvent: gitlab.Ptr("close"),
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

	acceptOpts := &gitlab.AcceptMergeRequestOptions{}

	if opts.Squash {
		acceptOpts.Squash = gitlab.Ptr(true)
	}
	if opts.RemoveSourceBranch {
		acceptOpts.ShouldRemoveSourceBranch = gitlab.Ptr(true)
	}
	if opts.MergeWhenPipelineSucceeds {
		acceptOpts.MergeWhenPipelineSucceeds = gitlab.Ptr(true)
	}
	if opts.MergeCommitMessage != "" {
		acceptOpts.MergeCommitMessage = gitlab.Ptr(opts.MergeCommitMessage)
	}
	if opts.SquashCommitMessage != "" {
		acceptOpts.SquashCommitMessage = gitlab.Ptr(opts.SquashCommitMessage)
	}

	_, _, err = c.gl.MergeRequests.AcceptMergeRequest(pid, mrIID, acceptOpts)
	return err
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
func (c *Client) CreateMergeRequest(projectID any, opts CreateMergeRequestOptions) (*models.MergeRequestDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	createOpts := &gitlab.CreateMergeRequestOptions{
		Title:              gitlab.Ptr(opts.Title),
		SourceBranch:       gitlab.Ptr(opts.SourceBranch),
		TargetBranch:       gitlab.Ptr(opts.TargetBranch),
		RemoveSourceBranch: gitlab.Ptr(opts.RemoveSourceBranch),
		Squash:             gitlab.Ptr(opts.Squash),
	}

	if opts.Description != "" {
		createOpts.Description = gitlab.Ptr(opts.Description)
	}

	// Handle draft status by prefixing title
	if opts.Draft {
		createOpts.Title = gitlab.Ptr("Draft: " + opts.Title)
	}

	mr, _, err := c.gl.MergeRequests.CreateMergeRequest(pid, createOpts)
	if err != nil {
		return nil, err
	}

	result := &models.MergeRequestDetail{
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
