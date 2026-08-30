# Redmine CLI Brief

## API Identity
- Domain: Redmine — self-hosted, open-source project management / issue tracking (issues, projects, wiki, time tracking, roadmap/versions, forums/news, repositories).
- Users: dev teams and PMs running their own Redmine instance; here specifically the *user of this devcontainer*, developing against a live throwaway local instance seeded with a Demo Project.
- Data profile: deeply relational — issues carry tracker/status/priority/assignee/category/version/custom-field references, journals (comment + field-change history), relations (blocks/precedes/duplicates), watchers, attachments, and per-issue time entries. Projects nest issues, versions, wiki pages, news, memberships, categories. This is an ideal shape for a local SQLite mirror: the API can't natively answer cross-entity questions (e.g. "average time-to-resolution by tracker across projects") without N+1 requests; a synced local store answers them with one SQL query.

## Reachability Risk
- None. Confirmed live: `GET /users/current.json` against `http://redmine:3000` with `X-Redmine-API-Key` returns `200` (verified twice — once during Phase 0 setup, once as the formal Phase 1.9 gate). This is the user's own self-hosted instance (`redmine:6.0` per `.devcontainer/docker-compose.yml`), REST API explicitly enabled by the `redmine-init` bootstrap service, admin API key pinned to a fixed dev value and already exported as `$REDMINE_API_KEY` / `$REDMINE_URL` in this container. No bot protection, no rate limiting beyond Redmine's own pagination cap (see below) — this is not a public/adversarial target.
- Probe-safe endpoint used: `GET /users/current.json` (no params, validates the key, matches the spec's natural `auth.verify_path` candidate).

## Top Workflows
1. **Triage and work issues** — list/filter issues by project, status, assignee, tracker, priority; view one issue with its full journal/history; create and update issues; add notes; change status/assignee (the bread-and-butter CRUD every competing tool leads with).
2. **Time tracking** — log time entries against issues, list/filter time entries by project/user/date range/activity, and (novel) roll them up into burndown/velocity reports joined against version/tracker locally — the API has no aggregate/report endpoint at all.
3. **Roadmap and release planning** — versions (target 1.4 "Alpha"), issues grouped by version, done_ratio and remaining-open-issue counts per version — Redmine's own Roadmap UI page has no REST equivalent; this has to be computed client-side from issues + versions.
4. **Project and admin housekeeping** — projects, memberships/roles, trackers, issue statuses, custom fields, groups, users — mostly read/reference data every wrapper exposes, needed so IDs (tracker_id, status_id, priority_id, project_id) can be resolved from human-typed names instead of memorized.
5. **Cross-project awareness / "what changed"** — search across issues/wiki/news/projects; a personal "since last sync" digest of newly created/updated/closed issues assigned to or watched by the current user, which requires join-and-filter logic no single Redmine endpoint provides.

## Table Stakes
- Full CRUD on issues, projects, time entries, versions, wiki pages, news, issue relations, watchers, memberships, groups, issue categories (this is what every MCP server / CLI wrapper above offers — see Absorb Manifest).
- Attachment upload/download (2-step: `POST /uploads` to get a token, then attach the token on issue create/update) and thumbnail retrieval.
- Reference-data lookups: trackers, issue statuses, enumerations (priorities, time-entry activities, document categories), roles, custom fields, queries (saved filters).
- `--json` / `--csv` / `--select` structured output, `--dry-run` for every mutation, typed exit codes, offline search once synced.
- Auth via `X-Redmine-API-Key` header (matches every wrapper's convention; the spec also models HTTP Basic, an API-key-in-query variant, and OAuth2, but the header form is what every real tool and this instance actually use).

## Data Layer
- Primary entities: `issues` (hub entity — carries FK-shaped refs to project/tracker/status/priority/author/assigned_to/category/version/parent), `projects`, `journals` (issue history/comments, nested under issues), `time_entries`, `versions`, `wiki_pages` (+ per-page version history), `news`, `issue_relations`, `memberships`, `users`, `groups`, `roles`, `trackers`, `issue_statuses`, `issue_categories`, `custom_fields`, `enumerations` (priorities / time-entry activities / document categories), `queries` (saved filters), `attachments`.
- Sync cursor: `updated_on` exists on issues, projects, time_entries, news, wiki_pages — use it for incremental `sync --since`. Reference/enum tables (trackers, statuses, priorities, roles) have no `updated_on` and change rarely — full resync each `sync` call is fine and cheap.
- Pagination: `offset`/`limit`, default `limit=25`, hard server-side max `limit=100` (not configurable without a Redmine-side patch — confirmed via redmine.org issue tracker). `total_count`/`limit`/`offset` are returned on every collection response, so the sync loop is a standard offset-walk to `total_count`, capped at 100/page.
- FTS/search: full local FTS across issues (subject/description), wiki page content, news, and journal notes once synced — this beats Redmine's own `/search` endpoint, which only searches live and does not let you compose further SQL/filter logic on top of results.

## Codebase Intelligence
- Source: `d-yoshi/redmine-openapi` (OpenAPI 3.0.3, MIT, "built from the official docs and source code, and tested against a running Redmine instance"). Latest release `7.0.0-r2`; closest version match to our live instance (`redmine:6.0`) is tag `6.1.3-r1` (verified against Redmine 6.1.3), which is what's staged as the spec source. 56 paths across 22 resource tags (Issues, Projects, Project Memberships, Users, Time Entries, News, Issue Relations, Versions, Wiki Pages, Queries, Attachments, Issue Statuses, Trackers, Enumerations, Issue Categories, Roles, Groups, Custom Fields, Search, Files, My Account, Journals, Repositories).
- Auth: `X-Redmine-API-Key` header (`apiKey` scheme, not bearer — matches exactly what's confirmed live). Canonical env var in the wild is `REDMINE_API_KEY` (used by this repo, by `onozaty/redmine-mcp-server`, and by every CLI/wrapper found) alongside `REDMINE_URL` for the base URL. The spec's scheme key is `ApiKey`, which would slug-derive to something other than `REDMINE_API_KEY` — needs `x-auth-env-vars: [REDMINE_API_KEY]` on that security scheme during Pre-Generation Auth Enrichment, plus `research.json`'s `auth.canonical_env_var`.
- Data model: `issues` is the hub — tracker_id/status_id/priority_id/project_id/author_id/assigned_to_id/fixed_version_id/category_id/parent_issue_id, each resolved against small reference tables. Journals are nested under an issue's `/issues/{id}.json?include=journals` and each journal carries `details[]` (field-level change records) — this is the "history" competing tools render as a timeline.
- Rate limiting: none beyond the 100-row page cap above; this is a self-hosted admin-owned instance.
- Architecture: `servers: - url: /` in the spec (relative) — the real base URL (`http://redmine:3000` here, arbitrary per self-hosted deployment) must come from a configurable `--base-url`/`REDMINE_URL` flag rather than being baked into the spec, same pattern as any other self-hosted API (Gitea, Grafana, etc.).

## Absorbed Feature Sources (for Phase 1.5 manifest)
- **onozaty/redmine-mcp-server** — MCP server, comprehensive tool-per-endpoint coverage (~90 tools across the same 22 categories), `readOnlyHint` annotations, allow/deny regex tool filtering, read-only mode flag. Best single ground-truth feature inventory found; built from this same OpenAPI spec family (credits `d-yoshi/redmine-openapi`) via Orval codegen.
- **MrJeffLarry/redmine-cli** (Go, `red-cli`) — packaged CLI (brew/scoop/apt), multi-instance support via `--rid` flag + per-instance auth config, `auth`/`config`/`issue`/`project`/`user` command groups.
- **egegunes/redmine-cli**, **diasjorge/redmine-cli**, **codevise/redmine_cli** — smaller CLIs; codevise notably stores auth in git config (project-local convenience, not adopted here — env var + config file matches the rest of the Printing Press auth convention).
- **python-redmine** (PyPI) — most popular Python wrapper, Django-ORM-style resource API; no CLI of its own, feature surface matches the REST API 1:1.
- **zacharyelston/redmine-mcp-server**, **Wint3rmute/redmine-mcp**, **jztan/redmine-mcp-server** — smaller/alternate MCP implementations; jztan's notably adds OAuth2 multi-user support and agile-board/CRM framing, no new REST surface beyond the spec.

## Product Thesis
- Name: **Redmine** (canonical prose name; slug `redmine`).
- Why it should exist: every existing Redmine tool (CLIs and MCP servers alike) is a thin, stateless pass-through to the REST API — one request in, one response out. None of them keep a local mirror, so nothing in the Redmine ecosystem can answer a cross-entity question (burndown by version, overload by assignee, time-to-resolution by tracker, "what changed since I last looked") without hand-rolling N+1 requests every time. A SQLite-backed sync + FTS + SQL layer turns Redmine's relational-but-siloed REST API into something queryable, and doing it as a generated Go CLI (not another MCP wrapper) means it also works from a shell script, cron job, or CI pipeline — not just an agent chat window.

## Build Priorities
1. Data layer + sync for the hub entities (issues, projects, journals, time_entries, versions, wiki_pages, news) — this unlocks every transcendence feature.
2. Absorb the full onozaty-derived tool inventory: CRUD across issues/projects/memberships/users/time_entries/news/relations/versions/wiki/attachments/groups/issue_categories, plus reference-data reads (trackers/statuses/enumerations/roles/custom_fields/queries), plus search and my-account.
3. Transcend: version/roadmap burndown, assignee-overload detection, stale-issue detection, "since" digest, offline cross-entity SQL, name→ID resolution for tracker/status/priority/project so commands never require memorizing numeric IDs.
