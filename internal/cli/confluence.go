package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/confluence"

	"github.com/spf13/cobra"
)

var confluenceCmd = &cobra.Command{
	Use:     "confluence",
	Aliases: []string{"cf"},
	Short:   "Confluence wiki management",
	Long:    `Commands for interacting with Confluence via OAuth.`,
}

var confluenceAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Confluence (opens browser)",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		client, err := confluence.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := client.EnsureAuth(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Authentication successful! Token saved.")
	},
}

var confluenceSpacesCmd = &cobra.Command{
	Use:   "spaces",
	Short: "List Confluence spaces",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		limit, _ := cmd.Flags().GetInt("limit")

		client, err := confluence.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		spaces, err := client.ListSpaces(ctx, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(spaces) == 0 {
			fmt.Println("No spaces found.")
			return
		}

		fmt.Printf("%-10s %-40s %s\n", "KEY", "NAME", "TYPE")
		fmt.Println(strings.Repeat("─", 64))
		for _, space := range spaces {
			fmt.Printf("%-10s %-40s %s\n",
				space.Key,
				truncate(space.Name, 40),
				space.Type,
			)
		}
		fmt.Printf("\n%d spaces\n", len(spaces))
	},
}

var confluenceSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Confluence content",
	Long: `Search Confluence using CQL (Confluence Query Language).

Plain text queries are automatically wrapped as: text ~ "query"
You can also provide raw CQL for more advanced searches.

Examples:
  dex confluence search "deployment guide"
  dex confluence search "type = page AND space = DEV"
  dex confluence search "label = architecture"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		query := strings.Join(args, " ")
		limit, _ := cmd.Flags().GetInt("limit")

		// Auto-wrap plain text as CQL text search
		if !strings.Contains(query, "=") && !strings.Contains(query, "~") {
			query = fmt.Sprintf(`text ~ "%s"`, query)
		}

		client, err := confluence.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		result, err := client.Search(ctx, query, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(result.Results) == 0 {
			fmt.Println("No results found.")
			return
		}

		siteURL := client.GetSiteURL()

		fmt.Printf("Found %d results (of %d total):\n\n", len(result.Results), result.TotalSize)
		for _, r := range result.Results {
			fmt.Printf("%-10s %-8s %s\n",
				r.Content.ID,
				r.Content.Type,
				r.Content.Title,
			)
			if r.FriendlyLastModified != "" {
				fmt.Printf("           Modified: %s\n", r.FriendlyLastModified)
			}
			if siteURL != "" && r.URL != "" {
				fmt.Printf("           %s%s\n", siteURL, r.URL)
			}
		}
	},
}

var confluencePageCmd = &cobra.Command{
	Use:   "page <id>",
	Short: "View a Confluence page",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client, err := confluence.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		page, err := client.GetPage(ctx, args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		siteURL := client.GetSiteURL()

		fmt.Printf("%s (v%d)\n", page.Title, page.Version.Number)
		if siteURL != "" && page.Links.WebUI != "" {
			fmt.Printf("URL: %s%s\n", siteURL, page.Links.WebUI)
		}
		fmt.Println()

		body := page.Body.Storage.Value
		if body != "" {
			fmt.Println(confluence.StripHTML(body))
		} else {
			fmt.Println("(empty page)")
		}
	},
}

func init() {
	confluenceCmd.AddCommand(confluenceAuthCmd)
	confluenceCmd.AddCommand(confluenceSpacesCmd)
	confluenceCmd.AddCommand(confluenceSearchCmd)
	confluenceCmd.AddCommand(confluencePageCmd)

	confluenceSpacesCmd.Flags().IntP("limit", "l", 25, "Maximum number of results")
	confluenceSearchCmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
}
