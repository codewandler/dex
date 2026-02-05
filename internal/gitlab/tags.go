package gitlab

import (
	"time"

	"dex/internal/models"

	"github.com/xanzy/go-gitlab"
)

func (c *Client) GetTags(projectID int, since time.Time) ([]models.Tag, error) {
	var allTags []models.Tag

	opts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	for {
		tags, resp, err := c.gl.Tags.ListTags(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, t := range tags {
			var createdAt time.Time
			if t.Commit != nil && t.Commit.CreatedAt != nil {
				createdAt = *t.Commit.CreatedAt
			}

			// Only include tags created after the cutoff
			if !createdAt.IsZero() && createdAt.Before(since) {
				continue
			}

			tag := models.Tag{
				Name:      t.Name,
				CreatedAt: createdAt,
			}
			if t.Message != "" {
				tag.Message = t.Message
			}
			allTags = append(allTags, tag)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}
