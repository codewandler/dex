# Homer Commands (`dex homer`)

SIP call tracing via Homer 7.x. Search and inspect SIP traffic, view message flows, export PCAPs, and analyze multi-leg calls.

Alias: `dex sip`

## Configuration & Credentials

Pass the Homer URL via `--url` on any command. Credentials are resolved automatically from `~/.dex/config.json` by matching the URL against configured endpoints:

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

**If the Homer URL is already known** (e.g., from prior context or conversation), just use `--url` directly — the CLI matches it against configured endpoints to retrieve credentials. No discover step needed.

**Credential resolution order:** endpoint-specific config → `HOMER_USERNAME`/`HOMER_PASSWORD` env vars → global `homer.username`/`homer.password` in config → default `admin`/`admin`.

**Auto-discovery fallback:** If no URL is provided via `--url`, config, or env var, commands automatically attempt K8s service discovery for `homer-webapp`.

## Discover Homer in Kubernetes
```bash
dex homer discover                # Find homer-webapp in current namespace
dex homer discover -n eu          # Search in specific namespace
```

Diagnostic command that tests connectivity and authentication. Only needed for troubleshooting — not a prerequisite for other commands.

## Search Calls
```bash
dex homer search --number "4921514174858"                  # Search by number (from_user and to_user, ± prefix)
dex homer search --number "4921514174858" --since 2h       # Narrow time range
dex homer search --from-user "999%" --to-user "12345"      # Filter by caller and callee
dex homer search --from-user "999%" --ua "Asterisk%"       # Combine with user agent filter
dex homer search --since 30m --limit 100                   # All calls, last 30m
dex homer search --call-id "abc123@host"                   # By Call-ID
dex homer search --at "2026-02-04 17:13"                   # ±5 minutes around timestamp
dex homer search --since "2026-02-04 10:00" --until "2026-02-04 12:00"  # Absolute range
dex homer search -q "from_user = '123' AND status = 200"   # Query with clean field names
dex homer search --number "123" -m INVITE -m BYE           # Filter by SIP method
dex homer search --number "123" -o json                    # JSON output
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
- `-m, --method` - Client-side SIP method filter (repeatable, e.g. `-m INVITE -m BYE`)
- `-l, --limit` - Maximum results (default: 200)
- `-o, --output` - Output format: `json` or `jsonl`

### Smart Input

All filter flags are internally translated to a Homer smart input expression. You can also use `-q`/`--query` to write custom expressions with field validation.

**Syntax:** `field operator 'value' [AND|OR field operator 'value' ...]`

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

**Parentheses:** The `-q` parser accepts parentheses for grouping (e.g., `from_user = '999%' AND (to_user = '123' OR to_user = '456')`), but Homer's smart input has known issues with complex nested expressions. Convenience flags (`--number`, `--from-user`, etc.) use a cartesian product approach that avoids this limitation. Prefer convenience flags when combining multiple OR-alternatives.

**Examples:**
```bash
dex homer search --from-user "999%" --to-user "12345" --ua "Asterisk%"
dex homer search -q "from_user = '49215%' AND status = 200"
dex homer search -q "method = 'INVITE' AND status != 200"
dex homer search -q "status = 200" --from-user "999%" --since 2h
```

## List Calls
```bash
dex homer calls --since 2h                             # All calls in last 2h
dex homer calls --number "31617554360" --since 1h      # Calls to number
dex homer calls --from-user "999%" --since 1h          # Filter by caller
dex homer calls --ua "FPBX%" --since 30m               # Filter by user agent
dex homer calls -q "ua = 'Asterisk%'" --since 1h       # Custom query
dex homer calls --at "2026-02-04 17:13"                # ±5 minutes around timestamp
dex homer calls --since 1h -o json                     # JSON output
```

Groups SIP messages by Call-ID and shows a call-level summary with direction and status.
Same filter flags as `search`, plus:
- `-l, --limit` - Maximum calls to return (default: 100)
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
dex homer show id1@host id2@host id3@host     # Combined flow for multiple calls
dex homer show <call-id> --raw                # Display raw SIP message bodies (headers + SDP)
dex homer show <call-id> --from 2h            # Expand time range
```

Displays the full SIP message flow (INVITE, 100 Trying, 180 Ringing, 200 OK, ACK, BYE, etc.) with source/destination IPs, ports, and timestamps. Multiple Call-IDs produce a merged, time-sorted flow.

### Show Flags
- `--from` - Time range start as duration (default: `10d`)
- `--to` - Time range end as duration (default: now)
- `--raw` - Display full raw SIP message bodies

## Export PCAP
```bash
dex homer export <call-id>                    # Export to <call-id>.pcap
dex homer export <call-id> -o trace.pcap      # Custom output file
dex homer export <call-id> --from 2h          # Expand time range
```

Exports SIP messages as a PCAP file for analysis in Wireshark or similar tools.

### Export Flags
- `--from` - Time range start as duration (default: `10d`)
- `--to` - Time range end as duration (default: now)
- `-o, --output` - Output file path (default: `<call-id>.pcap`)

## Call Quality / QoS
```bash
dex homer qos <call-id>                           # Show RTCP quality metrics
dex homer qos id1@host id2@host                   # Multiple calls
dex homer qos <call-id> --clock 16000             # Wideband codec (16 kHz)
dex homer qos <call-id> --latency 50              # Override assumed latency
dex homer qos <call-id> -o json                   # JSON output
dex homer qos <call-id> --from 2h                 # Expand time range
```

Fetches RTCP sender/receiver reports from Homer and computes per-stream quality metrics: packet loss, jitter (average and maximum), and an estimated MOS score.

### QoS Flags
- `--from` - Time range start as duration (default: `10d`)
- `--to` - Time range end as duration (default: now)
- `--clock` - RTP clock rate in Hz for jitter conversion (default: `8000` for G.711)
- `--latency` - Assumed one-way network latency in ms for MOS calculation (default: `20`)
- `-o, --output` - Output format: `json` or `jsonl`

### MOS Scale
MOS (Mean Opinion Score) is estimated using the E-model approximation:

| MOS | Quality |
|-----|---------|
| 4.3+ | Excellent |
| 3.6 - 4.3 | Good |
| 2.6 - 3.6 | Fair |
| < 2.6 | Poor |

MOS is not returned by Homer — it is computed client-side from latency, jitter, and packet loss. The `--latency` flag controls the assumed one-way network delay (default 20 ms). The `--clock` flag sets the RTP clock rate used to convert RTCP jitter values from timestamp units to milliseconds (default 8000 Hz for G.711 narrowband; use 16000 for wideband codecs like G.722).

## Analyze Call (Multi-Leg Correlation)
```bash
dex homer analyze <call-id> -c X-Acme-Call-ID                    # Correlate legs by header
dex homer analyze <call-id> -c X-Acme-Call-ID -H X-Acme    # Show matching headers as columns
dex homer analyze <call-id> -c X-Acme-Call-ID -N 4934155003500   # Include extra number in fan-out
dex homer analyze --from-user 4921514174858 --to-user 4934155003500 \
  --at "2026-02-04 17:13" -c X-Acme-Call-ID                      # Seed by caller/callee pair
```

Traces all SIP legs belonging to the same logical call by correlating a shared header value across Call-IDs. Produces a leg overview table and a ladder diagram showing the full message flow across all endpoints.

**How it works:**
1. Finds the seed call (by Call-ID or --from-user/--to-user pair)
2. Fans out by caller number (+ extra `-N` numbers) in a time window around the seed
3. Extracts correlation header values from INVITE messages
4. Filters candidates that share the same header value and overlap temporally
5. Renders leg table + ladder diagram

### Analyze Flags

Entry point (one required):
- Positional `<call-id>` - Seed by SIP Call-ID
- `--from-user` + `--to-user` - Seed by caller/callee pair

Required:
- `-c, --correlate` - SIP header to correlate legs by (exact match, repeatable)

Optional:
- `-H, --header` - SIP header prefix to show as extra table columns (prefix match, repeatable)
- `-N, --number` - Extra number to include in fan-out search (repeatable, e.g. agent extensions)
- `--since` - Time range start (default: `10d`)
- `--until` - Time range end (default: now)
- `--at` - Point in time ±5 min (mutually exclusive with `--since`/`--until`)
- `-l, --limit` - Max calls per search (default: 100)
- `-o, --output` - Output format: `json` or `jsonl`

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
- `-d, --debug` - Print API endpoint and request body

## Tips

- **Search results are session-based, not message-based.** Filters match individual SIP messages, but results include all messages belonging to matched Call-IDs. This means you may see messages where field values differ from your filter — these are other legs of the same call session (e.g., a PBX rewriting headers on a downstream INVITE).
- Default time range is 24 hours (`--since 24h`) for search/calls, 10 days for show/export/analyze
- Time values support durations (`30m`, `1h`, `2h30m`, `1d`, `7d`) and absolute timestamps (`2006-01-02 15:04`)
- Use `--at` for quick lookups around a known time
- Use `dex homer discover` only to troubleshoot connectivity — not needed before normal commands
- PCAP files can be opened directly in Wireshark
