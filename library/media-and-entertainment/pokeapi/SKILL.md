---
name: pp-pokeapi
description: "PokéAPI as a fully offline Pokédex with SQL, full-text search, type math, and a damage calculator no other Pokémon tool ships as a CLI. Trigger phrases: `look up a pokemon`, `what beats charizard`, `build a pokemon team`, `type matchup`, `evolution chain`, `use pokeapi`, `run pokeapi-pp-cli`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["pokeapi-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/cmd/pokeapi-pp-cli@latest","bins":["pokeapi-pp-cli"],"label":"Install via go install"}]}}'
---

# PokéAPI — Printing Press CLI

Most PokéAPI clients cache one request at a time. This CLI syncs the entire dataset to a local SQLite store and turns it into compound commands the live API can't answer: reverse ability search, move-effect filters, team partner suggestions, evolution-requirement reverse lookups, regional-form comparisons, and a Smogon-calc-style damage calculator. Plus all 98 endpoints, full-text search, and a SQL passthrough for whatever the typed commands don't cover.

## When to Use This CLI

Reach for this CLI when you want a fully-local, fully-queryable Pokédex. It is ideal for agents answering Pokémon questions without spending live API calls, for battle-planning workflows that combine type math and learnset filtering, and for any reverse-search question (find the move, the ability, the form, the requirement) that the live REST surface doesn't natively answer. Skip it for purely image-driven workflows where another tool already handles sprite rendering.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local store that compounds
- **`pokemon by-ability`** — Find every Pokémon with a given ability. Live API has no reverse index — this is a single SQL query against the local store.

  _Reach for this when an agent needs to enumerate Pokémon by trait without making thousands of API calls._

  ```bash
  pokeapi-pp-cli pokemon by-ability levitate --json --select name,types
  ```
- **`move find`** — Find moves by status effect, damage class, type, or target — e.g. all moves that paralyze a Steel-type.

  _Use this for battle planning questions framed by effect rather than by move name._

  ```bash
  pokeapi-pp-cli move find --effect paralyze --type-target steel --json
  ```
- **`team suggest`** — Given an in-progress team, score every remaining Pokémon by how well it covers the team's typing gaps. Returns top candidates.

  _Best for team-building agents that need objective gap-coverage scoring rather than vibes-based recommendations._

  ```bash
  pokeapi-pp-cli team suggest pikachu,charizard --slots 6 --json --select name,types,score
  ```
- **`pokemon diff-learnset`** — Compare the move learnsets of two Pokémon (often regional forms or megas) and surface what each can learn that the other cannot.

  _Useful when an agent needs to argue 'why pick form X over form Y' with concrete move evidence._

  ```bash
  pokeapi-pp-cli pokemon diff-learnset charizard charizard-mega-x --json
  ```
- **`pokemon history`** — Show how a Pokémon has changed over generations: type changes, stat changes, ability changes, and which generation it was introduced in.

  _Reach for this when answering historical Pokémon questions ('was Clefairy always a Fairy type?')._

  ```bash
  pokeapi-pp-cli pokemon history clefairy --json
  ```
- **`pokemon forms`** — List every form for a species (e.g. Vulpix vs Alolan Vulpix) with type, stat, and ability deltas inline.

  _Use when an agent needs a species-wide answer rather than a single-form payload._

  ```bash
  pokeapi-pp-cli pokemon forms vulpix --json
  ```
- **`evolve into`** — Given a target Pokémon, surface the species you would need to evolve and the conditions (item, level, friendship, time of day) to get there.

  _Pick this when the user knows the Pokémon they want and needs the path to it, not the inverse._

  ```bash
  pokeapi-pp-cli evolve into umbreon --json
  ```
- **`team gaps`** — List which of the 18 types your in-progress team has neither defensive resistance nor offensive super-effectiveness against.

  _Use when answering 'what would a hostile team most likely exploit on this lineup?'_

  ```bash
  pokeapi-pp-cli team gaps pikachu,charizard,blastoise --json
  ```
- **`encounters by-region`** — Render a region-level encounter table joining locations, areas, encounters, and species — every Pokémon you can find in (e.g.) Kanto.

  _When the question is regional ('what catches in Kanto?') rather than per-Pokémon._

  ```bash
  pokeapi-pp-cli encounters by-region kanto --version red --json
  ```

### Agent-native plumbing
- **`search`** — Full-text search across Pokémon, moves, abilities, items, and locations — names plus flavor text — with relevance ranking.

  _Agent fallback when names are partial or fuzzy; replaces multiple list calls + grep._

  ```bash
  pokeapi-pp-cli search "flame" --type move --limit 10 --json
  ```
- **`sql`** — Read-only SQL access to the local store. Power users and agents can compose joins the CLI doesn't expose directly.

  _Reach for this when a one-off question doesn't have a dedicated subcommand._

  ```bash
  pokeapi-pp-cli sql "SELECT id FROM resources WHERE resource_type='pokemon' ORDER BY id LIMIT 10" --json
  ```

### Battle math
- **`damage`** — Compute expected damage range for a move from one Pokémon to another, factoring in STAB, type effectiveness, level, and base stats.

  _Reach for this on every battle-planning question framed as 'will it KO?' or 'how hard does X hit Y?'_

  ```bash
  pokeapi-pp-cli damage charizard blastoise hydro-pump --level1 50 --level2 50 --json
  ```
- **`pokemon top`** — Rank Pokémon by a base stat (attack, special-attack, speed, hp, etc.), optionally filtered by type. Returns the top N.

  _Use when learning the meta or filling a role on a team where the question is 'who's the best X-type at Y?'_

  ```bash
  pokeapi-pp-cli pokemon top --by special-attack --type ghost --limit 10 --json
  ```

## Command Reference

**ability** — Manage ability

- `pokeapi-pp-cli ability list` — Abilities provide passive effects for Pokémon in battle or in the overworld. Pokémon have multiple possible...
- `pokeapi-pp-cli ability retrieve` — Abilities provide passive effects for Pokémon in battle or in the overworld. Pokémon have multiple possible...

**berry** — Manage berry

- `pokeapi-pp-cli berry list` — Berries are small fruits that can provide HP and status condition restoration, stat enhancement, and even damage...
- `pokeapi-pp-cli berry retrieve` — Berries are small fruits that can provide HP and status condition restoration, stat enhancement, and even damage...

**berry-firmness** — Manage berry firmness

- `pokeapi-pp-cli berry-firmness list` — Berries can be soft or hard. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Category:Berries_by_firmne...
- `pokeapi-pp-cli berry-firmness retrieve` — Berries can be soft or hard. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Category:Berries_by_firmne...

**berry-flavor** — Manage berry flavor

- `pokeapi-pp-cli berry-flavor list` — Flavors determine whether a Pokémon will benefit or suffer from eating a berry based on their **nature**. Check out...
- `pokeapi-pp-cli berry-flavor retrieve` — Flavors determine whether a Pokémon will benefit or suffer from eating a berry based on their **nature**. Check out...

**characteristic** — Manage characteristic

- `pokeapi-pp-cli characteristic list` — Characteristics indicate which stat contains a Pokémon's highest IV. A Pokémon's Characteristic is determined by...
- `pokeapi-pp-cli characteristic retrieve` — Characteristics indicate which stat contains a Pokémon's highest IV. A Pokémon's Characteristic is determined by...

**contest-effect** — Manage contest effect

- `pokeapi-pp-cli contest-effect list` — Contest effects refer to the effects of moves when used in contests.
- `pokeapi-pp-cli contest-effect retrieve` — Contest effects refer to the effects of moves when used in contests.

**contest-type** — Manage contest type

- `pokeapi-pp-cli contest-type list` — Contest types are categories judges used to weigh a Pokémon's condition in Pokémon contests. Check out...
- `pokeapi-pp-cli contest-type retrieve` — Contest types are categories judges used to weigh a Pokémon's condition in Pokémon contests. Check out...

**egg-group** — Manage egg group

- `pokeapi-pp-cli egg-group list` — Egg Groups are categories which determine which Pokémon are able to interbreed. Pokémon may belong to either one...
- `pokeapi-pp-cli egg-group retrieve` — Egg Groups are categories which determine which Pokémon are able to interbreed. Pokémon may belong to either one...

**encounter-condition** — Manage encounter condition

- `pokeapi-pp-cli encounter-condition list` — Conditions which affect what pokemon might appear in the wild, e.g., day or night.
- `pokeapi-pp-cli encounter-condition retrieve` — Conditions which affect what pokemon might appear in the wild, e.g., day or night.

**encounter-condition-value** — Manage encounter condition value

- `pokeapi-pp-cli encounter-condition-value list` — Encounter condition values are the various states that an encounter condition can have, i.e., time of day can be...
- `pokeapi-pp-cli encounter-condition-value retrieve` — Encounter condition values are the various states that an encounter condition can have, i.e., time of day can be...

**encounter-method** — Manage encounter method

- `pokeapi-pp-cli encounter-method list` — Methods by which the player might can encounter Pokémon in the wild, e.g., walking in tall grass. Check out...
- `pokeapi-pp-cli encounter-method retrieve` — Methods by which the player might can encounter Pokémon in the wild, e.g., walking in tall grass. Check out...

**evolution-chain** — Manage evolution chain

- `pokeapi-pp-cli evolution-chain list` — Evolution chains are essentially family trees. They start with the lowest stage within a family and detail evolution...
- `pokeapi-pp-cli evolution-chain retrieve` — Evolution chains are essentially family trees. They start with the lowest stage within a family and detail evolution...

**evolution-trigger** — Manage evolution trigger

- `pokeapi-pp-cli evolution-trigger list` — Evolution triggers are the events and conditions that cause a Pokémon to evolve. Check out...
- `pokeapi-pp-cli evolution-trigger retrieve` — Evolution triggers are the events and conditions that cause a Pokémon to evolve. Check out...

**gender** — Manage gender

- `pokeapi-pp-cli gender list` — Genders were introduced in Generation II for the purposes of breeding Pokémon but can also result in visual...
- `pokeapi-pp-cli gender retrieve` — Genders were introduced in Generation II for the purposes of breeding Pokémon but can also result in visual...

**generation** — Manage generation

- `pokeapi-pp-cli generation list` — A generation is a grouping of the Pokémon games that separates them based on the Pokémon they include. In each...
- `pokeapi-pp-cli generation retrieve` — A generation is a grouping of the Pokémon games that separates them based on the Pokémon they include. In each...

**growth-rate** — Manage growth rate

- `pokeapi-pp-cli growth-rate list` — Growth rates are the speed with which Pokémon gain levels through experience. Check out...
- `pokeapi-pp-cli growth-rate retrieve` — Growth rates are the speed with which Pokémon gain levels through experience. Check out...

**item** — An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have various uses, including healing, powering up, helping catch Pokémon, or to access a new area.

- `pokeapi-pp-cli item list` — An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have...
- `pokeapi-pp-cli item retrieve` — An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have...

**item-attribute** — Manage item attribute

- `pokeapi-pp-cli item-attribute list` — Item attributes define particular aspects of items, e.g.'usable in battle' or 'consumable'.
- `pokeapi-pp-cli item-attribute retrieve` — Item attributes define particular aspects of items, e.g.'usable in battle' or 'consumable'.

**item-category** — Manage item category

- `pokeapi-pp-cli item-category list` — Item categories determine where items will be placed in the players bag.
- `pokeapi-pp-cli item-category retrieve` — Item categories determine where items will be placed in the players bag.

**item-fling-effect** — Manage item fling effect

- `pokeapi-pp-cli item-fling-effect list` — The various effects of the move'Fling' when used with different items.
- `pokeapi-pp-cli item-fling-effect retrieve` — The various effects of the move'Fling' when used with different items.

**item-pocket** — Manage item pocket

- `pokeapi-pp-cli item-pocket list` — Pockets within the players bag used for storing items by category.
- `pokeapi-pp-cli item-pocket retrieve` — Pockets within the players bag used for storing items by category.

**language** — Manage language

- `pokeapi-pp-cli language list` — Languages for translations of API resource information.
- `pokeapi-pp-cli language retrieve` — Languages for translations of API resource information.

**location** — Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.

- `pokeapi-pp-cli location list` — Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.
- `pokeapi-pp-cli location retrieve` — Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.

**location-area** — Manage location area

- `pokeapi-pp-cli location-area list` — Location areas are sections of areas, such as floors in a building or cave. Each area has its own set of possible...
- `pokeapi-pp-cli location-area retrieve` — Location areas are sections of areas, such as floors in a building or cave. Each area has its own set of possible...

**machine** — Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is not certain that one specific TM or HM corresponds to a single Machine.

- `pokeapi-pp-cli machine list` — Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is...
- `pokeapi-pp-cli machine retrieve` — Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is...

**meta** — Manage meta

- `pokeapi-pp-cli meta` — Returns metadata about the current deployed version of the API, including the git commit hash, deploy date, and tag...

**move** — Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including those learned by Hidden Machine) can be used outside of battle as well, usually for the purpose of removing obstacles or exploring new areas.

- `pokeapi-pp-cli move list` — Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including...
- `pokeapi-pp-cli move retrieve` — Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including...

**move-ailment** — Manage move ailment

- `pokeapi-pp-cli move-ailment list` — Move Ailments are status conditions caused by moves used during battle. See...
- `pokeapi-pp-cli move-ailment retrieve` — Move Ailments are status conditions caused by moves used during battle. See...

**move-battle-style** — Manage move battle style

- `pokeapi-pp-cli move-battle-style list` — Styles of moves when used in the Battle Palace. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Battle_Fronti...
- `pokeapi-pp-cli move-battle-style retrieve` — Styles of moves when used in the Battle Palace. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Battle_Fronti...

**move-category** — Manage move category

- `pokeapi-pp-cli move-category list` — Very general categories that loosely group move effects.
- `pokeapi-pp-cli move-category retrieve` — Very general categories that loosely group move effects.

**move-damage-class** — Manage move damage class

- `pokeapi-pp-cli move-damage-class list` — Damage classes moves can have, e.g. physical, special, or non-damaging.
- `pokeapi-pp-cli move-damage-class retrieve` — Damage classes moves can have, e.g. physical, special, or non-damaging.

**move-learn-method** — Manage move learn method

- `pokeapi-pp-cli move-learn-method list` — Methods by which Pokémon can learn moves.
- `pokeapi-pp-cli move-learn-method retrieve` — Methods by which Pokémon can learn moves.

**move-target** — Manage move target

- `pokeapi-pp-cli move-target list` — Targets moves can be directed at during battle. Targets can be Pokémon, environments or even other moves.
- `pokeapi-pp-cli move-target retrieve` — Targets moves can be directed at during battle. Targets can be Pokémon, environments or even other moves.

**nature** — Manage nature

- `pokeapi-pp-cli nature list` — Natures influence how a Pokémon's stats grow. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Nature) for...
- `pokeapi-pp-cli nature retrieve` — Natures influence how a Pokémon's stats grow. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Nature) for...

**pal-park-area** — Manage pal park area

- `pokeapi-pp-cli pal-park-area list` — Areas used for grouping Pokémon encounters in Pal Park. They're like habitats that are specific to Pal Park.
- `pokeapi-pp-cli pal-park-area retrieve` — Areas used for grouping Pokémon encounters in Pal Park. They're like habitats that are specific to Pal Park.

**pokeathlon-stat** — Manage pokeathlon stat

- `pokeapi-pp-cli pokeathlon-stat list` — Pokeathlon Stats are different attributes of a Pokémon's performance in Pokéathlons. In Pokéathlons, competitions...
- `pokeapi-pp-cli pokeathlon-stat retrieve` — Pokeathlon Stats are different attributes of a Pokémon's performance in Pokéathlons. In Pokéathlons, competitions...

**pokedex** — Manage pokedex

- `pokeapi-pp-cli pokedex list` — A Pokédex is a handheld electronic encyclopedia device; one which is capable of recording and retaining information...
- `pokeapi-pp-cli pokedex retrieve` — A Pokédex is a handheld electronic encyclopedia device; one which is capable of recording and retaining information...

**pokemon** — Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and trained by battling with other Pokémon. Each Pokémon belongs to a specific species but may take on a variant which makes it differ from other Pokémon of the same species, such as base stats, available abilities and typings. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9mon_(species)) for greater detail.

- `pokeapi-pp-cli pokemon list` — Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and...
- `pokeapi-pp-cli pokemon retrieve` — Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and...

**pokemon-color** — Manage pokemon color

- `pokeapi-pp-cli pokemon-color list` — Colors used for sorting Pokémon in a Pokédex. The color listed in the Pokédex is usually the color most apparent...
- `pokeapi-pp-cli pokemon-color retrieve` — Colors used for sorting Pokémon in a Pokédex. The color listed in the Pokédex is usually the color most apparent...

**pokemon-form** — Manage pokemon form

- `pokeapi-pp-cli pokemon-form list` — Some Pokémon may appear in one of multiple, visually different forms. These differences are purely cosmetic. For...
- `pokeapi-pp-cli pokemon-form retrieve` — Some Pokémon may appear in one of multiple, visually different forms. These differences are purely cosmetic. For...

**pokemon-habitat** — Manage pokemon habitat

- `pokeapi-pp-cli pokemon-habitat list` — Habitats are generally different terrain Pokémon can be found in but can also be areas designated for rare or...
- `pokeapi-pp-cli pokemon-habitat retrieve` — Habitats are generally different terrain Pokémon can be found in but can also be areas designated for rare or...

**pokemon-shape** — Manage pokemon shape

- `pokeapi-pp-cli pokemon-shape list` — Shapes used for sorting Pokémon in a Pokédex.
- `pokeapi-pp-cli pokemon-shape retrieve` — Shapes used for sorting Pokémon in a Pokédex.

**pokemon-species** — Manage pokemon species

- `pokeapi-pp-cli pokemon-species list` — A Pokémon Species forms the basis for at least one Pokémon. Attributes of a Pokémon species are shared across all...
- `pokeapi-pp-cli pokemon-species retrieve` — A Pokémon Species forms the basis for at least one Pokémon. Attributes of a Pokémon species are shared across all...

**region** — Manage region

- `pokeapi-pp-cli region list` — A region is an organized area of the Pokémon world. Most often, the main difference between regions is the species...
- `pokeapi-pp-cli region retrieve` — A region is an organized area of the Pokémon world. Most often, the main difference between regions is the species...

**stat** — Manage stat

- `pokeapi-pp-cli stat list` — Stats determine certain aspects of battles. Each Pokémon has a value for each stat which grows as they gain levels...
- `pokeapi-pp-cli stat retrieve` — Stats determine certain aspects of battles. Each Pokémon has a value for each stat which grows as they gain levels...

**super-contest-effect** — Manage super contest effect

- `pokeapi-pp-cli super-contest-effect list` — Super contest effects refer to the effects of moves when used in super contests.
- `pokeapi-pp-cli super-contest-effect retrieve` — Super contest effects refer to the effects of moves when used in super contests.

**type** — Manage type

- `pokeapi-pp-cli type list` — Types are properties for Pokémon and their moves. Each type has three properties: which types of Pokémon it is...
- `pokeapi-pp-cli type retrieve` — Types are properties for Pokémon and their moves. Each type has three properties: which types of Pokémon it is...

**version** — Manage version

- `pokeapi-pp-cli version game-list` — Versions of the games, e.g., Red, Blue or Yellow.
- `pokeapi-pp-cli version game-retrieve` — Versions of the games, e.g., Red, Blue or Yellow.

**version-group** — Manage version group

- `pokeapi-pp-cli version-group list` — Version groups categorize highly similar versions of the games.
- `pokeapi-pp-cli version-group retrieve` — Version groups categorize highly similar versions of the games.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pokeapi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### What does my team leave exposed?

```bash
pokeapi-pp-cli team gaps pikachu,charizard,blastoise --json
```

Returns every type your team has neither resistance nor super-effective answer for. One local join.

### Reverse-search by effect

```bash
pokeapi-pp-cli move find --effect paralyze --type-target steel --json --select name,type,power,accuracy
```

Find paralyzing moves that hit Steel — using --select to keep the response narrow for agent context.

### Trim a profile to just what an agent needs

```bash
pokeapi-pp-cli pokemon profile pikachu --json --select name,types,stats.hp,stats.attack,abilities
```

Use --select with dotted paths to pluck only high-gravity fields out of a deeply nested response — keeps the agent's context light.

### Compare two regional forms

```bash
pokeapi-pp-cli pokemon diff-learnset vulpix vulpix-alola --json
```

Side-by-side learnset diff between Vulpix and Alolan Vulpix. One local query instead of two full API fetches.

### Custom join via SQL

```bash
pokeapi-pp-cli sql "SELECT id FROM resources WHERE resource_type='pokemon-species' ORDER BY id LIMIT 10" --json
```

Drop down to raw SQL when there's no first-class command — read-only, direct local-store access.

## Auth Setup

No authentication required.

Run `pokeapi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pokeapi-pp-cli ability list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pokeapi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pokeapi-pp-cli feedback --stdin < notes.txt
pokeapi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pokeapi-pp-cli/feedback.jsonl`. They are never POSTed unless `POKEAPI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POKEAPI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
pokeapi-pp-cli profile save briefing --json
pokeapi-pp-cli --profile briefing ability list
pokeapi-pp-cli profile list --json
pokeapi-pp-cli profile show briefing
pokeapi-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `pokeapi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/cmd/pokeapi-pp-cli@latest
   ```
3. Verify: `pokeapi-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/cmd/pokeapi-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pokeapi-pp-mcp -- pokeapi-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pokeapi-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pokeapi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pokeapi-pp-cli <command> --help`.
