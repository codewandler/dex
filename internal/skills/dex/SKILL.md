---
name: dex
description: Run dex CLI commands for Kubernetes, GitLab, Jira, Slack, GitHub, Loki, Homer, and Prometheus operations
user-invocable: true
---

# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, Jira, Slack, GitHub, Loki, Homer, Prometheus, and SQL operations. Run commands via Bash tool.

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
| Homer | `dex homer` | [references/homer.md](references/homer.md) |
| Prometheus | `dex prom` | [references/prometheus.md](references/prometheus.md) |
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
dex k8s forward ls                # List active port-forwards
dex k8s forward start <query>               # Smart discovery: auto-detect pod, port, namespace
dex k8s forward start <pod> <port> -n <ns>  # Explicit: start detached port-forward
dex k8s forward stop <name>       # Stop a port-forward
```

### GitLab (`dex gl`)
```bash
dex gl activity [--since 7d]      # Recent activity
dex gl commit ls <project>        # List project commits
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
dex jira link <KEY> <KEY> [-t type]  # Link issues together
dex jira update <KEY> [--flags]   # Update issue fields
dex jira transition <KEY> <status>   # Change issue status
dex jira comment <KEY> "message"  # Add comment (supports markdown)
dex jira comment-delete <KEY> <ID>  # Delete a comment
```

### Slack (`dex slack`)
```bash
dex slack send <channel> "msg"    # Send message (@mentions and #channels auto-resolved)
dex slack send <ch> "msg" -t <ts> # Reply to thread
dex slack edit <ch> <ts> "msg"   # Edit a message
dex slack delete <ch> <ts>       # Delete a message
dex slack mentions [--unhandled]  # My mentions today
dex slack search "query"          # Search messages
dex slack thread <url|ch:ts>      # View thread details
dex slack users [query]           # Search/list users (resolve handles)
dex slack users --channel <name>  # List users in a channel
dex slack channels                # List channels (resolve names)
dex slack channels --user <name>  # Channels a user belongs to
dex slack channel members <name>  # List members of a channel
```

### GitHub (`dex gh`)
```bash
dex gh auth                       # Interactive GitHub authentication
dex gh test                       # Test gh CLI authentication
dex gh clone <repo> [dest]        # Clone repo (uses gh CLI)
dex gh repo create <name> --public    # Create public repository
dex gh repo create <name> --private   # Create private repository
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
dex loki query '{job="app"}' --since 2d --until 1d  # Relative window
dex loki query '{job="app"}' --since "2026-02-04 15:00" --until "2026-02-04 16:00"
dex loki query '{job="app"}' --since "2026-02-04T15:00:00Z"  # UTC timestamp
dex loki query '{job="app"}' --since "2026-02-04 15:00" --utc # Interpret as UTC
dex loki query '{job="app"}' --labels pod,container  # Show specific labels
dex loki query '{job="app"}' --labels ""             # Show all labels
dex loki labels                   # List labels (current namespace)
dex loki labels -A                # List labels (all namespaces)
dex loki labels job               # List values for label
dex loki test                     # Test connection
```

### Prometheus (`dex prom`)
```bash
dex prom discover                 # Auto-discover Prometheus in k8s cluster
dex prom query 'up'               # Instant query
dex prom query 'up' -o json       # JSON output
dex prom query 'up' --time "2026-02-04 15:00"  # Query at specific time
dex prom query-range 'rate(http_requests_total[5m])' --since 1h  # Range query
dex prom query-range 'up' --since 30m --step 15s  # Custom step
dex prom query-range 'up' --since "2026-02-04 15:00" --until "2026-02-04 16:00"
dex prom labels                   # List all label names
dex prom labels job               # List values for label
dex prom labels -m 'up{job="x"}'  # Scoped to matching series
dex prom targets                  # Scrape targets
dex prom targets --state dropped  # Dropped targets
dex prom alerts                   # Active alerts
dex prom test                     # Test connection
```

### Homer (`dex homer`)
```bash
dex homer discover                # Find Homer via K8s service discovery
dex homer discover -n eu          # Discover in specific namespace
dex homer calls --since 1h        # List calls grouped by Call-ID
dex homer calls --number "123" --since 2h  # Calls to number in last 2h
dex homer calls --from-user "999%" --since 1h  # Filter by caller
dex homer calls -q "ua = 'Asterisk%'" --since 1h  # Custom query
dex homer calls --since 1h -o json  # JSON output
dex homer search --number "49215..."  # Search by number (from_user and to_user)
dex homer search --from-user "999%" --to-user "12345"  # Filter by caller/callee
dex homer search --from-user "999%" --ua "Asterisk%"   # Combine with user agent
dex homer search -q "from_user = '123' AND status = 200"  # Query with field validation
dex homer search --at "2026-02-04 17:13"  # Search around a specific time
dex homer search --number "123" -m INVITE -m BYE  # Filter by SIP method
dex homer search --number "123" -o json   # JSON output
dex homer show <call-id>          # Show SIP message flow
dex homer show id1 id2 id3        # Combined flow for multiple calls
dex homer show <call-id> --raw    # Show raw SIP message bodies
dex homer export <call-id>        # Export call as PCAP
dex homer analyze <call-id> -c X-Acme-Call-ID  # Correlate multi-leg call by header
dex homer analyze <call-id> -c X-Acme-Call-ID -H X-Acme -N 49341550035  # With extra columns and numbers
dex homer qos <call-id>           # Show RTCP quality metrics (jitter, loss, MOS)
dex homer qos <call-id> --clock 16000  # Custom RTP clock rate
dex homer qos <call-id> -o json   # JSON output
dex homer aliases                 # List IP/port aliases
dex homer endpoints               # List configured endpoints with URLs
```

### SQL (`dex sql`)
```bash
dex sql datasources               # List configured datasources
dex sql query -d <ds> "<query>"   # Execute SQL query
```

### Todo (`dex todo`)
```bash
dex todo add <TITLE> <DESC>       # Add a new todo
dex todo ls [--state STATE]       # List todos (filter: pending, in_progress, on_hold, done)
dex todo update <ID> [--flags]    # Update todo (--state, --title, --desc)
dex todo ref add <ID> <TYPE> <VAL>  # Add reference to todo
dex todo ref del <ID> <REF_ID>    # Remove reference from todo
```

### Claude Code (`dex claude`)
```bash
dex claude statusline             # Generate status line for Claude Code
```

## Tips

- Command aliases: `k8s`=`kube`=`kubernetes`, `gl`=`gitlab`, `gh`=`github`
- Use `-n` for namespace, `-A` for all namespaces in k8s
- MR format: `project!iid` (e.g., `my-group/my-project!123`)
- Loki URL: set `LOKI_URL` env var or use `--url` flag (auto-discovers if not set)
- Homer URL: set `HOMER_URL` env var or use `--url` flag (auto-discovers `homer-webapp` service in K8s)
- Prometheus URL: set `PROMETHEUS_URL` env var or use `--url` flag (auto-discovers if not set)
- Command aliases: `homer`=`sip`, `prom`=`prometheus`
