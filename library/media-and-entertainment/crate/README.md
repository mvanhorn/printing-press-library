# Discogs CLI

**Your record collection as a local database: pick tonight's record, subtract your shelf from a crate-digging search, and price your wantlist without a token.**

Syncs a Discogs collection and wantlist into local SQLite, then answers the questions the API cannot: what to play tonight, what to look for from a label you already collect, which wanted records are cheap right now, and what your shelf actually looks like by decade and genre. Database reads, public collections, and marketplace prices all work with no credential at all.

Learn more at [Discogs](https://www.discogs.com/developers).

Created by [@justinwfu](https://github.com/justinwfu) (justinwfu).

## Install

The recommended path installs both the `crate-pp-cli` binary and the `pp-crate` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install crate
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install crate --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install crate --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install crate --agent claude-code
npx -y @mvanhorn/printing-press-library install crate --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crate-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install crate --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-crate --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-crate --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install crate --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crate-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "crate": {
      "command": "crate-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credential is needed for database search, public collections and wantlists, or marketplace stats — that is the whole default path, rate limited to 25 requests per minute. A free personal access token from discogs.com/settings/developers raises the limit to 60 per minute and unlocks private collections and Discogs' own price suggestions. Set it as DISCOGS_TOKEN.

## Quick Start

```bash
# Mirror a public collection and wantlist into local SQLite. No token needed.
crate-pp-cli shelf-sync --user example

# See what the collection actually is, before asking it anything.
crate-pp-cli shelf --user example --by decade

# Pick something to play tonight, with the reason shown.
crate-pp-cli spin --user example --genre Rock

# What that label pressed that you do not already own.
crate-pp-cli dig --user example --label "Blue Note" --limit 10

# Wanted records that are cheap and for sale right now.
crate-pp-cli deals --user <username> --under 20 --limit 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`spin`** — Chooses a record off your own shelf to play tonight, with filters for genre, decade, or label, and says why it picked that one.

  _Reach for this when someone wants something to listen to from records they already own, rather than a recommendation to buy._

  ```bash
  crate-pp-cli spin --user example --genre Rock
  ```
- **`dig`** — Given a label, genre, style, or year, lists records matching it that you do NOT already own.

  _Reach for this before someone goes record shopping, or when they ask what to look for from a label or genre._

  ```bash
  crate-pp-cli dig --user example --label "Blue Note" --limit 10
  ```
- **`shelf`** — Breaks your collection down by decade, genre, style, label, and format to show what you actually collect.

  _Reach for this for questions about collecting patterns, blind spots, or how a collection has changed over time._

  ```bash
  crate-pp-cli shelf --user example --by decade
  ```

### Honest pricing
- **`floor`** — Totals the lowest current asking price across your collection, stated explicitly as a floor rather than an appraisal.

  _Reach for this when someone asks what their records are worth, but report it as a floor on asking prices, never as a valuation._

  ```bash
  crate-pp-cli floor --user example --limit 5
  ```
- **`deals`** — Cross-references your wantlist against current marketplace listings to show which wanted records are cheap and available right now.

  _Reach for this when someone asks what to buy now, or wants their wantlist prioritised by price and availability._

  ```bash
  crate-pp-cli deals --user <username> --under 20 --limit 3
  ```

## Recipes

### Sync then pick a record

```bash
crate-pp-cli shelf-sync --user example && crate-pp-cli spin --user example
```

One sync populates the local store; spin then runs offline against it.

### Dig a label you already collect

```bash
crate-pp-cli dig --user example --label "Blue Note" --year 1960-1969 --limit 15
```

Searches the label's catalogue and removes everything already on your shelf.

### Narrow a verbose search payload

```bash
crate-pp-cli database search "blue note" --type release --agent --select results.title,results.year,results.label
```

A Discogs search page runs to tens of KB; --select with dotted paths returns only the fields needed.

### Price the wantlist

```bash
crate-pp-cli deals --user <username> --under 25 --json --limit 5
```

Returns wanted records currently for sale below the threshold, as JSON for scripting.

### Shape of the shelf

```bash
crate-pp-cli shelf --user example --by label --limit 15
```

The labels that actually dominate the collection, counted across every page.

## Usage

Run `crate-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CRATE_CONFIG_DIR`, `CRATE_DATA_DIR`, `CRATE_STATE_DIR`, or `CRATE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CRATE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CRATE_HOME=/srv/crate
crate-pp-cli doctor
```

Under `CRATE_HOME=/srv/crate`, the four dirs resolve to `/srv/crate/config`, `/srv/crate/data`, `/srv/crate/state`, and `/srv/crate/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "crate": {
      "command": "crate-pp-mcp",
      "env": {
        "CRATE_HOME": "/srv/crate"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CRATE_DATA_DIR` overrides an explicit `--home` for that kind. Use `CRATE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CRATE_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `crate-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### artists

Manage artists

- **`crate-pp-cli artists get`** - Get an artist
- **`crate-pp-cli artists releases`** - List an artist's discography

### collection

Manage collection

- **`crate-pp-cli collection folders`** - Works without a token when the user's collection is public.
- **`crate-pp-cli collection releases`** - Folder 0 is the special "All" folder containing every record in the collection. Works without a token when the collection is public.

### database

Manage database

- **`crate-pp-cli database`** - The main discovery endpoint. Filter by label, genre, style, year, country, and format to find records matching a crate-digging brief.

### identity

Manage identity

- **`crate-pp-cli identity`** - Show which Discogs account the current token belongs to

### labels

Manage labels

- **`crate-pp-cli labels get`** - Get a record label
- **`crate-pp-cli labels releases`** - List a label's catalogue

### marketplace

Manage marketplace

- **`crate-pp-cli marketplace price-suggestions`** - Suggested prices by media condition (requires a token)
- **`crate-pp-cli marketplace stats`** - Keyless. Returns num_for_sale and lowest_price. Note this is the lowest price a seller is currently ASKING, not a sale price and not an appraisal.

### masters

Manage masters

- **`crate-pp-cli masters get`** - Get a master record, the abstract work behind all its pressings
- **`crate-pp-cli masters versions`** - List every pressing of a master, for comparing issues and countries

### releases

Manage releases

- **`crate-pp-cli releases get`** - Get one specific pressing of a record
- **`crate-pp-cli releases rating`** - Get a user's rating for a release

### users

Manage users

- **`crate-pp-cli users <username>`** - Get a user profile

### wantlist

Manage wantlist

- **`crate-pp-cli wantlist <username>`** - Works without a token when the user's wantlist is public.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`crate-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`crate-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`crate-pp-cli learnings list`** - Inspect taught rows
- **`crate-pp-cli learnings forget <query>`** - Undo a teach
- **`crate-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`crate-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`crate-pp-cli teach-pattern`** - Install a query/resource template up front
- **`crate-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CRATE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `crate-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
crate-pp-cli artists get mock-value

# JSON for scripting and agents
crate-pp-cli artists get mock-value --json

# Filter to specific fields
crate-pp-cli artists get mock-value --json --select id,name,status

# Dry run — show the request without sending
crate-pp-cli artists get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
crate-pp-cli artists get mock-value --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `CRATE_ARTIST_ID` resolves `{artist_id}`
- `CRATE_FOLDER_ID` resolves `{folder_id}`
- `CRATE_LABEL_ID` resolves `{label_id}`
- `CRATE_MASTER_ID` resolves `{master_id}`
- `CRATE_USERNAME` resolves `{username}`

Base URL: `https://api.discogs.com`

## Health Check

```bash
crate-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `crate-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/discogs-pp-cli/config.toml`; `--home`, `CRATE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429, or commands slowing to a crawl** — Unauthenticated requests are capped at 25 per minute. Set DISCOGS_TOKEN for 60 per minute, or use --limit to price fewer records per run; cached prices are reused.
- **Collection or wantlist returns 404** — The username does not exist. Discogs usernames are case-sensitive and the docs' placeholder names are not real accounts.
- **Collection returns 403** — That user's collection is private. Only the owner's own token can read it; set DISCOGS_TOKEN.
- **floor reports fewer records than the collection holds** — Pricing is rate limited, so floor prices a bounded number of records per run and says how many it covered. Re-run to extend coverage; results are cached.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**discogs-client (python)**](https://github.com/joalla/discogs_client) — Python
- [**disconnect**](https://github.com/bartve/disconnect) — JavaScript
- [**discogs-mcp-server**](https://github.com/cswkim/discogs-mcp-server) — TypeScript
- [**go-discogs**](https://github.com/irlndts/go-discogs) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
