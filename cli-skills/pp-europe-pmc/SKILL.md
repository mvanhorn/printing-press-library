---
name: pp-europe-pmc
description: "Every Europe PMC endpoint, plus preprint tracking, citation graphs, and text-mined annotation mining no other tool... Trigger phrases: `search europe pmc`, `find preprints about`, `citation graph`, `text mining annotations`, `BRCA1 literature`, `systematic review`, `europe pmc query`."
author: "Test User"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - europe-pmc-pp-cli
---

# Europe PMC — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `europe-pmc-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install europe-pmc --cli-only
   ```
2. Verify: `europe-pmc-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search, cite-track, and mine annotations across PubMed, preprints, European patents, and more. Sync results to a local SQLite store for offline full-text search, preprint lifecycle monitoring, and gene-disease relationship extraction. The only CLI that exposes Europe PMC's annotation API as structured commands.

## When to Use This CLI

Use this CLI when you need to search biomedical literature across sources PubMed doesn't cover (preprints, European patents, UK theses), extract text-mined gene-disease-chemical relationships, track preprint-to-publication lifecycles, or build citation networks for systematic reviews.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**annotations** — Text-mined annotations (genes, diseases, chemicals, organisms)

- `europe-pmc-pp-cli annotations by-article` — 
- `europe-pmc-pp-cli annotations by-entity` — 
- `europe-pmc-pp-cli annotations by-relationship` — 

**articles** — Search and retrieve articles from Europe PMC

- `europe-pmc-pp-cli articles lookup` — 
- `europe-pmc-pp-cli articles query` — 

**citations** — Articles that cite a given publication

- `europe-pmc-pp-cli citations` — 

**database-links** — Cross-references to biological databases (UniProt, PDB, ChEBI, etc.)

- `europe-pmc-pp-cli database-links` — 

**datalinks** — Unified data-literature links (databases + labs + text-mined)

- `europe-pmc-pp-cli datalinks` — 

**evaluations** — Post-publication peer reviews and editorial evaluations

- `europe-pmc-pp-cli evaluations` — 

**fields** — List all 143 searchable fields

- `europe-pmc-pp-cli fields` — 

**fulltext** — Full-text JATS XML for open-access PMC articles

- `europe-pmc-pp-cli fulltext` — 

**labs-links** — External links from third-party providers (Altmetric, BioStudies, etc.)

- `europe-pmc-pp-cli labs-links` — 

**references** — Publications referenced by a given article

- `europe-pmc-pp-cli references` — 

**source-profile** — Get hit count breakdown by source and publication type

- `europe-pmc-pp-cli source-profile` — 


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
europe-pmc-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Search for preprints on a topic

```bash
europe-pmc-pp-cli articles query --query "SRC:PPR AND semaglutide" --result-type core --agent --select resultList.result.title,resultList.result.doi,resultList.result.firstPublicationDate
```

Find bioRxiv/medRxiv preprints about semaglutide, returning only key fields

### Find gene-disease co-occurrences

```bash
europe-pmc-pp-cli annotations by-relationship --first-entity BRCA1 --second-entity "breast cancer" --json
```

Find papers where BRCA1 and breast cancer are co-mentioned in text-mined annotations

### Get citation graph for a paper

```bash
europe-pmc-pp-cli cite-graph --source MED --id 33024307 --depth 1 --json
```

Build a one-hop citation network around a key paper

### Check source breakdown for a query

```bash
europe-pmc-pp-cli source-profile breakdown --query "GLP-1 agonist safety" --json
```

See how many results come from PubMed vs preprints vs patents

### Get database cross-links for an article

```bash
europe-pmc-pp-cli database-links --source MED --id 33024307 --database UNIPROT --agent --select dbCrossReferenceList
```

Find UniProt protein entries linked to this paper

## Auth Setup

No authentication required.

Run `europe-pmc-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  europe-pmc-pp-cli articles query --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
europe-pmc-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
europe-pmc-pp-cli feedback --stdin < notes.txt
europe-pmc-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.europe-pmc-pp-cli/feedback.jsonl`. They are never POSTed unless `EUROPE_PMC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EUROPE_PMC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
europe-pmc-pp-cli profile save briefing --json
europe-pmc-pp-cli --profile briefing articles query --query example-value
europe-pmc-pp-cli profile list --json
europe-pmc-pp-cli profile show briefing
europe-pmc-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `europe-pmc-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add europe-pmc-pp-mcp -- europe-pmc-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which europe-pmc-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   europe-pmc-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `europe-pmc-pp-cli <command> --help`.
