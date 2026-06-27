# Utah Public Notice Website CLI

**One command to sweep Utah public-meeting notices for land-use approvals — county-wide, agenda-scanned, and diffed against what you've already seen.**

The state Public Notice Website makes you pick a ZIP and click through bodies one at a time. This CLI sweeps a whole county's towns in one call, keeps only planning/council/commission/board meetings, scans the inline agendas for rezones, subdivisions, CUPs, and variances, and with a local store tells you what's new since your last run. Built for tracking Delta City and Millard County development approvals.

## Install

The recommended path installs both the `utah-pmn-pp-cli` binary and the `pp-utah-pmn` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn --agent claude-code
npx -y @mvanhorn/printing-press-library install utah-pmn --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/cmd/utah-pmn-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/utah-pmn-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-utah-pmn --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-utah-pmn --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install utah-pmn --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/utah-pmn-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/cmd/utah-pmn-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "utah-pmn": {
      "command": "utah-pmn-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the PMN endpoint is reachable
utah-pmn-pp-cli doctor --dry-run

# land-use meetings across Millard County for the next month
utah-pmn-pp-cli millard --days 30

# Delta-area approval items for the next two months
utah-pmn-pp-cli landuse --location Delta --days 60

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### County-wide land-use monitoring
- **`millard`** — Sweep every Millard County town in one command and keep only land-use approval bodies (planning commissions, councils, county commission, boards).

  _Pick this when the user asks what development/zoning meetings are coming up in Millard County._

  ```bash
  utah-pmn-pp-cli millard --days 30 --agent
  ```
- **`landuse`** — Filter any notice set to meetings whose body or agenda involves zoning, subdivisions, rezones, CUPs, variances, annexations, or plats.

  _Pick this to cut a body's full calendar down to only the approval items that matter._

  ```bash
  utah-pmn-pp-cli landuse --location Delta --days 60 --agent
  ```
- **`agenda scan`** — Search the inline agenda text of upcoming notices for arbitrary terms (e.g. a project name, a parcel, an applicant), with surrounding context.

  _Pick this to find every meeting touching a specific project, parcel, or applicant._

  ```bash
  utah-pmn-pp-cli agenda scan "solar" --agent
  ```
- **`locations`** — List the curated Millard County town/ZIP set the sweep uses, so the coverage is transparent and editable.

  _Pick this to see or confirm which towns a county sweep covers._

  ```bash
  utah-pmn-pp-cli locations --agent
  ```

### Local state that compounds
- **`since`** — Show only notices first seen since the last run, so a scheduled run reports just what changed. The CLI records what it has seen locally; it does not depend on a separate sync step.

  _Pick this for a recurring/scheduled check so the user only sees new meetings._

  ```bash
  utah-pmn-pp-cli since --location Fillmore --agent
  ```
- **`watch`** — Track a saved list of public bodies (e.g. Delta City Council, Millard County Planning Commission) and report their upcoming meetings across locations.

  _Pick this when the user cares about specific bodies rather than a whole county._

  ```bash
  utah-pmn-pp-cli watch list --agent
  ```

## Recipes


### Monthly county scan

```bash
utah-pmn-pp-cli millard --days 30 --agent
```

All land-use bodies meeting in Millard County over the next month, structured for an agent.

### New since last check

```bash
utah-pmn-pp-cli since --location Fillmore --agent
```

Only notices first seen since the previous run — ideal on a schedule.

### Find a project by name

```bash
utah-pmn-pp-cli agenda scan "subdivision" --agent --select noticeId,publicBodyName,meetingStartTime
```

Every stored agenda mentioning the term, narrowed to the key fields.

## Usage

Run `utah-pmn-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UTAH_PMN_CONFIG_DIR`, `UTAH_PMN_DATA_DIR`, `UTAH_PMN_STATE_DIR`, or `UTAH_PMN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UTAH_PMN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UTAH_PMN_HOME=/srv/utah-pmn
utah-pmn-pp-cli doctor
```

Under `UTAH_PMN_HOME=/srv/utah-pmn`, the four dirs resolve to `/srv/utah-pmn/config`, `/srv/utah-pmn/data`, `/srv/utah-pmn/state`, and `/srv/utah-pmn/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "utah-pmn": {
      "command": "utah-pmn-pp-mcp",
      "env": {
        "UTAH_PMN_HOME": "/srv/utah-pmn"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `UTAH_PMN_DATA_DIR` overrides an explicit `--home` for that kind. Use `UTAH_PMN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UTAH_PMN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `utah-pmn-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### notice

A single notice detail page (full agenda, minutes, attached materials)

- **`utah-pmn-pp-cli notice`** - Fetch the full HTML detail page for a notice, including minutes and material links

### notices

Public meeting notices published by Utah public bodies (agendas inline)

- **`utah-pmn-pp-cli notices`** - List meeting notices near a ZIP code or city, within a date range. Returns past and future notices when startDate/endDate span them. Each notice carries the full agenda text inline.



## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
utah-pmn-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `utah-pmn-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `UTAH_PMN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A town returns no notices** — Nothing is scheduled in the window — widen --days or check another town with 'locations'.
- **Agenda text says TBD** — The body posted the meeting before finalizing its agenda; re-run closer to the meeting date or fetch the notice detail page.
