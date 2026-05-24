---
name: pp-airtable
description: "Every Airtable surface a CLI should have — plus a local SQLite mirror, cross-base SQL, and webhook tooling no SDK ships. Trigger phrases: `query airtable`, `sync airtable base`, `dump airtable schema`, `list airtable records`, `audit airtable webhooks`, `use airtable-pp-cli`, `run airtable`."
author: "joelsephus"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airtable-pp-cli
    install:
      - kind: go
        bins: [airtable-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/airtable/cmd/airtable-pp-cli
---

# Airtable — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airtable-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install airtable --cli-only
   ```
2. Verify: `airtable-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/airtable/cmd/airtable-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Airtable's official SDKs stop at REST pass-through; the community CLIs and MCP servers do the same. airtable-pp-cli absorbs every endpoint (records, schema, webhooks, comments, workspaces) with proactive 5-req/sec rate-budget tracking, then adds a local SQLite mirror you can `query` with raw SQL, a `webhooks fleet` view that no existing tool offers, and a `history record` reconstructor that turns webhook payloads into a real audit log.

## When to Use This CLI

Use airtable-pp-cli when an agent or script needs to read or write Airtable records reliably under the 5-req/sec-per-base rate limit, when you need cross-table or cross-base SQL over your data, when you need to operate webhooks across multiple bases or tenants, or when you need an audit trail of record changes that the Airtable UI hides behind clickthroughs. The local SQLite mirror is especially valuable for agents that re-query the same records repeatedly — sync once, query forever.

## Unique Capabilities

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

## Command Reference

**bases** — Workspace bases (Meta API)

- `airtable-pp-cli bases get-schema` — Get full schema (tables, fields, views) for a base
- `airtable-pp-cli bases list` — List bases the token can access

**comments** — Row-level comments on records

- `airtable-pp-cli comments create` — Add a comment to a record
- `airtable-pp-cli comments delete` — Delete a comment
- `airtable-pp-cli comments list` — List comments on a record
- `airtable-pp-cli comments update` — Edit a comment's text

**records** — Records — the dominant Airtable surface

- `airtable-pp-cli records create` — Create record(s). Use --fields for single, --records JSON for bulk.
- `airtable-pp-cli records delete` — Delete record(s) by ID
- `airtable-pp-cli records get` — Get a single record by ID
- `airtable-pp-cli records list` — List records from a table
- `airtable-pp-cli records replace` — Replace record(s) — clears any field not present in the payload
- `airtable-pp-cli records sync-csv` — Push CSV content into a synced table (synced tables only)
- `airtable-pp-cli records update` — Update record(s) — merges supplied fields, leaves others untouched
- `airtable-pp-cli records upsert` — Upsert records using Airtable's native performUpsert syntax

**tables** — Table and field management (Meta API)

- `airtable-pp-cli tables create` — Create a table in a base
- `airtable-pp-cli tables create-field` — Add a field to a table
- `airtable-pp-cli tables update` — Update table name or description
- `airtable-pp-cli tables update-field` — Update a field's name or description

**webhooks** — Change-notification webhooks for a base

- `airtable-pp-cli webhooks create` — Create a webhook on a base
- `airtable-pp-cli webhooks delete` — Delete a webhook
- `airtable-pp-cli webhooks enable-notifications` — Enable or disable notification delivery for a webhook
- `airtable-pp-cli webhooks list` — List webhooks registered for a base
- `airtable-pp-cli webhooks list-payloads` — List recorded change-event payloads for a webhook
- `airtable-pp-cli webhooks refresh` — Refresh a webhook's expiration timestamp

**whoami** — Auth introspection — returns current token's user and scopes

- `airtable-pp-cli whoami` — Return the current user ID and granted scopes

**workspaces** — Workspace metadata (Meta API)

- `airtable-pp-cli workspaces <workspaceId>` — List collaborators on a workspace


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airtable-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

Auto-batches the input into 10-record chunks (Airtable per-request cap) and emits JSON progress lines on stderr per batch.

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

## Auth Setup

Set `AIRTABLE_PAT` to a personal access token from airtable.com/create/tokens. Recommended scopes: `data.records:read`, `data.records:write`, `schema.bases:read`, `webhook:manage` for full functionality; `user.email:read` for the `whoami` health check. The CLI uses Bearer-token auth on every call and never persists the token to disk — set it in the env or per-profile config.

Run `airtable-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airtable-pp-cli bases list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
airtable-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airtable-pp-cli feedback --stdin < notes.txt
airtable-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/airtable-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRTABLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRTABLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
airtable-pp-cli profile save briefing --json
airtable-pp-cli --profile briefing bases list
airtable-pp-cli profile list --json
airtable-pp-cli profile show briefing
airtable-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `airtable-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/airtable/cmd/airtable-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airtable-pp-mcp -- airtable-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airtable-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airtable-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airtable-pp-cli <command> --help`.
