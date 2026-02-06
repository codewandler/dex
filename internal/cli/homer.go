package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/homer"
	"github.com/codewandler/dex/internal/k8s"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	homerHeaderColor  = color.New(color.FgCyan, color.Bold)
	homerDimColor     = color.New(color.FgHiBlack)
	homerSuccessColor = color.New(color.FgGreen)
	homerErrorColor   = color.New(color.FgRed)
	homerMethodColor  = color.New(color.FgYellow, color.Bold)
)

// getHomerClient handles the full discovery -> auth flow and returns a ready-to-use client
func getHomerClient(cmd *cobra.Command) (*homer.Client, error) {
	urlFlag, _ := cmd.Flags().GetString("url")
	namespace, _ := cmd.Flags().GetString("namespace")

	// 1. Resolve Homer URL
	homerURL, err := resolveHomerURL(urlFlag, namespace)
	if err != nil {
		return nil, err
	}

	// 2. Resolve credentials
	username, password := resolveHomerCredentials(homerURL)

	// 3. Create client and authenticate
	client := homer.NewClient(homerURL)
	if err := client.Authenticate(username, password); err != nil {
		return nil, fmt.Errorf("authentication failed at %s: %w", homerURL, err)
	}

	return client, nil
}

// resolveHomerURL finds the Homer URL from flag, config, or K8s discovery
func resolveHomerURL(urlFlag, namespace string) (string, error) {
	// 1. Explicit flag
	if urlFlag != "" {
		return urlFlag, nil
	}

	// 2. Config / env var
	cfg, err := config.Load()
	if err == nil && cfg.Homer.URL != "" {
		return cfg.Homer.URL, nil
	}

	// 3. K8s service discovery
	homerDimColor.Println("No Homer URL configured, attempting K8s service discovery...")
	url, err := discoverHomerURL(namespace)
	if err != nil {
		return "", fmt.Errorf("auto-discovery failed: %w\nTip: Use --url flag or set HOMER_URL environment variable", err)
	}
	homerDimColor.Printf("Discovered Homer at %s\n\n", url)
	return url, nil
}

// discoverHomerURL finds homer-webapp service in K8s
func discoverHomerURL(namespace string) (string, error) {
	ns := namespace
	if ns == "" {
		// Use current k8s namespace
		k8sClient, err := k8s.NewClient("")
		if err != nil {
			return "", fmt.Errorf("failed to connect to Kubernetes: %w", err)
		}
		ns = k8sClient.Namespace()
	}

	k8sClient, err := k8s.NewClient(ns)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Kubernetes: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc, err := k8sClient.GetService(ctx, "homer-webapp")
	if err != nil {
		return "", fmt.Errorf("service 'homer-webapp' not found in namespace %s: %w", ns, err)
	}

	// Determine port (default 80)
	port := 80
	for _, p := range svc.Spec.Ports {
		if p.Name == "http" || p.Port == 80 {
			port = int(p.Port)
			break
		}
	}

	return fmt.Sprintf("http://homer-webapp.%s.svc.cluster.local:%d", ns, port), nil
}

// resolveHomerCredentials finds credentials from config, env, K8s secrets, or defaults
func resolveHomerCredentials(homerURL string) (string, string) {
	normalized := strings.TrimRight(homerURL, "/")

	cfg, err := config.Load()
	if err == nil {
		// 1. Check endpoint-specific credentials
		for key, ep := range cfg.Homer.Endpoints {
			if strings.TrimRight(key, "/") == normalized {
				if ep.Username != "" && ep.Password != "" {
					return ep.Username, ep.Password
				}
			}
		}

		// 2. Check global Homer credentials (config file or env vars)
		if cfg.Homer.Username != "" && cfg.Homer.Password != "" {
			return cfg.Homer.Username, cfg.Homer.Password
		}
	}

	// 3. Default Homer credentials
	return "admin", "admin"
}

var homerCmd = &cobra.Command{
	Use:     "homer",
	Short:   "SIP call tracing via Homer",
	Long:    `Commands for searching and inspecting SIP traffic via Homer.`,
	Aliases: []string{"sip"},
}

var homerDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover Homer in Kubernetes cluster",
	Long: `Find Homer via K8s service discovery and test connectivity.

Looks for service 'homer-webapp' in the current namespace (or specified with -n).

Examples:
  dex homer discover
  dex homer discover -n eu`,
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")

		ns := namespace
		if ns == "" {
			k8sClient, err := k8s.NewClient("")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect to Kubernetes: %v\n", err)
				os.Exit(1)
			}
			ns = k8sClient.Namespace()
		}

		fmt.Printf("Searching for Homer in namespace %s...\n", ns)

		k8sClient, err := k8s.NewClient(ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to Kubernetes: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		svc, err := k8sClient.GetService(ctx, "homer-webapp")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Service 'homer-webapp' not found in namespace %s\n", ns)
			os.Exit(1)
		}

		// Determine port
		port := 80
		for _, p := range svc.Spec.Ports {
			if p.Name == "http" || p.Port == 80 {
				port = int(p.Port)
				break
			}
		}

		url := fmt.Sprintf("http://homer-webapp.%s.svc.cluster.local:%d", ns, port)
		homerDimColor.Printf("  Found service: %s/%s (port %d)\n", ns, svc.Name, port)

		// Test connectivity
		client := homer.NewClient(url)
		if err := client.TestConnection(); err != nil {
			homerErrorColor.Printf("  Connectivity: FAILED (%v)\n", err)
			os.Exit(1)
		}
		homerSuccessColor.Println("  Connectivity: OK")

		// Test auth with default credentials
		username, password := resolveHomerCredentials(url)
		if err := client.Authenticate(username, password); err != nil {
			homerErrorColor.Printf("  Authentication: FAILED (%v)\n", err)
			fmt.Fprintf(os.Stderr, "\nTip: Set HOMER_USERNAME/HOMER_PASSWORD or configure in ~/.dex/config.json\n")
			os.Exit(1)
		}
		homerSuccessColor.Println("  Authentication: OK")

		fmt.Println()
		homerHeaderColor.Println("Homer URL:")
		fmt.Printf("  %s\n\n", url)
		homerDimColor.Printf("To use: export HOMER_URL=%s\n", url)
	},
}

var homerSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search SIP calls",
	Long: `Search for SIP calls by caller, callee, call-id, or time range.

Examples:
  dex homer search --from 1h --caller "+4930..."
  dex homer search --from 2h --callee "100"
  dex homer search --from 30m --limit 20`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		fromStr, _ := cmd.Flags().GetString("from")
		toStr, _ := cmd.Flags().GetString("to")
		caller, _ := cmd.Flags().GetString("caller")
		callee, _ := cmd.Flags().GetString("callee")
		callID, _ := cmd.Flags().GetString("call-id")
		limit, _ := cmd.Flags().GetInt("limit")

		from, to, err := parseTimeRange(fromStr, toStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid time range: %v\n", err)
			os.Exit(1)
		}

		params := homer.SearchParams{
			From:   from,
			To:     to,
			Caller: caller,
			Callee: callee,
			CallID: callID,
			Limit:  limit,
		}

		result, err := client.SearchCalls(params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

		if len(result.Data) == 0 {
			homerDimColor.Println("No calls found.")
			return
		}

		line := strings.Repeat("─", 100)
		fmt.Println()
		homerHeaderColor.Printf("  SIP Calls (%d)\n", len(result.Data))
		fmt.Println("  " + line)
		fmt.Println()

		// Table header
		fmt.Printf("  %-20s  %-10s  %-20s  %-20s  %s\n",
			"DATE", "METHOD", "FROM", "TO", "CALL-ID")
		fmt.Println("  " + line)

		for _, call := range result.Data {
			method := call.Method
			if method == "" {
				method = call.MethodText
			}
			if method == "" {
				method = "-"
			}
			from := call.FromUser
			if from == "" {
				from = "-"
			}
			to := call.ToUser
			if to == "" {
				to = call.RuriUser
			}
			if to == "" {
				to = "-"
			}
			callid := call.CallID
			if len(callid) > 40 {
				callid = callid[:40] + "..."
			}

			dateStr := formatEpochMS(call.Date)
			fmt.Printf("  %-20s  ", dateStr)
			homerMethodColor.Printf("%-10s", method)
			fmt.Printf("  %-20s  %-20s  %s\n", from, to, callid)
		}
		fmt.Println()
	},
}

var homerShowCmd = &cobra.Command{
	Use:   "show <call-id>",
	Short: "Show SIP message flow for a call",
	Long: `Display the SIP message ladder for a specific call.

Examples:
  dex homer show abc123-def456@host
  dex homer show abc123-def456@host --from 2h`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		callID := args[0]
		fromStr, _ := cmd.Flags().GetString("from")
		toStr, _ := cmd.Flags().GetString("to")

		from, to, err := parseTimeRange(fromStr, toStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid time range: %v\n", err)
			os.Exit(1)
		}

		params := homer.SearchParams{
			From:   from,
			To:     to,
			CallID: callID,
			Limit:  200,
		}

		result, err := client.SearchCalls(params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get messages: %v\n", err)
			os.Exit(1)
		}

		if len(result.Data) == 0 {
			homerDimColor.Println("No messages found for this call-id.")
			homerDimColor.Println("Tip: Try expanding the time range with --from")
			return
		}

		line := strings.Repeat("─", 100)
		fmt.Println()
		homerHeaderColor.Printf("  SIP Message Flow - %s (%d messages)\n", callID, len(result.Data))
		fmt.Println("  " + line)
		fmt.Println()

		// Table header
		fmt.Printf("  %-24s  %-22s  %-7s %-22s  %s\n",
			"TIME", "SOURCE", "", "DESTINATION", "METHOD/STATUS")
		fmt.Println("  " + line)

		for _, msg := range result.Data {
			src := fmt.Sprintf("%s:%d", msg.SourceIP, int(msg.SourcePort))
			dst := fmt.Sprintf("%s:%d", msg.DestIP, int(msg.DestPort))

			if msg.AliasSrc != "" {
				src = msg.AliasSrc
			}
			if msg.AliasDst != "" {
				dst = msg.AliasDst
			}

			method := msg.Method
			if method == "" {
				method = msg.MethodText
			}

			dateStr := formatEpochMS(msg.Date)
			fmt.Printf("  %-24s  %-22s  -----> %-22s  ", dateStr, src, dst)
			homerMethodColor.Printf("%s\n", method)
		}
		fmt.Println()
	},
}

var homerExportCmd = &cobra.Command{
	Use:   "export <call-id>",
	Short: "Export call as PCAP file",
	Long: `Export SIP messages for a call as a PCAP file for analysis in Wireshark.

Examples:
  dex homer export abc123-def456@host
  dex homer export abc123-def456@host -o trace.pcap
  dex homer export abc123-def456@host --from 2h`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		callID := args[0]
		output, _ := cmd.Flags().GetString("output")
		fromStr, _ := cmd.Flags().GetString("from")
		toStr, _ := cmd.Flags().GetString("to")

		from, to, err := parseTimeRange(fromStr, toStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid time range: %v\n", err)
			os.Exit(1)
		}

		if output == "" {
			// Generate default filename from call-id
			safe := strings.NewReplacer("@", "_", ":", "_", "/", "_").Replace(callID)
			if len(safe) > 40 {
				safe = safe[:40]
			}
			output = safe + ".pcap"
		}

		params := homer.SearchParams{
			From:   from,
			To:     to,
			CallID: callID,
		}

		data, err := client.ExportPCAP(params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
			os.Exit(1)
		}

		if len(data) == 0 {
			homerDimColor.Println("No data to export for this call-id.")
			return
		}

		if err := os.WriteFile(output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file: %v\n", err)
			os.Exit(1)
		}

		homerSuccessColor.Printf("Exported %d bytes to %s\n", len(data), output)
	},
}

var homerAliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "List configured IP/port aliases",
	Long: `List all IP/port aliases configured in Homer.

Aliases map IP addresses to human-readable names for SIP trace display.

Examples:
  dex homer aliases`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		aliases, err := client.ListAliases()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list aliases: %v\n", err)
			os.Exit(1)
		}

		if len(aliases) == 0 {
			homerDimColor.Println("No aliases configured.")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		homerHeaderColor.Printf("  Homer Aliases (%d)\n", len(aliases))
		fmt.Println("  " + line)
		fmt.Println()

		fmt.Printf("  %-20s  %-8s  %-30s  %s\n", "IP", "PORT", "ALIAS", "STATUS")
		fmt.Println("  " + line)

		for _, a := range aliases {
			status := "active"
			statusColor := homerSuccessColor
			if !a.Status {
				status = "inactive"
				statusColor = homerDimColor
			}

			fmt.Printf("  %-20s  %-8d  %-30s  ", a.IP, int(a.Port), a.Alias)
			statusColor.Printf("%s\n", status)
		}
		fmt.Println()
	},
}

// formatEpochMS converts an epoch millisecond timestamp to a human-readable string
func formatEpochMS(ms int64) string {
	if ms == 0 {
		return "-"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

// parseTimeRange converts --from and --to flags into time.Time values
func parseTimeRange(fromStr, toStr string) (time.Time, time.Time, error) {
	to := time.Now()
	from := to.Add(-1 * time.Hour) // default: last 1 hour

	if fromStr != "" {
		dur, err := parseLokiDuration(fromStr) // reuse the existing duration parser
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from: %w", err)
		}
		from = to.Add(-dur)
	}

	if toStr != "" {
		dur, err := parseLokiDuration(toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to: %w", err)
		}
		to = time.Now().Add(-dur)
	}

	return from, to, nil
}

func init() {
	rootCmd.AddCommand(homerCmd)

	// Persistent flags (available to all subcommands)
	homerCmd.PersistentFlags().String("url", "", "Homer URL (overrides HOMER_URL config)")
	homerCmd.PersistentFlags().StringP("namespace", "n", "", "Kubernetes namespace for service discovery")

	// Subcommands
	homerCmd.AddCommand(homerDiscoverCmd)
	homerCmd.AddCommand(homerSearchCmd)
	homerCmd.AddCommand(homerShowCmd)
	homerCmd.AddCommand(homerExportCmd)
	homerCmd.AddCommand(homerAliasesCmd)

	// Search flags
	homerSearchCmd.Flags().String("from", "1h", "Time range start (e.g., 1h, 30m, 2d)")
	homerSearchCmd.Flags().String("to", "", "Time range end (default: now)")
	homerSearchCmd.Flags().String("caller", "", "Source number/URI")
	homerSearchCmd.Flags().String("callee", "", "Destination number/URI")
	homerSearchCmd.Flags().String("call-id", "", "SIP Call-ID")
	homerSearchCmd.Flags().IntP("limit", "l", 50, "Maximum results")

	// Show flags
	homerShowCmd.Flags().String("from", "1h", "Time range start")
	homerShowCmd.Flags().String("to", "", "Time range end (default: now)")

	// Export flags
	homerExportCmd.Flags().String("from", "1h", "Time range start")
	homerExportCmd.Flags().String("to", "", "Time range end (default: now)")
	homerExportCmd.Flags().StringP("output", "o", "", "Output file path (default: <call-id>.pcap)")
}
