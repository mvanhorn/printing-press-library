Manifest transcendence rows: 10 planned, 10 built. Phase 3 will not pass until all 10 ship.

## Phase 4 rework (network-code/sync gap)
- Generated `sync` cannot fill the `{parent}`=networks/{code} path param for this API shape (reserved-expansion spec), so the mirror never populated and 5 offline novel commands returned empty.
- Fix (user-approved): the 5 store-backed commands (adunits tree, inventory orphans, targeting where, targeting unused, since) now fetch live via --network through a shared `gamLoadResource` helper (mirror-first, live fallback, auto-cache). `since` force-refreshes.
- Removed dead helpers gamMissingMirror/gamListResources/gamStoreResourceTypes.
- Fixed narrative examples: report rerun (drop unsupported --date-range), report watch (--metric is 0-based column index), quickstart (dropped non-working sync; uses live adunits tree + inventory orphans).
- RETRO: generator sync can't supply a global path-context (network code) for Google reserved-expansion `{parent}` paths.

## Final state
- Novel features: 8 (report run/rerun/watch, lineitem pace, order graph, adunits tree, inventory orphans, since).
- Dropped: targeting where + targeting unused (REST v1 exposes no line-item targeting — SOAP-only). order graph simplified to order→line-items.
- Shipcheck: 6/6 legs PASS, scorecard 91/100 Grade A.
- Phase 5 live: validated 6 novel commands against a real GAM360 network; report commands correctly 403 on a read-only token.

## Publish-time live validation (write-scope token)
- Validated ALL 3 report commands end-to-end against the live GAM API (report run created+ran+fetched real rows; rerun row_count=1; watch baseline).
- BUG FOUND + FIXED: report run/rerun/watch were bounded by the root --timeout (60s), which cut off the async poll before reports finished (~3-4 min). Rebound to --report-timeout (default 120s->300s), decoupled from --timeout. Recorded as patch google-ad-manager-report-timeout-decouple.
- inventory orphans: added Example/Long (patch google-ad-manager-inventory-orphans-example).
- All 8 novel features now live-validated.
