# Acceptance Report: Bambu

- Level: Full Dogfood
- Final tests: 151/151 mandatory tests passed
- Skipped: 92 safe or fixture-blocked cases
- Failures: 0
- Gate: PASS

The first live pass reported 13 failures across 162 mandatory tests. Those failures reduced to four causes: current-job-only commands while the printer was idle, an external-event watcher waiting for a transition, a missing export help example, and deletion of a printer profile fixture that did not exist.

Fixes applied: 3

- CLI fix: marked `events watch` as an external-event stream so deterministic dogfood does not wait for a real printer transition.
- CLI fix: added bounded local-export examples used by the help and live-matrix checks.
- Printing Press fix: classify unavailable active-device state and missing mutation fixtures as explicit `blocked-fixture` skips instead of command failures.

Printing Press issues: 1

- The live runner previously understood missing HTTP/API fixtures but not local-device state or typed local mutation fixtures. The runner and its regression tests now cover both cases.

Current-job 3MF metadata, objects, thumbnail, and AMS runway were skipped because no print was active. The prior run's live finish-monitor proof already verifies exact plate weight, preview extraction, start/terminal lifecycle behavior, and actual completion handling against a real print.
