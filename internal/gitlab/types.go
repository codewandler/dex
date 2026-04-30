package gitlab

import (
	"strconv"
	"strings"
	"time"
)

// Commit represents a git commit in list views
type Commit struct {
	ID          string    `json:"id"`
	ShortID     string    `json:"short_id"`
	Title       string    `json:"title"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   time.Time `json:"created_at"`
	WebURL      string    `json:"web_url"`
}

// CommitDetail contains full commit information including the body/message
type CommitDetail struct {
	ID             string      `json:"id"`
	ShortID        string      `json:"short_id"`
	Title          string      `json:"title"`
	Message        string      `json:"message"` // Full commit message including body
	AuthorName     string      `json:"author_name"`
	AuthorEmail    string      `json:"author_email"`
	CommitterName  string      `json:"committer_name"`
	CommitterEmail string      `json:"committer_email"`
	CreatedAt      time.Time   `json:"created_at"`
	CommittedAt    time.Time   `json:"committed_at"`
	WebURL         string      `json:"web_url"`
	ProjectPath    string      `json:"project_path"`
	Stats          CommitStats `json:"stats"`
	ParentIDs      []string    `json:"parent_ids"`
}

// CommitStats contains addition/deletion statistics
type CommitStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

// MergeRequest represents a merge request in activity/summary views
type MergeRequest struct {
	IID       int       `json:"iid"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	WebURL    string    `json:"web_url"`
}

// MergeRequestDetail contains full MR information for detailed views
type MergeRequestDetail struct {
	IID               int                 `json:"iid"`
	Title             string              `json:"title"`
	Description       string              `json:"description"`
	State             string              `json:"state"`
	Author            string              `json:"author"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	MergedAt          *time.Time          `json:"merged_at,omitempty"`
	MergedBy          string              `json:"merged_by,omitempty"`
	WebURL            string              `json:"web_url"`
	SourceBranch      string              `json:"source_branch"`
	TargetBranch      string              `json:"target_branch"`
	ProjectPath       string              `json:"project_path"`
	Draft             bool                `json:"draft"`
	MergeStatus       string              `json:"merge_status"`
	HasConflicts      bool                `json:"has_conflicts"`
	Labels            []string            `json:"labels,omitempty"`
	Assignees         []string            `json:"assignees,omitempty"`
	Reviewers         []string            `json:"reviewers,omitempty"`
	Approvers         []string            `json:"approvers,omitempty"`
	Approved          bool                `json:"approved"`
	ApprovalsRequired int                 `json:"approvals_required"`
	ApprovalsLeft     int                 `json:"approvals_left"`
	ApprovedBy        []string            `json:"approved_by,omitempty"`
	Changes           MergeRequestChanges `json:"changes"`
	Commits           []MRCommit          `json:"commits,omitempty"`
	Files             []MRFile            `json:"files,omitempty"`
	Notes             []MRNote            `json:"notes,omitempty"`
	Discussions       []MRDiscussion      `json:"discussions,omitempty"`
}

// MergeRequestChanges contains diff statistics
type MergeRequestChanges struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Files     int `json:"files"`
}

// MRCommit is a simplified commit structure for MR commit lists
type MRCommit struct {
	ShortID   string    `json:"short_id"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// MRFile represents a file changed in a merge request
type MRFile struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	IsNew     bool   `json:"is_new"`
	IsDeleted bool   `json:"is_deleted"`
	IsRenamed bool   `json:"is_renamed"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Diff      string `json:"diff,omitempty"` // populated only with --show-diff
}

// MRNote represents a comment/note on a merge request
type MRNote struct {
	ID           int           `json:"id"`
	DiscussionID string        `json:"discussion_id,omitempty"`
	Body         string        `json:"body"`
	Author       string        `json:"author"`
	CreatedAt    time.Time     `json:"created_at"`
	System       bool          `json:"system"`
	Resolvable   bool          `json:"resolvable"`
	Resolved     bool          `json:"resolved"`
	Position     *NotePosition `json:"position,omitempty"`
}

// NotePosition contains position information for inline/diff comments
type NotePosition struct {
	NewPath string `json:"new_path"`
	OldPath string `json:"old_path"`
	NewLine int    `json:"new_line"`
	OldLine int    `json:"old_line"`
}

// MRDiscussion represents a discussion thread on a merge request
type MRDiscussion struct {
	ID             string   `json:"id"`
	IndividualNote bool     `json:"individual_note"`
	Notes          []MRNote `json:"notes"`
}

// MRDiffVersion contains version info for creating inline comments
type MRDiffVersion struct {
	HeadCommitSHA  string `json:"head_commit_sha"`
	BaseCommitSHA  string `json:"base_commit_sha"`
	StartCommitSHA string `json:"start_commit_sha"`
}

// Tag represents a git tag
type Tag struct {
	Name      string    `json:"name"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	WebURL    string    `json:"web_url"`
}

// ProjectActivity groups all activity for a single project
type ProjectActivity struct {
	ProjectID     int            `json:"project_id"`
	ProjectName   string         `json:"project_name"`
	ProjectPath   string         `json:"project_path"`
	WebURL        string         `json:"web_url"`
	Commits       []Commit       `json:"commits,omitempty"`
	MergeRequests []MergeRequest `json:"merge_requests,omitempty"`
	Tags          []Tag          `json:"tags,omitempty"`
}

func (p *ProjectActivity) HasActivity() bool {
	return len(p.Commits) > 0 || len(p.MergeRequests) > 0 || len(p.Tags) > 0
}

// ActivitySummary holds aggregate counts across all projects
type ActivitySummary struct {
	TotalProjects      int `json:"total_projects"`
	TotalCommits       int `json:"total_commits"`
	TotalMergeRequests int `json:"total_merge_requests"`
	TotalTags          int `json:"total_tags"`
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

// Contributor represents a project contributor
type Contributor struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Commits   int    `json:"commits"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// ProjectMetadata holds cached project information from the index
type ProjectMetadata struct {
	ID              int                `json:"id"`
	Name            string             `json:"name"`
	PathWithNS      string             `json:"path_with_namespace"`
	Description     string             `json:"description"`
	WebURL          string             `json:"web_url"`
	DefaultBranch   string             `json:"default_branch"`
	Visibility      string             `json:"visibility"`
	Topics          []string           `json:"topics,omitempty"`
	StarCount       int                `json:"star_count"`
	ForksCount      int                `json:"forks_count"`
	Languages       map[string]float32 `json:"languages,omitempty"`
	TopContributors []Contributor      `json:"top_contributors,omitempty"`
	LastActivityAt  time.Time          `json:"last_activity_at"`
	IndexedAt       time.Time          `json:"indexed_at"`
}

// PipelineSummary represents a pipeline in list views
type PipelineSummary struct {
	ID        int       `json:"id"`
	IID       int       `json:"iid"`
	ProjectID int       `json:"project_id"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	Ref       string    `json:"ref"`
	SHA       string    `json:"sha"`
	User      string    `json:"user,omitempty"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PipelineDetail contains full pipeline information
type PipelineDetail struct {
	ID             int           `json:"id"`
	IID            int           `json:"iid"`
	ProjectID      int           `json:"project_id"`
	Status         string        `json:"status"`
	Source         string        `json:"source"`
	Ref            string        `json:"ref"`
	SHA            string        `json:"sha"`
	BeforeSHA      string        `json:"before_sha,omitempty"`
	Tag            bool          `json:"tag"`
	YamlErrors     string        `json:"yaml_errors,omitempty"`
	User           string        `json:"user,omitempty"`
	WebURL         string        `json:"web_url"`
	Duration       int           `json:"duration"` // seconds
	QueuedDuration int           `json:"queued_duration"`
	Coverage       string        `json:"coverage,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	FinishedAt     *time.Time    `json:"finished_at,omitempty"`
	Jobs           []PipelineJob `json:"jobs,omitempty"`
}

// PipelineJob represents a job/build within a pipeline
type PipelineJob struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Stage          string     `json:"stage"`
	Status         string     `json:"status"`
	Ref            string     `json:"ref"`
	Tag            bool       `json:"tag"`
	AllowFailure   bool       `json:"allow_failure"`
	Duration       float64    `json:"duration"`
	QueuedDuration float64    `json:"queued_duration"`
	FailureReason  string     `json:"failure_reason,omitempty"`
	WebURL         string     `json:"web_url"`
	User           string     `json:"user,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

// GitLabIndex is the local project cache stored on disk
type GitLabIndex struct {
	Version         int               `json:"version"`
	GitLabURL       string            `json:"gitlab_url"`
	LastFullIndexAt time.Time         `json:"last_full_index_at"`
	Projects        []ProjectMetadata `json:"projects"`
	ProjectsByID    map[int]int       `json:"-"`
	ProjectsByPath  map[string]int    `json:"-"`
}

func NewGitLabIndex(gitlabURL string) *GitLabIndex {
	return &GitLabIndex{
		Version:        1,
		GitLabURL:      gitlabURL,
		Projects:       []ProjectMetadata{},
		ProjectsByID:   make(map[int]int),
		ProjectsByPath: make(map[string]int),
	}
}

func (idx *GitLabIndex) BuildLookupMaps() {
	idx.ProjectsByID = make(map[int]int)
	idx.ProjectsByPath = make(map[string]int)

	for i, p := range idx.Projects {
		idx.ProjectsByID[p.ID] = i
		idx.ProjectsByPath[p.PathWithNS] = i
	}
}

func (idx *GitLabIndex) FindProject(idOrPath string) *ProjectMetadata {
	if idx.ProjectsByID == nil || idx.ProjectsByPath == nil {
		idx.BuildLookupMaps()
	}

	// Try as ID first
	if id, err := strconv.Atoi(idOrPath); err == nil {
		if i, ok := idx.ProjectsByID[id]; ok {
			return &idx.Projects[i]
		}
	}

	// Try as path
	if i, ok := idx.ProjectsByPath[idOrPath]; ok {
		return &idx.Projects[i]
	}

	return nil
}

func (idx *GitLabIndex) UpsertProject(p ProjectMetadata) {
	if idx.ProjectsByID == nil || idx.ProjectsByPath == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.ProjectsByID[p.ID]; ok {
		idx.Projects[i] = p
		idx.ProjectsByPath[p.PathWithNS] = i
	} else {
		i := len(idx.Projects)
		idx.Projects = append(idx.Projects, p)
		idx.ProjectsByID[p.ID] = i
		idx.ProjectsByPath[p.PathWithNS] = i
	}
}

// ListProjects returns projects sorted by the given field with optional limit.
// filter is an optional substring matched case-insensitively against path and name.
func (idx *GitLabIndex) ListProjects(orderBy, sortDir string, limit int, filter string) []ProjectMetadata {
	if len(idx.Projects) == 0 {
		return nil
	}

	// Make a copy to sort
	projects := make([]ProjectMetadata, len(idx.Projects))
	copy(projects, idx.Projects)

	// Apply filter before sorting/limiting
	if filter != "" {
		f := strings.ToLower(filter)
		filtered := projects[:0]
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.PathWithNS), f) ||
				strings.Contains(strings.ToLower(p.Name), f) {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	// Sort based on field
	switch orderBy {
	case "name":
		if sortDir == "desc" {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.Name > b.Name })
		} else {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.Name < b.Name })
		}
	case "path":
		if sortDir == "desc" {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.PathWithNS > b.PathWithNS })
		} else {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.PathWithNS < b.PathWithNS })
		}
	case "created_at":
		// Index doesn't store created_at, fall back to last_activity
		fallthrough
	case "last_activity_at":
		fallthrough
	default:
		if sortDir == "asc" {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.LastActivityAt.Before(b.LastActivityAt) })
		} else {
			sortByProject(projects, func(a, b ProjectMetadata) bool { return a.LastActivityAt.After(b.LastActivityAt) })
		}
	}

	// Apply limit
	if limit > 0 && limit < len(projects) {
		projects = projects[:limit]
	}

	return projects
}

func sortByProject(projects []ProjectMetadata, less func(a, b ProjectMetadata) bool) {
	for i := 0; i < len(projects)-1; i++ {
		for j := i + 1; j < len(projects); j++ {
			if less(projects[j], projects[i]) {
				projects[i], projects[j] = projects[j], projects[i]
			}
		}
	}
}
