# Dreo CLI

**The only standalone CLI for Dreo smart-home devices — bulk control, live sensor streams, and a local history every other Dreo tool throws away.**

Every existing Dreo client is bundled inside Home Assistant or Homebridge; this is the first general-purpose CLI. It speaks the same WebSocket control protocol the reverse-engineered clients use, plus it keeps a local SQLite cache so bulk fan-out, cross-device sensor snapshots, sensor history, scenes, and alerts work even when the cloud is slow.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `dreo-pp-cli` binary and the `pp-dreo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dreo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dreo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dreo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dreo --agent claude-code
npx -y @mvanhorn/printing-press-library install dreo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/dreo/cmd/dreo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dreo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dreo --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-dreo --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install dreo --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dreo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DREO_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dreo": {
      "command": "dreo-pp-mcp",
      "env": {
        "DREO_USERNAME": "<your-dreo-email>",
        "DREO_PASSWORD": "<your-dreo-password>"
      }
    }
  }
}
```

</details>

## Authentication

Dreo has no public developer API. Authentication uses your Dreo account email and password against the same OAuth endpoint the Dreo iOS app calls — the password is MD5-hashed before the wire, and the bearer token returned is cached locally.

**Set these two env vars:**

```bash
export DREO_USERNAME='your-dreo-account-email'
export DREO_PASSWORD='your-dreo-password'
```

Then run any command. The CLI exchanges credentials for an access token on first use, caches both the credentials and the bearer to `~/.config/dreo-pp-cli/config.toml` at mode `0600`, and reuses them. When the bearer expires it transparently re-logs in using the cached credentials — Dreo's OAuth flow has no refresh token, so caching the credentials is what lets cron jobs and unattended runs survive token expiry without re-exporting env vars every session. Region discovery (`us`/`eu`) happens automatically on first login; pin it with `DREO_REGION=us` to skip the round-trip.

**Three ways to supply credentials:**

1. **Env vars (recommended):** Set `DREO_USERNAME` and `DREO_PASSWORD` and run `dreo-pp-cli auth login`.
2. **`--password-stdin` (scriptable, no leak):** Pipe the password from a secret manager — Docker-style:
   ```bash
   op read 'op://Personal/Dreo/password' | dreo-pp-cli auth login --username me@example.com --password-stdin
   pass dreo/password         | dreo-pp-cli auth login --username me@example.com --password-stdin
   ```
3. **`--password <value>` flag (insecure, warns):** Supported for ergonomics, but `auth login` prints a stderr warning every time because the plaintext lands in `/proc/<pid>/cmdline`, `ps aux`, audit logs, and shell history. Use only when you understand the trade-off.

**What's cached on disk.** `~/.config/dreo-pp-cli/config.toml` (mode `0600`, in your home directory) contains:
- `username` and `password` — the credentials you supplied via env vars (in plaintext, mirroring AWS CLI's `~/.aws/credentials` and Stripe CLI's config conventions)
- `access_token` — the OAuth bearer
- `region`, `token_expiry`, timestamps — non-sensitive metadata

Treat the file as sensitive: don't commit it to a public repo, don't share it, and don't sync it to cloud-storage providers without encryption. Mode `0600` keeps it readable only by your user account. To wipe everything (credentials and token): `dreo-pp-cli auth logout`.

**Useful commands:**
- `dreo-pp-cli auth login` — explicit login (also runs lazily on first authenticated command, and automatically on 401)
- `dreo-pp-cli auth status` — current authentication state (doesn't reveal the token or password)
- `dreo-pp-cli auth logout` — wipe cached token and persisted credentials
- `dreo-pp-cli doctor` — verify everything end-to-end

## Quick Start

```bash
# Verify credentials and login round-trip against the live API
dreo-pp-cli doctor

# See every device on your Dreo account with model, room, and online status
dreo-pp-cli devices list

# Whole-house temperature, humidity, and PM2.5 in one shot
dreo-pp-cli sensors --json

# Turn off every tower fan in one command — the bedtime ritual
dreo-pp-cli bulk --action off --type tower-fan

# Live WebSocket state stream as JSON lines — the realtime debug surface no other Dreo tool exposes
dreo-pp-cli watch --all

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-device control
- **`bulk`** — Power, mode, or speed across every device matching a type/room filter in one command.

  _Replaces the #1 user pain — tapping each device in the Dreo app at bedtime — with one cron-callable line._

  ```bash
  dreo-pp-cli bulk --action off --type tower-fan --dry-run
  ```
- **`scene save`** — Capture the current state across selected devices as a named scene and replay it later as parallel WebSocket frames.

  _Sam's nightly bedtime routine becomes one command; survives app updates._

  ```bash
  dreo-pp-cli scene save bedtime --all && dreo-pp-cli scene apply bedtime --dry-run
  ```

### Cross-device intelligence
- **`sensors`** — Aggregated temperature, humidity, and PM2.5 across every sensor-bearing device in one ranked table.

  _Answers the agent question 'what's the air quality across my house?' in one tool call._

  ```bash
  dreo-pp-cli sensors --json
  ```
- **`sensors record`** — Persist WebSocket state events to a local sensor_readings table and query temperature/humidity/PM2.5 over arbitrary time windows.

  _Answers 'when did the bedroom fan last go to sleep mode' and similar historical questions agents and users actually ask._

  ```bash
  dreo-pp-cli sensors query --metric temperature --since 1h --json
  ```
- **`alerts`** — Report devices with low filter life, empty water tank, offline heartbeat, or sensor readings past a threshold.

  _Surfaces actionable problems (filter, water, dead devices) without manually inspecting each device._

  ```bash
  dreo-pp-cli alerts --pm25-above 50 --json
  ```
- **`rooms`** — Group devices by room with on-count, average temperature, and average humidity per room.

  _Answers 'what's happening in my bedroom right now' in one query._

  ```bash
  dreo-pp-cli rooms --json
  ```
- **`devices search`** — Full-text search over cached device name, room, model, and serial.

  _Fast device lookup for scripts and agents without round-tripping the cloud._

  ```bash
  dreo-pp-cli devices search Fan
  ```

### Realtime observability
- **`watch`** — Tail-f for Dreo device state — every WebSocket update as a JSON line on stdout.

  _Enables automation debugging without running Home Assistant, and feeds agent stream-processing pipelines._

  ```bash
  dreo-pp-cli watch --all --json
  ```

## Usage

Run `dreo-pp-cli --help` for the full command reference and flag list.

## Commands

### devices

Discover and inspect Dreo devices on your account

- **`dreo-pp-cli devices list`** - List every Dreo device on your account
- **`dreo-pp-cli devices state`** - Read the current state snapshot for one device

### firmware

Read firmware metadata and check for updates

- **`dreo-pp-cli firmware`** - Check whether a firmware update is available for a device

### settings

Read and write persistent per-device settings

- **`dreo-pp-cli settings get`** - Get persistent settings for a device
- **`dreo-pp-cli settings update`** - Update persistent settings for a device

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dreo-pp-cli devices list

# JSON for scripting and agents
dreo-pp-cli devices list --json

# Filter to specific fields
dreo-pp-cli devices list --json --select id,name,status

# Dry run — show the request without sending
dreo-pp-cli devices list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dreo-pp-cli devices list --agent
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
dreo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dreo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DREO_USERNAME` | per_call | Yes | Set to your API credential. |
| `DREO_PASSWORD` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dreo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DREO_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports auth failure** — Verify DREO_USERNAME and DREO_PASSWORD are exported; if you changed your password in the Dreo app, run dreo-pp-cli auth login again
- **Empty device list** — Run dreo-pp-cli devices list --live to bypass the local cache and fetch fresh from the cloud
- **Commands return but device state does not change** — Add --wait to the set/bulk command so the CLI confirms the WebSocket state echo before exiting
- **Wrong region** — Set DREO_REGION=eu (or us) to skip the region-discovery round-trip; the CLI auto-detects on first login

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hass-dreo**](https://github.com/JeffSteinbok/hass-dreo) — Python (200 stars)
- [**homebridge-dreo**](https://github.com/zyonse/homebridge-dreo) — TypeScript (80 stars)
- [**hass-dreoverse**](https://github.com/dreo-team/hass-dreoverse) — Python (30 stars)
- [**pydreo-client**](https://github.com/dreo-team/pydreo-client) — Python (20 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
