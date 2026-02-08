package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/k8s"
	"github.com/codewandler/dex/internal/portforward"
	"github.com/codewandler/dex/internal/prometheus"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	promHeaderColor  = color.New(color.FgCyan, color.Bold)
	promLabelColor   = color.New(color.FgYellow)
	promValueColor   = color.New(color.FgWhite, color.Bold)
	promDimColor     = color.New(color.FgHiBlack)
	promSuccessColor = color.New(color.FgGreen)
	promErrorColor   = color.New(color.FgRed)
	promWarnColor    = color.New(color.FgYellow, color.Bold)
)

// getPrometheusURL returns the Prometheus URL from flag, config, or auto-discovery
func getPrometheusURL(urlFlag string) (string, error) {
	// 1. Check flag
	if urlFlag != "" {
		return urlFlag, nil
	}

	// 2. Check config
	cfg, err := config.Load()
	if err == nil && cfg.Prometheus.URL != "" {
		return cfg.Prometheus.URL, nil
	}

	// 3. Auto-discover
	promDimColor.Println("No Prometheus URL configured, attempting auto-discovery...")
	url, err := discoverPrometheusURL("")
	if err != nil {
		return "", fmt.Errorf("auto-discovery failed: %w\nTip: Use --url flag or set PROMETHEUS_URL environment variable", err)
	}
	promDimColor.Printf("Auto-discovered Prometheus at %s\n\n", url)
	return url, nil
}

// discoverPrometheusURL finds a working Prometheus URL in the current Kubernetes cluster
func discoverPrometheusURL(namespace string) (string, error) {
	if _, err := k8s.NewClient(""); err != nil {
		return "", fmt.Errorf("failed to connect to Kubernetes: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	searchNamespaces := []string{"monitoring", "prometheus", "observability", "kube-system", "prometheus-stack"}
	if namespace != "" {
		searchNamespaces = []string{namespace}
	}

	// Pod name exclusions
	excludes := []string{"alertmanager", "node-exporter", "pushgateway", "kube-state", "grafana"}

	type candidate struct {
		url       string
		namespace string
		name      string
		podIP     string
	}
	var candidates []candidate
	var lastErr error
	searched := 0

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
			if !strings.Contains(nameLower, "prometheus") {
				continue
			}

			// Exclude non-server pods
			skip := false
			for _, ex := range excludes {
				if strings.Contains(nameLower, ex) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			if pod.Status.Phase != "Running" || pod.Status.PodIP == "" {
				continue
			}

			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if port.ContainerPort == 9090 || port.Name == "http-web" || port.Name == "http" || port.Name == "web" {
						url := fmt.Sprintf("http://%s:%d", pod.Status.PodIP, port.ContainerPort)
						candidates = append(candidates, candidate{
							url:       url,
							namespace: pod.Namespace,
							name:      pod.Name,
							podIP:     pod.Status.PodIP,
						})
						break
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		if searched == 0 && lastErr != nil {
			return "", fmt.Errorf("failed to list pods in any namespace: %w", lastErr)
		}
		return "", fmt.Errorf("no Prometheus pods found in namespaces: %s", strings.Join(searchNamespaces, ", "))
	}

	// Check existing port-forwards first
	for _, c := range candidates {
		if info, exists := portforward.FindByNamespaceAndPod(c.namespace, c.name); exists {
			localURL := fmt.Sprintf("http://localhost:%d", info.LocalPort)
			probeClient := prometheus.NewProbeClient(localURL)
			if probeClient.TestConnection() == nil {
				return localURL, nil
			}
		}
	}

	// Try Pod IPs
	for _, c := range candidates {
		probeClient := prometheus.NewProbeClient(c.url)
		if probeClient.TestConnection() == nil {
			return c.url, nil
		}
	}

	c := candidates[0]
	return "", fmt.Errorf("found %d Prometheus pod(s) but none are reachable via Pod IP\n\nTip: Use port-forwarding instead:\n  dex k8s forward start %s -n %s\n  Then set PROMETHEUS_URL to the local endpoint shown in the output",
		len(candidates), c.name, c.namespace)
}

// formatMetricLabels formats a label map as {key="val", ...}, excluding __name__
func formatMetricLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		if k == "__name__" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// formatSampleValue formats a Prometheus sample value for display
func formatSampleValue(v interface{}) string {
	s := fmt.Sprintf("%v", v)
	switch s {
	case "+Inf":
		return "+Inf"
	case "-Inf":
		return "-Inf"
	case "NaN":
		return "NaN"
	default:
		return s
	}
}

// autoStep computes a step duration that produces ~250 data points
func autoStep(start, end time.Time) time.Duration {
	span := end.Sub(start)
	step := span / 250
	if step < time.Second {
		step = time.Second
	}
	return step
}

var promCmd = &cobra.Command{
	Use:     "prom",
	Aliases: []string{"prometheus"},
	Short:   "Query Prometheus metrics",
	Long:    `Commands for querying metrics from Prometheus.`,
}

// ── prom query ──────────────────────────────────────────────────────────────

var promQueryCmd = &cobra.Command{
	Use:   "query <promql>",
	Short: "Instant PromQL query",
	Long: `Execute an instant PromQL query and display results.

Examples:
  dex prom query 'up'
  dex prom query 'rate(http_requests_total[5m])'
  dex prom query 'up' --time "2026-02-04 15:00"
  dex prom query 'up' -o json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		timeStr, _ := cmd.Flags().GetString("time")
		output, _ := cmd.Flags().GetString("output")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		var evalTime time.Time
		if timeStr != "" {
			evalTime, err = parseTimeValueInLocation(timeStr, time.Local)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --time value: %v\n", err)
				os.Exit(1)
			}
		}

		client := prometheus.NewClient(promURL)
		samples, err := client.Query(args[0], evalTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}

		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(samples)
			return
		}

		if len(samples) == 0 {
			promDimColor.Println("No results.")
			return
		}

		for _, s := range samples {
			name := s.Metric["__name__"]
			if name == "" {
				name = "{}"
			}
			promHeaderColor.Print(name)
			labels := formatMetricLabels(s.Metric)
			if labels != "{}" {
				promLabelColor.Print(labels)
			}
			fmt.Println()

			if len(s.Value) == 2 {
				promValueColor.Printf("  %s\n", formatSampleValue(s.Value[1]))
			}
		}

		fmt.Println()
		promDimColor.Printf("(%d series)\n", len(samples))
	},
}

// ── prom query-range ────────────────────────────────────────────────────────

var promQueryRangeCmd = &cobra.Command{
	Use:   "query-range <promql>",
	Short: "Range PromQL query",
	Long: `Execute a range PromQL query and display results as a matrix.

Examples:
  dex prom query-range 'rate(http_requests_total[5m])' --since 1h
  dex prom query-range 'up' --since 30m --step 15s
  dex prom query-range 'up' --since "2026-02-04 15:00" --until "2026-02-04 16:00"
  dex prom query-range 'up' -o json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		stepStr, _ := cmd.Flags().GetString("step")
		utcFlag, _ := cmd.Flags().GetBool("utc")
		output, _ := cmd.Flags().GetString("output")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		loc := time.Local
		if utcFlag {
			loc = time.UTC
		}

		start, err := parseTimeValueInLocation(sinceStr, loc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid --since value: %v\n", err)
			os.Exit(1)
		}

		end, err := parseTimeValueInLocation(untilStr, loc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid --until value: %v\n", err)
			os.Exit(1)
		}

		if !start.Before(end) {
			fmt.Fprintf(os.Stderr, "Invalid time range: --since (%s) must be before --until (%s)\n",
				start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
			os.Exit(1)
		}

		var step time.Duration
		if stepStr != "" {
			step, err = parseLokiDuration(stepStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --step value: %v\n", err)
				os.Exit(1)
			}
		} else {
			step = autoStep(start, end)
		}

		client := prometheus.NewClient(promURL)
		series, err := client.QueryRange(args[0], start, end, step)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}

		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(series)
			return
		}

		if len(series) == 0 {
			promDimColor.Println("No results.")
			return
		}

		for i, s := range series {
			name := s.Metric["__name__"]
			if name == "" {
				name = "{}"
			}
			promHeaderColor.Print(name)
			labels := formatMetricLabels(s.Metric)
			if labels != "{}" {
				promLabelColor.Print(labels)
			}
			fmt.Printf(" (%d samples)\n", len(s.Values))

			for _, v := range s.Values {
				if len(v) < 2 {
					continue
				}
				// Parse timestamp
				var ts time.Time
				switch t := v[0].(type) {
				case float64:
					sec, frac := math.Modf(t)
					ts = time.Unix(int64(sec), int64(frac*1e9))
				}
				if utcFlag {
					ts = ts.UTC()
				}
				promDimColor.Printf("  %s  ", ts.Format("15:04:05"))
				promValueColor.Printf("%s\n", formatSampleValue(v[1]))
			}

			if i < len(series)-1 {
				fmt.Println()
			}
		}

		fmt.Println()
		promDimColor.Printf("(%d series)\n", len(series))
	},
}

// ── prom labels ─────────────────────────────────────────────────────────────

var promLabelsCmd = &cobra.Command{
	Use:   "labels [label]",
	Short: "List labels or label values",
	Long: `List all label names, or values for a specific label.

Examples:
  dex prom labels                       # List all label names
  dex prom labels job                   # List values for 'job'
  dex prom labels -m 'up{job="node"}'   # Scoped to matching series`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		urlFlag, _ := cmd.Flags().GetString("url")
		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		client := prometheus.NewClient(promURL)
		labels, err := client.Labels(nil)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var completions []string
		lower := strings.ToLower(toComplete)
		for _, l := range labels {
			if strings.Contains(strings.ToLower(l), lower) {
				completions = append(completions, l)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		match, _ := cmd.Flags().GetStringSlice("match")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client := prometheus.NewClient(promURL)

		if len(args) == 0 {
			// List all labels
			labels, err := client.Labels(match)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get labels: %v\n", err)
				os.Exit(1)
			}

			if len(labels) == 0 {
				promDimColor.Println("No labels found.")
				return
			}

			line := strings.Repeat("─", 50)
			fmt.Println()
			promHeaderColor.Printf("  Labels (%d)\n", len(labels))
			fmt.Println("  " + line)
			fmt.Println()

			for _, label := range labels {
				promLabelColor.Printf("  %s\n", label)
			}
			fmt.Println()
		} else {
			// List values for specific label
			label := args[0]
			values, err := client.LabelValues(label, match)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get label values: %v\n", err)
				os.Exit(1)
			}

			if len(values) == 0 {
				promDimColor.Printf("No values found for label '%s'.\n", label)
				return
			}

			line := strings.Repeat("─", 50)
			fmt.Println()
			promHeaderColor.Printf("  Values for '%s' (%d)\n", label, len(values))
			fmt.Println("  " + line)
			fmt.Println()

			for _, value := range values {
				promLabelColor.Printf("  %s\n", value)
			}
			fmt.Println()
		}
	},
}

// ── prom targets ────────────────────────────────────────────────────────────

var promTargetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "List scrape targets",
	Long: `List Prometheus scrape targets and their health status.

Examples:
  dex prom targets                  # Active targets (default)
  dex prom targets --state dropped  # Dropped targets
  dex prom targets --state any      # All targets`,
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		state, _ := cmd.Flags().GetString("state")
		output, _ := cmd.Flags().GetString("output")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client := prometheus.NewClient(promURL)
		targets, err := client.Targets(state)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get targets: %v\n", err)
			os.Exit(1)
		}

		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(targets)
			return
		}

		if len(targets) == 0 {
			promDimColor.Println("No targets found.")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		promHeaderColor.Printf("  Scrape Targets (%d)\n", len(targets))
		fmt.Println("  " + line)
		fmt.Println()

		for _, t := range targets {
			// Health indicator
			switch t.Health {
			case "up":
				promSuccessColor.Print("  ● ")
			case "down":
				promErrorColor.Print("  ● ")
			default:
				promDimColor.Print("  ○ ")
			}

			// Job and instance
			job := t.Labels["job"]
			instance := t.Labels["instance"]
			promHeaderColor.Printf("%s", job)
			if instance != "" {
				promDimColor.Printf(" (%s)", instance)
			}
			fmt.Println()

			// Scrape pool and URL
			promDimColor.Printf("    pool: %s\n", t.ScrapePool)
			promDimColor.Printf("    url:  %s\n", t.ScrapeURL)

			// Last scrape timing
			if !t.LastScrape.IsZero() {
				ago := time.Since(t.LastScrape).Truncate(time.Second)
				promDimColor.Printf("    last: %s ago (%.3fs)\n", ago, t.LastScrapeDuration)
			}

			// Error
			if t.LastError != "" {
				promErrorColor.Printf("    error: %s\n", t.LastError)
			}

			fmt.Println()
		}
	},
}

// ── prom alerts ─────────────────────────────────────────────────────────────

var promAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "List active alerts",
	Long: `List active Prometheus alerts.

Examples:
  dex prom alerts
  dex prom alerts -o json`,
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")
		output, _ := cmd.Flags().GetString("output")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client := prometheus.NewClient(promURL)
		alerts, err := client.Alerts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get alerts: %v\n", err)
			os.Exit(1)
		}

		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(alerts)
			return
		}

		if len(alerts) == 0 {
			promSuccessColor.Println("No active alerts.")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		promHeaderColor.Printf("  Alerts (%d)\n", len(alerts))
		fmt.Println("  " + line)
		fmt.Println()

		for _, a := range alerts {
			// State indicator
			switch a.State {
			case "firing":
				promErrorColor.Print("  🔥 ")
			case "pending":
				promWarnColor.Print("  ⏳ ")
			default:
				promDimColor.Printf("  [%s] ", a.State)
			}

			alertName := a.Labels["alertname"]
			if alertName == "" {
				alertName = "(unnamed)"
			}
			promHeaderColor.Println(alertName)

			// Labels (excluding alertname)
			labels := formatMetricLabels(a.Labels)
			if labels != "{}" {
				promLabelColor.Printf("    labels: %s\n", labels)
			}

			// Annotations
			if summary, ok := a.Annotations["summary"]; ok {
				promDimColor.Printf("    summary: %s\n", summary)
			}
			if desc, ok := a.Annotations["description"]; ok {
				promDimColor.Printf("    description: %s\n", desc)
			}

			// Active since
			if !a.ActiveAt.IsZero() {
				ago := time.Since(a.ActiveAt).Truncate(time.Second)
				promDimColor.Printf("    active: %s ago\n", ago)
			}

			// Value
			if a.Value != "" {
				promValueColor.Printf("    value: %s\n", a.Value)
			}

			fmt.Println()
		}
	},
}

// ── prom test ───────────────────────────────────────────────────────────────

var promTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Prometheus connection",
	Long: `Verify the connection to Prometheus is working.

Examples:
  dex prom test
  dex prom --url=http://localhost:9090 test`,
	Run: func(cmd *cobra.Command, args []string) {
		urlFlag, _ := cmd.Flags().GetString("url")

		promURL, err := getPrometheusURL(urlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client := prometheus.NewClient(promURL)

		fmt.Printf("Testing connection to %s...\n", promURL)

		if err := client.TestConnection(); err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}

		promSuccessColor.Println("Connection successful!")
	},
}

// ── prom discover ───────────────────────────────────────────────────────────

var promDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover Prometheus in Kubernetes cluster",
	Long: `Auto-discover Prometheus running in the current Kubernetes cluster.

Searches for Prometheus server pods in common namespaces (monitoring, prometheus,
observability, kube-system), gets their Pod IP, and probes connectivity.

Examples:
  dex prom discover
  dex prom discover -n monitoring`,
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")

		if _, err := k8s.NewClient(""); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to Kubernetes: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Println("Searching for Prometheus in cluster...")

		searchNamespaces := []string{"monitoring", "prometheus", "observability", "kube-system", "prometheus-stack"}
		if namespace != "" {
			searchNamespaces = []string{namespace}
		}

		excludes := []string{"alertmanager", "node-exporter", "pushgateway", "kube-state", "grafana"}

		type candidate struct {
			url       string
			namespace string
			name      string
			podIP     string
		}
		var candidates []candidate
		var lastErr error
		searched := 0

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
				if !strings.Contains(nameLower, "prometheus") {
					continue
				}

				skip := false
				for _, ex := range excludes {
					if strings.Contains(nameLower, ex) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				if pod.Status.Phase != "Running" || pod.Status.PodIP == "" {
					continue
				}

				for _, container := range pod.Spec.Containers {
					for _, port := range container.Ports {
						if port.ContainerPort == 9090 || port.Name == "http-web" || port.Name == "http" || port.Name == "web" {
							url := fmt.Sprintf("http://%s:%d", pod.Status.PodIP, port.ContainerPort)
							candidates = append(candidates, candidate{
								url:       url,
								namespace: pod.Namespace,
								name:      pod.Name,
								podIP:     pod.Status.PodIP,
							})
							break
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
			fmt.Fprintf(os.Stderr, "No Prometheus pods found in cluster.\n")
			fmt.Fprintf(os.Stderr, "Searched namespaces: %s\n", strings.Join(searchNamespaces, ", "))
			os.Exit(1)
		}

		fmt.Printf("Found %d candidate(s), testing connectivity...\n\n", len(candidates))

		var working []candidate
		for _, c := range candidates {
			// Check existing port-forward first
			if info, exists := portforward.FindByNamespaceAndPod(c.namespace, c.name); exists {
				localURL := fmt.Sprintf("http://localhost:%d", info.LocalPort)
				promDimColor.Printf("  %s/%s (port-forward: localhost:%d) ", c.namespace, c.name, info.LocalPort)

				probeClient := prometheus.NewProbeClient(localURL)
				if probeClient.TestConnection() == nil {
					promSuccessColor.Printf("✓ connected\n")
					working = append(working, candidate{
						url:       localURL,
						namespace: c.namespace,
						name:      c.name,
						podIP:     fmt.Sprintf("localhost:%d", info.LocalPort),
					})
					continue
				}
				promErrorColor.Printf("✗ port-forward exists but not reachable\n")
				continue
			}

			// Try Pod IP
			promDimColor.Printf("  %s/%s (%s) ", c.namespace, c.name, c.podIP)

			probeClient := prometheus.NewProbeClient(c.url)
			if probeClient.TestConnection() != nil {
				promErrorColor.Printf("✗ not reachable\n")
				continue
			}

			promSuccessColor.Printf("✓ connected\n")
			working = append(working, c)
		}

		fmt.Println()

		if len(working) == 0 {
			fmt.Fprintf(os.Stderr, "No reachable Prometheus instances found.\n\n")
			c := candidates[0]
			fmt.Fprintf(os.Stderr, "Tip: Use port-forwarding instead:\n")
			fmt.Fprintf(os.Stderr, "  dex k8s forward start %s -n %s\n", c.name, c.namespace)
			fmt.Fprintf(os.Stderr, "  Then set PROMETHEUS_URL to the local endpoint shown in the output\n")
			os.Exit(1)
		}

		promHeaderColor.Println("Prometheus URL:")
		fmt.Printf("  %s\n\n", working[0].url)

		promDimColor.Printf("To use: export PROMETHEUS_URL=%s\n", working[0].url)
	},
}

func init() {
	rootCmd.AddCommand(promCmd)

	// Persistent flag available to all subcommands
	promCmd.PersistentFlags().String("url", "", "Prometheus URL (overrides PROMETHEUS_URL config)")

	// Register subcommands
	promCmd.AddCommand(promQueryCmd)
	promCmd.AddCommand(promQueryRangeCmd)
	promCmd.AddCommand(promLabelsCmd)
	promCmd.AddCommand(promTargetsCmd)
	promCmd.AddCommand(promAlertsCmd)
	promCmd.AddCommand(promTestCmd)
	promCmd.AddCommand(promDiscoverCmd)

	// Query command flags
	promQueryCmd.Flags().String("time", "", "Evaluation time (timestamp, default: now)")
	promQueryCmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	// Query-range command flags
	promQueryRangeCmd.Flags().StringP("since", "s", "1h", "Start of time range (duration or timestamp)")
	promQueryRangeCmd.Flags().StringP("until", "u", "", "End of time range (duration or timestamp, default: now)")
	promQueryRangeCmd.Flags().String("step", "", "Query step (e.g. 15s, 1m; default: auto ~250 points)")
	promQueryRangeCmd.Flags().Bool("utc", false, "Interpret naive timestamps as UTC instead of local timezone")
	promQueryRangeCmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	// Labels command flags
	promLabelsCmd.Flags().StringSliceP("match", "m", nil, "Series selector(s) to scope labels (repeatable)")

	// Targets command flags
	promTargetsCmd.Flags().String("state", "active", "Target state filter: active, dropped, any")
	promTargetsCmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	// Alerts command flags
	promAlertsCmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	// Discover command flags
	promDiscoverCmd.Flags().StringP("namespace", "n", "", "Namespace to search (default: monitoring, prometheus, observability, ...)")
}
