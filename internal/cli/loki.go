package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/k8s"
	"github.com/codewandler/dex/internal/loki"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	lokiHeaderColor  = color.New(color.FgCyan, color.Bold)
	lokiLabelColor   = color.New(color.FgYellow)
	lokiTimeColor    = color.New(color.FgHiBlack)
	lokiLineColor    = color.New(color.FgWhite)
	lokiSuccessColor = color.New(color.FgGreen)
	lokiErrorColor   = color.New(color.FgRed)
)

// getLokiURL returns the Loki URL from flag, config, or auto-discovery
func getLokiURL(urlFlag string) (string, error) {
	// 1. Check flag
	if urlFlag != "" {
		return urlFlag, nil
	}

	// 2. Check config
	cfg, err := config.Load()
	if err == nil && cfg.Loki.URL != "" {
		return cfg.Loki.URL, nil
	}

	// 3. Auto-discover
	lokiTimeColor.Println("No Loki URL configured, attempting auto-discovery...")
	url, err := discoverLokiURL()
	if err != nil {
		return "", fmt.Errorf("auto-discovery failed: %w\nTip: Use --url flag or set LOKI_URL environment variable", err)
	}
	lokiTimeColor.Printf("Auto-discovered Loki at %s\n\n", url)
	return url, nil
}

// discoverLokiURL finds a working Loki URL in the current Kubernetes cluster
func discoverLokiURL() (string, error) {
	// Verify k8s connectivity first
	if _, err := k8s.NewClient(""); err != nil {
		return "", fmt.Errorf("failed to connect to Kubernetes: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Common namespaces to search
	searchNamespaces := []string{"monitoring", "loki", "observability", "logging", "loki-stack"}

	type candidate struct {
		url       string
		namespace string
		name      string
		podIP     string
	}
	var candidates []candidate
	var lastErr error
	searched := 0

	// Search for Loki pods by name pattern in each namespace
	for _, ns := range searchNamespaces {
		nsClient, err := k8s.NewClient(ns)
		if err != nil {
			lastErr = err
			continue
		}

		pods, err := nsClient.ListPods(ctx, false)
		if err != nil {
			lastErr = err
			continue
		}
		searched++

		for _, pod := range pods {
			nameLower := strings.ToLower(pod.Name)
			// Match loki pods but exclude promtail/agents
			if !strings.Contains(nameLower, "loki") || strings.Contains(nameLower, "promtail") {
				continue
			}

			// Skip pods that aren't running
			if pod.Status.Phase != "Running" {
				continue
			}

			// Skip pods without an IP
			if pod.Status.PodIP == "" {
				continue
			}

			// Find HTTP port (usually 3100)
			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if port.ContainerPort == 3100 || port.Name == "http-metrics" || port.Name == "http" {
						url := fmt.Sprintf("http://%s:%d", pod.Status.PodIP, port.ContainerPort)
						candidates = append(candidates, candidate{
							url:       url,
							namespace: pod.Namespace,
							name:      pod.Name,
							podIP:     pod.Status.PodIP,
						})
						break // Only add once per pod
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		if searched == 0 && lastErr != nil {
			return "", fmt.Errorf("failed to list pods in any namespace: %w", lastErr)
		}
		return "", fmt.Errorf("no Loki pods found in namespaces: %s", strings.Join(searchNamespaces, ", "))
	}

	// Test each candidate and return the first working one
	for _, c := range candidates {
		lokiClient, err := loki.NewClient(c.url)
		if err != nil {
			continue
		}

		// Try to get labels as a connectivity test
		_, err = lokiClient.Labels("")
		if err != nil {
			continue
		}

		return c.url, nil
	}

	return "", fmt.Errorf("found %d Loki pod(s) but none are reachable", len(candidates))
}

var lokiCmd = &cobra.Command{
	Use:   "loki",
	Short: "Query Loki logs",
	Long:  `Commands for querying logs from Loki.`,
}

var lokiQueryCmd = &cobra.Command{
	Use:   "query <logql>",
	Short: "Query logs with LogQL",
	Long: `Query logs from Loki using LogQL syntax.

By default, queries are scoped to the current Kubernetes namespace.
Use -A/--all-namespaces to query across all namespaces.

Examples:
  dex loki query '{job="my-app"}'              # Current namespace only
  dex loki query '{job="my-app"}' -A           # All namespaces
  dex loki query '{job="my-app"}' --since 1h
  dex loki query '{job="my-app"} |= "error"' --since 30m --limit 50
  dex loki --url=http://loki:3100 query '{app="nginx"}'`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		sinceStr, _ := cmd.Flags().GetString("since")
		limit, _ := cmd.Flags().GetInt("limit")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		namespace, _ := cmd.Flags().GetString("namespace")

		// Get Loki URL from flag, config, or auto-discovery
		lokiURL, err := getLokiURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		// Parse since duration
		since, err := parseLokiDuration(sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid since duration: %v\n", err)
			os.Exit(1)
		}

		query := args[0]

		// Apply namespace filter unless --all-namespaces is set
		if !allNamespaces {
			ns := namespace
			if ns == "" {
				// Get current k8s namespace
				k8sClient, err := k8s.NewClient("")
				if err == nil {
					ns = k8sClient.Namespace()
				}
			}
			if ns != "" {
				query = injectNamespaceIntoQuery(query, ns)
				lokiTimeColor.Printf("Namespace: %s (use -A for all namespaces)\n\n", ns)
			}
		}

		client, err := loki.NewClient(lokiURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Loki client: %v\n", err)
			os.Exit(1)
		}

		results, err := client.Query(query, since, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			lokiTimeColor.Println("No results found.")
			return
		}

		// Print results (oldest first for readability)
		for i := len(results) - 1; i >= 0; i-- {
			r := results[i]
			lokiTimeColor.Printf("%s ", r.Timestamp.Format("2006-01-02 15:04:05.000"))
			lokiLineColor.Println(r.Line)
		}

		fmt.Println()
		lokiTimeColor.Printf("(%d entries)\n", len(results))
	},
}

// injectNamespaceIntoQuery adds namespace selector to a LogQL query
func injectNamespaceIntoQuery(query, namespace string) string {
	// Find the opening brace of the stream selector
	braceIdx := strings.Index(query, "{")
	if braceIdx == -1 {
		// No stream selector, wrap the whole query
		return fmt.Sprintf(`{namespace="%s"} %s`, namespace, query)
	}

	// Find the closing brace
	closeBraceIdx := strings.Index(query, "}")
	if closeBraceIdx == -1 {
		return query // Invalid query, return as-is
	}

	// Check if namespace is already in the selector
	selector := query[braceIdx : closeBraceIdx+1]
	if strings.Contains(selector, "namespace=") || strings.Contains(selector, "namespace!=") ||
		strings.Contains(selector, "namespace~=") || strings.Contains(selector, "namespace!~") {
		return query // Already has namespace filter
	}

	// Inject namespace into selector
	if closeBraceIdx == braceIdx+1 {
		// Empty selector: {}
		return query[:braceIdx+1] + fmt.Sprintf(`namespace="%s"`, namespace) + query[closeBraceIdx:]
	}
	// Non-empty selector: {job="app"} -> {job="app",namespace="ns"}
	return query[:closeBraceIdx] + fmt.Sprintf(`,namespace="%s"`, namespace) + query[closeBraceIdx:]
}

var lokiLabelsCmd = &cobra.Command{
	Use:   "labels [label]",
	Short: "List labels or label values",
	Long: `List all label names, or values for a specific label.

By default, results are scoped to the current Kubernetes namespace.
Use -A/--all-namespaces to query across all namespaces.

Examples:
  dex loki labels              # List labels in current namespace
  dex loki labels -A           # List all labels across namespaces
  dex loki labels job          # List values for 'job' label
  dex loki labels namespace    # List values for 'namespace' label`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		namespace, _ := cmd.Flags().GetString("namespace")

		// Get Loki URL from flag, config, or auto-discovery
		lokiURL, err := getLokiURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		// Build namespace filter query
		var query string
		if !allNamespaces {
			ns := namespace
			if ns == "" {
				// Get current k8s namespace
				k8sClient, err := k8s.NewClient("")
				if err == nil {
					ns = k8sClient.Namespace()
				}
			}
			if ns != "" {
				query = fmt.Sprintf(`{namespace="%s"}`, ns)
				lokiTimeColor.Printf("Namespace: %s (use -A for all namespaces)\n\n", ns)
			}
		}

		client, err := loki.NewClient(lokiURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Loki client: %v\n", err)
			os.Exit(1)
		}

		if len(args) == 0 {
			// List all labels
			labels, err := client.Labels(query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get labels: %v\n", err)
				os.Exit(1)
			}

			if len(labels) == 0 {
				lokiTimeColor.Println("No labels found.")
				return
			}

			line := strings.Repeat("─", 50)
			fmt.Println()
			lokiHeaderColor.Printf("  Labels (%d)\n", len(labels))
			fmt.Println("  " + line)
			fmt.Println()

			for _, label := range labels {
				lokiLabelColor.Printf("  %s\n", label)
			}
			fmt.Println()
		} else {
			// List values for specific label
			label := args[0]
			values, err := client.LabelValues(label, query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get label values: %v\n", err)
				os.Exit(1)
			}

			if len(values) == 0 {
				lokiTimeColor.Printf("No values found for label '%s'.\n", label)
				return
			}

			line := strings.Repeat("─", 50)
			fmt.Println()
			lokiHeaderColor.Printf("  Values for '%s' (%d)\n", label, len(values))
			fmt.Println("  " + line)
			fmt.Println()

			for _, value := range values {
				lokiLabelColor.Printf("  %s\n", value)
			}
			fmt.Println()
		}
	},
}

var lokiTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Loki connection",
	Long: `Verify the connection to Loki is working.

Examples:
  dex loki test
  dex loki --url=http://loki:3100 test`,
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")

		// Get Loki URL from flag, config, or auto-discovery
		lokiURL, err := getLokiURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client, err := loki.NewClient(lokiURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Loki client: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Testing connection to %s...\n", lokiURL)

		if err := client.TestConnection(); err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}

		color.New(color.FgGreen).Println("Connection successful!")
	},
}

var lokiDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover Loki in Kubernetes cluster",
	Long: `Auto-discover Loki running in the current Kubernetes cluster.

Searches for Loki pods in common namespaces (monitoring, loki, observability, logging),
gets their Pod IP, and probes connectivity. Returns the discovered URL if successful.

Uses Pod IPs directly, which works with VPN access to the cluster network.

Examples:
  dex loki discover                   # Discover and test Loki
  dex loki discover -n my-namespace   # Search in specific namespace`,
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")

		// Verify k8s connectivity first
		if _, err := k8s.NewClient(""); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to Kubernetes: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Println("Searching for Loki in cluster...")

		// Common namespaces to search
		searchNamespaces := []string{"monitoring", "loki", "observability", "logging", "loki-stack"}
		if namespace != "" {
			searchNamespaces = []string{namespace}
		}

		type candidate struct {
			url       string
			namespace string
			name      string
			podIP     string
		}
		var candidates []candidate
		var lastErr error
		searched := 0

		// Search for Loki pods by name pattern in each namespace
		for _, ns := range searchNamespaces {
			nsClient, err := k8s.NewClient(ns)
			if err != nil {
				lastErr = err
				continue
			}

			pods, err := nsClient.ListPods(ctx, false)
			if err != nil {
				lastErr = err
				continue
			}
			searched++

			for _, pod := range pods {
				nameLower := strings.ToLower(pod.Name)
				// Match loki pods but exclude promtail/agents
				if !strings.Contains(nameLower, "loki") || strings.Contains(nameLower, "promtail") {
					continue
				}

				// Skip pods that aren't running
				if pod.Status.Phase != "Running" {
					continue
				}

				// Skip pods without an IP
				if pod.Status.PodIP == "" {
					continue
				}

				// Find HTTP port (usually 3100)
				for _, container := range pod.Spec.Containers {
					for _, port := range container.Ports {
						if port.ContainerPort == 3100 || port.Name == "http-metrics" || port.Name == "http" {
							url := fmt.Sprintf("http://%s:%d", pod.Status.PodIP, port.ContainerPort)
							candidates = append(candidates, candidate{
								url:       url,
								namespace: pod.Namespace,
								name:      pod.Name,
								podIP:     pod.Status.PodIP,
							})
							break // Only add once per pod
						}
					}
				}
			}
		}

		if len(candidates) == 0 {
			if searched == 0 && lastErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect to Kubernetes API: %v\n", lastErr)
				fmt.Fprintf(os.Stderr, "Tip: Check VPN connection and cluster access\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "No Loki pods found in cluster.\n")
			fmt.Fprintf(os.Stderr, "Searched namespaces: %s\n", strings.Join(searchNamespaces, ", "))
			os.Exit(1)
		}

		// Test each candidate
		fmt.Printf("Found %d candidate(s), testing connectivity...\n\n", len(candidates))

		var working []candidate
		for _, c := range candidates {
			lokiTimeColor.Printf("  %s/%s (%s) ", c.namespace, c.name, c.podIP)

			lokiClient, err := loki.NewClient(c.url)
			if err != nil {
				lokiErrorColor.Printf("✗ failed to create client\n")
				continue
			}

			// Try to get labels as a connectivity test
			_, err = lokiClient.Labels("")
			if err != nil {
				lokiErrorColor.Printf("✗ %v\n", err)
				continue
			}

			lokiSuccessColor.Printf("✓ connected\n")
			working = append(working, c)
		}

		fmt.Println()

		if len(working) == 0 {
			fmt.Fprintf(os.Stderr, "No reachable Loki instances found.\n")
			fmt.Fprintf(os.Stderr, "Tip: Make sure you have VPN access to the cluster network\n")
			os.Exit(1)
		}

		// Report the first working URL
		lokiHeaderColor.Println("Loki URL:")
		fmt.Printf("  %s\n\n", working[0].url)

		lokiTimeColor.Printf("To use: export LOKI_URL=%s\n", working[0].url)
	},
}

// parseLokiDuration parses human-readable durations like "1h", "30m", "1d", "2h30m"
func parseLokiDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Hour, nil // Default to 1 hour
	}

	// Handle days (not supported by time.ParseDuration)
	if strings.Contains(s, "d") {
		parts := strings.SplitN(s, "d", 2)
		days := 0
		if parts[0] != "" {
			d, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, fmt.Errorf("invalid duration: %s", s)
			}
			days = d
		}
		duration := time.Duration(days) * 24 * time.Hour

		// Parse remaining part (e.g., "2h30m" from "1d2h30m")
		if len(parts) > 1 && parts[1] != "" {
			rest, err := time.ParseDuration(parts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid duration: %s", s)
			}
			duration += rest
		}
		return duration, nil
	}

	// Try standard duration parsing (handles h, m, s)
	return time.ParseDuration(s)
}

func init() {
	rootCmd.AddCommand(lokiCmd)

	// Add persistent flag to loki command (available to all subcommands)
	lokiCmd.PersistentFlags().String("url", "", "Loki URL (overrides LOKI_URL config)")

	// Add subcommands
	lokiCmd.AddCommand(lokiQueryCmd)
	lokiCmd.AddCommand(lokiLabelsCmd)
	lokiCmd.AddCommand(lokiTestCmd)
	lokiCmd.AddCommand(lokiDiscoverCmd)

	// Query command flags
	lokiQueryCmd.Flags().StringP("since", "s", "1h", "Time range to query (e.g., 1h, 30m, 1d)")
	lokiQueryCmd.Flags().IntP("limit", "l", 1000, "Maximum number of entries to return")
	lokiQueryCmd.Flags().BoolP("all-namespaces", "A", false, "Query all namespaces (default: current k8s namespace)")
	lokiQueryCmd.Flags().StringP("namespace", "n", "", "Namespace to query (default: current k8s namespace)")

	// Labels command flags
	lokiLabelsCmd.Flags().BoolP("all-namespaces", "A", false, "List labels/values across all namespaces (default: current k8s namespace)")
	lokiLabelsCmd.Flags().StringP("namespace", "n", "", "Namespace to filter (default: current k8s namespace)")

	// Discover command flags
	lokiDiscoverCmd.Flags().StringP("namespace", "n", "", "Namespace to search (default: monitoring, loki, observability, logging)")
}
