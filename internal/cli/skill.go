package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/codewandler/dex/internal/skills"
	"github.com/codewandler/dex/internal/skillssh"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:     "skill",
	Aliases: []string{"skills"},
	Short:   "Manage and search skills",
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

var skillSearchLimit int

var skillSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for skills on skills.sh",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		client := skillssh.NewClient()
		result, err := client.Search(query, skillSearchLimit)
		if err != nil {
			return err
		}

		if len(result.Skills) == 0 {
			fmt.Printf("No skills found for %q\n", query)
			return nil
		}

		// Colors
		nameColor := color.New(color.FgCyan, color.Bold)
		sourceColor := color.New(color.FgYellow)
		dimColor := color.New(color.FgHiBlack)

		line := strings.Repeat("═", 60)
		fmt.Println()
		fmt.Println(line)
		fmt.Printf("  Skills matching %q (%d results)\n", query, result.Count)
		fmt.Println(line)
		fmt.Println()

		for _, skill := range result.Skills {
			nameColor.Printf("  %s\n", skill.Name)
			sourceColor.Printf("    %s", skill.Source)
			dimColor.Printf("  (%d installs)\n", skill.Installs)
		}

		fmt.Println()
		return nil
	},
}

func init() {
	skillSearchCmd.Flags().IntVarP(&skillSearchLimit, "limit", "n", 10, "Maximum number of results")

	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillSearchCmd)
}
