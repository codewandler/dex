package providers

import (
	"context"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/gh"
)

// GitHubProvider fetches GitHub PR/issue status
type GitHubProvider struct{}

func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) IsConfigured(_ *config.Config) bool {
	// GitHub uses gh CLI auth, always try to use it
	return true
}

func (p *GitHubProvider) Fetch(ctx context.Context) (map[string]any, error) {
	data := map[string]any{
		"PRs":    0,
		"Issues": 0,
	}

	client := gh.NewClient()

	// Get issues assigned to me
	issues, err := client.IssueList(gh.IssueListOptions{
		State:    "open",
		Assignee: "@me",
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}
	data["Issues"] = len(issues)

	// Get PRs assigned to me
	prs, err := client.PRList(gh.PRListOptions{
		State:    "open",
		Assignee: "@me",
		Limit:    100,
	})
	if err != nil {
		// Non-fatal, just don't show PR count
		return data, nil
	}
	data["PRs"] = len(prs)

	return data, nil
}
