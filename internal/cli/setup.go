package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/gitlab"
	"github.com/codewandler/dex/internal/jira"
	"github.com/codewandler/dex/internal/slack"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	setupHeader  = color.New(color.FgCyan, color.Bold)
	setupSuccess = color.New(color.FgGreen)
	setupError   = color.New(color.FgRed)
	setupDim     = color.New(color.FgHiBlack)
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure integrations interactively",
	Long: `Interactive setup wizard for dex integrations.

Walks through configuration for GitLab, Jira, and Slack.
Only prompts for integrations that aren't already configured and working.

Examples:
  dex setup`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		reader := bufio.NewReader(os.Stdin)

		// Load full config (including env vars) to check what's working
		fullCfg, err := config.Load()
		if err != nil {
			setupError.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

		// Load file-only config for modifications (so we don't lose env-only values)
		cfg, err := config.LoadFromFile()
		if err != nil {
			setupError.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
			os.Exit(1)
		}

		setupHeader.Println("dex setup")
		fmt.Println()

		configured := 0
		skipped := 0

		// GitLab
		if isGitLabWorking(fullCfg) {
			setupSuccess.Print("  GitLab      ")
			fmt.Println("✓ Already configured")
			skipped++
		} else {
			if setupGitLab(ctx, cfg, reader) {
				configured++
			}
		}

		// Jira
		if isJiraWorking(ctx, fullCfg) {
			setupSuccess.Print("  Jira        ")
			fmt.Println("✓ Already configured")
			skipped++
		} else {
			if setupJira(ctx, cfg, reader) {
				configured++
			}
		}

		// Slack
		if isSlackWorking(fullCfg) {
			setupSuccess.Print("  Slack       ")
			fmt.Println("✓ Already configured")
			skipped++
		} else {
			if setupSlack(ctx, cfg, reader) {
				configured++
			}
		}

		fmt.Println()
		if configured > 0 {
			setupSuccess.Printf("Setup complete! Configured %d integration(s).\n", configured)
		} else if skipped > 0 {
			setupSuccess.Println("All integrations already configured!")
		} else {
			setupDim.Println("No integrations configured.")
		}
		setupDim.Println("Run 'dex doctor' to verify connectivity.")
	},
}

// isGitLabWorking checks if GitLab is already configured and working
func isGitLabWorking(cfg *config.Config) bool {
	if cfg.GitLab.URL == "" || cfg.GitLab.Token == "" {
		return false
	}
	client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
	if err != nil {
		return false
	}
	_, err = client.TestAuth()
	return err == nil
}

// isJiraWorking checks if Jira is already configured and working
func isJiraWorking(ctx context.Context, cfg *config.Config) bool {
	if cfg.Jira.ClientID == "" || cfg.Jira.ClientSecret == "" {
		return false
	}
	if cfg.Jira.Token == nil {
		return false
	}
	client, err := jira.NewClient()
	if err != nil {
		return false
	}
	_, err = client.GetCurrentUser(ctx)
	return err == nil
}

// isSlackWorking checks if Slack is already configured and working
func isSlackWorking(cfg *config.Config) bool {
	if cfg.Slack.BotToken == "" {
		return false
	}
	client, err := slack.NewClient(cfg.Slack.BotToken)
	if err != nil {
		return false
	}
	_, err = client.TestAuth()
	return err == nil
}

// setupGitLab prompts for GitLab configuration
func setupGitLab(_ context.Context, cfg *config.Config, reader *bufio.Reader) bool {
	fmt.Println()
	setupHeader.Println("  GitLab")

	if !promptYesNo(reader, "  Configure GitLab?", false) {
		setupDim.Println("  Skipped")
		return false
	}

	// URL
	defaultURL := cfg.GitLab.URL
	if defaultURL == "" {
		defaultURL = "https://gitlab.com"
	}
	cfg.GitLab.URL = strings.TrimSuffix(promptString(reader, fmt.Sprintf("  GitLab URL [%s]", defaultURL), defaultURL), "/")

	// Token
	setupDim.Println("  Create a Personal Access Token at: " + cfg.GitLab.URL + "/-/user_settings/personal_access_tokens")
	setupDim.Println("  Required scopes: api, read_user")
	cfg.GitLab.Token = promptString(reader, "  Personal Access Token", "")

	if cfg.GitLab.Token == "" {
		setupError.Println("  ✗ Token required")
		return false
	}

	// Test
	client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
	if err != nil {
		setupError.Printf("  ✗ Failed to create client: %v\n", err)
		return false
	}

	user, err := client.TestAuth()
	if err != nil {
		setupError.Printf("  ✗ Authentication failed: %v\n", err)
		return false
	}

	// Save
	if err := config.Save(cfg); err != nil {
		setupError.Printf("  ✗ Failed to save config: %v\n", err)
		return false
	}

	setupSuccess.Printf("  ✓ Connected as @%s\n", user.Username)
	return true
}

// setupJira prompts for Jira configuration
func setupJira(ctx context.Context, cfg *config.Config, reader *bufio.Reader) bool {
	fmt.Println()
	setupHeader.Println("  Jira")

	if !promptYesNo(reader, "  Configure Jira?", false) {
		setupDim.Println("  Skipped")
		return false
	}

	setupDim.Println("  Create an OAuth 2.0 app at: https://developer.atlassian.com/console/myapps/")
	setupDim.Println("  Add callback URL: http://localhost:8089/callback (HTTP, not HTTPS!)")
	setupDim.Println("  Required scopes: read:jira-work, read:jira-user")

	cfg.Jira.ClientID = promptString(reader, "  OAuth Client ID", cfg.Jira.ClientID)
	cfg.Jira.ClientSecret = promptString(reader, "  OAuth Client Secret", cfg.Jira.ClientSecret)

	if cfg.Jira.ClientID == "" || cfg.Jira.ClientSecret == "" {
		setupError.Println("  ✗ Client ID and Secret required")
		return false
	}

	// Save OAuth credentials first
	if err := config.Save(cfg); err != nil {
		setupError.Printf("  ✗ Failed to save config: %v\n", err)
		return false
	}

	// Run OAuth flow
	setupDim.Println("  Starting OAuth flow...")
	client, err := jira.NewClient()
	if err != nil {
		setupError.Printf("  ✗ Failed to create client: %v\n", err)
		return false
	}

	if err := client.EnsureAuth(ctx); err != nil {
		setupError.Printf("  ✗ Authentication failed: %v\n", err)
		return false
	}

	// Reload config to get the token that was saved
	cfg, _ = config.LoadFromFile()

	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		setupError.Printf("  ✗ Failed to verify: %v\n", err)
		return false
	}

	setupSuccess.Printf("  ✓ Connected as %s\n", user.DisplayName)
	return true
}

// setupSlack prompts for Slack configuration
func setupSlack(ctx context.Context, cfg *config.Config, reader *bufio.Reader) bool {
	fmt.Println()
	setupHeader.Println("  Slack")

	if !promptYesNo(reader, "  Configure Slack?", false) {
		setupDim.Println("  Skipped")
		return false
	}

	fmt.Println("  Setup method:")
	fmt.Println("    1. OAuth (recommended)")
	fmt.Println("    2. Manual tokens")
	choice := promptString(reader, "  Choice [1]", "1")

	if choice == "2" {
		return setupSlackManual(cfg, reader)
	}

	return setupSlackOAuth(ctx, cfg, reader)
}

func setupSlackOAuth(ctx context.Context, cfg *config.Config, reader *bufio.Reader) bool {
	setupDim.Println("  Create a Slack app at: https://api.slack.com/apps")
	setupDim.Println("  Add OAuth redirect URL: https://localhost:8089/callback (HTTPS, not HTTP!)")
	setupDim.Println("  Bot scopes: channels:history, channels:read, chat:write, groups:history, groups:read, im:history, im:read, im:write, users:read")
	setupDim.Println("  User scopes: search:read, users:read")

	cfg.Slack.ClientID = promptString(reader, "  OAuth Client ID", cfg.Slack.ClientID)
	cfg.Slack.ClientSecret = promptString(reader, "  OAuth Client Secret", cfg.Slack.ClientSecret)

	if cfg.Slack.ClientID == "" || cfg.Slack.ClientSecret == "" {
		setupError.Println("  ✗ Client ID and Secret required")
		return false
	}

	// Save OAuth credentials first
	if err := config.Save(cfg); err != nil {
		setupError.Printf("  ✗ Failed to save config: %v\n", err)
		return false
	}

	// Run OAuth flow
	setupDim.Println("  Starting OAuth flow...")
	flow := slack.NewOAuthFlow(cfg)
	token, err := flow.StartAuthServer(ctx)
	if err != nil {
		setupError.Printf("  ✗ Authentication failed: %v\n", err)
		return false
	}

	// Update config with tokens
	cfg.Slack.BotToken = token.AccessToken
	cfg.Slack.UserToken = token.UserToken
	cfg.Slack.Token = token

	if err := config.Save(cfg); err != nil {
		setupError.Printf("  ✗ Failed to save config: %v\n", err)
		return false
	}

	// Test
	client, err := slack.NewClient(cfg.Slack.BotToken)
	if err != nil {
		setupError.Printf("  ✗ Failed to create client: %v\n", err)
		return false
	}

	resp, err := client.TestAuth()
	if err != nil {
		setupError.Printf("  ✗ Verification failed: %v\n", err)
		return false
	}

	setupSuccess.Printf("  ✓ Connected as %s\n", resp.User)
	return true
}

func setupSlackManual(cfg *config.Config, reader *bufio.Reader) bool {
	setupDim.Println("  Enter your Slack tokens from your app's OAuth page.")

	cfg.Slack.BotToken = promptString(reader, "  Bot Token (xoxb-...)", cfg.Slack.BotToken)

	if cfg.Slack.BotToken == "" {
		setupError.Println("  ✗ Bot token required")
		return false
	}

	cfg.Slack.UserToken = promptString(reader, "  User Token (xoxp-..., optional)", cfg.Slack.UserToken)

	// Test
	client, err := slack.NewClient(cfg.Slack.BotToken)
	if err != nil {
		setupError.Printf("  ✗ Failed to create client: %v\n", err)
		return false
	}

	resp, err := client.TestAuth()
	if err != nil {
		setupError.Printf("  ✗ Bot authentication failed: %v\n", err)
		return false
	}

	// Save
	if err := config.Save(cfg); err != nil {
		setupError.Printf("  ✗ Failed to save config: %v\n", err)
		return false
	}

	setupSuccess.Printf("  ✓ Connected as %s\n", resp.User)
	return true
}

// promptYesNo asks a yes/no question
func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}

	fmt.Printf("%s %s: ", prompt, suffix)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

// promptString asks for a string input
func promptString(reader *bufio.Reader, prompt, defaultValue string) string {
	fmt.Printf("%s: ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}
	return input
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
