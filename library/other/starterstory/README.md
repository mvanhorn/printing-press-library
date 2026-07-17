# Starter Story CLI

**Every Starter Story case study, idea, and business indexed locally — rank founder stories by revenue, search 13k+ entries offline, and diff what's new, none of which the site itself can do.**

Starter Story has no API and weak on-site filtering. This CLI builds a local SQLite index from the full sitemap so you can rank case studies by monthly revenue (top-revenue), search with section and revenue filters offline (search), and see what's new since your last sync (whats-new).

Learn more at [Starter Story](https://www.starterstory.com).

Created by [@waveriderai](https://github.com/waveriderai) (waveriderai).

## Install

The recommended path installs both the `starterstory-pp-cli` binary and the `pp-starterstory` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install starterstory
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install starterstory --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install starterstory --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install starterstory --agent claude-code
npx -y @mvanhorn/printing-press-library install starterstory --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/starterstory/cmd/starterstory-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/starterstory-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install starterstory --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-starterstory --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-starterstory --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install starterstory --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/starterstory-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/starterstory/cmd/starterstory-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "starterstory": {
      "command": "starterstory-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the site is reachable before syncing
starterstory-pp-cli doctor --dry-run

# build the local index from the sitemap (all sections)
starterstory-pp-cli index

# the headline: highest-revenue case studies
starterstory-pp-cli top-revenue --limit 20

# filtered offline search
starterstory-pp-cli hunt saas --section stories --min-revenue 20000

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local index that compounds
- **`top-revenue`** — Rank founder case studies by monthly revenue parsed from their titles.

  _Pick this to find the highest-revenue proven case studies without scrolling the site._

  ```bash
  starterstory-pp-cli top-revenue --limit 20 --agent
  ```
- **`whats-new`** — Diff the sitemap against your last sync to surface newly published stories, ideas, and businesses.

  _Track the StarterStory feed offline between syncs._

  ```bash
  starterstory-pp-cli whats-new --agent
  ```
- **`hunt`** — Full-text search the local index with section and minimum-revenue filters (hunt = filtered search).

  _Reach for this instead of the weak on-site search when you need filtered, structured results._

  ```bash
  starterstory-pp-cli hunt saas --section stories --min-revenue 20000 --agent
  ```
- **`grep`** — Keyword-filter across all indexed idea and story slugs offline.

  _Fast local keyword scan across the whole idea corpus._

  ```bash
  starterstory-pp-cli grep newsletter --agent
  ```
- **`stats`** — Counts by section plus the revenue distribution across the case-study corpus.

  _Understand the shape of the StarterStory corpus at a glance._

  ```bash
  starterstory-pp-cli stats --agent
  ```

## Recipes

### Highest-revenue SaaS case studies

```bash
starterstory-pp-cli hunt saas --section stories --min-revenue 50000 --agent --select slug,title,revenue
```

Filter the local index to SaaS stories over $50K/month and return only the key fields.

### Read one case study

```bash
starterstory-pp-cli stories i-turned-my-hobby-into-120k-month-apps --json
```

Fetch and extract a single case-study page.

### Browse a curated idea list

```bash
starterstory-pp-cli data micro-saas-ideas --json
```

Extract the linked items from a /data category page.

## Usage

Run `starterstory-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `STARTERSTORY_CONFIG_DIR`, `STARTERSTORY_DATA_DIR`, `STARTERSTORY_STATE_DIR`, or `STARTERSTORY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `STARTERSTORY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export STARTERSTORY_HOME=/srv/starterstory
starterstory-pp-cli doctor
```

Under `STARTERSTORY_HOME=/srv/starterstory`, the four dirs resolve to `/srv/starterstory/config`, `/srv/starterstory/data`, `/srv/starterstory/state`, and `/srv/starterstory/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "starterstory": {
      "command": "starterstory-pp-mcp",
      "env": {
        "STARTERSTORY_HOME": "/srv/starterstory"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `STARTERSTORY_DATA_DIR` overrides an explicit `--home` for that kind. Use `STARTERSTORY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `STARTERSTORY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `starterstory-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### breakdowns

Business / marketing breakdowns

- **`starterstory-pp-cli breakdowns <slug>`** - Fetch a breakdown page

### businesses

Real business profiles

- **`starterstory-pp-cli businesses <slug>`** - Fetch a business profile page

### data

Curated idea-list pages (e.g. micro-saas-ideas, gpt-wrapper-ideas)

- **`starterstory-pp-cli data <category>`** - Fetch a curated /data/<category> list page and extract its linked detail items

### ideas

Business idea pages

- **`starterstory-pp-cli ideas <slug>`** - Fetch a business idea page

### stories

Founder case studies (revenue baked into the page title)

- **`starterstory-pp-cli stories <slug>`** - Fetch a case study page (title, revenue, description, image, body)

### tools

Tool pages

- **`starterstory-pp-cli tools <slug>`** - Fetch a tool page


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`starterstory-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`starterstory-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`starterstory-pp-cli learnings list`** - Inspect taught rows
- **`starterstory-pp-cli learnings forget <query>`** - Undo a teach
- **`starterstory-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`starterstory-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`starterstory-pp-cli teach-pattern`** - Install a query/resource template up front
- **`starterstory-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `STARTERSTORY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `starterstory-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
starterstory-pp-cli breakdowns mock-value

# JSON for scripting and agents
starterstory-pp-cli breakdowns mock-value --json

# Filter to specific fields
starterstory-pp-cli breakdowns mock-value --json --select id,name,status

# Dry run — show the request without sending
starterstory-pp-cli breakdowns mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
starterstory-pp-cli breakdowns mock-value --agent
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
starterstory-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `starterstory-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/starterstory-pp-cli/config.toml`; `--home`, `STARTERSTORY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search or top-revenue returns nothing** — run 'starterstory-pp-cli index' first to build the local index
- **a story has no revenue in top-revenue** — revenue is parsed heuristically from the title; stories without a $/month title are ranked last
