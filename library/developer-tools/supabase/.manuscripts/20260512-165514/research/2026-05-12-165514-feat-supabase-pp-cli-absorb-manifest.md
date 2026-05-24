# Supabase Absorb Manifest

## Scope

Two surfaces under one CLI:

- **Management API** (`api.supabase.com/v1`) — 108-path OpenAPI spec, Bearer PAT auth. Generated as endpoint-mirror commands by the press.
- **Project APIs** (`<ref>.supabase.co`) — hand-written commands covering Auth Admin, Storage, PostgREST, and Edge Function invoke. These use the `SUPABASE_URL` + `SUPABASE_PUBLISHABLE_KEY` / `SUPABASE_SERVICE_ROLE_KEY` env vars and hit per-project hosts.

Why two surfaces, not multi-spec merge: different base URLs (api.supabase.com vs `<ref>.supabase.co`), different auth headers (`Authorization: Bearer` vs `apikey:`), different credential types. The Datadog retro showed multi-spec merges with divergent auth shapes break the templates. Cleaner to generate Management as primary and hand-write the project surface.

## Absorbed (match or beat every existing tool)

| # | Feature group | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | **Management API endpoint mirror (108 endpoints)** — orgs/projects/secrets/branches/functions/api-keys/snippets/database/billing/auth-config/storage-config/oauth/vanity-subdomain/network-bans/ssl-enforcement | supabase-community/supabase-mcp (Mgmt subset), official supabase CLI (curated subset) | Generated endpoint-mirror commands from Management OpenAPI | Full 108-endpoint coverage, `--json --select --dry-run`, agent-native by default, single-binary Go, MCP-ready |
| 2 | **Auth Admin: users list/create/invite/update/delete + factors + audit** | supabase-py SDK admin namespace, alexander-zuev/supabase-mcp-server | Hand-written `auth-admin` commands using service_role key | Missing from official CLI AND from supabase-community MCP — clean differentiation |
| 3 | **Storage buckets: list/create/get/update/empty/delete** | supabase-js storage client | Hand-written `storage buckets` commands | Full CRUD vs official CLI's basic ls/cp/mv/rm; agent-friendly output |
| 4 | **Storage objects: upload/download/list/move/copy/sign/delete** | supabase-js storage client | Hand-written `storage objects` commands | Signed URL generation + streaming uploads + `--public` shortcut |
| 5 | **PostgREST CRUD on user tables** — `select/insert/upsert/delete` with `--filter k=eq.v`, `--select cols`, `--order col.desc`, `--limit N` | supabase-js postgrest builder, supabase-py | Hand-written `pgrst <verb> <table>` (runtime `<table>` arg) | The runtime API surface the official CLI explicitly omits |
| 6 | **Edge Function invocation** | supabase-js `functions.invoke()` | Hand-written `functions invoke <name> --body @file.json` | Live runtime invocation; official CLI only deploys, doesn't invoke |
| 7 | **Cross-org sync to SQLite** — orgs+projects+functions+branches+secret_names | Custom (no competitor does this) | Generator-emitted `sync` workflow | Foundation for all transcendence |
| 8 | **Local SQL + FTS** — `sql` + `search` commands | Generator-emitted framework | Stock store-query surface across synced entities | Offline cross-project queries no competitor has |
| 9 | **Dual-credential `doctor`** | Generator-emitted framework command | Probes both `Authorization: Bearer <PAT>` and `apikey: <publishable>` paths | Validates both Management + project surfaces in one call |
| 10 | **Local-dev Docker stack + migrations + `db push`** | Official `supabase` CLI | OUT OF SCOPE | Use official CLI for `supabase start/db push/gen types`; this CLI focuses on runtime API surface |
| 11 | **Realtime WebSocket subscribe** | supabase-js realtime client | OUT OF SCOPE | WebSocket doesn't fit one-shot CLI; documented gap |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|-------------------------|
| 1 | Cross-project secret-name audit | `datadog-pp-cli` style: `secrets where-name <NAME>` | 9/10 | Diego | LEFT JOIN over `project_secrets_names + projects + organizations`; no Mgmt API endpoint answers this in one call |
| 2 | Function inventory rollup | `functions inventory [--org X]` | 8/10 | Diego | Aggregate over synced `project_functions`; surfaces "which projects deployed `stripe-webhook`?" and "which functions haven't redeployed in 90 days?" |
| 3 | Branch-drift sweep | `branches drift [--older-than 7d]` | 8/10 | Diego | Filter synced `project_branches` by age + status; the Tuesday cleanup ritual |
| 4 | Auth user lookup-with-context | `auth-admin lookup <email> [--context-table T --context-key col]` | 8/10 | Priya, Aria | Pairs real Auth Admin `GET /admin/users?email=` + optional PostgREST `select` on a user-named table joined on user_id |
| 5 | Project health rollup | `projects health` | 7/10 | Diego | LEFT JOIN across `projects + functions + branches + api_keys + secret_names`; one screen, the whole estate |
| 6 | RLS-aware PostgREST schema | `pgrst schema [--table X]` | 7/10 | Priya, Aria, Sam | Mgmt API `GET /v1/projects/{ref}/api/rest` + OpenAPI parse; documented replacement for anon-key path being removed April 2026 |
| 7 | Storage bucket usage rollup | `storage usage [--bucket X]` | 7/10 | Sam | Pages Storage list endpoint + sums sizes; answers "is my free tier full?" |
| 8 | Cross-project recent signups | `auth-admin recent [--since 7d]` | 7/10 | Diego, Aria | Fan-out Auth Admin calls per synced project, aggregate by created_at window |
| 9 | Orphan storage objects | `storage orphans <bucket> --reference-table T --reference-column c` | 7/10 | Sam | Storage list + PostgREST select + set difference; "delete avatars no profile row points to" |
| 10 | Secrets rotation audit | `secrets rotation [--older-than 180d]` | 6/10 | Diego | Age-sort over synced `project_secrets_names.updated_at`; security-posture sweep |

All 10 transcendence features build on the local store of orgs + projects + functions + branches + secret_names + api_keys, plus real API calls to Management/Auth/Storage. None depend on LLM, browser, or external services beyond Supabase APIs.

## Stubs

None planned. Every transcendence row is implementable in-session.

## Out-of-Scope (documented gaps)

- **Local-dev Docker stack** (`supabase start/stop/status/db reset`) — use the official `supabase` CLI. This CLI is runtime API surface, not dev-loop tooling.
- **Migrations & types gen** (`supabase db push/pull/diff/lint`, `supabase gen types`) — official CLI's specialty.
- **Realtime WebSocket subscribe** — doesn't fit one-shot CLI shape.
- **Edge Function deploy + serve** — covered by the Management API endpoint mirror (`POST /v1/projects/{ref}/functions/{slug}/deploy`) but local dev (`supabase functions serve`) belongs to the official CLI.
- **Auth user PII sync to local store** — Auth users are NOT synced to the local SQLite (PII risk, unbounded). Live queries only.
- **Storage object metadata sync to local store** — buckets fine to cache, objects deliberately not synced (unbounded). Live `storage objects list` only.
- **PostgREST per-project schema cache** — schema introspection is on-demand via the Management `/api/rest` endpoint (novel feature F6); not pre-synced because it's per-project dynamic.
