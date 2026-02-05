# TODO

## New Integrations

- [ ] **Loki** - Log aggregation queries
- [ ] **Grafana** - Dashboard and alerting integration
- [ ] **OpenRouter** - AI model support with REPL

## New Features

- [ ] `claude` command - Show info about Claude from settings file
- [ ] `config show` - Display current configuration
- [ ] `jira my --status` - Filter issues by status
- [ ] Todo list management

## Improvements

- [x] Move GitLab index from `~/.config/dex/gitlab-index.json` to `~/.dex/gitlab/index.json`
- [ ] Slack mentions: parallel classification with goroutine worker pool for faster uncached lookups

## GitLab MR Diff Tools (from wishlist)

These help debug inline comment issues by inspecting diff contents before posting.

- [ ] `mr diff --line <n>` - Inspect specific line with context (show type, content, surrounding lines)
- [ ] `mr diff --search <pattern>` - Find lines by content (regex support)
- [ ] `mr comment --dry-run` - Preview where inline comment will land, warn on empty/invalid lines
- [ ] Better inline comment errors - When 400 fails, explain why (line not in diff) and suggest fix

## Use Case Ideas

- "Find all Slack mentions from today, create a todo list"
