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
