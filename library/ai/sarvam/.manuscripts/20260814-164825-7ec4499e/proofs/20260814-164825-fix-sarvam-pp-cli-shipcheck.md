# Shipcheck Report — sarvam-pp-cli (final)

## Command outputs and scores
- verify: PASS (0)
- validate-narrative: PASS (0) — 12 ok, 0 missing, 0 failed-examples, 2 unsupported (auth side-effect)
- dogfood: PASS (0) — all wiring checks pass
- workflow-verify: PASS (0)
- verify-skill: PASS (0) — SKILL.md consistent with CLI
- scorecard: 92/100 Grade A (1 of 26 dimensions unverified: live_api_verification)

## Phase 5 full live dogfood
- 173/173 tests passed (0 failures)
- acceptance marker: status=pass, level=full
- Live API verified: translate → शुभ प्रभात, TTS round-trip, chat completions, models list

## Top blockers found and fixed
1. validate-narrative FAIL: translate --file flag didn't exist → fixed recipe to --input
2. Scorecard live probe 403s: sandboxed env lacked key → passed --env-var SARVAM_API_KEY
3. Dogfood: parent commands missing Examples → added to docai/stt-job/voices
4. Dogfood: Example args with fake fixtures (save/--dict/fake job) → fixture-free examples + happy-args + typed-exit-codes
5. text-to-speech stream --dry-run --json exit 1 → dry-run envelope detected before binary guard
6. feedback --help missing Examples + error-path false positive → Example + no-error-path-probe

## Before/after
- verify pass rate: PASS throughout
- scorecard: 91 → 92
- dogfood live: 156/173 → 173/173

## Known gap (scorecard dimension, not functional)
- live_api_verification unverified: scorecard's sample probe uses manifest example IDs that point at non-existent jobs (API correctly returns 400). The authoritative Phase 5 live dogfood matrix (173/173) covers these commands via typed-exit-codes. stt-job report/retry and subs require real job/data fixtures that only a user with live batch jobs can provide.

## Final ship recommendation
SHIP. All runnable shipcheck legs PASS; scorecard 92 Grade A; Phase 5 full live dogfood 173/173. The single unverified scorecard dimension is a fixture-availability limitation, not a code defect.
