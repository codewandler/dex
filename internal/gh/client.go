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
	cmd := exec.Command("gh", "auth", "status", "--json", "username")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	var result struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		// Fallback: try to get user from api
		return c.getCurrentUser()
	}

	return &AuthStatus{Username: result.Username}, nil
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
