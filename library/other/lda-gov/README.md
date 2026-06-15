# LDA.gov CLI

**Lobbying disclosure records with local sync, entity resolution, and evidence-grade exports.**

Search the official Lobbying Disclosure Act API, sync public filings into SQLite, and turn nested lobbying records into source-linked audits. The CLI keeps anonymous reads safe by default, supports optional registered API keys, and adds commands for entity resolution, anomaly checks, spend timelines, contribution totals, and graph exports.

Learn more at [LDA.gov](https://www.senate.gov/legislative/opr.htm).

Created by [@mherzog4](https://github.com/mherzog4) (Mherzog4).

## Install

The recommended path installs both the `lda-gov-pp-cli` binary and the `pp-lda-gov` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lda-gov
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lda-gov --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lda-gov --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lda-gov --agent claude-code
npx -y @mvanhorn/printing-press-library install lda-gov --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/lda-gov/cmd/lda-gov-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lda-gov-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lda-gov --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lda-gov --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lda-gov --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lda-gov --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lda-gov-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/lda-gov/cmd/lda-gov-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lda-gov": {
      "command": "lda-gov-pp-mcp"
    }
  }
}
```

</details>

## Authentication

LDA.gov works anonymously at a lower rate limit. For higher throughput, register for an API key at https://lda.gov/api/register/ and set LDA_API_KEY to the key value; the CLI sends it as Authorization: Token <key>. Legacy scripts may use SENATE_LDA_API_KEY or USSLDA_KEY, but new usage should prefer LDA_API_KEY.

## Quick Start

```bash
# Check CLI configuration without making a network request.
lda-gov-pp-cli doctor --dry-run

# Seed a bounded local mirror for analysis without exhausting anonymous quota.
lda-gov-pp-cli sync --resources filings,contributions,registrants,clients,lobbyists --resource-param filings:filing_year=2024 --resource-param contributions:filing_year=2024 --max-pages 1 --db ./lda.db

# Search the synced filing corpus locally.
lda-gov-pp-cli search Boeing --type filings --limit 10 --db ./lda.db

# Resolve an ambiguous name across registrants, clients, and lobbyists.
lda-gov-pp-cli entities resolve Boeing --agent --limit 10 --db ./lda.db

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence-grade local audits
- **`audit filings`** — Flag amendments, terminations, duplicate-risk filings, client/registrant conflicts, and missing source URLs.

  _Use this when an agent needs reproducible watchdog flags instead of raw filing rows._

  ```bash
  lda-gov-pp-cli audit filings --agent --limit 25
  ```
- **`audit spend`** — Aggregate disclosed lobbying income and expenses by client, registrant, issue, year, and quarter.

  _Use this when an agent needs analysis-ready spend totals rather than individual filings._

  ```bash
  lda-gov-pp-cli audit spend --client Boeing --from-year 2020 --to-year 2025 --csv
  ```
- **`contributions totals`** — Aggregate LD-203 contribution items by contributor, recipient, payee, item type, year, and report.

  _Use this when an agent needs contribution totals without writing a custom flattener._

  ```bash
  lda-gov-pp-cli contributions totals --year 2024 --csv
  ```

### Entity intelligence
- **`entities resolve`** — Rank matching registrants, clients, and lobbyists with official IDs, counts, last activity, and source URLs.

  _Use this before citing or joining an ambiguous company, firm, or lobbyist name._

  ```bash
  lda-gov-pp-cli entities resolve Boeing --agent --limit 10
  ```
- **`graph export`** — Export client-registrant-lobbyist-issue-government entity edges for graph analysis.

  _Use this when an agent needs a relationship graph for downstream NetworkX, Gephi, or SQL work._

  ```bash
  lda-gov-pp-cli graph export --client Boeing --format csv
  ```
- **`lobbyists covered-positions`** — List lobbyists' covered government positions connected to clients, registrants, and filing periods.

  _Use this when an agent needs exact official covered-position evidence tied to active lobbying records._

  ```bash
  lda-gov-pp-cli lobbyists covered-positions --client Boeing --csv
  ```

### Period monitoring
- **`reports quarter`** — Produce quarter stats for filings, amendments, terminations, top issues, top entities, spend, and LD-203 totals.

  _Use this after quarterly deadlines when an agent needs a compact, source-linked snapshot._

  ```bash
  lda-gov-pp-cli reports quarter --year 2024 --period year_end --agent --select top_issue,top_government_entity,filings
  ```

## Recipes


### Resolve a company before joining records

```bash
lda-gov-pp-cli entities resolve Boeing --agent --limit 10 --db ./lda.db
```

Ranks official registrant, client, and lobbyist matches so an agent can pick the right ID.

### Build a source-linked quarter snapshot

```bash
lda-gov-pp-cli reports quarter --year 2024 --period year_end --agent --select top_issue,top_government_entity,filings --db ./lda.db
```

Narrows a potentially large report to the fields an agent needs in context.

### Audit filings for cleanup flags

```bash
lda-gov-pp-cli audit filings --agent --limit 25 --db ./lda.db
```

Flags amendments, terminations, duplicate risks, and client/registrant conflicts.

### Export a lobbying relationship graph

```bash
lda-gov-pp-cli graph export --client Boeing --format csv --db ./lda.db
```

Creates edge rows connecting clients, registrants, lobbyists, issues, and government entities.

### Summarize LD-203 contribution counterparties

```bash
lda-gov-pp-cli contributions totals --year 2024 --csv --db ./lda.db
```

Flattens contribution items and aggregates them by counterparty and item type.

## Usage

Run `lda-gov-pp-cli --help` for the full command reference and flag list.

## Commands

### clients

Access Client information.

- **`lda-gov-pp-cli clients list`** - Returns all clients matching the provided filters.
- **`lda-gov-pp-cli clients retrieve`** - Returns all clients matching the provided filters.

### constants

An assorted list of constants found in the LDA REST API.

- **`lda-gov-pp-cli constants list-contribution-item-types`** - Returns all ContributionItemTypes.
- **`lda-gov-pp-cli constants list-countries`** - Returns all Countries.
- **`lda-gov-pp-cli constants list-filing-types`** - Returns all FilingTypes.
- **`lda-gov-pp-cli constants list-government-entities`** - Returns all GovernmentEntities.
- **`lda-gov-pp-cli constants list-lobbying-activity-general-issues`** - Returns all LobbyingActivityGeneralIssues.
- **`lda-gov-pp-cli constants list-lobbyist-prefixes`** - Returns all LobbyistPrefixes.
- **`lda-gov-pp-cli constants list-lobbyist-suffixes`** - Returns all LobbyistSuffixes.
- **`lda-gov-pp-cli constants list-states`** - Returns all States.

### contributions

Manage contributions

- **`lda-gov-pp-cli contributions list-reports`** - List reports
- **`lda-gov-pp-cli contributions retrieve-report`** - Returns all contributions matching the provided filters.

### filings

Access LD1 / LD2 filings.

- **`lda-gov-pp-cli filings list`** - List
- **`lda-gov-pp-cli filings retrieve`** - Returns all filings matching the provided filters.

### lobbyists

Access Lobbyist information.

- **`lda-gov-pp-cli lobbyists list`** - Returns all lobbyists matching the provided filters. The ID is a unique integer value identifying this
Lobbyist Name as reported by this Registrant.
- **`lda-gov-pp-cli lobbyists retrieve`** - Returns all lobbyists matching the provided filters. The ID is a unique integer value identifying this
Lobbyist Name as reported by this Registrant.

### registrants

Access Registrant information.

- **`lda-gov-pp-cli registrants list`** - Returns all registrants matching the provided filters.
- **`lda-gov-pp-cli registrants retrieve`** - Returns all registrants matching the provided filters.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lda-gov-pp-cli clients list

# JSON for scripting and agents
lda-gov-pp-cli clients list --json

# Filter to specific fields
lda-gov-pp-cli clients list --json --select id,name,status

# Dry run — show the request without sending
lda-gov-pp-cli clients list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lda-gov-pp-cli clients list --agent
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
lda-gov-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/lobbying-disclosure-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 Too Many Requests** — Wait for the Retry-After window, lower --max-pages, or use a registered key with LDA_API_KEY.
- **Pagination fails after page 1 for filings or contributions** — Add a filter such as --filing-year 2024 or sync with --resource-param filings:filing_year=2024.
- **Old examples point at lda.senate.gov** — Use the default lda.gov base URL; lda.senate.gov is deprecated and sunsets in 2026.
- **Audit command returns an empty JSON array** — Run sync first with the resources named by the command and pass the same --db path.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**lobbyR**](https://github.com/Lobbying-DisclosuRe/lobbyr) — R
- [**lobby**](https://github.com/christopherkenny/lobby) — R
- [**lobbying_disclosure_client**](https://github.com/dl-ee/lobbying_disclosure_client) — Ruby
- [**us-gov-open-data-mcp**](https://github.com/lzinga/us-gov-open-data-mcp) — TypeScript
- [**LegisMCP**](https://github.com/ruchit-p/LegisMCP) — TypeScript
- [**scraper_senate-lobbying-disclosures**](https://github.com/The-Politico/scraper_senate-lobbying-disclosures) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
