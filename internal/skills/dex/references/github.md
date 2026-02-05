# GitHub Commands (`dex gh`)

Wrapper around the `gh` CLI for GitHub operations.

## Prerequisites

Requires the [GitHub CLI](https://cli.github.com/) (`gh`) to be installed.

## Authentication
```bash
dex gh auth                       # Interactive authentication (runs gh auth login)
dex gh test                       # Test current authentication status
```

## Clone Repository
```bash
dex gh clone <repo> [dest]        # Clone a repository
dex gh clone owner/repo           # Clone using short form
dex gh clone owner/repo ./mydir   # Clone to specific directory
dex gh clone https://github.com/owner/repo  # Clone using full URL
```

## Issue Management

### List Issues
```bash
dex gh issue list                 # List open issues in current repo
dex gh issue ls                   # Alias for list
dex gh issue list -s closed       # List closed issues
dex gh issue list -s all          # List all issues
dex gh issue list -l bug          # Filter by label
dex gh issue list -a @me          # Filter by assignee
dex gh issue list -L 50           # Limit results (default 30)
dex gh issue list -R owner/repo   # List issues in different repo
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--state` | `-s` | Filter by state: `open`, `closed`, `all` (default: open) |
| `--label` | `-l` | Filter by label |
| `--assignee` | `-a` | Filter by assignee |
| `--limit` | `-L` | Maximum issues to fetch (default 30) |
| `--repo` | `-R` | Repository in `owner/repo` format |

### View Issue
```bash
dex gh issue view 123             # View issue #123 in current repo
dex gh issue view 123 -R owner/repo  # View issue in different repo
```

Output includes: title, state, author, created date, labels, assignees, URL, and body.

### Create Issue
```bash
dex gh issue create -t "Bug: login fails"                    # Title only
dex gh issue create -t "Bug" -b "Steps to reproduce..."      # With body
dex gh issue create -t "Bug" -l bug -l urgent                # With labels
dex gh issue create -t "Bug" -a username                     # With assignee
dex gh issue create -t "Bug" -R owner/repo                   # In different repo
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Issue title (required) |
| `--body` | `-b` | Issue body/description |
| `--label` | `-l` | Labels to add (repeatable) |
| `--assignee` | `-a` | Assignee username |
| `--repo` | `-R` | Repository in `owner/repo` format |

### Close Issue
```bash
dex gh issue close 123                              # Close issue #123
dex gh issue close 123 -c "Fixed in PR #456"        # With closing comment
dex gh issue close 123 -r "not planned"             # With reason
dex gh issue close 123 -R owner/repo                # In different repo
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--comment` | `-c` | Leave a closing comment |
| `--reason` | `-r` | Reason: `completed` or `not planned` |
| `--repo` | `-R` | Repository in `owner/repo` format |

## Tips

- Command aliases: `gh` = `github`
- All commands support `-R owner/repo` for cross-repo operations
- Issue list output shows issue number, title, and first label
- The `gh` CLI must be authenticated (`dex gh auth` or `gh auth login`)
