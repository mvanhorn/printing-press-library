# June Oven CLI

**Control a June oven from the terminal, plus a durable local cook history and repeatable named cooks that June's live-only cloud never keeps.**

Direct pair/preheat/watch/cancel control with no HomeKit and no June app, plus local history (record, log, curve, preheat-stats), workflow primitives (ready, eta), and named repeatable cooks (repeat). All agent-native JSON.

## Install

The recommended path installs both the `juneoven-pp-cli` binary and the `pp-juneoven` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install juneoven
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install juneoven --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install juneoven --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install juneoven --agent claude-code
npx -y @mvanhorn/printing-press-library install juneoven --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/juneoven/cmd/juneoven-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/juneoven-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install juneoven --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-juneoven --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-juneoven --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install juneoven --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/juneoven-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JUNE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/juneoven/cmd/juneoven-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "juneoven": {
      "command": "juneoven-pp-mcp",
      "env": {
        "JUNE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth is direct pairing: run `juneoven-pp-cli pair` once and type an 8-digit code on the oven. A signing key and access token are stored at ~/.config/juneoven-pp-cli/identity.json (0600).

## Quick Start

```bash
# confirm the oven is paired and online
juneoven-pp-cli status --json

# start a bake
juneoven-pp-cli preheat --temp 350

# watch live telemetry and camera frames
juneoven-pp-cli watch --seconds 60

# review recent cook history
juneoven-pp-cli log --since 7d

```

## Recipes

### Preheat and record

```bash
juneoven-pp-cli preheat --temp 450 && juneoven-pp-cli record --label pizza
```

Start a cook and capture its session to local history.

### Weekly cook by name

```bash
juneoven-pp-cli repeat sunday-roast
```

Replay a saved mode+temp+timer preset in one command.

### Export a cook's curve

```bash
juneoven-pp-cli curve 3 --format csv --agent --select ts,cavity_f
```

Pull one session's temperature series as CSV for plotting.

## Usage

Run `juneoven-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JUNEOVEN_CONFIG_DIR`, `JUNEOVEN_DATA_DIR`, `JUNEOVEN_STATE_DIR`, or `JUNEOVEN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JUNEOVEN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JUNEOVEN_HOME=/srv/juneoven
juneoven-pp-cli doctor
```

Under `JUNEOVEN_HOME=/srv/juneoven`, the four dirs resolve to `/srv/juneoven/config`, `/srv/juneoven/data`, `/srv/juneoven/state`, and `/srv/juneoven/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "juneoven": {
      "command": "juneoven-pp-mcp",
      "env": {
        "JUNEOVEN_HOME": "/srv/juneoven"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JUNEOVEN_DATA_DIR` overrides an explicit `--home` for that kind. Use `JUNEOVEN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JUNEOVEN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `juneoven-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

Temperatures are °F by default; add `--celsius` where a temperature is taken. Command results report the oven's own ack as `{"action","acked","status"}` where `status` is `success` or `not-allowed`.

- **`juneoven-pp-cli pair`** - Pair directly with the oven via an 8-digit code (run once).
- **`juneoven-pp-cli status`** - Connection state, idle/active, current target temperature.
- **`juneoven-pp-cli preheat --temp 350`** - Start a cook (`--mode bake` or `roast`).
- **`juneoven-pp-cli temp --temp 375`** - Change the active cook's target.
- **`juneoven-pp-cli timer --minutes 10`** - Set a cook timer.
- **`juneoven-pp-cli cancel`** - Stop the active cook.
- **`juneoven-pp-cli watch --seconds 120`** - Stream telemetry and camera-frame events as JSON lines.
- **`juneoven-pp-cli cam --timeout 15`** - Print the next interior camera frame's signed URL.

The generated `devices` command (`register`, `associated`) exposes the raw June REST endpoints and is used internally by `pair`; you normally won't call it directly.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cook workflow primitives
- **`ready`** — Wait for the oven to reach its target temperature, then exit 0 (non-zero on timeout).

  _Lets an agent synchronously gate the next cook step on preheat completion instead of busy-waiting._

  ```bash
  juneoven-pp-cli ready --timeout 20 --agent
  ```
- **`eta`** — Non-blocking estimate of time until the oven reaches target, from the live climb rate.

  _An agent can sequence the next action off a returned ETA without blocking._

  ```bash
  juneoven-pp-cli eta --agent
  ```
- **`repeat`** — Save a named mode+temp(+timer) preset in local SQLite and replay it with one word.

  _Collapses a repeated weekly cook into a single memorable command._

  ```bash
  juneoven-pp-cli repeat <name>
  ```

### Local history the cloud never keeps
- **`record`** — Capture the current cook (session + telemetry/camera activity) into local SQLite.

  _Turns ephemeral live cooks into a queryable local dataset the cloud never stores._

  ```bash
  juneoven-pp-cli record --label sourdough --agent
  ```
- **`log`** — List past cooks (mode, target, duration, outcome) from local history.

  _Answers historical questions about the oven that are impossible through June's API._

  ```bash
  juneoven-pp-cli log --since 7d --agent
  ```
- **`curve`** — Export one recorded cook's temperature/progress samples as JSON or CSV.

  _Gives a plottable series for one cook, for analysis or piping._

  ```bash
  juneoven-pp-cli curve 3 --format csv
  ```
- **`preheat-stats`** — Median/fastest/slowest time-to-target per cook over recorded cooks.

  _Shows how your specific oven performs over time and whether it is degrading._

  ```bash
  juneoven-pp-cli preheat-stats --cook bake --agent
  ```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
juneoven-pp-cli devices associated mock-value

# JSON for scripting and agents
juneoven-pp-cli devices associated mock-value --json

# Filter to specific fields
juneoven-pp-cli devices associated mock-value --json --select id,name,status

# Dry run — show the request without sending
juneoven-pp-cli devices associated mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
juneoven-pp-cli devices associated mock-value --agent
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
juneoven-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `juneoven-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/juneoven-pp-cli/config.toml`; `--home`, `JUNEOVEN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JUNE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `juneoven-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `juneoven-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $JUNE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **commands say 'no paired oven found'** — run `juneoven-pp-cli pair` and enter the code on the oven
- **ready/eta report no cavity temperature** — some June firmware streams only camera frames; use `watch` or the oven screen for the preheat signal
