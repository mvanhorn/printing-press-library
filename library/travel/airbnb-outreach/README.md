# Airbnb CLI

Script Airbnb's real authenticated surface — search stays, contact hosts and property owners with templated messages and photos, and keep an offline searchable archive of every conversation and saved listing.

Learn more at [Airbnb](https://www.airbnb.com).

## Install

The recommended path installs both the `airbnb-outreach-pp-cli` binary and the `pp-airbnb-outreach` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install airbnb
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install airbnb --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install airbnb --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install airbnb --agent claude-code
npx -y @mvanhorn/printing-press install airbnb --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbnb-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb-outreach --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb-outreach --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-airbnb-outreach skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-airbnb-outreach. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbnb-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airbnb": {
      "command": "airbnb-outreach-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Log in (import your Chrome session)

```bash
airbnb-outreach-pp-cli auth login --chrome
airbnb-outreach-pp-cli me --json
```

No password is handled — this imports your logged-in Airbnb session cookies from
Chrome. Close Chrome first if the cookie DB is locked, or paste a Cookie header
with `airbnb-outreach-pp-cli auth login --cookies "<header>"`. Public search works without
logging in.

### 3. Try Your First Command

```bash
airbnb-outreach-pp-cli search "Berlin, Germany" --checkin 2026-08-10 --checkout 2026-08-14 --adults 2 \
  --json --select searchResults.demandStayListing.id,searchResults.title
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Outreach that scales
- **`outreach run`** — Search a location and message the top hosts with a templated message in one command.

  _Reach for this when a user wants to contact many property owners at once (relocation, long-stay, business inquiry)._

  ```bash
  airbnb-outreach-pp-cli outreach run "Berlin, Germany" --message "Hi {name} team, interested in a monthly stay — possible?" --limit 5
  ```
- **`outreach crm`** — Local record of every host you contacted, when, and about which listing.

  _Use after an outreach run to track who was contacted before following up._

  ```bash
  airbnb-outreach-pp-cli outreach crm --json
  ```

### Local state that compounds
- **`archive search`** — Full-text search across every message thread you've synced, offline.

  _Use to find what a host promised across months of conversations without scrolling the web inbox._

  ```bash
  airbnb-outreach-pp-cli archive search "early check-in"
  ```
- **`watch check`** — Track saved listings' prices over time and report drops.

  _Use to catch a price drop on a listing a user is considering._

  ```bash
  airbnb-outreach-pp-cli watch add 49070135 --checkin 2026-08-10 --checkout 2026-08-14 && airbnb-outreach-pp-cli watch check
  ```

### Reachability mitigation
- **`ops refresh`** — Re-harvest Airbnb's current GraphQL persisted-query hashes from its own JS so the CLI survives Airbnb deploys.

  _Run this if commands start failing after an Airbnb update, before assuming the CLI is broken._

  ```bash
  airbnb-outreach-pp-cli ops refresh
  ```

## Usage

Run `airbnb-outreach-pp-cli --help` for the full command reference and flag list.

## Commands

Airbnb has no public API, so every command drives Airbnb's internal GraphQL
surface. Reads work for public data with no login; private data and all writes
need `auth login`.

### Auth
- **`auth login --chrome`** / **`auth login --cookies "<header>"`** — import your Airbnb session
- **`auth status`** / **`auth logout`** — check or clear the session

### Search & listings (public)
- **`search <location>`** — search stays by dates, guests, price, room type
- **`listing <id>`** — full listing detail (PDP sections)
- **`quote <id>`** — read-only price breakdown for a listing and dates

### Messaging & outreach (login required)
- **`inbox`** — list your message threads
- **`thread <id>`** — read a conversation
- **`contact <listing-id> --message "…" --checkin … --checkout …`** — start a conversation with a host (`--confirm` to send; Airbnb inquiries need trip dates)
- **`message send <thread-id> --text "…"`** — send a message (`--confirm` to send)
- **`message send-image <thread-id> --file photo.jpg`** — send photo(s) _(experimental — see Known Gaps)_
- **`message mark-read <thread-id>`** — mark a thread read _(experimental — see Known Gaps)_
- **`outreach run <location> --message "…"`** — bulk-contact hosts from a search (`--confirm` to send)
- **`outreach crm`** — local record of who you contacted

### Your data (login required)
- **`me`** — signed-in account
- **`wishlist list`** / **`wishlist items <id…>`** — saved listings
- **`trips`** — your reservations

### Local intelligence
- **`archive index`** / **`archive search <query>`** — offline full-text conversation search
- **`watch add|list|remove|check`** — track listing prices over time
- **`ops refresh`** / **`ops list`** — self-heal the GraphQL operation-hash registry

## Known Gaps

Every command's request shape was captured from Airbnb's live web client. Two are
not yet confirmed end-to-end:

- **`message send-image`** — the text-send path is validated, but the image
  upload chain (`GetSignedUrls` → `CreateMediaItems`) is unconfirmed; an upload
  step may return an error naming the failing step.
- **`message mark-read`** — Airbnb drives read-receipts over a realtime sync
  channel, so this REST mutation shape is unconfirmed and may error.

Persisted-query hashes rotate on Airbnb deploys; run **`ops refresh`** if a
command starts failing after an Airbnb update.

Writes default to a **preview** and require `--confirm` (or `--yes`) to actually
send. Nothing is charged automatically.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airbnb-outreach-pp-cli markets

# JSON for scripting and agents
airbnb-outreach-pp-cli markets --json

# Filter to specific fields
airbnb-outreach-pp-cli markets --json --select id,name,status

# Dry run — show the request without sending
airbnb-outreach-pp-cli markets --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airbnb-outreach-pp-cli markets --agent
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
airbnb-outreach-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/airbnb-outreach-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
