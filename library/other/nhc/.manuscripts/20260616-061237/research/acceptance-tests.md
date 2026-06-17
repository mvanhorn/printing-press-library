# nhc-pp — Acceptance Tests (against REAL fixtures)

Every test below runs against a file under
`/Users/abediaz/ghostex/chats/nhc-pp/fixtures/`. Expected values are the actual
contents of those fixtures (verified this session), so these can be run against
the generated CLI once it exists.

**Sourcing note (important):** the task example referenced `milton-2024.json`,
which **does not exist** — Milton and Beryl were never archived in
`CurrentStorms.json` (full-year Wayback CDX = 6 captures, only 2024-09-27 Helene
non-empty). Storm-list / storm-detail / graphics tests are therefore sourced from
**`helene-2024.json`** (3 storms: Helene, Isaac, John). Advisory-parsing tests
ARE sourced from the real **`milton.*.txt`** fixtures (Milton exists as an
advisory product). This substitution is the documented archival gap, not a gap in
coverage.

How to run: point the CLI at the fixture (e.g.
`nhc-pp storms --fixture <path>` or feed via stdin), capture stdout, assert. The
`jq`/`grep` assertions below are the executable contract.

---

## A. `storms` — list (source: helene-2024.json)

Fixture: `fixtures/currentstorms/helene-2024.json`

- **A1 — count.** Output `data.count` MUST equal `3`.
  `jq '.data.count' == 3`
- **A2 — names present.** `data.storms[].name` MUST contain `Helene`, `Isaac`,
  `John`.
- **A3 — id pattern.** Every `data.storms[].id` MUST match
  `^(al|ep)\d{6}$` and uppercased MUST match `(AL|EP)\d+2024`. Specifically
  Helene id MUST equal `al092024`.
- **A4 — typed vitals.** Helene `intensity_kt` MUST equal `50` (number, coerced
  from string `"50"`); `pressure_mb` MUST equal `972`; `lat` MUST equal `34.2`;
  `lon` MUST equal `-83.0`.
- **A5 — classifications.** Helene/John classification `TS`, Isaac `HU`.
- **A6 — link present.** Helene entry MUST surface
  `links.publicAdvisory == "https://www.nhc.noaa.gov/text/MIATCPAT4.shtml"`.

## B. `storm <id>` — detail (source: helene-2024.json)

- **B1 — forecast-cone link (the headline requirement).** `storm al092024` MUST
  surface the forecast cone URL exactly:
  `https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz`
  (mapped from `trackCone.kmzFile`).
  `jq -r '.data.gis.trackCone.kmz'` == that URL.
- **B2 — every product url.** `storm al092024` MUST surface:
  - publicAdvisory `…/text/MIATCPAT4.shtml`
  - forecastAdvisory `…/text/MIATCMAT4.shtml`
  - forecastDiscussion `…/text/MIATCDAT4.shtml`
  - windSpeedProbabilities `…/text/MIAPWSAT4.shtml`
  - forecastGraphics `…/graphics_at4.shtml`
- **B3 — null contract (Isaac).** `storm al102024` MUST emit
  `windWatchesWarnings`, `stormSurgeWatchWarningGIS`,
  `potentialStormSurgeFloodingGIS`, `peakSurgeKML` as JSON **`null`**
  (present-but-null, NOT omitted, NOT a fabricated URL).
  Isaac `trackCone.kmz` MUST equal
  `…/storm_graphics/api/AL102024_006adv_CONE.kmz` and `forecastGraphics` page
  MUST equal `…/graphics_at5.shtml`.
- **B4 — John null contract.** `storm ep102024`: `stormSurgeWatchWarningGIS` and
  `peakSurgeKML` MUST be `null`; `windWatchesWarnings` MUST be present (non-null).
- **B5 — 30-key fidelity / no fabrication (negative test).** The raw storm object
  has exactly 30 keys. The CLI MUST NOT emit `windHistory` or `keyMessages`
  anywhere (those keys do not exist in the feed).
  `jq '.. | objects | keys[]' | grep -ciE 'windHistory|keyMessages'` == 0.
- **B6 — not-found.** `storm al992024` (absent) MUST exit `3` and emit
  `error == "storm not found"` with an `active` list including `al092024`.

## C. `advisory` — parse text products (source: real milton.*.txt / helene.*.txt)

Fixtures: `fixtures/advisories/milton.public.txt`, `milton.fstadv.txt`,
`milton.discus.txt`, `helene.public.txt`.

- **C1 — TCP max winds (the headline requirement).**
  `advisory AL142024 --type tcp` on `milton.public.txt` MUST extract
  `fields.max_sustained_winds` containing `160 MPH` (real line:
  `MAXIMUM SUSTAINED WINDS...160 MPH...260 KM/H`).
- **C2 — TCP location & pressure.** Same call MUST extract
  `fields.location == "23.4N 86.5W"` and `fields.min_central_pressure`
  containing `915 MB`.
- **C3 — storm identity from ATCF id, not PIL.** Parsed `atcf_id` MUST equal
  `AL142024` (from line `NWS National Hurricane Center Miami FL       AL142024`),
  NOT derived from the AWIPS PIL `MIATCPAT4`. `storm` MUST equal `Milton`,
  `advisory_number` MUST equal `16`.
- **C4 — issuance time from the human line, not the placeholder.** `issued` MUST
  equal `1000 PM CDT Tue Oct 08 2024`. The CLI MUST NOT parse time from line 2
  `TTAA00 KNHC DDHHMM` (it is a literal placeholder). `issued` MUST NOT contain
  the string `DDHHMM`.
- **C5 — TCM is a different extractor.** `advisory AL142024 --type tcm` on
  `milton.fstadv.txt` MUST extract `max_sustained_winds` containing `140 KT`
  (real line: `MAX SUSTAINED WINDS 140 KT WITH GUSTS TO 175 KT.`) and
  `min_central_pressure` containing `915 MB` (from
  `ESTIMATED MINIMUM CENTRAL PRESSURE  915 MB`). It MUST handle the padded
  whitespace (`915 MB` parses despite double spaces upstream).
- **C6 — TCD body.** `advisory AL142024 --type tcd` on `milton.discus.txt` MUST
  return a non-empty discussion body and MUST recognize the terminator
  (`$$` / `Forecaster Pasch` / `NNNN`) — the parsed body MUST NOT include the
  `NNNN` trailer.
- **C7 — Helene cross-check.** `advisory AL092024 --type tcp` on
  `helene.public.txt` MUST parse `atcf_id == AL092024` (proving PIL `MIATCPAT4`
  reuse across storms is handled — same PIL, different storm).
- **C8 — raw passthrough.** `--raw` MUST emit the `<pre>` text beginning at the
  product header and MUST NOT contain HTML tags like `<pre>` or `&amp;`.

## D. `outlook` — TWO (source: two-atl.txt, two-epac.txt, two-cpac.txt)

Fixtures: `fixtures/two-atl.txt`, `fixtures/two/two-epac.txt`,
`fixtures/two/two-cpac.txt`.

- **D1 — Atlantic formation chances.** `outlook --basin atl` on `two-atl.txt`
  MUST extract `data.areas[0].formation_48h.percent == 60` and
  `data.areas[0].formation_7d.percent == 60` with
  `data.areas[0].formation_48h.level == "medium"` (real lines
  `* Formation chance through 48 hours...medium...60 percent.`).
- **D2 — issuance line.** Parsed `issued` MUST equal
  `200 AM EDT Tue Jun 16 2026`.
- **D3 — basin header recognition.** `two-atl.txt` header MUST be recognized as
  Atlantic (`ABNT20 KNHC` / `TWOAT`); `two-epac.txt` as E Pacific
  (`ABPZ20 KNHC` / `TWOEP`); `two-cpac.txt` as C Pacific
  (`ACPN50 PHFO` / `TWOCP`, issuing center "NWS Central Pacific Hurricane Center
  Honolulu HI").
- **D4 — graphic URL correctness (corrected codes).** `outlook --basin atl
  --graphics` MUST emit `two_2d == https://www.nhc.noaa.gov/xgtwo/two_atl_2d0.png`.
  `--basin ep` MUST emit `two_pac_2d0.png` (NOT `two_epac_2d0.png`).
  `--basin cp` MUST emit `two_cpac_2d0.png`. No emitted graphic URL may contain
  the substring `two_epac`.
- **D5 — corrected C Pacific text URL.** The source URL the CLI would fetch for
  `--basin cp` MUST be `HFOTWOCP.shtml`, NOT `MIATWOCP.shtml`.

## E. `alerts` — active tropical (source: milton FL snapshot + empty fixtures)

- **E1 — quiet/empty contract.** Against
  `fixtures/alerts/empty_hurricane_warning_2026-06-15.json`, `alerts` MUST exit
  `0` and emit `count == 0`, `alerts == []` (NOT an error, NOT null).
- **E2 — feature parse.** Against
  `fixtures/alerts/milton_hurricane_warning_feature_2024-10-09.json`, parsing
  that single feature MUST yield:
  `event == "Hurricane Warning"`, `severity == "Extreme"`,
  `areaDesc == "Coastal Hillsborough"`, `geometry_type == "MultiPolygon"`,
  `headline` containing `NWS Tampa Bay Ruskin FL`.
- **E3 — instruction nullable.** That feature's `instruction` MUST be `null`
  (present-but-null, the verified tropical contract).
- **E4 — rollup over the full snapshot.** Against
  `fixtures/alerts/milton_2024-10-09_FL_active.json` (204 FL features), an
  `alerts --area FL` run MUST produce `by_event` counts of at least:
  Hurricane Warning `51`, Tropical Storm Warning `46`, Storm Surge Warning `23`,
  Hurricane Watch `14`. (These are the verified counts.)
- **E5 — statements opt-in.** Default `alerts` MUST NOT include
  `Tropical Cyclone Statement`; `alerts --statements` MUST include it (8 present
  in the snapshot).

## F. `graphics <id>` (source: helene-2024.json)

- **F1 — cone/track/surge links.** `graphics al092024` MUST emit:
  - `cone_kmz` = `…/AL092024_016Aadv_CONE.kmz`
  - `track_kmz` = `…/AL092024_016Aadv_TRACK.kmz`
  - `peak_surge_kml` = `…/gis/peakSurge/AL092024_PeakStormSurge_016Aadv.kml`
  - `wind_radii_kmz` = `…/AL092024_forecastradii_016Aadv.kmz`
- **F2 — landing page is HTML, not PNG.** `forecast_graphics_page` MUST equal
  `…/graphics_at4.shtml` and MUST NOT end in `.png`/`.gif`/`.jpg` (no per-storm
  raw image exists).
- **F3 — null surge for Isaac.** `graphics al102024` MUST emit
  `peak_surge_kml == null` (Isaac had no surge product).

## G. `gis <id>` — MapServer link-out (source: NHC_tropical_weather_MapServer.json)

Fixture: `fixtures/mapserver/NHC_tropical_weather_MapServer.json`

- **G1 — link-out only.** `gis` output MUST contain ArcGIS REST layer URLs under
  the `MapServer/` service and MUST NOT contain any parsed map geometry/feature
  data (it is a link-out, not ingested).
- **G2 — AT1 slot layers.** For an Atlantic storm in slot AT1, output MUST
  reference layer id `8` "AT1 Forecast Cone", id `9` "AT1 Watch-Warning", id `17`
  "AT1 Advisory Wind Field" (verified layer names in the fixture).
- **G3 — service URL.** `service` MUST equal
  `https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer`.

## H. `brief` — compound + the quiet-season contract (the headline empty test)

- **H1 — quiet season (the canonical empty + outlook pointer).** With
  `storms` sourced from `fixtures/currentstorms/live-empty.json` and `alerts`
  from `fixtures/alerts/empty_hurricane_warning_2026-06-15.json` and `outlook`
  from `fixtures/two-atl.txt`, `brief --basin atl` MUST:
  - exit `0`,
  - emit `data.storms.count == 0` and `data.alerts.count == 0`,
  - STILL emit `data.outlook.atl.areas[0].formation_7d.percent == 60` (outlook
    carries value when zero storms),
  - emit a `summary` string that mentions "0 active storms" AND points to the
    outlook formation odds.
  This is the "zero active storms MUST return clean empty + point to outlook"
  requirement.
- **H2 — empty is not an error.** `live-empty.json` (`{"activeStorms": []}`)
  through `storms` MUST yield `count == 0`, exit `0`, and an `outlook_hint`
  field — never a non-zero exit, never a stack trace, never a fabricated storm.

## I. Cross-cutting invariants

- **I1 — JSON default.** Every command with no `--format` flag MUST emit valid
  JSON parseable by `jq` (envelope `{source, fetched_at, data}`).
- **I2 — human mode.** `--format human` (and `--md`) MUST emit non-JSON text and
  MUST still include the headline fact (e.g. storm name + cone URL).
- **I3 — User-Agent.** Live (non-fixture) calls MUST send
  `nhc-pp-cli/0.1 (github.com/abe238/nhc-pp-cli)`. (Regression
  guard: api.weather.gov returns 403 without it.)
- **I4 — no `limit` on active alerts.** The CLI MUST NOT send `limit` to
  `/alerts/active` (would 400); if a limit is desired it is applied client-side.
- **I5 — corrected URL strings nowhere regress.** No generated URL anywhere may
  contain `MIATWOCP.shtml` or `two_epac` (both are the wrong task-example forms).
