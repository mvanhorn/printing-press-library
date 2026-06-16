# PostgreSQL Admin CLI

Introspect PostgreSQL schemas, manage roles and permissions, monitor replication status, execute safe queries, manage database migrations, and backup/restore databases. A comprehensive CLI for PostgreSQL administration without requiring `psql` or extensive SQL knowledge.

Created by [@kieran](https://github.com/kieran) (Kieran).

## What It Does

This CLI provides a structured interface to PostgreSQL administration tasks:

- **Schema Introspection** — Explore tables, views, indexes, columns, constraints, sequences, and functions across all schemas
- **Role Management** — Create roles, grant/revoke permissions on schemas and tables, manage role membership
- **Replication Monitoring** — Check primary-replica topology, measure lag, monitor replication slots and WAL retention
- **Safe Query Execution** — Run SELECT queries and EXPLAIN ANALYZE with guardrails against accidental mutations
- **Migration Management** — List, apply, rollback database migrations with checksum validation and concurrent safety
- **Backup & Restore** — Create backups in multiple formats, verify integrity, and restore to new databases

All commands are **read-only by default** except for migrations and backup/restore operations, which come with explicit confirmations and dry-run mode.

## Install

The recommended path installs both the `postgresql-admin-pp-cli` binary and the `pp-postgresql-admin` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin --cli-only
```

For skill only (installs into Claude Code, Cursor, Codex, Gemini CLI, and other supported agents):

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin --skill-only
```

To constrain the skill install to specific agents:

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install postgresql-admin --agent claude-code --agent cursor
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline, etc.), install directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/postgresql-admin/cmd/postgresql-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Quick Start

### 1. Verify Installation

```bash
postgresql-admin-pp-cli --version
```

### 2. Test Connectivity

Before running any command, validate that your PostgreSQL connection is configured:

```bash
postgresql-admin-pp-cli doctor
```

This checks:
- Network connectivity to the PostgreSQL host
- Authentication (username/password)
- User permissions (required for introspection and admin operations)
- Replication configuration (if applicable)

### 3. Explore Your Schema

```bash
# List all schemas
postgresql-admin-pp-cli schema list

# List objects in public schema
postgresql-admin-pp-cli schema describe public

# Inspect a specific table
postgresql-admin-pp-cli schema inspect public.users
```

### 4. Query Safe Data

```bash
# Run a SELECT query
postgresql-admin-pp-cli query execute "SELECT id, name, email FROM users LIMIT 10"

# Output as JSON
postgresql-admin-pp-cli query execute "SELECT COUNT(*) as count FROM orders" --json

# Parameterized query (prevents SQL injection)
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE email = ?" --params "user@example.com"
```

## Authentication

PostgreSQL credentials are configured via **environment variables** (or config file) with env vars taking priority:

| Variable | Purpose | Example |
|----------|---------|---------|
| `POSTGRESQL_HOST` | Database host | localhost, db.example.com |
| `POSTGRESQL_PORT` | Port number | 5432 |
| `POSTGRESQL_USER` | Database user | postgres, admin |
| `POSTGRESQL_PASSWORD` | Password | (never commit to version control) |
| `POSTGRESQL_DBNAME` | Database name | postgres, myapp_db |
| `POSTGRESQL_SSLMODE` | SSL mode | disable, allow, prefer (default), require |

**Example:**
```bash
export POSTGRESQL_HOST=db.prod.example.com
export POSTGRESQL_PORT=5432
export POSTGRESQL_USER=admin
export POSTGRESQL_PASSWORD=secure_password_here
export POSTGRESQL_DBNAME=production

postgresql-admin-pp-cli schema list
```

**Config File Option (lower priority):**

Create `~/.config/postgresql-admin-pp-cli/config.toml`:

```toml
host = "db.example.com"
port = 5432
user = "admin"
password = "secret"
dbname = "production"
sslmode = "require"
```

Use the config file only for non-production credentials. For production, always use environment variables or a secure credential store.

## Common Examples

### Introspect a Production Database

```bash
# List all user-created schemas
postgresql-admin-pp-cli schema list --json | jq '.[] | select(.Type == "user")'

# Describe the public schema
postgresql-admin-pp-cli schema describe public

# Get full table metadata
postgresql-admin-pp-cli schema inspect public.orders --json | jq '.columns'
```

### Create a Read-Only User

```bash
# Create role
postgresql-admin-pp-cli roles create analyst --can-login --password analyst_pwd

# Grant schema usage
postgresql-admin-pp-cli roles grant --role analyst --on public --permissions USAGE

# Grant SELECT on tables
postgresql-admin-pp-cli roles grant --role analyst --on public.users --permissions SELECT
postgresql-admin-pp-cli roles grant --role analyst --on public.orders --permissions SELECT

# Verify permissions
postgresql-admin-pp-cli roles list --json
```

### Monitor Replication Health

```bash
# Check replication status
postgresql-admin-pp-cli replication status

# Watch replication slots
postgresql-admin-pp-cli replication slots

# JSON for monitoring systems
postgresql-admin-pp-cli replication status --json
```

### Apply Database Migrations

```bash
# Place migration files in migrations/ directory:
#   migrations/001_create_users_table.up.sql
#   migrations/001_create_users_table.down.sql
#   migrations/002_add_email_index.up.sql
#   migrations/002_add_email_index.down.sql

# List pending migrations
postgresql-admin-pp-cli migrations list

# Preview what would be applied
postgresql-admin-pp-cli migrations apply --dry-run

# Apply all pending migrations
postgresql-admin-pp-cli migrations apply

# Verify migration state
postgresql-admin-pp-cli migrations verify
```

### Create and Restore Backups

```bash
# Create backup
postgresql-admin-pp-cli backup create --format custom --output /backups/db-$(date +%Y%m%d).backup

# Verify backup
postgresql-admin-pp-cli backup verify /backups/db-20260615.backup

# List available backups
postgresql-admin-pp-cli backup list

# Restore to test database
postgresql-admin-pp-cli backup restore /backups/db-20260615.backup --target-db test_restore

# Restore specific schema only
postgresql-admin-pp-cli backup restore /backups/db-20260615.backup --schema public
```

### Query and Analysis

```bash
# Simple count
postgresql-admin-pp-cli query execute "SELECT COUNT(*) as user_count FROM users"

# EXPLAIN ANALYZE for performance tuning
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM large_table WHERE created_at > NOW() - INTERVAL '1 day'"

# JSON output for parsing
postgresql-admin-pp-cli query execute "SELECT * FROM users ORDER BY created_at DESC LIMIT 5" --json

# Parameterized (safe from SQL injection)
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE user_id = ?" --params "12345"
```

## Agent Mode

For use in scripts and CI/CD pipelines, add `--agent` to any command:

```bash
postgresql-admin-pp-cli schema list --agent
```

This automatically sets:
- `--json` — Structured JSON output
- `--compact` — Only essential fields
- `--no-input` — No interactive prompts
- `--no-color` — No ANSI color codes
- `--yes` — Skip confirmations

Example in a script:

```bash
#!/bin/bash
# Get all tables and process with jq
postgresql-admin-pp-cli schema describe public --agent --select name,type | jq '.[] | select(.type == "table")'
```

## Troubleshooting

### Connection Issues

```bash
# Validate connectivity and auth
postgresql-admin-pp-cli doctor

# Custom host/port
POSTGRESQL_HOST=db.custom.com POSTGRESQL_PORT=5433 postgresql-admin-pp-cli doctor

# With custom config
postgresql-admin-pp-cli doctor --config /etc/postgresql-admin/config.toml
```

### Permission Denied Errors

```bash
# Check current user permissions
postgresql-admin-pp-cli doctor --json

# List current roles
postgresql-admin-pp-cli roles list

# Verify table access
postgresql-admin-pp-cli schema inspect public.users
```

### Slow Queries

```bash
# Use EXPLAIN ANALYZE
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM large_table WHERE condition"

# Check indexes
postgresql-admin-pp-cli schema inspect public.large_table --json | jq '.indexes'
```

### Replication Lag

```bash
# Monitor lag
postgresql-admin-pp-cli replication status

# JSON for alerting
postgresql-admin-pp-cli replication status --json | jq '.[] | select(.Lag_ms > 1000)'
```

## Output Formats

All commands support multiple output formats:

| Flag | Format | Example Use |
|------|--------|------------|
| (default) | Human-readable table | Quick interactive viewing |
| `--json` | JSON array | Processing by agents/scripts |
| `--csv` | CSV with header | Spreadsheet import |
| `--plain` | Tab-separated | Piping to other tools |
| `--quiet` | One value per line | ID extraction for loops |

Example:

```bash
# Default (human-readable)
postgresql-admin-pp-cli roles list

# JSON for parsing
postgresql-admin-pp-cli roles list --json | jq '.[] | .Role'

# CSV for spreadsheet
postgresql-admin-pp-cli roles list --csv > roles.csv

# Quiet for piping
postgresql-admin-pp-cli schema list --quiet | while read schema; do
  postgresql-admin-pp-cli schema describe "$schema"
done
```

## Exit Codes

Scripts can check exit codes to detect errors:

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (bad flags) |
| 3 | Not found (table, role, schema) |
| 4 | Auth error (permission denied) |
| 5 | Server error (query failed) |
| 7 | Timeout |
| 10 | Config error |

Example:

```bash
postgresql-admin-pp-cli schema inspect public.users > /dev/null 2>&1
case $? in
  0) echo "Table found" ;;
  3) echo "Table not found" ;;
  *) echo "Error occurred" ;;
esac
```

## Install for Hermes

Install the CLI binary first:

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin --cli-only
```

Then install the focused Hermes skill:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-postgresql-admin --force
```

Or from within Hermes:

```
/skills install mvanhorn/printing-press-library/cli-skills/pp-postgresql-admin --force
```

Restart Hermes to activate the skill.

## Install for OpenClaw

Install both CLI and OpenClaw skill in one command:

```bash
npx -y @mvanhorn/printing-press-library install postgresql-admin --agent openclaw
```

Restart OpenClaw to activate.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for one-click install in Claude Desktop:

1. Download the `.mcpb` for your platform from [latest release](https://github.com/mvanhorn/printing-press-library/releases)
2. Double-click to open in Claude Desktop
3. Walk through the install wizard

Requires Claude Desktop 1.0.0 or later.

### Manual MCP Setup (Advanced)

If you need to configure Claude Desktop manually:

1. Install the binary: `npx -y @mvanhorn/printing-press-library install postgresql-admin --cli-only`
2. Find the binary path: `which postgresql-admin-pp-mcp`
3. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "postgresql-admin": {
      "command": "/path/to/postgresql-admin-pp-mcp",
      "args": [],
      "env": {
        "POSTGRESQL_HOST": "db.example.com",
        "POSTGRESQL_PORT": "5432",
        "POSTGRESQL_USER": "admin",
        "POSTGRESQL_PASSWORD": "secret",
        "POSTGRESQL_DBNAME": "production"
      }
    }
  }
}
```

4. Restart Claude Desktop

## Commands Overview

| Command | Purpose |
|---------|---------|
| `doctor` | Validate connectivity and permissions |
| `schema list` | List all schemas |
| `schema describe` | List objects in a schema |
| `schema inspect` | Full metadata for a table |
| `roles list` | List all roles |
| `roles create` | Create a new role |
| `roles grant` | Grant permissions |
| `roles delete` | Delete a role |
| `replication status` | Primary-replica topology and lag |
| `replication slots` | List replication slots |
| `migrations list` | List all migrations |
| `migrations apply` | Apply pending migrations |
| `migrations rollback` | Rollback migrations |
| `migrations verify` | Check migration state |
| `query execute` | Run a SELECT query |
| `backup create` | Create a backup |
| `backup list` | List available backups |
| `backup verify` | Check backup integrity |
| `backup restore` | Restore from backup |

For full documentation, see `SKILL.md` and run `postgresql-admin-pp-cli --help`.

## Requirements

- PostgreSQL 11.0 or later
- Go 1.26.3+ (if building from source)
- Network access to PostgreSQL instance

## License

Licensed under Apache-2.0. See LICENSE file.
