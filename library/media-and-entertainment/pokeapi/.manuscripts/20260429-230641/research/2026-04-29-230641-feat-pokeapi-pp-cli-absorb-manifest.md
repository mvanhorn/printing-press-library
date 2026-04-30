# PokéAPI CLI Absorb Manifest

## Sources surveyed

| Tool | Type | Highlights |
|---|---|---|
| **PokeAPI/pokeapi** (official) | OpenAPI 3.1 spec | 98 GET endpoints across 49 resources |
| **JoshGuarino/PokeGo** | Go SDK | 11 resource groups, 24-h in-memory cache |
| **beastmatser/aiopokeapi** | Python async SDK | Auto object cache, lazy `await fetch()` |
| **GregHilmes/pokebase** | Python SDK | `APIResource` lazy-loading, `SpriteResource` binary cache |
| **mtslzr/pokeapi-go** (used by pokecli-go) | Go SDK | Common Go reference |
| **pokeapi-js-wrapper** | JS browser SDK | Image caching via Service Worker |
| **pokeapi-typescript** | TS SDK | Promise + Collections cache |
| **mvanhorn/printing-press-library — pokeapi v2.3.6** | Existing CLI in public library | 97 MCP tools, 5 novel features (profile, evolution, matchups, moves, team coverage) |
| **NaveenBandarage/poke-mcp** | MCP server | Basic Pokemon info via MCP |
| **Jalajil/Poke-MCP** | MCP server | Battle-sim with STAB/type/status/crit |
| **hollanddd/pokedex-mcp** | MCP server | `fetch_pokemon`, `get_type_effectiveness`, `get_pokemon_encounters`, `search_pokemon` |
| **Sachin-crypto/Pokemon-MCP-Server** | MCP server | Detailed info, popular picks, tournament squad |
| **AmalieBjorgen/pokecli** | Rust CLI | TUI-style profile lookup |
| **hcourt/pokecli** | Go CLI | Search by type/name, show entity info |
| **mariusavram91/pokestats**, **kaseypcantu/pokeapi-pokedex**, **Hartesic/pokestats** | Tiny CLIs | Single-Pokemon stat lookup |

---

## Absorbed (match or beat everything that exists)

### Endpoint coverage (Priority 1 — generator emits all 98)

Each row below stands for a `list` + `retrieve` pair generated from the spec. The generator emits MCP endpoint mirrors for all of them automatically.

| # | Resource group | Endpoints | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|---|
| 1 | pokemon | list + retrieve + encounters (3) | PokeGo, all SDKs | Spec-generated cobra commands + SQLite store | Offline FTS, --json/--select/--csv, MCP-mirrored, idempotent |
| 2 | pokemon-species | list + retrieve | PokeGo | Spec-generated | Same |
| 3 | pokemon-form | list + retrieve | PokeGo | Spec-generated | Same |
| 4 | move | list + retrieve | All | Spec-generated | Same |
| 5 | type | list + retrieve | All | Spec-generated | Same |
| 6 | ability | list + retrieve | All | Spec-generated | Same |
| 7 | item | list + retrieve | All | Spec-generated | Same |
| 8 | location + location-area | list + retrieve (2 resources) | All | Spec-generated | Same |
| 9 | evolution-chain + evolution-trigger | list + retrieve | aiopokeapi | Spec-generated | Same |
| 10 | berry + berry-firmness + berry-flavor | list + retrieve (3 resources) | All | Spec-generated | Same |
| 11 | contest-effect + contest-type + super-contest-effect | list + retrieve (3 resources) | All | Spec-generated | Same |
| 12 | encounter-condition / -value / -method | list + retrieve (3 resources) | All | Spec-generated | Same |
| 13 | game / generation / version / version-group / pokedex / region | list + retrieve (6 resources) | All | Spec-generated | Same |
| 14 | machine | list + retrieve | All | Spec-generated | Same |
| 15 | move-ailment / -battle-style / -category / -damage-class / -learn-method / -target | list + retrieve (6 resources) | All | Spec-generated | Same |
| 16 | nature / stat / characteristic / gender / growth-rate / egg-group / pokeathlon-stat | list + retrieve (7 resources) | All | Spec-generated | Same |
| 17 | pokemon-color / -habitat / -shape | list + retrieve (3 resources) | All | Spec-generated | Same |
| 18 | item-attribute / -category / -fling-effect / -pocket | list + retrieve (4 resources) | All | Spec-generated | Same |
| 19 | language | list + retrieve | All | Spec-generated | Same |
| 20 | pal-park-area | list + retrieve | All | Spec-generated | Same |
| 21 | meta | retrieve only | Official spec | Spec-generated | Same |

→ **All 98 endpoints get spec-generated commands + MCP tools automatically.** That alone meets the absorbed-features bar.

### High-value compound features (already in v2.3.6 — must match)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 22 | Pokémon profile aggregation | mvanhorn/pokeapi v2.3.6 (`pokemon profile`); aiopokeapi `await fetch()` chains | `pokemon profile <name>` — one local-store join across pokemon + species + types + abilities + stats + moves | Offline; --json/--select/--compact; MCP-tagged read-only; one command instead of 7 sequential API calls |
| 23 | Evolution chain traversal | v2.3.6 (`pokemon evolution`); aiopokeapi | `pokemon evolution <name>` — species → chain → readable path with trigger conditions inline | Renders branch trees (eevee → 9 evolutions) without 9 separate fetches |
| 24 | Type matchups | v2.3.6 (`pokemon matchups`); pokedex-mcp `get_type_effectiveness`; Jalajil battle math | `pokemon matchups <name>` — defensive (4× / 2× / ½× / ¼× / 0×) + offensive (super-effective coverage) | Computed locally from `damage_relations`; --json shape stable across runs |
| 25 | Move filtering | v2.3.6 (`pokemon moves`) | `pokemon moves <name> --method level-up --version-group red-blue --max-level 30` | Add `--max-level` and `--min-level` not in v2.3.6 |
| 26 | Team coverage | v2.3.6 (`team coverage`); Sachin-crypto MCP "tournament squad" | `team coverage <p1>,<p2>,...` — shared weaknesses/resistances/immunities + offensive holes | Same shape as v2.3.6 to avoid breaking existing users; add SQL composability via `--json` |

### Compound features in other tools (must match)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 27 | Battle simulation (STAB, type, status, crit, speed-order) | Jalajil/Poke-MCP | `battle sim <p1> <p2> [--level1 N --level2 N --moves1 csv --moves2 csv --seed N]` | First CLI battle-sim; deterministic with `--seed`; local-store reads; --json turn log |
| 28 | Encounter location lookup | hollanddd/pokedex-mcp `get_pokemon_encounters` | `pokemon encounters <name>` — location-areas + version + method + level range + chance | Already in spec as a sub-path — first-class subcommand instead of buried under retrieve |
| 29 | Sprite retrieval & cache | GregHilmes/pokebase `SpriteResource`; pokeapi-js-wrapper Service Worker | `sprite get <name> [--variant default|female|back|shiny|official-artwork] [--out path] [--ascii]` | First CLI to ship sprite download + a small ASCII renderer for terminal preview |
| 30 | Partial-name search | hollanddd/pokedex-mcp `search_pokemon`; pokeapi-go | `search "<query>"` powered by FTS5 across pokemon/move/ability/item/location names + descriptions | Cross-resource search with relevance ranking; offline |
| 31 | "Popular picks" / curated lists | Sachin-crypto MCP | `pokemon popular [--region kanto] [--limit N]` reading a curated table from a static asset | Honest static curation (top-of-Smogon-OU, sample roster); no random data |

### Plumbing absorbs

| # | Feature | Best Source | Our Implementation |
|---|---|---|---|
| 32 | In-memory cache w/ TTL | PokeGo, all wrappers | Generator-emitted SQLite store ≫ any per-process cache |
| 33 | Lazy related-resource fetch | aiopokeapi, pokebase | Local-store joins replace lazy fetches entirely |
| 34 | Configurable base URL | PokeGo | `--base-url` global flag (config file + env override) |
| 35 | Pagination | All | Spec-derived `--limit` / `--offset`; agent-friendly `--all` for sync paths |

---

## Transcendence (only possible with our approach)

Each feature below is unreachable for the existing v2.3.6 CLI (which is endpoint-mirror + 5 aggregations) and unreachable for any wrapper (no full local index, no SQL, no FTS).

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| **N1** | Reverse-ability search | `pokemon by-ability <ability>` | Need `pokemon ↔ abilities` join across all ~1 200 Pokémon. Live API offers no reverse index. | 9/10 |
| **N2** | Reverse move-effect search | `move find --effect paralyze --type-target steel` | Joins moves on `meta.ailment.name`, `damage_class`, and computes type vs. type-target damage relations from local store. No API endpoint exists for this. | 9/10 |
| **N3** | Team partner suggester | `team suggest <pokemon1>[,pokemon2,...] [--slots 6] [--region]` | Computes coverage gaps for an in-progress team, scores remaining 1 200 Pokémon by how well each fills the gap, surfaces top N. Needs full pokémon×types×stats join, in O(n) over a local table. | 8/10 |
| **N4** | Move-learnset diff between forms | `pokemon diff-learnset <form1> <form2>` (e.g. `charizard`/`charizard-mega-x`) | Side-by-side learn-set comparison across two `pokemon` rows, joining `pokemon_moves`. Live API forces 2× full fetches and manual diff. | 8/10 |
| **N5** | Cross-generation timeline for a Pokémon | `pokemon history <name>` | Stitches `pokemon-species → generation`, `past-types`, `past-stats`, and `past-abilities` into a single timeline. Multiple API hops collapsed to one local query. | 7/10 |
| **N6** | Regional-form comparator | `pokemon forms <species>` | Lists every form (e.g. Vulpix → Vulpix + Alolan Vulpix), shows type / stat / ability deltas inline. | 7/10 |
| **N7** | Evolution-requirement reverse lookup | `evolve into <name>` | Searches `evolution_chain` rows for any chain whose target species matches `<name>`, surfaces required item / level / friendship / time-of-day. | 8/10 |
| **N8** | Coverage-gap finder for an in-progress team | `team gaps <p1>,<p2>,...` | Computes which of 18 types your team has neither resistance nor super-effective answer for, suggests Pokémon to plug each gap. | 8/10 |
| **N9** | Local FTS search w/ relevance ranking | `search "<query>"` | SQLite FTS5 over Pokémon + move + ability + item + location names + flavor text. No competitor ships this. | 9/10 |
| **N10** | SQL passthrough | `sql "<query>"` | Direct read-only access to the local store for power users / agents. | 7/10 |
| **N11** | Encounter map for a region | `encounters by-region <region>` | Joins `location ↔ location-area ↔ encounters ↔ pokemon-species` to render a region-level encounter table — impossible against the live API in fewer than ~40 calls. | 7/10 |
| **D1** | Damage calculator | `damage <attacker> <defender> <move> [--level1 N --level2 N]` | Smogon-calc-equivalent. Computes expected damage range using STAB, type effectiveness, level, and base stats from the local store. THE most-used competitive Pokémon tool — a damage calc is what teambuilders consult before every move choice. | 9/10 |
| **D2** | Stat ranker | `pokemon top --by <stat> [--type <t>] [--limit N]` | "Top 10 special attackers among Ghost-types." Single SQL query against the local pokemon table; live API offers no sort + filter. Real teambuilding meta question, distinct from `by-ability`. | 8/10 |

→ **13 transcendence features**, of which 13 score ≥ 7/10. Floor for inclusion in `research.json` `novel_features` is 5/10 — all 13 qualify.

### Why no battle simulator

A naive Pokémon battle simulator (originally proposed as N12) was dropped on critical review. Pokémon Showdown already provides a free, web-based, mechanically accurate simulator with abilities, items, weather, terrain, status interactions, and modern mechanics. A simulator built from PokeAPI alone would be strictly less accurate than Showdown, and competitive players who care about battle outcomes use damage calculators (covered by D1) for decision-making, not full simulations. Inclusion should be value-based, not possibility-based.

### Stub status

- **No stubs.** Every transcendence feature can be built fully against the local SQLite store; PokeAPI is read-only, has no auth, has no rate limit, and has < 10 000 documents.

---

## Group themes (for `narrative.novel_features.group`)

- **"Local store that compounds"** — N1, N2, N3, N4, N5, N6, N7, N8, N11 (anything that joins multiple resource groups locally).
- **"Agent-native plumbing"** — N9 (FTS search), N10 (SQL passthrough).
- **"Battle math"** — D1 (damage calc), D2 (stat ranker).

---

## Match summary

- **v2.3.6 had:** 5 novel features.
- **We will ship:** 5 v2.3.6 features (matched) + 13 transcendence features = **18 high-value compound features**, all backed by a local SQLite store none of the existing tools have. Each transcendence feature was filtered through a "real audience demand" review pass — speculative ones (battle simulator) were dropped in favor of features with clear community traction (damage calculator, stat ranker).
- **Endpoint surface:** 98 spec-generated commands (matches v2.3.6's 97 MCP tools, plus the runtime cobratree mirror exposes them all as MCP tools automatically with `mcp:read-only` annotations).
- **Total commands shipped:** ~98 endpoint-mirror + 17 compound + framework (`auth`, `doctor`, `version`, `sync`, `search`, `sql`, `context`) = ~120 commands.
