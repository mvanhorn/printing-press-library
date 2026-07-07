# Acceptance Report: google-ad-manager

Level: Quick (binary-owned live matrix) + manual live validation against the test network (read-only token).
Tests: 7/7 passed (runner) + 6 novel commands validated against real data.

## Live validation (real GAM360 network, read-only OAuth token)
- doctor: auth present, API reachable — PASS
- GET /v1/networks: token valid, network resolved — PASS
- adunits tree: built the ad-unit hierarchy from live data — PASS
- inventory orphans: 58 ad units + 5 placements scanned → 23 orphans — PASS
- since (--since 30d): 190 entities scanned, 1 changed — PASS
- order graph <order>: order + 9 line items — PASS
- lineitem pace <order>: schedule pace over 9 line items — PASS
- generated reads (orders): PASS
- report run/rerun/watch: 403 insufficient scopes on the read-only token (report creation is a write) — EXPECTED/correct error handling; full create→run→fetch needs an `admanager` (non-readonly) token.

## Scope change made during live validation (user-approved)
- DROPPED `targeting where` + `targeting unused`: GAM REST v1 line items expose NO targeting field (confirmed on list AND detail) — the line-item↔targeting linkage is SOAP-only, so these could not function. Removed rather than ship misleading output.
- SIMPLIFIED `order graph`: dropped the targeting→ad-units expansion (always empty in REST v1); now returns order → line items (goal, type, flight dates).
- Net: 8 fully-working novel features.

## Printing Press issues (for retro)
- Generated `sync` cannot fill the reserved-expansion `{parent}` (=networks/{code}) path param for Google-style specs, so the local mirror never populates from sync. Worked around by making the store-backed novel commands fetch live via --network and auto-cache.
- The converted spec's generic `{+parent}`/`{+name}` reserved-expansion templates break the generator's parent-child sync inference; a global path-context / network-code config would fix it.

Gate: PASS
