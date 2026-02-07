# Loki Commands (`dex loki`)

Query logs from Grafana Loki using LogQL.

## Discover Loki in Kubernetes
```bash
dex loki discover                 # Auto-discover Loki in current cluster
dex loki discover -n monitoring   # Search in specific namespace
```

Searches common namespaces (monitoring, loki, observability, logging) for Loki pods,
gets their Pod IP, tests connectivity, and returns the URL.

Uses Pod IPs directly - works with VPN access to cluster network (no need for port-forward).

## Configuration

Set `LOKI_URL` environment variable or add to `~/.dex/config.json`:
```json
{
  "loki": {
    "url": "http://loki:3100"
  }
}
```

Alternatively, use the `--url` flag on any command.

**Auto-discovery:** If no URL is configured, commands will automatically discover Loki in the current Kubernetes cluster.

## Query Logs
```bash
dex loki query '{job="my-app"}'                    # Current k8s namespace (default)
dex loki query '{job="my-app"}' -A                 # All namespaces
dex loki query '{job="my-app"}' -n prod            # Specific namespace
dex loki query '{job="my-app"}' --since 30m        # Last 30 minutes
dex loki query '{job="my-app"}' --since 1d         # Last day
dex loki query '{job="my-app"}' --since 2d --until 1d   # From 2 days ago to 1 day ago
dex loki query '{job="my-app"}' --since "2026-02-04 15:00" --until "2026-02-04 16:00"
dex loki query '{job="my-app"}' --since "2026-02-04T15:00:00Z"  # UTC timestamp via suffix
dex loki query '{job="my-app"}' --since "2026-02-04 15:00" --utc # Interpret as UTC
dex loki query '{job="my-app"}' --limit 50         # Limit results
dex loki query '{app="nginx"} |= "error"'          # Filter for "error"
dex loki query '{app="nginx"} |~ "5[0-9]{2}"'      # Regex filter (5xx errors)
dex loki query '{job="my-app"} | json'             # Parse JSON logs
dex loki query '{job="my-app"}' --labels pod,container  # Show specific labels
dex loki query '{job="my-app"}' --labels ""             # Show all labels
```

By default, queries are scoped to your current Kubernetes namespace. Use `-A` for all namespaces.

Output shows stream labels before each log line. By default only `app` and `pod` are shown. Use `--labels` to customize (comma-separated), or `--labels ""` to show all.

## Labels
```bash
dex loki labels                   # List labels (current k8s namespace)
dex loki labels -A                # List labels (all namespaces)
dex loki labels -n prod           # List labels in specific namespace
dex loki labels job               # List values for 'job' label
dex loki labels job -A            # List all values for 'job' label
dex loki labels namespace         # List values for 'namespace' label
```

By default, labels are scoped to your current Kubernetes namespace. Use `-A` for all namespaces.

## Test Connection
```bash
dex loki test                     # Verify Loki connection
dex loki --url=http://loki:3100 test  # Test specific URL
```

## LogQL Quick Reference

**Stream selectors:**
- `{job="app"}` - exact match
- `{job=~"app.*"}` - regex match
- `{job!="app"}` - not equal
- `{job!~"app.*"}` - regex not match

**Line filters:**
- `|= "error"` - contains string
- `!= "debug"` - does not contain
- `|~ "error|warn"` - regex match
- `!~ "debug|trace"` - regex not match

**Parsers:**
- `| json` - parse JSON
- `| logfmt` - parse logfmt
- `| pattern "<pattern>"` - custom pattern

## Tips

- Default time range is 1 hour (`--since 1h`), default end is now
- `--since` and `--until` accept durations (`30m`, `1h`, `2d`) or timestamps (`2006-01-02 15:04`, `2006-01-02T15:04:05Z`)
- Naive timestamps (no tz suffix) are interpreted in local timezone; use `--utc` to interpret as UTC
- Timestamps with explicit timezone suffix (`Z`, `+02:00`) always use the embedded timezone
- Default limit is 1000 entries (`--limit 1000`)
- Results are displayed oldest-first for readability
