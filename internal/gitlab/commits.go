package gitlab

import (
	"time"

	"dev-activity/internal/models"

	"github.com/xanzy/go-gitlab"
)

func (c *Client) GetCommits(projectID int, since time.Time) ([]models.Commit, error) {
	var allCommits []models.Commit

	opts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		Since: gitlab.Ptr(since),
	}

	for {
		commits, resp, err := c.gl.Commits.ListCommits(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, c := range commits {
			commit := models.Commit{
				ID:          c.ID,
				ShortID:     c.ShortID,
				Title:       c.Title,
				AuthorName:  c.AuthorName,
				AuthorEmail: c.AuthorEmail,
				WebURL:      c.WebURL,
			}
			if c.CreatedAt != nil {
				commit.CreatedAt = *c.CreatedAt
			}
			allCommits = append(allCommits, commit)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allCommits, nil
}
