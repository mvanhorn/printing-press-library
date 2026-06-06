# Tripadvisor CLI — Absorb Manifest

## Landscape
Only one real tool touches this API surface: **pab1it0/tripadvisor-mcp** (Python, ~56 stars) — a thin MCP mirror of the 5 Content API endpoints. No competing CLI, no Claude skill, no offline tool. Apify/SerpApi/Omkar are hosted scrapers (different no-key path, not endpoint-parity competitors). We match all 5 endpoints AND beat them with offline store, `--json`/`--select`/`--csv`, typed exit codes, `--dry-run`, and SQLite persistence.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Location search by name | tripadvisor-mcp search_locations | `tripadvisor-pp-cli find <query>` | --category/--lat-long bias, --json/--select, cached to store |
| 2 | Nearby location search | tripadvisor-mcp search_nearby_locations | `tripadvisor-pp-cli near <lat,long>` | typed flags, offline-cacheable, agent-native output |
| 3 | Location details | tripadvisor-mcp get_location_details | `tripadvisor-pp-cli show <locationId>` | leads with rating/num_reviews/ranking; --select; snapshotted for drift |
| 4 | Location reviews (UGC) | tripadvisor-mcp get_location_reviews | `tripadvisor-pp-cli reviews <locationId>` | labeled user-generated; --limit/--offset; --json |
| 5 | Location photos | tripadvisor-mcp get_location_photos | `tripadvisor-pp-cli photos <locationId>` | thumbnail/medium/large/original URLs; --source filter; --json |
| 6 | Offline full-text search | (none — we add it) | `tripadvisor-pp-cli search <term>` (framework FTS) | search cached locations offline, no API call |
| 7 | Raw SQL over cache | (none — we add it) | `tripadvisor-pp-cli sql` (framework, read-only) | compose queries over fetched locations |
| 8 | Health check | (none) | `tripadvisor-pp-cli doctor` | verifies key + reachability + cache report |

Naming note: `search` is reserved by the framework for offline FTS over the local store, so the **live API location search is `find <query>`**. All other verbs match the brief verbatim.

## Transcendence (only possible with our local store + aggregation)
| # | Feature | Command | Buildability | Why Only We Can Do This | Score |
|---|---------|---------|--------------|-------------------------|-------|
| 1 | Ranked best-of search | `best <query> --category --top N --sort rating\|reviews\|ranking` | hand-code | Fans out N bounded detail calls, dedups vs cache, sorts in one pass | 9 |
| 2 | Side-by-side compare | `compare <id> <id> [...]` | hand-code | Joins details + subratings + trip_types across records into one table | 9 |
| 3 | Geo-ranked shortlist | `nearby-best <lat,long> --category --min-rating --top N --max-scan N` | hand-code | Composes nearby + bounded detail fan-out + local filter/sort | 9 |
| 4 | Rating drift | `drift <id> [--threshold] [--since]` | hand-code | Needs a historical snapshot in the local store; API has no versioning | 8 |
| 5 | Location digest | `digest <id> [--reviews N]` | hand-code | Composes details+reviews+photos into one payload (1 call vs 3) | 7 |
| 6 | Traveler fit ranking | `fit <query> --traveler families\|couples\|solo\|business --top N` | hand-code | Weights trip_types distribution across N fetched records | 7 |

All transcendence commands: agent-native (`--json`/`--select`/`--agent`), `mcp:read-only`, bounded scan caps on fan-out (metered API: 5k calls/mo free), and reviews labeled as user-generated.

Total: 8 absorbed + 6 transcendence = 14 features (6 hand-code).
