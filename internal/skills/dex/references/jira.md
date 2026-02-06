# Jira Commands (`dex jira`)

## Authentication
```bash
dex jira auth                     # Authenticate via OAuth (opens browser)
```

## Projects
```bash
dex jira projects                 # List active projects (excludes archived)
dex jira projects --keys          # Output only project keys (one per line)
dex jira projects --archived      # Include archived projects
```

Project keys (e.g., DEV, TEL, SRE) are the prefixes used in issue keys like DEV-123.
Archived projects (names starting with "z[archive]") are hidden by default.

## Create Issue
```bash
dex jira create -p <project> -t <type> -s "<summary>" [options]
```

### Required Flags
- `-p, --project` - Project key (e.g., DEV, TEL)
- `-t, --type` - Issue type (Task, Bug, Story, Sub-task)
- `-s, --summary` - Issue summary/title

### Optional Flags
- `-d, --description` - Issue description (plain text)
- `-l, --labels` - Comma-separated labels
- `-a, --assignee` - Assignee (email or account ID)
- `--priority` - Priority (Lowest, Low, Medium, High, Highest)
- `--parent` - Parent issue key for subtasks (e.g., DEV-123)

### Examples
```bash
# Basic task
dex jira create -p DEV -t Task -s "Update API documentation"

# Bug with description
dex jira create -p DEV -t Bug -s "Login fails" -d "Users report 500 error on login"

# Story with labels
dex jira create -p TEL -t Story -s "Add dark mode" -l ui,enhancement

# Task with assignee and priority
dex jira create -p DEV -t Task -s "Fix tests" -a user@example.com --priority High

# Subtask under a parent issue
dex jira create -p DEV -t Sub-task -s "Write unit tests" --parent DEV-123
```

**Note:** After adding `write:jira-work` scope, users need to re-authenticate with `dex jira auth`.

## Delete Issue
```bash
dex jira delete <ISSUE-KEY> [ISSUE-KEY...]
```

Delete one or more Jira issues. Subtasks are automatically deleted with their parent.

### Examples
```bash
dex jira delete DEV-123
dex jira delete DEV-400 DEV-401 DEV-402
```

## Link Issues
```bash
dex jira link <SOURCE-KEY> <TARGET-KEY> [TARGET-KEY...] [-t <type>]
```

Create links between Jira issues. The first issue is the source, subsequent issues are linked to it.

### Flags
- `-t, --type` - Link type (default: "Relates")
- `--list-types` - List available link types in your Jira instance

### Common Link Types
| Type | Outward | Inward |
|------|---------|--------|
| Relates | relates to | relates to |
| Blocks | blocks | is blocked by |
| Cloners | clones | is cloned by |
| Duplicate | duplicates | is duplicated by |

### Examples
```bash
# List available link types
dex jira link --list-types

# Link two issues (default: Relates)
dex jira link DEV-123 DEV-456

# Link multiple issues to a source
dex jira link DEV-123 DEV-456 DEV-789

# DEV-123 blocks DEV-456
dex jira link DEV-123 DEV-456 -t Blocks

# DEV-123 duplicates DEV-456
dex jira link DEV-123 DEV-456 -t Duplicate
```

## Update Issue
```bash
dex jira update <ISSUE-KEY> [flags]
```

Update fields on an existing issue.

### Flags
- `-s, --summary` - New summary/title
- `-d, --description` - New description
- `-a, --assignee` - New assignee (email or account ID, empty string to unassign)
- `-p, --priority` - New priority (Lowest, Low, Medium, High, Highest)
- `--add-label` - Labels to add (can specify multiple)
- `--remove-label` - Labels to remove (can specify multiple)

### Examples
```bash
# Update summary
dex jira update DEV-123 --summary "New title"

# Update multiple fields
dex jira update DEV-123 --assignee user@example.com --priority High

# Manage labels
dex jira update DEV-123 --add-label urgent --add-label critical
dex jira update DEV-123 --remove-label backlog

# Unassign an issue
dex jira update DEV-123 --assignee ""
```

## Transition Issue
```bash
dex jira transition <ISSUE-KEY> [STATUS]
dex jira transition <ISSUE-KEY> --list
```

Move an issue through its workflow to a new status.

### Flags
- `-l, --list` - List available transitions for the issue

### Examples
```bash
# List available transitions
dex jira transition DEV-123 --list

# Transition to a new status
dex jira transition DEV-123 "In Progress"
dex jira transition DEV-123 Done
dex jira transition DEV-123 Review
```

## Comment on Issue
```bash
dex jira comment <ISSUE-KEY> "<MESSAGE>"
dex jira comment <ISSUE-KEY> --body "<MESSAGE>"
```

Add a comment to an issue.

### Flags
- `-b, --body` - Comment body (alternative to positional argument)

### Examples
```bash
dex jira comment DEV-123 "Working on this now"
dex jira comment DEV-123 --body "Multi-line comment
with more details here"
```

## Delete Comment
```bash
dex jira comment-delete <ISSUE-KEY> <COMMENT-ID>
```

Delete a comment from an issue. The comment ID is shown when adding a comment or visible in `dex jira view`.

### Examples
```bash
dex jira comment-delete DEV-123 10042
```

## View Issue (Full Details)
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

## Search Issues
```bash
dex jira my                       # Issues assigned to me (excludes Done)
dex jira my -l 50                 # Increase limit (default 20)
dex jira my -s "In Progress"      # Filter by status
dex jira my -s "Review"           # Filter by status
dex jira search "<JQL>"           # Search with JQL query
dex jira lookup KEY1 KEY2 KEY3    # Quick lookup of multiple issues
```

## JQL Search Examples

### Recent Activity
```bash
dex jira search "updated >= -7d ORDER BY updated DESC"
dex jira search "updated >= -30d ORDER BY updated DESC" -l 20
```

### By Status
```bash
dex jira search "status = 'In Progress' ORDER BY updated DESC"
dex jira search "status = 'Review' ORDER BY updated DESC"
dex jira search "status != Done ORDER BY priority DESC"
```

### By Type
```bash
dex jira search "type = Epic ORDER BY updated DESC"
dex jira search "type = Sub-task ORDER BY updated DESC"
dex jira search "type = Bug AND status != Done"
```

### By Project
```bash
dex jira search "project = DEV ORDER BY updated DESC"
dex jira search "project = SRE AND status = 'In Progress'"
dex jira search "project in (DEV, SRE) AND updated >= -7d"
```

### By Assignee
```bash
dex jira search "assignee = currentUser() AND status != Done"
dex jira search "assignee WAS currentUser() AND status = Done AND updated >= -30d"
```

### Combined Filters
```bash
dex jira search "project = DEV AND type = Epic AND status != Done"
dex jira search "priority = High AND status != Done ORDER BY created ASC"
dex jira search "labels = urgent AND status != Done"
```

### Text Search
```bash
dex jira search "summary ~ 'performance' ORDER BY updated DESC"
dex jira search "text ~ 'database index' ORDER BY updated DESC"
```
