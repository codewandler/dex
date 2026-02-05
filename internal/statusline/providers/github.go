package providers

import (
	"context"
	"os/exec"
	"strings"

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
		"PRs":       0,
		"Issues":    0,
		"RepoName":  "",
		"Reviewing": 0,
	}

	client := gh.NewClient()

	// Try to get current repo name for context
	repoName := getCurrentRepoName()
	if repoName != "" {
		data["RepoName"] = repoName
	}

	// Search for issues assigned to me globally
	issues, err := client.SearchIssues(gh.SearchIssuesOptions{
		Assignee: "@me",
		State:    "open",
		Limit:    100,
	})
	if err == nil {
		data["Issues"] = len(issues)
	}

	// Search for PRs assigned to me globally
	prs, err := client.SearchPRs(gh.SearchPRsOptions{
		Assignee: "@me",
		State:    "open",
		Limit:    100,
	})
	if err == nil {
		data["PRs"] = len(prs)
	}

	// Search for PRs where I'm requested as reviewer
	reviewing, err := client.SearchPRs(gh.SearchPRsOptions{
		ReviewRequest: "@me",
		State:         "open",
		Limit:         100,
	})
	if err == nil {
		data["Reviewing"] = len(reviewing)
	}

	// If no assigned issues globally but we're in a repo, show repo issues
	if data["Issues"] == 0 && repoName != "" {
		repoIssues, err := client.IssueList(gh.IssueListOptions{
			State: "open",
			Limit: 100,
		})
		if err == nil && len(repoIssues) > 0 {
			data["Issues"] = len(repoIssues)
			data["RepoIssues"] = true // Flag to indicate these are repo issues, not assigned
		}
	}

	return data, nil
}

// getCurrentRepoName returns the current git repo name if in a git repo
func getCurrentRepoName() string {
	cmd := exec.Command("gh", "repo", "view", "--json", "name", "-q", ".name")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
