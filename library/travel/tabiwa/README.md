# tabiwa by WESTER catalog CLI

**Compare regional tabiwa catalog products with payment units and original restriction evidence.**

Inspect published catalog summaries and requested-date membership. Full ticket terms, route coverage and stock remain unknown; finish on the canonical product page.

Generic root `search` uses local FTS over geography populated by `sync`; auto/local never perform provider text search, and explicit live is unsupported. Use `catalog search` for regional products. MCP search/SQL use that geography store. Save-capable catalog MCP tools are local writes because `save=true` persists selected private evidence; provider requests remain GET-only. Geography and saved-evidence tools remain reads.

## Install

The recommended path installs both the `tabiwa-pp-cli` binary and the `pp-tabiwa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tabiwa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tabiwa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tabiwa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tabiwa --agent claude-code
npx -y @mvanhorn/printing-press-library install tabiwa --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tabiwa/cmd/tabiwa-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tabiwa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tabiwa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tabiwa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tabiwa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tabiwa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tabiwa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. This native bundle contains only macOS Apple Silicon (`darwin-arm64`) peers. Other platforms need separately built target binaries or bundles; use the manual config below with a matching executable.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tabiwa/cmd/tabiwa-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tabiwa": {
      "command": "tabiwa-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public catalog reads need no login. The only sent cookie is the documented regionId display preference. Queue-it full detail pages are excluded from runtime access.

## Quick Start

```bash
# Start with a bounded dated catalog shortlist.
tabiwa-pp-cli catalog search --region 20 --category transportation --on 2026-10-28 --limit 3 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Catalog decisions
- **`catalog search`** — Discover regional products with a dated catalog filter and explicit scanned coverage.

  _Discover regional products with a dated catalog filter and explicit scanned coverage._

  ```bash
  tabiwa-pp-cli catalog search --region 20 --category transportation --on 2026-10-28 --limit 3 --agent
  ```
- **`catalog compare`** — Compare points-only and yen-quoted products without inventing currency conversion.

  _Compare points-only and yen-quoted products without inventing currency conversion._

  ```bash
  tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --agent
  ```
- **`catalog inspect`** — Read original Japanese restriction and redemption hints while complete terms remain unknown.

  _Read original Japanese restriction and redemption hints while complete terms remain unknown._

  ```bash
  tabiwa-pp-cli catalog inspect J0000900 --region 10 --agent
  ```
- **`catalog compare`** — Show whether selected products appear in a requested-date catalog without claiming ticket stock.

  _Show whether selected products appear in a requested-date catalog without claiming ticket stock._

  ```bash
  tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --on 2026-10-28 --agent
  ```
- **`catalog saved`** — Read saved selected products offline with their original observation times.

  _Read saved selected products offline with their original observation times._

  ```bash
  tabiwa-pp-cli catalog saved --region 10 --limit 3 --agent
  ```

## Recipes

### Payment units

```bash
tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --agent
```

Compare points-only and yen-quoted products without inventing currency conversion.

### Source restrictions

```bash
tabiwa-pp-cli catalog inspect J0000900 --region 10 --agent
```

Read original Japanese restriction and redemption hints while complete terms remain unknown.

### Compact discovery

```bash
tabiwa-pp-cli catalog search --region 20 --category transportation --limit 3 --agent --select products.id,products.name,products.price,coverage
```

Keep identity, unit-qualified quotes and scan coverage.

## Usage

Run `tabiwa-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.json` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TABIWA_CONFIG_DIR`, `TABIWA_DATA_DIR`, `TABIWA_STATE_DIR`, or `TABIWA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TABIWA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TABIWA_HOME=/srv/tabiwa
tabiwa-pp-cli doctor
```

Under `TABIWA_HOME=/srv/tabiwa`, the four dirs resolve to `/srv/tabiwa/config`, `/srv/tabiwa/data`, `/srv/tabiwa/state`, and `/srv/tabiwa/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "tabiwa": {
      "command": "tabiwa-pp-mcp",
      "env": {
        "TABIWA_HOME": "/srv/tabiwa"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TABIWA_DATA_DIR` overrides an explicit `--home` for that kind. Use `TABIWA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TABIWA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `tabiwa-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### catalog

Inspect published ticket catalog summaries and their limits

- **`tabiwa-pp-cli catalog`** - Discover catalog summaries with regional and date filters; not live stock

### geography

Resolve provider region-specific prefecture and area identifiers

- **`tabiwa-pp-cli geography`** - List source prefecture and area IDs for a selected regional catalog


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`tabiwa-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`tabiwa-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`tabiwa-pp-cli learnings list`** - Inspect taught rows
- **`tabiwa-pp-cli learnings forget <query>`** - Undo a teach
- **`tabiwa-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`tabiwa-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`tabiwa-pp-cli teach-pattern`** - Install a query/resource template up front
- **`tabiwa-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TABIWA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `tabiwa-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tabiwa-pp-cli catalog

# JSON for scripting and agents
tabiwa-pp-cli catalog --json
# Filter to specific fields
tabiwa-pp-cli catalog --json --select id,name,price

# Dry run — show the request without sending
tabiwa-pp-cli catalog --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tabiwa-pp-cli catalog --agent
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
tabiwa-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `tabiwa-pp-cli doctor` to see the resolved config, data, state, and cache directories. The settings file is `config.json` inside the resolved config directory; `--home`, `TABIWA_HOME`, and per-kind env vars can relocate it.

Public catalog reads need no credential file. The source client sends only the documented regional preference cookie; it does not import account or queue credentials.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `catalog search` or `geography list` to resolve product or provider place IDs

### API-specific
- **Source queue, HTML challenge or network error** — Stop and use the canonical browser link; do not retry around access controls.
