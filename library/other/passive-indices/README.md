# Passive Indices CLI

**Unifies NSE's index data with the ETFs and index funds that track them — a join neither source site offers.**

niftyindices.com publishes index levels and constituents; indiapassivefunds.com publishes the ETFs and index funds that track those indices. Nothing links the two. This CLI joins them locally so you can ask "what tracks NIFTY 50, and how well" in one command — plus offline search, agent-native JSON, and a local SQLite layer for historical constituent diffs.

## Install

The recommended path installs both the `passive-indices-pp-cli` binary and the `pp-passive-indices` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install passive-indices
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install passive-indices --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install passive-indices --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install passive-indices --agent claude-code
npx -y @mvanhorn/printing-press-library install passive-indices --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/passive-indices/cmd/passive-indices-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/passive-indices-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install passive-indices --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-passive-indices --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-passive-indices --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install passive-indices --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/passive-indices-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/passive-indices/cmd/passive-indices-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "passive-indices": {
      "command": "passive-indices-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check — no auth needed for either source
passive-indices-pp-cli doctor --dry-run

# Populate the local store with NSE's index list (fund data is fetched live per command, not synced)
passive-indices-pp-cli sync --resources index

# See every fund tracking NIFTY 50
passive-indices-pp-cli index funds "NIFTY 50"

# Rank those funds by cost and fidelity
passive-indices-pp-cli index tracking "NIFTY 50" --json

# Narrow a fund-vs-index comparison to just the fields that matter
passive-indices-pp-cli compare 12345 "NIFTY 50" --select fund.expense_ratio,index.level

```

## Recipes

### Find the cheapest tracker for an index

```bash
passive-indices-pp-cli index cheapest-tracker "NIFTY 50" --json
```

Ranks all funds tracking NIFTY 50 by expense ratio, cheapest first.

### Narrow a deeply nested fund detail response

```bash
passive-indices-pp-cli fund raw 12345 --agent --select response.f_29,response.f_36
```

indiapassivefunds' raw responses are deeply nested and field-coded; --select keeps only the fields an agent actually needs.

### See what changed in an index's makeup this quarter

```bash
passive-indices-pp-cli index constituents-diff "NIFTY 50" --since 90d
```

Diffs the current constituent snapshot against one from 90 days ago using locally-synced history.

### Check sector concentration before recommending an index

```bash
passive-indices-pp-cli index sectors "NIFTY 50" --json
```

Aggregates constituent counts by sector to flag concentration risk.

### Compare a held fund against its benchmark

```bash
passive-indices-pp-cli compare 12345 "NIFTY 50"
```

Side-by-side view of a fund's NAV/AUM/expense against its underlying index's level and top constituents.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source joins
- **`index funds <index>`** — See every ETF and index fund that tracks a given NSE index in one call.

  _Use when an agent needs to enumerate the passive products tracking an index without visiting two unrelated websites._

  ```bash
  passive-indices-pp-cli index funds "NIFTY 50" --json
  ```
- **`index tracking <index>`** — Rank every fund tracking an index by cost and NAV fidelity against the index level.

  _Use for a ranked table of all funds tracking an index by fidelity/cost, not just a plain membership list._

  ```bash
  passive-indices-pp-cli index tracking "NIFTY 50" --json
  ```
- **`index cheapest-tracker <index>`** — Find the lowest-cost fund tracking a given index.

  _Use when a user asks which fund to buy to track a specific index at the lowest cost._

  ```bash
  passive-indices-pp-cli index cheapest-tracker "NIFTY 50" --json
  ```
- **`fund nfo tracking <index>`** — See upcoming New Fund Offers that track a specific index.

  _Use to spot genuinely novel fund launches for an index versus yet another me-too NIFTY 50 fund._

  ```bash
  passive-indices-pp-cli fund nfo tracking "NIFTY Next 50" --json
  ```
- **`compare <schemeId> <index>`** — See a single fund's NAV/AUM/expense next to its benchmark index's level and top constituents, side by side.

  _Use for a single fund vs single index side-by-side; use index tracking for ranking multiple funds against an index._

  ```bash
  passive-indices-pp-cli compare 12345 "NIFTY 50" --json
  ```

### Local history
- **`index constituents-diff <index> --since <date>`** — See what changed in an index's constituent list (additions/removals) between two sync snapshots.

  _Use to detect index rebalancing changes (additions, removals) over time._

  ```bash
  passive-indices-pp-cli index constituents-diff "NIFTY 50" --since 30d --json
  ```
- **`index sectors <index>`** — See an index's constituents grouped by sector, with real per-constituent and per-sector weights (niftyindices' live sector-weight feed — also covers strategy indices with no published constituent CSV).

  _Use to assess sector concentration risk in an index before recommending a tracker._

  ```bash
  passive-indices-pp-cli index sectors "NIFTY 50" --json
  ```

### Agent-native plumbing
- **`fund raw <schemeId>`** — See a fund's raw API response with cryptic field codes resolved to human-readable names.

  _Use when debugging or when an agent needs the full raw field set beyond the flattened default view._

  ```bash
  passive-indices-pp-cli fund raw 12345 --json
  ```

## Usage

Run `passive-indices-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PASSIVE_INDICES_CONFIG_DIR`, `PASSIVE_INDICES_DATA_DIR`, `PASSIVE_INDICES_STATE_DIR`, or `PASSIVE_INDICES_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PASSIVE_INDICES_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PASSIVE_INDICES_HOME=/srv/passive-indices
passive-indices-pp-cli doctor
```

Under `PASSIVE_INDICES_HOME=/srv/passive-indices`, the four dirs resolve to `/srv/passive-indices/config`, `/srv/passive-indices/data`, `/srv/passive-indices/state`, and `/srv/passive-indices/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "passive-indices": {
      "command": "passive-indices-pp-mcp",
      "env": {
        "PASSIVE_INDICES_HOME": "/srv/passive-indices"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PASSIVE_INDICES_DATA_DIR` overrides an explicit `--home` for that kind. Use `PASSIVE_INDICES_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PASSIVE_INDICES_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `passive-indices-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### index

NSE indices — live levels and constituents (niftyindices.com)

- **`passive-indices-pp-cli index constituents`** - Constituent list (company, industry, symbol, ISIN) for a given index — no per-constituent weight field
- **`passive-indices-pp-cli index live`** - Live snapshot of every published NSE index (level, %chg, OHLC, 52w hi/lo)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`passive-indices-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`passive-indices-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`passive-indices-pp-cli learnings list`** - Inspect taught rows
- **`passive-indices-pp-cli learnings forget <query>`** - Undo a teach
- **`passive-indices-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`passive-indices-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`passive-indices-pp-cli teach-pattern`** - Install a query/resource template up front
- **`passive-indices-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PASSIVE_INDICES_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `passive-indices-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
passive-indices-pp-cli index constituents mock-value

# JSON for scripting and agents
passive-indices-pp-cli index constituents mock-value --json

# Filter to specific fields
passive-indices-pp-cli index constituents mock-value --json --select id,name,status

# Dry run — show the request without sending
passive-indices-pp-cli index constituents mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
passive-indices-pp-cli index constituents mock-value --agent
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
passive-indices-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `passive-indices-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/passive-indices-pp-cli/config.toml`; `--home`, `PASSIVE_INDICES_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `index live` (index names) or `fund search <query>` (scheme IDs) to find valid identifiers

### API-specific
- **indiapassivefunds calls return 401 after a while** — The Bearer token is short-lived (~24h); the CLI re-mints it automatically via the public token endpoint — re-run the command.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**jugaad-data**](https://github.com/jugaad-py/jugaad-data) — Python
- [**nsepython**](https://github.com/aeron7/nsepython) — Python
- [**nsepy**](https://github.com/swapniljariwala/nsepy) — Python
- [**nsetools**](https://github.com/vsjha18/nsetools) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
