<!-- slop-gate: off -->
# postmark-pp-cli shipcheck

## Umbrella (loop 1)
`cli-printing-press shipcheck --dir working/postmark-pp-cli --spec research/specs/postmark-merged.openapi.yaml --research-dir <run>` with POSTMARK_ACCOUNT_TOKEN + POSTMARK_SERVER_TOKEN (Helm) from a password manager in env.

| Leg | Result |
|---|---|
| verify | PASS, 103/103, 0 critical (mock mode) |
| validate-narrative | PASS |
| dogfood | PASS (verdict WARN: 1 dead generated helper `handleBinaryResponseDelivery`) |
| workflow-verify | PASS (no workflow manifest; skipped) |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS, 95/100 Grade A; sample output probe 3/3 |

Verdict: PASS (7/7), exit 0.

## Notes
- verify EXEC cells for `diagnose`, `overview`, `pulse` scored 2/3: the generic runtime mock returns a bare JSON array for any path ending in "s" (`/servers`, `/bounces`, `/stats/outbound/sends`); Postmark always returns envelope objects. Against a `{}` mock all three exit 0, and live runs succeed. Mock artifact, not a CLI bug.
- Scorecard gaps: Cache Freshness 5/10 (x-cache intentionally disabled: rate-limited API with unpublished limits; manual sync + doctor), Dead Code 4/5 (generated helper), Path Validity 9/10.

## Behavioral sample of every transcendence command (live, read-only)
- pulse: 6 servers, window vs baseline per server (e.g. 68 vs 246 over 28d), 0 flagged.
- templates check: 1 passed, 0 failed on the server's templates.
- recipient-domains: after syncing one server into a scratch DB, 20 domains totalling 380 sends = 380 synced recipients; one domain flagged high-bounce-rate (6/6 bounced).
- email send-once: plan mode with --sandbox prints payload and dedupe decision (ledger miss; Postmark search skipped for the test token), sent:false.
- servers bootstrap: plan for a new name lists server/broadcast-stream/domain as create, applied:false, no writes.

## Fixes applied before this loop
Header routing, --server resolution, token masking (response + cache + store), first-page offset, response-array fix, sync shaping, examples and flag descriptions, explicit health registration, documented-hosts file, de-identification. See build log.

## Ship recommendation: ship
All ship-threshold conditions hold: shipcheck exit 0, verify PASS, dogfood wiring clean, workflow-verify pass, verify-skill exit 0, scorecard 95, and every approved transcendence command returns correct live output.
