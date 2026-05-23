# mobalytics-lol-pp-cli — Absorb Manifest

The CLI absorbs every feature competitors expose for LoL champion data AND transcends them with cross-entity SQL queries no single site surfaces.

## Source Tools Catalogued

- **op.gg** (`opgg/op.gg-frontend`) — broad mode coverage (SR, ARAM, Arena, URF)
- **u.gg** — Duo tier list (unique), per-matchup builds
- **lolalytics.com** — per-matchup rune WR (unique), sample-size everywhere, high-elo filter
- **mobalytics.gg** — power spikes (signature), curated tier + algo blend
- **leagueofgraphs.com** — "mained" / one-trick filter (unique)
- **probuilds.net** — pro-only data (complement, link out)
- **Championify** (legacy) — direct write to LoL client item-set folder
- **Riot Data Dragon** — public static reference data (no auth, plain HTTP)
- **Existing CLIs** (`leagueoflegends-cli`, `league-cli`, `lol-wiki-cli`) — all Riot-API/DDragon-backed; none wrap aggregator data
- **MCP servers for LoL** (`kostadindev/League-of-Legends-MCP`, `jifrozen0110/mcp-riot`, Apify `lol-mcp-server`) — all Riot-API, NONE expose aggregator data → clean niche

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|--------------|
| 1 | Champion list with role/tag filter | op.gg, u.gg, mobalytics | (generated endpoint) champions list | Offline, SQL queryable, --select fields |
| 2 | Champion full details (abilities, lore, stats) | u.gg champion page | (generated endpoint) champions get | Offline, --json, integrates with build/counter data |
| 3 | Item catalog | All sites | (generated endpoint) items list | Offline, --select stats, SQL on item build-paths (`--select builds_from,builds_into`) |
| 4 | Rune trees and shards | All sites | (generated endpoint) runes list | Offline, joins to champion-build for full rune-page rendering |
| 5 | Summoner spells | All sites | (generated endpoint) summoner-spells list | Offline, ARAM-mode filter |
| 6 | Patch version catalog | All sites (in header) | (generated endpoint) versions list | List patches, query specific patch data |
| 7 | Champion tier ratings | mobalytics, u.gg, op.gg, lolalytics | mobalytics-lol-pp-cli tier-list | Free, agent-native, filterable by role/rank/patch |
| 8 | Champion build (items + skill order) | mobalytics, op.gg, u.gg | mobalytics-lol-pp-cli champion build | Free, --json, --select for AI agents |
| 9 | Champion runes (rec + shards) | All sites | mobalytics-lol-pp-cli champion runes | Free, offline cache, --to client format |
| 10 | Champion counters with WR/sample | mobalytics, op.gg, lolalytics | mobalytics-lol-pp-cli champion counters | --rank, --role, --patch filters, agent-native |
| 11 | Champion matchups (lane-by-lane WR) | lolalytics, mobalytics | mobalytics-lol-pp-cli champion matchups | Sample size shown like lolalytics, full matrix |
| 12 | Champion synergies (duo partners) | u.gg, mobalytics | mobalytics-lol-pp-cli champion synergies | u.gg pioneered standalone duo data |
| 13 | ARAM-specific data | mobalytics, op.gg, u.gg | mobalytics-lol-pp-cli champion aram | ARAM has different builds/runes/tier |
| 14 | Arena mode data | op.gg, u.gg, mobalytics | mobalytics-lol-pp-cli champion arena | Arena duos, augments |
| 15 | Power-spike data (early/mid/late) | mobalytics (signature) | (behavior in mobalytics-lol-pp-cli champion build) | Embedded in build output, also driving `power-spike` transcendence command |
| 16 | Skill order recommendation | All sites | (behavior in mobalytics-lol-pp-cli champion build) | Embedded in build output |
| 17 | Champion combo guides | mobalytics combos page | mobalytics-lol-pp-cli champion combos | Combat sequences, mechanics |
| 18 | Mained / one-trick filter | League of Graphs (unique) | mobalytics-lol-pp-cli champion mained | LeagueOfGraphs's unique signal |
| 19 | Per-matchup rune WR | lolalytics (unique) | (behavior in mobalytics-lol-pp-cli champion matchups, when sample ≥ threshold) | Lolalytics's signature differentiator, applied when Mobalytics data supports it |
| 20 | Item set export to LoL client | Championify (legacy precedent) | (behavior in mobalytics-lol-pp-cli champion build --to client) | Writes item-set JSON to LoL client folder |

## Transcendence (only possible with our approach)

Source: Phase 1.5 novel-features subagent (`2026-05-23-111659-novel-features-brainstorm.md`).

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Counter-pool analysis | mobalytics-lol-pp-cli counter-pool --our <c1,c2,c3> --their <c4,c5,c6> | hand-code | SQL join across `champion_matchups` for cartesian product of two pools, ordered by WR delta with sample floor. No single Mobalytics page shows pool×pool. (Sora) |
| 2 | Patch meta-shift | mobalytics-lol-pp-cli meta-shift --since-patch <N> | hand-code | Diff two `tier_snapshots` rows on tier and WR with sample guard. Mobalytics shows current tier only, not deltas. (Daria, Sora) |
| 3 | Head-to-head champion compare | mobalytics-lol-pp-cli compare <c1> <c2> | hand-code | Side-by-side join across `tier_snapshots`, `champion_builds`, `champion_matchups`; computes item-overlap %. No site renders two champions on one page. (Daria, Sora) |
| 4 | ARAM batch item-set export | mobalytics-lol-pp-cli item-set --aram --to client <c1,c2,...> | hand-code | Reads `champion_builds` ARAM rows, serializes to LoL client item-set JSON, writes to client folder. Championify did this for SR in 2019 and died. (Marco) |
| 5 | Duo-finder with candidate-pool filter | mobalytics-lol-pp-cli duo-finder --bot <c> --supports-from <c1,c2,c3> | hand-code | SQL filter: `WHERE champion=<bot> AND partner IN (<pool>) ORDER BY wr DESC`. u.gg has duo-finder but no pool restriction — coaches need it. (Sora) |
| 6 | Personal pool tier digest | mobalytics-lol-pp-cli pool-digest --pool <c1,...> | hand-code | Composite query: per-champion current tier, WR delta since last patch, top-1 counter, top-1 synergy. Replaces a 4-tab morning ritual. (Daria) |
| 7 | Power-spike phase filter | mobalytics-lol-pp-cli power-spike --phase <early\|mid\|late> | hand-code | Inverts Mobalytics's per-champion power-spike data into "give me everyone who spikes early." Mobalytics has the data but only on champion pages. (Marco, Sora) |
| 8 | Flex-pick detector | mobalytics-lol-pp-cli flex --rank <X> --min-roles 2 | hand-code | `tier_snapshots` self-join: champions appearing as ≥A-tier in 2+ roles. No site indexes flexibility — they all index by role first. (Sora) |
| 9 | Region-split tier compare | mobalytics-lol-pp-cli tier-list --compare-regions kr,euw,na | hand-code | Multi-region `tier_snapshots` pivot, same patch + rank, three regions side-by-side. Mobalytics defaults to one region per page-load. (Sora) |

All 9 transcendence rows are `hand-code` because the Mobalytics GraphQL data backing them is hand-built in Phase 3 (queries extracted from the JS bundle, not in the spec). The spec only emits Riot Data Dragon REST endpoints.

## Build Order

1. **Priority 0 — Foundation**: Riot Data Dragon endpoints (champions, items, runes, summoner spells, versions) via generated `champions`, `items`, `runes`, `summoner-spells`, `versions` commands; SQLite schema for all five reference tables plus `tier_snapshots`, `champion_builds`, `champion_matchups`, `champion_synergies`.
2. **Priority 1 — Absorbed (Mobalytics commands)**: tier-list, champion build/runes/counters/matchups/synergies/aram/arena/combos/mained (14 hand-built Mobalytics commands).
3. **Priority 2 — Transcendence**: 9 commands above.
4. **Priority 3 — Polish**: descriptions, --to client export format, FTS search.
