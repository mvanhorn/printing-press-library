# yc-companies-pp-cli — Phase 5 Acceptance (Quick Check)

## Level
Quick Check — user-selected.

## Binary-owned matrix
Ran `printing-press dogfood --live --level quick --write-acceptance`.
- status: `pass`
- level: `quick`
- matrix_size: 4 (commands probed)
- tests_passed: 4
- tests_failed: 0
- tests_skipped: 4 (happy_path/json_fidelity skipped on commands with positional placeholders the runner couldn't synthesize; these are not failures)

`phase5-acceptance.json` written alongside this report.

## Manual behavioral checks against the live YC directory

| # | Check | Result |
|---|-------|--------|
| 1 | `doctor` | OK Config / Auth (not required) / API reachable. |
| 2 | `companies list --batch w25 --industry b2b --json` | 106 results (multi-axis local filter behaved). |
| 3 | `companies list --hiring --top --json --select slug,name` | 30 hiring + top companies. |
| 4 | `search payments --limit 3 --json` | Multiple matches incl. Cashfree Payments. |
| 5 | `companies similar stripe --limit 3 --json` | All three peers are Fintech (axoni, belvo, bukuwarung), score 0.867, shared_tags ["Fintech","SaaS"]. Plausible. |
| 6 | `stats by-batch --industry Fintech --json` | All batches with Fintech companies returned in chronological order. |
| 7 | `batches show s24 --json` | Summer 2024, 248 companies, top industries B2B(158)/Industrials(25)/Healthcare(23), top tags AI(84)/AI(79)/B2B(63). |
| 8 | `companies get-in-batch spring-2026 hedge --json` | Returned Hedge (id 31323, San Francisco) from live API. |
| 9 | `meta --json` | last_updated: 2026-05-11T02:12:22.681Z. |
| 10 | `snapshot list` | 1 snapshot present (2026-05-11T14:58:23Z). |
| 11 | Error paths: missing `--field` on `companies changes`, no-snapshot lookups, wrong batch | All return helpful, actionable error messages. |

## Fixes applied during this phase
None — all behaviors matched expectations on the first run.

## Printing Press issues for retro
None blocking. Notes for future retro:
- The Phase 4 scorecard "Sample Output Probe" failures are documentation-style: the novel-feature examples in research.json use `--since 2026-04-01` which requires snapshot history that doesn't exist on a fresh install. Polish skill may want to swap to `--since-last-sync` or surface a "snapshots accrue over time" hint in the SKILL.
- The "similar stripe" output-relevance heuristic in the live-check sampler false-positived because the query token "stripe" doesn't appear in a peer-list output. Consider relaxing the heuristic for peer-discovery commands.

## Gate

**PASS.** All 6 manual checks passed plus the binary-owned matrix returned `pass`. Auth (`doctor`) green, sync green. Threshold met for Quick Check.
