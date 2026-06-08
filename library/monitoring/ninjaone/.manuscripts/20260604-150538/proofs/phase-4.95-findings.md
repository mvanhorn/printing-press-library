# Phase 4.9x Review Findings (NinjaOne)

## Phase 4.95 Local code review (hand-written Go) — autofixed in place
- E1 (ERROR) stale_devices.go: reboot cohort misaligned with sorted+truncated rows → `--limit`+`--reboot --apply` could reboot wrong devices. FIXED (derive reboot list from sorted rows via map[int64]njDevice).
- W (patch_gaps.go): redundant client-side severity EqualFold re-filter could silently zero results when API normalizes severity case. FIXED (removed; server param is authoritative).
- W (cf_hygiene.go): out-of-scope rows silently dropped. FIXED (added `skipped_out_of_scope` count to view).
- W (alert_clear.go): alerts whose device fell outside the capped device fetch silently excluded from org= predicate. FIXED (added `unresolved_device_orgs` count + note).
- Verified CLEAN: SQL parameterization (history table), resource leaks (defer rows.Close/db.Close), context threading, scan-cap truthfulness, cursor-loop termination.

## Phase 4.8/4.9 SKILL/README correctness — autofixed in place
- E1 (ERROR) README troubleshooting referenced renamed `alert-reset` + nonexistent `run-script`. FIXED → `patch-sweep, alert-clear`. (research.json source also fixed.)
- W1 README `--max-pages` (sweep has `--max-scan-pages`). FIXED in README + research.json.
- W3 alert-storms described as grouping by "location" (binary does not). FIXED in README + SKILL + research.json.
- W4 README `auth-status` → `auth status`. FIXED.
- W2 (RETRO CANDIDATE, not patched in generated logic): `which` capability index scorer only substring-matches the full query against description (no per-token description matching), so `which "find stale devices"` does not surface `stale-devices`. Generator-template limitation in internal/cli/which.go scoring. Description improved as partial mitigation; real fix belongs in the press → file at retro.
