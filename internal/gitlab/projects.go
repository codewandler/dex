package gitlab

import (
	"time"

	"github.com/xanzy/go-gitlab"
)

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
