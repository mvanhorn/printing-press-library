# EDEKA shipcheck (2026-11-17)

Final run: shipcheck-run5.txt - Verdict PASS (7/7 legs).

| Leg | Result |
|---|---|
| verify | PASS - pass rate 100% (53/53, 0 critical) |
| validate-narrative | PASS - 10 narrative commands, full examples |
| dogfood | PASS (WARN: 3 dead generated helpers: hasChangedLocalFlags, requiredInputWaived, retainCLIQueryParams); novel features 7/7; examples 10/10; dead flags 0 |
| workflow-verify | PASS (workflow-pass; no workflow manifest) |
| apify-audit | PASS |
| verify-skill | PASS (all checks, canonical sections) |
| scorecard | PASS - 89/100 Grade A; live sample probe 4/4 (3 mutating examples skipped) |

## Runs and blockers fixed
1. run1: scorecard HOLD "manifest evidence unavailable" (no .printing-press.json: the first generate failed its go-test gate on two Windows-only cliutil tests, so the manifest was never written) and the live probe timed out on `deals check`. Fix: regenerated a clean tree with --validate=false in scratch, rebuilt the work dir from it plus the hand-written files (also undoing a broken --force merge of cliutil tests), re-applied the Windows skip patch.
2. run2: verify-skill FAIL (hand-written commands wired through novelCommandHooks were invisible; `Use` strings with " | " confused the positional check). Fix: wiring moved into root.go with one variable per Find block (patch record edeka-root-command-wiring), Use strings normalised.
3. run3: PASS 7/7, probe still failing `deals check` (examples had no --market; the probe runs in a clean environment). Fix: examples and happy-args carry --market doerflinger-ulm.
4. run4: verify-skill FAIL (--market registered through a helper is invisible to the static check). Fix: inline flag registration. Also `deals check` searches now run in parallel on an anonymous shop (the shop serialises requests of one PHP session).
5. run5: PASS 7/7.

## Before/after
- verify pass rate: 100% in every run (53/53).
- scorecard: 87 (run1) -> 89 (run5).

## Behavioral samples (live, doerflinger-ulm)
- slots earliest -> Wed 2026-11-18 10:00-11:00 pickup; deals check milch kaffee butter -> 10 offers; price history 5915407005MXST -> recorded prices; compare markets milch --zip 89081 -> 2 markets with best match.

## Verdict
ship
