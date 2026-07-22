# Faa Registry CLI

CLI for the FAA Civil Aviation Registry. Look up any US-registered aircraft by
N-number, serial number, owner name, make/model, engine, dealer, or state/county
via the live registry.faa.gov inquiry app — and sync the FAA's daily Releasable
Aircraft Database (all ~315K active registrations, ~383K deregistered records,
~126K reserved N-numbers, plus aircraft-model and engine reference data) into a
local SQLite store for instant offline search, fleet reports, Mode S hex
decoding, and expiring-registration alerts.

Learn more at [Faa Registry](https://registry.faa.gov/aircraftinquiry/).

Created by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).

## Install

The recommended path installs both the `faa-registry-pp-cli` binary and the `pp-faa-registry` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install faa-registry
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install faa-registry --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install faa-registry --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install faa-registry --agent claude-code
npx -y @mvanhorn/printing-press-library install faa-registry --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/faa-registry/cmd/faa-registry-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/faa-registry-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install faa-registry --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-faa-registry --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-faa-registry --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install faa-registry --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/faa-registry-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/faa-registry/cmd/faa-registry-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "faa-registry": {
      "command": "faa-registry-pp-mcp"
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
faa-registry-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
faa-registry-pp-cli dealers --name example-value
```

## Usage

Run `faa-registry-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FAA_REGISTRY_CONFIG_DIR`, `FAA_REGISTRY_DATA_DIR`, `FAA_REGISTRY_STATE_DIR`, or `FAA_REGISTRY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FAA_REGISTRY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FAA_REGISTRY_HOME=/srv/faa-registry
faa-registry-pp-cli doctor
```

Under `FAA_REGISTRY_HOME=/srv/faa-registry`, the four dirs resolve to `/srv/faa-registry/config`, `/srv/faa-registry/data`, `/srv/faa-registry/state`, and `/srv/faa-registry/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "faa-registry": {
      "command": "faa-registry-pp-mcp",
      "env": {
        "FAA_REGISTRY_HOME": "/srv/faa-registry"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FAA_REGISTRY_DATA_DIR` overrides an explicit `--home` for that kind. Use `FAA_REGISTRY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FAA_REGISTRY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `faa-registry-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### aircraft

Live FAA registry lookups for individual aircraft (registration detail pages).

- **`faa-registry-pp-cli aircraft by-serial`** - Find aircraft by manufacturer serial number.
- **`faa-registry-pp-cli aircraft lookup`** - Look up an aircraft's full registration record by N-number (tail number, with or without the leading N).

### dealers

Live dealer-certificate searches.

- **`faa-registry-pp-cli dealers`** - Search FAA dealer certificates by dealer name.

### documents

Live document-index searches (recorded documents for collateral like airframes and engines).

- **`faa-registry-pp-cli documents`** - Search the FAA document index by collateral identifier.

### engines

Live engine-reference searches.

- **`faa-registry-pp-cli engines`** - Search the engine reference table by engine manufacturer and model.

### models

Live registry searches by aircraft make/model and reference data.

- **`faa-registry-pp-cli models`** - Search the aircraft model reference by manufacturer and model name, including the number of aircraft assigned to each model code.

### owners

Live registry searches by registered owner name.

- **`faa-registry-pp-cli owners`** - List all aircraft registered to an owner name (paginated).

### regions

Live registry searches by geography.

- **`faa-registry-pp-cli regions by-country`** - List US-registered aircraft whose owners are located in a given country.
- **`faa-registry-pp-cli regions by-state`** - List aircraft registered in a state and county (paginated).


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`faa-registry-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`faa-registry-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`faa-registry-pp-cli learnings list`** - Inspect taught rows
- **`faa-registry-pp-cli learnings forget <query>`** - Undo a teach
- **`faa-registry-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`faa-registry-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`faa-registry-pp-cli teach-pattern`** - Install a query/resource template up front
- **`faa-registry-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FAA_REGISTRY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `faa-registry-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
faa-registry-pp-cli dealers --name example-value

# JSON for scripting and agents
faa-registry-pp-cli dealers --name example-value --json

# Filter to specific fields
faa-registry-pp-cli dealers --name example-value --json --select id,name,status

# Dry run — show the request without sending
faa-registry-pp-cli dealers --name example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
faa-registry-pp-cli dealers --name example-value --agent
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
faa-registry-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `faa-registry-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/faa-registry-pp-cli/config.toml`; `--home`, `FAA_REGISTRY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
