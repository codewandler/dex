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
dex jira my                       # Issues assigned to me
dex jira my -l 50                 # Increase limit (default 20)
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
