# SEEK CLI — Phase 5.5 Polish

Run: 20260908-143250-21abe609 · mid-pipeline (STANDALONE_MODE=false) · ship_recommendation: **ship**

## Delta

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 82/100 | 82/100 | +0 |
| Verify | 100% | 100% | +0 |
| Dogfood | PASS | PASS | (generator-owned dead code only) |
| Live matrix | exercised | exercised | 5 pass / 1 env-fail (me new-jobs needs full SEEK session) |
| Go vet | 0 | 0 | +0 |
| Gosec (hand-authored novel code) | 0 | 0 | +0 |
| Tools-audit | 2 pending | 0 pending | -2 (both accepted, DO-NOT-EDIT generated files) |
| PII-audit | 0 | 0 | +0 |

## Fixes applied

- Accepted 2 tools-audit thin-short findings in the polish ledger with per-item
  DO-NOT-EDIT rationale (`list` in generated `internal/cli/platform_client.go`,
  `learnings list` in generated `internal/cli/teach.go`). Both Shorts are
  verb-led and precise; a durable fix is a generator template change.
- gofmt + clean rebuild verified; `go test ./internal/...` green.

## Skipped (all generator-owned or structural; retro candidates)

- Dead `--max-age` flag + dead helpers `handleBinaryResponseDelivery`,
  `hasChangedLocalFlags` in generated `root.go`/`helpers.go` — template emits a
  staleness-hint flag with no consumer because this CLI has no framework `sync`
  (search-only API). Likely also the cause of `cache_freshness 3/10`.
- dogfood "config inconsistency" — heuristic false positive on the dual
  cookie+token auth model in generated `config.go`; live auth verified working.
- dogfood Data Pipeline PARTIAL ("sync uses generic Upsert only") — generated
  syncer; Sync Correctness 10/10.
- scorecard `auth_protocol 2/10` — cookie auth has no bearer/basic/oauth scheme;
  the dimension is calibrated for standard schemes.
- scorecard `mcp_token_efficiency 7/10` — endpoint_tools=hidden guidance targets
  ~70-endpoint APIs; this spec has 1 typed endpoint + novel commands.
- gosec: 54 raw findings, **all in generator-owned files** (`cli/auth.go`,
  `store/store.go`, `client/*`, `platform/*`, `cmd/seek-pp-mcp`). **Zero** in
  hand-authored novel code (`salary.go`, `trends.go`, `company.go`,
  `classifications.go`, `listings_facets.go`, `me_new_jobs.go`, `listings_get.go`,
  `listings_count.go`, `seek_novel.go`, `internal/seekparse/`). Notable: G202 on
  `VACUUM INTO '<path>'` in generated `internal/platform/migration.go` — SQLite
  can't parameterise VACUUM INTO; code already single-quote-escapes.
- Phase 4.85 output-review re-run: status PASS, findings [].

## Notes

- The build lock for `seek-pp-cli` was stale (phase `shipcheck-fixing`, ~90 min,
  PID 46679); polish proceeded per the stale-lock rule.

## ship_recommendation: ship (further_polish_recommended: no)
