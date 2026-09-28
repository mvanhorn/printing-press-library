# Serply CLI

**Live Google, News, Scholar, Maps and Bing results, plus rank checks, SERP diffs and cited research briefs.**

Nine Serply search verticals as typed commands and MCP tools, with per-country proxies and device emulation on each. On top, rank finds a domain's position for a query, serp diff reports what moved since the last run, and research merges web, news and scholar results into one cited brief.

Created by [@googio](https://github.com/googio) (googio).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `serply-pp-cli` binary and the `pp-serply` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install serply
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install serply --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install serply --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install serply --agent claude-code
npx -y @mvanhorn/printing-press-library install serply --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/serply/cmd/serply-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/serply-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install serply --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-serply --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-serply --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install serply --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/serply-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SERPLY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/serply/cmd/serply-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "serply": {
      "command": "serply-pp-mcp",
      "env": {
        "SERPLY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check the install and config without spending credits
serply-pp-cli doctor --dry-run

# A first Google web search
serply-pp-cli web --q "model context protocol" --num 5

# Recent coverage from one Google News edition
serply-pp-cli news --q "model context protocol" --ceid US:en

# Where a domain ranks for a query
serply-pp-cli rank github.com --q "open source cli"

# A cited brief across web, news and scholar
serply-pp-cli research "retrieval augmented generation evaluation" --num 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### SEO checks
- **`rank`** — See the position of a domain for a query, optionally from a specific country, in one call.

  _Reach for this instead of a raw web search when the task is where a site ranks, not what the results are._

  ```bash
  serply-pp-cli rank github.com --q "open source cli" --x-proxy-location US --agent
  ```
- **`serp diff`** — See which URLs entered, left, or moved in a results page since the last time you ran the same query.

  _Use it for recurring monitoring of a query; the first run stores a baseline and later runs report only what changed._

  ```bash
  serply-pp-cli serp diff --q "best static site generator" --agent
  ```

### Agent research
- **`research`** — Get one deduplicated, numbered source list for a topic from web, news and scholar results at once.

  _Reach for this when an answer needs sources of more than one kind, such as current coverage plus papers._

  ```bash
  serply-pp-cli research "retrieval augmented generation evaluation" --num 5 --agent
  ```

## Recipes

### Narrow a web search to links

```bash
serply-pp-cli web --q "site:serply.io docs" --num 5 --agent --select results.title,results.link
```

Keeps agent context small by returning only titles and links.

### Rank from another country

```bash
serply-pp-cli rank serply.io --q "serp api" --x-proxy-location GB
```

Runs the search through a UK proxy and reports the first matching position.

### Watch a results page

```bash
serply-pp-cli serp diff --q "best static site generator"
```

First run stores a baseline; later runs list entered, left and moved URLs.

### Papers plus coverage

```bash
serply-pp-cli research "small language models" --num 5
```

One brief with numbered sources from web, news and scholar.

## Usage

Run `serply-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SERPLY_CONFIG_DIR`, `SERPLY_DATA_DIR`, `SERPLY_STATE_DIR`, or `SERPLY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SERPLY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SERPLY_HOME=/srv/serply
serply-pp-cli doctor
```

Under `SERPLY_HOME=/srv/serply`, the four dirs resolve to `/srv/serply/config`, `/srv/serply/data`, `/srv/serply/state`, and `/srv/serply/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "serply": {
      "command": "serply-pp-mcp",
      "env": {
        "SERPLY_HOME": "/srv/serply"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SERPLY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SERPLY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SERPLY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `serply-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### bing

Manage bing

- **`serply-pp-cli bing`** - Search Bing and return organic results.

### images

Manage images

- **`serply-pp-cli images`** - Search Google Images.

### job_search

Manage job search

- **`serply-pp-cli job-search`** - Search job listings indexed by Google Jobs.

### maps

Manage maps

- **`serply-pp-cli maps <query>`** - Search Google Maps for places. The query is a path segment.

### news

Manage news

- **`serply-pp-cli news`** - Search Google News and return recent articles.

### products

Manage products

- **`serply-pp-cli products`** - Search Amazon products with price, rating and review count.

### scholar

Manage scholar

- **`serply-pp-cli scholar`** - Search Google Scholar for papers, authors and citations.

### videos

Manage videos

- **`serply-pp-cli videos`** - Search Google Videos.

### web

Manage web

- **`serply-pp-cli web`** - Search Google and return organic results with title, link and description.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`serply-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`serply-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`serply-pp-cli learnings list`** - Inspect taught rows
- **`serply-pp-cli learnings forget <query>`** - Undo a teach
- **`serply-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`serply-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`serply-pp-cli teach-pattern`** - Install a query/resource template up front
- **`serply-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SERPLY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `serply-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
serply-pp-cli bing --q "model context protocol"

# JSON for scripting and agents
serply-pp-cli bing --q "model context protocol" --json
# Filter to specific fields
serply-pp-cli bing --q "model context protocol" --json --select description,link,realPosition

# Dry run — show the request without sending
serply-pp-cli bing --q "model context protocol" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
serply-pp-cli bing --q "model context protocol" --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
serply-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `serply-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/serply-pp-cli/config.toml`; `--home`, `SERPLY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SERPLY_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `serply-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `serply-pp-cli doctor` to check credentials
- Verify the environment variable is set without printing it: `test -n "$SERPLY_API_KEY" && echo set`
**Not found errors (exit code 3)**
- Check the command path; run `serply-pp-cli --help` for the list of verticals

### API-specific
- **401 or 403 on every call** — export SERPLY_API_KEY=<your key from serply.io>, then run serply-pp-cli doctor
- **Out of credits error** — Top up at https://serply.io; cached repeat queries stay free
- **Results look like the wrong country** — Pass --x-proxy-location GB (or another two-letter code) and --gl gb

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**crewAI Serply tools**](https://github.com/crewAIInc/crewAI) — Python (59104 stars)
- [**serply-inc/serply-python**](https://github.com/serply-inc/serply-python) — Python (1 stars)
- [**serply-inc/mcp**](https://github.com/serply-inc/mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
