# The Points Guy CLI

**The first CLI and local database for The Points Guy: search every article, look up any card's real terms, and turn points valuations into answers you can script.**

The Points Guy is the reference for points valuations and credit-card terms, but that data is buried in long articles and JS-rendered pages. This CLI mirrors TPG's valuations, card database, and content into a local SQLite store so you can search (live Algolia or offline FTS), value a balance with 'worth', decide points-vs-cash with 'redeem-check', and compare cards with 'cards compare' — all agent-native with --json and --select.

## Install

The recommended path installs both the `thepointsguy-pp-cli` binary and the `pp-thepointsguy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy --agent claude-code
npx -y @mvanhorn/printing-press-library install thepointsguy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/cmd/thepointsguy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thepointsguy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-thepointsguy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-thepointsguy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install thepointsguy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thepointsguy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/cmd/thepointsguy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "thepointsguy": {
      "command": "thepointsguy-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check: confirms the site is reachable and search credentials can be discovered; needs no key.
thepointsguy-pp-cli doctor --dry-run

# Live full-text search over TPG content.
thepointsguy-pp-cli search "amex platinum" --limit 5

# Look up a program's cents-per-point value.
thepointsguy-pp-cli valuations --program "Chase Ultimate Rewards"

# Mirror articles, cards, and valuations into the local store for offline + transcendence commands.
thepointsguy-pp-cli sync

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Points math that compounds
- **`redeem-check`** — Tells you whether to use points or pay cash for a specific booking, using TPG's valuation as the baseline.

  _Reach for this to answer 'points or cash?' for a concrete fare or rate instead of eyeballing a valuation table._

  ```bash
  thepointsguy-pp-cli redeem-check --program "Chase Ultimate Rewards" --points 60000 --cash 900 --agent
  ```
- **`worth`** — Converts a points/miles balance into an estimated dollar value using TPG's monthly valuation.

  _Reach for this to turn a balance into dollars in one call._

  ```bash
  thepointsguy-pp-cli worth --program "American AAdvantage" --points 75000 --agent
  ```
- **`portfolio`** — Values many balances across different programs at once from stdin or a file and totals them.

  _Reach for this to get a single dollar total across every loyalty currency you hold._

  ```bash
  thepointsguy-pp-cli portfolio "Amex Membership Rewards=120000" "United MileagePlus=50000" --agent
  ```

### Local state that compounds
- **`cards compare`** — Compares two or more credit cards across annual fee, APRs, welcome bonus, and rewards.

  _Reach for this to line up two cards' real terms without opening two tabs._

  ```bash
  thepointsguy-pp-cli cards compare chase-sapphire-preferred-card chase-sapphire-reserve --agent
  ```
- **`valuations drift`** — Shows how a program's cents-per-point value changed month over month.

  _Reach for this to see whether a currency is being devalued over time._

  ```bash
  thepointsguy-pp-cli valuations drift --program "Marriott Bonvoy" --months 6 --agent
  ```
- **`since`** — Lists everything TPG published in the last N hours or days across all categories.

  _Reach for this to catch up on new deals and news since you last checked._

  ```bash
  thepointsguy-pp-cli since 24h --agent
  ```

## Recipes

### Value a balance

```bash
thepointsguy-pp-cli worth --program "American AAdvantage" --points 75000 --agent
```

Turns a mileage balance into an estimated dollar value using TPG's valuation.

### Points or cash?

```bash
thepointsguy-pp-cli redeem-check --program "Chase Ultimate Rewards" --points 60000 --cash 900 --agent
```

Verdict on whether a specific redemption beats paying cash.

### Narrow a big search response

```bash
thepointsguy-pp-cli search "lounge access" --agent --select hits.title,hits.url,hits.category
```

Uses --select on the nested Algolia response so an agent only reads the fields it needs.

### Compare two cards

```bash
thepointsguy-pp-cli cards compare chase-sapphire-preferred-card chase-sapphire-reserve --agent
```

Side-by-side annual fee, APRs, welcome bonus, and rewards from the local mirror.

## Usage

Run `thepointsguy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `THEPOINTSGUY_CONFIG_DIR`, `THEPOINTSGUY_DATA_DIR`, `THEPOINTSGUY_STATE_DIR`, or `THEPOINTSGUY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `THEPOINTSGUY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export THEPOINTSGUY_HOME=/srv/thepointsguy
thepointsguy-pp-cli doctor
```

Under `THEPOINTSGUY_HOME=/srv/thepointsguy`, the four dirs resolve to `/srv/thepointsguy/config`, `/srv/thepointsguy/data`, `/srv/thepointsguy/state`, and `/srv/thepointsguy/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "thepointsguy": {
      "command": "thepointsguy-pp-mcp",
      "env": {
        "THEPOINTSGUY_HOME": "/srv/thepointsguy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `THEPOINTSGUY_DATA_DIR` overrides an explicit `--home` for that kind. Use `THEPOINTSGUY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `THEPOINTSGUY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `thepointsguy-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### articles

The Points Guy articles and news

- **`thepointsguy-pp-cli articles <section> <slug>`** - Fetch an article's structured page data by section and slug

### cards

The Points Guy credit-card database

- **`thepointsguy-pp-cli cards <slug>`** - Fetch a credit card's structured page data (fees, APRs, welcome bonus) by slug


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
thepointsguy-pp-cli articles mock-value mock-value

# JSON for scripting and agents
thepointsguy-pp-cli articles mock-value mock-value --json

# Filter to specific fields
thepointsguy-pp-cli articles mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
thepointsguy-pp-cli articles mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
thepointsguy-pp-cli articles mock-value mock-value --agent
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

## Health Check

```bash
thepointsguy-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `thepointsguy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/thepointsguy-pp-cli/config.toml`; `--home`, `THEPOINTSGUY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns nothing / credential error** — Run 'thepointsguy-pp-cli doctor' to re-discover the public Algolia app id and search key from the site bundle; they rotate with deploys.
- **worth/portfolio/drift return empty** — Run 'thepointsguy-pp-cli sync' first to populate the local valuations and card tables.
- **a card slug is not found** — Run 'thepointsguy-pp-cli cards list' to see valid slugs from the card sitemap.
