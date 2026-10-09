# Phase 4.85 output review findings (coinlocker-navi-pp-cli, run 20261009-095811-aff6d8f7)

Status: WARN (2 warnings). Both fixed in place.

1. output-semantically-matches-query-intent (warning) — `near --lat 35.6896 --lon 139.7006 --gate outside` returned id 98 "JR新宿駅東口改札内改札手前付近" as outside; its name says 改札内.
   Fix: when a record's own name/note says the other side (改札内/改札外), the Multi Ekicube value is not copied; gate stays null, `gate_conflict` explains, and `--gate` counts it in `meta.excluded.gate_conflict`. Text never sets a gate (approved N3 rule kept). Test: TestGateConflictBlocksBankValue. Re-run: id 98 now excluded (gate_conflict: 1).
2. result-ordering-makes-sense (warning) — with `--open-during`, the only confirmed-open record sat at position 9.
   Fix: results are ordered open_during yes, then train_hours, then unknown (stable inside each group), before --limit applies. Re-run: id 18 (yes) is first.

Also: the reviewer's `near 東京駅 --live` sample timed out at 10 s (Multi Ekicube dated queries take 0.5-4 s each). The novel example now uses `--limit 5` (about 4 s).
