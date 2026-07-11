# Shopify CLI Reprint — Shipcheck Report

## Verdict: ship

## Summary
- Reprinted shopify-pp-cli under Printing Press v4.28.0 (was v4.14.0).
- 4 new hand-coded transcendence features shipped: orders brief, bulk-operations wait, doctor throttle, store diff.
- Regen-merge Path B used to reconcile 35 previously-shipped hand-authored commands (report, shopifyql, growth, ops, merchandising, bulk-operations current/run-query, orders/customers tag) against the new generator baseline — required extensive manual reconciliation of framework-owned files (root.go, helpers.go, data_source.go, client package, cliutil, store) that had drifted between generator versions and were not cleanly auto-mergeable.
- Fixed a generator-level dry-run bug (auto-refresh + URL template-var resolution + GraphQL response envelope all ignored/mishandled --dry-run for templated per-tenant base URLs) — patched in internal/client and internal/cli, flagged for retro.
- Scorecard: 89% (import baseline) -> 96/100 Grade A (final).
- Final shipcheck: 7/7 legs PASS (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard).

## Known gaps (disclosed, not blocking)
- README.md/SKILL.md document only the 10 endpoint-mirror commands + 4 new novel features; the 35 preserved hand-authored commands (report/shopifyql/growth/ops/merchandising/tag mutations) are functional but undocumented in this pass.
- The shipped `auth audit` command models OAuth client_credentials semantics; Shopify's actual Admin API auth for this CLI is a static X-Shopify-Access-Token header. Likely copied from a different API template. Not touched this session (pre-existing, flagged for follow-up).
- Live command sampling (orders brief / bulk-operations wait / doctor throttle) not exercised — no SHOPIFY_ACCESS_TOKEN/SHOPIFY_SHOP available in this environment.
- gh CLI not authenticated in this environment — publish flow will need `gh auth login` first.
