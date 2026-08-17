# Cfpb Complaints CLI

Search published CFPB consumer complaint records and aggregations.

Build reproducible complaint cohorts, company pulses, peer comparisons, emerging-theme tables, and narrative evidence packets without presenting raw complaint counts as quality scores.

## Install

The recommended path installs both the `cfpb-complaints-pp-cli` binary and the `pp-cfpb-complaints` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints --agent claude-code
npx -y @mvanhorn/printing-press-library install cfpb-complaints --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/cmd/cfpb-complaints-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cfpb-complaints-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cfpb-complaints --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cfpb-complaints --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cfpb-complaints --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cfpb-complaints-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/cmd/cfpb-complaints-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cfpb-complaints": {
      "command": "cfpb-complaints-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Inspect a bounded published complaint cohort.
cfpb-complaints-pp-cli data-research --size 10 --state NY --agent

# Summarize one company's recent complaint mix.
cfpb-complaints-pp-cli company pulse --company 'CAPITAL ONE FINANCIAL CORPORATION' --window 30d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`company pulse`** — Summarize complaint volume, products, issues, responses, timeliness, narrative availability, and a prior-window delta for one company.
- **`compare companies`** — Compare companies inside one explicit cohort while labeling counts as non-market-adjusted complaint volume.
- **`emerging themes`** — Rank mechanical product and issue count changes between current and baseline windows without semantic or causal claims.
- **`narratives packet`** — Select a newest-first bounded packet of published narratives with complaint IDs, exact cohort dates, and availability caveats.
- **`watch changes`** — Compare the latest bounded cohort with a locally persisted prior observation and report newly observed complaint IDs and categories.

## Recipes

### Compare peers in one cohort

```bash
cfpb-complaints-pp-cli compare companies 'CAPITAL ONE FINANCIAL CORPORATION' 'DISCOVER BANK' --product 'Credit card' --window 90d --agent
```

Use identical filters and keep raw-volume caveats attached.

### Collect traceable narratives

```bash
cfpb-complaints-pp-cli narratives packet --company 'CAPITAL ONE FINANCIAL CORPORATION' --limit 10 --agent
```

Return complaint IDs with published narrative evidence.

## Usage

Run `cfpb-complaints-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CFPB_COMPLAINTS_CONFIG_DIR`, `CFPB_COMPLAINTS_DATA_DIR`, `CFPB_COMPLAINTS_STATE_DIR`, or `CFPB_COMPLAINTS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CFPB_COMPLAINTS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CFPB_COMPLAINTS_HOME=/srv/cfpb-complaints
cfpb-complaints-pp-cli doctor
```

Under `CFPB_COMPLAINTS_HOME=/srv/cfpb-complaints`, the four dirs resolve to `/srv/cfpb-complaints/config`, `/srv/cfpb-complaints/data`, `/srv/cfpb-complaints/state`, and `/srv/cfpb-complaints/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cfpb-complaints": {
      "command": "cfpb-complaints-pp-mcp",
      "env": {
        "CFPB_COMPLAINTS_HOME": "/srv/cfpb-complaints"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CFPB_COMPLAINTS_DATA_DIR` overrides an explicit `--home` for that kind. Use `CFPB_COMPLAINTS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CFPB_COMPLAINTS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `cfpb-complaints-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### data-research

Manage data research

- **`cfpb-complaints-pp-cli data-research`** - Search published consumer complaints


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`cfpb-complaints-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`cfpb-complaints-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`cfpb-complaints-pp-cli learnings list`** - Inspect taught rows
- **`cfpb-complaints-pp-cli learnings forget <query>`** - Undo a teach
- **`cfpb-complaints-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`cfpb-complaints-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`cfpb-complaints-pp-cli teach-pattern`** - Install a query/resource template up front
- **`cfpb-complaints-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CFPB_COMPLAINTS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `cfpb-complaints-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cfpb-complaints-pp-cli data-research

# JSON for scripting and agents
cfpb-complaints-pp-cli data-research --json

# Filter to specific fields
cfpb-complaints-pp-cli data-research --json --select id,name,status

# Dry run — show the request without sending
cfpb-complaints-pp-cli data-research --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cfpb-complaints-pp-cli data-research --agent
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
cfpb-complaints-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `cfpb-complaints-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cfpb-consumer-complaints-pp-cli/config.toml`; `--home`, `CFPB_COMPLAINTS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A cohort is empty** — 
- **Few narratives appear** — 
