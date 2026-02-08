# Slack Commands (`dex slack`)

## Authentication
```bash
dex slack auth                    # Authenticate via OAuth (opens browser)
dex slack test                    # Test current authentication
```

OAuth requires `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` configured.
Callback URL: `https://localhost:8089/callback`

## Identity Info
```bash
dex slack info                    # Show who you are (bot and user perspectives)
```

Shows authenticated identities for both tokens:
- **Bot token**: Used for sending messages, reading channels, listing users
- **User token**: Used for search API, mentions search

## Presence
```bash
dex slack presence                # Show current presence (requires users:read scope)
dex slack presence set auto       # Set to auto (online when active)
dex slack presence set away       # Set to away
```

Requires user token. Scopes: `users:read` (view), `users:write` (set).

## Index (Channels & Users)
```bash
dex slack index                   # Index channels and users (cached 24h)
dex slack index --force           # Force re-index
```

Index stored at `~/.dex/slack/index.json`. Required for channel/user name autocomplete and @username DMs.

## List Channels & Users
```bash
dex slack channels                    # List all indexed channels
dex slack channels --member           # Only channels bot is a member of
dex slack channels --user timo.friedl # Channels a user belongs to
dex slack channels --no-cache         # Fetch from API instead of index
dex slack users                   # List all indexed users
dex slack users john              # Search users matching "john"
dex slack users --channel dev-team    # Members of #dev-team
dex slack users john --channel dev    # Search within channel members
dex slack users --no-cache        # Fetch from API instead of index
```

## Channel Members
```bash
dex slack channel members dev-team    # List all members of #dev-team
dex slack channel members general     # List all members of #general
```

Shows member user IDs resolved to usernames from the index.
Requires `dex slack index` to have been run (member data is fetched for public non-archived channels).

## Send Message
```bash
# To channel (by name or ID)
dex slack send dev-team "Hello from dex!"
dex slack send C0123456789 "Hello!"

# With @mentions and #channel mentions (auto-resolved to Slack mentions)
dex slack send dev-team "Hey @john.doe check this!"
dex slack send dev-team "@alice @bob please review"
dex slack send dev-team "Check out #general for updates"
dex slack send dev-team "See discussion in #dev-team and ask @john.doe"

# Reply to thread (use -t with thread timestamp from previous send)
dex slack send dev-team "Follow up" -t 1770257991.873399
dex slack send dev-team "Another reply" --thread 1770257991.873399

# To user DM (requires im:write scope)
dex slack send @john.doe "Hey, check this out!"

# Send as user instead of bot (requires user token with chat:write scope)
dex slack send dev-team "Message from me" --as user
```

Notes:
- Channel names and @usernames autocomplete from index
- @mentions and #channel mentions in message body are auto-converted to Slack format
  - `@username` → `<@USER_ID>`
  - `#channel` → `<#CHANNEL_ID>`
- Use `-t <ts>` to continue a thread (ts returned from previous send)
- Use `--as user` to send as yourself instead of the bot

**Important:** When mentioning users or channels, always use the exact name from `dex slack users` or `dex slack channels`:
```bash
dex slack users john              # → john.doe
dex slack channels | grep dev     # → dev-team
dex slack send dev-team "Hey @john.doe check #general"
```
Partial names like `@john` or `#dev` won't resolve - use the full handle like `@john.doe` and exact channel name like `#dev-team`.

## Search Mentions
```bash
dex slack mentions                    # My mentions today (requires user token)
dex slack mentions --unhandled        # Only pending mentions (no reaction/reply from you)
dex slack mentions --bot              # Bot mentions today
dex slack mentions --user john.doe # Mentions of a specific user
dex slack mentions --user U0123456789 # By user ID
dex slack mentions --since 1h         # Mentions from last hour
dex slack mentions --since 7d         # Mentions from last 7 days
dex slack mentions --limit 50         # Show more results (default 20)
dex slack mentions --compact          # Compact table view
```

**Default behavior:**
- No flags: searches for mentions of the authenticated user (from user token)
- `--bot`: searches for mentions of the bot
- `--user <name>`: searches for mentions of a specific user
- `--unhandled`: filters to show only pending mentions

**Status categories:**
- `Pending` - No reaction or reply from you
- `Acked` - You reacted but didn't reply
- `Replied` - You replied in the thread

**Default time range:** Today (since midnight). Use `--since` to override.

## Search Messages
```bash
# General search (requires user token)
dex slack search "query"              # Search all messages
dex slack search "deployment"         # Find deployment-related messages
dex slack search "error" --since 1d   # Errors in last day
dex slack search "from:@john.doe"  # Messages from specific user
dex slack search "in:#dev-team"       # Messages in specific channel

# Extract Jira tickets from results
dex slack search "bug" --tickets              # Find tickets mentioned with "bug"
dex slack search "DEV-" --tickets             # Find all DEV tickets mentioned
dex slack search "TEL-" --tickets --since 7d  # TEL tickets from last week
dex slack search "urgent" --tickets --compact # Compact ticket list

# Output control
dex slack search "query" --limit 50   # More results (default 50)
dex slack search "query" --compact    # Compact table view
```

**Ticket extraction (`--tickets`):**
- Fetches Jira project keys to identify valid ticket patterns
- Extracts tickets like DEV-123, TEL-456 from message text
- Groups by ticket and shows mention count + permalinks

**Slack search syntax:**
- `from:@username` - Messages from a specific user
- `in:#channel` - Messages in a specific channel
- `has:link` - Messages containing links
- `before:YYYY-MM-DD`, `after:YYYY-MM-DD` - Date filters

## View Thread
```bash
# Fetch and display a thread with mention classification debug info
dex slack thread <url>                              # From Slack URL
dex slack thread <channel>:<ts>                     # Channel:timestamp format
dex slack thread <channel> <ts>                     # Separate arguments

# Examples
dex slack thread https://acme.slack.com/archives/C0123456789/p1769777574026209
dex slack thread C0123456789:1769777574.026209
dex slack thread C0123456789 1769777574.026209
dex slack thread C0123456789 p1769777574026209      # URL-style timestamp also works
```

Timestamps can be in Slack URL format (`p1769777574026209`) or API format (`1769777574.026209`).
