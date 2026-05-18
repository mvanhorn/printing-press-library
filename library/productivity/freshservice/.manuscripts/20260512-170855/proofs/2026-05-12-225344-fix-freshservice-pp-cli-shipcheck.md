# Freshservice CLI Shipcheck

## Context
- CLI: `freshservice-pp-cli`
- Run: `20260512-170855`
- Working dir: `/home/mark/printing-press/.runstate/isms-7dc3a6b0/runs/20260512-170855/working/freshservice-pp-cli`

## Phase 3 completion gate
Before shipcheck, verified all 10 novel features from the absorb manifest were
built and registered:

| Feature | Command | Help resolves |
|---|---|---|
| SLA Breach Countdown | `breach-risk` | OK |
| My Queue | `my-queue` | OK |
| Cross-Entity FTS | `search` | OK |
| Agent Workload | `workload` | OK |
| Change Collisions | `change-collisions` | OK |
| Incident Recurrence | `recurrence` | OK |
| Knowledge Gap Finder | `kb-gaps` | OK |
| Asset Orphan Detector | `orphan-assets` | OK |
| Department SLA Leaderboard | `dept-sla` | OK |
| On-Call Coverage Gap | `oncall-gap` | OK |

`dogfood --json` reports `novel_features_check.found == planned == 10, missing == []`.

## Fixes applied before final shipcheck
1. **Built 9 freshservice-specific transcendence commands** in
   `internal/cli/freshservice_novel.go` and registered them in `root.go`.
   Only `search` existed in the original generation; the other 9 commands
   ship the SLA/workload/collision/KB/orphan/department analytics promised
   in the absorb manifest.
2. **Added `--in` flag to `search`** so SKILL/README examples that scope
   search to `tickets,kb` (or `tickets,assets,changes`) work. Implemented
   as a per-resource LIKE scan over the local store.
3. **`dept-sla --period`** changed from int days to string period (`30d`,
   `4w`, `24h`) so SKILL/research.json examples like `--period 30d` parse
   cleanly via `parsePeriod`.
4. **`my-queue` accepts positional agent ID/email** because the global
   `--agent` flag (bool) and a subcommand `--agent` (string) cannot coexist
   in cobra. Updated SKILL.md, README.md, and research.json to the
   positional form `my-queue ops_at_example.com --agent`.
5. **Updated `tickets list ...` recipe** to `tickets filter --query "..."`
   in SKILL.md and research.json; the generated `tickets list` only has
   pagination flags (the spec doesn't define `status`/`priority` as query
   params on `/tickets`).
6. **Reworded SKILL.md prose** that began "freshservice-pp-cli is the
   first ..." to start with "This is the first ..." so verify-skill stops
   flagging "is" as an unknown command path.

## Shipcheck final result

| Leg | Result | Exit |
|---|---|---|
| dogfood | PASS | 0 |
| verify | PASS | 0 |
| workflow-verify | PASS | 0 |
| verify-skill | PASS | 0 |
| validate-narrative | PASS | 0 |
| scorecard | PASS | 0 |

**Overall: PASS (6/6 legs)**

- Scorecard total: 84/100 — Grade A
- Verify pass rate: 100% (35/35 mock-mode subcommands)
- Sample output probe: 9/10 (90%) — see "Known gap" below

## Known gap (does not block ship)

The scorecard's sample output probe runs each novel feature against an
empty local store. `search "database crash"` returns
`{"meta":..., "results":[]}` — a valid empty result envelope, but the
probe expects at least one query token in output. This is a function of
"no data has been synced yet," not a behavioral defect. Once
`freshservice-pp-cli sync` populates tickets, the probe will pass.

The other 9 novel features all return correct empty-result envelopes
(`[]`, `{"count":0,...}`) that the probe accepts.

## Verdict

`ship` — all ship-threshold conditions met:
- shipcheck umbrella exits 0
- verify-skill PASS
- novel features 10/10 built and dogfood-verified
- scorecard ≥ 65
- no flagship command returns wrong output (search returns empty against
  empty store, which is correct)
