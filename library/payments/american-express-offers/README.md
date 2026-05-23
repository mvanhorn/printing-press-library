# American Express Offers CLI

List card-linked offers and savings on your American Express card

Learn more at [American Express Offers](https://functions.americanexpress.com).

## Install

The recommended path installs both the `american-express-offers-pp-cli` binary and the `pp-american-express-offers` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install american-express-offers
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install american-express-offers --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install american-express-offers --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install american-express-offers --agent claude-code
npx -y @mvanhorn/printing-press-library install american-express-offers --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/american-express-offers-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-american-express-offers --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-american-express-offers --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-american-express-offers skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-american-express-offers. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
american-express-offers-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/american-express-offers-current).
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
    "american-express-offers": {
      "command": "american-express-offers-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to [americanexpress.com](https://www.americanexpress.com) in Chrome, then:

```bash
american-express-offers-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
american-express-offers-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
# List your available offers
american-express-offers-pp-cli offers list-offers

# Check your total savings
american-express-offers-pp-cli offers get-savings
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`analytics --type offers --group-by category`** — Group your synced card-linked offers by category to see at a glance where the bulk of your enrollable rewards live (Dining, Travel, Shopping, etc.).

  _Lets an agent answer 'which category has the most offers I haven't enrolled in yet' in one call instead of pulling and tallying the live list itself._

  ```bash
  american-express-offers-pp-cli analytics --type offers --group-by category --agent
  ```
- **`workflow archive`** — Snapshot every available and enrolled offer into the local SQLite store in one command, so subsequent reads and analytics work offline and at zero rate-limit cost.

  _Lets an agent run a tight loop of `--data-source local` queries against the offer set without repeatedly hitting the live AMEX hub._

  ```bash
  american-express-offers-pp-cli workflow archive
  ```

### Realtime monitoring
- **`tail --interval 60s`** — Poll the offers hub at a chosen interval and emit NDJSON change events when an offer is added, expires, or flips between available and enrolled.

  _Gives an agent a streaming hook to react to new statement-credit offers without having to schedule its own diffing job._

  ```bash
  american-express-offers-pp-cli tail --interval 60s | jq 'select(.event == "change")'
  ```

## Usage

Run `american-express-offers-pp-cli --help` for the full command reference and flag list.

## Commands

### offers

Card-linked offers on your American Express card

- **`american-express-offers-pp-cli offers get-savings`** - Show total amount saved via enrolled card offers
- **`american-express-offers-pp-cli offers list-offers`** - List available and enrolled card offers


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
american-express-offers-pp-cli offers list-offers

# JSON for scripting and agents
american-express-offers-pp-cli offers list-offers --json

# Filter to specific fields
american-express-offers-pp-cli offers list-offers --json --select id,name,status

# Dry run — show the request without sending
american-express-offers-pp-cli offers list-offers --dry-run

# Agent mode — JSON + compact + no prompts in one flag
american-express-offers-pp-cli offers list-offers --agent
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

## Cookbook

```bash
# List all available offers
american-express-offers-pp-cli offers list-offers

# Filter to only enrolled offers
american-express-offers-pp-cli offers list-offers --enrolled

# Filter offers by category (DINING, TRAVEL, SHOPPING, ENTERTAINMENT, SERVICES)
american-express-offers-pp-cli offers list-offers --category DINING

# Filter by offer type (ONLINE, IN_STORE, NEW, FEATURED, STATEMENT_CREDIT, EXTRA_REWARDS)
american-express-offers-pp-cli offers list-offers --filter STATEMENT_CREDIT

# Sort offers alphabetically
american-express-offers-pp-cli offers list-offers --sort A_TO_Z

# Show only key fields in compact form
american-express-offers-pp-cli offers list-offers --json --select id,name,status

# Show your total savings
american-express-offers-pp-cli offers get-savings

# Archive all data locally for offline access
american-express-offers-pp-cli workflow archive

# Check local archive status
american-express-offers-pp-cli workflow status

# Sync data to local SQLite for analytics
american-express-offers-pp-cli sync

# Count enrolled offers
american-express-offers-pp-cli analytics --type offers --group-by status

# Watch for offer changes in real time
american-express-offers-pp-cli tail --interval 60s | jq 'select(.event == "change")'

# Export all offers as CSV
american-express-offers-pp-cli offers list-offers --csv > offers.csv

# Pipe offers to jq for custom filtering
american-express-offers-pp-cli offers list-offers --json | jq '.[] | select(.enrolled == true)'
```

## Health Check

```bash
american-express-offers-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/american-express-offers-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `american-express-offers-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://apigw.americanexpress.com/servicing/v1/contact_management/chats/polls/inquiry_results
- Capture coverage: 543 API entries from 2241 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Protection signals: cloudflare (90% confidence), akamai (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_FAM2kB — Derived from observed POST /CDH_JN/e/b/HUzGnmcaKw2T/h7YE6GDrirh0kmYG/Xmlu/SQlrIE/FAM2kB traffic.; create_Targeting.php — Derived from observed POST /{id}/Targeting.php traffic.; create_beacon — Derived from observed POST /beacon traffic.; create_errors — Derived from observed POST /_/report/errors traffic.; create_events — Derived from observed POST /personalization/v2/customers/treatments/events traffic.; create_inquiry_results — Derived from observed POST /servicing/v1/contact_management/chats/inquiry_results traffic.; get_Asset.php — Derived from observed GET /{id}/Asset.php traffic.; list_axp_personalized_marketing.json — Derived from observed GET /cdaas/one-app/modules/axp-personalized-marketing/1.28.0/en-us/axp-personalized-marketing.json traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
