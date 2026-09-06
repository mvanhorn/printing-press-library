# Amc Theatres CLI

**Official AMC showtimes and movie-night planning from an agent-ready CLI.**

Query every documented AMC Showtime API v2 read operation or get a ranked movie plan by theatre or location, with explicit production and sandbox support.

Created by [@Avanderheyde](https://github.com/Avanderheyde) (Alderik).

## Install

The recommended path installs both the `amc-theatres-pp-cli` binary and the `pp-amc-theatres` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres --agent claude-code
npx -y @mvanhorn/printing-press-library install amc-theatres --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/cmd/amc-theatres-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amc-theatres-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-amc-theatres --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-amc-theatres --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install amc-theatres --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amc-theatres-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AMC_THEATRES_VENDOR_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/cmd/amc-theatres-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "amc-theatres": {
      "command": "amc-theatres-pp-mcp",
      "env": {
        "AMC_THEATRES_VENDOR_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Create an AMC developer application, set its vendor key, optionally set a user authorization token, and choose production or sandbox.

## Quick Start

```bash
# Rank evening showtimes at one AMC theatre.
amc-theatres-pp-cli movie-plan "Dune" --theatre 1234 --date 2026-07-24 --after 18:00 --agent

# Find nearby IMAX screenings and rank them by distance and start time.
amc-theatres-pp-cli movie-plan "Dune" --latitude 40.7128 --longitude -74.0060 --format IMAX --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Movie planning
- **`movie-plan`** — Find, filter, and rank AMC showtimes by theatre or current location without purchasing tickets.

  _Agents can return a short ranked set of relevant screenings without inventing purchase capabilities._

  ```bash
  amc-theatres-pp-cli movie-plan "Dune" --latitude 40.7128 --longitude -74.0060 --after 18:00 --format IMAX --agent
  ```

## Recipes

### Plan a movie at one theatre

```bash
amc-theatres-pp-cli movie-plan "Dune" --theatre 1234 --date 2026-07-24 --after 18:00 --agent
```

Queries dated theatre showtimes, filters the movie and earliest time, and ranks matching screenings.

### Find nearby premium-format showtimes

```bash
amc-theatres-pp-cli movie-plan "Dune" --latitude 40.7128 --longitude -74.0060 --format IMAX --agent
```

Queries AMC's current-location view and ranks matching premium-format screenings by distance and time.

## Usage

Run `amc-theatres-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AMC_THEATRES_CONFIG_DIR`, `AMC_THEATRES_DATA_DIR`, `AMC_THEATRES_STATE_DIR`, or `AMC_THEATRES_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AMC_THEATRES_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AMC_THEATRES_HOME=/srv/amc-theatres
amc-theatres-pp-cli doctor
```

Under `AMC_THEATRES_HOME=/srv/amc-theatres`, the four dirs resolve to `/srv/amc-theatres/config`, `/srv/amc-theatres/data`, `/srv/amc-theatres/state`, and `/srv/amc-theatres/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "amc-theatres": {
      "command": "amc-theatres-pp-mcp",
      "env": {
        "AMC_THEATRES_HOME": "/srv/amc-theatres"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AMC_THEATRES_DATA_DIR` overrides an explicit `--home` for that kind. Use `AMC_THEATRES_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AMC_THEATRES_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `amc-theatres-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### movie-plan

Find, filter, and rank AMC showtimes without purchasing tickets. Choose exactly one location mode: `--theatre <number>`, or both `--latitude` and `--longitude`. Optional `--date`, `--after`, and `--format` filters narrow the results.

```bash
amc-theatres-pp-cli movie-plan "Dune" --theatre 1234 --date 2026-07-24 --after 18:00
amc-theatres-pp-cli movie-plan "Dune" --latitude 40.7128 --longitude -74.0060 --format IMAX --agent
```

This workflow is deliberately read-only: it does not buy tickets, choose seats, or submit payment.

### showtimes

Manage showtimes

- **`amc-theatres-pp-cli showtimes get-by-id`** - Retrieves showtime details for the provided ID.
- **`amc-theatres-pp-cli showtimes search-by-lat-long`** - Retrieves a paged list of showtimes for a date and coordinates.

### theatres

Manage theatres

- **`amc-theatres-pp-cli theatres movies get-earliest-showtime-by-theatre`** - Retrieves the earliest showtime for a movie at an AMC theatre.
- **`amc-theatres-pp-cli theatres showtimes get-by-theatre`** - Retrieves paged showtimes for an AMC theatre.
- **`amc-theatres-pp-cli theatres showtimes search`** - Retrieves paged showtimes for an AMC theatre on a specified date.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`amc-theatres-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`amc-theatres-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`amc-theatres-pp-cli learnings list`** - Inspect taught rows
- **`amc-theatres-pp-cli learnings forget <query>`** - Undo a teach
- **`amc-theatres-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`amc-theatres-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`amc-theatres-pp-cli teach-pattern`** - Install a query/resource template up front
- **`amc-theatres-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AMC_THEATRES_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `amc-theatres-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
amc-theatres-pp-cli showtimes get-by-id mock-value

# JSON for scripting and agents
amc-theatres-pp-cli showtimes get-by-id mock-value --json

# Filter to specific fields
amc-theatres-pp-cli showtimes get-by-id mock-value --json --select id,name,status

# Dry run — show the request without sending
amc-theatres-pp-cli showtimes get-by-id mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
amc-theatres-pp-cli showtimes get-by-id mock-value --agent
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
amc-theatres-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `amc-theatres-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/amc-theatres-showtime-pp-cli/config.toml`; `--home`, `AMC_THEATRES_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AMC_THEATRES_VENDOR_KEY` | per_call | Yes | Set to your API credential. |
| `AMC_THEATRES_AUTH_TOKEN` | per_call | No | Optional user-scoped authorization token sent as `X-AMC-Auth-Token`. |
| `AMC_THEATRES_ENV` | config | No | `production` (default) or `sandbox`; switches only the official AMC host. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `amc-theatres-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `amc-theatres-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AMC_THEATRES_VENDOR_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **The API reports that the vendor key is missing or invalid.** — Run auth setup and export AMC_THEATRES_VENDOR_KEY from the official AMC developer portal.
- **A sandbox credential is being sent to production.** — Set AMC_THEATRES_ENV=sandbox; explicit custom base URLs remain unchanged for tests and controlled deployments.
