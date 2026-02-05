package providers

import (
	"context"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/jira"
)

// JiraProvider fetches Jira issue status
type JiraProvider struct{}

func NewJiraProvider() *JiraProvider {
	return &JiraProvider{}
}

func (p *JiraProvider) Name() string {
	return "jira"
}

func (p *JiraProvider) IsConfigured(cfg *config.Config) bool {
	return cfg.RequireJira() == nil && cfg.Jira.Token != nil
}

func (p *JiraProvider) Fetch(ctx context.Context) (map[string]any, error) {
	data := map[string]any{
		"Open": 0,
	}

	client, err := jira.NewClient()
	if err != nil {
		return nil, err
	}

	// Ensure we're authenticated
	if err := client.EnsureAuth(ctx); err != nil {
		return nil, err
	}

	// Get my open issues
	issues, err := client.GetMyIssues(ctx, 100)
	if err != nil {
		return nil, err
	}

	data["Open"] = len(issues.Issues)

	return data, nil
}
