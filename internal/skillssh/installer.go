package skillssh

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/codewandler/dex/internal/gh"
)

// Installer handles downloading and installing skills from GitHub
type Installer struct {
	ghClient *gh.Client
}

// NewInstaller creates a new skill installer
func NewInstaller() *Installer {
	return &Installer{
		ghClient: gh.NewClient(),
	}
}

// Install downloads a skill from GitHub and installs it to the target directory.
// It clones the repository using gh CLI, copies the skill files, and cleans up.
func (i *Installer) Install(skill *Skill, targetDir string) (int, error) {
	// Check if gh CLI is available
	if !i.ghClient.IsAvailable() {
		return 0, fmt.Errorf("gh CLI is not available or not authenticated. Run 'gh auth login' first")
	}

	// Clone to temp directory
	repoPath, err := i.ghClient.CloneToTemp(skill.Source)
	if err != nil {
		return 0, fmt.Errorf("failed to clone repository: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(repoPath)) // Clean up temp directory

	// Find the skill directory in the cloned repo
	skillSourceDir := filepath.Join(repoPath, "skills", skill.Name)
	if _, err := os.Stat(skillSourceDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("skill %q not found in repository %s (expected at skills/%s/)", skill.Name, skill.Source, skill.Name)
	}

	// Create target directory
	skillTargetDir := filepath.Join(targetDir, skill.Name)
	if err := os.MkdirAll(skillTargetDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory: %w", err)
	}

	// Copy all files from the skill directory
	installed := 0
	err = filepath.WalkDir(skillSourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path within skill directory
		relPath, err := filepath.Rel(skillSourceDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(skillTargetDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Read and write file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}

		installed++
		return nil
	})

	if err != nil {
		return installed, fmt.Errorf("failed to copy skill files: %w", err)
	}

	return installed, nil
}
