# ACRIS CLI

**Query NYC property records — deeds, mortgages, parties, and full BBL history — straight from NYC Open Data, with cross-dataset joins no raw ACRIS query can do.**

ACRIS publishes its property records as four separate NYC Open Data datasets (Master, Legals, Parties, Document Control Codes). This CLI gives you direct, scriptable access to each, plus joined commands — `bbl`, `debt`, `document`, `party-search` — that chain the datasets the way a title search actually needs.

## Install

The recommended path installs both the `acris-pp-cli` binary and the `pp-acris` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install acris
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install acris --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install acris --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install acris --agent claude-code
npx -y @mvanhorn/printing-press-library install acris --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/acris/cmd/acris-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/acris-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install acris --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-acris --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-acris --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install acris --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/acris-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ACRIS_APP_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/acris/cmd/acris-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "acris": {
      "command": "acris-pp-mcp",
      "env": {
        "ACRIS_APP_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is wired and NYC Open Data is reachable (no credential required).
acris-pp-cli doctor --dry-run

# See the mortgage document type codes you can filter on.
acris-pp-cli doc-types --class-code-description "MORTGAGES & INSTRUMENTS" --limit 10

# Resolve a BBL to its recorded documents.
acris-pp-cli legals --borough 1 --block 852 --lot 134

# Get the full recorded-document history for that BBL in one call.
acris-pp-cli bbl --borough 1 --block 852 --lot 134 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-dataset joins
- **`bbl`** — Resolve a borough/block/lot to its full recorded-document history in one call.

  _When an agent has an address or BBL and needs the chain of title and recordings, this is the single command that returns it._

  ```bash
  acris-pp-cli bbl --borough 1 --block 852 --lot 134 --json
  ```
- **`debt`** — List the mortgage and debt-instrument recordings for a BBL with amounts and dates.

  _Surfaces apparent debt attached to a property without manually cross-referencing three datasets._

  ```bash
  acris-pp-cli debt --borough 1 --block 852 --lot 134 --json
  ```
- **`document`** — Assemble one document's master record, all its property (BBL) legals, and all its parties into a single object.

  _Gives an agent the complete picture of a single recording in one call instead of three._

  ```bash
  acris-pp-cli document 2023012300123001 --json
  ```

### Entity discovery
- **`party-search`** — Find recorded documents by partial party name (grantor, grantee, mortgagor, mortgagee).

  _Entity-driven discovery: find every document an LLC or individual is named on without knowing the exact recorded spelling._

  ```bash
  acris-pp-cli party-search --name "MADISON" --limit 25 --json
  ```

## Recipes

### Mortgage history for a property

```bash
acris-pp-cli debt --borough 1 --block 852 --lot 134 --json
```

Lists mortgage and debt-instrument recordings for the BBL with amounts and dates.

### Find documents naming a party

```bash
acris-pp-cli party-search --name "MADISON" --limit 25 --json
```

Substring search over recorded party names; returns the documents they appear on.

### Recent high-value deeds in Manhattan

```bash
acris-pp-cli documents list --doc-type DEED --recorded-borough 1 --where "document_amt > 10000000" --order "recorded_datetime DESC" --json --select document_id,document_amt,recorded_datetime
```

Combines a server-side SoQL filter with --select to keep the payload small for agents.

### Full picture of one recording

```bash
acris-pp-cli document 2023012300123001 --json
```

Merges the master record, all property legals (BBLs), and all parties for a single document.

## Usage

Run `acris-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ACRIS_CONFIG_DIR`, `ACRIS_DATA_DIR`, `ACRIS_STATE_DIR`, or `ACRIS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ACRIS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ACRIS_HOME=/srv/acris
acris-pp-cli doctor
```

Under `ACRIS_HOME=/srv/acris`, the four dirs resolve to `/srv/acris/config`, `/srv/acris/data`, `/srv/acris/state`, and `/srv/acris/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "acris": {
      "command": "acris-pp-mcp",
      "env": {
        "ACRIS_HOME": "/srv/acris"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ACRIS_DATA_DIR` overrides an explicit `--home` for that kind. Use `ACRIS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ACRIS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `acris-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### doc-types

Document type code lookup — translate ACRIS doc_type codes to descriptions and classes (ACRIS Document Control Codes, 7isb-wh4c)

- **`acris-pp-cli doc-types`** - List document control codes, optionally filtered by class

### documents

Recorded document master records — deeds, mortgages, assignments, satisfactions (ACRIS Real Property Master, bnx9-e6tj)

- **`acris-pp-cli documents get`** - Fetch a single document master record by document ID
- **`acris-pp-cli documents list`** - List or filter recorded documents by type, borough, CRFN, or SoQL query

### legals

Document-to-property (borough/block/lot) mappings — resolve a BBL to its recorded documents (ACRIS Real Property Legals, 8h5j-fqxa)

- **`acris-pp-cli legals`** - List property legal records by BBL, address, or document ID

### parties

Parties (grantors, grantees, mortgagors, mortgagees) named on recorded documents (ACRIS Real Property Parties, 636b-3b5g)

- **`acris-pp-cli parties`** - List parties by document ID, exact name, or party type


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
acris-pp-cli doc-types

# JSON for scripting and agents
acris-pp-cli doc-types --json

# Filter to specific fields
acris-pp-cli doc-types --json --select id,name,status

# Dry run — show the request without sending
acris-pp-cli doc-types --dry-run

# Agent mode — JSON + compact + no prompts in one flag
acris-pp-cli doc-types --agent
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
acris-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `acris-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/acris-pp-cli/config.toml`; `--home`, `ACRIS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ACRIS_APP_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `acris-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `acris-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ACRIS_APP_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 or throttling errors on rapid queries** — Set an app token: export ACRIS_APP_TOKEN=<token from data.cityofnewyork.us>. Anonymous access is rate-limited.
- **Empty results for a known property** — Block and lot are not zero-padded in ACRIS (use 852 and 134, not 00852/0134); confirm the borough code (1=Manhattan ... 5=Staten Island).
