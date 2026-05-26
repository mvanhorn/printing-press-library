# Phase 5 — Live Full Dogfood

Date: 2026-05-18
Auth: `NYLAS_API_KEY` (71-char Nylas v3 application key, live)
Spec: `nylas-api-spec-enriched.yaml` (121 paths)
Level: `full`

## Result

- **Verdict:** FAIL (1053 pass / 19 fail / rest skip out of 1072 tests = 98.2% pass)
- **Previous run** (before auth fix): 49 failures. Auth fix reduced by 30.

## Critical bug fix applied during Phase 5

The generated CLI read `NYLAS_ACCESS_TOKEN` from env, but Nylas v3's canonical env var is `NYLAS_API_KEY` (matches their SDKs and `.well-known/agent-skills`). Every API call returned 401. Generator drift from spec-to-config.

Files touched:
- `internal/config/config.go` — read `NYLAS_API_KEY` (primary) with `NYLAS_ACCESS_TOKEN` legacy fallback; dropped unreachable OAuth dead branch in `AuthHeader()`
- `internal/cli/helpers.go`, `internal/cli/auth.go`, `internal/mcp/tools.go`, `internal/cli/agent_context.go` — `NYLAS_ACCESS_TOKEN` → `NYLAS_API_KEY` in all user-facing strings; advertised env-var name in agent-context manifest
- All four files: stale `https://en.wikipedia.org/wiki/Representational_State_Transfer` placeholder → `https://dashboard-v3.nylas.com`
- `auth.go` logout — detect both env vars, prefer `NYLAS_API_KEY`

## Remaining 19 failures — categorized

### Environmental (13) — not code bugs
- `admin list-domains` ×2 (happy_path, json_fidelity): 401 "Nonce is required" — endpoint requires Nylas Service Account auth, not API key
- `connect info-oauth2-token` ×2: 401 "Token expired" — needs a per-grant OAuth access token, not the application API key
- `connect get-oauth2-flow` ×1: 404 "Application not found" — needs a configured OAuth application
- `grants get-by-access-token` ×2: 401 — `/v3/grants/me` is the access-token-introspection endpoint; semantics mismatch with API key auth
- `migration-tools migration-get-jobs` ×2: 404 — application has no migration data

### Real code bugs (6) — polish should auto-fix
- `sync` exit -1 (happy_path): timeout-killed; 30s probe timeout too short for full multi-resource sync. Either chunk sync per-resource in the probe or document expected sync runtime.
- `workflow archive` (json_fidelity) exit None: command writes streaming `{"event":"sync_start",...}` events to stdout, breaking JSON fidelity. Probe expects a single JSON value.
- `export` (json_fidelity) "invalid JSON": export emits NDJSON (one object per line), which is the documented format. The probe rejects NDJSON. Either tag the command with `Annotations["output:ndjson"]` or change the probe.
- `jobs get` / `jobs list` / `jobs prune` (kind=help) exit None: usage text emitted to stdout but missing the `Use:` line or aliases the probe scans for.
- `scheduling get-availability` ×2 (happy_path, json_fidelity): 400 "failed to decode end_time" — the probe substitutes the literal placeholder `"example-value"` for `--end-time`, which expects Unix seconds. Probe value-synthesis bug.
- `connect get-oauth2-flow` happy_path warning: probe defaults `--response-type "example-value"` instead of `code`. Combined with the 404 above, this command is unfixable without app config — could be marked `mcp:requires-app-config`.

## Live novel-feature evidence

Of the 12 novel features, these returned real Nylas data during the run:
- `grants list` — returned actual grants (used by other test fixtures to derive grant IDs)
- `messages send` confirm-by-hash flow — verified the hash-required guard
- `--agent` flag — toggled across the matrix

Local-mirror features (`since`, `search`, `gravity`, `response-time`, `sql`, `export`) are gated on `sync` running first; the timeout-killed sync left these to fail their happy_path probes. Polish will need to either initialise a stub DB before the probe or accept these as second-pass.

## Move-on disposition

PROCEED to Phase 5.5 polish. Two judgements:
1. The auth fix is the load-bearing finding — verdict moves from "fundamentally broken" to "98.2% live pass". Polish's auto-fix loop can address the residual 6 real bugs.
2. The 13 environmental failures should be downgraded in the probe matrix (annotations or per-endpoint skip rules) rather than treated as code defects.
