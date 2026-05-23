---
name: pp-buildalert
description: "Pull your BuildAlert dashboard offline — every matched planning lead, transaction, and ROI metric queryable via SQL/FTS"
author: "Muhammad Khan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - buildalert-pp-cli
    install:
      - kind: go
        bins: [buildalert-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/cmd/buildalert-pp-cli
---

# Buildalert — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `buildalert-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install buildalert --cli-only
   ```
2. Verify: `buildalert-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/cmd/buildalert-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

BuildAlert UK construction lead platform — planning applications across 400+ UK councils, with applicant addresses and agent details for paid subscribers.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### ZAZU integration
- **`zazu-diff`** — List BuildAlert leads that aren't yet in your ZAZU bd-mirror.sqlite, so you know exactly what BuildAlert's 400+ council coverage adds to your own scrapers.

  _Pick this when you need to know which BuildAlert leads to ingest into ZAZU without duplicating effort._

  ```bash
  buildalert-pp-cli zazu-diff --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --agent
  ```
- **`pending-letters`** — Surface BuildAlert leads where canSendLetter is true, no letter has been sent yet, and ZAZU's own send log has no record either — the morning worklist for outreach.

  _Use this as the single actionable list when deciding which homeowners to contact today._

  ```bash
  buildalert-pp-cli pending-letters --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --agent --select reference,address,estimationValueBand,distanceAway
  ```
- **`letter-conflict`** — Find BuildAlert leads where letterBeenSent is true AND ZAZU's send log already records a Telegram send for the same applicant — i.e., a double-mailed homeowner.

  _Run before any BuildAlert letter campaign to catch homeowners ZAZU already mailed._

  ```bash
  buildalert-pp-cli letter-conflict --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --json
  ```
- **`coverage`** — Group leads by council across both BuildAlert and ZAZU stores; flag councils where BuildAlert has volume but ZAZU has nothing (and vice versa).

  _Use this when planning which UK councils to add to ZAZU's scraper coverage next._

  ```bash
  buildalert-pp-cli coverage --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --json
  ```

### Spend visibility
- **`analytics --type transactions`** — Aggregate the £2-per-letter charges across the local transactions mirror, grouped by council, project type, or month — the answers your accountant asks for.

  _Use this for monthly or quarterly cost reviews; pipe to CSV for spreadsheet handoff._

  ```bash
  buildalert-pp-cli analytics --type transactions --group-by council --json
  ```
- **`roi-per-lead`** — Join transactions × tracking × applications locally to produce per-lead rows: cost, reply received, work won, total return — sorted by ROI.

  _Use this to identify which project types and councils convert at the best £/work-won ratio._

  ```bash
  buildalert-pp-cli roi-per-lead --json --select reference,cost,replied,workWon
  ```

### Offline querying
- **`nearby`** — Compute haversine distance from any UK postcode against the local mirror's lat/lng coordinates; return all leads inside a radius without re-hitting the API.

  _Use this to evaluate radius expansion experiments or to score leads against a satellite-office postcode._

  ```bash
  buildalert-pp-cli nearby --postcode HA1 --radius 10 --json --select reference,distance,fullDescription
  ```

## Command Reference

**health** — Backend liveness check.

- `buildalert-pp-cli health` — Lightweight liveness probe. Returns {'success': true}.

**leads** — Planning-application leads matched to the user's filters across UK councils.

- `buildalert-pp-cli leads` — List planning-application leads matched to the user's saved filters.

**letter_templates** — User's saved letter templates for the £2-per-letter outreach flow.

- `buildalert-pp-cli letter_templates` — List the user's letter templates plus the baseLogoUrl for the templated letterhead.

**tracking** — ROI tracking - letters sent, replies, conversion rate, work won, total return, plus chartData.

- `buildalert-pp-cli tracking` — ROI summary + per-letter tracking entries in a date window.

**transactions** — BuildAlert letter-send transactions (£2/letter, £2.50/postcard purchases).

- `buildalert-pp-cli transactions` — List letter-send transactions in a date window. Requires both dateFrom and dateTo as unix-seconds.

**user** — Authenticated user profile, dashboard summary, and filter preferences.

- `buildalert-pp-cli user dashboard` — Dashboard overview - newLeadsCount, letterSentCount, lastLetterSentDate, credits, totalPlanningApplications
- `buildalert-pp-cli user details` — Same payload as `user profile` (alias). Returns user profile + filter preferences.
- `buildalert-pp-cli user profile` — Authenticated user profile - email, role, profession, company, credits, postCode, radius, longitude, latitude


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
buildalert-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to www.buildalert.uk in Chrome, then:

```bash
buildalert-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `buildalert-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  buildalert-pp-cli leads --agent --select id,name,status
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
buildalert-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
buildalert-pp-cli feedback --stdin < notes.txt
buildalert-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.buildalert-pp-cli/feedback.jsonl`. They are never POSTed unless `BUILDALERT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BUILDALERT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
buildalert-pp-cli profile save briefing --json
buildalert-pp-cli --profile briefing leads
buildalert-pp-cli profile list --json
buildalert-pp-cli profile show briefing
buildalert-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `buildalert-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/cmd/buildalert-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add buildalert-pp-mcp -- buildalert-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which buildalert-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   buildalert-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `buildalert-pp-cli <command> --help`.
