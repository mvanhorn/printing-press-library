---
name: pp-openfda
description: "Printing Press CLI for Openfda. The FDA safety data terminal. Every drug adverse event, device recall, food contamination, and product label the FDA..."
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openfda-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/openfda/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Openfda — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openfda-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install openfda --cli-only
   ```
2. Verify: `openfda-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The FDA safety data terminal. Every drug adverse event, device recall, food contamination, and product label the FDA publishes — synced locally, searchable offline, with trend analysis no other tool offers.


## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**animal-events** — Animal drug and device adverse event reports.

- `openfda-pp-cli animal-events` — Search animal adverse event reports

**device-510k** — Premarket notification submissions demonstrating substantial equivalence.

- `openfda-pp-cli device-510k` — Search 510(k) clearance records

**device-classification** — Medical device product codes, specialty areas, and regulatory class.

- `openfda-pp-cli device-classification` — Search device classifications

**device-covid19** — COVID-19 serological testing evaluation data.

- `openfda-pp-cli device-covid19` — Search COVID-19 serology test evaluations

**device-events** — Medical device adverse event reports (MAUDE/MDR) — injuries, deaths, malfunctions.

- `openfda-pp-cli device-events count` — Count device events by field
- `openfda-pp-cli device-events list` — Search device adverse event reports

**device-pma** — Class III medical device premarket approval decisions.

- `openfda-pp-cli device-pma` — Search premarket approval records

**device-recall-detail** — Detailed device recall actions addressing defects or health risks.

- `openfda-pp-cli device-recall-detail` — Search device recall action details

**device-recalls** — Medical device recall enforcement reports.

- `openfda-pp-cli device-recalls` — Search device recall enforcement reports

**device-registration** — Medical device manufacturing establishment registrations and product listings.

- `openfda-pp-cli device-registration` — Search device registrations and listings

**device-udi** — Global Unique Device Identification Database (GUDID).

- `openfda-pp-cli device-udi` — Search unique device identifiers

**drug-approvals** — FDA-approved drug products since 1939 — applications, submissions, and marketing status.

- `openfda-pp-cli drug-approvals` — Search approved drug products

**drug-events** — Reports of drug side effects, medication errors, product quality problems (FAERS). 4.9M+ reports since 2003.

- `openfda-pp-cli drug-events count` — Count adverse events by field
- `openfda-pp-cli drug-events list` — Search drug adverse event reports

**drug-labels** — Structured product information including prescribing info, black box warnings, indications.

- `openfda-pp-cli drug-labels` — Search drug product labels

**drug-ndc** — National Drug Code directory — product identifiers, packaging, and classification.

- `openfda-pp-cli drug-ndc` — Search NDC directory

**drug-recalls** — Drug product recall enforcement reports.

- `openfda-pp-cli drug-recalls count` — Count drug recalls by field
- `openfda-pp-cli drug-recalls list` — Search drug recall enforcement reports

**drug-shortages** — Current and historical drug shortages from manufacturing issues, delays, and discontinuations.

- `openfda-pp-cli drug-shortages` — Search drug shortages

**food-events** — CAERS reports — food, dietary supplement, and cosmetic adverse events.

- `openfda-pp-cli food-events` — Search food/supplement adverse event reports

**food-recalls** — Food product recall enforcement reports.

- `openfda-pp-cli food-recalls count` — Count food recalls by field
- `openfda-pp-cli food-recalls list` — Search food recall enforcement reports

**nsde** — Non-Standardized Drug Entities — drug names that don't map to standard terminology.

- `openfda-pp-cli nsde` — Search non-standardized drug entities

**substance** — Substance data from the FDA substance registration system.

- `openfda-pp-cli substance` — Search substance records

**tobacco-problems** — Tobacco product problem reports.

- `openfda-pp-cli tobacco-problems` — Search tobacco problem reports


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openfda-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Set your API key via environment variable:

```bash
export FDA_API_KEY="<your-key>"
```

Or persist it in ``.

Run `openfda-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openfda-pp-cli animal-events --agent --select id,name,status
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
openfda-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openfda-pp-cli feedback --stdin < notes.txt
openfda-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.openfda-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENFDA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENFDA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openfda-pp-cli profile save briefing --json
openfda-pp-cli --profile briefing animal-events
openfda-pp-cli profile list --json
openfda-pp-cli profile show briefing
openfda-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `openfda-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openfda-pp-mcp -- openfda-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openfda-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openfda-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openfda-pp-cli <command> --help`.
