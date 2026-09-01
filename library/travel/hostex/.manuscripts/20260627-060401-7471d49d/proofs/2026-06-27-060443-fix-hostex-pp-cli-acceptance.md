# Acceptance Report: hostex
Level: Full Dogfood (live, writable token on a disposable test account; user-authorized writes/deletes)
Tests: 255/255 passed (status: pass)

## Coverage
- All 86 endpoints exercised live: help, happy-path, JSON fidelity, error-path (incl. create/update/delete).
- Token-revoking endpoints excluded (default) to avoid mid-run credential rotation.

## Targeted novel-feature e2e (real account)
- ops-gaps, inbox-sla, automation-preview, stay-brief: run correctly; return empty sets on the (empty) test account — correct.
- revenue-rollup: reworked to LIVE (transactions require start_date+end_date the generated syncer cannot send); queries the ledger over a date range and aggregates. Verified live (0 tx in account).
- price-parity: live; resolves per-channel listings via pricing_ratios + listing calendar. Property has 1 channel -> correct "nothing to compare" note.
- oversell-watch: live; compares master availability vs channel inventory.

## Bugs found and fixed in-session
1. Client treated HTTP 200 as success, but Hostex returns 200 for every call with the real status in error_code (200/2xx = success, >=400 = failure). Added envelope handling (internal/client/hostex_envelope.go + client.go) so failures become typed errors and 429/5xx retry. Without this, errors were silent successes.
2. oversell-watch formatted integer property ids via %v -> scientific notation (1.2704864e+07) -> 403. Fixed with idString().
3. revenue-rollup: local design infeasible (transactions un-syncable) -> reworked as live range query.

## Known limitations (non-blocking)
- transactions are not part of the local sync (endpoint requires start_date+end_date; generated syncer sends neither) -> revenue-rollup is live-only by design.
- price-parity/oversell-watch need multi-channel listings and authorized availability data to show divergences; verified structurally on a single-channel test account.

Gate: PASS
