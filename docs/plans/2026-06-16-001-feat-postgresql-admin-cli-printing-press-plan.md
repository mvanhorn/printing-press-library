---
name: postgresql-admin-cli-printing-press
title: "feat: PostgreSQL Admin CLI for Printing Press"
created: 2026-06-16
status: ready
execution: code
origin: null
---

# feat: PostgreSQL Admin CLI for Printing Press

## Summary

Build a full-featured PostgreSQL Admin CLI in Go as the first real-world test case for the Printing Press validation workflow. The CLI will support schema introspection, user/role management, replication monitoring, arbitrary query execution, migration management, and backup/restore operations. It will be generated via Printing Press (using an internal YAML spec), then validated through all three validation stages (spec-validator, briefing, pre-commit hooks) before final code generation. The CLI integrates with the docker-hub pattern used throughout the printing-press-library ecosystem.

## Problem Frame

The Printing Press CLI generation framework needs a production-grade test case to validate the end-to-end workflow: spec generation → spec validation → edge-case research → pre-commit enforcement → successful generation. PostgreSQL Admin CLI is an ideal candidate because:

1. **Well-defined problem domain**: PostgreSQL has extensive system catalogs and well-documented APIs (libpq, SQL commands)
2. **Multi-feature scope**: Schema introspection, role management, replication, migrations, backup/restore exercise different code patterns and validation challenges
3. **Known edge cases**: MVCC races, cascading state changes, concurrent operations, error recovery scenarios are well-understood
4. **Real-world complexity**: Enough complexity to stress the validation workflow (error types, rate limits, state management) without being a research project

By completing this CLI, the Printing Press team gains confidence that the validation workflow catches real issues early and produces robust, first-pass-successful code generation.

---

## Requirements & Scope

### Core Scope (Confirmed)

The PostgreSQL Admin CLI will cover six major capability areas:

1. **Schema Introspection** — Discover and inspect PostgreSQL schema objects (schemas, tables, columns, indexes, constraints, sequences, views, functions)
2. **User and Role Management** — Create, list, modify, and delete PostgreSQL roles; grant/revoke permissions across objects
3. **Replication Status Monitoring** — Monitor logical and physical replication, replica lag, WAL retention, replication slots
4. **Arbitrary Query Execution** — Execute SQL queries with safety guardrails against the target database
5. **Migration Management** — List, apply, rollback, and verify database migrations; track migration state
6. **Backup and Restore** — Create backups (wrapper around pg_dump), restore from backup, verify backup integrity

### Scope Boundaries

**Out of Scope (Deferred to Follow-Up Work)**
- Query optimization or query plan analysis (use `EXPLAIN ANALYZE` directly for now)
- Automatic failover orchestration (monitoring only; manual failover via role change)
- Distributed transaction coordination across multiple PostgreSQL instances
- Custom backup compression algorithms (rely on pg_dump formats: custom, tar, plaintext)
- Interactive SQL shell (use `psql` for that; CLI focuses on admin operations)
- Multi-cluster management (single cluster at a time)

---

## High-Level Technical Design

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  PostgreSQL Admin CLI (postgresql-admin-pp-cli)            │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Cobra Command Tree (cmd/...)                        │  │
│  │ • schema {list, describe, create, drop}            │  │
│  │ • roles {list, create, grant, revoke, delete}      │  │
│  │ • migrations {list, status, apply, rollback}       │  │
│  │ • replication {status, slots, monitor}             │  │
│  │ • query {execute} (with safety guards)             │  │
│  │ • backup {create, list, restore, verify}           │  │
│  │ • doctor (validate connectivity & permissions)     │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│         ┌────────────────┼────────────────┐                 │
│         ↓                ↓                ↓                 │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │   Config   │  │   pgx Client │  │  Formatting  │        │
│  │ (TOML +    │  │  (connection │  │  (output     │        │
│  │  env vars) │  │   pooling)   │  │   modes)     │        │
│  └────────────┘  └──────────────┘  └──────────────┘        │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ↓                                  │
│              ┌───────────────────────┐                      │
│              │  PostgreSQL Instance  │                      │
│              │  (Queries via pgx)    │                      │
│              └───────────────────────┘                      │
│                          │                                  │
│              ┌───────────┴───────────┐                      │
│              ↓                       ↓                      │
│        [System Catalogs]        [Local SQLite]             │
│        (introspection)          (cache + state)            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

**KTD-1: pgx Driver with Connection Pooling**
- Use `jackc/pgx` (modern, connection pooling, prepared statements) instead of `lib/pq` or ORM
- Rationale: Native Go, better performance, control over concurrency behavior
- Application: `internal/client/client.go` wraps pgx with retry logic and timeout handling

**KTD-2: TOML Configuration + Environment Override**
- Config file: `~/.config/postgresql-admin-pp-cli/config.toml` (XDG-compliant)
- Env var precedence: `POSTGRESQL_HOST`, `POSTGRESQL_PORT`, `POSTGRESQL_USER`, `POSTGRESQL_PASSWORD`, `POSTGRESQL_DBNAME`, `POSTGRESQL_SSLMODE`
- Rationale: Follows docker-hub pattern; env var override enables CI/test isolation
- Application: `internal/config/config.go` loads and validates credentials

**KTD-3: Local SQLite Cache for Schema Metadata**
- Purpose: Enable offline schema introspection for agents; cache results of expensive queries
- Location: `~/.config/postgresql-admin-pp-cli/store.db` (WAL mode, concurrent read-safe)
- Lifecycle: `schema sync` populates cache; `schema list --data-source local` queries cache
- Rationale: Mirrors outlook-email pattern for read-only offline capability
- Application: `internal/store/store.go` with schema versioning and migration safety

**KTD-4: Cobra Command Structure with Persistent Flags**
- Root flags: `--json`, `--csv`, `--plain`, `--quiet` (output modes), `--compact` (minimal fields), `--config`, `--agent` (auto-expands to json+compact+no-input+no-color+yes)
- Subcommands: Organized by resource (schema, roles, migrations, replication, query, backup)
- Rationale: Standardized across all Printing Press CLIs; agents expect these flags
- Application: `internal/cli/root.go` defines flags; per-command files (schema.go, roles.go, etc.) implement subcommands

**KTD-5: Doctor Command as Validation Entrypoint**
- Purpose: Validate connectivity, authentication, and permissions before any operation
- Checks: TCP reachability, PostgreSQL protocol handshake, auth success, user permissions (CREATE, USAGE, EXECUTE)
- Output: Structured JSON when `--json` flag set; colored human-readable output by default
- Rationale: Every failure without doctor is a mystery; doctor catches 80% of setup issues
- Application: `internal/cli/doctor.go` (first command to implement)

**KTD-6: Output Modes (JSON, CSV, Table, Plain, Quiet)**
- JSON: Full structured data with metadata; agent-safe
- CSV: Tabular results for piping to other tools
- Table: Human-readable, TTY-only (auto-detected), colored
- Plain: Tab-separated, scriptable
- Quiet: One value per line (IDs for lists)
- Rationale: Standardized across printing-press-library; agents use JSON+compact
- Application: `internal/cli/helpers.go` with routing logic

**KTD-7: Safe Query Execution with Guardrails**
- Allowed: SELECT, EXPLAIN ANALYZE, introspection queries (pg_* system functions)
- Blocked: Data mutations (INSERT, UPDATE, DELETE), DDL (CREATE, ALTER, DROP), transaction control (BEGIN, COMMIT)
- Rationale: Prevent accidental data loss in CLI context; use `psql` for full SQL access
- Application: `internal/cli/query.go` with query parser and allowlist validation

**KTD-8: Migration Model — SQL Files in Git with State Tracking**
- Each migration is two files: `migrations/NNN_name.up.sql` and `down.sql`
- State tracked in `schema_migrations(id, name, applied_at, checksum)` table
- Checksum validates file integrity; detects hand-edits post-application
- Rationale: Aligns with standard database migration tools; idempotent by design
- Application: `internal/cli/migrations.go` with verification and rollback logic

**KTD-9: Backup Integration with pg_dump/pg_restore**
- Strategy: Wrap `pg_dump` and `pg_restore` instead of custom logic
- Rationale: Proven, tested, standard tool; avoids reimplementing complex backup logic
- Application: `internal/cli/backup.go` spawns pg_dump/pg_restore subprocesses with orchestration

**KTD-10: Error Exit Codes (Unix Standard)**
- 0: Success
- 2: Usage error (bad flags, invalid arguments)
- 3: Not found (role doesn't exist, schema not found)
- 4: Authentication error (invalid password, permission denied)
- 5: Database/API error (server error, query failed)
- 7: Timeout
- 10: Configuration error (bad config file, credentials missing)
- Rationale: Standardized across printing-press-library; enables scripting with exit code checks
- Application: `internal/cli/helpers.go` with error wrappers (usageErr, authErr, apiErr, etc.)

---

## Implementation Units

### U1. Project Setup & Core Infrastructure

**Goal:** Establish directory structure, module initialization, core abstractions (config, client, output formatting) that all commands depend on.

**Requirements:** 
- Foundation for all other units (U2-U8)
- Printing Press library conventions (cmd/, internal/, SKILL.md, README.md, AGENTS.md)
- Go module with key dependencies (pgx, go-toml, Cobra)

**Dependencies:** None (first unit)

**Files to Create:**
- `library/developer-tools/postgresql-admin/cmd/postgresql-admin-pp-cli/main.go`
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (command tree, persistent flags, PreRunE)
- `library/developer-tools/postgresql-admin/internal/cli/helpers.go` (output routing, error codes, formatting)
- `library/developer-tools/postgresql-admin/internal/config/config.go` (TOML loading, env override, validation)
- `library/developer-tools/postgresql-admin/internal/client/client.go` (pgx wrapper, query execution, error handling)
- `library/developer-tools/postgresql-admin/internal/cliutil/text.go` (formatting utilities)
- `library/developer-tools/postgresql-admin/internal/store/schema.sql` (SQLite schema for cache)
- `library/developer-tools/postgresql-admin/go.mod`, `go.sum` (Go module with pgx, go-toml, Cobra, pgx/pgtest)
- `library/developer-tools/postgresql-admin/.printing-press.json` (manifest stub)
- `library/developer-tools/postgresql-admin/Makefile` (build targets)
- `library/developer-tools/postgresql-admin/.golangci.yml` (linter config)
- `library/developer-tools/postgresql-admin/LICENSE`, `NOTICE`

**Approach:**
1. Create directory structure mirroring `docker-hub` example
2. Initialize Go module with `go mod init github.com/mvanhorn/printing-press-library/library/developer-tools/postgresql-admin`
3. Implement core abstractions:
   - `config.Config` struct: host, port, user, password, dbname, sslmode
   - `Config.Load()`: read TOML, apply env var overrides, validate required fields
   - `client.Client`: pgx connection pool, Query(), QueryRow(), Exec() methods with timeout
   - `helpers.Output()`: route to JSON/CSV/table/plain/quiet based on flags
   - Error wrappers: usageErr, authErr, apiErr, notFoundErr (sets exit codes)
4. Define root command in `root.go`: ParentFlags struct (json, csv, plain, quiet, compact, config, agent), AddCommand stubs for U2-U8
5. Implement Makefile targets: `make build`, `make test`, `make lint`, `make install`

**Patterns to Follow:**
- `library/developer-tools/docker-hub/internal/cli/root.go` (flag structure, PreRunE validation)
- `library/developer-tools/docker-hub/internal/config/config.go` (TOML + env override pattern)
- `library/developer-tools/docker-hub/internal/client/client.go` (HTTP client wrapper; adapt for pgx)
- `library/social-and-messaging/multimail/internal/store/store.go` (SQLite WAL mode, schema versioning)

**Test Scenarios:**
- Config loads from TOML file without env vars set
- Env vars override TOML values (POSTGRESQL_HOST overrides config.toml host)
- Missing required field (e.g., no host) → error with guidance
- Password is never logged, only labeled as "from env:POSTGRESQL_PASSWORD"
- Output flag routing: --json formats as JSON, --csv as CSV, --plain as tab-separated, --quiet as one-per-line, default as table (human-readable)
- --agent flag expands to --json --compact --no-input --no-color --yes
- Exit codes: 2 for bad flags, 10 for config error, 5 for connection error
- Connection pool accepts concurrent requests (3+ simultaneous queries succeed)

**Verification:**
1. `go build ./...` succeeds; binary `postgresql-admin-pp-cli` runs
2. `postgresql-admin-pp-cli --version` outputs version (git commit)
3. `postgresql-admin-pp-cli --help` shows usage with all persistent flags
4. `postgresql-admin-pp-cli --config /tmp/test.toml` loads custom config path
5. Test database connection succeeds with doctor command (next unit)

---

### U2. Doctor Command (Validation & Diagnostics)

**Goal:** Implement `doctor` command to validate connectivity, authentication, and permissions. Serve as entry point for troubleshooting setup issues.

**Requirements:**
- Validates all preconditions before other commands run
- User-friendly diagnostics with structured JSON output (--json flag)
- Reports connectivity, auth, and permission status

**Dependencies:** U1 (config, client, output helpers)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/doctor.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add doctor command)

**Approach:**
1. Implement `doctor` command with the following checks (in order):
   - **TCP Reachability**: Can we reach host:port? (net.Dial check)
   - **PostgreSQL Protocol**: Does the server speak PostgreSQL? (attempt pgx.Connect, catch protocol-level errors)
   - **Authentication**: Is the username/password valid? (pgx auth succeeds or fails with clear error)
   - **User Permissions**: Can user execute required operations? (SELECT on pg_tables, CREATE on public schema, CREATE SCHEMA, etc.)
   - **Replication State** (optional): Is WAL level set correctly? (SELECT current_setting('wal_level'))
2. Report each check with ✓/✗ indicator (human mode) or structured JSON (--json mode)
3. Label credential source: "from config" or "from env:POSTGRESQL_PASSWORD"
4. Never print actual password values; label only

**Patterns to Follow:**
- `library/commerce/printify/internal/cli/doctor.go` (validation steps, colored output, error hints)
- `library/productivity/superhuman/internal/client/client.go` (APIError pattern; adapt for pgx errors)

**Test Scenarios:**
- TCP reachable, auth succeeds, permissions ok → "All checks passed" (✓ for all)
- TCP unreachable → "Cannot reach host:port; check network/firewall"
- Auth fails (wrong password) → "Authentication failed; check POSTGRESQL_PASSWORD or config.toml password"
- User lacks CREATE permission on schema → "CREATE permission missing on schema public; suggest GRANT CREATE ON SCHEMA public TO user"
- Replication not configured → "WAL level is 'replica'; replication commands available" (informational, not error)
- Output with --json: `{"checks": [{"name": "connectivity", "status": "ok"}, ...], "overall": "ok"}`

**Verification:**
1. `postgresql-admin-pp-cli doctor` with valid config → all checks pass
2. `postgresql-admin-pp-cli doctor` with wrong password → auth error with helpful message
3. `postgresql-admin-pp-cli doctor --json` → valid JSON output with all checks
4. Doctor command does NOT modify any state (read-only queries only)

---

### U3. Schema Introspection (Tables, Columns, Indexes, Views)

**Goal:** Implement schema discovery and inspection commands. Allow users to explore table structures, columns, indexes, constraints, and views.

**Requirements:**
- List schemas, tables, columns, indexes, constraints, sequences, views
- Describe individual objects with full metadata
- Handle edge cases (temp tables, partitioned tables, inherited tables, circular FKs)
- Output modes: JSON (full metadata), table (human-readable), CSV (spreadsheet-friendly)

**Dependencies:** U1 (client, config), U2 (doctor for validation)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/schema.go`
- `library/developer-tools/postgresql-admin/internal/cli/schema_test.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add schema command)

**Approach:**
1. Implement subcommands:
   - `schema list` → list all schemas (system + user)
   - `schema describe <schema>` → list all objects in schema (tables, views, indexes, sequences)
   - `schema inspect <schema>.<table>` → full table metadata (columns, types, nullability, default values, indexes, constraints, FKs)
2. Queries use PostgreSQL system catalogs:
   - `pg_namespace` for schemas
   - `pg_class` (with relkind filtering) for tables/views/sequences
   - `pg_attribute` for columns
   - `pg_index` for indexes
   - `pg_constraint` for constraints and FKs
3. Handle edge cases:
   - Temp tables (ephemeral; show warning in output)
   - Unlogged tables (warn that data lost on crash)
   - Materialized views (label as such, show refresh state)
   - Partitioned tables (show parent/child hierarchy, don't infinite-loop on cycles)
4. Output modes:
   - JSON: Full metadata with all fields
   - Table: Human-readable columns (name, type, nullability, default)
   - CSV: Column headers + data rows

**Patterns to Follow:**
- `library/productivity/outlook-email/internal/cli/` (per-endpoint command structure)
- Flow analysis findings: gracefully skip objects dropped during introspection; validate temp table session-local scope

**Test Scenarios:**
- List schemas → shows public, information_schema, pg_catalog, pg_toast
- Describe public schema → shows all tables/views/sequences in public
- Inspect users table → full columns, types, indexes, constraints, FKs
- Inspect table with FK constraint → shows referenced table and column
- Circular FK (table a refs b, table b refs a) → output shows cycle without infinite loop
- Temp table (in pg_temp schema) → warning "temporary tables are session-local"
- Object dropped mid-introspection → graceful skip; report "object dropped during introspection"
- Output with --json → valid JSON with metadata
- Output with --csv → header row + data rows, spreadsheet-compatible
- Output with --select name,type → JSON with only name and type fields

**Verification:**
1. `schema list` returns accurate schema list
2. `schema describe public` includes all objects in public schema
3. `schema inspect table_name` shows correct columns and types
4. Circular FK detection prevents infinite loops
5. Tests include temp tables, partitioned tables, inherited tables edge cases

---

### U4. Role Management (Create, List, Grant, Revoke)

**Goal:** Implement user and role management commands. Allow users to create/modify/delete roles and grant/revoke permissions on objects.

**Requirements:**
- List roles with metadata (membership, permissions, inheritance)
- Create roles (interactive password prompt or --password flag)
- Grant/revoke permissions on schemas, tables, functions
- Delete roles (with cascade option for dependent objects)
- Validate role names, detect circular membership

**Dependencies:** U1 (client, config), U2 (doctor)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/roles.go`
- `library/developer-tools/postgresql-admin/internal/cli/roles_test.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add roles command)

**Approach:**
1. Implement subcommands:
   - `roles list` → all roles with membership hierarchy
   - `roles create <name>` → create role (prompt: password?, inherit?, replication?)
   - `roles grant --member <role> --to <container> --on <object> --perm <list>` → grant permissions
   - `roles revoke --member <role> --from <container>` → revoke membership
   - `roles delete <name> [--cascade]` → delete role
2. Use system catalogs:
   - `pg_roles` for role list
   - `pg_auth_members` for membership
   - `pg_default_acl` for default privileges
3. Safety checks:
   - Validate role exists before grant/revoke
   - Detect circular membership before granting (SQL does this, but warn user first)
   - Cascade analysis (find all dependent objects before delete --cascade)
4. Error handling:
   - "Role already exists"
   - "Role not found"
   - "Circular membership detected"
   - "Permission denied (insufficient privilege)"

**Patterns to Follow:**
- `docker-hub` example for command structure
- Flow analysis findings: warn on cascading revoke, password policy mismatch, circular membership

**Test Scenarios:**
- Create role with password → interactively set password or via --password
- Create role with --replication flag → role has REPLICATION flag set
- List roles → shows all roles with membership counts
- Grant SELECT on table to role → role can select from table
- Revoke SELECT from role → role can no longer select
- Delete role with no dependents → succeeds
- Delete role with dependents (--cascade) → cascades to dependent objects
- Circular membership (role_a member of role_b, role_b member of role_a) → error before attempting GRANT
- Permission denied (user lacks CREATE on SCHEMA) → error with helpful message

**Verification:**
1. Created roles appear in `roles list`
2. Grants are reflected in role permissions (query pg_default_acl)
3. Revokes remove permissions
4. Circular membership detected before GRANT attempt
5. Tests cover membership hierarchy, permission inheritance

---

### U5. Replication Monitoring

**Goal:** Implement replication status monitoring. Provide visibility into primary-replica state, lag, WAL retention, replication slots.

**Requirements:**
- Show primary-replica topology (which replicas connected to primary)
- Measure replica lag (bytes and time)
- Monitor replication slots (logical and physical)
- Detect cascading replication and sync mode
- Warn when capacity (WAL retention) exceeds disk space

**Dependencies:** U1 (client, config), U2 (doctor)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/replication.go`
- `library/developer-tools/postgresql-admin/internal/cli/replication_test.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add replication command)

**Approach:**
1. Implement subcommands:
   - `replication status` → primary node with list of connected replicas (lag, state, slot)
   - `replication slots` → list logical and physical replication slots with retention info
   - `replication monitor` → continuous watch mode (tail replicas every N seconds)
2. Queries:
   - `pg_stat_replication` for active walsenders
   - `pg_replication_slots` for slot inventory
   - `pg_wal_lsn_diff()` for lag calculation
   - `synchronous_standby_names` GUC for sync mode
3. Edge cases:
   - Replica unreachable → show "unreachable; last known lag Xm ago"
   - Cascading replication (replica→replica→replica) → document limitation; suggest --self-as-primary on replica
   - Sync mode mismatch (expected N replicas, got M) → warn "PRIMARY STALLED" if write-blocked
   - Slot retention > disk_free → warn "capacity exceeded; slots will auto-drop"
4. Output:
   - JSON: full replica inventory with all fields
   - Table: summary with lag, state, sync status

**Patterns to Follow:**
- Flow analysis findings: handle unreachable replicas, cascading topology, sync quorum mismatch

**Test Scenarios:**
- Primary with 2 replicas → status shows both with lag
- Replica unreachable → status shows "unreachable; last contact Xm ago"
- Cascading replication (primary→r1→r2) → status only shows r1; suggest --self-as-primary on r1
- Replication slot retention > disk free → warning "capacity at risk"
- Sync mode: 2 expected, 1 connected → warning "PRIMARY STALLED (write blocked)"
- WAL level = replica → replication available
- WAL level = minimal → replication unavailable (error)

**Verification:**
1. `replication status` shows accurate replica lag (via pg_stat_replication)
2. Slot retention is measured correctly
3. Sync mode quorum is validated against pg_stat_replication
4. Unreachable replicas are handled gracefully (no timeout hang)
5. Tests include cascading replication, quorum mismatch, capacity warning scenarios

---

### U6. Migration Management (Apply, Rollback, Verify)

**Goal:** Implement database migration management. Allow users to define, apply, and rollback migrations with state tracking and safety checks.

**Requirements:**
- List migrations from disk (migrations/ directory)
- Show applied status (query schema_migrations table)
- Apply pending migrations in order
- Rollback migrations
- Detect and prevent common errors (circular deps, partial state, checksum mismatch)
- Verify migration integrity (checksums, state consistency)

**Dependencies:** U1 (client, config), U2 (doctor), U3 (schema introspection for state verification)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/migrations.go`
- `library/developer-tools/postgresql-admin/internal/cli/migrations_test.go`
- `library/developer-tools/postgresql-admin/schema_migrations.sql` (schema_migrations table creation)

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add migrations command)

**Approach:**
1. Implement subcommands:
   - `migrations list` → all migrations on disk with applied status
   - `migrations status` → detailed status (pending, applied, failed)
   - `migrations apply [--dry-run]` → apply pending migrations
   - `migrations rollback [--to <id>]` → rollback to specified version
   - `migrations verify [--fix]` → check for inconsistencies and auto-fix if safe
2. Migration file structure:
   - `migrations/NNN_name.up.sql` (application SQL)
   - `migrations/NNN_name.down.sql` (rollback SQL)
   - NNN is zero-padded sequence number (001, 002, etc.)
3. State tracking (schema_migrations table):
   - id (unique)
   - name (migration name)
   - applied_at (timestamp)
   - checksum (SHA256 of .up.sql to detect hand-edits)
4. Safety checks:
   - Circular dependencies → error before apply
   - Checksum mismatch → error "file changed post-application"
   - Partial state (applied but not logged) → detect via verify; offer --fix to reapply down and up
   - Concurrent applies → pessimistic locking (BEGIN IMMEDIATE, check version, then apply)
   - Large migration timeout → query `pg_stat_activity` to show progress
5. Dry-run mode:
   - Show what would be applied without executing
   - Estimate transaction time (heuristic based on file size)

**Patterns to Follow:**
- `library/social-and-messaging/multimail/internal/store/schema_version_test.go` (migration testing patterns)
- Flow analysis findings: checksum validation, concurrent safety, circular dependency detection, partial state recovery

**Test Scenarios:**
- List migrations → shows all .sql files in migrations/ directory
- Status → pending migrations show as "pending", applied show as "applied"
- Apply → runs all pending migrations in order, logs to schema_migrations
- Dry-run → shows migrations that would be applied, does not execute
- Rollback → runs down SQL in reverse order
- Circular dependency (M2→M1, M3→M2, M1→M3) → error before apply
- Checksum mismatch (file changed) → error "cannot re-apply; file changed"
- Partial state (up ran, down failed) → verify detects; --fix reruns down
- Concurrent applies from two instances → pessimistic lock ensures serialization
- Large migration → timeout with progress indicator
- Verify integrity → checks schema_migrations consistency with applied schema

**Verification:**
1. `migrations list` shows accurate migration list
2. `migrations apply` applies all pending, none already applied
3. Circular dependencies are detected before apply
4. Checksums prevent re-application of modified files
5. Tests include concurrent safety, partial state recovery, rollback scenarios

---

### U7. Query Execution (Safe Query Interface)

**Goal:** Implement a safe SQL query execution command. Allow users to run queries with guardrails against accidental data mutations.

**Requirements:**
- Execute SELECT queries and return results in output modes
- Block mutations (INSERT, UPDATE, DELETE) and DDL (CREATE, ALTER, DROP)
- Block transaction control (BEGIN, COMMIT, ROLLBACK)
- Support parameterized queries to prevent SQL injection
- Return results in JSON, CSV, table, plain, quiet modes

**Dependencies:** U1 (client, config), U2 (doctor)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/query.go`
- `library/developer-tools/postgresql-admin/internal/cli/query_test.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add query command)

**Approach:**
1. Implement `query execute` subcommand:
   - `postgresql-admin query execute "SELECT * FROM users LIMIT 10"`
   - `postgresql-admin query execute "SELECT id, name FROM users WHERE id = ?" --params "123"`
2. Query safety:
   - Parse query (simple lexer, not full parser)
   - Allowlist: SELECT, EXPLAIN (ANALYZE), introspection functions (pg_tables, information_schema, etc.)
   - Blocklist: INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, BEGIN, COMMIT, ROLLBACK, TRUNCATE, GRANT, REVOKE
   - Reject if blocklisted keyword found at statement start
3. Parameterized queries:
   - Use `?` placeholders
   - `--params "val1|val2|val3"` to pass parameter values
   - Prevent SQL injection via parameterization (pgx.Row.Query with args)
4. Result formatting:
   - Introspect result columns (name, type)
   - JSON: array of objects with column names as keys
   - CSV: header row + data rows
   - Table: formatted columns
   - Plain: tab-separated
   - Quiet: one value per line

**Patterns to Follow:**
- Docker-hub pattern for command structure
- Flow analysis findings: safe query allowlist

**Test Scenarios:**
- `query execute "SELECT * FROM users"` → results in selected output format
- `query execute "SELECT id, name FROM users WHERE id = ?" --params "123"` → parameterized, safe from injection
- `query execute "INSERT INTO users VALUES (...)"` → blocked; error "mutations not allowed via query command"
- `query execute "CREATE TABLE t (id INT)"` → blocked; error "DDL not allowed"
- `query execute "BEGIN"` → blocked; error "transaction control not allowed"
- Output with --json → array of objects with column names
- Output with --csv → CSV with header row
- Output with --table → formatted columns (human-readable)
- Output with --plain → tab-separated
- Output with --quiet → one value per line (useful for piping IDs)

**Verification:**
1. Safe SELECT queries execute successfully
2. Parameterized queries prevent SQL injection
3. Mutations, DDL, and transaction control are blocked
4. Results format correctly in all output modes
5. Tests include parameterization, injection prevention, blocklist validation

---

### U8. Backup and Restore

**Goal:** Implement backup creation and restore operations. Wrap `pg_dump` and `pg_restore` for safe, standard backup management.

**Requirements:**
- Create backups in multiple formats (custom, tar, plaintext)
- List existing backups
- Verify backup integrity
- Restore from backups
- Handle selective restore (schema/table selection)
- Validate capacity before backup

**Dependencies:** U1 (client, config), U2 (doctor)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/backup.go`
- `library/developer-tools/postgresql-admin/internal/cli/backup_test.go`

**Files to Modify:**
- `library/developer-tools/postgresql-admin/internal/cli/root.go` (add backup command)

**Approach:**
1. Implement subcommands:
   - `backup create [--format {custom,tar,plaintext}] [--output <path>]` → creates backup via pg_dump
   - `backup list` → shows existing backups (size, created at, verified status)
   - `backup verify <backup_file>` → validates backup integrity (pg_restore --list, checksum)
   - `backup restore <backup_file> [--target-db <name>] [--schema <schema>]` → restore via pg_restore
2. Backup creation:
   - Spawn `pg_dump` subprocess with flags (format, compression, parallel workers)
   - Stream output to file with atomic write (write to temp file, then rename)
   - Pre-check disk space (`--check-disk-space`); error if insufficient
   - Show progress (bytes written, duration)
3. Backup verification:
   - `pg_restore --list <backup>` to validate format
   - Checksum file for corruption detection
   - Optional restore-to-temp validation (full restore to temp DB, then drop)
4. Backup restoration:
   - Spawn `pg_restore` subprocess
   - Handle selective restore (--schema flag)
   - Warn if target DB exists; prompt for replace/abort/create-as-backup_restore
   - Track applied roles and objects (report "X FK skipped (target not in scope)" for selective)
5. Error handling:
   - Disk full during backup → error with "required X GB, available Y GB"
   - Corrupted backup → error with "CRC mismatch at offset N; restore likely to fail"
   - Missing backup dependency (incremental chain) → error "depends on backup X (missing)"

**Patterns to Follow:**
- Flow analysis findings: capacity checks, selective restore, corruption detection

**Test Scenarios:**
- Create backup (custom format, gzip) → produces valid backup file
- Create backup (check disk space) → error if insufficient space
- Verify backup → succeeds if valid, fails if corrupted
- Restore backup to new DB → all objects restored
- Restore backup to existing DB → prompt for replace/abort (with --force flag to skip)
- Selective restore (--schema public) → only public schema restored; FK warnings for external refs
- List backups → shows size, created at, verified status
- Incremental backup chain (full + incremental) → incremental depends on full
- Restore with missing dependency → error "depends on backup X (missing)"
- Backup with timeout/disk full → graceful error; partial file cleaned up

**Verification:**
1. Backups are created successfully and are valid pg_dump format
2. Backups can be restored to new databases
3. Selective restore works correctly
4. Disk space is validated before backup
5. Tests include incremental backups, selective restore, corruption scenarios

---

### U9. Documentation and Printing Press Integration

**Goal:** Write SKILL.md, README.md, and AGENTS.md documentation. Create `.printing-press.json` manifest and archive research/proof manuscripts. Ensure CLI is ready for Printing Press publication.

**Requirements:**
- SKILL.md: agent-facing documentation with flags, examples, auth setup
- README.md: human-facing documentation with install, quickstart, examples
- AGENTS.md: local development guide
- .printing-press.json: provenance manifest
- .manuscripts/: research findings and proof screenshots
- dogfood-results.json: structural validation results

**Dependencies:** U1-U8 (all other units must be complete)

**Files to Create:**
- `library/developer-tools/postgresql-admin/SKILL.md`
- `library/developer-tools/postgresql-admin/README.md`
- `library/developer-tools/postgresql-admin/AGENTS.md`
- `library/developer-tools/postgresql-admin/.printing-press.json` (final manifest)
- `library/developer-tools/postgresql-admin/.manuscripts/<run-id>/research/` (edge cases, design rationale)
- `library/developer-tools/postgresql-admin/.manuscripts/<run-id>/proofs/` (sample command outputs)

**Approach:**
1. **SKILL.md** (agent-facing):
   - Prerequisites: PostgreSQL 11+, pgx driver, config setup
   - When to Use: "When you need to introspect, manage roles, monitor replication, or execute queries against PostgreSQL"
   - Command Reference: section per command (doctor, schema, roles, migrations, replication, query, backup)
   - Auth Setup: describe POSTGRESQL_HOST, POSTGRESQL_PASSWORD env vars
   - Agent Mode: document --agent, --select, --deliver flags
   - Output Modes: JSON, CSV, table, plain, quiet examples
   - Exit Codes: document 0, 2, 3, 4, 5, 7, 10
   - Include examples of each command with expected output
2. **README.md** (human-facing):
   - What it does: brief description
   - Installation: npm installer, pre-built binary, build from source
   - Quick Start: doctor command, schema list, roles list
   - Examples: realistic workflows (introspect schema, create role, apply migrations, backup)
   - Authentication: how to set up credentials
   - MCP Server (if applicable)
3. **AGENTS.md** (local development):
   - Local operating contract (doctor command verifies connectivity)
   - Development setup (go build, go test, docker postgres for testing)
   - Customization notes (if any)
   - Defers to README/SKILL for full documentation
4. **.printing-press.json** (manifest):
   ```json
   {
     "schema_version": 1,
     "generated_at": "ISO8601",
     "printing_press_version": "3.9.0",
     "api_name": "postgresql-admin",
     "display_name": "PostgreSQL Admin",
     "cli_name": "postgresql-admin-pp-cli",
     "creator": {"handle": "kieran", "name": "Kieran"},
     "spec_format": "internal",
     "spec_source": "internal-yaml",
     "auth_type": "password",
     "auth_env_vars": ["POSTGRESQL_HOST", "POSTGRESQL_PORT", "POSTGRESQL_USER", "POSTGRESQL_PASSWORD", "POSTGRESQL_DBNAME"],
     "novel_features": [
       {"name": "doctor", "command": "doctor", "description": "Validate connectivity and permissions"},
       {"name": "safe query execution", "command": "query execute", "description": "Execute SELECT queries with mutation guardrails"}
     ]
   }
   ```
5. **.manuscripts/**:
   - Research: document design decisions, edge cases discovered, patterns followed
   - Proofs: capture sample `--help` output, example command runs with realistic data

**Patterns to Follow:**
- `library/developer-tools/docker-hub/SKILL.md` (structure, content depth)
- `library/developer-tools/docker-hub/README.md` (human-facing examples)
- `library/developer-tools/docker-hub/.printing-press.json` (manifest format)

**Test Scenarios:**
- SKILL.md exists and contains all commands + examples
- README.md has installation instructions and quickstart
- AGENTS.md describes local operating contract
- .printing-press.json has required fields (api_name, cli_name, spec_format, auth_type)
- .manuscripts/ directory has research and proofs subdirectories
- dogfood-results.json shows validation passed (skills, path checks, etc.)

**Verification:**
1. SKILL.md is readable and complete
2. README.md has clear installation and usage examples
3. .printing-press.json is valid JSON with required fields
4. .manuscripts/ contains research and proof artifacts
5. All files follow Printing Press library conventions

---

### U10. Testing and Validation

**Goal:** Implement comprehensive test coverage. Validate CLI against Printing Press validation workflow (spec-validator, briefing, pre-commit hooks).

**Requirements:**
- Unit tests for config, client, helpers, command logic
- Integration tests with local PostgreSQL (or Docker)
- Edge case coverage (concurrent operations, error recovery, state inconsistency)
- End-to-end tests of major workflows
- Passing all Printing Press validation checks (linting, types, dogfood)

**Dependencies:** U1-U9 (testing happens continuously during development)

**Files to Create:**
- `library/developer-tools/postgresql-admin/internal/cli/*_test.go` (per-command tests)
- `library/developer-tools/postgresql-admin/internal/client/client_test.go`
- `library/developer-tools/postgresql-admin/internal/config/config_test.go`
- `library/developer-tools/postgresql-admin/Makefile` (test targets)
- `library/developer-tools/postgresql-admin/.github/workflows/test.yml` (CI pipeline)

**Approach:**
1. **Unit tests** (no DB required):
   - Config loading: TOML parsing, env var override, validation
   - Output formatting: JSON, CSV, table, plain, quiet modes
   - Error codes: exit code mapping for each error type
   - Query safety: allowlist validation, injection prevention
   - Argument parsing: flag combinations, missing required args
2. **Integration tests** (with PostgreSQL):
   - Use `pgx/pgtest` or spawn `docker run postgres` for test DB
   - Doctor command: connectivity, auth, permission validation
   - Schema introspection: list schemas, describe tables, edge cases
   - Role management: create, grant, revoke, delete
   - Migration: apply, rollback, checksum validation, concurrent safety
   - Replication: status reporting (if replica available)
   - Backup: create, verify, restore
3. **Edge case coverage**:
   - Concurrent queries (3+ goroutines to same database)
   - Partial failures (connection loss mid-operation)
   - Object dropped during introspection
   - Role circular membership
   - Migration checksum mismatch
   - Backup capacity exceeded
   - Timeout handling
4. **End-to-end tests** (major workflows):
   - Complete schema introspection flow
   - Role creation + permission grant workflow
   - Migration apply + rollback workflow
   - Backup create + restore workflow

**Patterns to Follow:**
- `library/social-and-messaging/multimail/internal/store/schema_version_test.go` (migration testing, concurrent safety)
- Flow analysis findings: test scenarios for edge cases

**Test Scenarios:**
- Config: TOML loads, env vars override, missing field errors
- Doctor: connectivity succeeds, auth fails with clear message, permission checks pass
- Schema: list, describe, inspect with edge cases (temp tables, circular FKs, dropped objects)
- Roles: create, list, grant, revoke, delete with circular detection
- Migrations: apply, rollback, checksum validation, concurrent safety
- Backup: create, verify, restore, capacity checks
- Query: safe SELECT allowed, mutations blocked, parameterization works
- Concurrent operations: 3+ goroutines query simultaneously without data corruption
- Connection loss: mid-operation failure recovers gracefully

**Verification:**
1. `go test ./...` passes all tests
2. `go vet ./...` no linting issues
3. `govulncheck ./...` no known vulnerabilities
4. Coverage: at least 70% of code paths exercised by tests
5. CI pipeline passes (GitHub Actions or equivalent)

---

## Key Technical Decisions

See High-Level Technical Design section above for KTD-1 through KTD-10.

---

## Testing Strategy

### Test Coverage by Unit

| Unit | Test Type | Key Scenarios |
|------|-----------|---------------|
| U1 | Unit | Config loading, env override, output routing, error codes |
| U2 | Integration | Connectivity, auth success/fail, permission validation |
| U3 | Integration | Schema list, table describe, edge cases (temp tables, cycles) |
| U4 | Integration | Role creation, permissions, circular membership detection |
| U5 | Integration | Replication status, lag calculation, sync mode |
| U6 | Integration | Migration apply, rollback, checksum, concurrent safety |
| U7 | Unit | Query safety, allowlist validation, injection prevention |
| U8 | Integration | Backup create/restore, capacity checks, selective restore |
| U9 | Manual | Documentation completeness, manifest validity |
| U10 | E2E | Full workflows: introspection → schema edit → migration → verify |

### Execution Posture

**Test-first for core abstractions (U1, U7):** Query safety and output formatting are policy-heavy; test-first clarifies requirements.

**Characterization-first for edge cases (U3, U6):** Schema introspection and migration handling have many MVCC races and state edge cases; characterization tests capture known behaviors.

**Integration-forward for database operations (U2-U6, U8):** Most commands require a live PostgreSQL instance; integration tests are the primary validation path.

---

## Risk Analysis & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|-----------|
| MVCC race in schema introspection | Medium | Incomplete results, confusing UX | Gracefully skip dropped objects; document temp table session-locality |
| Concurrent migration failures | Medium | Data corruption | Pessimistic locking (BEGIN IMMEDIATE); testing includes concurrent safety |
| Backup capacity overflow | Medium | Incomplete backup file | Pre-check disk space before backup; atomic write (temp → rename) |
| Role circular membership | Low | GRANT command fails at DB | Validate before GRANT; test circular detection |
| Replication lag visibility | Medium | User sees stale lag data | Show "last updated N seconds ago"; warn if primary unreachable |
| Query injection via --params | Low | SQL injection in parameterized query | Use pgx parameterization (?) to prevent; test injection scenarios |
| Migration checksum false positive | Low | User unable to re-apply legitimate changes | Hash only the SQL content; ignore file timestamps and whitespace |

---

## Deferred to Follow-Up Work

- Query optimization and plan analysis (users can run EXPLAIN ANALYZE directly)
- Automatic failover orchestration (monitoring only; manual failover via role change)
- Distributed transaction coordination (single cluster focus)
- Interactive SQL shell (use psql for that)
- Advanced backup scenarios (compression algorithms, cloud storage integration)
- Custom backup formats (rely on pg_dump standard formats)
- Multi-cluster management (single cluster at a time)

---

## Verification Checklist

- [ ] U1: Core infrastructure builds and runs
- [ ] U2: Doctor command validates connectivity, auth, permissions
- [ ] U3: Schema introspection lists and describes objects correctly
- [ ] U4: Role management creates, grants, revokes, handles circular membership
- [ ] U5: Replication monitoring reports lag, slots, sync status
- [ ] U6: Migration management applies, rolls back, validates checksums; concurrent safety verified
- [ ] U7: Query execution blocks mutations and DDL; allows safe SELECT
- [ ] U8: Backup/restore creates valid backups, restores correctly, validates capacity
- [ ] U9: Documentation complete (SKILL.md, README.md, AGENTS.md); manifest valid
- [ ] U10: Test suite passes (unit, integration, e2e); linting passes; no known vulns
- [ ] All three Printing Press validation stages pass (spec-validator, briefing, pre-commit)
- [ ] CLI generates and publishes successfully via Printing Press pipeline
- [ ] First-pass CI success: generated CLI passes GitHub checks without rework

