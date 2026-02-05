package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/gitlab"
	"github.com/codewandler/dex/internal/jira"
	"github.com/codewandler/dex/internal/k8s"
	"github.com/codewandler/dex/internal/slack"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	doctorHeader  = color.New(color.FgCyan, color.Bold)
	doctorSuccess = color.New(color.FgGreen)
	doctorError   = color.New(color.FgRed)
	doctorWarn    = color.New(color.FgYellow)
	doctorDim     = color.New(color.FgHiBlack)
	doctorLabel   = color.New(color.FgWhite)
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check integration health",
	Long: `Check the health of all configured integrations.

Tests connectivity and authentication for GitLab, Jira, Slack, and Kubernetes.

Examples:
  dex doctor`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			doctorError.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

		doctorHeader.Println("dex doctor")
		fmt.Println()

		ctx := context.Background()
		errors := 0
		warnings := 0

		// Check GitLab
		doctorLabel.Print("  GitLab      ")
		if status, warn := checkGitLab(cfg); status != "" {
			fmt.Println(status)
			if warn {
				warnings++
			}
		} else {
			errors++
		}

		// Check Jira
		doctorLabel.Print("  Jira        ")
		if status, warn := checkJira(ctx, cfg); status != "" {
			fmt.Println(status)
			if warn {
				warnings++
			}
		} else {
			errors++
		}

		// Check Slack
		doctorLabel.Print("  Slack       ")
		if status, warn := checkSlack(cfg); status != "" {
			fmt.Println(status)
			if warn {
				warnings++
			}
		} else {
			errors++
		}

		// Check Kubernetes
		doctorLabel.Print("  Kubernetes  ")
		if status, warn := checkKubernetes(ctx); status != "" {
			fmt.Println(status)
			if warn {
				warnings++
			}
		} else {
			errors++
		}

		fmt.Println()
		if errors == 0 && warnings == 0 {
			doctorSuccess.Println("All integrations healthy!")
		} else {
			if errors > 0 {
				doctorError.Printf("%d error(s)", errors)
				if warnings > 0 {
					fmt.Print(", ")
				}
			}
			if warnings > 0 {
				doctorWarn.Printf("%d warning(s)", warnings)
			}
			fmt.Println()
		}
	},
}

// checkGitLab tests GitLab connectivity. Returns status string and whether it's a warning.
// Empty status means error was already printed.
func checkGitLab(cfg *config.Config) (string, bool) {
	if err := cfg.RequireGitLab(); err != nil {
		doctorError.Print("✗ ")
		doctorDim.Print("Not configured ")
		fmt.Print("(run 'dex setup')")
		return "", false
	}

	client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
	if err != nil {
		doctorError.Printf("✗ Failed to create client: %v", err)
		return "", false
	}

	user, err := client.TestAuth()
	if err != nil {
		doctorError.Printf("✗ Auth failed: %v", err)
		return "", false
	}

	return doctorSuccess.Sprint("✓ ") + fmt.Sprintf("@%s ", user.Username) + doctorDim.Sprintf("(%s)", cfg.GitLab.URL), false
}

// checkJira tests Jira connectivity
func checkJira(ctx context.Context, cfg *config.Config) (string, bool) {
	if err := cfg.RequireJira(); err != nil {
		doctorError.Print("✗ ")
		doctorDim.Print("Not configured ")
		fmt.Print("(run 'dex setup')")
		return "", false
	}

	if cfg.Jira.Token == nil {
		doctorError.Print("✗ ")
		doctorDim.Print("Not authenticated ")
		fmt.Print("(run 'dex jira auth')")
		return "", false
	}

	client, err := jira.NewClient()
	if err != nil {
		doctorError.Printf("✗ Failed to create client: %v", err)
		return "", false
	}

	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		doctorError.Printf("✗ Auth failed: %v", err)
		return "", false
	}

	siteURL := cfg.Jira.Token.SiteURL
	if siteURL == "" {
		siteURL = "connected"
	}

	return doctorSuccess.Sprint("✓ ") + user.DisplayName + " " + doctorDim.Sprintf("(%s)", siteURL), false
}

// checkSlack tests Slack connectivity
func checkSlack(cfg *config.Config) (string, bool) {
	if err := cfg.RequireSlack(); err != nil {
		doctorError.Print("✗ ")
		doctorDim.Print("Not configured ")
		fmt.Print("(run 'dex setup')")
		return "", false
	}

	client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
	if err != nil {
		doctorError.Printf("✗ Failed to create client: %v", err)
		return "", false
	}

	botResp, err := client.TestAuth()
	if err != nil {
		doctorError.Printf("✗ Bot auth failed: %v", err)
		return "", false
	}

	status := doctorSuccess.Sprint("✓ ") + fmt.Sprintf("Bot: %s", botResp.User)

	if client.HasUserToken() {
		userResp, err := client.TestUserAuth()
		if err != nil {
			status += doctorDim.Sprint(" | ") + doctorWarn.Sprint("User: ✗")
			return status, true // warning
		}
		status += doctorDim.Sprint(" | ") + fmt.Sprintf("User: @%s", userResp.User)
	} else {
		status += doctorDim.Sprint(" | User: not configured")
	}

	return status, false
}

// checkKubernetes tests Kubernetes connectivity
func checkKubernetes(ctx context.Context) (string, bool) {
	contexts, err := k8s.ListContexts()
	if err != nil {
		doctorError.Print("✗ ")
		doctorDim.Print("No kubeconfig found")
		return "", false
	}

	if len(contexts) == 0 {
		doctorError.Print("✗ ")
		doctorDim.Print("No contexts configured")
		return "", false
	}

	// Find current context
	var currentCtx string
	for _, c := range contexts {
		if c.Current {
			currentCtx = c.Name
			break
		}
	}

	// Try to connect
	client, err := k8s.NewClient("")
	if err != nil {
		doctorError.Printf("✗ Failed to connect: %v", err)
		return "", false
	}

	version, err := client.TestConnection(ctx)
	if err != nil {
		doctorError.Printf("✗ Connection failed: %v", err)
		return "", false
	}

	return doctorSuccess.Sprint("✓ ") + currentCtx + " " + doctorDim.Sprintf("(%s)", version), false
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
