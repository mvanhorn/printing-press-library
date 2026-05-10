---
name: pp-ncbi-entrez
description: "Every E-utility, plus a local database for citation tracking, drift detection, and cross-database queries no other... Trigger phrases: `search pubmed`, `find papers about`, `look up gene`, `fetch sequence`, `NCBI search`, `citation snowball`, `pubmed query`."
author: "Test User"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ncbi-entrez-pp-cli
---

# NCBI Entrez — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ncbi-entrez-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ncbi-entrez --cli-only
   ```
2. Verify: `ncbi-entrez-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search, fetch, and link records across PubMed, Gene, Protein, and dozens more NCBI databases. Sync results to a local SQLite store for offline full-text search, citation snowballing, and publication velocity monitoring. The only NCBI CLI with structured JSON output, agent-native design, and persistent state across sessions.

## When to Use This CLI

Use this CLI when you need to search PubMed or other NCBI databases from the terminal, track citation graphs over time, run proximity searches that PubMed cannot express, or build reproducible literature review workflows with PRISMA-compliant deduplication.

## Unique Capabilities

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

## Command Reference

**ecitmatch** — Match citation strings to PubMed IDs

- `ncbi-entrez-pp-cli ecitmatch` — 

**efetch** — Download records in specific formats

- `ncbi-entrez-pp-cli efetch` — 

**egquery** — Get record counts across all NCBI databases for a query

- `ncbi-entrez-pp-cli egquery` — 

**einfo** — List databases or show database field/link metadata

- `ncbi-entrez-pp-cli einfo` — 

**elink** — Find related records within or across NCBI databases

- `ncbi-entrez-pp-cli elink` — 

**epost** — Upload UIDs to the NCBI History server

- `ncbi-entrez-pp-cli epost` — 

**esearch** — Search an NCBI database and return matching UIDs

- `ncbi-entrez-pp-cli esearch` — 

**espell** — Get spelling suggestions for search terms

- `ncbi-entrez-pp-cli espell` — 

**esummary** — Return document summaries (DocSums) for UIDs

- `ncbi-entrez-pp-cli esummary` — 


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ncbi-entrez-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find recent papers on a drug safety signal

```bash
ncbi-entrez-pp-cli esearch --db pubmed --term "semaglutide AND (thyroid OR medullary)" --retmax 20 --agent --select esearchresult.idlist
```

Search for recent papers linking semaglutide to thyroid concerns, returning only key fields for efficient agent processing

### Build a citation graph from seed papers

```bash
ncbi-entrez-pp-cli snowball --seed 35924517 --depth 2 --json
```

Recursively follow cited-by links to discover the full citation network around a paper

### Cross-database gene-to-literature lookup

```bash
ncbi-entrez-pp-cli elink --dbfrom gene --db pubmed --id 672 --json
```

Find all PubMed articles linked to BRCA1 (Gene ID 672)

### Fetch protein sequences in FASTA format

```bash
ncbi-entrez-pp-cli efetch --db protein --id NP_009225.1 --rettype fasta
```

Download the BRCA1 protein sequence

### Monitor publication velocity for a topic

```bash
ncbi-entrez-pp-cli watch list --trending --agent --select query,current_count,velocity,trend
```

Check which of your watched queries are trending up, with only the fields an agent needs

## Auth Setup
Set your API key via environment variable:

```bash
export NCBI_API_KEY="<your-key>"
```

Or persist it in ``.

Run `ncbi-entrez-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ncbi-entrez-pp-cli esearch --db example-value --term example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ncbi-entrez-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ncbi-entrez-pp-cli feedback --stdin < notes.txt
ncbi-entrez-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ncbi-entrez-pp-cli/feedback.jsonl`. They are never POSTed unless `NCBI_ENTREZ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NCBI_ENTREZ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ncbi-entrez-pp-cli profile save briefing --json
ncbi-entrez-pp-cli --profile briefing esearch --db example-value --term example-value
ncbi-entrez-pp-cli profile list --json
ncbi-entrez-pp-cli profile show briefing
ncbi-entrez-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `ncbi-entrez-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ncbi-entrez-pp-mcp -- ncbi-entrez-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ncbi-entrez-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ncbi-entrez-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ncbi-entrez-pp-cli <command> --help`.
