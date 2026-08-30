# QuickCommerce API CLI

**Compare live Indian product prices, stock, and delivery ETAs from the terminal, then keep the history locally.**

QuickCommerce CLI covers the provider's REST and hosted-MCP surface while adding a local mirror for repeatable analysis. Use history, fastest delivery, credit planning, stale-data checks, and unit-price views to turn location-sensitive responses into decisions.

## Install

The recommended path installs both the `quickcommerce-pp-cli` binary and the `pp-quickcommerce` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce --agent claude-code
npx -y @mvanhorn/printing-press-library install quickcommerce --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/cmd/quickcommerce-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quickcommerce-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-quickcommerce --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-quickcommerce --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install quickcommerce --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quickcommerce-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `QUICKCOMMERCE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/cmd/quickcommerce-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "quickcommerce": {
      "command": "quickcommerce-pp-mcp",
      "env": {
        "QUICKCOMMERCE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set QUICKCOMMERCE_API_KEY to the API key from the QuickCommerce dashboard. The CLI sends it as X-API-Key and keeps paid calls explicit; platform discovery is available without a key.

## Quick Start

```bash
# Confirm the CLI configuration path without making a paid request.
quickcommerce-pp-cli doctor --dry-run

# See which platforms support search, item lookup, and ETA.
quickcommerce-pp-cli platforms --json

# Preview a location-aware product search request.
quickcommerce-pp-cli products --query milk --latitude 12.9021 --longitude 77.6639 --platform BlinkIt --dry-run

# Preview a cross-platform comparison before spending credits.
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`history prices`** — Query local observations to see price, stock, rating, and availability movement over time.

  _Choose this when an agent needs to explain what changed instead of fetching only the current value._

  ```bash
  quickcommerce-pp-cli history prices --item 501346 --since 30d --agent
  ```
- **`history diff`** — Show the field-level changes between the latest saved observations.

  _Choose this after ingestion when an agent needs a precise price, stock, or ETA delta._

  ```bash
  quickcommerce-pp-cli history diff --item 501346 --latest 2 --agent
  ```
- **`mirror ingest`** — Persist real QuickCommerce command responses and metadata into the local SQLite mirror.

  _Choose this when an agent wants future history, offline search, or a reproducible observation record._

  ```bash
  quickcommerce-pp-cli mirror ingest --stdin --agent
  ```

### Decision-ready comparisons
- **`delivery fastest`** — Rank currently available delivery options while preserving closed or unparseable platforms.

  _Choose this when an agent must recommend a viable fastest platform rather than list raw ETA rows._

  ```bash
  quickcommerce-pp-cli delivery fastest --location 12.9021,77.6639 --agent
  ```
- **`mirror coverage`** — Report observed, missing, and stale platform coverage for a location.

  _Choose this when an agent needs to know whether a location has usable evidence across platforms._

  ```bash
  quickcommerce-pp-cli mirror coverage --location 12.9021,77.6639 --agent
  ```
- **`prices value`** — Compare price per unit from explicit pack quantities without guessing missing units.

  _Choose this when the cheapest sticker price may not be the cheapest comparable quantity._

  ```bash
  quickcommerce-pp-cli prices value --query milk --location 12.9021,77.6639 --agent
  ```

### Agent-native safety
- **`requests plan`** — Calculate fan-out credit cost and affordability before making paid platform requests.

  _Choose this before an agent launches a multi-platform search or ETA fan-out._

  ```bash
  quickcommerce-pp-cli requests plan --platforms blinkit,zepto --operation search --agent
  ```
- **`mirror stale`** — Find saved product and ETA observations that are older than a chosen trust window.

  _Choose this before making decisions from local data whose location-sensitive freshness matters._

  ```bash
  quickcommerce-pp-cli mirror stale --max-age 24h --agent
  ```

## Recipes

### Compare milk prices

```bash
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto,Swiggy --latitude 12.9021 --longitude 77.6639 --agent --select results
```

Ask for only the cross-platform result map instead of the full response envelope.

### Plan a paid fan-out

```bash
quickcommerce-pp-cli requests plan --platforms blinkit,zepto,swiggy --operation search --json
```

Check the expected credit cost before making a multi-platform search.

### Rank delivery speed

```bash
quickcommerce-pp-cli delivery fastest --location 12.9021,77.6639 --agent
```

Rank viable open platforms while retaining unavailable rows for explanation.

### Find stale observations

```bash
quickcommerce-pp-cli mirror stale --max-age 24h --json
```

Detect local data that needs a deliberate refresh.

### Inspect an item trend

```bash
quickcommerce-pp-cli history prices --item 501346 --since 30d --agent
```

Review saved price, inventory, and availability movement for one item. 

## Usage

Run `quickcommerce-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `QUICKCOMMERCE_CONFIG_DIR`, `QUICKCOMMERCE_DATA_DIR`, `QUICKCOMMERCE_STATE_DIR`, or `QUICKCOMMERCE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `QUICKCOMMERCE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export QUICKCOMMERCE_HOME=/srv/quickcommerce
quickcommerce-pp-cli doctor
```

Under `QUICKCOMMERCE_HOME=/srv/quickcommerce`, the four dirs resolve to `/srv/quickcommerce/config`, `/srv/quickcommerce/data`, `/srv/quickcommerce/state`, and `/srv/quickcommerce/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "quickcommerce": {
      "command": "quickcommerce-pp-mcp",
      "env": {
        "QUICKCOMMERCE_HOME": "/srv/quickcommerce"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `QUICKCOMMERCE_DATA_DIR` overrides an explicit `--home` for that kind. Use `QUICKCOMMERCE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `QUICKCOMMERCE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `quickcommerce-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Inspect API credit balance and packs.

- **`quickcommerce-pp-cli account`** - Check credit balance, usage, and expiry.

### comparison

Compare products across multiple platforms.

- **`quickcommerce-pp-cli comparison`** - Search and compare products across multiple platforms.

### delivery

Check delivery timing and store availability.

- **`quickcommerce-pp-cli delivery compare`** - Compare delivery ETAs across quick-commerce platforms.
- **`quickcommerce-pp-cli delivery eta`** - Get delivery ETA and store availability for one platform.

### items

Fetch current details for a platform item.

- **`quickcommerce-pp-cli items`** - Get live price, stock, and availability for an item ID.

### platforms

Discover platform support and ETA scope.

- **`quickcommerce-pp-cli platforms`** - List platforms supported by each endpoint.

### products

Search products by keyword, location, and platform.

- **`quickcommerce-pp-cli products`** - Search products on one platform by keyword and location.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`quickcommerce-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`quickcommerce-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`quickcommerce-pp-cli learnings list`** - Inspect taught rows
- **`quickcommerce-pp-cli learnings forget <query>`** - Undo a teach
- **`quickcommerce-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`quickcommerce-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`quickcommerce-pp-cli teach-pattern`** - Install a query/resource template up front
- **`quickcommerce-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `QUICKCOMMERCE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `quickcommerce-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639

# JSON for scripting and agents
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --json
# Filter to specific fields
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --json --select query,platforms,lat

# Dry run — show the request without sending
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
quickcommerce-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `quickcommerce-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/quickcommerce-pp-cli/config.toml`; `--home`, `QUICKCOMMERCE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `QUICKCOMMERCE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `quickcommerce-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `quickcommerce-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $QUICKCOMMERCE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `quickcommerce-pp-cli products --json`, `quickcommerce-pp-cli items --json`, or `quickcommerce-pp-cli platforms --json` to inspect available data.

### API-specific
- **401 invalid or missing API key** — Export QUICKCOMMERCE_API_KEY and rerun quickcommerce-pp-cli doctor --json.
- **402 no credits remaining** — Run quickcommerce-pp-cli account --json, then reduce the platform list or add credits.
- **422 invalid platform or location parameters** — Run quickcommerce-pp-cli platforms --json and use a supported platform with decimal latitude/longitude.
- **DMart, JioMart, or Minutes returns location errors** — Add --pincode with the six-digit delivery pincode.
- **Local results may be stale** — Run quickcommerce-pp-cli mirror stale --max-age 24h --json before relying on the mirror.
