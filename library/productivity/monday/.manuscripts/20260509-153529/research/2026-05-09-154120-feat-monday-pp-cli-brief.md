# Monday.com CLI Brief

## API Identity
- Domain: project management / work-OS, board-based ("a project is a board, a row is an item, a cell is a column value")
- Users: ops/PMs/team leads who run their org on Monday boards (HR, sales pipelines, support queues, dev sprints, content calendars, OKRs)
- API surface: single GraphQL endpoint at `https://api.monday.com/v2`, versioned via `API-Version` header (current `2026-04`); ~45 named operations spanning boards, items, columns, groups, workspaces, folders, docs, updates, forms, assets, users, teams, notifications, dashboards, widgets, notetaker meetings, dev sprints, agents, custom activities, and a universal cross-resource search. No OpenAPI; the schema IS the spec.
- Data profile: ~13 first-class entity types (workspaces, folders, boards, groups, items, columns, column_values, updates, replies, assets, users, teams, notifications, docs). Items live inside groups inside boards inside folders inside workspaces; cross-board item-relations exist as a 2026 addition. Column values are typed (status, dropdown, person, date, timeline, mirror, formula, …) with each type a different JSON shape — this is the central pain point for tooling.
- Auth: API token in `Authorization` header, **no `Bearer` prefix** (verified at developer.monday.com/api-reference/docs/getting-started). Token is scoped to the issuing user's permissions; private boards a user can't see are invisible to the API.
- Rate limit: complexity-based. Single call cap 5M points; per-account 10M points/minute (1M for trial/free). Nested queries grow exponentially; bulk reads of column values trip limits fast.

## Reachability Risk
- None. Monday.com is a major paid SaaS ($1B+ ARR), the API is officially supported, used in production by Monday's own apps framework, and exercised by the maintained Node SDK, Python SDK, and MCP server. No GitHub issues report blanket 403 / WAF / Cloudflare blocking; intermittent failures are quota-related.

## Top Workflows
1. **Sync a board, then query/edit it offline.** Pull a whole board (groups, items, all column values) into local SQLite, run filters/searches/SQL against it without burning API points, change a column value when needed.
2. **"What changed in this sprint / this board / this team this week?"** Read board activity logs and updates, group by item/user/column, surface diffs vs. last sync.
3. **Bulk column-value updates from CSV / spreadsheet / pipe.** Update status/owner/date for many items in one transaction (each item is one mutation, but coalesced from a single source file with `--dry-run`).
4. **Sprint / dev-board reporting.** Velocity, what's stale, who's overloaded, what slipped — joins items + assignees + status + due-date + activity that no single API call returns.
5. **Cross-board search and reverse-lookup.** Universal text search; "where is this person mentioned across every board they have access to?"; "what items reference this customer in any column?"

## Codebase Intelligence
- Source: GitHub mondaycom/mcp (TypeScript, 401★) — `packages/agent-toolkit/src/core/tools/platform-api-tools/` enumerates ~50 hand-tuned tools.
- Auth: `Authorization: <raw-token>` header (no Bearer); `API-Version` header pinned per request.
- Data model: items_page cursor-based pagination (default page size 25, max 500). `column_values` polymorphic — every type is a different shape; SDK exposes typed accessors; agents typically need `column_values { id text value type column { title } }`.
- Rate limiting: complexity is queryable via `complexity { query before after reset_in_x_seconds }` on every operation. Tools must check `before` and back off pre-flight.
- Architecture: GraphQL is queryable for self-introspection; `__schema` is exposed; tools like `get_graphql_schema` and `get_type_details` are used to lazy-load schema fragments.

## Source Priority
- Single source — Monday.com only. Primary path is the documented GraphQL API at `https://api.monday.com/v2`. No spec inversion risk.

## User Vision
- The user (at Plexiz) works in a stack where Monday is the work-OS but creatives also see their work in Linear — and Slack today, Notion soon. The constant motion is "I'm always crossing between the two." The CLI should make that bridge concrete: every Monday item should be exportable in a shape that joins cleanly to the corresponding Linear / Notion / Slack record by a designated cross-system ID column. Linear and Notion integrations belong in their own printed CLIs (`linear-pp-cli`, `notion-pp-cli`); this CLI's job is to be the Monday-side of the join.

## Product Thesis
- **Name:** monday-pp-cli (binary)
- **Why it should exist:** Every existing tool — official MCP, Node SDK, Python SDK — is online-only and per-call. None of them give you a local searchable mirror of your boards, none of them turn the typed column values into clean column-shaped tables, none of them coalesce bulk edits into a `--dry-run`-able batch, none of them know how to ask "what changed since 2 hours ago?" or "who's overloaded this sprint?" Power users currently glue together `curl + jq + a Python script` per question. The pitch: every feature the official MCP has, beaten with offline SQL, FTS5 search, agent-native `--json --select --csv` output, batched bulk edits, and 8–10 transcendence commands that only work because everything is in the local store.

## Build Priorities
1. **Data layer for the full graph.** workspaces / folders / boards / groups / items / columns / column_values / updates / users / teams. Sync via items_page cursor; persist column_values as flattened typed columns AND raw JSON. FTS5 over item names, update text, and column text values.
2. **Match the official MCP feature-for-feature.** Every operation in `platform-api-tools/` becomes a typed CLI command with `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes, complexity-aware retry.
3. **Transcend with offline-only commands.** `since`, `stale`, `bottleneck`, `velocity`, `column-drift`, `whoami-load`, `mentions`, `column-types`, `bulk-edit`, `complexity-budget` — see Phase 1.5 manifest.
4. **Agent-native MCP surface.** Code orchestration + intents pattern (Cloudflare style) given the >50-tool surface; remote (HTTP) transport so it can run hosted; SKILL.md with concrete recipes that show `--select` over deeply-nested column-value payloads.
