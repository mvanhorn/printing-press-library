# Polymarket + Kalshi CLI

Every Polymarket and Kalshi market in one slim agent-native CLI, with cross-venue topic search and screens no other tool has.

Read-only by design and by CI lint - the binary structurally cannot trade. `topic <name>` returns every related Polymarket + Kalshi market/event/tag in one ~3KB ranked bundle (vs the official Polymarket CLI's ~250KB firehose). Local SQLite + FTS5 keeps queries instant and free after one sync. Six screens (`trending`, `resolving`, `liquid`, `mispriced`, `movers`, `new`) cover the workflows agents and odds researchers run every week.

Printed by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `prediction-goat-pp-cli` binary and the `pp-prediction-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install prediction-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install prediction-goat --cli-only
```

For skill only - installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install prediction-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable - agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install prediction-goat --agent claude-code
npx -y @mvanhorn/printing-press install prediction-goat --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/prediction-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-prediction-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-prediction-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-prediction-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-prediction-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle - Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/prediction-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "prediction-goat": {
      "command": "prediction-goat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. Both Polymarket Gamma and Kalshi v2 trade-api public endpoints work without a key for everything this CLI does (markets, events, tags, series, orderbooks, prices). The binary contains no signing code, no wallet handling, and no order-placement endpoints - CI lint enforces this on every PR.

## Quick Start

```bash
# One-time pull of Polymarket and Kalshi market/event/tag data into local SQLite (~30MB, no API key).
prediction-goat-pp-cli sync


# The killer feature: every related market across both venues, ranked, slim by default.
prediction-goat-pp-cli topic kanye --json


# Top 24h volume across both Polymarket and Kalshi in one call.
prediction-goat-pp-cli trending --json --limit 20


# Side-by-side implied probability on the same topic across venues.
prediction-goat-pp-cli compare 'arizona basketball' --json


# Markets settling within the next 7 days, sorted by liquidity.
prediction-goat-pp-cli resolving --week --json


# Same-outcome markets where Polymarket and Kalshi disagree by 5pp or more.
prediction-goat-pp-cli mispriced --threshold 0.05 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-venue intelligence
- `topic` - Get every related Polymarket and Kalshi market for a topic in one slim ranked ~3KB bundle - kanye, argentina, chatgpt-5.

  _When an agent needs current odds on a topic, this is the one-call answer across both venues without fanning out to two platform tools and re-ranking by hand._

  ```bash
  prediction-goat-pp-cli topic kanye --json
  ```
- `mispriced` - Find same-outcome markets where Polymarket and Kalshi disagree on implied probability by more than a threshold.

  _The clearest signal that one venue is wrong or one side is mispricing - useful for calibration research, not trading._

  ```bash
  prediction-goat-pp-cli mispriced --threshold 0.05 --json
  ```
- `compare` - Side-by-side YES/NO and implied probability for the same topic across Polymarket and Kalshi.

  _Tells an agent or analyst 'which venue has the better/different number on this outcome' in one read-only call._

  ```bash
  prediction-goat-pp-cli compare 'arizona basketball' --json
  ```
- `markets diff` - Field-by-field structural diff between a specific Polymarket market and a specific Kalshi market.

  _When you already know the two slugs/tickers (e.g. from `topic <theme>`), diff shows you exactly where the venues disagree._

  ```bash
  prediction-goat-pp-cli markets diff <pm-slug> <kalshi-ticker> --json
  ```

### Screens
- `trending` - Top movers by 24h volume across both venues, ranked.

  _One command answers 'what should I be watching today' without scraping two homepages._

  ```bash
  prediction-goat-pp-cli trending --json --limit 20
  ```
- `resolving` - Markets resolving in the next week/month/days, sorted by liquidity.

  _Tells an agent 'what's about to settle' without re-paging two cursors._

  ```bash
  prediction-goat-pp-cli resolving --week --json
  ```
- `liquid` - Markets above a normalized volume/liquidity floor across both venues.

  _Filters out thin markets that will move on a single 100-dollar bet._

  ```bash
  prediction-goat-pp-cli liquid --min-volume 100000 --json
  ```
- `movers` - Biggest implied-probability deltas over a 24h or 7d window across both venues.

  _Surfaces narrative shifts (price-driven) vs hype shifts (volume-driven from )._

  ```bash
  prediction-goat-pp-cli movers --window 7d --json
  ```
- `new` - Markets created in the last N days across both venues.

  _Newly listed markets are where the alpha and mispricings live._

  ```bash
  prediction-goat-pp-cli new --days 7 --json
  ```

### Live-on-read freshness

Every command that surfaces a price (`yesProbability`, `yes_ask_dollars`, `volume_24h_fp`) refetches from the upstream API at command time. The local SQLite store is treated as a discovery index, never as a price source — so a question like "what are the odds Lakers win tonight" never gets served yesterday's cached spread. The response envelope carries `meta.price_source` (`live` / `stale` / `mixed`) and `meta.index_synced_at` so an agent can detect stale-index conditions. Human-mode output prints a `Index synced 14h ago, prices live` footer.

### Learning surface: `teach` and `recall`

The CLI gets smarter every time an LLM uses it, without any user-typed `learn` commands. The LLM contract:

- **Before** running `topic` / `compare` for a new question:

  ```bash
  prediction-goat-pp-cli recall "what are Portugal's odds at the world cup" --agent
  ```

  Returns confirmed mappings from prior sessions matched by token-set Jaccard (>= 0.6). If `found=true` with `confidence>=2`, skip discovery and go straight to live price fetch for the returned tickers - 2 CLI calls instead of 7.

- **After** finalizing a response that includes tickers/slugs, but **before** emitting it to the human:

  ```bash
  prediction-goat-pp-cli teach \
    --query "what are Portugal's odds at the world cup" \
    --resource KXMENWORLDCUP-26-PT \
    --resource will-portugal-win-the-2026-fifa-world-cup-912 &
  ```

  Silent on success (zero stdout/stderr), safe to background with `&`. Errors go to `~/.local/share/prediction-goat-pp-cli/teach.log`. Idempotent on `(query_pattern, resource_id, action)` - calling it twice bumps confidence instead of duplicating.

Inspection and undo:

```bash
prediction-goat-pp-cli learnings list --agent          # all rows
prediction-goat-pp-cli learnings list --query "btc"    # substring filter
prediction-goat-pp-cli forget "portugal world cup" --all
```

Disable for a single call with `--no-learn`, or globally with `PREDICTION_GOAT_NO_LEARN=true`. The reranking layer in `topic` and `compare` applies the learnings automatically (`boost` moves a hit to position 0 or inserts it synthetically; `hide` drops; `alias_of` rewrites).

## Usage

Run `prediction-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### comments

Comment system and user interactions

- `prediction-goat-pp-cli comments get-by-id` - Get comments by comment id
- `prediction-goat-pp-cli comments get-by-user-address` - Get comments by user address
- `prediction-goat-pp-cli comments list` - List comments

### events

Event management and event-related operations

- `prediction-goat-pp-cli events get` - Get event by id
- `prediction-goat-pp-cli events get-by-slug` - Get event by slug
- `prediction-goat-pp-cli events get-creator` - Get event creator by id
- `prediction-goat-pp-cli events list` - List events
- `prediction-goat-pp-cli events list-creators` - List event creators
- `prediction-goat-pp-cli events list-keyset` - Returns events using cursor-based (keyset) pagination for stable, efficient paging through large result sets. Use `next_cursor` from each response as `after_cursor` in the next request. The `offset` parameter is explicitly rejected; use `after_cursor` instead.
- `prediction-goat-pp-cli events list-pagination` - List events (paginated)
- `prediction-goat-pp-cli events list-sport-results` - List sport events results

### markets

Market data and market-related operations

- `prediction-goat-pp-cli markets get` - Get market by id
- `prediction-goat-pp-cli markets get-abridged` - Query abridged markets by information filters
- `prediction-goat-pp-cli markets get-by-slug` - Get market by slug
- `prediction-goat-pp-cli markets get-information` - Query markets by information filters
- `prediction-goat-pp-cli markets list` - List markets
- `prediction-goat-pp-cli markets list-keyset` - Returns markets using cursor-based (keyset) pagination for stable, efficient paging through large result sets. Use `next_cursor` from each response as `after_cursor` in the next request. The `offset` parameter is explicitly rejected; use `after_cursor` instead.

### profiles

User profile management

- `prediction-goat-pp-cli profiles <user_address>` - Get public profile by user address

### public-profile

Manage public profile

- `prediction-goat-pp-cli public-profile` - Get public profile by wallet address

### public-search

Manage public search

- `prediction-goat-pp-cli public-search` - Search markets, events, and profiles

### series

Series management and related operations

- `prediction-goat-pp-cli series get` - Get series by id
- `prediction-goat-pp-cli series list` - List series

### series-summary

Manage series summary

- `prediction-goat-pp-cli series-summary get-by-id` - Get series summary by id
- `prediction-goat-pp-cli series-summary get-by-slug` - Get series summary by slug

### sports

Sports-related endpoints including teams and game data

- `prediction-goat-pp-cli sports get-market-types` - Get valid sports market types
- `prediction-goat-pp-cli sports get-metadata` - Get sports metadata information

### status

Manage status

- `prediction-goat-pp-cli status` - Gamma API Health check

### tags

Tag management and related tag operations

- `prediction-goat-pp-cli tags get` - Get tag by id
- `prediction-goat-pp-cli tags get-by-slug` - Get tag by slug
- `prediction-goat-pp-cli tags get-related-by-slug` - Get related tags (relationships) by tag slug
- `prediction-goat-pp-cli tags get-related-to-atag-by-slug` - Get tags related to a tag slug
- `prediction-goat-pp-cli tags list` - List tags

### teams

Manage teams

- `prediction-goat-pp-cli teams get` - Get team by id
- `prediction-goat-pp-cli teams list` - List teams


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
prediction-goat-pp-cli comments list

# JSON for scripting and agents
prediction-goat-pp-cli comments list --json

# Filter to specific fields
prediction-goat-pp-cli comments list --json --select id,name,status

# Dry run - show the request without sending
prediction-goat-pp-cli comments list --dry-run

# Agent mode - JSON + compact + no prompts in one flag
prediction-goat-pp-cli comments list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- Non-interactive - never prompts, every input is a flag
- Pipeable - `--json` output to stdout, errors to stderr
- Filterable - `--select id,name` returns only fields you need
- Previewable - `--dry-run` shows the request without sending
- Explicit retries - add `--idempotent` to create retries when a no-op success is acceptable
- Confirmable - `--yes` for explicit confirmation of destructive actions
- Piped input - write commands can accept structured input when their help lists `--stdin`
- Offline-friendly - sync/search commands can use the local SQLite store when available
- Agent-safe by default - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
prediction-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/markets-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
Not found errors (exit code 3)
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- `topic` returns no results after install - Run `prediction-goat-pp-cli sync` first. Topic search reads the local SQLite store - without a sync, FTS5 has nothing to rank.
- `mispriced` or `compare` returns fewer pairs than expected - The PM-to-Kalshi outcome pairing relies on topic-text overlap; some markets simply don't have a counterpart on the other venue. Try `topic <name> --json` first to see what's available.
- Polymarket prices look stale after sync - Re-run `sync --incremental` - Polymarket markets refresh in real time but the local mirror is a snapshot; the sync command refreshes only changed rows.
- Kalshi endpoint returns empty events list - Set `--venue polymarket` on the failing command to confirm it's a Kalshi-side reachability issue, then check `prediction-goat-pp-cli doctor --json`.

---

## Known Gaps

These are upstream API behaviors, not bugs in this CLI. Live smoke testing flagged them; they do not affect the headline workflows (`topic`, `trending`, `resolving`, `liquid`, `new`, `mispriced`, `movers`, `compare`, `markets diff`).

- `comments list` returns HTTP 422 without filter args. Polymarket Gamma's `/comments` endpoint requires `parent_entity_id` and `entity_entity_type` query params. Without them the API responds with a validation error. Pass the required filters or use `comments by-user <address>` instead.
- `events get-creator <id>` may return HTTP 404 on older event IDs. Polymarket prunes creator records for archived events. Pull a current event ID from `events list --closed=false` first.
- `public-profile --address <addr>` returns HTTP 404 on synthetic addresses. Polymarket only resolves real wallet addresses that have traded on the platform. Supply a real address (e.g., from `trending --json --select items.id` then `markets get <id>` then look up traders).
- `tags get-related-by-slug <slug>` returns HTTP 404 on unknown slugs. Lookup is exact-match. Use `tags list` to find a real slug first.
- `workflow archive --json` emits JSON Lines (one event per line), not a single JSON document. This is a framework command that streams sync events as they happen. Pipe through `jq -s '.'` to coalesce or use `--no-color` and parse line-by-line.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [polymarket-cli](https://github.com/Polymarket/polymarket-cli) - Python
- [prediction-mcp](https://github.com/shaanmajid/prediction-mcp) - TypeScript
- [prediction-market-mcp](https://github.com/JamesANZ/prediction-market-mcp) - TypeScript
- [polymarket-mcp (Gamma)](https://github.com/pab1it0/polymarket-mcp) - Python
- [polymarket-mcp (CLOB)](https://github.com/CarlosIbCu/polymarket-mcp) - Python
- [kalshi-pp-cli](https://github.com/mvanhorn/printing-press-library/tree/main/library/payments/kalshi) - Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
