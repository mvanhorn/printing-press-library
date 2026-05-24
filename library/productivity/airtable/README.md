# Airtable CLI

**Every Airtable surface a CLI should have — plus a local SQLite mirror, cross-base SQL, and webhook tooling no SDK ships.**

Airtable's official SDKs stop at REST pass-through; the community CLIs and MCP servers do the same. airtable-pp-cli absorbs every endpoint (records, schema, webhooks, comments, workspaces) with proactive 5-req/sec rate-budget tracking, then adds a local SQLite mirror you can `query` with raw SQL, a `webhooks fleet` view that no existing tool offers, and a `history record` reconstructor that turns webhook payloads into a real audit log.

Printed by [@joelsephus](https://github.com/joelsephus) (joelsephus).

## Install

The recommended path installs both the `airtable-pp-cli` binary and the `pp-airtable` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airtable
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airtable --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airtable --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airtable --agent claude-code
npx -y @mvanhorn/printing-press-library install airtable --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/airtable/cmd/airtable-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airtable-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airtable --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airtable --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-airtable skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-airtable. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airtable-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AIRTABLE_PAT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/airtable/cmd/airtable-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airtable": {
      "command": "airtable-pp-mcp",
      "env": {
        "AIRTABLE_PAT": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `AIRTABLE_PAT` to a personal access token from airtable.com/create/tokens. Recommended scopes: `data.records:read`, `data.records:write`, `schema.bases:read`, `webhook:manage` for full functionality; `user.email:read` for the `whoami` health check. The CLI uses Bearer-token auth on every call and never persists the token to disk — set it in the env or per-profile config.

## Quick Start

```bash
# Health check: confirms AIRTABLE_PAT is set and the CLI builds outgoing requests correctly. Safe to run without network.
airtable-pp-cli doctor --dry-run

# First real call — lists every base your PAT can see. Pages auto-followed.
airtable-pp-cli bases list

# Dump a base's full table+field+view schema as JSON for your README or LLM prompt.
airtable-pp-cli bases get-schema appXXX

# Pull records modified in the last week into a local SQLite mirror so `query`, `analytics`, `stale`, and `traverse` work offline.
airtable-pp-cli sync --resources records --since 7d --db ./airtable.db

# Cross-table SQL against the mirror — the headline differentiator.
airtable-pp-cli query "SELECT COUNT(*) FROM appXXX__customers" --db ./airtable.db

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local mirror that compounds
- **`query`** — Run arbitrary SQL over a local SQLite mirror of one or more Airtable bases. Tables surface as `<base_slug>__<table_slug>`; linked-record arrays are stored as JSON columns you can `json_each()` over.

  _When an agent needs an answer that joins records across two or more tables (or bases) and the user has already synced, this is one local SQL query instead of two paginated REST round-trips plus client-side stitching._

  ```bash
  airtable-pp-cli query "SELECT a.name, COUNT(p.id) FROM accounts__customers a JOIN pipeline__opps p ON p.account_id = a.id GROUP BY a.name" --db ./airtable.db --agent
  ```
- **`changes`** — Joins the synced webhook_payloads table with the current records snapshot to report what fields changed across a base in the last N hours/days, grouped by table, field, or actor.

  _Agents and digest-builders can answer 'what's new since Friday' without setting up a webhook receiver or scraping per-record revision history._

  ```bash
  airtable-pp-cli changes --since 7d --group-by table --db ./airtable.db --agent --select changes.table,changes.field,changes.count
  ```
- **`stale`** — Find records older than a duration, optionally filtered by a singleSelect status field, joined against the local cache's last_modified_time.

  _Drives content-calendar audits, follow-up reminders, and stale-deal sweeps without burning rate-limit budget on a full table scan every time._

  ```bash
  airtable-pp-cli stale appXXX tblYYY --field Status --equals Draft --older-than 30d --db ./airtable.db --json
  ```
- **`traverse`** — Walk linked-record relationships from a starting record up to a depth limit; render as a tree (`--pretty`) or as ndjson edges (`--json`).

  _Lets agents resolve 'what's connected to this record' in one offline call, instead of pagination loops that trip the 5 req/sec rate limit._

  ```bash
  airtable-pp-cli traverse appXXX recYYY --depth 2 --db ./airtable.db --json
  ```
- **`history record`** — Reconstruct a record's edit timeline (field-level diffs in cursor order) from the synced webhook_payloads table.

  _Auditing 'who changed Status from Open to Won, when' becomes a single command instead of a clickthrough in the web UI._

  ```bash
  airtable-pp-cli history record appXXX recYYY --db ./airtable.db --pretty
  ```

### Rate-aware fan-out
- **`schema drift`** — Compare cached schemas of every base under the active profile against a fresh fan-out of `bases get_schema`; report added, removed, and renamed tables and fields. Exit 1 on drift for CI.

  _Catches the 'someone added a singleSelect choice and broke our ETL' failure mode that hits multi-base setups every week._

  ```bash
  airtable-pp-cli schema drift --all-bases --db ./airtable.db --agent
  ```
- **`webhooks fleet`** — List every webhook on every base under the active profile (or across `--all-profiles`), sorted by time-until-expiration, with last-payload age.

  _Integration engineers managing webhooks across many bases see expirations and silent failures in one report instead of N lookups._

  ```bash
  airtable-pp-cli webhooks fleet --all-profiles --agent --select webhooks.baseId,webhooks.expiresAt,webhooks.lastPayloadAge
  ```

## Recipes


### Sync a base and query it with SQL

```bash
airtable-pp-cli sync --resources records --since 7d --db ./airtable.db && airtable-pp-cli query "SELECT name, status FROM appXXX__customers WHERE last_modified_time > datetime('now','-1 day')" --db ./airtable.db --json
```

Pulls the last week of record changes into SQLite, then runs raw SQL against the mirror — no API round-trip for the query.

### Audit what changed across a base since Friday

```bash
airtable-pp-cli changes --since 3d --group-by table --db ./airtable.db --agent --select changes.table,changes.field,changes.count
```

Joins webhook_payloads with current records to report grouped change counts; perfect for Monday-morning digest scripts.

### Bulk upsert from a JSON file with progress

```bash
airtable-pp-cli records bulk-upsert appXXX tblYYY --records-file ./customers.json --merge-on Email --batch-progress
```

Auto-batches the input into 10-record chunks (Airtable's per-request cap), shows JSON progress lines on stderr per batch. For the generated single-batch variant, see `airtable-pp-cli records upsert --help`.

### Surface deeply nested webhook payload data for an agent

```bash
airtable-pp-cli webhooks list-payloads appXXX achXXX --agent --select payloads.changedTablesById.tblYYY.changedRecordsById,payloads.timestamp
```

Webhook payloads are deeply nested; `--select` with dotted paths narrows the response so an agent doesn't burn context parsing irrelevant fields. Pair `--agent` with `--select` on any nested response.

### Find stale Draft articles older than 30 days

```bash
airtable-pp-cli stale appXXX tblYYY --field Status --equals Draft --older-than 30d --db ./airtable.db --pretty
```

Local query against the synced records cache — no API call, no rate-limit consumption, suitable to run on a cron.

## Usage

Run `airtable-pp-cli --help` for the full command reference and flag list.

## Commands

### bases

Workspace bases (Meta API)

- **`airtable-pp-cli bases get-schema`** - Get full schema (tables, fields, views) for a base
- **`airtable-pp-cli bases list`** - List bases the token can access

### comments

Row-level comments on records

- **`airtable-pp-cli comments create`** - Add a comment to a record
- **`airtable-pp-cli comments delete`** - Delete a comment
- **`airtable-pp-cli comments list`** - List comments on a record
- **`airtable-pp-cli comments update`** - Edit a comment's text

### records

Records — the dominant Airtable surface

- **`airtable-pp-cli records create`** - Create record(s). Use --fields for single, --records JSON for bulk.
- **`airtable-pp-cli records delete`** - Delete record(s) by ID
- **`airtable-pp-cli records get`** - Get a single record by ID
- **`airtable-pp-cli records list`** - List records from a table
- **`airtable-pp-cli records replace`** - Replace record(s) — clears any field not present in the payload
- **`airtable-pp-cli records sync-csv`** - Push CSV content into a synced table (synced tables only)
- **`airtable-pp-cli records update`** - Update record(s) — merges supplied fields, leaves others untouched
- **`airtable-pp-cli records upsert`** - Upsert records using Airtable's native performUpsert syntax

### tables

Table and field management (Meta API)

- **`airtable-pp-cli tables create`** - Create a table in a base
- **`airtable-pp-cli tables create-field`** - Add a field to a table
- **`airtable-pp-cli tables update`** - Update table name or description
- **`airtable-pp-cli tables update-field`** - Update a field's name or description

### webhooks

Change-notification webhooks for a base

- **`airtable-pp-cli webhooks create`** - Create a webhook on a base
- **`airtable-pp-cli webhooks delete`** - Delete a webhook
- **`airtable-pp-cli webhooks enable-notifications`** - Enable or disable notification delivery for a webhook
- **`airtable-pp-cli webhooks list`** - List webhooks registered for a base
- **`airtable-pp-cli webhooks list-payloads`** - List recorded change-event payloads for a webhook
- **`airtable-pp-cli webhooks refresh`** - Refresh a webhook's expiration timestamp

### whoami

Auth introspection — returns current token's user and scopes

- **`airtable-pp-cli whoami`** - Return the current user ID and granted scopes

### workspaces

Workspace metadata (Meta API)

- **`airtable-pp-cli workspaces <workspaceId>`** - List collaborators on a workspace


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airtable-pp-cli bases list

# JSON for scripting and agents
airtable-pp-cli bases list --json

# Filter to specific fields
airtable-pp-cli bases list --json --select id,name,status

# Dry run — show the request without sending
airtable-pp-cli bases list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airtable-pp-cli bases list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
airtable-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/airtable-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AIRTABLE_PAT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `airtable-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AIRTABLE_PAT`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 UNAUTHORIZED on every call** — Token is expired or revoked. Regenerate at airtable.com/create/tokens, then re-export `AIRTABLE_PAT` and re-run `airtable-pp-cli whoami get`.
- **HTTP 429 RATE_LIMITED bursts during sync** — Airtable enforces 5 req/sec/base. The CLI auto-backs-off 30s on 429 by default; lower concurrency further with `--retry-base-ms 1000` or split the sync by resource with `sync --resources records`.
- **records create returns INVALID_VALUE_FOR_COLUMN on a singleSelect** — Use `airtable-pp-cli records create appXXX tblYYY --fields '{...}' --dry-run` first — it validates against the cached schema and reports the exact field error.
- **Webhook payloads stop arriving** — Webhook may have expired (7-day TTL by default). Run `airtable-pp-cli webhooks fleet` to see expirations across all bases; refresh with `airtable-pp-cli webhooks refresh appXXX achXXX`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**airtable.js**](https://github.com/Airtable/airtable.js) — JavaScript (1200 stars)
- [**pyairtable**](https://github.com/gtalarico/pyairtable) — Python (820 stars)
- [**airtable-mcp-server**](https://github.com/domdomegg/airtable-mcp-server) — TypeScript (444 stars)
- [**airtable-schema-generator**](https://github.com/bjelline/airtable-schema-generator) — JavaScript (60 stars)
- [**airtable-cli (arnoldadlv)**](https://github.com/arnoldadlv/airtable-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
