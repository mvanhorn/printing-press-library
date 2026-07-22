# Bambu Lab CLI

**Agent-ready Bambu print monitoring with exact plate previews and start/finish payloads.**

For people wiring an agent or code to monitor a Bambu print and post updates to Discord, a webhook, or another automation. Use normal local access for live MQTT status, exact-current-job 3MF weight and preview, provider-neutral start/finish events, and local history; Developer Mode, LAN Only Mode, Bambu Cloud, printer control, and farm management are intentionally unsupported.

Learn more at [Bambu Lab](https://printingpress.dev).

Created by [@twidtwid](https://github.com/twidtwid) (Todd Dailey).

## Install

The recommended path installs both the `bambu-pp-cli` binary and the `pp-bambu` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bambu
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bambu --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bambu --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bambu --agent claude-code
npx -y @mvanhorn/printing-press-library install bambu --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bambu/cmd/bambu-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bambu-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bambu --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bambu --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bambu --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bambu --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bambu-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BAMBU_SERIAL` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bambu/cmd/bambu-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bambu": {
      "command": "bambu-pp-mcp",
      "env": {
        "BAMBU_SERIAL": "<your-printer-serial>",
        "BAMBU_ACCESS_CODE": "<your-local-access-code>"
      }
    }
  }
}
```

</details>

## Authentication

Set BAMBU_SERIAL and BAMBU_ACCESS_CODE from the printer network settings. BAMBU_HOST is optional because discovery follows DHCP changes; set it only when multicast SSDP cannot cross the local network. The CLI uses normal local access only, validates the printer CA, and matches the peer certificate CN to the configured serial.

## Quick Start

```bash
# Validate local configuration and command shape without contacting or changing the printer.
bambu-pp-cli doctor --dry-run

# Find the configured serial on the LAN without hard-coding a DHCP address.
bambu-pp-cli discover --agent

# Fetch a fresh status while keeping the agent payload compact.
bambu-pp-cli printer status --agent --select state,job.name,job.percent,job.estimated_finish_at,temperatures,ams_active

# Monitor one print; the CLI manages its event log and preview assets privately.
bambu-pp-cli events monitor --agent

# Record a fresh observation and current-job baseline; LAN sync does not backfill historical transitions.
bambu-pp-cli sync --resources snapshots,jobs,transitions --since 24h

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### History that improves live decisions
- **`job eta`** — Correct the printer's finish estimate using the error pattern from prior runs of the same job.

  _Use this when an automation or person needs a finish time grounded in this printer's observed history._

  ```bash
  bambu-pp-cli job eta --agent
  ```
- **`history failure-correlations`** — Rank the printer, filament, plate, firmware, speed, and temperature contexts most associated with failed jobs.

  _Use this when repeated failures need evidence-backed triage rather than inspection of one job._

  ```bash
  bambu-pp-cli history failure-correlations --since 30d --agent
  ```
- **`job timeline`** — Reconstruct one print's stages, pauses, layers, temperature recovery, and errors as an ordered timeline.

  _Use this to explain where one print slowed, paused, recovered, or failed._

  ```bash
  bambu-pp-cli job timeline --latest --agent
  ```
- **`job repeats`** — Compare duration, pauses, material, errors, and outcomes across repeated runs of the same plate.

  _Use this to judge whether a recurring print is stable and repeatable._

  ```bash
  bambu-pp-cli job repeats "Colored Accents" --agent
  ```

### Material-aware operation
- **`ams runway`** — Estimate whether the active AMS tray can cover the current plate's remaining weight, with explicit unknown output when tray estimates or multi-material mapping are ambiguous.

  _Use this before leaving a long print unattended or deciding which spool needs attention._

  ```bash
  bambu-pp-cli ams runway --agent
  ```

### Integration resilience
- **`printer field-diff`** — Compare first and latest persisted redacted MQTT schemas for added, removed, and type-changed fields across a selected window.

  _Use this to inspect structural report changes between locally persisted observations._

  ```bash
  bambu-pp-cli printer field-diff --since 7d --agent
  ```

## Recipes

### Automation-ready print events

```bash
bambu-pp-cli events monitor --agent
```

Run this before starting the print. The command waits for one print, streams its start payload with available embedded 3MF project/profile titles, object names, weight, and preview, exits after the terminal payload, and stores the same NDJSON plus any preview in a private timestamped data directory. The matching terminal payload reuses that enriched identity without repeating the attachment. `job.name` prefers the embedded 3MF project title, then a sole printable object's extension-free name; `job.source_name` preserves the printer label, while `job.project_name`, `job.profile_name`, and `job.objects` remain explicit. The local Bambu Studio window filename is not transmitted in the printer-side 3MF and cannot be recovered through LAN MQTT/FTPS. Use `--output-dir` only when another program requires a specific location. Display-started jobs may expose only printer-resident G-code and therefore omit 3MF metadata.

`events watch` is the lower-level primitive for long-running consumers that manage their own lifecycle and asset storage.

For a daemon or bot that should keep watching across many jobs, always give the otherwise-unbounded watcher an explicit operational lifetime:

```bash
bambu-pp-cli events watch --agent --asset-dir <dir> --timeout 24h
```

### Compact current-print summary

```bash
bambu-pp-cli printer status --agent --select state,job.name,job.percent,job.estimated_finish_at,job.current_layer,job.total_layers,ams_active
```

Keep only the fields a notification or agent needs.

### Check filament runway

```bash
bambu-pp-cli ams runway --agent
```

Estimate whether loaded material can finish the current plate.

### Explain the latest print

```bash
bambu-pp-cli job timeline --latest --agent
```

Reconstruct stages, pauses, layer progress, temperature recovery, and errors.

### Find recurring failure context

```bash
bambu-pp-cli history failure-correlations --since 30d --agent
```

Group failed jobs by printer, filament, plate, firmware, speed, and temperature context.

## Usage

Run `bambu-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BAMBU_CONFIG_DIR`, `BAMBU_DATA_DIR`, `BAMBU_STATE_DIR`, or `BAMBU_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BAMBU_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BAMBU_HOME=/srv/bambu
bambu-pp-cli doctor
```

Under `BAMBU_HOME=/srv/bambu`, the four dirs resolve to `/srv/bambu/config`, `/srv/bambu/data`, `/srv/bambu/state`, and `/srv/bambu/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "bambu": {
      "command": "bambu-pp-mcp",
      "env": {
        "BAMBU_HOME": "/srv/bambu"
      }
    }
  }
}
```

Path precedence matters: an ambient per-kind variable such as `BAMBU_DATA_DIR` overrides an explicit `--home` for that kind. Use `BAMBU_HOME` or the per-kind variables for durable installation relocation; treat `--home` as the weaker per-invocation setting.

Relocation is one-way. Unsetting `BAMBU_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `bambu-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### observations

Locally persisted, redacted printer observations collected from LAN status and event commands.

- **`bambu-pp-cli observations`** - List locally persisted printer observations.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bambu-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bambu-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bambu-pp-cli learnings list`** - Inspect taught rows
- **`bambu-pp-cli learnings forget <query>`** - Undo a teach
- **`bambu-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bambu-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bambu-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bambu-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BAMBU_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bambu-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bambu-pp-cli observations

# JSON for scripting and agents
bambu-pp-cli observations --json

# Filter to specific fields
bambu-pp-cli observations --json --select id,name,status

# Dry run — show the request without sending
bambu-pp-cli observations --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bambu-pp-cli observations --agent
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
bambu-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `bambu-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/bambu-pp-cli/config.toml`; `--home`, `BAMBU_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BAMBU_SERIAL` | per_call | Yes | Bambu printer serial used for discovery filtering, MQTT topics, and certificate identity checks. |
| `BAMBU_ACCESS_CODE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bambu-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bambu-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BAMBU_SERIAL`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Discovery finds no printer** — Set BAMBU_HOST only if multicast SSDP cannot reach the printer across your VLAN or subnet.
- **Certificate identity mismatch** — Verify BAMBU_SERIAL against the printer; do not disable certificate verification.
- **MQTT connects but status remains empty** — Verify the access code and that TCP 8883 is reachable, then run bambu-pp-cli doctor --agent.
- **FTPS reports session reuse required** — Run bambu-pp-cli doctor --agent and report the output; this CLI must negotiate implicit TLS data-channel session reuse.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Bambuddy**](https://github.com/maziggy/bambuddy) — Python (2502 stars)
- [**Bambu Lab Home Assistant**](https://github.com/greghesp/ha-bambulab) — Python (2244 stars)
- [**BambuTools API**](https://github.com/BambuTools/bambulabs_api) — Python (318 stars)
- [**Bambu Printer MCP**](https://github.com/DMontgomery40/bambu-printer-mcp) — TypeScript (76 stars)
- [**bambu-cli**](https://github.com/tobiasbischoff/bambu-cli) — Go (23 stars)
- [**bambu-node**](https://github.com/THE-SIMPLE-MARK/bambu-node) — TypeScript (18 stars)
- [**Bambu MCP**](https://github.com/schwarztim/bambu-mcp) — TypeScript (15 stars)
- [**Madcow Bambu core**](https://github.com/twidtwid/madcowbot) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
