# Scientific Consensus CLI

**Aggregate PubMed, OpenAlex, Crossref, and Europe PMC into evidence summaries, consensus scores, and study-design pyramids no single search tool produces.**

Scientific Consensus turns large collections of papers into actionable evidence. It scores consensus across sources (`consensus`), classifies studies by design and renders evidence pyramids (`evidence`), detects gaps and controversies, and persists everything to a local SQLite store you can query offline with `--json`. Fully keyless; optional AI keys upgrade summarization.

Learn more at [Scientific Consensus](https://docs.openalex.org/).

Created by [@laci141](https://github.com/laci141) (laci141).

## Install

The recommended path installs both the `scientific-consensus-pp-cli` binary and the `pp-scientific-consensus` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus --agent claude-code
npx -y @mvanhorn/printing-press-library install scientific-consensus --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/cmd/scientific-consensus-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scientific-consensus-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-scientific-consensus --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-scientific-consensus --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install scientific-consensus --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scientific-consensus-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/cmd/scientific-consensus-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scientific-consensus": {
      "command": "scientific-consensus-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No API key required for any command. Optional env vars raise limits or enable AI summarization: NCBI_API_KEY (PubMed, higher rate limit), SEMANTIC_SCHOLAR_API_KEY (Semantic Scholar enrichment), and ANTHROPIC_API_KEY / OPENAI_API_KEY / DEEPSEEK_API_KEY / GEMINI_API_KEY (enhanced summarization; first configured key wins — DeepSeek sits after Anthropic/OpenAI and before Gemini/Groq/Mistral; OpenAI-compatible providers sample at temperature 0). Everything works without them.

## Quick Start

```bash
# Verify per-source reachability before running analyses.
scientific-consensus doctor --dry-run

# Cross-source search to confirm data flows.
scientific-consensus search "vitamin d covid" --limit 10

# The headline command: an evidence-backed consensus verdict.
scientific-consensus consensus "vitamin D reduces respiratory infections"

# See the evidence pyramid for the same topic.
scientific-consensus evidence "vitamin d covid"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence intelligence
- **`consensus`** — Answer 'what does the evidence say about X' with a Consensus Score, Confidence Score, and Evidence Strength across all sources.

  _Reach for this when an agent needs an evidence-backed yes/no/mixed verdict instead of a raw paper list._

  ```bash
  scientific-consensus consensus "vitamin D reduces respiratory infections" --agent
  ```
- **`evidence`** — Classify retrieved studies by design (meta-analysis to case report) and render the evidence pyramid for a topic.

  _Reach for this to judge whether a claim rests on RCTs/meta-analyses or just case series._

  ```bash
  scientific-consensus evidence "intermittent fasting weight loss" --agent
  ```
- **`compare`** — Run two consensus analyses side-by-side to compare competing claims or interventions.

  _Reach for this when an agent must weigh two interventions or contradictory claims._

  ```bash
  scientific-consensus compare "statins reduce mortality" "statins increase diabetes risk" --agent
  ```
- **`reproducibility`** — Estimate reproducibility by detecting replication studies, sample sizes, and pre-registration cues.

  _Reach for this to gauge how well-replicated a finding is._

  ```bash
  scientific-consensus reproducibility "power posing" --agent
  ```
- **`quality`** — Estimate overall study quality from design, venue prestige, sample-size cues, and citation mass.

  _Reach for this for a quick quality signal before deep reading._

  ```bash
  scientific-consensus quality "omega-3 depression" --agent
  ```

### Discovery
- **`gaps`** — Identify understudied populations, missing long-term/replication/RCT studies, and future directions for a topic.

  _Reach for this to find what research is missing, not just what exists._

  ```bash
  scientific-consensus gaps "pediatric long covid" --agent
  ```
- **`controversies`** — Surface conflicting studies, contradictory conclusions, and rapidly changing evidence for a topic.

  _Reach for this when the question is 'is this settled or disputed?'_

  ```bash
  scientific-consensus controversies "saturated fat heart disease" --agent
  ```
- **`funding`** — Analyze funding patterns and funder concentration for a research topic.

  _Reach for this to see who funds research on a topic (potential conflicts)._

  ```bash
  scientific-consensus funding "e-cigarette safety" --agent
  ```

### Utilities
- **`convert`** — Translate a DOI to a PMID or vice versa using the OpenAlex work index. Pass exactly one of `--doi` or `--pmid`; the other identifier is returned along with the title.

  _Reach for this whenever a tool has one identifier type but needs the other._

  ```bash
  scientific-consensus convert --doi 10.1136/bmj.i6583
  scientific-consensus convert --pmid 32939066
  ```
- **`batch`** — Run consensus analysis for multiple claims from one or more files (plain text, one claim per line, blank lines and `#` comments skipped). Accepts globs; duplicates are deduplicated. Returns a summary table or a flat JSON array.

  _Reach for this to score many claims at once without shell-looping._

  ```bash
  scientific-consensus batch claims.txt
  scientific-consensus batch claims*.txt --limit 20 --json
  ```
- **`report`** — Export an analyzed works report for a topic as an Excel (`.xlsx`) workbook. The `Works` sheet has one row per study (title, first author, year, DOI, PMID, venue, design, stance, stance confidence, citations, open access); the `Summary` sheet carries the query metadata and stance/design aggregates. Same classification engine as `consensus` and `evidence` — but the destination is a spreadsheet, not a terminal.

  _Reach for this to hand results to Excel, Google Sheets, or any spreadsheet-based literature-screening workflow._

  ```bash
  scientific-consensus report "vitamin D respiratory infections" --output report.xlsx
  scientific-consensus report "microplastics" -o mp.xlsx --claim "microplastics harm human health" --limit 100
  ```
- **`citations`** — Build a citation network around a seed work (by `--doi`, `--pmid`, or OpenAlex `--id`). `--depth` (max 2) controls hop count; `--max-nodes` caps total nodes and API calls. `--direction` selects `cited-by`, `references`, or `both`. `--json` emits flat `nodes` + `edges` arrays for a web graph renderer; the default is a compact human summary.

  _Reach for this to trace influence, find high-impact neighbors, or build a network visualization._

  ```bash
  scientific-consensus citations --doi 10.1136/bmj.i6583
  scientific-consensus citations --id W2741809807 --depth 2 --max-nodes 80 --direction cited-by --json
  ```

### Trends
- **`emerging`** — Detect the fastest-growing research areas and exploding publication trends.

  _Reach for this to spot hot research areas before they peak._

  ```bash
  scientific-consensus emerging --field neuroscience --agent
  ```
- **`drift`** — Compare a field's topic distribution between two year windows to spot emerging and fading subtopics.

  _Reach for this to see how a field's focus shifted over time._

  ```bash
  scientific-consensus drift "machine learning genomics" --from 2015 --to 2025 --agent
  ```
- **`watch`** — Monitor a topic and report major new publications since the last run.

  _Reach for this to keep an agent or researcher current on a fast-moving topic._

  ```bash
  scientific-consensus watch "GLP-1 cardiovascular outcomes" --agent
  ```

## Recipes


### Evidence-backed verdict for an agent

```bash
scientific-consensus consensus "creatine improves cognition" --agent --select verdict,consensus_score,confidence,study_count
```

Returns a compact JSON verdict an agent can act on without parsing papers.

### Evidence pyramid as a table

```bash
scientific-consensus evidence "mediterranean diet cardiovascular" --csv
```

Study-design distribution from meta-analyses down to case reports.

### Compare two claims

```bash
scientific-consensus compare "intermittent fasting weight loss" "calorie counting weight loss" --agent
```

Side-by-side consensus and evidence strength for competing approaches.

### Curate a deduped reading list

```bash
scientific-consensus curate "crispr off-target effects" --format bibtex --limit 25
```

Ranked, cross-source, DOI-deduplicated reading list exported as BibTeX.

### Track a fast-moving topic

```bash
scientific-consensus watch "GLP-1 cardiovascular outcomes" --agent
```

Reports new publications since the last run from the local baseline.

### Translate a DOI to PMID

```bash
scientific-consensus convert --doi 10.1136/bmj.i6583
```

Returns the PMID, canonical DOI, and title from the OpenAlex work index. Use `--pmid` for the reverse direction.

### Batch consensus across many claims

```bash
scientific-consensus batch claims.txt --limit 20 --json
```

One claim per line; blank lines and `#` comments skipped; globs (`claims*.txt`) and duplicate files handled automatically. `--json` returns a flat array: `[{"claim":"...","result":{...}}, ...]`. Per-claim errors are recorded in `"error"` and do not abort the batch.

### Explore a citation network

```bash
scientific-consensus citations --doi 10.1136/bmj.i6583 --depth 1 --max-nodes 50 --json
```

Returns `{"nodes":[{"id","title","year","cited_by_count"},...], "edges":[{"from","to"},...]}`  bounded by `--max-nodes`. `--direction` selects `cited-by` (papers citing the seed), `references` (papers the seed cites), or `both` (default). `--depth 2` expands one additional hop.

## Usage

Run `scientific-consensus-pp-cli --help` for the full command reference and flag list.

## Commands

### authors

Search and retrieve authors

- **`scientific-consensus-pp-cli authors get`** - Get a single author by OpenAlex ID
- **`scientific-consensus-pp-cli authors search`** - Search authors

### funders

Research funders

- **`scientific-consensus-pp-cli funders`** - Search funders

### institutions

Search and retrieve institutions

- **`scientific-consensus-pp-cli institutions get`** - Get a single institution by OpenAlex ID
- **`scientific-consensus-pp-cli institutions search`** - Search institutions

### sources

Journal (source) metadata

- **`scientific-consensus-pp-cli sources get`** - Get a journal (source) by ISSN or OpenAlex ID
- **`scientific-consensus-pp-cli sources search`** - Search sources (journals)

### topics

Research topics

- **`scientific-consensus-pp-cli topics`** - Search topics

### works

Search and retrieve scholarly works

- **`scientific-consensus-pp-cli works get`** - Get a single work by OpenAlex ID, DOI, or PMID
- **`scientific-consensus-pp-cli works search`** - Search scholarly works

### convert

DOI ↔ PMID identifier translation (keyless, via OpenAlex)

- **`scientific-consensus-pp-cli convert --doi <doi>`** - Look up the PMID (and title) for a DOI
- **`scientific-consensus-pp-cli convert --pmid <pmid>`** - Look up the DOI (and title) for a PMID

Exactly one of `--doi` or `--pmid` is required. DOI and PMID inputs are normalized (strips `https://doi.org/`, `doi:`, `pmid:` prefixes; lowercases DOIs). Returns `{"input","input_type","found","doi","pmid","title"}` under `--json`.

### batch

Batch consensus over claim files

- **`scientific-consensus-pp-cli batch <file|glob> [...]`** - Score every claim in one or more files

One claim per line; blank lines and lines beginning with `#` are skipped. Multiple arguments are expanded as globs and deduplicated. Supports `--limit`, `--year-from`, and `--enrich` (same as `consensus`). `--json` output is a flat array of `{"claim","result":{...}}` objects; items with lookup failures carry `"error"` instead of `"result"`.

### report

Excel report export (keyless, via OpenAlex)

- **`scientific-consensus-pp-cli report <query> --output <file.xlsx>`** - Export analyzed works as a two-sheet Excel workbook

Key flags:

| Flag | Default | Notes |
|------|---------|-------|
| `--output`, `-o` | (required) | Path of the `.xlsx` file to write |
| `--claim` | the query | Claim the stance classifier scores each work against |
| `--filter` | none | OpenAlex filter expression (e.g. `from_publication_date:2020-01-01`) |
| `--limit` | 50 | Maximum works to analyze and export (max 200) |

The workbook always contains both sheets; an empty result set still produces a valid file with headers so downstream tooling never sees a missing artifact. With `--json`/`--agent` the command prints a machine-readable summary (`file`, `works`, `total_matches`, stance counts, `apex_design`, `stance_method`) instead of prose.

### citations

Citation-network graph (keyless, via OpenAlex)

- **`scientific-consensus-pp-cli citations --doi <doi>`** - Build a citation graph from a DOI seed
- **`scientific-consensus-pp-cli citations --pmid <pmid>`** - Build a citation graph from a PMID seed
- **`scientific-consensus-pp-cli citations --id <W...>`** - Build a citation graph from an OpenAlex ID seed

Exactly one seed flag is required. Key flags:

| Flag | Default | Notes |
|------|---------|-------|
| `--depth` | 1 | Hops to expand (max 2) |
| `--max-nodes` | 50 | Hard cap on nodes and API calls |
| `--direction` | `both` | `both`, `cited-by`, or `references` |

`--json` output: `{"seed","seed_title","depth","direction","node_count","edge_count","nodes":[...],"edges":[...]}`. Each node: `{"id","title","year","cited_by_count"}`. Each edge: `{"from","to"}` (always `citer → cited`). A node that fails to fetch is skipped; one missing work does not abort the graph.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
scientific-consensus-pp-cli authors get mock-value

# JSON for scripting and agents
scientific-consensus-pp-cli authors get mock-value --json

# Filter to specific fields
scientific-consensus-pp-cli authors get mock-value --json --select id,name,status

# Dry run — show the request without sending
scientific-consensus-pp-cli authors get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
scientific-consensus-pp-cli authors get mock-value --agent
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
- **Progress-silent in agent/pipe contexts** - analysis commands print a transient progress line to stderr in interactive terminals; it is suppressed automatically under `--json`, `--agent`, `--compact`, `--csv`, `--quiet`, `--plain`, `--select`, and any non-TTY stderr

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
scientific-consensus-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/scientific-consensus-via-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Semantic Scholar results missing or sparse** — Semantic Scholar rate-limits keyless requests (HTTP 429). Set SEMANTIC_SCHOLAR_API_KEY or rely on OpenAlex/PubMed/Europe PMC; commands degrade gracefully.
- **PubMed throttling on large syncs** — Set NCBI_API_KEY to raise the rate limit from 3 to 10 requests/second.
- **consensus reports method=heuristic** — Stance classification is lexical without an AI key. Set ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, or GEMINI_API_KEY for AI-assisted stance detection (priority: Anthropic, OpenAI, DeepSeek, then Gemini/Groq/Mistral; sampling is temperature 0).
- **Empty results offline** — Run 'scientific-consensus sync "<topic>"' first to populate the local store, or use --data-source live.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**semanticscholar**](https://github.com/danielnsilva/semanticscholar) — Python (400 stars)
- [**pyalex**](https://github.com/J535D165/pyalex) — Python (350 stars)
- [**habanero**](https://github.com/sckott/habanero) — Python (250 stars)
- [**metapub**](https://github.com/metapub/metapub) — Python (200 stars)
- [**europepmc**](https://github.com/ropensci/europepmc) — R (150 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
