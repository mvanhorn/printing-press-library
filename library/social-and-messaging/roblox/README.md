# Roblox CLI

**Investigate Roblox's fragmented public web data from one agent-friendly CLI.**

Query public users, groups, avatars, games, badges, inventory, catalog, and thumbnails across Roblox API hosts. Cross-resource investigation commands and a local SQLite store turn isolated responses into reproducible research workflows.

## Install

The recommended path installs both the `roblox-pp-cli` binary and the `pp-roblox` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install roblox
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install roblox --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install roblox --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install roblox --agent claude-code
npx -y @mvanhorn/printing-press-library install roblox --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/cmd/roblox-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roblox-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install roblox --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-roblox --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-roblox --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install roblox --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
roblox-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roblox-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/cmd/roblox-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "roblox": {
      "command": "roblox-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public lookup commands require no credentials. Some legacy Roblox endpoints require a browser session cookie and CSRF handling; this CLI does not claim that account mutations are available without those protections. Roblox Open Cloud API keys and OAuth are a separate creator-administration surface.

## Quick Start

```bash
# Check local configuration without making an API request.
roblox-pp-cli doctor --dry-run

# Fetch a known public user as structured JSON.
roblox-pp-cli users get 1 --agent

# Build a cross-resource public identity view.
roblox-pp-cli investigate user 1 --agent

```

## Unique Features

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

## Usage

Run `roblox-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ROBLOX_CONFIG_DIR`, `ROBLOX_DATA_DIR`, `ROBLOX_STATE_DIR`, or `ROBLOX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ROBLOX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ROBLOX_HOME=/srv/roblox
roblox-pp-cli doctor
```

Under `ROBLOX_HOME=/srv/roblox`, the four dirs resolve to `/srv/roblox/config`, `/srv/roblox/data`, `/srv/roblox/state`, and `/srv/roblox/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `ROBLOX_DATA_DIR` overrides an explicit `--home` for that kind. Use `ROBLOX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ROBLOX_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `roblox-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### asset-thumbnail-animated

Manage asset thumbnail animated

- **`roblox-pp-cli asset-thumbnail-animated`** - Thumbnails asset animated.

### asset-to-category

Manage asset to category

- **`roblox-pp-cli asset-to-category`** - Lists a mapping for assets to category IDs to convert from inventory ID to catalog ID. Creates a mapping to link 'Get More' button in inventory page to the relevant catalog page.

### asset-to-subcategory

Manage asset to subcategory

- **`roblox-pp-cli asset-to-subcategory`** - Lists a mapping for assets to subcategory IDs to convert from inventory ID to catalog ID. Creates a mapping to link 'Get More' button in inventory page to the relevant catalog page.

### assets

Manage assets

- **`roblox-pp-cli assets`** - Thumbnails assets.

### assets-thumbnail-3d

Manage assets thumbnail 3d

- **`roblox-pp-cli assets-thumbnail-3d`** - Thumbnails assets.

### avatar

Manage avatar

- **`roblox-pp-cli avatar create`** - Requests the authenticated user's thumbnail be redrawn.
- **`roblox-pp-cli avatar create-setbodycolors`** - Sets the authenticated user's body colors.
- **`roblox-pp-cli avatar create-setplayeravatartype`** - This is the avatar type chosen on the Avatar page. Some games can override this and force your character to be R6 or R15.
- **`roblox-pp-cli avatar create-setscales`** - Sets the authenticated user's scales.
- **`roblox-pp-cli avatar list`** - Returns details about the authenticated user's avatar.
- **`roblox-pp-cli avatar list-metadata`** - Returns metadata used by the avatar page of the website.

### avatar-2

Manage avatar 2

- **`roblox-pp-cli avatar-2 create`** - Sets the authenticated user's body colors.
- **`roblox-pp-cli avatar-2 create-setwearingassets`** - Only allows items that you own, are not expired, and are wearable asset types.
Any assets being worn before this method is called are automatically removed.
- **`roblox-pp-cli avatar-2 get`** - Includes assets, bodycolors, and playerAvatarType.
- **`roblox-pp-cli avatar-2 get-users`** - Gets a list of outfits for the specified user.
- **`roblox-pp-cli avatar-2 list`** - Returns details about the authenticated user's avatar.

### avatar-outfits

Manage avatar outfits

- **`roblox-pp-cli avatar-outfits create`** - Fails if any of the assetIds are not owned by the user, or not wearable types.
The name property of the request is optional as one will be auto-generated when the request has a null name.
- **`roblox-pp-cli avatar-outfits update`** - Fails if the user does not own any of the assetIds or if they are not wearable asset types.
Accepts partial updates.

### avatar-rules

Manage avatar rules

- **`roblox-pp-cli avatar-rules`** - BodyColorsPalette is a list of valid brickColors you can choose for your avatar.
WearableAssetTypes contains a list of asset types with names, ids, and the maximum number that you can wear at a time.
Does not include packages because they cannot be worn on your avatar directly.
PlayerAvatarTypes are the types of avatars you can choose between.

### avatar-users

Manage avatar users


### badges

Manage badges

- **`roblox-pp-cli badges`** - Thumbnails badge icons.

### badges-2

Manage badges 2

- **`roblox-pp-cli badges-2 get`** - Gets badge information by the badge Id.
- **`roblox-pp-cli badges-2 list`** - Gets metadata about the badges system.
- **`roblox-pp-cli badges-2 update`** - Updates badge configuration.

### badges-user

Manage badges user

- **`roblox-pp-cli badges-user <badgeId>`** - Removes a badge from the authenticated user.

### badges-users

Manage badges users


### batch

Manage batch

- **`roblox-pp-cli batch`** - Returns a list of thumbnails with varying types and sizes

### birthdate

Manage birthdate

- **`roblox-pp-cli birthdate create`** - Update the user's birthdate
- **`roblox-pp-cli birthdate list`** - Get the user's birthdate

### bundles

Manage bundles

- **`roblox-pp-cli bundles`** - Get bundle thumbnails for the given CSV of bundle ids

### catalog

Manage catalog

- **`roblox-pp-cli catalog`** - There is an item count limit per request. Exceeding this returns 400 Bad Request.

### catalog-assets

Manage catalog assets


### catalog-bundles

Manage catalog bundles

- **`roblox-pp-cli catalog-bundles`** - Returns details about the given bundleIds.

### catalog-search

Manage catalog search

- **`roblox-pp-cli catalog-search`** - This endpoint is for search by item type ids.

### catalog-users

Manage catalog users


### categories

Manage categories

- **`roblox-pp-cli categories`** - Lists Category Names and their Ids.

### collectible-items

Manage collectible items


### collections

Manage collections

- **`roblox-pp-cli collections create`** - Adds an item to the appropriate collection
- **`roblox-pp-cli collections delete`** - Removes an item to the appropriate collection

### description

Manage description

- **`roblox-pp-cli description create`** - Update the user's description
- **`roblox-pp-cli description list`** - Get the user's description

### developer-products

Manage developer products

- **`roblox-pp-cli developer-products`** - Thumbnails developer product icons.

### display-names

Manage display names

- **`roblox-pp-cli display-names`** - Validate a display name for a new user.

### economy-user

Manage economy user

- **`roblox-pp-cli economy-user`** - Currency can only be retrieved for the authenticated user.

### favorites

Manage favorites

- **`roblox-pp-cli favorites create`** - Create a favorite for an asset by the authenticated user.
- **`roblox-pp-cli favorites create-users`** - Create a favorite for the bundle by the authenticated user.
- **`roblox-pp-cli favorites delete`** - Delete a favorite for an asset by the authenticated user.
- **`roblox-pp-cli favorites delete-users`** - Delete favorite for the bundle by the authenticated user.
- **`roblox-pp-cli favorites get`** - Gets the favorite count for the given asset Id.
- **`roblox-pp-cli favorites get-bundles`** - Gets the favorite count for the given bundle Id.
- **`roblox-pp-cli favorites get-users`** - Gets the favorite model for the asset and user.
- **`roblox-pp-cli favorites get-users-2`** - Gets the favorite model for the bundle and user.
- **`roblox-pp-cli favorites get-users-3`** - Lists the marketplace assets favorited by a given user with the given assetTypeId.
- **`roblox-pp-cli favorites get-users-4`** - Lists the bundles favorited by a given user with the given bundle subtypeId.Switched to EAAS style pagination cursors since July 2024.

### featured-content

Manage featured content

- **`roblox-pp-cli featured-content create`** - Sets the featured event for a group
- **`roblox-pp-cli featured-content delete`** - Deletes the featured event for a group
- **`roblox-pp-cli featured-content list`** - Gets the featured event for a group

### friends-users

Manage friends users


### game-passes

Manage game passes

- **`roblox-pp-cli game-passes`** - Thumbnails game pass icons.

### game-start-info

Manage game start info

- **`roblox-pp-cli game-start-info`** - The server will call this on game server start to request general information about the universe
This is version 1.1, which returns an entry from the UniverseAvatarType enum.
During mixed mode this may return unreliable results.

### games

Manage games

- **`roblox-pp-cli games get`** - Get games recommendations based on a given universe
- **`roblox-pp-cli games list`** - Gets a list of games' detail
- **`roblox-pp-cli games list-gamesproductinfo`** - Gets a list of games' product info, used to purchase a game
- **`roblox-pp-cli games list-multigetplacedetails`** - Get place details
- **`roblox-pp-cli games list-multigetplayabilitystatus`** - Gets a list of universe playability statuses for the authenticated user

### games-2

Manage games 2


### games-users

Manage games users


### gender

Manage gender

- **`roblox-pp-cli gender create`** - Update the user's gender
- **`roblox-pp-cli gender list`** - Get the user's gender

### groups

Manage groups

- **`roblox-pp-cli groups`** - Fetches thumbnail URLs for a list of groups. Ids that do not correspond to groups will be filtered out.

### groups-2

Manage groups 2

- **`roblox-pp-cli groups-2 create`** - This endpoint will charge Robux for the group purchase.
Accepts "icon" and "coverPhoto" in Files object. Defaults to first file if "icon" is not present.
Http status code 413 is thrown when the group icon file size is too large.
- **`roblox-pp-cli groups-2 create-policies`** - Gets group policy info used for compliance.
- **`roblox-pp-cli groups-2 get`** - Gets group information
- **`roblox-pp-cli groups-2 list`** - Gets Groups contextual information:
Max number of groups a user can be part of.
Current number of groups a user is a member of.
Whether to show/hide certain features based on device type.
- **`roblox-pp-cli groups-2 list-configuration`** - Gets Group configuration contextual information
- **`roblox-pp-cli groups-2 list-search`** - Search for groups by keyword.
- **`roblox-pp-cli groups-2 list-search-2`** - Should only be used for direct lookups where a user is inputting a group name, shouldn't be used for search pages.
- **`roblox-pp-cli groups-2 list-search-3`** - Although there is no reason for this to require an authenticated user right now, in the future,
we will use coco to return different suggested groups based upon that user's request context
- **`roblox-pp-cli groups-2 update`** - Updates the group icon.

### groups-user

Manage groups user

- **`roblox-pp-cli groups-user create`** - Sets the authenticated user's primary group
- **`roblox-pp-cli groups-user delete`** - Removes the authenticated user's primary group
- **`roblox-pp-cli groups-user list`** - Gets groups that the authenticated user has requested to join

### groups-users

Manage groups users


### inventory

Manage inventory

- **`roblox-pp-cli inventory <assetId>`** - Give up an asset owned by the authenticated user.
Assets that are created by Roblox user or are limited edition are not eligible for deletion
and will return NotEligibleForDelete.

### inventory-assets

Manage inventory assets


### inventory-users

Manage inventory users


### inventory-users-2

Manage inventory users 2


### metadata

Manage metadata

- **`roblox-pp-cli metadata`** - List

### my

Manage my

- **`roblox-pp-cli my create`** - Create
- **`roblox-pp-cli my get`** - Get
- **`roblox-pp-cli my list`** - Get the number of friends a user has
- **`roblox-pp-cli my list-friends`** - Get all users that friend requests with targetUserId using exclusive start paging
- **`roblox-pp-cli my list-newfriendrequests`** - List newfriendrequests
- **`roblox-pp-cli my list-trustedfriends`** - Get the number of trusted friends a user has
- **`roblox-pp-cli my list-trustedfriends-2`** - Get all incoming trusted friend requests using exclusive start paging.

### outfits

Manage outfits


### packages

Manage packages


### places

Manage places

- **`roblox-pp-cli places`** - Fetches game icon URLs for a list of places. Ids that do not correspond to a valid place will be filtered out.

### presence

Manage presence

- **`roblox-pp-cli presence`** - Get user

### roles

Manage roles

- **`roblox-pp-cli roles`** - Gets the Roles by their ids.

### subcategories

Manage subcategories

- **`roblox-pp-cli subcategories`** - Lists Subcategory Names and their Ids.

### thumbnails-games

Manage thumbnails games

- **`roblox-pp-cli thumbnails-games list`** - Fetches game icon URLs for a list of universes' root places. Ids that do not correspond to a valid universe will be filtered out.
The ordering of the results is not guaranteed to be the same as the inputs. In order to correlated inputs with outputs please
use the 'targetId' of the objects in the result array.
- **`roblox-pp-cli thumbnails-games list-multiget`** - Fetch game thumbnail URLs for a list of universe IDs.

### thumbnails-users

Manage thumbnails users

- **`roblox-pp-cli thumbnails-users list`** - Get Avatar Full body shots for the given CSV of userIds
- **`roblox-pp-cli thumbnails-users list-avatar3d`** - Get Avatar 3d object for a user
- **`roblox-pp-cli thumbnails-users list-avatarbust`** - Get Avatar Busts for the given CSV of userIds
- **`roblox-pp-cli thumbnails-users list-avatarheadshot`** - Get Avatar Headshots for the given CSV of userIds
- **`roblox-pp-cli thumbnails-users list-outfit3d`** - Get 3d object for an outfit
- **`roblox-pp-cli thumbnails-users list-outfits`** - Get outfits for the given CSV of userOutfitIds

### topic

Manage topic

- **`roblox-pp-cli topic`** - Get topic given TopicRequestModel.

### universes

Manage universes


### user

Manage user

- **`roblox-pp-cli user create`** - Returns whether or not the current user is following each userId in a list of userIds
- **`roblox-pp-cli user list`** - Return the number of pending friend requests.
- **`roblox-pp-cli user list-trustedfriendrequests`** - Return the number of pending trusted friend requests.

### usernames

Manage usernames

- **`roblox-pp-cli usernames`** - This endpoint will also check previous usernames.
Does not require X-CSRF-Token protection because this is essentially a get request but as a POST to avoid URI limits.

### users

Manage users

- **`roblox-pp-cli users create`** - Does not require X-CSRF-Token protection because this is essentially a get request but as a POST to avoid URI limits.
- **`roblox-pp-cli users get`** - Gets detailed user information by id.
- **`roblox-pp-cli users list`** - Gets the minimal authenticated user.
- **`roblox-pp-cli users list-authenticated`** - Gets the age bracket of the authenticated user.
- **`roblox-pp-cli users list-authenticated-2`** - Gets the country code of the authenticated user.
- **`roblox-pp-cli users list-authenticated-3`** - Gets the (public) roles of the authenticated user, such as `"Soothsayer"` and `"BetaTester"`.
- **`roblox-pp-cli users list-search`** - Searches for users by keyword.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`roblox-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`roblox-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`roblox-pp-cli learnings list`** - Inspect taught rows
- **`roblox-pp-cli learnings forget <query>`** - Undo a teach
- **`roblox-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`roblox-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`roblox-pp-cli teach-pattern`** - Install a query/resource template up front
- **`roblox-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ROBLOX_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `roblox-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
roblox-pp-cli asset-thumbnail-animated --asset-id 42

# JSON for scripting and agents
roblox-pp-cli asset-thumbnail-animated --asset-id 42 --json

# Filter to specific fields
roblox-pp-cli asset-thumbnail-animated --asset-id 42 --json --select id,name,status

# Dry run — show the request without sending
roblox-pp-cli asset-thumbnail-animated --asset-id 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
roblox-pp-cli asset-thumbnail-animated --asset-id 42 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `ROBLOX_COLLECTIBLE_ITEM_ID` resolves `{collectibleItemId}`
- `ROBLOX_TARGET_USER_ID` resolves `{targetUserId}`

Base URL: `https://users.roblox.com`

## Health Check

```bash
roblox-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `roblox-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/roblox-pp-cli/config.toml`; `--home`, `ROBLOX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ROBLOX_COLLECTIBLEITEMID` | endpoint | Yes |  |
| `ROBLOX_TARGETUSERID` | endpoint | Yes |  |
| `ROBLOX_COOKIES` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `roblox-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `roblox-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ROBLOX_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A command returns HTTP 401 or 403** — Check the Creator Hub endpoint reference; the legacy operation may require Cookie/CSRF auth or an Open Cloud credential and is outside anonymous lookup.
- **A public GET returns stale data** — Retry with --no-cache to bypass the five-minute response cache.
