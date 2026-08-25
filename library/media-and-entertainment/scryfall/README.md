# Scryfall CLI



Created by [@veltri-23](https://github.com/veltri-23) (Hunter Veltri).

## Install

The recommended path installs both the `scryfall-pp-cli` binary and the `pp-scryfall` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install scryfall
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install scryfall --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install scryfall --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install scryfall --agent claude-code
npx -y @mvanhorn/printing-press-library install scryfall --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/scryfall/cmd/scryfall-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scryfall-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install scryfall --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-scryfall --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-scryfall --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install scryfall --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scryfall-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/scryfall/cmd/scryfall-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scryfall": {
      "command": "scryfall-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
scryfall-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
scryfall-pp-cli cards search --q example-value
```

## Usage

Run `scryfall-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SCRYFALL_CONFIG_DIR`, `SCRYFALL_DATA_DIR`, `SCRYFALL_STATE_DIR`, or `SCRYFALL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SCRYFALL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SCRYFALL_HOME=/srv/scryfall
scryfall-pp-cli doctor
```

Under `SCRYFALL_HOME=/srv/scryfall`, the four dirs resolve to `/srv/scryfall/config`, `/srv/scryfall/data`, `/srv/scryfall/state`, and `/srv/scryfall/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "scryfall": {
      "command": "scryfall-pp-mcp",
      "env": {
        "SCRYFALL_HOME": "/srv/scryfall"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SCRYFALL_DATA_DIR` overrides an explicit `--home` for that kind. Use `SCRYFALL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SCRYFALL_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `scryfall-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### bulk-data

Manage bulk data

- **`scryfall-pp-cli bulk-data`** - Returns a List of all bulk data items on Scryfall.

### cards

Manage cards

- **`scryfall-pp-cli cards autocomplete`** - Autocomplete
- **`scryfall-pp-cli cards get-all`** - Returns a `List` object that contains all cards in Scryfall’s database. This method is paginated, returning 175 cards at a time. The cards are ordered roughly newest to oldest.
- **`scryfall-pp-cli cards get-by-arena-id`** - Get by arena id
- **`scryfall-pp-cli cards get-by-cardmarket-id`** - Fetch a card by its Cardmarket ID
- **`scryfall-pp-cli cards get-by-code-by-number`** - Get by code by number
- **`scryfall-pp-cli cards get-by-id`** - Get by id
- **`scryfall-pp-cli cards get-by-mtgo-id`** - Get by mtgo id
- **`scryfall-pp-cli cards get-by-multiverse-id`** - Get by multiverse id
- **`scryfall-pp-cli cards get-by-tcgplayer-id`** - Fetch a card by its TCGplayer ID
- **`scryfall-pp-cli cards get-named`** - Get named
- **`scryfall-pp-cli cards get-random`** - Get random
- **`scryfall-pp-cli cards post-collection`** - Accepts a JSON body with an identifiers array (max 75). Supported identifier keys: id, mtgo_id, arena_id, multiverse_id, name, set + collector_number, collector_number.
- **`scryfall-pp-cli cards rulings-get-by-mtgo-id`** - Rulings get by mtgo id
- **`scryfall-pp-cli cards rulings-get-by-multiverse-id`** - Rulings get by multiverse id
- **`scryfall-pp-cli cards search`** - Returns a List object containing Cards found using a fulltext search string. This string supports the same [fulltext search system](https://scryfall.com/docs/syntax) that the main site uses.

### catalog

Manage catalog

- **`scryfall-pp-cli catalog get-artifact-types`** - Get artifact types
- **`scryfall-pp-cli catalog get-card-names`** - Get card names
- **`scryfall-pp-cli catalog get-creature-types`** - Get creature types
- **`scryfall-pp-cli catalog get-enchantment-types`** - Get enchantment types
- **`scryfall-pp-cli catalog get-land-types`** - Get land types
- **`scryfall-pp-cli catalog get-loyalties`** - Get loyalties
- **`scryfall-pp-cli catalog get-planeswalker-types`** - Get planeswalker types
- **`scryfall-pp-cli catalog get-powers`** - Get powers
- **`scryfall-pp-cli catalog get-spell-types`** - Get spell types
- **`scryfall-pp-cli catalog get-toughnesses`** - Get toughnesses
- **`scryfall-pp-cli catalog get-watermarks`** - Get watermarks
- **`scryfall-pp-cli catalog get-word-bank`** - Get word bank

### sets

Manage sets

- **`scryfall-pp-cli sets get-all`** - Returns a List object of all Sets on Scryfall
- **`scryfall-pp-cli sets get-by-code`** - Returns a `Set` with the given set code. The code can be either the `code` or the `mtgo_code` for the set.
- **`scryfall-pp-cli sets get-by-id`** - Returns a `Set` with the given Scryfall `id`.
- **`scryfall-pp-cli sets get-by-tcgplayer-id`** - Returns a `Set` with the given `tcgplayer_id`, also known as the `groupId` on [TCGplayer’s API](https://docs.tcgplayer.com/docs).

### symbology

Manage symbology

- **`scryfall-pp-cli symbology get-all`** - Get all
- **`scryfall-pp-cli symbology parse-mana`** - Parse mana


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`scryfall-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`scryfall-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`scryfall-pp-cli learnings list`** - Inspect taught rows
- **`scryfall-pp-cli learnings forget <query>`** - Undo a teach
- **`scryfall-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`scryfall-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`scryfall-pp-cli teach-pattern`** - Install a query/resource template up front
- **`scryfall-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SCRYFALL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `scryfall-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
scryfall-pp-cli cards search --q example-value

# JSON for scripting and agents
scryfall-pp-cli cards search --q example-value --json

# Filter to specific fields
scryfall-pp-cli cards search --q example-value --json --select id,name,status

# Dry run — show the request without sending
scryfall-pp-cli cards search --q example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
scryfall-pp-cli cards search --q example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
scryfall-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `scryfall-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/scryfall-pp-cli/config.toml`; `--home`, `SCRYFALL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `SCRYFALL_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
