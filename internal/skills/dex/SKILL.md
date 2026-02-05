---
name: dex
description: Run dex CLI commands for Kubernetes, GitLab, Jira, Slack, and Loki operations
user-invocable: true
---

# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, Jira, Slack, and Loki operations. Run commands via Bash tool.

## Setup & Diagnostics

```bash
dex setup                         # Interactive setup wizard (only prompts for unconfigured integrations)
dex doctor                        # Check health of all configured integrations
dex upgrade                       # Upgrade to latest version
dex upgrade -v v0.2.0             # Upgrade to specific version
dex version                       # Print version information
dex completion bash|zsh|fish      # Generate shell completions
dex skill install                 # Install skill to ~/.claude/skills/dex/
dex skill show                    # Print skill content to stdout
```

## Integrations

| Integration | Command | Reference |
|-------------|---------|-----------|
| Kubernetes | `dex k8s` | [references/kubernetes.md](references/kubernetes.md) |
| GitLab | `dex gl` | [references/gitlab.md](references/gitlab.md) |
| Jira | `dex jira` | [references/jira.md](references/jira.md) |
| Slack | `dex slack` | [references/slack.md](references/slack.md) |
| Loki | `dex loki` | [references/loki.md](references/loki.md) |

## Quick Reference

### Kubernetes (`dex k8s`)
```bash
dex k8s ctx ls                    # List contexts
dex k8s ns ls                     # List namespaces
dex k8s pod ls [-A] [-n ns]       # List pods
dex k8s pod logs <name> [-f]      # Stream pod logs
dex k8s svc ls                    # List services
```

### GitLab (`dex gl`)
```bash
dex gl activity [--since 7d]      # Recent activity
dex gl mr ls                      # List open MRs
dex gl mr show <project!iid>      # Show MR details
dex gl mr create "<title>"        # Create MR from current branch
```

### Jira (`dex jira`)
```bash
dex jira my                       # Issues assigned to me
dex jira view <KEY>               # View issue details
dex jira search "<JQL>"           # Search with JQL
```

### Slack (`dex slack`)
```bash
dex slack send <channel> "msg"    # Send message
dex slack send <ch> "msg" -t <ts> # Reply to thread
dex slack mentions [--unhandled]  # My mentions today
dex slack search "query"          # Search messages
dex slack thread <url|ch:ts>      # View thread details
```

### Loki (`dex loki`)
```bash
dex loki discover                 # Auto-discover Loki in k8s cluster
dex loki query '{job="app"}'      # Query (current k8s namespace)
dex loki query '{job="app"}' -A   # Query all namespaces
dex loki query '{job="app"}' -n prod  # Query specific namespace
dex loki query '{app="x"} |= "error"' -s 30m  # Filter + time range
dex loki labels                   # List label names
dex loki labels job               # List values for label
dex loki test                     # Test connection
```

## Tips

- Command aliases: `k8s`=`kube`=`kubernetes`, `gl`=`gitlab`
- Use `-n` for namespace, `-A` for all namespaces in k8s
- MR format: `project!iid` (e.g., `sre/helm!2903`)
- Loki URL: set `LOKI_URL` env var or use `--url` flag
