# Cpsc Recalls CLI

Query US Consumer Product Safety Commission recall records.

Turn CPSC recall records into bounded inventory candidates, change observations, hazard composition tables, and actionable source packets without overstating fuzzy matches.

## Install

The recommended path installs both the `cpsc-recalls-pp-cli` binary and the `pp-cpsc-recalls` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls --agent claude-code
npx -y @mvanhorn/printing-press-library install cpsc-recalls --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/cpsc-recalls/cmd/cpsc-recalls-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cpsc-recalls-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cpsc-recalls --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cpsc-recalls --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cpsc-recalls --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cpsc-recalls-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/cpsc-recalls/cmd/cpsc-recalls-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cpsc-recalls": {
      "command": "cpsc-recalls-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Inspect one official recall record.
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall --recall-id 10000 --agent --dry-run

# Build an actionable remedy and contact packet.
cpsc-recalls-pp-cli packet 10000 --agent --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`inventory-check`** — Screen a bounded CSV inventory against CPSC product fields and expose exact identifiers and token-overlap evidence for every candidate.
- **`watch changes`** — Compare a bounded brand or product recall observation with its prior local snapshot and report new or materially changed records.
- **`hazard-pulse`** — Flatten recent hazard, remedy, injury, and product relationships while keeping recall counts distinct from incident rates.
- **`packet`** — Join one recall's affected products, remedy, contact, images, incidents, and official URL into an action packet.

## Recipes

### Screen an inventory

```bash
cpsc-recalls-pp-cli inventory-check --inventory products.csv --agent --dry-run
```

Return candidate matches with exact fields and token-overlap evidence.

### Review recent hazards

```bash
cpsc-recalls-pp-cli hazard-pulse --window 30d --agent --dry-run
```

Describe recent recall composition without rate claims.

## Usage

Run `cpsc-recalls-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CPSC_RECALLS_CONFIG_DIR`, `CPSC_RECALLS_DATA_DIR`, `CPSC_RECALLS_STATE_DIR`, or `CPSC_RECALLS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CPSC_RECALLS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CPSC_RECALLS_HOME=/srv/cpsc-recalls
cpsc-recalls-pp-cli doctor
```

Under `CPSC_RECALLS_HOME=/srv/cpsc-recalls`, the four dirs resolve to `/srv/cpsc-recalls/config`, `/srv/cpsc-recalls/data`, `/srv/cpsc-recalls/state`, and `/srv/cpsc-recalls/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cpsc-recalls": {
      "command": "cpsc-recalls-pp-mcp",
      "env": {
        "CPSC_RECALLS_HOME": "/srv/cpsc-recalls"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CPSC_RECALLS_DATA_DIR` overrides an explicit `--home` for that kind. Use `CPSC_RECALLS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CPSC_RECALLS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `cpsc-recalls-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### cpsc-recall-retrieval-recall

Manage cpsc recall retrieval recall

- **`cpsc-recalls-pp-cli cpsc-recall-retrieval-recall`** - Query CPSC recall records


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`cpsc-recalls-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`cpsc-recalls-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`cpsc-recalls-pp-cli learnings list`** - Inspect taught rows
- **`cpsc-recalls-pp-cli learnings forget <query>`** - Undo a teach
- **`cpsc-recalls-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`cpsc-recalls-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`cpsc-recalls-pp-cli teach-pattern`** - Install a query/resource template up front
- **`cpsc-recalls-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CPSC_RECALLS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `cpsc-recalls-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall

# JSON for scripting and agents
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall --json

# Filter to specific fields
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall --json --select id,name,status

# Dry run — show the request without sending
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cpsc-recalls-pp-cli cpsc-recall-retrieval-recall --agent
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
cpsc-recalls-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `cpsc-recalls-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cpsc-recall-retrieval-pp-cli/config.toml`; `--home`, `CPSC_RECALLS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **An inventory candidate is ambiguous** — 
