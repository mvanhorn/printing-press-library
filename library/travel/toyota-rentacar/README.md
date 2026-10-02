# TOYOTA Rent a Car CLI

**Live Japan rental class estimates, distinct shops, and a careful booking handoff.**

Resolve pickup and return shops, compare real dated class offers, and inspect Toyota’s one-way surcharge and option policies. Final totals and individual driving eligibility remain explicit until Toyota verifies them.

Learn more at [TOYOTA Rent a Car](https://rent.toyota.co.jp).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Local build and planning contract

This verified build is local; public-library installation instructions below apply only after publication. From this checkout:

```bash
go build -o toyota-rentacar-pp-cli ./cmd/toyota-rentacar-pp-cli
go build -o toyota-rentacar-pp-mcp ./cmd/toyota-rentacar-pp-mcp
./toyota-rentacar-pp-cli shops search --keyword "Kyoto Station" --limit 3 --agent
```

Use `shops search/get`, `cars quote`, `rental options/eligibility`, `oneway quote`, and `booking handoff`. Dates without an offset are JST; choose future dates within three months,30-minute increments,and a rental within one calendar month. Replace the dated examples with your dates. `cars quote` confirms the source's shop/date/category/options context and operating calendars before reporting class inventory.

`source_estimate_jpy` is Toyota's class-page rental estimate. `confirmed_full_total_jpy` stays null; waiver/NOC,ETC/JAF,seats,tires,model selection and one-way inclusion are not inferred. Tolls,fuel and other charges require separate confirmation. This anonymous search does not select or verify a driver/license profile; reconfirm the applicable booking rate. A class does not guarantee its representative model. `oneway quote` returns a date-independent surcharge and explicitly unknown dated inventory. `booking handoff` only prefills the pickup shop; re-enter dates,return shop,class and options on Toyota.

Live planning results include UTC fetch time,upstream request count,response bytes and elapsed milliseconds. Requests use in-memory anonymous cookies,adaptive source throttling,a14-request cap and2MiB response limit; `--timeout` bounds the whole operation. Inventory is fetched live. The optional framework local store never establishes current inventory. No bookings,payments,terms acceptance or personal documents are submitted. The `source` mirror commands return page metadata and links; open their source URLs for full policy text.

## Install

The recommended path installs both the `toyota-rentacar-pp-cli` binary and the `pp-toyota-rentacar` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar --agent claude-code
npx -y @mvanhorn/printing-press-library install toyota-rentacar --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/cmd/toyota-rentacar-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toyota-rentacar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-toyota-rentacar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-toyota-rentacar --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install toyota-rentacar --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toyota-rentacar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/cmd/toyota-rentacar-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "toyota-rentacar": {
      "command": "toyota-rentacar-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public read-only HTTP. No API key,account login,browser dependency,or saved cookies.

## Quick Start

```bash
# Check setup without network.
toyota-rentacar-pp-cli doctor --dry-run

# Resolve distinct shops.
toyota-rentacar-pp-cli shops search --keyword "Kyoto Station" --limit 3 --agent

# Inspect source option and waiver fees.
toyota-rentacar-pp-cli rental options --agent

# Compare dated class estimates in JST.
toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent

# Run Toyota’s independent surcharge calculator.
toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent

```

## Unique Features

These commands provide Toyota-specific rental planning flows.

### Rental planning
- **`shops search`** — Resolve distinct Toyota branches and return restrictions.

  _Preserves first-party branch identities and Japanese names._

  ```bash
  toyota-rentacar-pp-cli shops search --keyword "Kyoto Station" --limit 3 --agent
  ```
- **`cars quote`** — Compare real dated class offers with explicit price assumptions.

  _Verifies source-echoed dates and shops before reporting offers._

  ```bash
  toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent
  ```
- **`rental options`** — Read live option and waiver fees with source units.

  _Separates optional fee estimates from unknown final totals._

  ```bash
  toyota-rentacar-pp-cli rental options --agent
  ```
- **`oneway quote`** — Run Toyota’s route surcharge simulator.

  _Keeps date-independent fees separate from dated class stock._

  ```bash
  toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent
  ```
- **`booking handoff`** — Print a canonical booking link and reusable rental checklist.

  _Date and options survive as a checklist rather than false session-bound links._

  ```bash
  toyota-rentacar-pp-cli booking handoff --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --agent
  ```

## Cookbook


### Resolve shops

```bash
toyota-rentacar-pp-cli shops search --keyword "Tokyo Station" --limit 3 --agent --select shops.id,shops.name,shops.name_jp,shops.oneway_returns
```

Keep branch identities and return restrictions.

### Dated compact offers

```bash
toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent
```

Class prices are source estimates; models are representative.

### One-way surcharge

```bash
toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent
```

Calculator fee has no dated inventory guarantee.

### Eligibility guidance

```bash
toyota-rentacar-pp-cli rental eligibility --agent
```

Read first-party document rules;Toyota verifies the real documents.

## Recipes

### Resolve shops

```bash
toyota-rentacar-pp-cli shops search --keyword "Tokyo Station" --limit 3 --agent --select shops.id,shops.name,shops.name_jp,shops.oneway_returns
```

Keep branch identities and return restrictions.

### Dated compact offers

```bash
toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent
```

Class prices are source estimates; models are representative.

### One-way surcharge

```bash
toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent
```

Calculator fee has no dated inventory guarantee.

### Eligibility guidance

```bash
toyota-rentacar-pp-cli rental eligibility --agent
```

Read first-party document rules;Toyota verifies the real documents.

## Usage

Run `toyota-rentacar-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TOYOTA_RENTACAR_CONFIG_DIR`, `TOYOTA_RENTACAR_DATA_DIR`, `TOYOTA_RENTACAR_STATE_DIR`, or `TOYOTA_RENTACAR_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TOYOTA_RENTACAR_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TOYOTA_RENTACAR_HOME=/srv/toyota-rentacar
toyota-rentacar-pp-cli doctor
```

Under `TOYOTA_RENTACAR_HOME=/srv/toyota-rentacar`, the four dirs resolve to `/srv/toyota-rentacar/config`, `/srv/toyota-rentacar/data`, `/srv/toyota-rentacar/state`, and `/srv/toyota-rentacar/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "toyota-rentacar": {
      "command": "toyota-rentacar-pp-mcp",
      "env": {
        "TOYOTA_RENTACAR_HOME": "/srv/toyota-rentacar"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TOYOTA_RENTACAR_DATA_DIR` overrides an explicit `--home` for that kind. Use `TOYOTA_RENTACAR_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TOYOTA_RENTACAR_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `toyota-rentacar-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### source

Public first-party source documents; use shops/cars/rental/oneway for structured planning

- **`toyota-rentacar-pp-cli source eligibility`** - Read official driving-document guidance
- **`toyota-rentacar-pp-cli source insurance`** - Read insurance and NOC policy source
- **`toyota-rentacar-pp-cli source locations`** - Read public keyword shop document
- **`toyota-rentacar-pp-cli source one-way`** - Read one-way restrictions and simulator link
- **`toyota-rentacar-pp-cli source options`** - Read option policy source


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`toyota-rentacar-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`toyota-rentacar-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`toyota-rentacar-pp-cli learnings list`** - Inspect taught rows
- **`toyota-rentacar-pp-cli learnings forget <query>`** - Undo a teach
- **`toyota-rentacar-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`toyota-rentacar-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`toyota-rentacar-pp-cli teach-pattern`** - Install a query/resource template up front
- **`toyota-rentacar-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TOYOTA_RENTACAR_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `toyota-rentacar-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
toyota-rentacar-pp-cli source eligibility

# JSON for scripting and agents
toyota-rentacar-pp-cli source eligibility --json
# Filter to specific fields by name
toyota-rentacar-pp-cli source eligibility --json --select <field>[,<field>...]

# Dry run — show the request without sending
toyota-rentacar-pp-cli source eligibility --dry-run

# Agent mode — JSON + compact + no prompts in one flag
toyota-rentacar-pp-cli source eligibility --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
toyota-rentacar-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `toyota-rentacar-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `TOYOTA_RENTACAR_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Toyota returns a source error or changed rental dates.** — Choose dates within three months and a return after pickup,then retry using the canonical booking URL in the error.
- **No fully confirmed total.** — Open booking handoff and verify all extras,one-way fees and final total on Toyota before payment.
