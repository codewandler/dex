package cli

import (
	"context"
	"encoding/json"
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
	homerWarnColor    = color.New(color.FgHiYellow)
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
	client.Debug, _ = cmd.Flags().GetBool("debug")
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
	Long: `Search for SIP calls by number, caller/callee, user agent, or time range.

Filter flags (--number, --from-user, --to-user, --ua) are combined as AND conditions into a
Homer smart input query. Use -q for custom expressions with field validation.

Available fields: from_user, to_user, ruri_user, user_agent (alias: ua),
  cseq, method, status, call_id (alias: sid)

Examples:
  dex homer search --number "4921514174858"
  dex homer search --from-user "999%" --to-user "12345"
  dex homer search --from-user "999%" --ua "Asterisk%"
  dex homer search -q "from_user = '123' AND status = 200"
  dex homer search -q "from_user = '999%' AND (to_user = '123' OR to_user = '456')"
  dex homer search --at "2026-02-04 17:13"
  dex homer search --number "4921514174858" -m INVITE -m BYE
  dex homer search --number "4921514174858" -o jsonl`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		atStr, _ := cmd.Flags().GetString("at")
		query, _ := cmd.Flags().GetString("query")
		number, _ := cmd.Flags().GetString("number")
		fromUser, _ := cmd.Flags().GetString("from-user")
		toUser, _ := cmd.Flags().GetString("to-user")
		ua, _ := cmd.Flags().GetString("ua")
		callID, _ := cmd.Flags().GetString("call-id")
		methods, _ := cmd.Flags().GetStringSlice("method")
		limit, _ := cmd.Flags().GetInt("limit")
		output, _ := cmd.Flags().GetString("output")

		var from, to time.Time

		if atStr != "" {
			if cmd.Flags().Changed("since") || cmd.Flags().Changed("until") {
				fmt.Fprintf(os.Stderr, "Cannot use --at together with --since/--until\n")
				os.Exit(1)
			}
			at, err := parseTimeValue(atStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --at: %v\n", err)
				os.Exit(1)
			}
			from = at.Add(-5 * time.Minute)
			to = at.Add(5 * time.Minute)
		} else {
			from, err = parseTimeValue(sinceStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --since: %v\n", err)
				os.Exit(1)
			}
			if untilStr == "" {
				to = time.Now()
			} else {
				to, err = parseTimeValue(untilStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid --until: %v\n", err)
					os.Exit(1)
				}
			}
		}

		if output == "" {
			homerDimColor.Printf("  Time range: %s → %s\n\n", from.Format("2006-01-02 15:04:05"), to.Format("2006-01-02 15:04:05"))
		}

		// Build smartinput from flags. Each flag produces a set of OR-alternatives
		// (e.g. with/without + prefix). The cartesian product of all sets is computed
		// so that AND binds within each term and OR separates terms — no parentheses
		// needed since Homer uses standard AND-before-OR precedence.
		var criteria [][]string
		if number != "" {
			bare := strings.TrimPrefix(number, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", plus),
				fmt.Sprintf("data_header.to_user = '%s'", bare),
				fmt.Sprintf("data_header.to_user = '%s'", plus),
			})
		}
		if fromUser != "" {
			bare := strings.TrimPrefix(fromUser, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", plus),
			})
		}
		if toUser != "" {
			bare := strings.TrimPrefix(toUser, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.to_user = '%s'", bare),
				fmt.Sprintf("data_header.to_user = '%s'", plus),
			})
		}
		if ua != "" {
			criteria = append(criteria, []string{fmt.Sprintf("data_header.user_agent = '%s'", ua)})
		}
		if query != "" {
			parsed, err := homer.ParseQuery(query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid query: %v\n", err)
				os.Exit(1)
			}
			criteria = append(criteria, []string{parsed})
		}

		params := homer.SearchParams{
			From:       from,
			To:         to,
			SmartInput: buildSmartInput(criteria),
			CallID:     callID,
			Limit:      limit,
		}

		result, err := client.SearchCalls(params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

		// Convert to clean records
		records := homer.ToSearchRecords(result.Data)

		// Client-side method filter
		if len(methods) > 0 {
			methodSet := make(map[string]bool, len(methods))
			for _, m := range methods {
				methodSet[strings.ToUpper(m)] = true
			}
			filtered := records[:0]
			for _, r := range records {
				if methodSet[strings.ToUpper(r.Method)] {
					filtered = append(filtered, r)
				}
			}
			records = filtered
		}

		// JSON/JSONL output
		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(records)
			return
		}
		if output == "jsonl" {
			enc := json.NewEncoder(os.Stdout)
			for _, r := range records {
				enc.Encode(r)
			}
			return
		}

		if len(records) == 0 {
			homerDimColor.Println("No calls found.")
			return
		}

		// Compute dynamic column widths
		maxSrcWidth := 0
		maxDstWidth := 0
		maxUAWidth := len("USER-AGENT")
		for _, r := range records {
			if w := len(fmt.Sprintf("%s:%d", r.SrcIP, r.SrcPort)); w > maxSrcWidth {
				maxSrcWidth = w
			}
			if w := len(fmt.Sprintf("%s:%d", r.DstIP, r.DstPort)); w > maxDstWidth {
				maxDstWidth = w
			}
			if len(r.UserAgent) > maxUAWidth {
				maxUAWidth = len(r.UserAgent)
			}
		}
		routeWidth := maxSrcWidth + 3 + maxDstWidth

		// Total row width: indent(2) + DATE(20) + gap(2) + ROUTE + gap(2) + CALL-ID(30) + gap(2) + METHOD(10) + gap(2) + FROM(20) + gap(2) + TO(20) + gap(2) + UA
		lineWidth := 20 + 2 + routeWidth + 2 + 30 + 2 + 10 + 2 + 20 + 2 + 20 + 2 + maxUAWidth
		line := strings.Repeat("─", lineWidth)
		fmt.Println()
		homerHeaderColor.Printf("  SIP Calls (%d)\n", len(records))
		fmt.Println("  " + line)
		fmt.Println()

		routeHeader := fmt.Sprintf("%-*s", routeWidth, "ROUTE")
		fmt.Printf("  %-20s  %s  %-30s  %-10s  %-20s  %-20s  %s\n",
			"DATE", routeHeader, "CALL-ID", "METHOD", "FROM", "TO", "USER-AGENT")
		fmt.Println("  " + line)

		for _, r := range records {
			method := r.Method
			if method == "" {
				method = "-"
			}
			fromUser := r.FromUser
			if fromUser == "" {
				fromUser = "-"
			}
			toUser := r.ToUser
			if toUser == "" {
				toUser = "-"
			}

			fmt.Printf("  %-20s  ", r.Date.Format("2006-01-02 15:04:05"))
			printRoute(r.SrcIP, r.SrcPort, r.DstIP, r.DstPort, maxSrcWidth, routeWidth)
			fmt.Print("  ")
			printSessionID(r.CallID, 30)
			fmt.Print("  ")
			homerMethodColor.Printf("%-10s", method)
			fmt.Printf("  %-20s  %-20s  ", fromUser, toUser)
			printUserAgent(r.UserAgent)
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

var homerEndpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "List configured Homer endpoints",
	Long: `List all Homer endpoints configured in ~/.dex/config.json.

Shows the default URL and any endpoint-specific credential overrides.

Examples:
  dex homer endpoints`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

		hasDefault := cfg.Homer.URL != ""
		hasEndpoints := len(cfg.Homer.Endpoints) > 0

		if !hasDefault && !hasEndpoints {
			homerDimColor.Println("No Homer endpoints configured.")
			homerDimColor.Println("Tip: Set HOMER_URL or configure endpoints in ~/.dex/config.json")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		homerHeaderColor.Println("  Homer Endpoints")
		fmt.Println("  " + line)
		fmt.Println()

		if hasDefault {
			fmt.Printf("  %-10s  %s\n", "Default:", cfg.Homer.URL)
			creds := "custom"
			if cfg.Homer.Username == "" {
				creds = "default (admin)"
			}
			homerDimColor.Printf("  %-10s  credentials: %s\n", "", creds)
			fmt.Println()
		}

		if hasEndpoints {
			for url, ep := range cfg.Homer.Endpoints {
				label := ""
				if url == cfg.Homer.URL {
					label = " (default)"
				}
				fmt.Printf("  %s%s\n", url, label)
				creds := "custom"
				if ep.Username == "" {
					creds = "not set (will use global)"
				}
				homerDimColor.Printf("    credentials: %s\n", creds)
			}
			fmt.Println()
		}

		count := len(cfg.Homer.Endpoints)
		if hasDefault {
			// Count default if not already in endpoints map
			if _, ok := cfg.Homer.Endpoints[cfg.Homer.URL]; !ok {
				count++
			}
		}
		homerDimColor.Printf("  %d endpoint(s) configured\n", count)
		fmt.Println()
	},
}

var homerCallsCmd = &cobra.Command{
	Use:   "calls",
	Short: "List calls grouped by Call-ID",
	Long: `Search and display calls grouped by Call-ID with direction and status.

Supports the same filter flags as search (--number, --from-user, --to-user, --ua, -q)
and the same time range options (--since, --until, --at).

Examples:
  dex homer calls --since 1h
  dex homer calls --number "31617554360" --since 2h
  dex homer calls --from-user "999%" --since 1h
  dex homer calls --ua "FPBX%" --since 30m
  dex homer calls -q "ua = 'Asterisk%'" --since 1h
  dex homer calls --at "2026-02-04 17:13"
  dex homer calls --since 1h -o json`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		atStr, _ := cmd.Flags().GetString("at")
		number, _ := cmd.Flags().GetString("number")
		fromUser, _ := cmd.Flags().GetString("from-user")
		toUser, _ := cmd.Flags().GetString("to-user")
		ua, _ := cmd.Flags().GetString("ua")
		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")
		output, _ := cmd.Flags().GetString("output")

		var from, to time.Time

		if atStr != "" {
			if cmd.Flags().Changed("since") || cmd.Flags().Changed("until") {
				fmt.Fprintf(os.Stderr, "Cannot use --at together with --since/--until\n")
				os.Exit(1)
			}
			at, err := parseTimeValue(atStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --at: %v\n", err)
				os.Exit(1)
			}
			from = at.Add(-5 * time.Minute)
			to = at.Add(5 * time.Minute)
		} else {
			from, err = parseTimeValue(sinceStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --since: %v\n", err)
				os.Exit(1)
			}
			if untilStr == "" {
				to = time.Now()
			} else {
				to, err = parseTimeValue(untilStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid --until: %v\n", err)
					os.Exit(1)
				}
			}
		}

		if output == "" {
			homerDimColor.Printf("  Time range: %s → %s\n\n", from.Format("2006-01-02 15:04:05"), to.Format("2006-01-02 15:04:05"))
		}

		// Build smartinput from flags (same logic as search command).
		var criteria [][]string
		if number != "" {
			bare := strings.TrimPrefix(number, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", plus),
				fmt.Sprintf("data_header.to_user = '%s'", bare),
				fmt.Sprintf("data_header.to_user = '%s'", plus),
			})
		}
		if fromUser != "" {
			bare := strings.TrimPrefix(fromUser, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", plus),
			})
		}
		if toUser != "" {
			bare := strings.TrimPrefix(toUser, "+")
			plus := "+" + bare
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.to_user = '%s'", bare),
				fmt.Sprintf("data_header.to_user = '%s'", plus),
			})
		}
		if ua != "" {
			criteria = append(criteria, []string{fmt.Sprintf("data_header.user_agent = '%s'", ua)})
		}
		if query != "" {
			parsed, err := homer.ParseQuery(query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid query: %v\n", err)
				os.Exit(1)
			}
			criteria = append(criteria, []string{parsed})
		}

		params := homer.SearchParams{
			From:       from,
			To:         to,
			SmartInput: buildSmartInput(criteria),
		}
		calls, err := client.FetchCalls(params, number, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

		// JSON/JSONL output
		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(calls)
			return
		}
		if output == "jsonl" {
			enc := json.NewEncoder(os.Stdout)
			for _, c := range calls {
				enc.Encode(c)
			}
			return
		}

		if len(calls) == 0 {
			homerDimColor.Println("No calls found.")
			return
		}

		line := strings.Repeat("─", 110)
		fmt.Println()
		homerHeaderColor.Printf("  Calls (%d)\n", len(calls))
		fmt.Println("  " + line)
		fmt.Println()

		// Compute dynamic column widths
		maxCallIDWidth := len("CALL-ID")
		maxTimeWidth := len("TIME")
		for _, c := range calls {
			if len(c.CallID) > maxCallIDWidth {
				maxCallIDWidth = len(c.CallID)
			}
			tw := len(formatCallTime(c))
			if tw > maxTimeWidth {
				maxTimeWidth = tw
			}
		}

		lineWidth := maxTimeWidth + 2 + maxCallIDWidth + 2 + 20 + 2 + 20 + 2 + 12
		line = strings.Repeat("─", lineWidth)

		// Table header
		fmt.Printf("  %-*s  %-*s  %-20s  %-20s  %s\n",
			maxTimeWidth, "TIME", maxCallIDWidth, "CALL-ID", "FROM", "TO", "STATUS")
		fmt.Println("  " + line)

		for _, c := range calls {
			caller := c.Caller
			if caller == "" {
				caller = "-"
			}
			callee := c.Callee
			if callee == "" {
				callee = "-"
			}

			printCallTime(c, maxTimeWidth)
			fmt.Print("  ")
			printCallID(c.CallID, maxCallIDWidth)
			fmt.Printf("  %-20s  %-20s  ", caller, callee)
			formatCallStatus(c.Status)
			fmt.Print("\n")
		}
		fmt.Println()
	},
}

// buildSmartInput constructs a Homer smartinput expression from criteria.
// Each criterion is a set of OR-alternatives (e.g. number with/without + prefix).
// The cartesian product of all criteria is computed: AND within each product term,
// OR between terms. Homer uses standard AND-before-OR precedence so no parentheses
// are needed.
func buildSmartInput(criteria [][]string) string {
	if len(criteria) == 0 {
		return ""
	}

	// Compute cartesian product of all criteria
	products := [][]string{{}}
	for _, alts := range criteria {
		var next [][]string
		for _, product := range products {
			for _, alt := range alts {
				term := make([]string, len(product)+1)
				copy(term, product)
				term[len(product)] = alt
				next = append(next, term)
			}
		}
		products = next
	}

	// Format: AND within each product, OR between products
	terms := make([]string, len(products))
	for i, product := range products {
		terms[i] = strings.Join(product, " AND ")
	}
	return strings.Join(terms, " OR ")
}

// formatCallTime formats start, end, and duration into a compact time string.
// Same day:  "2026-02-04 16:53:06 - 17:12:08 (19m2s)"
// Diff day:  "2026-02-04 23:59:00 - 2026-02-05 00:01:00 (2m)"
// No end:    "2026-02-04 16:53:06 - <na>"
func formatCallTime(c homer.CallSummary) string {
	start := c.StartTime.Format("2006-01-02 15:04:05")

	if c.MsgCount <= 1 {
		return start + " - <na>"
	}

	dur := formatDuration(c.Duration)

	if c.StartTime.Format("2006-01-02") == c.EndTime.Format("2006-01-02") {
		return fmt.Sprintf("%s - %s (%s)", start, c.EndTime.Format("15:04:05"), dur)
	}

	return fmt.Sprintf("%s - %s (%s)", start, c.EndTime.Format("2006-01-02 15:04:05"), dur)
}

// printCallTime prints the call time with coloring, padded to width.
// The <na> marker for missing end times is printed in orange.
func printCallTime(c homer.CallSummary, width int) {
	s := formatCallTime(c)
	if c.MsgCount <= 1 {
		// Print everything before <na> normally, then <na> in orange
		prefix := c.StartTime.Format("2006-01-02 15:04:05") + " - "
		fmt.Print("  " + prefix)
		homerWarnColor.Print("<na>")
		if pad := width - len(s); pad > 0 {
			fmt.Print(strings.Repeat(" ", pad))
		}
	} else {
		fmt.Printf("  %-*s", width, s)
	}
}

// formatDuration formats a duration into a compact human-readable string (e.g., "53s", "18m12s", "1h5m").
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// printCallID prints a Call-ID with the local part in standout color and the @host part dimmed,
// padded to the given width.
func printCallID(callID string, width int) {
	if idx := strings.Index(callID, "@"); idx >= 0 {
		homerHeaderColor.Print(callID[:idx])
		homerDimColor.Print(callID[idx:])
	} else {
		homerHeaderColor.Print(callID)
	}
	if pad := width - len(callID); pad > 0 {
		fmt.Print(strings.Repeat(" ", pad))
	}
}

// formatCallStatus prints a colored status string
func formatCallStatus(status string) {
	switch status {
	case "answered":
		homerSuccessColor.Printf("%-12s", status)
	case "busy", "cancelled", "no answer":
		homerMethodColor.Printf("%-12s", status)
	case "failed":
		homerErrorColor.Printf("%-12s", status)
	case "ringing":
		homerDimColor.Printf("%-12s", status)
	default:
		fmt.Printf("%-12s", "-")
	}
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

// splitSessionID splits a formatted session ID into hash and host parts for colored output.
func splitSessionID(callID string) (hash string, host string) {
	sid := homer.FormatSessionID(callID)
	if sid == "" {
		return "-", ""
	}
	if idx := strings.Index(sid, "@"); idx >= 0 {
		return sid[:idx], sid[idx:]
	}
	return sid, ""
}

// printSessionID prints a colored session ID padded to the given width.
func printSessionID(callID string, width int) {
	hash, host := splitSessionID(callID)
	full := hash + host
	padding := ""
	if len(full) < width {
		padding = strings.Repeat(" ", width-len(full))
	}
	homerHeaderColor.Print(hash)
	homerDimColor.Print(host)
	fmt.Print(padding)
}

// printUserAgent prints a formatted user agent with special coloring for known types.
func printUserAgent(ua string) {
	if strings.HasPrefix(ua, "Asterisk ") {
		homerMethodColor.Print("Asterisk")
		homerDimColor.Println(" " + ua[9:])
	} else if strings.HasPrefix(ua, "FPBX ") {
		homerHeaderColor.Print("FPBX")
		homerDimColor.Println(" " + ua[5:])
	} else {
		homerDimColor.Println(ua)
	}
}

// printRoute prints a colored "srcIP:port → dstIP:port" route padded to totalWidth display characters.
// Source side is padded to srcWidth so arrows align. IP is normal, port is dim.
func printRoute(srcIP string, srcPort int, dstIP string, dstPort int, srcWidth int, totalWidth int) {
	srcStr := fmt.Sprintf("%s:%d", srcIP, srcPort)
	srcPad := ""
	if len(srcStr) < srcWidth {
		srcPad = strings.Repeat(" ", srcWidth-len(srcStr))
	}

	// Print source
	fmt.Print(srcIP)
	homerDimColor.Printf(":%d", srcPort)
	fmt.Print(srcPad)

	// Arrow (→ is 1 display char)
	homerDimColor.Print(" → ")

	// Print destination
	fmt.Print(dstIP)
	homerDimColor.Printf(":%d", dstPort)

	// Pad to total width: src(padded) + " → "(3 display) + dst
	dstStr := fmt.Sprintf("%s:%d", dstIP, dstPort)
	used := srcWidth + 3 + len(dstStr)
	if used < totalWidth {
		fmt.Print(strings.Repeat(" ", totalWidth-used))
	}
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

// parseTimeValue parses a string that is either a duration (e.g., "1h", "30m", "2d")
// or an absolute timestamp (e.g., "2026-02-04 17:13", "2026-02-04T17:13:00").
// Durations are interpreted as "that long ago from now".
func parseTimeValue(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now(), nil
	}

	// Try absolute timestamp formats (most specific first)
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, nil
		}
	}

	// Try duration (e.g., "1h", "30m", "2d")
	dur, err := parseLokiDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be a duration (e.g., 1h, 30m, 2d) or timestamp (e.g., 2006-01-02 15:04): %s", s)
	}
	return time.Now().Add(-dur), nil
}

func init() {
	rootCmd.AddCommand(homerCmd)

	// Persistent flags (available to all subcommands)
	homerCmd.PersistentFlags().String("url", "", "Homer URL (overrides HOMER_URL config)")
	homerCmd.PersistentFlags().StringP("namespace", "n", "", "Kubernetes namespace for service discovery")
	homerCmd.PersistentFlags().BoolP("debug", "d", false, "Print API endpoint and request body")

	// Subcommands
	homerCmd.AddCommand(homerDiscoverCmd)
	homerCmd.AddCommand(homerSearchCmd)
	homerCmd.AddCommand(homerShowCmd)
	homerCmd.AddCommand(homerExportCmd)
	homerCmd.AddCommand(homerEndpointsCmd)
	homerCmd.AddCommand(homerCallsCmd)
	homerCmd.AddCommand(homerAliasesCmd)

	// Search flags
	homerSearchCmd.Flags().String("since", "24h", "Start of time range (duration like 1h, 30m or timestamp like 2006-01-02 15:04)")
	homerSearchCmd.Flags().String("until", "", "End of time range (default: now)")
	homerSearchCmd.Flags().String("at", "", "Point in time to search around (±5 minutes)")
	homerSearchCmd.Flags().StringP("query", "q", "", "Query expression (e.g., \"from_user = '123' AND status = 200\")")
	homerSearchCmd.Flags().String("number", "", "Phone number (searches from_user and to_user with and without + prefix)")
	homerSearchCmd.Flags().String("from-user", "", "Filter by SIP from_user")
	homerSearchCmd.Flags().String("to-user", "", "Filter by SIP to_user")
	homerSearchCmd.Flags().String("ua", "", "Filter by SIP User-Agent")
	homerSearchCmd.Flags().String("call-id", "", "SIP Call-ID")
	homerSearchCmd.Flags().StringSliceP("method", "m", nil, "Filter by SIP method (repeatable, e.g. -m INVITE -m BYE)")
	homerSearchCmd.Flags().IntP("limit", "l", 200, "Maximum results")
	homerSearchCmd.Flags().StringP("output", "o", "", "Output format: json or jsonl")

	// Show flags
	homerShowCmd.Flags().String("from", "1h", "Time range start")
	homerShowCmd.Flags().String("to", "", "Time range end (default: now)")

	// Export flags
	homerExportCmd.Flags().String("from", "1h", "Time range start")
	homerExportCmd.Flags().String("to", "", "Time range end (default: now)")
	homerExportCmd.Flags().StringP("output", "o", "", "Output file path (default: <call-id>.pcap)")

	// Calls flags
	homerCallsCmd.Flags().String("since", "24h", "Start of time range (duration like 1h, 30m or timestamp like 2006-01-02 15:04)")
	homerCallsCmd.Flags().String("until", "", "End of time range (default: now)")
	homerCallsCmd.Flags().String("at", "", "Point in time to search around (±5 minutes)")
	homerCallsCmd.Flags().String("number", "", "Phone number (searches from_user and to_user with and without + prefix)")
	homerCallsCmd.Flags().String("from-user", "", "Filter by SIP from_user")
	homerCallsCmd.Flags().String("to-user", "", "Filter by SIP to_user")
	homerCallsCmd.Flags().String("ua", "", "Filter by SIP User-Agent")
	homerCallsCmd.Flags().StringP("query", "q", "", "Query expression (e.g., \"from_user = '123' AND status = 200\")")
	homerCallsCmd.Flags().IntP("limit", "l", 100, "Maximum number of calls to return")
	homerCallsCmd.Flags().StringP("output", "o", "", "Output format: json or jsonl")
}
