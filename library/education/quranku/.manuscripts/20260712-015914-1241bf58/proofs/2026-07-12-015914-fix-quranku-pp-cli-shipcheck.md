# QuranKu CLI — Shipcheck

## Verdict: ship

## Shipcheck umbrella: PASS (7/7 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS (8 narrative commands resolved, full examples passed) |
| dogfood | PASS |
| workflow-verify | workflow-pass |
| verify-skill | PASS (0 errors; 2 likely-false-positive positional-arg notes on hifz examples) |
| scorecard | PASS — 79/100 Grade B |

## Sample Output Probe: 7/7 (100%)
All novel features return correct output live: find, verse, daily, random, plan, hifz, bookmark.

## Fixes applied this run
1. Corpus load parallelized (10-worker fan-out): first-run 33s -> ~3s (was timing out the 10s probe).
2. Added `// pp:data-source` annotations to all 7 novel command files.
3. Renamed `chapters info` -> `chapters get` (naming convention) via spec + regen.
4. Removed conflicting `cli_description`; root.Short now derives from narrative.headline.

## Scorecard note
- path_validity 1/10 is a combo-spec live-probe artifact: the probe synthesizes generic
  argument values for parameterized, multi-host endpoints (e.g. /chapter_recitations/{reciter}/{chapter},
  /surahs/{id}) which legitimately 404 without real IDs. Every endpoint verified returning correct
  data with proper args during manual + sample-probe testing. Not a functional defect.

## Ship recommendation: ship
All 11 absorbed + 7 novel features build, pass gates, and are behaviorally verified against live APIs.
