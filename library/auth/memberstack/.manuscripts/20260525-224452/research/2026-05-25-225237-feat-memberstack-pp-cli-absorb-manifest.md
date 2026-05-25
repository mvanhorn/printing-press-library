# Memberstack CLI Absorb Manifest

Sources surveyed:
- `@memberstack/admin` npm (Node SDK) — every public method.
- Official Memberstack MCP Server (Beta), ~68 tools — feature parity reference.
- `memberstack-skills` Claude Code skill (thin wrapper over MCP).
- Zapier / Composio / LobeHub MCP integrations (subset of MCP).
- Memberstack public docs: Member Actions, Verification, Data Tables, Plans, Permissions.
- **No competing CLI exists.** The CLI space for Memberstack is empty.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | List members | `@memberstack/admin` `members.list()` / MCP `list_members` | `(generated endpoint) members list` | Local SQLite mirror, FTS5 search, `--json --select`, cursor pagination handled automatically |
| 2 | Get member by ID or email | `@memberstack/admin` `members.retrieve()` | `(generated endpoint) members get <id-or-email>` | Offline lookup after sync, agent-native `--json` |
| 3 | Create member | `@memberstack/admin` `members.create()` | `(generated endpoint) members create` | `--dry-run`, typed exit codes, idempotent on duplicate-email |
| 4 | Update member | `@memberstack/admin` `members.update()` | `(generated endpoint) members update <id>` | Supports `--stdin` for piped JSON; preserves untouched fields |
| 5 | Delete member | `@memberstack/admin` `members.delete()` | `(generated endpoint) members delete <id>` | `--dry-run`, `--delete-stripe-customer`, `--cancel-stripe-subscriptions` flags |
| 6 | Add free plan to member | `@memberstack/admin` `members.addFreePlan()` | `(generated endpoint) plans add-free <member-id> --plan <pln_*>` | `--dry-run`, typed validation of `pln_*` prefix |
| 7 | Remove free plan from member | `@memberstack/admin` `members.removeFreePlan()` | `(generated endpoint) plans remove-free <member-id> --plan <pln_*>` | Same as above |
| 8 | Verify member JWT | `@memberstack/admin` `verifyToken()` / MCP `verify_token` | `memberstack-pp-cli token verify <jwt>` | Works offline against JWKS once cached; also offers `token decode` |
| 9 | List data tables | MCP `list_data_tables` | `(generated endpoint) data-tables list` | Local mirror of schemas, agent-native |
| 10 | Get data table schema | MCP `get_data_table` | `(generated endpoint) data-tables get <key>` | Offline after sync |
| 11 | Create data record | MCP `create_data_record` | `(generated endpoint) records create <table-key>` | `--stdin`, `--dry-run` |
| 12 | Update data record | MCP `update_data_record` | `(generated endpoint) records update <table-key> <record-id>` | `--stdin`, `--dry-run` |
| 13 | Delete data record | MCP `delete_data_record` | `(generated endpoint) records delete <table-key> <record-id>` | `--dry-run` |
| 14 | Query data records (Prisma) | MCP `query_data_records` | `(generated endpoint) records query <table-key>` | Accepts Prisma envelope on stdin or short-form `--where` / `--order-by` flags |
| 15 | Full data sync to local store | (none exists) | `(behavior in memberstack-pp-cli sync)` | First-class — no other tool persists Memberstack data locally |
| 16 | Cross-entity FTS search | (none exists) | `(behavior in memberstack-pp-cli search <query>)` | First-class — no other tool offers offline full-text member search |
| 17 | Composable SQL access | (none exists) | `(behavior in memberstack-pp-cli sql)` | Power-user escape hatch over the local store |
| 18 | Health check / doctor | (none exists) | `(behavior in memberstack-pp-cli doctor)` | Auth, network, store, plan-id sanity |

(`(generated endpoint) ...` rows are emitted by the Printing Press from the spec. `(behavior in ...)` rows are part of the generator's framework commands. None of these are stubs.)

## Transcendence (only possible with our approach)

Every row below requires SOMETHING that the official SDK and MCP do not provide: a local SQLite mirror, agent-native filtering, JWT decoding without a network roundtrip, or a workflow that composes several API calls.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | **Stale members** — last-login older than N days | `memberstack-pp-cli stale --days 30` | hand-code | Requires `lastLogin` joined against a local snapshot; the API returns it but neither the dashboard nor the MCP filters on it. Output is `--json --select id,email,lastLogin` ready for piping to bulk-delete. |
| 2 | **Plan coverage matrix** — which plans does each member hold | `memberstack-pp-cli plan-coverage` | hand-code | Pivots `planConnections[].planId` across all members locally; flags members with zero active plans. Memberstack's UI shows this one member at a time. |
| 3 | **Custom-fields flatten** — pivot every member's custom fields into one CSV/JSON table | `memberstack-pp-cli fields flatten --csv` | hand-code | The Admin API returns `customFields` as a nested map; nobody else flattens it for analysis. Critical for marketing/BI exports. |
| 4 | **Token decode (offline)** — decode a Memberstack JWT without a network call | `memberstack-pp-cli token decode <jwt>` | hand-code | Pure local JWT parse — `verify` requires the secret; `decode` only needs the JWT. Useful for debugging frontend issues. The MCP cannot do this offline. |
| 5 | **Bulk delete by predicate** — wipe sandbox/test members matching a filter | `memberstack-pp-cli bulk delete --where "email LIKE '%@test.local'" --dry-run` | hand-code | Combines local SQL query + per-row delete with `--dry-run` preview. No competing tool offers a transactional bulk delete. |
| 6 | **Watch new signups (live tail)** — poll cursor and print new members as they arrive | `memberstack-pp-cli watch --since 5m` | hand-code | Cursor-pollable; useful for `\| jq` pipelines and ops notifications. Memberstack has no streaming API. |
| 7 | **Audit (drift)** — diff local snapshot vs live to see what changed | `memberstack-pp-cli audit` | hand-code | Requires two snapshots — only possible because we persist locally. Surfaces silent changes to permissions, plan-connections, custom-fields. |
| 8 | **Plan-id lookup** — resolve `pln_*` IDs to human plan names from a one-time bootstrap | `memberstack-pp-cli plans list` | hand-code | Plan metadata isn't on the REST API; we cache it from `planConnections[].planId` observed across sync, building a `(planId, count, lastSeen)` index. The dashboard requires manual lookup. |
| 9 | **Records query shorthand** — Prisma envelope from short flags | `memberstack-pp-cli records find <table> --where 'inStock=true' --order-by price:asc --limit 10` | hand-code | Builds the Prisma `findMany` payload from flags; no one writes that envelope by hand twice. Saves agents from constructing JSON. |
| 10 | **Snapshot export** — dump everything to one tarball for backup / GDPR requests | `memberstack-pp-cli export --output ./backup-$(date +%F).tgz` | hand-code | Combines `sync --full` + serialize-to-disk + integrity hash. No competing tool offers an end-to-end snapshot. |

**10 transcendence features, all hand-code (~50–150 LoC each plus root wiring).**

## Source Priority

Single source (`memberstack`). No combo-CLI ordering applies.

## Pre-build scope statement

- **Absorbed:** 18 rows (14 generator-emitted, 4 framework behaviors).
- **Transcendent:** 10 rows, all hand-code.
- **Stubs:** 0. Every row ships fully implemented.
- **Risks:** Memberstack's "plans" surface is dashboard-only; we infer plan IDs from sync data rather than calling a `/plans` endpoint that doesn't exist. `webhook-sim` was considered and dropped — it would need an HMAC over a synthetic payload that the user's server would have to accept, which is more "test harness" than "CLI feature."
