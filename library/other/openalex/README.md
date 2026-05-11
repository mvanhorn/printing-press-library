# Openalex CLI

The OpenAlex API provides access to a comprehensive catalog of scholarly works, authors, sources, institutions, topics, keywords, publishers, and funders. OpenAlex indexes over 250 million scholarly works.

Learn more at [Openalex](https://openalex.org).

Printed by [@hiten-shah](https://github.com/hiten-shah) (Hiten Shah).

## Install

The recommended path installs both the `openalex-pp-cli` binary and the `pp-openalex` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install openalex
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install openalex --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openalex-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openalex --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openalex --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-openalex skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-openalex. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export OPENALEX_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/openalex-pp-cli/config.toml`.

### 3. Verify Setup

```bash
openalex-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
openalex-pp-cli authors list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Research graph retrieval

- **`works list`** — Search scholarly works with compact JSON, field selection, filters, sorting, and live API fallback.
- **`authors list`** — Search authors and retrieve canonical OpenAlex author records for citation and collaborator research.
- **`institutions list`** — Search institutions with country/type metadata for affiliation and ecosystem analysis.
- **`sources list`** — Search journals, repositories, and venues with compact agent-oriented output.
- **`topics list`** — Explore OpenAlex topics for research-area discovery and downstream filtering.

## Usage

Run `openalex-pp-cli --help` for the full command reference and flag list.

## Commands

### authors

People who create scholarly works

- **`openalex-pp-cli authors get`** - Retrieve a single author by OpenAlex ID or ORCID.
- **`openalex-pp-cli authors list`** - Get a list of authors with optional filtering, searching, sorting, and pagination.

### autocomplete

Fast typeahead search for any entity type

- **`openalex-pp-cli autocomplete autocomplete`** - Fast typeahead search returning up to 10 results. Use for search-as-you-type interfaces.

### awards

Research grants and funding awards

- **`openalex-pp-cli awards get`** - Retrieve a single award by its OpenAlex ID.
- **`openalex-pp-cli awards list`** - Get a list of research grants and funding awards.

### changefiles

Manage changefiles

- **`openalex-pp-cli changefiles get`** - Get details for a specific date's changefile, including which entity types changed, how many records, and download links in JSONL and Parquet formats. For a full guide on using changefiles to keep your local data current, see [Download Changefiles](/download/changefiles). **Requires a paid plan** — contact sales@openalex.org or see [pricing](https://openalex.org/pricing).
- **`openalex-pp-cli changefiles list`** - List all available changefile dates. Each date has downloadable files containing every entity record that was created or modified on that day. For a full guide on using changefiles to keep your local data current, see [Download Changefiles](/download/changefiles). **Requires a paid plan** — contact sales@openalex.org or see [pricing](https://openalex.org/pricing).

### concepts

Legacy taxonomy of research areas (deprecated - use Topics instead)

- **`openalex-pp-cli concepts get`** - **DEPRECATED:** Use Topics instead. Retrieve a single concept by OpenAlex ID.
- **`openalex-pp-cli concepts list`** - **DEPRECATED:** Use Topics instead. Get a list of concepts from the legacy taxonomy.

### continents

Geographic continents (7 total)

- **`openalex-pp-cli continents get`** - Retrieve a single continent by its Wikidata Q-ID.
- **`openalex-pp-cli continents list`** - Get a list of continents (7 total).

### countries

Geographic countries for filtering research by location

- **`openalex-pp-cli countries get-country`** - Retrieve a single country by its ISO 3166-1 alpha-2 code.
- **`openalex-pp-cli countries list`** - Get a list of countries. Useful for filtering works by author affiliation country.

### domains

Top-level categories in the topic hierarchy (4 total)

- **`openalex-pp-cli domains get`** - Retrieve a single domain by its ID (1-4).
- **`openalex-pp-cli domains list`** - Get a list of domains (top-level topic categories). There are only 4 domains: Life Sciences, Social Sciences, Physical Sciences, and Health Sciences.

### fields

Second-level categories in the topic hierarchy (26 total)

- **`openalex-pp-cli fields get`** - Retrieve a single field by its ID.
- **`openalex-pp-cli fields list`** - Get a list of fields (second-level topic categories). There are 26 fields spread across 4 domains.

### funders

Organizations that fund research

- **`openalex-pp-cli funders get`** - Retrieve a single funder by OpenAlex ID or Crossref Funder ID.
- **`openalex-pp-cli funders list`** - Get a list of funders with optional filtering, searching, sorting, and pagination.

### institution-types

Types of institutions (education, healthcare, company, etc.)

- **`openalex-pp-cli institution-types list`** - Get a list of institution types (education, healthcare, company, etc.).

### institutions

Universities, research organizations, and other affiliations

- **`openalex-pp-cli institutions get`** - Retrieve a single institution by OpenAlex ID or ROR.
- **`openalex-pp-cli institutions list`** - Get a list of institutions with optional filtering, searching, sorting, and pagination.

### keywords

Short phrases identified from works' topics

- **`openalex-pp-cli keywords get`** - Retrieve a single keyword by OpenAlex ID.
- **`openalex-pp-cli keywords list`** - Get a list of keywords with optional filtering, searching, sorting, and pagination.

### languages

Languages of scholarly works

- **`openalex-pp-cli languages get`** - Retrieve a single language by its ISO 639-1 code.
- **`openalex-pp-cli languages list`** - Get a list of languages used in scholarly works.

### licenses

Open access licenses (CC BY, CC BY-SA, etc.)

- **`openalex-pp-cli licenses list`** - Get a list of open access licenses (CC BY, CC BY-SA, etc.).

### publishers

Companies and organizations that publish scholarly works

- **`openalex-pp-cli publishers get`** - Retrieve a single publisher by OpenAlex ID.
- **`openalex-pp-cli publishers list`** - Get a list of publishers with optional filtering, searching, sorting, and pagination.

### rate-limit

Manage rate limit

- **`openalex-pp-cli rate-limit get`** - Check your current rate limit status including usage and remaining allowance.

### sdgs

UN Sustainable Development Goals (17 total)

- **`openalex-pp-cli sdgs get`** - Retrieve a single Sustainable Development Goal by its ID (1-17).
- **`openalex-pp-cli sdgs list`** - Get a list of UN Sustainable Development Goals. There are 17 SDGs.

### source-types

Types of sources (journal, repository, conference, etc.)

- **`openalex-pp-cli source-types list`** - Get a list of source types (journal, repository, conference, etc.).

### sources

Journals, repositories, and other venues where works are hosted

- **`openalex-pp-cli sources get`** - Retrieve a single source by OpenAlex ID or ISSN.
- **`openalex-pp-cli sources list`** - Get a list of sources (journals, repositories, conferences) with optional filtering, searching, sorting, and pagination.

### subfields

Third-level categories in the topic hierarchy (254 total)

- **`openalex-pp-cli subfields get`** - Retrieve a single subfield by its ID.
- **`openalex-pp-cli subfields list`** - Get a list of subfields (third-level topic categories). There are 254 subfields spread across 26 fields.

### text

Manage text

- **`openalex-pp-cli text classify`** - **DEPRECATED:** This endpoint is deprecated and not recommended for new projects. It will not receive updates or support.

Classify arbitrary text to find relevant OpenAlex topics. Costs $0.01 per request.

### topics

Research topics automatically assigned to works

- **`openalex-pp-cli topics get`** - Retrieve a single topic by OpenAlex ID.
- **`openalex-pp-cli topics list`** - Get a list of topics with optional filtering, searching, sorting, and pagination. Topics are research areas automatically assigned to works.

### work-types

Types of scholarly works (article, book, dataset, etc.)

- **`openalex-pp-cli work-types list`** - Get a list of work types (article, book, dataset, etc.).

### works

Scholarly documents like journal articles, books, datasets, and theses

- **`openalex-pp-cli works get`** - Retrieve a single work by its OpenAlex ID or external ID (DOI, PMID, PMCID, MAG ID). External IDs can be passed as full URLs or URN format (e.g., `doi:10.1234/example` or `pmid:12345678`).
- **`openalex-pp-cli works list`** - Get a list of scholarly works with optional filtering, searching, sorting, and pagination. Works include journal articles, books, datasets, theses, and more.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openalex-pp-cli authors list

# JSON for scripting and agents
openalex-pp-cli authors list --json

# Filter to specific fields
openalex-pp-cli authors list --json --select id,name,status

# Dry run — show the request without sending
openalex-pp-cli authors list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openalex-pp-cli authors list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-openalex -g
```

Then invoke `/pp-openalex <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add openalex openalex-pp-mcp -e OPENALEX_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openalex-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENALEX_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openalex": {
      "command": "openalex-pp-mcp",
      "env": {
        "OPENALEX_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
openalex-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/openalex-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENALEX_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openalex-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENALEX_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
