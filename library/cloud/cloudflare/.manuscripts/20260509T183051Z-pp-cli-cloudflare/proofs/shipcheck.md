# Cloudflare CLI — Shipcheck Report

## Final Verdict: **ship-with-gaps**

5/6 shipcheck legs PASS. The single failure is a printing-press tooling artifact (scorecard rejects Cloudflare's spec semantic validation due to an unresolved `bearerAuth` security scheme reference in Cloudflare's own `api-schemas` repo).

## Scorecard

Run without `--spec` (the canonical no-tooling-issue mode): **84/100 — Grade A**

| Dimension | Score |
|---|---|
| Output Quality | 10/10 |
| Breadth | 10/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 9/10 |
| Agent Workflow | 9/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 4/5 |
| Dead Code | 5/5 |

## Shipcheck Summary (last run, retry-3 with PYTHONUTF8=1)

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 27.436s |
| verify | PASS | 0 | 40.476s |
| workflow-verify | PASS | 0 | 214ms |
| verify-skill | PASS | 0 | 8.655s |
| validate-narrative | PASS | 0 | 4.681s |
| scorecard | FAIL | 3 | 1.633s |

## What was built

- **181 product resources, 2873 endpoint-mirror commands** generated from the official Cloudflare OpenAPI spec (cloudflare/api-schemas, 9.76 MB JSON).
- **8 transcendence commands** spanning Idempotent infra (`dns apply`, `redirect set`), Verification (`propagate watch`, `cache purge release`), Cross-product (`where-is`, `zones diff`, `worker bindings show`), and Composition (`setup_zone`).
- **Cloudflare-style auth**: API Token (Bearer) preferred via `CLOUDFLARE_API_TOKEN`; legacy `CLOUDFLARE_API_EMAIL` + `CLOUDFLARE_API_KEY` supported as fallback. The auto-generated config picked the wrong scheme; we manually corrected the config + client + doctor per the SKILL Phase 2 safety net.
- **40 MB single-file binary** (`cloudflare-pp-cli.exe`).

## Top blockers found and fixed during shipcheck

1. **Windows portability bug in printing-press v4.2.0**: `filepath.Join` was used to construct `embed.FS` paths, producing `\` separators that the virtual FS can't read on Windows. Patched the source (`generator.go:2706` and `plan_generate.go:93`), rebuilt the binary. Also patched `naming.go:ValidationBinary` to add `.exe` suffix on Windows.
2. **Auth picked the wrong env var**: Generator chose `CLOUDFLARE_API_EMAIL` from the OpenAPI's multi-scheme security list. Patched `internal/config/config.go`, `internal/client/client.go`, and `internal/cli/doctor.go` to prefer `CLOUDFLARE_API_TOKEN` (Bearer) and fall back to legacy email+key.
3. **Novel-feature dry-run made API calls before short-circuit**: validate-narrative caught it — examples like `setup_zone example.com --origin ... --dry-run` tried to look up the zone on the API and failed with "zone not found" (because verify env has no real auth). Patched all 7 mutating/composing novel commands to short-circuit `dryRunOK(flags)` BEFORE any API call.
4. **`--proxied=false` parsed as flag literal `proxied=false` by verify-skill**: Renamed to space-separated form (`--proxied false`) in SKILL.md, README.md, and research.json examples.
5. **Python `cp1252` codec failed on a non-ASCII domain in SKILL.md examples**: Set `PYTHONUTF8=1` for shipcheck invocation.

## Known Gaps (why ship-with-gaps)

The single remaining failure — `scorecard --spec ...` — is caused by an unresolved `bearerAuth` `$ref` in Cloudflare's own OpenAPI spec. The printing-press scorecard's strict spec semantic validator refuses to score in that mode. Running the same scorecard without `--spec` produces an 84/100 (Grade A) score with no warnings. Fix would require patching Cloudflare's spec or relaxing the scorecard's validator — both out-of-session.

This is documented in the README's `## Known Gaps` section. Users of this CLI are unaffected; the spec-validation step is internal to the printing-press tooling, not a runtime concern of the CLI.
