# Prometheus Commands (`dex prom`)

Query metrics from Prometheus using PromQL.

## Discover Prometheus in Kubernetes
```bash
dex prom discover                 # Auto-discover Prometheus in current cluster
dex prom discover -n monitoring   # Search in specific namespace
```

Searches common namespaces (monitoring, prometheus, observability, kube-system) for Prometheus server pods, gets their Pod IP, tests connectivity, and returns the URL.

Excludes non-server pods (alertmanager, node-exporter, pushgateway, kube-state-metrics, grafana).

## Configuration

Set `PROMETHEUS_URL` environment variable or add to `~/.dex/config.json`:
```json
{
  "prometheus": {
    "url": "http://prometheus:9090"
  }
}
```

Alternatively, use the `--url` flag on any command.

**Auto-discovery:** If no URL is configured, commands will automatically discover Prometheus in the current Kubernetes cluster.

## Instant Query
```bash
dex prom query 'up'                                  # Current values
dex prom query 'rate(http_requests_total[5m])'        # Rate over 5 minutes
dex prom query 'up{job="node-exporter"}'              # Filter by label
dex prom query 'up' --time "2026-02-04 15:00"         # Query at specific time
dex prom query 'up' -o json                           # JSON output
```

## Range Query
```bash
dex prom query-range 'rate(http_requests_total[5m])' --since 1h
dex prom query-range 'up' --since 30m --step 15s
dex prom query-range 'up' --since 2d --until 1d
dex prom query-range 'up' --since "2026-02-04 15:00" --until "2026-02-04 16:00"
dex prom query-range 'up' --since "2026-02-04T15:00:00Z"   # UTC timestamp via suffix
dex prom query-range 'up' --since "2026-02-04 15:00" --utc  # Interpret as UTC
dex prom query-range 'up' -o json                     # JSON output
```

When `--step` is omitted, it auto-calculates to produce ~250 data points (like Grafana).

## Labels
```bash
dex prom labels                             # List all label names
dex prom labels job                         # List values for 'job' label
dex prom labels -m 'up{job="node"}'         # Scoped to matching series
dex prom labels job -m 'up'                 # Values for 'job' scoped to 'up' series
```

Tab completion is available for label names.

## Targets
```bash
dex prom targets                    # Active targets (default)
dex prom targets --state dropped    # Dropped targets
dex prom targets --state any        # All targets
dex prom targets -o json            # JSON output
```

## Alerts
```bash
dex prom alerts                     # List active alerts
dex prom alerts -o json             # JSON output
```

## Test Connection
```bash
dex prom test                                    # Verify Prometheus connection
dex prom --url=http://localhost:9090 test         # Test specific URL
```

## PromQL Quick Reference

**Selectors:**
- `up` - metric name
- `up{job="node"}` - exact match
- `up{job=~"node.*"}` - regex match
- `up{job!="node"}` - not equal
- `up{job!~"node.*"}` - regex not match

**Functions:**
- `rate(metric[5m])` - per-second rate over window
- `increase(metric[1h])` - total increase over window
- `sum(metric) by (job)` - aggregate by label
- `avg(metric) by (instance)` - average by label
- `histogram_quantile(0.99, rate(metric[5m]))` - percentile from histogram

**Operators:**
- `metric > 0` - filter by value
- `metric_a / metric_b` - arithmetic
- `metric_a and metric_b` - set intersection
- `metric_a or metric_b` - set union

## Tips

- Default time range for `query-range` is 1 hour (`--since 1h`), default end is now
- `--since` and `--until` accept durations (`30m`, `1h`, `2d`) or timestamps (`2006-01-02 15:04`)
- Naive timestamps (no tz suffix) are interpreted in local timezone; use `--utc` to interpret as UTC
- Use `-o json` for machine-readable output
- The `--match`/`-m` flag on `labels` is repeatable for multiple series selectors
