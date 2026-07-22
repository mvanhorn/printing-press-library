# Jimmy Johns CLI

CLI for the Jimmy John's ordering API. Browse stores, menus, and product
modifiers; manage your cart; place orders; view rewards and saved payments.
Backed by Jimmy John's proprietary API at www.jimmyjohns.com/api and
authenticated via cookies imported from a logged-in Chrome session
(PerimeterX clearance + JJ session cookies).

Created by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `jimmy-johns-pp-cli` binary and the `pp-jimmy-johns` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns --agent claude-code
npx -y @mvanhorn/printing-press-library install jimmy-johns --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/cmd/jimmy-johns-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jimmy-johns-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jimmy-johns --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jimmy-johns --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install jimmy-johns --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
jimmy-johns-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jimmy-johns-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/cmd/jimmy-johns-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jimmy-johns": {
      "command": "jimmy-johns-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to jimmyjohns.com in Chrome, then:

```bash
jimmy-johns-pp-cli auth login --chrome
```

Or import an existing browser capture:

```bash
jimmy-johns-pp-cli auth login --cookies-file storage-state.json
```

`--cookies-file` accepts Playwright storage-state JSON or a raw `Cookie:` header text file. The Chrome path requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
jimmy-johns-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
jimmy-johns-pp-cli stores list
```

## Usage

Run `jimmy-johns-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JIMMY_JOHNS_CONFIG_DIR`, `JIMMY_JOHNS_DATA_DIR`, `JIMMY_JOHNS_STATE_DIR`, or `JIMMY_JOHNS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JIMMY_JOHNS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JIMMY_JOHNS_HOME=/srv/jimmy-johns
jimmy-johns-pp-cli doctor
```

Under `JIMMY_JOHNS_HOME=/srv/jimmy-johns`, the four dirs resolve to `/srv/jimmy-johns/config`, `/srv/jimmy-johns/data`, `/srv/jimmy-johns/state`, and `/srv/jimmy-johns/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "jimmy-johns": {
      "command": "jimmy-johns-pp-mcp",
      "env": {
        "JIMMY_JOHNS_HOME": "/srv/jimmy-johns"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JIMMY_JOHNS_DATA_DIR` overrides an explicit `--home` for that kind. Use `JIMMY_JOHNS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JIMMY_JOHNS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `jimmy-johns-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

User account, profile, addresses, and saved payments

- **`jimmy-johns-pp-cli account current`** - Get the authenticated user's profile (name, email, preferences).
- **`jimmy-johns-pp-cli account delivery-addresses`** - List the authenticated user's saved delivery addresses.
- **`jimmy-johns-pp-cli account login`** - Authenticate with email + password. Sets JJ session cookies.
- **`jimmy-johns-pp-cli account saved-payments`** - List the authenticated user's saved payment methods.
- **`jimmy-johns-pp-cli account web-token`** - Refresh the web session token (called internally by the SPA).

### menu

Menu products, filters, and modifier options

- **`jimmy-johns-pp-cli menu product-filters`** - List available menu filter dimensions (categories, dietary tags, allergens).
- **`jimmy-johns-pp-cli menu product-modifiers`** - List modifier groups (bread, toppings, add-ons) for a specific product.
- **`jimmy-johns-pp-cli menu products`** - List menu products for the current store (subs, sides, drinks, cookies, catering).

### order

Cart and order management

- **`jimmy-johns-pp-cli order add-items`** - Add one or more items to the current cart in a single call.
- **`jimmy-johns-pp-cli order current`** - Get the current in-progress order/cart.
- **`jimmy-johns-pp-cli order upsell`** - Get upsell suggestions for the current cart (sides, drinks, cookies).

### rewards

Freaky Fast Rewards points balance and catalog

- **`jimmy-johns-pp-cli rewards catalog`** - List available reward redemptions for the current points balance.
- **`jimmy-johns-pp-cli rewards summary`** - Get the authenticated user's rewards points balance and recent activity.

### stores

Jimmy John's store locations and operating info

- **`jimmy-johns-pp-cli stores get-disclaimers`** - Get store-specific disclaimers (delivery zone caveats, hours warnings).
- **`jimmy-johns-pp-cli stores list`** - List stores. Accepts an address search or filter; returns stores with hours, distance, pickup/delivery flags.

### system

System utilities (Google Maps signing for store finder)

- **`jimmy-johns-pp-cli system`** - Sign a Google Maps URL for client-side use (used internally by store finder)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`jimmy-johns-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`jimmy-johns-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`jimmy-johns-pp-cli learnings list`** - Inspect taught rows
- **`jimmy-johns-pp-cli learnings forget <query>`** - Undo a teach
- **`jimmy-johns-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`jimmy-johns-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`jimmy-johns-pp-cli teach-pattern`** - Install a query/resource template up front
- **`jimmy-johns-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `JIMMY_JOHNS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `jimmy-johns-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jimmy-johns-pp-cli stores list

# JSON for scripting and agents
jimmy-johns-pp-cli stores list --json

# Filter to specific fields
jimmy-johns-pp-cli stores list --json --select id,name,status

# Dry run — show the request without sending
jimmy-johns-pp-cli stores list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jimmy-johns-pp-cli stores list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `JIMMY_JOHNS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `jimmy-johns-pp-cli menu`
- `jimmy-johns-pp-cli menu get`
- `jimmy-johns-pp-cli menu list`
- `jimmy-johns-pp-cli menu search`
- `jimmy-johns-pp-cli stores`
- `jimmy-johns-pp-cli stores get`
- `jimmy-johns-pp-cli stores list`
- `jimmy-johns-pp-cli stores search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
jimmy-johns-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `jimmy-johns-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/jimmy-johns-pp-cli/config.toml`; `--home`, `JIMMY_JOHNS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `jimmy-johns-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
