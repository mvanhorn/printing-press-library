# PostgreSQL Admin CLI Agent Guide

This directory is a generated `postgresql-admin-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
postgresql-admin-pp-cli doctor --json
postgresql-admin-pp-cli --help
```

Use runtime discovery instead of relying on a copied command list:

```bash
postgresql-admin-pp-cli which "<capability>" --json
postgresql-admin-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
postgresql-admin-pp-cli schema list --agent
postgresql-admin-pp-cli roles list --agent
postgresql-admin-pp-cli backup create --format custom --output /tmp/backup.custom --agent
```

Before running an unfamiliar command that may mutate remote state (backup, migrations, roles), inspect its help and prefer a dry-run:

```bash
postgresql-admin-pp-cli migrations --help
postgresql-admin-pp-cli migrations apply --dry-run --agent
postgresql-admin-pp-cli backup create --help
postgresql-admin-pp-cli backup create --format custom --output /tmp/test.backup --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear. Database operations are not reversible without a backup.

## Development Setup

### Prerequisites

- Go 1.26.3 or later
- PostgreSQL 11.0 or later (for testing)
- Docker (optional, for isolated test instances)

### Build and Test

From the CLI directory:

```bash
# Build the binary
go build ./cmd/postgresql-admin-pp-cli

# Run unit tests
go test ./internal/cli/... -v

# Run all tests
go test ./... -v

# Lint with golangci-lint
golangci-lint run ./...

# Check for vulnerabilities
govulncheck ./...
```

### Test Database Setup

For integration tests, use a local PostgreSQL instance or Docker:

```bash
# Docker PostgreSQL
docker run --name test-postgres \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_USER=testuser \
  -p 5432:5432 \
  -d postgres:15

# Set environment for tests
export POSTGRESQL_HOST=localhost
export POSTGRESQL_PORT=5432
export POSTGRESQL_USER=testuser
export POSTGRESQL_PASSWORD=testpass
export POSTGRESQL_DBNAME=postgres

# Run tests
go test ./... -v
```

## Local Customizations

This directory is **generated output** — a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

For documentation (README.md, SKILL.md) edits, those changes survive regen-to-regen and are maintained separately. Prefer edits to those files for user-facing changes; use `.printing-press-patches/` only for code-level customizations.

## For Agents Working on This CLI

### Schema Introspection

When users ask to "explore the database", start with:

```bash
postgresql-admin-pp-cli schema list --agent
```

Then narrow down to a specific schema:

```bash
postgresql-admin-pp-cli schema describe public --agent --select name,type
```

Finally, inspect individual tables:

```bash
postgresql-admin-pp-cli schema inspect public.users --agent
```

### Role and Permission Management

To set up user access:

1. Check existing roles:
   ```bash
   postgresql-admin-pp-cli roles list --agent
   ```

2. Create role if needed:
   ```bash
   postgresql-admin-pp-cli roles create read_user --can-login --password pass --agent
   ```

3. Grant permissions:
   ```bash
   postgresql-admin-pp-cli roles grant --role read_user --on public --permissions USAGE --agent
   postgresql-admin-pp-cli roles grant --role read_user --on public.users --permissions SELECT --agent
   ```

### Safe Query Execution

The `query execute` command blocks mutations (INSERT, UPDATE, DELETE) and DDL (CREATE, ALTER, DROP). Use it for SELECT queries:

```bash
# Safe for agents
postgresql-admin-pp-cli query execute "SELECT COUNT(*) FROM users" --agent

# Parameterized (prevents SQL injection)
postgresql-admin-pp-cli query execute "SELECT * FROM users WHERE id = ?" --params "123" --agent

# Not allowed (will error)
postgresql-admin-pp-cli query execute "INSERT INTO users VALUES (...)" --agent  # Error
postgresql-admin-pp-cli query execute "DROP TABLE users" --agent  # Error
```

### Migration Management

When users need to apply migrations:

1. List pending migrations:
   ```bash
   postgresql-admin-pp-cli migrations list --agent
   ```

2. Preview with dry-run:
   ```bash
   postgresql-admin-pp-cli migrations apply --dry-run --agent
   ```

3. Apply with confirmation:
   ```bash
   postgresql-admin-pp-cli migrations apply --agent
   ```

4. Verify integrity after apply:
   ```bash
   postgresql-admin-pp-cli migrations verify --agent
   ```

### Backup and Restore Workflows

For production safety:

1. Always create backups before schema changes:
   ```bash
   postgresql-admin-pp-cli backup create --format custom --output /backups/pre-migration.backup --agent
   ```

2. Verify backup:
   ```bash
   postgresql-admin-pp-cli backup verify /backups/pre-migration.backup --agent
   ```

3. Restore to test database first:
   ```bash
   postgresql-admin-pp-cli backup restore /backups/pre-migration.backup --target-db test_restore --agent
   ```

4. Only then apply changes to production.

### Replication Monitoring

For primary-replica setups:

```bash
# Check replication status
postgresql-admin-pp-cli replication status --agent

# Monitor slots
postgresql-admin-pp-cli replication slots --agent

# Alert if lag exceeds threshold
postgresql-admin-pp-cli replication status --agent | jq '.[] | select(.Lag_ms > 1000)'
```

## Defers to README/SKILL for Full Documentation

This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs. For:

- **Installation instructions**: See README.md
- **Command reference and examples**: See SKILL.md
- **Detailed troubleshooting**: See README.md
- **Authentication setup**: See README.md
- **Output formats and exit codes**: See SKILL.md
