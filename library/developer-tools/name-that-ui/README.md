# NameThatUI CLI

**Turn vague UI language into exact, source-backed component and style guidance that coding agents can use directly.**

Identify components and visual styles from colloquial descriptions, retrieve canonical anatomy and platform symbols, and translate macOS terminology without authentication. Local sync powers compact context packs, terminology linting, project inventories, and change-impact checks while every answer preserves its NameThatUI source link.

Learn more at [NameThatUI](https://namethatui.com).

Created by [@HenryBranchAdams](https://github.com/HenryBranchAdams) (HenryBranchAdams).

## Install

The recommended path installs both the `name-that-ui-pp-cli` binary and the `pp-name-that-ui` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui --agent claude-code
npx -y @mvanhorn/printing-press-library install name-that-ui --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/cmd/name-that-ui-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/name-that-ui-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-name-that-ui --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-name-that-ui --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install name-that-ui --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/name-that-ui-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/cmd/name-that-ui-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "name-that-ui": {
      "command": "name-that-ui-pp-mcp"
    }
  }
}
```

</details>

## Authentication

NameThatUI is public and requires no account, token, cookies, or browser session. The CLI uses anonymous standard HTTP for live lookups and stores only public reference data locally.

## Quick Start

```bash
# Preview doctor checks without network access; this does not test reachability.
name-that-ui-pp-cli doctor --dry-run

# Resolve an imprecise visual description into canonical component and part candidates.
name-that-ui-pp-cli identify "pale pill behind the menu bar icon" --agent --select results.name,results.platform,results.part,results.source_url

# Retrieve the source-backed definition, aliases, mappings, and related guidance.
name-that-ui-pp-cli component get macos/menu-bar --agent

# Build a deterministic local reference for offline search and project-aware commands.
name-that-ui-pp-cli sync --resources catalog,styles --latest-only

# Create a bounded implementation packet for a coding agent.
name-that-ui-pp-cli context-pack --component web/combobox --style glassmorphism --framework web --agent --select component.name,parts,apis,style_signals,cautions,source_urls

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-ready guidance
- **`context-pack`** — Assemble one bounded, source-backed implementation packet for a component, optional style, and target framework.

  _Use this when an agent needs enough precise context to implement or repair a UI without hopping across reference pages._

  ```bash
  name-that-ui-pp-cli context-pack --component web/combobox --style glassmorphism --framework web --agent --select component.name,parts,apis,style_signals,cautions,source_urls
  ```
- **`crosswalk`** — See one UI concept across plain language, component parts, AppKit, SwiftUI, and ARIA or HTML terminology.

  _Use this when product language must become exact framework terminology without relying on a model's memory._

  ```bash
  name-that-ui-pp-cli crosswalk "menu bar extra" --agent
  ```

### Project-aware reference
- **`lint`** — Find colloquial or ambiguous UI terms in prose and return canonical, source-backed candidates without rewriting the file.

  _Use this before coding when tickets, prompts, or design specs may use names that send an agent toward the wrong component._

  ```bash
  name-that-ui-pp-cli lint ./README.md --agent
  ```
- **`inventory`** — Map UI API symbols in a source tree to canonical components and source references.

  _Use this to ground a UI repair or design-system review in the components that actually appear in the project._

  ```bash
  name-that-ui-pp-cli inventory . --agent --select files.path,files.matches.symbol,files.matches.component,files.matches.source_url
  ```

### Guidance over time
- **`impact`** — Show which project files may be affected by component or style guidance changes since a prior snapshot.

  _Use this after syncing updates to focus review on code whose source-backed guidance actually changed._

  ```bash
  name-that-ui-pp-cli impact . --since 2026-07-13 --agent
  ```

## Recipes

### Name an unfamiliar component part

```bash
name-that-ui-pp-cli identify "pale pill behind the menu bar icon" --agent --select results.name,results.part,results.score,results.source_url
```

Resolve colloquial language while keeping ambiguity and provenance visible.

### Choose between similar patterns

```bash
name-that-ui-pp-cli component compare select combobox --agent
```

Retrieve concise source-backed differences before committing to an interaction pattern.

### Translate a macOS concept

```bash
name-that-ui-pp-cli crosswalk "menu bar extra" --agent
```

Join product language to AppKit and SwiftUI terminology.

### Audit terminology in a design spec

```bash
name-that-ui-pp-cli lint ./README.md --agent
```

Find imprecise UI names without performing generative rewriting.

### Review guidance changes against a project

```bash
name-that-ui-pp-cli impact . --since 2026-07-13 --agent
```

Focus review on source files connected to changed catalog records.

## Usage

Run `name-that-ui-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | `data.db` and public reference mirrors |
| `state` | `teach.log` and local learning state |
| `cache` | Regenerable cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NAME_THAT_UI_CONFIG_DIR`, `NAME_THAT_UI_DATA_DIR`, `NAME_THAT_UI_STATE_DIR`, or `NAME_THAT_UI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NAME_THAT_UI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NAME_THAT_UI_HOME=/srv/name-that-ui
name-that-ui-pp-cli doctor
```

Under `NAME_THAT_UI_HOME=/srv/name-that-ui`, the four dirs resolve to `/srv/name-that-ui/config`, `/srv/name-that-ui/data`, `/srv/name-that-ui/state`, and `/srv/name-that-ui/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "name-that-ui": {
      "command": "name-that-ui-pp-mcp",
      "env": {
        "NAME_THAT_UI_HOME": "/srv/name-that-ui"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NAME_THAT_UI_DATA_DIR` overrides an explicit `--home` for that kind. Use `NAME_THAT_UI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NAME_THAT_UI_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `name-that-ui-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### catalog

Read the public NameThatUI catalog

- **`name-that-ui-pp-cli catalog get`** - Fetch a canonical UI element page
- **`name-that-ui-pp-cli catalog list`** - List UI element pages from the public structured catalog

### reference

Read cross-framework and update references

Reference commands fetch their public data directly; `reference` is not a `sync --resources` value.

- **`name-that-ui-pp-cli reference feed`** - Fetch recent NameThatUI catalog additions
- **`name-that-ui-pp-cli reference sitemap`** - Fetch the public catalog sitemap
- **`name-that-ui-pp-cli reference translate`** - Fetch the AppKit-to-SwiftUI translation table

### translate

Look up published plain-language, AppKit, and SwiftUI mappings without authentication.

```bash
name-that-ui-pp-cli translate NSButton --from appkit --to swiftui --limit 10
```

### updates

Merge public RSS and sitemap entries, optionally filtering known timestamps.

```bash
name-that-ui-pp-cli updates --since 2026-07-01 --kind all --limit 25
```

### semantic-search

Use NameThatUI's public semantic search and reranker

- **`name-that-ui-pp-cli semantic-search`** - Rank candidate UI elements or styles for a colloquial description

### styles

Read the public visual-style atlas

- **`name-that-ui-pp-cli styles get`** - Fetch a visual-style guidance page
- **`name-that-ui-pp-cli styles list`** - List visual-style pages

### style

Read only the synced visual-style mirror (`sync --resources styles`)

- **`name-that-ui-pp-cli style identify <description>`** - Rank styles using upstream name, signal, and section evidence
- **`name-that-ui-pp-cli style list`** - List synced styles by name
- **`name-that-ui-pp-cli style get <slug-or-name>`** - Get a full synced style record
- **`name-that-ui-pp-cli style signals <slug-or-name>`** - Get upstream signals
- **`name-that-ui-pp-cli style compare <left> <right>`** - Compare full records and source overlap
- **`name-that-ui-pp-cli style code <slug-or-name>`** - Get only upstream code or implementation sections
- **`name-that-ui-pp-cli style cautions <slug-or-name>`** - Get only upstream accessibility or caution sections


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`name-that-ui-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`name-that-ui-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`name-that-ui-pp-cli learnings list`** - Inspect taught rows
- **`name-that-ui-pp-cli learnings forget <query>`** - Undo a teach
- **`name-that-ui-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`name-that-ui-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`name-that-ui-pp-cli teach-pattern`** - Install a query/resource template up front
- **`name-that-ui-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NAME_THAT_UI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `name-that-ui-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
name-that-ui-pp-cli catalog list

# JSON for scripting and agents
name-that-ui-pp-cli catalog list --json

# Filter to specific fields
name-that-ui-pp-cli catalog list --json --select id,name,status

# Dry run — show the request without sending
name-that-ui-pp-cli catalog list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
name-that-ui-pp-cli catalog list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `NAME_THAT_UI_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `name-that-ui-pp-cli catalog`
- `name-that-ui-pp-cli catalog get`
- `name-that-ui-pp-cli catalog list`
- `name-that-ui-pp-cli context-pack`
- `name-that-ui-pp-cli crosswalk`
- `name-that-ui-pp-cli impact`
- `name-that-ui-pp-cli inventory`
- `name-that-ui-pp-cli lint`
- `name-that-ui-pp-cli styles`
- `name-that-ui-pp-cli styles get`
- `name-that-ui-pp-cli styles list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
name-that-ui-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `name-that-ui-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/name-that-ui-pp-cli/config.toml`; `--home`, `NAME_THAT_UI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A live lookup returns no candidates for a visual description.** — Run `name-that-ui-pp-cli search "your terms" --limit 10` to inspect local alias and fuzzy-phrase matches, then retry with the closest canonical wording.
- **Local project commands report that reference data is missing or stale.** — Run `name-that-ui-pp-cli sync --resources catalog,styles --full` before retrying.
- **The public HTML shape changed and a detail parser fails.** — Run `name-that-ui-pp-cli doctor --json` and use the preserved source URL for manual confirmation until the parser is updated.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://namethatui.com/
- Capture coverage: 17 API entries from 221 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
