# Etsy Seller Dashboard CLI

**First-party Etsy demand and marketing evidence, reconciled locally.**

Bring Marketplace Insights, Etsy Ads, Offsite Ads, and Sales & Discounts into one read-only CLI. Preserve short-lived observations in SQLite, compare aligned periods, and turn disconnected seller dashboards into deterministic listing-level decisions without inventing attribution or profit.

Learn more at [Etsy Seller Dashboard](https://www.etsy.com).

Created by [@horknfbr](https://github.com/horknfbr) (horknfbr).

## Data Boundaries

- Marketplace Insights does not expose country-filtered metrics or keyword clicks/CTR in the captured seller surface.
- Etsy Ads search-term attribution is unavailable; the CLI does not infer keyword revenue from listing titles.
- Promotion order count, item count, and average order value remain optional until Etsy returns those fields.
- All Etsy-side mutations are intentionally excluded.

## Install

The recommended path installs both the `etsy-seller-dashboard-pp-cli` binary and the `pp-etsy-seller-dashboard` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --agent claude-code
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ecommerce/etsy-seller-dashboard/cmd/etsy-seller-dashboard-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etsy-seller-dashboard-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-etsy-seller-dashboard --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-etsy-seller-dashboard --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install etsy-seller-dashboard --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
etsy-seller-dashboard-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etsy-seller-dashboard-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ecommerce/etsy-seller-dashboard/cmd/etsy-seller-dashboard-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "etsy-seller-dashboard": {
      "command": "etsy-seller-dashboard-pp-mcp"
    }
  }
}
```

</details>

## Authentication

These are private Shop Manager surfaces protected by the seller's Etsy session and DataDome. Sign in to Etsy in local Chrome, then use the CLI auth flow to import session cookies at runtime. Credentials remain local; doctor detects missing or expired browser-session material. The CLI does not change ad status, Offsite Ads enrollment, or promotions.

## Quick Start

```bash
# Check the binary, local store, and Etsy browser-session prerequisites without changing seller settings or spending keyword quota.
etsy-seller-dashboard-pp-cli doctor --dry-run

# Persist aligned read-only observations from the four peer dashboard surfaces.
etsy-seller-dashboard-pp-cli sync --shop-id 12345678 --resources marketplace-insights,ads,offsite-ads,promotions --since 30d

# Produce a compact, agent-shaped weekly listing review queue with source timestamps.
etsy-seller-dashboard-pp-cli listing action-queue --agent --select listing_id,action,reasons,observed_at

# Review separately attributed spend, fees, and revenue without a false net-profit total.
etsy-seller-dashboard-pp-cli economics reconcile --agent

# Compare a promotion window with an equal-length baseline and explicit non-causal wording.
etsy-seller-dashboard-pp-cli promotion observed-lift PROMOTION_ID --agent

# Surface deterministic historical outliers after recurring sync.
etsy-seller-dashboard-pp-cli growth anomalies --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-surface decisions
- **`listing action-queue`** — Rank listing-level research and marketing review actions from all four synchronized Etsy surfaces.

  _Use this before a weekly catalog review to decide which listings need research, ad review, promotion review, or no intervention._

  ```bash
  etsy-seller-dashboard-pp-cli listing action-queue --agent --select listing_id,action,reasons,observed_at
  ```

### Marketing economics
- **`economics reconcile`** — Reconcile onsite spend, offsite fees, attributed revenue, and promotion observations without inventing net profit.

  _Use this before changing marketing spend or comparing channel performance._

  ```bash
  etsy-seller-dashboard-pp-cli economics reconcile --agent
  ```

### Promotion analysis
- **`promotion observed-lift`** — Compare one promotion window with an equal-length prior window across promotion, onsite-ad, and offsite observations.

  _Use this while reviewing an active or recently ended promotion._

  ```bash
  etsy-seller-dashboard-pp-cli promotion observed-lift PROMOTION_ID --agent
  ```

### Cross-channel acquisition
- **`acquisition channel-gap`** — Find listings whose separately labeled onsite and offsite efficiency signals diverge.

  _Use this during weekly channel-allocation review to surface onsite-strong, offsite-strong, balanced, and insufficient-data listings._

  ```bash
  etsy-seller-dashboard-pp-cli acquisition channel-gap --agent
  ```

### Quota-aware research
- **`quota allocate`** — Prioritize scarce Marketplace Insights searches where stale demand evidence affects the most consequential listing decisions.

  _Use this before spending the weekly Marketplace Insights allowance._

  ```bash
  etsy-seller-dashboard-pp-cli quota allocate --agent
  ```

### Search and acquisition
- **`listing visibility-gap`** — Compare explicit listing-keyword visibility observations with onsite and offsite performance without fabricating keyword revenue.

  _Use this after demand, rank, and marketing snapshots sync to distinguish visibility gaps from acquisition-efficiency gaps._

  ```bash
  etsy-seller-dashboard-pp-cli listing visibility-gap --agent
  ```

### Historical monitoring
- **`growth anomalies`** — Detect deterministic weekly outliers and coincident movements across all four synchronized surfaces.

  _Use this for recurring portfolio review when raw dashboard totals hide exceptions._

  ```bash
  etsy-seller-dashboard-pp-cli growth anomalies --agent
  ```

## Recipes

### Prepare the weekly four-surface review

```bash
etsy-seller-dashboard-pp-cli sync --shop-id 12345678 --resources marketplace-insights,ads,offsite-ads,promotions --since 30d
```

Refresh aligned observations before running any cross-surface local analysis.

### Choose listing review priorities

```bash
etsy-seller-dashboard-pp-cli listing action-queue --agent --select listing_id,action,reasons,observed_at
```

Return deterministic action codes and supporting source timestamps without a live mutation.

### Reconcile marketing economics

```bash
etsy-seller-dashboard-pp-cli economics reconcile --agent
```

Preserve Etsy Ads, Offsite Ads, and promotion attribution boundaries while exposing exact subtotals and exclusions.

### Find portfolio exceptions

```bash
etsy-seller-dashboard-pp-cli growth anomalies --agent
```

Detect source-linked outliers from local history after the minimum observation window is met.

## Usage

Run `etsy-seller-dashboard-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ETSY_SELLER_DASHBOARD_CONFIG_DIR`, `ETSY_SELLER_DASHBOARD_DATA_DIR`, `ETSY_SELLER_DASHBOARD_STATE_DIR`, or `ETSY_SELLER_DASHBOARD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ETSY_SELLER_DASHBOARD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ETSY_SELLER_DASHBOARD_HOME=/srv/etsy-seller-dashboard
etsy-seller-dashboard-pp-cli doctor
```

Under `ETSY_SELLER_DASHBOARD_HOME=/srv/etsy-seller-dashboard`, the four dirs resolve to `/srv/etsy-seller-dashboard/config`, `/srv/etsy-seller-dashboard/data`, `/srv/etsy-seller-dashboard/state`, and `/srv/etsy-seller-dashboard/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "etsy-seller-dashboard": {
      "command": "etsy-seller-dashboard-pp-mcp",
      "env": {
        "ETSY_SELLER_DASHBOARD_HOME": "/srv/etsy-seller-dashboard"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ETSY_SELLER_DASHBOARD_DATA_DIR` overrides an explicit `--home` for that kind. Use `ETSY_SELLER_DASHBOARD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ETSY_SELLER_DASHBOARD_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `etsy-seller-dashboard-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### ads

Review read-only Etsy Ads listing and account performance

- **`etsy-seller-dashboard-pp-cli ads <shop_id>`** - List Etsy Ads performance by listing

### marketplace-insights

Research first-party Etsy search demand, competition, trends, and saved terms

- **`etsy-seller-dashboard-pp-cli marketplace-insights data <shop_id>`** - Fetch the current Marketplace Insights data payload
- **`etsy-seller-dashboard-pp-cli marketplace-insights overview`** - Fetch the authenticated Marketplace Insights dashboard
- **`etsy-seller-dashboard-pp-cli marketplace-insights saved-searches <shop_id>`** - List saved Marketplace Insights search terms
- **`etsy-seller-dashboard-pp-cli marketplace-insights search <query>`** - Run an authenticated Marketplace Insights search
- **`etsy-seller-dashboard-pp-cli marketplace-insights trending-categories <shop_id>`** - List trending Etsy marketplace categories
- **`etsy-seller-dashboard-pp-cli marketplace-insights trending-terms <shop_id>`** - List trending Etsy search terms

### offsite-ads

Review read-only Offsite Ads attribution, traffic, channels, listings, and fees

- **`etsy-seller-dashboard-pp-cli offsite-ads channels <shop_id>`** - List Offsite Ads click performance by external channel
- **`etsy-seller-dashboard-pp-cli offsite-ads listings <shop_id>`** - List Offsite Ads clicks, orders, and attributed revenue by listing
- **`etsy-seller-dashboard-pp-cli offsite-ads summary <shop_id>`** - Fetch Offsite Ads attributed revenue, fees, orders, and new buyers
- **`etsy-seller-dashboard-pp-cli offsite-ads traffic <shop_id>`** - Fetch Offsite Ads click series and an optional comparison period

### promotions

Review read-only Etsy sales, promo codes, bundles, and targeted offers

- **`etsy-seller-dashboard-pp-cli promotions <shop_id>`** - List promotions and available revenue statistics


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`etsy-seller-dashboard-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`etsy-seller-dashboard-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`etsy-seller-dashboard-pp-cli learnings list`** - Inspect taught rows
- **`etsy-seller-dashboard-pp-cli learnings forget <query>`** - Undo a teach
- **`etsy-seller-dashboard-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`etsy-seller-dashboard-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`etsy-seller-dashboard-pp-cli teach-pattern`** - Install a query/resource template up front
- **`etsy-seller-dashboard-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ETSY_SELLER_DASHBOARD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `etsy-seller-dashboard-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
etsy-seller-dashboard-pp-cli marketplace-insights search mock-value

# JSON for scripting and agents
etsy-seller-dashboard-pp-cli marketplace-insights search mock-value --json

# Filter to specific fields
etsy-seller-dashboard-pp-cli marketplace-insights search mock-value --json --select id,name,status

# Dry run — show the request without sending
etsy-seller-dashboard-pp-cli marketplace-insights search mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
etsy-seller-dashboard-pp-cli marketplace-insights search mock-value --agent
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

## Health Check

```bash
etsy-seller-dashboard-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `etsy-seller-dashboard-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/etsy-seller-dashboard-pp-cli/config.toml`; `--home`, `ETSY_SELLER_DASHBOARD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ETSY_COOKIE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `etsy-seller-dashboard-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `etsy-seller-dashboard-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ETSY_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports missing or expired Etsy session material** — Sign in to Etsy in local Chrome, rerun the CLI auth import, and retry doctor. Do not paste cookies into shell history.
- **a local analysis reports stale or unsynchronized resources** — Run sync for the named resources and matching date range, then repeat the analysis.
- **Ads search terms or promotion order/item/AOV fields are unavailable** — Treat them as unsupported optional data. Do not infer missing attribution or substitute title-token matching.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `ETSY_SELLER_DASHBOARD_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.
