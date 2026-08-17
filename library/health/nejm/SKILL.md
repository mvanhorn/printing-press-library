---
name: pp-nejm
description: "Browse, search, and triage New England Journal of Medicine articles from the terminal Trigger phrases: `what's in the current NEJM issue`, `search NEJM for`, `get NEJM article`, `recent NEJM articles`, `NEJM articles on`, `use nejm`, `run nejm`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nejm-pp-cli
    install:
      - kind: go
        bins: [nejm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/nejm/cmd/nejm-pp-cli
---

# New England Journal of Medicine — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nejm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nejm --cli-only
   ```
2. Verify: `nejm-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/nejm/cmd/nejm-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

NEJM has no public API. This CLI builds an offline-queryable, agent-native corpus of NEJM articles sourced from OpenAlex's CC0 scholarly index (abstracts, authors, publication types, open-access flags), plus per-article metadata by DOI and full-text search over everything you have synced. Novel local-state commands like 'since', 'digest', 'reading-list', 'trends', and 'open-access' do things the website and PubMed wrappers cannot.

## When to Use This CLI

Use this CLI when an agent or clinician needs current peer-reviewed NEJM literature metadata programmatically: the current issue, recently published articles, an article's abstract and bibliographic data by DOI, or an offline searchable corpus filtered by specialty, type, author, or free-access. It is the right choice when you want structured, composable output without driving a Cloudflare-gated browser.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to read full article text behind the NEJM paywall; it surfaces abstracts and metadata only.
- Do not use it for a broad multi-journal literature review; use a PubMed/Crossref tool for cross-journal search.
- Do not use the corpus 'search' command before syncing; it searches only locally synced data. For live archive-wide queries without a sync, use 'openalex search'.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`since`** — Show NEJM articles first seen in your local corpus within a time window.

  _Reach for this when an agent needs only what changed since the last check, not the whole issue._

  ```bash
  nejm-pp-cli since 48h --agent
  ```
- **`digest`** — Triage the current issue grouped by specialty or type with abstracts and free/paywalled flags.

  _Use to summarize the week's NEJM issue in one structured payload instead of fetching each article._

  ```bash
  nejm-pp-cli digest --agent
  ```
- **`reading-list`** — Queue NEJM articles by DOI locally and track read/unread state.

  _Use to persist a clinician's to-read queue across sessions without an account._

  ```bash
  nejm-pp-cli reading-list add 10.1056/NEJMoa2506905
  ```

### Second data source: OpenAlex (CC0)

- **`openalex search`** — Search NEJM works live in the OpenAlex scholarly index: abstracts, author lists, citation counts, and open-access flags.

  _Use when you need abstracts or citation counts without syncing and without the Cloudflare-gated NEJM transport; data is CC0-licensed._

  ```bash
  nejm-pp-cli openalex search --query "vitamin D" --from-year 2015 --to-year 2026 --per-page 10 --sort cited
  ```

  Flags: `--query` (full-text search), `--from-year`/`--to-year` (publication-year range), `--per-page` (1-50, default 20), `--sort` (`cited` or `date`).

  Set `NEJM_OPENALEX_MAILTO` to your email to enroll OpenAlex requests (live search and `sync`) in the [polite pool](https://docs.openalex.org/how-to-use-the-api/rate-limits-and-authentication) — a dedicated, faster server pool. Unset, requests use the anonymous pool.

### Corpus analytics

- **`trends`** — Show how NEJM output distributes by specialty, article type, and issue across the synced corpus.

  _Use to quantify NEJM coverage patterns no single page exposes._

  ```bash
  nejm-pp-cli trends --agent
  ```
- **`open-access`** — Surface free full-text NEJM articles in the corpus, newest first.

  _Use for unsubscribed readers who can only act on freely-readable articles._

  ```bash
  nejm-pp-cli open-access --agent --limit 20
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**article** — NEJM articles by DOI

- `nejm-pp-cli article <doi>` — Fetch an NEJM article's metadata and abstract by DOI (e.g. 10.1056/NEJMoa2506905)

**specialty** — NEJM specialty sections

- `nejm-pp-cli specialty` — List NEJM specialty sections


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nejm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Current issue, agent-native, narrowed fields

```bash
nejm-pp-cli current --agent --select articles.doi,articles.title,articles.authors
```

Return only the fields an agent needs from the current issue, as compact JSON.

### What's new in the last two days

```bash
nejm-pp-cli since 48h --agent
```

List articles first seen in the local corpus within the window.

### Triage the week by specialty

```bash
nejm-pp-cli digest --group specialty --agent
```

Group the current issue by specialty with one-line abstracts.

### Free full-text only

```bash
nejm-pp-cli open-access --limit 20 --agent
```

Surface freely-readable NEJM articles, newest first.

### Cite an article

```bash
nejm-pp-cli article cite 10.1056/NEJMoa2506905 --format bibtex
```

Emit a BibTeX entry from the local article record.

## Auth Setup

No authentication required.

Run `nejm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nejm-pp-cli article mock-value --agent --select id,name,status
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
nejm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nejm-pp-cli feedback --stdin < notes.txt
nejm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/nejm-pp-cli/feedback.jsonl`. They are never POSTed unless `NEJM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NEJM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nejm-pp-cli profile save briefing --json
nejm-pp-cli --profile briefing article mock-value
nejm-pp-cli profile list --json
nejm-pp-cli profile show briefing
nejm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nejm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/nejm/cmd/nejm-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nejm-pp-mcp -- nejm-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nejm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nejm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nejm-pp-cli <command> --help`.
