# Flighty CLI Shipcheck Report

## Command
```
cli-printing-press shipcheck --dir $CLI_WORK_DIR --spec $SPEC --research-dir $API_RUN_DIR --api-key live-noauth
```
(`--api-key live-noauth` forces live mode; the spec has `auth.type: none` so no credential is ever sent.)

## Final results (second run, after fixes)
| Leg | Result | Notes |
|---|---|---|
| verify | PASS | live mode, 34/34 command probes, Data Pipeline PASS |
| validate-narrative | PASS | strict + full-examples (9 ok, 0 failed) |
| dogfood | PASS | novel_features_check: 7 planned / 7 found |
| workflow-verify | PASS | |
| apify-audit | PASS | n/a |
| verify-skill | PASS | SKILL.md matches shipped CLI |
| scorecard | PASS | **89/100 Grade A** |

## Verdict: **PASS (7/7 legs)** — recommendation: ship

## First-run failures and fixes applied
1. **verify FAIL — "Data Pipeline: sync crashed"**: verify defaulted to mock-server mode (no `--api-key`); the spec-derived mock cannot synthesize the Next.js RSC HTML surface. Fix: run verify in live mode (`--api-key live-noauth`; spec has `auth.type: none` so nothing is sent). Live sync verified: 314 records.
2. **validate-narrative FAIL**: research.json quickstart said `sync --resources airport` (singular); the actual resource is `airports`. Fixed research.json (quickstart + troubleshoots).
3. **Sample Output Probe failures (2/7)**: `airports worst`, `airports airline`, `airports nearby` required a synced local store, which the sandboxed live-check HOME lacks. Fix: reclassified `worst`/`nearby`/`airline` from `pp:data-source local` to `auto` — local mirror first (with sync hints), one live catalog fetch when the mirror is empty. `airports diff` intentionally stays `local` (snapshot history is inherently local-only).
4. **Scorecard Dead Code 2/5**: noted; the gap list flags 5 MCP tools as auth-required — cosmetic (the spec has no auth); readiness still "full".

## Data-source strategy notes
- `list`/`tv`/`show`/`arrivals`/`departures` (generated endpoint commands): auto (live fetch + write-through + network-fallback to store).
- `worst`/`nearby`/`airline`: auto (local-first, live fallback).
- `diff`: local-only by design.
- `find-flight`/`compare`/`route`: auto (live detail/board fetches with graceful partial-failure accounting).

## Evidence highlights (live)
- `airports list --json`: 155 airports with region + status + delay summaries.
- `airports show den --json`: METAR `KDEN 271653Z VRB03KT 10SM FEW090...`, flight rules VFR, 1059 arrival ops.
- `airports find-flight den 2381 --json`: UA2381 "2h 45m Late", 09:45→12:30, Gate B10.
- `airports worst --limit 5`: Manchester 4385m cumulative delay, MAJOR_ISSUES.
- `airports airline UA --top 5`: 49 delayed / 2 canceled of 51 across LGA/LHR.
- `airports route man cdg --json`: forward 86% delayed (6/7) + reverse view.
- `airports diff` after two syncs: ADD NORMAL→MINOR_ISSUES, network-wide delay deltas.
