# nhc-pp — Agent-Native Command Design

Design philosophy: **muscle memory for agents.** Token-efficient, JSON-first,
compound commands that answer a whole question in one call, link-outs instead of
binary blobs. Every command maps to a verified source in
`nhc-data-sources.md`; field names below come from the real fixtures.

## Global conventions

- **`--json` is the default** (machine output). `--format human` (alias
  `--md`/`--markdown`) renders a compact human/markdown view. No other output
  modes.
- **`--basin atl|ep|cp`** is the one clean basin token everywhere. The CLI maps
  it internally to the per-product code mess (text `MIA/HFO`, graphics
  `atl/pac/cpac`, RSS `atlc/epac/cpac`). Users never type those.
- **Storm id** accepts the ATCF form case-insensitively: `al092024` / `AL092024`,
  and a loose `helene` name match (resolved against the active feed).
- **User-Agent** is sent on every request:
  `nhc-pp-cli/0.1 (github.com/abe238/nhc-pp-cli)`. Mandatory for
  `api.weather.gov` (403 without).
- **`--fixture <path>` (and stdin)** reads a saved payload from disk instead of
  fetching, for testing/offline use. Every command MUST accept it; this is the
  hook the acceptance tests exercise (live data is empty off-season). When
  `--fixture` is given the CLI parses the file with the same code path as a live
  response and sets `source` to the fixture path. Reading from stdin (e.g.
  `cat file | nhc-pp storms -`) behaves identically.
- **Exit codes:** `0` success (including a clean empty result), `3` not-found
  (unknown storm id / advisory number), `4` upstream HTTP error, `2` usage error.
- **Empty is not an error.** Zero active storms → HTTP 200 → exit 0 → a clean
  empty envelope **plus an outlook pointer** (see Empty handling). Never a stack
  trace, never a fabricated storm.
- **Never fabricate absent fields.** `wind history`, `key messages`, and per-storm
  raw image URLs are not in the feed; no command surfaces them.
- **Link-out, don't download.** GIS/KMZ/KML/ZIP and the MapServer are emitted as
  URLs (`links` object), never fetched into the payload.

Every JSON payload carries a small envelope:

```json
{ "source": "<url fetched>", "fetched_at": "<ISO8601>", "data": { ... } }
```

---

## Command surface (summary)

| Command | Purpose | One-call value |
|---|---|---|
| `storms` | List active storms | Every active storm + key vitals + link-outs in one call |
| `storm <id>` | Full detail for one storm | Vitals + all product URLs + all GIS link-outs, nulls handled |
| `advisory <id> --type tcp\|tcd\|tcm` | Fetch + parse a text product | Parsed fields + raw text, no HTML scraping by the agent |
| `outlook --basin atl\|ep\|cp` | Tropical Weather Outlook | Parsed TWO text + formation chances + graphic link-outs |
| `alerts` | Active tropical NWS alerts | All 6 watch/warning types in ONE api.weather.gov call |
| `graphics <id>` | Per-storm graphic/GIS link-outs | Cone, track, surge, wind-radii URLs without parsing |
| `gis <id>` | MapServer layer link-outs | Cite-able ArcGIS layer URLs (link-out only) |
| `brief` | Compound situational report | storms + outlook + alerts in a single command |

---

## `storms` — list active storms

- **Purpose:** the default "what's happening" call.
- **Source:** `CurrentStorms.json` (#1).
- **Inputs:** `[--basin atl|ep|cp]` (filter), `--format`.
- **Output JSON `data`:**

```json
{
  "count": 3,
  "storms": [
    {
      "id": "al092024", "name": "Helene", "binNumber": "AT4",
      "classification": "TS", "intensity_kt": 50, "pressure_mb": 972,
      "lat": 34.2, "lon": -83.0, "movementDir": 360, "movementSpeed_kt": 30,
      "lastUpdate": "2024-09-27T12:00:00.000Z",
      "links": {
        "publicAdvisory": "https://www.nhc.noaa.gov/text/MIATCPAT4.shtml",
        "forecastGraphics": "https://www.nhc.noaa.gov/graphics_at4.shtml",
        "cone_kmz": "https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz"
      }
    }
  ]
}
```

- **Compound value:** one call returns all 3 storms with vitals + the three most
  useful link-outs each. (`intensity`/`pressure` are coerced from the feed's
  strings to integers; raw strings preserved are available via `storm <id>`.)
- **Empty:** `{"count":0,"storms":[],"outlook_hint":"run `nhc-pp outlook --basin atl`"}`.

## `storm <id>` — full detail for one storm

- **Source:** `CurrentStorms.json` (#1), single storm.
- **Inputs:** `<id>` (required, ATCF or name), `--format`.
- **Output JSON `data`:** all 30 storm keys, normalized. Scalars promoted
  (typed), plus a flat `links` block of every non-null product URL and GIS
  link-out:

```json
{
  "id": "al092024", "name": "Helene", "classification": "TS",
  "intensity_kt": 50, "pressure_mb": 972, "lat": 34.2, "lon": -83.0,
  "products": {
    "publicAdvisory":     {"advNum":"016A","url":"…/text/MIATCPAT4.shtml"},
    "forecastAdvisory":   {"advNum":"016", "url":"…/text/MIATCMAT4.shtml"},
    "forecastDiscussion": {"advNum":"016", "url":"…/text/MIATCDAT4.shtml"},
    "windSpeedProbabilities": {"advNum":"016","url":"…/text/MIAPWSAT4.shtml"}
  },
  "gis": {
    "trackCone":        {"kmz":"…/AL092024_016Aadv_CONE.kmz"},
    "forecastTrack":    {"kmz":"…/AL092024_016Aadv_TRACK.kmz"},
    "windWatchesWarnings": null,
    "peakSurge":        null
  }
}
```

- **Null handling (verified contract):** Isaac (`al102024`) has
  `windWatchesWarnings`/`stormSurgeWatchWarningGIS`/`potentialStormSurgeFloodingGIS`/`peakSurgeKML`
  = null. The CLI emits those keys as JSON `null` (present-but-null), never omits
  silently, never fakes a URL.
- **Compound value:** one call = vitals + every advisory product URL + every GIS
  link-out, so an agent never has to re-derive `AL092024_…_CONE.kmz` patterns.
- **Not-found:** unknown id → exit 3, `{"error":"storm not found","id":"…","active":[…ids…]}`.

## `advisory <id> --type tcp|tcd|tcm` — fetch + parse a text product

- **Purpose:** hand the agent a parsed advisory instead of raw HTML.
- **Source:** advisory archive (#2-4), `.shtml` `<pre>` extraction.
- **Inputs:**
  - `<id>` (ATCF/name).
  - `--type tcp|tcd|tcm` (default `tcp`). `tcp`=public, `tcd`=discussion,
    `tcm`=forecast/marine (the `fstadv` product). `--type pws` optional
    (windSpeedProbabilities text).
  - `--number NNN` (advisory number; default = latest from the active feed's
    `advNum`). Accepts the `_a` intermediate suffix.
  - `--raw` (emit only the extracted `<pre>` text), `--format`.
- **Output JSON `data`:**

```json
{
  "atcf_id": "AL142024", "storm": "Milton", "type": "tcp",
  "advisory_number": "16", "issued": "1000 PM CDT Tue Oct 08 2024",
  "issued_utc": "2024-10-09T03:00:00Z",
  "fields": {
    "location": "23.4N 86.5W",
    "max_sustained_winds": "160 MPH...260 KM/H",
    "present_movement": "NE OR 55 DEGREES AT 12 MPH...19 KM/H",
    "min_central_pressure": "915 MB...27.02 INCHES"
  },
  "sections": { "watches_warnings": "…", "hazards": "…", "next_advisory": "…" },
  "raw": "<full pre text>"
}
```

- **Per-product extractors (required, NOT one regex):** TCP is MPH-primary,
  dot-delimited (`MAXIMUM SUSTAINED WINDS...`); TCM is KT, space-delimited,
  ALL-CAPS (`MAX SUSTAINED WINDS 140 KT WITH GUSTS TO 175 KT.`). TCD has no
  vitals block — emit title + discussion body + forecast period table.
- **Storm identity from the body ATCF id** (`AL142024`), never the AWIPS PIL
  (reused across storms). **Issuance time from line ~6**, never the `TTAA00 …
  DDHHMM` placeholder line.
- **Time zones:** TCP/TCD local (CDT/EDT), TCM UTC — `issued` keeps the original
  string, `issued_utc` is the normalized value (product-aware tz).
- **Compound value:** one call does fetch + `<pre>` extract + entity decode +
  field parse + section split, so the agent gets structured data, not HTML.

## `outlook --basin atl|ep|cp` — Tropical Weather Outlook

- **Purpose:** the always-on "is anything brewing" answer (carries value even
  with zero named storms).
- **Source:** TWO text (#6). Maps `atl→MIATWOAT`, `ep→MIATWOEP`,
  `cp→HFOTWOCP` (corrected; `MIATWOCP` 404s).
- **Inputs:** `--basin atl|ep|cp` (default `atl`), `--graphics` (also emit PNG
  link-outs), `--format`.
- **Output JSON `data`:**

```json
{
  "basin": "atl", "issued": "200 AM EDT Tue Jun 16 2026",
  "areas": [
    {"name":"Northwestern Gulf of America (AL90)",
     "formation_48h":{"level":"medium","percent":60},
     "formation_7d":{"level":"medium","percent":60},
     "text":"A trough of low pressure…"}
  ],
  "graphics": {
    "two_2d": "https://www.nhc.noaa.gov/xgtwo/two_atl_2d0.png",
    "two_7d": "https://www.nhc.noaa.gov/xgtwo/two_atl_7d0.png"
  }
}
```

- **Graphics basin code mapping (corrected):** `atl→atl`, `ep→pac` (NOT `epac`),
  `cp→cpac`. Verified PNG endpoints.
- **Parsing:** strip the `en Espanol` anchor; product runs `000`→`$$`; pull the
  two `* Formation chance through …` lines (verified format
  `…48 hours...medium...60 percent.`).
- **Compound value:** one call = parsed outlook + formation odds + both graphic
  link-outs, in any basin.

## `alerts` — active tropical NWS alerts

- **Purpose:** all live watches/warnings in one call.
- **Source:** `api.weather.gov/alerts/active` (#5).
- **Inputs:** `[--area FL]` (state filter), `--statements` (also include
  `Tropical Cyclone Statement`), `--format`.
- **Default single call (comma-list = OR, verified):**

```
?event=Hurricane Warning,Hurricane Watch,Tropical Storm Warning,Tropical Storm Watch,Storm Surge Warning,Storm Surge Watch
```

- **Output JSON `data`:**

```json
{
  "count": 1,
  "alerts": [
    {"id":"…","event":"Hurricane Warning","severity":"Extreme",
     "areaDesc":"Coastal Hillsborough",
     "headline":"Hurricane Warning issued October 9 at 2:10PM EDT by NWS Tampa Bay Ruskin FL",
     "instruction": null, "geometry_type":"MultiPolygon",
     "effective":"…","expires":"…"}
  ],
  "by_event": {"Hurricane Warning":1}
}
```

- **Nullable fields (verified):** `instruction` was null on every tropical alert
  in the Milton snapshot; emit as `null`. `geometry` can be null/Polygon/MultiPolygon.
- **Do NOT use `limit`** on `/alerts/active` (HTTP 400). UA mandatory.
- **Compound value:** all 6 tropical product types in ONE request, with a
  `by_event` rollup — no 6-call fan-out, no client-side filtering.
- **Empty:** `{"count":0,"alerts":[]}` exit 0 (the verified quiet contract).

## `graphics <id>` — per-storm graphic / GIS link-outs

- **Purpose:** hand back the cone/track/surge/wind URLs without the agent parsing
  the feed.
- **Source:** `CurrentStorms.json` GIS sub-objects (#1).
- **Inputs:** `<id>`, `[--kind cone|track|surge|wind|all]` (default `all`),
  `--format`.
- **Output JSON `data`:**

```json
{
  "id":"al092024","name":"Helene",
  "links":{
    "cone_kmz":"…/AL092024_016Aadv_CONE.kmz",
    "track_kmz":"…/AL092024_016Aadv_TRACK.kmz",
    "forecast_track_zip":"…/gis/forecast/archive/al092024_5day_016A.zip",
    "wind_radii_kmz":"…/AL092024_forecastradii_016Aadv.kmz",
    "wsp_34kt_kmz":"…/2024092706_wsp34knt120hr_5km.kmz",
    "peak_surge_kml":"…/gis/peakSurge/AL092024_PeakStormSurge_016Aadv.kml",
    "watch_warning_kmz":"…/AL092024_016Aadv_WW.kmz",
    "forecast_graphics_page":"https://www.nhc.noaa.gov/graphics_at4.shtml"
  }
}
```

- **No per-storm PNG exists** — `forecast_graphics_page` is the HTML landing page.
  Document this so the agent doesn't expect an image.
- **Null kinds emitted as null** (e.g. Isaac's `peak_surge_kml: null`).
- **Compound value:** every link-out for one storm in one call.

## `gis <id>` — MapServer layer link-outs (link-out only)

- **Purpose:** cite-able ArcGIS REST layer URLs for mapping clients.
- **Source:** MapServer (#9) — **never ingested**, only referenced.
- **Inputs:** `<id>` (resolves to a per-storm slot AT1-AT5/EP1-EP5/CP1-CP5),
  `--format`.
- **Output JSON `data`:** layer ids + names + REST URLs, e.g.

```json
{
  "service":"https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer",
  "layers":[
    {"id":8,"name":"AT1 Forecast Cone","url":"…/MapServer/8"},
    {"id":9,"name":"AT1 Watch-Warning","url":"…/MapServer/9"},
    {"id":17,"name":"AT1 Advisory Wind Field","url":"…/MapServer/17"}
  ]
}
```

- **Compound value:** maps a storm to its ArcGIS slot layers in one call.
- Off-season, per-storm slots are empty; the outlook group (ids 0,1,2,3,398,399)
  and Probabilistic Winds (394-397) are the populated layers.

## `brief` — compound situational report

- **Purpose:** the single command that answers "what's the tropical situation?"
- **Inputs:** `[--basin atl|ep|cp]` (default all three), `--format`.
- **Behavior:** fans out to `storms` + `outlook` (per requested basin) + `alerts`
  and merges into one envelope:

```json
{
  "data": {
    "storms": { "count": 0, "storms": [] },
    "outlook": { "atl": {…}, "ep": {…}, "cp": {…} },
    "alerts": { "count": 0, "alerts": [] },
    "summary": "0 active storms; Atlantic 7-day formation: 60% (AL90); 0 active tropical alerts."
  }
}
```

- **Compound value:** the flagship one-call answer — active storms + outlook
  (formation odds) + active alerts, with a one-line `summary`.
- **Empty/quiet:** all three sub-blocks return their clean empty contracts; the
  `summary` still carries the outlook formation odds, so `brief` is useful even in
  the dead of off-season. This is the canonical "zero active storms → point to
  outlook" path.

---

## Error / empty handling matrix

| Situation | HTTP | Exit | Output |
|---|---|---|---|
| Zero active storms | 200 | 0 | `{count:0, storms:[]}` + `outlook_hint` |
| Zero active alerts | 200 | 0 | `{count:0, alerts:[]}` (verified empty contract) |
| Unknown storm id | n/a | 3 | `{error:"storm not found", active:[…]}` |
| Unknown advisory number | 404 | 3 | `{error:"advisory not found"}` |
| Bad basin token | n/a | 2 | usage error |
| Upstream 5xx / 403 | 4xx/5xx | 4 | `{error:"upstream", status:NNN, url:…}` |
| Missing UA on api.weather.gov | 403 | 4 | (CLI always sends UA, so this is prevented) |

## Notes baked in from the verified findings

- **Task example flag list (`--type tcp|tcd`) was incomplete:** `tcm` (the
  `fstadv` product) and optional `pws` are supported.
- **Two task URL examples were wrong and are corrected in the CLI:**
  C Pacific TWO `HFOTWOCP.shtml` (not `MIATWOCP`); E Pacific graphic `two_pac_*.png`
  (not `two_epac_*`).
- **`milton-2024.json` does not exist** (Milton/Beryl never archived in
  CurrentStorms.json). Storm-list/detail behavior is anchored to `helene-2024.json`;
  advisory parsing is anchored to the real `milton.*.txt` fixtures.
- **Absent fields are absent:** no command promises `wind history`,
  `key messages`, or per-storm raw image URLs.
