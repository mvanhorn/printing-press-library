Polish pass:
  Verify:      100% -> 100%
  Scorecard:   96 -> 96
  gosec (hand-written): 1 -> 0 (narrow #nosec G304 on operator-supplied CSV path in meta_import.go, recorded in hand-patches.txt)
  Tools-audit: 0 pending
  Skipped: 27 gosec findings in generator-emitted files (retro); 4 spec-mirror commands without examples; live check thin (no credentials by design).
  ship_recommendation: ship
