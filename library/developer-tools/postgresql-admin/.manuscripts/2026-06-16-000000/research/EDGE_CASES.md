# PostgreSQL Admin CLI — Edge Cases and Discoveries

## Schema Introspection Edge Cases

### 1. Objects Dropped During Introspection

**Scenario:** A table is dropped by a concurrent session while we're querying its metadata.

**Behavior:**
- Schema list query succeeds (pg_namespace is transactional per-namespace)
- Table describe query skips dropped table
- Table inspect query returns error: "table not found (dropped during introspection)"
- Output includes warning: "Note: 1 object dropped during introspection; result may be incomplete"

**Implementation:**
```go
// Graceful skip with warning
rows, err := db.Query("SELECT ... FROM pg_class WHERE relname = ?", tableName)
if err != nil {
    if isNotFoundError(err) {
        // Skip with warning
        continue
    }
    return err
}
```

**Test Case:**
- Start introspection on a schema with 10 tables
- Parallel goroutine drops a table mid-query
- Verify: introspection completes with 9 tables + warning message

### 2. Temporary Tables (Session-Local)

**Scenario:** User creates a temp table in pg_temp schema (session-specific).

**Behavior:**
- Schema list shows `pg_temp` as "temporary" type
- Table describe shows temp tables with warning: "Temporary tables are session-local and may not exist in other sessions"
- inspect on temp table succeeds but warns: "Temporary table; contents are local to your session"

**Implementation:**
- Detect schema name starting with `pg_temp`
- Add warning flag to output schema
- Metadata still accurate for current session

**Test Case:**
- Create temp table: `CREATE TEMP TABLE t1 (id INT);`
- Run `schema describe pg_temp` → shows t1 with temp warning
- Exit session → temp table gone
- New session: `schema describe pg_temp` → t1 not listed

### 3. Circular Foreign Keys

**Scenario:** Table A references table B; table B references table A.

**Behavior:**
- Schema inspect on table A lists: "FK to B (b_id)"
- Schema inspect on table B lists: "FK to A (a_id)"
- Introspection terminates cleanly (no infinite loop)

**Implementation:**
- Follow FK graph with visited-set tracking
- Output shows cycle markers: "→ B (id) → A (id) [cycle detected]"

**Test Case:**
```sql
CREATE TABLE a (id SERIAL PRIMARY KEY, b_id INT);
CREATE TABLE b (id SERIAL PRIMARY KEY, a_id INT REFERENCES a(id));
ALTER TABLE a ADD CONSTRAINT fk_a_b FOREIGN KEY (b_id) REFERENCES b(id);

postgresql-admin-pp-cli schema inspect public.a
# Output shows cycle marker
```

### 4. Partitioned Tables

**Scenario:** Table is partitioned (range, hash, list).

**Behavior:**
- Inspect shows partition key: "Partitioned by: range on (created_at)"
- Lists all partitions: "p_2026_q1, p_2026_q2, ..."
- Inspecting a partition shows parent: "Partition of: orders_archive"

**Implementation:**
- Query `pg_partitioned_table` for partition info
- Follow parent/child relationships

**Test Case:**
```sql
CREATE TABLE orders_archive (
  id BIGINT, 
  created_at TIMESTAMP
) PARTITION BY RANGE (created_at);

CREATE TABLE p_2026_q1 PARTITION OF orders_archive 
  FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');

postgresql-admin-pp-cli schema inspect public.orders_archive
# Shows partition structure
```

## Role Management Edge Cases

### 5. Circular Role Membership

**Scenario:** Role A is member of role B; role B is member of role A.

**Behavior:**
- `roles grant --role A --parent B` when B is already descendant of A → error before DB attempt
- Error message: "Circular membership detected: A → C → B → A"
- No modification to database

**Implementation:**
- Build transitive closure of pg_auth_members
- Check for cycles before executing GRANT
- Report cycle path for debugging

**Test Case:**
- Create roles: `CREATE ROLE role_a; CREATE ROLE role_b;`
- Grant A ← C ← B ← A cycle
- `roles grant --role B --parent C` when C is already member of B → detect cycle

### 6. Cascading Role Deletions

**Scenario:** Role has dependent objects (privileges, memberships).

**Behavior:**
- `roles delete cascade_victim` (no --cascade) → error: "Role has 3 dependent objects"
- `roles delete cascade_victim --cascade` → lists objects to be dropped, prompts for confirmation
- Actual DROP CASCADE executed only after confirmation

**Implementation:**
- Query for dependent objects before delete
- Report objects: owned tables, owned schemas, memberships
- Require `--yes` or interactive confirmation
- Execute `DROP ROLE ... CASCADE`

### 7. Login-Less Groups

**Scenario:** Role created without CANLOGIN (group role).

**Behavior:**
- `roles list` shows `can_login: false` for group role
- Group can be parent (A can be member of group_b)
- `roles create group_name` defaults to `--no-login` (unless `--can-login` explicitly passed)

**Implementation:**
- Query `rolcanlogin` from pg_roles
- Show in list output
- Default behavior respects user intent

### 8. Replication Role

**Scenario:** Role created with REPLICATION privilege.

**Behavior:**
- `roles create repl_user --replication` sets the REPLICATION attribute
- `roles list` shows `replication: true` for that role
- Role can initiate replication even without table privileges

**Implementation:**
- Support `--replication` flag in roles create
- Query `rolreplication` from pg_roles

## Migration Management Edge Cases

### 9. Checksum Mismatch (File Changed Post-Application)

**Scenario:** User edits migration file after it was applied.

**Behavior:**
- `migrations list` shows "✗ checksum mismatch" for that migration
- `migrations verify` reports: "Migration 002_add_index.up.sql: file changed post-application"
- `migrations apply` refuses to re-apply: error 2

**Implementation:**
- Store SHA256(up.sql) in schema_migrations table
- Compare stored checksum with computed from disk file
- If mismatch: error with guidance

**Test Case:**
- Apply migration 002
- Edit 002_add_index.up.sql (add a comment)
- Run `migrations verify` → detects mismatch

### 10. Partial State (Up Executed, Down Failed)

**Scenario:** Migration apply succeeded (up.sql ran); down.sql later fails.

**Behavior:**
- Database schema changed by up.sql
- schema_migrations table shows migration as applied (checksum stored)
- `migrations rollback` tries to run down.sql → fails
- `migrations verify --fix` detects inconsistency; offers to re-run down

**Implementation:**
- Run down in transaction; if fails, report: "Rollback failed for X"
- Suggest manual intervention or re-run with diagnostic output

### 11. Concurrent Migration Applies

**Scenario:** Two instances try to apply migrations simultaneously.

**Behavior:**
- Instance 1: `BEGIN IMMEDIATE`, checks schema_migrations
- Instance 2: `BEGIN IMMEDIATE`, checks schema_migrations
- Instance 1: applies migration, commits
- Instance 2: checks again; sees migration applied by instance 1
- Instance 2: skips already-applied migrations, applies new ones if any

**Implementation:**
- Before each migration: re-check schema_migrations (in same transaction)
- If migration already applied: skip with info message
- Pessimistic lock prevents race condition

**Test Case:**
- Two CLI instances with same config
- Both run `migrations apply`
- Verify: both complete successfully with no duplicates

## Query Execution Edge Cases

### 12. SQL Injection Attempt

**Scenario:** User tries to bypass guardrails with injection.

**Behavior:**
```bash
# Attempt 1: Direct mutation
postgresql-admin-pp-cli query execute "DELETE FROM users"
# Error: Mutations not allowed. Use migrations instead.

# Attempt 2: Parameterized injection
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE id = ?" --params "123; DROP TABLE users; --"
# Works correctly (params are NOT interpreted as SQL)

# Attempt 3: Literal injection
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE id = '123'; DROP TABLE users; --'"
# Error: Blocked statement detected (DROP at statement boundary)
```

**Implementation:**
- Allowlist-based guard (not regex-based)
- Parameterized queries prevent SQL injection
- Statement parser rejects at statement boundary only (not in string literals)

### 13. Timeout on Long-Running Query

**Scenario:** Query takes longer than timeout.

**Behavior:**
- Default timeout: 30 seconds (configurable via --timeout)
- Query executing exceeds timeout
- Error: "Query timeout (30s); consider using EXPLAIN ANALYZE to diagnose performance"
- Exit code: 7 (timeout)

**Implementation:**
- Set query context deadline: `context.WithTimeout(ctx, flags.timeout)`
- Catch context.DeadlineExceeded
- Report estimated execution time if available

### 14. No Results

**Scenario:** Query returns zero rows.

**Behavior:**
- JSON output: `[]` (empty array)
- Table output: shows header only, "No results" message
- Plain output: empty (no output)
- Quiet output: empty (no output)
- Exit code: 0 (success, not an error)

## Backup and Restore Edge Cases

### 15. Disk Space Exhausted During Backup

**Scenario:** pg_dump encounters disk full.

**Behavior:**
- Pre-check: "Required 5.2 GB, available 2.0 GB; backup canceled"
- During backup: if disk fills mid-write, pg_dump subprocess exits with error
- Cleanup: temp backup file deleted (atomic write)
- Error returned with guidance: "Insufficient disk space; free space and retry"

**Implementation:**
- Pre-check via `statvfs()` or equivalent
- Write to temp file (e.g., `/tmp/backup.custom.tmp`)
- Only rename to final path on success
- Clean up temp file on error

### 16. Backup Format Validation

**Scenario:** User tries to restore from corrupted backup.

**Behavior:**
```bash
postgresql-admin-pp-cli backup verify corrupted.backup
# Error: "Backup file is corrupted; header mismatch at offset 0"
```

**Implementation:**
- `pg_restore --list` reads backup header and TOC
- If header is invalid: error immediately
- Checksum validation on custom format

### 17. Selective Restore with Missing Dependencies

**Scenario:** Restoring only public schema, but table has FK to pg_catalog (rare but possible).

**Behavior:**
- Restore proceeds for public schema objects
- FKs to external schemas: warning "Skipped X foreign keys (target not in scope)"
- Restore completes with warnings; exit code 0
- Output: `{"restored": 15, "skipped": 2, "warnings": ["..."]}`

**Implementation:**
- Capture pg_restore output
- Parse for constraint warnings
- Aggregate and display

### 18. Restore to Existing Database

**Scenario:** Target database exists; user runs restore without --force.

**Behavior:**
- Prompt: "Database 'test_db' already exists. (a)bort, (b)ackup and replace, or (c)reate as 'test_db_restore'?"
- Interactive mode: prompts; agent mode (--no-input): error
- With --force: silently replaces (after backing up original to test_db_<timestamp>.backup)

**Implementation:**
- Check if database exists: `SELECT 1 FROM pg_database WHERE datname = ?`
- Prompt user for action
- Create backup before replace if --force

## Replication Edge Cases

### 19. Cascading Replication (Replica→Replica→Replica)

**Scenario:** Primary has replica R1; R1 has replica R2.

**Behavior:**
- `replication status` on primary: shows R1 connected
- Does NOT show R2 (it's not directly connected to primary)
- `replication status` on R1: shows R2 connected
- Guidance: "For cascading replication status, run on each replica"

**Implementation:**
- Query only pg_stat_replication (direct connections to this instance)
- Document cascading topology limitation
- Suggest running command on intermediate replica for full view

### 20. Replication Slot Retention > Disk Free

**Scenario:** Replication slot retains more WAL than available disk.

**Behavior:**
- `replication slots` shows: "Slot 'replica_slot' has 2000 MB retained WAL; disk has 500 MB free"
- Warning: "Capacity at risk; slot may auto-drop or primary may deadlock"
- Exit code: 0 (not fatal yet, but warning)

**Implementation:**
- Query `pg_wal_lsn_diff()` for WAL bytes
- Check available disk with `df` or equivalent
- Compare and warn if close

## Performance Considerations

### 21. Large Schema Introspection

**Scenario:** Database has 1000+ tables; user runs `schema describe public`.

**Behavior:**
- Query completes (pg_class query is indexed)
- Output: JSON array with 1000+ entries (may be large, ~500 KB+)
- Compact mode: reduces size (no descriptions, fewer fields)
- Agent mode: automatic --compact → smaller output

**Implementation:**
- Queries are indexed (no full table scans)
- Pagination not needed (CLIs typically for exploration, not streaming)
- Client responsible for piping to `less` or `jq` if needed

### 22. Large Migration Rollback

**Scenario:** Rolling back 50 migrations.

**Behavior:**
- Each down.sql executed in reverse order
- Pessimistic lock held for entire rollback (blocks other operations)
- Progress: "Applied rollback 1/50, 2/50, ..." printed to stderr
- Total time: depends on down.sql complexity (seconds to minutes)

**Implementation:**
- Execute in single transaction (all-or-nothing)
- Progress updates via stderr
- No cancel support (use DB timeout if stuck)

## Authentication Edge Cases

### 23. Password with Special Characters

**Scenario:** Password contains `"`, `\`, or other special characters.

**Behavior:**
- Via env var: shell escaping handles it (use single quotes)
- Via config file: TOML escaping (use triple quotes for complex passwords)
- Via stdin: prompts for password, reads as-is

**Examples:**
```bash
# Env var: single quotes prevent shell interpolation
export POSTGRESQL_PASSWORD='pass"word\with$pecial'

# Config file: TOML string escaping
password = "pass\"word\\with$pecial"

# Interactive prompt: reads as-is
postgresql-admin-pp-cli doctor
# Enter password: pass"word\with$pecial
```

### 24. User Without Replication Privilege Checking Replication Status

**Scenario:** User lacks permission to query pg_stat_replication.

**Behavior:**
- `replication status` → error 4: "Permission denied; user lacks privileges to query replication views"
- Guidance: "Ask database administrator to grant pg_monitor role to your user"

**Implementation:**
- Catch permission error from pg_stat_replication query
- Map to exit code 4 (auth error)

## Summary

These edge cases are discovered and handled during:
1. **Unit testing** — config parsing, output formatting
2. **Integration testing** — with real PostgreSQL instances
3. **Manual testing** — edge case scenarios from the plan
4. **Dogfood testing** — Printing Press validation workflow

All edge cases include:
- Clear error messages guiding user to resolution
- Appropriate exit codes for automation
- JSON output for programmatic handling
- Test vectors in corresponding `*_test.go` files
