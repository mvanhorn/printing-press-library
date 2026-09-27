# Acceptance report: Walkerplus

Level: full. Gate: PASS. Printing Press runner: 50/50 executed checks passed; 0 failures; 40 skips/unverified cases retained in the runner report. The user required full live verification in the approved brief, so no redundant depth confirmation was needed.

All five approved domain commands have passing happy-path and JSON-fidelity checks. Skips cover inapplicable positional-error checks and inherited hidden framework/profile/feedback workflows; they are not claimed as tested. Additional deterministic CLI validation covers invalid dates, city/prefecture conflicts and bounds.

Skip reasons:
- 10: no positional argument
- 1: mutating command dry-run only
- 5: mutating command requires --allow-destructive
- 2: command does not honour --dry-run
- 3: non-id positional "text" at depth 0
- 4: mutating command; error_path would call live API without --dry-run
- 11: non-id positional "name" at depth 0
- 2: no --dry-run short-circuit
- 2: non-id positional "query" at depth 0

Supplemental live proofs validate Tokyo, Kyoto, Hokkaido, Miyagi and Osaka; city code/slug/Japanese alias resolution; festival/exhibition/seasonal categories; free and indoor filters; starts/ends/overlap modes; exact edition and location relevance; schedule caveats and lazy detail fetch counts. See cli-live-*.txt and cli-acceptance-report.json. No mutations or authenticated customer data.

Source-bound phase5-acceptance.json was emitted by the runner, never hand-written. No CLI fixes were needed during this matrix.

Final refresh: the marker was regenerated after a deterministic test correction during polish. dogfood-live-full-final.json again passes 50/50 executed checks with the same 40 disclosed skips. Final full external package passes, 59.950s; vet/build and all other packages passed in the integrated sweep.
