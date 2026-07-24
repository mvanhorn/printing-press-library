# Movieglu CLI

**Cross-chain movie showtimes and safe cinema booking handoffs from one agent-ready CLI.**

Resolve a film by name, rank nearby showtimes, and continue to the cinema website without manually joining film IDs, cinema IDs, dates, and dynamic format objects.

## Install

The recommended path installs both the `movieglu-pp-cli` binary and the `pp-movieglu` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install movieglu
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install movieglu --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install movieglu --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install movieglu --agent claude-code
npx -y @mvanhorn/printing-press-library install movieglu --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/cmd/movieglu-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/movieglu-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install movieglu --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-movieglu --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-movieglu --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install movieglu --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/movieglu-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MOVIEGLU_CREDENTIALS` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/cmd/movieglu-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "movieglu": {
      "command": "movieglu-pp-mcp",
      "env": {
        "MOVIEGLU_CREDENTIALS": "<x-api-key>",
        "MOVIEGLU_CLIENT": "<client username>",
        "MOVIEGLU_AUTHORIZATION": "<authorization value>",
        "MOVIEGLU_TERRITORY": "US",
        "MOVIEGLU_GEOLOCATION": "40.7128;-74.0060"
      }
    }
  }
}
```

</details>

## Authentication

Request MovieGlu evaluation credentials, then configure the API key, client username, authorization value, licensed territory, and optional geolocation.

## Quick Start

```bash
# Rank nearby evening showtimes for a film.
movieglu-pp-cli movie-night "Superman" --date 2026-07-24 --after 19:00 --agent

# Return the cinema checkout handoff without launching it.
movieglu-pp-cli movie-night "Superman" --date 2026-07-24 --after 19:00 --booking-link --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Movie planning
- **`movie-night`** — Resolve a film name, rank nearby showtimes, and optionally return the cinema booking handoff.

  _Agents can answer the user’s actual movie-night question without manually joining IDs and showtime objects._

  ```bash
  movieglu-pp-cli movie-night "Superman" --after 19:00 --booking-link --agent
  ```

## Recipes

### Plan an evening movie

```bash
movieglu-pp-cli movie-night "Superman" --date 2026-07-24 --after 19:00 --agent
```

Resolves the film and ranks nearby evening screenings.

### Continue to cinema checkout

```bash
movieglu-pp-cli movie-night "Superman" --date 2026-07-24 --after 19:00 --booking-link --agent
```

Returns an HTTPS cinema booking URL but leaves seat selection and payment to the user.

## Usage

Run `movieglu-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MOVIEGLU_CONFIG_DIR`, `MOVIEGLU_DATA_DIR`, `MOVIEGLU_STATE_DIR`, or `MOVIEGLU_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MOVIEGLU_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MOVIEGLU_HOME=/srv/movieglu
movieglu-pp-cli doctor
```

Under `MOVIEGLU_HOME=/srv/movieglu`, the four dirs resolve to `/srv/movieglu/config`, `/srv/movieglu/data`, `/srv/movieglu/state`, and `/srv/movieglu/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "movieglu": {
      "command": "movieglu-pp-mcp",
      "env": {
        "MOVIEGLU_HOME": "/srv/movieglu"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MOVIEGLU_DATA_DIR` overrides an explicit `--home` for that kind. Use `MOVIEGLU_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MOVIEGLU_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `movieglu-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### movie-night

Resolve a film name against films now showing, find nearby showtimes, filter by date/time, and rank results by cinema distance then start time.

- **`movieglu-pp-cli movie-night <film name>`** - Rank nearby showtimes
- **`--booking-link`** - Fetch the cinema booking handoff for the first option
- **`--launch`** - Explicitly open that HTTPS URL; never launches by default

### cinema-show-times

Manage cinema show times

- **`movieglu-pp-cli cinema-show-times`** - List a cinema's showtimes for a date

### cinemas-nearby

Manage cinemas nearby

- **`movieglu-pp-cli cinemas-nearby`** - List cinemas nearest a location

### closest-showing

Manage closest showing

- **`movieglu-pp-cli closest-showing`** - Find the nearest cinemas showing a film

### film-show-times

Manage film show times

- **`movieglu-pp-cli film-show-times`** - List nearby showtimes for a selected film

### films-now-showing

Manage films now showing

- **`movieglu-pp-cli films-now-showing`** - List the top films now showing

### purchase-confirmation

Manage purchase confirmation

- **`movieglu-pp-cli purchase-confirmation`** - Returns a cinema website URL, often with film, date, and time preselected. This API does not select seats or process payment.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`movieglu-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`movieglu-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`movieglu-pp-cli learnings list`** - Inspect taught rows
- **`movieglu-pp-cli learnings forget <query>`** - Undo a teach
- **`movieglu-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`movieglu-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`movieglu-pp-cli teach-pattern`** - Install a query/resource template up front
- **`movieglu-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MOVIEGLU_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `movieglu-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
movieglu-pp-cli cinema-show-times --cinema-id 550e8400-e29b-41d4-a716-446655440000 --date 2026-01-15

# JSON for scripting and agents
movieglu-pp-cli cinema-show-times --cinema-id 550e8400-e29b-41d4-a716-446655440000 --date 2026-01-15 --json

# Filter to specific fields
movieglu-pp-cli cinema-show-times --cinema-id 550e8400-e29b-41d4-a716-446655440000 --date 2026-01-15 --json --select id,name,status

# Dry run — show the request without sending
movieglu-pp-cli cinema-show-times --cinema-id 550e8400-e29b-41d4-a716-446655440000 --date 2026-01-15 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
movieglu-pp-cli cinema-show-times --cinema-id 550e8400-e29b-41d4-a716-446655440000 --date 2026-01-15 --agent
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
movieglu-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `movieglu-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/movieglu-pp-cli/config.toml`; `--home`, `MOVIEGLU_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MOVIEGLU_CREDENTIALS` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `movieglu-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `movieglu-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MOVIEGLU_CREDENTIALS`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **The CLI reports a missing MovieGlu environment variable.** — Run auth setup and configure every value MovieGlu supplied for the licensed territory.
