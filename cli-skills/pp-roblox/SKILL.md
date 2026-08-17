---
name: pp-roblox
description: "Investigate Roblox's fragmented public web data from one agent-friendly CLI. Trigger phrases: `look up a Roblox user`, `investigate a Roblox group`, `compare Roblox users`, `research a Roblox game`, `use Roblox`, `run Roblox`."
author: "Kieran Maynard"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - roblox-pp-cli
    install:
      - kind: go
        bins: [roblox-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/cmd/roblox-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/social-and-messaging/roblox/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Roblox — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `roblox-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install roblox --cli-only
   ```
2. Verify: `roblox-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/cmd/roblox-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Query public users, groups, avatars, games, badges, inventory, catalog, and thumbnails across Roblox API hosts. Cross-resource investigation commands and a local SQLite store turn isolated responses into reproducible research workflows.

## When to Use This CLI

Use this CLI for public Roblox identity, group, game, catalog, badge, inventory, presence, or thumbnail research and for repeatable agent workflows over those records. Prefer its investigation and local-history commands when a task spans multiple Roblox API hosts.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI instead of Roblox Studio for experience development.
- Do not use anonymous commands for account mutations that require Cookie and CSRF state.
- Do not treat this legacy public-data CLI as the Roblox Open Cloud creator-administration API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-resource investigations
- **`investigate user`** — Assemble a public Roblox user's identity, avatar, groups, badges, inventory visibility, and thumbnails in one result.

  _Use this when an agent needs broad public context for one Roblox identity rather than a single record._

  ```bash
  roblox-pp-cli investigate user 1 --agent
  ```
- **`investigate group`** — Resolve a Roblox group, its owner, roles, and other public context in one command.

  _Use this for repeatable public ownership and affiliation checks on a Roblox group._

  ```bash
  roblox-pp-cli investigate group 1 --agent
  ```
- **`catalog creator-footprint`** — Summarize a Roblox creator's public games, bundles, and catalog-facing identity in one result.

  _Use this for creator or competitor research that spans more than one catalog endpoint._

  ```bash
  roblox-pp-cli catalog creator-footprint 1 --agent
  ```
- **`ecosystem game`** — Build a game-centered view of public universe details, creator context, media, and related badges.

  _Use this for game or competitor research that spans more than a single universe response._

  ```bash
  roblox-pp-cli ecosystem game 1534453623 --agent
  ```

### Local relationship analysis
- **`network overlap`** — Find shared public groups and relationship records between two locally synced Roblox users.

  _Use this when comparing two Roblox identities and their observable public affiliations._

  ```bash
  roblox-pp-cli network overlap --user-a 1 --user-b 156 --agent
  ```

## Command Reference

**asset-thumbnail-animated** — Manage asset thumbnail animated

- `roblox-pp-cli asset-thumbnail-animated` — Thumbnails asset animated.

**asset-to-category** — Manage asset to category

- `roblox-pp-cli asset-to-category` — Lists a mapping for assets to category IDs to convert from inventory ID to catalog ID.

**asset-to-subcategory** — Manage asset to subcategory

- `roblox-pp-cli asset-to-subcategory` — Lists a mapping for assets to subcategory IDs to convert from inventory ID to catalog ID.

**assets** — Manage assets

- `roblox-pp-cli assets` — Thumbnails assets.

**assets-thumbnail-3d** — Manage assets thumbnail 3d

- `roblox-pp-cli assets-thumbnail-3d` — Thumbnails assets.

**avatar** — Manage avatar

- `roblox-pp-cli avatar create` — Requests the authenticated user's thumbnail be redrawn.
- `roblox-pp-cli avatar create-setbodycolors` — Sets the authenticated user's body colors.
- `roblox-pp-cli avatar create-setplayeravatartype` — This is the avatar type chosen on the Avatar page.
- `roblox-pp-cli avatar create-setscales` — Sets the authenticated user's scales.
- `roblox-pp-cli avatar list` — Returns details about the authenticated user's avatar.
- `roblox-pp-cli avatar list-metadata` — Returns metadata used by the avatar page of the website.

**avatar-2** — Manage avatar 2

- `roblox-pp-cli avatar-2 create` — Sets the authenticated user's body colors.
- `roblox-pp-cli avatar-2 create-setwearingassets` — Only allows items that you own, are not expired, and are wearable asset types.
- `roblox-pp-cli avatar-2 get` — Includes assets, bodycolors, and playerAvatarType.
- `roblox-pp-cli avatar-2 get-users` — Gets a list of outfits for the specified user.
- `roblox-pp-cli avatar-2 list` — Returns details about the authenticated user's avatar.

**avatar-outfits** — Manage avatar outfits

- `roblox-pp-cli avatar-outfits create` — Fails if any of the assetIds are not owned by the user, or not wearable types.
- `roblox-pp-cli avatar-outfits update` — Fails if the user does not own any of the assetIds or if they are not wearable asset types. Accepts partial updates.

**avatar-rules** — Manage avatar rules

- `roblox-pp-cli avatar-rules` — BodyColorsPalette is a list of valid brickColors you can choose for your avatar.

**avatar-users** — Manage avatar users


**badges** — Manage badges

- `roblox-pp-cli badges` — Thumbnails badge icons.

**badges-2** — Manage badges 2

- `roblox-pp-cli badges-2 get` — Gets badge information by the badge Id.
- `roblox-pp-cli badges-2 list` — Gets metadata about the badges system.
- `roblox-pp-cli badges-2 update` — Updates badge configuration.

**badges-user** — Manage badges user

- `roblox-pp-cli badges-user <badgeId>` — Removes a badge from the authenticated user.

**badges-users** — Manage badges users


**batch** — Manage batch

- `roblox-pp-cli batch` — Returns a list of thumbnails with varying types and sizes

**birthdate** — Manage birthdate

- `roblox-pp-cli birthdate create` — Update the user's birthdate
- `roblox-pp-cli birthdate list` — Get the user's birthdate

**bundles** — Manage bundles

- `roblox-pp-cli bundles` — Get bundle thumbnails for the given CSV of bundle ids

**catalog** — Manage catalog

- `roblox-pp-cli catalog` — There is an item count limit per request. Exceeding this returns 400 Bad Request.

**catalog-assets** — Manage catalog assets


**catalog-bundles** — Manage catalog bundles

- `roblox-pp-cli catalog-bundles` — Returns details about the given bundleIds.

**catalog-search** — Manage catalog search

- `roblox-pp-cli catalog-search` — This endpoint is for search by item type ids.

**catalog-users** — Manage catalog users


**categories** — Manage categories

- `roblox-pp-cli categories` — Lists Category Names and their Ids.

**collectible-items** — Manage collectible items


**collections** — Manage collections

- `roblox-pp-cli collections create` — Adds an item to the appropriate collection
- `roblox-pp-cli collections delete` — Removes an item to the appropriate collection

**description** — Manage description

- `roblox-pp-cli description create` — Update the user's description
- `roblox-pp-cli description list` — Get the user's description

**developer-products** — Manage developer products

- `roblox-pp-cli developer-products` — Thumbnails developer product icons.

**display-names** — Manage display names

- `roblox-pp-cli display-names` — Validate a display name for a new user.

**economy-user** — Manage economy user

- `roblox-pp-cli economy-user` — Currency can only be retrieved for the authenticated user.

**favorites** — Manage favorites

- `roblox-pp-cli favorites create` — Create a favorite for an asset by the authenticated user.
- `roblox-pp-cli favorites create-users` — Create a favorite for the bundle by the authenticated user.
- `roblox-pp-cli favorites delete` — Delete a favorite for an asset by the authenticated user.
- `roblox-pp-cli favorites delete-users` — Delete favorite for the bundle by the authenticated user.
- `roblox-pp-cli favorites get` — Gets the favorite count for the given asset Id.
- `roblox-pp-cli favorites get-bundles` — Gets the favorite count for the given bundle Id.
- `roblox-pp-cli favorites get-users` — Gets the favorite model for the asset and user.
- `roblox-pp-cli favorites get-users-2` — Gets the favorite model for the bundle and user.
- `roblox-pp-cli favorites get-users-3` — Lists the marketplace assets favorited by a given user with the given assetTypeId.
- `roblox-pp-cli favorites get-users-4` — Lists the bundles favorited by a given user with the given bundle subtypeId.

**featured-content** — Manage featured content

- `roblox-pp-cli featured-content create` — Sets the featured event for a group
- `roblox-pp-cli featured-content delete` — Deletes the featured event for a group
- `roblox-pp-cli featured-content list` — Gets the featured event for a group

**friends-users** — Manage friends users


**game-passes** — Manage game passes

- `roblox-pp-cli game-passes` — Thumbnails game pass icons.

**game-start-info** — Manage game start info

- `roblox-pp-cli game-start-info` — The server will call this on game server start to request general information about the universe This is version 1.

**games** — Manage games

- `roblox-pp-cli games get` — Get games recommendations based on a given universe
- `roblox-pp-cli games list` — Gets a list of games' detail
- `roblox-pp-cli games list-gamesproductinfo` — Gets a list of games' product info, used to purchase a game
- `roblox-pp-cli games list-multigetplacedetails` — Get place details
- `roblox-pp-cli games list-multigetplayabilitystatus` — Gets a list of universe playability statuses for the authenticated user

**games-2** — Manage games 2


**games-users** — Manage games users


**gender** — Manage gender

- `roblox-pp-cli gender create` — Update the user's gender
- `roblox-pp-cli gender list` — Get the user's gender

**groups** — Manage groups

- `roblox-pp-cli groups` — Fetches thumbnail URLs for a list of groups. Ids that do not correspond to groups will be filtered out.

**groups-2** — Manage groups 2

- `roblox-pp-cli groups-2 create` — This endpoint will charge Robux for the group purchase. Accepts 'icon' and 'coverPhoto' in Files object.
- `roblox-pp-cli groups-2 create-policies` — Gets group policy info used for compliance.
- `roblox-pp-cli groups-2 get` — Gets group information
- `roblox-pp-cli groups-2 list` — Gets Groups contextual information: Max number of groups a user can be part of.
- `roblox-pp-cli groups-2 list-configuration` — Gets Group configuration contextual information
- `roblox-pp-cli groups-2 list-search` — Search for groups by keyword.
- `roblox-pp-cli groups-2 list-search-2` — Should only be used for direct lookups where a user is inputting a group name, shouldn't be used for search pages.
- `roblox-pp-cli groups-2 list-search-3` — Although there is no reason for this to require an authenticated user right now, in the future
- `roblox-pp-cli groups-2 update` — Updates the group icon.

**groups-user** — Manage groups user

- `roblox-pp-cli groups-user create` — Sets the authenticated user's primary group
- `roblox-pp-cli groups-user delete` — Removes the authenticated user's primary group
- `roblox-pp-cli groups-user list` — Gets groups that the authenticated user has requested to join

**groups-users** — Manage groups users


**inventory** — Manage inventory

- `roblox-pp-cli inventory <assetId>` — Give up an asset owned by the authenticated user.

**inventory-assets** — Manage inventory assets


**inventory-users** — Manage inventory users


**inventory-users-2** — Manage inventory users 2


**metadata** — Manage metadata

- `roblox-pp-cli metadata` — List

**my** — Manage my

- `roblox-pp-cli my create` — Create
- `roblox-pp-cli my get` — Get
- `roblox-pp-cli my list` — Get the number of friends a user has
- `roblox-pp-cli my list-friends` — Get all users that friend requests with targetUserId using exclusive start paging
- `roblox-pp-cli my list-newfriendrequests` — List newfriendrequests
- `roblox-pp-cli my list-trustedfriends` — Get the number of trusted friends a user has
- `roblox-pp-cli my list-trustedfriends-2` — Get all incoming trusted friend requests using exclusive start paging.

**outfits** — Manage outfits


**packages** — Manage packages


**places** — Manage places

- `roblox-pp-cli places` — Fetches game icon URLs for a list of places. Ids that do not correspond to a valid place will be filtered out.

**presence** — Manage presence

- `roblox-pp-cli presence` — Get user

**roles** — Manage roles

- `roblox-pp-cli roles` — Gets the Roles by their ids.

**subcategories** — Manage subcategories

- `roblox-pp-cli subcategories` — Lists Subcategory Names and their Ids.

**thumbnails-games** — Manage thumbnails games

- `roblox-pp-cli thumbnails-games list` — Fetches game icon URLs for a list of universes' root places.
- `roblox-pp-cli thumbnails-games list-multiget` — Fetch game thumbnail URLs for a list of universe IDs.

**thumbnails-users** — Manage thumbnails users

- `roblox-pp-cli thumbnails-users list` — Get Avatar Full body shots for the given CSV of userIds
- `roblox-pp-cli thumbnails-users list-avatar3d` — Get Avatar 3d object for a user
- `roblox-pp-cli thumbnails-users list-avatarbust` — Get Avatar Busts for the given CSV of userIds
- `roblox-pp-cli thumbnails-users list-avatarheadshot` — Get Avatar Headshots for the given CSV of userIds
- `roblox-pp-cli thumbnails-users list-outfit3d` — Get 3d object for an outfit
- `roblox-pp-cli thumbnails-users list-outfits` — Get outfits for the given CSV of userOutfitIds

**topic** — Manage topic

- `roblox-pp-cli topic` — Get topic given TopicRequestModel.

**universes** — Manage universes


**user** — Manage user

- `roblox-pp-cli user create` — Returns whether or not the current user is following each userId in a list of userIds
- `roblox-pp-cli user list` — Return the number of pending friend requests.
- `roblox-pp-cli user list-trustedfriendrequests` — Return the number of pending trusted friend requests.

**usernames** — Manage usernames

- `roblox-pp-cli usernames` — This endpoint will also check previous usernames.

**users** — Manage users

- `roblox-pp-cli users create` — Does not require X-CSRF-Token protection because this is essentially a get request but as a POST to avoid URI limits.
- `roblox-pp-cli users get` — Gets detailed user information by id.
- `roblox-pp-cli users list` — Gets the minimal authenticated user.
- `roblox-pp-cli users list-authenticated` — Gets the age bracket of the authenticated user.
- `roblox-pp-cli users list-authenticated-2` — Gets the country code of the authenticated user.
- `roblox-pp-cli users list-authenticated-3` — Gets the (public) roles of the authenticated user, such as `'Soothsayer'` and `'BetaTester'`.
- `roblox-pp-cli users list-search` — Searches for users by keyword.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
roblox-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a public user response

```bash
roblox-pp-cli users get 1 --agent --select results.id,results.name,results.displayName
```

Return only stable identity fields to conserve agent context.

### Investigate a group

```bash
roblox-pp-cli investigate group 1 --agent
```

Resolve public group and owner context together.

### Refresh selected local resources

```bash
roblox-pp-cli sync --resources users,groups --max-pages 1
```

Populate a bounded local snapshot for later search and comparison.

## Auth Setup

Public lookup commands require no credentials. Some legacy Roblox endpoints require a browser session cookie and CSRF handling; this CLI does not claim that account mutations are available without those protections. Roblox Open Cloud API keys and OAuth are a separate creator-administration surface.

Run `roblox-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  roblox-pp-cli asset-thumbnail-animated --asset-id 42 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ROBLOX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ROBLOX_CONFIG_DIR`, `ROBLOX_DATA_DIR`, `ROBLOX_STATE_DIR`, `ROBLOX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ROBLOX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `roblox-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "roblox": {
        "command": "roblox-pp-mcp",
        "env": {
          "ROBLOX_HOME": "/srv/roblox"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ROBLOX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ROBLOX_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
roblox-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "roblox-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `roblox-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `roblox-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `roblox-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
roblox-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
roblox-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
roblox-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
roblox-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`roblox-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ROBLOX_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
roblox-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
roblox-pp-cli feedback --stdin < notes.txt
roblox-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ROBLOX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROBLOX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
roblox-pp-cli profile save briefing --json
roblox-pp-cli --profile briefing asset-thumbnail-animated --asset-id 42
roblox-pp-cli profile list --json
roblox-pp-cli profile show briefing
roblox-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `roblox-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/cmd/roblox-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add roblox-pp-mcp -- roblox-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which roblox-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   roblox-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `roblox-pp-cli <command> --help`.
