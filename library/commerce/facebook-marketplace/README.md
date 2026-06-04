# Facebook Marketplace CLI

**A write-gated Marketplace seller CLI for search, listing creation, photo upload, local watches, and replies.**

Facebook Marketplace is an authenticated browser surface, so this CLI treats the browser session as the credential and keeps writes opt-in. It turns captured Marketplace GraphQL traffic into repeatable commands, then layers local watches, matches, stale listing checks, and seller drafting on top.

Learn more at [Facebook Marketplace](https://www.facebook.com).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `facebook-marketplace-pp-cli` binary and the `pp-facebook-marketplace` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install facebook-marketplace
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install facebook-marketplace --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install facebook-marketplace --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install facebook-marketplace --agent claude-code
npx -y @mvanhorn/printing-press-library install facebook-marketplace --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/facebook-marketplace/cmd/facebook-marketplace-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/facebook-marketplace-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-facebook-marketplace --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-facebook-marketplace --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install facebook-marketplace --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
facebook-marketplace-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/facebook-marketplace-current).
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
    "facebook-marketplace": {
      "command": "facebook-marketplace-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Run `facebook-marketplace-pp-cli auth login --chrome` while logged in to Facebook in Chrome. The captured browser session is the credential; do not store session material in the Dropbox project workspace.

## Quick Start

```bash
# Capture the local Facebook browser session before any live command.
facebook-marketplace-pp-cli doctor

# Confirm the session still reaches Marketplace.
facebook-marketplace-pp-cli doctor

# Run the captured search operation with a realistic variable payload.
facebook-marketplace-pp-cli marketplace-search content --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Seller workflow
- **`draft`** — Draft a Marketplace title, description, and price suggestion from photos and notes.

  _Use this when preparing a seller listing before opening a write-gated post flow._

  ```bash
  facebook-marketplace-pp-cli draft --photos chair-front.jpg,chair-tag.jpg --notes "walnut dining chair, small scratch on back" --json
  ```
- **`reply`** — Prepare and send a seller inbox reply only when `--write` and doctor gating both pass.

  _Use this only for human-approved sell-side messaging._

  ```bash
  facebook-marketplace-pp-cli reply --thread 1525836598898750 --message "Yes, it is still available." --write --json
  ```

### Buy-side workflow
- **`watch add`** — Persist a Marketplace search watch with deterministic keyword, price, and distance filters.

  _Use this when the agent needs to monitor Marketplace without deciding relevance on every raw result._

  ```bash
  facebook-marketplace-pp-cli watch add --name "eames" --query "eames lounge" --max-price 1500 --radius 60 --must-have-keywords "chair,lounge" --json
  ```
- **`matches`** — Show new watch matches after deterministic filtering.

  _Use this when an agent needs the shortlist worth showing a human buyer._

  ```bash
  facebook-marketplace-pp-cli matches --new --json
  ```

### Local mirror
- **`stale`** — Find local seller listings older than seven days with no engagement.

  _Use this when deciding which seller listings need price changes or renewal._

  ```bash
  facebook-marketplace-pp-cli stale --days 7 --json
  ```

## Usage

Run `facebook-marketplace-pp-cli --help` for the full command reference and flag list.

## Commands

### composer

Sell-side composer helper operations.

- **`facebook-marketplace-pp-cli composer price_prediction`** - Fetch Marketplace composer price prediction.
- **`facebook-marketplace-pp-cli composer root`** - Fetch Marketplace listing composer metadata.
- **`facebook-marketplace-pp-cli composer shipping_options`** - Fetch calculated shipping options for a draft listing.

### inbox

Marketplace inbox and messaging operations.

- **`facebook-marketplace-pp-cli inbox list`** - Fetch Marketplace inbox overview.
- **`facebook-marketplace-pp-cli inbox message_seller`** - Send a Marketplace seller message.
- **`facebook-marketplace-pp-cli inbox seller_threads`** - Fetch Marketplace seller inbox threads.
- **`facebook-marketplace-pp-cli inbox seller_threads_page`** - Fetch a page of Marketplace seller inbox threads.

### listing

Listing detail and sell-side listing operations.

- **`facebook-marketplace-pp-cli listing change_availability`** - Change a Marketplace listing availability state.
- **`facebook-marketplace-pp-cli listing create`** - Create a Marketplace listing from a prepared composer payload; pass `--photo` to upload local photos first.
- **`facebook-marketplace-pp-cli listing delete`** - Delete a Marketplace for-sale item.
- **`facebook-marketplace-pp-cli listing get`** - Fetch a Marketplace listing detail page payload.
- **`facebook-marketplace-pp-cli listing media`** - Fetch Marketplace listing media payload.
- **`facebook-marketplace-pp-cli listing upload-photo`** - Upload a local photo and return the Marketplace composer `photo_id`.

### marketplace

Marketplace browse and location operations.

- **`facebook-marketplace-pp-cli marketplace browse_feed`** - Fetch Marketplace browse feed results.
- **`facebook-marketplace-pp-cli marketplace set_browse_radius`** - Set Marketplace browse radius.
- **`facebook-marketplace-pp-cli marketplace set_buy_location`** - Set Marketplace buying location.

### marketplace_search

Marketplace search operations.

- **`facebook-marketplace-pp-cli marketplace_search content`** - Search Marketplace listings.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource

# JSON for scripting and agents
facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource --json

# Filter to specific fields
facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource --json --select id,name,status

# Dry run — show the request without sending
facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
facebook-marketplace-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/facebook-marketplace-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FACEBOOK_MARKET_COOKIES` | harvested | Yes | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `facebook-marketplace-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FACEBOOK_MARKET_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Marketplace commands return a login or checkpoint page.** — Open Facebook in Chrome, resolve the checkpoint manually, rerun `auth login --chrome`, then rerun `doctor`.
- **A write command refuses to run.** — Rerun `doctor`; writes require both `--write` and a recent passing doctor check.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport over HTTP/3 for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
