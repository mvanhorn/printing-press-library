# Withings CLI

**Every Withings metric in one offline-first, agent-native CLI — with a local SQLite mirror and recomposition, recovery, sleep-debt and clinician-report analytics no Withings tool has.**

Pulls your weight/body-composition, activity, sleep, heart/ECG and workouts from the official Withings API into a local SQLite store you can search, query with SQL, and export as JSON or CSV. On top of that it adds local-join analytics — `recomp`, `recovery`, `bp-report`, `sleep debt`, `digest`, `correlate` — that no single API call can produce.

## Install

The recommended path installs both the `withings-pp-cli` binary and the `pp-withings` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install withings
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install withings --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install withings --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install withings --agent claude-code
npx -y @mvanhorn/printing-press-library install withings --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/withings/cmd/withings-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/withings-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install withings --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-withings --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-withings --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install withings --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/withings-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WITHINGS_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/withings/cmd/withings-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "withings": {
      "command": "withings-pp-mcp",
      "env": {
        "WITHINGS_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Withings uses OAuth2. Register a free app at the Withings developer portal, run `withings-pp-cli auth login` once to authorize in your browser, and the CLI stores and auto-rotates your refresh token (Withings refresh tokens are single-use, so the CLI persists each new one for you).

## Quick Start

```bash
# Check config and reachability before anything else (works without auth).
withings-pp-cli doctor --dry-run

# Mirror the last 90 days into local SQLite.
withings-pp-cli sync --resources measure,activity,sleep --since 90d

# See your real fat-vs-lean recomposition trend.
withings-pp-cli recomp --since 90d

# Structured 'what changed' snapshot for an agent.
withings-pp-cli digest --since 24h --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local-join analytics
- **`recomp`** — See whether you're actually recomposing — fat mass down while lean mass holds — on a de-noised rolling-average weight, not scale-weight whiplash.

  _Reach for this instead of raw measures when the question is 'is my cut/bulk working', not 'what did I weigh on Tuesday'._

  ```bash
  withings-pp-cli recomp --since 90d --agent
  ```
- **`recovery`** — Weigh recent workout HR-zone load against your recovery markers (resting HR, HRV, sleep score) to catch overtraining before it catches you.

  _Use when deciding whether to push or back off; it correlates how hard you trained with how recovered you are._

  ```bash
  withings-pp-cli recovery --since 14d --agent
  ```
- **`sleep debt`** — Cumulative sleep deficit against your target over a rolling window — the number the per-night summary never adds up for you.

  _Use for sleep-deficit accounting over time; for one night use 'sleep summary'._

  ```bash
  withings-pp-cli sleep debt --window 14d --agent
  ```
- **`correlate`** — Pearson + best-lag correlation between any two daily metrics (weight vs sleep score, steps vs resting HR).

  _The flexible fallback when no curated readout fits the metric pair you care about._

  ```bash
  withings-pp-cli correlate weight sleep_score --since 90d --agent
  ```

### Clinician + agent reports
- **`bp-report`** — A dated blood-pressure + AFib table with your own annotations (medication changes, symptoms) — the clean history to hand a cardiologist.

  _Use for a doctor-ready BP/AFib history; prefer it over raw 'heart list' when you need the annotated report._

  ```bash
  withings-pp-cli bp-report --since 90d --agent
  ```
- **`digest`** — One structured 'what changed since <time> across all my metrics' snapshot — built for piping into an agent.

  _The go-to for LLM health Q&A: one cursor-driven structured pull instead of N endpoint calls._

  ```bash
  withings-pp-cli digest --since 24h --agent
  ```

## Recipes


### Mirror everything, then ask offline

```bash
withings-pp-cli sync --resources measure,activity,sleep,workouts,heart --since 180d
```

One incremental sync populates the local store for fast offline analytics.

### Agent-native health digest

```bash
withings-pp-cli digest --since 24h --agent --select weight,sleep_score,resting_hr,new_afib
```

Narrowed structured output an agent can consume without parsing the full envelope.

### Doctor-ready BP/AFib history

```bash
withings-pp-cli bp-report --since 90d --note 2026-06-03=started-amlodipine --csv
```

A dated BP + AFib table with your medication-change annotation, exported as CSV.

### Is my cut working?

```bash
withings-pp-cli recomp --since 90d
```

De-noised weight trend plus fat-down/muscle-held verdict over the window.

## Usage

Run `withings-pp-cli --help` for the full command reference and flag list.

## Commands

### activity

Daily and intraday activity: steps, distance, calories, heart-rate zones

- **`withings-pp-cli activity get`** - Daily activity aggregates (getactivity).
- **`withings-pp-cli activity intraday`** - High-resolution intraday activity series (getintradayactivity).

### devices

Linked Withings devices

- **`withings-pp-cli devices`** - List linked devices (getdevice): model, battery, last sync.

### goals

User goals: steps, sleep, weight

- **`withings-pp-cli goals`** - Get user goals (getgoals).

### heart

ECG/AFib recordings and blood-pressure readings

- **`withings-pp-cli heart ecg`** - Get a single raw ECG signal by signalid (get).
- **`withings-pp-cli heart list`** - List heart recordings (list): ECG signalids, AFib classification, BP.

### measure

Body measurements: weight, body composition, blood pressure, SpO2, temperature, ECG intervals

- **`withings-pp-cli measure`** - Get body measurements (getmeas). Filter by type codes and date range.

### notify

Webhook subscriptions (Notify)

- **`withings-pp-cli notify get`** - Get one subscription (get).
- **`withings-pp-cli notify list`** - List webhook subscriptions (list).
- **`withings-pp-cli notify revoke`** - Revoke a webhook subscription (revoke).
- **`withings-pp-cli notify subscribe`** - Subscribe a webhook (subscribe).

### sleep

Sleep stage series and per-night summaries

- **`withings-pp-cli sleep series`** - High-frequency sleep stage series (get).
- **`withings-pp-cli sleep summary`** - Per-night sleep summary (getsummary): durations, HR, RR, snoring, AHI, sleep score.

### workouts

Workout sessions with HR zones, distance, calories

- **`withings-pp-cli workouts`** - List workout sessions (getworkouts).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
withings-pp-cli activity get

# JSON for scripting and agents
withings-pp-cli activity get --json

# Filter to specific fields
withings-pp-cli activity get --json --select id,name,status

# Dry run — show the request without sending
withings-pp-cli activity get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
withings-pp-cli activity get --agent
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
withings-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/withings-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WITHINGS_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `withings-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `withings-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WITHINGS_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / token expired** — Run `withings-pp-cli auth refresh` (or `auth login` to re-authorize); access tokens last 3h and the CLI rotates the single-use refresh token automatically.
- **status 601 in responses** — Rate limit (120 req/min). Slow down or rely on local sync + offline queries instead of repeated live calls.
- **empty results after sync** — Confirm the resource was synced: `withings-pp-cli sync --resources measure` then `withings-pp-cli measure get --limit 5`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**withings-sync**](https://github.com/jaroslawhartman/withings-sync) — Python (670 stars)
- [**python_withings_api**](https://github.com/vangorra/python_withings_api) — Python (110 stars)
- [**withings-mcp**](https://github.com/akutishevsky/withings-mcp) — TypeScript (26 stars)
- [**aiowithings**](https://github.com/joostlek/python-withings) — Python (7 stars)
- [**withings-go**](https://github.com/zono-dev/withings-go) — Go (6 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
