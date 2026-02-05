package providers

import (
	"context"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/gitlab"
)

// GitLabProvider fetches GitLab MR status
type GitLabProvider struct {
	cfg *config.Config
}

func NewGitLabProvider(cfg *config.Config) *GitLabProvider {
	return &GitLabProvider{cfg: cfg}
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) IsConfigured(cfg *config.Config) bool {
	return cfg.RequireGitLab() == nil
}

func (p *GitLabProvider) Fetch(ctx context.Context) (map[string]any, error) {
	data := map[string]any{
		"Assigned":  0,
		"Reviewing": 0,
	}

	client, err := gitlab.NewClient(p.cfg.GitLab.URL, p.cfg.GitLab.Token)
	if err != nil {
		return nil, err
	}

	// Get MRs assigned to me
	assignedMRs, err := client.ListMergeRequests(gitlab.ListMergeRequestsOptions{
		Scope: "assigned_to_me",
		State: "opened",
	})
	if err != nil {
		return nil, err
	}
	data["Assigned"] = len(assignedMRs)

	// Get all open MRs and filter by reviewer (no API filter available)
	allMRs, err := client.ListMergeRequests(gitlab.ListMergeRequestsOptions{
		Scope: "all",
		State: "opened",
		Limit: 100,
	})
	if err != nil {
		// Non-fatal, just don't show reviewing count
		return data, nil
	}

	// Get current user to filter reviewers
	user, err := client.TestAuth()
	if err != nil {
		return data, nil
	}

	// Count MRs where I'm a reviewer
	var reviewing int
	for _, mr := range allMRs {
		for _, reviewer := range mr.Reviewers {
			if reviewer == user.Username {
				reviewing++
				break
			}
		}
	}
	data["Reviewing"] = reviewing

	return data, nil
}
