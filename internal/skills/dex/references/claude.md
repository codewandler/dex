# Claude Code Integration

Commands for integrating dex with Claude Code.

## Status Line

Generate a customizable status line for Claude Code's terminal UI.

```bash
dex claude statusline
```

### Output Example

```
Opus 42% $0.05 | ☸ prod/api | 🦊 3 assigned, 1 reviewing | 📋 5 open | 💬 @2
```

### How It Works

1. Claude Code calls `dex claude statusline` every ~300ms
2. The command reads session data from stdin (JSON with model, context usage, cost)
3. Fetches data from configured integrations (cached to disk per session)
4. Outputs a single formatted line combining Claude metrics and integration data

### Configuration

Configure in `~/.dex/config.json` under `status_line`:

```json
{
  "status_line": {
    "format": "{{if .Claude}}{{.Claude}} | {{end}}{{if .K8s}}☸ {{.K8s}}{{end}}...",
    "segments": {
      "claude": {
        "enabled": true,
        "format": "{{.Model}} {{.ContextUsed}}%{{if .Cost}} ${{printf \"%.2f\" .Cost}}{{end}}"
      },
      "k8s": {
        "enabled": true,
        "format": "{{.Context}}/{{.Namespace}}{{if .Issues}} ({{.Issues}}){{end}}",
        "cache_ttl": "30s"
      },
      "gitlab": {
        "enabled": true,
        "format": "{{if .Assigned}}{{.Assigned}} assigned{{end}}{{if and .Assigned .Reviewing}}, {{end}}{{if .Reviewing}}{{.Reviewing}} reviewing{{end}}",
        "cache_ttl": "2m"
      },
      "github": {
        "enabled": true,
        "format": "{{if .PRs}}{{.PRs}} PRs{{end}}{{if and .PRs .Issues}}, {{end}}{{if .Issues}}{{.Issues}} issues{{end}}",
        "cache_ttl": "2m"
      },
      "jira": {
        "enabled": true,
        "format": "{{.Open}} open",
        "cache_ttl": "2m"
      },
      "slack": {
        "enabled": true,
        "format": "@{{.Mentions}}",
        "cache_ttl": "1m"
      }
    }
  }
}
```

### Segment Data

Each segment provides different template variables:

| Segment | Variables | Description |
|---------|-----------|-------------|
| **claude** | `Model`, `ContextUsed`, `ContextRemaining`, `Cost`, `LinesAdded`, `LinesRemoved` | Claude session metrics from stdin |
| **k8s** | `Context`, `Namespace`, `Issues` | Current K8s context, namespace, and pod issues |
| **gitlab** | `Assigned`, `Reviewing` | MRs assigned to you or where you're a reviewer |
| **github** | `PRs`, `Issues` | Open PRs and issues assigned to you |
| **jira** | `Open` | Open issues assigned to you |
| **slack** | `Mentions` | Pending mentions in the last 24 hours |

### Claude Code Setup

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "dex claude statusline",
    "padding": 0
  }
}
```

### Caching

Data is cached to disk at `~/.dex/claude/statusline/{session_id}.json`:

| Segment | Default TTL |
|---------|-------------|
| **claude** | No caching (data comes from stdin) |
| **k8s** | 30 seconds |
| **gitlab** | 2 minutes |
| **github** | 2 minutes |
| **jira** | 2 minutes |
| **slack** | 1 minute |

Cache is per-session, so different Claude windows have independent caches.

### Disabling Segments

To disable a segment, set `enabled: false`:

```json
{
  "status_line": {
    "segments": {
      "slack": { "enabled": false },
      "github": { "enabled": false }
    }
  }
}
```

Segments are also automatically disabled if their integration is not configured.
