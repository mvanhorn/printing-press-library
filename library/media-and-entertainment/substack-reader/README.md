# Substack Reader CLI

**Read any Substack publication as a local, full-text-searchable corpus — keyless for free posts, your own session for what you subscribe to.**

Substack Reader archives whole publications into a local SQLite mirror you can search, SQL-query, and read offline. Free posts need no login; paid posts you're entitled to unlock with your own session cookie — never redistributed, always opt-in. Unlike every other Substack tool it builds a corpus that compounds instead of fetching live per call.

## Install

The recommended path installs both the `substack-reader-pp-cli` binary and the `pp-substack-reader` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install substack-reader
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install substack-reader --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install substack-reader --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install substack-reader --agent claude-code
npx -y @mvanhorn/printing-press-library install substack-reader --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/cmd/substack-reader-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-reader-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install substack-reader --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-substack-reader --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-substack-reader --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install substack-reader --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-reader-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/cmd/substack-reader-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "substack-reader": {
      "command": "substack-reader-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Free/public posts are keyless — zero setup. To read paid posts you already subscribe to, provide your own Substack session cookie (substack.sid); this reads only what you are already entitled to and is never required for free content.

## Quick Start

```bash
# confirm reachability and config before anything else
substack-reader-pp-cli doctor --dry-run

# mirror a publication's recent posts into the local corpus
substack-reader-pp-cli archive astralcodexten --limit 50

# full-text search the corpus offline
substack-reader-pp-cli search "open thread"

# read a single post's full text
substack-reader-pp-cli read astralcodexten/open-thread-441

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local corpus that compounds
- **`archive`** — Archive a whole Substack publication into a local SQLite mirror you can read, search, and query offline — no other Substack tool builds a persistent corpus.

  _Reach for this to turn a live newsletter into a durable, queryable knowledge base instead of re-fetching every time._

  ```bash
  substack-reader-pp-cli archive astralcodexten --limit 200
  ```
- **`sql`** — Run read-only SQL over your local Substack corpus for arbitrary analytics — post cadence, audience mix, longest posts — from data you've already archived.

  _Reach for this for ad-hoc analytics over what you've archived, without re-fetching or writing code._

  ```bash
  substack-reader-pp-cli sql "SELECT audience, count(*) FROM posts GROUP BY audience"
  ```

### Entitlement-aware reading
- **`read`** — Read a post's full text; free posts keyless, and paid posts you subscribe to via your own session cookie — with an honest 'preview only, you're not entitled' signal.

  _Use to pull a specific post's full text into an agent workflow, respecting exactly what the user is entitled to._

  ```bash
  substack-reader-pp-cli read astralcodexten/open-thread-441
  ```

### Topic & comparative intelligence
- **`digest`** — A time-windowed digest across every publication in your local corpus — what's new since you last synced, ranked, in one view.

  _Use as a personal 'what did I miss across my newsletters' briefing._

  ```bash
  substack-reader-pp-cli digest --since 7d
  ```
- **`author-compare`** — Compare two publications' cadence, topics, and free/paid mix from the local corpus.

  _Use to size up a newsletter before subscribing, or to study what a successful author publishes._

  ```bash
  substack-reader-pp-cli author-compare astralcodexten blog.bytebytego.com
  ```

## Recipes

### Build a searchable corpus

```bash
substack-reader-pp-cli archive astralcodexten --limit 200 && substack-reader-pp-cli search "prediction markets"
```

Mirror a publication then search it offline with FTS ranking.

### Narrow a large post to fields

```bash
substack-reader-pp-cli read astralcodexten/open-thread-441 --agent --select title,post_date,audience,body_html
```

Pull only the fields an agent needs from a verbose post object.

### Audience mix analytics

```bash
substack-reader-pp-cli sql "SELECT audience, count(*) FROM posts GROUP BY audience"
```

Read-only SQL over the local corpus for arbitrary analytics.

## Usage

Run `substack-reader-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SUBSTACK_READER_CONFIG_DIR`, `SUBSTACK_READER_DATA_DIR`, `SUBSTACK_READER_STATE_DIR`, or `SUBSTACK_READER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SUBSTACK_READER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SUBSTACK_READER_HOME=/srv/substack-reader
substack-reader-pp-cli doctor
```

Under `SUBSTACK_READER_HOME=/srv/substack-reader`, the four dirs resolve to `/srv/substack-reader/config`, `/srv/substack-reader/data`, `/srv/substack-reader/state`, and `/srv/substack-reader/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "substack-reader": {
      "command": "substack-reader-pp-mcp",
      "env": {
        "SUBSTACK_READER_HOME": "/srv/substack-reader"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SUBSTACK_READER_DATA_DIR` overrides an explicit `--home` for that kind. Use `SUBSTACK_READER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SUBSTACK_READER_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `substack-reader-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### categories

Browse Substack's publication categories

- **`substack-reader-pp-cli categories browse`** - List publications in a category
- **`substack-reader-pp-cli categories list`** - List all Substack categories

### publications

Discover Substack publications

- **`substack-reader-pp-cli publications <query>`** - Search Substack publications by name (best-effort; may return few results anonymously)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`substack-reader-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`substack-reader-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`substack-reader-pp-cli learnings list`** - Inspect taught rows
- **`substack-reader-pp-cli learnings forget <query>`** - Undo a teach
- **`substack-reader-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`substack-reader-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`substack-reader-pp-cli teach-pattern`** - Install a query/resource template up front
- **`substack-reader-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SUBSTACK_READER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `substack-reader-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
substack-reader-pp-cli categories list

# JSON for scripting and agents
substack-reader-pp-cli categories list --json

# Filter to specific fields
substack-reader-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
substack-reader-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
substack-reader-pp-cli categories list --agent
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
substack-reader-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `substack-reader-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/substack-reader-pp-cli/config.toml`; `--home`, `SUBSTACK_READER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A paid post reads as preview-only despite being subscribed** — Check that your substack.sid session cookie is set (SUBSTACK_SESSION or the config cookie file) and current — an expired or mistyped cookie silently falls back to the public preview. The reader unlocks paid posts through Substack's authenticated by-id endpoint, so a valid session is all that is needed; you never target a particular host yourself.
- **Empty results / HTTP 429** — You are being rate-limited; slow down (archive is self-throttled) — Substack sets no rate headers, so pace requests.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Substack2Markdown**](https://github.com/timf34/Substack2Markdown) — Python (492 stars)
- [**sbstck-dl**](https://github.com/alexferrari88/sbstck-dl) — Go (224 stars)
- [**substack_api**](https://github.com/NHagar/substack_api) — Python (217 stars)
- [**mcp-writer-substack**](https://github.com/jonathan-politzki/mcp-writer-substack) — Python (31 stars)
- [**substack_mcp**](https://github.com/dkyazzentwatwa/substack_mcp) — Python (13 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
