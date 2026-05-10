# Europe PMC CLI

**Every Europe PMC endpoint, plus preprint tracking, citation graphs, and text-mined annotation mining no other tool offers**

Search, cite-track, and mine annotations across PubMed, preprints, European patents, and more. Sync results to a local SQLite store for offline full-text search, preprint lifecycle monitoring, and gene-disease relationship extraction. The only CLI that exposes Europe PMC's annotation API as structured commands.

## Install

The recommended path installs both the `europe-pmc-pp-cli` binary and the `pp-europe-pmc` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install europe-pmc
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install europe-pmc --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/europe-pmc-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-europe-pmc --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-europe-pmc --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-europe-pmc skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-europe-pmc. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Search Europe PMC and get structured JSON
europe-pmc-pp-cli articles query --query "CRISPR base editing" --page-size 10 --json


# Fetch full article metadata by PMID
europe-pmc-pp-cli articles lookup --source MED --id 33024307 --result-type core --json


# Find papers that cite this article
europe-pmc-pp-cli citations list --source MED --id 33024307 --json


# Find papers with text-mined BRCA1 annotations
europe-pmc-pp-cli annotations by-entity --entity BRCA1 --json


# Get hit counts by source (PubMed, preprints, patents)
europe-pmc-pp-cli source-profile breakdown --query "malaria vaccine" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Preprint intelligence
- **`track-preprint`** — Track preprints through their lifecycle from posting to peer-reviewed publication, with time-to-publication metrics

  _When an agent needs to monitor whether preprints in a research area have been peer-reviewed and published_

  ```bash
  europe-pmc-pp-cli track-preprint --query "SRC:PPR AND CRISPR" --check-updates --agent
  ```
- **`ppr-intel`** — Monitor preprint servers by topic, tracking citation velocity and time-to-publication trends

  _When an agent needs early-warning signals about emerging research trends before peer review_

  ```bash
  europe-pmc-pp-cli ppr-intel --topic "machine learning" --since 2025-01 --agent
  ```

### Text mining intelligence
- **`mine-relations`** — Build a knowledge graph of gene-disease-chemical relationships from text-mined annotations across papers

  _When an agent needs structured gene-disease or drug-target relationships extracted from literature_

  ```bash
  europe-pmc-pp-cli mine-relations --gene BRCA1 --disease "breast cancer" --agent
  ```
- **`section-search`** — Search within specific article sections (Methods, Results, Discussion) and extract section-filtered annotations

  _When an agent needs to find how a technique is used in practice (Methods) vs what it found (Results)_

  ```bash
  europe-pmc-pp-cli section-search --query "CRISPR" --section METHODS --agent
  ```

### Citation intelligence
- **`cite-graph`** — Recursively walk citations and references to build a navigable citation graph in local SQLite

  _When an agent needs to find bridge papers, citation clusters, or influential works in a research area_

  ```bash
  europe-pmc-pp-cli cite-graph --source MED --id 33024307 --depth 2 --direction both --agent
  ```

### Systematic review automation
- **`systematic-review`** — Automate PRISMA systematic review workflow with cross-source deduplication across MED, PMC, and PPR

  _When an agent is helping prepare a systematic review and needs PRISMA-compliant identification and deduplication_

  ```bash
  europe-pmc-pp-cli systematic-review --query "GLP-1 agonist cardiovascular" --tag strategy-a --agent
  ```
- **`dedup`** — Resolve any identifier (DOI, PMID, PMCID, PPR ID) to all other identifiers and deduplicate mixed-format ID lists

  _When an agent has collected papers from multiple databases and needs to remove duplicates that differ only in ID scheme_

  ```bash
  europe-pmc-pp-cli dedup --doi 10.1038/s41579-020-00459-7 --agent
  ```

### Research analytics
- **`grant-impact`** — Aggregate grant-linked publications with citation counts, OA compliance rates, and second-order citation impact

  _When an agent is assessing research impact for a funding body or grant review_

  ```bash
  europe-pmc-pp-cli grant-impact --agency "Wellcome Trust" --grant-id WT098051 --agent
  ```

## Usage

Run `europe-pmc-pp-cli --help` for the full command reference and flag list.

## Commands

### annotations

Text-mined annotations (genes, diseases, chemicals, organisms)

- **`europe-pmc-pp-cli annotations by-article`** - 
- **`europe-pmc-pp-cli annotations by-entity`** - 
- **`europe-pmc-pp-cli annotations by-relationship`** - 

### articles

Search and retrieve articles from Europe PMC

- **`europe-pmc-pp-cli articles lookup`** - 
- **`europe-pmc-pp-cli articles query`** - 

### citations

Articles that cite a given publication

- **`europe-pmc-pp-cli citations list`** - 

### database-links

Cross-references to biological databases (UniProt, PDB, ChEBI, etc.)

- **`europe-pmc-pp-cli database-links list`** - 

### datalinks

Unified data-literature links (databases + labs + text-mined)

- **`europe-pmc-pp-cli datalinks list`** - 

### evaluations

Post-publication peer reviews and editorial evaluations

- **`europe-pmc-pp-cli evaluations list`** - 

### fields

List all 143 searchable fields

- **`europe-pmc-pp-cli fields list`** - 

### fulltext

Full-text JATS XML for open-access PMC articles

- **`europe-pmc-pp-cli fulltext get`** - 

### labs-links

External links from third-party providers (Altmetric, BioStudies, etc.)

- **`europe-pmc-pp-cli labs-links list`** - 

### references

Publications referenced by a given article

- **`europe-pmc-pp-cli references list`** - 

### source-profile

Get hit count breakdown by source and publication type

- **`europe-pmc-pp-cli source-profile breakdown`** - 


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
europe-pmc-pp-cli articles query --query example-value

# JSON for scripting and agents
europe-pmc-pp-cli articles query --query example-value --json

# Filter to specific fields
europe-pmc-pp-cli articles query --query example-value --json --select id,name,status

# Dry run — show the request without sending
europe-pmc-pp-cli articles query --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
europe-pmc-pp-cli articles query --query example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-europe-pmc -g
```

Then invoke `/pp-europe-pmc <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add europe-pmc europe-pmc-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/europe-pmc-current).
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
    "europe-pmc": {
      "command": "europe-pmc-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
europe-pmc-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty results for a query that works on europepmc.org** — Europe PMC web uses automatic MeSH expansion; add --synonym true to your query
- **Annotations endpoint returns empty for a PMID** — Use SOURCE:ID format (e.g., MED:33024307), not bare PMID. Max 8 articles per request.
- **Full-text XML returns 404** — Only PMC articles with open access have full text. Use PMC ID format (e.g., PMC7537588), not PMID.
- **Cursor pagination stops early** — Check hitCount in response. If results < hitCount, use nextCursorMark for the next page.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**europepmc (R)**](https://github.com/ropensci/europepmc) — R (55 stars)
- [**pyEuropePMC**](https://github.com/JonasHeinickeBio/pyEuropePMC) — Python (10 stars)
- [**Scientific-Papers-MCP**](https://github.com/benedict2310/Scientific-Papers-MCP) — Python (5 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
