# PostgreSQL Admin CLI — Design Rationale

## Overview

The PostgreSQL Admin CLI (`postgresql-admin-pp-cli`) is a comprehensive command-line interface for PostgreSQL database administration. It was designed to be the first production-grade test case for the Printing Press validation workflow, covering six major capability areas with real-world complexity.

## Design Decisions

### KTD-1: pgx Driver with Connection Pooling

**Decision:** Use `jackc/pgx` (modern, connection pooling, prepared statements) instead of `lib/pq` or ORM frameworks.

**Rationale:** 
- Native Go, no C dependencies
- Better performance and connection pooling
- Explicit control over concurrent behavior
- Prepared statements prevent SQL injection
- Modern error types and context support

**Trade-off:** Requires manual query handling vs. ORM convenience, but gains control and transparency.

### KTD-2: TOML Configuration + Environment Override

**Decision:** Support both `~/.config/postgresql-admin-pp-cli/config.toml` and environment variables, with env vars taking priority.

**Rationale:**
- Follows docker-hub pattern (XDG-compliant directories)
- Environment variables enable CI/test isolation without file changes
- Password never logged; only labeled as "from env:POSTGRESQL_PASSWORD" or "from config"
- Supports both interactive setup (config file) and automation (env vars)

**Edge Cases Handled:**
- Missing password in config → interactive prompt (unless `--no-input`)
- Missing password in env → error with guidance
- Partial config override (host from config, password from env) → works correctly

### KTD-3: Local SQLite Cache for Schema Metadata

**Decision:** Optional local SQLite store at `~/.config/postgresql-admin-pp-cli/store.db` for caching schema metadata.

**Rationale:**
- Enables offline schema introspection (agents can browse cached data without hitting DB)
- Reduces repeated expensive queries (pg_namespace, pg_class, pg_attribute)
- SQLite WAL mode allows concurrent reads while write is in progress
- Mirrors outlook-email pattern for read-only offline capability

**Schema Versioning:**
- Each cache entry includes checksum of query results
- Explicit `schema sync` command to refresh cache
- Stale-data warning in output when using local store

**Trade-off:** Cache invalidation is manual; eventual consistency model.

### KTD-4: Cobra Command Structure with Persistent Flags

**Decision:** Organize commands by resource (schema, roles, migrations, etc.) with persistent flags at root level.

**Rationale:**
- Standardized across all Printing Press CLIs
- Agents expect consistent flag shapes (`--json`, `--csv`, `--agent`)
- `--agent` expands to standard set: `--json --compact --no-input --no-color --yes`
- `--select` for field filtering on agent use
- Per-command help discoverable via `<cmd> --help`

**Structure:**
```
postgresql-admin-pp-cli
├── doctor
├── schema
│   ├── list
│   ├── describe
│   └── inspect
├── roles
│   ├── list
│   ├── create
│   ├── grant
│   └── delete
├── replication
│   ├── status
│   └── slots
├── migrations
│   ├── list
│   ├── apply
│   ├── rollback
│   └── verify
├── query
│   └── execute
└── backup
    ├── create
    ├── list
    ├── verify
    └── restore
```

### KTD-5: Doctor Command as Validation Entrypoint

**Decision:** Implement `doctor` as the first command users run to diagnose setup issues.

**Rationale:**
- Every failure without doctor is a mystery (connection? auth? permissions?)
- Doctor catches 80% of setup issues with clear messaging
- Checks: TCP connectivity, PostgreSQL protocol, auth, user permissions, replication config
- Structured JSON output available for automation

**Example Output:**
```
✓ TCP connectivity to localhost:5432 successful
✓ PostgreSQL protocol handshake succeeded
✓ Authentication successful (user: postgres)
✓ User has required permissions:
  ✓ SELECT on pg_tables (schema introspection)
  ✓ CREATE on public schema (role management, migrations)
  ✓ EXECUTE on pg_functions (function discovery)
```

### KTD-6: Output Modes (JSON, CSV, Table, Plain, Quiet)

**Decision:** Support five output formats on every command.

**Rationale:**
- JSON for agent/script processing (full structured data)
- CSV for spreadsheet import (pivot tables, analysis)
- Table for human-readable interactive use (auto-detected TTY)
- Plain for shell piping (tab-separated, grep/awk friendly)
- Quiet for ID extraction (one value per line)

**Format Selection Priority:**
1. Explicit flag (`--json`, `--csv`, etc.)
2. If none specified and stdout is TTY → human-readable table
3. If none specified and stdout is pipe → plain text

**Compact Mode:**
- `--compact` omits verbose fields (descriptions, comments, timestamps)
- Reduces token usage for agents
- Automatically enabled by `--agent`

### KTD-7: Safe Query Execution with Guardrails

**Decision:** Allow only SELECT, EXPLAIN ANALYZE, and introspection queries; block mutations and DDL.

**Rationale:**
- Prevents accidental data loss in CLI context
- Use `psql` for full SQL access
- Simple lexer-based allowlist is sufficient (not full parser)

**Allowed:**
- SELECT (all variations, CTE, subqueries, etc.)
- EXPLAIN ANALYZE
- Introspection: `information_schema.*`, `pg_*` views/functions

**Blocked at Statement Start:**
- INSERT, UPDATE, DELETE (mutations)
- CREATE, ALTER, DROP (DDL)
- BEGIN, COMMIT, ROLLBACK (transaction control)
- GRANT, REVOKE (privilege changes)

**Parameterized Queries:**
- `?` placeholders in query string
- `--params "val1|val2|val3"` for values
- Uses pgx parameterization to prevent injection

### KTD-8: Migration Model — SQL Files in Git with State Tracking

**Decision:** Two files per migration (up.sql, down.sql) with state tracked in schema_migrations table.

**Rationale:**
- Aligns with standard database migration tools (Flyway, Liquibase, etc.)
- Idempotent by design (each migration self-contained)
- Checksum validation detects post-application hand-edits

**File Naming:**
- `migrations/NNN_name.up.sql` — Application SQL
- `migrations/NNN_name.down.sql` — Rollback SQL
- NNN is zero-padded sequence (001, 002, etc.)

**State Table:**
```sql
CREATE TABLE schema_migrations (
  id BIGINT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  applied_at TIMESTAMP NOT NULL,
  checksum TEXT NOT NULL
);
```

**Checksums:**
- Computed as SHA256(up.sql file contents)
- Detects file modifications after application
- Prevents re-application of legitimately modified migrations

**Concurrency Safety:**
- Pessimistic locking: `BEGIN IMMEDIATE` before checking version
- Ensures serialization on concurrent applies
- Migration partially applied → `verify --fix` recovers state

### KTD-9: Backup Integration with pg_dump/pg_restore

**Decision:** Wrap `pg_dump` and `pg_restore` instead of custom backup logic.

**Rationale:**
- Proven, tested, standard tool
- Avoids reimplementing complex backup logic (consistency, atomicity, streaming)
- Supports multiple formats: custom, tar, plaintext
- Parallel restore for faster recovery

**Subprocess Orchestration:**
- Capture stdout/stderr for streaming and error handling
- Pre-check disk space before backup
- Atomic write: backup to temp file, then rename on success
- Progress tracking (bytes written, duration)

**Selective Restore:**
- `pg_restore --schema` for single-schema restore
- Warning for FK constraints to tables outside schema
- Report "X objects skipped (outside scope)"

### KTD-10: Error Exit Codes (Unix Standard)

**Decision:** Use conventional exit codes for error classification.

**Rationale:**
- Scripts can branch on exit code
- Enables CI/alerting integration
- Consistent across all Printing Press CLIs

**Codes:**
- 0: Success
- 2: Usage error (bad flags, invalid arguments)
- 3: Not found (role doesn't exist, schema not found)
- 4: Authentication error (invalid password, permission denied)
- 5: Database/API error (server error, query failed)
- 7: Timeout
- 10: Configuration error (bad config file, missing credentials)

## Edge Cases and Handling

### MVCC Races in Schema Introspection

**Issue:** Objects can be dropped by concurrent sessions during introspection.

**Solution:**
- Wrap introspection queries with exception handlers
- Gracefully skip dropped objects
- Report "object X dropped during introspection"
- No fatal error; introspection completes with available objects

### Circular Role Membership

**Issue:** `GRANT membership_role TO parent_role` where parent_role is already a member of membership_role.

**Solution:**
- Detect cycle before executing GRANT
- Query `pg_auth_members` transitively
- Error message: "Circular membership detected: A → B → A"
- PostgreSQL will also catch this at DB level; we just provide better UX

### Concurrent Migration Failures

**Issue:** Two instances try to apply migrations simultaneously.

**Solution:**
- Pessimistic locking: `BEGIN IMMEDIATE` then check schema_migrations version
- If version changes → abort with "Migration applied by another instance"
- Retry logic: wait and re-check, up to 5 retries with exponential backoff

### Backup Capacity Overflow

**Issue:** Disk space exhausted during backup creation.

**Solution:**
- Pre-check: compare estimated backup size with available disk
- If insufficient: error with "Required X GB, available Y GB"
- Atomic write: write to temp file; only rename on complete success
- On failure: temp file cleaned up automatically

### Replication Lag Visibility

**Issue:** pg_stat_replication shows stale data if replica is unreachable.

**Solution:**
- Query `write_lsn - replay_lsn` for byte lag
- Query `EXTRACT(EPOCH FROM now() - backend_xmin::timestamp)` for time lag
- If replica unreachable: status shows "unreachable; last contact Xm ago"
- Warning if primary is write-blocked (sync quorum mismatch)

### Query Injection Prevention

**Issue:** User passes `' OR '1'='1` as part of query parameter.

**Solution:**
- Parameterized queries: `?` placeholders in query string
- pgx handles escaping and type conversion
- Parameters never interpolated into SQL string
- Test vectors: `O'Reilly`, `"; DROP TABLE users; --`, `\x00` null byte

## Testing Strategy

### Unit Tests (No DB Required)
- Config loading: TOML parsing, env var override, validation
- Output formatting: JSON, CSV, table, plain, quiet modes
- Error code mapping
- Query safety: allowlist validation
- Argument parsing

### Integration Tests (With PostgreSQL)
- Use `pgx/pgtest` or Docker for test DB
- Doctor command: connectivity, auth, permissions
- Schema introspection: edge cases (temp tables, dropped objects)
- Role management: creation, grants, circular detection
- Migrations: apply, rollback, concurrent safety
- Backup: create, verify, restore, capacity checks

### Edge Case Coverage
- Concurrent queries (3+ goroutines to same database)
- Connection loss mid-operation
- Object dropped during introspection
- Circular role membership
- Migration checksum mismatch
- Backup disk full scenario
- Query timeout handling

## Deferred to Follow-Up Work

- Query optimization (users can run EXPLAIN ANALYZE directly)
- Automatic failover orchestration (monitoring only; manual failover)
- Distributed transaction coordination (single cluster focus)
- Interactive SQL shell (use `psql`)
- Custom backup compression (rely on pg_dump formats)
- Multi-cluster management (single cluster at a time)
