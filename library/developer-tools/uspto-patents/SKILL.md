---
name: pp-uspto-patents
description: "Every patent prosecution feature from every existing tool, plus offline search, family trees, and portfolio... Trigger phrases: `search patents`, `look up patent`, `patent prosecution history`, `check PTAB status`, `patent portfolio analysis`, `use uspto`."
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - uspto-patents-pp-cli
---

# USPTO Patents — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `uspto-patents-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install uspto-patents --cli-only
   ```
2. Verify: `uspto-patents-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search and look up US patent applications, track prosecution history, traverse family trees, and monitor competitor portfolios — all with a local SQLite cache for offline search and cross-entity queries. Works with the USPTO Open Data Portal API.

## When to Use This CLI

Use the USPTO Patents CLI when you need to search US patent applications, look up prosecution history, check PTAB trial status, or analyze patent portfolios. Best for IP due diligence, prior-art searches, competitor monitoring, and patent landscape analysis. Ideal for agents that need structured patent data without managing API pagination or rate limits.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Prosecution intelligence
- **`patent timeline`** — See every prosecution event in chronological order — office actions, amendments, continuity filings, and PTAB proceedings merged into one stream

  _When evaluating a patent's prosecution strength, agents need the full event history in one call instead of querying three separate endpoints_

  ```bash
  uspto-patents-pp-cli patent timeline 14412875 --json
  ```
- **`patent family`** — Walk the full patent family tree — continuations, divisionals, and CIPs — rendered as an indented tree or flat table

  _Patent families often span dozens of related applications; agents evaluating IP portfolios need the full tree, not just direct parents and children_

  ```bash
  uspto-patents-pp-cli patent family 14412875 --json
  ```
- **`patent batch-history`** — Fetch prosecution history for a list of patents in one command — transactions, continuity, and assignments with automatic rate-limit throttling

  _Agents evaluating prior art need prosecution context for multiple patents without managing rate limits or individual API calls_

  ```bash
  uspto-patents-pp-cli patent batch-history 14412875 15123456 16789012 --json
  ```
- **`patent one-look`** — Get the full current state of any patent in one command — status, owner, family depth, PTA days, PTAB exposure, and attorney of record

  _Agents evaluating a patent need the complete current picture without making half a dozen sequential API calls_

  ```bash
  uspto-patents-pp-cli patent one-look 14412875 --json
  ```

### Portfolio analytics
- **`portfolio snapshot`** — Get aggregate stats for any assignee's patent portfolio — filing counts, grant rates, art-unit distribution, and status breakdown

  _Agents doing competitive intelligence need portfolio-level metrics, not individual application records_

  ```bash
  uspto-patents-pp-cli portfolio snapshot "Apple Inc" --json
  ```
- **`portfolio diff`** — See what changed in an assignee's patent portfolio since your last check — new filings, status changes, and ownership transfers

  _Agents monitoring competitor IP activity need change detection, not full portfolio dumps they have to diff themselves_

  ```bash
  uspto-patents-pp-cli portfolio diff "Apple Inc" --since 2026-04-01 --json
  ```

### Local state that compounds
- **`related`** — Find patents in your local cache that share the same art unit, assignee, or inventor as a given patent

  _Agents doing prior-art search need to discover related patents by multiple dimensions, not just keyword search_

  ```bash
  uspto-patents-pp-cli related 14412875 --json
  ```

## Command Reference

**datasets** — Manage datasets

- `uspto-patents-pp-cli datasets get` — Bulk data- find a product by its identifier (shortName)
- `uspto-patents-pp-cli datasets get-products` — Returns a 302 redirect to the actual download location for the given productIdentifier and fileName.
- `uspto-patents-pp-cli datasets list` — Query parameters are optional. When no query parameters supplied, top 25 applications are returned

**patent** — Manage patent

- `uspto-patents-pp-cli patent create` — Search patent application status codes and status code description
- `uspto-patents-pp-cli patent create-appeals` — Search appeals decisions using json payload
- `uspto-patents-pp-cli patent create-appeals-2` — Download appeals decisions search results in json or csv format using json payload
- `uspto-patents-pp-cli patent create-applications` — Search patent applications by supplying json payload
- `uspto-patents-pp-cli patent create-applications-2` — Download patent data by supplying json payload
- `uspto-patents-pp-cli patent create-interferences` — Request body matches PatentSearchRequest; example tailored for interferences.
- `uspto-patents-pp-cli patent create-interferences-2` — Download interferences decisions search results in json or csv format using json payload
- `uspto-patents-pp-cli patent create-trials` — Search trials decisions documents using json payload
- `uspto-patents-pp-cli patent create-trials-2` — Search trials documents using json payload
- `uspto-patents-pp-cli patent create-trials-3` — Search trials proceedings using json payload
- `uspto-patents-pp-cli patent create-trials-4` — Download trials decisions documents search results in json or csv format using json payload
- `uspto-patents-pp-cli patent create-trials-5` — Download trials documents search results in json or csv format using json payload
- `uspto-patents-pp-cli patent create-trials-6` — Download trials proceedings search results in json or csv format using json payload
- `uspto-patents-pp-cli patent get` — Patent application data for a provided application number
- `uspto-patents-pp-cli patent get-appeals` — Retrieve appeals decisions by document Identifier
- `uspto-patents-pp-cli patent get-appeals-2` — Retrieve appeals decisions by appeal number
- `uspto-patents-pp-cli patent get-applications` — Get patent term adjustment data for an application number
- `uspto-patents-pp-cli patent get-applications-2` — Get patent assignment data for an application number
- `uspto-patents-pp-cli patent get-applications-3` — Associated (pgpub, grant) documents meta-data for an application
- `uspto-patents-pp-cli patent get-applications-4` — Get attorney/agent data for an application number
- `uspto-patents-pp-cli patent get-applications-5` — Get continuity data for an application number
- `uspto-patents-pp-cli patent get-applications-6` — Documents details for an application number
- `uspto-patents-pp-cli patent get-applications-7` — Get foreign-priority data for an application number
- `uspto-patents-pp-cli patent get-applications-8` — Get patent application meta data
- `uspto-patents-pp-cli patent get-applications-9` — Get transaction data for an application number
- `uspto-patents-pp-cli patent get-interferences` — Returns a single interference decision document for the provided `documentIdentifier`.
- `uspto-patents-pp-cli patent get-interferences-2` — Returns one or more interference decision records for the provided `interferenceNumber`.
- `uspto-patents-pp-cli patent get-trials` — Retrieve a single trials decisions document by document identifier
- `uspto-patents-pp-cli patent get-trials-2` — Retrieve a single trials document by document identifier
- `uspto-patents-pp-cli patent get-trials-3` — Retrieve a single trials proceeding by trial number
- `uspto-patents-pp-cli patent get-trials-4` — Retrieve all trials decisions documents by trial number
- `uspto-patents-pp-cli patent get-trials-5` — Retrieve all trials documents by trial number
- `uspto-patents-pp-cli patent list` — Search patent application status codes and status code description
- `uspto-patents-pp-cli patent list-appeals` — Search appeals decisions using query parameters
- `uspto-patents-pp-cli patent list-appeals-2` — Download appeals decisions search results in json or csv format using query parameters
- `uspto-patents-pp-cli patent list-applications` — Query parameters are optional. When no query parameters supplied, top 25 applications are returned
- `uspto-patents-pp-cli patent list-applications-2` — Query parameters are optional. When no query parameters supplied, top 25 applications are returned
- `uspto-patents-pp-cli patent list-interferences` — Query interference decisions (decision type/category, party names, date ranges, etc.).
- `uspto-patents-pp-cli patent list-interferences-2` — Download interferences decisions search results in json or csv format using query parameters
- `uspto-patents-pp-cli patent list-trials` — Search trials decisions documents using query parameters
- `uspto-patents-pp-cli patent list-trials-2` — Search trials documents using query parameters
- `uspto-patents-pp-cli patent list-trials-3` — Search trials proceedings using query parameters
- `uspto-patents-pp-cli patent list-trials-4` — Download trials decisions documents search results in json or csv format using query parameters
- `uspto-patents-pp-cli patent list-trials-5` — Download trials document search results in json or csv format using query parameters
- `uspto-patents-pp-cli patent list-trials-6` — Download trials proceedings search results in json or csv format using query parameters

**petition** — Manage petition

- `uspto-patents-pp-cli petition create` — Search petition decision applications by supplying json payload
- `uspto-patents-pp-cli petition create-decisions` — Download petition decision data by supplying json payload
- `uspto-patents-pp-cli petition get` — Petition decision application data for a provided application number
- `uspto-patents-pp-cli petition list` — Query parameters are optional. When no query parameters supplied, top 25 petition decisions are returned
- `uspto-patents-pp-cli petition list-decisions` — Query parameters are optional. When no query parameters supplied, top 25 petition decisions are returned


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
uspto-patents-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Prior art search with prosecution context

```bash
uspto-patents-pp-cli patent list-applications --q 'machine learning neural network' --json --select applicationNumberText,applicationMetaData.firstInventorName,applicationMetaData.filingDate,applicationMetaData.applicationStatusDescriptionText
```

Search for ML patents and extract key fields for analysis

### Full prosecution timeline

```bash
uspto-patents-pp-cli patent timeline 14412875 --json --select date,eventType,description
```

See every prosecution event for a patent in chronological order

### Competitor portfolio monitoring

```bash
uspto-patents-pp-cli portfolio diff "Google LLC" --since 2026-04-01 --json --agent
```

Check what changed in Google's patent portfolio since last month

### Patent family exploration

```bash
uspto-patents-pp-cli patent family 14412875 --json --agent
```

Walk the full continuity tree to find all related applications

## Auth Setup

Get a free API key from data.uspto.gov (requires ID.me identity verification). Set it with `export USPTO_API_KEY=your-key`. Run `doctor` to verify connectivity.

Run `uspto-patents-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  uspto-patents-pp-cli datasets list --agent --select id,name,status
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
uspto-patents-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
uspto-patents-pp-cli feedback --stdin < notes.txt
uspto-patents-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.uspto-patents-pp-cli/feedback.jsonl`. They are never POSTed unless `USPTO_PATENTS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `USPTO_PATENTS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
uspto-patents-pp-cli profile save briefing --json
uspto-patents-pp-cli --profile briefing datasets list
uspto-patents-pp-cli profile list --json
uspto-patents-pp-cli profile show briefing
uspto-patents-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `uspto-patents-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add uspto-patents-pp-mcp -- uspto-patents-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which uspto-patents-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   uspto-patents-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `uspto-patents-pp-cli <command> --help`.
