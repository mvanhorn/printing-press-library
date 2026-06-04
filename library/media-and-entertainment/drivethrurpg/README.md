# Drivethrurpg CLI

Curated seed spec for public DriveThruRPG catalog search and authenticated library downloads.

Created by [@kingdomseed](https://github.com/kingdomseed) (Jason Holt).

## Install

The recommended path installs both the `drivethrurpg-pp-cli` binary and the `pp-drivethrurpg` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install drivethrurpg
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install drivethrurpg --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install drivethrurpg --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install drivethrurpg --agent claude-code
npx -y @mvanhorn/printing-press-library install drivethrurpg --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drivethrurpg-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-drivethrurpg --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-drivethrurpg --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install drivethrurpg --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drivethrurpg-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DRIVETHRURPG_DTRPG_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "drivethrurpg": {
      "command": "drivethrurpg-pp-mcp",
      "env": {
        "DRIVETHRURPG_DTRPG_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export DRIVETHRURPG_DTRPG_TOKEN="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/drivethrurpg-pp-cli/config.toml`.

### 3. Verify Setup

```bash
drivethrurpg-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
drivethrurpg-pp-cli categories list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Catalog discovery

- **`search`** — Search DriveThruRPG's public product catalog by keyword while keeping JSON output agent-friendly.

  _Agents can answer store-discovery questions before asking the user for credentials._

  ```bash
  drivethrurpg-pp-cli search "Cyberpunk" --agent --limit 5 --data-source live
  ```

### Authenticated library

- **`order-products`** — List owned DriveThruRPG order products and file indexes using the Library App Application Key flow.

  _Agents can inspect owned purchases and choose the right file index without scraping website pages._

  ```bash
  drivethrurpg-pp-cli order-products --agent --page-size 10
  ```
- **`download`** — Prepare, poll, and save a purchased DriveThruRPG file from a single command.

  _Agents can execute the full authenticated file retrieval workflow without hand-rolling polling logic._

  ```bash
  drivethrurpg-pp-cli download ORDER_PRODUCT_ID --dry-run --agent
  ```

## Usage

Run `drivethrurpg-pp-cli --help` for the full command reference and flag list.

## Commands

### auth-key

Manage auth key

- **`drivethrurpg-pp-cli auth-key`** - Exchange a DriveThruRPG Library App Application Key for API tokens

### categories

Manage categories

- **`drivethrurpg-pp-cli categories get-category`** - Get public category details
- **`drivethrurpg-pp-cli categories list`** - List public categories

### filters

Manage filters

- **`drivethrurpg-pp-cli filters get`** - Get public filter details
- **`drivethrurpg-pp-cli filters list`** - List public filters

### order-products

Manage order products

- **`drivethrurpg-pp-cli order-products`** - List authenticated user's purchased library products

### products

Manage products

- **`drivethrurpg-pp-cli products get`** - Get public product details
- **`drivethrurpg-pp-cli products search`** - Search and list public DriveThruRPG products

### publishers

Manage publishers

- **`drivethrurpg-pp-cli publishers <publisherId>`** - Get public publisher details

### reviews

Manage reviews

- **`drivethrurpg-pp-cli reviews`** - List public reviews, optionally for a product

### search-ahead

Manage search ahead

- **`drivethrurpg-pp-cli search-ahead`** - Search ahead across products, publishers, categories, and filters

### special-offers

Manage special offers

- **`drivethrurpg-pp-cli special-offers`** - List active public special offers


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
drivethrurpg-pp-cli categories list

# JSON for scripting and agents
drivethrurpg-pp-cli categories list --json

# Filter to specific fields
drivethrurpg-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
drivethrurpg-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
drivethrurpg-pp-cli categories list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
drivethrurpg-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/drivethrurpg-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DRIVETHRURPG_DTRPG_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `drivethrurpg-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `drivethrurpg-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DRIVETHRURPG_DTRPG_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
