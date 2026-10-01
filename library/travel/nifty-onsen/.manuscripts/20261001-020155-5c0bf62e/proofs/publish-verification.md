# Publication verification

Current source publication validation passes: manifest, source-bound Phase 5, tidy, reachable govulncheck, vet, build, help, version, verify-skill, patches and manuscripts. The standalone module-path warning is resolved by canonical publish packaging.

Full live dogfood was freshly rerun at publication time: 54 passes, 41 safe skips, zero failures and no hollow feature coverage. Each approved provider workflow ran against the real public source. Acceptance is in `phase5-acceptance.json`; the compact result is `publish-live-verification.json`. Raw API-response dumps remain private and are omitted from this package.

Publisher repairs preserve the creator name and fill the originally missing handle through the current generator's missing-handle backfill. MCP metadata now describes seven actual provider workflows plus the separate context tool. HTTP MCP has a 10-second header deadline and 60-second idle deadline; streaming responses have no whole-response timeout. Matching reprint guards are included.

Pinned gosec v2.26.1 was freshly rerun: 21 inherited scaffold findings remain, zero authored provider findings, and the prior HTTP header-deadline finding is resolved. `publish-security.json` carries the exact current findings. The builder's earlier 22-finding scan and independent review are historical evidence before the publisher timeout change, not a clean scan of the final source.

Limits: unofficial public website integration; finite nearby candidate window; unknown admission/coupon eligibility or bookable inventory unless explicit source evidence. No coupon redemption, bookings, purchases or account actions.

Exact canonical package checks: `publish validate` passes every check including canonical module path; `go test -count=1 ./...` passes; the library SKILL verifier reports zero errors and four recognized dynamic-command false positives. Mandatory package secret scan and additional vendor/email/bearer/structural scans are clear; static public attribution and test-query names are documented false positives. No executable payload remains.

## Review corrections

Greptile round one: invalid all-miss field selections now return usage errors with empty stdout in agent/JSON/compact/CSV/plain/quiet output; focused regressions first reproduced the original emission and then passed after correction. Valid projection and stale comparison tests remain passing.

Release metadata uses the canonical CLI linker symbol. The personal tap is not provisioned, so the unused deprecated Homebrew publisher is omitted; no live Homebrew installation is claimed. Official GoReleaser configuration check passes without deprecated properties.

After both review fixes, fresh full live acceptance again passes 54 checks with zero failures and no hollow coverage; the full canonical-package test suite passes, and official GoReleaser configuration validation passes. Acceptance and compact proof are refreshed for the current source.
