# NCCPL CLI — HANDOFF (5 Sep 2026)

## Locations
- Library (promoted): `~/printing-press/library/nccpl`  — binary `nccpl-pp-cli`
- Working dir:        `~/printing-press/.runstate/personal-74f0f171/runs/20260905-000548-4451d26d/working/nccpl-pp-cli`
- Manuscripts:        `~/printing-press/manuscripts/nccpl/20260905-000548-4451d26d/`
- CLI store:          `~/.local/share/nccpl-pp-cli/data.db`
- Research DB:        `~/psx-research/data/research.db`  (table `nccpl_panel`, 871k rows)
- Evidence:           `manuscripts/.../proofs/cloudflare-investigation.md` (17 hypotheses)
- Retro queue:        `manuscripts/.../proofs/retro-candidates.md` (9 items, UNFILED)

## State
Promoted. Score **75/100 Grade B**. category `payments` (same as `psx`). 10 novel features.
shipcheck 6/7 legs PASS; phase5 marker `status: pass` (quick, 14/14).
3 unverified dims: `path_validity`, `auth_protocol`, `live_api_verification` — all trace to
NCCPL being unreachable by HTTP.

## The 11 hand-written commands
`flows` (scstrade FIPI/LIPI, unattended) · `capture` (controlled Chrome, `--launch`, `--stride`)
· `ingest` (HAR) · `sync` · `coverage` · `verify` · `panel` · `universe` · `leverage`
· `risk-changes` · `contract-check`

## THE CENTRAL FACT — read before retrying anything
`cf_clearance` **cannot be replayed by any non-browser HTTP client**. Proven:
a byte-exact TLS fingerprint match to real Chrome 149 (`curl_cffi chrome145/146` — identical
`ja4` `t13d1516h2_8daaf6152771_d8a2da3f94cd`, `ja4_r`, `peetprint_hash`
`1d4ffe9b0e34acac0bd883fa7f79d7b5`, akamai fp), exact header set, valid cookies, over BOTH
h2 and h3 → **same 403 challenge as sending no cookies at all**. The cookie is IGNORED, not
rejected. 17 hypotheses eliminated; do not re-test them (see the evidence file).

**BUT ACCESS IS NOT DEAD.** A *headed* real Chrome in a throwaway profile SELF-SOLVES the
challenge and then reads everything:
  GET  /api/fipi/latest-date            -> 200
  POST /api/var-margins/data            -> 200, 1091 rows
  POST /api/open-positions/data         -> 200, 68 rows
  POST /api/slb-market-information/data -> 200, 3 rows
Headless is hard-blocked (`HeadlessChrome/149` UA is the tell). Three clearance grades seen:
426 (headless) / 533 (daily profile) / 597 (fresh headed).

## Data already collected
- **Free float: 173 snapshots, 2016-09-01 → 2026-09-04, 125,230 rows.** Monthly stride.
  In `research.db.nccpl_panel` (871,462 rows; metrics free_float, var_value, hair_cut,
  half_hour_avg_rate, 26week_avg, acc_qty%). All 157 spot snapshots join to `daily_bars`.
- **flows** (FIPI/LIPI sector × investor): 807 rows, 2026-08-24..09-04, 9/9 dates pass both
  NCCPL invariants.

## Research findings produced
1. **§0ak answered — do NOT settle it.** FFC 71.2% of Fertilizer, HUBC 74.8% of Power Gen,
   six sectors >80%, TOBACCO 98.3%. Cap-weighted sector dispersion is largely single-stock.
2. **§0ao control PASSED.** 27 symbols left NCCPL's universe since 2024; 23 still in
   `fundamentals`; **0** pass `fundamentals_live`. Independent source validates that patch.
3. **Universe collapsed 575 → 463 symbols between 2018 and 2019.** Any backtest crossing that
   boundary compares two different markets.

## Gotchas
- `free_float` is SHARE COUNT, not currency. × close = free-float mkt cap.
- Dataset includes futures (`SYM-SEP`, `SYM-OCT`) repeating the spot free float. Filter
  `symbol NOT LIKE '%-%'` for cross-sectional work.
- scstrade `loadmain` returns PERCENTAGE SHARES summing to 100, not flows. Use
  `loadfipisector`.
- Three date encodings: single-date `YYYY-MM-DD`; `fipi`/`lipi` `DD/MM/YYYY`; sector-wise
  `YYYY-MM-DD`. Wrong one = empty array with HTTP 200.
- Envelope keys differ per endpoint; `fipi-normal`→`records` but `lipi-normal`→`data`.
- `capture` chunk by year (~14 dates); the CDP socket dies past ~18 consecutive fetches.

## Open
1. Rotate NCCPL cookies in Chrome (overnight transcript leak, scrubbed; `cf_clearance` valid to 2027).
2. File the 9 retro candidates (`/printing-press-retro`).
3. Raise score 75 → 90.
4. Crack unattended live NCCPL fetch (currently needs `capture --launch`).
5. Publish to the public library under `payments`.
