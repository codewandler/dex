package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"dex/internal/skills"

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
		content, err := skills.DexSkill()
		if err != nil {
			return fmt.Errorf("failed to read embedded skill: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		skillDir := filepath.Join(homeDir, ".claude", "skills", "dex")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("failed to create skill directory: %w", err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write skill file: %w", err)
		}

		fmt.Printf("Installed dex skill to %s\n", skillPath)
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
