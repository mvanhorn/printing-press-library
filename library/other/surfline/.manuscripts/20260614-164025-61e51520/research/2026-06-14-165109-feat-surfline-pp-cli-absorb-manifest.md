# Surfline CLI Absorb Manifest

Sources catalogued (source-verified): TGOlson/surfline (TS, precise types), mdecourcy/go-surfline-api (exact query strings), mhelmetag/surflinef (Go, ★83, auth), Mircobrb/pysurfline (Python), kylerberry/surfline-nodejs (search/overview shapes), englishar/surfline-mcp-server (★7, MCP tools), swrobel/meta-surf-forecast (★336, aggregator).

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Spot search by name | surfline-nodejs `search`, MCP | surfline-pp-cli search | Offline FTS after taxonomy sync, --json, returns spotId+breadcrumb |
| 2 | Wave + swells forecast | all wrappers `wave` | (generated endpoint) spots wave | Typed, --json/--select, --days/--interval/--units, SQLite snapshot |
| 3 | Wind forecast | go-surfline-api `GetWindForecast` | (generated endpoint) spots wind | directionType (Onshore/Offshore/Cross-shore), gust, --corrected |
| 4 | Tides forecast | all wrappers `tides` | (generated endpoint) spots tides | HIGH/LOW/NORMAL + heights, local-time render |
| 5 | Weather forecast | go-surfline-api `GetWeatherForecast` | (generated endpoint) spots weather | temp/condition/pressure + sunlightTimes |
| 6 | Conditions (am/pm rating) | TGOlson `conditions` | (generated endpoint) spots conditions | forecaster name, humanRelation, occasionalHeight |
| 7 | Rating forecast | go-surfline-api `GetSpotForecastRating` | (generated endpoint) spots rating | VERY_POOR..EPIC enum + numeric value |
| 8 | Sunlight forecast | go-surfline-api `GetSunlightForecast` | (generated endpoint) spots sunlight | dawn/sunrise/sunset/dusk local times |
| 9 | Combined forecast | pysurfline, TGOlson `combined` | (generated endpoint) spots forecast | One call: forecasts+tides+sunrise/sunset |
| 10 | Multi-spot batch info | TGOlson `fetchSpotInfo` (POST /batch) | (generated endpoint) spots batch | Rich per-spot object incl cameras/current conditions in one call |
| 11 | Spot details | pysurfline `spots/details` | (generated endpoint) spots details | Spot metadata (name, location, abilityLevels) |
| 12 | Region conditions | surflinef `GetConditions`, go-surfline-api | (generated endpoint) regions conditions | subregionId-keyed, --days |
| 13 | Taxonomy browse | TGOlson `fetchTaxonomy`/`fetchEarthTaxonomy` | (generated endpoint) taxonomy | geoname→region→subregion→spot tree, --max-depth |
| 14 | Nearby buoys | go-surfline-api `GetNearbyBuoys` | (generated endpoint) buoys nearby | lat/lon/distance/limit, free NOAA-style fallback data |
| 15 | Premium token auth | surflinef `PostLogin` (/trusted/token) | surfline-pp-cli auth login / auth set-token / auth status | Password-grant login + token store; unlocks 7–17 day horizon |
| 16 | Units overrides | go-surfline-api units[...] params | (behavior in surfline-pp-cli spots wave) --units ft/m, --wind-units kts/mph | Per-field unit flags wired into all forecast cmds |
| 17 | Days / interval control | all wrappers days+intervalHours | (behavior in surfline-pp-cli spots wave) --days --interval | Bounded scan; >6 days requires token (honest gating msg) |
| 18 | Forecaster notes/report | MCP `get_forecaster_notes`, /spots/reports | (generated endpoint) spots report | Human forecaster narrative + current conditions |
| 19 | Live cam + rewind URL | TGOlson Camera type (batch) | (behavior in surfline-pp-cli spots cams) cams | stillUrl/streamUrl/rewindBaseUrl, isPremium flag |
| 20 | MCP "best spot" | MCP `get_best_spot` | (covered by transcend #1 rank) | Ranks a set, not hardcoded to 11 spots |

## Transcendence (only possible with our approach) — from adversarial-cut subagent (8 survivors >=5/10)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|-------------------------|------------------|
| 1 | Dawn-patrol one-shot | now <spot> | 9 | hand-code | Joins wave+swells+wind+tides+rating on shared unix timestamp into one hour-by-hour readout no single API call returns | Use for a single spot's next few hours as a paddle/no-paddle line readout. Do NOT use it to compare spots; use 'rank' for that. |
| 2 | Multi-spot rank | rank <spots...> | 9 | hand-code | One POST /kbyg/spots/batch, transparent sum of wave+wind+swell optimalScore, sorted best-first; official multi-cam is web-only and unsorted | Use to compare a set of spots and pick today's best. For one spot's hour-by-hour detail use 'now'; for a whole subregion use 'region-best'. |
| 3 | Optimal-window scanner | windows <spot> | 8 | hand-code | Emits contiguous blocks where wave/wind/swell optimalScore are all >=1, intersected with sunlightTimes for daylight | Use to find the good time blocks at one spot today/this week. It already drops after-dark windows, so there is no separate daylight command. |
| 4 | Raw conditions dump | raw <spot> | 8 | hand-code | Emits min/max/optimalScore/humanRelation + swell components + wind directionType/gust as pipe-friendly table/JSON, no rating editorializing | Use when you want unfiltered numeric fields to pipe elsewhere. For a ranked/judged view use 'rank' or the rating in 'now'. |
| 5 | Buoy vs forecast cross-check | buoy-check <spot> | 7 | hand-code | Joins /kbyg/buoys/nearby observed swell against the spot's wave forecast for the same window, side by side | Use to sanity-check the forecast against live buoy observations. It does not judge spots; use 'rank' to choose between them. |
| 6 | Scriptable alert rules | alert add / alert run | 8 | hand-code | Stores swell/wind/tide threshold rules in local SQLite; alert run fetches fresh forecast, evaluates, prints matches and sets exit code for cron | Use to define and cron-evaluate condition rules. For an interactive one-time look use 'now'/'windows'; this is for unattended runs. |
| 7 | Forecast-history journal | journal log / journal show <spot> | 7 | hand-code | Snapshots combined forecast into local SQLite keyed by spotId+timestamp; the API has NO personal history | Use to record and review forecast snapshots over time. To diff forecast-vs-later-actual use 'journal drift' (future view). |
| 8 | Offline spot search | search --offline | 6 | spec-emits | FTS over locally-synced taxonomy (name + breadcrumb) so search resolves names→spotIds with no network | none |

Minimum transcendence met (8). Hand-code rows: 7 (now, rank, windows, raw, buoy-check, alert, journal). spec-emits: 1 (offline search — framework `search` already reads the local store).

## Stubs
None planned. Cam stream/rewind returns URLs (no video download); that is full behavior, not a stub.
