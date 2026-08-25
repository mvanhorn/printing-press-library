# Ashby CLI

**Search, filter, sync, and inspect public Ashby job postings without an API key.**

Turn any known Ashby job-board name into structured, agent-friendly job data while enforcing Ashby's public-listing boundary.

## Install

The recommended path installs both the `ashby-pp-cli` binary and the `pp-ashby` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ashby
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ashby --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ashby --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ashby --agent claude-code
npx -y @mvanhorn/printing-press-library install ashby --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/job-boards/ashby/cmd/ashby-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ashby-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ashby --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ashby --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ashby --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ashby --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ashby-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/job-boards/ashby/cmd/ashby-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ashby": {
      "command": "ashby-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No Ashby API key or customer account is required. Authenticated recruiting, candidate, and application-management APIs are intentionally out of scope.

## Quick Start

```bash
# List publicly visible jobs from Ashby's board
ashby-pp-cli postings list ashby

# Return a compact agent-friendly shortlist
ashby-pp-cli postings list ashby --remote --department Engineering --agent --select id,title,location,department

# Create a local mirror and search it offline
ashby-pp-cli sync ashby && ashby-pp-cli search engineer --board ashby

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Public job intelligence
- **`postings list`** — Lists only publicly listed Ashby jobs and supports structured filtering across department, team, location, workplace type, employment type, publication date, currency, and compensation range.

  _Agents and job seekers get keyless machine-readable discovery without surfacing an unlisted posting returned upstream._

  ```bash
  ashby-pp-cli postings list ashby --remote --department Engineering --agent --select id,title,location,department
  ```

## Recipes

### Find remote engineering roles

```bash
ashby-pp-cli postings list ashby --remote --department Engineering
```

Uses structured remote and department filters.

### Find published compensation

```bash
ashby-pp-cli postings list ashby --include-compensation --has-compensation --currency USD --salary-min 150000
```

Filters published structured compensation.

### Build a local index

```bash
ashby-pp-cli sync ashby --include-compensation
```

Mirrors a known public board into SQLite.

### Search a synced board

```bash
ashby-pp-cli search product --board ashby --limit 20
```

Queries the local full-text index.

## Usage

Run `ashby-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ASHBY_CONFIG_DIR`, `ASHBY_DATA_DIR`, `ASHBY_STATE_DIR`, or `ASHBY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ASHBY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ASHBY_HOME=/srv/ashby
ashby-pp-cli doctor
```

Under `ASHBY_HOME=/srv/ashby`, the four dirs resolve to `/srv/ashby/config`, `/srv/ashby/data`, `/srv/ashby/state`, and `/srv/ashby/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ashby": {
      "command": "ashby-pp-mcp",
      "env": {
        "ASHBY_HOME": "/srv/ashby"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ASHBY_DATA_DIR` overrides an explicit `--home` for that kind. Use `ASHBY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ASHBY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ashby-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### postings

Browse public Ashby job boards. General discovery always excludes postings where `isListed` is false.

- **`ashby-pp-cli postings list <jobBoardName>`** - List publicly listed jobs with structured filters.
- **`ashby-pp-cli postings get <jobBoardName> <postingID>`** - Retrieve one publicly listed posting.
- **`ashby-pp-cli sync <jobBoardName>...`** - Synchronize selected boards into local SQLite.
- **`ashby-pp-cli search <query>`** - Full-text search synchronized postings.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ashby-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ashby-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ashby-pp-cli learnings list`** - Inspect taught rows
- **`ashby-pp-cli learnings forget <query>`** - Undo a teach
- **`ashby-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ashby-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ashby-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ashby-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ASHBY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ashby-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ashby-pp-cli postings list ashby

# JSON for scripting and agents
ashby-pp-cli postings list ashby --json
# Filter to specific fields
ashby-pp-cli postings list ashby --json --select id,title,location

# Dry run — show the request without sending
ashby-pp-cli postings list ashby --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ashby-pp-cli postings list ashby --agent
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
ashby-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ashby-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ashby-pp-cli/config.toml`; `--home`, `ASHBY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **The board returns not found or no jobs** — Use the board slug from jobs.ashbyhq.com/<slug>, not the display name or full URL.
- **Compensation filters return no matches** — Add --include-compensation and confirm the board publishes structured compensation.
