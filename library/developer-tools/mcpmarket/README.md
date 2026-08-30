# MCP Market CLI

**Every MCP Market search, leaderboard, and category browse — plus trending deltas, snapshot diffs, and author portfolios no other tool tracks.**

MCP Market has no official CLI today. This one mirrors the public catalog (servers, clients, Agent Skills) locally as you browse it, and because it's stateful across runs, it can answer questions the live website simply cannot: what's trending fastest, what changed since last week, and what one author has shipped across every entity type.

## Install

The recommended path installs both the `mcpmarket-pp-cli` binary and the `pp-mcpmarket` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket --agent claude-code
npx -y @mvanhorn/printing-press-library install mcpmarket --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/cmd/mcpmarket-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mcpmarket-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mcpmarket --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mcpmarket --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install mcpmarket --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mcpmarket-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/cmd/mcpmarket-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mcpmarket": {
      "command": "mcpmarket-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI can reach mcpmarket.com before doing anything else
mcpmarket-pp-cli doctor --dry-run

# search the live catalog directly, no setup required
mcpmarket-pp-cli server search "web scraping" --json

# look up full detail on one server
mcpmarket-pp-cli server get firecrawl --json

# browse servers — each browse also mirrors results locally
mcpmarket-pp-cli server list --json

# after browsing on two different days, see what's rising fastest
mcpmarket-pp-cli trending --since 7d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`trending`** — See which MCP servers/skills are rising fastest vs. holding steady, computed from your own browse history.

  _Use this before picking a tool when momentum matters more than raw popularity._

  ```bash
  mcpmarket-pp-cli trending --since 7d --json
  ```
- **`diff`** — See exactly which servers, clients, or skills were added, removed, or changed between two snapshots.

  _Use this to audit what changed in the catalog over a release cycle._

  ```bash
  mcpmarket-pp-cli diff --from 2026-08-01 --to 2026-08-27 --json
  ```
- **`author`** — See everything one GitHub org has published across servers, skills, and clients in one view.

  _Use this to evaluate an author's full footprint before trusting one of their servers._

  ```bash
  mcpmarket-pp-cli author mendableai --json
  ```
- **`leaderboard`** — See what the top-100 leaderboard looked like on a past date, not just right now.

  _Use this to answer 'what was trending three weeks ago' without external archives._

  ```bash
  mcpmarket-pp-cli leaderboard --as-of 2026-08-01 --json
  ```
- **`watch category`** — Flag newly-appeared listings in a category since your last check.

  _Use this to stay current on a category you track without re-browsing it manually._

  ```bash
  mcpmarket-pp-cli watch category "developer-tools" --json
  ```
- **`dedupe`** — Surface near-identical listings from different authors within a category.

  _Use this to avoid installing three servers that all do the same thing._

  ```bash
  mcpmarket-pp-cli dedupe --category api-development --json
  ```

### Agent-native plumbing
- **`stack`** — Chain the similar-tools recommendation graph multiple hops to build a ranked shortlist around a server you already use.

  _Use this to discover a full toolchain, not just one related item at a time._

  ```bash
  mcpmarket-pp-cli stack firecrawl --depth 2 --json
  ```

## Recipes

### Find the best PDF tool and see what it's related to

```bash
mcpmarket-pp-cli server search "pdf" --json --select 0.name,0.url
```

narrow a broad search then feed a result's slug into stack

### Check who's trending in the last week

```bash
mcpmarket-pp-cli trending --since 7d --json --select 0.name,0.delta
```

select just the fields that matter to avoid parsing a verbose payload

### See a server's full detail

```bash
mcpmarket-pp-cli server get firecrawl --json
```

one call returns description, features, author, and related items

### Build a toolchain around a server you already use

```bash
mcpmarket-pp-cli stack firecrawl --depth 2 --json
```

walks the similar-tools graph two hops out

### Audit an author's full footprint

```bash
mcpmarket-pp-cli author mendableai --json
```

see every server, client, and skill one GitHub org has published (from what's been browsed locally)

## Usage

Run `mcpmarket-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MCPMARKET_CONFIG_DIR`, `MCPMARKET_DATA_DIR`, `MCPMARKET_STATE_DIR`, or `MCPMARKET_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MCPMARKET_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MCPMARKET_HOME=/srv/mcpmarket
mcpmarket-pp-cli doctor
```

Under `MCPMARKET_HOME=/srv/mcpmarket`, the four dirs resolve to `/srv/mcpmarket/config`, `/srv/mcpmarket/data`, `/srv/mcpmarket/state`, and `/srv/mcpmarket/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "mcpmarket": {
      "command": "mcpmarket-pp-mcp",
      "env": {
        "MCPMARKET_HOME": "/srv/mcpmarket"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MCPMARKET_DATA_DIR` overrides an explicit `--home` for that kind. Use `MCPMARKET_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MCPMARKET_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `mcpmarket-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### category

Catalog categories

- **`mcpmarket-pp-cli category`** - List all catalog categories

### mcpclient

MCP clients (apps like Zed, Cline, Cherry Studio) listed on MCP Market

- **`mcpmarket-pp-cli mcpclient <slug>`** - Get details for one MCP client

### server

MCP servers listed on MCP Market

- **`mcpmarket-pp-cli server get`** - Get details for one MCP server
- **`mcpmarket-pp-cli server similar`** - Related/similar servers or skills

### skill

Agent Skills listed on MCP Market

- **`mcpmarket-pp-cli skill <slug>`** - Get details for one Agent Skill


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`mcpmarket-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`mcpmarket-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`mcpmarket-pp-cli learnings list`** - Inspect taught rows
- **`mcpmarket-pp-cli learnings forget <query>`** - Undo a teach
- **`mcpmarket-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`mcpmarket-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`mcpmarket-pp-cli teach-pattern`** - Install a query/resource template up front
- **`mcpmarket-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MCPMARKET_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `mcpmarket-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mcpmarket-pp-cli category

# JSON for scripting and agents
mcpmarket-pp-cli category --json
# Filter to specific fields by name
mcpmarket-pp-cli category --json --select <field>[,<field>...]

# Dry run — show the request without sending
mcpmarket-pp-cli category --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mcpmarket-pp-cli category --agent
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
mcpmarket-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `mcpmarket-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `MCPMARKET_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **requests to mcpmarket.com return errors or empty pages** — the site sits behind a Vercel bot-protection checkpoint; this CLI ships Chrome-fingerprint HTTP transport by default — no action needed, but retry if you see transient 429s
- **trending/diff/author/watch commands return no data or a 'not enough history' note** — these compare against a local snapshot from browsing (server list/search/category/leaderboard/daily). Browse the catalog at least twice, on different days, then retry

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `MCPMARKET_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
