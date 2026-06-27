# Vibe Signal CLI

**One question, every low-friction source: a recency-aware, cited signal report instead of ten raw source dumps.**

Vibe Signal composes the catalog's source surfaces into a single editorial research loop. Ask what people are saying now about a topic and get themes backed by raw evidence (report), pull the citable items behind a claim (evidence), and see which sources are covered (sources list). v1 ships the no-auth sources: Hacker News and Techmeme.

## Install

The recommended path installs both the `vibe-signal-pp-cli` binary and the `pp-vibe-signal` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal --agent claude-code
npx -y @mvanhorn/printing-press-library install vibe-signal --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/cmd/vibe-signal-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vibe-signal-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vibe-signal --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vibe-signal --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install vibe-signal --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vibe-signal-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/cmd/vibe-signal-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vibe-signal": {
      "command": "vibe-signal-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI is wired and sources are reachable
vibe-signal-pp-cli doctor --dry-run

# see which sources are included and their auth needs
vibe-signal-pp-cli sources list

# the core workflow: a cross-source signal report
vibe-signal-pp-cli report "AI browser agents" --window 30d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Composed editorial workflow
- **`report`** — Ask one question across Hacker News and Techmeme and get a recency-aware signal report with per-source coverage.

  _Reach for this when you need the current conversation on a topic across sources, not one source's raw dump._

  ```bash
  vibe-signal-pp-cli report "AI browser agents" --window 30d --json
  ```
- **`evidence`** — List the raw, cited items (post URL, author, timestamp, points, comments) backing a topic from a chosen source.

  _Reach for this when you need to quote or link real posts behind a claim, not a paraphrase._

  ```bash
  vibe-signal-pp-cli evidence "AI browser agents" --source hackernews --limit 20 --json
  ```
- **`sources list`** — Show which sources are wired in, their auth needs, and which command syncs them.

  _Reach for this to see coverage and auth requirements before running a report._

  ```bash
  vibe-signal-pp-cli sources list --json
  ```

## Recipes

### Cross-source report as JSON for ranking

```bash
vibe-signal-pp-cli report "local-first software" --window 14d --json --select query,themes,coverage
```

Narrow the report envelope to the fields a downstream ranker needs.

### Pull citable HN evidence

```bash
vibe-signal-pp-cli evidence "local-first software" --source hackernews --limit 15 --json
```

Get raw items (url, author, points, comments) behind the topic.

### Check coverage before reporting

```bash
vibe-signal-pp-cli sources list --json
```

Confirm which sources are wired and free before running a report.

## Usage

Run `vibe-signal-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `VIBE_SIGNAL_CONFIG_DIR`, `VIBE_SIGNAL_DATA_DIR`, `VIBE_SIGNAL_STATE_DIR`, or `VIBE_SIGNAL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `VIBE_SIGNAL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export VIBE_SIGNAL_HOME=/srv/vibe-signal
vibe-signal-pp-cli doctor
```

Under `VIBE_SIGNAL_HOME=/srv/vibe-signal`, the four dirs resolve to `/srv/vibe-signal/config`, `/srv/vibe-signal/data`, `/srv/vibe-signal/state`, and `/srv/vibe-signal/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "vibe-signal": {
      "command": "vibe-signal-pp-mcp",
      "env": {
        "VIBE_SIGNAL_HOME": "/srv/vibe-signal"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `VIBE_SIGNAL_DATA_DIR` overrides an explicit `--home` for that kind. Use `VIBE_SIGNAL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `VIBE_SIGNAL_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `vibe-signal-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### hn

Hacker News source: search (Algolia) and item lookup (Firebase)

- **`vibe-signal-pp-cli hn item`** - Get a single HN item (story/comment) with score and comment count
- **`vibe-signal-pp-cli hn relevance`** - Search HN by relevance (popularity-ranked) for a topic
- **`vibe-signal-pp-cli hn stories`** - Search HN stories by recency for a topic


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vibe-signal-pp-cli hn item mock-value

# JSON for scripting and agents
vibe-signal-pp-cli hn item mock-value --json

# Filter to specific fields
vibe-signal-pp-cli hn item mock-value --json --select id,name,status

# Dry run — show the request without sending
vibe-signal-pp-cli hn item mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vibe-signal-pp-cli hn item mock-value --agent
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
vibe-signal-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `vibe-signal-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `VIBE_SIGNAL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **report returns no evidence** — run `vibe-signal-pp-cli sources sync` first, or widen --window
- **a source shows partial/failed coverage** — check `sources list`; the source may be rate-limited — rerun shortly

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hackernews-pp-cli**](https://github.com/mvanhorn/printing-press-library/tree/main/library/media-and-entertainment/hackernews) — Go
- [**techmeme-pp-cli**](https://github.com/mvanhorn/printing-press-library/tree/main/library/productivity/techmeme) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
