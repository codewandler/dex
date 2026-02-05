# SQL Commands (`dex sql`)

## Configuration

Add datasources to `~/.dex/config.json`:

```json
{
  "sql": {
    "datasources": {
      "eu:read": {
        "host": "db.example.com",
        "port": 3306,
        "username": "readonly",
        "password": "secret",
        "database": "mydb"
      }
    }
  }
}
```

Port defaults to 3306 (MySQL) if not specified.

## List Datasources
```bash
dex sql datasources               # List configured datasource names
```

## Query
```bash
dex sql query -d <DATASOURCE> "<QUERY>"
```

### Examples
```bash
# Simple query
dex sql query -d eu:read "SELECT * FROM users LIMIT 10"

# Count query
dex sql query -d eu:read "SELECT COUNT(*) FROM orders WHERE created_at > '2024-01-01'"

# Show tables
dex sql query -d eu:read "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() LIMIT 10"

# Describe table
dex sql query -d eu:read "DESCRIBE users"
```

### Output Format
Results are displayed in a formatted table with:
- Column headers
- Aligned columns (max 50 chars per column)
- Row count at the end

### Timeout
Queries have a 60-second timeout by default.

### Read-Only Mode
Connections are read-only by default (`SET SESSION TRANSACTION READ ONLY`). This prevents accidental data modification.
