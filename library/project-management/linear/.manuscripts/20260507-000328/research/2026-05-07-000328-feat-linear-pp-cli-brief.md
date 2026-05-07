# Linear CLI Brief (reprint, v3.10.0)

This is a reprint of the v1.2.0 Linear CLI under the current Printing Press. Prior research from `manuscripts/linear/20260407-234803/` is reused (29 days old; user explicitly chose reuse + re-validate). Only the deltas that matter for v3 are re-derived; everything else is carried forward verbatim.

## API Identity
- Domain: project management / issue tracking for engineering teams
- Users: software engineers, engineering managers, product managers, DevOps
- Data profile: GraphQL API at `https://api.linear.app/graphql`, ~98 entity types, 43k-line schema, cursor-based Connection pagination, mutations follow `{action}{Entity}` pattern
- Auth: API key (`Authorization: <key>`, env `LINEAR_API_KEY`) or OAuth2

## Reachability Risk
- None. Re-probed `viewer { id name email }` → HTTP 200. Linear's API is well-maintained.

## Top Workflows (carried from prior)
1. Issue triage and sprint planning — review inbox, assign priorities, move issues to cycles
2. Sprint execution — view my assigned issues, update status, track cycle progress and burndown
3. Project health monitoring — project status, milestone tracking, team velocity over time
4. PR/branch workflow — create branch from issue, link PRs, track development progress
5. Backlog grooming — find stale issues, detect duplicates, prioritize by impact

## Table Stakes (carried from prior)
- Full issue CRUD with filtering, sorting, assignment
- Project, cycle, team, user, label, workflow-state, comment, document, milestone, initiative, attachment management
- Notification handling, triage queue, favorites, custom views, webhooks
- Git branch integration (create branch from issue ID), search across issues
- `--json` for all commands, watch mode for real-time changes

## Data Layer (carried from prior, unchanged)
- Primary entities: Issues, Projects, Cycles, Teams, Users, Labels, WorkflowStates, Comments, Documents, Milestones, Initiatives
- Sync cursor: `updatedAt`-based incremental sync with cursor pagination
- FTS/search: issue titles, descriptions, comments via SQLite FTS5 (v3 emits content-linked FTS triggers — fixes the v1 fragility)
- High-gravity fields: issue identifier (ABC-123), title, state, priority, assignee, cycle, project, labels, due date, estimate

## User Vision
- Reprint of the v1.2.0 Linear CLI under v3.10.0. The prior shipped at 90/100 / 97% verify with `ship-with-gaps` — 2 deferred transcendence features (`projects burndown`, `cycles compare`) and sparse promoted-command examples. The v3 reprint must close those gaps; `ship-with-gaps` is no longer an acceptable verdict.
- **Live-testing constraint:** API key is provided for Phase 5 dogfood. Only mutate Linear tickets I create in this session (with an obvious `pp-test-` prefix or similar marker). Never edit, archive, or delete pre-existing tickets in the user's workspace.

## MCP Surface (re-validated for v3)
- The v1.2.0 CLI shipped 63 MCP tools (endpoint-mirror only) with `mcp_ready: full` but no remote transport, no orchestration pair, and no intents. That puts it well above the 50-tool threshold where the v3 scorecard's `mcp_remote_transport`, `mcp_tool_design`, and `mcp_surface_strategy` dimensions all penalize default endpoint-mirror surfaces.
- Recommended for v3 reprint (the Cloudflare pattern):
  - `mcp.transport: [stdio, http]` — adds streamable HTTP so cloud-hosted agents can connect
  - `mcp.orchestration: code` — emits a thin `linear_search` + `linear_execute` pair that covers the surface in ~1K tokens
  - `mcp.endpoint_tools: hidden` — suppresses raw per-endpoint mirrors (they remain reachable through `linear_execute`)
  - `mcp.intents` — define 3–5 multi-step intents for the highest-frequency workflows (triage, sprint planning, daily standup, backlog grooming, PR-ready issues)

## Codebase Intelligence
- The official `@linear/sdk` lives at `linear/linear` (TypeScript). Auth is `Authorization: <api-key>` header (no `Bearer` prefix for personal API keys).
- Rate limits: documented as ~1500 complexity points/hour for personal API keys. Mutations cost more than queries. Conservative GraphQL queries (single-level connections) are required to stay under the per-request complexity ceiling.
- Connection pattern is universal: every list field is `*Connection { nodes { ... }, pageInfo { hasNextPage, endCursor } }`. v3's GraphQL sync template should generate cursor-paginated queries that hit this shape.

## Spec Strategy
- GraphQL only. Use the SDL at `https://github.com/linear/linear/blob/master/packages/sdk/src/schema.graphql` (same source as v1.2.0).
- v3 generator emits GraphQL-specific client + sync templates (per the v1.2.0 retro WU-3) and dedups type fields (WU-1). These were the largest manual-fix sources in v1; verify they're in v3.10.0 during Phase 2 build.

## Build Priorities (reprint-aware)
1. Foundation: SQLite store for all primary entities + sync + FTS5 (content-linked triggers, v3 default)
2. Absorb: every command from the v1 manifest (40 absorbed features). v3 should generate examples on promoted commands automatically.
3. Transcend: all 8 transcendence features, INCLUDING the 2 v1.2.0 deferred (`projects burndown`, `cycles compare`)
4. v3-only: spec MCP enrichment (transport, orchestration, intents) before generation

## v3 Re-validation Notes
- **Transport / reachability:** unchanged from v1
- **Scoring rubrics:** v3 adds Phase 4.85 (output review), Phase 4.9 (README/SKILL correctness), agent-readiness review, tools-audit. The prior CLI predates these so polish-loop fixes will surface; budget for them.
- **Auth modes:** unchanged (api_key)
- **MCP surface:** changed substantially — see MCP Surface section above. Pre-generation enrichment is mandatory at 63+ typed tools.
- **Discovery:** unchanged (clean spec, no browser-sniff)
