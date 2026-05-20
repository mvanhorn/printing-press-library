# Supabase CLI — Shipcheck Report

## Verdict: PASS (6/6 legs)

| Leg | Result | Exit | Elapsed |
|-----|--------|------|---------|
| dogfood | PASS | 0 | 1.5s |
| verify | PASS | 0 | 2.2s |
| workflow-verify | PASS | 0 | 15ms |
| verify-skill | PASS | 0 | 241ms |
| validate-narrative | PASS | 0 | 172ms |
| scorecard | PASS | 0 | 111ms |

## Scorecard: 92/100 (Grade A)

- Output Modes: 10/10, Auth: 10/10, Error Handling: 10/10, Doctor: 10/10, Agent Native: 10/10, Local Cache: 10/10, Breadth: 10/10, Workflows: 10/10, Path Validity: 10/10, Data Pipeline Integrity: 10/10, Sync Correctness: 10/10
- MCP Remote Transport: 10/10, MCP Tool Design: 10/10, MCP Surface Strategy: 10/10 (Cloudflare pattern applied cleanly)
- MCP Quality 8/10, Auth Protocol 9/10, Agent Workflow 9/10, Insight 8/10, Vision 8/10, README 8/10, Terminal UX 9/10
- Cache Freshness 5/10 (cache freshness helper not emitted — recurrence of retro #1131)
- Type Fidelity 3/5, Dead Code 5/5

## Top blockers found + fixes applied

| # | Blocker | Leg | Fix |
|---|---|---|---|
| 1 | Spec had no `servers:` block (Management API base URL) | generate-time refusal (#1012 protection) | Added `servers: [{url: https://api.supabase.com}]` |
| 2 | Spec had 169 dangling `fga_permissions` security refs (declared on operations, not in `components.securitySchemes`) | scorecard FAIL | Added stub `fga_permissions` apiKey scheme to `components.securitySchemes` |
| 3 | Spec had dangling `oauth2` security refs | scorecard FAIL | Added stub `oauth2` authorizationCode scheme to `components.securitySchemes` |
| 4 | narrative.quickstart referenced `pgrst select` and `storage objects sign` (both known gaps, not built) | validate-narrative FAIL (2 examples) | Replaced with `sync --json` and `secrets where-name STRIPE_KEY --json` — commands that actually exist |
| 5 | narrative.recipes referenced `pgrst select` | validate-narrative FAIL | Removed deprecated recipes; kept the 5 recipes that map to built commands |

## Before / after

- Before fixes: dogfood PASS, verify PASS, workflow-verify PASS, verify-skill PASS, validate-narrative FAIL (2 examples), scorecard FAIL (fga_permissions ref)
- After: 6/6 PASS, scorecard 92/100 Grade A.

## Behavioral correctness

All 8 transcendence novel-feature commands pass per-row Cobra resolution — `<cli> <leaf> --help` returns the correct Usage line for each:
- `secrets where-name <NAME> [flags]`
- `functions inventory [flags]`
- `branches drift [flags]`
- `auth-admin lookup <email> [flags]`
- `pgrst schema [flags]`
- `projects estate [flags]`
- `storage usage [flags]`
- `auth-admin recent [flags]`

Cannot run live behavioral correctness on local-only novel features (`secrets where-name`, `functions inventory`, `branches drift`, `projects estate`, `secrets rotation`) without first syncing the Management API — and the user provided project keys (not a Management PAT), so sync will fail. Phase 5 will exercise the project-surface novels (`auth-admin lookup`, `auth-admin recent`, `storage usage`, `pgrst schema` — the last two against the user's project).

## Generator behavior observations

1. **`x-mcp` enrichment applied cleanly** in the single-spec case (contrast with Datadog's multi-spec `mergeSpecs` bug #1044). MCP surface emits the Cloudflare orchestration pair instead of 108 endpoint mirrors.
2. **`Hidden: true` on resource parents** recurred (issue #1209 from Datadog retro). Required manually unhiding `projects`, `organizations`, `branches`.
3. **Naming collision** (`projects health` from spec vs novel rollup) — required renaming the novel to `projects estate`. Not unique to Supabase; any time a spec has an endpoint named after a desired novel feature, the agent must rename or namespace.
4. **Spec missing `servers:` block** correctly refused (post-#1012 protection). Good signal.
5. **Dangling security scheme refs in spec** (`fga_permissions`, `oauth2`) caused scorecard hard-fail. Spec-side cleanup needed before scorecard could parse. Recurrence pattern — was the same shape as Datadog's `AuthZ` issue (just a different scheme name).

## Final verdict: `ship`

All ship-threshold conditions met:
- shipcheck exits 0, all legs PASS ✓
- verify is PASS ✓
- dogfood doesn't fail on spec parsing, binary path, or skipped examples ✓
- dogfood wiring checks pass ✓
- workflow-verify is `workflow-pass` ✓
- verify-skill exits 0 ✓
- scorecard is 92 (≥ 65) ✓
- All 8 novel-feature commands have valid Cobra-resolvable paths ✓

## Known Gaps (documented; do not block ship)

- **Hand-written project-surface CRUD wrappers**: Auth Admin bulk operations (create/invite/update/delete users, MFA factors), Storage object lifecycle (upload/download/sign/delete), PostgREST row CRUD (select/insert/upsert/delete), Edge Function invoke. These were absorbed-but-deferred at user-approved Phase 3 scope cut. Documented in README's Known Gaps section.
- **Cache Freshness 5/10**: scorer wants a cache-freshness helper that isn't emitted by the generator. Retro #1131 already filed against this.
- **Cross-project `auth-admin recent` is single-project effectively**: SUPABASE_URL points at ONE project, and the same service_role key only authorizes against that project's Auth API. The command lists synced projects and would call each one's Auth Admin, but skips projects whose ref differs from the env URL with a clear `multi_project_fanout_limit` reason. True cross-project fan-out needs per-project credentials in env — documented in the command's `--help`.
