package gitlab

import (
	"time"

	"github.com/xanzy/go-gitlab"
)

// ListProjectsOptions configures the ListProjects call
type ListProjectsOptions struct {
	Limit   int    // Maximum number of projects to return (0 = all)
	OrderBy string // Sort field: "created_at", "last_activity_at", "name", "path", "id"
	Sort    string // Sort direction: "asc" or "desc"
}

// ListProjects returns projects with configurable sorting and limit
func (c *Client) ListProjects(opts ListProjectsOptions) ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project

	// Set defaults
	orderBy := opts.OrderBy
	if orderBy == "" {
		orderBy = "last_activity_at"
	}
	sort := opts.Sort
	if sort == "" {
		sort = "desc"
	}

	listOpts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		OrderBy:    gitlab.Ptr(orderBy),
		Sort:       gitlab.Ptr(sort),
		Membership: gitlab.Ptr(true),
	}

	for {
		projects, resp, err := c.gl.Projects.ListProjects(listOpts)
		if err != nil {
			return nil, err
		}

		allProjects = append(allProjects, projects...)

		// Check if we've reached the requested limit
		if opts.Limit > 0 && len(allProjects) >= opts.Limit {
			allProjects = allProjects[:opts.Limit]
			break
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return allProjects, nil
}

func (c *Client) GetActiveProjects(since time.Time) ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project

	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		LastActivityAfter: gitlab.Ptr(since),
		OrderBy:           gitlab.Ptr("last_activity_at"),
		Sort:              gitlab.Ptr("desc"),
		Membership:        gitlab.Ptr(true),
	}

	for {
		projects, resp, err := c.gl.Projects.ListProjects(opts)
		if err != nil {
			return nil, err
		}

		allProjects = append(allProjects, projects...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allProjects, nil
}
