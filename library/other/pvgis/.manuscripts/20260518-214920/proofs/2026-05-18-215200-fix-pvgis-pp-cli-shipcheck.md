# Shipcheck Report — pvgis-pp-cli

## Final verdict
**PASS (6/6 legs)** — ship.

## Legs
| Leg | Result | Elapsed |
|---|---|---|
| dogfood | PASS | 1.5s |
| verify | PASS | 2.2s |
| workflow-verify | PASS | 20ms |
| verify-skill | PASS | 514ms |
| validate-narrative | PASS | 278ms |
| scorecard | PASS (Total 81/100, Grade A) | 58ms |

## Scorecard breakdown
- Strong (≥9/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Quality, Local Cache, Path Validity, Sync Correctness, Dead Code (5/5), Agent Workflow (9/10)
- Polish gaps (not ship-blocking): mcp_token_efficiency 4/10, insight 4/10, cache_freshness 5/10, mcp_remote_transport 5/10

## Fix loops
1. **Loop 1 — outputformat default.** Initial generate left `outputformat` flag default empty; PVGIS responded with plaintext CSV-style tables instead of JSON. Fixed: changed default to `"json"` in all 5 endpoint command files (promoted_radiation, promoted_horizon, promoted_weather, production_monthly, production_hourly).
2. **Loop 2 — missing optimal-tilt command.** `validate-narrative` flagged `production optimal-tilt` in quickstart + recipe but it wasn't built. Added `newProductionOptimalTiltCmd` to novel.go (tilt sweep with fixed azimuth, returns best_tilt + best_e_y).
3. **Loop 3 — SKILL.md horizon reference.** SKILL.md and README.md had `horizon profile --lat ...` but `horizon` is a single-promoted shortcut (no `profile` subcommand). Fixed with sed.

## Phase 3 built features
**Absorbed (10, generator-emitted):**
- `radiation` (MRcalc) — monthly irradiance
- `production monthly` (PVcalc) — monthly/annual yield
- `production hourly` (seriescalc) — 8760-row hourly timeseries
- `weather` (tmy) — TMY 8760-hour climate
- `horizon` (printhorizon) — DEM-derived horizon profile
- + `doctor`, `sync`, `search`, `workflow`, `api` framework commands

**Transcendence (6, hand-written in internal/cli/novel.go):**
- `sites rank` — CSV-batch yield ranking
- `sites diff` — baseline-vs-target delta
- `production sweep` — 2D tilt×azimuth grid
- `production compare` — SARAH3 vs NSRDB vs ERA5 (correctly surfaces NSRDB 400-error for EU sites)
- `production optimal-tilt` — tilt sweep, fixed azimuth
- `weather similar` — TMY 12-month signature distance

## Honest gaps / known limitations
- `production compare`: PVGIS v5_3 rejects `PVGIS-NSRDB` for European coordinates with HTTP 400. The comparator surfaces this as a per-DB `error` entry (not a CLI failure), and the `summary.spread_*` numbers come from databases that succeeded. This is correct behavior.
- `outputformat=json` is now hardcoded as the default — `--outputformat csv` still works for users who want it.
