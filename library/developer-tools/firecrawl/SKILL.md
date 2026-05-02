---
name: pp-firecrawl
description: "Printing Press CLI for Firecrawl. API for interacting with Firecrawl services to perform web scraping and crawling tasks."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["firecrawl-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/firecrawl-pp-cli/cmd/firecrawl-pp-cli@latest","bins":["firecrawl-pp-cli"],"label":"Install via go install"}]}}'
---

# Firecrawl — Printing Press CLI

API for interacting with Firecrawl services to perform web scraping and crawling tasks.

## Command Reference

**batch** — Manage batch

- `firecrawl-pp-cli batch cancel-scrape` — Cancel a batch scrape job
- `firecrawl-pp-cli batch get-scrape-errors` — Get the errors of a batch scrape job
- `firecrawl-pp-cli batch get-scrape-status` — Get the status of a batch scrape job
- `firecrawl-pp-cli batch scrape-and-extract-from-urls` — Scrape multiple URLs and optionally extract information using an LLM

**crawl** — Manage crawl

- `firecrawl-pp-cli crawl cancel` — Cancel a crawl job
- `firecrawl-pp-cli crawl get-active` — Get all active crawls for the authenticated team
- `firecrawl-pp-cli crawl get-status` — Get the status of a crawl job
- `firecrawl-pp-cli crawl urls` — Crawl multiple URLs based on options

**deep-research** — Manage deep research

- `firecrawl-pp-cli deep-research get-status` — Get the status and results of a deep research operation
- `firecrawl-pp-cli deep-research start` — Start a deep research operation on a query

**extract** — Manage extract

- `firecrawl-pp-cli extract data` — Extract structured data from pages using LLMs
- `firecrawl-pp-cli extract get-status` — Get the status of an extract job

**firecrawl-search** — Manage firecrawl search

- `firecrawl-pp-cli firecrawl-search` — Search and optionally scrape search results

**llmstxt** — Manage llmstxt

- `firecrawl-pp-cli llmstxt generate-llms-txt` — Generate LLMs.txt for a website
- `firecrawl-pp-cli llmstxt get-llms-txt-status` — Get the status and results of an LLMs.txt generation job

**map** — Manage map

- `firecrawl-pp-cli map` — Map multiple URLs based on options

**scrape** — Manage scrape

- `firecrawl-pp-cli scrape` — Scrape a single URL and optionally extract information using an LLM

**team** — Manage team

- `firecrawl-pp-cli team get-credit-usage` — Get remaining credits for the authenticated team
- `firecrawl-pp-cli team get-token-usage` — Get remaining tokens for the authenticated team (Extract only)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
firecrawl-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
firecrawl-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `FIRECRAWL_BEARER_AUTH` as an environment variable.

Run `firecrawl-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  firecrawl-pp-cli batch cancel-scrape mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
firecrawl-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
firecrawl-pp-cli feedback --stdin < notes.txt
firecrawl-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.firecrawl-pp-cli/feedback.jsonl`. They are never POSTed unless `FIRECRAWL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FIRECRAWL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
firecrawl-pp-cli profile save briefing --json
firecrawl-pp-cli --profile briefing batch cancel-scrape mock-value
firecrawl-pp-cli profile list --json
firecrawl-pp-cli profile show briefing
firecrawl-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `firecrawl-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/firecrawl-pp-cli/cmd/firecrawl-pp-cli@latest
   ```
3. Verify: `firecrawl-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/firecrawl-pp-cli/cmd/firecrawl-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add firecrawl-pp-mcp -- firecrawl-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which firecrawl-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   firecrawl-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `firecrawl-pp-cli <command> --help`.
