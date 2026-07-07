---
name: pp-rechtspraak
description: "The first CLI for Dutch court decisions with rich local narrowing for long-tail legal keywords, an appeal-chain walker Trigger phrases: `look up Dutch court decision`, `find a rechtspraak ECLI`, `narrow Dutch case search by keyword`, `exclude term from rechtspraak results`, `walk the appeal chain`, `Dutch case law search`, `use rechtspraak`, `run rechtspraak`."
author: "markvandeven"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - rechtspraak-pp-cli
---

# Rechtspraak — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `rechtspraak-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install rechtspraak --cli-only
   ```
2. Verify: `rechtspraak-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/rechtspraak/cmd/rechtspraak-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Dutch ECLI from data.rechtspraak.nl, indexed locally so include/exclude/phrase/regex narrowing cuts a 1000-result topic down to a usable set — the upstream API has no free-text search, so this is the only place that workflow exists. Walks the cassation chain in one command, exports vindplaatsen as BibTeX, and exposes every command as an MCP tool — the first MCP server for Dutch case law.

## When to Use This CLI

Reach for this CLI when an agent needs to research, monitor, or correlate Dutch court decisions. Strong fit for Dutch legal research, appellate workflow assistance, jurisprudence-corpus building, and any agent that needs structured access to ECLIs without parsing raw RDF/XML. Pairs naturally with `tenderned` and `pdok-location` for a full Dutch civic-data agent stack.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Graph walks across decisions
- **`chain`** — Walk the cassation, conclusie, and eerdere-aanleg graph for a Dutch court decision in both directions and print the full chain as a tree or JSON.

  _Use this whenever an agent needs the full procedural history of a Dutch decision (cassation chain, A-G conclusie, lower-court rulings) in one call instead of clicking through rechtspraak.nl tabs by hand._

  ```bash
  rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --depth 5 --agent
  ```
- **`citations`** — Extract the citation list (vindplaatsen) for a decision and emit as BibTeX, CSV, or JSON ready for legal-brief footnotes.

  _Reach for this when an agent is drafting a Dutch legal brief and needs the citation list for a cited decision without parsing raw RDF/XML._

  ```bash
  rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex
  ```
- **`conclusie`** — Given a Hoge Raad decision ECLI, return the matching A-G conclusie (or reverse).

  _When an agent reads a Hoge Raad ruling, immediately pull the Advocate-General's conclusie so the reasoning context is in scope._

  ```bash
  rechtspraak-pp-cli conclusie ECLI:NL:HR:2024:1 --agent
  ```

### Local state that compounds
- **`dossier`** — List every decision sharing a case number across instances, sorted chronologically.

  _Use this when tracking a single Dutch case across the rechtbank, gerechtshof, and Hoge Raad — the API offers no case-file filter, only the local index does._

  ```bash
  rechtspraak-pp-cli dossier 22/00155 --agent
  ```
- **`watch`** — Poll for new decisions matching a filter, dedupe against the local SQLite, and emit only the new ECLIs — cron-friendly with --quiet.

  _Wire this into a cron or agent loop to receive only newly-published Dutch court decisions on your filter — no manual rechtspraak.nl refreshing._

  ```bash
  rechtspraak-pp-cli watch --court HR --subject belastingrecht --since 7d --quiet --agent
  ```
- **`sync archive`** — Download the official weekly full-corpus archive and bulk-insert to the local SQLite — orders of magnitude faster than paging the Atom feed for backfills.

  _Use this when bootstrapping a research corpus or rebuilding the local DB; live sync against the Atom feed is the wrong tool for backfills._

  ```bash
  rechtspraak-pp-cli sync archive --week 2024-W01
  ```

### Agent-native plumbing
- **`code`** — Bidirectional offline lookup against the Instanties controlled vocabulary — court code to full name and PSI URI, or fuzzy name to code.

  _When an agent needs to resolve a Dutch court name, code, or PSI URI without round-tripping to the API, this is the lookup._

  ```bash
  rechtspraak-pp-cli code RBAMS --agent
  ```
- **`mcp serve`** — Expose every rechtspraak command as an MCP tool — the first MCP server for Dutch case law.

  _Wire this into Claude Desktop or any MCP-aware agent to give it native access to Dutch court decisions, citations, and case-files._

  ```bash
  rechtspraak-pp-cli mcp serve
  ```
- **`ecli parse`** — Pure-local regex parse of an ECLI into country, court, year, and sequence — with validation — and `ecli url` for the deeplink.

  _Use this whenever an agent needs to validate or destructure an ECLI before fetching content or chaining into another command._

  ```bash
  rechtspraak-pp-cli ecli parse ECLI:NL:HR:2024:1 --json
  ```

### Search narrowing for long-tail legal terms
- **`narrow`** — Read an ECLI list from stdin (or piped from search/watch/sync), fetch each decision's content from the live API (one HTTP call per ECLI, paced via the shared rate limiter), apply local include/exclude/phrase/regex filters against title+summary+body, and emit the narrowed list — the defining workflow for keyword-overlap-heavy Dutch legal research.

  _Use this when a metadata-only search returns hundreds or thousands of decisions and an agent needs to narrow by phrasing — chain it after search, watch, or sync. The same vocabulary (`--keyword`, `--exclude`, `--phrase`, `--regex`) also lives on `search` directly for single-shot use._

  ```bash
  rechtspraak-pp-cli uitspraken search --subject strafrecht --court HR --from 2024-01-01 --agent | rechtspraak-pp-cli narrow --keyword huurprijs --exclude "kort geding" --phrase "huurprijswijziging" --agent
  ```

## Command Reference

**courts** — Dutch courts (Instanties) controlled vocabulary - 79KB authoritative list

- `rechtspraak-pp-cli courts` — List every Dutch rechtsprekende instantie with PSI URI, full name, afkorting code, type, and Begin/EndDate.

**foreign-courts** — Foreign courts (BuitenlandseInstanties) referenced by Dutch decisions

- `rechtspraak-pp-cli foreign-courts` — Foreign court catalog: ECHR, CJEU, and ~5000 EU member-state courts with PSI URI, multilingual name, and country code

**foreign-decisions** — Non-Dutch decisions registered with LJN codes (NietNederlandseUitspraken)

- `rechtspraak-pp-cli foreign-decisions` — Bridge from foreign ECLIs (CJEU, ECHR) to old Dutch LJN codes - ~173k entries

**procedures** — Procedure types (Proceduresoorten) controlled vocabulary

- `rechtspraak-pp-cli procedures` — Procedure types (cassatie, hoger beroep, kort geding, etc.) with PSI URIs

**relations** — Formal relation types (FormeleRelaties) between decisions

- `rechtspraak-pp-cli relations` — Relation taxonomy

**subjects** — Subject areas (Rechtsgebieden) controlled vocabulary - hierarchical

- `rechtspraak-pp-cli subjects` — Subject-area taxonomy with PSI URIs, names, and parent-child hierarchy

**uitspraken** — Search and fetch Dutch court decisions (ECLI register)

- `rechtspraak-pp-cli uitspraken get` — Fetch a single decision by ECLI - returns full RDF metadata plus the inhoudsindicatie summary and the uitspraak body
- `rechtspraak-pp-cli uitspraken image` — Fetch an embedded image from a decision body by its imagedata identifier
- `rechtspraak-pp-cli uitspraken search` — Search the ECLI index by date, court, subject, type. Per IVO 1.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
rechtspraak-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Iteratively narrow an over-broad result set

```bash
rechtspraak-pp-cli uitspraken search --subject strafrecht --court HR --from 2024-01-01 --keyword huurprijs --exclude "kort geding" --phrase "huurprijswijziging" --annotate-count --agent
```

Upstream API returns metadata-only matches; without --scan-body the --keyword/--exclude/--phrase/--regex flags filter against title+summary in-memory (no fetch). With --scan-body each candidate's body is fetched and matched as well. --annotate-count prints total/fetched/post-narrow counts after the run.

### Walk the full procedural history of a Hoge Raad decision

```bash
rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --agent --select chain.ecli,chain.court,chain.date,chain.type
```

The response is deeply nested across instances; the dotted --select projection keeps the agent's context lean by emitting only the chain skeleton.

### Daily new-decision watch with narrowing

```bash
rechtspraak-pp-cli watch --court HR --subject belastingrecht --since 1d --keyword "omkering bewijslast" --quiet --agent
```

Exits silently if no new decisions match both the upstream filter (HR + belastingrecht + last 24h) AND the local keyword filter — drop-in for cron + mail.

### Pipe-compose narrowing across commands

```bash
rechtspraak-pp-cli uitspraken search --court HR --from 2024-01-01 --quiet | rechtspraak-pp-cli narrow --keyword belastingrecht --exclude conclusie --agent
```

Read ECLIs from any source, apply local filters, emit narrowed list — the same vocabulary as search but composable across pipelines.

### Bibliography export for a brief

```bash
rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex
```

Emits the vindplaatsen as BibTeX entries with stable keys — paste directly into a LaTeX brief.

### Resolve a court name to its PSI URI

```bash
rechtspraak-pp-cli code "Rechtbank Amsterdam" --json --select code,name,psi_uri
```

Fully offline after first sync; useful in agent chains that need to construct a search filter.

## Auth Setup

No authentication required.

Run `rechtspraak-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  rechtspraak-pp-cli courts --agent --select id,name,status
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
rechtspraak-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
rechtspraak-pp-cli feedback --stdin < notes.txt
rechtspraak-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/rechtspraak-pp-cli/feedback.jsonl`. They are never POSTed unless `RECHTSPRAAK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RECHTSPRAAK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
rechtspraak-pp-cli profile save briefing --json
rechtspraak-pp-cli --profile briefing courts
rechtspraak-pp-cli profile list --json
rechtspraak-pp-cli profile show briefing
rechtspraak-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `rechtspraak-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add rechtspraak-pp-mcp -- rechtspraak-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which rechtspraak-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   rechtspraak-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `rechtspraak-pp-cli <command> --help`.
