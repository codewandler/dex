---
name: dex
description: Run dex CLI commands for Kubernetes, GitLab, Jira, Slack, GitHub, and Loki operations
user-invocable: true
---

# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, Jira, Slack, GitHub, Loki, and SQL operations. Run commands via Bash tool.

**IMPORTANT:** When the user's request matches an integration (e.g., GitLab MRs, Kubernetes pods, Slack messages), you MUST load the corresponding reference file from the table below before executing commands. The reference files contain the full command documentation needed for correct usage.

## Setup & Diagnostics

```bash
dex setup                         # Interactive setup wizard (only prompts for unconfigured integrations)
dex doctor                        # Check health of all configured integrations
dex upgrade                       # Upgrade to latest version
dex upgrade -v v0.2.0             # Upgrade to specific version
dex version                       # Print version information
dex completion bash|zsh|fish      # Generate shell completions
dex install                       # Install dex skill to ~/.claude/skills/dex/
dex skill show                    # Print dex skill content to stdout
dex skill search <query>          # Search for skills on skills.sh
dex skill install <name>          # Install skill from skills.sh (local: ./.claude/skills/)
dex skill install <name> -g       # Install skill globally (~/.claude/skills/)
```

## Integrations

| Integration | Command | Reference |
|-------------|---------|-----------|
| Kubernetes | `dex k8s` | [references/kubernetes.md](references/kubernetes.md) |
| GitLab | `dex gl` | [references/gitlab.md](references/gitlab.md) |
| GitHub | `dex gh` | [references/github.md](references/github.md) |
| Jira | `dex jira` | [references/jira.md](references/jira.md) |
| Slack | `dex slack` | [references/slack.md](references/slack.md) |
| Loki | `dex loki` | [references/loki.md](references/loki.md) |
| SQL | `dex sql` | [references/sql.md](references/sql.md) |
| Claude Code | `dex claude` | [references/claude.md](references/claude.md) |

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
dex jira my -s "In Progress"      # Filter by status
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

### GitHub (`dex gh`)
```bash
dex gh auth                       # Interactive GitHub authentication
dex gh test                       # Test gh CLI authentication
dex gh clone <repo> [dest]        # Clone repo (uses gh CLI)
dex gh issue ls                   # List open issues
dex gh issue ls --no-label        # List issues without labels
dex gh issue view <number>        # View issue details
dex gh issue create -t "title"    # Create new issue
dex gh issue edit <num> -a "label"    # Add label to issue
dex gh issue edit <num> -r "label"    # Remove label from issue
dex gh issue comment <num> -b "text"  # Comment on issue
dex gh issue close <number>       # Close an issue
dex gh label ls                   # List labels
dex gh label create "name"        # Create a label
dex gh label delete "name"        # Delete a label
dex gh release ls                 # List releases
dex gh release view [tag]         # View release (latest if no tag)
dex gh release create <tag> -n "notes"  # Create release
dex gh release create <tag> --generate-notes  # Auto-generate notes
```

### Loki (`dex loki`)
```bash
dex loki discover                 # Auto-discover Loki in k8s cluster
dex loki query '{job="app"}'      # Query (current k8s namespace)
dex loki query '{job="app"}' -A   # Query all namespaces
dex loki query '{job="app"}' -n prod  # Query specific namespace
dex loki query '{app="x"} |= "error"' -s 30m  # Filter + time range
dex loki labels                   # List labels (current namespace)
dex loki labels -A                # List labels (all namespaces)
dex loki labels job               # List values for label
dex loki test                     # Test connection
```

### SQL (`dex sql`)
```bash
dex sql datasources               # List configured datasources
dex sql query -d <ds> "<query>"   # Execute SQL query
```

### Claude Code (`dex claude`)
```bash
dex claude statusline             # Generate status line for Claude Code
```

## Tips

- Command aliases: `k8s`=`kube`=`kubernetes`, `gl`=`gitlab`, `gh`=`github`
- Use `-n` for namespace, `-A` for all namespaces in k8s
- MR format: `project!iid` (e.g., `sre/helm!2903`)
- Loki URL: set `LOKI_URL` env var or use `--url` flag (auto-discovers if not set)
