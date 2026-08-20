# PDOK Location CLI — Novel Features Brainstorm

Full subagent output (audit trail). Survivors flow into the absorb manifest's
transcendence table; killed candidates stay here so future retros can see what
we rejected and why.

## Customer model

**Persona 1: Lotte, civic-tech data engineer at a Dutch municipality**

*Today (without this CLI):* Lotte has a recurring task to enrich a CSV of
citizen-reported incident locations (street address strings, sometimes with
typos) into clean BAG addresses with lat/lon + RD coordinates for the GIS
team. She has the Locatieserver `/free` docs open in one tab, a Jupyter
notebook with `requests` and a hand-rolled WKT parser in another, and a third
tab with the EPSG.io page to remember which CRS RD is. She cannot answer "for
these 50K rows, which ones failed to geocode and why" without a custom
error-logging loop.

*Weekly ritual:* Every Monday she ingests last week's incident exports,
geocodes them, and hands a GeoJSON to the GIS team. Friday she also runs a
"did this week's new addresses land in the right gemeente" sanity check.

*Frustration:* Batch geocoding a CSV is held together with notebook glue. She
re-writes the WKT parser, the score filter, the retry loop, and the RD↔WGS84
conversion every project. Nobody ships this as a binary.

**Persona 2: Sven, full-stack dev building a Dutch address autocomplete form**

*Today (without this CLI):* Sven is wiring a React form to PDOK's `/suggest` +
`/lookup` chain. He needs to test "given this user input, what id does
suggest return, and what's the full geometry the lookup yields?" — and he
wants to do it from the terminal while iterating, not via Postman. He has a
Postman collection and a `curl` cheat-sheet in his Notion. He cannot answer
"is this id the same address as the one I cached yesterday" without writing a
script.

*Weekly ritual:* Every sprint includes 2-3 sessions of "demo this form with
these 8 representative addresses." Sven also routinely needs to confirm the
geometry shape (POINT vs MULTIPOLYGON) returned for a given type before
wiring the map render.

*Frustration:* The suggest→lookup chain is two manual `curl` calls per
address, plus eyeballing the WKT to figure out what shape came back. He wants
one command that hands him the GeoJSON.

**Persona 3: Marieke, GIS analyst at a water board (waterschap)**

*Today (without this CLI):* Marieke works with field crews who send GPS
coordinates from a phone. She needs to map each point to the nearest
hectometer marker (for road-side incidents) or the parcel it falls on (for
water-infrastructure work). She uses QGIS interactively, but field crews send
batches via email and she does the lookup manually. She cannot answer "what
gemeente + provincie does this RD coordinate fall in" without loading a
shapefile.

*Weekly ritual:* Twice a week she processes field reports — reverse geocoding
a list of RD or WGS84 coords into "nearest address + gemeente + parcel"
tuples and pushing them into the asset-management system.

*Frustration:* QGIS is interactive-only. The Locatieserver `/reverse`
endpoint exists but returns one result per call, and she has no way to filter
by type or get the parcel and the address together without two calls.

**Persona 4: Joris, journalist / open-data hobbyist**

*Today (without this CLI):* Joris is investigating cadastral patterns
("which parcels in Amsterdam-Zuid changed hands last year"). He clicks around
BAG Viewer and PDOK Loket, copies cadastral aanduidingen into a spreadsheet,
and re-types them into the Locatieserver `/free` box. He cannot answer "for
this gemeente, list every parcel inside this bounding box" from the command
line.

*Weekly ritual:* Weekly investigations involve pulling 20-200 features for a
target gemeente / bbox / postcode range and joining them against a story
spreadsheet.

*Frustration:* The PDOK web tools are click-heavy and never let him save the
result as a clean CSV with proper headers. He wants a CLI that takes a bbox
and a postcode prefix and dumps a CSV.

## Candidates (pre-cut)

| # | Name | Command | Source | One-liner | Persona | Inline verdict |
|---|------|---------|--------|-----------|---------|----------------|
| C1 | Batch CSV geocode | `batch geocode <file.csv> --address-col street` | (a) Lotte, (b) batch workflow, (f) WKT-to-coord conversion built in | Geocode every row of a CSV, write out new CSV with lat/lon/RD/score/match-quality | Lotte | Keep |
| C2 | Suggest→lookup one-shot | `resolve <prefix>` | (a) Sven, (f) suggest→lookup chain as one transaction | One call that suggests then auto-lookups the top hit, returns full GeoJSON | Sven | Keep |
| C3 | Cross-source nearest | `nearest --lat --lon` (or `--rd-x --rd-y`) | (a) Marieke, (b) reverse + parcel join, (c) join across types | Single call returns nearest adres + nearest perceel + nearest hectometerpaal + gemeente containing point | Marieke | Keep |
| C4 | RD↔WGS84 convert | `convert rd-to-ll 121200 488000` / `convert ll-to-rd 4.76 52.64` | (b) RD coord system identity, (f) Dutch coord conversion primer | Pure-math coord conversion in the binary — no API call needed | Lotte, Marieke | Keep |
| C5 | Validate postcode | `validate-postcode 1012LG` | (b) Dutch postcode shape | Shape-valid + canonical form | Sven, Lotte | Keep, low score |
| C6 | Offline gemeente gazetteer | `gemeente get amsterdam`, `gemeente list --provincie zuid-holland` | (b), (c) | After one-time sync (342 rows), serve gemeente + provincie lookups offline | Lotte, Joris | Keep |
| C7 | Local FTS over cached lookups | `search "amsterdam centraal"` | (c), (f) | After N geocodes/lookups, search the local cache | Lotte, Sven | Keep |
| C8 | Bbox→CSV dump | `features in-bbox --bbox 4.8,52.3,4.9,52.4 --collections adres,perceel --csv` | (a) Joris, (b) Location API bbox identity | Pull all features inside a bbox across N collections, emit one wide CSV | Joris | Keep |
| C9 | Confidence-thresholded best match | `top <query> --min-score 7.0 --require-type adres` | (a) Lotte | Returns single best match if it clears the score bar | Lotte | Keep |
| C10 | Address diff / canonicalize | `canonicalize "Spuistr 23 A'dam"` | (a) Sven | Run free→lookup and return the canonical weergavenaam | Sven | Merge into C2 |
| C11 | Cadastral aanduiding parser | `perceel lookup --aanduiding "AMR03 N 1234"` | (b) BRK identity | Parse aanduiding into the Location-API perceel feature | Joris, Marieke | Keep |
| C12 | Gemeente hierarchy roll-up | `gemeente of-point --lat --lon` | (b), (c) | Which gemeente + provincie contains this point | Marieke | Keep |
| C13 | Hectometerpaal nearest | `hectometer nearest --lat --lon --road A2` | (b) NWB markers | Reverse-geocode type-locked to hectometerpaal | Marieke | Cut (sibling of C3) |
| C14 | WKT/GeoJSON converter | `convert wkt-to-geojson "POINT(...)"` | (f) WKT-to-GeoJSON built in | Pure-format converter | Sven, Lotte | Keep |
| C15 | Doctor / health check | `doctor` | (f) | Probe both base URLs | All | Drop (table stakes) |
| C16 | Watch suggest as you type | `suggest --watch` | (a) Sven | Streaming TTY mode | Sven | Cut (scope creep) |
| C17 | Map render preview | `preview <id> --open` | (a) Sven | Open a local HTML map | Sven | Cut (external dep) |
| C18 | Routing / nearest road segment | `route nearest-segment ...` | (b) NWB roads | Cut — routing isn't in either spec | — | Cut |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Batch CSV geocode | `batch geocode <file.csv> --address-col street [--out result.csv]` | 10/10 | hand-code | Reads CSV, hits Locatieserver `/free` per row with `cliutil.AdaptiveLimiter`, writes `geocode_cache` to SQLite, emits new CSV with `lat,lon,rd_x,rd_y,score,match_type,match_id,error` columns | Brief Top Workflow #1; Persona Lotte; competing tools lack batch |
| 2 | Suggest→lookup one-shot | `resolve <text> [--geojson]` | 10/10 | hand-code | Calls `/suggest` top-1, feeds id into `/lookup`, parses WKT geometry into GeoJSON, caches lookup in SQLite | Brief Top Workflow #2; Persona Sven; competitors don't combine |
| 3 | Cross-source nearest | `nearest --lat --lon` (or `--rd-x --rd-y`) | 10/10 | hand-code | Fans out `/reverse` across types `adres,perceel,hectometerpaal,weg`, joins with offline `gemeenten`/`provincies`, returns one record with all four neighbours + admin context | Brief Top Workflow #3; Persona Marieke |
| 4 | RD ↔ WGS84 convert | `convert rd-to-ll <x> <y>` / `convert ll-to-rd <lon> <lat>` | 9/10 | hand-code | Pure-math EPSG:28992 ↔ EPSG:4326 transform in Go, zero API calls | Brief Codebase Intelligence; Personas Lotte + Marieke; EPSG is a Dutch identity feature |
| 5 | WKT ↔ GeoJSON convert | `convert wkt-to-geojson <wkt>` / `convert geojson-to-wkt <stdin>` | 9/10 | hand-code | Parse WKT POINT/POLYGON/MULTIPOLYGON/MULTILINESTRING offline → emit GeoJSON, or reverse | Brief Codebase Intelligence; used internally by resolve/lookup |
| 6 | Offline gemeente / provincie gazetteer | `gemeente get <name>`, `gemeente list --provincie <name>`, `provincie list` | 9/10 | hand-code | One-time sync seeds 342 gemeenten + 12 provincies; subsequent calls hit local SQLite only, with FTS-backed fuzzy name match | Brief Data Layer; Personas Lotte, Joris |
| 7 | Local FTS over cached results | `search <text> [--type adres,gemeente,...]` | 8/10 | hand-code | FTS5 query over `addresses_fts` + `gemeenten` + `lookups`; returns local hits with provenance | Brief Data Layer; Personas Lotte, Sven |
| 8 | Bbox multi-collection CSV dump | `features in-bbox --bbox <x1,y1,x2,y2> --collections adres,perceel [--csv]` | 8/10 | hand-code | Iterates Location API `/collections/{c}/items?bbox=...` for each requested collection, paginates, flattens to wide CSV | Brief Top Workflow #6; Persona Joris; bbox is Location API only |
| 9 | Confidence-gated best match | `top <query> --min-score <n> [--require-type adres]` | 7/10 | hand-code | `/free` top-1 result, exits non-zero if `score < min` or type filter unmet; stdout is the single match | Persona Lotte; used internally by batch geocode |
| 10 | Perceel by kadastrale aanduiding | `perceel lookup --aanduiding "AMR03 N 1234"` | 8/10 | hand-code | Parser splits aanduiding into kadastrale gemeente / sectie / perceelnummer, queries Location API `/collections/perceel/items` | Brief Data Layer (BRK/DKK identity); Persona Joris |
| 11 | Gemeente containing point | `gemeente of-point --lat <> --lon <>` (or `--rd-x --rd-y`) | 8/10 | hand-code | Uses cached `gemeenten` centroids + `/reverse?type=gemeente` fallback to return gemeente + provincie holding the point | Persona Marieke; Brief Data Layer |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|---------------------------|
| C5 validate-postcode | Thin regex; persona uses inside forms, not at CLI; collapses into C2's input normalization | C2 resolve |
| C10 canonicalize | Same workflow as C2 (suggest→lookup top hit), no independent surface | C2 resolve |
| C13 hectometer nearest | Strict `/reverse?type=hectometerpaal` alias; C3 nearest already returns hectometerpaal in its fan-out, with road context if asked | C3 nearest |
| C15 doctor | Table-stakes health check; already in absorb manifest #16 conformance probe | (absorbed) doctor |
| C16 suggest --watch | TUI / streaming-loop scope; violates one-command rule and scope-creep kill check | C2 resolve |
| C17 preview --open | External browser dependency + scope creep; not service-specific | C14 wkt-to-geojson (still emits geometry, user pipes to their own viewer) |
| C18 route nearest-segment | Routing isn't in either PDOK spec; needs an external routing service the brief doesn't authorize | C3 nearest (returns the road segment but not a route) |
