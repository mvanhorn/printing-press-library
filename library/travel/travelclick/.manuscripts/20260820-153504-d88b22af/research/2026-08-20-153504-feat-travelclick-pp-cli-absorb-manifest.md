# TravelClick CLI Absorb Manifest

## Search coverage

Searched GitHub code search (`travelclick`, `ihotelier`), the public printing-press-library registry (423 entries — closest matches `hotel-goat`, `hotel-tonight`, `hotelist`, `booking-com`, all OTA/aggregator-side, none touch a CRS/booking-engine's own consumer widget), and general web research for MCP servers, SDK wrappers, and CLIs. **No existing tool, MCP server, SDK wrapper, or CLI touches the TravelClick/iHotelier consumer booking widget.** The only GitHub hits (`roli854/travelclick`, `api-evangelist/travelclick-amadeus`) cover TravelClick's *separate* B2B SOAP/XML CRS-to-PMS integration, not this surface. This is a greenfield build — the "Absorbed" table below absorbs from the widget's own feature set (nothing to out-compete; nothing pre-existing to lose parity with).

## Absorbed (match or beat everything the widget itself does)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search room availability & rate plans for specific dates | TravelClick widget itself (no competing tool found) | `travelclick-pp-cli rates search` | Scriptable, `--json`, no browser tab per hotel |
| 2 | Calendar of lowest rate per day over a date range | TravelClick widget itself | `travelclick-pp-cli rates calendar` | Same, plus feeds local price-drift tracking (see Transcendence #4) |
| 3 | Validate a corporate/rate-access code | TravelClick widget itself | `travelclick-pp-cli codes validate-corporate` | Structured valid/invalid result instead of an inline UI toast |
| 4 | Validate a group-attendee code | TravelClick widget itself | `travelclick-pp-cli codes validate-group` | Same |
| 5 | Hotel profile, policies, mandatory fees | TravelClick widget itself | `travelclick-pp-cli hotel info` | Surfaces the mandatory nightly fee's full text explicitly instead of a fine-print modal |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Multi-hotel rate compare | `rates compare --hotels 102306,<id2>,<id3> --check-in 2026-09-15 --check-out 2026-09-18` | 8/10 | hand-code | Fans out `rates search` across N hotel IDs in parallel, ranks by lowest total (rate + mandatory fee) | Brief Top Workflow #5: each hotel's widget is siloed to itself; nothing lets a traveler compare boutique properties side by side without opening N browser tabs | Use this to compare several specific hotels for the same dates. Do NOT use it to scan one hotel across many dates; use `rates cheapest-night` instead. |
| 2 | Cheapest night across saved hotels | `rates cheapest-night --hotels made-nyc,<alias2> --from 2026-09-01 --to 2026-10-31` | 8/10 | hand-code | Fans out `rates calendar` across N hotel IDs and returns the single best (hotel, date) combination client-side | Brief Top Workflow #2 (per-hotel calendar) extended cross-hotel; no widget or competitor offers a cross-property calendar since each widget only knows its own hotel | Use this to find the best date+hotel combo across several properties. Do NOT use it for a single hotel's calendar; use `rates calendar` instead. |
| 3 | Saved hotel aliases | `hotels alias add made-nyc 102306` / `hotels alias list` | 6/10 | hand-code | Local SQLite table mapping a memorable alias to a numeric hotel ID; `learn.entity_lookup_seeds` bootstraps with Made Hotel NYC (102306) | Brief Data Layer section: hotel IDs are not memorable; the widget URL itself buries the ID in a query string | none |
| 4 | Rate-snapshot price-drift history | `rates search 102306 --check-in ... --check-out ... --save` then `analytics price-drift --hotel 102306` | 7/10 | hand-code | Persists each `search`/`calendar` call's rates into local SQLite with a captured-at timestamp; drift analytics diffs snapshots over time | Brief Product Thesis: mirrors the price-drift pattern already shipped in this user's own `hotel-tonight` and `1688` CLIs; no OTA/CRS surface exposes historical pricing itself | none |
| 5 | Rate-code check across saved hotels | `codes check-all ACME2026 --type corporate --hotels made-nyc,<alias2>` | 6/10 | hand-code | Runs `codes validate-corporate`/`validate-group` against every saved/aliased hotel and reports which ones accept the code | Persona "Nomad Ned" (corporate nomad): a client/event code is often valid chain-wide or across a group's properties, but the widget only lets you test one hotel at a time | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Rate forecast (`rates forecast --days 60`) | Requires trend extrapolation math well beyond a few captured snapshots — no real predictive signal to build on yet, pure scope creep for v1 | Price-drift history (#4) — ships the raw historical data a future forecast could build on |
| Code guess/fuzz (`codes search-suggest`) | Brute-forcing plausible codes against a real hotel's API is exactly the kind of probing behavior this project's cardinal rules prohibit ("do not invent mutating payloads or brute-force probe") | Code validation (Absorbed #3/#4) — already gives instant, honest yes/no feedback for a code the user actually has |
| Standalone fees-check command (`hotel fees-check --hotels <csv>`) | Thin wrapper: it's just `hotel info` called across N hotels filtering for one field — doesn't clear the "wrapper vs leverage" bar on its own | Multi-hotel rate compare (#1) — its output already surfaces each hotel's mandatory fee inline per result |

## Stubs

None. Every row above (Absorbed and Transcendence) ships fully implemented — no `(stub)` dispositions in this manifest.
