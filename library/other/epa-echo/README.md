# Epa Echo CLI

Resolve EPA-regulated facilities, inspect transparent compliance evidence, and retain change history without opaque scores.

Resolve EPA-regulated facilities, inspect transparent compliance evidence, and retain change history without creating opaque risk scores.

## Install

The recommended path installs both the `epa-echo-pp-cli` binary and the `pp-epa-echo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install epa-echo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install epa-echo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install epa-echo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install epa-echo --agent claude-code
npx -y @mvanhorn/printing-press-library install epa-echo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/epa-echo/cmd/epa-echo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/epa-echo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install epa-echo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-epa-echo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-epa-echo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install epa-echo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/epa-echo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/epa-echo/cmd/epa-echo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "epa-echo": {
      "command": "epa-echo-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Resolve a name to source-preserving FRS matches.
epa-echo-pp-cli resolve facility --name 'Exxon' --state TX --agent

# Create or compare a detailed facility snapshot.
epa-echo-pp-cli dossier diff 110009441979 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`resolve facility`** — Rank bounded facility matches while preserving FRS identifiers, addresses, program status fields, and query provenance.
- **`dossier diff`** — Compare a live detailed facility report with the prior local snapshot and list changed ECHO sections without assigning a score.
- **`watch`** — Fetch a bounded CSV portfolio of facility IDs and atomically report changed detailed-report sections since the prior run.
- **`nearby explain`** — List facilities within an EPA-supported radius and expose the compliance, inspection, action, and penalty fields behind each concern.
- **`effluent trend`** — Return ECHO's reported Clean Water Act effluent compliance and exceedance records for one stable facility identifier.
- **`enforcement timeline`** — Collect dated inspection, violation, formal-action, and enforcement sections from a detailed facility report without causal inference.

## Recipes

### Explain nearby facilities

```bash
epa-echo-pp-cli nearby explain --latitude 41.83 --longitude -71.42 --radius 5 --agent
```

Return bounded nearby facilities with the ECHO fields driving each concern flag.

### Order enforcement evidence

```bash
epa-echo-pp-cli enforcement timeline 110009441979 --agent
```

Keep reported actions and penalties attached to their source sections.

## Usage

Run `epa-echo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EPA_ECHO_CONFIG_DIR`, `EPA_ECHO_DATA_DIR`, `EPA_ECHO_STATE_DIR`, or `EPA_ECHO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EPA_ECHO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EPA_ECHO_HOME=/srv/epa-echo
epa-echo-pp-cli doctor
```

Under `EPA_ECHO_HOME=/srv/epa-echo`, the four dirs resolve to `/srv/epa-echo/config`, `/srv/epa-echo/data`, `/srv/epa-echo/state`, and `/srv/epa-echo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "epa-echo": {
      "command": "epa-echo-pp-mcp",
      "env": {
        "EPA_ECHO_HOME": "/srv/epa-echo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EPA_ECHO_DATA_DIR` overrides an explicit `--home` for that kind. Use `EPA_ECHO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EPA_ECHO_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `epa-echo-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### dfr-rest-services-get-dfr

Manage dfr rest services get dfr

- **`epa-echo-pp-cli dfr-rest-services-get-dfr`** - Get a detailed multi-program facility report

### echo-rest-services-get-facilities

Manage echo rest services get facilities

- **`epa-echo-pp-cli echo-rest-services-get-facilities`** - Start an all-media facility search

### echo-rest-services-get-qid

Manage echo rest services get qid

- **`epa-echo-pp-cli echo-rest-services-get-qid`** - Retrieve a page of facility results by query ID


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`epa-echo-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`epa-echo-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`epa-echo-pp-cli learnings list`** - Inspect taught rows
- **`epa-echo-pp-cli learnings forget <query>`** - Undo a teach
- **`epa-echo-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`epa-echo-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`epa-echo-pp-cli teach-pattern`** - Install a query/resource template up front
- **`epa-echo-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EPA_ECHO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `epa-echo-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
epa-echo-pp-cli dfr-rest-services-get-dfr --p-id 110009441979

# JSON for scripting and agents
epa-echo-pp-cli dfr-rest-services-get-dfr --p-id 110009441979 --json

# Filter to specific fields
epa-echo-pp-cli dfr-rest-services-get-dfr --p-id 110009441979 --json --select FacilitySummary

# Dry run — show the request without sending
epa-echo-pp-cli dfr-rest-services-get-dfr --p-id 110009441979 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
epa-echo-pp-cli dfr-rest-services-get-dfr --p-id 110009441979 --agent
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
epa-echo-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `epa-echo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/epa-echo-pp-cli/config.toml`; `--home`, `EPA_ECHO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Many facilities match** — 
