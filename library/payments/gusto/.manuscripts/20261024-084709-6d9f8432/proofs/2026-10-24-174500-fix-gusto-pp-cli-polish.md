# Polish: gusto-pp-cli (Phase 5.5)

Skill: printing-press-polish (forked), args = $CLI_WORK_DIR plus the Phase 3 bundle (4 planned, 4 built, none missing, not a sub-60 reprint).

```
                    Before    After     Delta
  Scorecard:        81/100    85/100    +4
  Verify:           97%       97%       0 (32/33; only browser-session-proof, approved harness gap)
  Tools-audit:      0         0         0
  PII:              9         0         -9
  gosec (hand):     1         0         -1
  Dogfood:          WARN      PASS
```

Fixes applied:
- CLI description matched to the research headline (spec.yaml, root.go, .printing-press.json, manifest.json).
- 9 unused endpoint-mirror helpers removed from internal/cli/helpers.go (dead_code 1/5 -> 5/5).
- payrolls verify opens --from through filepath.Clean (gosec G304).
- Test fixtures: masked account and bank name replaced with placeholders.
- payrolls compare prints the change as signed dollars and notes an all-$0.00 draft; TestSignedMoney added.
- payrolls verify: an employee missing from the draft reports got="not on draft" with the full message in `detail`.

Skipped: 99 gosec findings in generated files (retro); auth_protocol 2/10 and cache_freshness 5/10 are structural for a cookie-session, live-only CLI; dry-run exit-0 behaviour kept; raw JSON on mileage/prep dry-run in a TTY (cosmetic).

Parent follow-up done: the two research files and two proof/discovery files that named the business, owner or operator were scrubbed; superseded dogfood results were removed; the kept results file has ids replaced with a placeholder UUID. Live dogfood re-run after polish: PASS 157/157, 3 proof-covered, draft left empty.

ship_recommendation: ship
