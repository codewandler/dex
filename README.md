<div align="center">

<img src="dex-avatar.svg" alt="dex logo" width="128" height="128">

### **dex** — a new developer experience

*Swiss army knife for engineers and AI agents*

</div>

---

## For Humans

```bash
# Install
go install github.com/codewandler/dex@latest

# Get started
dex -h                         # Show help
dex setup                      # Configure integrations
dex doctor                     # Check integration health

# Examples
dex k8s pod ls -A              # List all pods
dex gl mr ls                   # List open MRs
dex jira my                    # My assigned issues
```

## For AI Agents

This CLI is primarily designed as a tool for AI agents (Claude, etc.) to interact with engineering infrastructure. It ships with a Claude Code skill at `.claude/skills/dex/` that provides full command documentation.

Invoke via `/dex` in Claude Code, or see `CLAUDE.md` for development info.

## Requirements

- Go 1.21+
- [Task](https://taskfile.dev)

## Integration Setup

Before running `dex setup`, create the necessary OAuth apps/tokens:

### GitLab

1. Go to your GitLab instance → Settings → Access Tokens
2. Create a Personal Access Token with scopes: `api`, `read_user`
3. Run `dex setup` and enter your GitLab URL and token

### Jira

1. Create an OAuth 2.0 app at https://developer.atlassian.com/console/myapps/
2. Add permissions: `read:jira-work`, `read:jira-user`
3. Add callback URL: **`http://localhost:8089/callback`** (HTTP, not HTTPS)
4. Run `dex setup` and enter your Client ID and Secret

### Slack

1. Create a Slack app at https://api.slack.com/apps
2. Add OAuth redirect URL: **`https://localhost:8089/callback`** (HTTPS, not HTTP)
3. Add Bot Token Scopes: `channels:history`, `channels:read`, `chat:write`, `groups:history`, `groups:read`, `im:history`, `im:read`, `im:write`, `users:read`
4. Add User Token Scopes: `search:read`, `users:read`
5. Run `dex setup` and enter your Client ID and Secret

> **Note:** Jira uses HTTP for the callback, Slack uses HTTPS. This is intentional.
