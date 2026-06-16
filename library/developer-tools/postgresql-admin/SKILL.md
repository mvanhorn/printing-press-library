---
name: pp-postgresql-admin
description: "PostgreSQL Admin CLI. Introspect schemas, manage roles, monitor replication, execute safe queries, apply migrations, and backup/restore databases."
author: "Kieran"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - postgresql-admin-pp-cli
    install:
      - kind: go
        bins: [postgresql-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/postgresql-admin/cmd/postgresql-admin-pp-cli
---

# PostgreSQL Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `postgresql-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install postgresql-admin --cli-only
   ```
2. Verify: `postgresql-admin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/postgresql-admin/cmd/postgresql-admin-pp-cli@latest
```

**Platform Requirements:**
- PostgreSQL 11.0 or later (uses modern system catalogs and replication APIs)
- pgx driver (bundled in the CLI; no external PostgreSQL client library required)
- Network access to PostgreSQL instance (port 5432 by default, or custom port via config)

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need to:

- **Introspect schemas**: Discover and explore tables, columns, indexes, constraints, sequences, views, and functions
- **Manage roles and permissions**: Create roles, grant/revoke permissions on schemas and tables
- **Monitor replication**: Check primary-replica topology, measure lag, monitor replication slots
- **Execute safe queries**: Run SELECT queries and EXPLAIN ANALYZE with mutation guardrails
- **Manage migrations**: List, apply, rollback, and verify database migrations with checksums
- **Backup and restore**: Create and verify backups, restore from backups with selective restore support

## When NOT to Use This CLI

Do not use this CLI for:

- **Interactive SQL shells**: Use `psql` for full SQL access and interactive development
- **Query optimization**: Use `EXPLAIN ANALYZE` directly via the `query execute` command
- **Automatic failover**: This CLI monitors replication state but does not orchestrate failover
- **Multi-cluster management**: This CLI targets one PostgreSQL cluster at a time
- **Bulk data mutations**: Use `psql` or application code for large INSERT/UPDATE/DELETE operations
- **Custom backup formats**: This CLI wraps standard `pg_dump` and `pg_restore`; it does not implement custom compression or cloud storage

## Auth Setup

PostgreSQL authentication requires a database connection. The CLI accepts credentials via:

1. **Environment variables** (highest priority):
   - `POSTGRESQL_HOST` — database host (default: localhost)
   - `POSTGRESQL_PORT` — port number (default: 5432)
   - `POSTGRESQL_USER` — username (default: postgres)
   - `POSTGRESQL_PASSWORD` — password (required if database requires auth)
   - `POSTGRESQL_DBNAME` — database name (default: postgres)
   - `POSTGRESQL_SSLMODE` — SSL mode (default: prefer; values: disable, allow, prefer, require)

2. **Config file** (lower priority, env vars override):
   - Path: `~/.config/postgresql-admin-pp-cli/config.toml` (XDG-compliant)
   - Format:
     ```toml
     host = "db.example.com"
     port = 5432
     user = "admin"
     password = "secret"
     dbname = "production"
     sslmode = "require"
     ```

**Never commit credentials to version control.** Always use environment variables or secure config files with restricted permissions (`chmod 600`).

Run `postgresql-admin-pp-cli doctor` to validate connectivity and permissions before running other commands.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures:

  ```bash
  postgresql-admin-pp-cli schema list --agent --select name,type
  ```
- **Non-interactive** — never prompts, all input via flags
- **Read-only by default** — commands are read-only except backup/restore and migrations (all with guardrails)
- **Structured errors** — JSON error objects with codes and context

## Command Reference

### doctor — Validate Connectivity and Permissions

Validate PostgreSQL connectivity, authentication, and user permissions. Run this first to diagnose setup issues.

**Usage:**
```bash
postgresql-admin-pp-cli doctor
postgresql-admin-pp-cli doctor --json
```

**Checks:**
- TCP connectivity to host:port
- PostgreSQL protocol handshake
- Authentication (username/password validity)
- User permissions (SELECT on pg_tables, CREATE on schemas)
- Replication configuration (WAL level, replication role)

**Examples:**
```bash
# Check connectivity and auth
postgresql-admin-pp-cli doctor

# JSON output for scripts
postgresql-admin-pp-cli doctor --json

# With custom config
postgresql-admin-pp-cli doctor --config /tmp/prod.toml
```

**Exit Codes:**
- `0` — All checks passed
- `4` — Authentication failed (check password and user)
- `5` — Database/server error (check server logs)
- `7` — Timeout connecting (check network and firewall)
- `10` — Config error (missing host, invalid config file)

---

### schema — Explore Schema Objects

Discover and inspect PostgreSQL schema objects (tables, views, indexes, columns, constraints).

#### schema list — List all schemas

List all schemas in the cluster (system and user-created).

**Usage:**
```bash
postgresql-admin-pp-cli schema list
postgresql-admin-pp-cli schema list --json
postgresql-admin-pp-cli schema list --csv
```

**Example Output (table):**
```
Name              Owner     Type       Description
public            postgres  user       Standard public schema
pg_catalog        postgres  system     PostgreSQL system catalog
information_schema postgres system     ANSI information schema
pg_temp           postgres  system     Temporary objects
```

**Example Output (JSON):**
```json
[
  {"Name": "public", "Owner": "postgres", "Type": "user", "Description": "Standard public schema"},
  {"Name": "pg_catalog", "Owner": "postgres", "Type": "system", "Description": "PostgreSQL system catalog"}
]
```

---

#### schema describe — List objects in a schema

List all tables, views, indexes, and sequences in a schema.

**Usage:**
```bash
postgresql-admin-pp-cli schema describe [schema_name]
postgresql-admin-pp-cli schema describe public --json
```

**Examples:**
```bash
# List objects in public schema
postgresql-admin-pp-cli schema describe public

# JSON output
postgresql-admin-pp-cli schema describe public --json

# CSV format
postgresql-admin-pp-cli schema describe public --csv

# Select specific fields
postgresql-admin-pp-cli schema describe public --json --select name,type,size_bytes
```

**Example Output (table):**
```
Name              Type       Size (bytes)  Owner     Description
users             table      16384         postgres  User accounts
user_roles        table      8192          postgres  User-role assignments
idx_users_email   index      8192          postgres  Email lookup index
sequence_seq      sequence   8192          postgres  Auto-increment
views_active      view       -             postgres  Active users view
```

---

#### schema inspect — Full table metadata

Inspect full metadata for a table: columns, types, nullability, defaults, indexes, constraints, foreign keys.

**Usage:**
```bash
postgresql-admin-pp-cli schema inspect [schema].[table]
postgresql-admin-pp-cli schema inspect public.users
postgresql-admin-pp-cli schema inspect public.users --json
```

**Examples:**
```bash
# Inspect users table in public schema
postgresql-admin-pp-cli schema inspect public.users

# JSON output
postgresql-admin-pp-cli schema inspect public.users --json

# Compact output for agents
postgresql-admin-pp-cli schema inspect public.users --agent --select columns,indexes,constraints
```

**Example Output (table):**
```
Column          Type              Nullable  Default          Indexes            Constraints
id              bigint            NO        nextval(...)     idx_users_pkey     PRIMARY KEY
email           text              NO        -                idx_users_email    UNIQUE
created_at      timestamp         NO        now()            -                  -
updated_at      timestamp         YES       -                -                  -
```

**Exit Codes:**
- `0` — Schema inspection succeeded
- `3` — Schema or table not found
- `5` — Query error (permissions, syntax, etc.)

---

### roles — Manage Roles and Permissions

Create, list, delete roles and grant/revoke permissions on database objects.

#### roles list — List all roles

List all PostgreSQL roles (users, groups) with membership information.

**Usage:**
```bash
postgresql-admin-pp-cli roles list
postgresql-admin-pp-cli roles list --json
```

**Examples:**
```bash
# List all roles
postgresql-admin-pp-cli roles list

# JSON format
postgresql-admin-pp-cli roles list --json

# CSV for spreadsheet import
postgresql-admin-pp-cli roles list --csv
```

**Example Output (table):**
```
Role          Type     Can Login  Can Create  Replication  Member Of
postgres      user     YES        YES         YES          -
app_user      user     YES        NO          NO           app_group
readonly      group    NO         NO          NO           -
```

---

#### roles create — Create a new role

Create a PostgreSQL role (user or group).

**Usage:**
```bash
postgresql-admin-pp-cli roles create [role_name]
postgresql-admin-pp-cli roles create app_user --password secret123 --can-login
```

**Flags:**
- `--password <password>` — Set password (prompted if not provided with `--no-input`)
- `--can-login` — Allow role to log in (default: false for groups)
- `--can-create-db` — Allow role to create databases
- `--can-create-role` — Allow role to create roles
- `--replication` — Allow role to initiate replication
- `--inherit` — Allow role to inherit parent role privileges (default: true)

**Examples:**
```bash
# Create user role interactively
postgresql-admin-pp-cli roles create analyst

# Create with flags
postgresql-admin-pp-cli roles create readonly_user --can-login --password mypass

# Create group role (no login)
postgresql-admin-pp-cli roles create finance_team

# Agent mode (no prompts)
postgresql-admin-pp-cli roles create bot_user --password secret --can-login --agent
```

**Exit Codes:**
- `0` — Role created successfully
- `2` — Usage error (bad flags)
- `4` — Permission denied (user lacks CREATE ROLE)
- `5` — Role already exists

---

#### roles grant — Grant permissions

Grant permissions on schemas or tables to a role.

**Usage:**
```bash
postgresql-admin-pp-cli roles grant --role [role] --to [parent_role] --on [object] --permissions [list]
```

**Flags:**
- `--role <name>` — Role to grant to (required)
- `--parent <parent_role>` — Parent role (for role membership)
- `--on <schema.object>` — Object to grant on (schema or table)
- `--permissions <list>` — Comma-separated permissions (SELECT, INSERT, UPDATE, DELETE, EXECUTE, USAGE, CREATE)

**Examples:**
```bash
# Grant SELECT on table to role
postgresql-admin-pp-cli roles grant --role readonly_user --on public.users --permissions SELECT

# Grant schema usage
postgresql-admin-pp-cli roles grant --role analyst --on public --permissions USAGE

# Make role member of group
postgresql-admin-pp-cli roles grant --role analyst --parent finance_team

# Grant multiple permissions
postgresql-admin-pp-cli roles grant --role app_user --on public.orders --permissions SELECT,INSERT,UPDATE
```

**Exit Codes:**
- `0` — Grant succeeded
- `3` — Role or object not found
- `4` — Permission denied
- `5` — Grant error (circular membership, etc.)

---

#### roles delete — Delete a role

Delete a PostgreSQL role.

**Usage:**
```bash
postgresql-admin-pp-cli roles delete [role_name]
postgresql-admin-pp-cli roles delete app_user --cascade
```

**Flags:**
- `--cascade` — Drop dependent objects (use with caution)
- `--yes` — Skip confirmation prompt

**Examples:**
```bash
# Delete role (fails if role has dependents)
postgresql-admin-pp-cli roles delete old_user

# Delete with dependents
postgresql-admin-pp-cli roles delete old_user --cascade --yes

# Agent mode
postgresql-admin-pp-cli roles delete orphan_role --cascade --agent
```

**Exit Codes:**
- `0` — Role deleted
- `3` — Role not found
- `4` — Permission denied
- `5` — Role has dependents (use --cascade)

---

### replication — Monitor Replication Status

Monitor primary-replica topology, lag, replication slots, and WAL retention.

#### replication status — Primary replica topology

Show primary node with list of connected replicas, including lag and status.

**Usage:**
```bash
postgresql-admin-pp-cli replication status
postgresql-admin-pp-cli replication status --json
```

**Examples:**
```bash
# Show replication status
postgresql-admin-pp-cli replication status

# JSON format for scripts
postgresql-admin-pp-cli replication status --json

# Agent mode
postgresql-admin-pp-cli replication status --agent
```

**Example Output (table):**
```
Replica          Lag (bytes)  Lag (time)      State           Sync  Slot
replica-1        1024         2ms             streaming       async replica_slot_1
replica-2        2048         5ms             streaming       sync  replica_slot_2
```

**Exit Codes:**
- `0` — Status retrieved successfully
- `5` — Replication not configured (WAL level = minimal)

---

#### replication slots — List replication slots

List logical and physical replication slots with retention info.

**Usage:**
```bash
postgresql-admin-pp-cli replication slots
postgresql-admin-pp-cli replication slots --json
```

**Examples:**
```bash
# List all slots
postgresql-admin-pp-cli replication slots

# JSON format
postgresql-admin-pp-cli replication slots --json

# CSV for analysis
postgresql-admin-pp-cli replication slots --csv
```

**Example Output (table):**
```
Slot Name        Type        Active  Retained WAL (MB)  Status
replica_slot_1   physical    YES     1024              active
replica_slot_2   physical    YES     2048              active
logical_slot_1   logical     NO      512               inactive
```

---

### migrations — Manage Database Migrations

List, apply, rollback, and verify database migrations with checksum validation.

#### migrations list — List all migrations

List all migration files from the `migrations/` directory with applied status.

**Usage:**
```bash
postgresql-admin-pp-cli migrations list
postgresql-admin-pp-cli migrations list --json
```

**Examples:**
```bash
# List migrations
postgresql-admin-pp-cli migrations list

# JSON format
postgresql-admin-pp-cli migrations list --json

# Show pending migrations only
postgresql-admin-pp-cli migrations list --status pending
```

**Example Output (table):**
```
ID    Name                      Status    Applied At            Checksum
001   create_users_table        applied   2026-06-01 10:00:00   abc123...
002   add_email_index           applied   2026-06-02 11:00:00   def456...
003   create_orders_table       pending   -                     ghi789...
```

---

#### migrations apply — Apply pending migrations

Apply all pending migrations in order.

**Usage:**
```bash
postgresql-admin-pp-cli migrations apply
postgresql-admin-pp-cli migrations apply --dry-run
```

**Flags:**
- `--dry-run` — Show what would be applied without executing
- `--migrations-dir <path>` — Override default migrations directory

**Examples:**
```bash
# Apply all pending migrations
postgresql-admin-pp-cli migrations apply

# Dry-run to preview
postgresql-admin-pp-cli migrations apply --dry-run

# Agent mode with dry-run first
postgresql-admin-pp-cli migrations apply --dry-run --agent
postgresql-admin-pp-cli migrations apply --agent --yes
```

**Exit Codes:**
- `0` — All pending migrations applied
- `2` — Checksum mismatch (migration file changed post-application)
- `5` — Migration error (SQL syntax, constraint violation, etc.)

---

#### migrations rollback — Rollback migrations

Rollback migrations to a previous state.

**Usage:**
```bash
postgresql-admin-pp-cli migrations rollback --to [id]
postgresql-admin-pp-cli migrations rollback --dry-run
```

**Flags:**
- `--to <migration_id>` — Rollback to this migration (inclusive)
- `--dry-run` — Show what would be rolled back
- `--yes` — Skip confirmation

**Examples:**
```bash
# Show pending rollbacks
postgresql-admin-pp-cli migrations rollback --dry-run

# Rollback to migration 002
postgresql-admin-pp-cli migrations rollback --to 002 --yes

# Agent mode
postgresql-admin-pp-cli migrations rollback --to 002 --agent
```

**Exit Codes:**
- `0` — Rollback successful
- `2` — Target migration not found
- `5` — Rollback error

---

#### migrations verify — Verify migration integrity

Check for inconsistencies in migration state and fix if safe.

**Usage:**
```bash
postgresql-admin-pp-cli migrations verify
postgresql-admin-pp-cli migrations verify --fix
```

**Flags:**
- `--fix` — Auto-fix inconsistencies (re-apply/rollback as needed)

**Examples:**
```bash
# Check for issues
postgresql-admin-pp-cli migrations verify

# Auto-fix if safe
postgresql-admin-pp-cli migrations verify --fix --agent
```

**Exit Codes:**
- `0` — Migration state is consistent
- `5` — Inconsistencies detected (use --fix to repair)

---

### query — Execute Safe SQL Queries

Execute SELECT queries with guardrails against mutations and DDL.

#### query execute — Run a SELECT query

Execute a SQL query and return results in selected format.

**Usage:**
```bash
postgresql-admin-pp-cli query execute "[SQL]"
postgresql-admin-pp-cli query execute "SELECT * FROM users LIMIT 10"
postgresql-admin-pp-cli query execute "SELECT id, name FROM users WHERE id = ?" --params "123"
```

**Flags:**
- `--params <value>` — Parameterized query values (pipe-separated: "val1|val2|val3")

**Allowed Statements:**
- SELECT (all variations)
- EXPLAIN ANALYZE
- Introspection functions (information_schema.*, pg_* views)

**Blocked Statements:**
- INSERT, UPDATE, DELETE (mutations)
- CREATE, ALTER, DROP (DDL)
- BEGIN, COMMIT, ROLLBACK (transaction control)
- GRANT, REVOKE (privilege changes)

**Examples:**
```bash
# Simple SELECT
postgresql-admin-pp-cli query execute "SELECT id, name FROM users LIMIT 5"

# JSON output
postgresql-admin-pp-cli query execute "SELECT * FROM users" --json

# Parameterized query
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE id = ?" --params "123"

# EXPLAIN ANALYZE
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM large_table"

# Agent mode
postgresql-admin-pp-cli query execute "SELECT COUNT(*) FROM orders" --agent
```

**Example Output (JSON):**
```json
[
  {"id": 1, "name": "Alice", "email": "alice@example.com"},
  {"id": 2, "name": "Bob", "email": "bob@example.com"}
]
```

**Example Output (table):**
```
id  name   email
--  -----  ----------------
1   Alice  alice@example.com
2   Bob    bob@example.com
```

**Exit Codes:**
- `0` — Query executed successfully
- `2` — Query is not allowed (mutations, DDL, etc.)
- `5` — Query error (syntax, permissions, not found)

---

### backup — Create, Verify, and Restore Backups

Create database backups, verify integrity, and restore from backups.

#### backup create — Create a backup

Create a backup using `pg_dump`.

**Usage:**
```bash
postgresql-admin-pp-cli backup create
postgresql-admin-pp-cli backup create --format custom --output /tmp/db.backup
```

**Flags:**
- `--format <format>` — Backup format: `custom` (default), `tar`, `plaintext`
- `--output <path>` — Output file path (default: `backup-TIMESTAMP.custom`)
- `--check-disk-space` — Verify disk space before backup (default: true)
- `--parallel <n>` — Parallel workers (default: 1)

**Examples:**
```bash
# Create backup in default location
postgresql-admin-pp-cli backup create

# Custom format to specific path
postgresql-admin-pp-cli backup create --format custom --output /backups/prod.backup

# Plaintext SQL format
postgresql-admin-pp-cli backup create --format plaintext --output /backups/prod.sql

# Parallel backup with compression
postgresql-admin-pp-cli backup create --format custom --parallel 4 --output /backups/fast.backup

# Agent mode
postgresql-admin-pp-cli backup create --format custom --output /tmp/backup.custom --agent
```

**Exit Codes:**
- `0` — Backup created successfully
- `5` — Backup error (permissions, disk full, etc.)

---

#### backup list — List backups

List existing backup files with sizes and creation dates.

**Usage:**
```bash
postgresql-admin-pp-cli backup list
postgresql-admin-pp-cli backup list --json
```

**Examples:**
```bash
# List backups in default directory
postgresql-admin-pp-cli backup list

# JSON output
postgresql-admin-pp-cli backup list --json

# CSV format
postgresql-admin-pp-cli backup list --csv
```

**Example Output (table):**
```
Filename                     Size (MB)  Created At            Verified
backup-20260601-100000.bak   256.5      2026-06-01 10:00:00   YES
backup-20260602-110000.bak   512.3      2026-06-02 11:00:00   YES
backup-20260615-000000.bak   128.0      2026-06-15 00:00:00   PENDING
```

---

#### backup verify — Verify backup integrity

Validate backup file integrity using `pg_restore --list`.

**Usage:**
```bash
postgresql-admin-pp-cli backup verify [backup_file]
postgresql-admin-pp-cli backup verify /backups/prod.backup
```

**Examples:**
```bash
# Verify backup
postgresql-admin-pp-cli backup verify backup-20260601.backup

# Verify with JSON output
postgresql-admin-pp-cli backup verify backup-20260601.backup --json

# Agent mode
postgresql-admin-pp-cli backup verify /backups/prod.backup --agent
```

**Exit Codes:**
- `0` — Backup is valid
- `5` — Backup is corrupted or unreadable

---

#### backup restore — Restore from backup

Restore database from a backup file.

**Usage:**
```bash
postgresql-admin-pp-cli backup restore [backup_file]
postgresql-admin-pp-cli backup restore /backups/prod.backup --target-db restored_db
```

**Flags:**
- `--target-db <name>` — Target database name (default: original database)
- `--schema <schema>` — Restore specific schema only
- `--force` — Force restore even if target DB exists
- `--dry-run` — Show what would be restored

**Examples:**
```bash
# Restore to new database
postgresql-admin-pp-cli backup restore backup-20260601.backup --target-db test_db

# Restore specific schema
postgresql-admin-pp-cli backup restore backup-20260601.backup --schema public

# Dry-run to preview
postgresql-admin-pp-cli backup restore backup-20260601.backup --target-db test_db --dry-run

# Force restore with agent mode
postgresql-admin-pp-cli backup restore backup-20260601.backup --target-db prod --force --agent
```

**Exit Codes:**
- `0` — Restore successful
- `5` — Restore error (bad backup, permissions, etc.)

---

## Output Modes

All commands support multiple output formats:

| Flag | Format | Use Case |
|------|--------|----------|
| (default) | Human-readable table | Interactive use, quick viewing |
| `--json` | JSON array of objects | Parsing by agents/scripts |
| `--csv` | CSV with header row | Spreadsheet import |
| `--plain` | Tab-separated values | Piping to other tools |
| `--quiet` | One value per line | Simple piping, ID extraction |

**Examples:**
```bash
# JSON for agent processing
postgresql-admin-pp-cli schema list --json | jq '.[] | select(.Type == "user")'

# CSV for spreadsheet
postgresql-admin-pp-cli roles list --csv > roles.csv

# Quiet for ID piping
postgresql-admin-pp-cli schema list --quiet | xargs -I {} postgresql-admin-pp-cli schema describe {}

# Plain text for shell scripting
postgresql-admin-pp-cli query execute "SELECT id FROM users" --plain
```

## Exit Codes

All commands follow Unix exit code conventions:

| Code | Meaning | When to Use |
|------|---------|-----------|
| 0 | Success | Command completed successfully |
| 2 | Usage error | Invalid flags, missing required args, bad format |
| 3 | Not found | Resource not found (role, table, schema) |
| 4 | Auth error | Authentication failed, permission denied |
| 5 | API error | Server error, query failed, constraint violation |
| 7 | Timeout | Connection timeout, slow query timeout |
| 10 | Config error | Missing config, bad credentials, invalid path |

**Examples:**
```bash
# Check if schema exists
if postgresql-admin-pp-cli schema inspect public.users > /dev/null 2>&1; then
  echo "Table found"
else
  if [ $? -eq 3 ]; then
    echo "Table not found"
  fi
fi

# Script with error handling
postgresql-admin-pp-cli roles create newuser --password secret || {
  case $? in
    4) echo "Permission denied" ;;
    5) echo "Role already exists" ;;
    *) echo "Unknown error" ;;
  esac
}
```

## Common Workflows

### Introspect a Database Schema

```bash
# List all schemas
postgresql-admin-pp-cli schema list

# Describe public schema
postgresql-admin-pp-cli schema describe public

# Inspect specific table
postgresql-admin-pp-cli schema inspect public.users

# JSON output for programmatic use
postgresql-admin-pp-cli schema inspect public.users --json
```

### Set Up a Read-Only User

```bash
# Create the role
postgresql-admin-pp-cli roles create analyst --can-login --password secure123

# Grant schema usage
postgresql-admin-pp-cli roles grant --role analyst --on public --permissions USAGE

# Grant SELECT on tables
postgresql-admin-pp-cli roles grant --role analyst --on public.users --permissions SELECT
postgresql-admin-pp-cli roles grant --role analyst --on public.orders --permissions SELECT
```

### Monitor Replication

```bash
# Check replication status
postgresql-admin-pp-cli replication status

# Monitor slots
postgresql-admin-pp-cli replication slots

# JSON output for alerts/dashboards
postgresql-admin-pp-cli replication status --json
```

### Apply Database Migrations

```bash
# Check pending migrations
postgresql-admin-pp-cli migrations list

# Preview what would be applied
postgresql-admin-pp-cli migrations apply --dry-run

# Apply all pending migrations
postgresql-admin-pp-cli migrations apply

# Verify integrity
postgresql-admin-pp-cli migrations verify
```

### Backup and Restore

```bash
# Create backup
postgresql-admin-pp-cli backup create --format custom --output /backups/today.backup

# Verify backup
postgresql-admin-pp-cli backup verify /backups/today.backup

# List available backups
postgresql-admin-pp-cli backup list

# Restore to test database
postgresql-admin-pp-cli backup restore /backups/today.backup --target-db test_db
```

### Query Analysis

```bash
# Run simple query
postgresql-admin-pp-cli query execute "SELECT COUNT(*) as user_count FROM users"

# EXPLAIN ANALYZE for performance
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM large_table WHERE id = 123"

# Parameterized query
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE email = ?" --params "user@example.com"

# JSON output for parsing
postgresql-admin-pp-cli query execute "SELECT id, name FROM users" --json
```

## Troubleshooting

### Connection Failures

```bash
# Validate connectivity
postgresql-admin-pp-cli doctor

# Check with custom host/port
POSTGRESQL_HOST=db.prod.example.com POSTGRESQL_PORT=5433 postgresql-admin-pp-cli doctor

# With config file
postgresql-admin-pp-cli doctor --config /etc/postgresql-admin/prod.toml
```

### Permission Errors

```bash
# Check current user's permissions
postgresql-admin-pp-cli doctor

# List current roles
postgresql-admin-pp-cli roles list --agent

# Inspect table permissions
postgresql-admin-pp-cli schema inspect public.users --agent
```

### Slow Queries

```bash
# Run EXPLAIN ANALYZE
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM large_table"

# JSON output for parsing
postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT COUNT(*) FROM orders WHERE created_at > NOW() - INTERVAL '1 day'" --json
```

### Replication Issues

```bash
# Check replication status
postgresql-admin-pp-cli replication status

# Monitor slots
postgresql-admin-pp-cli replication slots

# JSON for alerting systems
postgresql-admin-pp-cli replication status --json | jq '.[] | select(.Lag_ms > 1000)'
```

## Version and Help

```bash
# Show version
postgresql-admin-pp-cli --version

# Show general help
postgresql-admin-pp-cli --help

# Show command-specific help
postgresql-admin-pp-cli schema --help
postgresql-admin-pp-cli roles --help
postgresql-admin-pp-cli backup --help

# Find matching command by capability
postgresql-admin-pp-cli which "list all tables"
```
