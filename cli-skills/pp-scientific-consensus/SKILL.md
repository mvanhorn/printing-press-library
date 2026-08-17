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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/scientific-consensus/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Scientific Consensus — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `scientific-consensus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install scientific-consensus --cli-only
   ```
2. Verify: `scientific-consensus-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

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

## Auth Setup

No API key required for any command. Optional env vars raise limits or enable AI summarization: NCBI_API_KEY (PubMed, higher rate limit), SEMANTIC_SCHOLAR_API_KEY (Semantic Scholar enrichment), and OPENAI_API_KEY / ANTHROPIC_API_KEY / GEMINI_API_KEY (enhanced summarization). Everything works without them.

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
