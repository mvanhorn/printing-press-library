# Acceptance Report: umami

  Level: Full Dogfood
  Tests: 403/403 passed (231 skipped: write-side without fixture plan, framework-internal)
  Gate: **PASS**

## Run history
- Run 1 (pre server upgrade): 373/405 — 32 failures. Triage: 28 = endpoints absent from the operator's older Umami v3 build (boards, dashboard, replays, revenue GET, events stats, event-data pivot, session-data stats, shares); 2 = reports list (server requires websiteId); 2 = CLI-side (stale watch example with --quiet; json probe on binary endpoint).
- Fixes applied (loop 1): watch example fixed; version-gate hint decorator (HTML-404 → "endpoint not available on your Umami server version"; missing-websiteId 400 → names --website-id); reports list happy fixture; event-data pivot fixture (--event-name); event-data events fixture (--event, works around a server 500 when the param is absent — upstream bug, reproduced with raw curl); raw binary `websites export` endpoint removed from the spec (friendly top-level `export` remains the implementation of manifest row 27).
- Operator upgraded the Umami server to the latest v3 (verified: previously-404 routes now 200; new token format).
- Run 2 (post upgrade): 398/405 — remaining 7 were the fixture/server-bug items above.
- Run 3 (final): **403/403 PASS**, exit 0.

## Live behavioral checks (real instance, redacted)
- portfolio: all sites rolled up with growth deltas, 0 fetch failures
- seo: correct organic/Google shares on the test site; entry pages real
- watch: real deviations vs 28d weekday baselines; domains resolve
- coverage: found one genuinely silent duplicate tracker entry
- new-referrers: real first-seen referrer domains after snapshot history
- export: downloads a valid ZIP archive
- reports run-breakdown/run-utm: real data (filters auto-injected)

## Fixes applied total: 6 (see shipcheck + phase-4.95 logs for the earlier loops)

## Printing Press issues for retro: 5
1. Object body param needing an always-sent default ({}) not expressible in spec → hand decorator required.
2. `generate --force` merge loses hand AddCommand wiring in root.go on every regen (re-applied idempotently 4×).
3. Generated `auth set-token` command is never registered by generated auth.go.
4. Regen merge dropped generated `internal/mcp/intents.go` while tools.go kept calling RegisterIntents (build break; restored by hand as no-op).
5. Dogfood json_fidelity probe runs against `response_format: binary` endpoints where --json correctly refuses.

## Upstream Umami issue observed (not CLI)
GET /api/websites/{id}/event-data/events returns 500 unless the `event` query param is provided (reproduced with raw curl on latest v3).
