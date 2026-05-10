# NCBI Entrez CLI

**Every E-utility, plus a local database for citation tracking, drift detection, and cross-database queries no other NCBI tool offers**

Search, fetch, and link records across PubMed, Gene, Protein, and dozens more NCBI databases. Sync results to a local SQLite store for offline full-text search, citation snowballing, and publication velocity monitoring. The only NCBI CLI with structured JSON output, agent-native design, and persistent state across sessions.

## Install

The recommended path installs both the `ncbi-entrez-pp-cli` binary and the `pp-ncbi-entrez` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ncbi-entrez
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ncbi-entrez --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ncbi-entrez-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ncbi-entrez --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ncbi-entrez --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ncbi-entrez skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ncbi-entrez. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Search PubMed and get structured JSON
ncbi-entrez-pp-cli esearch --db pubmed --term "CRISPR base editing" --retmax 10 --json


# Fetch a paper's abstract by PMID
ncbi-entrez-pp-cli efetch --db pubmed --id 35924517 --rettype abstract


# Sync results to local SQLite for offline use
ncbi-entrez-pp-cli sync --full


# Proximity search over cached abstracts
ncbi-entrez-pp-cli local-search "adverse NEAR/5 cardiac" --rank bm25


# List all available NCBI databases
ncbi-entrez-pp-cli info

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`snowball`** — Recursively build a citation graph from seed papers, tracking the expanding frontier across sessions so you see exactly which new papers cite your seeds

  _When an agent needs to expand a literature review or detect emerging citation clusters around a drug safety signal_

  ```bash
  ncbi-entrez-pp-cli snowball --seed 35924517,36071053 --depth 2 --agent
  ```
- **`local-search`** — Full-text search over cached abstracts with FTS5 proximity operators, regex, and BM25 ranking that PubMed itself cannot do

  _When an agent needs proximity search or regex patterns over biomedical text that PubMed search cannot express_

  ```bash
  ncbi-entrez-pp-cli local-search "chimeric NEAR/3 receptor" --rank bm25 --agent
  ```

### Surveillance and alerting
- **`watch`** — Track PubMed publication counts over time for a saved query and alert when velocity departs from baseline

  _When an agent is monitoring whether a pharmacovigilance signal is gaining traction in the literature_

  ```bash
  ncbi-entrez-pp-cli watch list --trending --agent
  ```
- **`drift`** — Compare snapshots of a saved search to show which PMIDs appeared, disappeared, or were retracted since a given date

  _When an agent needs to audit what changed in a literature surveillance query for pharmacovigilance compliance_

  ```bash
  ncbi-entrez-pp-cli drift my-safety-review --since 2025-06-01 --agent
  ```

### Systematic review automation
- **`cite-match`** — Resolve a BibTeX or CSV of references to PMIDs via ECitMatch and flag retracted papers in one pass

  _When an agent is validating a manuscript's reference list for retractions or unresolvable citations_

  ```bash
  ncbi-entrez-pp-cli cite-match --input references.bib --check-retractions --agent
  ```

## Usage

Run `ncbi-entrez-pp-cli --help` for the full command reference and flag list.

## Commands

### ecitmatch

Match citation strings to PubMed IDs

- **`ncbi-entrez-pp-cli ecitmatch resolve`** - 

### efetch

Download records in specific formats

- **`ncbi-entrez-pp-cli efetch retrieve`** - 

### egquery

Get record counts across all NCBI databases for a query

- **`ncbi-entrez-pp-cli egquery counts`** - 

### einfo

List databases or show database field/link metadata

- **`ncbi-entrez-pp-cli einfo show`** - 

### elink

Find related records within or across NCBI databases

- **`ncbi-entrez-pp-cli elink find`** - 

### epost

Upload UIDs to the NCBI History server

- **`ncbi-entrez-pp-cli epost upload`** - 

### esearch

Search an NCBI database and return matching UIDs

- **`ncbi-entrez-pp-cli esearch query`** - 

### espell

Get spelling suggestions for search terms

- **`ncbi-entrez-pp-cli espell check`** - 

### esummary

Return document summaries (DocSums) for UIDs

- **`ncbi-entrez-pp-cli esummary docsum`** - 


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ncbi-entrez-pp-cli esearch --db example-value --term example-value

# JSON for scripting and agents
ncbi-entrez-pp-cli esearch --db example-value --term example-value --json

# Filter to specific fields
ncbi-entrez-pp-cli esearch --db example-value --term example-value --json --select id,name,status

# Dry run — show the request without sending
ncbi-entrez-pp-cli esearch --db example-value --term example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ncbi-entrez-pp-cli esearch --db example-value --term example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ncbi-entrez -g
```

Then invoke `/pp-ncbi-entrez <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ncbi-entrez ncbi-entrez-pp-mcp -e NCBI_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ncbi-entrez-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NCBI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ncbi-entrez": {
      "command": "ncbi-entrez-pp-mcp",
      "env": {
        "NCBI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
ncbi-entrez-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NCBI_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ncbi-entrez-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NCBI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 Too Many Requests** — Set NCBI_API_KEY for 10 req/s (free at ncbi.nlm.nih.gov/account/settings)
- **Empty results for a query that works on PubMed website** — PubMed website uses automatic MeSH expansion; add [MeSH] field tags to your query
- **EFetch returns XML when you expected text** — Use --format abstract for plain text or --format medline for MEDLINE format
- **Large result set truncated at 10000** — Use sync with --all to paginate through the full result set

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Biopython Entrez**](https://github.com/biopython/biopython) — Python (4200 stars)
- [**EDirect**](https://github.com/NCBI-Hackathons/EDirectCookbook) — Perl (120 stars)
- [**biogo/ncbi**](https://github.com/biogo/ncbi) — Go (45 stars)
- [**easy-entrez**](https://github.com/krassowski/easy-entrez) — Python (30 stars)
- [**entrez-mcp-server**](https://github.com/QuentinCody/entrez-mcp-server) — Python (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
