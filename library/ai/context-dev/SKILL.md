---
name: pp-context-dev
description: "Printing Press CLI for Context.dev. Agent-friendly website intelligence, brand enrichment, scraping, crawling, structured extraction, screenshots, styleguides, competitor maps, source packs, and change digests."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - context-dev-pp-cli
    install:
      - kind: go
        bins: [context-dev-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/context-dev/cmd/context-dev-pp-cli
---

# Context Dev — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `context-dev-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install context-dev --cli-only
   ```
2. Verify: `context-dev-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/context-dev/cmd/context-dev-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Agent-friendly Context.dev CLI for website intelligence, brand enrichment, scraping, crawling, structured extraction, screenshots, styleguides, competitor maps, source packs, and change digests.

## Command Reference

**first-class workflows** — Prefer these for common Context.dev operator tasks.

- `context-dev-pp-cli scrape <url> --agent` — Scrape one URL to clean Markdown.
- `context-dev-pp-cli crawl <seed> --max-pages 5 --agent` — Crawl same-domain pages. Use `--estimate`; above 25 pages requires `--confirm` or `--yes`.
- `context-dev-pp-cli extract <url> --schema schema.json --agent` — Extract typed JSON using a JSON Schema file.
- `context-dev-pp-cli styleguide <domain|url> --agent` — Extract design-system data.
- `context-dev-pp-cli screenshot <domain|url> --agent` — Capture a screenshot preview.
- `context-dev-pp-cli entity-discover --type company --name "<name>" --location "<place>" --agent` — Fielded public entity discovery with search, brand/scrape enrichment, ranking, and provenance. Valid types: `company`, `venue`, `provider`, `school`, `agency`, `other`.
- `context-dev-pp-cli brand-brief <domain|url> --agent` — Normalized brand profile with domain, website, title, description, logo, colors, fonts, socials, contact surfaces, screenshot, summary, and provenance.
- `context-dev-pp-cli competitor-map --domain <domain|url> --max 5 --agent` — Competitor set clustered by category/market with `why_ranked`, overlap signals, and provenance. Use `--query` when no seed domain exists.
- `context-dev-pp-cli crawl-budget-plan <seed> --max-pages 25 --agent` — Estimate `urlRegex`, max pages, credits, risks, and recommended crawl command without spending crawl credits.
- `context-dev-pp-cli source-pack --query "<query>" --max-sources 5 --agent` — Search and scrape cited source packs. Add `--schema schema.json` to run extraction per source. Search failure is non-zero; zero results are `status: "no_results"`.
- `context-dev-pp-cli website-change-digest <domain|url> --agent` — Snapshot scrape/styleguide/screenshot in the CLI state dir and diff against the prior local snapshot.
- `context-dev-pp-cli schema-lab --url <url> --url <url> --schema schema.json --agent` — Run an extraction schema across samples and report field fill rates, parse failures, misses, raw statuses, and provenance.
- `context-dev-pp-cli brand-kit <domain|url> --agent` — On-demand brand kit (alias: `asset-pack`): logo, palette, fonts, styleguide, screenshot, favicon, socials, provenance.
- `context-dev-pp-cli brand-qa <domain|url> --question "What is the return policy?" --agent` — Ask a natural-language question about a brand's website and get a grounded answer with the URLs analyzed and provenance.
- `context-dev-pp-cli email-enrich founders@example.com --agent` — Turn a work email into a company profile and signup-form prefill fields (company name, website, industry) with provenance.
- `context-dev-pp-cli ticker-enrich AAPL --agent` — Resolve a public company from a stock ticker or ISIN to a brand profile plus NAICS/SIC industry codes. Auto-detects ticker vs ISIN; pass `--exchange` to disambiguate a ticker.
- `context-dev-pp-cli trust-check <domain|url> --agent` — Consistency/risk signal report across website, socials, address, phone, logo, title/domain, and web signals. Do not describe output as fraud determination.
- `context-dev-pp-cli lead-enrich-batch leads.csv --domain-column domain --name-column name --location-column location --output enriched.json --resume --agent` — Batch-enrich CSV rows with per-row success/error, provenance, and failure reason. Use `--strict` only when one bad row should fail the batch.


For the second-wave workflows, prefer `--agent` for machine-stable JSON. Multi-credit commands support `--estimate`; all workflows support `--dry-run`. Estimate and dry-run responses contain `estimated_credits` and `planned_requests` and do not spend credits. `website-change-digest` persists snapshots under the resolved CLI state directory, never under the repo. `entity-discover` rejects free-form blobs and sensitive/person-identifying context; pass only public field values.

**brand** — Manage brand

- `context-dev-pp-cli brand create` — Signal that you may fetch brand data for a particular domain soon to improve latency.
- `context-dev-pp-cli brand create-ai` — Given a single URL, determines if it is a product page and extracts the product information.
- `context-dev-pp-cli brand create-ai-2` — Extract product information from a brand's website.
- `context-dev-pp-cli brand create-ai-3` — Use AI to extract specific data points from a brand's website.
- `context-dev-pp-cli brand create-prefetchbyemail` — Signal that you may fetch brand data for a particular domain soon to improve latency.
- `context-dev-pp-cli brand list` — Retrieve logos, backdrops, colors, industry, description, and more from any domain
- `context-dev-pp-cli brand list-retrievebyemail` — Retrieve brand information using an email address while detecting disposable and free email addresses.
- `context-dev-pp-cli brand list-retrievebyisin` — Retrieve brand information using an ISIN (International Securities Identification Number).
- `context-dev-pp-cli brand list-retrievebyname` — Retrieve brand information using a company name.
- `context-dev-pp-cli brand list-retrievebyticker` — Retrieve brand information using a stock ticker symbol.
- `context-dev-pp-cli brand list-retrievesimplified` — Returns a simplified version of brand data containing only essential information: domain, title, colors, logos
- `context-dev-pp-cli brand list-transactionidentifier` — Endpoint specially designed for platforms that want to identify transaction data by the transaction title.

**people** — Manage people

- `context-dev-pp-cli people` — Retrieve and normalize a person profile from identifiers.

**web** — Manage web

- `context-dev-pp-cli web create` — Performs a crawl starting from a given URL, extracts page content as Markdown
- `context-dev-pp-cli web create-extract` — Crawl a website, use the provided JSON Schema and instructions to prioritize relevant internal links
- `context-dev-pp-cli web create-search` — Search the web and optionally scrape each result to Markdown in one round-trip.
- `context-dev-pp-cli web list` — Analyze a company's landing page and web search evidence to return direct competitors for the same product or market.
- `context-dev-pp-cli web list-fonts` — Scrape font information from a website including font families, usage statistics, fallbacks, and element/word counts.
- `context-dev-pp-cli web list-naics` — Classify any brand into 2022 NAICS industry codes from its domain or name.
- `context-dev-pp-cli web list-scrape` — Scrapes the given URL and returns the raw HTML content of the page.
- `context-dev-pp-cli web list-scrape-2` — Extract image assets from a web page, including standard URLs, inline SVGs, data URIs, responsive image sources
- `context-dev-pp-cli web list-scrape-3` — Scrapes the given URL into LLM usable Markdown.
- `context-dev-pp-cli web list-scrape-4` — Crawl an entire website's sitemap and return all discovered page URLs.
- `context-dev-pp-cli web list-screenshot` — Capture a screenshot of a website.
- `context-dev-pp-cli web list-sic` — Classify any brand into Standard Industrial Classification (SIC) codes from its domain or name.
- `context-dev-pp-cli web list-styleguide` — Extract a comprehensive design system from a website including colors, typography, spacing, shadows, and UI components.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
context-dev-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `context-dev-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
context-dev-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `CONTEXT_DEV_API_KEY` as an environment variable. `CONTEXT_API_KEY` is accepted as a fallback when `CONTEXT_DEV_API_KEY` is unset; the legacy generated `CONTEXT_DEV_BEARER_AUTH` variable remains supported for compatibility.

Run `context-dev-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  context-dev-pp-cli brand list --domain example-value --agent --select id,name,status
  ```

- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CONTEXT_DEV_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CONTEXT_DEV_CONFIG_DIR`, `CONTEXT_DEV_DATA_DIR`, `CONTEXT_DEV_STATE_DIR`, `CONTEXT_DEV_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CONTEXT_DEV_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `context-dev-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CONTEXT_DEV_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CONTEXT_DEV_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
context-dev-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
context-dev-pp-cli feedback --stdin < notes.txt
context-dev-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CONTEXT_DEV_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CONTEXT_DEV_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what _surprised_ you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink            | Effect                                                                                          |
| --------------- | ----------------------------------------------------------------------------------------------- |
| `stdout`        | Default; write to stdout only                                                                   |
| `file:<path>`   | Atomically write output to `<path>` (tmp + rename)                                              |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
context-dev-pp-cli profile save briefing --json
context-dev-pp-cli --profile briefing brand list --domain example-value
context-dev-pp-cli profile list --json
context-dev-pp-cli profile show briefing
context-dev-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning                       |
| ---- | ----------------------------- |
| 0    | Success                       |
| 2    | Usage error (wrong arguments) |
| 3    | Resource not found            |
| 4    | Authentication required       |
| 5    | API error (upstream issue)    |
| 7    | Rate limited (wait and retry) |
| 10   | Config error                  |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `context-dev-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/context-dev/cmd/context-dev-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add context-dev-pp-mcp -- context-dev-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which context-dev-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   context-dev-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `context-dev-pp-cli <command> --help`.
