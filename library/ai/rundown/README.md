# The Rundown University CLI

**Search the community's AI workflows offline, and rank them by week - two things the site's own API cannot do.**

The Rundown's community feed has no date filter and no single-post endpoint, so questions like 'best rated this week' or 'read me that whole workflow' need a local mirror. This CLI syncs every workflow into SQLite, then answers them instantly with `top --since`, `use-cases`, `show`, `digest`, `tools rank` and `stack`. No account or API key is needed - every read endpoint is public.

## Install

The recommended path installs both the `rundown-pp-cli` binary and the `pp-rundown` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rundown
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rundown --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rundown --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rundown --agent claude-code
npx -y @mvanhorn/printing-press-library install rundown --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/rundown/cmd/rundown-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rundown-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rundown --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rundown --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rundown --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rundown --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rundown-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/rundown/cmd/rundown-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rundown": {
      "command": "rundown-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. Every read endpoint used by this CLI (`/posts`, `/posts/{id}/comments`, `/tools`, `/leaderboard`) is served publicly without a session. Upvoting, bookmarking, commenting and posting do require a signed-in Clerk session and are deliberately not implemented here.

## Quick Start

```bash
# Confirm the community API is reachable - no key or login needed
rundown-pp-cli doctor

# Mirror every workflow into local SQLite; everything below is then offline and instant
rundown-pp-cli sync

# The best-rated workflows from the last week
rundown-pp-cli top --since 7d --limit 5

# Ask whether the community has already solved a problem
rundown-pp-cli use-cases "cold email outreach"

# A rollup of what landed this week and which tools drove it
rundown-pp-cli digest --since 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`top`** — Rank the highest-upvoted workflows inside a time window like 7d or 30d.

  _This is the answer to 'bring me the best workflows this week' in one call instead of paging the feed and eyeballing dates._

  ```bash
  rundown-pp-cli top --since 7d --limit 5 --agent
  ```
- **`digest`** — Summarise a time window: how many workflows landed, the top posts, the most-used tools and the busiest authors.

  _Use this for a standing 'what happened in the community' check rather than scrolling the feed._

  ```bash
  rundown-pp-cli digest --since 7d --agent
  ```
- **`tools rank`** — Rank AI tools by how often they appear in workflows and by the upvotes those workflows earned.

  _Use this to see which tools the community actually builds with, as opposed to which ones merely exist in the dropdown._

  ```bash
  rundown-pp-cli tools rank --since 30d --limit 15
  ```
- **`stack`** — Show which other tools appear alongside a given tool, so you can see the stacks people really run.

  _Use this when picking complementary tooling, or to answer 'what do people pair with X'._

  ```bash
  rundown-pp-cli stack claude-code --limit 10
  ```

### Search that actually finds things
- **`use-cases`** — Answer 'are there any workflows for X' by blending the server's semantic search with local full-text search, then ranking by upvotes.

  _Reach for this whenever someone asks whether the community has already solved a problem, before designing a workflow from scratch._

  ```bash
  rundown-pp-cli use-cases "cold email outreach" --limit 5 --agent
  ```
- **`show`** — Print one workflow in full - body, tools, industries, author and comments - in a single call.

  _Use this after search to actually read a workflow instead of returning a truncated feed card._

  ```bash
  rundown-pp-cli show 89da5324-f822-4a4b-a30e-b33cfac60a95
  ```

## Recipes

### Best workflows this week

```bash
rundown-pp-cli top --since 7d --limit 5
```

Windows the local mirror by createdAt and ranks by upvotes - the query the live API cannot express.

### Does a use case exist for this?

```bash
rundown-pp-cli use-cases "invoice reconciliation" --limit 5
```

Runs the server's semantic search and local FTS together, de-duplicates, and ranks what comes back by upvotes.

### Narrow to one tool and read the winners

```bash
rundown-pp-cli posts --tool claude-code --sort top --limit 5 --agent --select id,title,upvoteCount
```

Server-side tool filtering with a trimmed agent payload - only three fields come back instead of full post bodies.

### What do people pair with n8n?

```bash
rundown-pp-cli stack n8n --limit 10
```

Self-joins the mirrored post-to-tool mapping to surface the stacks that co-occur in real workflows.

### Read one workflow end to end

```bash
rundown-pp-cli show 89da5324-f822-4a4b-a30e-b33cfac60a95
```

Reassembles the post and its comment thread into a single readable document, body included in full.

## Usage

Run `rundown-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RUNDOWN_CONFIG_DIR`, `RUNDOWN_DATA_DIR`, `RUNDOWN_STATE_DIR`, or `RUNDOWN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RUNDOWN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RUNDOWN_HOME=/srv/rundown
rundown-pp-cli doctor
```

Under `RUNDOWN_HOME=/srv/rundown`, the four dirs resolve to `/srv/rundown/config`, `/srv/rundown/data`, `/srv/rundown/state`, and `/srv/rundown/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rundown": {
      "command": "rundown-pp-mcp",
      "env": {
        "RUNDOWN_HOME": "/srv/rundown"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RUNDOWN_DATA_DIR` overrides an explicit `--home` for that kind. Use `RUNDOWN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RUNDOWN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `rundown-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### comments

Discussion threads attached to workflow posts

- **`rundown-pp-cli comments <post_id>`** - List comments on a workflow post

### leaderboard

Weekly community contributor leaderboard

- **`rundown-pp-cli leaderboard`** - This week's top contributors by points (server always returns the weekly window)

### posts

Community workflow posts

- **`rundown-pp-cli posts`** - List community workflow posts with server-side filters

### tools

Catalogue of AI tools referenced by community workflows

- **`rundown-pp-cli tools`** - List every tool slug the community can tag a workflow with


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rundown-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rundown-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rundown-pp-cli learnings list`** - Inspect taught rows
- **`rundown-pp-cli learnings forget <query>`** - Undo a teach
- **`rundown-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rundown-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rundown-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rundown-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RUNDOWN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rundown-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rundown-pp-cli comments mock-value

# JSON for scripting and agents
rundown-pp-cli comments mock-value --json
# Filter to specific fields
rundown-pp-cli comments mock-value --json --select id,postId,parentCommentId

# Dry run — show the request without sending
rundown-pp-cli comments mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rundown-pp-cli comments mock-value --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `RUNDOWN_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `rundown-pp-cli leaderboard`
- `rundown-pp-cli leaderboard get`
- `rundown-pp-cli leaderboard list`
- `rundown-pp-cli leaderboard search`
- `rundown-pp-cli posts`
- `rundown-pp-cli posts get`
- `rundown-pp-cli posts list`
- `rundown-pp-cli posts search`
- `rundown-pp-cli tools`
- `rundown-pp-cli tools get`
- `rundown-pp-cli tools list`
- `rundown-pp-cli tools search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
rundown-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `rundown-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rundown-pp-cli/config.toml`; `--home`, `RUNDOWN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **top / digest / stack return nothing** — Run `rundown-pp-cli sync` first - these commands read the local mirror, not the live API.
- **`--limit` above 50 silently returns only 50 rows** — The server caps page size at 50. Use `sync` plus the local commands when you need the whole corpus.
- **`--tool` or `--industry` returns a 400 'not recognized' error** — Pass the slug, not the display name. Run `rundown-pp-cli tools` for valid tool slugs.
- **`leaderboard --period month` still returns weekly data** — Known upstream behaviour - the API ignores the period parameter and always returns the weekly window.
