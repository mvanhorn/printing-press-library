# NHC Data Sources — Verified Catalog

Every claim below is grounded in real HTTP captures and the fixtures saved under
`/Users/abediaz/ghostex/chats/nhc-pp/fixtures/`. Status codes, byte counts, field
names, and URLs are observed, not inferred. Fetches used
`User-Agent: nhc-pp-cli/0.1 (github.com/abe238/nhc-pp-cli)`.

## Quick reference table

| # | Source | URL / pattern | Status (observed) | Content-Type | Auth (UA) | Format | Quiet-season behavior |
|---|--------|---------------|-------------------|--------------|-----------|--------|-----------------------|
| 1 | Active storms feed | `https://www.nhc.noaa.gov/CurrentStorms.json` | 200 | application/json | UA recommended | JSON | `{"activeStorms": []}` (24 bytes) |
| 2 | Public advisory (TCP) | `…/archive/<yyyy>/<basinNN>/<id>.public.NNN.shtml` (+ `?text`) | 200 | text/html | UA recommended | HTML `<pre>` | n/a (archive, on-demand) |
| 3 | Forecast discussion (TCD) | `…/archive/<yyyy>/<basinNN>/<id>.discus.NNN.shtml` | 200 | text/html | UA recommended | HTML `<pre>` | n/a |
| 4 | Forecast/marine advisory (TCM) | `…/archive/<yyyy>/<basinNN>/<id>.fstadv.NNN.shtml` | 200 | text/html | UA recommended | HTML `<pre>` | n/a |
| 5 | Active NWS alerts | `https://api.weather.gov/alerts/active?event=<list>` | 200 (403 w/o UA) | application/geo+json | **UA MANDATORY** | GeoJSON | `features: []`, HTTP 200 |
| 6 | Tropical Weather Outlook text | `…/text/MIATWOAT.shtml` / `MIATWOEP.shtml` / `HFOTWOCP.shtml` | 200 | text/html | UA recommended | HTML `<pre>` | always-on; carries value when 0 storms |
| 7 | TWO graphics (PNG) | `…/xgtwo/two_<atl\|pac\|cpac>_<2d0\|7d0>.png` | 200 | image/png | UA recommended | PNG (link-out) | always-on |
| 8 | RSS index feeds | `…/index-at.xml` / `index-ep.xml` / `index-cp.xml` | 200 | text/xml | UA recommended | RSS 2.0 | TWO item + "no tropical cyclones" placeholder |
| 9 | GIS MapServer | `https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer` | 200 | application/json | UA recommended | ArcGIS REST (**link-out only**) | GTWO layers populated; per-storm slots empty |

---

## 1. Active storms feed — CurrentStorms.json

- **URL:** `https://www.nhc.noaa.gov/CurrentStorms.json`
- **Status:** HTTP 200, `application/json`. Live (2026-06-15): 24 bytes.
- **Quiet season:** payload is literally `{"activeStorms": []}` (verified: `live-empty.json`, `CurrentStorms-empty.json`).
- **Cadence:** advisory-driven; updates each cycle (~6 h) plus intermediate
  advisories. NHC pages route through CloudFront with `cache-control max-age=60`,
  so expect ~1 min freshness latency.

### Payload shape (real field names)

Top level: single key `activeStorms` (array). Each storm object has **exactly 30
keys** (verified by enumeration on `helene-2024.json`).

Scalar fields (real Helene values, `helene-2024.json`):

```
id              "al092024"     basin+number+year, lowercase (ATCF AL092024 appears UPPERCASE inside URLs)
binNumber       "AT4"          Atlantic bin; Isaac "AT5"; John "EP5" (East Pacific)
name            "Helene"
classification  "TS"           HU=hurricane, TS=tropical storm (Isaac was "HU")
intensity       "50"           kt, STRING (Isaac "65", John "55")
pressure        "972"          mb, STRING (Isaac "981", John "988")
latitude        "34.2N"        STRING with hemisphere suffix
longitude       "83.0W"
latitudeNumeric  34.2          FLOAT (W/S negative)
longitudeNumeric -83.0
movementDir      360           INTEGER degrees
movementSpeed    30            INTEGER kt
lastUpdate      "2024-09-27T12:00:00.000Z"   ISO-8601 UTC
```

Nested **product** objects `{advNum, issuance, fileUpdateTime, url}` — `url` is a `.shtml`:

```
publicAdvisory          url  https://www.nhc.noaa.gov/text/MIATCPAT4.shtml   (advNum "016A")
forecastAdvisory        url  …/text/MIATCMAT4.shtml
windSpeedProbabilities  url  …/text/MIAPWSAT4.shtml
forecastDiscussion      url  …/text/MIATCDAT4.shtml
forecastGraphics        url  …/graphics_at4.shtml   (HTML landing page, NOT an image)
```

Nested **GIS** objects (`{advNum, issuance, fileUpdateTime, zipFile, kmzFile}`) — real Helene URLs:

```
forecastTrack         kmzFile …/storm_graphics/api/AL092024_016Aadv_TRACK.kmz
trackCone             kmzFile …/storm_graphics/api/AL092024_016Aadv_CONE.kmz      <- forecast cone
windWatchesWarnings   kmzFile …/storm_graphics/api/AL092024_016Aadv_WW.kmz        (null when no watches)
initialWindExtent     kmzFile …/storm_graphics/api/AL092024_initialradii_016Aadv.kmz
forecastWindRadiiGIS  kmzFile …/storm_graphics/api/AL092024_forecastradii_016Aadv.kmz
```

GIS variants with different sub-keys (real Helene URLs):

```
bestTrackGIS                   {issuance,fileUpdateTime,zipFile,kmzFile}  (NO advNum) …/gis/best_track/al092024_best_track.kmz
earliestArrivalTimeTSWindsGIS  {advNum,issuance,fileUpdateTime,kmzFile}   …/AL092024_016adv_earliest_reasonable_toa_34.kmz
mostLikelyTimeTSWindsGIS       {advNum,issuance,fileUpdateTime,kmzFile}   …/AL092024_016adv_most_likely_toa_34.kmz
windSpeedProbabilitiesGIS      {issuance,fileUpdateTime,zipFile5km,zipFile0p5deg,kmzFile34kt,kmzFile50kt,kmzFile64kt}
                                                                          …/2024092706_wsp34knt120hr_5km.kmz
stormSurgeWatchWarningGIS      {advNum,issuance,fileUpdateTime,kmlFile}   …/AL092024_WatchWarningSS_016adv.kml   (null when n/a)
potentialStormSurgeFloodingGIS {advNum,issuance,fileUpdateTime,zipFile,zipFileTidalMask} …/AL0924_16_inundation.zip (null when n/a)
peakSurgeKML                   {advNum,issuance,fileUpdateTime,peakSurgeKMLFile} …/gis/peakSurge/AL092024_PeakStormSurge_016Aadv.kml (null when n/a)
```

### Nullability contract (observed, not inferred)

Any nested product/GIS object can be **null** when not applicable. Verified in
`helene-2024.json`:

- **Isaac** (`al102024`, mid-Atlantic): `windWatchesWarnings`,
  `stormSurgeWatchWarningGIS`, `potentialStormSurgeFloodingGIS`, `peakSurgeKML`
  are all **null**. `trackCone` and `forecastGraphics` present
  (`graphics_at5.shtml`, `AL102024_006adv_CONE.kmz`).
- **John** (`ep102024`): `stormSurgeWatchWarningGIS`, `peakSurgeKML` null;
  `windWatchesWarnings` present.

The CLI must handle **null sub-objects**, not just empty. Never assume a field is
present.

### Fields the task asked for that DO NOT exist in this feed

- **`wind history`** — no `windHistory` key (key enumeration = 30 keys, absent).
- **`key messages`** — no `keyMessages` key (absent).
- **Raw per-storm image URLs** (`.png`/`.gif`/`.jpg`) — none. Graphics are GIS
  files (`.kmz`/`.zip`/`.kml`) plus `.shtml` landing pages. `forecastGraphics.url`
  is an HTML page, not an image.

Do not fabricate these. (The schema is anchored to the 2024-09-27 Helene capture;
later seasons may differ.)

### Task → key mapping

| Task asks for | Actual key |
|---|---|
| forecast cone | `trackCone` |
| wind speed probabilities (text) | `windSpeedProbabilities` |
| wind speed probabilities (GIS) | `windSpeedProbabilitiesGIS` |
| arrival of TS winds | `earliestArrivalTimeTSWindsGIS` + `mostLikelyTimeTSWindsGIS` |
| peak storm surge | `peakSurgeKML` (+ `potentialStormSurgeFloodingGIS`) |
| key messages | ABSENT |
| wind history | ABSENT |

### Archival note

Wayback captured this exact URL only 6 times in all of 2024 (full-year CDX): 4
empty + 2 non-empty, both **2024-09-27 (Hurricane Helene)**. **No Milton (Oct
2024) or Beryl (Jul 2024) snapshot of CurrentStorms.json exists.** The
`helene-2024.json` fixture (13:04Z, advNum 016A, surge fields populated) is the
primary non-empty contract; `helene-2024-earlier.json` (10:38Z) is the same 3
storms one cycle earlier.

---

## 2-4. Advisory products (TCP / TCD / TCM) — archive

- **URL pattern:** `https://www.nhc.noaa.gov/archive/<yyyy>/<basinNN>/<id>.<kind>.NNN.shtml`
  - `kind`: `public` (TCP), `discus` (TCD), `fstadv` (TCM/forecast).
  - `.public_a.NNN.shtml` = intermediate/special public advisory (the `_a` suffix
    exists; saw it in the Milton index).
- **Status:** all HTTP 200, `text/html; charset=UTF-8`. Verified: Milton AL14
  #16 (public/discus/fstadv), Helene AL09 #14 (public/discus).
- **Lighter variant:** `?text` returns the same single `<pre>` body, chrome
  stripped (~14.7 KB vs 31.9 KB for Milton TCP). Same extraction logic.

### Body extraction

Product text lives in **exactly one `<pre>` block** per `.shtml`, wrapped as
`<div class='textbackground'><div class='textproduct'><pre>…</pre></div></div>`.
Extract with non-greedy `/<pre>(.*?)<\/pre>/s`, then decode `&amp; &lt; &gt;`
(plain-ASCII products). Verified: `perl -0777 -ne 'print $1 if /<pre>(.*?)<\/pre>/s'`.

### Header lines (first ~7 lines of `<pre>`, verified on `milton.public.txt`)

```
Line 1: ZCZC MIATCPAT4 ALL          AWIPS PIL. MIATC{P|D|M}AT4 -> P=TCP, D=TCD, M=TCM
Line 2: TTAA00 KNHC DDHHMM          WMO heading. UNSUBSTITUTED placeholder in archive — DO NOT parse time
Line ~4: Hurricane Milton Advisory Number  16     (TCM is ALL-CAPS FORECAST/ADVISORY; note double-space before number)
Line ~5: NWS National Hurricane Center Miami FL       AL142024    <- ATCF id, the reliable storm key
Line ~6: 1000 PM CDT Tue Oct 08 2024              issuance time, the line to actually use (TCM uses UTC)
```

**The AWIPS PIL (`MIATCPAT4`) is reused across storms in the same basin** — the
trailing "4" is a basin slot, NOT the storm number. Use the body ATCF id
(`AL092024` / `AL142024`) to identify the storm.

### Notable fields differ by product type (a single regex will NOT parse all three)

Verified line text:

```
              TCP (milton.public.txt)                         TCM (milton.fstadv.txt)
MAX WINDS     MAXIMUM SUSTAINED WINDS...160 MPH...260 KM/H    MAX SUSTAINED WINDS 140 KT WITH GUSTS TO 175 KT.
MOVEMENT      PRESENT MOVEMENT...NE OR 55 DEGREES AT 12 MPH   PRESENT MOVEMENT TOWARD THE NORTHEAST OR  55 DEGREES AT  10 KT
PRESSURE      MINIMUM CENTRAL PRESSURE...915 MB...27.02 IN    ESTIMATED MINIMUM CENTRAL PRESSURE  915 MB
LOCATION      LOCATION...23.4N 86.5W                          HURRICANE CENTER LOCATED NEAR 23.4N  86.5W AT 09/0300Z
```

TCP is MPH-primary with dot delimiters; TCM is KT, space-delimited, ALL-CAPS
title. **Times: TCP is storm-local (CDT/EDT), TCM is UTC** — tz handling must be
product-aware.

### TCP section anchors (header + dashed-underline rule)

`SUMMARY OF … INFORMATION`, `WATCHES AND WARNINGS`, `DISCUSSION AND OUTLOOK`,
`HAZARDS AFFECTING LAND` (STORM SURGE / RAINFALL / WIND / TORNADOES / SURF),
`NEXT ADVISORY`. Watches parse as `CHANGES WITH THIS ADVISORY:` →
`SUMMARY OF WATCHES AND WARNINGS IN EFFECT:` → per-hazard groups with `* `-prefixed area lines.

### Terminator (all products)

`$$` then `Forecaster <Name>` then `NNNN` (verified on `milton.discus.txt`:
`$$` / `Forecaster Pasch` / `NNNN`).

### Parsing cautions

Whitespace is irregular and significant: double-space before advisory numbers,
padded UTC (`AT  09/0300Z`), right-aligned ATCF ids with variable leading spaces.
Use flexible `\s+`, never fixed positions.

---

## 5. Active NWS alerts — api.weather.gov

- **URL:** `https://api.weather.gov/alerts/active?event=<comma-list>`
- **Status:** HTTP 200, `application/geo+json` **with UA**. **Without UA → HTTP
  403** Akamai "Access Denied" HTML. **UA is MANDATORY on every call.**
- **Single-call tropical filter** (comma list = OR, verified to filter correctly):

```
https://api.weather.gov/alerts/active?event=Hurricane%20Warning,Hurricane%20Watch,Tropical%20Storm%20Warning,Tropical%20Storm%20Watch,Storm%20Surge%20Warning,Storm%20Surge%20Watch
```

  Proof: `?event=Flood%20Warning,Heat%20Advisory` returned exactly 47+20=67
  features. No client-side filtering or 6 separate calls needed.

### Empty contract (verbatim, `empty_hurricane_warning_2026-06-15.json`)

```json
{"@context":{"@version":"1.1"},"type":"FeatureCollection","features":[],"title":"Current Hurricane Warning events","updated":"2026-06-16T05:41:25+00:00"}
```

Always `type:FeatureCollection`, HTTP 200 even with zero alerts. `features` is an
empty array (never null/missing). An unknown event value is NOT rejected (no enum
validation). All 6 tropical filters returned 0 features today (no active system) —
the expected quiet contract.

### GeoJSON feature shape

Top-level: `id`, `type` ("Feature"), `geometry`, `properties`. Key
`properties`: `event`, `headline`, `areaDesc`, `severity`, `description`,
`instruction` (**nullable**), plus `effective/onset/expires/ends`, `sender`,
`senderName`, `affectedZones`, `geocode`, `references`, `web`, etc.

Real Milton Hurricane Warning (`milton_hurricane_warning_feature_2024-10-09.json`):

```
event       "Hurricane Warning"
severity    "Extreme"
headline    "Hurricane Warning issued October 9 at 2:10PM EDT by NWS Tampa Bay Ruskin FL"
areaDesc    "Coastal Hillsborough"
instruction null            <- nullable; null on every tropical alert in the Milton snapshot
geometry    MultiPolygon
```

Live Flood Warning reference (`live_alert_feature_example_2026-06-15.json`):
`instruction` set ("Turn around, don't drown…"), geometry `Polygon`, severity
`Severe`. So `instruction` and `geometry` are both nullable.

### Gotchas

- `limit` is **rejected with HTTP 400 on `/alerts/active`** (valid only on
  `/alerts`).
- The `/alerts` archive accepts `start`/`end` ISO-8601 but **does not retain back
  to Oct 2024 Milton** (returned 0). Historical examples came from Wayback
  (`milton_2024-10-09_FL_active.json`, 204 FL features: 51 Hurricane Warning, 46
  TS Warning, 23 Storm Surge Warning, 14 Hurricane Watch). Wayback payloads are
  gzip-encoded.
- Maximal "everything tropical" could add `Tropical Cyclone Statement` (8 of 204
  in the Milton snapshot). Keep the 6-type list primary; append statements only
  if in scope.
- **Cadence:** real-time; alerts appear/expire continuously.

---

## 6. Tropical Weather Outlook (TWO) text — always-on, 3 basins

- **URLs (corrected; two task examples were wrong):**
  - Atlantic: `https://www.nhc.noaa.gov/text/MIATWOAT.shtml` (200)
  - E Pacific: `https://www.nhc.noaa.gov/text/MIATWOEP.shtml` (200)
  - C Pacific: `https://www.nhc.noaa.gov/text/HFOTWOCP.shtml` (200) — **NOT
    `MIATWOCP.shtml`, which 404s** (CPHC/PHFO issues it, not Miami).
- **No `.txt` form exists** (all 404). Must parse the `.shtml` `<pre>`.
- **Status:** 200, `text/html`.

### Extraction contract

Page has exactly one `<pre>` block; inside, the first child is an
`<b><a …>en Espanol</a></b>` anchor that must be stripped; product begins at
`000` and ends at `$$` + Forecaster line (CP also has trailing `NNNN`).

WMO/AWIPS headers (verified): Atlantic `ABNT20 KNHC` / `TWOAT`; E Pacific
`ABPZ20 KNHC` / `TWOEP`; C Pacific `ACPN50 PHFO` / `TWOCP`.

Each TWO ends with two formation lines (verified, `two-atl.txt`):

```
* Formation chance through 48 hours...medium...60 percent.
* Formation chance through 7 days...medium...60 percent.
```

The TWO is the always-on value carrier when zero named storms exist. (The
`two-atl.txt` fixture shows an active development area "Northwestern Gulf of
America (AL90)" at 60% — still the no-named-storm state.)

- **Cadence:** issued 4×/day (roughly 0200/0800/1400/2000 local), more often when
  systems are active. `cache-control max-age=60`.

---

## 7. TWO graphics (PNG link-out)

- **URL:** `https://www.nhc.noaa.gov/xgtwo/two_<code>_<2d0|7d0>.png`
- **Basin codes (corrected; note inconsistency vs text):** Atlantic `atl`,
  E Pacific `pac` (**NOT `epac`** — `two_epac_*.png` 404s), C Pacific `cpac`.
- **Status:** all 200, `image/png`, valid PNG magic `89504e470d0a1a0a`. Sizes
  observed: `two_atl_2d0.png` 209844 B, `two_atl_7d0.png` 68326 B,
  `two_pac_2d0.png` 148796 B, `two_cpac_2d0.png` 142762 B.
- These are **basin-level outlook** graphics (not per-storm). Treat as link-out.

---

## 8. RSS index feeds

- **URLs:** `https://www.nhc.noaa.gov/index-at.xml` / `index-ep.xml` /
  `index-cp.xml`. Status 200, `text/xml`, RSS 2.0.
- **Channel:** `title`, `link`, `description`, `language`, `pubDate`.
- **Quiet state (observed, `index-at.xml`): exactly 2 items** —
  1. **Tropical Weather Outlook** item whose `description` (CDATA) is the full TWO
     text with literal `<br />` breaks, link `gtwo.php?basin=<atlc|epac|cpac>`.
  2. Placeholder **"There are no tropical cyclones at this time."** with
     description "No tropical cyclones as of <pubDate>", link to NHC root.
- **Active state NOT observed** (off-season). Structural inference: per-storm
  advisory items replace the placeholder while the TWO item persists. **Re-verify
  during an active period.**
- **Cadence:** `pubDate` was within ~1 min of fetch; ~1 min freshness.
- Times are GMT/UTC (RFC 822 pubDate) or product-local in body text.

---

## 9. GIS MapServer — LINK-OUT ONLY (do not ingest)

- **URL:** `https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer`
  (`?f=json` → 200, `application/json`, 92075 B).
- `mapName` "Tropical Weather", **400 layers**.
- **Quiet-season populated layers** (the GTWO outlook group, verified):
  - id 0 Graphical Tropical Weather Outlook
  - id 1 Two-Day: Current Location
  - id 2 Seven-Day: Current Location
  - id 3 Seven-Day: Potential Development Region
  - id 398 Seven-Day: Development Motion
  - id 399 Seven-Day Outlook
- **Per-storm slots** pre-allocated AT1-AT5, EP1-EP5, CP1-CP5 (each a group:
  Forecast Cone, Forecast Track, Forecast Points, Watch-Warning, Advisory Wind
  Field, Past Track, Wind Radii, Arrival Time of TS Winds, Inundation/Tidal
  Mask), **empty until a storm is named**.
- Cite-worthy link-out layer ids (verified): 8 AT1 Forecast Cone, 9 AT1
  Watch-Warning, 17 AT1 Advisory Wind Field; global Probabilistic Winds group ids
  394 (group), 395 (34 kt), 396 (50 kt), 397 (64 kt).
- **The CLI links to this; it never parses it as a data source.**

---

## Cross-source gotchas (verified)

1. **UA is mandatory only on `api.weather.gov`** (403 without). NHC `.gov` pages
   served fine but UA is good citizenship.
2. **Basin codes are NOT consistent across products:** TWO text C Pacific = HFO/PHFO;
   TWO graphic C Pacific = `cpac`; E Pacific graphic = `pac` while text uses
   `MIA…TWOEP`; RSS `gtwo.php?basin=` uses `atlc`/`epac`/`cpac`. The CLI should
   expose clean `atl|ep|cp` tokens and map internally.
3. **AWIPS PIL is not a unique storm key** — use the body ATCF id.
4. **TTAA00 line 2 is an unsubstituted placeholder** in the archive — never parse
   issuance time from it.
5. **CurrentStorms nested objects are nullable** — handle null, not just empty.
6. **wind history, key messages, per-storm raw image URLs do not exist** in the
   feed — do not fabricate.
