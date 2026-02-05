package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const installPath = "github.com/codewandler/dex"

var upgradeVersion string
var upgradeNoInstall bool

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade dex to the latest or specified version",
	Long: `Upgrade dex by running go install.

By default, installs the latest version and updates skill files.
Use --version to install a specific version.
Use --no-install to skip updating skill files.

Examples:
  dex upgrade              # Install latest version + update skills
  dex upgrade -v v0.2.0    # Install specific version + update skills
  dex upgrade --no-install # Install latest version only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		version := "latest"
		if upgradeVersion != "" {
			version = upgradeVersion
		}

		installArg := fmt.Sprintf("%s@%s", installPath, version)
		fmt.Printf("Installing %s...\n", installArg)

		goCmd := exec.Command("go", "install", installArg)
		goCmd.Stdout = os.Stdout
		goCmd.Stderr = os.Stderr

		if err := goCmd.Run(); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}

		fmt.Println("Upgrade complete!")

		// Show new version
		dexCmd := exec.Command("dex", "version")
		dexCmd.Stdout = os.Stdout
		dexCmd.Stderr = os.Stderr
		dexCmd.Run() // Ignore error - just informational

		// Install skill files unless --no-install is set
		if !upgradeNoInstall {
			fmt.Println()
			installed, err := RunDexInstall()
			if err != nil {
				return fmt.Errorf("skill install failed: %w", err)
			}
			homeDir, _ := os.UserHomeDir()
			skillDir := filepath.Join(homeDir, ".claude", "skills", "dex")
			fmt.Printf("Installed %d skill files to %s\n", installed, skillDir)
		}

		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeVersion, "version", "v", "", "Version to install (e.g., v0.2.0)")
	upgradeCmd.Flags().BoolVar(&upgradeNoInstall, "no-install", false, "Skip installing skill files after upgrade")
}
