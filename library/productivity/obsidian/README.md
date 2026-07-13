# obsidian-pp-cli

**Read-only vault analytics for [Obsidian](https://obsidian.md) — works offline against a local SQLite mirror.**

`obsidian-pp-cli` is a Go CLI plus MCP server that wraps Obsidian's official `obsidian` binary (v1.12+) for live reads and maintains a local SQLite mirror for sub-100 ms compound analytics. The 13 live read commands wrap the upstream CLI directly via subprocess; the Tier-3 commands (`health`, `stale`, `orphans`, `broken`, `vault-sql`, `load`) query the mirror so they answer instantly even when Obsidian is closed.

**V1 is read-only by design.** Write commands (create / delete / append / prepend / move / property:set) are deferred to V2 pending the upstream `markdown-patch` frontmatter-corruption fix. Skipping writes in V1 means zero corruption exposure — every command in this CLI either reads from the live `obsidian` binary or queries a local SQLite copy of your vault.

Run `obsidian-pp-cli sync` with Obsidian open (or `sync --vault-path <abs>` with no Obsidian at all) to refresh the mirror; all Tier-3 commands then run offline. Each vault gets its own mirror DB — read commands follow the most recently synced vault automatically.

Created by [@DrDriftwood](https://github.com/DrDriftwood) (Angelo Pullen).

## Install

The recommended path installs both the `obsidian-pp-cli` binary and the `pp-obsidian` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install obsidian
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install obsidian --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install obsidian --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install obsidian --agent claude-code
npx -y @mvanhorn/printing-press-library install obsidian --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/obsidian/cmd/obsidian-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/obsidian-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install obsidian --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-obsidian --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-obsidian --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install obsidian --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/obsidian-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/obsidian/cmd/obsidian-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "obsidian": {
      "command": "obsidian-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
obsidian-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
obsidian-pp-cli deadends
```

## Usage

Run `obsidian-pp-cli --help` for the full command reference and flag list.

## Commands

### Live reads (require Obsidian running)

These commands shell out to the official `obsidian` binary and return current vault state. They need Obsidian open with a vault loaded.

- **`obsidian-pp-cli notes <name>`** — Read a note's contents.
- **`obsidian-pp-cli notes <name> backlinks list`** — List backlinks to a note.
- **`obsidian-pp-cli notes <name> links list`** — List outgoing links from a note.
- **`obsidian-pp-cli notes <name> properties read-property <property>`** — Read a frontmatter property value.
- **`obsidian-pp-cli live-search <query>`** — Live full-text search via the running Obsidian process. (Distinct from `search`, which queries the local mirror — titles/paths only.)
- **`obsidian-pp-cli live-search context <query>`** — Live full-text search with matching line context.
- **`obsidian-pp-cli tags`** — List tags in the vault.
- **`obsidian-pp-cli tasks`** — List tasks in the vault.
- **`obsidian-pp-cli files`** — List files in the vault.
- **`obsidian-pp-cli folders`** — List folders in the vault.
- **`obsidian-pp-cli deadends`** — Notes with no outgoing links (live pass-through).
- **`obsidian-pp-cli unresolved`** — Unresolved wikilink targets (live pass-through).
- **`obsidian-pp-cli vault`** — Vault metadata.

### Mirror-backed analytics (work offline once `sync` has run)

- **`obsidian-pp-cli sync`** — Walk a vault and populate its local SQLite mirror. Resolves Obsidian's active vault by default; pass `--vault-path <abs>` to walk a vault directory directly with no Obsidian dependency (point it at the INNER content folder on nested layouts — sync warns loudly when it detects a wrapper). `--max-files=N` bounds work for testing or CI.
- **`obsidian-pp-cli health`** — Composite vault-health score (connectivity, freshness, integrity, consistency). `--explain` prints the scoring formula.
- **`obsidian-pp-cli orphans`** — Notes with no incoming wikilinks, ranked by age, with title and word count for triage.
- **`obsidian-pp-cli stale --days=N`** — Notes not modified in N days that still have incoming wikilinks (triage candidates).
- **`obsidian-pp-cli broken`** — Unresolved wikilinks plus their source notes — answers "where is the broken link?", not just "what is broken?"
- **`obsidian-pp-cli vault-sql <query>`** — Raw SELECT against the mirror (read-only). Schema: notes, obsidian_tags, obsidian_links, frontmatter_kv.
- **`obsidian-pp-cli load`** — Quick coverage report (note count, tag count, link count, last sync).
- **`obsidian-pp-cli search <query>`** — Local-mirror search over note titles and paths (fast, offline). Pair with `live-search` for body-text search via the running Obsidian process.
- **`obsidian-pp-cli workflow status`** — Verbose mirror coverage report (alias of `load`).

### Mirror staleness signaling

Mirror-backed commands check whether the local SQLite copy is older than 24h. If Obsidian is running, they suggest re-running `sync`; if it isn't, they warn that results may be stale. Sync is the only path back to a current mirror.

### Multi-vault mirrors (added 2026-07-13)

Each vault mirrors into its own DB file (`vault-<name>-<hash>.db` under `~/.local/share/obsidian-pp-cli/`), and `sync` records the most recently synced vault in `current.json`. Read commands follow that pointer by default, so the "sync, then query" flow is unchanged — but syncing vault B no longer prunes vault A's rows, and each vault keeps an independent incremental checkpoint. `--db` still overrides everything (and skips the pointer update). The legacy shared `data.db` remains as a read fallback for pre-multi-vault mirrors.

Two silent failure modes motivated this (both hit in production on 2026-07-10): a mirror rooted at the OUTER wrapper of a nested vault layout (`~/Desktop/<Name>/<Name>/`) path-prefixes every note — ~16.7k false broken links and a meaningless integrity score with no error raised — and same-machine sibling vaults deleting each other's mirror rows on every sync. `sync` now detects the wrapper signature and warns with the exact `--vault-path` fix.

### V2 (not in V1)

Write commands (create, delete, append, prepend, move, property:set) are intentionally absent in V1. They wait on the upstream `markdown-patch` frontmatter-corruption fix. Skipping them in V1 means zero corruption exposure — V1 reads from the live binary or a SQLite copy of your vault, and never writes back. Non-macOS platforms are also V2. (Multi-vault mirrors, originally slated for V2, shipped 2026-07-13 — see above.)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
obsidian-pp-cli deadends

# JSON for scripting and agents
obsidian-pp-cli deadends --json

# Filter to specific fields
obsidian-pp-cli deadends --json --select id,name,status

# Dry run — show the request without sending
obsidian-pp-cli deadends --dry-run

# Agent mode — JSON + compact + no prompts in one flag
obsidian-pp-cli deadends --agent
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
obsidian-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/obsidian-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
