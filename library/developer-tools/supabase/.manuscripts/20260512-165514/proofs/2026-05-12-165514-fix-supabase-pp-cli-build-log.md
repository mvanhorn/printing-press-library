# Supabase CLI — Phase 3 Build Log

## Scope decisions during the run

- **Spec source:** Management API only (`https://raw.githubusercontent.com/supabase/supabase/master/apps/docs/spec/api_v1_openapi.json`, 108 path items). Auth and Storage specs downloaded for reference but not merged — different auth shape (`apikey:` header vs `Authorization: Bearer`) and different base URL (`<ref>.supabase.co` vs `api.supabase.com`) would have triggered the same multi-spec/auth conflation that broke Datadog generation.
- **Spec patches applied:**
  - Added `servers: [{url: https://api.supabase.com}]` block (spec doesn't ship one; generator now refuses to ship a CLI with no base URL — issue #1012's protection).
  - Added `x-mcp: {transport: [stdio, http], orchestration: code, endpoint_tools: hidden}` for Cloudflare-pattern MCP — confirmed working (25 MCP tools generated, not 108+ endpoint mirrors).
  - Added `x-auth-env-vars: [SUPABASE_ACCESS_TOKEN]` to the bearer security scheme so the generator emits the canonical env var name (not a slug-derived one).
- **Phase 3 scope contraction (re-approved by user):** Build 8 novel transcendence features deeply; document hand-written project-surface CRUD wrappers (Auth Admin user CRUD, Storage object lifecycle, PostgREST select/insert/upsert/delete, Edge Function invoke) as Known Gaps in the README for a follow-up polish session. The 108 Management endpoint mirrors stay (generator emits them at near-zero marginal cost).

## Novel features built (8)

| # | Command | File | Pattern | Status |
|---|---|---|---|---|
| 1 | `secrets where-name <NAME>` | `internal/cli/secrets_top.go` | Pure local SQL join (`secrets`+`projects`+`organizations`) | ✓ |
| 2 | `functions inventory` | `internal/cli/functions_top.go` | Local SQL aggregate over `functions`+`projects` | ✓ |
| 3 | `branches drift --older-than 7d` | `internal/cli/branches_drift.go` | Local SQL age filter over `branches` table | ✓ |
| 4 | `auth-admin lookup <email>` | `internal/cli/auth_admin.go` | Live Auth Admin call + optional PostgREST join | ✓ |
| 5 | `pgrst schema [--table X]` | `internal/cli/pgrst_top.go` | Management API `/v1/projects/{ref}/api/rest` + OpenAPI parse | ✓ |
| 6 | `projects estate` | `internal/cli/projects_estate.go` | LEFT JOIN across 5 locally-synced tables | ✓ |
| 7 | `storage usage` | `internal/cli/storage_top.go` | Storage `list bucket` + per-bucket `object/list` aggregation | ✓ |
| 8 | `auth-admin recent --since 7d` | `internal/cli/auth_admin.go` | Fan-out Auth Admin per synced project + window aggregation | ✓ |
| Bonus | `secrets rotation --older-than 180d` | `internal/cli/secrets_top.go` | Local SQL age-sort over `secrets.updated_at` | ✓ |

Also shipped: `internal/cli/project_surface.go` — small ad-hoc HTTP helper for the project-runtime APIs (Auth, Storage, PostgREST) used by features 4, 7, 8.

All 8 follow the verify-friendly RunE shape (`len(args)==0 → cmd.Help()`, `dryRunOK(flags) → return nil`, no `Args:cobra.MinimumNArgs`, no `MarkFlagRequired`). All read-only commands carry `mcp:read-only` annotation.

## Renames during build

- The novel `projects health` (per the original research.json) collided with the spec-derived `projects health` command that wraps `/v1/projects/{ref}/health/services`. Renamed to **`projects estate`** to avoid the collision. research.json updated accordingly.

## Edits to generated files

- `internal/cli/branches.go`: removed `Hidden: true`; added `cmd.AddCommand(newBranchesDriftCmd(flags))`.
- `internal/cli/projects.go`: removed `Hidden: true`; added `cmd.AddCommand(newProjectsEstateCmd(flags))`.
- `internal/cli/organizations.go`: removed `Hidden: true`.
- `internal/cli/root.go`: registered 5 new top-level parents — `secrets`, `functions`, `auth-admin`, `pgrst`, `storage`.

These are the same `Hidden:true`-on-resource-parents fixes as the Datadog retro filed in #1209.

## Generator behavior

- **MCP code-orchestration applied cleanly.** No "1269 MCP tools" warning. The single-spec generation case for `x-mcp` works correctly (in contrast to the Datadog v1+v2 multi-spec case that hit #1044's mergeSpecs bug).
- **Two unhealthy patterns from Datadog did NOT recur:** the OAuth2-over-apiKey multi-scheme selection bug (#979) didn't fire because Supabase only declares one scheme (Bearer HTTP); the slug-derived env var bug didn't fire because `x-auth-env-vars` on the scheme pinned the env var name.
- **One known issue did recur:** `Hidden: true` on resource parents (#1209). Three files needed manual unhiding.

## Intentionally deferred / Known Gaps

These are absorbed-but-not-built rows from the Phase 1.5 manifest. README will document them:

- **Auth Admin user CRUD** (create, invite, update, delete, MFA factors). The novel `auth-admin lookup` and `auth-admin recent` ship; the bulk-CRUD wrappers don't. Use the Supabase dashboard or `supabase-js` admin namespace for now.
- **Storage object lifecycle** (upload, download, sign URL, move, copy, delete). Bucket-level `storage usage` ships; object-level wrappers don't. Use `supabase-js` storage client.
- **PostgREST CRUD** (`pgrst select/insert/upsert/delete`). Schema introspection (`pgrst schema`) ships; row CRUD doesn't. Use `curl` against `/rest/v1/<table>` with the apikey + Authorization headers, or supabase-js.
- **Edge Function invoke**. Function lifecycle is in the Management endpoint mirror (`projects functions deploy/list/get/etc.`); runtime invocation (`POST /functions/v1/<name>`) is a documented gap.

## Build status

```
PASS go mod tidy
PASS govulncheck ./...
PASS go vet ./...
PASS go build ./...
PASS build runnable binary
PASS supabase-pp-cli --help
PASS supabase-pp-cli version
PASS supabase-pp-cli doctor
```

All 8 generator quality gates green. All 8 transcendence commands pass per-row Cobra resolution (`<cli> <leaf> --help` returns a clean Usage line with the leaf path + flags, not a parent fall-through).
