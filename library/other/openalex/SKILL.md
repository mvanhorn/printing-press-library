---
name: pp-openalex
description: "Search OpenAlex's catalog of 250M+ scholarly works, authors, sources, institutions, topics, keywords, publishers, and funders. Trigger phrases: `find scholarly papers on`, `look up research by <author>`, `citations for <work>`, `use openalex`."
author: "Hiten Shah"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openalex-pp-cli
---

# Openalex — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openalex-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install openalex --cli-only
   ```
2. Verify: `openalex-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/openalex/cmd/openalex-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Research graph retrieval

- **`works list`** — Search scholarly works with compact JSON, field selection, filters, sorting, and live API fallback.
- **`authors list`** — Search authors and retrieve canonical OpenAlex author records for citation and collaborator research.
- **`institutions list`** — Search institutions with country/type metadata for affiliation and ecosystem analysis.
- **`sources list`** — Search journals, repositories, and venues with compact agent-oriented output.
- **`topics list`** — Explore OpenAlex topics for research-area discovery and downstream filtering.

## Command Reference

**authors** — People who create scholarly works

- `openalex-pp-cli authors get` — Retrieve a single author by OpenAlex ID or ORCID.
- `openalex-pp-cli authors list` — Get a list of authors with optional filtering, searching, sorting, and pagination.

**autocomplete** — Fast typeahead search for any entity type

- `openalex-pp-cli autocomplete` — Fast typeahead search returning up to 10 results. Use for search-as-you-type interfaces.

**awards** — Research grants and funding awards

- `openalex-pp-cli awards get` — Retrieve a single award by its OpenAlex ID.
- `openalex-pp-cli awards list` — Get a list of research grants and funding awards.

**changefiles** — Manage changefiles

- `openalex-pp-cli changefiles get` — Get details for a specific date's changefile, including which entity types changed, how many records, and download...
- `openalex-pp-cli changefiles list` — List all available changefile dates. Each date has downloadable files containing every entity record that was...

**concepts** — Legacy taxonomy of research areas (deprecated - use Topics instead)

- `openalex-pp-cli concepts get` — **DEPRECATED:** Use Topics instead. Retrieve a single concept by OpenAlex ID.
- `openalex-pp-cli concepts list` — **DEPRECATED:** Use Topics instead. Get a list of concepts from the legacy taxonomy.

**continents** — Geographic continents (7 total)

- `openalex-pp-cli continents get` — Retrieve a single continent by its Wikidata Q-ID.
- `openalex-pp-cli continents list` — Get a list of continents (7 total).

**countries** — Geographic countries for filtering research by location

- `openalex-pp-cli countries get-country` — Retrieve a single country by its ISO 3166-1 alpha-2 code.
- `openalex-pp-cli countries list` — Get a list of countries. Useful for filtering works by author affiliation country.

**domains** — Top-level categories in the topic hierarchy (4 total)

- `openalex-pp-cli domains get` — Retrieve a single domain by its ID (1-4).
- `openalex-pp-cli domains list` — Get a list of domains (top-level topic categories). There are only 4 domains: Life Sciences, Social Sciences,...

**fields** — Second-level categories in the topic hierarchy (26 total)

- `openalex-pp-cli fields get` — Retrieve a single field by its ID.
- `openalex-pp-cli fields list` — Get a list of fields (second-level topic categories). There are 26 fields spread across 4 domains.

**funders** — Organizations that fund research

- `openalex-pp-cli funders get` — Retrieve a single funder by OpenAlex ID or Crossref Funder ID.
- `openalex-pp-cli funders list` — Get a list of funders with optional filtering, searching, sorting, and pagination.

**institution-types** — Types of institutions (education, healthcare, company, etc.)

- `openalex-pp-cli institution-types` — Get a list of institution types (education, healthcare, company, etc.).

**institutions** — Universities, research organizations, and other affiliations

- `openalex-pp-cli institutions get` — Retrieve a single institution by OpenAlex ID or ROR.
- `openalex-pp-cli institutions list` — Get a list of institutions with optional filtering, searching, sorting, and pagination.

**keywords** — Short phrases identified from works' topics

- `openalex-pp-cli keywords get` — Retrieve a single keyword by OpenAlex ID.
- `openalex-pp-cli keywords list` — Get a list of keywords with optional filtering, searching, sorting, and pagination.

**languages** — Languages of scholarly works

- `openalex-pp-cli languages get` — Retrieve a single language by its ISO 639-1 code.
- `openalex-pp-cli languages list` — Get a list of languages used in scholarly works.

**licenses** — Open access licenses (CC BY, CC BY-SA, etc.)

- `openalex-pp-cli licenses` — Get a list of open access licenses (CC BY, CC BY-SA, etc.).

**publishers** — Companies and organizations that publish scholarly works

- `openalex-pp-cli publishers get` — Retrieve a single publisher by OpenAlex ID.
- `openalex-pp-cli publishers list` — Get a list of publishers with optional filtering, searching, sorting, and pagination.

**rate-limit** — Manage rate limit

- `openalex-pp-cli rate-limit` — Check your current rate limit status including usage and remaining allowance.

**sdgs** — UN Sustainable Development Goals (17 total)

- `openalex-pp-cli sdgs get` — Retrieve a single Sustainable Development Goal by its ID (1-17).
- `openalex-pp-cli sdgs list` — Get a list of UN Sustainable Development Goals. There are 17 SDGs.

**source-types** — Types of sources (journal, repository, conference, etc.)

- `openalex-pp-cli source-types` — Get a list of source types (journal, repository, conference, etc.).

**sources** — Journals, repositories, and other venues where works are hosted

- `openalex-pp-cli sources get` — Retrieve a single source by OpenAlex ID or ISSN.
- `openalex-pp-cli sources list` — Get a list of sources (journals, repositories, conferences) with optional filtering, searching, sorting, and pagination.

**subfields** — Third-level categories in the topic hierarchy (254 total)

- `openalex-pp-cli subfields get` — Retrieve a single subfield by its ID.
- `openalex-pp-cli subfields list` — Get a list of subfields (third-level topic categories). There are 254 subfields spread across 26 fields.

**text** — Manage text

- `openalex-pp-cli text` — **DEPRECATED:** This endpoint is deprecated and not recommended for new projects. It will not receive updates or...

**topics** — Research topics automatically assigned to works

- `openalex-pp-cli topics get` — Retrieve a single topic by OpenAlex ID.
- `openalex-pp-cli topics list` — Get a list of topics with optional filtering, searching, sorting, and pagination. Topics are research areas...

**work-types** — Types of scholarly works (article, book, dataset, etc.)

- `openalex-pp-cli work-types` — Get a list of work types (article, book, dataset, etc.).

**works** — Scholarly documents like journal articles, books, datasets, and theses

- `openalex-pp-cli works get` — Retrieve a single work by its OpenAlex ID or external ID (DOI, PMID, PMCID, MAG ID). External IDs can be passed as...
- `openalex-pp-cli works list` — Get a list of scholarly works with optional filtering, searching, sorting, and pagination. Works include journal...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openalex-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Set your API key via environment variable:

```bash
export OPENALEX_API_KEY="<your-key>"
```

Or persist it in `~/.config/openalex-pp-cli/config.toml`.

Run `openalex-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openalex-pp-cli authors list --agent --select id,name,status
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
openalex-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openalex-pp-cli feedback --stdin < notes.txt
openalex-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.openalex-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENALEX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENALEX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openalex-pp-cli profile save briefing --json
openalex-pp-cli --profile briefing authors list
openalex-pp-cli profile list --json
openalex-pp-cli profile show briefing
openalex-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `openalex-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openalex-pp-mcp -- openalex-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openalex-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openalex-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openalex-pp-cli <command> --help`.
