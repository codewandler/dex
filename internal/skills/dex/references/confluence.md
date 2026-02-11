# Confluence Reference

## Authentication

```bash
dex confluence auth                    # Opens browser for OAuth flow
```

Requires `CONFLUENCE_CLIENT_ID` and `CONFLUENCE_CLIENT_SECRET` (env vars or `~/.dex/config.json`).

Uses the same Atlassian OAuth 2.0 infrastructure as Jira but with separate tokens and Confluence-specific scopes.

## Commands

### List Spaces

```bash
dex confluence spaces                  # List all accessible spaces
dex confluence spaces -l 50            # Limit results
```

**Output columns:** KEY, NAME, TYPE

### Search

```bash
dex confluence search "deployment guide"           # Plain text (auto-wrapped as CQL)
dex confluence search "type = page AND space = DEV" # Raw CQL query
dex confluence search "label = architecture"        # Search by label
dex confluence search "query" -l 10                 # Limit results
```

Plain text queries (without `=` or `~`) are automatically wrapped as `text ~ "query"`.

**Output:** ID, type, title, last modified, URL

### View Page

```bash
dex confluence page <id>               # View page content (HTML stripped to plain text)
```

Shows title, version, URL, and page body content.

## Aliases

`dex cf` is equivalent to `dex confluence`.

## Configuration

```json
{
  "confluence": {
    "client_id": "your-oauth-client-id",
    "client_secret": "your-oauth-client-secret",
    "token": { ... }
  }
}
```

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `CONFLUENCE_CLIENT_ID` | OAuth 2.0 client ID |
| `CONFLUENCE_CLIENT_SECRET` | OAuth 2.0 client secret |
