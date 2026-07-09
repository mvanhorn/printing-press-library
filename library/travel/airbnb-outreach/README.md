# Airbnb CLI

**Script Airbnb's real logged-in surface — search stays, contact hosts with templated messages and photos, and keep an offline archive of every conversation.**

Airbnb has no public API, so this CLI drives its internal GraphQL surface directly: it authenticates by importing your logged-in Chrome session (no password), searches stays, reads and sends messages, and adds bulk host outreach, a local CRM, offline conversation search, and price watching that the website has no equivalent for.

Learn more at [Airbnb](https://www.airbnb.com).

Created by [@jimpresting](https://github.com/jimpresting) (JimPresting).

## Install

The recommended path installs both the `airbnb-outreach-pp-cli` binary and the `pp-airbnb-outreach` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach --agent claude-code
npx -y @mvanhorn/printing-press-library install airbnb-outreach --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbnb-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb-outreach --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airbnb-outreach --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install airbnb-outreach --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

## Authentication

No password is handled. Run 'auth login --chrome' to import your Airbnb session cookies from Chrome (close Chrome first, or paste a Cookie header with 'auth login --cookies'). Public search works with no login; private data (inbox, messages, trips, wishlists) needs the session.

## Quick Start

```bash
# import your logged-in Airbnb session from Chrome
airbnb-outreach-pp-cli auth login --chrome


# confirm you're authenticated
airbnb-outreach-pp-cli me --json


# search stays; note listing ids
airbnb-outreach-pp-cli search "Berlin, Germany" --checkin 2026-08-10 --checkout 2026-08-14 --adults 2 --json --select id,name,price,rating


# list your message threads
airbnb-outreach-pp-cli inbox --json


# preview a bulk outreach (add --confirm to send)
airbnb-outreach-pp-cli outreach run "Berlin, Germany" --message "Hi {name}, interested in a long stay" --limit 5

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

## Recipes


### Search and narrow with --select

```bash
airbnb-outreach-pp-cli search "Lisbon" --price-max 120 --agent --select searchResults.demandStayListing.id,searchResults.title,searchResults.structuredDisplayPrice.primaryLine.accessibilityLabel
```

Search returns large nested payloads; --agent with --select keeps only the fields you need.

### Contact one host (guarded)

```bash
airbnb-outreach-pp-cli contact 49070135 --message "Hi, is a monthly stay possible?" --confirm
```

Opens a conversation with a listing's host; without --confirm it only previews.

### Archive then search conversations

```bash
airbnb-outreach-pp-cli archive index && airbnb-outreach-pp-cli archive search "deposit"
```

Pull your inbox into local SQLite, then full-text search it offline.

### Watch a listing's price

```bash
airbnb-outreach-pp-cli watch add 49070135 --checkin 2026-08-10 --checkout 2026-08-14 && airbnb-outreach-pp-cli watch check --json
```

Snapshot a listing's price and detect changes on later checks.

## Usage

Run `airbnb-outreach-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AIRBNB_CONFIG_DIR`, `AIRBNB_DATA_DIR`, `AIRBNB_STATE_DIR`, or `AIRBNB_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AIRBNB_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AIRBNB_HOME=/srv/airbnb
airbnb-outreach-pp-cli doctor
```

Under `AIRBNB_HOME=/srv/airbnb`, the four dirs resolve to `/srv/airbnb/config`, `/srv/airbnb/data`, `/srv/airbnb/state`, and `/srv/airbnb/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "airbnb": {
      "command": "airbnb-outreach-pp-mcp",
      "env": {
        "AIRBNB_HOME": "/srv/airbnb"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AIRBNB_DATA_DIR` overrides an explicit `--home` for that kind. Use `AIRBNB_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AIRBNB_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `airbnb-outreach-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### markets

Public locale/market metadata (used for the health check)

- **`airbnb-outreach-pp-cli markets`** - Fetch Airbnb locale/market metadata (public, no auth)


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

Run `airbnb-outreach-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/airbnb-outreach-pp-cli/config.toml`; `--home`, `AIRBNB_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login --chrome: 'cannot access the file'** — Close Chrome (it locks the cookie DB), then re-run; or use 'auth login --cookies "<header>"'.
- **A command returns 'blocked by Airlock'** — Open airbnb.com in a browser, pass the human check, then run 'auth login --chrome' again.
- **'unknown operation' or a command suddenly fails** — Run 'airbnb-outreach-pp-cli ops refresh' to re-harvest Airbnb's current operation hashes.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pyairbnb**](https://github.com/johnbalvin/pyairbnb) — Python
- [**airbnb-cli (various)**](https://github.com/topics/airbnb) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
