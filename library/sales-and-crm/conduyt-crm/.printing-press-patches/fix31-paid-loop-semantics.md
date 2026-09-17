# Fix 31 — paid-loop semantics

- Strictly validate non-negative integral verification counters and stop on
  contradictory or stalled progress before another paid POST.
- Preserve classified auth and not-found exits when partial-output helpers
  report client failures; malformed responses remain API-class failures.
- Regression coverage locks request counts and status-code mappings.
