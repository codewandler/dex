package gitlab

import (
	"time"

	"github.com/codewandler/dex/internal/models"

	"github.com/xanzy/go-gitlab"
)

// ListProjectCommitsOptions configures the project commit list query
type ListProjectCommitsOptions struct {
	ProjectID string    // project path or numeric ID (required)
	Branch    string    // ref filter (branch/tag)
	Since     time.Time // commits after this time
	Limit     int       // max results (default 20)
}

// ListProjectCommits lists commits for a specific project with filtering
func (c *Client) ListProjectCommits(opts ListProjectCommitsOptions) ([]models.Commit, error) {
	pid, err := c.resolveProjectID(opts.ProjectID)
	if err != nil {
		return nil, err
	}

	if opts.Limit == 0 {
		opts.Limit = 20
	}

	var allCommits []models.Commit

	listOpts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: min(opts.Limit, 100),
			Page:    1,
		},
	}
	if opts.Branch != "" {
		listOpts.RefName = gitlab.Ptr(opts.Branch)
	}
	if !opts.Since.IsZero() {
		listOpts.Since = gitlab.Ptr(opts.Since)
	}

	for {
		commits, resp, err := c.gl.Commits.ListCommits(pid, listOpts)
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

		if len(allCommits) >= opts.Limit || resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	if len(allCommits) > opts.Limit {
		allCommits = allCommits[:opts.Limit]
	}

	return allCommits, nil
}

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

// GetCommit fetches detailed information about a single commit
func (c *Client) GetCommit(projectID interface{}, sha string) (*models.CommitDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	commit, _, err := c.gl.Commits.GetCommit(pid, sha)
	if err != nil {
		return nil, err
	}

	detail := &models.CommitDetail{
		ID:             commit.ID,
		ShortID:        commit.ShortID,
		Title:          commit.Title,
		Message:        commit.Message,
		AuthorName:     commit.AuthorName,
		AuthorEmail:    commit.AuthorEmail,
		CommitterName:  commit.CommitterName,
		CommitterEmail: commit.CommitterEmail,
		WebURL:         commit.WebURL,
		ParentIDs:      commit.ParentIDs,
	}

	if commit.CreatedAt != nil {
		detail.CreatedAt = *commit.CreatedAt
	}
	if commit.CommittedDate != nil {
		detail.CommittedAt = *commit.CommittedDate
	}
	if commit.Stats != nil {
		detail.Stats = models.CommitStats{
			Additions: commit.Stats.Additions,
			Deletions: commit.Stats.Deletions,
			Total:     commit.Stats.Total,
		}
	}

	return detail, nil
}
