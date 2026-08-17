# Raindrop CLI

**Complete Raindrop.io control plus offline search, safe cleanup, and durable knowledge workflows.**

Use every documented bookmark, collection, tag, highlight, file, import, export and backup operation from one Go CLI. SQLite sync powers historical diffs, resumable review, explainable related-bookmark search and crash-safe automation while the matching MCP server exposes the same command tree to agents.

Learn more at [Raindrop](https://developer.raindrop.io).

## Install

The recommended path installs both the `raindrop-pp-cli` binary and the `pp-raindrop` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install raindrop
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install raindrop --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install raindrop --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install raindrop --agent claude-code
npx -y @mvanhorn/printing-press-library install raindrop --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/raindrop/cmd/raindrop-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/raindrop-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install raindrop --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-raindrop --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-raindrop --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install raindrop --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/raindrop-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RAINDROP_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/raindrop/cmd/raindrop-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "raindrop": {
      "command": "raindrop-pp-mcp",
      "env": {
        "RAINDROP_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set RAINDROP_TOKEN, run `raindrop auth set-token`, or place a test token in `.raindrop-token`. Tokens are sent only as `Authorization: Bearer` to api.raindrop.io and are never written into output artifacts.

## Quick Start

```bash
# Check config, storage and command wiring without credentials.
raindrop doctor --dry-run

# Inspect local mirror freshness before network access.
raindrop sync status --agent

# Preview generated API request safely.
raindrop raindrops search --collection 0 --perpage 5 --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Mirror bookmarks, collections, tags and highlights for instant offline search.

  _Use before local analysis or when repeated remote scans would waste rate limit._

  ```bash
  raindrop sync --agent --home /tmp/raindrop-pp
  ```
- **`changes`** — Show field-level bookmark changes between sync snapshots.

  _Use to audit automation and discover recent library edits._

  ```bash
  raindrop changes --since 7d --agent
  ```

### Safe organization
- **`inbox review`** — Process Unsorted bookmarks in resumable, bounded review sessions.

  _Use for safe inbox-zero organization without repeated prompts._

  ```bash
  raindrop inbox review --limit 10 --agent
  ```
- **`tag health`** — Discover case variants, near-duplicates, singleton tags and merge candidates.

  _Use before bulk tag cleanup instead of guessing merge targets._

  ```bash
  raindrop tag health --agent
  ```
- **`duplicates plan`** — Choose canonical bookmarks while preserving tags, notes and highlights.

  _Use when duplicate cleanup must not discard richer metadata._

  ```bash
  raindrop duplicates plan --canonical richest --agent
  ```

### Knowledge workflows
- **`revisit`** — Resurface useful forgotten bookmarks without repeating recent suggestions.

  _Use to turn a bookmark archive back into an active reading system._

  ```bash
  raindrop revisit --older-than 180d --limit 20 --agent
  ```
- **`related`** — Find explainable related bookmarks offline using text, tags and domains.

  _Use for research synthesis without a hosted semantic-search dependency._

  ```bash
  raindrop related --limit 10 --agent
  ```
- **`highlights digest`** — Create deduplicated Markdown or JSONL study digests from highlights.

  _Use to turn annotations into portable research output._

  ```bash
  raindrop highlights digest --since 30d --group-by tag --agent
  ```

### Automation safety
- **`workflow status`** — Run crash-safe bounded triage queues with retry and manual-review states.

  _Use for unattended agents that must resume safely after failures._

  ```bash
  raindrop workflow status --agent --home /tmp/raindrop-pp
  ```

## Recipes

### Narrow remote search

```bash
raindrop raindrops search --collection 0 --search 'site:github.com #go' --agent --select items._id,items.title,items.link,items.tags
```

Return only fields an agent needs.

### Refresh offline mirror

```bash
raindrop sync --agent --home /tmp/raindrop-pp
```

Incrementally fetch library resources into SQLite.

### Audit tags

```bash
raindrop tag health --agent
```

Find deterministic cleanup candidates without mutation.

### Build reading queue

```bash
raindrop revisit --older-than 180d --limit 20 --agent
```

Resurface forgotten high-value items.

## Usage

Run `raindrop-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RAINDROP_CONFIG_DIR`, `RAINDROP_DATA_DIR`, `RAINDROP_STATE_DIR`, or `RAINDROP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RAINDROP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RAINDROP_HOME=/srv/raindrop
raindrop-pp-cli doctor
```

Under `RAINDROP_HOME=/srv/raindrop`, the four dirs resolve to `/srv/raindrop/config`, `/srv/raindrop/data`, `/srv/raindrop/state`, and `/srv/raindrop/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "raindrop": {
      "command": "raindrop-pp-mcp",
      "env": {
        "RAINDROP_HOME": "/srv/raindrop"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RAINDROP_DATA_DIR` overrides an explicit `--home` for that kind. Use `RAINDROP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RAINDROP_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `raindrop-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### bookmark_transfer

Manage bookmark transfer

- **`raindrop-pp-cli bookmark-transfer export-bookmarks`** - Export bookmarks in various formats
- **`raindrop-pp-cli bookmark-transfer import-bookmarks`** - Import bookmarks from files or external services
- **`raindrop-pp-cli bookmark-transfer import-from-url`** - Import bookmarks from a remote URL hosting a bookmarks file

### collection

Manage collection

- **`raindrop-pp-cli collection create`** - Creates a new collection with the specified properties
- **`raindrop-pp-cli collection delete`** - Permanently deletes a collection and all its bookmarks
- **`raindrop-pp-cli collection empty-trash`** - Permanently delete all bookmarks in the trash collection
- **`raindrop-pp-cli collection get`** - Retrieves detailed information about a specific collection
- **`raindrop-pp-cli collection update`** - Updates properties of an existing collection

### collections

Manage collections

- **`raindrop-pp-cli collections get-all`** - Retrieves all collections for the authenticated user
- **`raindrop-pp-cli collections remove-empty`** - Remove all empty collections from the account
- **`raindrop-pp-cli collections reorder`** - Change the sort order of collections
- **`raindrop-pp-cli collections toggle-expansion`** - Expand or collapse collections in the UI

### file

Manage file

- **`raindrop-pp-cli file delete`** - Delete an uploaded file
- **`raindrop-pp-cli file get`** - Download or retrieve information about an uploaded file

### filters

Manage filters

- **`raindrop-pp-cli filters`** - Returns available filters such as tags, domains, and highlights to refine searches

### highlights

Manage highlights

- **`raindrop-pp-cli highlights add-to-bookmark`** - Creates a new text highlight for a specific bookmark
- **`raindrop-pp-cli highlights delete`** - Permanently removes a highlight from a bookmark
- **`raindrop-pp-cli highlights get-all`** - Retrieves all highlights from a user's bookmarks with pagination
- **`raindrop-pp-cli highlights get-by-collection`** - Retrieves all highlights from bookmarks in a specific collection
- **`raindrop-pp-cli highlights update`** - Modifies an existing highlight's text, note, or color

### raindrop

Manage raindrop

- **`raindrop-pp-cli raindrop create-bookmark`** - Creates a new bookmark with automatic metadata extraction
- **`raindrop-pp-cli raindrop delete-bookmark`** - Moves a bookmark to trash (soft delete)
- **`raindrop-pp-cli raindrop get-bookmark`** - Retrieves detailed information about a specific bookmark
- **`raindrop-pp-cli raindrop suggest-for-url`** - Suggest tags and collections for a new URL
- **`raindrop-pp-cli raindrop update-bookmark`** - Updates properties of an existing bookmark
- **`raindrop-pp-cli raindrop upload-file`** - Upload a file and create a bookmark from it

### raindrops

Manage raindrops

- **`raindrop-pp-cli raindrops batch-delete-bookmarks`** - Delete multiple bookmarks at once
- **`raindrop-pp-cli raindrops batch-delete-bookmarks-in-collection`** - Delete multiple bookmarks in a specific collection or empty trash/collection
- **`raindrop-pp-cli raindrops batch-tag-bookmarks`** - Batch operation to add or remove tags from multiple bookmarks
- **`raindrop-pp-cli raindrops batch-update-bookmarks`** - Update multiple bookmarks across all collections
- **`raindrop-pp-cli raindrops batch-update-bookmarks-in-collection`** - Update properties of multiple bookmarks in a specific collection at once
- **`raindrop-pp-cli raindrops bulk-move-bookmarks`** - Move multiple bookmarks to a different collection
- **`raindrop-pp-cli raindrops get-all-bookmarks`** - Retrieves all bookmarks from all collections with filtering options
- **`raindrop-pp-cli raindrops get-bookmarks-by-collection`** - Retrieves bookmarks from a specific collection with filtering options
- **`raindrop-pp-cli raindrops get-multiple-bookmarks`** - Retrieves multiple bookmarks by their IDs
- **`raindrop-pp-cli raindrops get-single-bookmark`** - Retrieves comprehensive details about a specific bookmark by ID
- **`raindrop-pp-cli raindrops search`** - Search bookmarks with advanced filtering options

### tags

Manage tags

- **`raindrop-pp-cli tags delete-all`** - Delete multiple tags from all bookmarks
- **`raindrop-pp-cli tags delete-collection`** - Delete multiple tags from bookmarks in a specific collection
- **`raindrop-pp-cli tags get-all`** - Retrieves all unique tags used in the user's bookmarks
- **`raindrop-pp-cli tags get-all-alt`** - Alternative endpoint to retrieve all tags
- **`raindrop-pp-cli tags get-by-collection`** - Retrieves all tags used in bookmarks within a specific collection
- **`raindrop-pp-cli tags rename-or-merge-all`** - Rename or merge tags across all collections
- **`raindrop-pp-cli tags rename-or-merge-collection`** - Rename or merge tags within a specific collection

### user

Manage user

- **`raindrop-pp-cli user get-profile`** - Retrieves the authenticated user's profile information
- **`raindrop-pp-cli user get-stats`** - Retrieves account-wide statistics for the authenticated user


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`raindrop-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`raindrop-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`raindrop-pp-cli learnings list`** - Inspect taught rows
- **`raindrop-pp-cli learnings forget <query>`** - Undo a teach
- **`raindrop-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`raindrop-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`raindrop-pp-cli teach-pattern`** - Install a query/resource template up front
- **`raindrop-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RAINDROP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `raindrop-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
raindrop-pp-cli collection get mock-value

# JSON for scripting and agents
raindrop-pp-cli collection get mock-value --json
# Filter to specific fields
raindrop-pp-cli collection get mock-value --json --select item,result

# Dry run — show the request without sending
raindrop-pp-cli collection get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
raindrop-pp-cli collection get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
raindrop-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `raindrop-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/raindrop-io-pp-cli/config.toml`; `--home`, `RAINDROP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RAINDROP_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `raindrop-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `raindrop-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RAINDROP_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **401 Unauthorized** — Refresh test token at https://app.raindrop.io/settings/integrations and update RAINDROP_TOKEN.
- **429 Too Many Requests** — Let built-in Retry-After backoff finish or reduce sync page concurrency.
- **Offline results look stale** — Run `raindrop sync` and inspect `raindrop sync status`.
