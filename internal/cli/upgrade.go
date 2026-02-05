package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const installPath = "github.com/codewandler/dex"

var upgradeVersion string

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade dex to the latest or specified version",
	Long: `Upgrade dex by running go install.

By default, installs the latest version. Use --version to install a specific version.

Examples:
  dex upgrade              # Install latest version
  dex upgrade -v v0.2.0    # Install specific version`,
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

		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeVersion, "version", "v", "", "Version to install (e.g., v0.2.0)")
}
