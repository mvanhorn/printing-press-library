# Googlehealth CLI

View and manage health and fitness metrics and measurement data from Fitbit, Pixel Watch, and other third-party devices and apps.

Created by [@coopdogGGs](https://github.com/coopdogGGs) (ryanc00per).

## Install

The recommended path installs both the `googlehealth-pp-cli` binary and the `pp-googlehealth` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install googlehealth
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install googlehealth --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install googlehealth --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install googlehealth --agent claude-code
npx -y @mvanhorn/printing-press-library install googlehealth --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/googlehealth/cmd/googlehealth-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/googlehealth-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install googlehealth --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-googlehealth --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-googlehealth --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install googlehealth --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/googlehealth-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLEHEALTH_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/googlehealth/cmd/googlehealth-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "googlehealth": {
      "command": "googlehealth-pp-mcp",
      "env": {
        "GOOGLEHEALTH_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
googlehealth-pp-cli doctor

googlehealth-pp-cli sync --resources steps,distance,daily-resting-heart-rate

googlehealth-pp-cli trends --window 7 --json

googlehealth-pp-cli correlate --a steps --b daily-resting-heart-rate --max-lag 3 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`trends`** — Per-metric trailing rolling-average trend lines with net first-to-last delta, de-noising day-to-day scale-weight and resting-HR whiplash
- **`streaks`** — Current and longest consecutive-calendar-day streaks where a metric met a goal (e.g. 10k steps, resting HR under 60), with calendar gaps breaking the run
- **`correlate`** — Pearson correlation plus best-lag scan between any two daily metrics (steps vs resting HR, sleep vs HRV) over locally synced history
- **`sync`** — Mirror Google Health data points across data types into a local SQLite store for offline analysis
- **`search`** — FTS5 full-text search across locally synced Google Health records

## Usage

Run `googlehealth-pp-cli --help` for the full command reference and flag list.

## Commands

### projects

Manage projects


### users

Manage users



## Output Formats

Every command supports these output flags:

- `--json` — structured output for scripting and agents
- `--select id,name,status` — filter to specific fields
- `--dry-run` — preview the request without sending
- `--agent` — JSON + compact + non-interactive in one flag

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
googlehealth-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-health-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLEHEALTH_OAUTH2C` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `googlehealth-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `googlehealth-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLEHEALTH_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
