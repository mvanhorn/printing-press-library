# Acceptance report: Ikyu

Level: Full Dogfood, authorized by the original live-E2E brief and approved seven-command scope; no additional depth prompt needed. Read-only defaults used; no destructive opt-in or account state.

Gate: PASS. Binary-owned marker phase5-acceptance.json records 62/62 executed checks passed, zero failures, and 38 skipped/unverified probes. All seven accommodation commands passed live happy-path, JSON fidelity and zero-IO dry-run checks.

The skip set covers framework mutation/fixture cases and two probes of the generated raw properties HTML helper that received HTTP403. That helper is outside the supported seven-command stay workflow; its blocked access is disclosed rather than claimed verified. Separate independent live assertions cover both hotel and ryokan offers, including visible earn-mode headline/points, all ten price fields, source names/meals/cancellation, nightly dates, occupancy, pagination, negative filter coverage and field selection.

Fixes in this phase: parent stay examples and standard top-level dry-run action metadata. Fresh source-bound full matrix then passed. No acceptance marker was hand-authored or modified.

Efficiency: every command was measured cold and warm, including output bytes, actual requests, decoded response bytes, wall time and per-process OS peak RSS. All seven warm repeats made zero requests. See efficiency-metrics.json.
