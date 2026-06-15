# Toodledo CLI Brief

## API Identity
- **Domain:** GTD-style task/to-do management. Toodledo (toodledo.com), public REST API at `https://api.toodledo.com/3/`, v3 + OAuth 2.0 (v2 deprecated, v1 dead). JSON + XML responses.
- **Users:** Power users running Getting-Things-Done workflows: folders=projects, contexts (@home/@work), goals (lifetime/long-term/short-term), GTD statuses (Next Action, Waiting, Someday…), stars, due/start dates, subtasks (Pro), notes, outlines, custom lists.
- **Data profile:** A user's whole task universe is a few thousand rows across 9 resource types. Highly relational (tasks reference folder/context/goal/location ids). Perfect for a local SQLite mirror.

## Reachability Risk
- **Moderate (not low).** v3 API is live and reachable in 2026 (all official docs return HTTP 200). Platform is in maintenance mode under newer ownership; official client wrappers are mostly abandoned.
- **Cloudflare WAF** has intermittently 403'd `account/token.php` for correctly-formed requests (Feb-2024 forum incident, fixed server-side). → CLI must send a sane `User-Agent` and treat 403 distinctly from 401 (auth failure vs WAF).
- **Rate limits (hard):** 100 API calls per access token; ~1000 calls/hour/user; token mint capped at 10 access tokens/hour; access token lifetime 2 hours; refresh token expires after 30 days of non-use. → Local store + incremental sync is REQUIRED, not optional.
- Probe-safe endpoint: `GET /account/get.php` (read-only account info, no params).

## Top Workflows
1. **Capture fast** — `task add "<title>" --due tomorrow --context @home --priority high --folder Inbox` (name→id resolution, NL dates).
2. **"What do I do now?"** — `next-actions [--context @work]` (GTD Next Action list, sorted priority desc / due asc).
3. **Weekly review** — `review` (inbox / overdue / stalled projects / waiting-for / someday) — offline, from the local mirror.
4. **Triage & complete** — `tasks list --overdue`, `task complete <id>`, bulk complete/edit.
5. **Search everything offline** — `search "<text>"` across tasks + notes + outlines via FTS, no API calls.

## Table Stakes (from todoist/TaskWarrior/Things/TickTick survey)
- CRUD task (add/edit/complete/reopen/delete/move), list+filter (due, priority, context, folder, tag, star, status, completed), search, sort.
- Saved views: today / upcoming / overdue / inbox / someday / completed(logbook).
- Recurring tasks (iCal RRULE), subtasks (parent/child), bulk operations, NL due-date parsing.
- First-class folders/contexts/goals/locations/tags + notes. Sync with local cache. JSON output everywhere.
- Stats/summary surfaces (counts by status/priority/folder/context).

## Data Layer
- **Primary entities:** tasks (hub), folders, contexts, goals, locations, notes, outlines, lists+rows, account.
- **Sync cursor:** `account/get.php` returns `lastedit_*` / `lastdelete_*` GMT unix timestamps per resource; `tasks/get.php?after=<ts>` returns only rows modified since; `*/deleted.php?after=<ts>` returns deletions. Incremental sync stays under the 100-calls/token budget.
- **FTS/search:** tasks.title+note, notes.title+text, outlines.title+note → SQLite FTS5. Fully offline.
- **Enum decode on read:** status 0..10, priority -1..3, goal level 0/1/2 → human labels in the store/output.

## Codebase Intelligence (user's MCP — ground truth)
- Source: `~/ai/toodledo-mcp` (wwilson1017/toodledo-mcp), TypeScript, 17 MCP tools. Proven-working v3 client.
- **Auth:** OAuth2 authorization-code. Authorize `…/3/account/authorize.php`, token `…/3/account/token.php` (POST, **HTTP Basic** `base64(client_id:client_secret)`, grant_type authorization_code → refresh_token), scopes `basic tasks notes outlines lists write`. API calls send `access_token` (Bearer header also accepted by Toodledo v3).
- **Data model:** task fields confirmed; status/priority enums confirmed; folders/contexts/goals/locations shapes confirmed. Writes are form-urlencoded; **task add/edit/delete use a `tasks=<JSON-array-string>` batch param** (folders/contexts/goals/locations use simple `id`/`name` form fields). `tasks/get` returns a `{num,total}` metadata element first, then the rows.
- **Architecture insight:** the MCP's value is name→id resolution (folder/context/goal by name) and 3 GTD aggregations (next_actions, review, dashboard) computed locally over a full task fetch — exactly the compound features our SQLite store makes instant and offline.

## User Vision (briefing)
- "I already built an open-source Toodledo MCP (wwilson1017/toodledo-mcp); that should be in the mix." → Every MCP tool is an absorb row; the 3 GTD workflows seed transcendence features; the CLI should beat the MCP by being offline, rate-limit-aware, and SQL-composable.

## Source Priority
- Single source (Toodledo public v3 API). No combo, no inversion risk.

## Generation Challenges (carry into spec + Phase 3)
1. **OAuth token exchange/refresh needs HTTP Basic**, not form-encoded creds (generator default). → hand-fix `auth login` + refresh in Phase 3; flag for retro (RFC 6749 client_secret_basic should be a generator option).
2. **`.php` endpoint paths** — internal YAML `path:` carries them verbatim.
3. **Task batch writes** (`tasks=<JSON string>` form field) and **metadata-first get response** — likely hand-built task add/edit/complete/delete + custom sync parsing.
4. **Cloudflare 403 vs 401** — required `User-Agent` header + 403-aware error message.
5. **Subtasks require Pro** — detect via `account/get`, warn on non-Pro.

## Product Thesis
- **Name:** `toodledo-pp-cli` — "Toodledo from the terminal, built for agents."
- **Why it should exist:** There is **no maintained full Toodledo CLI, no real MCP server beyond the user's, and no JS SDK** — a genuinely open lane. Toodledo's punishing rate limits make a naive API-wrapper unusable; a local-SQLite mirror with incremental sync + offline FTS search + JSON-everywhere + agent-native MCP is the only shape that's actually pleasant to use, and nothing else offers it.

## Build Priorities
1. **Data layer + incremental sync** (all 9 resources, `lastedit_*`/`after` cursors, deleted-items reconciliation) + offline FTS search + SQL.
2. **Absorb:** full CRUD for tasks/folders/contexts/goals/locations/notes/outlines/lists, list+filter+sort, saved views, name→id resolution, enum decode, bulk ops.
3. **Transcend:** GTD next-actions / weekly-review / dashboard offline; rate-budget-aware sync; stalled-project & inbox detection; cross-resource search.
4. **Polish:** OAuth Basic-auth fix, Cloudflare-aware errors, Pro-subtask warning, rich help/examples.
