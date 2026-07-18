# Vehicle Safety CLI

**Build source-attributed vehicle safety dossiers, honest comparisons, and recall-change reports from NHTSA data.**

Query NHTSA recalls, complaints, ratings, product catalogs, and VIN decoding, then preserve observations locally. Compound commands distinguish raw reports from rates and model-wide campaigns from VIN-specific open recalls.

## Install

The recommended path installs both the `vehicle-safety-pp-cli` binary and the `pp-vehicle-safety` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety --agent claude-code
npx -y @mvanhorn/printing-press-library install vehicle-safety --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/vehicle-safety/cmd/vehicle-safety-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vehicle-safety-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vehicle-safety --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vehicle-safety --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install vehicle-safety --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vehicle-safety-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/vehicle-safety/cmd/vehicle-safety-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vehicle-safety": {
      "command": "vehicle-safety-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify local configuration and data-source reachability.
vehicle-safety-pp-cli doctor

# Inspect source recall records with narrowed agent output.
vehicle-safety-pp-cli recalls list-by-vehicle --make Honda --model Civic --model-year 2020 --agent --select results.NHTSACampaignNumber,results.Component

# Build the complete model-year dossier.
vehicle-safety-pp-cli dossier --year 2020 --make Honda --model Civic --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence-first vehicle research
- **`dossier`** — See identity, recall and complaint summaries, and available crash ratings in one source-attributed report; add --include-records for complete source records.

  _Use this instead of stitching several NHTSA responses together manually._

  ```bash
  vehicle-safety-pp-cli dossier --year 2020 --make Honda --model Civic --agent
  ```
- **`signals`** — Place complaint and recall events on one normalized timeline and identify the official flat-file sources required for investigations and communications.

  _Use this to inspect structured precursor signals without causal or rate claims._

  ```bash
  vehicle-safety-pp-cli signals --year 2020 --make Honda --model Civic --agent
  ```
- **`compare`** — Compare two model years while keeping raw complaint counts and missing denominators explicit.

  _Use this for a defensible comparison rather than a synthetic risk score._

  ```bash
  vehicle-safety-pp-cli compare '2020 Honda Civic' '2020 Toyota Corolla' --agent
  ```
- **`recall-coverage`** — Decode one VIN, list matching model-level campaigns, and keep VIN eligibility and repair status explicitly unverified with a link to NHTSA's official lookup.

  _Use this when model recall eligibility and VIN open-recall status must not be conflated._

  ```bash
  vehicle-safety-pp-cli recall-coverage 1HGCV1F34LA000001 --agent
  ```

### Local history that compounds
- **`watch`** — Report newly observed recall campaigns and remedy changes for saved vehicles.

  _Use this for finite, auditable fleet change detection._

  ```bash
  vehicle-safety-pp-cli watch --garage vehicles.csv --agent
  ```
- **`bulletin-bridge`** — Show structured complaint and manufacturer-communication co-occurrence candidates.

  _Use this for auditable overlap without semantic or causal inference._

  ```bash
  vehicle-safety-pp-cli bulletin-bridge --year 2020 --make Honda --model Civic --communications-file communications.tsv --agent
  ```

## Recipes

### Narrow a recall response

```bash
vehicle-safety-pp-cli recalls list-by-vehicle --make Honda --model Civic --model-year 2020 --agent --select results.NHTSACampaignNumber,results.Component,results.Remedy
```

Return only campaign, component, and remedy evidence.

### Compare two model years

```bash
vehicle-safety-pp-cli compare '2020 Honda Civic' '2020 Toyota Corolla' --agent
```

Compare source records without inventing a risk score.

### Reconcile recall coverage

```bash
vehicle-safety-pp-cli recall-coverage 1HGCV1F34LA000001 --agent
```

Decode the VIN and list model-level campaigns while linking to the official lookup for unverified VIN repair status.

## Usage

Run `vehicle-safety-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `VEHICLE_SAFETY_CONFIG_DIR`, `VEHICLE_SAFETY_DATA_DIR`, `VEHICLE_SAFETY_STATE_DIR`, or `VEHICLE_SAFETY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `VEHICLE_SAFETY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export VEHICLE_SAFETY_HOME=/srv/vehicle-safety
vehicle-safety-pp-cli doctor
```

Under `VEHICLE_SAFETY_HOME=/srv/vehicle-safety`, the four dirs resolve to `/srv/vehicle-safety/config`, `/srv/vehicle-safety/data`, `/srv/vehicle-safety/state`, and `/srv/vehicle-safety/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "vehicle-safety": {
      "command": "vehicle-safety-pp-mcp",
      "env": {
        "VEHICLE_SAFETY_HOME": "/srv/vehicle-safety"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `VEHICLE_SAFETY_DATA_DIR` overrides an explicit `--home` for that kind. Use `VEHICLE_SAFETY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `VEHICLE_SAFETY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `vehicle-safety-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### complaints

Manage complaints

- **`vehicle-safety-pp-cli complaints`** - List owner complaints for a model year, make, and model

### products

Manage products

- **`vehicle-safety-pp-cli products list-vehicle-makes`** - List makes for a model year and issue type
- **`vehicle-safety-pp-cli products list-vehicle-model-years`** - List vehicle model years for an issue type
- **`vehicle-safety-pp-cli products list-vehicle-models`** - List models for a make, model year, and issue type

### recalls

Manage recalls

- **`vehicle-safety-pp-cli recalls get-campaign`** - Get a recall by NHTSA campaign number
- **`vehicle-safety-pp-cli recalls list-by-vehicle`** - List recalls for a model year, make, and model

### safety-ratings

Manage safety ratings

- **`vehicle-safety-pp-cli safety-ratings get`** - Get crash ratings for a NHTSA vehicle ID
- **`vehicle-safety-pp-cli safety-ratings list-by-vehicle`** - List crash-rating vehicle variants

### vehicles

Manage vehicles

- **`vehicle-safety-pp-cli vehicles <vin>`** - Decode a VIN into normalized vehicle attributes


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`vehicle-safety-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`vehicle-safety-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`vehicle-safety-pp-cli learnings list`** - Inspect taught rows
- **`vehicle-safety-pp-cli learnings forget <query>`** - Undo a teach
- **`vehicle-safety-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`vehicle-safety-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`vehicle-safety-pp-cli teach-pattern`** - Install a query/resource template up front
- **`vehicle-safety-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `VEHICLE_SAFETY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `vehicle-safety-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vehicle-safety-pp-cli safety-ratings get mock-value

# JSON for scripting and agents
vehicle-safety-pp-cli safety-ratings get mock-value --json

# Filter to specific fields
vehicle-safety-pp-cli safety-ratings get mock-value --json --select id,name,status

# Dry run — show the request without sending
vehicle-safety-pp-cli safety-ratings get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vehicle-safety-pp-cli safety-ratings get mock-value --agent
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
vehicle-safety-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `vehicle-safety-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/vehicle-safety-pp-cli/config.toml`; `--home`, `VEHICLE_SAFETY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **VIN lookup is throttled** — Retry later and keep garage checks small; NHTSA prohibits bulk VIN lookup through this API.
- **Complaint totals differ between vehicles** — Treat totals as raw published reports, not incidence rates; NHTSA does not provide an exposure denominator here.
