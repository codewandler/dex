---
name: dex
description: Run dex CLI commands for Kubernetes, GitLab, and Jira operations
user-invocable: true
---

# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, and Jira operations. Run commands via Bash tool.

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

## Tips

- All k8s commands support shell completion for resource names
- Pod logs `-c` flag autocompletes container names
- GitLab project names autocomplete from local index
- Use `-n` for namespace, `-A` for all namespaces in k8s commands
- Command aliases: `k8s`=`kube`=`kubernetes`, `gl`=`gitlab`, `mr`=`merge-request`
- Generate shell completions: `dex completion bash|zsh|fish|powershell`
