package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
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
		maxCallIDWidth := len("CALL-ID")
		maxUAWidth := len("USER-AGENT")
		for _, r := range records {
			if w := len(fmt.Sprintf("%s:%d", r.SrcIP, r.SrcPort)); w > maxSrcWidth {
				maxSrcWidth = w
			}
			if w := len(fmt.Sprintf("%s:%d", r.DstIP, r.DstPort)); w > maxDstWidth {
				maxDstWidth = w
			}
			if len(r.CallID) > maxCallIDWidth {
				maxCallIDWidth = len(r.CallID)
			}
			if len(r.UserAgent) > maxUAWidth {
				maxUAWidth = len(r.UserAgent)
			}
		}
		routeWidth := maxSrcWidth + 3 + maxDstWidth

		lineWidth := 20 + 2 + routeWidth + 2 + maxCallIDWidth + 2 + 10 + 2 + 20 + 2 + 20 + 2 + maxUAWidth
		line := strings.Repeat("─", lineWidth)
		fmt.Println()
		homerHeaderColor.Printf("  SIP Calls (%d)\n", len(records))
		fmt.Println("  " + line)
		fmt.Println()

		routeHeader := fmt.Sprintf("%-*s", routeWidth, "ROUTE")
		fmt.Printf("  %-20s  %s  %-*s  %-10s  %-20s  %-20s  %s\n",
			"DATE", routeHeader, maxCallIDWidth, "CALL-ID", "METHOD", "FROM", "TO", "USER-AGENT")
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
			printCallID(r.CallID, maxCallIDWidth)
			fmt.Print("  ")
			homerMethodColor.Printf("%-10s", method)
			fmt.Printf("  %-20s  %-20s  ", fromUser, toUser)
			printUserAgent(r.UserAgent)
		}
		fmt.Println()
	},
}

var homerShowCmd = &cobra.Command{
	Use:   "show <call-id> [call-id...]",
	Short: "Show SIP message flow for one or more calls",
	Long: `Display the SIP message ladder for one or more calls.

Multiple Call-IDs can be provided to show a combined message flow sorted by timestamp.
Use --raw to display the full raw SIP message bodies (headers + SDP).
Default time range is 10 days (matching Homer retention).

Examples:
  dex homer show abc123-def456@host
  dex homer show id1@host id2@host id3@host
  dex homer show abc123-def456@host --raw
  dex homer show abc123-def456@host --from 2h`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		fromStr, _ := cmd.Flags().GetString("from")
		toStr, _ := cmd.Flags().GetString("to")
		raw, _ := cmd.Flags().GetBool("raw")

		from, to, err := parseTimeRange(fromStr, toStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid time range: %v\n", err)
			os.Exit(1)
		}

		// Search for each Call-ID and merge results
		var merged *homer.SearchResult
		for _, callID := range args {
			params := homer.SearchParams{
				From:   from,
				To:     to,
				CallID: callID,
				Limit:  200,
			}
			result, err := client.SearchCalls(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get messages for %s: %v\n", callID, err)
				os.Exit(1)
			}
			merged = homer.MergeSearchResults(merged, result)
		}

		if merged == nil || len(merged.Data) == 0 {
			homerDimColor.Println("No messages found for the given call-id(s).")
			homerDimColor.Println("Tip: Try expanding the time range with --from")
			return
		}

		// Sort merged results by timestamp
		sort.Slice(merged.Data, func(i, j int) bool {
			return merged.Data[i].Date < merged.Data[j].Date
		})

		if raw {
			// Fetch full transaction with raw SIP bodies
			txnParams := homer.SearchParams{From: from, To: to}
			txn, err := client.GetTransaction(txnParams, merged.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get raw messages: %v\n", err)
				os.Exit(1)
			}

			// Sort transaction messages by timestamp
			sort.Slice(txn.Data.Messages, func(i, j int) bool {
				return txn.Data.Messages[i].CreateDate < txn.Data.Messages[j].CreateDate
			})

			printed := 0
			for _, msg := range txn.Data.Messages {
				if !msg.IsSIP() {
					continue
				}
				if printed > 0 {
					fmt.Println()
				}
				proto := "UDP"
				if msg.Protocol == 6 {
					proto = "TCP"
				}
				ts := time.UnixMilli(msg.CreateDate)
				homerDimColor.Printf("── %s %s  %s:%d → %s:%d ──\n",
					proto, ts.Format("2006-01-02 15:04:05.000"),
					msg.SrcIP, msg.SrcPort, msg.DstIP, msg.DstPort)
				fmt.Println(msg.Raw)
				printed++
			}
			if printed == 0 {
				homerDimColor.Println("No raw SIP messages available.")
			}
			return
		}

		label := args[0]
		if len(args) > 1 {
			label = fmt.Sprintf("%d call-ids", len(args))
		}

		line := strings.Repeat("─", 100)
		fmt.Println()
		homerHeaderColor.Printf("  SIP Message Flow - %s (%d messages)\n", label, len(merged.Data))
		fmt.Println("  " + line)
		fmt.Println()

		// Table header
		fmt.Printf("  %-24s  %-22s  %-7s %-22s  %s\n",
			"TIME", "SOURCE", "", "DESTINATION", "METHOD/STATUS")
		fmt.Println("  " + line)

		for _, msg := range merged.Data {
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

// parseTimeValueInLocation parses a string that is either a duration (e.g., "1h", "30m", "2d")
// or an absolute timestamp (e.g., "2026-02-04 17:13", "2026-02-04T17:13:00").
// Timestamps with explicit timezone suffixes (Z, +02:00) use the embedded timezone.
// Naive timestamps (no tz suffix) use the provided location.
// Durations are interpreted as "that long ago from now".
func parseTimeValueInLocation(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now(), nil
	}

	// Try timezone-aware formats first (embedded tz takes precedence)
	tzFormats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05Z07:00",
	}
	for _, f := range tzFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}

	// Try naive timestamp formats (use provided location)
	naiveFormats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, f := range naiveFormats {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
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

// parseTimeValue parses a string that is either a duration or timestamp using local timezone.
func parseTimeValue(s string) (time.Time, error) {
	return parseTimeValueInLocation(s, time.Local)
}

var homerAnalyzeCmd = &cobra.Command{
	Use:   "analyze [call-id]",
	Short: "Find all SIP legs belonging to the same call",
	Long: `Analyze a SIP call by correlating legs via a shared header value (e.g., X-Acme-Call-ID).

Starting from a seed call (by Call-ID or by from/to user), the command:
1. Fetches the seed call's raw SIP messages
2. Extracts the correlation header value(s) from INVITE messages
3. Fans out to find other legs in the same time window by phone number
4. Filters candidates that share the same correlation header value

Entry point (one required):
  Positional <call-id>     A specific SIP Call-ID as the seed
  --from-user + --to-user  Caller/callee pair (needs --at or --since for time)

Examples:
  dex homer analyze BW171313801040226178186286@62.156.74.72 \
    -c X-Acme-Call-ID --url https://homer.example.com/

  dex homer analyze BW171313801040226178186286@62.156.74.72 \
    -c X-Acme-Call-ID -H X-Acme --at "2026-02-04 17:13" \
    --url https://homer.example.com/

  dex homer analyze --from-user 4921514174858 --to-user 4934155003500 \
    --at "2026-02-04 17:13" -c X-Acme-Call-ID --url https://homer.example.com/`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getHomerClient(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		correlateHeaders, _ := cmd.Flags().GetStringSlice("correlate")
		displayHeaders, _ := cmd.Flags().GetStringSlice("header")
		extraNumbers, _ := cmd.Flags().GetStringSlice("number")
		fromUser, _ := cmd.Flags().GetString("from-user")
		toUser, _ := cmd.Flags().GetString("to-user")
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		atStr, _ := cmd.Flags().GetString("at")
		limit, _ := cmd.Flags().GetInt("limit")
		output, _ := cmd.Flags().GetString("output")

		if len(correlateHeaders) == 0 {
			fmt.Fprintf(os.Stderr, "At least one --correlate (-c) header is required\n")
			os.Exit(1)
		}

		hasCallID := len(args) == 1
		hasFromTo := fromUser != "" && toUser != ""

		if !hasCallID && !hasFromTo {
			fmt.Fprintf(os.Stderr, "Provide a Call-ID argument or both --from-user and --to-user\n")
			os.Exit(1)
		}
		if hasCallID && hasFromTo {
			fmt.Fprintf(os.Stderr, "Provide either a Call-ID argument or --from-user/--to-user, not both\n")
			os.Exit(1)
		}

		// --- Step 1: Find seed call ---
		var seedParams homer.SearchParams
		if hasCallID {
			// Search by Call-ID, wide time window
			var from, to time.Time
			if atStr != "" {
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
			seedParams = homer.SearchParams{
				From:   from,
				To:     to,
				CallID: args[0],
				Limit:  200,
			}
		} else {
			// Search by from/to user
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

			var criteria [][]string
			bareFrom := strings.TrimPrefix(fromUser, "+")
			plusFrom := "+" + bareFrom
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.from_user = '%s'", bareFrom),
				fmt.Sprintf("data_header.from_user = '%s'", plusFrom),
			})
			bareTo := strings.TrimPrefix(toUser, "+")
			plusTo := "+" + bareTo
			criteria = append(criteria, []string{
				fmt.Sprintf("data_header.to_user = '%s'", bareTo),
				fmt.Sprintf("data_header.to_user = '%s'", plusTo),
			})

			seedParams = homer.SearchParams{
				From:       from,
				To:         to,
				SmartInput: buildSmartInput(criteria),
				Limit:      limit,
			}
		}

		seedResult, err := client.SearchCalls(seedParams)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Seed search failed: %v\n", err)
			os.Exit(1)
		}
		if len(seedResult.Data) == 0 {
			homerDimColor.Println("No seed call found.")
			return
		}

		// Group seed messages by Call-ID
		seedCalls := homer.GroupCalls(seedResult.Data, "")
		if len(seedCalls) == 0 {
			homerDimColor.Println("No seed call found.")
			return
		}

		// When using --from-user/--to-user, require exactly one Call-ID
		if hasFromTo && len(seedCalls) > 1 {
			fmt.Fprintf(os.Stderr, "Ambiguous: found %d calls matching from/to user. Re-run with a specific Call-ID:\n\n", len(seedCalls))
			// Sort by start time for display
			sort.Slice(seedCalls, func(i, j int) bool {
				return seedCalls[i].StartTime.Before(seedCalls[j].StartTime)
			})
			for _, c := range seedCalls {
				fmt.Fprintf(os.Stderr, "  %s  %s  %s → %s\n",
					c.StartTime.Format("2006-01-02 15:04:05"), c.CallID, c.Caller, c.Callee)
			}
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}

		seedCall := seedCalls[0]

		// Extract caller number from seed for fan-out
		seedFromUser := seedCall.Caller

		// --- Step 2: Fan out by caller number + extra numbers ---
		// Build a flat OR of all numbers to search for. The seed's from_user is
		// always included. Extra --number values widen the search to find legs
		// involving agents/extensions that don't share the caller number.
		// Correlation header filtering (step 4) weeds out false positives.
		margin := 30 * time.Minute
		fanFrom := seedCall.StartTime.Add(-margin)
		fanTo := seedCall.EndTime.Add(margin)

		var fanAlternatives []string
		if seedFromUser != "" {
			bare := strings.TrimPrefix(seedFromUser, "+")
			fanAlternatives = append(fanAlternatives,
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", "+"+bare),
			)
		}
		for _, num := range extraNumbers {
			bare := strings.TrimPrefix(num, "+")
			fanAlternatives = append(fanAlternatives,
				fmt.Sprintf("data_header.from_user = '%s'", bare),
				fmt.Sprintf("data_header.from_user = '%s'", "+"+bare),
				fmt.Sprintf("data_header.to_user = '%s'", bare),
				fmt.Sprintf("data_header.to_user = '%s'", "+"+bare),
			)
		}

		var fanCriteria [][]string
		if len(fanAlternatives) > 0 {
			fanCriteria = append(fanCriteria, fanAlternatives)
		}

		fanParams := homer.SearchParams{
			From:       fanFrom,
			To:         fanTo,
			SmartInput: buildSmartInput(fanCriteria),
		}

		fanCalls, err := client.FetchCalls(fanParams, "", limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fan-out search failed: %v\n", err)
			os.Exit(1)
		}

		// Collect all messages from fan-out calls + seed into a merged SearchResult
		var fanRecords []homer.CallRecord
		for _, c := range fanCalls {
			fanRecords = append(fanRecords, c.Messages...)
		}
		fanResult := &homer.SearchResult{Data: fanRecords}

		// Merge seed results into fan-out (seed Call-ID may not appear in phone-based search)
		fanResult = homer.MergeSearchResults(fanResult, seedResult)

		if len(fanResult.Data) == 0 {
			homerDimColor.Println("  No candidate legs found.")
			return
		}

		// --- Step 3: Extract correlation headers from all candidates ---
		candidateTxn, err := client.GetTransaction(fanParams, fanResult.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get candidate raw messages: %v\n", err)
			os.Exit(1)
		}

		// Build map: Call-ID -> set of header values, and reverse: header value -> set of Call-IDs
		callIDValues := make(map[string]map[string]map[string]bool) // callID -> header -> values
		valueCallIDs := make(map[string]map[string]map[string]bool) // header -> value -> callIDs

		for _, h := range correlateHeaders {
			valueCallIDs[h] = make(map[string]map[string]bool)
		}

		for _, msg := range candidateTxn.Data.Messages {
			if !msg.IsSIP() || msg.Raw == "" {
				continue
			}
			if !strings.HasPrefix(msg.Raw, "INVITE ") {
				continue
			}
			for _, h := range correlateHeaders {
				val := homer.ExtractSIPHeader(msg.Raw, h)
				if val == "" {
					continue
				}
				if callIDValues[msg.CallID] == nil {
					callIDValues[msg.CallID] = make(map[string]map[string]bool)
				}
				if callIDValues[msg.CallID][h] == nil {
					callIDValues[msg.CallID][h] = make(map[string]bool)
				}
				callIDValues[msg.CallID][h][val] = true

				if valueCallIDs[h][val] == nil {
					valueCallIDs[h][val] = make(map[string]bool)
				}
				valueCallIDs[h][val][msg.CallID] = true
			}
		}

		// --- Step 4: Find the correlation group containing the seed ---
		// Group all Call-IDs by shared header values, then pick the group
		// that temporally overlaps with the seed call.
		// The seed (external leg) may not have the header itself, but the
		// internal legs spawned from it do.

		// Find all unique correlation values and their Call-ID sets
		allGroups := make(map[string]map[string]bool) // "header:value" -> set of Call-IDs
		for _, h := range correlateHeaders {
			for val, cids := range valueCallIDs[h] {
				key := h + ":" + val
				allGroups[key] = cids
			}
		}

		if len(allGroups) == 0 {
			homerWarnColor.Println("  No correlation header values found in any candidate INVITEs")
			homerDimColor.Printf("  Searched %d SIP messages for headers: %s\n", len(candidateTxn.Data.Messages), strings.Join(correlateHeaders, ", "))
			return
		}

		// Group fan-out data by Call-ID to check temporal overlap
		allCandidateCalls := homer.GroupCalls(fanResult.Data, "")
		candidateByCallID := make(map[string]homer.CallSummary)
		for _, c := range allCandidateCalls {
			candidateByCallID[c.CallID] = c
		}

		// For each correlation group, check if any member overlaps temporally with the seed
		matchingCallIDs := make(map[string]bool)
		matchingCallIDs[seedCall.CallID] = true

		fmt.Println()
		for groupKey, cids := range allGroups {
			// Check temporal overlap: any Call-ID in this group starts within
			// a small window around the seed call's start time?
			// Internal legs are spawned within seconds of the external INVITE.
			overlaps := false
			for cid := range cids {
				if c, ok := candidateByCallID[cid]; ok {
					if c.StartTime.After(seedCall.StartTime.Add(-5*time.Second)) &&
						c.StartTime.Before(seedCall.StartTime.Add(30*time.Second)) {
						overlaps = true
						break
					}
				}
			}
			if !overlaps {
				continue
			}
			// This group overlaps with seed — include all its Call-IDs
			parts := strings.SplitN(groupKey, ":", 2)
			homerDimColor.Printf("  Correlating via %s: ", parts[0])
			homerHeaderColor.Println(parts[1])
			for cid := range cids {
				matchingCallIDs[cid] = true
			}
		}
		fmt.Println()

		// --- Step 4b: Multi-hop number correlation ---
		// Include fan-out legs that involve a -N number. The -N flag signals
		// user intent: "this number is related to this call." Any fan-out
		// leg whose FROM or TO matches a -N number is included, even if it
		// doesn't share the correlation header.
		if len(extraNumbers) > 0 {
			extraNumberSet := make(map[string]bool)
			for _, num := range extraNumbers {
				bare := strings.TrimPrefix(num, "+")
				if bare != "" {
					extraNumberSet[bare] = true
				}
			}

			addedHop := false
			for _, c := range allCandidateCalls {
				if matchingCallIDs[c.CallID] {
					continue
				}
				callerBare := strings.TrimPrefix(c.Caller, "+")
				calleeBare := strings.TrimPrefix(c.Callee, "+")

				if !extraNumberSet[callerBare] && !extraNumberSet[calleeBare] {
					continue
				}

				if !addedHop {
					homerDimColor.Println("  Including related legs (via -N number):")
					addedHop = true
				}
				homerDimColor.Printf("    %s (%s → %s)\n", c.CallID, c.Caller, c.Callee)
				matchingCallIDs[c.CallID] = true
			}
			if addedHop {
				fmt.Println()
			}
		}

		// --- Step 5: Display correlated legs ---
		// Group fan-out results
		allCalls := homer.GroupCalls(fanResult.Data, "")

		// Filter to only matching Call-IDs
		var correlated []homer.CallSummary
		for _, c := range allCalls {
			if matchingCallIDs[c.CallID] {
				correlated = append(correlated, c)
			}
		}

		// Also ensure seed call is included (it might not be in the fan-out results)
		seedIncluded := false
		for _, c := range correlated {
			if c.CallID == seedCall.CallID {
				seedIncluded = true
				break
			}
		}
		if !seedIncluded {
			correlated = append(correlated, seedCall)
		}

		// Sort by start time
		sort.Slice(correlated, func(i, j int) bool {
			return correlated[i].StartTime.Before(correlated[j].StartTime)
		})

		// JSON/JSONL output
		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(correlated)
			return
		}
		if output == "jsonl" {
			enc := json.NewEncoder(os.Stdout)
			for _, c := range correlated {
				enc.Encode(c)
			}
			return
		}

		// Build transaction message index by Call-ID
		txnByCallID := make(map[string][]homer.TransactionMessage)
		for _, msg := range candidateTxn.Data.Messages {
			txnByCallID[msg.CallID] = append(txnByCallID[msg.CallID], msg)
		}

		// Fix up status and duration from transaction data.
		// The fan-out discovery may only return a subset of messages per call,
		// so status and end time can be wrong. Transaction data has everything.
		for i := range correlated {
			msgs := txnByCallID[correlated[i].CallID]
			if len(msgs) == 0 {
				continue
			}
			// Derive status from highest SIP response code
			var highestCode int
			var latestTS int64
			for _, m := range msgs {
				if m.CreateDate > latestTS {
					latestTS = m.CreateDate
				}
				if !m.IsSIP() || m.Raw == "" {
					continue
				}
				// Response lines start with "SIP/2.0 NNN"
				if strings.HasPrefix(m.Raw, "SIP/2.0 ") {
					parts := strings.Fields(m.Raw)
					if len(parts) >= 2 {
						if code, err := strconv.Atoi(parts[1]); err == nil && code > highestCode {
							highestCode = code
						}
					}
				}
			}
			if highestCode > 0 {
				switch {
				case highestCode >= 200 && highestCode < 300:
					correlated[i].Status = "answered"
				case highestCode == 486:
					correlated[i].Status = "busy"
				case highestCode == 487:
					correlated[i].Status = "cancelled"
				case highestCode == 408 || highestCode == 480:
					correlated[i].Status = "no answer"
				case highestCode >= 400:
					correlated[i].Status = "failed"
				case highestCode >= 100:
					correlated[i].Status = "ringing"
				}
			}
			if latestTS > 0 {
				endTime := time.UnixMilli(latestTS)
				if endTime.After(correlated[i].EndTime) {
					correlated[i].EndTime = endTime
					correlated[i].Duration = endTime.Sub(correlated[i].StartTime)
				}
			}
		}

		// Find first INVITE raw body per Call-ID
		firstInviteRaw := make(map[string]string)
		for callID, msgs := range txnByCallID {
			for _, msg := range msgs {
				if msg.IsSIP() && strings.HasPrefix(msg.Raw, "INVITE ") {
					firstInviteRaw[callID] = msg.Raw
					break
				}
			}
		}

		// Extract dynamic display columns from -H prefix matching
		var dynColumns []string
		dynColumnSet := make(map[string]bool)
		legDynValues := make(map[string]map[string]string) // callID -> headerName -> value

		if len(displayHeaders) > 0 {
			for _, c := range correlated {
				rawMsg, ok := firstInviteRaw[c.CallID]
				if !ok {
					continue
				}
				vals := make(map[string]string)
				for _, prefix := range displayHeaders {
					for name, val := range homer.ExtractSIPHeadersByPrefix(rawMsg, prefix) {
						vals[name] = val
						if !dynColumnSet[name] {
							dynColumnSet[name] = true
							dynColumns = append(dynColumns, name)
						}
					}
				}
				legDynValues[c.CallID] = vals
			}
			sort.Strings(dynColumns)
		}

		// Compute dynamic column widths
		dynColWidths := make(map[string]int)
		for _, col := range dynColumns {
			dynColWidths[col] = len(col)
		}
		for _, c := range correlated {
			vals := legDynValues[c.CallID]
			for _, col := range dynColumns {
				if w := len(vals[col]); w > dynColWidths[col] {
					dynColWidths[col] = w
				}
			}
		}

		// Compute t0 for relative time
		var t0 time.Time
		if len(correlated) > 0 {
			t0 = correlated[0].StartTime
		}

		// --- Block 1: Leg overview table ---
		maxTimeWidth := len("TIME")
		maxCallIDWidth := len("CALL-ID")
		maxFromWidth := len("FROM")
		maxToWidth := len("TO")
		maxRouteWidth := len("ROUTE")

		type legDisplay struct {
			timeStr string
			callID  string
			from    string
			to      string
			route   string
			status  string
			dynVals map[string]string
		}
		var rows []legDisplay
		for _, c := range correlated {
			timeStr := formatCorrelateTime(c, t0)
			route := homer.FormatRoute(homer.DeriveRoute(c.Messages))
			from := c.Caller
			if from == "" {
				from = "-"
			}
			to := c.Callee
			if to == "" {
				to = "-"
			}
			dynVals := legDynValues[c.CallID]
			if dynVals == nil {
				dynVals = make(map[string]string)
			}
			rows = append(rows, legDisplay{
				timeStr: timeStr,
				callID:  c.CallID,
				from:    from,
				to:      to,
				route:   route,
				status:  c.Status,
				dynVals: dynVals,
			})
			if len(timeStr) > maxTimeWidth {
				maxTimeWidth = len(timeStr)
			}
			if len(c.CallID) > maxCallIDWidth {
				maxCallIDWidth = len(c.CallID)
			}
			if len(from) > maxFromWidth {
				maxFromWidth = len(from)
			}
			if len(to) > maxToWidth {
				maxToWidth = len(to)
			}
			if len(route) > maxRouteWidth {
				maxRouteWidth = len(route)
			}
		}

		lineWidth := maxTimeWidth + 2 + maxCallIDWidth + 2 + maxFromWidth + 2 + maxToWidth + 2 + maxRouteWidth + 2 + 12
		for _, col := range dynColumns {
			lineWidth += 2 + dynColWidths[col]
		}
		line := strings.Repeat("─", lineWidth)

		dateStr := ""
		if len(correlated) > 0 {
			dateStr = " - " + t0.Format("2006-01-02")
		}
		homerHeaderColor.Printf("  Correlated Legs (%d)%s\n", len(correlated), dateStr)
		fmt.Println("  " + line)
		fmt.Println()

		fmt.Printf("  %-*s  %-*s  %-*s  %-*s  %-*s",
			maxTimeWidth, "TIME",
			maxCallIDWidth, "CALL-ID",
			maxFromWidth, "FROM",
			maxToWidth, "TO",
			maxRouteWidth, "ROUTE")
		for _, col := range dynColumns {
			fmt.Printf("  %-*s", dynColWidths[col], col)
		}
		fmt.Printf("  %s\n", "STATUS")
		fmt.Println("  " + line)

		for _, r := range rows {
			fmt.Printf("  %-*s  ", maxTimeWidth, r.timeStr)
			printCallID(r.callID, maxCallIDWidth)
			fmt.Printf("  %-*s  %-*s  %-*s", maxFromWidth, r.from, maxToWidth, r.to, maxRouteWidth, r.route)
			for _, col := range dynColumns {
				val := r.dynVals[col]
				if val == "" {
					val = "-"
				}
				fmt.Printf("  %-*s", dynColWidths[col], val)
			}
			fmt.Print("  ")
			formatCallStatus(r.status)
			fmt.Println()
		}
		fmt.Println()

		// --- Block 2: SIP message flow (ladder diagram) ---
		// Collect SIP messages from correlated Call-IDs
		var flowMsgs []homer.TransactionMessage
		for _, msg := range candidateTxn.Data.Messages {
			if msg.IsSIP() && matchingCallIDs[msg.CallID] {
				flowMsgs = append(flowMsgs, msg)
			}
		}
		if len(flowMsgs) == 0 {
			return
		}

		sort.Slice(flowMsgs, func(i, j int) bool {
			return flowMsgs[i].CreateDate < flowMsgs[j].CreateDate
		})

		// Determine endpoint order (left to right following INVITE chain from seed)
		endpoints := correlateEndpointOrder(flowMsgs, seedCall.CallID)
		epIndex := make(map[string]int)
		for i, ep := range endpoints {
			epIndex[ep] = i
		}

		// Build leg index (Call-ID -> leg number)
		legIndex := make(map[string]int)
		for i, c := range correlated {
			legIndex[c.CallID] = i + 1
		}

		// Map endpoints to notable phone numbers.
		// Build set of numbers the user cares about (from -N, --from-user, --to-user).
		notableNumbers := make(map[string]bool)
		for _, num := range extraNumbers {
			bare := strings.TrimPrefix(num, "+")
			if bare != "" {
				notableNumbers[bare] = true
			}
		}
		if fromUser != "" {
			notableNumbers[strings.TrimPrefix(fromUser, "+")] = true
		}
		if toUser != "" {
			notableNumbers[strings.TrimPrefix(toUser, "+")] = true
		}

		// Scan INVITE messages: source IP hosts FromUser, destination IP hosts ToUser
		epNumbers := make(map[string]string) // IP -> first notable number seen
		if len(notableNumbers) > 0 {
			for _, msg := range flowMsgs {
				if !msg.IsSIP() || !strings.HasPrefix(msg.Raw, "INVITE ") {
					continue
				}
				fromBare := strings.TrimPrefix(msg.FromUser, "+")
				toBare := strings.TrimPrefix(msg.ToUser, "+")
				if notableNumbers[fromBare] && epNumbers[msg.SrcIP] == "" {
					epNumbers[msg.SrcIP] = msg.FromUser
				}
				if notableNumbers[toBare] && epNumbers[msg.DstIP] == "" {
					epNumbers[msg.DstIP] = msg.ToUser
				}
			}
		}

		// Compute column width (min 16, fits longest endpoint label + padding)
		flowColWidth := 16
		for _, ep := range endpoints {
			if w := len(ep) + 4; w > flowColWidth {
				flowColWidth = w
			}
			if num, ok := epNumbers[ep]; ok {
				if w := len(num) + 4; w > flowColWidth {
					flowColWidth = w
				}
			}
		}

		// Time prefix width: "15:04:05 (+999ms) " = 19 chars
		flowTimeWidth := 20

		flowTotalWidth := flowTimeWidth + len(endpoints)*flowColWidth + 8
		flowLine := strings.Repeat("─", flowTotalWidth)

		homerHeaderColor.Println("  Message Flow")
		fmt.Println("  " + flowLine)
		fmt.Println()

		// Endpoint header labels (IP), centered around the pipe position
		fmt.Printf("  %-*s", flowTimeWidth, "")
		homerDimColor.Println(flowBuildLabelRow(endpoints, len(endpoints), flowColWidth))

		// Endpoint sub-labels (phone numbers, if any), centered around the pipe
		hasSubLabels := len(epNumbers) > 0
		if hasSubLabels {
			numLabels := make([]string, len(endpoints))
			for i, ep := range endpoints {
				if num, ok := epNumbers[ep]; ok {
					numLabels[i] = num
				}
			}
			fmt.Printf("  %-*s", flowTimeWidth, "")
			homerHeaderColor.Println(flowBuildLabelRow(numLabels, len(endpoints), flowColWidth))
		}

		// Initial pipe row
		fmt.Printf("  %-*s", flowTimeWidth, "")
		pipeRow := buildFlowPipeRow(len(endpoints), flowColWidth)
		fmt.Println(pipeRow)

		// Render each SIP message as a ladder arrow
		for _, msg := range flowMsgs {
			srcIdx, srcOK := epIndex[msg.SrcIP]
			dstIdx, dstOK := epIndex[msg.DstIP]
			if !srcOK || !dstOK || srcIdx == dstIdx {
				continue
			}

			method := correlateMethodFromRaw(msg.Raw)
			if method == "" {
				method = msg.Method
			}
			if method == "" {
				continue
			}

			msgTime := time.UnixMilli(msg.CreateDate)
			offset := msgTime.Sub(t0)
			timeStr := formatFlowOffset(msgTime, offset)

			arrowRow := buildFlowArrowRow(len(endpoints), flowColWidth, srcIdx, dstIdx, method)

			homerDimColor.Printf("  %-*s", flowTimeWidth, timeStr)
			fmt.Print(arrowRow)

			if leg, ok := legIndex[msg.CallID]; ok {
				homerDimColor.Printf("  Leg %d", leg)
			}
			fmt.Println()
		}

		// Final pipe row
		fmt.Printf("  %-*s", flowTimeWidth, "")
		fmt.Println(pipeRow)
		fmt.Println()
	},
}

// formatCorrelateTime formats a compact relative time string for correlate output.
// Format: "HH:MM:SS (+Xs)  duration" where offset is relative to t0.
func formatCorrelateTime(c homer.CallSummary, t0 time.Time) string {
	start := c.StartTime.Format("15:04:05")
	offset := c.StartTime.Sub(t0)

	var offsetStr string
	if offset < time.Second {
		offsetStr = "(+0s)"
	} else if offset < time.Minute {
		offsetStr = fmt.Sprintf("(+%ds)", int(offset.Seconds()))
	} else {
		offsetStr = fmt.Sprintf("(+%s)", formatDuration(offset))
	}

	dur := ""
	if c.MsgCount > 1 {
		dur = "  " + formatDuration(c.Duration)
	}

	return fmt.Sprintf("%s %s%s", start, offsetStr, dur)
}

// correlateEndpointOrder traces the INVITE chain to order endpoints left-to-right.
// Starts from the seed call's first INVITE source, then follows INVITE destinations.
func correlateEndpointOrder(msgs []homer.TransactionMessage, seedCallID string) []string {
	var ordered []string
	seen := make(map[string]bool)

	// Find seed call's first INVITE source → leftmost endpoint
	for _, m := range msgs {
		if m.CallID == seedCallID && m.IsSIP() && strings.HasPrefix(m.Raw, "INVITE ") {
			if !seen[m.SrcIP] {
				ordered = append(ordered, m.SrcIP)
				seen[m.SrcIP] = true
			}
			if !seen[m.DstIP] {
				ordered = append(ordered, m.DstIP)
				seen[m.DstIP] = true
			}
			break
		}
	}

	// BFS: follow INVITE destinations from known endpoints
	for i := 0; i < len(ordered); i++ {
		for _, m := range msgs {
			if !m.IsSIP() || !strings.HasPrefix(m.Raw, "INVITE ") {
				continue
			}
			if m.SrcIP == ordered[i] && !seen[m.DstIP] {
				ordered = append(ordered, m.DstIP)
				seen[m.DstIP] = true
			}
		}
	}

	// Add any remaining IPs not discovered via INVITE chain
	for _, m := range msgs {
		if !m.IsSIP() {
			continue
		}
		for _, ip := range []string{m.SrcIP, m.DstIP} {
			if !seen[ip] {
				ordered = append(ordered, ip)
				seen[ip] = true
			}
		}
	}

	return ordered
}

// flowBuildLabelRow builds a row of labels centered around pipe positions.
// Each label[i] is centered at column position i*colWidth within a buffer
// of numCols*colWidth bytes. Labels are guaranteed to have at least 1 space
// between them and won't extend past the buffer.
func flowBuildLabelRow(labels []string, numCols, colWidth int) string {
	total := numCols * colWidth
	buf := make([]byte, total)
	for i := range buf {
		buf[i] = ' '
	}

	// First pass: compute start positions
	type placement struct {
		start int
		label string
	}
	placements := make([]placement, 0, len(labels))
	for i, label := range labels {
		if label == "" {
			continue
		}
		center := i * colWidth
		start := center - len(label)/2
		if start < 0 {
			start = 0
		}
		placements = append(placements, placement{start, label})
	}

	// Second pass: ensure minimum 1 space gap between adjacent labels
	for i := 1; i < len(placements); i++ {
		prevEnd := placements[i-1].start + len(placements[i-1].label)
		if placements[i].start <= prevEnd {
			placements[i].start = prevEnd + 1
		}
	}

	// Write to buffer
	for _, p := range placements {
		for j, ch := range []byte(p.label) {
			pos := p.start + j
			if pos < total {
				buf[pos] = ch
			}
		}
	}
	return string(buf)
}

// buildFlowPipeRow builds a pipe row for the ladder diagram: "│" at each column center.
func buildFlowPipeRow(numCols, colWidth int) string {
	buf := make([]rune, numCols*colWidth)
	for i := range buf {
		buf[i] = ' '
	}
	for i := range numCols {
		buf[i*colWidth] = '│'
	}
	return string(buf)
}

// buildFlowArrowRow builds an arrow row for the ladder diagram.
// Draws a gapless arrow from srcIdx to dstIdx with the method label centered on it.
func buildFlowArrowRow(numCols, colWidth, srcIdx, dstIdx int, method string) string {
	buf := make([]rune, numCols*colWidth)
	for i := range buf {
		buf[i] = ' '
	}

	leftIdx := srcIdx
	rightIdx := dstIdx
	if srcIdx > dstIdx {
		leftIdx = dstIdx
		rightIdx = srcIdx
	}

	// Place pipes for columns outside the arrow range
	for i := range numCols {
		if i < leftIdx || i > rightIdx {
			buf[i*colWidth] = '│'
		}
	}

	// Draw line between left and right column positions
	leftPos := leftIdx * colWidth
	rightPos := rightIdx * colWidth
	for i := leftPos; i <= rightPos; i++ {
		buf[i] = '─'
	}

	// Source/destination: keep │ with space separating it from the arrow.
	// Arrowhead sits one position inside the space.
	if srcIdx < dstIdx {
		// left=source, right=destination
		buf[leftPos] = '│'
		buf[leftPos+1] = ' '
		buf[rightPos] = '│'
		buf[rightPos-1] = ' '
		buf[rightPos-2] = '▶'
	} else {
		// right=source, left=destination
		buf[rightPos] = '│'
		buf[rightPos-1] = ' '
		buf[leftPos] = '│'
		buf[leftPos+1] = ' '
		buf[leftPos+2] = '◀'
	}

	// Intermediate columns: crossing character where arrow passes through
	for i := leftIdx + 1; i < rightIdx; i++ {
		buf[i*colWidth] = '─'
	}

	// Place method label in the widest segment between crossings.
	label := []rune(" " + method + " ")
	// Collect segment boundaries: [segStart, segEnd) pairs where label can go.
	// Each segment runs between source/crossings/destination, leaving 1 char
	// padding around each crossing and 2 chars at source/destination for "│ "/"▶ │".
	boundaries := []int{leftPos + 2}
	for i := leftIdx + 1; i < rightIdx; i++ {
		pos := i * colWidth
		boundaries = append(boundaries, pos)   // end of prev segment (at crossing)
		boundaries = append(boundaries, pos+1) // start of next segment (after crossing)
	}
	boundaries = append(boundaries, rightPos-2)

	// Find widest segment
	bestStart, bestWidth := 0, 0
	for i := 0; i < len(boundaries)-1; i += 2 {
		segStart := boundaries[i]
		segEnd := boundaries[i+1]
		w := segEnd - segStart
		if w > bestWidth {
			bestWidth = w
			bestStart = segStart
			// Center label in this segment
		}
	}

	if len(label) <= bestWidth {
		segMid := bestStart + bestWidth/2
		labelStart := segMid - len(label)/2
		copy(buf[labelStart:], label)
	}

	return string(buf)
}

// correlateMethodFromRaw extracts the SIP method or response code from a raw SIP message.
func correlateMethodFromRaw(raw string) string {
	if raw == "" {
		return ""
	}
	firstLine := raw
	if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
		firstLine = raw[:idx]
	}
	firstLine = strings.TrimRight(firstLine, "\r")

	if strings.HasPrefix(firstLine, "SIP/") {
		// Response: "SIP/2.0 200 OK" → "200"
		parts := strings.SplitN(firstLine[4:], " ", 3)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Request: "INVITE sip:... SIP/2.0" → "INVITE"
	if idx := strings.IndexByte(firstLine, ' '); idx >= 0 {
		return firstLine[:idx]
	}

	return ""
}

// formatFlowOffset formats "HH:MM:SS (+offset)" for the flow diagram.
func formatFlowOffset(t time.Time, d time.Duration) string {
	clock := t.Format("15:04:05")
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%s (+%dms)", clock, ms)
	}
	s := float64(ms) / 1000
	if s < 60 {
		return fmt.Sprintf("%s (+%.1fs)", clock, s)
	}
	return fmt.Sprintf("%s (+%s)", clock, formatDuration(d))
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
	homerCmd.AddCommand(homerAnalyzeCmd)

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
	homerShowCmd.Flags().String("from", "10d", "Time range start (default: 10 days)")
	homerShowCmd.Flags().String("to", "", "Time range end (default: now)")
	homerShowCmd.Flags().Bool("raw", false, "Display raw SIP message bodies")

	// Export flags
	homerExportCmd.Flags().String("from", "10d", "Time range start (default: 10 days)")
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

	// Analyze flags
	homerAnalyzeCmd.Flags().StringSliceP("correlate", "c", nil, "SIP header to correlate legs by (exact match, repeatable, required)")
	homerAnalyzeCmd.Flags().StringSliceP("header", "H", nil, "SIP header prefix to show as table columns (prefix match, repeatable)")
	homerAnalyzeCmd.Flags().StringSliceP("number", "N", nil, "Extra number to include in fan-out search (e.g., agent extension)")
	homerAnalyzeCmd.Flags().String("from-user", "", "Seed: SIP from_user")
	homerAnalyzeCmd.Flags().String("to-user", "", "Seed: SIP to_user")
	homerAnalyzeCmd.Flags().String("since", "10d", "Time range start (default: 10 days)")
	homerAnalyzeCmd.Flags().String("until", "", "Time range end (default: now)")
	homerAnalyzeCmd.Flags().String("at", "", "Point in time ±5 min")
	homerAnalyzeCmd.Flags().IntP("limit", "l", 100, "Max calls per search")
	homerAnalyzeCmd.Flags().StringP("output", "o", "", "Output format: json, jsonl")
}
