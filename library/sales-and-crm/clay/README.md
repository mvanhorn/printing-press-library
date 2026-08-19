# Clay CLI

**The only Clay tool that can build a table, not just read one.**

Clay's own CLI and MCP stop at reading tables that already exist. This CLI creates tables, adds text, formula, enrichment, and HTTP API columns, and generates formulas from plain language. It also turns a table's column graph into portable JSON with blueprint export, so a proven lead-gen table becomes a file you can commit, diff, and rebuild in a new market.

Learn more at [Clay](https://api.clay.com).

Created by [@adeamos83](https://github.com/adeamos83) (Ade Amos).

## Install

The recommended path installs both the `clay-pp-cli` binary and the `pp-clay` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clay
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clay --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clay --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install clay --agent claude-code
npx -y @mvanhorn/printing-press-library install clay --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/cmd/clay-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clay-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clay --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clay --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clay --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clay --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
clay-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clay-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/cmd/clay-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "clay": {
      "command": "clay-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Clay has two separate credentials and this CLI uses both. Table and column authoring runs against Clay's app API and authenticates with your browser session cookie, so run auth login --chrome while signed in to app.clay.com. The public subcommands use a Public API key from Settings, Account, API keys (beta); set it as CLAY_API_KEY. Neither credential works on the other surface.

## Quick Start

```bash
# confirm both credentials and API reachability before anything else
clay-pp-cli doctor --dry-run

# import the app.clay.com session cookie that authorizes table authoring
clay-pp-cli auth login --chrome

# confirm the workspace id you will pass to authoring commands
clay-pp-cli workspace get 1234567

# create the table you are about to build columns on
clay-pp-cli tables create 1234567 --workbook-id wb_abc --template basic

# add the first column the enrichments will bind to
clay-pp-cli columns create 1234567 t_abc --type text --name Company

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Design as code
- **`blueprint export`** — Snapshot a table's entire column graph to portable JSON you can commit to git.

  _Reach for this when a table design is worth keeping or reviewing; it is the only way to get Clay column configuration into version control._

  ```bash
  clay-pp-cli blueprint export t_abc123 --workspace 1234567 --agent
  ```
- **`blueprint apply`** — Rebuild a captured table design in a new workbook, remapping formula references automatically.

  _Reach for this to clone a proven lead-gen table into a new market without rebuilding columns by hand._

  ```bash
  clay-pp-cli blueprint apply ./plumbing-austin.json --workbook wb_abc --name 'Plumbing Dallas' --workspace 1234567
  ```
- **`columns link`** — Connect two tables by creating a lookup column that pulls the matching row from another table on a join key.

  _Reach for this to wire one table's data into another; it is the scriptable form of Clay's Lookup enrichment._

  ```bash
  clay-pp-cli columns link t_prospects --from t_citycache --on City --workspace 1234567
  ```
- **`columns set-formula`** — Read a column's current formula with references resolved to column names, edit it, and write it back.

  _Reach for this to change a formula without deleting and recreating the column._

  ```bash
  clay-pp-cli columns set-formula t_abc f_xyz --formula "{{Company}}" --workspace 1234567
  ```
- **`formulas generate`** — Turn a natural-language prompt into a Clay formula that references the target table's real columns.

  _Reach for this to draft a formula against a specific table, then apply it with 'columns set-formula'._

  ```bash
  clay-pp-cli formulas generate --table t_example123 --prompt "combine business name and city" --agent
  ```

### Table intelligence
- **`columns graph`** — Show which columns feed which, resolved from formula field references.

  _Reach for this before deleting or renaming a column, to see what breaks downstream._

  ```bash
  clay-pp-cli columns graph t_abc123 --workspace 1234567 --agent
  ```
- **`columns doctor`** — Find formulas pointing at deleted columns and columns nothing consumes.

  _Reach for this before a large enrichment run so broken formulas do not burn credits._

  ```bash
  clay-pp-cli columns doctor t_abc123 --workspace 1234567 --agent
  ```
- **`enrichments compare`** — Rank enrichment providers for a job and flag which ones your workspace can actually run today.

  _Reach for this when choosing between providers; it is advisory and never spends credits._

  ```bash
  clay-pp-cli enrichments compare "find work email" --workspace 1234567 --agent
  ```
- **`tables diff`** — Structurally compare two tables' column graphs, including formulas and enrichment bindings.

  _Reach for this to see what drifted between a template table and a copy._

  ```bash
  clay-pp-cli tables diff t_abc123 t_def456 --workspace 1234567 --agent
  ```
- **`workbooks graph`** — Show how every table and CSV source in a workbook feeds the others, with each table's credit estimate.

  _Reach for this to understand an unfamiliar workbook before changing anything in it._

  ```bash
  clay-pp-cli workbooks graph wb_abc123 --workspace 1234567 --agent
  ```
- **`errors`** — Report why columns failed, combining Clay's structured validation errors with per-column run status counts.

  _Reach for this when a column is empty and you need to know whether the request was rejected or the run failed._

  ```bash
  clay-pp-cli errors t_abc123 --workspace 1234567 --agent
  ```
- **`tables rows`** — Read table rows with generated field ids resolved to real column names, optionally filtered by cell run status.

  _Reach for this to see which cells failed and why, without decoding f_ field ids by hand._

  ```bash
  clay-pp-cli tables rows t_abc123 --status ERROR --limit 20 --agent
  ```

### Local state that compounds
- **`watch`** — Block until every column's enrichment runs settle, reporting per-column status counts.

  _Reach for this in a script after triggering enrichment, instead of polling the UI._

  ```bash
  clay-pp-cli watch t_abc123 --workspace 1234567 --interval 10s --agent
  ```

## Recipes

### Capture a proven table as code

```bash
clay-pp-cli blueprint export t_abc123 --agent > plumbing-austin.json
```

Serializes every column, formula, and enrichment binding into YAML you can commit.

### Clone that design into a new market

```bash
clay-pp-cli blueprint apply ./plumbing-austin.json --workbook wb_abc --name "Plumbing Dallas"
```

Recreates the table and remaps formula field references to the new column ids.

### Narrow a verbose table read for an agent

```bash
clay-pp-cli tables get 1234567 t_abc123 --agent --select fields.id,fields.name,fields.type,fields.typeSettings.actionKey
```

Table detail responses are large; selecting only the field graph keeps agent context small.

### Find a provider you can actually run

```bash
clay-pp-cli enrichments compare "find work email" --agent
```

Ranks catalog providers and marks the ones your workspace already has credentials for.

### Check a table before spending credits

```bash
clay-pp-cli columns doctor t_abc123 --agent
```

Reports formulas referencing deleted columns and columns nothing consumes.

## Usage

Run `clay-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CLAY_CONFIG_DIR`, `CLAY_DATA_DIR`, `CLAY_STATE_DIR`, or `CLAY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CLAY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CLAY_HOME=/srv/clay
clay-pp-cli doctor
```

Under `CLAY_HOME=/srv/clay`, the four dirs resolve to `/srv/clay/config`, `/srv/clay/data`, `/srv/clay/state`, and `/srv/clay/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "clay": {
      "command": "clay-pp-mcp",
      "env": {
        "CLAY_HOME": "/srv/clay"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CLAY_DATA_DIR` overrides an explicit `--home` for that kind. Use `CLAY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CLAY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `clay-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Connected provider credentials (API keys) in the workspace

- **`clay-pp-cli accounts by-type`** - List connected accounts for one provider type
- **`clay-pp-cli accounts get`** - Get one connected provider account
- **`clay-pp-cli accounts list`** - List every connected provider account
- **`clay-pp-cli accounts provider`** - Get auth metadata for a provider type

### columns

Create and configure table columns: text, formula, enrichment, and HTTP API

- **`clay-pp-cli columns create`** - Add a column to a table
- **`clay-pp-cli columns delete`** - Delete a column from a table
- **`clay-pp-cli columns update`** - Rename or reconfigure an existing column, including changing its formula

### enrichments

Search Clay's enrichment provider catalog and resolve action parameters

- **`clay-pp-cli enrichments params`** - Resolve the dynamic parameter schema for an enrichment action
- **`clay-pp-cli enrichments search`** - Search the enrichment and provider catalog

### public

Clay's documented Public API (requires CLAY_API_KEY, not the browser session)

- **`clay-pp-cli public me`** - Get the authenticated user and workspace for the public API key
- **`clay-pp-cli public search-create`** - Create a GTM database search from structured filters
- **`clay-pp-cli public search-fields`** - List filter fields available for a search source type
- **`clay-pp-cli public tables-query`** - Run a structured query across Clay tables (Enterprise sync required)

### tables

Create and manage Clay tables

- **`clay-pp-cli tables count`** - Count records in a table
- **`clay-pp-cli tables create`** - Create a table inside a workbook
- **`clay-pp-cli tables get`** - Get a table with its full field list
- **`clay-pp-cli tables hydrate`** - Create or hydrate rows, returning each cell with value and run-status metadata
- **`clay-pp-cli tables record-ids`** - List the ordered record ids for a table view (step 1 of a row read)
- **`clay-pp-cli tables records`** - Fetch cell values for specific record ids (step 2 of a row read)
- **`clay-pp-cli tables runstatus`** - Per-column enrichment run status counts
- **`clay-pp-cli tables schema`** - Get the column schema for a table view
- **`clay-pp-cli tables update`** - Rename a table or change its settings

### workbooks

Workbooks group related tables

- **`clay-pp-cli workbooks create`** - Create a workbook
- **`clay-pp-cli workbooks get`** - Get a workbook
- **`clay-pp-cli workbooks overview`** - List every table inside a workbook
- **`clay-pp-cli workbooks update`** - Rename or update a workbook

### workspace

Workspace metadata, permissions, sources, and connected accounts

- **`clay-pp-cli workspace get`** - Get workspace details
- **`clay-pp-cli workspace me`** - Get the authenticated Clay user for the browser session
- **`clay-pp-cli workspace permissions`** - List the authenticated user's workspace permissions
- **`clay-pp-cli workspace sources`** - List record sources configured in the workspace
- **`clay-pp-cli workspace subroutines`** - List saved subroutines (reusable enrichment functions)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`clay-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`clay-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`clay-pp-cli learnings list`** - Inspect taught rows
- **`clay-pp-cli learnings forget <query>`** - Undo a teach
- **`clay-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`clay-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`clay-pp-cli teach-pattern`** - Install a query/resource template up front
- **`clay-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CLAY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `clay-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clay-pp-cli accounts list mock-value

# JSON for scripting and agents
clay-pp-cli accounts list mock-value --json
# Filter to specific fields by name
clay-pp-cli accounts list mock-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
clay-pp-cli accounts list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clay-pp-cli accounts list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
clay-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `clay-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/clay-pp-cli/config.toml`; `--home`, `CLAY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clay-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every /v3 command returns 401** — Your browser session expired. Re-run clay-pp-cli auth login --chrome while signed in to app.clay.com.
- **public subcommands return 401 but table commands work** — CLAY_API_KEY is missing or is a workspace key. Create a Public API key under Settings, Account, API keys (beta).
- **tables_query returns auth_forbidden** — Structured table query needs Enterprise API table sync. Use tables records instead.
- **A formula column returns empty after blueprint apply** — Run clay-pp-cli columns doctor <tableId> to find formula references that did not remap.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**agent-plugins**](https://github.com/clay-run/agent-plugins) — Shell (101 stars)
- [**clay-gtm-cli**](https://github.com/bcharleson/clay-gtm-cli) — TypeScript (28 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
