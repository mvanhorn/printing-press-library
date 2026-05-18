# Supabase — Novel Features Brainstorm (full subagent audit trail)

## Customer model

### Persona 1: Priya, full-stack Next.js dev (1-2 production projects)
**Today**: Runs the Supabase web dashboard in one tab, `psql` against the connection string in a terminal, `supabase functions deploy` from her repo. Support ticket → click into Auth, search by email, flip email-confirmed, copy user ID into SQL editor to look at their orders row.

**Weekly ritual**: Deploys 1-2 edge functions on Friday push day. Reads Auth → Users every few days. Adds 1-2 new secrets per sprint via Secrets UI. Hand-edits one row a week in SQL editor for support tickets.

**Frustration**: Auth Admin and PostgREST row-level CRUD don't exist in the official CLI, so any support workflow forces a browser context switch. Cannot script "show me this user and their last 5 orders" without writing a Node script.

### Persona 2: Diego, platform engineer at a 12-project agency
**Today**: 12 Supabase projects across 3 orgs, 5 client apps. Uses official `supabase` CLI for migrations. Security team asks "which projects have STRIPE_KEY?" → he opens 12 browser tabs and eyeballs each Secrets page.

**Weekly ritual**: Mon — secret-name audit across client projects. Tue — preview branch drift review. Wed — API key rotation. Thu — review Edge Function deploys from 2-3 client engineers.

**Frustration**: No tool answers "across all my orgs/projects, where is X?" in one call. supabase-community MCP shows Management API in chat but doesn't cache or join across projects. Cannot `SELECT project_ref FROM projects WHERE secret_name = 'STRIPE_KEY'` anywhere today.

### Persona 3: Aria, support-tier-2 AI agent
**Today**: Runs as Claude Code agent with service_role key in env. "This user's MFA is broken, fix it" → only dashboard or raw curl-to-GoTrue. Falls back to instructing a human to click.

**Weekly ritual**: Triages ~30 user tickets/week. Each: look up user in Auth Admin, check last sign-in, sometimes flip email-confirmed or remove stale MFA, peek at their `memberships` row.

**Frustration**: Auth Admin endpoints unreachable from official CLI AND supabase-community MCP. PostgREST CRUD missing from MCP. Generates brittle Python snippets instead of running typed commands with `--json`.

### Persona 4: Sam, indie Flutter hacker
**Today**: Single org, single project. Uses Supabase Studio for everything. Has Claude Code but can't ask it to "delete this orphaned storage object" — no tool exists.

**Weekly ritual**: Uploads bucket assets weekly. 1-2 RLS policies. Invokes edge functions during dev to test webhook payloads. Signs Storage URLs to share screenshots with beta users.

**Frustration**: Storage object lifecycle + signed-URL generation requires Node REPL with supabase-js. Wants `<binary> storage objects sign bucket/path --expires 1h` piped to `pbcopy`.

## Candidates (pre-cut)

| # | Candidate | Source | Verdict |
|---|---|---|---|
| C1 | `secrets where-name <NAME>` — cross-project secret audit | c | Keep |
| C2 | `functions inventory [--org X]` — cross-project function rollup | c | Keep |
| C3 | `branches drift` — stale preview branches | c+b | Keep |
| C4 | `auth-admin lookup <email> [--context-table T]` | a | Keep (tighten) |
| C5 | `storage usage [--bucket X]` — size/count per bucket | b | Keep |
| C6 | `pgrst schema [--table X]` — schema via Mgmt API | f | Keep |
| C7 | `projects health` — local-store estate rollup | c | Keep |
| C8 | `storage orphans <bucket> --reference-table T` | b | Keep (watch scope) |
| C9 | `auth-admin recent --since 7d` — cross-project signups | c+b | Keep |
| C10 | `functions replay <name> --log-id <id>` | b | Cut (logs API risk) |
| C11 | `secrets rotation` — age-sort audit | f | Keep |
| C12 | `pgrst peek <table>` — first 5 rows | b | Cut (thin wrapper) |
| C13 | `doctor matrix` — reachability across all projects | a | Cut (overlap + rate-limit risk) |
| C14 | `views install` — canned SQL views | c | Cut (UX sugar, no new capability) |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---|---|---|---|---|
| 1 | Cross-project secret-name audit | `secrets where-name <NAME>` | 9/10 | Diego | Local SQL join over `project_secrets_names` + `projects` + `organizations`; no Mgmt API endpoint answers this |
| 2 | Function inventory rollup | `functions inventory [--org X]` | 8/10 | Diego | Local SQL aggregate over synced `project_functions`; competitors are per-project only |
| 3 | Branch-drift sweep | `branches drift [--older-than 7d]` | 8/10 | Diego | Local SQL age filter over `project_branches`; service-specific concept |
| 4 | Auth user lookup-with-context | `auth-admin lookup <email> [--context-table T --context-key col]` | 8/10 | Priya, Aria | Pairs real Auth Admin `GET /admin/users?email=` + optional PostgREST `select` — neither surface exists in official CLI or supabase-community MCP |
| 5 | Project health rollup | `projects health` | 7/10 | Diego | LEFT JOIN across projects + functions + branches + api_keys + secret_names tables |
| 6 | RLS-aware schema introspection | `pgrst schema [--table X]` | 7/10 | Priya, Aria, Sam | Real Mgmt API `GET /v1/projects/{ref}/api/rest` + OpenAPI parse; documented replacement for anon-key path that's being removed Apr 2026 |
| 7 | Storage bucket usage rollup | `storage usage [--bucket X]` | 7/10 | Sam | Pages Storage list endpoint + sums sizes; free-tier ceiling answer |
| 8 | Cross-project recent signups | `auth-admin recent [--since 7d]` | 7/10 | Diego, Aria | Fan-out Auth Admin calls per synced project, aggregate by created_at window |
| 9 | Orphan storage objects | `storage orphans <bucket> --reference-table T --reference-column c` | 7/10 | Sam | Real Storage list + real PostgREST select + set difference; Sam's "delete avatars no profile points to" |
| 10 | Secrets rotation audit | `secrets rotation [--older-than 180d]` | 6/10 | Diego | Local SQL age-sort over `project_secrets_names.updated_at` |

(Note: subagent listed 8 survivors; one additional candidate `projects health` was kept inline. Re-counting and re-scoring against rubric — final 10 survive ≥5/10, but to keep the shipping scope tight I'm filing the top 8 as novel transcendence features and treating `projects health` + `secrets rotation` as bonus rollups that flow naturally from the same local-store data. They cost nothing extra to ship.)

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| `functions replay <name> --log-id <id>` (C10) | Mgmt logs API reliability + scope creep on plain `functions invoke` | F4 (auth-admin lookup) — same pair-two-endpoints pattern, but proven |
| `pgrst peek <table>` (C12) | Thin rename of `pgrst select <table> --limit 5` | F6 (pgrst schema) — gives typed schema not 5-row sample |
| `doctor matrix` (C13) | Overlaps framework `doctor`; rate-limit risk on live-hammering many projects | F5 (projects health) — same "estate state" question via local store |
| `views install` (C14) | UX sugar over `sql`; encourages view/schema drift | F1/F2/F3/F10 — each materialized rollup IS the canned view |
