# Texas Roadhouse - call ahead seating

Waitlist CLI for Texas Roadhouse. Find a nearby store, read the quote, join and leave the list.

Learn more at [Texas Roadhouse](https://www.texasroadhouse.com).

## Install

The recommended path installs both the `texas-roadhouse-pp-cli` binary and the `pp-texas-roadhouse` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse --agent claude-code
npx -y @mvanhorn/printing-press-library install texas-roadhouse --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/texas-roadhouse-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-texas-roadhouse --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-texas-roadhouse --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install texas-roadhouse --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/texas-roadhouse-current).
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
    "texas-roadhouse": {
      "command": "texas-roadhouse-pp-mcp"
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
texas-roadhouse-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
texas-roadhouse-pp-cli mapbox mock-value
```

## Usage

Run `texas-roadhouse-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TEXAS_ROADHOUSE_CONFIG_DIR`, `TEXAS_ROADHOUSE_DATA_DIR`, `TEXAS_ROADHOUSE_STATE_DIR`, or `TEXAS_ROADHOUSE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TEXAS_ROADHOUSE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TEXAS_ROADHOUSE_HOME=/srv/texas-roadhouse
texas-roadhouse-pp-cli doctor
```

Under `TEXAS_ROADHOUSE_HOME=/srv/texas-roadhouse`, the four dirs resolve to `/srv/texas-roadhouse/config`, `/srv/texas-roadhouse/data`, `/srv/texas-roadhouse/state`, and `/srv/texas-roadhouse/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "texas-roadhouse": {
      "command": "texas-roadhouse-pp-mcp",
      "env": {
        "TEXAS_ROADHOUSE_HOME": "/srv/texas-roadhouse"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TEXAS_ROADHOUSE_DATA_DIR` overrides an explicit `--home` for that kind. Use `TEXAS_ROADHOUSE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TEXAS_ROADHOUSE_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `texas-roadhouse-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### mapbox

Operations on mapbox.places

- **`texas-roadhouse-pp-cli mapbox <id>`** - GET /api/mapbox/geocoding/v5/mapbox.places/{id}

### stores

Operations on near

- **`texas-roadhouse-pp-cli stores`** - GET /api/stores/near

### texasroadhouse

Operations on test

- **`texas-roadhouse-pp-cli texasroadhouse cancel`** - Cancel a waitlist request. Live cancel requires `--yes`; `--dry-run` previews without POSTing.
- **`texas-roadhouse-pp-cli texasroadhouse checkin`** - Check in once the party is at the host stand. Live check-in requires `--yes`; `--dry-run` previews without POSTing.
- **`texas-roadhouse-pp-cli texasroadhouse get-quote`** - GET /api/texasroadhouse/waitlist/{waitlist_id}/quote
- **`texas-roadhouse-pp-cli texasroadhouse get-settings`** - GET /api/texasroadhouse/waitlist/{waitlist_id}/settings
- **`texas-roadhouse-pp-cli texasroadhouse get-status`** - GET waitlist request status. Query clientid=texasroadhouse.
- **`texas-roadhouse-pp-cli texasroadhouse get-test`** - GET /api/texasroadhouse/waitlist/{waitlist_id}/test
- **`texas-roadhouse-pp-cli texasroadhouse submit`** - Join a store waitlist. Live join requires `--yes`; `--dry-run` previews without POSTing.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`texas-roadhouse-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`texas-roadhouse-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`texas-roadhouse-pp-cli learnings list`** - Inspect taught rows
- **`texas-roadhouse-pp-cli learnings forget <query>`** - Undo a teach
- **`texas-roadhouse-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`texas-roadhouse-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`texas-roadhouse-pp-cli teach-pattern`** - Install a query/resource template up front
- **`texas-roadhouse-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TEXAS_ROADHOUSE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `texas-roadhouse-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
texas-roadhouse-pp-cli mapbox mock-value

# JSON for scripting and agents
texas-roadhouse-pp-cli mapbox mock-value --json
# Filter to specific fields
texas-roadhouse-pp-cli mapbox mock-value --json --select bbox,center,context

# Dry run — show the request without sending
texas-roadhouse-pp-cli mapbox mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
texas-roadhouse-pp-cli mapbox mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
texas-roadhouse-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `texas-roadhouse-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/texas-roadhouse-pp-cli/config.toml`; `--home`, `TEXAS_ROADHOUSE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `TEXAS_ROADHOUSE_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.texasroadhouse.com/api/mapbox/geocoding/v5/mapbox.places/65804.json
- Capture coverage: 7 API entries from 7 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: get_mapbox.places — Derived from observed GET /api/mapbox/geocoding/v5/mapbox.places/{id} traffic.; get_quote — Derived from observed GET /api/texasroadhouse/waitlist/{waitlist_id}/quote traffic.; get_settings — Derived from observed GET /api/texasroadhouse/waitlist/{waitlist_id}/settings traffic.; get_test — Derived from observed GET /api/texasroadhouse/waitlist/{waitlist_id}/test traffic.; list_near — Derived from observed GET /api/stores/near traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
