# Snipd CLI

**Turn your Snipd snips into a local, ranked, full-text-searchable corpus — search a concept across every note, quote, and transcript, pull the exact quote, and synthesize across shows from the command line, plus an MCP for agents.**

Your snips are trapped in a mobile app. This CLI pulls them into a local SQLite mirror with a full-text index, then lets you search, quote, filter, and aggregate them in kilobytes instead of scrolling an app. The same commands are exposed as MCP tools so an agent can reason over your own listening.

## Install

The recommended path installs both the `snipd-pp-cli` binary and the `pp-snipd` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install snipd
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install snipd --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install snipd --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install snipd --agent claude-code
npx -y @mvanhorn/printing-press-library install snipd --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/cmd/snipd-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/snipd-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install snipd --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-snipd --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-snipd --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install snipd --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/snipd-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SNIPD_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/cmd/snipd-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "snipd": {
      "command": "snipd-pp-mcp",
      "env": {
        "SNIPD_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Snipd's export API authenticates with **your own Snipd account Bearer token**. Get it with a one-time browser sign-in — **you don't need Obsidian, the Snipd Obsidian plugin, or the mobile app** (the page is titled "Obsidian integration" only because the CLI reuses Snipd's export API):

1. Generate any random UUID — e.g. `uuidgen` — and call it `X`.
2. Open `https://app.snipd.com/obsidian/auth?uuid=X` and sign in with your Snipd account (Google, Apple, or email).
3. Open `https://api.snipd.com/v1/public/api/obsidian/auth?uuid=X` — it shows `{"token":"…"}`. Copy the token.
4. Save it: `snipd-pp-cli auth set-token <token>` (or `export SNIPD_TOKEN=<token>` for one session).

The token is personal and account-scoped — read-only, stored locally, and it never leaves your machine. If it stops working, repeat the steps for a fresh one (about a minute).

## Quick Start

```bash
# Health check — confirms the token and reachability before you pull.
snipd-pp-cli doctor --dry-run

# Build (or refresh) the local corpus from your Snipd account.
snipd-pp-cli pull

# Ranked full-text search across every snip's note, quote, and transcript.
snipd-pp-cli search "orchestration"

# A compact evidence feed scoped to a concept, ready for an agent to synthesize.
snipd-pp-cli synthesize "AI and jobs" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local snip corpus that compounds
- **`pull`** — Pull your Snipd snips into a local SQLite mirror you can search and query offline — the app has no export-to-query path.

  _Run this first: it is the populate step every search, quote, filter, and aggregate reads from._

  ```bash
  snipd-pp-cli pull --agent
  ```
- **`filter`** — Slice the corpus by show, favorite, tag, date, or duration and return compact rows or counts.

  _Use this for structured slices; reach for search when the criterion is a free-text concept._

  ```bash
  snipd-pp-cli filter --show "Finding Our Way" --favorite --agent
  ```
- **`aggregate`** — Roll up counts and distributions across your corpus — snips per show, top tags, favorites, episode density, and timeline.

  _Use this to understand the shape of your listening — where your snips cluster — in one call._

  ```bash
  snipd-pp-cli aggregate by-show --agent
  ```

### Agent-native retrieval over your own snips
- **`quote`** — Get the verbatim pull-quote and speaker for matching snips — the distinct unit other tools concatenate into transcript prose and lose.

  _Reach for this when you need a citable verbatim line, not a paraphrase or the whole transcript._

  ```bash
  snipd-pp-cli quote "thinking partner" --agent
  ```
- **`synthesize`** — Gather a deduped, snippet-only evidence set (title + note + quote + deep-link) scoped by query, show, or topic — sized for an agent to reason over.

  _Use this to assemble the evidence for a cross-show answer without dragging full transcripts through context._

  ```bash
  snipd-pp-cli synthesize "AI and jobs" --limit 20 --agent
  ```

## Recipes

### Find the quote behind a claim

```bash
snipd-pp-cli quote "thought partner" --agent
```

Returns the verbatim pull-quote and speaker for snips matching the phrase.

### Compact evidence for a cross-show answer

```bash
snipd-pp-cli synthesize "personas vs archetypes" --limit 15 --agent --select snip_id,show,title,note,url
```

A deduped, snippet-only feed narrowed to exactly the fields an agent needs, from a deeply nested store.

### Everything from one show

```bash
snipd-pp-cli filter --show "NN/G UX Podcast" --agent
```

All snips for a show as compact rows, ordered by episode and timestamp.

### Where do my snips cluster?

```bash
snipd-pp-cli aggregate by-show
```

A one-call rollup of snip counts per show.

## Usage

Run `snipd-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SNIPD_CONFIG_DIR`, `SNIPD_DATA_DIR`, `SNIPD_STATE_DIR`, or `SNIPD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SNIPD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SNIPD_HOME=/srv/snipd
snipd-pp-cli doctor
```

Under `SNIPD_HOME=/srv/snipd`, the four dirs resolve to `/srv/snipd/config`, `/srv/snipd/data`, `/srv/snipd/state`, and `/srv/snipd/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "snipd": {
      "command": "snipd-pp-mcp",
      "env": {
        "SNIPD_HOME": "/srv/snipd"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SNIPD_DATA_DIR` overrides an explicit `--home` for that kind. Use `SNIPD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SNIPD_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `snipd-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### catalog

The upstream export catalog — which of your episodes are available to pull, and what changed

- **`snipd-pp-cli catalog`** - List your episodes available for export (ids, snip counts, batches). Pass updated_after for an incremental view of what changed.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`snipd-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`snipd-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`snipd-pp-cli learnings list`** - Inspect taught rows
- **`snipd-pp-cli learnings forget <query>`** - Undo a teach
- **`snipd-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`snipd-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`snipd-pp-cli teach-pattern`** - Install a query/resource template up front
- **`snipd-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SNIPD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `snipd-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
snipd-pp-cli catalog

# JSON for scripting and agents
snipd-pp-cli catalog --json

# Filter to specific fields
snipd-pp-cli catalog --json --select id,name,status

# Dry run — show the request without sending
snipd-pp-cli catalog --dry-run

# Agent mode — JSON + compact + no prompts in one flag
snipd-pp-cli catalog --agent
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
snipd-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `snipd-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/snipd-pp-cli/config.toml`; `--home`, `SNIPD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SNIPD_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `snipd-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `snipd-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SNIPD_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search/quote/filter return nothing** — Run `snipd-pp-cli pull` first — retrieval reads the local mirror, which is empty until you pull.
- **401 / auth error on pull** — Your token expired or was revoked. Get a fresh one with the browser sign-in in [Authentication](#authentication), then set it as SNIPD_TOKEN (or `snipd-pp-cli auth set-token <token>`).
- **a search term returns fewer hits than expected** — Use plain stemmed words, not `*` prefixes — the index is porter-stemmed, so `run` already matches `running`.
