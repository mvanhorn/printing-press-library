# v0-pp-cli Shipcheck Report

## Summary
- **Verdict:** ship
- **Shipcheck:** PASS (7/7 legs)
- **Scorecard:** 93/100 (Grade A)
- **Verify:** 100% pass rate (PASS)
- **Dogfood:** PASS — 8/8 novel features survived, 0 dead flags, examples valid
- **Validate Narrative:** PASS — 13 narrative commands resolved + full examples
- **Workflow Verify:** PASS
- **Verify Skill:** PASS — 0 errors
- **Live probes:** 6/8 pass with API key; 2 local-store probes (search/sync) need synced DB (documented)

## Command Outputs
- shipcheck: PASS (7/7 legs) with `--api-key` + `--env-var V0_API_KEY` (verify runs live → 100%)
- verify pass rate: 100% (live mode)
- scorecard total: 93/100 Grade A
- novel_features_check: planned 8, found 8

## Top Blockers Found
1. research.json shape (alternatives/troubleshoots must be objects) — fixed
2. Novel command scaffolds were clobbered by regen until research.json declared novel_features; re-implemented generator-named scaffolds
3. `--tree`/`--url` flags lacked dry-run guards (validate-narrative failure) — fixed
4. Fixture chat IDs were v1-era (422 on v2) — replaced with live v2 chat `e9wHlJbq0m8`
5. verify-skill positional-args mismatch (chats create recipe used positional message) — switched to `--message`

## Fixes Applied
- research.json: object-shaped alternatives/troubleshoots, clean command paths, v2 fixture IDs
- dry-run guards in chats_files --tree / chats_preview --url
- `.printing-press-patches/` records for 3 generated-file patches (model_usage table, --tree, --url)
- spec: `type: array` list responses for syncable detection; cli_description aligned with headline

## Live Probe Notes
- Probe runs in sandboxed HOME without API key → auth-required commands 401 in probe but pass under real key (verified manually)
- Offline search / sync need the local mirror; documented in README Quick Start

## Final Recommendation
**ship** — all ship-threshold conditions met; no functional bugs in shipping-scope features.
