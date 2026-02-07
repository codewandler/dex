# Homer Commands (`dex homer`)

SIP call tracing via Homer 7.x. Search and inspect SIP traffic, view message flows, and export PCAPs.

Alias: `dex sip`

## Discover Homer in Kubernetes
```bash
dex homer discover                # Find homer-webapp in current namespace
dex homer discover -n eu          # Search in specific namespace
```

Looks for service `homer-webapp` in the specified namespace (or current K8s namespace), tests connectivity, and verifies authentication.

## Configuration

Set `HOMER_URL` environment variable or add to `~/.dex/config.json`:
```json
{
  "homer": {
    "url": "http://homer-webapp.eu.svc.cluster.local:80"
  }
}
```

Alternatively, use the `--url` flag on any command.

**Auto-discovery:** If no URL is configured, commands will automatically look for service `homer-webapp` in the current K8s namespace.

### Credentials

Credentials are resolved in this order:
1. **Endpoint-specific config** in `~/.dex/config.json`:
   ```json
   {
     "homer": {
       "endpoints": {
         "http://homer-webapp.eu.svc.cluster.local:80": {
           "username": "admin",
           "password": "secret"
         }
       }
     }
   }
   ```
2. **Environment variables:** `HOMER_USERNAME` / `HOMER_PASSWORD`
3. **Global config:** `homer.username` / `homer.password` in config file
4. **Default:** `admin` / `admin` (Homer default)

## Search Calls
```bash
dex homer search --number "4921514174858"                       # Search by number (from_user and to_user, with and without + prefix)
dex homer search --number "4921514174858" --since 2h            # Narrow time range
dex homer search --from-user "999%" --to-user "12345"     # Filter by caller and callee
dex homer search --from-user "999%" --ua "Asterisk%"      # Combine with user agent filter
dex homer search --since 30m --limit 100                  # All calls, last 30m
dex homer search --call-id "abc123@host"                  # By Call-ID
dex homer search --at "2026-02-04 17:13"                  # ±5 minutes around timestamp
dex homer search --since "2026-02-04 10:00" --until "2026-02-04 12:00"  # Absolute range
dex homer search -q "from_user = '123' AND status = 200"  # Query with clean field names
dex homer search --number "4921514174858" -d                    # Debug: show API request
```

### Search Flags

All filter flags (`--number`, `--from-user`, `--to-user`, `--ua`, `-q`) are combined as AND conditions into a single smart input query sent to Homer.

- `--number` - Phone number (searches from_user and to_user with and without `+` prefix)
- `--from-user` - Filter by SIP from_user
- `--to-user` - Filter by SIP to_user
- `--ua` - Filter by SIP User-Agent
- `-q, --query` - Query expression with field validation (see Smart Input below)
- `--since` - Start of time range: duration (e.g., `1h`, `30m`, `2d`) or timestamp (e.g., `2006-01-02 15:04`) (default: `24h`)
- `--until` - End of time range: duration or timestamp (default: now)
- `--at` - Point in time to search around (±5 minutes). Mutually exclusive with `--since`/`--until`
- `--call-id` - SIP Call-ID filter
- `-l, --limit` - Maximum results (default: 200)
- `-d, --debug` - Print API endpoint and request body

### Smart Input

All filter flags are internally translated to a Homer smart input expression. You can also use `-q`/`--query` to write custom expressions with field validation.

**Syntax:** `field operator 'value' [AND|OR field operator 'value' ...]`

Parentheses are supported for grouping: `from_user = '999%' AND (to_user = '123' OR to_user = '456')`

**Available fields:**

| Field | Alias | Description |
|-------|-------|-------------|
| `from_user` | | SIP From user |
| `to_user` | | SIP To user |
| `ruri_user` | | SIP Request-URI user |
| `user_agent` | `ua` | SIP User-Agent |
| `cseq` | | SIP CSeq |
| `method` | | SIP method (INVITE, BYE, etc.) |
| `status` | | SIP response status code |
| `call_id` | `sid` | SIP Call-ID / session ID |

Unknown field names produce an error — no need to guess Homer's internal `data_header.` prefixes, the parser maps them automatically.

**Operators:** `=`, `!=` (use `%` as wildcard with `=`, Homer auto-converts to LIKE)

**Examples:**
```bash
# Convenience flags (combined as AND)
dex homer search --from-user "999%" --to-user "12345" --ua "Asterisk%"

# Query expression with clean field names
dex homer search -q "from_user = '49215%' AND status = 200"
dex homer search -q "method = 'INVITE' AND status != 200"

# Grouping with parentheses
dex homer search -q "from_user = '999%' AND (to_user = '123' OR to_user = '456')"

# Mix query with convenience flags
dex homer search -q "status = 200" --from-user "999%" --since 2h
```

## List Calls
```bash
dex homer calls                                       # All calls in last 24h
dex homer calls --since 2h                             # All calls in last 2h
dex homer calls --number "31617554360" --since 1h      # Calls to number
dex homer calls --from-user "999%" --since 1h          # Filter by caller
dex homer calls --ua "FPBX%" --since 30m               # Filter by user agent
dex homer calls -q "ua = 'Asterisk%'" --since 1h       # Custom query
dex homer calls --at "2026-02-04 17:13"                # ±5 minutes around timestamp
dex homer calls --since "2026-02-04 10:00" --until "2026-02-04 12:00"  # Absolute range
dex homer calls --since 1h -o json                     # JSON output
dex homer calls --number "31617554360" --since 30m --limit 50
```

Groups SIP messages by Call-ID and shows a call-level summary with direction and status.
Supports the same filter flags and time range options as `search`.

### Calls Flags
- `--number` - Phone number to search (matches from_user and to_user with and without `+` prefix)
- `--from-user` - Filter by SIP from_user
- `--to-user` - Filter by SIP to_user
- `--ua` - Filter by SIP User-Agent
- `-q, --query` - Query expression with field validation (see Smart Input)
- `--since` - Start of time range: duration (e.g., `1h`, `30m`, `2d`) or timestamp (default: `24h`)
- `--until` - End of time range: duration or timestamp (default: now)
- `--at` - Point in time to search around (±5 minutes). Mutually exclusive with `--since`/`--until`
- `-l, --limit` - Maximum results per API call (default: 100)
- `-o, --output` - Output format: `json` or `jsonl`

### Call Status Values
- **answered** - 200 OK received
- **busy** - 486 Busy Here
- **cancelled** - 487 Request Terminated
- **no answer** - 408 Request Timeout / 480 Temporarily Unavailable
- **failed** - Other 4xx-6xx responses
- **ringing** - Only 1xx provisional responses seen

### Workflow: Find a Specific Call
1. Search by number: `dex homer calls --number "123" --since 2h`
2. Find the call in the output, copy the Call-ID
3. Inspect message flow: `dex homer show <call-id>`
4. Export for Wireshark: `dex homer export <call-id>`

## Show Call Message Flow
```bash
dex homer show <call-id>                      # Show SIP message ladder
dex homer show <call-id> --from 2h            # Expand time range
```

Displays the full SIP message flow (INVITE, 100 Trying, 180 Ringing, 200 OK, ACK, BYE, etc.) with source/destination IPs, ports, and timestamps.

## Export PCAP
```bash
dex homer export <call-id>                    # Export to <call-id>.pcap
dex homer export <call-id> -o trace.pcap      # Custom output file
dex homer export <call-id> --from 2h          # Expand time range
```

Exports SIP messages as a PCAP file for analysis in Wireshark or similar tools.

## List Configured Endpoints
```bash
dex homer endpoints                           # List all configured endpoints with URLs
```

Shows all Homer endpoints from `~/.dex/config.json`, including the default URL and any endpoint-specific credential overrides.

## List Aliases
```bash
dex homer aliases                             # List all IP/port aliases
```

Shows IP-to-name mappings configured in Homer for readable SIP trace display.

## Global Flags

These flags are available on all Homer subcommands:
- `--url` - Homer URL (overrides config/env/discovery)
- `-n, --namespace` - K8s namespace for service discovery

## Tips

- **Search results are session-based, not message-based.** Filters match individual SIP messages, but results include all messages belonging to matched Call-IDs. This means you may see messages where field values differ from your filter — these are other legs of the same call session (e.g., a PBX rewriting headers on a downstream INVITE).
- Default time range is 24 hours (`--since 24h`)
- Default result limit is 200 (`--limit 200`)
- Time values support durations (`30m`, `1h`, `2h30m`, `1d`, `7d`) and absolute timestamps (`2006-01-02 15:04`)
- Use `--at` for quick lookups around a known time
- Use `dex homer discover` to verify connectivity before searching
- PCAP files can be opened directly in Wireshark
