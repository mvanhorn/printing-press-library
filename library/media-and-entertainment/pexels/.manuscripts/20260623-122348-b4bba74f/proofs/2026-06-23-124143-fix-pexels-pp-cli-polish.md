# Pexels CLI — Polish Proof (Phase 5.5)

Mid-pipeline polish (STANDALONE_MODE=false). Ship recommendation: **ship**.

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Scorecard | 93 | 95 | +2 |
| Verify | 100% | 100% | — |
| Dogfood | PASS | PASS | — |
| Go vet | 0 | 0 | — |
| Gosec (hand-authored) | 6 | 0 | -6 |
| Tools-audit | 0 | 0 | — |
| PII-audit | 2 pending | 0 | -2 |

## Fixes
- Cleared 6 hand-authored gosec findings (0750 dir perms, 0600 file perms in download.go/attribution_export.go; narrow #nosec G304 with durable reasons in rateledger.go).
- Changed download default --output from "." (cwd) to "pexels-downloads/" so dogfood/acceptance runs no longer litter the publishable CLI root.
- Removed 64 stray download artifacts; resolved both PII findings at source (public Pexels profile ID false-positive).
- Added .gitignore (media, sidecars, download dir, local DBs, polish ledgers).

## Deferred (generator retro candidates, non-blocking)
- search defaults to --data-source auto (live) vs the "offline re-search of synced media" novel-feature framing; search.go is DO-NOT-EDIT (lacks pp:data-source local annotation). Feature works via --data-source local (verified).
- Template-leftover examples in generated search help.
- 28 gosec findings in generator-emitted DO-NOT-EDIT files (G201/G204/G104/G117/G119).

ship_recommendation: ship | further_polish_recommended: no
