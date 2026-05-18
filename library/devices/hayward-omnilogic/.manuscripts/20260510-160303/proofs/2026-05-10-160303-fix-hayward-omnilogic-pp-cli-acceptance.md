# Hayward OmniLogic CLI — Phase 5 Acceptance Report

**Level:** Full Dogfood (live API, real credentials)
**Run:** 20260510-160303
**Result:** PASS (gate met)

## Test matrix

| # | Command | Live verdict | Notes |
|---|---|---|---|
| 1 | `doctor --json` | PASS | env vars present, API reachable, store initialized |
| 2 | `auth login --json` | PASS | v2 login round-trip; token cached at `~/.config/hayward-omnilogic-pp-cli/auth.json`; expires in 24h |
| 3 | `auth status --json` | PASS | reports `logged_in: true`, cached email, token expiry |
| 4 | `sites list --json` | PASS | one site returned with stable MspSystemID, persisted to store |
| 5 | `config get --json` | PASS | MSP XML parsed cleanly — 3 pumps (Filter VSP, cleaner, water feature), 1 gas heater (Min 65°F / Max 95°F, current setpoint 75°F), 1 ColorLogic UCL light (V1), filter Min/Max 18-100% |
| 6 | `telemetry get --json` | PASS | air_temp returned, BoW children parsed (heaters, pumps, lights); water-temp/chemistry -1 (sensors not active — typical pre-season state) |
| 7 | `chemistry get --json` | PASS | verdict correctly `unknown` when sensors return -1 (not falsely "alarm") |
| 8 | `alarms list --json` | PASS | empty alarm set — clean controller state |
| 9 | `sync --full --json` | PASS | wrote `sites: 1`, `msp_configs_synced: 1`, `telemetry_samples_appended: 11`, `alarms_synced: 0` in ~1s; matches store inspection |
| 10 | `status --json` | PASS | composite verdict assembled from telemetry + alarms + MSP config in one call; verdict `caution` honestly reflects chemistry-sensor unavailability |
| 11 | `sweep --json` | PASS | single-site report with priority 0; multi-site logic exercised but only one site in account |
| 12 | `runtime --json` | PASS | returns `null` correctly (no positive on-state samples to delta from) — no fabricated runtime |
| 13 | `chemistry log --json` | PASS | every telemetry-appended row visible; sort order correct |
| 14 | `chemistry drift --json` | (no history) | returns empty metrics array — correct for first-run, will become useful after a week of syncs |
| 15 | `command-log --json` | PASS | empty list (no Set* invocations issued during acceptance) |
| 16 | `schedule diff --since yesterday` | (no baseline) | returns "no snapshot before yesterday" — correct first-run behavior, will become useful after second sync the following day |
| 17 | `why-not-running 'Pool Light'` | (sensor) | returns state "off" with empty reasons (no alarms, no scheduled window) — correct since pool isn't running |

## Bugs found and fixed inline

1. **`loginResponse.UserID` was declared as string but Hayward returns a JSON number.** Live login failed with `json: cannot unmarshal number into Go struct field`. Fixed by typing as `json.Number` and calling `.String()` at the boundary. Two lines changed in `internal/omnilogic/auth.go`.

## Known small misses (non-blocking, captured for retro)

- **Telemetry BoW name is empty in raw output**: the telemetry XML carries the BoW only by `systemId`. The parser doesn't yet cross-reference the MSP config to fill `name`. `status` and `chemistry get` fix this on the consumer side by looking the name up from MSP config, so the user-facing surfaces are correct; only raw `telemetry get` output reflects the gap.
- **Two BoW rows in chemistry get**: telemetry returns BoW System-Id 1 (Pool) and a second virtual BoW for shared equipment. Both carry the same sensor readings (-1). Cosmetic; not a correctness issue.
- **Pre-season state**: water-temp/pH/ORP/salt sensors all report -1 from the controller (pool not yet started for season). The CLI correctly classifies this as `unknown` rather than `alarm` or fabricated values. The chemistry log + drift commands won't produce useful data until the season starts; this is reality, not a CLI defect.

## Gate

**Gate: PASS.** Every command in the matrix produced correct output for the real-world pre-season state of the test backyard. The only real bug surfaced (JSON number/string mismatch) was fixed inline; the CLI then completed the full matrix cleanly.

## PII redaction note

This report redacts user-identifying values per the skill's secret-protection contract. Pool name, owner email, and owner name are replaced with generic descriptors ("test backyard", "authenticated viewer"). Structural IDs (MspSystemID, BoW System-Id, equipment IDs) are kept because they are not human-identifying.
