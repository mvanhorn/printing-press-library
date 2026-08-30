# Acceptance Report: irail

  Level: Quick Check at Phase 5 (user-selected; Full dogfood was the recommendation)
         FULL dogfood then reran at publish time and passed - see below
  Runner: cli-printing-press dogfood --live --level quick
  Tests: 17/17 passed, 0 failed, 7 skipped (error_path probes on commands that
         take no positional argument)
  Gate: PASS

## Publish-time gate (authoritative)

The publish flow reruns the live gate at full level against the exact tree being
published. That run superseded the Phase 5 quick check:

    cli-printing-press dogfood --live --level full
    203 matrix entries: 130 passed, 0 failed, 73 skipped
    phase5-acceptance.json -> status pass, level full, matrix_size 130

So the whole command surface was exercised live before the library PR opened,
not just the quick-check subset.

## Runner matrix (Phase 5 quick check)
Covered help, happy-path and JSON-fidelity for the quick-check subset, including
`doctor` (config ok, auth not required, API reachable) against the live API from a
sandboxed HOME with an empty cache.

## Supplementary behavioural testing (run directly, beyond the quick matrix)
Because Quick Check leaves most of the 20-command surface unexercised, each novel
command was additionally exercised against the live API and its output inspected:

| Check | Result |
|---|---|
| `stations search gent` ranks busiest first | Gent-Sint-Pieters, Gent-Dampoort, Gentbrugge |
| `stations search FBMZ` telegraph code | exact match Brussel-Zuid/Bruxelles-Midi |
| `stations search` negative query | 0 results + honest note, nothing fabricated |
| `stations facilities` Ghent | step_free true, elevator/wheelchair/Blue-bike true, 7 days of desk hours |
| `stations facilities` via code FGSP | resolves to the same station |
| `transfer-risk` Oostende->Hasselt | 6 connections; Leuven transfer required 300s vs actual 1020s -> ok |
| `disruptions route` Ghent->Brussels | scanned 30 national entries, 3 route stations, 2 planned works matched |
| `observe` round 1 | 42 departures recorded |
| `changes` after one round | 0 changes + honest note (refuses to invent a diff) |
| `observe` round 2 | 84 stored |
| `punctuality` with data | 84 samples across 20 trains, typed numbers |
| `punctuality` on empty store | honest stderr hint + `[]`, no fabrication |
| `leave-by` +3h | recommends latest viable departure, 55m slack |
| `leave-by` past time | rolls to tomorrow and says so (fixes clirail's documented bug) |
| `saved` add/list/remove | round-trips, remove of a missing name exits 3 |
| `occupancy report` default | prints payload, sends nothing |

Typed-output verification: `punctuality` and `transfer-risk` emit real JSON
numbers and booleans (`required_known` is a boolean, `avg_delay_seconds` a number).

Exit codes verified: 2 for usage errors (7 cases), 3 for not-found (2 cases),
0 for help and dry-run (6 cases).

## Failures
None.

## Fixes applied during Phase 5 review
1. Corrected the narrative's typed-output claim. The raw endpoint commands pass
   iRail's string-encoded scalars through unchanged; only the analysis commands
   emit typed values. Added a troubleshooting entry so agents comparing
   `.delay > 300` in jq are not surprised.
2. Fixed a reachable panic: RFC3339 timestamps were sliced at a fixed offset for
   human output, and iRail omits time fields on some connections.

## Printing Press issues for retro
1. The generated syncer ignores spec param defaults, so an API needing a constant
   query parameter syncs the wrong content type.
2. Cache-freshness pre-read treats every resource as syncable, including those
   with required parameters.
3. `root.go` titles the CLI from the slug rather than `narrative.display_name`.

## Note on redaction
This CLI touches no personal data. iRail is an anonymous public timetable API and
no credential, account or user identifier appears anywhere in this run.
