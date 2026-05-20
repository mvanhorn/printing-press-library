---
name: pp-adminbyrequest
description: "Every Admin By Request portal action, plus a local SQLite mirror of audit, events, inventory and requests for ad-hoc... Trigger phrases: `list pending admin by request elevations`, `approve adminbyrequest request`, `deny adminbyrequest request`, `generate offline elevation PIN`, `check who keeps requesting admin elevation`, `sync admin by request audit log`, `use adminbyrequest`, `run adminbyrequest`."
author: "joltsconsulting"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - adminbyrequest-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/adminbyrequest/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Admin By Request — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `adminbyrequest-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install adminbyrequest --cli-only
   ```
2. Verify: `adminbyrequest-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pull audit log, events, inventory and requests into a local store with one sync, then approve or deny elevations, generate offline PIN codes, and answer cross-resource questions (repeat requestors, agent-version drift, audit/event correlation) the portal does not surface.

## When to Use This CLI

Use this CLI when you are administering an Admin By Request tenant from a terminal or AI agent: triaging pending requests in bulk, pulling audit data into a SIEM you have not yet wired up, generating offline PINs without opening the portal, or answering cross-resource questions (repeat requestors, agent-version drift) that the portal does not directly support.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`requests repeat-offenders`** — Surface the top users by elevation-request count over a configurable window, so admins can spot patterns the portal does not visualize.

  _When tracing repeated elevation attempts across a fleet, this is the single query that names the people; without it, an agent has to walk every request and aggregate manually._

  ```bash
  adminbyrequest-pp-cli requests repeat-offenders --window 30d --json
  ```
- **`correlate`** — Given an audit log entry ID, join it to nearby events on the same computer to reconstruct a full elevation timeline.

  _Incident response often asks what else happened on this machine when this admin session opened; this is the single command that answers it._

  ```bash
  adminbyrequest-pp-cli correlate 50461167 --window 5m --json
  ```

### Fleet hygiene
- **`inventory drift`** — List endpoints whose AbR client version is older than a target version, useful for upgrade campaigns.

  _Before any compliance audit, the agent should know which endpoints are running an old client; this is the one-liner._

  ```bash
  adminbyrequest-pp-cli inventory drift --client-version 8.7.2 --json
  ```
- **`inventory risk-score`** — Score each endpoint by elevation frequency, local-admin count, and AbR client version recency.

  _Lets an admin focus remediation effort on the endpoints most likely to be abused._

  ```bash
  adminbyrequest-pp-cli inventory risk-score --top 10 --json
  ```

### Reachability mitigation
- **`quota forecast`** — Track local API call count against the 100k-per-day quota and predict whether the current pace will exceed it before midnight.

  _Reachability mitigation: an agent that calls the API in a loop needs to know before it bricks the tenant for the day._

  ```bash
  adminbyrequest-pp-cli quota forecast --json
  ```

### Compliance and review
- **`requests denied-reasons`** — Tokenize the free-text deniedReason field across all denied requests and emit a top-N word distribution.

  _For compliance review and tone-of-policy checks, surfacing the actual words used is faster than reading each row._

  ```bash
  adminbyrequest-pp-cli requests denied-reasons --top 20 --json
  ```
- **`report compliance`** — Render audit entries for a user or computer over a window in a format suitable for auditors (CSV or markdown).

  _Auditor evidence requests always want a single artifact; this generates it in one command._

  ```bash
  adminbyrequest-pp-cli report compliance --since 2026-01-01 --user CHRISCOOMBES --format md
  ```

## Command Reference

**auditlog** — Admin session and elevation audit log entries

- `adminbyrequest-pp-cli auditlog delta` — Delta query for audit log changes (use timeNow/deltaTime ticks for resumable sync)
- `adminbyrequest-pp-cli auditlog list` — List audit log entries (admin sessions, app elevations, denied requests). Pagination is by id cursor.

**events** — Security and operational events emitted by AbR clients

- `adminbyrequest-pp-cli events` — List events. Use code filter to narrow to specific event types.

**inventory** — Endpoint inventory: hardware, software, OS, AbR agent version

- `adminbyrequest-pp-cli inventory list` — List inventoried endpoints. Use wantsoftware/wanthardware to expand.
- `adminbyrequest-pp-cli inventory pin` — Generate an offline elevation PIN code for a device. Pass --challenge for challenge-response mode.

**requests** — Pending, approved, and denied elevation requests

- `adminbyrequest-pp-cli requests approve` — Approve a pending elevation request.
- `adminbyrequest-pp-cli requests deny` — Deny a pending elevation request.
- `adminbyrequest-pp-cli requests list` — List elevation requests filtered by status.


**Hand-written commands**

- `adminbyrequest-pp-cli correlate <auditlog-id>` — Join an audit log entry to nearby events on the same computer (window in minutes)
- `adminbyrequest-pp-cli report` — Render compliance reports from synced data
- `adminbyrequest-pp-cli quota` — Track local API call count vs AbR's 100k/day quota


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
adminbyrequest-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Triage pending requests as JSON

```bash
adminbyrequest-pp-cli requests list --status pending --agent --select id,user.account,reason
```

Surface the pending elevation queue in agent-friendly JSON narrowed to the fields needed to decide. Approve or deny individual ids with `requests approve <id> --reason ...` or `requests deny <id> --reason ...`.

### Compliance evidence for one user

```bash
adminbyrequest-pp-cli report compliance --since 2026-01-01 --user CHRISCOOMBES --format md > evidence.md
```

Render every audit log entry for a user since a date into a markdown report.

### Find every device on an old client

```bash
adminbyrequest-pp-cli inventory drift --client-version 8.7.2 --json --select name,abrClientVersion
```

Inventory drift filters the synced inventory snapshot for endpoints below the target client version.

### Check today's API quota consumption

```bash
adminbyrequest-pp-cli quota show --agent
```

Local counter of API calls made today; the CLI tracks them in `~/.cache/adminbyrequest-pp-cli/http/` so this command itself never costs an API call.

### Audit-to-event timeline narrowed via --select

```bash
adminbyrequest-pp-cli correlate 50461167 --window 5m --agent --select audit.application.name,audit.user.account,events.eventText,events.eventTime
```

Joins audit entry to nearby events on the same computer; --select narrows the deeply nested response so the agent does not parse the full payload.

## Auth Setup

Set ADMINBYREQUEST_API_KEY in your environment. The CLI sends it via the apikey header. AbR enforces a 100,000-call daily quota per tenant; the CLI tracks calls locally and warns you before exhaustion via quota forecast.

Run `adminbyrequest-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  adminbyrequest-pp-cli auditlog list --agent --select id,name,status
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
adminbyrequest-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
adminbyrequest-pp-cli feedback --stdin < notes.txt
adminbyrequest-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.adminbyrequest-pp-cli/feedback.jsonl`. They are never POSTed unless `ADMINBYREQUEST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ADMINBYREQUEST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
adminbyrequest-pp-cli profile save briefing --json
adminbyrequest-pp-cli --profile briefing auditlog list
adminbyrequest-pp-cli profile list --json
adminbyrequest-pp-cli profile show briefing
adminbyrequest-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `adminbyrequest-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add adminbyrequest-pp-mcp -- adminbyrequest-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which adminbyrequest-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   adminbyrequest-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `adminbyrequest-pp-cli <command> --help`.
