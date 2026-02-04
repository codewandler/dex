# dex

Engineer's CLI for Kubernetes, GitLab, and Jira.

## For Humans

```bash
# Install
task install

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
- Environment variables for each integration (see `CLAUDE.md`)
