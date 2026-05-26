# Mobalytics LoL CLI Brief

## API Identity
- Domain: League of Legends meta-analytics (champion builds, tiers, counters, runes, ARAM)
- Users: LoL players from Iron through Challenger; coaches; analysts; agents asking "what's the best build for X this patch"
- Data profile: ~170 champions × N roles × M ranks × patch × region → tier/WR/PR/BR snapshots; ~250 items, ~70 runes/shards, ~15 summoner spells (reference); per-champion: recommended builds, skill order, counters, synergies, power spikes, ARAM build
- Spec: none. Discovery via browser-sniff against `https://mobalytics.gg/lol/champions/<name>/` and the tier-list page. Likely Next.js with embedded SSR JSON or a `/api/lol/...` GraphQL/REST surface.

## Reachability Risk
- **Low.** Cloudflare gates default UAs but `probe-reachability` returned `mode: standard_http` (confidence 0.95) on the champion page with stdlib HTTP. Printed CLI ships plain HTTP transport (no Surf, no clearance cookie).
- Cloudflare interstitial appears for `curl` with a UA header alone; full Chrome header set or stdlib `net/http` defaults pass cleanly. No evidence of JS challenge / Turnstile.
- No GitHub-issue history of Mobalytics scrapers getting blocked. The only public scraper (`cashmerebuffalo/Mobalytics-Scraper`) has 0 stars and last saw activity in 2021 — it worked with BeautifulSoup, implying static HTML/SSR-JSON.

## Top Workflows
1. **Look up a champion's current build and runes.** "What runes should I run on Aatrox top this patch in Diamond+?" → returns runes, items, skill order, summoner spells.
2. **Check tier ratings.** "Is Yone S-tier mid this patch?" → tier badge, WR/PR/BR, sample size, trend vs prior patch.
3. **Find counters and synergies.** "Who counters Darius?" / "Who pairs well with Yuumi?" → ranked list with WR and sample.
4. **ARAM-specific data.** Different builds, tier list, power-spike notes for ARAM.
5. **Agentic "compare two champions on the same dimensions"** — head-to-head WR/build/counter overlap.
6. **Patch-meta delta.** "What moved up since patch 14.10?" → champions that gained/lost tier.

## Table Stakes (Absorb Targets)
- **Champion stats**: tier, WR, PR, BR, sample, role-splits — covered by every competitor.
- **Builds**: starter/core/situational items per phase; skill order; summoner spells — op.gg, u.gg, lolalytics, mobalytics.
- **Runes**: primary + secondary tree, shards — all sites.
- **Counters & synergies**: top-N with WR + sample — all sites.
- **ARAM tab**: separate build/tier set — op.gg, mobalytics, u.gg.
- **Tier list with filters**: role, rank, patch — all.
- **Power spikes**: early/mid/late phase rating — mobalytics's signature feature.
- **Mained / one-trick filter**: LeagueOfGraphs unique → absorb as transcendence.
- **Duo synergy tier list**: u.gg unique → absorb as transcendence.
- **Per-matchup rune WR**: lolalytics unique → absorb if surfaced in sniff.

## Codebase Intelligence (DeepWiki/MCP/scraper survey)
- **No source for Mobalytics aggregator data exists yet.** Closest: `cashmerebuffalo/Mobalytics-Scraper` (Python+BS4, 0★, stale 2021). Useful as evidence the data is SSR-rendered, not a clue for endpoints.
- **MCP servers already exist for Riot API** (`kostadindev/League-of-Legends-MCP`, `jifrozen0110/mcp-riot`, Apify `lol-mcp-server` with 26 tools). NONE expose aggregator data. The **CLI's MCP mode is a clean niche** — agentic tier/build/counter lookups across millions of games is not currently agent-accessible.
- **Riot wrappers (reference, not absorb)**: KnutZuidema/golio (Go, 89★), riot-watcher (Py), cassiopeia (Py). These cover Data Dragon (static asset CDN — champions.json, items.json, runes) which we can absorb as offline reference data.

## Auth
- **No auth required for public champion / tier-list pages.** Mobalytics Plus exists ($/mo) but gates profile/overlay/coaching features, not champion-page data.
- The printed CLI ships `auth.type: none`.
- An optional logged-in browser session could unlock a user's own match-history-overlay data, but that's profile territory — out of scope for a champion-data CLI.

## Data Layer (SQLite)
- **Primary entities** (high-gravity, joinable, time-series cacheable):
  - `champions` — ~170 rows, slowly versioned per patch, joins everything (id, name, role-tags, base stats, abilities)
  - `items` — ~250 rows, patch-versioned (id, name, cost, stats, builds-from, builds-into)
  - `runes` — ~70 rows (trees + shards), rare changes (id, name, tree, slot, description)
  - `summoner_spells` — ~15 rows, near-static
  - `tier_snapshots` — **the actual time-series table.** Champion × role × patch × rank-bucket × region → tier, WR, PR, BR, sample, computed_at. This is what powers "what's the meta right now" + "what moved" + delta commands.
  - `champion_builds` — Champion × role × patch × rank → recommended items (starter/core/situational), runes (primary/secondary/shards), skill order, summoner spells
  - `champion_matchups` — Champion × opponent × role × patch × rank → WR, sample, lane outcome
- **Sync cursor**: by patch number — when patch changes, refresh tier_snapshots + builds + matchups. Reference data (champions, items, runes) refreshes per patch from Data Dragon.
- **FTS / search**: champion name (with skin/title aliases), item name, rune name.

## Product Thesis
- **Name**: `mobalytics-lol-pp-cli` (binary), brand display "Mobalytics LoL"
- **Why it should exist**: There is no agent-accessible source of LoL aggregator data. Every existing MCP server wraps Riot's match/profile API. Players reflexively `op.gg` a champion before queue — the CLI does that in their terminal AND from Claude. Offline SQLite cache means tier comparisons, patch-meta deltas, and counter aggregation across champions become single commands, not multiple page loads.
- **Edge over `curl mobalytics.gg`**: real navigation parsing, structured JSON, `--select` field filtering, local FTS over the synced corpus, cross-champion comparisons via SQL, agent-native exit codes and `--dry-run`.

## Build Priorities
1. **Foundation (Priority 0)** — SQLite schema + sync command pulling champion list, tier snapshots, and per-champion build data from the sniffed endpoints. Data Dragon as fallback reference loader for items/runes/champions.
2. **Absorb (Priority 1)** — every endpoint discovered in sniff exposed as a typed command: `champion`, `champion build`, `champion runes`, `champion counters`, `champion matchups`, `champion aram`, `tier-list`, `tier-list aram`. Filters: `--role`, `--rank`, `--patch`, `--region`.
3. **Transcend (Priority 2)** — compound queries that no single site has in one place:
   - `compare <champ1> <champ2>` — head-to-head on build, counters, tier
   - `meta-shift --since-patch <N>` — what moved up/down vs prior patch
   - `counter-pool --pool <c1,c2,c3>` — given my champion pool, who do I have a good answer for
   - `duo-finder --bot <champ>` — best support pairings (u.gg's idea, applied to mobalytics data)
   - `one-trick <champ>` — only show data from mained players (LeagueOfGraphs's idea)
   - `power-spike --phase early` — list every champion that spikes early this patch
   - `item-set --to client` — export builds as a LoL client item-set JSON file (Championify's idea, agent-native)
