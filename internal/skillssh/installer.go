package skillssh

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GitHubTreeEntry represents an entry in the GitHub tree API response
type GitHubTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitHubTreeResponse represents the GitHub tree API response
type GitHubTreeResponse struct {
	SHA       string            `json:"sha"`
	URL       string            `json:"url"`
	Tree      []GitHubTreeEntry `json:"tree"`
	Truncated bool              `json:"truncated"`
}

// GitHubBlobResponse represents the GitHub blob API response
type GitHubBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

// Installer handles downloading and installing skills from GitHub
type Installer struct {
	httpClient *http.Client
	ghToken    string
}

// NewInstaller creates a new skill installer
func NewInstaller() *Installer {
	return &Installer{
		httpClient: &http.Client{},
		ghToken:    os.Getenv("GITHUB_TOKEN"),
	}
}

// Install downloads a skill from GitHub and installs it to the target directory
func (i *Installer) Install(skill *Skill, targetDir string) (int, error) {
	// Parse source: owner/repo
	parts := strings.Split(skill.Source, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid source format: %s (expected owner/repo)", skill.Source)
	}
	owner, repo := parts[0], parts[1]

	// Get the repository tree to find the skill directory
	skillPath := fmt.Sprintf("skills/%s", skill.Name)
	tree, err := i.getTree(owner, repo, "main")
	if err != nil {
		// Try master branch
		tree, err = i.getTree(owner, repo, "master")
		if err != nil {
			return 0, fmt.Errorf("failed to get repository tree: %w", err)
		}
	}

	// Find all files under the skill path
	var filesToDownload []GitHubTreeEntry
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.HasPrefix(entry.Path, skillPath+"/") {
			filesToDownload = append(filesToDownload, entry)
		}
	}

	if len(filesToDownload) == 0 {
		return 0, fmt.Errorf("skill %q not found in repository %s", skill.Name, skill.Source)
	}

	// Create target directory
	skillTargetDir := filepath.Join(targetDir, skill.Name)
	if err := os.MkdirAll(skillTargetDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory: %w", err)
	}

	// Download and install each file
	installed := 0
	for _, entry := range filesToDownload {
		// Get relative path within skill directory
		relPath := strings.TrimPrefix(entry.Path, skillPath+"/")
		destPath := filepath.Join(skillTargetDir, relPath)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return installed, fmt.Errorf("failed to create directory for %s: %w", relPath, err)
		}

		// Download file content
		content, err := i.getBlob(entry.URL)
		if err != nil {
			return installed, fmt.Errorf("failed to download %s: %w", relPath, err)
		}

		// Write file
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return installed, fmt.Errorf("failed to write %s: %w", relPath, err)
		}

		installed++
	}

	return installed, nil
}

func (i *Installer) getTree(owner, repo, ref string) (*GitHubTreeResponse, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, ref)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if i.ghToken != "" {
		req.Header.Set("Authorization", "token "+i.ghToken)
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var tree GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}

	return &tree, nil
}

func (i *Installer) getBlob(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if i.ghToken != "" {
		req.Header.Set("Authorization", "token "+i.ghToken)
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var blob GitHubBlobResponse
	if err := json.NewDecoder(resp.Body).Decode(&blob); err != nil {
		return nil, err
	}

	if blob.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", blob.Encoding)
	}

	// Remove newlines from base64 content (GitHub adds them)
	content := strings.ReplaceAll(blob.Content, "\n", "")
	return base64.StdEncoding.DecodeString(content)
}
