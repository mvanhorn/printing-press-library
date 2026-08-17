# Blacklane Absorb Manifest

No existing Blacklane CLI / MCP / wrapper exists (verified). Net-new.

## Absorbed (match the website's public quote surface)
| # | Feature | Source | Our Implementation | Added value |
|---|---------|--------|--------------------|-------------|
| 1 | Point-to-point transfer quote | blacklane.com booking widget | `quote <from> <to> --at` → POST /prices | --json, offline log, no UI |
| 2 | Hourly quote | booking widget "By the hour" | `quote <from> --hourly Nh --at` (duration secs) | scriptable |
| 3 | Vehicle-class catalog | service catalog | `catalog get <slug>` GET /packages/{slug} | cached, searchable |
| 4 | Address resolution | Google/SSR autocomplete | OSM Nominatim geocode (free, no key) | self-contained |
| 5 | Service-area check | (SSR-only graphql) | empty quote ⇒ not served, surfaced honestly | — |

## Transcendence (only possible with local SQLite + agent-native output)
| # | Feature | Command | Why only us |
|---|---------|---------|-------------|
| 1 | Fare watch / history | `watch`, `history` | Requires local price snapshots over time |
| 2 | Cheapest-window compare | `compare --dates ...` | Requires fan-out quotes + local aggregation |
| 3 | Multi-leg trip total | `trip --leg ... --leg ...` | Requires sequencing+summing quotes locally |
| 4 | Class fit advisor | `fit --pax --bags` | Requires join of catalog capacity + live price |
| 5 | Quote ledger + FTS | `log`, `search`, `sql` | Requires SQLite store of every quote |

## Out of scope (Auth0-gated): bookings, ride status, wallet, payment methods, create-booking. No booking, ever.
