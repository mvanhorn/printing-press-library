# MAX review convergence — round 2

Verdict: **FIXES REQUIRED — two medium issues remain**. The same dedicated reviewer inspected the fixes; no new reviewers were dispatched and source was not edited. Eleven original findings are cleared; R7 is partially fixed but its raw compressed-body boundary remains bypassable. The new reservation-permission/seat-map logic also exposes the February29 fallback issue below.

Evidence: `max-review-convergence/command-summary.json`, current-source independent CLI/MCP binaries in that directory, `independent-regressions.log`, `mcp-context.jsonl`, `source-hashes.json`, and `automatic-gzip.log`. The initial staged binary was stale, so consequential runtime reproductions were rerun with binaries independently compiled from current source; the saved outputs reflect those current binaries.

## Cleared findings

- R1: ordinary advance and processing-gap seat maps are false; unreserved maps are suppressed.
- R2/R3: oversized annual opening/eligibility remain null with bilingual rules and a separate known one-month fallback; five ordinary/four Green maxima, six/five-person incompatibility, overnight null eligibility/party fit, and unreserved rejection reproduce correctly.
- R4: offline and local timetable commands now emit local provenance. Builder's fresh live branch proof records correct live provenance for sources/products and Japanese reference pages.
- R5: independent all-class429 and timetable429 fixtures preserve typed errors. Failure aggregation source uses errors.Join; any quote429 survives later failures; partial successful quotes remain intact.
- R6: independent PDF-drift fixture returns fresh publications and suppresses samples. Offline current-link comparison is null and snapshot URLs are separate.
- R8: independent auto-rate test remains at2.0RPS after repeated successes; source clamps disabled/high/nonfinite overrides and keeps smaller caller limits.
- R9: current MCP context no longer recommends sync/search/cursor pagination, and SQL's scaffold-only limitation is explicit.
- R10: invalid offline timetable date now exits2.
- R11/R12: unsupported exclusivity prose is removed; current handoff renders `adults: 1, children: 0`.

R7's original plain-body reproduction now rejects a2,097,178-byte response. The builder's manual gzip/deflate decoded-limit tests pass, but they do not exercise the inner transport's automatic decompression.

## Remaining R7 — P2: Automatic decompression hides raw wire bytes from the cap

Location: `internal/client/smartex_public.go:37`–`:41`.

An inner Go transport transparently decompresses an automatically negotiated gzip response before returning it to this wrapper. `resp.Body` is then decoded, and Content-Encoding is removed; the wrapper's raw limiter cannot see the original wire bytes.

Independent local TLS reproduction: ten valid gzip members with65,535-byte extra headers create **655,710 wire bytes**, decoding to only90 bytes. With the normal Go transport beneath smartEXPublicTransport, the GET succeeds with90 returned bytes and no error, exceeding the promised512KiB raw limit. See `max-review-convergence/automatic-gzip.log`; the fixture is in the source-identical isolated review module `<local-artifact>`.

Fix: Prevent transparent inner gzip negotiation/decompression so the wrapper sees wire bytes, for example by explicitly supplying a supported Accept-Encoding request header when none was supplied, preserving existing explicit headers. Then test both automatic/default request negotiation and explicitly encoded responses; enforce raw and every decoded-layer cap.

## R13 — P2: February29 unknown annual opening suppresses known one-month sales

Location: `internal/smartex/planning.go:160`, `:198`, `:210`, `:219`.

Reproduce: `window --date 2028-02-29 --now 2028-02-01T12:00:00+09:00 --agent` returns `window_status:unknown`, `reservation_operations_permitted_by_calendar:false`, and `seat_map_available_now:false`, even though its known standard sales field is2028-01-29T10:00JST. Before that fallback, it also asserts false calendar permission where annual opening is documented as unknown.

Evidence: `max-review-convergence/leap_known_sales.json`, `leap_unknown.json`. The one-month fallback remains a known calendar boundary; uncertainty about the earliest annual opening does not negate later normal sales. The same fallback treatment was correctly applied to oversized annual uncertainty.

Fix: When actual earliest opening is unknown, keep permission null before the known one-month fallback; after that boundary, compute ordinary within-window permission and seat-map rules normally while retaining null earliest annual opening. Add before/at/after fallback cases and retain actual close/date-passed conditions.

## Phase 14–17 update

Phase14 skill semantics and Phase15 docs now clear the original findings. Phase16 original fresh live sample run had10/10 eligible PASS; the builder's refreshed current-build end-to-end run is also10/10 PASS (`live-e2e-final/summary.json`). Current handoff format warning is cleared. Phase17 remains open for R7/R13 above; the original independent throttle/drift/plain-cap/rate fixtures now pass.

No account, booking, payment, message or publication action was performed. No new issue was found in the reserved cliutil/cobratree source. Generic transport/context template roots remain retro candidates as recorded by the builder.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

Overall convergence is not yet achieved. Resolve the two issues and send the same reviewer the narrow fixes/verification for round3.
