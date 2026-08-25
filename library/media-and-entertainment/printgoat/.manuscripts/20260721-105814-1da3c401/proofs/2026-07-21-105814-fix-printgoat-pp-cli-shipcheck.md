# printgoat Shipcheck Report

## Verdict: PASS (7/7 legs)

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS (9/9 narrative commands resolved, full examples pass) |
| dogfood | PASS (10/10 novel features found; 0 dead flags/functions) |
| workflow-verify | PASS (no workflow manifest, skipped cleanly) |
| apify-audit | PASS (no Apify references) |
| verify-skill | PASS (0 errors after fix, see below) |
| scorecard | PASS — 90/100, Grade A |

## Fix loop (1 iteration)

**Before:** verify-skill FAILED with 15 positional-args errors — 6 novel commands (`duplicates`, `formats gaps`, `history diff`, `snapshot verify`, `similar`, `designer stats`) consumed a positional arg in `RunE` but their Cobra `Use:` string was left over from the original TODO-stub scaffold (e.g. `Use: "duplicates"` instead of `Use: "duplicates <query>"`), so SKILL.md/README.md examples using that arg were flagged as invalid.

**Fix:** corrected each `Use:` string to declare its actual positional signature (`history diff` uses `[model-key]` since `--all` makes it optional; the rest use required `<...>` placeholders). Also added the 10 missing `// pp:data-source <value>` annotations dogfood's structural check wanted on the novel commands (`computed` for cross-source/join commands, `local` for pure local aggregation in `designer stats`, `live` for `job download`).

**After:** verify-skill 0 errors, dogfood clean, full shipcheck PASS.

## Live behavioral verification (beyond the mechanical legs)

Scorecard's Sample Output Probe reported 5/10 on its live-check pass. Manually verified all 5 "failures" are not real defects:
- `history diff printables:3161` and `formats gaps printables:3161` — both return correct, rich real output (confirmed by direct invocation); the probe's failure was a literal-substring heuristic looking for `"printables:3161"` as one token, which never appears since the JSON correctly separates `source`/`model_id` fields. False negative, not a bug.
- `similar thingiverse:2409854` — the specific Thingiverse ID used in the generated example is a real ID that Thingiverse itself now returns 403 "Thing has not been published" for (external content-state issue, not a code defect).
- `job resume job-20260721-01` and `snapshot verify batch-march-orders` — both require a prior `job download`/`snapshot create` to have created that job/snapshot ID first; the probe invokes each example command standalone rather than in sequence. Expected/inherent to features whose whole point is referencing prior local state.

## Scorecard detail

Total: 90/100 (Grade A). Notable dimensions: Auth 10/10, Doctor 10/10, Agent Native 10/10, MCP Quality 9/10, Type Fidelity 5/5, Dead Code 5/5. Lower dimensions: Cache Freshness 5/10 (expected — printgoat's local store is a download-history/dedup cache, not a full-mirror sync target, per the research brief's Data Layer decision), MCP Tool Design 7/10, Insight 7/10.

## Ship recommendation: ship
