# Great Clips CLI

First CLI for Great Clips Online Check-In. Drives the Great Clips customer
webservices (webservices.greatclips.com) and the ICS Net Check-In wait /
check-in service (www.stylewaretouch.net) from one Go binary.

## v0.2 status (2026-05-11)

**Live and working with a captured Auth0 JWT:**
- `salons search --term <zip>` — finds salons by zip/city
- `salons get --num <id>` — full salon detail
- `hours --salon <id>` — 14-day hours forecast
- `geo --query <zip>` — resolve zip to lat/lng
- All dry-run output, JSON serialization, MCP server, doctor, sync, sql, search

**Signing implementation verified byte-identical to the SPA's algorithm.**
Every outbound request to `www.stylewaretouch.net` is automatically signed
with the HMAC `?t=<timestamp>&s=<signature>` query parameters that the ICS
Net Check-In service requires. Verified by closed-loop golden vector test
in `internal/icssign/sign_test.go`.

**Not yet working live, despite correct signing:**
- `wait`, `checkin`, `status`, `cancel` (all `www.stylewaretouch.net` endpoints)
- `customer profile` (`/cmp2/profile/get` on webservices)

Both are blocked on per-host JWT audience scope: the GreatClips SPA fetches
separate Auth0 access tokens for `webservices.greatclips.com/customer`,
`webservices.greatclips.com/cmp2`, and `www.stylewaretouch.net`. v0.2 captures
one of these and routes it to every host; the wrong-audience tokens are
rejected at the gateway. Closing this gap (v0.3) requires either:
1. Replicating the SPA's `getAccessTokenSilently({audience})` flow against
   `cid.greatclips.com/authorize` + `/oauth/token` using Chrome's HttpOnly
   `cid.greatclips.com/auth0` cookie, or
2. A `auth set-token --audience <name>` paste flow that holds three tokens.

The signing port is the load-bearing v0.2 work; once audience scoping is
fixed in v0.3, every stylewaretouch endpoint should light up immediately.

Learn more at [Great Clips](https://webservices.greatclips.com).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `greatclips-pp-cli` binary and the `pp-greatclips` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install greatclips
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install greatclips --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install greatclips --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install greatclips --agent claude-code
npx -y @mvanhorn/printing-press-library install greatclips --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/greatclips/cmd/greatclips-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/greatclips-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-greatclips --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-greatclips --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install greatclips --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/greatclips-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GREATCLIPS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "greatclips": {
      "command": "greatclips-pp-mcp",
      "env": {
        "GREATCLIPS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Great Clips uses HttpOnly session cookies from an Auth0 tenant at cid.greatclips.com. Browser-sniff confirmed all calls succeed via cookies attached by the browser, not via an Authorization header. v0.1 of this CLI ships the request shape and host routing; to make real calls, paste the cookies from a logged-in Chrome session into ~/.config/greatclips-pp-cli/config.toml. Set GREATCLIPS_TOKEN to any placeholder so doctor passes (the env var is kept for v0.2 when bearer support might land).

## Quick Start

```bash
# Verifies config and base URL reachability (set GREATCLIPS_TOKEN=placeholder first so the env-var check passes; real auth is cookie-based and documented in README)
greatclips-pp-cli doctor

# Confirms profile call shape (GET /cmp2/profile/get)
greatclips-pp-cli customer profile --dry-run

# Wait-time request for one salon (Island Square on Mercer Island)
greatclips-pp-cli wait --store-number 8991 --dry-run --json

# Party-of-four check-in request body; remove --dry-run when cookies are configured
greatclips-pp-cli checkin --first-name Matt --last-name VanHorn --phone-number '(206) 555-0100' --salon-number 8991 --guests 4 --dry-run --json

# Position-in-line endpoint shape
greatclips-pp-cli status --dry-run

```

## Usage

Run `greatclips-pp-cli --help` for the full command reference and flag list.

## Commands

### cancel

Cancel your active check-in

- **`greatclips-pp-cli cancel submit`** - Cancel the active check-in for this account

### checkin

Submit a check-in for yourself plus a party (1-5 people)

- **`greatclips-pp-cli checkin submit`** - Add yourself and optionally other party members to a salon waitlist

### customer

Read your Great Clips customer profile (name, phone, favorites, recent visits)

- **`greatclips-pp-cli customer profile`** - Get the authenticated customer's profile

### geo

Resolve a zip code or city term to latitude/longitude

- **`greatclips-pp-cli geo postal_code`** - Resolve a zip/postal code to lat/lng/city/state

### hours

Read salon hours (today plus 14-day forecast with special hours)

- **`greatclips-pp-cli hours upcoming`** - Get 14-day hours forecast for one salon

### salons

Search and look up Great Clips salons

- **`greatclips-pp-cli salons get`** - Get a single salon by its salon number
- **`greatclips-pp-cli salons search`** - Search salons by zip code, city, or coordinates within a radius

### status

Check your current position in line for an active check-in

- **`greatclips-pp-cli status get`** - Get your active check-in status (position in line, estimated wait)

### wait

Read estimated wait times from the ICS Net Check-In service

- **`greatclips-pp-cli wait one`** - Get wait time for one salon (body is a single-element array of {storeNumber})

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
greatclips-pp-cli salons get --num example-value

# JSON for scripting and agents
greatclips-pp-cli salons get --num example-value --json

# Filter to specific fields
greatclips-pp-cli salons get --num example-value --json --select id,name,status

# Dry run — show the request without sending
greatclips-pp-cli salons get --num example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
greatclips-pp-cli salons get --num example-value --agent
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
greatclips-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/greatclips-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GREATCLIPS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `greatclips-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GREATCLIPS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports 'missing required: GREATCLIPS_TOKEN'** — Set GREATCLIPS_TOKEN to any non-empty value (the upstream uses cookies, not a Bearer; v0.1 keeps the env var so doctor passes).
- **checkin returns 401 or 403 against the real API** — v0.1 does not yet attach Chrome cookies. Open https://app.greatclips.com, log in, copy the auth0 session cookies for both webservices.greatclips.com and www.stylewaretouch.net, paste into ~/.config/greatclips-pp-cli/config.toml under cookie keys (cookie auth integration is a v0.2 gap).
- **wait returns object body but the API wants an array** — The waitTime endpoint expects a JSON array of {storeNumber}. v0.1 emits the object form; patch internal/client.go to wrap the body in [] or use --stdin and pipe the array shape.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://app.greatclips.com/
- Capture coverage: 9 API entries from 22 total network entries
- Reachability: standard_http (90% confidence)
- Protocols: (0% confidence), (0% confidence), (0% confidence)
- Generation hints: Two-host single-auth: emit one bearer-token client that handles both webservices.greatclips.com and www.stylewaretouch.net., POST /api/store/waitTime takes a JSON array body of {storeNumber} objects, not a single object. Single-salon call sends an array of length 1., Auth captured live via Claude-in-Chrome MCP from a logged-in Chrome session; the printed CLI should ship a `auth login --chrome` helper command for the equivalent flow.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
