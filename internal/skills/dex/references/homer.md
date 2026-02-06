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
dex homer search                              # Last 1h (default)
dex homer search --from 1h                    # Last hour
dex homer search --from 2h --caller "+4930..."  # By caller
dex homer search --from 30m --callee "100"    # By callee
dex homer search --call-id "abc123@host"      # By Call-ID
dex homer search --from 1d --limit 100        # Custom limit
dex homer search --from 2h --to 1h            # Between 2h and 1h ago
```

### Search Flags
- `--from` - Time range start as duration ago (default: `1h`)
- `--to` - Time range end as duration ago (default: now)
- `--caller` - Source number/URI filter
- `--callee` - Destination number/URI filter
- `--call-id` - SIP Call-ID filter
- `-l, --limit` - Maximum results (default: 50)

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

- Default time range is 1 hour (`--from 1h`)
- Default result limit is 50 (`--limit 50`)
- Duration supports: `30m`, `1h`, `2h30m`, `1d`, `7d`
- Use `dex homer discover` to verify connectivity before searching
- PCAP files can be opened directly in Wireshark
