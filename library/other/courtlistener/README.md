# Courtlistener CLI

Search case law and query authenticated RECAP litigation records.

Search case law publicly and, with a CourtListener token, build bounded docket briefs, relationship maps, filing watches, and RECAP availability audits.

## Install

The recommended path installs both the `courtlistener-pp-cli` binary and the `pp-courtlistener` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install courtlistener
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install courtlistener --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install courtlistener --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install courtlistener --agent claude-code
npx -y @mvanhorn/printing-press-library install courtlistener --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/courtlistener/cmd/courtlistener-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/courtlistener-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install courtlistener --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-courtlistener --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-courtlistener --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install courtlistener --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/courtlistener-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COURTLISTENER_TOKEN_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/courtlistener/cmd/courtlistener-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "courtlistener": {
      "command": "courtlistener-pp-mcp",
      "env": {
        "COURTLISTENER_TOKEN_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Run a bounded public case-law search.
courtlistener-pp-cli legal-search --q antitrust --type r --page-size 5 --agent

# Create or compare a bounded public search observation.
courtlistener-pp-cli new-filings --query antitrust --type r --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`docket`** — Join one authenticated docket with bounded entries, documents, parties, and counsel in source chronology.
- **`new-filings`** — Persist a bounded newest-first search observation and report newly observed CourtListener result IDs.
- **`party`** — Query authenticated party records by exact supplied name while preserving docket and API identifiers.
- **`counsel`** — Query authenticated attorney records by supplied name and retain observed docket and party relationships.
- **`judge`** — Return sourced judge/person metadata and clearly prohibit outcome prediction or causal scoring.
- **`recap-gaps`** — Classify bounded docket document records by CourtListener availability fields without implying complete PACER coverage.

## Recipes

### Audit RECAP availability

```bash
courtlistener-pp-cli recap-gaps 12345 --agent
```

Separate available files from metadata-only or unavailable document records.

## Usage

Run `courtlistener-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COURTLISTENER_CONFIG_DIR`, `COURTLISTENER_DATA_DIR`, `COURTLISTENER_STATE_DIR`, or `COURTLISTENER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COURTLISTENER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COURTLISTENER_HOME=/srv/courtlistener
courtlistener-pp-cli doctor
```

Under `COURTLISTENER_HOME=/srv/courtlistener`, the four dirs resolve to `/srv/courtlistener/config`, `/srv/courtlistener/data`, `/srv/courtlistener/state`, and `/srv/courtlistener/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "courtlistener": {
      "command": "courtlistener-pp-mcp",
      "env": {
        "COURTLISTENER_HOME": "/srv/courtlistener"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COURTLISTENER_DATA_DIR` overrides an explicit `--home` for that kind. Use `COURTLISTENER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COURTLISTENER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `courtlistener-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### attorneys

Manage attorneys

- **`courtlistener-pp-cli attorneys`** - List

### docket-entries

Manage docket entries

- **`courtlistener-pp-cli docket-entries`** - List

### dockets

Manage dockets

- **`courtlistener-pp-cli dockets <id>`** - Get

### legal_search

Manage legal search

- **`courtlistener-pp-cli legal-search`** - Search opinions and RECAP records

### parties

Manage parties

- **`courtlistener-pp-cli parties`** - List

### people

Manage people

- **`courtlistener-pp-cli people <id>`** - Get judge

### recap-documents

Manage recap documents

- **`courtlistener-pp-cli recap-documents`** - List


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`courtlistener-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`courtlistener-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`courtlistener-pp-cli learnings list`** - Inspect taught rows
- **`courtlistener-pp-cli learnings forget <query>`** - Undo a teach
- **`courtlistener-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`courtlistener-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`courtlistener-pp-cli teach-pattern`** - Install a query/resource template up front
- **`courtlistener-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `COURTLISTENER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `courtlistener-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
courtlistener-pp-cli attorneys

# JSON for scripting and agents
courtlistener-pp-cli attorneys --json

# Filter to specific fields
courtlistener-pp-cli attorneys --json --select id,name,status

# Dry run — show the request without sending
courtlistener-pp-cli attorneys --dry-run

# Agent mode — JSON + compact + no prompts in one flag
courtlistener-pp-cli attorneys --agent
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
courtlistener-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `courtlistener-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/courtlistener-pp-cli/config.toml`; `--home`, `COURTLISTENER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COURTLISTENER_TOKEN_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `courtlistener-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `courtlistener-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COURTLISTENER_TOKEN_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Authentication credentials were not provided** — 
