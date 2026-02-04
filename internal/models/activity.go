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
