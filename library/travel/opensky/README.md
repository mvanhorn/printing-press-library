# Opensky CLI

Public OpenSky Network API (https://opensky-network.org/api). Live aircraft
state vectors and historical flight data collected from a worldwide network
of ADS-B receivers. No authentication required for the v1 endpoints; the
service applies per-IP rate limits (anonymous access is throttled, roughly
a handful of requests per minute on the time-window endpoints).

Generic CLI: query-based, no account. `opensky states live` shows aircraft
over a bounding box right now; `opensky flights arrivals --airport KJFK`
lists flights arriving at JFK in the default recent window (begin/end
default to now-2h..now via a local patch); `opensky flights aircraft
--icao24 <hex>` replays one aircraft's trajectory.

Coordinates are WGS84 decimal degrees; airports are ICAO codes (KJFK, EGLL,
EDDF); timestamps are UNIX epoch seconds; icao24 is the 6-hex-digit
transponder address.

Created by [@mvanhorn](https://github.com/mvanhorn) (Hunter Veltri).

## Install

The recommended path installs both the `opensky-pp-cli` binary and the `pp-opensky` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install opensky
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install opensky --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install opensky --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install opensky --agent claude-code
npx -y @mvanhorn/printing-press-library install opensky --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/opensky/cmd/opensky-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/opensky-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install opensky --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-opensky --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-opensky --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install opensky --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/opensky-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/opensky/cmd/opensky-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "opensky": {
      "command": "opensky-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
opensky-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
opensky-pp-cli states
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`flights get-departures --airport KJFK`** — begin/end default to the last hour on every /flights/* command (OpenSky requires the window and anonymous access caps it at 1h), so live queries work with zero epoch math. Override with --begin/--end.
- **`flights get-aircraft-trajectory --icao24 <hex>`** — OpenSky answers HTTP 404 with an empty array when a window has no flights; the CLI treats that as an empty result (exit 0) instead of the generic resource-not-found error.
- **`states --lamin 40.5 --lomin -74.5 --lamax 41.5 --lomax -73.5`** — Live ADS-B state vectors over any bounding box with no API key; --icao24 filters to one transponder for a single-aircraft view.

## Usage

Run `opensky-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OPENSKY_CONFIG_DIR`, `OPENSKY_DATA_DIR`, `OPENSKY_STATE_DIR`, or `OPENSKY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OPENSKY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OPENSKY_HOME=/srv/opensky
opensky-pp-cli doctor
```

Under `OPENSKY_HOME=/srv/opensky`, the four dirs resolve to `/srv/opensky/config`, `/srv/opensky/data`, `/srv/opensky/state`, and `/srv/opensky/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "opensky": {
      "command": "opensky-pp-mcp",
      "env": {
        "OPENSKY_HOME": "/srv/opensky"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OPENSKY_DATA_DIR` overrides an explicit `--home` for that kind. Use `OPENSKY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OPENSKY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `opensky-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### flights

Manage flights

- **`opensky-pp-cli flights get-aircraft-trajectory`** - Full trajectory for one aircraft within a time window
- **`opensky-pp-cli flights get-departures`** - Flights departing an airport within a time window

### states

Manage states

- **`opensky-pp-cli states`** - Live aircraft states over a bounding box or for one transponder


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`opensky-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`opensky-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`opensky-pp-cli learnings list`** - Inspect taught rows
- **`opensky-pp-cli learnings forget <query>`** - Undo a teach
- **`opensky-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`opensky-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`opensky-pp-cli teach-pattern`** - Install a query/resource template up front
- **`opensky-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OPENSKY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `opensky-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
opensky-pp-cli states

# JSON for scripting and agents
opensky-pp-cli states --json

# Filter to specific fields
opensky-pp-cli states --json --select id,name,status

# Dry run — show the request without sending
opensky-pp-cli states --dry-run

# Agent mode — JSON + compact + no prompts in one flag
opensky-pp-cli states --agent
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
opensky-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `opensky-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/opensky-network-pp-cli/config.toml`; `--home`, `OPENSKY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
