# Context Dev CLI

Agent-friendly Context.dev CLI for website intelligence, brand enrichment, scraping, crawling, structured extraction, screenshots, styleguides, competitor maps, source packs, and change digests.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `context-dev-pp-cli` binary and the `pp-context-dev` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install context-dev
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install context-dev --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install context-dev --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install context-dev --agent claude-code
npx -y @mvanhorn/printing-press-library install context-dev --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/context-dev/cmd/context-dev-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/context-dev-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->

## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install context-dev --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-context-dev --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-context-dev --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install context-dev --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/context-dev-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CONTEXT_DEV_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/context-dev/cmd/context-dev-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "context-dev": {
      "command": "context-dev-pp-mcp",
      "env": {
        "CONTEXT_DEV_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
context-dev-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable. `CONTEXT_DEV_API_KEY` wins when both variables are set; `CONTEXT_API_KEY` is accepted as a fallback for shared Context.dev tooling.

```bash
export CONTEXT_DEV_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
context-dev-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
context-dev-pp-cli brand list --domain example-value
```

## First-Class Workflows

These commands sit on top of the full generated API surface and use the same auth, `--json`, `--agent`, `--dry-run`, timeout, and delivery flags.

- `context-dev-pp-cli scrape https://example.com/page --json` returns clean Markdown for one URL.
- `context-dev-pp-cli crawl https://example.com --max-pages 5 --json` crawls same-domain pages. Use `--estimate` before spending credits; `--confirm` or `--yes` is required above 25 pages.
- `context-dev-pp-cli extract https://example.com --schema schema.json --json` extracts typed JSON using a JSON Schema file.
- `context-dev-pp-cli styleguide example.com --json` extracts design-system information.
- `context-dev-pp-cli screenshot example.com --json` captures a screenshot preview.
- `context-dev-pp-cli entity-discover --type company --name "Acme" --location "Austin, TX" --max-candidates 5 --json` composes web search, brand retrieval, scrape enrichment, and heuristic ranking. Output keys are `entity_type`, `name`, `description`, `location`, `address`, `website`, `socials`, `logo`, `source_url`, `score`, and `provenance`.
- `context-dev-pp-cli brand-brief example.com --json` normalizes brand, styleguide, screenshot, scrape summary, contact surfaces, and provenance into `domain`, `website`, `title`, `description`, `logo`, `colors`, `fonts`, `socials`, `contact_surfaces`, `screenshot`, `summary`, and `provenance`.
- `context-dev-pp-cli competitor-map --domain example.com --market "US SMB" --max 5 --json` maps adjacent entities, enriches candidates, clusters by category/market, and returns `why_ranked`, `category`, `overlap_signals`, and `provenance` per competitor. Use `--query` instead of `--domain` when no seed website is available.
- `context-dev-pp-cli crawl-budget-plan https://example.com --max-pages 25 --json` plans `urlRegex`, same-domain scope, likely coverage, risk warnings, estimated credits, and the recommended crawl command without calling credit-spending crawl endpoints.
- `context-dev-pp-cli source-pack --query "Context.dev brand API" --max-sources 5 --schema schema.json --json` searches, scrapes top sources, optionally extracts fields, and emits cited JSON/Markdown with source URLs. Search API failures return non-zero; zero search results return `status: "no_results"`.
- `context-dev-pp-cli website-change-digest example.com --json` snapshots scrape/styleguide/screenshot under the resolved state dir and diffs against the prior local snapshot. Output includes changed copy, changed links/facts, visual identity changes, screenshot references, timestamps, and provenance.
- `context-dev-pp-cli schema-lab --url https://example.com/a --url https://example.com/b --schema schema.json --json` runs extraction across sample pages and reports field fill rates, parse failures, example misses, and raw per-URL status. Partial URL failures are reported in results without aborting the batch.
- `context-dev-pp-cli brand-kit example.com --json` generates an on-demand brand kit (alias: `asset-pack`): logo, palette, fonts/styleguide, screenshot, favicon, socials, and provenance.
- `context-dev-pp-cli brand-qa example.com --question "What is the return policy?" --json` answers a natural-language question grounded on the brand's website, returning the answer, the URLs analyzed, and provenance.
- `context-dev-pp-cli email-enrich founders@example.com --json` turns a work email into a company profile and signup-form prefill fields (company name, website, industry) with provenance.
- `context-dev-pp-cli ticker-enrich AAPL --json` resolves a public company from a stock ticker or ISIN (auto-detected) to a brand profile plus NAICS/SIC industry codes. Pass `--exchange` to disambiguate a ticker.
- `context-dev-pp-cli trust-check example.com --json` compares website, socials, address, phone, logo, title/domain consistency, and basic web signals. It reports consistency/risk signals only; it does not claim fraud.
- `context-dev-pp-cli lead-enrich-batch leads.csv --domain-column domain --name-column name --location-column location --output enriched.json --resume --rate-limit 1 --json` enriches CSV rows with per-row success/error, provenance, and failure reason. One bad row does not fail the batch unless `--strict` is set.


All multi-credit workflows support `--estimate`; every workflow supports `--dry-run`. These modes emit a JSON plan with `estimated_credits` and `planned_requests` and do not call Context.dev endpoints. `website-change-digest` stores snapshots only under the CLI state directory resolved from `--home`, `CONTEXT_DEV_HOME`, `CONTEXT_DEV_STATE_DIR`, XDG state, or the platform default; it never writes snapshots into the repo.

## Usage

Run `context-dev-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind     | Contents                                                                                                         |
| -------- | ---------------------------------------------------------------------------------------------------------------- |
| `config` | User-editable settings such as `config.toml` and saved profiles                                                  |
| `data`   | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state`  | Runtime state such as persisted queries, jobs, and `teach.log`                                                   |
| `cache`  | Regenerable HTTP/cache files                                                                                     |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CONTEXT_DEV_CONFIG_DIR`, `CONTEXT_DEV_DATA_DIR`, `CONTEXT_DEV_STATE_DIR`, or `CONTEXT_DEV_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CONTEXT_DEV_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CONTEXT_DEV_HOME=/srv/context-dev
context-dev-pp-cli doctor
```

Under `CONTEXT_DEV_HOME=/srv/context-dev`, the four dirs resolve to `/srv/context-dev/config`, `/srv/context-dev/data`, `/srv/context-dev/state`, and `/srv/context-dev/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "context-dev": {
      "command": "context-dev-pp-mcp",
      "env": {
        "CONTEXT_DEV_HOME": "/srv/context-dev"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CONTEXT_DEV_DATA_DIR` overrides an explicit `--home` for that kind. Use `CONTEXT_DEV_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CONTEXT_DEV_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `context-dev-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### brand

Manage brand

- **`context-dev-pp-cli brand create`** - Signal that you may fetch brand data for a particular domain soon to improve latency.
- **`context-dev-pp-cli brand create-ai`** - Given a single URL, determines if it is a product page and extracts the product information.
- **`context-dev-pp-cli brand create-ai-2`** - Extract product information from a brand's website. We will analyze the website and return a list of products with details such as name, description, image, pricing, features, and more.
- **`context-dev-pp-cli brand create-ai-3`** - Use AI to extract specific data points from a brand's website. The AI will crawl the website and extract the requested information based on the provided data points.
- **`context-dev-pp-cli brand create-prefetchbyemail`** - Signal that you may fetch brand data for a particular domain soon to improve latency. This endpoint accepts an email address, extracts the domain from it, validates that it's not a disposable or free email provider, and queues the domain for prefetching.
- **`context-dev-pp-cli brand list`** - Retrieve logos, backdrops, colors, industry, description, and more from any domain
- **`context-dev-pp-cli brand list-retrievebyemail`** - Retrieve brand information using an email address while detecting disposable and free email addresses. Disposable and free email addresses (like gmail.com, yahoo.com) will throw a 422 error.
- **`context-dev-pp-cli brand list-retrievebyisin`** - Retrieve brand information using an ISIN (International Securities Identification Number).
- **`context-dev-pp-cli brand list-retrievebyname`** - Retrieve brand information using a company name.
- **`context-dev-pp-cli brand list-retrievebyticker`** - Retrieve brand information using a stock ticker symbol.
- **`context-dev-pp-cli brand list-retrievesimplified`** - Returns a simplified version of brand data containing only essential information: domain, title, colors, logos, and backdrops. Optimized for faster responses and reduced data transfer.
- **`context-dev-pp-cli brand list-transactionidentifier`** - Endpoint specially designed for platforms that want to identify transaction data by the transaction title.

### people

Manage people

- **`context-dev-pp-cli people`** - Retrieve and normalize a person profile from identifiers.

### web

Manage web

- **`context-dev-pp-cli web create`** - Performs a crawl starting from a given URL, extracts page content as Markdown, and returns results for all crawled pages.
- **`context-dev-pp-cli web create-extract`** - Crawl a website, use the provided JSON Schema and instructions to prioritize relevant internal links, and extract structured data from the selected pages.
- **`context-dev-pp-cli web create-search`** - Search the web and optionally scrape each result to Markdown in one round-trip.
- **`context-dev-pp-cli web list`** - Analyze a company's landing page and web search evidence to return direct competitors for the same product or market.
- **`context-dev-pp-cli web list-fonts`** - Scrape font information from a website including font families, usage statistics, fallbacks, and element/word counts.
- **`context-dev-pp-cli web list-naics`** - Classify any brand into 2022 NAICS industry codes from its domain or name.
- **`context-dev-pp-cli web list-scrape`** - Scrapes the given URL and returns the raw HTML content of the page.
- **`context-dev-pp-cli web list-scrape-2`** - Extract image assets from a web page, including standard URLs, inline SVGs, data URIs, responsive image sources, metadata, CSS backgrounds, video posters, and embeds. The base request costs 1 credit. When enrichment is enabled, the entire call costs 5 credits.
- **`context-dev-pp-cli web list-scrape-3`** - Scrapes the given URL into LLM usable Markdown.
- **`context-dev-pp-cli web list-scrape-4`** - Crawl an entire website's sitemap and return all discovered page URLs.
- **`context-dev-pp-cli web list-screenshot`** - Capture a screenshot of a website.
- **`context-dev-pp-cli web list-sic`** - Classify any brand into Standard Industrial Classification (SIC) codes from its domain or name. Choose between the original SIC system (`original_sic`) or the latest SIC list maintained by the SEC (`latest_sec`).
- **`context-dev-pp-cli web list-styleguide`** - Extract a comprehensive design system from a website including colors, typography, spacing, shadows, and UI components.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
context-dev-pp-cli brand list --domain example-value

# JSON for scripting and agents
context-dev-pp-cli brand list --domain example-value --json

# Filter to specific fields
context-dev-pp-cli brand list --domain example-value --json --select id,name,status

# Dry run — show the request without sending
context-dev-pp-cli brand list --domain example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
context-dev-pp-cli brand list --domain example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
context-dev-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `context-dev-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/context-pp-cli/config.toml`; `--home`, `CONTEXT_DEV_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name                      | Kind     | Required | Description                                              |
| ------------------------- | -------- | -------- | -------------------------------------------------------- |
| `CONTEXT_DEV_API_KEY`     | per_call | Yes      | Preferred Context.dev API credential.                    |
| `CONTEXT_API_KEY`         | per_call | No       | Fallback credential when `CONTEXT_DEV_API_KEY` is unset. |
| `CONTEXT_DEV_BEARER_AUTH` | per_call | No       | Backward-compatible generated credential variable.       |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `context-dev-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting

**Authentication errors (exit code 4)**

- Run `context-dev-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CONTEXT_DEV_API_KEY`
  **Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
