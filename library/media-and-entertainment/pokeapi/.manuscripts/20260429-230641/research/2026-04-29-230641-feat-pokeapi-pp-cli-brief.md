# PokéAPI CLI Brief

## API Identity
- **Domain:** Pokémon reference data (species, moves, items, abilities, types, evolution chains, locations, encounters, sprites, version metadata).
- **Users:** Pokémon hobbyists, competitive battlers, fan-game/ROM-hack devs, educational projects, AI agents answering Pokémon questions, fan-site builders.
- **Data profile:** Read-only consumption-only REST API. ~10 000 documents across 49 top-level resources. 98 GET endpoints (every resource has list + retrieve; `/pokemon` has list + retrieve + encounters). Largest individual response is `/pokemon/{id}/` which can exceed 100 KB.
- **Spec source:** Official OpenAPI 3.1.0, version 2.7.0 — `https://raw.githubusercontent.com/PokeAPI/pokeapi/master/openapi.yml` (265 KB).
- **Auth:** None. Static-hosted since Nov 2018; no rate limits, but the project asks consumers to cache locally to reduce hosting cost. This is *the* perfect API for a local-store-backed CLI.

## Reachability Risk
- **None.** Spec returns 200 (271 848 bytes). Live API `/pokemon/1` returns the expected `bulbasaur` payload with all fields. No bot protection, no auth challenge, no historical 403/429 issue volume.

## Top Workflows
1. **Look up a Pokémon's profile.** "What's pikachu? What types, stats, abilities, signature moves?"
2. **Plan a battle / counter a threat.** "What beats charizard? What does my team's typing leave exposed?"
3. **Build a balanced team.** "Pick 6 Pokémon whose combined typing covers everything and shares few weaknesses."
4. **Trace evolution.** "What does eevee evolve into and how?"
5. **Filter learnable moves.** "What moves does bulbasaur learn by level-up in red-blue?"
6. **Find encounter locations.** "Where does growlithe spawn in red-blue?"
7. **Browse / search with no internet.** Hobbyists already cache locally — make this first-class.
8. **Reverse search by effect.** "What moves can paralyze a Steel-type? Which Pokémon have Levitate?"

## Table Stakes (every competing tool has these)
- List + retrieve every resource (Pokémon, move, item, ability, type, etc.) — 98 endpoints.
- Pokémon profile aggregation (combine pokemon + species + types + abilities + stats + moves).
- Type effectiveness chart / weakness analysis.
- Evolution chain traversal.
- Move filtering by learn method / version group / level.
- Team coverage (shared weaknesses, offensive coverage).
- Battle simulation with STAB / type effectiveness / status / crit (Jalajil/Poke-MCP).
- Sprite retrieval and caching (pokebase).
- "Popular picks" / tournament squad shortcut (Sachin-crypto/Pokemon-MCP-Server).
- Local cache (every wrapper has this — it's the obvious-first move for a static API).

## Data Layer
- **Primary entities:** pokemon, pokemon-species, pokemon-form, type, ability, move, item, location, location-area, evolution-chain, generation, version-group.
- **Sync cursor:** Static dataset — full sync, then refresh on `--rebuild` or new API version. No incremental cursor needed.
- **FTS/search:** Pokémon name, species name, move name+description, ability name+description, item name+description, location name. ~10 000 documents total — well within SQLite FTS5 sweet spot.
- **Joins that unlock NoI features:** pokemon ↔ types ↔ damage_relations (matchup math), pokemon ↔ pokemon_moves ↔ moves (learnset queries), evolution_chain ↔ pokemon_species ↔ pokemon (evolution traversal), pokemon ↔ encounters ↔ location_areas (where-to-find), abilities ↔ pokemon (reverse ability search).

## Codebase Intelligence (existing v2.3.6 in public library)
- Generated 2026-04-26 with printing-press v2.3.6 (we are now on v3.0.1 — significant version delta covering: scoring rubrics, MCP surface annotations, agent-native review, output-review, tools-audit, SKILL coherence checks, runtime cobratree mirror).
- 97 MCP tools, full MCP-ready, auth_type=none.
- 5 novel features shipped (in `.printing-press.json`):
  - `pokemon profile` — aggregate from core + species + types + abilities + stats + move counts.
  - `pokemon evolution` — species → chain → readable path.
  - `pokemon matchups` — weaknesses / resistances / immunities + offensive coverage.
  - `pokemon moves` — filter by learn-method / version-group / level.
  - `team coverage` — comma-separated team analysis.
- README is endpoint-oriented (one section per resource), SKILL is generated-default style.

## User Vision
- The user explicitly chose REST over GraphQL.
- The user wants a **clean from-scratch regen** because the printing press has been upgraded — v2.3.6 → v3.0.1 changes the scoring rubric, MCP surface, novel-feature handling, and SKILL prose substantially.
- The user provided three reference wrappers as inspiration:
  - **JoshGuarino/PokeGo** (Go) — resource-group method API, in-memory cache with 24-hour expiry, configurable URL/logger. Reference for client shape in Go.
  - **beastmatser/aiopokeapi** (Python async) — context-manager client, lazy `await fetch()` for related objects, automatic object cache.
  - **GregHilmes/pokebase** (Python) — `APIResource` lazy-loading, `SpriteResource` for binary image cache, `type_()` helper to dodge name shadowing.
- Common thread across all three: **caching is the headline feature**. Our local SQLite store is a stronger version of every cache strategy these wrappers ship.

## Product Thesis
- **Name:** PokéAPI CLI (binary `pokeapi-pp-cli`).
- **Why it should exist (vs the v2.3.6 in the public library and every wrapper above):** Static, append-only API with ~10 000 documents and zero rate limit is the textbook case for a local SQLite-backed CLI. Existing tools all do per-request caching; nobody ships the **whole dataset** as a queryable index. Once it's local, agent-native questions like "moves that can paralyze a Steel-type" or "Pokémon with Levitate that don't share Charizard's Stealth-Rock weakness" become single SQL queries instead of dozens of API calls.

## Build Priorities
1. **Priority 0 — data layer for everything.** Tables for pokemon, species, types, abilities, moves, evolution chains, locations, encounters, items, generations, version groups; FTS5 index across name + flavor text. `sync` populates the lot in one pass (49 resources × ~200 entities each = ~10 000 rows, all GET, no auth — minutes-not-hours).
2. **Priority 1 — match every absorbed feature.** All 98 endpoint mirrors, plus the 5 novel features the v2.3.6 already shipped (profile / evolution / matchups / moves / team coverage), plus battle-sim from Jalajil/Poke-MCP, plus sprite handling from pokebase, plus encounter lookup from pokedex-mcp.
3. **Priority 2 — transcend.** Compound queries that only work with the whole dataset locally: reverse ability search, move-effect reverse search ("paralyze + steel"), team partner suggester, coverage-gap finder, move-learnset diff between forms, evolution-requirement reverse lookup, regional-form comparator, generation timeline.

## Phase 1.7 / 1.8 Decisions (recorded in `browser-sniff-gate.json`)
- Browser-sniff: **skip-silent** (spec is the canonical complete source).
- Crowd-sniff: skip (no gap evidence; the OpenAPI spec is more authoritative than any community SDK).

## Phase 1.9 Reachability
- **PASS.** Spec 200, API `/pokemon/1` 200 with valid payload.
