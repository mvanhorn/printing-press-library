# Scrape.do CLI

**The first CLI for Scrape-do with Google SERP scraping plus a credit and concurrency governor**

Every Scrape.do surface — the core scraper plus the whole Google family (search, maps, news, shopping, flights, hotels, trends) — wrapped with an offline SQLite history. The governor estimates credit cost before every call with `cost`, debits a local ledger from the authoritative cost header with `budget`, and gates concurrent requests against your plan's live ceiling so an agent swarm never 429s itself or burns the monthly budget. SERPs become queryable history: `drift` and `movers` surface rank changes offline with no re-spend.

## Install

The recommended path installs both the `scrape-do-pp-cli` binary and the `pp-scrape-do` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install scrape-do
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install scrape-do --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install scrape-do --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install scrape-do --agent claude-code
npx -y @mvanhorn/printing-press-library install scrape-do --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/cmd/scrape-do-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-do-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-do --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-do --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-scrape-do skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-scrape-do. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-do-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SCRAPEDO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/cmd/scrape-do-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scrape-do": {
      "command": "scrape-do-pp-mcp",
      "env": {
        "SCRAPEDO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Scrape.do uses a single API token passed as the `token` query parameter. Set it as `SCRAPEDO_API_KEY` in your environment and the CLI never logs the value. The same token drives the core scraper and every Google and Ready-API endpoint.

## Quick Start

```bash
# Confirm SCRAPEDO_API_KEY is set and the API is reachable.
scrape-do-pp-cli doctor

# The primary workflow: a structured Google SERP, stored locally.
scrape-do-pp-cli google search "best crm software" --json

# See remaining credits, concurrency headroom, and burn-rate forecast.
scrape-do-pp-cli budget

# Run the search again later, then diff the rank changes offline.
scrape-do-pp-cli drift "best crm software"

# General page scrape with JS rendering, returned as clean markdown.
scrape-do-pp-cli scrape "https://example.com" --render --markdown

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Offline SERP intelligence
- **`drift`** — Diff a Google query's two most recent stored SERPs and see exactly which results moved, appeared, or dropped — entirely offline, no credits spent.

  _When an agent needs to know whether a ranking changed since last check, this answers it without re-spending a 10-credit SERP call._

  ```bash
  scrape-do-pp-cli drift "best crm software" --json
  ```
- **`movers`** — Scan every tracked query's latest-versus-previous SERP snapshot and surface only the queries whose top positions moved past a threshold.

  _Turns hundreds of tracked keywords into a short 'what changed this week' list an agent can act on._

  ```bash
  scrape-do-pp-cli movers --threshold 3 --agent
  ```

### Credit & concurrency governance
- **`batch`** — Dispatch a list of URLs or queries through a shared concurrency lease and per-call credit ledger, auto-retrying only the non-billed 429/502/510 classes and stopping before a credit ceiling.

  _Lets an agent swarm hammer one account at full speed without 429 storms or blowing the monthly budget._

  ```bash
  scrape-do-pp-cli batch --input urls.txt --max-credits 500 --agent
  ```
- **`budget`** — Attribute spend by mode and query-family from a local ledger debited off the authoritative per-call cost header, joined with cached account state to forecast burn-rate against days remaining.

  _Tells an agent how much budget is left and which workloads are eating it before the account hits a hard 401._

  ```bash
  scrape-do-pp-cli budget --agent
  ```
- **`cost`** — Print the exact credit cost a request will incur — accounting for render, super-proxy, Google endpoints, and per-domain overrides — before spending a single credit.

  _Lets an agent compare the cost of cheap vs expensive scrape modes and pick the cheapest path that works._

  ```bash
  scrape-do-pp-cli cost --url https://www.linkedin.com/company/example --render --super
  ```

## Recipes


### Narrow a deep SERP payload for an agent

```bash
scrape-do-pp-cli google search "coffee makers" --agent --select organic_results.position,organic_results.title,organic_results.link
```

The SERP JSON is large and deeply nested; --select with dotted paths returns just rank, title, and link so an agent doesn't burn context parsing the full payload.

### Estimate cost before an expensive scrape

```bash
scrape-do-pp-cli cost --url https://www.linkedin.com/company/example --render --super
```

Prints the expected credits (LinkedIn domain override + render + super proxy) with no API spend, so you can choose the cheapest mode that works.

### Fan out a URL list under the concurrency cap

```bash
scrape-do-pp-cli batch --input urls.txt --max-credits 500 --agent
```

Dispatches every URL through the shared concurrency lease, retries only the non-billed 429/502/510 classes, and stops before the 500-credit ceiling.

### See what ranked-changes happened this week

```bash
scrape-do-pp-cli movers --threshold 3 --agent
```

Compares each tracked query's two latest stored SERPs and lists only the queries whose top positions moved by 3 or more — entirely offline.

### Query your stored SERP history with SQL

```bash
scrape-do-pp-cli sql "SELECT domain, COUNT(*) c FROM serp_organic GROUP BY domain ORDER BY c DESC LIMIT 10"
```

Read-only SQL over the local store — share-of-voice by domain across every stored SERP, with no credit spent.

## Usage

Run `scrape-do-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Live Scrape.do account state: subscription status, concurrency allowance and headroom, monthly credit cap and remaining credits.

- **`scrape-do-pp-cli account`** - Fetch live account state: IsActive, ConcurrentRequest, RemainingConcurrentRequest, MaxMonthlyRequest, RemainingMonthlyRequest. Free; rate-limited to 10 requests/minute upstream.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
scrape-do-pp-cli account

# JSON for scripting and agents
scrape-do-pp-cli account --json

# Filter to specific fields
scrape-do-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
scrape-do-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
scrape-do-pp-cli account --agent
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
scrape-do-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/scrape-do-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SCRAPEDO_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `scrape-do-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `scrape-do-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SCRAPEDO_API_KEY`
**Empty or unexpected results**
- For Google commands, confirm the query and locale flags (`--gl`, `--hl`, `--google-domain`) match what you expect
- Inspect what's stored locally with `scrape-do-pp-cli sql "SELECT * FROM serp_snapshots ORDER BY fetched_at DESC LIMIT 5"`
- `drift`/`movers` need at least two snapshots of the same query — run `google search` twice over time first

### API-specific
- **429 Too Many Requests during a fan-out** — You hit the plan's concurrent-request cap; the lease backs off automatically — lower throughput with `batch --max-concurrency <N>` or upgrade the plan.
- **401 with no obvious auth error** — Scrape.do returns 401 when monthly credits are exhausted or the subscription is suspended; check `scrape-do-pp-cli budget` and the dashboard.
- **doctor reports auth not configured** — export SCRAPEDO_API_KEY=<your-token> (the value is never logged); re-run doctor.
- **A render request returned raw HTML instead of JSON** — returnJSON requires render — pass both --return-json and --render.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Bright Data CLI**](https://github.com/brightdata/cli) — TypeScript (211 stars)
- [**amazon-scraper**](https://github.com/scrape-do/amazon-scraper) — JavaScript (23 stars)
- [**scrapedo-scrapers**](https://github.com/scrape-do/scrapedo-scrapers) — Python (21 stars)
- [**@scrape-do/client (node-client)**](https://github.com/scrape-do/node-client) — TypeScript (7 stars)
- [**scrapy-scrapedo**](https://github.com/scrape-do/scrapy-scrapedo) — Python
- [**scrapingbee-python**](https://github.com/ScrapingBee/scrapingbee-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
