# Alaska Airlines CLI

**Search Alaska Airlines flights and check Atmos Rewards balance from the terminal, with offline-cached airports and agent-native JSON output.**

Single static binary that talks to alaskaair.com's SvelteKit endpoints over a Chrome-fingerprinted HTTP transport. Offline SQLite cache for the airport catalog. Cookie-import auth via your logged-in Chrome session for endpoints that need a guestsession JWT. This CLI is read-only; the final pay POST is not replayable from a static binary and is not attempted.

Learn more at [Alaska Airlines](https://www.alaskaair.com).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `alaska-airlines-pp-cli` binary and the `pp-alaska-airlines` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines --agent claude-code
npx -y @mvanhorn/printing-press-library install alaska-airlines --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/alaska-airlines/cmd/alaska-airlines-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alaska-airlines-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-alaska-airlines --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-alaska-airlines --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install alaska-airlines --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
alaska-airlines-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alaska-airlines-current).
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
    "alaska-airlines": {
      "command": "alaska-airlines-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Run `auth login --chrome` once. It extracts Alaska's cookies from your logged-in Chrome profile (AS_ACNT, AS_NAME, guestsession, ASSession, etc.) via your macOS keychain. Future commands replay them via Surf transport with a Chrome TLS fingerprint. Run `doctor` to verify the session is valid.

## Quick Start

```bash
# import your Alaska cookies from Chrome - one-time setup
alaska-airlines-pp-cli auth login --chrome

# verify auth and API reachability
alaska-airlines-pp-cli doctor

# populate the local airport store
alaska-airlines-pp-cli airports sync

# search for a family of 6, round trip
alaska-airlines-pp-cli flights search --origin SFO --destination SEA --depart 2026-11-27 --return 2026-11-30 --adults 2 --children 4 --json

# check current Atmos Rewards points balance
alaska-airlines-pp-cli atmos-rewards balance --json

```

## Known Gaps

These capabilities were scoped in the manuscript for this CLI but were not implemented in this generation. They are not currently available; planning around them will fail with "unknown command" or "unknown flag" errors:

- **Pre-checkout deeplink booking** (`book prepare`). The final pay POST is CSRF-tokened and unreplayable from a static Go binary, but the deeplink to the pre-filled cart was planned. Not implemented; the CLI ships read-only.
- **Family-of-N seat finder** (`flights search --want-seats-together`). Filter for flights with N contiguous economy seats. Not implemented.
- **Fare drift watch** (`flights watch`). Save a search, poll on an interval, notify on a price drop. Not implemented.
- **Award-vs-cash cost-per-mile** (`flights compare --points`). Cents-per-Atmos-point comparison across fare classes. Not implemented.
- **Atmos status progress** (`atmos status`). Tier progress and miles-to-next-tier joined view. Use `atmos-rewards balance` for the raw points balance instead.
- **Multi-city smart search** (`flights search multi`). Multi-leg composition. Not implemented; run separate `flights search` invocations per leg.
- **Cookie expiry pre-flight check** (`doctor --auth`). Dedicated JWT decode flag. Not implemented; `doctor` performs the general auth/reachability check.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`flights search --select`** — Generic --select flag narrows the ~50KB search/results JSON to just the fields the agent needs.

  _Lets the agent ask for exactly what it needs and skip the chrome._

  ```bash
  alaska-airlines-pp-cli flights search --origin SFO --destination SEA --depart 2026-11-27 --json --select flights.flightNumber,flights.fares.saver.price,flights.duration
  ```

## Usage

Run `alaska-airlines-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Login status, session tokens

- **`alaska-airlines-pp-cli account login-status`** - Check if current cookie session is authenticated
- **`alaska-airlines-pp-cli account token`** - Refresh primary session JWT

### airports

Airport lookup, codeshare partner info, and full catalog

- **`alaska-airlines-pp-cli airports get`** - Get airport details by IATA code, including codeshare carrier coverage
- **`alaska-airlines-pp-cli airports list`** - Full Alaska Airlines + codeshare airport list (IATA + city + region + lat/lon)

### atmos-rewards

Atmos Rewards loyalty program account data

- **`alaska-airlines-pp-cli atmos-rewards balance`** - Current Atmos Rewards points balance
- **`alaska-airlines-pp-cli atmos-rewards token-refresh`** - Refresh Atmos Rewards token via cookie session

### cart

Cart state for a constructed itinerary deeplink (read-only inspection)

- **`alaska-airlines-pp-cli cart`** - Inspect cart state for a constructed itinerary deeplink (requires `--leg1` and `--adults` to identify the cart)

### flights

Search Alaska Airlines flights with pricing, fare classes, and flexible-date matrices

- **`alaska-airlines-pp-cli flights business`** - Alaska for Business program metadata
- **`alaska-airlines-pp-cli flights et-info`** - Electronic ticket info / general metadata
- **`alaska-airlines-pp-cli flights get-features`** - Feature flags scoped to a user (used internally by site)
- **`alaska-airlines-pp-cli flights search`** - Search flights between two airports for given dates and passenger mix. Returns the fare matrix per flight per cabin class (Saver / Main / Premium / First).
- **`alaska-airlines-pp-cli flights shoulder-dates`** - Flexible-date pricing matrix - get fares for dates near your target

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
alaska-airlines-pp-cli airports list

# JSON for scripting and agents
alaska-airlines-pp-cli airports list --json

# Filter to specific fields
alaska-airlines-pp-cli airports list --json --select id,name,status

# Dry run — show the request without sending
alaska-airlines-pp-cli airports list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
alaska-airlines-pp-cli airports list --agent
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
alaska-airlines-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `alaska-airlines-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login fails with keychain access denied** — macOS asked Chrome for 'Chrome Safe Storage' access; click Always Allow in the keychain prompt and retry
- **search returns empty results but the website works** — your guestsession JWT may be expired; run `auth login --chrome` to re-import cookies
- **atmos-rewards balance returns 401** — the Atmos Rewards token cookie expired; run `auth login --chrome` to re-import cookies
- **Surf transport throws TLS errors** — rebuild the binary or set ALASKA_DISABLE_SURF=1 to fall back to stdlib http (lower-fidelity fingerprint, may be flagged)

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**awardwiz**](https://github.com/lg/awardwiz) — TypeScript (350 stars)
- [**flightplan**](https://github.com/flightplan-tool/flightplan) — JavaScript (130 stars)
- [**flight-award-scraper**](https://github.com/igolaizola/flight-award-scraper) — Go (18 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
