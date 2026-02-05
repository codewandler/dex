---
name: dex
description: Run dex CLI commands for Kubernetes, GitLab, Jira, and Slack operations
user-invocable: true
---

# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, Jira, and Slack operations. Run commands via Bash tool.

## Kubernetes (`dex k8s`)

### Contexts
```bash
dex k8s ctx ls                    # List all kubeconfig contexts (* = current)
```

### Namespaces
```bash
dex k8s ns ls                     # List all namespaces
```

### Pods
```bash
dex k8s pod ls                    # List pods in current namespace
dex k8s pod ls -A                 # List pods in all namespaces
dex k8s pod ls -n kube-system     # List pods in specific namespace
dex k8s pod show <name>           # Show pod details (containers, conditions, labels)
dex k8s pod show <name> -n <ns>   # Show pod in specific namespace
```

### Pod Logs (stern-like)
```bash
dex k8s pod logs <name>           # Stream logs (all containers if multi-container)
dex k8s pod logs <name> -f        # Follow logs
dex k8s pod logs <name> --tail 100    # Last N lines
dex k8s pod logs <name> --since 1h    # Logs from last hour (supports: 30m, 1h, 1d)
dex k8s pod logs <name> -c <container>  # Specific container only
dex k8s pod logs <name> -i "error"    # Include only lines matching regex
dex k8s pod logs <name> -e "debug"    # Exclude lines matching regex
dex k8s pod logs <name> -p        # Previous container instance
```

### Services
```bash
dex k8s svc ls                    # List services in current namespace
dex k8s svc ls -A                 # List services in all namespaces
dex k8s svc ls -n kube-system     # List services in specific namespace
dex k8s svc show <name>           # Show service details (ports, selectors, ingress)
```

## GitLab (`dex gl` or `dex gitlab`)

### Activity
```bash
dex gl activity                   # Show activity from last 14 days
dex gl activity --since 7d        # Activity from last 7 days
dex gl activity --since 4h        # Activity from last 4 hours
```

### Project Index
```bash
dex gl index                      # Index all accessible projects (cached 24h)
dex gl index --force              # Force re-index
```

### Projects
```bash
dex gl proj ls                    # List projects (from index)
dex gl proj ls -n 50              # List 50 projects
dex gl proj ls --sort name        # Sort by name (also: created, activity, path)
dex gl proj ls --sort created -d  # Sort descending (default for dates, ascending for names)
dex gl proj ls --no-cache         # Fetch from API instead of index
dex gl proj show <id|path>        # Show project details
dex gl proj show <id> --no-cache  # Always fetch from API, bypass cache
```

### Commits
```bash
dex gl commit show <project> <sha>   # Show full commit details (message body, stats)
dex gl commit show 742 95a1e625      # By project ID
dex gl commit show group/proj abc123 # By project path
```

### Merge Requests
```bash
# List MRs
dex gl mr ls                         # List open MRs (excludes WIP/drafts)
dex gl mr ls -n 50                   # List 50 MRs
dex gl mr ls --state merged          # List merged MRs
dex gl mr ls --state closed          # List closed MRs
dex gl mr ls --state all             # All MRs regardless of state
dex gl mr ls --scope created_by_me   # MRs you created
dex gl mr ls --scope assigned_to_me  # MRs assigned to you
dex gl mr ls --include-wip           # Include WIP/draft MRs
dex gl mr ls --conflicts-only        # Only show MRs with merge conflicts

# Show MR details (use project!iid format)
dex gl mr show <project!iid>         # Show full MR details with discussion IDs
dex gl mr show sre/helm!2903         # Example
dex gl mr show sre/helm!2903 --show-diff  # Include file diffs in output

# Open in browser
dex gl mr open <project!iid>         # Open MR in default browser
dex gl mr open sbf/services!2483     # Example

# Add comment to MR
dex gl mr comment <project!iid> "message"  # Add a comment
dex gl mr comment sbf/services!2483 "LGTM"  # Example
echo "Long comment" | dex gl mr comment sbf/services!2483 -  # From stdin

# Reply to discussion thread (use discussion ID from mr show output)
dex gl mr comment <project!iid> "reply" --reply-to <discussion-id>
dex gl mr comment proj!123 "Fixed!" --reply-to abc123def456...

# Add inline comment on file/line
dex gl mr comment <project!iid> "comment" --file <path> --line <n>
dex gl mr comment proj!123 "Use a constant" --file src/main.go --line 42

# Add emoji reaction to MR or comment
dex gl mr react <project!iid> <emoji>           # React to MR
dex gl mr react proj!123 thumbsup               # Example
dex gl mr react proj!123 :heart:                # Colons optional
dex gl mr react proj!123 rocket --note <id>     # React to specific note/comment

# Close MR
dex gl mr close <project!iid>                   # Close a merge request
dex gl mr close sre/helm!2903                   # Example

# Approve MR
dex gl mr approve <project!iid>                 # Approve a merge request
dex gl mr approve sre/helm!2903                 # Example

# Merge MR
dex gl mr merge <project!iid>                   # Merge a merge request
dex gl mr merge proj!123 --squash               # Squash commits
dex gl mr merge proj!123 --remove-source-branch # Delete branch after merge
dex gl mr merge proj!123 --when-pipeline-succeeds  # Merge when CI passes
dex gl mr merge proj!123 -m "Custom message"    # Custom merge commit message

# Create MR (auto-detects project and branch from git)
dex gl mr create "<title>"                      # Create MR from current branch to main
dex gl mr create "Fix bug" --target develop     # Specify target branch
dex gl mr create "WIP" --draft                  # Create as draft
dex gl mr create "Feature" -d "Description"     # With description
dex gl mr create "Feature" --squash             # Squash on merge
dex gl mr create "Feature" --remove-source-branch  # Delete branch after merge
dex gl mr create "Feature" -p group/proj -s branch # Explicit project and source
```

## Jira (`dex jira`)

### Authentication
```bash
dex jira auth                     # Authenticate via OAuth (opens browser)
```

### Projects
```bash
dex jira projects                 # List active projects (excludes archived)
dex jira projects --keys          # Output only project keys (one per line)
dex jira projects --archived      # Include archived projects
```

Project keys (e.g., DEV, TEL, SRE) are the prefixes used in issue keys like DEV-123.
Archived projects (names starting with "z[archive]") are hidden by default.

### View Issue (Full Details)
```bash
dex jira view <KEY>               # View issue with description, comments, links, subtasks
dex jira view TEL-112             # Example: full ticket details
```

The view command shows:
- Basic info (type, status, priority, assignee, reporter, labels, dates)
- Parent issue (for subtasks)
- Subtasks list (for parent tickets)
- Linked issues (blocks, is blocked by, relates to, etc.)
- Full description (parsed from Atlassian Document Format)
- All comments with authors and timestamps

### Search Issues
```bash
dex jira my                       # Issues assigned to me
dex jira my -l 50                 # Increase limit (default 20)
dex jira search "<JQL>"           # Search with JQL query
dex jira lookup KEY1 KEY2 KEY3    # Quick lookup of multiple issues
```

### Useful JQL Search Examples
```bash
# Recent activity
dex jira search "updated >= -7d ORDER BY updated DESC"
dex jira search "updated >= -30d ORDER BY updated DESC" -l 20

# By status
dex jira search "status = 'In Progress' ORDER BY updated DESC"
dex jira search "status = 'Review' ORDER BY updated DESC"
dex jira search "status != Done ORDER BY priority DESC"

# By type
dex jira search "type = Epic ORDER BY updated DESC"
dex jira search "type = Sub-task ORDER BY updated DESC"
dex jira search "type = Bug AND status != Done"

# By project
dex jira search "project = DEV ORDER BY updated DESC"
dex jira search "project = SRE AND status = 'In Progress'"
dex jira search "project in (DEV, SRE) AND updated >= -7d"

# By assignee
dex jira search "assignee = currentUser() AND status != Done"
dex jira search "assignee WAS currentUser() AND status = Done AND updated >= -30d"

# Combined filters
dex jira search "project = DEV AND type = Epic AND status != Done"
dex jira search "priority = High AND status != Done ORDER BY created ASC"
dex jira search "labels = urgent AND status != Done"

# Text search
dex jira search "summary ~ 'performance' ORDER BY updated DESC"
dex jira search "text ~ 'database index' ORDER BY updated DESC"
```

## Slack (`dex slack`)

### Authentication
```bash
dex slack auth                    # Authenticate via OAuth (opens browser)
dex slack test                    # Test current authentication
```

OAuth requires `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` configured.
Callback URL: `https://localhost:8089/callback`

### Identity Info
```bash
dex slack info                    # Show who you are (bot and user perspectives)
```

Shows authenticated identities for both tokens:
- **Bot token**: Used for sending messages, reading channels, listing users
- **User token**: Used for search API, mentions search

Useful for understanding which identity will perform actions.

### Presence
```bash
dex slack presence                # Show current presence (requires users:read scope)
dex slack presence set auto       # Set to auto (online when active)
dex slack presence set away       # Set to away
```

Requires user token. Scopes needed:
- `users:read` - for viewing presence
- `users:write` - for setting presence

### Index (Channels & Users)
```bash
dex slack index                   # Index channels and users (cached 24h)
dex slack index --force           # Force re-index
```

Index stored at `~/.dex/slack/index.json` (channels and users combined).

Required for channel/user name autocomplete and @username DMs.

### List Channels
```bash
dex slack channels                # List all indexed channels
dex slack channels --member       # Only channels bot is a member of (can post to)
dex slack channels --no-cache     # Fetch from API instead of index
```

### List Users
```bash
dex slack users                   # List all indexed users
dex slack users --no-cache        # Fetch from API instead of index
```

### Send Message
```bash
# To channel (by name or ID)
dex slack send dev-team "Hello from dex!"
dex slack send C03JDUBJD0D "Hello!"

# With @mentions in message (auto-resolved to Slack mentions)
dex slack send dev-team "Hey @timo.friedl check this!"
dex slack send dev-team "@alice @bob please review"

# Reply to thread (use -t with thread timestamp from previous send)
dex slack send dev-team "Follow up" -t 1770257991.873399
dex slack send dev-team "Another reply" --thread 1770257991.873399

# To user DM (requires im:write scope)
dex slack send @timo.friedl "Hey, check this out!"

# Send as user instead of bot (requires user token with chat:write scope)
dex slack send dev-team "Message from me" --as user
```

- Channel names and @usernames autocomplete from index
- @mentions in message body are auto-converted to `<@USER_ID>` format
- Use `-t <ts>` to continue a thread (ts returned from previous send)
- Use `--as user` to send as yourself instead of the bot (requires SLACK_USER_TOKEN)

### Search Mentions
```bash
dex slack mentions                    # My mentions today (requires user token)
dex slack mentions --unhandled        # Only pending mentions (no reaction/reply from you)
dex slack mentions --bot              # Bot mentions today
dex slack mentions --user timo.friedl # Mentions of a specific user
dex slack mentions --user U03HY52RQLV # By user ID
dex slack mentions --since 1h         # Mentions from last hour
dex slack mentions --since 7d         # Mentions from last 7 days
dex slack mentions --limit 50         # Show more results (default 20)
dex slack mentions --compact          # Compact table view
```

**Default behavior:**
- No flags: searches for mentions of the authenticated user (from user token)
- `--bot`: searches for mentions of the bot
- `--user <name>`: searches for mentions of a specific user
- `--unhandled`: filters to show only pending mentions

**Status categories:**
- `Pending` - No reaction or reply from you (bot or user)
- `Acked` - You reacted but didn't reply
- `Replied` - You replied in the thread

**Default time range:** Today (since midnight). Use `--since` to override.

**Requires user token** for search API. Falls back to channel scanning with bot token only.

Time filters: `1h`, `30m`, `1d`, `7d`, `2w` etc.

Default expanded view shows full message text, timestamps, and Slack permalinks. Use `--compact` for a condensed table.

### Search Messages (requires user token)
```bash
# General search
dex slack search "query"              # Search all messages
dex slack search "deployment"         # Find deployment-related messages
dex slack search "error" --since 1d   # Errors in last day
dex slack search "from:@timo.friedl"  # Messages from specific user
dex slack search "in:#dev-team"       # Messages in specific channel

# Extract Jira tickets from results
dex slack search "bug" --tickets              # Find tickets mentioned with "bug"
dex slack search "DEV-" --tickets             # Find all DEV tickets mentioned
dex slack search "TEL-" --tickets --since 7d  # TEL tickets from last week
dex slack search "urgent" --tickets --compact # Compact ticket list

# Output control
dex slack search "query" --limit 50   # More results (default 50)
dex slack search "query" --compact    # Compact table view
```

**Ticket extraction (`--tickets`):**
- Fetches Jira project keys to identify valid ticket patterns
- Extracts tickets like DEV-123, TEL-456 from message text
- Groups by ticket and shows mention count + permalinks
- Useful for finding "which tickets were discussed recently"

**Slack search syntax:**
- `from:@username` - Messages from a specific user
- `in:#channel` - Messages in a specific channel
- `has:link` - Messages containing links
- `before:YYYY-MM-DD`, `after:YYYY-MM-DD` - Date filters

## Tips

- All k8s commands support shell completion for resource names
- Pod logs `-c` flag autocompletes container names
- GitLab project names autocomplete from local index
- Slack channel names and @usernames autocomplete from local index
- Use `-n` for namespace, `-A` for all namespaces in k8s commands
- Command aliases: `k8s`=`kube`=`kubernetes`, `gl`=`gitlab`, `mr`=`merge-request`
- Generate shell completions: `dex completion bash|zsh|fish|powershell`
