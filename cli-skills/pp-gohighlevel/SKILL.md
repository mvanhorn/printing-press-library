---
name: pp-gohighlevel
description: "The terminal for GoHighLevel. Bulk ops, dedup, and pipeline reports in seconds — local cache, agent-native JSON,... Trigger phrases: `search GHL contacts`, `GHL pipeline funnel`, `stale recruits in GoHighLevel`, `bulk tag GHL contacts`, `dedup GoHighLevel contacts`, `use ghlcli`, `run ghlcli`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gohighlevel-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/gohighlevel/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# GoHighLevel — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gohighlevel-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install gohighlevel --cli-only
   ```
2. Verify: `gohighlevel-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/cmd/gohighlevel-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you run GoHighLevel as a daily operations system, especially across multiple sub-accounts. It is built for agency operators and brokerage directors who need terminal-fast contact lookups, cross-pipeline funnel views, dedup, and SQL-shaped queries across their CRM data. The local SQLite cache makes it the right choice for bulk ops (tag 5000 contacts), recurring reports (Monday L10 prep), and agent-driven workflows that would otherwise require dozens of API round-trips.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`opp stale`** — Find opportunities sitting in a stage longer than N days, using synthesized stage-entry timestamps from sync history.

  _When the user asks 'which recruits have been sitting in this stage too long', this is the only path that resolves it without re-running their custom dashboard script._

  ```bash
  gohighlevel-pp-cli opp stale --pipeline "Non-KW" --stage "Recruit Lead" --days 30 --json
  ```
- **`opp funnel`** — Stage-by-stage count and total monetary value for a pipeline, output as TSV ready to paste into a dashboard sheet.

  _Replaces the user's Monday L10 prep script with a one-liner; the agent can re-run it on demand with different pipeline filters._

  ```bash
  gohighlevel-pp-cli opp funnel --pipeline "Non-KW" --tsv
  ```
- **`sql`** — Run read-only SQL against the local SQLite mirror of contacts, opportunities, pipelines, stages, tags, custom_fields, conversations, messages, and appointments.

  _The cross-entity differentiator. Anything an agent wants to answer about the GHL data becomes one SQL statement instead of three paginated API calls assembled in Python._

  ```bash
  gohighlevel-pp-cli sql "SELECT name, COUNT(*) FROM opportunities GROUP BY pipelineStageId ORDER BY 2 DESC" --json
  ```
- **`contact dedup`** — Group contacts by lowercased email and E.164 phone, score by filled-field count and recency, emit a merge plan.

  _Replaces a 200-line Python script the operator runs weekly. Agents can preview the merge before any data changes._

  ```bash
  gohighlevel-pp-cli contact dedup --by email,phone --dry-run --json
  ```
- **`contact decay`** — Find opportunities in a stage whose contacts have had no inbound or outbound messages in N days.

  _Catches recruits going cold before they're lost. The operator's existing alert script becomes a one-liner the agent can re-run with different thresholds._

  ```bash
  gohighlevel-pp-cli contact decay --stage "Engaged" --idle-days 30 --json
  ```
- **`recruit hot`** — Rank recruits by a composite of production signals, engagement, and recruit-tag count.

  _Monday L10 prep, before Kymber asks. Agents can re-tune the threshold without re-running the entire Python pipeline._

  ```bash
  gohighlevel-pp-cli recruit hot --threshold 25 --tsv
  ```
- **`convo thread`** — Reconstruct a chronological SMS+email+call thread for a single contact from the local messages table.

  _Before any recruit outreach, the operator wants the whole conversation in one place. Agents can read the full history without four MCP calls._

  ```bash
  gohighlevel-pp-cli convo thread --contact jane@example.com --json
  ```

### Service-specific guardrails
- **`field id`** — Translate human-readable custom field names into opaque GHL IDs, with did-you-mean suggestions on typos.

  _Pair `field id` with `sql` to query contacts by custom-field value without hardcoding 20-character GHL IDs; eliminates an entire class of broken-after-rename bugs._

  ```bash
  gohighlevel-pp-cli field id "Agent Affiliation"
  ```
- **`config use`** — Named profiles for each GHL sub-account (KWCP, THINK), with --location flag honored across every command.

  _Prevents the wrong-tenant footgun. Agents can be told 'use the kwcp profile' once at session start and never think about it again._

  ```bash
  gohighlevel-pp-cli config use kwcp && gohighlevel-pp-cli --location think opp funnel --pipeline "Non-KW"
  ```
- **`doctor`** — Validate the PIT token (auto-lowercases the prefix), ping the active location, and report local cache state.

  _First command to run when anything is acting funny. Agents can call it before complex flows to short-circuit a token or cache problem._

  ```bash
  gohighlevel-pp-cli doctor --json
  ```

### Agent-native plumbing
- **`contact bulk-tag`** — Apply or remove a tag across many contacts read from stdin, with chunked 100-at-a-time delivery and connection-drop retry.

  _The highest-leverage daily op for any agency — after every training, after every campaign, after every import. Without this the operator hand-rolls retry logic every time._

  ```bash
  cat emails.csv | gohighlevel-pp-cli contact bulk-tag --tag "Attended_2026-05-19" --dry-run
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**calendars** — Manage calendars

- `gohighlevel-pp-cli calendars create` — Create
- `gohighlevel-pp-cli calendars create-appointment` — Create appointment
- `gohighlevel-pp-cli calendars delete-appointment` — Delete appointment
- `gohighlevel-pp-cli calendars get` — Get
- `gohighlevel-pp-cli calendars get-appointment` — Get appointment
- `gohighlevel-pp-cli calendars list` — List
- `gohighlevel-pp-cli calendars list-events` — List calendar events for a location
- `gohighlevel-pp-cli calendars update-appointment` — Update appointment

**contacts** — Manage contacts

- `gohighlevel-pp-cli contacts bulk-update-tags` — Bulk add/remove tags across many contacts
- `gohighlevel-pp-cli contacts create` — Create a contact
- `gohighlevel-pp-cli contacts delete` — Delete a contact
- `gohighlevel-pp-cli contacts find-duplicate` — Find duplicate contact by email or name
- `gohighlevel-pp-cli contacts get` — Get a contact by id
- `gohighlevel-pp-cli contacts search` — GHL's /contacts/search has a hard 100-page cap. Use the `startAfter` cursor returned in the response for pagination...
- `gohighlevel-pp-cli contacts update` — Update a contact
- `gohighlevel-pp-cli contacts upsert` — Upsert contact by email or phone

**conversations** — Manage conversations

- `gohighlevel-pp-cli conversations get` — Get
- `gohighlevel-pp-cli conversations search` — Search conversations (Version 2021-04-15)
- `gohighlevel-pp-cli conversations send-message` — Send an SMS, email, or other message

**locations** — Manage locations

- `gohighlevel-pp-cli locations get` — Get
- `gohighlevel-pp-cli locations search` — Search locations (agency-level)

**opportunities** — Manage opportunities

- `gohighlevel-pp-cli opportunities create-opportunity` — Create opportunity
- `gohighlevel-pp-cli opportunities delete-opportunity` — Delete opportunity
- `gohighlevel-pp-cli opportunities get-opportunity` — Get opportunity
- `gohighlevel-pp-cli opportunities list-pipelines` — List pipelines for a location
- `gohighlevel-pp-cli opportunities search` — Search opportunities across a pipeline
- `gohighlevel-pp-cli opportunities update-opportunity` — Update opportunity
- `gohighlevel-pp-cli opportunities upsert-opportunity` — Upsert opportunity

**surveys** — Manage surveys

- `gohighlevel-pp-cli surveys` — List

**users** — Manage users

- `gohighlevel-pp-cli users get` — Get
- `gohighlevel-pp-cli users search` — Search users in a location

**workflows** — Manage workflows

- `gohighlevel-pp-cli workflows` — List workflows for a location


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gohighlevel-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pipeline funnel snapshot for the Monday L10

```bash
gohighlevel-pp-cli opp funnel --pipeline "Non-KW" --tsv
```

Aggregates opportunities by stage in your local cache; TSV output pastes directly into a dashboard sheet.

### Find recruits stuck in Engaged for 30+ days

```bash
gohighlevel-pp-cli opp stale --pipeline "Non-KW" --stage "Engaged" --days 30 --json
```

Uses the local stage_transitions table built from sync diffs — answers a question GHL's API alone cannot.

### Bulk-tag everyone who attended a training

```bash
cat ~/Downloads/attendees.csv | gohighlevel-pp-cli contact bulk-tag --tag "Attended_2026-05-19" --dry-run
```

Reads emails from stdin, chunks 100 at a time with retry on connection drops. Drop --dry-run to apply.

### Look up a custom-field ID by its human name

```bash
gohighlevel-pp-cli field id "Agent Affiliation"
```

Returns the opaque GHL field ID for a human-readable custom-field name. Pair with `gohighlevel-pp-cli sql "SELECT * FROM contacts WHERE ..."` to query by custom field value without hardcoding IDs.

### Cross-entity query (SQL over the cache)

```bash
gohighlevel-pp-cli sql "SELECT c.email, o.name, o.monetaryValue FROM opportunities o JOIN contacts c ON o.contactId=c.id ORDER BY o.dateUpdated DESC LIMIT 50" --json
```

Any join across the synced tables becomes one query — exactly the shape agents want to ask. Use --select with dotted paths to pluck nested fields like customFields.value.

## Auth Setup

GoHighLevel uses Private Integration Tokens (PIT). Set GHL_PIT_TOKEN in your environment (lowercase pit- prefix — capital Pit- returns 401 Invalid JWT). Optionally set GHL_LOCATION_ID for the active sub-account, or use named profiles via `gohighlevel-pp-cli config use <name>` to switch between locations like KWCP and THINK.

Run `gohighlevel-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
gohighlevel-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gohighlevel-pp-cli feedback --stdin < notes.txt
gohighlevel-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gohighlevel-pp-cli/feedback.jsonl`. They are never POSTed unless `GOHIGHLEVEL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOHIGHLEVEL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gohighlevel-pp-cli profile save briefing --json
gohighlevel-pp-cli --profile briefing calendars list --location-id 550e8400-e29b-41d4-a716-446655440000
gohighlevel-pp-cli profile list --json
gohighlevel-pp-cli profile show briefing
gohighlevel-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gohighlevel-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gohighlevel-pp-mcp -- gohighlevel-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gohighlevel-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gohighlevel-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gohighlevel-pp-cli <command> --help`.
