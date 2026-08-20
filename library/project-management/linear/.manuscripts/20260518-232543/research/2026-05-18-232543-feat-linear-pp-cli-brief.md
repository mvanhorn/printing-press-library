# Linear CLI Brief (v4.9.0 reprint)

This is a reprint of the v3.10.0 Linear CLI (published 2026-05-07, run_id `20260507-000328`) under the current Printing Press v4.9.0. The reprint redoes Phase 1 from scratch — prior brief was missing the `## Users` section the Pass 2 subagent pre-flight now requires. Prior personas, workflows, and entity inventory are reused as inputs; the v4 machine delta is re-evaluated against current scoring rubrics, MCP surface defaults, agent-native parity, and supply-chain hardening.

## API Identity
- Domain: project management and issue tracking for software teams
- Users: software engineers, engineering managers, product managers, agent operators driving Linear via MCP
- Data profile: GraphQL API at `https://api.linear.app/graphql`; ~590 GraphQL types, ~367 inputs, ~93 enums; cursor-based Connection pagination on every list field; mutations follow `{action}{Entity}` naming
- Auth: API key (`Authorization: <key>`, env `LINEAR_API_KEY`, no `Bearer` prefix) or OAuth2

## Reachability Risk
- None. Linear's GraphQL endpoint is well-maintained and stable. Re-probe `viewer { id }` in Phase 1.9.

## Users

The reprint personas carry forward from `manuscripts/linear/20260507-000328/research/2026-05-07-000328-novel-features-brainstorm.md`. They are still the right four — the API has not added a new persona class in 11 days; the v4 machine delta does not change who uses Linear.

### Persona 1 — Maya, engineering manager

Runs a 9-person platform team. Lives between three Linear tabs (current cycle, active project, triage inbox), Slack, and a Friday-update Google Doc. Wants to answer "are we going to land the API migration this cycle?" without eyeballing the cycle board, "who is overloaded?" without scrolling N assignee lists, and "what slipped from last cycle, grouped by why?" without writing a saved view. Friday ritual: stakeholder update with cycle progress %, shipped, slipped, projected landings, blocked work. Monday ritual: sprint planning with last-cycle velocity informing rebalance. Linear gives her the data but not the shape.

### Persona 2 — Devon, staff engineer mid-project

Owns a 40-issue project spanning two teams and three cycles. Day starts on `linear.app/issues` filtered to "assignee:me, state:!Done", flips to project page for blocked work, flips to triage for incoming bugs. Keeps a terminal open with a feature branch and a TODO.md in the repo. Wants "what's blocked because of me" as a personal queue (Linear shows blocking relations per-issue, not as a queue). Wants `git checkout -b` from issue ID. Creates test issues mid-debug and forgets to clean them up.

### Persona 3 — Priya, product manager doing portfolio review

Owns three initiatives spanning seven projects across four teams. Tuesday portfolio review with leadership: per-initiative health, milestone slippage, projected completion. Wednesday: triages incoming feature requests. Linear's initiative view is a rollup of static project dates set by whoever made the project — not a projected-from-velocity view. Cannot answer "which milestone in my portfolio is most at risk this month?" in one place.

### Persona 4 — Sam, agent operator driving Linear via MCP

Uses Claude Code daily. Bolted Linear onto it via a third-party MCP with 60+ endpoint-mirror tools. Agent can call `getIssue` and `createIssue` but loses the plot mid-ritual (triage, sprint planning) because the endpoint-mirror surface is too thin for joined workflows and too token-heavy at 60+ tools. Daily morning prompt: "what should I work on today?" expects a ranked, joined view without naming the joins. Friday cleanup: "delete anything you created this week in test state" — requires a contract for "tickets the agent itself created" that no existing tool ships.

## Top Workflows

1. **Issue triage and sprint planning** — review inbox, assign priorities, move issues into cycles (Maya, Devon)
2. **Sprint execution** — see my assigned issues, update status, track cycle burndown (Devon, Sam)
3. **Project health monitoring** — project status, milestone tracking, team velocity over time (Maya, Priya)
4. **PR/branch workflow** — create git branch from issue, link PRs, track development progress (Devon)
5. **Backlog grooming** — find stale issues, detect duplicates, prioritize by impact (Maya, Priya)
6. **Daily standup / what-do-I-do-now** — agent-driven joined query across priority + cycle deadline + blocked-state (Sam)
7. **Portfolio review** — initiative-rollup with projected landing dates (Priya)

## Table Stakes

- Full issue CRUD with filtering by team, state, priority, assignee, cycle, project, label
- Projects, cycles, teams, users, labels, workflow states, comments, documents, milestones, initiatives, attachments, notifications, triage, favorites, custom views, webhooks
- Git branch integration from issue ID
- Search across issue titles, descriptions, comments
- `--json`, `--select`, `--csv`, `--dry-run` on every mutation
- Real-time watch mode

## Data Layer

- Primary entities: Issues, Projects, Cycles, Teams, Users, Labels, WorkflowStates, Comments, Documents, Milestones, Initiatives, IssueRelations
- Sync cursor: `updatedAt`-based incremental sync with cursor pagination over Connection edges
- FTS5 indexes: issue titles + descriptions, comments, documents (content-linked triggers, v4 default)
- Local-only tables: `pp_created` (agent fixture ledger), per-cycle snapshots for historical velocity, issue-history rows for slip detection
- High-gravity fields: issue identifier (ABC-123), title, state, priority, assignee, cycle, project, labels, due date, estimate

## User Vision

The user's hand-off vision (verbatim from `/printing-press-reprint` invocation): the reprint should benefit from v4 machine improvements (MCP surface, agent-native parity, supply-chain hardening, scoring). Prior headline strengths — offline SQLite sync, FTS5 search, cross-cycle comparison, project burndown, `pp_created` fixture lifecycle — are validated and worth carrying forward. The reprint should re-evaluate them against the v4 machine: which still pull weight, which the machine now provides for free, which should be reframed.

Live-testing constraint: API key provided for Phase 5. Only mutate Linear tickets created in this session (with an obvious `pp-test-` or `[printing-press reprint test]` marker). Never mutate any pre-existing ticket, project, cycle, comment, label, team, or member.

## Codebase Intelligence

- Official `@linear/sdk` lives at `linear/linear` (TypeScript monorepo). Schema is at `packages/sdk/src/schema.graphql` (the file used as the spec source).
- Auth: API key passed verbatim in `Authorization` header (no `Bearer` prefix); personal API keys are workspace-scoped.
- Rate limits: ~1500 complexity points/hour for personal API keys. Mutations cost more than queries. Conservative single-level connection queries are required to stay under the per-request complexity ceiling.
- Connection pattern is universal: every list field is `*Connection { nodes { ... }, pageInfo { hasNextPage, endCursor } }`. The v4 GraphQL sync template generates cursor-paginated queries from this shape automatically.
- Community ecosystem (carried from prior): Finesssee/linear-cli (Rust, 60+ commands, most comprehensive), schpet/linear-cli (Ruby, git-aware), czottmann/linearis (Deno, agent-optimized), dorkitude/linctl (Go, Cobra), evangodon/linear-cli (Go), official Linear MCP at mcp.linear.app, tacticlaunch/mcp-linear, @linear/sdk, linear-api (Python).

## Spec Strategy

- GraphQL only. Reuse the SDL at `linear/linear/packages/sdk/src/schema.graphql` (same source as v3.10.0 — frozen snapshot 11 days old, no material drift expected).
- v4 generator emits GraphQL-specific client + sync templates; the dedup-type-fields path that v1 needed manual fixes for is built in. Verify in Phase 2.

## MCP Surface (v4 enrichment plan)

Linear's spec produces ~63 endpoint-mirror tools — well above the >50 threshold where the scorecard's `mcp_remote_transport`, `mcp_tool_design`, and `mcp_surface_strategy` dimensions all penalize default endpoint-mirror surfaces. The Cloudflare pattern applies:

- `mcp.transport: [stdio, http]` — adds streamable HTTP so cloud-hosted agents can connect
- `mcp.orchestration: code` — emits a thin `linear_search` + `linear_execute` pair covering the surface in ~1K tokens
- `mcp.endpoint_tools: hidden` — suppresses raw per-endpoint mirrors (still reachable through `linear_execute`)
- `mcp.intents` — 3-5 named multi-step intents for the highest-frequency workflows: `triage_inbox`, `daily_standup`, `sprint_plan`, `weekly_update`, `backlog_grooming`

This enrichment is the single biggest v4 lift over the v3.10.0 reprint baseline: prior CLI shipped 63 endpoint-mirror tools with `mcp_ready: full` but no transport, no orchestration, no intents.

## v4 Machine Delta Re-validation

Working through the bucket categories from the skill's mandatory version-bump prompt (v3.10.0 → v4.9.0 is a 9-minor jump):

| Bucket | v4 delta | Brief assumption to update |
|---|---|---|
| Transport / reachability | `probe-reachability` is now invoked from Phase 1.7 / 1.9; runtime classification is deterministic | Unchanged for Linear (standard_http) — note in reachability section |
| Scoring rubrics | Phase 4.85 (output review), Phase 4.9 (README/SKILL audit), Phase 4.95 (native code review + `/simplify`), agentic SKILL review (4.8), agent-readiness reviewer | Prior CLI predates all of these → polish loops will surface issues; budget the time |
| Auth modes | Unchanged surface (api_key + bearer + oauth2 + cookie + composed + session_handshake) | Linear stays on api_key — no change |
| MCP surface | Cobratree runtime walker mirrors Cobra commands; agent-native annotations (`mcp:read-only`, `mcp:hidden`); typed exit-code declaration; side-effect verifier; dogfood-env curtailment | All apply to hand-written novel commands; transcendence rows must declare these annotations correctly |
| Discovery | Unchanged for Linear (clean GraphQL spec) | Skip browser-sniff and crowd-sniff |

## Product Thesis

**Name:** linear-pp-cli

**Why it should exist:** Every existing Linear CLI is online-only and most are thin endpoint mirrors. Sam's MCP surface burns thousands of tokens on tool enumeration before it can answer "what should I work on today." Maya cannot answer cycle-over-cycle questions without exporting CSVs. Devon cannot see his blocking queue. Priya cannot get a projected portfolio rollup. Burning all of Linear into local SQLite plus FTS5 unlocks compound queries that no online API call can answer in one round-trip, and the v4 MCP orchestration pair makes the surface affordable for agents at any reasonable workspace size.

## Build Priorities

1. **Foundation** — SQLite store for all primary entities, sync command (full + incremental), FTS5 indexes (content-linked triggers — v4 default)
2. **Absorb** — every command from the v3 manifest (40 absorbed features), regenerated under v4 templates with current naming/exit-codes/annotations
3. **Transcend** — all 12 transcendence features, with `projects burndown` and `cycles compare` no longer deferred (v4 GraphQL sync template makes both buildable)
4. **MCP enrichment** — spec-level `mcp:` block declared before generation (transport, orchestration, intents, endpoint_tools)
5. **Agent-native parity** — `mcp:read-only` annotations on every novel read command; typed exit codes on commands with intentional non-zero control flow; `IsVerifyEnv` / `IsDogfoodEnv` short-circuits on any future side-effect / long-running command (none planned for Linear, but the pattern survives in `pp-cleanup`'s mutation guard)
