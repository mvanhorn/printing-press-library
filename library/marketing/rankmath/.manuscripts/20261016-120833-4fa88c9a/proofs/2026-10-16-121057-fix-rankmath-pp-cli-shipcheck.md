# rankmath-pp-cli shipcheck

## Run 1 (exit 3)
- verify PASS, validate-narrative PASS, workflow-verify PASS (no manifest), verify-skill PASS, scorecard 94/100 A
- dogfood FAIL: (1) portfolio read RANKMATH_USERNAME/RANKMATH_APP_PASSWORD via os.Getenv, bypassing saved auth; (2) dead helper printStampedDryRunOutput.

## Fixes
- portfolio: default credentials via novelAuthHeader(flags); per-site RANKMATH_USERNAME_<SLUG>/RANKMATH_APP_PASSWORD_<SLUG> override kept (Codex).
- helpers.go: removed unused generated printStampedDryRunOutput (recorded in .printing-press-patches/hand-patches.txt).

## Run 2 (exit 0)
- 7/7 legs PASS; dogfood PASS (0 dead flags/functions, 5/5 novel features); scorecard 95/100 Grade A (MCP Quality 8, Insight 7, Data Pipeline 7, Dead Code 4/5).
- verify pass rate: 3/4 live-check samples (1 skipped) both runs.

## Behavioral samples (live, public, read-only, example.com)
- audit --max-fetch 40: 92 inventory items, 40 fetched, 0 fetch failures; 40 missing_description, 40 missing_schema, 19 long_title, 1 missing_canonical (confirmed against raw HTML: pages ship no meta description / JSON-LD).
- reindex --since 30d: 2 candidates, preview only (log_unavailable without auth).
- head get https://example.com/: parsed via HTML fallback (headless getHead off).
- doctor modules: both sites expose Rank Math; site A 118 routes, site B 108 (no ai-visibility).
- coverage / opportunities / portfolio: require credentials (401 rest_forbidden live); verified by httptest integration tests only.

## Verdict
ship — all threshold conditions met; authenticated novel commands carry mock-only verification (documented risk, not a known bug).
