package gitlab

import (
	"time"

	"dev-activity/internal/models"

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
