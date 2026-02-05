package gh

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps the gh CLI for GitHub operations
type Client struct{}

// NewClient creates a new GitHub CLI wrapper
func NewClient() *Client {
	return &Client{}
}

// Clone clones a GitHub repository to the specified destination.
// repoURL can be:
//   - Full URL: https://github.com/owner/repo
//   - Short form: owner/repo
//
// If dest is empty, clones to the repo name in current directory.
func (c *Client) Clone(repoURL, dest string) error {
	// Normalize the repo URL
	repo := normalizeRepo(repoURL)

	args := []string{"repo", "clone", repo}
	if dest != "" {
		args = append(args, dest)
	}

	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh repo clone failed: %w", err)
	}

	return nil
}

// CloneToTemp clones a repository to a temporary directory and returns the path.
// The caller is responsible for cleaning up the directory.
func (c *Client) CloneToTemp(repoURL string) (string, error) {
	repo := normalizeRepo(repoURL)

	// Extract repo name for the temp dir
	parts := strings.Split(repo, "/")
	repoName := parts[len(parts)-1]

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "gh-clone-"+repoName+"-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	dest := filepath.Join(tempDir, repoName)

	args := []string{"repo", "clone", repo, dest}
	cmd := exec.Command("gh", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("gh repo clone failed: %s: %w", string(output), err)
	}

	return dest, nil
}

// IsAvailable checks if the gh CLI is installed and authenticated
func (c *Client) IsAvailable() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

// IsInstalled checks if the gh CLI is installed (regardless of auth status)
func (c *Client) IsInstalled() bool {
	cmd := exec.Command("gh", "--version")
	return cmd.Run() == nil
}

// AuthStatus represents the authentication status from gh CLI
type AuthStatus struct {
	Username string
	Protocol string
}

// GetAuthStatus returns the current authentication status
func (c *Client) GetAuthStatus() (*AuthStatus, error) {
	cmd := exec.Command("gh", "auth", "status", "--json", "hosts")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	var result struct {
		Hosts map[string][]struct {
			Login       string `json:"login"`
			Active      bool   `json:"active"`
			GitProtocol string `json:"gitProtocol"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		// Fallback: try to get user from api
		return c.getCurrentUser()
	}

	// Find the active account on github.com
	for _, accounts := range result.Hosts {
		for _, account := range accounts {
			if account.Active {
				return &AuthStatus{
					Username: account.Login,
					Protocol: account.GitProtocol,
				}, nil
			}
		}
	}

	// No active account found, try fallback
	return c.getCurrentUser()
}

// Login runs gh auth login interactively
func (c *Client) Login() error {
	cmd := exec.Command("gh", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh auth login failed: %w", err)
	}

	return nil
}

// getCurrentUser gets the current user via the API
func (c *Client) getCurrentUser() (*AuthStatus, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	username := strings.TrimSpace(string(output))
	if username == "" {
		return nil, fmt.Errorf("empty username returned")
	}

	return &AuthStatus{Username: username}, nil
}

// Issue represents a GitHub issue
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
	CreatedAt string   `json:"createdAt"`
	URL       string   `json:"url"`
	Body      string   `json:"body"`
}

// IssueListOptions contains options for listing issues
type IssueListOptions struct {
	State    string // open, closed, all
	Label    string
	Assignee string
	Limit    int
	Repo     string // optional: owner/repo
}

// IssueList lists issues in a repository
func (c *Client) IssueList(opts IssueListOptions) ([]Issue, error) {
	args := []string{"issue", "list", "--json", "number,title,state,author,labels,assignees,createdAt,url"}

	if opts.State != "" {
		args = append(args, "--state", opts.State)
	}
	if opts.Label != "" {
		args = append(args, "--label", opts.Label)
	}
	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}
	if opts.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Repo != "" {
		args = append(args, "--repo", opts.Repo)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh issue list failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh issue list failed: %w", err)
	}

	var rawIssues []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		CreatedAt string `json:"createdAt"`
		URL       string `json:"url"`
	}

	if err := json.Unmarshal(output, &rawIssues); err != nil {
		return nil, fmt.Errorf("failed to parse issues: %w", err)
	}

	issues := make([]Issue, len(rawIssues))
	for i, raw := range rawIssues {
		labels := make([]string, len(raw.Labels))
		for j, l := range raw.Labels {
			labels[j] = l.Name
		}
		assignees := make([]string, len(raw.Assignees))
		for j, a := range raw.Assignees {
			assignees[j] = a.Login
		}
		issues[i] = Issue{
			Number:    raw.Number,
			Title:     raw.Title,
			State:     raw.State,
			Author:    raw.Author.Login,
			Labels:    labels,
			Assignees: assignees,
			CreatedAt: raw.CreatedAt,
			URL:       raw.URL,
		}
	}

	return issues, nil
}

// IssueView retrieves a single issue by number
func (c *Client) IssueView(number int, repo string) (*Issue, error) {
	args := []string{"issue", "view", fmt.Sprintf("%d", number), "--json", "number,title,state,author,labels,assignees,createdAt,url,body"}

	if repo != "" {
		args = append(args, "--repo", repo)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh issue view failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh issue view failed: %w", err)
	}

	var raw struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		CreatedAt string `json:"createdAt"`
		URL       string `json:"url"`
		Body      string `json:"body"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse issue: %w", err)
	}

	labels := make([]string, len(raw.Labels))
	for i, l := range raw.Labels {
		labels[i] = l.Name
	}
	assignees := make([]string, len(raw.Assignees))
	for i, a := range raw.Assignees {
		assignees[i] = a.Login
	}

	return &Issue{
		Number:    raw.Number,
		Title:     raw.Title,
		State:     raw.State,
		Author:    raw.Author.Login,
		Labels:    labels,
		Assignees: assignees,
		CreatedAt: raw.CreatedAt,
		URL:       raw.URL,
		Body:      raw.Body,
	}, nil
}

// IssueCreateOptions contains options for creating an issue
type IssueCreateOptions struct {
	Title    string
	Body     string
	Labels   []string
	Assignee string
	Repo     string
}

// IssueCreate creates a new issue
func (c *Client) IssueCreate(opts IssueCreateOptions) (*Issue, error) {
	args := []string{"issue", "create", "--title", opts.Title}

	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	}
	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}
	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}
	if opts.Repo != "" {
		args = append(args, "--repo", opts.Repo)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue create failed: %s", string(output))
	}

	// gh issue create returns the URL of the created issue
	url := strings.TrimSpace(string(output))

	// Extract issue number from URL (e.g., https://github.com/owner/repo/issues/123)
	parts := strings.Split(url, "/")
	if len(parts) < 1 {
		return nil, fmt.Errorf("unexpected output from gh issue create: %s", url)
	}

	var number int
	fmt.Sscanf(parts[len(parts)-1], "%d", &number)

	return &Issue{
		Number: number,
		Title:  opts.Title,
		Body:   opts.Body,
		URL:    url,
		State:  "open",
	}, nil
}

// IssueCloseOptions contains options for closing an issue
type IssueCloseOptions struct {
	Number  int
	Comment string
	Reason  string // "completed" or "not planned"
	Repo    string
}

// IssueClose closes an issue
func (c *Client) IssueClose(opts IssueCloseOptions) error {
	args := []string{"issue", "close", fmt.Sprintf("%d", opts.Number)}

	if opts.Comment != "" {
		args = append(args, "--comment", opts.Comment)
	}
	if opts.Reason != "" {
		args = append(args, "--reason", opts.Reason)
	}
	if opts.Repo != "" {
		args = append(args, "--repo", opts.Repo)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh issue close failed: %s", string(output))
	}

	return nil
}

// normalizeRepo converts various GitHub URL formats to owner/repo format
func normalizeRepo(repoURL string) string {
	// Remove trailing .git
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle full HTTPS URL
	if repo, found := strings.CutPrefix(repoURL, "https://github.com/"); found {
		return repo
	}

	// Handle git@ URL
	if repo, found := strings.CutPrefix(repoURL, "git@github.com:"); found {
		return repo
	}

	// Already in owner/repo format
	return repoURL
}
