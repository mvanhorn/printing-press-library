# Tech Twitter CLI — Shipcheck Proof

## Shipcheck umbrella: PASS (6/6 legs)

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | runtime + auto-fix |
| validate-narrative | PASS | every README/SKILL example resolves under PRINTING_PRESS_VERIFY=1 |
| dogfood | PASS | wiring, dead-flag, novel-feature checks; novel_features_check planned=7 found=7 |
| workflow-verify | PASS | primary workflow |
| verify-skill | PASS | flag-names, flag-commands, positional-args, canonical-sections |
| scorecard | PASS | 91/100 Grade A |

## Scorecard: 91/100 — Grade A
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native,
  MCP Quality, MCP Remote Transport, Local Cache, Workflows, Path Validity,
  Data Pipeline Integrity, Sync Correctness
- 9/10: Breadth, Vision, Agent Workflow
- 7/10: MCP Desc Quality, MCP Token Efficiency, Insight
- 5/10: Cache Freshness (intentionally not enabled — curated stream is sync-based;
  no pre-read upstream refresh path that adds value)
- Type Fidelity 2/5 (spec `types` define a high-value field subset; full JSON is
  preserved in the store regardless), Dead Code 5/5

## Live dogfood: PASS (full) — 101/101
- Auth: none (public read-only API). matrix_size=101, tests_passed=101, tests_failed=0.

## Bugs found and fixed before ship (fix-now, not deferred)
1. **`tweets get <non-uuid>`** returned exit 0 with ~100KB of homepage HTML (non-UUID id
   fails the site allowlist → 307→/→200 HTML). **Fixed:** UUID validation rejects non-UUID
   ids with a usage error (exit 2) before any request.
2. **`time-travel <invalid-date>`** returned exit 0 empty. **Fixed:** date validation
   (today/yesterday/latest/YYYY-MM-DD) → usage error (exit 2) on bad input.
3. **`tweets monthly`** happy-path failed under dogfood because the synthesized month was
   42 (API correctly 400s "Invalid year or month"). **Fixed:** realistic example
   (`--year 2026 --month 6`) + `pp:happy-args` annotation so dogfood uses valid inputs.

## Added on user request (mid-build, in scope)
- Removed `products` surface (no resource/store; live `agent --kind launches` still passes
  through server-side).
- Added novel `time-travel` command (homepage Time Travel panel port; live + offline).
- Added top-level `trending` command (live-first, offline-ranked from the local mirror) —
  the discoverable front door to `tweets trending`; matches the homepage headline surface.

## Known non-issues
- Sample Output Probe 5/7: `since 24h` and `evidence read-list --select ...` "fail" a
  token-echo heuristic (the window arg never appears in tweet data; `--select` deliberately
  strips the `kind` field). Both produce correct output, verified manually. Not blocking;
  no leg failed.

## Final ship recommendation: **ship**
All ship-threshold conditions met; no known functional bugs in shipping-scope features.
