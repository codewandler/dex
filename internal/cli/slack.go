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
	Short: "Index all accessible Slack channels",
	Long: `Scan and cache all Slack channels the bot has access to.

The index is stored at ~/.dex/slack/index.json and enables:
- Addressing channels by name instead of ID
- Fast channel lookups for autocomplete

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
					fmt.Printf("Index is fresh (%s old, %d channels). Use --force to re-index.\n",
						formatSlackIndexAge(age), len(idx.Channels))
					return
				}
			}
		}

		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		fmt.Print("Indexing channels...")

		idx, err := client.IndexChannels(func(completed, total int) {
			fmt.Printf("\rIndexing channels... %d/%d", completed, total)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to index channels: %v\n", err)
			os.Exit(1)
		}

		if err := slack.SaveIndex(idx); err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to save index: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\rIndexed %d channels for %s\n", len(idx.Channels), idx.TeamName)
	},
}

func formatSlackIndexAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

var slackSendCmd = &cobra.Command{
	Use:   "send <channel> <message>",
	Short: "Send a message to a channel",
	Long: `Send a message to a Slack channel.

The channel can be specified by ID or name (if indexed).

Examples:
  dex slack send dev-team "Hello from dex!"
  dex slack send C03JDUBJD0D "Hello from dex!"`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeSlackChannelNames,
	Run: func(cmd *cobra.Command, args []string) {
		channelArg := args[0]
		message := args[1]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Resolve channel name to ID
		channelID := slack.ResolveChannel(channelArg)

		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		ts, err := client.PostMessage(channelID, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send message: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Message sent (ts: %s)\n", ts)
	},
}

var slackReplyCmd = &cobra.Command{
	Use:   "reply <channel> <thread_ts> <message>",
	Short: "Reply to a thread",
	Long: `Reply to a message thread in Slack.

The channel can be specified by ID or name (if indexed).

Examples:
  dex slack reply dev-team 1234567890.123456 "Thanks for the update!"
  dex slack reply C03JDUBJD0D 1234567890.123456 "Thanks for the update!"`,
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: completeSlackChannelNames,
	Run: func(cmd *cobra.Command, args []string) {
		channelArg := args[0]
		threadTS := args[1]
		message := args[2]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireSlack(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		// Resolve channel name to ID
		channelID := slack.ResolveChannel(channelArg)

		client, err := slack.NewClient(cfg.Slack.BotToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Slack client: %v\n", err)
			os.Exit(1)
		}

		ts, err := client.ReplyToThread(channelID, threadTS, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to reply: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Reply sent (ts: %s)\n", ts)
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

// completeSlackChannelNames provides shell completion for channel names from the index
// Only completes channels the bot is a member of (can post to)
func completeSlackChannelNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument (channel)
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	idx, err := slack.LoadIndex()
	if err != nil || len(idx.Channels) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	toCompleteLower := strings.ToLower(toComplete)

	for _, ch := range idx.Channels {
		// Only complete channels we're a member of
		if !ch.IsMember {
			continue
		}
		if strings.Contains(strings.ToLower(ch.Name), toCompleteLower) {
			completions = append(completions, ch.Name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	slackCmd.AddCommand(slackAuthCmd)
	slackCmd.AddCommand(slackIndexCmd)
	slackCmd.AddCommand(slackSendCmd)
	slackCmd.AddCommand(slackReplyCmd)
	slackCmd.AddCommand(slackChannelsCmd)

	slackIndexCmd.Flags().BoolP("force", "f", false, "Force re-index even if cache is fresh")
	slackChannelsCmd.Flags().Bool("no-cache", false, "Fetch from API instead of using local index")
	slackChannelsCmd.Flags().BoolP("member", "m", false, "Only show channels bot is a member of")
}
