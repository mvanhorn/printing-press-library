# Shipcheck — wpengine-pp-cli

## Final result (run 2)
- Verdict: **PASS (7/7 legs)** — verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard
- Scorecard: **97/100 Grade A** (omitted from denominator: mcp_description_quality, mcp_token_efficiency, live_api_verification)
- Verify pass rate: 100% (leg PASS, exit 0)
- Sample output probe: 6/7 — the one miss is advisory: `whois clientsite.com` example uses a placeholder domain that (correctly) resolves to `[]` + stderr hint against the operator's real fleet.

## Before/after
- Run 1: FAIL (2/7 legs) — validate-narrative + verify-skill both flagged a recipe invoking `wpengine-pp-cli sql`, a command this generation does not emit.
- Fix: replaced the recipe in research.json with `analytics --type installs --group-by php_version`; regenerated with `--force`.
- Regen side-effect: regen-merge preserved run-1 `internal/mcp/tools.go` (calls `RegisterIntents`) while the fresh tree no longer emits `intents.go` (recipe-derived intents changed) → build break. Repaired by taking the fresh tree's `tools.go`/`tools_test.go`. **Retro candidate (machine bug):** regen-merge can preserve a templated file whose conditionally-emitted sibling disappeared from the fresh tree, producing an unbuildable merge.
- Run 2: PASS 7/7.

## Known quality notes (accepted)
- Cache Freshness 5/10 — deliberate: cache.enabled left off (undocumented server rate limits; manual `sync` + generated staleness hints instead of pre-read refresh).
- MCP Quality 8/10 — Cloudflare pattern applied (68 endpoint tools > 50 → code orchestration, hidden endpoint mirrors, stdio+http).
- wp_version arrives null in the API's install list responses; `audit versions` reports it honestly as "unknown".

## Behavioral verification (real fleet, read-only)
- `audit certs --expiring 90d`: found genuinely expired certs (up to ~1,400 days).
- `audit backups --stale 7d --env production`: 0 rows (platform daily backups) — verified against synced backups table.
- `audit versions`: PHP {7.4:7, 8.2:11, 8.4:3}; `--php-below 8.2` → 7 installs; `--drift` → 4 sites with real prod/staging PHP drift (site names resolved).
- `audit domains`: 114 scanned; platform hostnames excluded from missing_cert by default (94→52).
- `whois <real-domain>`: resolved install/site/account/cert status + expiry; negative test exit 3.
- `audit usage --horizon 30d`: real finding — projected bandwidth 1,461.98 GB vs 1,000 GB limit → status "over". String-encoded numerics handled via cliutil.ExtractNumber.
- `guard`: --dry-run and PRINTING_PRESS_VERIFY short-circuits verified; bad --purge → exit 2. Live run deferred to Phase 5 (real mutation; needs explicit consent).

## Ship recommendation
**ship** — pending Phases 4.8–4.95 reviews and Phase 5 live dogfood.
