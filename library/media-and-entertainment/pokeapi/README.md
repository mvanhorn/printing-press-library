# PokéAPI CLI

**PokéAPI as a fully offline Pokédex with SQL, full-text search, type math, and a damage calculator no other Pokémon tool ships as a CLI.**

Most PokéAPI clients cache one request at a time. This CLI syncs the entire dataset to a local SQLite store and turns it into compound commands the live API can't answer: reverse ability search, move-effect filters, team partner suggestions, evolution-requirement reverse lookups, regional-form comparisons, and a Smogon-calc-style damage calculator. Plus all 98 endpoints, full-text search, and a SQL passthrough for whatever the typed commands don't cover.

Learn more at [PokéAPI](https://pokeapi.co/docs/v2).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/cmd/pokeapi-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pokeapi-current).

## Authentication

PokéAPI is a public, read-only API. No keys, no tokens, no login. Run `pokeapi-pp-cli sync` once and you have the whole Pokédex on disk.

## Quick Start

```bash
# First — fetch every resource into the local SQLite store. Takes a few minutes; you only do this once per release of the API.
pokeapi-pp-cli sync


# Core agent-friendly profile: types, abilities, stats, key moves — one local query.
pokeapi-pp-cli pokemon profile pikachu --json


# Build a balanced team starting from a partial roster. Selects only the high-gravity fields.
pokeapi-pp-cli team suggest pikachu,charizard --slots 6 --json --select name,types,score


# Reverse search by effect — moves that paralyze a Steel-type. The live API can't do this in any single call.
pokeapi-pp-cli move find --effect paralyze --type-target steel --json


# Damage calculator — expected damage range factoring in STAB, type effectiveness, level, and stats.
pokeapi-pp-cli damage charizard blastoise hydro-pump --json

```

## Unique Features

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

## Usage

Run `pokeapi-pp-cli --help` for the full command reference and flag list.

## Commands

### ability

Manage ability

- **`pokeapi-pp-cli ability list`** - Abilities provide passive effects for Pokémon in battle or in the overworld. Pokémon have multiple possible abilities but can have only one ability at a time. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Ability) for greater detail.
- **`pokeapi-pp-cli ability retrieve`** - Abilities provide passive effects for Pokémon in battle or in the overworld. Pokémon have multiple possible abilities but can have only one ability at a time. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Ability) for greater detail.

### berry

Manage berry

- **`pokeapi-pp-cli berry list`** - Berries are small fruits that can provide HP and status condition restoration, stat enhancement, and even damage negation when eaten by Pokémon. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Berry) for greater detail.
- **`pokeapi-pp-cli berry retrieve`** - Berries are small fruits that can provide HP and status condition restoration, stat enhancement, and even damage negation when eaten by Pokémon. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Berry) for greater detail.

### berry-firmness

Manage berry firmness

- **`pokeapi-pp-cli berry-firmness list`** - Berries can be soft or hard. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Category:Berries_by_firmness) for greater detail.
- **`pokeapi-pp-cli berry-firmness retrieve`** - Berries can be soft or hard. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Category:Berries_by_firmness) for greater detail.

### berry-flavor

Manage berry flavor

- **`pokeapi-pp-cli berry-flavor list`** - Flavors determine whether a Pokémon will benefit or suffer from eating a berry based on their **nature**. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Flavor) for greater detail.
- **`pokeapi-pp-cli berry-flavor retrieve`** - Flavors determine whether a Pokémon will benefit or suffer from eating a berry based on their **nature**. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Flavor) for greater detail.

### characteristic

Manage characteristic

- **`pokeapi-pp-cli characteristic list`** - Characteristics indicate which stat contains a Pokémon's highest IV. A Pokémon's Characteristic is determined by the remainder of its highest IV divided by 5 (gene_modulo). Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Characteristic) for greater detail.
- **`pokeapi-pp-cli characteristic retrieve`** - Characteristics indicate which stat contains a Pokémon's highest IV. A Pokémon's Characteristic is determined by the remainder of its highest IV divided by 5 (gene_modulo). Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Characteristic) for greater detail.

### contest-effect

Manage contest effect

- **`pokeapi-pp-cli contest-effect list`** - Contest effects refer to the effects of moves when used in contests.
- **`pokeapi-pp-cli contest-effect retrieve`** - Contest effects refer to the effects of moves when used in contests.

### contest-type

Manage contest type

- **`pokeapi-pp-cli contest-type list`** - Contest types are categories judges used to weigh a Pokémon's condition in Pokémon contests. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Contest_condition) for greater detail.
- **`pokeapi-pp-cli contest-type retrieve`** - Contest types are categories judges used to weigh a Pokémon's condition in Pokémon contests. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Contest_condition) for greater detail.

### egg-group

Manage egg group

- **`pokeapi-pp-cli egg-group list`** - Egg Groups are categories which determine which Pokémon are able to interbreed. Pokémon may belong to either one or two Egg Groups. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Egg_Group) for greater detail.
- **`pokeapi-pp-cli egg-group retrieve`** - Egg Groups are categories which determine which Pokémon are able to interbreed. Pokémon may belong to either one or two Egg Groups. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Egg_Group) for greater detail.

### encounter-condition

Manage encounter condition

- **`pokeapi-pp-cli encounter-condition list`** - Conditions which affect what pokemon might appear in the wild, e.g., day or night.
- **`pokeapi-pp-cli encounter-condition retrieve`** - Conditions which affect what pokemon might appear in the wild, e.g., day or night.

### encounter-condition-value

Manage encounter condition value

- **`pokeapi-pp-cli encounter-condition-value list`** - Encounter condition values are the various states that an encounter condition can have, i.e., time of day can be either day or night.
- **`pokeapi-pp-cli encounter-condition-value retrieve`** - Encounter condition values are the various states that an encounter condition can have, i.e., time of day can be either day or night.

### encounter-method

Manage encounter method

- **`pokeapi-pp-cli encounter-method list`** - Methods by which the player might can encounter Pokémon in the wild, e.g., walking in tall grass. Check out Bulbapedia for greater detail.
- **`pokeapi-pp-cli encounter-method retrieve`** - Methods by which the player might can encounter Pokémon in the wild, e.g., walking in tall grass. Check out Bulbapedia for greater detail.

### evolution-chain

Manage evolution chain

- **`pokeapi-pp-cli evolution-chain list`** - Evolution chains are essentially family trees. They start with the lowest stage within a family and detail evolution conditions for each as well as Pokémon they can evolve into up through the hierarchy.
- **`pokeapi-pp-cli evolution-chain retrieve`** - Evolution chains are essentially family trees. They start with the lowest stage within a family and detail evolution conditions for each as well as Pokémon they can evolve into up through the hierarchy.

### evolution-trigger

Manage evolution trigger

- **`pokeapi-pp-cli evolution-trigger list`** - Evolution triggers are the events and conditions that cause a Pokémon to evolve. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Methods_of_evolution) for greater detail.
- **`pokeapi-pp-cli evolution-trigger retrieve`** - Evolution triggers are the events and conditions that cause a Pokémon to evolve. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Methods_of_evolution) for greater detail.

### gender

Manage gender

- **`pokeapi-pp-cli gender list`** - Genders were introduced in Generation II for the purposes of breeding Pokémon but can also result in visual differences or even different evolutionary lines. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Gender) for greater detail.
- **`pokeapi-pp-cli gender retrieve`** - Genders were introduced in Generation II for the purposes of breeding Pokémon but can also result in visual differences or even different evolutionary lines. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Gender) for greater detail.

### generation

Manage generation

- **`pokeapi-pp-cli generation list`** - A generation is a grouping of the Pokémon games that separates them based on the Pokémon they include. In each generation, a new set of Pokémon, Moves, Abilities and Types that did not exist in the previous generation are released.
- **`pokeapi-pp-cli generation retrieve`** - A generation is a grouping of the Pokémon games that separates them based on the Pokémon they include. In each generation, a new set of Pokémon, Moves, Abilities and Types that did not exist in the previous generation are released.

### growth-rate

Manage growth rate

- **`pokeapi-pp-cli growth-rate list`** - Growth rates are the speed with which Pokémon gain levels through experience. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Experience) for greater detail.
- **`pokeapi-pp-cli growth-rate retrieve`** - Growth rates are the speed with which Pokémon gain levels through experience. Check out [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Experience) for greater detail.

### item

An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have various uses, including healing, powering up, helping catch Pokémon, or to access a new area.

- **`pokeapi-pp-cli item list`** - An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have various uses, including healing, powering up, helping catch Pokémon, or to access a new area.
- **`pokeapi-pp-cli item retrieve`** - An item is an object in the games which the player can pick up, keep in their bag, and use in some manner. They have various uses, including healing, powering up, helping catch Pokémon, or to access a new area.

### item-attribute

Manage item attribute

- **`pokeapi-pp-cli item-attribute list`** - Item attributes define particular aspects of items, e.g."usable in battle" or "consumable".
- **`pokeapi-pp-cli item-attribute retrieve`** - Item attributes define particular aspects of items, e.g."usable in battle" or "consumable".

### item-category

Manage item category

- **`pokeapi-pp-cli item-category list`** - Item categories determine where items will be placed in the players bag.
- **`pokeapi-pp-cli item-category retrieve`** - Item categories determine where items will be placed in the players bag.

### item-fling-effect

Manage item fling effect

- **`pokeapi-pp-cli item-fling-effect list`** - The various effects of the move"Fling" when used with different items.
- **`pokeapi-pp-cli item-fling-effect retrieve`** - The various effects of the move"Fling" when used with different items.

### item-pocket

Manage item pocket

- **`pokeapi-pp-cli item-pocket list`** - Pockets within the players bag used for storing items by category.
- **`pokeapi-pp-cli item-pocket retrieve`** - Pockets within the players bag used for storing items by category.

### language

Manage language

- **`pokeapi-pp-cli language list`** - Languages for translations of API resource information.
- **`pokeapi-pp-cli language retrieve`** - Languages for translations of API resource information.

### location

Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.

- **`pokeapi-pp-cli location list`** - Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.
- **`pokeapi-pp-cli location retrieve`** - Locations that can be visited within the games. Locations make up sizable portions of regions, like cities or routes.

### location-area

Manage location area

- **`pokeapi-pp-cli location-area list`** - Location areas are sections of areas, such as floors in a building or cave. Each area has its own set of possible Pokémon encounters.
- **`pokeapi-pp-cli location-area retrieve`** - Location areas are sections of areas, such as floors in a building or cave. Each area has its own set of possible Pokémon encounters.

### machine

Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is not certain that one specific TM or HM corresponds to a single Machine.

- **`pokeapi-pp-cli machine list`** - Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is not certain that one specific TM or HM corresponds to a single Machine.
- **`pokeapi-pp-cli machine retrieve`** - Machines are the representation of items that teach moves to Pokémon. They vary from version to version, so it is not certain that one specific TM or HM corresponds to a single Machine.

### meta

Manage meta

- **`pokeapi-pp-cli meta list`** - Returns metadata about the current deployed version of the API, including the git commit hash, deploy date, and tag (if any).

### move

Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including those learned by Hidden Machine) can be used outside of battle as well, usually for the purpose of removing obstacles or exploring new areas.

- **`pokeapi-pp-cli move list`** - Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including those learned by Hidden Machine) can be used outside of battle as well, usually for the purpose of removing obstacles or exploring new areas.
- **`pokeapi-pp-cli move retrieve`** - Moves are the skills of Pokémon in battle. In battle, a Pokémon uses one move each turn. Some moves (including those learned by Hidden Machine) can be used outside of battle as well, usually for the purpose of removing obstacles or exploring new areas.

### move-ailment

Manage move ailment

- **`pokeapi-pp-cli move-ailment list`** - Move Ailments are status conditions caused by moves used during battle. See [Bulbapedia](https://bulbapedia.bulbagarden.net/wiki/Status_condition) for greater detail.
- **`pokeapi-pp-cli move-ailment retrieve`** - Move Ailments are status conditions caused by moves used during battle. See [Bulbapedia](https://bulbapedia.bulbagarden.net/wiki/Status_condition) for greater detail.

### move-battle-style

Manage move battle style

- **`pokeapi-pp-cli move-battle-style list`** - Styles of moves when used in the Battle Palace. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Battle_Frontier_(Generation_III)) for greater detail.
- **`pokeapi-pp-cli move-battle-style retrieve`** - Styles of moves when used in the Battle Palace. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Battle_Frontier_(Generation_III)) for greater detail.

### move-category

Manage move category

- **`pokeapi-pp-cli move-category list`** - Very general categories that loosely group move effects.
- **`pokeapi-pp-cli move-category retrieve`** - Very general categories that loosely group move effects.

### move-damage-class

Manage move damage class

- **`pokeapi-pp-cli move-damage-class list`** - Damage classes moves can have, e.g. physical, special, or non-damaging.
- **`pokeapi-pp-cli move-damage-class retrieve`** - Damage classes moves can have, e.g. physical, special, or non-damaging.

### move-learn-method

Manage move learn method

- **`pokeapi-pp-cli move-learn-method list`** - Methods by which Pokémon can learn moves.
- **`pokeapi-pp-cli move-learn-method retrieve`** - Methods by which Pokémon can learn moves.

### move-target

Manage move target

- **`pokeapi-pp-cli move-target list`** - Targets moves can be directed at during battle. Targets can be Pokémon, environments or even other moves.
- **`pokeapi-pp-cli move-target retrieve`** - Targets moves can be directed at during battle. Targets can be Pokémon, environments or even other moves.

### nature

Manage nature

- **`pokeapi-pp-cli nature list`** - Natures influence how a Pokémon's stats grow. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Nature) for greater detail.
- **`pokeapi-pp-cli nature retrieve`** - Natures influence how a Pokémon's stats grow. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Nature) for greater detail.

### pal-park-area

Manage pal park area

- **`pokeapi-pp-cli pal-park-area list`** - Areas used for grouping Pokémon encounters in Pal Park. They're like habitats that are specific to Pal Park.
- **`pokeapi-pp-cli pal-park-area retrieve`** - Areas used for grouping Pokémon encounters in Pal Park. They're like habitats that are specific to Pal Park.

### pokeathlon-stat

Manage pokeathlon stat

- **`pokeapi-pp-cli pokeathlon-stat list`** - Pokeathlon Stats are different attributes of a Pokémon's performance in Pokéathlons. In Pokéathlons, competitions happen on different courses; one for each of the different Pokéathlon stats. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9athlon) for greater detail.
- **`pokeapi-pp-cli pokeathlon-stat retrieve`** - Pokeathlon Stats are different attributes of a Pokémon's performance in Pokéathlons. In Pokéathlons, competitions happen on different courses; one for each of the different Pokéathlon stats. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9athlon) for greater detail.

### pokedex

Manage pokedex

- **`pokeapi-pp-cli pokedex list`** - A Pokédex is a handheld electronic encyclopedia device; one which is capable of recording and retaining information of the various Pokémon in a given region with the exception of the national dex and some smaller dexes related to portions of a region. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pokedex) for greater detail.
- **`pokeapi-pp-cli pokedex retrieve`** - A Pokédex is a handheld electronic encyclopedia device; one which is capable of recording and retaining information of the various Pokémon in a given region with the exception of the national dex and some smaller dexes related to portions of a region. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pokedex) for greater detail.

### pokemon

Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and trained by battling with other Pokémon. Each Pokémon belongs to a specific species but may take on a variant which makes it differ from other Pokémon of the same species, such as base stats, available abilities and typings. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9mon_(species)) for greater detail.

- **`pokeapi-pp-cli pokemon list`** - Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and trained by battling with other Pokémon. Each Pokémon belongs to a specific species but may take on a variant which makes it differ from other Pokémon of the same species, such as base stats, available abilities and typings. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9mon_(species)) for greater detail.
- **`pokeapi-pp-cli pokemon retrieve`** - Pokémon are the creatures that inhabit the world of the Pokémon games. They can be caught using Pokéballs and trained by battling with other Pokémon. Each Pokémon belongs to a specific species but may take on a variant which makes it differ from other Pokémon of the same species, such as base stats, available abilities and typings. See [Bulbapedia](http://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9mon_(species)) for greater detail.

### pokemon-color

Manage pokemon color

- **`pokeapi-pp-cli pokemon-color list`** - Colors used for sorting Pokémon in a Pokédex. The color listed in the Pokédex is usually the color most apparent or covering each Pokémon's body. No orange category exists; Pokémon that are primarily orange are listed as red or brown.
- **`pokeapi-pp-cli pokemon-color retrieve`** - Colors used for sorting Pokémon in a Pokédex. The color listed in the Pokédex is usually the color most apparent or covering each Pokémon's body. No orange category exists; Pokémon that are primarily orange are listed as red or brown.

### pokemon-form

Manage pokemon form

- **`pokeapi-pp-cli pokemon-form list`** - Some Pokémon may appear in one of multiple, visually different forms. These differences are purely cosmetic. For variations within a Pokémon species, which do differ in more than just visuals, the 'Pokémon' entity is used to represent such a variety.
- **`pokeapi-pp-cli pokemon-form retrieve`** - Some Pokémon may appear in one of multiple, visually different forms. These differences are purely cosmetic. For variations within a Pokémon species, which do differ in more than just visuals, the 'Pokémon' entity is used to represent such a variety.

### pokemon-habitat

Manage pokemon habitat

- **`pokeapi-pp-cli pokemon-habitat list`** - Habitats are generally different terrain Pokémon can be found in but can also be areas designated for rare or legendary Pokémon.
- **`pokeapi-pp-cli pokemon-habitat retrieve`** - Habitats are generally different terrain Pokémon can be found in but can also be areas designated for rare or legendary Pokémon.

### pokemon-shape

Manage pokemon shape

- **`pokeapi-pp-cli pokemon-shape list`** - Shapes used for sorting Pokémon in a Pokédex.
- **`pokeapi-pp-cli pokemon-shape retrieve`** - Shapes used for sorting Pokémon in a Pokédex.

### pokemon-species

Manage pokemon species

- **`pokeapi-pp-cli pokemon-species list`** - A Pokémon Species forms the basis for at least one Pokémon. Attributes of a Pokémon species are shared across all varieties of Pokémon within the species. A good example is Wormadam; Wormadam is the species which can be found in three different varieties, Wormadam-Trash, Wormadam-Sandy and Wormadam-Plant.
- **`pokeapi-pp-cli pokemon-species retrieve`** - A Pokémon Species forms the basis for at least one Pokémon. Attributes of a Pokémon species are shared across all varieties of Pokémon within the species. A good example is Wormadam; Wormadam is the species which can be found in three different varieties, Wormadam-Trash, Wormadam-Sandy and Wormadam-Plant.

### region

Manage region

- **`pokeapi-pp-cli region list`** - A region is an organized area of the Pokémon world. Most often, the main difference between regions is the species of Pokémon that can be encountered within them.
- **`pokeapi-pp-cli region retrieve`** - A region is an organized area of the Pokémon world. Most often, the main difference between regions is the species of Pokémon that can be encountered within them.

### stat

Manage stat

- **`pokeapi-pp-cli stat list`** - Stats determine certain aspects of battles. Each Pokémon has a value for each stat which grows as they gain levels and can be altered momentarily by effects in battles.
- **`pokeapi-pp-cli stat retrieve`** - Stats determine certain aspects of battles. Each Pokémon has a value for each stat which grows as they gain levels and can be altered momentarily by effects in battles.

### super-contest-effect

Manage super contest effect

- **`pokeapi-pp-cli super-contest-effect list`** - Super contest effects refer to the effects of moves when used in super contests.
- **`pokeapi-pp-cli super-contest-effect retrieve`** - Super contest effects refer to the effects of moves when used in super contests.

### type

Manage type

- **`pokeapi-pp-cli type list`** - Types are properties for Pokémon and their moves. Each type has three properties: which types of Pokémon it is super effective against, which types of Pokémon it is not very effective against, and which types of Pokémon it is completely ineffective against.
- **`pokeapi-pp-cli type retrieve`** - Types are properties for Pokémon and their moves. Each type has three properties: which types of Pokémon it is super effective against, which types of Pokémon it is not very effective against, and which types of Pokémon it is completely ineffective against.

### version

Manage version

- **`pokeapi-pp-cli version game-list`** - Versions of the games, e.g., Red, Blue or Yellow.
- **`pokeapi-pp-cli version game-retrieve`** - Versions of the games, e.g., Red, Blue or Yellow.

### version-group

Manage version group

- **`pokeapi-pp-cli version-group list`** - Version groups categorize highly similar versions of the games.
- **`pokeapi-pp-cli version-group retrieve`** - Version groups categorize highly similar versions of the games.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pokeapi-pp-cli ability list

# JSON for scripting and agents
pokeapi-pp-cli ability list --json

# Filter to specific fields
pokeapi-pp-cli ability list --json --select id,name,status

# Dry run — show the request without sending
pokeapi-pp-cli ability list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pokeapi-pp-cli ability list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add pokeapi pokeapi-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pokeapi": {
      "command": "pokeapi-pp-mcp"
    }
  }
}
```

## Health Check

```bash
pokeapi-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/pokeapi-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Commands like `search` or `sql` return empty results.** — Run `pokeapi-pp-cli sync` first — these commands read from the local store.
- **Sync seems stuck on a single resource.** — PokéAPI is static-hosted and rarely fails, but a flaky network can stall. Re-run `pokeapi-pp-cli sync --resume` to pick up where you left off.
- **Sprite preview shows raw bytes.** — Pipe through `--ascii` for terminal-rendered art, or use `--out path.png` to save the binary.
- **A Pokémon name returns 'not found' but you spelled it right.** — PokéAPI uses dashed slugs — try `mr-mime`, `farfetchd`, or run `pokeapi-pp-cli search "<name>"` for a fuzzy match.
- **The damage calculator's numbers feel low compared to a real battle.** — The calc covers STAB + type + level + base stats. It does not model abilities, items, weather, terrain, or status — those need a full simulator like Pokémon Showdown.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**JoshGuarino/PokeGo**](https://github.com/JoshGuarino/PokeGo) — Go
- [**beastmatser/aiopokeapi**](https://github.com/beastmatser/aiopokeapi) — Python
- [**GregHilmes/pokebase**](https://github.com/GregHilmes/pokebase) — Python
- [**Jalajil/Poke-MCP**](https://github.com/Jalajil/Poke-MCP) — Python
- [**hollanddd/pokedex-mcp**](https://github.com/hollanddd/pokedex-mcp) — Python
- [**Sachin-crypto/Pokemon-MCP-Server**](https://github.com/Sachin-crypto/Pokemon-MCP-Server) — Python
- [**AmalieBjorgen/pokecli**](https://github.com/AmalieBjorgen/pokecli) — Rust
- [**hcourt/pokecli**](https://github.com/hcourt/pokecli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
