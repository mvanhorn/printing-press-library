# MotoGP CLI — Phase 5 Acceptance

- Level: Full Dogfood (live, no auth required)
- Matrix: 153 tests, 153 passed, 0 failed, 72 skipped (blocked-fixture / help-only)
- Gate: PASS

## Fixes applied during dogfood (23 → 0 failures)
1. happy_args/examples camelCase → kebab-case flag names (--season-uuid, --season-year, ...).
2. Positional happy_args required angle-bracket labels (`<legacy-id>=7444`) — without brackets
   the runtime silently ignored them and chained a wrong-type UUID into `{legacyId}` → 404.
3. teams: uses broadcast-category UUIDs (737ab122…+2025), not results-API UUIDs.
4. Novel commands: check dryRunOK before the arg-count guard, then return a usage error
   (non-zero) on missing args instead of help (exit 0) — satisfies the invalid-argument path.
5. circuit-history: returns not-found (exit 3) when the circuit matches zero events; empty
   result (exit 0) reserved for a real circuit with no finished races yet.
6. livetiming endpoint (empty 200 body between sessions) replaced by a hand-built `live`
   command that reports {"active":false,"status":"no active session"} with exit 0.
7. entry resource removed: the documented /event/{id}/entry endpoint is dead (HTTP 400
   "No static resource" for every event/category combo on the live API).

## Behavioral verification (real data)
- results 2024 qatar motogp race → Bagnaia, 25 pts (correct).
- title-race 2024 motogp → R1 Bagnaia, R2 Martin leads 41, R3 Viñales wins/Martin leads 54 (correct).
- circuit-history mugello → 2025 M.Marquez, 2024/23/22 Bagnaia (correct).
- since 2025 --winners → Thailand/Argentina M.Marquez, Americas Bagnaia (correct).
- h2h resolves current riders, reports ambiguity for same-surname brothers.

## Known gaps
- Name resolution (results/title-race/h2h/career/circuit-history) covers current-season
  riders and synced seasons only; retired riders use raw `riders get/stats` with UUID/legacy-id.
- `live` returns "no active session" between sessions (feed only populated during a live session).
