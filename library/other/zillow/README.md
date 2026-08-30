# Zillow CLI

**Official Zillow Research data, normalized for explainable housing-market decisions.**

Sync public Zillow Research time series, compare regions, and build sourced affordability, supply, demand, negotiation, and relocation analysis. Core commands never scrape Zillow.com; optional Bridge requests remain separately authorized and uncached.

## Install

The recommended path installs both the `zillow-pp-cli` binary and the `pp-zillow` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zillow
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zillow --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zillow --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zillow --agent claude-code
npx -y @mvanhorn/printing-press-library install zillow --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zillow-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zillow --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zillow --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zillow --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zillow --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zillow-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zillow": {
      "command": "zillow-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public Zillow Research commands require no authentication. Bridge access is optional and requires an approved BRIDGE_ACCESS_TOKEN; the CLI sends it only as bearer authorization and never caches Bridge responses.

## Quick Start

```bash
# Confirm paths and local runtime health without credentials.
zillow-pp-cli doctor

# Resolve a human place name against official dataset coverage.
zillow-pp-cli region resolve "Austin, TX" --agent

# Fetch a sourced regional snapshot.
zillow-pp-cli summary "Austin, TX" --agent

# Turn several official series into an explainable buyer-leverage view.
zillow-pp-cli negotiation "Austin, TX" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Client decisions
- **`affordability gap`** — Compare household income with Zillow's regional homeowner-income-needed estimate.

  _Use when an agent must turn a generic affordability estimate into a client-specific answer._

  ```bash
  zillow-pp-cli affordability gap "Austin, TX" --income 120000 --agent
  ```
- **`yield-proxy`** — Join ZORI and ZHVI on a common date for a transparent gross rent-to-value proxy.

  _Use for quick, explainable rent-versus-value comparisons without pretending the proxy is net yield._

  ```bash
  zillow-pp-cli yield-proxy "Austin, TX" --agent
  ```
- **`buy-vs-rent`** — Run a visible-assumption ownership-versus-rent cash-flow scenario.

  _Use when an agent needs an inspectable scenario rather than a black-box recommendation._

  ```bash
  zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent
  ```
- **`negotiation`** — Combine price cuts, sale-to-list ratio, days pending, and inventory into an explainable score.

  _Use for buyer preparation when every score component must be shown._

  ```bash
  zillow-pp-cli negotiation "Austin, TX" --agent
  ```

### Market structure
- **`supply-ratio`** — Join inventory and sales nowcast into an approximate months-of-inventory ratio.

  _Use when an agent needs a compact supply-balance signal with its approximation stated._

  ```bash
  zillow-pp-cli supply-ratio "Austin, TX" --agent
  ```
- **`turning-points`** — Find dated slope sign reversals across temperature, inventory, days pending, and ZHVI.

  _Use when an agent must identify when a market changed direction, not merely its current level._

  ```bash
  zillow-pp-cli turning-points "Austin, TX" --months 24 --agent
  ```
- **`tier-spread`** — Compare bottom-, middle-, and top-tier ZHVI growth.

  _Use when affordability tiers may be moving differently from the headline market._

  ```bash
  zillow-pp-cli tier-spread "Austin, TX" --agent
  ```
- **`demand-pressure`** — Combine rental demand and for-sale market momentum with component-level evidence.

  _Use when an agent must compare rental and ownership demand in one sourced view._

  ```bash
  zillow-pp-cli demand-pressure "Austin, TX" --agent
  ```
- **`new-build-gap`** — Compare new-construction pricing and sales activity with typical regional home value.

  _Use when an agent needs to quantify the new-build premium and activity context._

  ```bash
  zillow-pp-cli new-build-gap "Austin, TX" --agent
  ```

### Regional screening
- **`shortlist`** — Rank regions with explicit user weights and visible min-max-normalized components.

  _Use for relocation or market-screening work where ranking criteria must remain auditable._

  ```bash
  zillow-pp-cli shortlist --regions "394355,394530" --weight zhvi=-0.4 --weight zori=0.6 --agent
  ```
- **`breadth`** — Measure rising, falling, and unchanged regions by state or geography type.

  _Use when an agent needs to tell whether a trend is broad or isolated._

  ```bash
  zillow-pp-cli breadth --months 12 --group-by state --agent -- zhvi
  ```

### Evidence quality
- **`quality audit`** — Detect missing observations, duplicate regions, coverage gaps, and large monthly jumps.

  _Use before analysis when freshness, gaps, or anomalous changes could invalidate a conclusion._

  ```bash
  zillow-pp-cli quality audit --jump-threshold 20 --agent -- zhvi
  ```
- **`explain`** — Show formulas, datasets, freshness behavior, and caveats for compound commands.

  _Use before relying on a compound score or scenario in consequential work._

  ```bash
  zillow-pp-cli explain --agent -- negotiation
  ```

### Agent delivery
- **`client-brief`** — Compose deterministic Markdown or JSON briefs from sourced regional metrics.

  _Use when an agent must hand a human a concise brief without inventing narrative facts._

  ```bash
  zillow-pp-cli client-brief "Austin, TX" --income 120000 --format markdown
  ```

## Recipes

### Buyer negotiation brief

```bash
zillow-pp-cli negotiation "Austin, TX" --agent
```

Shows leverage score, every component, source dates, and caveats.

### Buy-versus-rent scenario

```bash
zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent
```

Compares cash flow using visible financing, growth, tax, insurance, maintenance, and transaction assumptions.

### Regional shortlist

```bash
zillow-pp-cli shortlist --regions "394355,394530" --weight zhvi=-0.4 --weight zori=0.6 --agent
```

Ranks candidate markets while exposing normalized components and user-supplied weights.

### Compact sourced snapshot

```bash
zillow-pp-cli summary "Austin, TX" --agent --select results.region,results.metrics,results.evidence
```

Keeps only the region, core metrics, and evidence fields for low-context agent use.

### Offline SQL

```bash
zillow-pp-cli sql "SELECT metric, COUNT(*) AS rows FROM observations GROUP BY metric" --agent
```

Queries normalized observations locally after sync with a read-only SQL gate.

## Usage

Run `zillow-pp-cli --help` for the full command reference and flag list.

## Market intelligence

The core CLI reads 17 public CSV datasets from [Zillow Research](https://www.zillow.com/research/data/). It does not scrape Zillow.com, automate consumer pages, or call private Zillow endpoints.

```bash
# Latest sourced market snapshot
zillow-pp-cli summary "Austin, TX" --agent

# Explainable buyer leverage from price cuts, sale-to-list ratio,
# days pending, and inventory
zillow-pp-cli negotiation "Austin, TX" --agent

# Compare ownership and rent cash flow with explicit assumptions
zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent

# Generate a deterministic brief suitable for client review
zillow-pp-cli client-brief "Austin, TX" --income 120000 --format markdown

# See every formula, source, and caveat before using a compound metric
zillow-pp-cli explain --agent -- negotiation
```

Additional analysis commands cover affordability gaps, rent/value yield, supply absorption, turning points, market breadth, regional shortlists, price-tier divergence, demand pressure, new-construction gaps, and dataset quality.

## Offline analysis

Sync normalized observations to SQLite, then query or export without another download:

```bash
zillow-pp-cli sync market --agent
zillow-pp-cli sql "SELECT metric, COUNT(*) AS rows FROM observations GROUP BY metric" --agent
zillow-pp-cli rank --data-source local --limit 20 --agent -- zhvi
zillow-pp-cli export zhvi --data-source local --csv
```

Saved watches persist locally and report changes between runs:

```bash
zillow-pp-cli watch add austin --region "Austin, TX" --metrics zhvi,zori,inventory
zillow-pp-cli watch run austin --agent
```

## Optional Bridge access

Approved Zillow/MLS partners can make read-only Bridge Web API requests. Set `BRIDGE_ACCESS_TOKEN`, then use `bridge status` and `bridge request`. Tokens are sent only as bearer authorization; Bridge responses are never cached. Public Zillow Research commands need no authentication.

```bash
zillow-pp-cli bridge status --agent
zillow-pp-cli bridge request --dataset <approved-dataset-id> --resource Property --top 10 --agent
```

Created by [@veltri-23](https://github.com/veltri-23) (Hunter Veltri).

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZILLOW_CONFIG_DIR`, `ZILLOW_DATA_DIR`, `ZILLOW_STATE_DIR`, or `ZILLOW_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZILLOW_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZILLOW_HOME=/srv/zillow
zillow-pp-cli doctor
```

Under `ZILLOW_HOME=/srv/zillow`, the four dirs resolve to `/srv/zillow/config`, `/srv/zillow/data`, `/srv/zillow/state`, and `/srv/zillow/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zillow": {
      "command": "zillow-pp-mcp",
      "env": {
        "ZILLOW_HOME": "/srv/zillow"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZILLOW_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZILLOW_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZILLOW_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `zillow-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### research

Official Zillow Research regional time-series downloads.

- **`zillow-pp-cli research days-pending`** - Download monthly metro mean days-to-pending observations.
- **`zillow-pp-cli research homeowner-income`** - Download monthly metro income-needed estimates for a typical home purchase with 20 percent down.
- **`zillow-pp-cli research inventory`** - Download monthly metro for-sale inventory observations.
- **`zillow-pp-cli research market-temperature`** - Download monthly metro market-temperature index observations.
- **`zillow-pp-cli research new-con-price`** - Download monthly metro new-construction median sale prices.
- **`zillow-pp-cli research new-con-price-per-sqft`** - Download monthly metro new-construction median sale price per square foot.
- **`zillow-pp-cli research new-con-sales`** - Download monthly metro new-construction sales counts.
- **`zillow-pp-cli research price-cut-share`** - Download monthly metro share of listings with a price cut.
- **`zillow-pp-cli research sale-to-list`** - Download monthly metro mean sale-to-list ratio observations.
- **`zillow-pp-cli research sales`** - Download monthly metro sales-count nowcast observations.
- **`zillow-pp-cli research total-monthly-payment`** - Download monthly metro total housing payment estimates for a typical home purchase with 20 percent down.
- **`zillow-pp-cli research zhvf`** - Download monthly metro Zillow Home Value Forecast growth observations.
- **`zillow-pp-cli research zhvi`** - Download monthly metro Zillow Home Value Index observations.
- **`zillow-pp-cli research zhvi-bottom-tier`** - Download monthly metro bottom-tier Zillow Home Value Index observations.
- **`zillow-pp-cli research zhvi-top-tier`** - Download monthly metro top-tier Zillow Home Value Index observations.
- **`zillow-pp-cli research zordi`** - Download monthly metro Zillow Observed Renter Demand Index observations.
- **`zillow-pp-cli research zori`** - Download monthly metro Zillow Observed Rent Index observations.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zillow-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zillow-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zillow-pp-cli learnings list`** - Inspect taught rows
- **`zillow-pp-cli learnings forget <query>`** - Undo a teach
- **`zillow-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zillow-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zillow-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zillow-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZILLOW_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zillow-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zillow-pp-cli research days-pending

# JSON for scripting and agents
zillow-pp-cli research days-pending --json

# Filter to specific fields
zillow-pp-cli research days-pending --json --select id,name,status

# Dry run — show the request without sending
zillow-pp-cli research days-pending --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zillow-pp-cli research days-pending --agent
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
zillow-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `zillow-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zillow-pp-cli/config.toml`; `--home`, `ZILLOW_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **A region name is ambiguous or not found.** — Run zillow-pp-cli region resolve "<city, state>" --metric zhvi --agent and reuse the returned region ID.
- **Local mode says a dataset is unavailable.** — Run zillow-pp-cli sync market --agent, then retry with --data-source local.
- **Zillow Research returns HTTP 429.** — Honor the reported Retry-After value, reduce request frequency with --rate-limit, and retry later.
- **Bridge reports no token.** — Obtain dataset approval first, then set BRIDGE_ACCESS_TOKEN in the invoking process environment.
