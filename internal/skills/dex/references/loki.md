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

Alternatively, use the `--url` flag on any command, or run `dex loki discover` to find it automatically.

## Query Logs
```bash
dex loki query '{job="my-app"}'                    # Current k8s namespace (default)
dex loki query '{job="my-app"}' -A                 # All namespaces
dex loki query '{job="my-app"}' -n prod            # Specific namespace
dex loki query '{job="my-app"}' --since 30m        # Last 30 minutes
dex loki query '{job="my-app"}' --since 1d         # Last day
dex loki query '{job="my-app"}' --limit 50         # Limit results
dex loki query '{app="nginx"} |= "error"'          # Filter for "error"
dex loki query '{app="nginx"} |~ "5[0-9]{2}"'      # Regex filter (5xx errors)
dex loki query '{job="my-app"} | json'             # Parse JSON logs
```

By default, queries are scoped to your current Kubernetes namespace. Use `-A` for all namespaces.

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

- Default time range is 1 hour (`--since 1h`)
- Default limit is 1000 entries (`--limit 1000`)
- Results are displayed oldest-first for readability
- Duration supports: `30m`, `1h`, `2h30m`, `1d`, `7d`
