# EverBee CLI

**Research Etsy products, shops, and keywords from EverBee in a repeatable agent-ready workflow.**

EverBee is strongest inside an interactive browser. This CLI turns captured product analytics, shop analyzer, and keyword research workflows into repeatable commands with JSON, local snapshots, search, SQL, and cross-workflow opportunity scoring.

Learn more at [EverBee](https://api.everbee.com).

Created by [@horknfbr](https://github.com/horknfbr) (horknfbr).

## Install

The recommended path installs both the `everbee-pp-cli` binary and the `pp-everbee` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install everbee
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install everbee --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install everbee --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install everbee --agent claude-code
npx -y @mvanhorn/printing-press-library install everbee --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/everbee-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-everbee --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-everbee --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install everbee --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/everbee-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EVERBEE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "everbee": {
      "command": "everbee-pp-mcp",
      "env": {
        "EVERBEE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

EverBee uses Google login in the browser. Captured API requests authenticate with an `x-access-token` header; set `EVERBEE_ACCESS_TOKEN` for CLI calls until browser-login replay is proven.

## Quick Start

```bash
# Confirm the access token and API reachability before running research commands.
everbee-pp-cli doctor

# Pull the product analytics table for the current account.
everbee-pp-cli product-analytics --per-page 25 --time-range last_30_days --json

# Inspect keyword suggestions in a narrowed JSON shape.
everbee-pp-cli keyword-research --type-of-search keyword --json --select data

# Fetch shop analyzer results for competitor research.
everbee-pp-cli shops --per-page 25 --json

# Rank product opportunities after data has been synced or fetched.
everbee-pp-cli opportunity shortlist --query "teacher gift" --limit 25 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-workflow opportunity scoring

Insight commands read local research snapshots first. If matching data is missing or stale, they refresh only the EverBee data needed for that query and save the result locally for repeat analysis. Use `--no-refresh` for offline/local-only runs, `--refresh` to force a targeted pull, and `--max-age` to control freshness.

- **`opportunity shortlist`** — Rank Etsy product opportunities by combining product analytics, keyword demand, competition, and local trend history.

  _Use this when an agent needs a short list of products worth researching or creating next._

  ```bash
  everbee-pp-cli opportunity shortlist --query "teacher gift" --limit 25 --agent
  ```
- **`niche score`** — Score a niche by weighing search demand, competition, product saturation, pricing, and trend movement.

  _Use this before committing to a product niche or SEO direction._

  ```bash
  everbee-pp-cli niche score --keyword "mother's day mug" --agent
  ```

### Competitor intelligence
- **`shop gaps`** — Find competitor shop openings from product mix, pricing bands, tags, and keyword coverage.

  _Use this when comparing a target Etsy shop against market demand._

  ```bash
  everbee-pp-cli shop gaps --shop competitor-shop --agent
  ```
- **`competitors watch`** — Detect competitor changes in top products, price bands, and tags across saved shop snapshots.

  _Use this to monitor shops without manually reopening EverBee dashboards._

  ```bash
  everbee-pp-cli competitors watch --shop competitor-shop --agent
  ```

### SEO and tag strategy
- **`tags gap`** — Compare winning listing tags against a target shop or keyword set to reveal missing SEO coverage.

  _Use this when optimizing tags from competitor evidence instead of guessing._

  ```bash
  everbee-pp-cli tags gap --query candle --shop my-shop --agent
  ```
- **`keywords cluster`** — Group related keyword suggestions by term overlap, demand, competition, and opportunity score.

  _Use this to turn raw keyword suggestions into listing-title and tag themes._

  ```bash
  everbee-pp-cli keywords cluster --seed "wedding sign" --agent
  ```
- **`listing audit`** — Audit a listing's keyword and tag fit using EverBee-derived product and keyword context.

  _Use this when checking whether a listing matches the market signals behind a niche._

  ```bash
  everbee-pp-cli listing audit --listing-id 123456789 --agent
  ```

### Local history that compounds
- **`trends diff`** — Compare saved research snapshots to show which products, shops, or keywords moved over time.

  _Use this when deciding whether a niche is growing, fading, or seasonally spiking._

  ```bash
  everbee-pp-cli trends diff --query "teacher gift" --days 30 --agent
  ```

## Recipes

### Narrow product analytics for agents

```bash
everbee-pp-cli product-analytics --per-page 25 --time-range last_30_days --agent --select data
```

Fetch a compact product analytics payload for downstream ranking.

### Cluster keywords from a seed

```bash
everbee-pp-cli keywords cluster --seed "wedding sign" --agent
```

Group keyword suggestions into usable listing and tag themes.

### Find competitor openings

```bash
everbee-pp-cli shop gaps --shop competitor-shop --agent
```

Compare competitor shop data against keyword and product opportunities.

### Track niche movement

```bash
everbee-pp-cli trends diff --query "teacher gift" --days 30 --agent
```

Use saved snapshots to identify rising or fading research targets.

## Usage

Run `everbee-pp-cli --help` for the full command reference and flag list.

## Commands

### folders

Operations on folders

- **`everbee-pp-cli folders`** - GET /folders

### keyword_research

Operations on default_keyword_suggestion

- **`everbee-pp-cli keyword-research`** - GET /keyword_research/default_keyword_suggestion

### management_modals

Operations on management_modals

- **`everbee-pp-cli management-modals`** - GET /management_modals

### product_analytics

Operations on default_product_analytics

- **`everbee-pp-cli product-analytics`** - GET /product_analytics/default_product_analytics

### shops

Operations on shops

- **`everbee-pp-cli shops`** - GET /shops

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
everbee-pp-cli folders

# JSON for scripting and agents
everbee-pp-cli folders --json

# Filter to specific fields
everbee-pp-cli folders --json --select id,name,status

# Dry run — show the request without sending
everbee-pp-cli folders --dry-run

# Agent mode — JSON + compact + no prompts in one flag
everbee-pp-cli folders --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `EVERBEE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `everbee-pp-cli competitors watch`
- `everbee-pp-cli folders`
- `everbee-pp-cli folders get`
- `everbee-pp-cli folders list`
- `everbee-pp-cli folders search`
- `everbee-pp-cli keyword_research`
- `everbee-pp-cli keyword_research get`
- `everbee-pp-cli keyword_research list`
- `everbee-pp-cli keyword_research search`
- `everbee-pp-cli opportunity shortlist`
- `everbee-pp-cli product_analytics`
- `everbee-pp-cli product_analytics get`
- `everbee-pp-cli product_analytics list`
- `everbee-pp-cli product_analytics search`
- `everbee-pp-cli report export`
- `everbee-pp-cli shops`
- `everbee-pp-cli shops get`
- `everbee-pp-cli shops list`
- `everbee-pp-cli shops search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
everbee-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/everbee-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EVERBEE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `everbee-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `everbee-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EVERBEE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or 403 from EverBee endpoints** — Set a fresh `EVERBEE_ACCESS_TOKEN` captured from a logged-in EverBee browser session.
- **Empty product or keyword results** — Run the matching EverBee app workflow in the browser first, then refresh the token and retry with a smaller `--per-page` value.
- **Opportunity commands return no local history** — Run `everbee-pp-cli sync --full` or fetch product, shop, and keyword commands before derived analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://track.getgist.com/projects/7tn4opfe/end_users/ping
- Capture coverage: 31 API entries from 191 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: create_b — Derived from observed POST /b traffic.; create_monitoring — Derived from observed POST /monitoring traffic.; list_default_keyword_suggestion — Derived from observed GET /keyword_research/default_keyword_suggestion traffic.; list_default_product_analytics — Derived from observed GET /product_analytics/default_product_analytics traffic.; list_folders — Derived from observed GET /folders traffic.; list_management_modals — Derived from observed GET /management_modals traffic.; list_ping — Derived from observed GET /projects/7tn4opfe/end_users/ping traffic.; list_shops — Derived from observed GET /shops traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
