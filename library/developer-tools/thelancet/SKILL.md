---
name: pp-thelancet
description: "Search Lancet articles offline and analyze author impact, institutional networks Trigger phrases: `search Lancet articles for`, `rank the top authors in Lancet`, `which institutions publish most in`, `how has this journal's focus changed`, `build a reading list on`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - thelancet-pp-cli
    install:
      - kind: go
        bins: [thelancet-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/thelancet/cmd/thelancet-pp-cli
---

# The Lancet — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `thelancet-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install thelancet --cli-only
   ```
2. Verify: `thelancet-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/thelancet/cmd/thelancet-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Lancet CLI gives researchers and institutions offline access to The Lancet family of journals via OpenAlex. Beyond search and fetch, it ranks researchers by citation impact within a specialty (rank-authors), maps institutional co-authorship networks (mesh), tracks which institutions are publishing more over time (affiliation-growth), and detects how a journal's editorial focus shifts (drift) — insights impossible from single API queries or paywalled tools.

## When to Use This CLI

Use this CLI to answer questions about The Lancet's research landscape offline and without API throttling: literature reviews, tracking institutional publication trends, finding collaborators, and analyzing how medical research evolves over time.

## Anti-triggers

Do not use this CLI for:
- Do NOT use this CLI for real-time article feeds or breaking news; it syncs periodically from OpenAlex, not continuously
- Do NOT use this for journal submission or peer-review status; it mirrors published works, not the editorial pipeline
- Do NOT use this for full-text access to paywalled articles; it returns metadata, topics, and links only

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local data compounds
- **`rank-authors`** — Rank researchers at your institution by citation impact within a specific Lancet journal or topic.

  _Grant managers and department heads use this to identify who is leading at your institution in a specific field; agents can use it to find high-impact collaborators._

  ```bash
  thelancet-pp-cli rank-authors --institution 'Harvard' --journal lancet-oncology --json
  ```
- **`mesh`** — Find researchers at your institution who co-author Lancet papers and quantify how their work connects.

  _Librarians and research directors use this to uncover collaboration opportunities and map how an institution's researchers connect through Lancet._

  ```bash
  thelancet-pp-cli mesh --org 'Stanford University' --json
  ```
- **`affiliation-growth`** — Track institutions gaining publication velocity in Lancet journals; identify rising research centers.

  _Strategy teams track this to understand competitive research landscapes and identify emerging partner institutions._

  ```bash
  thelancet-pp-cli affiliation-growth --journal lancet-neurology --years 5 --json
  ```
- **`visibility-gap`** — Find authors whose citation impact is out of step with the prestige of the Lancet journals they publish in.

  _Grant managers and research directors use this to identify under-recognized talent for promotion or collaboration._

  ```bash
  thelancet-pp-cli visibility-gap --institution 'Oxford' --json
  ```

### Offline analysis
- **`drift`** — Compare how a journal's topic distribution shifts between two time windows; spot emerging and fading specialties.

  _Research strategists use this to align their work with journal editorial priorities and detect when journal scope shifts._

  ```bash
  thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024 --json
  ```
- **`curate`** — Auto-generate ranked reading lists for a topic, sorted by date/citations/relevance, exportable as Markdown or BibTeX.

  _Librarians and researchers use this to quickly assemble authoritative reading lists without manual screening._

  ```bash
  thelancet-pp-cli curate --topic 'gene therapy' --sort citations --output bibtex
  ```

## Command Reference

**authors** — Search and retrieve authors

- `thelancet-pp-cli authors get` — Get a single author by OpenAlex ID
- `thelancet-pp-cli authors search` — Search authors

**sources** — Journal (source) metadata

- `thelancet-pp-cli sources <id>` — Get a journal (source) by ISSN or OpenAlex ID

**works** — Search and retrieve Lancet articles

- `thelancet-pp-cli works get` — Get a single article by OpenAlex ID or DOI
- `thelancet-pp-cli works search` — Search Lancet articles (works)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
thelancet-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Build a topic reading list for a literature review

```bash
thelancet-pp-cli curate --topic 'machine learning diagnosis' --sort citations --output bibtex > review.bib
```

Generate a citation-ranked, BibTeX-exportable reading list on a topic across Lancet journals in one command.

### Narrow a deeply nested works response for an agent

```bash
thelancet-pp-cli works search --search 'immunotherapy' --per-page 10 --agent --select results.title,results.cited_by_count,results.authorships.author.display_name
```

OpenAlex works are large and deeply nested; pair --agent with dotted --select paths to return only the fields you need and save context.

### Spot editorial drift in a specialty journal

```bash
thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024 --top-n 15
```

Compare topic distributions across two multi-year windows to see which specialties the journal now publishes more or less of.

## Local JSON API (`serve`)

`serve` starts a local HTTP server that exposes the two analytics engines as read-only JSON endpoints — the bridge between this CLI's local mirror and the Lancet web portal (or any local tool that prefers HTTP over shelling out):

```bash
thelancet-pp-cli serve
thelancet-pp-cli serve --listen 127.0.0.1:9090
```

- **`GET /affiliations`** — mirrors `affiliation-growth`. Query params: `journal` (slug, default all), `years` (default 5), `threshold` (default 2), `limit` (default 25). Returns the same JSON array (`institution`, `recent_count`, `prior_count`, `growth`).

  ```bash
  curl 'http://127.0.0.1:8080/affiliations?journal=lancet-neurology&years=5'
  ```
- **`GET /authors`** — mirrors `rank-authors`. Query params: `journal`, `institution` (substring match), `limit` (default 25). Returns the same JSON array (`author_id`, `author_name`, `works`, `total_citations`, `avg_citations`).

  ```bash
  curl 'http://127.0.0.1:8080/authors?institution=Oxford&limit=10'
  ```

Invalid parameters return `400` with `{"error": "..."}`; engine failures return `500` with a generic structured error. Use `--db` to point at a non-default database path. Stop with Ctrl+C (graceful shutdown).

**Security note:** the server binds to loopback (`127.0.0.1:8080`) by default and is strictly read-only — it never syncs, writes, or mutates data. Do not point `--listen` at a public interface; there is no authentication.

## Auth Setup

No authentication required.

Run `thelancet-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  thelancet-pp-cli authors get mock-value --agent --select id,name,status
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
thelancet-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
thelancet-pp-cli feedback --stdin < notes.txt
thelancet-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/thelancet-pp-cli/feedback.jsonl`. They are never POSTed unless `THELANCET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `THELANCET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
thelancet-pp-cli profile save briefing --json
thelancet-pp-cli --profile briefing authors get mock-value
thelancet-pp-cli profile list --json
thelancet-pp-cli profile show briefing
thelancet-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `thelancet-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/thelancet/cmd/thelancet-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add thelancet-pp-mcp -- thelancet-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which thelancet-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   thelancet-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `thelancet-pp-cli <command> --help`.
