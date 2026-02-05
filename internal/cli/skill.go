package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/codewandler/dex/internal/skills"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage dex skills",
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install dex skill to ~/.claude/skills/dex/",
	RunE: func(cmd *cobra.Command, args []string) error {
		skillFS, err := skills.DexSkillFS()
		if err != nil {
			return fmt.Errorf("failed to get embedded skill filesystem: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		skillDir := filepath.Join(homeDir, ".claude", "skills", "dex")

		// Walk and install all files from embedded FS
		var installed int
		err = fs.WalkDir(skillFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			destPath := filepath.Join(skillDir, path)

			if d.IsDir() {
				return os.MkdirAll(destPath, 0755)
			}

			content, err := fs.ReadFile(skillFS, path)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", path, err)
			}

			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}

			installed++
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to install skill files: %w", err)
		}

		fmt.Printf("Installed %d files to %s\n", installed, skillDir)
		return nil
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print dex skill content to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := skills.DexSkill()
		if err != nil {
			return fmt.Errorf("failed to read embedded skill: %w", err)
		}

		fmt.Print(string(content))
		return nil
	},
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillShowCmd)
}
