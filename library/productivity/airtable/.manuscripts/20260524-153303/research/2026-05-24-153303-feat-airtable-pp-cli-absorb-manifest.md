# Airtable CLI Absorb Manifest

## Source Tools Catalog (informational)

- airtable.js (official Node SDK)                  — github.com/Airtable/airtable.js
- pyairtable (community Python SDK)                — github.com/gtalarico/pyairtable
- domdomegg/airtable-mcp-server (MCP, 444 stars)   — github.com/domdomegg/airtable-mcp-server (v1.13.0, 2026-03-07)
- felores/airtable-mcp (MCP)                       — github.com/felores/airtable-mcp
- rashidazarang/airtable-mcp (MCP)                 — github.com/rashidazarang/airtable-mcp
- ngs/mcp-server-airtable (MCP)                    — github.com/ngs/mcp-server-airtable
- mike-lischke/vscode-airtable-formulas (VSCode)   — github.com/mike-lischke/vscode-airtable-formulas
- arnoldadlv/airtable-cli (CLI, Mar 2026)          — github.com/arnoldadlv/airtable-cli (most-recent CLI)
- robertjurvanen-max/airtable-cli (CLI)            — github.com/robertjurvanen-max/airtable-cli
- pyairtable-cli (pyairtable companion CLI)        — bundled with pyairtable
- airtable-export (Simon Willison)                 — github.com/simonw/airtable-export (record-dump tool)
- airtable-html (Simon Willison)                   — github.com/simonw/airtable-html (HTML browser)
- node-airtable-schema (schema dumper)             — npm `airtable-schema-generator`
- airtable-ruby (community Ruby SDK)               — github.com/airtable/airtable-ruby
- airtable-go (community Go SDK)                   — github.com/fabioberger/airtable-go
- airtable-php (community PHP SDK)                 — github.com/sleiman/airtable-php
- airtable-blocks-cli (official Blocks/Extensions) — github.com/Airtable/blocks
- airtable_client (Elixir SDK)                     — hex.pm/packages/airtable_client

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|--------------|
| 1 | List bases the token can access | airtable-mcp-server `list_bases`, airtable.js `base.list()` | `(generated endpoint) bases list` | --json/--tsv/--pretty, cursor pagination auto-followed, --max-results cap, --profile switching, exit-code 5 on 401 |
| 2 | Get full schema (tables + fields + views) for a base | airtable-mcp-server `describe_table`, pyairtable `Base.schema()` | `(generated endpoint) bases get_schema` | --include visibleFieldIds, --json/--yaml/--markdown, response cached to `~/.cache/airtable-pp-cli/<baseId>/schema.json` for offline reuse |
| 3 | Create a new table in a base | airtable-mcp-server `create_table` | `(generated endpoint) tables create` | --fields accepts JSON file path or inline JSON, --dry-run validates field-type spec client-side before POST |
| 4 | Update table name or description | airtable-mcp-server `update_table` | `(generated endpoint) tables update` | --name and --description as separate flags (SDKs require raw JSON body), exit-code 4 on table-not-found |
| 5 | Add a field to a table | airtable-mcp-server `create_field` | `(generated endpoint) tables create_field` | Field-type validation against the 30+ Airtable types before POST, --options accepts file path, hint when options are required for the chosen type |
| 6 | Update a field's name or description | airtable-mcp-server `update_field` | `(generated endpoint) tables update_field` | --name and --description as discrete flags, prints before/after diff in --pretty mode |
| 7 | List records in a table | airtable.js `table.select()`, airtable-mcp-server `list_records` | `(generated endpoint) records list` | --filter-formula, --max-records, --page-size, auto-follow cursor pagination, --sort, --view, --fields (repeatable), --json/--tsv/--csv/--pretty, --return-fields-by-id, 429 backoff with jitter, --offline reads from local cache |
| 8 | Get one record by ID | airtable.js `table.find(id)`, airtable-mcp-server `get_record` | `(generated endpoint) records get` | --json/--pretty, --return-fields-by-id, exit-code 4 on not-found, --offline reads cached mirror |
| 9 | Create record(s) | airtable.js `table.create()`, airtable-mcp-server `create_record` | `(generated endpoint) records create` | --fields (single, JSON), --records (bulk JSON file or inline), --typecast, --dry-run validates against schema cache, auto-batches at 10 records/request when bulk |
| 10 | Update record(s) — partial merge | airtable.js `table.update()`, airtable-mcp-server `update_records` | `(generated endpoint) records update` | --records (JSON file or inline), --typecast, auto-batches at 10, --idempotency-key for safe retry, prints per-record success/failure summary |
| 11 | Replace record(s) — full overwrite | airtable.js `table.replace()` | `(generated endpoint) records replace` | Warns on stdout when fields-in-schema-but-not-in-payload would be cleared, --typecast, auto-batches at 10 |
| 12 | Upsert via Airtable's native performUpsert | airtable.js `table.create({performUpsert})` (manual JSON) | `(generated endpoint) records upsert` | --merge-on flag generates `performUpsert.fieldsToMergeOn` automatically (no SDK exposes this cleanly), auto-batches at 10, reports created vs updated counts |
| 13 | Delete record(s) by ID | airtable.js `table.destroy()`, airtable-mcp-server `delete_records` | `(generated endpoint) records delete` | --records as repeatable flag, accepts IDs from stdin, auto-batches at 10, --yes to skip confirmation, returns deleted IDs as JSON |
| 14 | Push CSV into a synced table | (no SDK exposes /syncCsv) | `(generated endpoint) records sync_csv` | --csv accepts file path or `-` for stdin, validates table is sync-eligible via schema cache before POST |
| 15 | List comments on a record | airtable-mcp-server `list_comments`, pyairtable `Record.comments()` | `(generated endpoint) comments list` | Cursor auto-follow, --max-results, --json/--pretty (renders @mentions as resolvable user names via schema cache) |
| 16 | Add a comment to a record | airtable-mcp-server `create_comment` | `(generated endpoint) comments create` | --text accepts `-` for stdin (paste long markdown), --parent-comment for threaded replies, --mentioned accepts `@username` resolved from collaborator cache instead of raw `usrXXX` IDs |
| 17 | Edit a comment's text | (gap — no MCP/CLI exposes this) | `(generated endpoint) comments update` | --text accepts stdin, prints before/after diff |
| 18 | Delete a comment | (gap — most MCPs lack this) | `(generated endpoint) comments delete` | --yes to skip confirmation, exit-code 4 on not-found |
| 19 | List webhooks on a base | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks list` | --json/--pretty, shows time-until-expiration and last-notification-status in --pretty |
| 20 | Create a webhook | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks create` | --specification accepts file path or inline JSON, --notification-url validated as HTTPS before POST, --filters/--includes shortcut flags that build the specification object client-side |
| 21 | Delete a webhook | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks delete` | --yes to skip confirmation |
| 22 | List webhook payloads | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks list_payloads` | --cursor, --limit, auto-follow until `mightHaveMore=false`, --since-cursor for incremental drain, persists cursor to `~/.cache/airtable-pp-cli/<baseId>/<webhookId>/cursor` |
| 23 | Refresh a webhook's expiration | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks refresh` | Returns new expiration timestamp, exit-code 4 on not-found |
| 24 | Enable/disable webhook notifications | (gap — no existing CLI/MCP) | `(generated endpoint) webhooks enable_notifications` | --enable/--disable mutually-exclusive flags (cleaner than `enable=true/false` JSON body) |
| 25 | Auth introspection (current user + scopes) | (gap — pyairtable has it, no CLI does) | `(generated endpoint) whoami get` | --json, --pretty lists scope names with human-readable descriptions, used as health-check command |
| 26 | List workspace collaborators | (gap — no existing CLI/MCP) | `(generated endpoint) workspaces get_collaborators` | --include collaborators,inviteLinks, --json/--pretty with role grouping |
| 27 | Cross-table search across all tables in a base | domdomegg/airtable-mcp-server `search_records` (single table) | `airtable-pp-cli search <baseId> <query>` | Fans out across every table in the base (via schema cache), parallelizes with rate-limit awareness, surfaces matches with table+field context, --json/--pretty, --regex, --field-type-filter |
| 28 | Pretty Markdown record display | pyairtable's `pprint()` (Python-only, terminal-only) | `(behavior in airtable-pp-cli records list) --pretty` and `(behavior in airtable-pp-cli records get) --pretty` | Field-type-aware rendering (dates formatted to local tz, attachments shown as links, linked-records resolved to primary-field values via schema cache), pipeable Markdown so users can `\| glow` |
| 29 | Full schema dump to a single file | airtable-schema-generator (npm, JS-output only) | `airtable-pp-cli schema dump <baseId>` | Hand-built fan-out: calls `bases get_schema`, normalizes, emits --format json/yaml/markdown/sql-ddl; SQL DDL emits a CREATE TABLE per Airtable table for downstream SQLite mirror use |
| 30 | Diff local cached schema vs live | (no existing tool) | `airtable-pp-cli schema diff <baseId>` | Compares `~/.cache/airtable-pp-cli/<baseId>/schema.json` against a fresh `bases get_schema` call; reports added/removed/renamed tables and fields with exit-code 1 if drift detected (CI-friendly) |
| 31 | Formula evaluator / dry-run | (no SDK or CLI does this) | `airtable-pp-cli formula test <baseId> <tableId> <formula>` | Hand-built: fetches a small sample of records, runs the formula server-side against each via `filterByFormula`, reports which records would match and why; --sample-size, --record-id to test against a specific row |
| 32 | Bulk upsert with progress + retry | airtable.js requires manual ≤10 batching + manual 429 handling | `(behavior in airtable-pp-cli records upsert) --batch-progress --resume` | Auto-batches arbitrarily-sized input into 10-record chunks, shows progress bar on stderr, persists per-batch state to `~/.cache/airtable-pp-cli/upsert/<runId>.json` so a 429-killed run can resume with --resume <runId> |
| 33 | Webhook payload tailing | (no tool — payloads are pull-only) | `airtable-pp-cli webhooks tail <baseId> <webhookId>` | Hand-built: polls `webhooks list_payloads` continuously with adaptive backoff, streams payloads as ndjson to stdout, persists cursor between runs; --since cursor or --from-beginning |
| 34 | Field-type-aware client-side record validation | (no tool) | `(behavior in airtable-pp-cli records create) --dry-run` and `(behavior in airtable-pp-cli records update) --dry-run` | Single source of truth for all 30+ Airtable field types (singleSelect choice IDs, linked-record IDs, attachment URLs, date formats, number constraints); validates payload against cached schema before any POST, exit-code 2 with per-field error report |
| 35 | Multi-profile / multi-workspace switching | pyairtable env-var only | `(behavior in airtable-pp-cli config) --profile <name>` (and `--profile` flag on every command) | TOML config at `~/.config/airtable-pp-cli/config.toml` with named profiles (PAT + default-base + default-workspace per profile), `airtable-pp-cli config profile add/list/use/remove`, AIRTABLE_PP_PROFILE env var override |
| 36 | Built-in 429 backoff with jitter on every call | (gap — most SDKs/CLIs leave it to the caller) | `(behavior in airtable-pp-cli)` global retry middleware | Automatic 30s+jitter backoff on 429 per the Airtable docs, --retry-max and --retry-base-ms flags, per-base rate budget tracking (5 req/sec/base) with proactive throttle to avoid hitting 429 in the first place |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | Cross-base SQL over local mirror | query "SELECT ..." --db ./airtable.db | hand-code | Requires local SQLite mirror of multiple bases joined by linked-record IDs; no Airtable API or UI joins across bases |
| 2 | What-changed window | changes --since 7d --group-by table --db ./airtable.db | hand-code | Requires synced `webhook_payloads` table joined against current `records` snapshot — no single API call returns "what fields changed in the last N hours" |
| 3 | Stale-record finder | stale <baseId> <tableId> --field <name> --older-than 30d --db | hand-code | Local SELECT on synced `records.last_modified_time` with optional singleSelect filter — Airtable formula language cannot express "older than 30 days AND status=X" cleanly across views |
| 4 | Schema-drift watcher (multi-base) | schema drift --all-bases --db ./airtable.db | hand-code | Compares cached schemas against fan-out of `bases get_schema` across multiple bases; airtable-mcp-server has no drift watcher; extends absorb #30 from single-base to N-base |
| 5 | Webhook fleet manager | webhooks fleet --resources bases | hand-code | Iterates every base under active profile (--all-profiles for multi-tenant), reports (baseId, webhookId, expires_in, last_payload_age) sorted by expiry — no CLI/MCP exposes any webhook surface |
| 6 | Linked-record graph traversal | traverse <baseId> <recordId> --depth 2 --db | hand-code | Recursive walk over synced records following linked-record array fields; offline; no SDK or CLI does this |
| 7 | Record revision history from payloads | history record <baseId> <recordId> --db | hand-code | Filters local `webhook_payloads` by record_id, reconstructs field-level edit timeline in cursor order — revision history is UI-only in Airtable |
