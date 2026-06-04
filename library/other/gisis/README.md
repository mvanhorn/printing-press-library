# GISIS CLI

**Authoritative IMO ship particulars on the command line, plus a local cache that turns one-shot lookups into a compounding vessel index.**

GISIS is the IMO's canonical ship registry, gated by a login + Cloudflare Turnstile and only readable from a server-rendered web app. This CLI scrapes it politely (1 req/2-3s), caches every result to local SQLite, exposes 'ship get' as an MCP tool, and adds maritime-DD-specific commands like 'ship history' for flag-hop detection and 'owner fleet' for counterparty exposure that the GISIS web app itself can't answer.

Learn more at [GISIS](https://gisis.imo.org).

Created by [@6myfzqx6bv-ctrl](https://github.com/6myfzqx6bv-ctrl) (ivory_elephant).

## Install

The recommended path installs both the `gisis-pp-cli` binary and the `pp-gisis` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gisis
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gisis --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gisis --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gisis --agent claude-code
npx -y @mvanhorn/printing-press-library install gisis --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gisis/cmd/gisis-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gisis-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gisis --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-gisis --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install gisis --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
gisis-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gisis-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/gisis/cmd/gisis-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gisis": {
      "command": "gisis-pp-mcp"
    }
  }
}
```

</details>

## Authentication

GISIS requires a free IMO Web Accounts login (https://webaccounts.imo.org/Register.aspx?App=GISISPublic) with a Cloudflare Turnstile challenge. Programmatic login is blocked, so this CLI uses the press-auth companion: install it once, run 'press-auth login gisis.imo.org', a controlled Chrome window opens for you to sign in, and your cookies are captured into the macOS keychain. The CLI then reads cookies on demand. Sessions die after ~30 min of inactivity; run 'gisis-pp-cli auth ping' from cron to keep them warm, or re-run 'press-auth login' when the session expires.

## Quick Start

```bash
# Health check first — confirms binary works without making network calls.
gisis-pp-cli doctor --dry-run

# Install the cookie companion (one-time).
go install github.com/mvanhorn/cli-printing-press/v4/cmd/press-auth@latest

# Capture your GISIS session cookies via a controlled Chrome window.
press-auth login gisis.imo.org --login-url https://webaccounts.imo.org/Login.aspx?App=GISISPublic --jwt-carrier-cookie ASP.NET_SessionId

# Verify auth — should report cookies present and session live.
gisis-pp-cli doctor --json

# Fetch authoritative ship particulars by IMO number.
gisis-pp-cli ship get 9966233 --json

# Browse your accumulating local cache.
gisis-pp-cli ship list --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`ship batch`** — Resolve a list of IMOs in one invocation, honoring the 1 req/2-3s throttle and persisting each to the local cache.

  _When an agent needs particulars for many vessels, this is the polite + persistent path. Single IMO? Use 'ship get'._

  ```bash
  gisis-pp-cli ship batch --imos 9966233,9123456 --json
  ```
- **`ship list`** — Browse vessels you have already fetched, with filters on flag/owner/type and full-text search on name/owner.

  _When you want to find a vessel you saw before, this beats re-fetching from GISIS._

  ```bash
  gisis-pp-cli ship list --owner "ACME" --type Tanker --json
  ```
- **`ship pin`** — Pin vessels for an active deal or story, then refresh only the pinned ones at a chosen cadence.

  _Use this to keep a working set of vessels current without re-fetching the whole cache._

  ```bash
  gisis-pp-cli ship pin 9966233 --label deal-2026-Q2 && gisis-pp-cli ship refresh --pinned --older-than 30d
  ```
- **`ship stale`** — List cached vessels whose particulars haven't been refreshed in N days.

  _Compliance recency: when you need to know which dossier vessels need re-checking._

  ```bash
  gisis-pp-cli ship stale --older-than 30d --pinned --json
  ```

### Maritime due-diligence signals
- **`ship history`** — Show how a vessel's particulars have changed across snapshots — flag, name, owner, operator, classification society, status.

  _Flag-hopping and ownership changes are the textbook sanctions-bypass tells in maritime DD. Use this when you need temporal context, not a snapshot._

  ```bash
  gisis-pp-cli ship history 9966233 --json
  ```
- **`owner fleet`** — List every cached vessel for a given owner string (the Companies module isn't in v1).

  _Counterparty exposure and related-vessel discovery without hitting the deferred Companies module._

  ```bash
  gisis-pp-cli owner fleet "ACME SHIPPING LTD" --like --json
  ```

### Reachability mitigation
- **`auth ping`** — Single fast GET to /Public/SHIPS/Default.aspx; exits 0 if session is live, non-zero if re-login needed.

  _Long batch jobs and unattended cron tasks need a cheap way to know if the session is still alive._

  ```bash
  gisis-pp-cli auth ping
  ```

## Recipes

### Look up an IMO with structured output

```bash
gisis-pp-cli ship get 9966233 --json --select name,flag,type,gross_tonnage,registered_owner
```

Returns the four high-gravity fields needed for a KYC entry.

### Run a nightly batch over a watchlist

```bash
gisis-pp-cli ship batch --file ~/watchlist.txt --json | jq -c '.'
```

Resolves every IMO in the file under the throttle and streams JSON-lines.

### Spot a flag change on a pinned vessel

```bash
gisis-pp-cli ship refresh --pinned --older-than 7d && gisis-pp-cli ship history 9966233 --json
```

Refresh stale pinned ships, then check the diff on one of them.

### Find all cached ships under one owner

```bash
gisis-pp-cli owner fleet "COSCO SHIPPING" --like --json
```

Synthesizes counterparty exposure from accumulated lookups.

### Keep the session alive from cron

```bash
*/20 * * * * /usr/local/bin/gisis-pp-cli auth ping >/dev/null 2>&1
```

ASP.NET sessions time out at ~30 min idle; ping every 20 to stay warm.

## Usage

Run `gisis-pp-cli --help` for the full command reference and flag list.

## Commands

### ship

Ship particulars — authoritative IMO vessel registry data (name, flag, type, gross tonnage, ownership, classification society).

- **`gisis-pp-cli ship <IMONumber>`** - Get ship particulars by IMO number from the IMO Ship and Company Particulars module.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gisis-pp-cli ship mock-value

# JSON for scripting and agents
gisis-pp-cli ship mock-value --json

# Filter to specific fields
gisis-pp-cli ship mock-value --json --select id,name,status

# Dry run — show the request without sending
gisis-pp-cli ship mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gisis-pp-cli ship mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
gisis-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gisis-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gisis-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports 'session expired' or 'login wall detected'** — Re-run 'press-auth login gisis.imo.org' to capture fresh cookies.
- **ship get returns 'WebLogin redirect' error** — Same as above — session died. Re-run 'press-auth login gisis.imo.org'.
- **press-auth login window times out without capturing** — Add --complete-selector a[href*=Logoff] so press-auth knows when login is done.
- **throttle warnings on batch lookups** — Default throttle is 1 req/2-3s per the API's polite-use convention. Increase --delay if GISIS starts rate-limiting.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**equasis-cli**](https://github.com/rhinonix/equasis-cli) — Python (17 stars)
- [**vesselapi-mcp**](https://github.com/vessel-api/vesselapi-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
