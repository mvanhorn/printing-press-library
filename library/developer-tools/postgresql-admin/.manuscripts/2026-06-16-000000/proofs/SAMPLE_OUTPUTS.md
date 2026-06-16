# PostgreSQL Admin CLI — Sample Command Outputs

This document contains representative output from key commands, demonstrating the CLI's capabilities and output formats.

## doctor Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli doctor

Validating PostgreSQL connectivity and permissions...

✓ TCP connectivity to localhost:5432 successful
✓ PostgreSQL protocol handshake succeeded (PostgreSQL 14.5)
✓ Authentication successful (user: postgres from config.toml)

Checking permissions:
✓ SELECT on pg_tables (schema introspection)
✓ SELECT on pg_attribute (column metadata)
✓ SELECT on pg_index (index discovery)
✓ SELECT on pg_constraint (constraint inspection)
✓ CREATE on public schema (role management, migrations)
✓ EXECUTE on pg_functions (function discovery)

Replication configuration:
✓ WAL level: replica (replication available)
✓ max_wal_senders: 10

All checks passed. Ready to proceed.
```

### JSON Output

```json
{
  "connectivity": {
    "tcp": {"status": "ok", "message": "localhost:5432 reachable"},
    "protocol": {"status": "ok", "message": "PostgreSQL 14.5"},
    "auth": {"status": "ok", "user": "postgres", "source": "config.toml"}
  },
  "permissions": {
    "schema_introspection": {"status": "ok"},
    "role_management": {"status": "ok"},
    "migrations": {"status": "ok"},
    "replication": {"status": "ok"}
  },
  "replication_config": {
    "wal_level": "replica",
    "max_wal_senders": 10,
    "replication_available": true
  },
  "overall": "ok"
}
```

---

## schema list Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli schema list

Name              Owner     Type       Description
public            postgres  user       Standard public schema
pg_catalog        postgres  system     PostgreSQL system catalog
information_schema postgres system     ANSI information schema
pg_toast          postgres  system     Toast storage schema
pg_temp           postgres  temporary  Temporary objects
audit_log         app_user  user       Audit trail schema
```

### JSON Output

```json
[
  {
    "Name": "public",
    "Owner": "postgres",
    "Type": "user",
    "Description": "Standard public schema"
  },
  {
    "Name": "pg_catalog",
    "Owner": "postgres",
    "Type": "system",
    "Description": "PostgreSQL system catalog"
  },
  {
    "Name": "information_schema",
    "Owner": "postgres",
    "Type": "system",
    "Description": "ANSI information schema"
  },
  {
    "Name": "audit_log",
    "Owner": "app_user",
    "Type": "user",
    "Description": "Audit trail schema"
  }
]
```

### CSV Output

```
Name,Owner,Type,Description
public,postgres,user,Standard public schema
pg_catalog,postgres,system,PostgreSQL system catalog
information_schema,postgres,system,ANSI information schema
pg_toast,postgres,system,Toast storage schema
pg_temp,postgres,temporary,Temporary objects
audit_log,app_user,user,Audit trail schema
```

---

## schema inspect Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli schema inspect public.users

Table: public.users
Owner: app_user
Size: 2.5 MB
Rows (estimate): 15000

Columns:
  id                      bigint            NOT NULL  nextval('users_id_seq'::regclass)
  email                   text              NOT NULL  -
  username                text              NOT NULL  -
  created_at              timestamp         NOT NULL  now()
  updated_at              timestamp         YES       -
  last_login              timestamp         YES       -
  is_active               boolean           NOT NULL  true

Indexes:
  users_pkey (PRIMARY KEY on id)
  users_email_key (UNIQUE on email)
  idx_users_username (on username)
  idx_users_created_at (on created_at DESC)

Constraints:
  users_pkey: PRIMARY KEY (id)
  users_email_key: UNIQUE (email)
  users_username_not_empty: CHECK (username != '')

Foreign Keys:
  None

Grant Information:
  SELECT: postgres, app_user, analyst
  INSERT: app_user
  UPDATE: app_user
```

### JSON Output (Compact)

```json
{
  "schema": "public",
  "name": "users",
  "owner": "app_user",
  "size_bytes": 2621440,
  "row_estimate": 15000,
  "columns": [
    {
      "name": "id",
      "type": "bigint",
      "nullable": false,
      "default": "nextval('users_id_seq'::regclass)",
      "constraints": ["PRIMARY KEY"]
    },
    {
      "name": "email",
      "type": "text",
      "nullable": false,
      "default": null,
      "constraints": ["UNIQUE"]
    }
  ],
  "indexes": [
    {
      "name": "users_pkey",
      "columns": ["id"],
      "type": "btree",
      "unique": true,
      "primary": true
    },
    {
      "name": "idx_users_created_at",
      "columns": ["created_at"],
      "type": "btree",
      "desc": true
    }
  ],
  "constraints": [
    {"name": "users_pkey", "type": "PRIMARY KEY", "columns": ["id"]},
    {"name": "users_email_key", "type": "UNIQUE", "columns": ["email"]}
  ]
}
```

---

## roles list Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli roles list

Role              Type     Can Login  Can Create  Replication  Member Of
postgres          user     YES        YES         YES          -
app_user          user     YES        NO          NO           app_group
analyst           user     YES        NO          NO           readonly_group
readonly_group    group    NO         NO          NO           -
app_group         group    NO         NO          NO           -
bot_user          user     YES        NO          NO           -
```

### JSON Output

```json
[
  {
    "role": "postgres",
    "type": "user",
    "can_login": true,
    "can_create_db": true,
    "can_create_role": true,
    "replication": true,
    "member_of": []
  },
  {
    "role": "app_user",
    "type": "user",
    "can_login": true,
    "can_create_db": false,
    "can_create_role": false,
    "replication": false,
    "member_of": ["app_group"]
  },
  {
    "role": "readonly_group",
    "type": "group",
    "can_login": false,
    "can_create_db": false,
    "can_create_role": false,
    "replication": false,
    "member_of": []
  }
]
```

---

## replication status Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli replication status

Primary: localhost:5432 (role: master)
Connected replicas: 2

Replica          Lag (bytes)  Lag (time)     State           Sync Status
standby-1        1024         2.5 ms         streaming       async
standby-2        2048         5.1 ms         streaming       sync

Replication Configuration:
  synchronous_standby_names: "standby-2"
  max_wal_senders: 10
  wal_keep_size: 1 GB
```

### JSON Output

```json
{
  "primary": {
    "host": "localhost",
    "port": 5432,
    "role": "master"
  },
  "replicas": [
    {
      "application_name": "standby-1",
      "state": "streaming",
      "sync_state": "async",
      "sync_priority": null,
      "lsn": "0/6B8F308",
      "lag_bytes": 1024,
      "lag_seconds": 0.0025,
      "connected_since": "2026-06-15T10:30:00Z",
      "client_addr": "192.168.1.51"
    },
    {
      "application_name": "standby-2",
      "state": "streaming",
      "sync_state": "sync",
      "sync_priority": 1,
      "lsn": "0/6B8F000",
      "lag_bytes": 2048,
      "lag_seconds": 0.0051,
      "connected_since": "2026-06-15T11:15:00Z",
      "client_addr": "192.168.1.52"
    }
  ],
  "sync_config": {
    "synchronous_standby_names": "standby-2",
    "expected_sync": 1,
    "connected_sync": 1,
    "write_blocked": false
  }
}
```

---

## migrations list Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli migrations list

ID    Name                        Status    Applied At            Checksum
001   create_users_table          applied   2026-06-01 10:00:00   a1b2c3d4...
002   add_email_unique_index      applied   2026-06-02 11:00:00   e5f6g7h8...
003   create_orders_table         applied   2026-06-05 09:30:00   i9j0k1l2...
004   add_order_status_enum       pending   -                     m3n4o5p6...
005   create_audit_log_table      pending   -                     q7r8s9t0...
```

### JSON Output

```json
[
  {
    "id": 1,
    "name": "create_users_table",
    "status": "applied",
    "applied_at": "2026-06-01T10:00:00Z",
    "checksum": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
    "up_file": "migrations/001_create_users_table.up.sql",
    "down_file": "migrations/001_create_users_table.down.sql"
  },
  {
    "id": 2,
    "name": "add_email_unique_index",
    "status": "applied",
    "applied_at": "2026-06-02T11:00:00Z",
    "checksum": "e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0",
    "up_file": "migrations/002_add_email_unique_index.up.sql",
    "down_file": "migrations/002_add_email_unique_index.down.sql"
  },
  {
    "id": 4,
    "name": "add_order_status_enum",
    "status": "pending",
    "applied_at": null,
    "checksum": null,
    "up_file": "migrations/004_add_order_status_enum.up.sql",
    "down_file": "migrations/004_add_order_status_enum.down.sql"
  }
]
```

---

## query execute Command

### SELECT Query - JSON Output

```
$ postgresql-admin-pp-cli query execute "SELECT id, name, email FROM users LIMIT 3" --json

[
  {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com"
  },
  {
    "id": 2,
    "name": "Bob",
    "email": "bob@example.com"
  },
  {
    "id": 3,
    "name": "Charlie",
    "email": "charlie@example.com"
  }
]
```

### SELECT Query - Human-Readable Table

```
$ postgresql-admin-pp-cli query execute "SELECT id, name, email FROM users LIMIT 3"

id  name     email
--  -------  --------------------
1   Alice    alice@example.com
2   Bob      bob@example.com
3   Charlie  charlie@example.com
```

### EXPLAIN ANALYZE Output

```
$ postgresql-admin-pp-cli query execute "EXPLAIN ANALYZE SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '1 day'"

Seq Scan on orders  (cost=0.00..1234.56 rows=5678 width=512) (actual time=10.25..45.32 rows=4123 loops=1)
  Filter: (created_at > (now() - '1 day'::interval))
  Rows Removed by Filter: 4567
Planning Time: 0.234 ms
Execution Time: 45.678 ms
```

### Parameterized Query

```
$ postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE id = ?" --params "123" --json

[
  {
    "id": 123,
    "name": "User 123",
    "email": "user123@example.com",
    "created_at": "2025-01-15T10:30:00Z",
    "is_active": true
  }
]
```

### Blocked Query (Mutation)

```
$ postgresql-admin-pp-cli query execute "INSERT INTO users (name, email) VALUES ('Test', 'test@example.com')"

Error: Mutations not allowed via query command
  SQL mutation (INSERT) is blocked for safety
  Use psql for full SQL access, or use migrations for schema changes
Exit code: 2
```

---

## backup create Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli backup create --format custom --output /backups/db-20260615.backup

Creating backup...
  Format: custom
  Output: /backups/db-20260615.backup
  Compression: gzip (default)
  Parallel workers: 1

Checking disk space...
  Required: 4.2 GB
  Available: 250 GB
  ✓ Sufficient space

Backup progress:
  [████████████████████████████████████████] 100%
  Size: 4.1 GB
  Duration: 1m 45s

Backup created successfully.
```

### JSON Output

```json
{
  "status": "success",
  "backup": {
    "path": "/backups/db-20260615.backup",
    "format": "custom",
    "compression": "gzip",
    "size_bytes": 4398046511,
    "created_at": "2026-06-15T12:00:00Z",
    "duration_seconds": 105
  }
}
```

---

## backup list Command

### Human-Readable Output

```
$ postgresql-admin-pp-cli backup list

Filename                     Size (MB)  Created At            Verified
backup-20260601-100000.bak   4102.3     2026-06-01 10:00:00   YES
backup-20260605-090000.bak   4156.7     2026-06-05 09:30:00   YES
backup-20260610-150000.bak   4089.2     2026-06-10 15:00:00   PENDING
backup-20260615-120000.bak   4341.1     2026-06-15 12:00:00   PENDING
```

---

## backup restore Command

### Dry-Run Output

```
$ postgresql-admin-pp-cli backup restore /backups/db-20260615.backup --target-db test_restore --dry-run --json

{
  "action": "restore",
  "status": "dry_run",
  "backup": "/backups/db-20260615.backup",
  "target_db": "test_restore",
  "will_create_database": true,
  "objects_to_restore": {
    "schemas": 5,
    "tables": 23,
    "indexes": 45,
    "sequences": 8,
    "functions": 12,
    "views": 6
  },
  "estimated_duration_seconds": 120
}
```

### Actual Restore Output

```
$ postgresql-admin-pp-cli backup restore /backups/db-20260615.backup --target-db test_restore

Restoring from /backups/db-20260615.backup...
  Format: custom
  Target database: test_restore
  Creating database... ✓

Restore progress:
  Schemas: 5/5 ✓
  Tables: 23/23 ✓
  Indexes: 45/45 ✓
  Sequences: 8/8 ✓
  Functions: 12/12 ✓
  Views: 6/6 ✓

Restore completed successfully.
  Duration: 1m 42s
  Objects restored: 99
```

---

## Error Examples

### Connection Error

```
$ POSTGRESQL_HOST=nonexistent.example.com postgresql-admin-pp-cli doctor

Error: Cannot reach PostgreSQL at nonexistent.example.com:5432
  dial: connection refused

Check:
  1. Is the host correct? (current: nonexistent.example.com)
  2. Is PostgreSQL running on port 5432?
  3. Is there a firewall blocking the connection?

Exit code: 7 (timeout)
```

### Authentication Error

```
$ POSTGRESQL_PASSWORD=wrong postgresql-admin-pp-cli doctor

Error: Authentication failed
  FATAL: password authentication failed for user "postgres"

Check:
  1. Is the password correct?
  2. Is the username correct? (current: postgres)
  3. Does the user have login privilege?

Exit code: 4
```

### Not Found Error

```
$ postgresql-admin-pp-cli schema inspect public.nonexistent_table

Error: Table not found: public.nonexistent_table

Available tables in public schema:
  users
  orders
  products

Exit code: 3
```

### Permission Error

```
$ postgresql-admin-pp-cli roles create new_role

Error: Permission denied
  You lack the CREATE ROLE privilege

Ask your database administrator to grant the CREATE ROLE privilege:
  GRANT CREATE ROLE ON DATABASE mydb TO current_user;

Exit code: 4
```

---

## Output with --agent Flag

### Schema List - Agent Mode

```
$ postgresql-admin-pp-cli schema list --agent

[{"Name":"public","Type":"user"},{"Name":"pg_catalog","Type":"system"},{"Name":"information_schema","Type":"system"}]
```

All formatting condensed:
- JSON format (--json)
- Minimal fields (--compact)
- No ANSI colors
- No interactive prompts

---

## Version and Help

### --version Output

```
$ postgresql-admin-pp-cli --version

postgresql-admin-pp-cli 2026.6.1
```

### --help Output

```
$ postgresql-admin-pp-cli --help

Manage PostgreSQL databases via the PostgreSQL API.

Add --agent to any command for JSON output + non-interactive mode.
Run 'postgresql-admin-pp-cli doctor' to verify connectivity and permissions.

Usage:
  postgresql-admin-pp-cli [command]

Available Commands:
  backup        Create, verify, and restore backups
  completion    Generate the autocompletion script for the specified shell
  doctor        Validate connectivity and permissions
  help          Help about any command
  migrations    Manage database migrations
  query         Execute safe SQL queries
  replication   Monitor replication status
  roles         Manage roles and permissions
  schema        Explore PostgreSQL schema objects

Flags:
  -h, --help                 help for postgresql-admin-pp-cli
      --agent                Set all agent-friendly defaults (--json --compact --no-input --no-color --yes)
      --config string        Config file path
      --csv                  Output as CSV (table and array responses)
      --json                 Output as JSON
      --no-color             Disable colored output
      --no-input             Disable all interactive prompts (for CI/agents)
      --plain                Output as plain tab-separated text
      --quiet                Bare output, one value per line
      --select string        Comma-separated fields to include in output
      --timeout duration     Request timeout (default 30s)
  -v, --version              version for postgresql-admin-pp-cli
      --yes                  Skip confirmation prompts (for agents and scripts)

Use "postgresql-admin-pp-cli [command] --help" for more information about a command.
```
