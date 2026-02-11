# dex - the engineer's CLI

## Overview

Swiss army knife CLI for engineers. Usable standalone but primarily designed as a tool for AI agents via the Claude skill at `internal/skills/dex/SKILL.md`.

**Integrations:**
- **Kubernetes** - Cluster management (contexts, namespaces, pods, services, logs)
- **GitLab** - Activity tracking, MRs, commits, project management
- **GitHub** - Repository operations via gh CLI (avoids API rate limits)
- **Jira** - Issue management (OAuth)
- **Confluence** - Wiki search and page viewing (OAuth)
- **Slack** - Messaging (send, reply, channel index)
- **Prometheus** - PromQL queries, scrape targets, alerts

## Project Structure

```
dex/
├── dex.go                         # Entry point
├── internal/
│   ├── cli/                       # Cobra CLI commands
│   │   ├── root.go                # Root command
│   │   ├── gh.go                  # GitHub commands (gh/github)
│   │   ├── gitlab.go              # GitLab commands (gl/gitlab)
│   │   ├── confluence.go           # Confluence commands (cf/confluence)
│   │   ├── jira.go                # Jira commands
│   │   ├── k8s.go                 # Kubernetes commands
│   │   ├── prometheus.go          # Prometheus commands (prom/prometheus)
│   │   └── slack.go               # Slack commands
│   ├── gh/                        # GitHub CLI wrapper
│   │   └── client.go              # gh CLI wrapper for clone operations
│   ├── atlassian/                 # Shared Atlassian OAuth (Jira, Confluence)
│   ├── config/config.go           # Unified configuration (file + env)
│   ├── confluence/                # Confluence API client
│   ├── gitlab/                    # GitLab API client
│   │   ├── client.go              # Main client
│   │   ├── commits.go             # Commit operations
│   │   ├── mergerequests.go       # MR operations
│   │   ├── projects.go            # Project operations
│   │   ├── index.go               # Local project index
│   │   └── tags.go                # Tag operations
│   ├── jira/                      # Jira OAuth client
│   ├── k8s/                       # Kubernetes client
│   ├── prometheus/                # Prometheus API client
│   ├── slack/                     # Slack API client
│   │   ├── client.go              # Main client
│   │   └── index.go               # Channel index
│   ├── models/                    # Data structures
│   ├── output/                    # Terminal formatting
│   └── skills/dex/                # Claude skill definition
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

## Configuration

Configuration is loaded from `~/.dex/config.json` with environment variable overrides.

**Config file structure:**
```json
{
  "activity_days": 14,
  "gitlab": {
    "url": "https://gitlab.example.com",
    "token": "your-personal-access-token"
  },
  "jira": {
    "client_id": "your-oauth-client-id",
    "client_secret": "your-oauth-client-secret",
    "token": { ... }
  },
  "confluence": {
    "client_id": "your-oauth-client-id",
    "client_secret": "your-oauth-client-secret",
    "token": { ... }
  },
  "slack": {
    "bot_token": "xoxb-...",
    "app_token": "xapp-...",
    "user_token": "xoxp-..."
  }
}
```

**Environment variables (override file values):**

| Variable | Description |
|----------|-------------|
| `GITLAB_URL` | GitLab instance URL |
| `GITLAB_PERSONAL_TOKEN` | Personal access token |
| `JIRA_CLIENT_ID` | Jira OAuth 2.0 client ID |
| `JIRA_CLIENT_SECRET` | Jira OAuth 2.0 client secret |
| `CONFLUENCE_CLIENT_ID` | Confluence OAuth 2.0 client ID |
| `CONFLUENCE_CLIENT_SECRET` | Confluence OAuth 2.0 client secret |
| `SLACK_CLIENT_ID` | Slack OAuth client ID |
| `SLACK_CLIENT_SECRET` | Slack OAuth client secret |
| `SLACK_BOT_TOKEN` | Slack bot token (xoxb-...) |
| `SLACK_APP_TOKEN` | Slack app token for Socket Mode (xapp-...) |
| `SLACK_USER_TOKEN` | Slack user token for search API (xoxp-...) |
| `PROMETHEUS_URL` | Prometheus server URL |
| `ACTIVITY_DAYS` | Default days for activity lookback |

## Command Overview

```bash
dex k8s ...    # Kubernetes (aliases: kube, kubernetes)
dex gl ...     # GitLab (aliases: gitlab)
dex gh ...     # GitHub (aliases: github) - wraps gh CLI
dex jira ...   # Jira
dex cf ...     # Confluence (aliases: confluence)
dex slack ...  # Slack
dex prom ...   # Prometheus (aliases: prometheus)
```

For full command reference, see `internal/skills/dex/SKILL.md`.

## Claude Skill

This CLI ships with a Claude Code skill at `internal/skills/dex/SKILL.md` that documents all commands for AI agent use. The skill enables AI assistants to:
- Query Kubernetes clusters
- Browse GitLab activity and MRs
- Clone GitHub repositories
- Look up Jira issues
- Search and view Confluence pages
- Interact with MRs (comment, react, view diffs)
- Send Slack messages and reply to threads

## Jira OAuth Setup

1. Create OAuth 2.0 app at https://developer.atlassian.com/console/myapps/
2. Add callback URL: `http://localhost:8089/callback`
3. Add Jira API permissions: `read:jira-work`, `read:jira-user`
4. Set `JIRA_CLIENT_ID` and `JIRA_CLIENT_SECRET` (env or config file)
5. Run `dex jira auth`

Token stored in `~/.dex/config.json` under `jira.token`.

## Confluence OAuth Setup

1. Use the same OAuth 2.0 app at https://developer.atlassian.com/console/myapps/ (or create a separate one)
2. Add callback URL: `http://localhost:8089/callback`
3. Add Confluence API permissions: `read:confluence-content.all`, `read:confluence-space.summary`, `search:confluence`
4. Set `CONFLUENCE_CLIENT_ID` and `CONFLUENCE_CLIENT_SECRET` (env or config file)
5. Run `dex confluence auth`

Token stored in `~/.dex/config.json` under `confluence.token`.

## Slack OAuth Setup

1. Create Slack app at https://api.slack.com/apps
2. Add OAuth redirect URL: `https://localhost:8089/callback`
3. Add Bot Token Scopes: `channels:history`, `channels:read`, `chat:write`, `groups:history`, `groups:read`, `im:history`, `im:read`, `im:write`, `mpim:history`, `mpim:read`, `users:read`
4. Add User Token Scopes: `search:read`, `users:read`, `users:write`, `chat:write`
5. Set `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` (env or config file)
6. Run `dex slack auth`

Tokens stored in `~/.dex/config.json` under `slack.token`, `slack.bot_token`, and `slack.user_token`.

## Development

### Slack Usage

**Always use `#dex-releases` channel** for testing Slack send/reply functionality. Never send test messages to other channels to avoid disrupting co-workers.

**Always @mention people by name** when the user asks to inform or notify someone. Look up their Slack handle (e.g., `@john.doe`) and include it in the message so they get a notification. Never post about someone without tagging them when the intent is to let them know.

### No Confidential Data in Source

**NEVER use real internal project names, paths, or IDs in source code, help text, or examples.** This is a public repository. Always use generic placeholders like `my-group/my-project!123` or `group/project!456` instead of real company project paths.

### Git Workflow

**NEVER commit without explicit user instruction.** Wait for the user to say "commit" before running `git commit`. Completing a task does not imply permission to commit.

### GitHub Issues

**Do NOT close issues prematurely.** When implementing a feature from a GitHub issue:
- Create the issue before starting work
- Reference the issue in commits (`Refs: #123`)
- Do NOT close the issue when implementation works locally
- Issues are closed only after a release that includes the fix (see Release Process)

### Documentation Sync

When adding new features or commands, **always update all documentation**:
1. `internal/skills/dex/SKILL.md` - Full command reference for AI agents
2. `internal/skills/dex/references/*.md` - Detailed reference docs per integration
3. `CLAUDE.md` - Project structure and dev info
4. `README.md` - Keep examples current

**IMPORTANT:** Only edit skill files in `internal/skills/dex/`. Never edit `~/.claude/skills/dex/` directly - those files are installed from the internal package via `task install`.

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

### Release Process

When the user asks to "release" or "publish" the current changes:

1. **Fetch remote tags**: `git fetch --tags`
2. **Find the latest tag**: `git describe --tags --abbrev=0`
3. **Review changes since last tag**: `git log <last-tag>..HEAD --oneline`
4. **Determine next version** using semver:
   - `feat:` commits → bump minor version (e.g., v0.1.0 → v0.2.0)
   - `fix:` commits only → bump patch version (e.g., v0.1.0 → v0.1.1)
   - Breaking changes → bump major version
5. **Generate changelog** from commits since last tag:
   - Group by type: Features (`feat:`), Fixes (`fix:`), Other
   - Format as markdown with commit links: `- Description ([abc1234](https://github.com/codewandler/dex/commit/abc1234))`
   - Example:
     ```markdown
     ## Features
     - Add GitHub release management ([5fb4a46](https://github.com/codewandler/dex/commit/5fb4a46))

     ## Fixes
     - Fix pod log streaming ([abc1234](https://github.com/codewandler/dex/commit/abc1234))
     ```
6. **Create annotated tag** with summary:
   ```bash
   git tag -a v0.X.Y -m "Release v0.X.Y

   - Feature 1
   - Feature 2
   - Fix 1"
   ```
7. **Push commits and tag**: `git push && git push --tags`
8. **Create GitHub release** with rich changelog:
   ```bash
   dex gh release create v0.X.Y --notes "<markdown changelog from step 5>"
   ```
9. **Announce in `#dex-releases`**: Send a Slack message with:
   - The new version number
   - Link to the GitHub release
   - Brief summary of key changes
   - Install command: `go install github.com/codewandler/dex@latest`
10. **Close resolved issues**: Close any GitHub issues that were fixed in this release with a comment linking to the release (e.g., "Released in v0.19.0")
