# Full live acceptance

Level: Full, explicitly authorized by the batch brief. Gate: PASS, produced by the Printing Press runner (phase5-acceptance.json). Mandatory matrix: 106/106 pass, 0 fail. Auth type:none, all calls publicly testable. No booking/payment/account/reservation operation or --allow-destructive was used.

The runner records 74 additional skip/unverified checks. These concern absent positional error paths on flag-only commands, ancillary local catalog/profile fixtures, unapproved mutating/feedback operations, and dry-run diagnostic recognition. Each of the ten planning commands passed help, happy path, JSON fidelity and dry-run JSON; meaningful invalid/calendar/parser/throttle/cap paths are covered by Go regressions and explicit live/local branch proofs.

Fourteen reference dry-run rows were marked skip because the runner combined JSON stdout with human diagnostics from stderr. Direct separate-stream verification confirms all14 emit pure valid JSON stdout, dry_run:true/source:dry-run, and no request; proof reference-dry-run-stdout.json. This is a runner/template diagnostic-recognition retro candidate, not a live/dry-run request leak.

Skip reason counts: {"non-id positional \"interface\" at depth 0": 2, "command does not honour --dry-run": 16, "no positional argument": 28, "mutating command dry-run only": 1, "mutating command requires --allow-destructive": 5, "non-id positional \"text\" at depth 0": 3, "mutating command; error_path would call live API without --dry-run": 4, "non-id positional \"name\" at depth 0": 11, "no --dry-run short-circuit": 2, "non-id positional \"query\" at depth 0": 2}

Source-supported scope: stations/routes all46 names and three corridors; live adult basic fares all classes; product/window/baggage/boarding/change/refund planning; current basic publications with15 curated incomplete examples; exact canonical booking handoff. Live domain proof10/10 (live-e2e-final), corrected live provenance5/5, independent MAX reviewer PASS at round3. Child/product price matrices and dated operation/seat inventory remain unknown; bilingual oversized/confirmation conflicts remain explicit.

Builder fixes:13 review findings across two fix rounds; no live matrix bug remains. Printing Press retro candidates: unbounded reference template, absent domain ceiling, stateless MCP boilerplate, unsupported exclusivity claim, combined-stream dry-run recognition, and generated/reserved static-analysis findings (gosec-triage.json).
