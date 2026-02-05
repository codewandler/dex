# dex - the engineer's CLI

## Overview

Swiss army knife CLI for engineers. Usable standalone but primarily designed as a tool for AI agents via the Claude skill at `.claude/skills/dex`.

**Integrations:**
- **Kubernetes** - Cluster management (contexts, namespaces, pods, services, logs)
- **GitLab** - Activity tracking, MRs, commits, project management
- **Jira** - Issue management (OAuth)

## Project Structure

```
dex/
├── cmd/dex/main.go                # Entry point
├── internal/
│   ├── cli/                       # Cobra CLI commands
│   │   ├── root.go                # Root command
│   │   ├── gitlab.go              # GitLab commands (gl/gitlab)
│   │   ├── jira.go                # Jira commands
│   │   └── k8s.go                 # Kubernetes commands
│   ├── config/config.go           # Environment configuration
│   ├── gitlab/                    # GitLab API client
│   │   ├── client.go              # Main client
│   │   ├── commits.go             # Commit operations
│   │   ├── mergerequests.go       # MR operations
│   │   ├── projects.go            # Project operations
│   │   ├── index.go               # Local project index
│   │   └── tags.go                # Tag operations
│   ├── jira/                      # Jira OAuth client
│   ├── k8s/                       # Kubernetes client
│   ├── models/                    # Data structures
│   └── output/                    # Terminal formatting
├── .claude/skills/dex/            # Claude skill definition
└── templates/                     # Templates
```

## Build & Install

Requires [Task](https://taskfile.dev).

```bash
task build                          # Build to ./bin/dex
task install                        # Install to $GOPATH/bin
task clean                          # Remove build artifacts
```

After changes, run `task install` to install the updated binary.

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITLAB_URL` | For GitLab | GitLab instance URL |
| `GITLAB_PERSONAL_TOKEN` | For GitLab | Personal access token |
| `JIRA_CLIENT_ID` | For Jira | OAuth 2.0 client ID |
| `JIRA_CLIENT_SECRET` | For Jira | OAuth 2.0 client secret |

## Command Overview

```bash
dex k8s ...    # Kubernetes (aliases: kube, kubernetes)
dex gl ...     # GitLab (aliases: gitlab)
dex jira ...   # Jira
```

For full command reference, see `.claude/skills/dex/SKILL.md`.

## Claude Skill

This CLI ships with a Claude Code skill at `.claude/skills/dex/SKILL.md` that documents all commands for AI agent use. The skill enables AI assistants to:
- Query Kubernetes clusters
- Browse GitLab activity and MRs
- Look up Jira issues
- Interact with MRs (comment, react, view diffs)

## Jira OAuth Setup

1. Create OAuth 2.0 app at https://developer.atlassian.com/console/myapps/
2. Add callback URL: `http://localhost:8089/callback`
3. Add Jira API permissions: `read:jira-work`, `read:jira-user`
4. Set `JIRA_CLIENT_ID` and `JIRA_CLIENT_SECRET`
5. Run `dex jira auth`

Token stored at `~/.config/jira-oauth/token.json`

## Development

### Documentation Sync

When adding new features or commands, **always update all documentation**:
1. `.claude/skills/dex/SKILL.md` - Full command reference for AI agents
2. `CLAUDE.md` - Project structure and dev info
3. `README.md` - Keep examples current

### Commit Conventions

Use semantic commits with a title and body:

```
feat: Add deployment status to k8s pod show

Display deployment owner and replica status when viewing pod details.
Shows which deployment/replicaset owns the pod.

Refs: DEV-123
```

**Prefixes:**
- `feat:` - New feature
- `fix:` - Bug fix
- `refactor:` - Code restructuring
- `docs:` - Documentation only
- `chore:` - Build, deps, tooling

**Format:**
- Title: imperative mood, lowercase after prefix
- Body: explain what and why (not how)
- `Refs:` tagline for ticket numbers (not in title)
