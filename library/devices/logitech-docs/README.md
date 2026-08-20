# Logitech CLI

**Every Logitech manual, spec sheet, and install guide — searchable offline, agent-native, no browser.**

support.logi.com is a Zendesk portal with a clean JSON API hiding behind it. logitech-docs turns 33k+ reference documents into a greppable local index: find specs by product, pull install guides, and full-text search inside manuals without touching a browser.

## Install

The recommended path installs both the `logitech-docs-pp-cli` binary and the `pp-logitech-docs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs --agent claude-code
npx -y @mvanhorn/printing-press-library install logitech-docs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/cmd/logitech-docs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/logitech-docs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-logitech-docs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-logitech-docs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install logitech-docs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/logitech-docs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/cmd/logitech-docs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "logitech-docs": {
      "command": "logitech-docs-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. The support site and its Help Center API are public.

## Quick Start

```bash
# health check, works without credentials
logitech-docs-pp-cli doctor --dry-run

# see the 9 support categories
logitech-docs-pp-cli categories list

# search the live docs
logitech-docs-pp-cli articles search --query "MeetUp"

# pull the spec sheet directly
logitech-docs-pp-cli docs spec "MeetUp"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Doc discovery
- **`docs`** — Search Logitech support docs by friendly type: spec sheet, manual, install guide, FAQ, download, warranty.

  _Agents reach for this instead of guessing Zendesk label strings for spec sheets vs install guides._

  ```bash
  logitech-docs-pp-cli docs spec "MeetUp" --agent
  ```
- **`download`** — Resolve download01.logi.com file links in an article and fetch them (with --dry-run and local cache).

  _Scriptable downloads of firmware/software/PDF manuals without clicking through the site._

  ```bash
  logitech-docs-pp-cli download 360023302754 --dry-run
  ```

### Local state that compounds
- **`find`** — Full-text search inside synced manuals and spec sheets from the local store.

  _Search manuals offline, across a product family at once, without the Cloudflare-fronted site._

  ```bash
  logitech-docs-pp-cli find "camera dimensions" --agent
  ```
- **`compare`** — Side-by-side spec comparison between two products from synced spec sheets.

  _AV integrators compare dimensions and compatibility across products before quoting._

  ```bash
  logitech-docs-pp-cli compare "MeetUp" "Rally Bar" --agent
  ```

## Recipes

### Grab a spec sheet

```bash
logitech-docs-pp-cli docs spec "MeetUp" --select title,html_url
```

narrow a large search response to just name and link

### Full-text search inside manuals

```bash
logitech-docs-pp-cli docs manual "pairing" --agent --select title,html_url
```

live full-text search across manual bodies, narrowed for agents

### Preview a download

```bash
logitech-docs-pp-cli download 360023302754 --dry-run
```

see the resolved file URL before fetching

## Usage

Run `logitech-docs-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LOGITECH_DOCS_CONFIG_DIR`, `LOGITECH_DOCS_DATA_DIR`, `LOGITECH_DOCS_STATE_DIR`, or `LOGITECH_DOCS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LOGITECH_DOCS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LOGITECH_DOCS_HOME=/srv/logitech-docs
logitech-docs-pp-cli doctor
```

Under `LOGITECH_DOCS_HOME=/srv/logitech-docs`, the four dirs resolve to `/srv/logitech-docs/config`, `/srv/logitech-docs/data`, `/srv/logitech-docs/state`, and `/srv/logitech-docs/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "logitech-docs": {
      "command": "logitech-docs-pp-mcp",
      "env": {
        "LOGITECH_DOCS_HOME": "/srv/logitech-docs"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LOGITECH_DOCS_DATA_DIR` overrides an explicit `--home` for that kind. Use `LOGITECH_DOCS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LOGITECH_DOCS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `logitech-docs-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### articles

Support articles: manuals, spec sheets, install guides, FAQs, downloads

- **`logitech-docs-pp-cli articles by-section`** - List articles in one section (server-side scoped)
- **`logitech-docs-pp-cli articles get`** - Get a single article including its HTML body
- **`logitech-docs-pp-cli articles list`** - List articles (filter by label)
- **`logitech-docs-pp-cli articles search`** - Search articles by text

### categories

Support categories (Mice and Pointers, Keyboards, Webcams, Gaming, ...)

- **`logitech-docs-pp-cli categories`** - List support categories

### sections

Product-family sections within the support site

- **`logitech-docs-pp-cli sections`** - List sections (optionally filtered by category)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`logitech-docs-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`logitech-docs-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`logitech-docs-pp-cli learnings list`** - Inspect taught rows
- **`logitech-docs-pp-cli learnings forget <query>`** - Undo a teach
- **`logitech-docs-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`logitech-docs-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`logitech-docs-pp-cli teach-pattern`** - Install a query/resource template up front
- **`logitech-docs-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `LOGITECH_DOCS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `logitech-docs-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
logitech-docs-pp-cli articles list

# JSON for scripting and agents
logitech-docs-pp-cli articles list --json
# Filter to specific fields
logitech-docs-pp-cli articles list --json --select id,name,title

# Dry run — show the request without sending
logitech-docs-pp-cli articles list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
logitech-docs-pp-cli articles list --agent
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

## Health Check

```bash
logitech-docs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `logitech-docs-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `LOGITECH_DOCS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns 404 / RecordNotFound** — use the locale-free search form (articles search), not a locale-prefixed path
- **empty article body in list output** — list endpoints omit body; use articles get <id> for the full HTML
