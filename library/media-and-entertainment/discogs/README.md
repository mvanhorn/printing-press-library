# Discogs CLI

**Every Discogs feature — plus a local price-history mirror the API doesn't keep, so you can watch your wantlist like a limit-order book, spot underpriced records, and track your collection's value over time.**

A single Go binary over the full Discogs API — database, collection, wantlist, and marketplace — with an offline SQLite mirror and an MCP server. Because Discogs keeps no price history, this CLI snapshots prices locally, which unlocks fills (wantlist limit orders), undervalued detection, portfolio value over time, and condition-matched comps that no other Discogs tool offers.

## Install

The recommended path installs both the `discogs-pp-cli` binary and the `pp-discogs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install discogs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install discogs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install discogs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install discogs --agent claude-code
npx -y @mvanhorn/printing-press-library install discogs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/cmd/discogs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/discogs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install discogs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-discogs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-discogs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install discogs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/discogs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DISCOGS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/cmd/discogs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "discogs": {
      "command": "discogs-pp-mcp",
      "env": {
        "DISCOGS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Database search and lookups are public. Everything tied to a user — collection, wantlist, marketplace, identity — needs a free personal access token from discogs.com/settings/developers, set as DISCOGS_TOKEN. The client sends it as `Authorization: Discogs token=<token>` and always sends a descriptive User-Agent (Discogs returns 403 without one).

## Quick Start

```bash
# Confirm config and reachability before anything else.
discogs-pp-cli doctor --dry-run

# Public database search — works without a token.
discogs-pp-cli database search "nevermind" --artist Nirvana --format Vinyl

# Confirm your DISCOGS_TOKEN resolves to your account.
discogs-pp-cli identity whoami

# Populate the local mirror so offline and price features work.
discogs-pp-cli sync --resources wantlist,collection

# The flagship: wantlist items now selling at or below your limit.
discogs-pp-cli fills --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local price history (the moat)
- **`fills`** — Treat each wantlist entry as a standing limit order and surface the ones now selling at or below the max price you set.

  _Reach for this to answer 'which of my wants are buyable right now' without checking hundreds of listings by hand under the 60/min limit._

  ```bash
  discogs-pp-cli fills --agent
  ```
- **`portfolio`** — Chart your collection's value over time with per-record contribution to the change and cost-basis P&L.

  _Use it when the question is 'is my collection up or down, and because of what' rather than 'what is it worth today'._

  ```bash
  discogs-pp-cli portfolio --since 90d --agent
  ```
- **`undervalued`** — Flag releases whose current asking price sits below their own trailing/suggested baseline.

  _Pick this for market-wide mispricing where no preset limit exists; use 'fills' when the trigger is a limit you set._

  ```bash
  discogs-pp-cli undervalued --scope wantlist --agent
  ```
- **`comps`** — The condition-by-condition price picture for one release, with local snapshot history the API can't provide.

  _Use it to price or value a single specific release; use 'pressings' to compare different versions of the same album._

  ```bash
  discogs-pp-cli comps 249504 --agent
  ```
- **`pressings`** — Rank all versions/pressings of a master release by value and liquidity so you know which pressing to buy, want, or sell.

  _Use it to choose among pressings of an album; use 'comps' for the condition breakdown of one specific pressing._

  ```bash
  discogs-pp-cli pressings 96559 --agent
  ```

### Collector decisioning
- **`sell-plan`** — Rank your inventory or collection by net-after-fee proceeds and liquidity so you know what to sell now.

  _Reach for this to decide what to list this week; use 'comps' to price one release._

  ```bash
  discogs-pp-cli sell-plan --source collection --agent
  ```
- **`identify`** — Resolve a physical record's catalog number or barcode to the exact release and show if you own it, want it, and what it's worth.

  _Use it crate-digging in a shop to make a buy/skip call fast; use 'search' for open-ended text queries._

  ```bash
  discogs-pp-cli identify --barcode 0720642442524 --agent
  ```

## Recipes

### Deals on your wantlist

```bash
discogs-pp-cli fills --agent
```

Lists wantlist items whose lowest marketplace asking has reached the max price you set, with the change since last check.

### Narrow a verbose release payload

```bash
discogs-pp-cli database release 249504 --agent --select id,title,year,labels.name,formats.descriptions,lowest_price
```

A Discogs release is tens of KB; dotted --select paths return only the fields you need so an agent doesn't burn context.

### What to sell this week

```bash
discogs-pp-cli sell-plan --source collection --agent
```

Ranks your records by net-after-fee proceeds and liquidity.

### Identify a record in-hand

```bash
discogs-pp-cli identify --barcode 0720642442524 --agent
```

Resolves the barcode to a release and tells you if you own it, want it, and its current value.

### Value trend over the last quarter

```bash
discogs-pp-cli portfolio --since 90d --agent
```

Charts collection value change and the records that drove it, against cost basis.

## Usage

Run `discogs-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DISCOGS_CONFIG_DIR`, `DISCOGS_DATA_DIR`, `DISCOGS_STATE_DIR`, or `DISCOGS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DISCOGS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DISCOGS_HOME=/srv/discogs
discogs-pp-cli doctor
```

Under `DISCOGS_HOME=/srv/discogs`, the four dirs resolve to `/srv/discogs/config`, `/srv/discogs/data`, `/srv/discogs/state`, and `/srv/discogs/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "discogs": {
      "command": "discogs-pp-mcp",
      "env": {
        "DISCOGS_HOME": "/srv/discogs"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DISCOGS_DATA_DIR` overrides an explicit `--home` for that kind. Use `DISCOGS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DISCOGS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `discogs-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### collection

Browse and manage your Discogs collection (requires a token).

- **`discogs-pp-cli collection add`** - Add a release to a collection folder.
- **`discogs-pp-cli collection create-folder`** - Create a new collection folder.
- **`discogs-pp-cli collection delete-folder`** - Delete a collection folder (must be empty).
- **`discogs-pp-cli collection fields`** - List the custom fields defined on your collection.
- **`discogs-pp-cli collection find`** - Find a release's instances in your collection.
- **`discogs-pp-cli collection folder`** - Get one collection folder (0 = All).
- **`discogs-pp-cli collection folders`** - List your collection folders.
- **`discogs-pp-cli collection items`** - List releases in a collection folder (0 = All).
- **`discogs-pp-cli collection rate-instance`** - Set the rating on a collection instance.
- **`discogs-pp-cli collection remove`** - Remove a release instance from a folder.
- **`discogs-pp-cli collection rename-folder`** - Rename a collection folder.
- **`discogs-pp-cli collection value`** - Get the min/median/max market value of your collection.

### database

Search Discogs and look up releases, masters, artists, and labels (public; a token raises the rate limit).

- **`discogs-pp-cli database artist`** - Get an artist by ID.
- **`discogs-pp-cli database artist-releases`** - List an artist's releases.
- **`discogs-pp-cli database community-rating`** - Get the community rating for a release.
- **`discogs-pp-cli database label`** - Get a label by ID.
- **`discogs-pp-cli database label-releases`** - List releases on a label.
- **`discogs-pp-cli database master`** - Get a master release (the canonical version group).
- **`discogs-pp-cli database master-versions`** - List all versions (pressings) of a master release.
- **`discogs-pp-cli database rate`** - Set your rating (1-5) for a release.
- **`discogs-pp-cli database release`** - Get a specific release by ID.
- **`discogs-pp-cli database search`** - Search the Discogs database. Public data; a token raises the rate limit from 25 to 60/min.
- **`discogs-pp-cli database unrate`** - Delete your rating for a release.
- **`discogs-pp-cli database user-rating`** - Get a specific user's rating for a release.

### identity

Your Discogs identity, profile, and contributions (requires a token).

- **`discogs-pp-cli identity contributions`** - List a user's database contributions.
- **`discogs-pp-cli identity profile`** - Get a user's public profile.
- **`discogs-pp-cli identity submissions`** - List a user's database submissions.
- **`discogs-pp-cli identity whoami`** - Show the authenticated user's identity (username, id).

### inventory

Request and download CSV exports of your marketplace inventory (requires a token).

- **`discogs-pp-cli inventory export`** - Request a new CSV export of your inventory.
- **`discogs-pp-cli inventory export-download`** - Download a finished inventory export CSV.
- **`discogs-pp-cli inventory export-get`** - Get the status of an inventory export.
- **`discogs-pp-cli inventory exports`** - List your recent inventory exports.
- **`discogs-pp-cli inventory upload-get`** - Get the status of an inventory CSV upload.
- **`discogs-pp-cli inventory uploads`** - List recent inventory CSV uploads.

### lists

User lists (curated collections of database items).

- **`discogs-pp-cli lists get`** - Get one list and its items.
- **`discogs-pp-cli lists user`** - List a user's public lists.

### marketplace

Marketplace listings, orders, fees, price suggestions, and per-release stats (requires a token).

- **`discogs-pp-cli marketplace add-order-message`** - Post a message to an order.
- **`discogs-pp-cli marketplace create-listing`** - Create a new marketplace listing.
- **`discogs-pp-cli marketplace delete-listing`** - Delete a marketplace listing.
- **`discogs-pp-cli marketplace fee`** - Calculate the Discogs marketplace fee for a sale price (USD).
- **`discogs-pp-cli marketplace fee-currency`** - Calculate the marketplace fee for a price in a specific currency.
- **`discogs-pp-cli marketplace inventory`** - List a seller's marketplace inventory.
- **`discogs-pp-cli marketplace listing`** - Get one marketplace listing.
- **`discogs-pp-cli marketplace order`** - Get one marketplace order.
- **`discogs-pp-cli marketplace order-messages`** - List messages on an order.
- **`discogs-pp-cli marketplace orders`** - List your marketplace orders (as seller).
- **`discogs-pp-cli marketplace price-suggestions`** - Get suggested marketplace prices per media condition for a release (seller token).
- **`discogs-pp-cli marketplace stats`** - Get marketplace stats for a release (lowest price, number for sale).
- **`discogs-pp-cli marketplace update-listing`** - Edit an existing marketplace listing.
- **`discogs-pp-cli marketplace update-order`** - Update an order's status or shipping.

### wantlist

Browse and manage your Discogs wantlist (requires a token).

- **`discogs-pp-cli wantlist add`** - Add a release to your wantlist.
- **`discogs-pp-cli wantlist edit`** - Edit the notes/rating on a wantlist item.
- **`discogs-pp-cli wantlist list`** - List your wantlist.
- **`discogs-pp-cli wantlist remove`** - Remove a release from your wantlist.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`discogs-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`discogs-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`discogs-pp-cli learnings list`** - Inspect taught rows
- **`discogs-pp-cli learnings forget <query>`** - Undo a teach
- **`discogs-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`discogs-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`discogs-pp-cli teach-pattern`** - Install a query/resource template up front
- **`discogs-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `DISCOGS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `discogs-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
discogs-pp-cli database search mock-value

# JSON for scripting and agents
discogs-pp-cli database search mock-value --json

# Filter to specific fields
discogs-pp-cli database search mock-value --json --select id,name,status

# Dry run — show the request without sending
discogs-pp-cli database search mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
discogs-pp-cli database search mock-value --agent
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

## Health Check

```bash
discogs-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `discogs-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/discogs-pp-cli/config.toml`; `--home`, `DISCOGS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DISCOGS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `discogs-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `discogs-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DISCOGS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Forbidden on every request** — Discogs requires a descriptive User-Agent; the CLI sends one automatically — rebuild if you see this.
- **429 / rate limited** — Discogs allows 60 req/min authenticated. Run `sync` and use offline commands; the client backs off on the X-Discogs-Ratelimit-Remaining header.
- **401 Unauthorized on collection/wantlist/marketplace** — Set DISCOGS_TOKEN (free token at discogs.com/settings/developers). Database search works without one.
- **fills or undervalued returns nothing** — Run `sync` first and let a few price snapshots accumulate; these features read local price history.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**python3-discogs-client**](https://github.com/joalla/discogs_client) — Python (409 stars)
- [**discogs-mcp-server**](https://github.com/cswkim/discogs-mcp-server) — TypeScript (115 stars)
- [**discodos**](https://github.com/JOJ0/discodos) — Python (83 stars)
- [**go-discogs**](https://github.com/irlndts/go-discogs) — Go (52 stars)
- [**discogs-client**](https://github.com/lionralfs/discogs-client) — TypeScript (39 stars)
- [**discogs-mcp**](https://github.com/rianvdm/discogs-mcp) — TypeScript (13 stars)
- [**agent-discogs**](https://github.com/jmfontaine/agent-discogs) — Python (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
