# Zotero CLI

**Your entire Zotero research library, searchable offline — version-based incremental sync into SQLite with full-text search no other Zotero tool has.**

zotero-research-library-pp-cli mirrors your library into a local SQLite cache using the API's version cursors, then answers research-grounding questions offline: ranked full-text search with snippets, collection trees, recent-additions triage, and Better BibTeX citekey lookups. What the Web API meters at 100 items a page, your cache answers in milliseconds.

## Install

The recommended path installs both the `zotero-research-library-pp-cli` binary and the `pp-zotero-research-library` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library --agent claude-code
npx -y @mvanhorn/printing-press-library install zotero-research-library --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/cmd/zotero-research-library-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zotero-research-library-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zotero-research-library --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zotero-research-library --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zotero-research-library --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zotero-research-library-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZOTERO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/cmd/zotero-research-library-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zotero-research-library": {
      "command": "zotero-research-library-pp-mcp",
      "env": {
        "ZOTERO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Create an API key at zotero.org/settings/keys and set ZOTERO_API_KEY (or run auth set-token). The CLI resolves your numeric userID automatically via /keys/current and stores the library-prefixed base URL in config — you never need to look up your user ID.

## Quick Start

```bash
# Verify config shape without touching the API
zotero-research-library-pp-cli doctor --dry-run

# First full mirror of items, collections, tags, and full text
zotero-research-library-pp-cli sync --full

# The grounding ritual: what does my library say about this?
zotero-research-library-pp-cli ground "velocity based training" --limit 10

# What did I add this week
zotero-research-library-pp-cli items recent --days 7 --agent

# The library's shape at a glance
zotero-research-library-pp-cli collections tree --counts

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Research grounding that compounds
- **`ground`** — Ranked full-text search over your synced library — title, abstract, creators, tags, and attachment full text — answered offline in under a second, with snippets and citation-ready metadata.

  _Reach for this when asked what the user's own library says about a topic._

  ```bash
  zotero-research-library-pp-cli ground "hamstring injury prevention" --limit 10 --agent
  ```
- **`items recent`** — What landed in the library recently, sorted by dateAdded from the local cache, with a --days cutoff and pipeable keys.

  _Use for weekly triage of new additions._

  ```bash
  zotero-research-library-pp-cli items recent --days 7 --agent
  ```
- **`collections tree`** — Renders the full collection hierarchy with per-node item counts in one command — a shape the flat API cannot return in one call.

  _Use for a hierarchical overview of how the library is organized._

  ```bash
  zotero-research-library-pp-cli collections tree --counts --agent
  ```

### Manuscript plumbing
- **`cite`** — Resolves a Better BibTeX citekey to its item from the local cache and exports citekey-faithful BibTeX; sync backfills citekeys via the desktop's local JSON-RPC when available.

  _Use when a manuscript needs a citation by its Better BibTeX citekey._

  ```bash
  zotero-research-library-pp-cli cite smithHamstring2024 --format bibtex
  ```

### Agent-native plumbing
- **`cache status`** — Sync cursors, row and FTS counts, and last-sync age from the local cache; 'cache reindex' rebuilds the FTS index from base tables with zero network.

  _Use to check cache freshness offline before trusting search results._

  ```bash
  zotero-research-library-pp-cli cache status --agent
  ```

## Recipes

### Research-grounding query

```bash
zotero-research-library-pp-cli ground "return to play criteria" --limit 10 --agent --select key,title,creators,date,DOI
```

Offline ranked search with citation-ready fields; the first step of the grounding workflow.

### Full-text deep search

```bash
zotero-research-library-pp-cli ground "nordic hamstring" --fulltext --agent
```

Extends the search into attachment full text and resolves hits to their parent papers.

### Weekly additions to BibTeX

```bash
zotero-research-library-pp-cli items recent --days 7 --agent --select key,title
```

Fresh additions ready to pipe into an export.

### Scope a search to a collection

```bash
zotero-research-library-pp-cli ground "acl injury" --collection HAMSTRING --limit 5
```

Collection-scoped grounding using the local join table.

### Live server-side search fallback

```bash
zotero-research-library-pp-cli items list --q "tendon" --qmode everything --limit 10 --json
```

When you need unsynced server state, the generated endpoint carries the API's full search grammar.

## Usage

Run `zotero-research-library-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZOTERO_RESEARCH_LIBRARY_CONFIG_DIR`, `ZOTERO_RESEARCH_LIBRARY_DATA_DIR`, `ZOTERO_RESEARCH_LIBRARY_STATE_DIR`, or `ZOTERO_RESEARCH_LIBRARY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZOTERO_RESEARCH_LIBRARY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZOTERO_RESEARCH_LIBRARY_HOME=/srv/zotero-research-library
zotero-research-library-pp-cli doctor
```

Under `ZOTERO_RESEARCH_LIBRARY_HOME=/srv/zotero-research-library`, the four dirs resolve to `/srv/zotero-research-library/config`, `/srv/zotero-research-library/data`, `/srv/zotero-research-library/state`, and `/srv/zotero-research-library/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zotero-research-library": {
      "command": "zotero-research-library-pp-mcp",
      "env": {
        "ZOTERO_RESEARCH_LIBRARY_HOME": "/srv/zotero-research-library"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZOTERO_RESEARCH_LIBRARY_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZOTERO_RESEARCH_LIBRARY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZOTERO_RESEARCH_LIBRARY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `zotero-research-library-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### collections

Hierarchical collections organizing the library

- **`zotero-research-library-pp-cli collections get`** - One collection by key
- **`zotero-research-library-pp-cli collections items`** - Items in a collection (including children)
- **`zotero-research-library-pp-cli collections list`** - All collections
- **`zotero-research-library-pp-cli collections subcollections`** - Subcollections of a collection
- **`zotero-research-library-pp-cli collections top`** - Top-level collections

### deleted

Deletion tombstones for sync

- **`zotero-research-library-pp-cli deleted`** - Keys of items/collections/searches/tags deleted since a library version

### fulltext

Full-text content index

- **`zotero-research-library-pp-cli fulltext`** - Attachment keys whose full-text content changed since a version

### items

Bibliographic items in the library

- **`zotero-research-library-pp-cli items children`** - Child notes and attachments of an item
- **`zotero-research-library-pp-cli items fulltext`** - Full-text content of an attachment item
- **`zotero-research-library-pp-cli items get`** - Get one item by key
- **`zotero-research-library-pp-cli items list`** - List items. Supports q (phrase search), qmode=everything (includes fulltext), tag filters with || OR and - negation, itemType, and since=<version>
- **`zotero-research-library-pp-cli items top`** - Top-level items only (no child notes/attachments)
- **`zotero-research-library-pp-cli items trash`** - Items in the trash

### keys

API key introspection

- **`zotero-research-library-pp-cli keys`** - Resolve the current API key to userID, username, and access scopes (doctor probe + userID discovery)

### searches

Saved searches

- **`zotero-research-library-pp-cli searches get`** - One saved search
- **`zotero-research-library-pp-cli searches list`** - All saved searches

### tags

Tags across the library

- **`zotero-research-library-pp-cli tags`** - All tags in the library


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zotero-research-library-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zotero-research-library-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zotero-research-library-pp-cli learnings list`** - Inspect taught rows
- **`zotero-research-library-pp-cli learnings forget <query>`** - Undo a teach
- **`zotero-research-library-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zotero-research-library-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zotero-research-library-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zotero-research-library-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZOTERO_RESEARCH_LIBRARY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zotero-research-library-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zotero-research-library-pp-cli collections list

# JSON for scripting and agents
zotero-research-library-pp-cli collections list --json

# Filter to specific fields
zotero-research-library-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
zotero-research-library-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zotero-research-library-pp-cli collections list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `ZOTERO_RESEARCH_LIBRARY_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `zotero-research-library-pp-cli collections`
- `zotero-research-library-pp-cli collections get`
- `zotero-research-library-pp-cli collections list`
- `zotero-research-library-pp-cli collections search`
- `zotero-research-library-pp-cli deleted`
- `zotero-research-library-pp-cli deleted get`
- `zotero-research-library-pp-cli deleted list`
- `zotero-research-library-pp-cli deleted search`
- `zotero-research-library-pp-cli items`
- `zotero-research-library-pp-cli items get`
- `zotero-research-library-pp-cli items list`
- `zotero-research-library-pp-cli items search`
- `zotero-research-library-pp-cli searches`
- `zotero-research-library-pp-cli searches get`
- `zotero-research-library-pp-cli searches list`
- `zotero-research-library-pp-cli searches search`
- `zotero-research-library-pp-cli tags`
- `zotero-research-library-pp-cli tags get`
- `zotero-research-library-pp-cli tags list`
- `zotero-research-library-pp-cli tags search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
zotero-research-library-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `zotero-research-library-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zotero-research-library-pp-cli/config.toml`; `--home`, `ZOTERO_RESEARCH_LIBRARY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZOTERO_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `zotero-research-library-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zotero-research-library-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZOTERO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Forbidden on every call** — Key lacks library access — regenerate at zotero.org/settings/keys with 'Allow library access' checked
- **ground returns nothing** — The cache is empty — run 'sync --full' first; check 'cache status' for cursors and row counts
- **cite finds no citekeys** — Citekeys come from the Better BibTeX plugin via the desktop app — open Zotero desktop and re-run 'sync'
