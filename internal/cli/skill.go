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

var skillInstallGlobal bool
var skillInstallSource string

var skillInstallCmd = &cobra.Command{
	Use:   "install <skill-name>",
	Short: "Install a skill from skills.sh",
	Long: `Install a skill from skills.sh to your local or global Claude skills directory.

By default, skills are installed to ./.claude/skills/ (local project).
Use --global to install to ~/.claude/skills/ instead.

Examples:
  dex skill install kubernetes-specialist
  dex skill install kubernetes-specialist --global
  dex skill install kubernetes-specialist --source jeffallan/claude-skills`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: skillNameCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		skillName := args[0]

		client := skillssh.NewClient()
		installer := skillssh.NewInstaller()

		// Resolve skill from skills.sh or use provided source
		var skill *skillssh.Skill
		if skillInstallSource != "" {
			skill = &skillssh.Skill{
				Name:   skillName,
				Source: skillInstallSource,
			}
		} else {
			var err error
			skill, err = client.Resolve(skillName)
			if err != nil {
				return err
			}
		}

		// Determine target directory
		var targetDir string
		if skillInstallGlobal {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			targetDir = filepath.Join(homeDir, ".claude", "skills")
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			targetDir = filepath.Join(cwd, ".claude", "skills")
		}

		fmt.Printf("Installing %s from %s...\n", skill.Name, skill.Source)

		installed, err := installer.Install(skill, targetDir)
		if err != nil {
			return err
		}

		fmt.Printf("Installed %d files to %s/%s\n", installed, targetDir, skill.Name)
		return nil
	},
}

// skillNameCompletion provides tab completion for skill names
func skillNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client := skillssh.NewClient()

	// If user is typing, search for matching skills
	if toComplete != "" {
		result, err := client.Search(toComplete, 10)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var completions []string
		seen := make(map[string]bool)
		for _, skill := range result.Skills {
			if !seen[skill.Name] {
				completions = append(completions, fmt.Sprintf("%s\t%s (%d installs)", skill.Name, skill.Source, skill.Installs))
				seen[skill.Name] = true
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}

	// Show popular skills when no input yet
	skills, err := client.ListPopular(20)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	seen := make(map[string]bool)
	for _, skill := range skills {
		if !seen[skill.Name] {
			completions = append(completions, fmt.Sprintf("%s\t%s (%d installs)", skill.Name, skill.Source, skill.Installs))
			seen[skill.Name] = true
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// RunDexInstall installs the dex skill files to ~/.claude/skills/dex/
// Returns the number of files installed and any error.
func RunDexInstall() (int, error) {
	skillFS, err := skills.DexSkillFS()
	if err != nil {
		return 0, fmt.Errorf("failed to get embedded skill filesystem: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("failed to get home directory: %w", err)
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
		return 0, fmt.Errorf("failed to install skill files: %w", err)
	}

	return installed, nil
}

var dexInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the dex skill to ~/.claude/skills/dex/",
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, err := RunDexInstall()
		if err != nil {
			return err
		}

		homeDir, _ := os.UserHomeDir()
		skillDir := filepath.Join(homeDir, ".claude", "skills", "dex")
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
			// OSC 8 hyperlink: \033]8;;URL\007TEXT\033]8;;\007
			githubURL := fmt.Sprintf("https://github.com/%s", skill.Source)
			fmt.Printf("    \033]8;;%s\007", githubURL)
			sourceColor.Printf("%s", skill.Source)
			fmt.Print("\033]8;;\007")
			dimColor.Printf("  (%d installs)\n", skill.Installs)
		}

		fmt.Println()
		return nil
	},
}

func init() {
	skillSearchCmd.Flags().IntVarP(&skillSearchLimit, "limit", "n", 10, "Maximum number of results")

	skillInstallCmd.Flags().BoolVarP(&skillInstallGlobal, "global", "g", false, "Install to ~/.claude/skills/ instead of ./.claude/skills/")
	skillInstallCmd.Flags().StringVarP(&skillInstallSource, "source", "s", "", "GitHub source (owner/repo) to install from")

	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillSearchCmd)

	rootCmd.AddCommand(dexInstallCmd)
}
