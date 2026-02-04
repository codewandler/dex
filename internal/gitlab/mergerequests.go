package gitlab

import (
	"time"

	"dev-activity/internal/models"

	"github.com/xanzy/go-gitlab"
)

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
