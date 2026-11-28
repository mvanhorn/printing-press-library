# Acceptance Report: meta-ads-library

- **Run ID:** `20261127-082434-b7f7d73b`
- **Level:** Full Dogfood (`--level full --auth-tier free`)
- **Tests:** 141/141 passed (0 failed, 0 hollow features, 98 skipped on optional/tier-gated/dry-run-only paths)
- **Auth Context:** Zero-auth public GraphQL session bootstrap (`ads`, `advertisers`, `media`, `sync`, and all 7 local intelligence commands verified live against Meta Ad Library; official Graph API `archive` annotated `pp:requires-tier: authenticated` and skipped cleanly on free tier)
- **Failures:** None
- **Fixes applied (CLI fix):**
  - Added `pp:happy-args` (`--limit=5`) to `winners`, `hooks`, and `funnels` so all 7 novel transcendence features are exercised live with zero hollow coverage.
  - Added structured exit-code-3 (`notFoundErr`) + JSON error envelopes when explicit advertiser/keyword filters yield zero local records on `winners`, `hooks`, `funnels`, `diff`, `transparency`, and `sov`.
  - Added early exit-code-4 (`authErr`) credential guard on `archive` when `META_ACCESS_TOKEN` is unset (replacing upstream Graph API HTTP 500 `OAuthException code 1`) and annotated `archive` with `pp:requires-tier: authenticated`.
- **Printing Press issues:**
  - `tier_routing` with `default_tier: free` (`auth: none`) + credential tier emits undefined placeholder-credential helpers in generated `internal/client/client.go` (retro candidate).
  - Generator hardcodes Go `1.26.10` in `go.mod` and `SKILL.md` canonical section while `GO-2026-6859` requires `1.26.11` (retro candidate).
- **Gate:** **PASS**

## Evidence

- Acceptance marker: `phase5-acceptance.json` (`status: "pass"`, `tests_passed: 141`, `tests_failed: 0`, `coverage_hollow: false`)
- Full matrix log: `2026-11-27-082434-dogfood-results.json`
