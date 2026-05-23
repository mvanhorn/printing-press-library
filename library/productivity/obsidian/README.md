# Obsidian CLI

**Every Obsidian CLI feature, plus protocol-aware frontmatter enforcement, instant offline FTS5 search, and token-efficient agent reads no other Obsidian tool ships.**

A filesystem-direct CLI for Obsidian vaults that enforces the UCE three-layer-memory protocol (Knowledge Graph / Events / Patterns) on every write, indexes the whole vault into a local SQLite store for sub-100ms full-text search and cross-note SQL queries, and emits agent-friendly progressive-disclosure output so an LLM can pick what to read without ingesting full notes. An optional rest subcommand passes through to the coddingtonbear/obsidian-local-rest-api community plugin when Obsidian is running.

Printed by [@dstevens](https://github.com/dstevens) (Damien Stevens).

## Install

The recommended path installs both the `obsidian-pp-cli` binary and the `pp-obsidian` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install obsidian
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install obsidian --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/obsidian-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-obsidian --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-obsidian --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-obsidian skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-obsidian. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required for filesystem mode — set OBSIDIAN_VAULT_PATH to your vault directory and you're done. Optional --rest mode requires the Obsidian Local REST API plugin and a bearer token (OBSIDIAN_REST_TOKEN).

## Quick Start

```bash
# Verify the vault path is set and the store is reachable before doing anything else.
obsidian-pp-cli doctor


# Walk the vault once to populate the local SQLite index. Subsequent commands query the store, not the filesystem.
obsidian-pp-cli sync


# Run the three-layer protocol lint and see every note that won't pass downstream extraction.
obsidian-pp-cli lint --severity error --json


# Sub-100ms FTS5 search returning only path and description, ideal for agents.
obsidian-pp-cli search 'buttermilk' --select path,description --json


# Token-efficient entity read: paths and one-line descriptions instead of full notes.
obsidian-pp-cli entity dossier '[[Jeff Smith]]' --layer description --json

```

## Known Gaps

- **`rest` subtree requires the Obsidian app + Local REST API plugin.** The CLI runs in filesystem mode by default and needs no token. `rest *` commands (e.g. `rest commands`, `rest exec-command`, `rest active`) proxy the [coddingtonbear/obsidian-local-rest-api](https://github.com/coddingtonbear/obsidian-local-rest-api) community plugin and only work when the Obsidian desktop app is running with the plugin enabled and `OBSIDIAN_REST_TOKEN` is set. `doctor` does not verify plugin reachability — if a `rest *` command fails with a connection error, confirm the plugin is enabled in Obsidian's settings.
- **No unit tests in `internal/vault`.** The vault package (frontmatter parse/assemble, atomic write) is exercised end-to-end by the CLI commands but has no direct unit tests yet. Contributors making changes there should run a full CLI smoke (`sync` → `lint` → `note read`) until a test suite lands.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Three-layer protocol enforcement
- **`lint`** — Walks the vault and reports frontmatter violations with severity tiers, encoding the three-layer-memory protocol rules used by the UCE pipeline.

  _Reach for this before handing the vault to any downstream extractor (cm, search, sync). A protocol error costs hours of silent extraction drift; lint catches them at write time._

  ```bash
  obsidian-pp-cli lint --severity error --json
  ```
- **`migrate`** — Fixes the mechanical subset of lint violations: ISO date coercion, type enum normalization, fill missing description from body.

  _Use this after onboarding an old vault or after a manual editing spree. Always start with --dry-run._

  ```bash
  obsidian-pp-cli migrate --rule date-iso --dry-run
  ```
- **`layers stats`** — Counts of notes per memory layer (Knowledge Graph / Events / Patterns) with type breakdown, average age, and recent-write velocity.

  _Run before a triage session to see where the vault is heavy or light: too many Events, no new Patterns, abandoned Knowledge-Graph entities._

  ```bash
  obsidian-pp-cli layers stats --json
  ```
- **`readiness`** — Filters lint findings to the rule subset that the downstream cm extraction pipeline depends on (missing description, missing type, bad date format).

  _Run before a Tuck sync. Fixing readiness errors here is cheap; debugging them in cm output is expensive._

  ```bash
  obsidian-pp-cli readiness --since 2026-04-01 --json
  ```

### Fact and decision tracking
- **`facts graduation-candidates`** — Lists entities whose inline fact count is approaching or past the 20-fact threshold for graduation to a TOML sidecar file.

  _Pick this when a person/project file is starting to feel heavy. Graduating to TOML before 20 keeps frontmatter loadable._

  ```bash
  obsidian-pp-cli facts graduation-candidates --threshold 20 --json
  ```
- **`facts decision-trace`** — Given a decision_trace_id, returns every fact across the vault that cites it, ordered by timestamp.

  _Reach for this when auditing how a decision propagated through the vault. Required when reconstructing a decision for a stakeholder or post-mortem._

  ```bash
  obsidian-pp-cli facts decision-trace DT-2026-0142 --json
  ```
- **`provenance`** — Reads source frontmatter on a note plus the source field on every fact in that note; prints a chain showing where each datum came from.

  _Reach for this when a fact looks wrong or disputed. The chain shows whether it came from a transcript, manual edit, or agent._

  ```bash
  obsidian-pp-cli provenance 'People/Jeff Smith.md' --json
  ```

### Agent-native reads
- **`entity dossier`** — Joins notes + frontmatter + facts + backlinks + tags for one entity into a single agent-readable block.

  _Use this as the default first read when an agent needs context about a person, company, or project — replaces grep + cat + parse._

  ```bash
  obsidian-pp-cli entity dossier '[[Jeff Smith]]' --layer description --json
  ```
- **`stale`** — Lists notes whose mtime predates a threshold, optionally filtered by type.

  _Run weekly to find meetings/journals that never got promoted to a Pattern, or active entities that have gone cold._

  ```bash
  obsidian-pp-cli stale --type meeting --older-than 90d --json
  ```
- **`daily append`** — Resolves today's daily-note path, creates it from the periodic-note template (with protocol-compliant frontmatter) if missing, and appends under a named section.

  _Default capture path for transcript ingest, journal entries, and any 'remember this' agent task._

  ```bash
  obsidian-pp-cli daily append 'Talked to Mark about Servosity pricing' --section '## Notes'
  ```

## Usage

Run `obsidian-pp-cli --help` for the full command reference and flag list.

## Commands

### rest

Optional Local REST API passthrough (requires the Obsidian app running with the Local REST API community plugin enabled).

- **`obsidian-pp-cli rest active`** - Read the file currently open in the Obsidian editor (requires --rest mode).
- **`obsidian-pp-cli rest append-active`** - Append text to the file currently open in the Obsidian editor.
- **`obsidian-pp-cli rest commands`** - List available Obsidian commands (REST plugin only).
- **`obsidian-pp-cli rest delete-active`** - Delete the file currently open in the Obsidian editor.
- **`obsidian-pp-cli rest exec-command`** - Execute an Obsidian command by its ID.
- **`obsidian-pp-cli rest ping`** - Verify the Local REST API plugin is reachable.
- **`obsidian-pp-cli rest search-simple`** - Simple text search through the REST plugin.
- **`obsidian-pp-cli rest tags`** - List all tags in the vault via the REST plugin.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
obsidian-pp-cli rest active

# JSON for scripting and agents
obsidian-pp-cli rest active --json

# Filter to specific fields
obsidian-pp-cli rest active --json --select id,name,status

# Dry run — show the request without sending
obsidian-pp-cli rest active --dry-run

# Agent mode — JSON + compact + no prompts in one flag
obsidian-pp-cli rest active --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-obsidian -g
```

Then invoke `/pp-obsidian <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add obsidian obsidian-pp-mcp -e OBSIDIAN_REST_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/obsidian-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OBSIDIAN_REST_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "obsidian": {
      "command": "obsidian-pp-mcp",
      "env": {
        "OBSIDIAN_REST_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

Verified recipes against the live CLI. Every flag below was confirmed against `obsidian-pp-cli <cmd> --help`.

```bash
# Surface only the lint errors that block downstream UCE extraction.
obsidian-pp-cli readiness --json | jq '.[] | select(.severity=="error")'

# Auto-fix mechanical lint violations across the whole vault.
obsidian-pp-cli migrate --rule all

# Fact graduation: list entities approaching the 20-fact threshold.
obsidian-pp-cli facts graduation-candidates --threshold 18 --json

# Find every fact citing one decision trace id, ordered by time.
obsidian-pp-cli facts decision-trace DT-2026-0142 --json

# Cross-note SQL — recent meeting notes, JSON output (read-only SELECT only).
obsidian-pp-cli sql "SELECT path, description FROM notes WHERE type='meeting' ORDER BY mtime DESC LIMIT 5" --json

# Per-layer health: Knowledge Graph vs Events vs Patterns counts and write velocity.
obsidian-pp-cli layers stats --json

# Token-efficient entity dossier — paths + descriptions only, no full notes.
obsidian-pp-cli entity dossier '[[Damien Stevens]]' --layer description --json

# Find stale meetings older than 90 days that never graduated to a pattern.
obsidian-pp-cli stale --type meeting --older-than 90d --json

# Audit one note's provenance: frontmatter `source` plus every fact's source and decision_trace_id.
obsidian-pp-cli provenance 'People/Damien Stevens.md' --json

# Capture a transcript fragment into today's daily note under a named section.
obsidian-pp-cli daily append 'Talked to Mark about pricing' --section '## Notes'

# REST passthrough: execute an Obsidian command palette action (requires plugin + OBSIDIAN_REST_TOKEN).
obsidian-pp-cli rest exec-command editor:save-file
```

## Health Check

```bash
obsidian-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/obsidian-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OBSIDIAN_REST_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `obsidian-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OBSIDIAN_REST_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **vault path not set** — export OBSIDIAN_VAULT_PATH=/absolute/path/to/your/vault — the CLI refuses to operate on a missing path.
- **search returns nothing on a known note** — Run obsidian-pp-cli sync to repopulate the index; sync is incremental on subsequent runs.
- **lint reports errors after a manual edit** — Run obsidian-pp-cli migrate --dry-run to preview mechanical fixes (date format, type enum); rerun without --dry-run to apply.
- **--rest commands return connection refused** — The Obsidian app must be running and the Local REST API community plugin enabled. Token in OBSIDIAN_REST_TOKEN.
- **links broken reports many false positives** — Sync first — broken-link detection requires the link index to be current. Wikilinks to files renamed outside this CLI may also show as broken.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**coddingtonbear/obsidian-local-rest-api**](https://github.com/coddingtonbear/obsidian-local-rest-api) — TypeScript (1500 stars)
- [**Yakitrak/obsidian-cli**](https://github.com/Yakitrak/obsidian-cli) — Go (600 stars)
- [**cyanheads/obsidian-mcp-server**](https://github.com/cyanheads/obsidian-mcp-server) — TypeScript (500 stars)
- [**StevenStavrakis/obsidian-mcp**](https://github.com/StevenStavrakis/obsidian-mcp) — TypeScript (400 stars)
- [**bitbonsai/mcpvault**](https://github.com/bitbonsai/mcpvault) — Go (200 stars)
- [**jwhonce/obsidian-cli**](https://github.com/jwhonce/obsidian-cli) — Python (150 stars)
- [**davidpp/obsidian-cli**](https://github.com/davidpp/obsidian-cli) — TypeScript (100 stars)
- [**mattjoyce/obsave**](https://github.com/mattjoyce/obsave) — Python (80 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
