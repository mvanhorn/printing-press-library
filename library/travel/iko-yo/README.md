# Iko-yo Trip CLI

**Iko-yo Trip: find selected family trips and compare published dates, fees and explicit family facts.**

Discover Iko-yo Trip’s local experiences and municipal events, inspect evidence, and compare a small family shortlist. Keyword and date filters apply only to the reported bounded listing window; cached records retain their observation times. The full core Iko-yo catalog is not integrated.

Learn more at [Iko-yo Trip](https://trip.iko-yo.net).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `iko-yo-pp-cli` binary and the `pp-iko-yo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install iko-yo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install iko-yo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install iko-yo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install iko-yo --agent claude-code
npx -y @mvanhorn/printing-press-library install iko-yo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/iko-yo/cmd/iko-yo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iko-yo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install iko-yo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-iko-yo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-iko-yo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install iko-yo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iko-yo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/iko-yo/cmd/iko-yo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "iko-yo": {
      "command": "iko-yo-pp-mcp"
    }
  }
}
```

</details>

## Authentication

The supported Trip pages use ordinary public HTTP and need no account or API key. Source records are published information, not live availability.

## Quick Start

```bash
# Check the local CLI setup without a network request.
iko-yo-pp-cli doctor --dry-run

# Read one Saitama event listing page and inspect its coverage.
iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --max-pages 1 --agent

# Inspect published age, facilities and qualified child/adult fees.
iko-yo-pp-cli trip inspect spots/8220 --agent

# Compare the selected facts and expose unknown requirements.
iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --age-months 24 --amenities indoor,nursing --agent

# Revisit facts saved by earlier source reads.
iko-yo-pp-cli trip cached Mooovi --agent

```

## Unique Features

Source-specific planning commands with explicit evidence and coverage.

### Find family-trip candidates
- **`trip discover`** — Find regional candidates while seeing exactly which listing pages and records were checked.

  _Choose this when a trip window matters and complete source coverage is unavailable._

  ```bash
  iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --from 2026-11-14 --to 2026-11-15 --max-pages 2 --agent
  ```

### Check published conditions
- **`trip inspect`** — See published age, indoor, nursing and changing evidence with unknown fields exposed.

  _Choose this before deciding whether published family facts meet a requirement._

  ```bash
  iko-yo-pp-cli trip inspect spots/8220 --agent
  ```
- **`trip compare`** — Compare separate fee statements, payment conditions and explicit application intervals.

  _Choose this to compare published costs and deadlines without assuming a total or seat availability._

  ```bash
  iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --as-of 2026-10-03 --agent
  ```
- **`trip compare`** — Compare selected outings against age, requested amenities and a trip date using supported, excluded and unknown states.

  _Choose this for a small family shortlist that needs explicit constraints._

  ```bash
  iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --age-months 24 --amenities indoor,nursing,changing --agent
  ```

### Revisit saved facts
- **`trip cached`** — Search saved family facts with their original observation times and collection coverage.

  _Choose this to revisit saved records without network access._

  ```bash
  iko-yo-pp-cli trip cached Mooovi --kind spots --agent
  ```

## Recipes

### Narrow event candidates

```bash
iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --max-pages 1 --agent --select records.ref,records.name,records.schedule,coverage
```

Inspect published event dates and the exact scanned source window.

### Published family facts

```bash
iko-yo-pp-cli trip inspect spots/8220 --agent
```

Read age and facility evidence and unknowns for one source record.

### Compare a family shortlist

```bash
iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --as-of 2026-10-03 --age-months 24 --amenities indoor,nursing,changing --agent
```

Compare constraints, charge qualifiers and application boundaries.

### Offline saved facts

```bash
iko-yo-pp-cli trip cached Mooovi --kind spots --agent
```

Search only previously saved normalized records with their provenance.

## Coverage and evidence

This integration uses Iko-yo Trip's selected nationwide family experiences and municipal/local events. It does not expose the full `iko-yo.net` catalog or its core age and facility filters.

| Command | Source and limits |
|---------|-------------------|
| `trip discover` | One to five listing pages total; publication order includes upcoming and ended events. Local keyword/date filters report scanned records, remaining pages, omitted matches and unknown schedules. |
| `trip inspect` | One selected detail page with bounded factual evidence, source/official URLs, observation time, qualified fees and explicit application terms. |
| `trip compare` | Up to eight selected details; numeric age and amenity evidence use supported/excluded/unknown states. Individual operation within a date span remains unknown. |
| `trip cached` | Partial saved facts only; independently bounded record scan with original observation times and bounded collection provenance. |

A zero match describes the scanned window only. Dates are published schedules, fees retain payment/admission qualifiers, and capacity or lotteries do not establish current seats. Missing amenities stay unknown. Local-only commands reject forced live sourcing; live discovery rejects forced local sourcing. Auto detail reads fall back to an existing observation only on network failure, with a warning; HTTP access errors and rate limits remain errors.

## Usage

Run `iko-yo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IKO_YO_CONFIG_DIR`, `IKO_YO_DATA_DIR`, `IKO_YO_STATE_DIR`, or `IKO_YO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IKO_YO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IKO_YO_HOME=/srv/iko-yo
iko-yo-pp-cli doctor
```

Under `IKO_YO_HOME=/srv/iko-yo`, the four dirs resolve to `/srv/iko-yo/config`, `/srv/iko-yo/data`, `/srv/iko-yo/state`, and `/srv/iko-yo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "iko-yo": {
      "command": "iko-yo-pp-mcp",
      "env": {
        "IKO_YO_HOME": "/srv/iko-yo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IKO_YO_DATA_DIR` overrides an explicit `--home` for that kind. Use `IKO_YO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IKO_YO_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `iko-yo-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### events

Operations on events

- **`iko-yo-pp-cli events get`** - Inspect published Iko-yo Trip event facts
- **`iko-yo-pp-cli events list`** - Fetch structured links from /events
- **`iko-yo-pp-cli events prefecture`** - Discover Iko-yo Trip events in a source prefecture
- **`iko-yo-pp-cli events region`** - Discover Iko-yo Trip events in a source region

### spots

Operations on spots

- **`iko-yo-pp-cli spots get`** - Inspect published Iko-yo Trip spot facts
- **`iko-yo-pp-cli spots list`** - Fetch structured links from /spots


### Saved facts and privacy

`trip` commands save only normalized public source facts unless `--no-cache` is set. Successful Trip invocations disable automatic query learning and journaling so per-invocation age/date constraints are not turned into travel histories. The framework's optional learning commands remain available for non-personal command patterns. Do not teach child profiles, contributor identities, or private trip histories. Use `--no-learn` or `IKO_YO_NO_LEARN=true` for other commands when appropriate.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
iko-yo-pp-cli events list

# JSON for scripting and agents
iko-yo-pp-cli events list --json
# Filter to specific fields by name
iko-yo-pp-cli events list --json --select ref,name,observed_at

# Dry run — show the request without sending
iko-yo-pp-cli events list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
iko-yo-pp-cli events list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select ref,name,observed_at` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - `trip cached` and `trip inspect --data-source local` read selected saved facts
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
iko-yo-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `iko-yo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/iko-yo-pp-cli/config.toml`; `--home`, `IKO_YO_HOME`, and per-kind env vars can relocate it.

Trip commands use fixed public HTTPS routes and do not load custom request headers. Profiles can reuse supported planning/output flags. No account or credentials are needed.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **No matches in a bounded discovery window.** — Inspect coverage and unknown-date counts, then increase trip discover --max-pages up to 5 or choose a source prefecture; this result does not establish source-wide absence.
- **An amenity, age band or event schedule is unknown.** — Use trip inspect for the selected reference and follow its source/official links; missing evidence does not mean absent or eligible.
- **Core Iko-yo pages return 403.** — Use the supported Trip commands; the core catalog is outside this integration.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://trip.iko-yo.net/
- Capture coverage: 0 API entries from 6 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: html_scrape (55% confidence)
- Auth signals: none

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
