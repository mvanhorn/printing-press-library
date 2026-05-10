# UK Train Goat CLI

**The only Go-native, agent-native, MCP-exposed UK rail CLI; live boards, journey planning, and a local SQLite store that remembers your commute.**

uk-train-goat wraps the free National Rail OpenLDBWS API with a Cobra command tree, an MCP server, and an offline station database. Live departures, arrivals, journey planning A->B, and service status all run from one terminal command and ship with a programmatic eval grader that pins tool descriptions for LLM agents.

Learn more at [UK Train Goat](https://www.nationalrail.co.uk/).

Printed by [@ahujasachin92](https://github.com/ahujasachin92) (Sachin Ahuja).

## Install

The recommended path installs both the `uk-train-goat-pp-cli` binary and the `pp-uk-train-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install uk-train-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install uk-train-goat --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/cmd/uk-train-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uk-train-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-uk-train-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-uk-train-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-uk-train-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-uk-train-goat. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Set your free OpenLDBWS access token; obtain one at realtime.nationalrail.co.uk/OpenLDBWSRegistration
uk-train-goat-pp-cli auth login


# Populate the local SQLite station list once; powers offline CRS lookup
uk-train-goat-pp-cli sync


# Resolve a station name to its CRS code with no network call
uk-train-goat-pp-cli stations --search paddington


# Show the next 5 departures from Paddington
uk-train-goat-pp-cli board PAD --num 5


# Plan a journey A->B and pipe to jq or an agent
uk-train-goat-pp-cli journey RDG PAD --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`go`** — Run a saved commute in one keystroke; resolves origin, destination, and time-window from the local store.

  _Pick this when an agent needs the user's named recurring route resolved from local state, not a fresh A->B planning call._

  ```bash
  uk-train-goat-pp-cli go morning --json --select services.std,services.platform,services.etd
  ```
- **`saved status`** — Fans out across every saved route and prints a single ranked status table for all your commutes.

  _Pick this on disruption mornings when a user has multiple saved commutes and wants one answer, not N separate board calls._

  ```bash
  uk-train-goat-pp-cli saved status --json --select routes.name,routes.next.std,routes.next.etd
  ```
- **`recent`** — Re-runs your recent A->B searches with fresh live data, side-by-side, so iterative trip planning is one keystroke per refresh.

  _Pick this when the user is iterating dates or origins for one trip; every previous query stays one keystroke away._

  ```bash
  uk-train-goat-pp-cli recent --json --select queries.from,queries.to,queries.next.std
  ```

### Agent-native plumbing
- **`stations`** — FTS5 search over the local UK CRS code table; resolves Paddington -> PAD with no network call.

  _Pick this whenever the user names a station in prose; resolve to CRS once, then call any other uk-train-goat command._

  ```bash
  uk-train-goat-pp-cli stations --search kings --json --select results.crs,results.name
  ```
- **`board`** — Accept a comma-separated list of CRS codes and merge departures across all of them into one ranked time-ordered list.

  _Pick this when an agent needs the next London-bound (or other regional cluster) departure across multiple terminals at once._

  ```bash
  uk-train-goat-pp-cli board PAD,KGX,EUS --in 30m --json --select services.origin,services.std,services.destination
  ```
- **`eval`** — Programmatic eval grader that scores an LLM agent against a fixture suite of natural-language UK-rail prompts; 80% pass-rate threshold blocks ship.

  _Run this in CI on changes to internal/cli or internal/mcp; failing evals mean an agent will pick the wrong tool or wrong args._

  ```bash
  EVAL_AGENT_MODEL=claude-sonnet-4-6 uk-train-goat-pp-cli eval --json
  ```

### Service-specific content
- **`why`** — Combines GetServiceDetails delay/cancel reasons with adjacent operator alerts into one screen explaining what is going wrong with one service.

  _Pick this when the user wants to understand a delay, not just see it._

  ```bash
  uk-train-goat-pp-cli why L8rW0bMonHt3K4IengVPQw== --json
  ```
- **`journey`** — Ranks A->B options by combined scheduled-time, current delay, and platform-known signal so on-time-but-later beats earlier-but-late.

  _Pick this for trip planning during disruption; choosing the earliest scheduled departure is often wrong when delays exist._

  ```bash
  uk-train-goat-pp-cli journey RDG PAD --rank --json --select journeys.std,journeys.delay,journeys.platform
  ```

## Usage

Run `uk-train-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### status

Internal placeholder resource. Real commands are hand-authored against the OpenLDBWS wrapper.

- **`uk-train-goat-pp-cli status ping`** - Placeholder; deleted post-generate. Real CLI commands live in internal/cli/board.go etc.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
uk-train-goat-pp-cli status

# JSON for scripting and agents
uk-train-goat-pp-cli status --json

# Filter to specific fields
uk-train-goat-pp-cli status --json --select id,name,status

# Dry run — show the request without sending
uk-train-goat-pp-cli status --dry-run

# Agent mode — JSON + compact + no prompts in one flag
uk-train-goat-pp-cli status --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-uk-train-goat -g
```

Then invoke `/pp-uk-train-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/cmd/uk-train-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add uk-train-goat uk-train-goat-pp-mcp -e LDBWS_API_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uk-train-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LDBWS_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/cmd/uk-train-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "uk-train-goat": {
      "command": "uk-train-goat-pp-mcp",
      "env": {
        "LDBWS_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
uk-train-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/uk-train-goat-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LDBWS_API_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `uk-train-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LDBWS_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized when calling board / journey** — Re-run `uk-train-goat-pp-cli auth login` with a token from realtime.nationalrail.co.uk/OpenLDBWSRegistration. The CLI uses LDBWS_API_TOKEN, not the wrapper's NR_ACCESS_TOKEN.
- **stations --search returns nothing** — Run `uk-train-goat-pp-cli sync` to populate the local station table; first install ships an empty store.
- **fare command returns experimental warning** — fare scrapes nationalrail.co.uk and is marked experimental; if the upstream layout drifted, file an issue. Do not depend on fare data for booking.
- **doctor reports OpenLDBWS unreachable** — Check connectivity to lite.realtime.nationalrail.co.uk; the API runs there over HTTPS.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**caminad/ldb-cli**](https://github.com/caminad/ldb-cli) — JavaScript
- [**lucygoodchild/mcp-national-rail**](https://github.com/lucygoodchild/mcp-national-rail) — JavaScript
- [**mattsalt/national-rail-darwin**](https://github.com/mattsalt/national-rail-darwin) — JavaScript
- [**ChrisThoung/national-rail**](https://github.com/ChrisThoung/national-rail) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
