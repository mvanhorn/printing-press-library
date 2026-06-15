# Roadside America — Phase 5 Acceptance (Full live dogfood)

**Level:** Full Dogfood
**Gate: PASS** — 64/64 tests passed (live, against roadsideamerica.com).
Machine marker: `proofs/phase5-acceptance.json` → `{"status":"pass","level":"full","matrix_size":64,"tests_passed":64}`.

The live matrix exercised every leaf command's help / happy-path / JSON-fidelity / error-path probes against the real site, using each command's `Example:` args (e.g. `state TX`, `show 2055`, `category biggest`, `compare TX CA`, `near "Austin, TX"`). Polite rate limiting (~1 req/3s) and the local cache kept request volume low; `sync`/`trip` curtail work under `PRINTING_PRESS_DOGFOOD=1`.

No auth required (public site), so no credential was needed. Coordinates / place geocoding (Nominatim) and the SQLite cache both verified end-to-end.

Fixes applied during shipcheck (see shipcheck proof): data-pipeline (`sql` plain output + real `sync`), verify-skill (real python3 vs the modern-python shim), `near`/`trip` verify hermeticity, and the `show` writeup JS-leak bug.

Printing Press issues for retro:
- `sql`/`search` framework commands not auto-emitted for an all-HTML spec (hand-built).
- v4.22.1 data-pipeline gate requires rows-after-sync even in mock mode; HTML-scrape CLIs can't populate from the mock, so a verify-mode seed is needed.
- dogfood reads the generated `defaultSyncResources()` (empty) rather than the actual `sync` behavior → stale WARN.
