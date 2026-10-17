# Shipcheck — investorpaisa-pp-cli

Command: `cli-printing-press shipcheck --dir <work> --spec <sniff spec> --research-dir <run> --no-live-check`
(live-check deferred to Phase 5 live dogfood because the data API is login-gated.)

## Run 1 (FAIL 2/7)
- validate-narrative: quickstart `screen ... --sort rs` failed because the staged binary was stale (built at generation, before the hand-written commands). Fixed by rebuilding build/stage/bin.
- dogfood: (a) `movers` flagged as hand-rolled (reached the API through ipLoader) -> added `// pp:client-call`; (b) config-consistency heuristic read garbage fields off the generated `auth set-token` (`"", "", token, ""`). The generated set-token also demanded a JWT, which is wrong for an Auth.js UUID session cookie, so set-token was reworked to accept the session cookie value or a cookie string (normalizeSessionCookie, unit tested).

## Run 2 (PASS 7/7)
| Leg | Result |
|---|---|
| verify | PASS 100% (49/49, 0 critical; 4 read commands exec-score 2/3 in mock mode because the mock server does not return the site's response shapes) |
| validate-narrative | PASS (8 ok, 0 failed) |
| dogfood | PASS (novel features 8/8; wiring consistent) |
| workflow-verify | workflow-pass |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | 87/100 Grade A (auth_protocol 2/10 — cookie auth; cache freshness 5/10; MCP: 16 tools, 1 public/15 auth-required) |

## Behavioral checks
All 8 novel commands exercised against a loopback fake of the site (ip_e2e_test.go): screen (+ --saved, bad field/expr errors), movers (RS join, min-rs), quote (+missing symbols, news, watchlists), brief, history (+local cache replay), changes/breadth/sectors --trend across two stored days, sync -> snapshots -> changes, 401 surfaces as an auth error, local mode makes zero network calls. Real-site sampling happens in live dogfood.

## Before/after
- verify pass rate 100% both runs; scorecard 87 (run 1) -> 87 (run 2); legs 5/7 -> 7/7.

## Recommendation: ship
