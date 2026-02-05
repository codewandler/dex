package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/jira"
	"github.com/codewandler/dex/internal/models"
	"github.com/codewandler/dex/internal/slack"

	"github.com/spf13/cobra"
)

var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "Slack messaging",
	Long:  `Commands for interacting with Slack.`,
}

var slackAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Slack (opens browser)",
	Long: `Authenticate with Slack using OAuth.

Requires SLACK_CLIENT_ID and SLACK_CLIENT_SECRET to be configured.
Opens your browser to authorize the app, then saves the tokens.

Configure the callback URL in your Slack app: https://localhost:8089/callback

Examples:
  dex slack auth`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		if cfg.Slack.ClientID == "" || cfg.Slack.ClientSecret == "" {
			fmt.Fprintf(os.Stderr, "Slack OAuth not configured.\n")
			fmt.Fprintf(os.Stderr, "Set SLACK_CLIENT_ID and SLACK_CLIENT_SECRET or add to ~/.dex/config.json\n")
			os.Exit(1)
		}

		oauth := slack.NewOAuthFlow(cfg)
		token, err := oauth.StartAuthServer(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("Authentication successful!")
		fmt.Printf("  Team:      %s (%s)\n", token.TeamName, token.TeamID)
		fmt.Printf("  Bot Token: %s...%s\n", token.AccessToken[:10], token.AccessToken[len(token.AccessToken)-4:])
		if token.UserToken != "" {
			fmt.Printf("  User Token: %s...%s\n", token.UserToken[:10], token.UserToken[len(token.UserToken)-4:])
		}
		fmt.Println("\nTokens saved to ~/.dex/config.json")
	},
}

var slackTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Slack authentication",
	Long: `Test the current Slack authentication.

Verifies that the configured tokens are valid and shows identity info.

Examples:
  dex slack test`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		resp, err := client.TestAuth()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Authenticated as: %s\n", resp.User)
		fmt.Printf("Team: %s\n", resp.Team)
		fmt.Printf("Bot ID: %s\n", resp.BotID)
	},
}

var slackInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show authenticated identities",
	Long: `Show who you are from bot and user perspectives.

Displays the authenticated identities for both the bot token and user token.
This helps understand which identity will be used for different operations:

Bot token (SLACK_BOT_TOKEN):
- Sending messages
- Reading channel history
- Listing channels and users

User token (SLACK_USER_TOKEN):
- Search API (search, mentions)
- Actions that require user context

Examples:
  dex slack info`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		// Bot identity
		fmt.Println("Bot Identity (bot token)")
		fmt.Println("────────────────────────────────────────────────────────────────")
		botResp, err := client.TestAuth()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to authenticate: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  Name:      %s\n", botResp.User)
		fmt.Printf("  User ID:   %s\n", botResp.UserID)
		fmt.Printf("  Bot ID:    %s\n", botResp.BotID)
		fmt.Printf("  Team:      %s\n", botResp.Team)
		fmt.Printf("  Team ID:   %s\n", botResp.TeamID)
		fmt.Println()
		fmt.Println("  Used for: sending messages, reading channels, listing users")
		fmt.Println()

		// User identity
		fmt.Println("User Identity (user token)")
		fmt.Println("────────────────────────────────────────────────────────────────")
		if !client.HasUserToken() {
			fmt.Println("  Not configured (set SLACK_USER_TOKEN for search capabilities)")
		} else {
			userResp, err := client.TestUserAuth()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to authenticate: %v\n", err)
			} else {
				fmt.Printf("  Name:      %s\n", userResp.User)
				fmt.Printf("  User ID:   %s\n", userResp.UserID)
				fmt.Printf("  Team:      %s\n", userResp.Team)
				fmt.Printf("  Team ID:   %s\n", userResp.TeamID)
				fmt.Println()
				fmt.Println("  Used for: search API, mentions search")
			}
		}
	},
}

var slackPresenceCmd = &cobra.Command{
	Use:   "presence",
	Short: "Show or set presence status",
	Long: `Show your current presence status or set it.

Without arguments, shows your current presence. Use 'set' subcommand to change it.

Requires user token with:
- users:read scope for viewing presence
- users:write scope for setting presence

Examples:
  dex slack presence              # Show current presence
  dex slack presence set auto     # Set to auto (online when active)
  dex slack presence set away     # Set to away`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		if cfg.Slack.UserToken == "" {
			fmt.Fprintf(os.Stderr, "User token required for presence (set SLACK_USER_TOKEN)\n")
			os.Exit(1)
		}

		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		// Get user ID from user token
		userResp, err := client.TestUserAuth()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get user info: %v\n", err)
			os.Exit(1)
		}

		presence, err := client.GetUserPresence(userResp.UserID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get presence: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("User:     %s (%s)\n", userResp.User, userResp.UserID)
		fmt.Printf("Presence: %s\n", presence.Presence)
		if presence.AutoAway {
			fmt.Println("          (auto away due to inactivity)")
		}
		if presence.ManualAway {
			fmt.Println("          (manually set to away)")
		}
	},
}

var slackPresenceSetCmd = &cobra.Command{
	Use:   "set <auto|away>",
	Short: "Set presence status",
	Long: `Set your presence status.

Valid values:
  auto  - Automatically set based on activity (online when active)
  away  - Manually set to away

Examples:
  dex slack presence set auto     # Back to automatic presence
  dex slack presence set away     # Set to away`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		presenceArg := args[0]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		if cfg.Slack.UserToken == "" {
			fmt.Fprintf(os.Stderr, "User token required for presence (set SLACK_USER_TOKEN)\n")
			os.Exit(1)
		}

		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		if err := client.SetUserPresence(presenceArg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set presence: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Presence set to: %s\n", presenceArg)
	},
}

var slackIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index Slack channels and users",
	Long: `Scan and cache all Slack channels and users.

Index is stored at ~/.dex/slack/index.json and enables:
- Addressing channels by name instead of ID
- Sending DMs via @username
- Fast lookups for autocomplete

Examples:
  dex slack index           # Index if cache is older than 24h
  dex slack index --force   # Force re-index regardless of cache age`,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Check if index is fresh (< 24h old)
		if !force {
			idx, err := slack.LoadIndex()
			if err == nil && !idx.LastFullIndexAt.IsZero() {
				age := time.Since(idx.LastFullIndexAt)
				if age < 24*time.Hour {
					fmt.Printf("Index is fresh (%s old, %d channels, %d users). Use --force to re-index.\n",
						formatSlackIndexAge(age), len(idx.Channels), len(idx.Users))
					return
				}
			}
		}

		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		fmt.Print("Indexing...")
		idx, err := client.IndexAll(
			func(completed, total int) {
				fmt.Printf("\rIndexing channels... %d/%d", completed, total)
			},
			func(completed, total int) {
				fmt.Printf("\rIndexing users... %d/%d   ", completed, total)
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to index: %v\n", err)
			os.Exit(1)
		}

		if err := slack.SaveIndex(idx); err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to save index: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\rIndexed %d channels, %d users for %s\n", len(idx.Channels), len(idx.Users), idx.TeamName)
	},
}

func formatSlackIndexAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

var slackSendCmd = &cobra.Command{
	Use:   "send <channel|@user> <message>",
	Short: "Send a message to a channel or user",
	Long: `Send a message to a Slack channel or user DM.

The target can be:
- Channel name (requires index): dev-team
- Channel ID: C03JDUBJD0D
- Username with @ prefix for DM: @timo.friedl (requires im:write scope)
- User ID: U03HY52RQLV

Use --thread/-t to reply to a specific thread.
Use --as to choose the sender identity (bot or user).
@mentions in the message body are auto-resolved to Slack user mentions.

Examples:
  dex slack send dev-team "Hello from dex!"
  dex slack send dev-team "Hey @timo.friedl check this!"  # @mention in message
  dex slack send dev-team "Follow up" -t 1770257991.873399  # Reply to thread
  dex slack send @timo.friedl "Hey, check this out!"      # DM (requires im:write)
  dex slack send dev-team "Message as me" --as user       # Send as user (not bot)`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeSlackTargets,
	Run: func(cmd *cobra.Command, args []string) {
		targetArg := args[0]
		message := args[1]
		threadTS, _ := cmd.Flags().GetString("thread")
		sendAs, _ := cmd.Flags().GetString("as")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Validate --as flag
		if sendAs != "bot" && sendAs != "user" {
			fmt.Fprintf(os.Stderr, "Invalid --as value: %q (must be 'bot' or 'user')\n", sendAs)
			os.Exit(1)
		}

		// Create client with appropriate token
		var client *slack.Client
		if sendAs == "user" {
			if cfg.Slack.UserToken == "" {
				fmt.Fprintf(os.Stderr, "User token required for --as=user (set SLACK_USER_TOKEN)\n")
				os.Exit(1)
			}
			client, err = slack.NewClient(cfg.Slack.UserToken)
		} else {
			client, err = slack.NewClient(cfg.Slack.BotToken)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		var channelID string

		// Check if target is a user (@username or user ID starting with U)
		if strings.HasPrefix(targetArg, "@") {
			username := strings.TrimPrefix(targetArg, "@")
			userID := slack.ResolveUser(username)

			// Open DM conversation with user
			dmChannelID, err := client.OpenConversation(userID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open DM with user: %v\n", err)
				os.Exit(1)
			}
			channelID = dmChannelID
		} else {
			// Resolve channel name to ID
			channelID = slack.ResolveChannel(targetArg)
		}

		// Resolve @mentions in message body to <@USER_ID> format
		message = slack.ResolveMentions(message)

		var ts string
		if threadTS != "" {
			// Reply to thread
			ts, err = client.ReplyToThread(channelID, threadTS, message)
		} else {
			// New message
			ts, err = client.PostMessage(channelID, message)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send message: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Message sent (ts: %s)\n", ts)
	},
}

var slackChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List Slack channels",
	Long: `List Slack channels from the local index.

By default shows all indexed channels. Use --member to filter to only
channels the bot is a member of (can post to).

Examples:
  dex slack channels              # List all indexed channels
  dex slack channels --member     # Only channels bot can post to
  dex slack channels --no-cache   # Fetch from API instead of index`,
	Run: func(cmd *cobra.Command, args []string) {
		noCache, _ := cmd.Flags().GetBool("no-cache")
		memberOnly, _ := cmd.Flags().GetBool("member")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Use index by default (like GitLab)
		if !noCache {
			idx, err := slack.LoadIndex()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
				os.Exit(1)
			}
			if len(idx.Channels) == 0 {
				fmt.Println("No channels indexed. Run 'dex slack index' first.")
				os.Exit(1)
			}

			printChannelHeader()
			count := 0
			for _, ch := range idx.Channels {
				if memberOnly && !ch.IsMember {
					continue
				}
				printChannel(ch.ID, ch.Name, ch.IsPrivate, ch.IsMember, ch.NumMembers)
				count++
			}
			fmt.Printf("\n%d channels (from index, %s old)\n", count, formatSlackIndexAge(time.Since(idx.LastFullIndexAt)))
			return
		}

		// Fetch from API with --no-cache
		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		channels, err := client.ListChannels()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list channels: %v\n", err)
			os.Exit(1)
		}

		if len(channels) == 0 {
			fmt.Println("No channels found (bot may not be a member of any channels)")
			return
		}

		printChannelHeader()
		count := 0
		for _, ch := range channels {
			if memberOnly && !ch.IsMember {
				continue
			}
			printChannel(ch.ID, ch.Name, ch.IsPrivate, ch.IsMember, ch.NumMembers)
			count++
		}
		fmt.Printf("\n%d channels\n", count)
	},
}

func printChannelHeader() {
	fmt.Printf("%-15s %-30s %-8s %s\n", "ID", "NAME", "MEMBER", "USERS")
	fmt.Println("────────────────────────────────────────────────────────────────────")
}

func printChannel(id, name string, isPrivate, isMember bool, numMembers int) {
	displayName := name
	if isPrivate {
		displayName = "🔒 " + name
	}
	memberStr := ""
	if isMember {
		memberStr = "✓"
	}
	fmt.Printf("%-15s %-30s %-8s %d\n", id, displayName, memberStr, numMembers)
}

var slackUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "List Slack users",
	Long: `List Slack users from the local index.

Examples:
  dex slack users              # List all indexed users
  dex slack users --no-cache   # Fetch from API instead of index`,
	Run: func(cmd *cobra.Command, args []string) {
		noCache, _ := cmd.Flags().GetBool("no-cache")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Use index by default
		if !noCache {
			idx, err := slack.LoadIndex()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load user index: %v\n", err)
				os.Exit(1)
			}
			if len(idx.Users) == 0 {
				fmt.Println("No users indexed. Run 'dex slack index' first.")
				os.Exit(1)
			}

			printUserHeader()
			count := 0
			for _, u := range idx.Users {
				printUser(u.ID, u.Username, u.DisplayName, u.IsBot)
				count++
			}
			fmt.Printf("\n%d users (from index, %s old)\n", count, formatSlackIndexAge(time.Since(idx.LastFullIndexAt)))
			return
		}

		// Fetch from API with --no-cache
		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		users, err := client.ListUsers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list users: %v\n", err)
			os.Exit(1)
		}

		printUserHeader()
		count := 0
		for _, u := range users {
			if u.Deleted || u.ID == "USLACKBOT" {
				continue
			}
			printUser(u.ID, u.Name, u.Profile.DisplayName, u.IsBot)
			count++
		}
		fmt.Printf("\n%d users\n", count)
	},
}

func printUserHeader() {
	fmt.Printf("%-15s %-25s %-30s %s\n", "ID", "USERNAME", "DISPLAY NAME", "TYPE")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")
}

func printUser(id, username, displayName string, isBot bool) {
	userType := "user"
	if isBot {
		userType = "bot"
	}
	fmt.Printf("%-15s %-25s %-30s %s\n", id, username, displayName, userType)
}

// completeSlackUsers provides shell completion for usernames (without @ prefix)
func completeSlackUsers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	idx, err := slack.LoadIndex()
	if err != nil || len(idx.Users) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	toCompleteLower := strings.ToLower(toComplete)

	for _, u := range idx.Users {
		if u.IsBot {
			continue
		}
		if strings.Contains(strings.ToLower(u.Username), toCompleteLower) {
			completions = append(completions, u.Username)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeSlackTargets provides shell completion for channels and @users
func completeSlackTargets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	toCompleteLower := strings.ToLower(toComplete)

	// If starts with @, complete usernames
	if strings.HasPrefix(toComplete, "@") {
		idx, err := slack.LoadIndex()
		if err != nil || len(idx.Users) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		searchTerm := strings.TrimPrefix(toCompleteLower, "@")
		for _, u := range idx.Users {
			if u.IsBot {
				continue
			}
			if strings.Contains(strings.ToLower(u.Username), searchTerm) {
				completions = append(completions, "@"+u.Username)
			}
		}
	} else {
		// Complete channel names
		chIdx, err := slack.LoadIndex()
		if err != nil || len(chIdx.Channels) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		for _, ch := range chIdx.Channels {
			if !ch.IsMember {
				continue
			}
			if strings.Contains(strings.ToLower(ch.Name), toCompleteLower) {
				completions = append(completions, ch.Name)
			}
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

var slackMentionsCmd = &cobra.Command{
	Use:   "mentions",
	Short: "Search for mentions of a user",
	Long: `Search for messages that mention a specific user.

By default shows mentions of the authenticated user (from user token) from today.
Use --bot to search for mentions of the bot instead.
Use --user to search for mentions of a specific user by username or ID.
Use --unhandled to show only pending mentions (no reaction or reply from you).

Status categories:
  Pending  - No reaction or reply from you
  Acked    - You reacted but didn't reply
  Replied  - You replied in the thread

Examples:
  dex slack mentions                    # My mentions (today)
  dex slack mentions --unhandled        # Only pending mentions
  dex slack mentions --bot              # Bot mentions (today)
  dex slack mentions --user timo.friedl # Mentions of a specific user
  dex slack mentions --user U03HY52RQLV # Mentions by user ID
  dex slack mentions --limit 50         # Show more results
  dex slack mentions --since 1h         # Mentions from last hour
  dex slack mentions --since 7d         # Mentions from last 7 days
  dex slack mentions --compact          # Compact table view`,
	Run: func(cmd *cobra.Command, args []string) {
		userArg, _ := cmd.Flags().GetString("user")
		botFlag, _ := cmd.Flags().GetBool("bot")
		limit, _ := cmd.Flags().GetInt("limit")
		compact, _ := cmd.Flags().GetBool("compact")
		sinceStr, _ := cmd.Flags().GetString("since")
		unhandled, _ := cmd.Flags().GetBool("unhandled")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Create client with user token if available (enables search API)
		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		// Load index for name resolution
		idx, _ := slack.LoadIndex()

		// Determine user ID to search for
		var userID string
		var targetDesc string
		if userArg != "" {
			// Explicit user specified
			userID = slack.ResolveUser(userArg)
			targetDesc = userID
		} else if botFlag {
			// Search for bot mentions
			userID, err = client.GetBotUserID()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get bot user ID: %v\n", err)
				os.Exit(1)
			}
			targetDesc = userID + " (bot)"
		} else {
			// Default: search for authenticated user's mentions (requires user token)
			if !client.HasUserToken() {
				fmt.Fprintf(os.Stderr, "User token required for default mentions search.\n")
				fmt.Fprintf(os.Stderr, "Use --bot to search for bot mentions, or configure SLACK_USER_TOKEN.\n")
				os.Exit(1)
			}
			userResp, err := client.TestUserAuth()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get user identity: %v\n", err)
				os.Exit(1)
			}
			userID = userResp.UserID
			targetDesc = userID + " (me)"
		}

		// Collect user IDs and bot IDs for status classification
		var myUserIDs []string
		var myBotIDs []string
		botUserID, _ := client.GetBotUserID()
		if botUserID != "" {
			myUserIDs = append(myUserIDs, botUserID)
		}
		botID, _ := client.GetBotID()
		if botID != "" {
			myBotIDs = append(myBotIDs, botID)
		}
		if client.HasUserToken() {
			if userResp, err := client.TestUserAuth(); err == nil {
				if userResp.UserID != botUserID {
					myUserIDs = append(myUserIDs, userResp.UserID)
				}
			}
		}

		// Parse since duration (defaults to today if not specified)
		var sinceUnix int64
		var sinceDesc string
		if sinceStr != "" {
			duration := parseSlackDuration(sinceStr)
			if duration > 0 {
				sinceTime := time.Now().Add(-duration)
				sinceUnix = sinceTime.Unix()
				sinceDesc = fmt.Sprintf(" since %s", formatSlackSinceTime(sinceTime, duration))
			}
		} else {
			// Default to today (midnight)
			now := time.Now()
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			sinceUnix = midnight.Unix()
			sinceDesc = " since today"
		}

		var mentions []slack.Mention
		var total int

		// Use search API if user token available, otherwise fall back to channel scanning
		if client.HasUserToken() {
			fmt.Printf("Searching all channels for mentions of %s%s...\n", targetDesc, sinceDesc)
			mentions, total, err = client.SearchMentions(userID, limit, sinceUnix)
		} else {
			// Fall back to scanning channels bot is a member of
			if idx == nil || len(idx.Channels) == 0 {
				fmt.Fprintf(os.Stderr, "No channels indexed. Run 'dex slack index' first.\n")
				os.Exit(1)
			}

			var channelIDs []string
			for _, ch := range idx.Channels {
				if ch.IsMember {
					channelIDs = append(channelIDs, ch.ID)
				}
			}

			if len(channelIDs) == 0 {
				fmt.Println("Bot is not a member of any channels.")
				return
			}

			fmt.Printf("Scanning %d channels for mentions of %s%s...\n", len(channelIDs), targetDesc, sinceDesc)
			mentions, err = client.GetMentionsInChannels(userID, channelIDs, limit, sinceUnix)
			total = len(mentions)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get mentions: %v\n", err)
			os.Exit(1)
		}

		if len(mentions) == 0 {
			fmt.Printf("No mentions found for %s\n", targetDesc)
			return
		}

		// Classify mention status (with caching)
		statusCache, _ := slack.LoadMentionStatusCache()
		cacheHits := 0
		fmt.Print("Classifying mentions...")
		for i := range mentions {
			// Use parent thread timestamp if this is a thread reply, otherwise use message timestamp
			classifyTS := mentions[i].Timestamp
			if mentions[i].ThreadTS != "" {
				classifyTS = mentions[i].ThreadTS
			}

			// Check cache first (only Replied/Acked are cached)
			if cached := statusCache.Get(mentions[i].ChannelID, classifyTS); cached != "" {
				mentions[i].Status = cached
				cacheHits++
			} else {
				mentions[i].Status = client.ClassifyMentionStatus(mentions[i].ChannelID, classifyTS, myUserIDs, myBotIDs)
				statusCache.Set(mentions[i].ChannelID, classifyTS, mentions[i].Status)
			}
			fmt.Printf("\rClassifying mentions... %d/%d", i+1, len(mentions))
		}
		fmt.Println()
		if cacheHits > 0 {
			fmt.Printf("(%d cached, %d checked)\n", cacheHits, len(mentions)-cacheHits)
		}
		_ = slack.SaveMentionStatusCache(statusCache)

		// Filter if --unhandled is set
		if unhandled {
			var filtered []slack.Mention
			for _, m := range mentions {
				if m.Status == slack.MentionStatusPending {
					filtered = append(filtered, m)
				}
			}
			mentions = filtered
		}

		if len(mentions) == 0 {
			if unhandled {
				fmt.Printf("No pending mentions found for %s\n", targetDesc)
			} else {
				fmt.Printf("No mentions found for %s\n", targetDesc)
			}
			return
		}

		fmt.Println()

		if compact {
			printMentionHeaderWithStatus()
		}

		for i, m := range mentions {
			channelName := m.ChannelName
			if channelName == "" {
				if ch := idx.FindChannel(m.ChannelID); ch != nil {
					channelName = ch.Name
				} else {
					channelName = m.ChannelID
				}
			}

			username := m.Username
			if username == "" {
				if u := idx.FindUser(m.UserID); u != nil {
					username = u.Username
				} else {
					username = m.UserID
				}
			}

			ts := parseSlackTimestamp(m.Timestamp)

			if compact {
				text := truncateText(m.Text, 50)
				printMentionCompactWithStatus(ts, channelName, username, string(m.Status), text)
			} else {
				text := resolveUserMentions(m.Text, idx)
				printMentionExpandedWithStatus(i+1, ts, channelName, username, string(m.Status), text, m.Permalink)
			}
		}
		if unhandled {
			fmt.Printf("\nFound %d pending mentions\n", len(mentions))
		} else if total > len(mentions) {
			fmt.Printf("\nShowing %d of %d total mentions\n", len(mentions), total)
		} else {
			fmt.Printf("\nFound %d mentions\n", len(mentions))
		}
	},
}

func printMentionHeaderWithStatus() {
	fmt.Printf("%-19s %-20s %-15s %-8s %s\n", "TIME", "CHANNEL", "FROM", "STATUS", "MESSAGE")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────")
}

func printMentionCompactWithStatus(ts, channel, from, status, text string) {
	fmt.Printf("%-19s %-20s %-15s %-8s %s\n", ts, truncateText(channel, 20), truncateText(from, 15), status, text)
}

func printMentionExpandedWithStatus(num int, ts, channel, from, status, text, permalink string) {
	fmt.Printf("── %d ──────────────────────────────────────────────────────────────────────────\n", num)
	fmt.Printf("#%s  •  %s  •  @%s  •  [%s]\n", channel, ts, from, status)
	if permalink != "" {
		fmt.Printf("%s\n", permalink)
	}
	fmt.Println()
	fmt.Println(text)
	fmt.Println()
}

// resolveUserMentions converts <@USER_ID> to @username for readability
func resolveUserMentions(text string, idx *models.SlackIndex) string {
	if idx == nil {
		return text
	}

	result := text
	i := 0
	for i < len(result) {
		// Find <@U pattern
		start := strings.Index(result[i:], "<@")
		if start == -1 {
			break
		}
		start += i

		// Find closing >
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		end += start

		// Extract user ID (skip <@ prefix)
		userID := result[start+2 : end]
		// Handle format like <@U123|display_name>
		if pipeIdx := strings.Index(userID, "|"); pipeIdx != -1 {
			userID = userID[:pipeIdx]
		}

		// Resolve to username
		replacement := "@" + userID
		if u := idx.FindUser(userID); u != nil {
			replacement = "@" + u.Username
		}

		result = result[:start] + replacement + result[end+1:]
		i = start + len(replacement)
	}

	return result
}

func parseSlackTimestamp(ts string) string {
	// Slack timestamp is Unix time with decimal, e.g., "1612345678.123456"
	var sec int64
	fmt.Sscanf(ts, "%d", &sec)
	if sec == 0 {
		return ts
	}
	return time.Unix(sec, 0).Format("2006-01-02 15:04:05")
}

func truncateText(s string, maxLen int) string {
	// Remove newlines and excessive whitespace
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// parseSlackDuration parses a duration string like "30m", "4h", "7d"
func parseSlackDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	// Extract number and unit
	var num int
	var unit string
	for i, c := range s {
		if c < '0' || c > '9' {
			fmt.Sscanf(s[:i], "%d", &num)
			unit = s[i:]
			break
		}
	}

	if num == 0 {
		return 0
	}

	switch unit {
	case "m", "min", "mins":
		return time.Duration(num) * time.Minute
	case "h", "hr", "hrs", "hour", "hours":
		return time.Duration(num) * time.Hour
	case "d", "day", "days":
		return time.Duration(num) * 24 * time.Hour
	case "w", "week", "weeks":
		return time.Duration(num) * 7 * 24 * time.Hour
	default:
		return 0
	}
}

// formatSlackSinceTime returns a human-readable description of the time range
func formatSlackSinceTime(since time.Time, duration time.Duration) string {
	if duration < time.Hour {
		return fmt.Sprintf("%s (%d minutes ago)", since.Format("15:04"), int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%s (%d hours ago)", since.Format("15:04"), int(duration.Hours()))
	}
	return since.Format("2006-01-02")
}

var slackSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Slack messages",
	Long: `Search Slack messages using the search API (requires user token with search:read scope).

Use --tickets to extract and display Jira ticket references from results.
Ticket extraction requires Jira authentication to fetch project keys.

Query supports Slack search syntax:
- from:@username - Messages from a specific user
- in:#channel - Messages in a specific channel
- has:link - Messages containing links
- before:YYYY-MM-DD, after:YYYY-MM-DD - Date filters

Examples:
  dex slack search "deployment"              # Search for deployment
  dex slack search "error" --since 1d        # Errors in last day
  dex slack search "from:@timo.friedl"       # Messages from user
  dex slack search "bug" --tickets           # Find tickets mentioned with "bug"
  dex slack search "DEV-" --tickets          # Find all DEV tickets mentioned`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		sinceStr, _ := cmd.Flags().GetString("since")
		extractTickets, _ := cmd.Flags().GetBool("tickets")
		compact, _ := cmd.Flags().GetBool("compact")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		if !client.HasUserToken() {
			fmt.Fprintf(os.Stderr, "Error: User token required for search (set SLACK_USER_TOKEN with search:read scope)\n")
			os.Exit(1)
		}

		// Parse since duration
		var sinceUnix int64
		if sinceStr != "" {
			duration := parseSlackDuration(sinceStr)
			if duration > 0 {
				sinceTime := time.Now().Add(-duration)
				sinceUnix = sinceTime.Unix()
			}
		}

		// Load index for name resolution
		idx, _ := slack.LoadIndex()

		// Get Jira project keys if ticket extraction is requested
		var projectKeys []string
		if extractTickets {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			jiraClient, err := jira.NewClient()
			if err == nil {
				projectKeys, _ = jiraClient.GetProjectKeys(ctx)
			}
			cancel()

			if len(projectKeys) == 0 {
				fmt.Fprintf(os.Stderr, "Warning: Could not fetch Jira project keys. Using common patterns.\n")
				// Fallback to common project key pattern
				projectKeys = []string{"DEV", "TEL", "OPS", "SEC", "QA"}
			}
		}

		results, total, err := client.Search(query, limit, sinceUnix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return
		}

		// Collect all tickets if extraction is enabled
		allTickets := make(map[string][]string) // ticket -> permalinks where mentioned
		if extractTickets {
			for _, r := range results {
				tickets := slack.ExtractTickets(r.Text, projectKeys)
				for _, t := range tickets {
					allTickets[t] = append(allTickets[t], r.Permalink)
				}
			}
		}

		fmt.Println()

		if extractTickets && len(allTickets) > 0 {
			// Output ticket-focused view
			fmt.Printf("Found %d tickets in %d messages:\n\n", len(allTickets), len(results))

			// Sort tickets for consistent output
			var ticketList []string
			for t := range allTickets {
				ticketList = append(ticketList, t)
			}
			sort.Strings(ticketList)

			for _, ticket := range ticketList {
				links := allTickets[ticket]
				fmt.Printf("  %-12s (%d mentions)\n", ticket, len(links))
				if !compact {
					for _, link := range links {
						fmt.Printf("    %s\n", link)
					}
				}
			}

			if total > len(results) {
				fmt.Printf("\nSearched %d of %d total matches\n", len(results), total)
			}
		} else {
			// Standard search output
			if compact {
				printSearchHeader()
			}

			for i, r := range results {
				channelName := r.ChannelName
				if channelName == "" {
					if ch := idx.FindChannel(r.ChannelID); ch != nil {
						channelName = ch.Name
					} else {
						channelName = r.ChannelID
					}
				}

				username := r.Username
				if username == "" {
					if u := idx.FindUser(r.UserID); u != nil {
						username = u.Username
					} else {
						username = r.UserID
					}
				}

				ts := parseSlackTimestamp(r.Timestamp)

				if compact {
					text := truncateText(r.Text, 60)
					printSearchCompact(ts, channelName, username, text)
				} else {
					text := resolveUserMentions(r.Text, idx)
					printSearchExpanded(i+1, ts, channelName, username, text, r.Permalink)
				}
			}

			if total > len(results) {
				fmt.Printf("\nShowing %d of %d total results\n", len(results), total)
			} else {
				fmt.Printf("\nFound %d results\n", len(results))
			}
		}
	},
}

var slackThreadCmd = &cobra.Command{
	Use:   "thread <url-or-timestamp>",
	Short: "Show a thread and debug mention classification",
	Long: `Fetch and display a Slack thread with classification debug info.

Accepts either a Slack URL or channel:timestamp format.

Examples:
  dex slack thread https://babelforce.slack.com/archives/C03JDUBJD0D/p1769777574026209
  dex slack thread C03JDUBJD0D:1769777574.026209`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		input := args[0]

		// Parse input - either URL or channel:timestamp
		var channelID, threadTS string
		if strings.HasPrefix(input, "http") {
			// Parse URL: https://babelforce.slack.com/archives/C03JDUBJD0D/p1769777574026209
			parts := strings.Split(input, "/")
			for i, part := range parts {
				if part == "archives" && i+2 < len(parts) {
					channelID = parts[i+1]
					// Convert p1769777574026209 to 1769777574.026209
					tsRaw := parts[i+2]
					// Remove query params if present
					if idx := strings.Index(tsRaw, "?"); idx != -1 {
						tsRaw = tsRaw[:idx]
					}
					if strings.HasPrefix(tsRaw, "p") {
						tsRaw = tsRaw[1:]
					}
					if len(tsRaw) > 10 {
						threadTS = tsRaw[:10] + "." + tsRaw[10:]
					} else {
						threadTS = tsRaw
					}
					break
				}
			}
		} else if strings.Contains(input, ":") {
			// Parse channel:timestamp format
			parts := strings.SplitN(input, ":", 2)
			channelID = parts[0]
			threadTS = parts[1]
		}

		if channelID == "" || threadTS == "" {
			fmt.Fprintf(os.Stderr, "Could not parse input. Use URL or channel:timestamp format.\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		client, err := slack.NewClientWithUserToken(cfg.Slack.BotToken, cfg.Slack.UserToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		// Load index for username resolution
		idx, _ := slack.LoadIndex()

		// Collect user IDs for classification
		var myUserIDs []string
		var myBotIDs []string
		botUserID, _ := client.GetBotUserID()
		if botUserID != "" {
			myUserIDs = append(myUserIDs, botUserID)
		}
		botID, _ := client.GetBotID()
		if botID != "" {
			myBotIDs = append(myBotIDs, botID)
		}
		if client.HasUserToken() {
			if userResp, err := client.TestUserAuth(); err == nil {
				if userResp.UserID != botUserID {
					myUserIDs = append(myUserIDs, userResp.UserID)
				}
			}
		}

		fmt.Printf("Channel: %s\n", channelID)
		fmt.Printf("Thread:  %s\n", threadTS)
		fmt.Printf("My User IDs: %v\n", myUserIDs)
		fmt.Printf("My Bot IDs:  %v\n", myBotIDs)
		fmt.Println()

		// Fetch thread
		replies, err := client.GetThreadReplies(channelID, threadTS)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get thread: %v\n", err)
			os.Exit(1)
		}

		if len(replies) == 0 {
			fmt.Println("No messages found in thread.")
			return
		}

		// Display each message
		fmt.Printf("Thread has %d messages:\n", len(replies))
		fmt.Println("────────────────────────────────────────────────────────────────────────────────")

		for i, msg := range replies {
			username := msg.User
			if u := idx.FindUser(msg.User); u != nil {
				username = u.Username
			}

			ts := parseSlackTimestamp(msg.Timestamp)
			label := "Reply"
			if i == 0 {
				label = "Parent"
			}

			// Check if this message is from "me"
			isMe := false
			for _, myID := range myUserIDs {
				if msg.User == myID {
					isMe = true
					break
				}
			}
			for _, myBotID := range myBotIDs {
				if msg.BotID == myBotID {
					isMe = true
					break
				}
			}

			meMarker := ""
			if isMe {
				meMarker = " [ME]"
			}

			fmt.Printf("\n[%d] %s - %s - @%s (User:%s Bot:%s)%s\n", i, label, ts, username, msg.User, msg.BotID, meMarker)
			text := resolveUserMentions(msg.Text, idx)
			fmt.Printf("    %s\n", truncateText(text, 100))
		}

		fmt.Println()
		fmt.Println("────────────────────────────────────────────────────────────────────────────────")

		// Run classifier
		status := client.ClassifyMentionStatus(channelID, threadTS, myUserIDs, myBotIDs)
		fmt.Printf("\nClassification result: %s\n", status)

		// Explain the result
		switch status {
		case slack.MentionStatusReplied:
			fmt.Println("  → Found a reply from one of your user/bot IDs")
		case slack.MentionStatusAcked:
			fmt.Println("  → Found a reaction from you, but no reply")
		case slack.MentionStatusPending:
			fmt.Println("  → No reply or reaction from you found")
		}
	},
}

func printSearchHeader() {
	fmt.Printf("%-19s %-20s %-15s %s\n", "TIME", "CHANNEL", "FROM", "MESSAGE")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────")
}

func printSearchCompact(ts, channel, from, text string) {
	fmt.Printf("%-19s %-20s %-15s %s\n", ts, truncateText(channel, 20), truncateText(from, 15), text)
}

func printSearchExpanded(num int, ts, channel, from, text, permalink string) {
	fmt.Printf("── %d ──────────────────────────────────────────────────────────────────────────\n", num)
	fmt.Printf("#%s  •  %s  •  @%s\n", channel, ts, from)
	if permalink != "" {
		fmt.Printf("%s\n", permalink)
	}
	fmt.Println()
	fmt.Println(text)
	fmt.Println()
}

func init() {
	slackCmd.AddCommand(slackAuthCmd)
	slackCmd.AddCommand(slackTestCmd)
	slackCmd.AddCommand(slackInfoCmd)
	slackCmd.AddCommand(slackPresenceCmd)
	slackCmd.AddCommand(slackIndexCmd)
	slackCmd.AddCommand(slackSendCmd)
	slackCmd.AddCommand(slackChannelsCmd)
	slackCmd.AddCommand(slackUsersCmd)
	slackCmd.AddCommand(slackMentionsCmd)
	slackCmd.AddCommand(slackSearchCmd)
	slackCmd.AddCommand(slackThreadCmd)

	slackPresenceCmd.AddCommand(slackPresenceSetCmd)

	slackIndexCmd.Flags().BoolP("force", "f", false, "Force re-index even if cache is fresh")
	slackSendCmd.Flags().StringP("thread", "t", "", "Thread timestamp to reply to")
	slackSendCmd.Flags().String("as", "bot", "Send as 'bot' (default) or 'user'")
	slackChannelsCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")
	slackChannelsCmd.Flags().BoolP("member", "m", false, "Only show channels bot is a member of")
	slackUsersCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")
	slackMentionsCmd.Flags().StringP("user", "u", "", "User to search mentions for (username or ID)")
	slackMentionsCmd.Flags().BoolP("bot", "b", false, "Search for bot mentions instead of your own")
	slackMentionsCmd.Flags().IntP("limit", "l", 20, "Maximum number of results to show")
	slackMentionsCmd.Flags().BoolP("compact", "c", false, "Compact table view")
	slackMentionsCmd.Flags().StringP("since", "s", "", "Time period to look back (e.g., 1h, 30m, 7d); defaults to today")
	slackMentionsCmd.Flags().Bool("unhandled", false, "Only show pending mentions (no reaction or reply)")
	_ = slackMentionsCmd.RegisterFlagCompletionFunc("user", completeSlackUsers)

	slackSearchCmd.Flags().IntP("limit", "l", 50, "Maximum number of results")
	slackSearchCmd.Flags().StringP("since", "s", "", "Time period to look back (e.g., 1h, 30m, 7d)")
	slackSearchCmd.Flags().BoolP("tickets", "t", false, "Extract and display Jira ticket references")
	slackSearchCmd.Flags().BoolP("compact", "c", false, "Compact output (less detail)")
}
