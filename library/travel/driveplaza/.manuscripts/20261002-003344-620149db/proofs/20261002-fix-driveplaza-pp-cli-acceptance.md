# Acceptance report — Drive Plaza

Level: Full dogfood, preauthorized by the batch brief. Public no-auth read-only source; no purchases or external mutation.

Gate: PASS. Binary-owned runner exercised 113 mandatory matrix tests, 0 failures, 91 explicit skips/unverified checks over 38 commands. The full result records all 204 cases; skips are not passes. Every planning leaf and all three low-level references passed help, real happy-path and JSON fidelity. All nine domain leaves also passed dry-run JSON. Runner binary: `<CLI_DIR>/build/stage/bin/driveplaza-pp-cli`.

Independent focused live suite: 17 checks/20 upstream requests; eight additional variant checks; four MCP checks. Covered all five vehicle classes, departure/arrival, priorities, ETC column choice, both source road exclusions, five waypoints/invalid six, directional availability, bilingual/detail identities, service filters, scan/output bounds, source dates, null unknowns, real empty CSV/plain, whole-call timeout, field selection and official handoffs. All pass. Meaningful parser/HTTP/CLI/MCP regressions also pass.

Runner skip reasons:
- 2: non-id positional "interface" at depth 0.
- 5: command does not honour --dry-run.
- 27: no positional argument.
- 7: mutating command dry-run only.
- 14: mutating command requires --allow-destructive.
- 3: non-id positional "text" at depth 0.
- 7: mutating command; error_path would call live API without --dry-run.
- 6: blocked-fixture: required API parameter.
- 7: non-id positional "query" at depth 0.
- 11: non-id positional "name" at depth 0.
- 2: no --dry-run short-circuit.

The six blocked-fixture cases concern generated local `learnings confirm/reject`, not a provider or domain endpoint; temporary-store confirmation/rejection lifecycle is covered by `internal/cli/learnings_candidates_test.go`. Do not infer current provider coverage from those local framework tests. Printing Press fixture planning should recognize local candidates and provide isolated IDs. Other local writes and delivery mutations were not authorized as live external actions, and were skipped by the runner's default policy.

Three reference dry-run skips are static-detection false positives: direct supplemental invocations succeed with exit0, valid dry-run JSON, and no provider request; see `evidence/reference-dryrun.json`. Missing-positional error tests are inapplicable to flag-only commands; domain bad dates, times, facility IDs, overlong names and waypoint counts have real exit2/no-I/O regression coverage and independent negative checks. No flagship feature is blocked.

Fixes applied before acceptance: source exclusion serialization/echo guards; source heading and directional identity contracts; vehicle/date/time-kind quote echo requirements; invalid-input exits; empty tabular output; parsed/bounded reference tools; corrected MCP context and documentation. Ten independent review findings cleared in two rounds by the one required MAX reviewer. No scope dropped.

Evidence: `20261002-dogfood-results.json`, runner-written `phase5-acceptance.json` (source fingerprint attached), `independent-MAX-review.md`, `phase-4.85-findings.md`, and working-tree `evidence/live/`, `evidence/mcp-acceptance.json`. Domain data limitations remain in README: conditional quotes, dated non-East facilities, weekday-only hours, and advisory/current-status unknowns.

After all polish Go edits, runner-owned final acceptance remained PASS (113 passed/0 failed/91 explicit skipped-unverified), in `20261002-dogfood-final.json` and refreshed `phase5-acceptance.json`. All7 post-review shipcheck legs PASS; final verify29/29, scorecard78/B with five actual passing live samples, zero pending tools/PII findings. The same sole reviewer cleared metadata/doc round3.
