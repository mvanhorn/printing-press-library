# Mobalytics LoL Browser-Sniff Report

## Architecture Discovery

**Site**: mobalytics.gg/lol/champions/<slug>/
**Reachability**: `probe-reachability` returns `standard_http` for HTML pages and GraphQL endpoints with proper headers.
**Rendering**: Fully server-side rendered (SSR) — `window.__PRELOADED_STATE__` is mostly empty; client-side hydration does NOT re-fetch champion data. Tab navigation triggers full page reloads.
**Authentication**: None required for public champion data. Mobalytics Plus is account-overlay only and does not gate champion-page data.

## Data Sources (Identified)

### 1. Riot Data Dragon (Primary Reference)
- Base: `https://ddragon.leagueoflegends.com/`
- Plain HTTP, no auth, CDN-cached, public
- Endpoints captured in HAR:
  - `GET /api/versions.json` — patch list (current: 16.10.1)
  - `GET /cdn/{ver}/data/{lang}/champion.json` — champion summary list
  - `GET /cdn/{ver}/data/{lang}/champion/{name}.json` — full champion detail (abilities, lore, stats)
  - `GET /cdn/{ver}/data/{lang}/item.json` — full item catalog
  - `GET /cdn/{ver}/data/{lang}/runesReforged.json` — rune trees + shards
  - `GET /cdn/{ver}/data/{lang}/summoner.json` — summoner spells

### 2. Mobalytics LoL GraphQL (Aggregator Data — Phase 3 hand-built)
- **Dynamic queries**: `POST https://mobalytics.gg/api/lol/graphql/v1/query`
  - Plain HTTP works (CF gates GET HTML but not GraphQL POST with proper headers)
  - Introspection disabled
- **Static queries**: `POST https://mobalytics.gg/api/league/gql/static/v1`
  - Plain HTTP works cleanly (mode: standard_http)
- 86 GraphQL operations extracted from JS bundles (see `graphql-operations.json`):
  - **Counter/matchup ops**: `LolChampionCountersOptionsQuery`, `LolChampionCountersStatsOptionsQuery`, `LolChampionCountersTabStaticQuery`
  - **Wiki/guide ops**: `LolNgfWikiDocumentQuery`, `LolNgfUgFeaturedDocumentQuery`, `LolNgfUgNormalDocumentByIdQuery`, `LolNgfUgNormalDocumentBySlugQuery`, `LolNgfStDocumentQuery`, `LolNgfUgTemplateDocumentQuery`
  - **Profile ops**: `LolNgfProfilePageQuery`, `LolNgfProfilePageDocumentTypesQuery`
  - Plus 60+ fragments providing nested field selections
- Query parameters: `slug`, `role` (TOP/JUNGLE/MID/ADC/SUPPORT), `patch` (e.g., `patch_26_10`), `queue` (RANKED_SOLO), `rank` (EMERALD_PLUS, etc.), `region` (ALL or specific)

## Generation Strategy

**Phase 2 (generator)**: Use Riot Data Dragon HAR → produces CLI with `versions`, `champions`, `champion-detail`, `items`, `runes`, `summoner-spells` commands. Foundation for the local store.

**Phase 3 (hand-built)**: Add Mobalytics-specific commands using the extracted GraphQL queries:
- `tier-list` — top champions by tier/WR/PR/BR across roles
- `champion build <slug>` — recommended items/runes/skill order
- `champion counters <slug>` — best/worst matchups with WR
- `champion matchups <slug>` — full matchup table
- `champion aram <slug>` — ARAM-specific build
- `compare <c1> <c2>` — head-to-head (novel)
- `counter-pool --pool <list>` — given my champion pool, suggest answers to enemy (novel)
- `meta-shift --since <patch>` — tier movement vs prior patch (novel)
- `power-spike --phase <early|mid|late>` — list champions spiking in a phase (novel, mobalytics-signature)

## Runtime Notes

- HTTP transport: `standard` (stdlib Go HTTP works for both DDragon and Mobalytics GraphQL)
- No clearance cookie needed
- No proxy-envelope (single GraphQL endpoint, not proxy-routed)
- Champion slugs: lowercase ASCII (e.g., `aatrox`, `kai-sa` for Kai'Sa, `wukong` for MonkeyKing)
- ARAM, Arena, Counters, Guide, Combos are separate URLs per champion: `/lol/champions/<slug>/{build,aram-builds,arena-builds,counters,guide,combos}`

## Replayability Verdict

PASS. The discovered surface is replayable via plain HTTP:
- Riot DDragon: no challenges
- Mobalytics GraphQL: returns valid GraphQL responses with proper headers (User-Agent + Origin + Referer)
- HTML pages also accessible for fallback parsing (probe-reachability confirmed)
