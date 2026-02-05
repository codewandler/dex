package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
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
	Short: "Test Slack authentication",
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
@mentions in the message body are auto-resolved to Slack user mentions.

Examples:
  dex slack send dev-team "Hello from dex!"
  dex slack send dev-team "Hey @timo.friedl check this!"  # @mention in message
  dex slack send dev-team "Follow up" -t 1770257991.873399  # Reply to thread
  dex slack send @timo.friedl "Hey, check this out!"      # DM (requires im:write)`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeSlackTargets,
	Run: func(cmd *cobra.Command, args []string) {
		targetArg := args[0]
		message := args[1]
		threadTS, _ := cmd.Flags().GetString("thread")

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

func init() {
	slackCmd.AddCommand(slackAuthCmd)
	slackCmd.AddCommand(slackIndexCmd)
	slackCmd.AddCommand(slackSendCmd)
	slackCmd.AddCommand(slackChannelsCmd)
	slackCmd.AddCommand(slackUsersCmd)

	slackIndexCmd.Flags().BoolP("force", "f", false, "Force re-index even if cache is fresh")
	slackSendCmd.Flags().StringP("thread", "t", "", "Thread timestamp to reply to")
	slackChannelsCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")
	slackChannelsCmd.Flags().BoolP("member", "m", false, "Only show channels bot is a member of")
	slackUsersCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")
}
