# MotoGP CLI Absorb Manifest

## Absorbed (match or beat every existing tool)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List seasons | robschmitt docs | `seasons` | Offline cache, --json |
| 2 | Season events/calendar | racingmike | `calendar <year> [class]` | Finished flag + winner, offline |
| 3 | Categories per season/event | docs | `categories` | Auto from cache |
| 4 | Sessions for event/category | racingmike | `sessions <year> <event> <class>` | Name-resolved, conditions shown |
| 5 | Session classification/results | xNegis, ParsaD23 | `results <year> <event> <class> [session]` | UUID auto-resolve, points |
| 6 | Grid / qualifying positions | docs | `grid <year> <event> <class>` | Name-resolved |
| 7 | Entry list | docs | `entry <year> <event> <class>` | Name-resolved |
| 8 | Championship standings | all importers | `standings <year> <class>` | Offline, --json |
| 9 | Standings document files (PDF) | docs | `standings files <year> <class>` | Direct URLs |
| 10 | Qualifying/BMW award standings | docs | `standings bmw <year>` | — |
| 11 | All riders | docs | `riders [--class]` | FTS search, offline |
| 12 | Rider profile | docs | `riders get <name>` | Name-resolve |
| 13 | Rider career stats | docs | `riders stats <name>` | Wins/podiums/poles |
| 14 | Rider season-by-season stats | docs | `riders statistics <name>` | — |
| 15 | Teams + rosters | racingmike | `teams <year> <class>` | Offline |
| 16 | Live timing lite | racingmike, motogp-zero | `live` | Agent-native JSON snapshot |
| 17 | Full data sync to local store | racingmike | `sync [--season]` | SQLite, FTS, offline everything |
| 18 | Offline full-text search | (none) | `search <term>` | Riders/events/teams |
| 19 | Raw SQL over local store | (none) | `sql "<query>"` | Analyst power tool |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Human-name → UUID resolver (year/class/event/rider all by name) | (cross-cutting; powers every command) | hand-code | Requires local resolution cache; API only accepts chained UUIDs |
| 2 | Championship title-race progression | `title-race <year> <class>` | hand-code | Joins every round's classification into a points-over-rounds table; no single endpoint |
| 3 | Rider head-to-head | `h2h <riderA> <riderB>` | hand-code | Aggregates career + season stats across both riders locally |
| 4 | Circuit history (winners by venue over years) | `circuit-history <event> [class]` | hand-code | Requires multi-season join across events+classifications |
| 5 | Rider career timeline | `career <name>` | hand-code | Merges profile + per-season stats into one view |
| 6 | "What did I miss" recent results | `since <year>` | hand-code | Time-windowed aggregation of finished events + winners |
| 7 | Calendar ICS export | `calendar <year> --ics` | hand-code | Generates ICS from synced events (racingmike parity, offline) |

## Source tools (for README credits)
- robschmitt/MotoGP-API (docs) — endpoint documentation
- micheleberardi/racingmike_motogp_import (Python) — importer, ICS, live timing, WorldSBK
- ParsaD23/MotoGP-API (Python) — results reader
- xNegis/MOTOGP-API (Java) — category/year results
- ChrisUser/motogp-zero (TypeScript) — live timing dashboard

## Notes
- No stubs planned. Live timing (`live`) returns whatever the feed gives (empty out of session) — documented, not stubbed.
- Hand-code count: 7 transcendence features. Spec-emits: 19 absorbed read commands (generator-emitted from YAML spec).
