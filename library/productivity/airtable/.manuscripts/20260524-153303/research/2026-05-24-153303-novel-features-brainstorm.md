## Customer model

Four personas drawn from the brief's Users line (ops/RevOps/marketing/product, integration devs, agents/AI assistants) and the Top Workflows. Each one already feels concrete pain that the absorbed 36-feature surface only partially solves.

### 1. Priya — RevOps lead at a 60-person SaaS startup
- **Today:** Owns three Airtable bases (Pipeline, Accounts, Renewals). Linked-record relationships across all three. Pulls weekly board metrics by exporting CSVs from each base and stitching them in Google Sheets because Airtable can't `JOIN` across bases and `filterByFormula` runs one base at a time.
- **Weekly ritual:** Monday morning — re-export Pipeline + Accounts, run a sumifs in Sheets, paste a screenshot into Slack. Wednesday — re-pull after the SDR ICP refresh. Friday — reconcile renewals against billing.
- **Frustration:** The hand-stitch loses 2 hours/week and breaks whenever a field is renamed. She's hit the 5 req/sec/base wall twice in one quarter doing a one-shot Python script. She wants `SELECT a.arr, b.csm FROM pipeline a JOIN accounts b ON ...` and the ability to schedule it.

### 2. Marcus — Integration engineer at a mid-market agency
- **Today:** Wires Airtable into n8n + Make for 8 client tenants. Each client has 1–3 bases and a different PAT. He maintains brittle JS shims for upserts because no SDK exposes `performUpsert.fieldsToMergeOn` cleanly, and he hand-rolls 429 backoff in every workflow.
- **Weekly ritual:** Daily — debug one tenant's failed upsert by curling the API and diffing payloads against the schema. Weekly — audit which webhooks are about to expire (7-day default) across all tenants and refresh them.
- **Frustration:** No tool gives him "tail the webhook payload stream so I can see what fires when a user clicks save." He grep's nginx logs from the receiver because the Airtable side is opaque until a payload arrives. He also has zero visibility into per-base rate-budget consumption across tenants.

### 3. Sam — Agent/AI assistant developer
- **Today:** Building an LLM agent that answers "what's the status of project X" by reading from Airtable. The agent burns tokens re-fetching the same records on every turn and trips 429s during demos. The schema-aware validation gap means hallucinated field names go straight to POST and silently corrupt records.
- **Weekly ritual:** Pre-demo — manually warm a cache by listing every record in every relevant table. Post-demo — reconcile what the agent wrote vs. what was intended (no audit trail beyond Airtable's revision history, which the API doesn't expose).
- **Frustration:** No primitive for "give me the diff between what I tried to write and what got persisted." No way to run an LLM tool call in `--dry-run` mode against a local mirror without burning live API budget. No cross-base search the agent can call as a single tool.

### 4. Lena — Marketing ops manager
- **Today:** Runs a content calendar in Airtable. Heavy use of singleSelect (Status), multipleSelects (Channels), linked-records (Authors → Articles), and attachments (hero images). Her ETL into the data warehouse breaks every time someone adds a singleSelect choice and the downstream Snowflake schema doesn't know about it.
- **Weekly ritual:** Monday — pull last week's published articles, group by channel, send a digest. Quarterly — audit which Status values are stale (not updated in >30d).
- **Frustration:** She wants a "what changed in this base since Friday" report without setting up webhooks (she has no receiver). She also wants to know which records reference an attachment that's about to expire (Airtable attachment URLs expire in ~2h via signed URLs — this trips a lot of integrations).

## Candidates (pre-cut)

12 candidates. Sources: (a) persona-driven, (b) service-specific, (c) cross-entity local query. The transcendence value-add categories from the brief (local SQLite mirror, schema-aware validation, rate-budget choreography, webhook aggregation, cross-base queries) all map in.

| # | Candidate | Command sketch | Source | Rationale (1 line) |
|---|-----------|----------------|--------|--------------------|
| C1 | Cross-base SQL query over local mirror | `airtable-pp-cli query "SELECT ..." --db` | (c) | Priya's #1 frustration; absorb item #36 (cross-base SQL) was P3 thesis but never landed as a command. SQLite FTS5 + multi-base joins. |
| C2 | Local sync engine (one-shot + scheduled) | `airtable-pp-cli sync --resources records,schema --since 7d --db` | (b) | Powers C1; framework `sync` vocabulary. Uses webhook cursor when available, falls back to Last Modified Time poll. |
| C3 | What-changed analytics over synced base | `airtable-pp-cli analytics --type changes --since 7d --db` | (c) | Lena's weekly digest, Marcus's audit. Reads webhook_payloads + records snapshots locally. |
| C4 | Webhook payload tail (real-time stream) | already shipped as item #33 in absorb | — | SKIP — already in manifest as `webhooks tail`. |
| C5 | Schema-drift watcher across multiple bases | `airtable-pp-cli analytics --type schema-drift --resources bases --db` | (c) | Lena's "new singleSelect choice broke Snowflake" — extends absorb item #30 (single-base diff) to N-bases. |
| C6 | Stale-record finder by field-type pattern | `airtable-pp-cli stale <baseId> <tableId> --field Status --older-than 30d --db` | (c) | Lena's "what Status values are stale" — local query, hand-code, calls `hintIfStale`. |
| C7 | Attachment URL expiration scanner | `airtable-pp-cli attachments expiring --within 2h --db` | (b) | Lena's signed-URL pain. Scans synced records, flags attachment fields with timestamps near the 2h boundary. |
| C8 | Write-attempt audit log (dry-run vs live diff) | `airtable-pp-cli audit writes --since 1d --db` | (a) | Sam's "what got persisted vs what I tried" — records every write through the CLI to a local audit table, joinable to live. |
| C9 | Rate-budget dashboard across PAT profiles | `airtable-pp-cli analytics --type rate-budget --resources bases` | (a) | Marcus's multi-tenant 429 pain. Reports per-base req/sec utilization from local request log. |
| C10 | Linked-record graph traversal | `airtable-pp-cli traverse <baseId> <recordId> --depth 2 --db` | (c) | Sam + Priya — "show me everything connected to this account." Local-only graph walk over linked-record fields. |
| C11 | Webhook fleet manager (multi-base lifecycle) | `airtable-pp-cli webhooks fleet --resources bases` | (b) | Marcus's "which webhooks expire this week across 8 tenants" — fans out across configured profiles. |
| C12 | Field-rename impact report | `airtable-pp-cli impact field-rename <baseId> <tableId> <fieldName> --db` | (c) | Marcus's brittleness + Lena's downstream-schema-break — scans local mirror for filterByFormula references, audit_log queries that name the field. |
| C13 | Bulk record search across all bases in a workspace | `airtable-pp-cli search <query> --workspace <wsId>` | (b) | Extends absorb item #27 (single-base search) to workspace-scope. Powered by local mirror FTS5. |
| C14 | Record-revision reconstruction from webhook payloads | `airtable-pp-cli history record <baseId> <recordId> --db` | (c) | Sam's "no audit trail" — replays webhook_payloads cursor history to reconstruct the field-level edit timeline. |
| C15 | Schema-typed SQLite mirror DDL emitter | `airtable-pp-cli sync init <baseId> --db --emit-ddl` | (b) | Powers everything downstream. Generates CREATE TABLE statements from Airtable schema (extends absorb item #29's SQL DDL output by actually applying it to the mirror). |

## Survivors and kills

Cut from 14 live candidates to 7 survivors. Kill criteria applied: reimplementation of absorbed feature, LLM dependency, scope creep (>200 lines or background process), low evidence, redundant with framework commands.

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Cross-base SQL over local mirror | `airtable-pp-cli query "SELECT ..." --db ./airtable.db` | 9/10 | hand-code | Reads from the local SQLite mirror (built by `sync`), exposes tables as `<base_slug>__<table_slug>`, runs raw SQL with `hintIfUnsynced` warning if mirror is empty. No API call. | Brief Product Thesis explicitly calls out "no existing tool does cross-base joins"; absorb #36 ("cross-base SQL") was thesis-level but not in the 36-feature manifest as a command; pyairtable issues #427/#313/#142 all about 429 from re-fetching the same data |
| 2 | What-changed analytics window | `airtable-pp-cli analytics --type changes --since 7d --db ./airtable.db` | 8/10 | hand-code | Joins local `webhook_payloads` table (populated by `sync`) with current `records` snapshot. Groups change events by table + field + actor. Calls `hintIfStale` before returning. | Lena persona weekly digest; Marcus audit ritual; brief Data Layer section names webhook payloads as canonical incremental-sync mechanism; no existing CLI exposes this |
| 3 | Stale-record finder | `airtable-pp-cli stale <baseId> <tableId> --field <name> --older-than 30d --db` | 7/10 | hand-code | Local SELECT against synced `records` table filtering on `last_modified_time` and (optionally) a singleSelect value. Calls `hintIfStale`. Fully offline. | Lena ritual ("which Status values are stale"); brief Data Layer documents Last Modified Time as a first-class sync cursor; no Airtable UI or CLI surface for this |
| 4 | Schema-drift watcher (multi-base) | `airtable-pp-cli analytics --type schema-drift --resources bases --db` | 7/10 | hand-code | Reads cached schemas from `~/.cache/airtable-pp-cli/<baseId>/schema.json` (absorbed feature #2), compares against live `bases get_schema` fan-out, emits drift report grouped by base. Extends absorbed feature #30 from single-base to N-bases. | Lena's "new singleSelect choice broke Snowflake" pain; airtable-mcp-server has no drift watcher; absorbed #30 limited to one base |
| 5 | Webhook fleet manager | `airtable-pp-cli webhooks fleet --resources bases` | 7/10 | hand-code | Iterates every base reachable under the active profile (or all configured profiles with `--all-profiles`), calls `webhooks list` for each, reports `(baseId, webhookId, expires_in, last_payload_age)` sorted by expiry. Reuses absorbed `webhooks list/refresh` for the actions. | Marcus persona explicit ("audit which webhooks are about to expire across 8 tenants"); no existing CLI/MCP exposes any webhook surface (per absorb manifest items #19–#24); brief P1 Webhooks priority |
| 6 | Linked-record graph traversal | `airtable-pp-cli traverse <baseId> <recordId> --depth 2 --db` | 6/10 | hand-code | Recursive walk over synced `records` table following linked-record array fields up to `--depth`. Renders as indented tree (`--pretty`) or ndjson edges (`--json`). Calls `hintIfStale`. No API calls. | Sam's agent use case ("everything connected to this account"); brief data profile explicitly calls out heavy linked-record usage; no SDK or CLI does this |
| 7 | Record revision history from payloads | `airtable-pp-cli history record <baseId> <recordId> --db` | 6/10 | hand-code | Filters local `webhook_payloads` table by `record_id`, reconstructs field-level edit timeline in cursor order. Renders `--pretty` as timestamped diff per field, `--json` as event array. Calls `hintIfStale` if last sync >1h ago. | Sam persona ("no audit trail beyond Airtable's revision history"); brief calls out webhook payloads as ordered change events with monotonic cursor; Airtable revision history is UI-only, not in any API |

All seven survivors are `hand-code` because they read the local SQLite mirror (or fan out across multiple bases), neither of which the spec-driven generator can emit from the OpenAPI surface.

**Buildability proofs (per rubric):**
1. `query` — uses local SQLite mirror at `--db` path to execute user-supplied SQL with no external dependencies beyond the absorbed `sync` command's outputs.
2. `analytics --type changes` — uses local `webhook_payloads` and `records` tables to compute change counts grouped by table/field/actor, no API call.
3. `stale` — uses local `records.last_modified_time` column to compute records older than `--older-than`, no API call.
4. `analytics --type schema-drift` — uses live `GET /v0/meta/bases/{baseId}/tables` per base plus cached schema files to compute add/remove/rename deltas, no external dependencies.
5. `webhooks fleet` — uses live `GET /v0/bases/{baseId}/webhooks` per base across configured profiles to compute expiry-ordered report, no external dependencies.
6. `traverse` — uses local `records` table linked-record JSON columns to compute a depth-limited graph walk, no API call.
7. `history record` — uses local `webhook_payloads` table filtered by `record_id` to compute an ordered edit timeline, no API call.

### Killed candidates

| # | Candidate | Kill reason |
|---|-----------|-------------|
| C2 | Local sync engine | Reframed and absorbed into framework `sync` command (vocabulary already stable per reminders). Not a novel feature — it's the framework primitive that powers survivors #1–#3, #6, #7. Listed here only for completeness; the survivors depend on it. |
| C4 | Webhook payload tail | Already shipped — absorb manifest #33 (`webhooks tail`). Redundant. |
| C7 | Attachment URL expiration scanner | **Reimplementation risk.** Airtable signed-URL expiration is not exposed in the API response — would require parsing the URL's query-string `Expires` parameter or computing from a heuristic. Brief never confirms a stable expiry field. Scored 3/10 on Research Backing. Cut. |
| C8 | Write-attempt audit log | **Scope creep.** Requires a write-interceptor middleware persisting every request body, plus a reconciliation engine to diff against live state. >200 lines, needs new data model not covered by sync. Possible but better as a v2 after sync ships. Cut for v1. |
| C9 | Rate-budget dashboard | **Scope creep + verifiability.** Requires a request log middleware writing every API call to local storage. Useful but redundant with the absorbed `--retry-base-ms` instrumentation (manifest #37) and harder to verify in dogfood without burning real rate budget. Cut. |
| C12 | Field-rename impact report | **Verifiability low.** Detecting `filterByFormula` references requires either parsing user shell history or asking users to register their formulas with the CLI. No clean primitive. Marcus's pain is real but the mechanical version (grep local audit log for the field name) needs C8 to ship first. Cut. |
| C13 | Workspace-scoped search | **Redundant with absorbed #27.** The absorb manifest's `search` already fans out across a base; extending to workspace adds one outer loop and conflicts with the stable framework `search` vocabulary (`--type <single resource>`, not multi-base). Reframe is just calling `search` per base in a shell loop. Cut. |
| C15 | Schema-typed SQLite mirror DDL emitter | Reframed and folded into framework `sync init` / `sync --resources schema`. Not novel on its own — it's a sub-step of survivor #1's prerequisite. Cut as a standalone feature. |

Seven survivors, all scoring >=6/10, all `hand-code`, all read from the local SQLite mirror or fan out across multiple bases — the two categories the spec-driven generator cannot emit.
