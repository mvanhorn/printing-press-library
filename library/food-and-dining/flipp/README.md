# Flipp CLI

**Find local Flipp flyer deals, coupons, and grocery savings by ZIP or postal code.**

Search local weekly flyers, coupons, merchants, and item clippings from Flipp's unauthenticated web endpoints. The CLI adds agent-native JSON, local SQLite, basket comparison, expiring-deal views, and unit-price helpers for grocery planning.

## Install

The recommended path installs both the `flipp-pp-cli` binary and the `pp-flipp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install flipp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install flipp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install flipp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install flipp --agent claude-code
npx -y @mvanhorn/printing-press-library install flipp --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/cmd/flipp-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flipp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install flipp --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flipp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flipp --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install flipp --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flipp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/cmd/flipp-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flipp": {
      "command": "flipp-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the CLI and local configuration without calling Flipp.
flipp-pp-cli doctor --dry-run

# Resolve the default ZIP or postal code from the current public IP.
flipp-pp-cli location --json

# Search local flyer and ecommerce deals for a staple item.
flipp-pp-cli items milk --zip 85001 --json --select items.name,items.current_price,ecom_items.name,ecom_items.current_price

# Find active local flyers and IDs to inspect.
flipp-pp-cli flyers list --zip 85001 --json --select flyers.id,flyers.merchant,flyers.valid_to

# Compare a simple shopping basket across local merchants.
flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local savings workflows
- **`basket price`** — Compare a grocery list across nearby merchants and see the cheapest practical basket.

  _Use this when an agent needs to choose where to shop for a list rather than find one item._

  ```bash
  flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent
  ```
- **`deals scan`** — Scan curated staple categories and rank local deals by discount, urgency, and merchant.

  _Use this when the user asks for the best grocery savings nearby without naming exact products._

  ```bash
  flipp-pp-cli deals scan --category groceries --zip 85001 --min-discount 25 --agent
  ```

### Local snapshots
- **`expiring`** — Find local flyer and coupon savings that expire within a chosen window.

  _Use this when timing matters and the agent needs to prioritize deals before they disappear._

  ```bash
  flipp-pp-cli expiring --days 3 --zip 85001 --agent
  ```
- **`coverage`** — Show which nearby merchants have active food flyers, searchable items, and coupon coverage.

  _Use this before shopping-plan work to understand which stores have enough data to compare._

  ```bash
  flipp-pp-cli coverage --zip 85001 --agent
  ```
- **`watchlist add`** — Persist target prices for recurring staples and compare them against future synced snapshots.

  _Use this when the user wants recurring savings alerts rather than a one-time search._

  ```bash
  flipp-pp-cli watchlist add milk --target-price 3.50 --zip 85001 --agent
  ```

### Deal quality
- **`unit-price`** — Normalize item prices by package size when Flipp listings include parseable quantities.

  _Use this when raw prices are misleading because package sizes differ._

  ```bash
  flipp-pp-cli unit-price milk --zip 85001 --agent
  ```

## Recipes

### Compare a short grocery basket

```bash
flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent --select merchant,total,items.name,items.price
```

Groups matching deals by merchant so an agent can pick the cheapest local trip.

### Find expiring food savings

```bash
flipp-pp-cli expiring --days 3 --zip 85001 --agent
```

Prioritizes local deals whose flyer validity window is closing.

### Inspect one flyer clipping list

```bash
flipp-pp-cli flyers items 8005907 --sid 1234567890123456 --json --select name,price,cutout_image_url
```

Returns flyer item text plus image URLs for ambiguous listings.

### Scan grocery category deals

```bash
flipp-pp-cli deals scan --category groceries --zip 85001 --min-discount 25 --agent
```

Fans out staple searches and ranks the best local discounts.

## Usage

Run `flipp-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FLIPP_CONFIG_DIR`, `FLIPP_DATA_DIR`, `FLIPP_STATE_DIR`, or `FLIPP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FLIPP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FLIPP_HOME=/srv/flipp
flipp-pp-cli doctor
```

Under `FLIPP_HOME=/srv/flipp`, the four dirs resolve to `/srv/flipp/config`, `/srv/flipp/data`, `/srv/flipp/state`, and `/srv/flipp/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "flipp": {
      "command": "flipp-pp-mcp",
      "env": {
        "FLIPP_HOME": "/srv/flipp"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FLIPP_DATA_DIR` overrides an explicit `--home` for that kind. Use `FLIPP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FLIPP_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `flipp-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### flyers

Browse active local flyers and flyer items

- **`flipp-pp-cli flyers data`** - Fetch the combined local flyer and coupon data payload
- **`flipp-pp-cli flyers items`** - List item clippings from a specific flyer
- **`flipp-pp-cli flyers list`** - List active flyers for a ZIP or postal code

### items

Search Flipp flyer and ecommerce deal items

- **`flipp-pp-cli items <q>`** - Search local flyer and ecommerce deals by keyword

### location

Resolve a default Flipp location

- **`flipp-pp-cli location`** - Detect the current public IP location as a Flipp postal code

### merchants

List merchants Flipp tracks

- **`flipp-pp-cli merchants`** - List Flipp merchants available for a ZIP or postal code


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flipp-pp-cli flyers list

# JSON for scripting and agents
flipp-pp-cli flyers list --json

# Filter to specific fields
flipp-pp-cli flyers list --json --select id,name,status

# Dry run — show the request without sending
flipp-pp-cli flyers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flipp-pp-cli flyers list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `FLIPP_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `flipp-pp-cli flyers`
- `flipp-pp-cli flyers get`
- `flipp-pp-cli flyers list`
- `flipp-pp-cli flyers search`
- `flipp-pp-cli merchants`
- `flipp-pp-cli merchants get`
- `flipp-pp-cli merchants list`
- `flipp-pp-cli merchants search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
flipp-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `flipp-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/flipp-pp-cli/config.toml`; `--home`, `FLIPP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Search results include irrelevant keyword matches.** — Use a phrase query such as `flipp-pp-cli items "whole milk" --zip 85001 --json`, then filter with `--select` or downstream `jq`.
- **A flyer item has an empty or ambiguous price.** — Run `flipp-pp-cli flyers items <flyer-id> --json --select name,price,cutout_image_url` and inspect the clipping image.
- **A ZIP code returns sparse food results.** — Run `flipp-pp-cli coverage --zip 85001` to see which merchants and flyer categories are available locally.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**flipp-cli**](https://github.com/thomas-chong/flipp-cli) — TypeScript
- [**flippscrape**](https://github.com/Kiizon/flippscrape) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
