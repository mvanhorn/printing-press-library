---
name: pp-lda-gov
description: "Lobbying disclosure records with local sync, entity resolution, and evidence-grade exports. Trigger phrases: `search lobbying filings`, `resolve an LDA client`, `audit lobbying spend`, `check LD-203 contributions`, `export lobbying graph`, `use LDA.gov`, `run lda-gov`."
author: "Mherzog4"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - lda-gov-pp-cli
    install:
      - kind: go
        bins: [lda-gov-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/lda-gov/cmd/lda-gov-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/lda-gov/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# LDA.gov — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `lda-gov-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install lda-gov --cli-only
   ```
2. Verify: `lda-gov-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/lda-gov/cmd/lda-gov-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search the official Lobbying Disclosure Act API, sync public filings into SQLite, and turn nested lobbying records into source-linked audits. The CLI keeps anonymous reads safe by default, supports optional registered API keys, and adds commands for entity resolution, anomaly checks, spend timelines, contribution totals, and graph exports.

## When to Use This CLI

Use this CLI when you need official LDA.gov records, local analysis, repeatable watchdog checks, or agent-friendly evidence exports. It is strongest for lobbying filings, contribution reports, registrant/client/lobbyist lookup, source citation, and longitudinal analysis across synced data.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for FEC campaign-finance data; use a dedicated FEC source.
- Do not use this CLI for legal advice about LDA filing obligations.
- Do not use this CLI for live bill status or congressional vote data; use Congress.gov or another legislative API.
- Do not use this CLI to infer missed filing obligations without corroborating records.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Evidence-grade local audits
- **`audit filings`** — Flag amendments, terminations, duplicate-risk filings, client/registrant conflicts, and missing source URLs.

  _Use this when an agent needs reproducible watchdog flags instead of raw filing rows._

  ```bash
  lda-gov-pp-cli audit filings --agent --limit 25
  ```
- **`audit spend`** — Aggregate disclosed lobbying income and expenses by client, registrant, issue, year, and quarter.

  _Use this when an agent needs analysis-ready spend totals rather than individual filings._

  ```bash
  lda-gov-pp-cli audit spend --client Boeing --from-year 2020 --to-year 2025 --csv
  ```
- **`contributions totals`** — Aggregate LD-203 contribution items by contributor, recipient, payee, item type, year, and report.

  _Use this when an agent needs contribution totals without writing a custom flattener._

  ```bash
  lda-gov-pp-cli contributions totals --year 2024 --csv
  ```

### Entity intelligence
- **`entities resolve`** — Rank matching registrants, clients, and lobbyists with official IDs, counts, last activity, and source URLs.

  _Use this before citing or joining an ambiguous company, firm, or lobbyist name._

  ```bash
  lda-gov-pp-cli entities resolve Boeing --agent --limit 10
  ```
- **`graph export`** — Export client-registrant-lobbyist-issue-government entity edges for graph analysis.

  _Use this when an agent needs a relationship graph for downstream NetworkX, Gephi, or SQL work._

  ```bash
  lda-gov-pp-cli graph export --client Boeing --format csv
  ```
- **`lobbyists covered-positions`** — List lobbyists' covered government positions connected to clients, registrants, and filing periods.

  _Use this when an agent needs exact official covered-position evidence tied to active lobbying records._

  ```bash
  lda-gov-pp-cli lobbyists covered-positions --client Boeing --csv
  ```

### Period monitoring
- **`reports quarter`** — Produce quarter stats for filings, amendments, terminations, top issues, top entities, spend, and LD-203 totals.

  _Use this after quarterly deadlines when an agent needs a compact, source-linked snapshot._

  ```bash
  lda-gov-pp-cli reports quarter --year 2024 --period year_end --agent --select top_issue,top_government_entity,filings
  ```

## Command Reference

**clients** — Access Client information.

- `lda-gov-pp-cli clients list` — Returns all clients matching the provided filters.
- `lda-gov-pp-cli clients retrieve` — Returns all clients matching the provided filters.

**constants** — An assorted list of constants found in the LDA REST API.

- `lda-gov-pp-cli constants list-contribution-item-types` — Returns all ContributionItemTypes.
- `lda-gov-pp-cli constants list-countries` — Returns all Countries.
- `lda-gov-pp-cli constants list-filing-types` — Returns all FilingTypes.
- `lda-gov-pp-cli constants list-government-entities` — Returns all GovernmentEntities.
- `lda-gov-pp-cli constants list-lobbying-activity-general-issues` — Returns all LobbyingActivityGeneralIssues.
- `lda-gov-pp-cli constants list-lobbyist-prefixes` — Returns all LobbyistPrefixes.
- `lda-gov-pp-cli constants list-lobbyist-suffixes` — Returns all LobbyistSuffixes.
- `lda-gov-pp-cli constants list-states` — Returns all States.

**contributions** — Manage contributions

- `lda-gov-pp-cli contributions list-reports` — List reports
- `lda-gov-pp-cli contributions retrieve-report` — Returns all contributions matching the provided filters.

**filings** — Access LD1 / LD2 filings.

- `lda-gov-pp-cli filings list` — List
- `lda-gov-pp-cli filings retrieve` — Returns all filings matching the provided filters.

**lobbyists** — Access Lobbyist information.

- `lda-gov-pp-cli lobbyists list` — Returns all lobbyists matching the provided filters.
- `lda-gov-pp-cli lobbyists retrieve` — Returns all lobbyists matching the provided filters.

**registrants** — Access Registrant information.

- `lda-gov-pp-cli registrants list` — Returns all registrants matching the provided filters.
- `lda-gov-pp-cli registrants retrieve` — Returns all registrants matching the provided filters.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
lda-gov-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Resolve a company before joining records

```bash
lda-gov-pp-cli entities resolve Boeing --agent --limit 10 --db ./lda.db
```

Ranks official registrant, client, and lobbyist matches so an agent can pick the right ID.

### Build a source-linked quarter snapshot

```bash
lda-gov-pp-cli reports quarter --year 2024 --period year_end --agent --select top_issue,top_government_entity,filings --db ./lda.db
```

Narrows a potentially large report to the fields an agent needs in context.

### Audit filings for cleanup flags

```bash
lda-gov-pp-cli audit filings --agent --limit 25 --db ./lda.db
```

Flags amendments, terminations, duplicate risks, and client/registrant conflicts.

### Export a lobbying relationship graph

```bash
lda-gov-pp-cli graph export --client Boeing --format csv --db ./lda.db
```

Creates edge rows connecting clients, registrants, lobbyists, issues, and government entities.

### Summarize LD-203 contribution counterparties

```bash
lda-gov-pp-cli contributions totals --year 2024 --csv --db ./lda.db
```

Flattens contribution items and aggregates them by counterparty and item type.

## Auth Setup

LDA.gov works anonymously at a lower rate limit. For higher throughput, register for an API key at https://lda.gov/api/register/ and set LDA_API_KEY to the key value; the CLI sends it as Authorization: Token <key>. Legacy scripts may use SENATE_LDA_API_KEY or USSLDA_KEY, but new usage should prefer LDA_API_KEY.

Run `lda-gov-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  lda-gov-pp-cli clients list --agent --select id,name,status
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
lda-gov-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
lda-gov-pp-cli feedback --stdin < notes.txt
lda-gov-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/lda-gov-pp-cli/feedback.jsonl`. They are never POSTed unless `LDA_GOV_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LDA_GOV_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
lda-gov-pp-cli profile save briefing --json
lda-gov-pp-cli --profile briefing clients list
lda-gov-pp-cli profile list --json
lda-gov-pp-cli profile show briefing
lda-gov-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `lda-gov-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/lda-gov/cmd/lda-gov-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add lda-gov-pp-mcp -- lda-gov-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which lda-gov-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   lda-gov-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `lda-gov-pp-cli <command> --help`.
