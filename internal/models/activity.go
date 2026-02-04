package models

import "time"

type Commit struct {
	ID          string
	ShortID     string
	Title       string
	AuthorName  string
	AuthorEmail string
	CreatedAt   time.Time
	WebURL      string
}

// CommitDetail contains full commit information including the body/message
type CommitDetail struct {
	ID             string
	ShortID        string
	Title          string
	Message        string // Full commit message including body
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	CreatedAt      time.Time
	CommittedAt    time.Time
	WebURL         string
	ProjectPath    string
	Stats          CommitStats
	ParentIDs      []string
}

// CommitStats contains addition/deletion statistics
type CommitStats struct {
	Additions int
	Deletions int
	Total     int
}

type MergeRequest struct {
	IID       int
	Title     string
	State     string
	Author    string
	CreatedAt time.Time
	UpdatedAt time.Time
	WebURL    string
}

// MergeRequestDetail contains full MR information for detailed views
type MergeRequestDetail struct {
	IID          int
	Title        string
	Description  string
	State        string
	Author       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MergedAt     *time.Time
	MergedBy     string
	WebURL       string
	SourceBranch string
	TargetBranch string
	ProjectPath  string
	Draft        bool
	MergeStatus  string
	HasConflicts bool
	Labels       []string
	Assignees    []string
	Reviewers    []string
	Approvers    []string
	Changes      MergeRequestChanges
	Commits     []MRCommit     // populated on detail view
	Files       []MRFile       // populated on detail view
	Notes       []MRNote       // populated on detail view (flat list for backwards compat)
	Discussions []MRDiscussion // populated on detail view (threaded discussions)
}

// MergeRequestChanges contains diff statistics
type MergeRequestChanges struct {
	Additions int
	Deletions int
	Files     int
}

// MRCommit is a simplified commit structure for MR commit lists
type MRCommit struct {
	ShortID   string
	Title     string
	Author    string
	CreatedAt time.Time
}

// MRFile represents a file changed in a merge request
type MRFile struct {
	OldPath     string
	NewPath     string
	IsNew       bool
	IsDeleted   bool
	IsRenamed   bool
	Additions   int
	Deletions   int
	Diff        string // populated only with --show-diff
}

// MRNote represents a comment/note on a merge request
type MRNote struct {
	ID           int
	DiscussionID string // ID of the discussion thread this note belongs to
	Body         string
	Author       string
	CreatedAt    time.Time
	System       bool           // true for system-generated notes (e.g., "mentioned in commit")
	Resolvable   bool           // true for notes that can be resolved (e.g., inline comments)
	Resolved     bool           // true if the note has been resolved
	Position     *NotePosition  // position info for inline comments (nil for regular comments)
}

// NotePosition contains position information for inline/diff comments
type NotePosition struct {
	NewPath string // file path in the new version
	OldPath string // file path in the old version
	NewLine int    // line number in new version (0 if not applicable)
	OldLine int    // line number in old version (0 if not applicable)
}

// MRDiscussion represents a discussion thread on a merge request
type MRDiscussion struct {
	ID             string   // discussion ID (used for replies)
	IndividualNote bool     // true if this is a single note, not a thread
	Notes          []MRNote // notes in this discussion thread
}

// MRDiffVersion contains version info for creating inline comments
type MRDiffVersion struct {
	HeadCommitSHA  string
	BaseCommitSHA  string
	StartCommitSHA string
}

type Tag struct {
	Name      string
	Message   string
	CreatedAt time.Time
	WebURL    string
}

type ProjectActivity struct {
	ProjectID       int
	ProjectName     string
	ProjectPath     string
	WebURL          string
	Commits         []Commit
	MergeRequests   []MergeRequest
	Tags            []Tag
}

func (p *ProjectActivity) HasActivity() bool {
	return len(p.Commits) > 0 || len(p.MergeRequests) > 0 || len(p.Tags) > 0
}

type ActivitySummary struct {
	TotalProjects      int
	TotalCommits       int
	TotalMergeRequests int
	TotalTags          int
}

func CalculateSummary(activities []ProjectActivity) ActivitySummary {
	summary := ActivitySummary{}
	for _, a := range activities {
		if a.HasActivity() {
			summary.TotalProjects++
			summary.TotalCommits += len(a.Commits)
			summary.TotalMergeRequests += len(a.MergeRequests)
			summary.TotalTags += len(a.Tags)
		}
	}
	return summary
}
