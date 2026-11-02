# Acceptance Report: gusto

- Level: Full Dogfood (operator chose Full on 2026-10-24).
- Tests: 157/157 passed (runner `dogfood --live --allow-destructive`, results `2026-10-24-202511-dogfood-results.json`). Mutating commands still run under `--dry-run` in the matrix; only `payrolls compare --no-calculate` runs live via `pp:live-happy-path`.
- Proof-covered features: `payrolls mileage`, `payrolls prep`, `payrolls verify` -> `payrolls-write-lifecycle.md` (a real $0.01 write on the open draft, read back, verified, removed; the draft ended with 0 hours and no reimbursements).
- Auth context: cookie session (press-auth import), passed to the runner through `GUSTO_CONFIG`.
- Gate: PASS

## Failures (first run) and fixes applied: 6
- `tail session --json` printed nothing when the authenticated viewer had no sign-in events. CLI fix: an empty page emits one `{"event":"empty"}` NDJSON line. **Printing Press issue** (tail template).
- `local-data purge --dry-run --json` had no `action` field, which the runner requires. CLI fix: added `"action": "local-data purge"`. **Printing Press issue** (template and runner disagree).
- Hollow `payrolls compare`: added `pp:live-happy-path` with `--no-calculate` (reads stored totals; no draft change).
- Hollow `payrolls verify`: added `--stdin` (timesheet as a JSON array), a `pp:happy-stdin` fixture and a stdin-first Example. The fixture's placeholder employee exits 1 (declared), so verify is also proof-covered.
- Hollow `payrolls mileage` / `payrolls prep`: `pp:verified-by-proof`.
- The runner appends `--stdin` to the Example args, so an Example that already has `--from` conflicts. Fixed by ordering the Examples.

## Printing Press issues: 4
- tail template emits zero lines for an empty page (invalid NDJSON for a single poll).
- local-data purge dry-run JSON lacks the `action` field the live runner checks.
- The runner treats a declared non-zero exit as a skip, so a gate command (exit 1 on mismatch) can never clear hollow coverage with a placeholder fixture.
- `pp:happy-stdin` must be JSON, and the runner appends `--stdin` to Example args without removing a conflicting file flag.
