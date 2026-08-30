---
name: pp-scientific-consensus
description: "Aggregate PubMed, OpenAlex, Crossref, and Europe PMC into evidence summaries, consensus scores Trigger phrases: `what does the evidence say about`, `scientific consensus on`, `is there consensus that`, `evidence pyramid for`, `research gaps in`, `compare the evidence for`, `use scientific-consensus`, `run scientific-consensus`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - scientific-consensus-pp-cli
    install:
      - kind: go
        bins: [scientific-consensus-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/cmd/scientific-consensus-pp-cli
---

# Scientific Consensus — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `scientific-consensus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install scientific-consensus --cli-only
   ```
2. Verify: `scientific-consensus-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/cmd/scientific-consensus-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Scientific Consensus turns large collections of papers into actionable evidence. It scores consensus across sources (`consensus`), classifies studies by design and renders evidence pyramids (`evidence`), detects gaps and controversies, and persists everything to a local SQLite store you can query offline with `--json`. Fully keyless; optional AI keys upgrade summarization.

## When to Use This CLI

Use Scientific Consensus when an agent or researcher needs to know what the evidence says about a claim, not just find papers. It is the right tool for evidence synthesis, consensus scoring, study-design classification, gap/controversy detection, and topic monitoring across biomedical and general scientific literature. It excels when offline persistence and agent-native JSON matter.

## Anti-triggers

Do not use this CLI for:
- Do not use for retrieving the full text PDF of a specific paper (use the publisher or Europe PMC full-text directly).
- Do not use for non-scholarly web search or news.
- Do not use as a citation manager replacement for writing (use Zotero); it exports BibTeX but does not manage libraries.
- Do not treat heuristic consensus/quality scores as peer-reviewed conclusions.

## Unique Capabilities

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

  _Reach for this whenever an agent has one identifier type but needs the other (e.g. a citation tool wants a PMID, a DOI resolver gives you a DOI)._

  ```bash
  scientific-consensus convert --doi 10.1136/bmj.i6583 --agent
  scientific-consensus convert --pmid 32939066 --agent
  ```
- **`batch`** — Run consensus analysis for multiple claims from one or more files (plain text, one claim per line, blank lines and `#` comments skipped). Accepts globs; duplicates are deduplicated. Returns a summary table or a flat JSON array, one item per claim.

  _Reach for this when an agent must score many claims at once without shell-looping over `consensus`._

  ```bash
  scientific-consensus batch claims.txt --agent
  scientific-consensus batch claims*.txt --limit 20 --json
  ```
- **`report`** — Export an analyzed works report for a topic as an Excel (`.xlsx`) workbook: a `Works` sheet (one row per study with title, first author, year, DOI, PMID, venue, design, stance, stance confidence, citations, open access) and a `Summary` sheet (query metadata plus stance/design aggregates). Uses the same design/stance engine as `consensus` and `evidence`. Unlike `export` (raw JSONL/JSON API dumps), `report` writes analyzed, spreadsheet-ready results.

  _Reach for this when a researcher wants to hand off results to Excel, Google Sheets, or any spreadsheet-based screening workflow._

  ```bash
  scientific-consensus report "vitamin D respiratory infections" --output report.xlsx
  scientific-consensus report "microplastics" -o mp.xlsx --claim "microplastics harm human health" --limit 100 --agent
  ```
- **`citations`** — Build a citation network around a seed work (by `--doi`, `--pmid`, or `--id`): the works citing it (`cited-by`), the works it references (`references`), or both. Bounded by `--depth` (max 2 hops) and `--max-nodes` (hard cap). `--json` returns flat `nodes` + `edges` arrays ready for a web graph renderer; the human default is a compact summary.

  _Reach for this to trace influence, find high-impact neighbors, or feed a network-visualization tool._

  ```bash
  scientific-consensus citations --doi 10.1136/bmj.i6583 --agent
  scientific-consensus citations --id W2741809807 --depth 2 --max-nodes 80 --direction cited-by --agent
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

## Command Reference

**authors** — Search and retrieve authors

- `scientific-consensus-pp-cli authors get` — Get a single author by OpenAlex ID
- `scientific-consensus-pp-cli authors search` — Search authors

**funders** — Research funders

- `scientific-consensus-pp-cli funders` — Search funders

**institutions** — Search and retrieve institutions

- `scientific-consensus-pp-cli institutions get` — Get a single institution by OpenAlex ID
- `scientific-consensus-pp-cli institutions search` — Search institutions

**sources** — Journal (source) metadata

- `scientific-consensus-pp-cli sources get` — Get a journal (source) by ISSN or OpenAlex ID
- `scientific-consensus-pp-cli sources search` — Search sources (journals)

**topics** — Research topics

- `scientific-consensus-pp-cli topics` — Search topics

**works** — Search and retrieve scholarly works

- `scientific-consensus-pp-cli works get` — Get a single work by OpenAlex ID, DOI, or PMID
- `scientific-consensus-pp-cli works search` — Search scholarly works

**convert** — DOI ↔ PMID identifier translation

- `scientific-consensus-pp-cli convert --doi <doi>` — Look up the PMID (and title) for a DOI
- `scientific-consensus-pp-cli convert --pmid <pmid>` — Look up the DOI (and title) for a PMID

**batch** — Batch consensus over claim files

- `scientific-consensus-pp-cli batch <file|glob> [...]` — Run consensus analysis for every claim in one or more files (blank lines and `# comments` skipped; globs and duplicate files are handled automatically)

**report** — Excel report export

- `scientific-consensus-pp-cli report <query> --output <file.xlsx>` — Export analyzed works (design + stance classified) as a two-sheet Excel workbook; `--claim` overrides the stance target, `--filter` narrows with an OpenAlex filter, `--limit` caps the works analyzed

**citations** — Citation-network graph

- `scientific-consensus-pp-cli citations --doi <doi>` — Build a citation graph from a DOI seed
- `scientific-consensus-pp-cli citations --pmid <pmid>` — Build a citation graph from a PMID seed
- `scientific-consensus-pp-cli citations --id <W...>` — Build a citation graph from an OpenAlex ID seed

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
scientific-consensus-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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
scientific-consensus convert --doi 10.1136/bmj.i6583 --agent
```

Returns the PMID, DOI, and title from the OpenAlex work index. Use `--pmid` for the reverse direction.

### Run consensus on a batch of claims

```bash
scientific-consensus batch claims.txt --limit 20 --agent
```

One claim per line; blank lines and `#` comments are skipped; globs and duplicate files are handled. Returns a verdict, consensus score, and evidence strength for each claim as a flat JSON array under `--json`.

### Explore a paper's citation network

```bash
scientific-consensus citations --doi 10.1136/bmj.i6583 --depth 1 --max-nodes 50 --agent
```

Returns `nodes` (id, title, year, cited\_by\_count) and `edges` (from → to) bounded by `--max-nodes`. Use `--direction cited-by` for papers citing the seed, `references` for papers it cites, or `both` (default). `--depth 2` expands one additional hop.

## Auth Setup

No API key required for any command. Optional env vars raise limits or enable AI summarization: NCBI_API_KEY (PubMed, higher rate limit), SEMANTIC_SCHOLAR_API_KEY (Semantic Scholar enrichment), and ANTHROPIC_API_KEY / OPENAI_API_KEY / DEEPSEEK_API_KEY / GEMINI_API_KEY (enhanced summarization; first configured key wins — DeepSeek sits after Anthropic/OpenAI and before Gemini/Groq/Mistral; OpenAI-compatible providers sample at temperature 0). Everything works without them.

Run `scientific-consensus-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  scientific-consensus-pp-cli authors get mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

Long-running analysis commands (`consensus`, `evidence`, `quality`, `reproducibility`, `watch`, `gaps`, `controversies`, `compare`, `batch`, `citations`) print a self-rewriting progress line to stderr while processing works. This is suppressed automatically under `--json`, `--agent`, `--compact`, `--csv`, `--quiet`, `--plain`, `--select`, and any non-TTY stderr, so it never appears in piped or agent contexts.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
scientific-consensus-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
scientific-consensus-pp-cli feedback --stdin < notes.txt
scientific-consensus-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/scientific-consensus-pp-cli/feedback.jsonl`. They are never POSTed unless `SCIENTIFIC_CONSENSUS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCIENTIFIC_CONSENSUS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
scientific-consensus-pp-cli profile save briefing --json
scientific-consensus-pp-cli --profile briefing authors get mock-value
scientific-consensus-pp-cli profile list --json
scientific-consensus-pp-cli profile show briefing
scientific-consensus-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `scientific-consensus-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/cmd/scientific-consensus-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add scientific-consensus-pp-mcp -- scientific-consensus-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which scientific-consensus-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   scientific-consensus-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `scientific-consensus-pp-cli <command> --help`.
