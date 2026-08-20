# Supabase CLI Brief

## API Identity
- **Domain:** Backend-as-a-service for indie/startup teams. Postgres + Auth (GoTrue) + Storage + Edge Functions + Realtime, organized into projects under organizations. The de-facto "Firebase alternative" for new SaaS.
- **Users:** Full-stack devs (Next.js / Remix / Flutter / mobile) shipping product features; platform engineers managing many projects across orgs; agents triaging users, querying tables, deploying functions, sweeping configurations.
- **Data profile:** Mixed read/write. Project DB tables are user-defined (PostgREST exposes them dynamically). Auth users, Storage objects, and the org/project/secret config plane are first-class entities the platform manages. High read traffic against PostgREST and Storage; configuration changes (secrets, branches, function deploys) are infrequent but high-stakes.

## Reachability Risk
- **None.** No 403/blocked/deprecated issues on `supabase/supabase-js`, `supabase/supabase-py`, or `supabase/cli`. The only "deprecation" signal is legacy JWT anon/service_role keys being phased out for `sb_publishable_*`/`sb_secret_*` formats, and PostgREST's anon-key OpenAPI fetch being removed in April 2026 (we route schema introspection through Management API instead).

## Top Workflows
1. **PostgREST CRUD on user tables** — `GET/POST/PATCH/DELETE /rest/v1/<table>` with `select=`, `=eq.`, `order=`, `limit=`, `Prefer: resolution=merge-duplicates`. Project's bread-and-butter API.
2. **Auth Admin user management** — `GET /auth/v1/admin/users`, create/invite/update/delete, list MFA factors, query audit log. **Requires service_role key.** Notably missing from both the official supabase CLI and the supabase-community MCP.
3. **Storage object lifecycle** — buckets list/create/empty/delete; objects upload/download/sign/list/move/delete. Public + signed URL generation.
4. **Management plane: orgs/projects/secrets/functions/branches** — list/get/create/delete across the platform's config plane. ~108 endpoints in the Management OpenAPI.
5. **Edge Function invocation** — `POST /functions/v1/<name>` with body + headers; user-defined signatures.
6. **Cross-project sweeps** (transcendence territory) — "which projects have STRIPE_KEY secret", "which orgs have branches with un-merged DB drift", "who joined Auth this week across all our projects".

## Table Stakes (competition matrix)
- Official `supabase` CLI: local-dev (Docker/migrations/gen types), function deploy, secrets, projects/orgs, branches, link/login. **NOT covered:** PostgREST row CRUD, Auth Admin, runtime function invoke, Storage object CRUD (only basic ls/cp).
- `supabase-community/supabase-mcp` (2.7k★): Management API + raw SQL exec + migrations + advisors + types gen + branching + function deploy. **NOT covered:** Auth Admin.
- `alexander-zuev/supabase-mcp-server` (~820★): SQL exec + Mgmt API + Auth Admin (via Python SDK) + logs.

Match expectation: every endpoint in the Management OpenAPI as an endpoint-mirror command, plus the project-surface commands the official CLI and most MCPs skip.

## Data Layer
- **Primary entities (sync to local SQLite):** organizations, projects, edge functions per project, branches per project, secret **names** per project (NEVER values), api-keys metadata per project.
- **Excluded from sync:** Auth users (PII + per-project + unbounded), Storage objects (unbounded), user DB rows (user-defined schema).
- **Sync cursor:** Management API endpoints are mostly list-style without explicit pagination cursors (small N: orgs in dozens, projects in low hundreds for power users); use `synced_at` for staleness.
- **FTS/search:** project name + project ref + org name; function slug + function name. Tag tables are small enough for SQL filters without FTS.

## Codebase Intelligence
- **Source:** Research aggregated from supabase/cli (Go binary), supabase-community/supabase-mcp (TypeScript), supabase-js (TS SDK), supabase-py (Python SDK), and Management API OpenAPI introspection.
- **Auth shapes (three credential types):**
  - **PAT** (`sbp_...`): `Authorization: Bearer <PAT>` → Management API (`api.supabase.com/v1/...`). Env: `SUPABASE_ACCESS_TOKEN`.
  - **Publishable key** (`sb_publishable_...` or legacy JWT `eyJ...`): header `apikey: <key>` → project APIs (PostgREST, Auth, Storage, Functions). Env: `SUPABASE_PUBLISHABLE_KEY` (canonical) or `SUPABASE_ANON_KEY` (legacy alias).
  - **Secret key** (`sb_secret_...` or legacy JWT): same `apikey: <key>` header but bypasses RLS and unlocks Auth Admin endpoints. Env: `SUPABASE_SERVICE_ROLE_KEY` (canonical) or `SUPABASE_SECRET_KEY`.
  - **Edge Functions gotcha:** keys go in `apikey:` not `Authorization: Bearer`. Sending publishable/secret as Bearer → 401.
- **Data model:** Org → Projects → (DB tables | Auth users | Storage buckets | Edge Functions | Branches | Secrets | API keys). Cross-org rollup is a natural local-store transcendence target.
- **Rate limiting:** Per-endpoint, undocumented thresholds; observed via 429. Management API has stricter limits than project APIs.
- **Architecture note:** PostgREST's spec is dynamically generated per project from the DB schema; the anon-key `/rest/v1/` OpenAPI fetch is being removed April 2026. Schema introspection for offline planning must go through the Management API's project-OpenAPI endpoint with `Read-only project database access` scope.

## Spec Source Decision
- **Primary spec:** `https://raw.githubusercontent.com/supabase/supabase/master/apps/docs/spec/api_v1_openapi.json` → Management API, 108 path items, OpenAPI 3.0 JSON. Downloaded to `$API_RUN_DIR/specs/supabase-management.json` (456KB).
- **Secondary surface — Auth (GoTrue):** `https://raw.githubusercontent.com/supabase/auth/master/openapi.yaml` → 43 path items. Downloaded for reference but NOT merged with the primary spec (different auth shape: `apikey` vs `Authorization: Bearer`, different base URL: `<ref>.supabase.co` vs `api.supabase.com`).
- **Hand-written project-surface absorbs:** Auth Admin (5 commands), Storage (~8 commands), PostgREST CRUD (4 verbs over runtime `<table>` arg), Edge Function invoke (1 command). These read `SUPABASE_URL` + `SUPABASE_PUBLISHABLE_KEY` or `SUPABASE_SERVICE_ROLE_KEY` from env and hit `<ref>.supabase.co/...`.

## Live Testing Profile (Phase 5)
- **User provided:** `SUPABASE_URL`, `SUPABASE_PUBLISHABLE_KEY` (new format `sb_publishable_...`), `SUPABASE_SERVICE_ROLE_KEY` (legacy JWT — works fine alongside new publishable key).
- **NOT provided:** `SUPABASE_ACCESS_TOKEN` (PAT for Management API). Phase 5 must mark Management-API endpoints as `BLOCKED_FIXTURE` for live testing; project-surface commands (Auth Admin, Storage, PostgREST, Functions invoke) are live-testable against the provided project `bvfvwyymezgryrcllnqs`.

## Product Thesis
- **Name:** `supabase-pp-cli` (slug: `supabase`)
- **Why it should exist:** The official `supabase` CLI is Docker + migrations + dev-loop tooling, not a runtime API surface. The supabase-community MCP covers Management + raw SQL but skips Auth Admin entirely. There is no agent-native, single-binary Go CLI that exposes the *runtime* Supabase surface — PostgREST row CRUD, Auth Admin user CRUD, Storage object lifecycle, Function invoke — with `--json --select --dry-run` consistency, alongside the full Management API endpoint mirror. Add a local SQLite cache of orgs+projects+functions+branches+secret-names and you can answer cross-project queries no live API resolves in one call ("which projects have STRIPE_KEY", "which functions exist across all orgs", "what branches need merging").

## Build Priorities
1. **Foundation (P0):** Management API spec ingest, generated client with PAT bearer auth, secondary project-surface client with apikey-header auth, store schema for orgs/projects/functions/branches/secret_names, sync command per Management entity.
2. **Absorbed (P1):** all 108 Management endpoint-mirror commands; hand-written Auth Admin (5), Storage (8), PostgREST CRUD (4), Functions invoke (1) — these use `SUPABASE_URL` + project keys.
3. **Transcendence (P2):** cross-project secret-name audit, function inventory across orgs, branch-drift sweep, schema-introspection cache, Auth user drift detection.

## Open Risks
- **Two distinct auth shapes** in one CLI (Bearer PAT for Management vs apikey header for project surface). Must not be conflated by templates. Generator's `x-auth-vars` declares the primary Bearer scheme; hand-written project commands read project env vars directly.
- **PostgREST is per-project dynamic.** Generated commands cannot know user table names at build time — `pgrst select <table>` takes table as a runtime positional arg.
- **Realtime out of scope** (WebSocket, doesn't fit one-shot CLI). Documented gap.
- **Edge Function invocation signatures are user-defined.** Generic `functions invoke <name> --body @payload.json` rather than typed per-function commands.
- **PII risk on Auth users.** If we ever sync them, store only structural fields (id, email, created_at) behind explicit opt-in flag. Default sync excludes Auth users entirely.
- **Service_role key bypasses RLS.** Hand-written commands that use it (Auth Admin, raw Storage) must clearly label that limitation in their help text.
