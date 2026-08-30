# Acceptance Report: youtube (reprint 20260819-035139)

Level: Full Dogfood ("more than full" per operator)
Tests: 167/167 passed (press-owned live matrix, exit 0, marker written by runner)
  - Run 1: 146/167 — 20 endpoint happy-path probes lacked YouTube's required params (fixed via pp:happy-args + real examples per the generator's own TODO), 1 missing Examples section (feedback).
  - Run 2: 166/167 — feedback Example had landed inside the Long string; fixed as a proper field.
  - Run 3: 167/167 PASS.

Behavioral gauntlet on top (17/17 after re-verification):
  watch add x2 ✓ · backfill 60+60 videos ✓ · monitor 20 snapshots + comments ✓ · velocity 10 evaluated ✓ ·
  growth both channels ≥2 snapshots ✓ · breakouts ranked with 7/10 term-relevant titles, views/sub computed ✓ ·
  comments-mine 195 channel-wide comments, like-desc ordering, 20 keywords ✓ · packaging thumbnail file on disk
  (113 KB) + hook text 706 chars ✓ (two initial FAILs were a test-harness capture artifact; re-verified strictly) ·
  workspace isolation proven both directions ✓ · key ring stores/masks/removes, full value never printed ✓ ·
  negative path exits 3 cleanly ✓ · transcript --format markdown renders ✓

Fixes applied during Phase 5: 12 (10 endpoint happy-args/example fills, 1 feedback Example placement x2 attempts, 1 search-list example quoting)
Printing Press issues for retro: sampler SIGBUS staging race; generator emits TODO-placeholder examples that guarantee first full-dogfood failures on parameter-mandatory APIs.

Gate: PASS
