---
name: pp-pushpress
description: "Every PushPress /v3 endpoint that exists, plus the going-dark report and the trainer-dashboard one-liner the API... Trigger phrases: `who hasn't been to the gym in N days`, `show me my pushpress members`, `going-dark report`, `pushpress check-ins today`, `member 360 for <email>`, `use pushpress`, `run pushpress-pp-cli`."
author: "Alex Puckhaber"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pushpress-pp-cli
---

# PushPress — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pushpress-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pushpress --cli-only
   ```
2. Verify: `pushpress-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps PushPress's /v3 Platform API (members, check-ins, app installs, webhooks, messages) with a Go binary that caches every entity in local SQLite and adds the cross-entity views the API doesn't: `going-dark` for daily churn audits, `roster` for the trainer dashboard, `kpi today` for the business cockpit, `member` for coach prep. Five categories absent from /v3 (plans/billing, classes, leads, signup-source, tasks/notes, cancellations) ship as honest gap-flagged commands that point at the documented /v2 follow-up rather than silently dropping.

## When to Use This CLI

Reach for `pp-pushpress` when you need member-attendance signals an agent can act on — going-dark lists for re-engagement, per-member visit history before a coaching session, daily KPI tickers for the dashboard. Skip it for billing or class-definition queries; those need the /v2 follow-up and currently print a 'not supported' message.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Churn signals
- **`going-dark`** — List members whose most-recent check-in is older than N days (the operator's daily churn signal). Local SQLite join — no PushPress API endpoint computes this.

  _Reach for this when answering 'who do I need to re-engage' — the daily-churn question every gym owner runs by hand._

  ```bash
  pushpress-pp-cli going-dark --days 14 --json
  ```
- **`recency`** — Bucket all active members by days-since-last-checkin; emits count + sample of names per bucket.

  _Reach for this for the daily KPI dashboard — one histogram of who's still active, who's drifting._

  ```bash
  pushpress-pp-cli recency --bucket 7,14,30,60,90 --json
  ```
- **`kpi today`** — One pass over local store: signups today, check-ins today, active members, going-dark @ 14d / 30d. JSON-friendly for cron.

  _Reach for this from a business-dashboard cron job — one row, no parsing._

  ```bash
  pushpress-pp-cli kpi today --json
  ```

### Coach surface
- **`roster`** — One line per active member: id, name, plan, status, last_visit, days_since. The trainer-dashboard's default view.

  _Reach for this from coach prep, trainer-dashboard renders, or any 'list my members' agent task._

  ```bash
  pushpress-pp-cli roster --json
  ```
- **`member`** — Single command: profile + plan/status + first_seen + last_seen + total_checkins + last 10 check-ins + current streak + cadence trend.

  _Reach for this before a coaching session, an at-risk-member outreach, or any per-member context an agent needs._

  ```bash
  pushpress-pp-cli member user@example.com
  ```

### Operator analytics
- **`class-mix`** — Histogram of class names from local check-ins over a window: counts + percent share per class.

  _Reach for this when answering 'what class is most popular this month' or 'which class is bleeding members'._

  ```bash
  pushpress-pp-cli class-mix --days 30 --json
  ```

## Command Reference

**apps** — manage PushPress appl ecosystem

- `pushpress-pp-cli apps get` — Get details of a specific app
- `pushpress-pp-cli apps list` — List all available apps

**checkins** — Manage checkins

- `pushpress-pp-cli checkins get` — Get a check-in by ID
- `pushpress-pp-cli checkins list` — Get a list of all check-ins

**company** — Manage company

- `pushpress-pp-cli company` — Get company details associated with the API key

**customers** — Manage customers

- `pushpress-pp-cli customers get` — Get a customer by ID
- `pushpress-pp-cli customers list` — Get a list of all customers in a

**keys** — Manage keys

- `pushpress-pp-cli keys create-api` — Create a new API key for a company.
- `pushpress-pp-cli keys delete-api` — Permanently delete an API key from the system.
- `pushpress-pp-cli keys get-api` — Retrieve a single API key.
- `pushpress-pp-cli keys list-api` — List all active API keys for a client.

**messages** — Manage messages

- `pushpress-pp-cli messages send-email` — Send an email
- `pushpress-pp-cli messages send-ping` — Send a ping notification via Ably Realtime
- `pushpress-pp-cli messages send-push` — Send a push notification

**webhooks** — create and configure webhooks for PushPress events

- `pushpress-pp-cli webhooks create` — Create a new webhook to subscribe to one or more events
- `pushpress-pp-cli webhooks delete` — Delete a specific webhook
- `pushpress-pp-cli webhooks get` — Get details of a specific webhook
- `pushpress-pp-cli webhooks list` — List all registered webhooks
- `pushpress-pp-cli webhooks update` — Update the URL or events for an existing webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pushpress-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday-morning churn audit

```bash
pushpress-pp-cli going-dark --days 14 --json --select customer_id,name,days_since_last_visit
```

Lists every active member who hasn't checked in for 14+ days, narrowed to just the fields a follow-up campaign needs.

### Pre-session coach prep

```bash
pushpress-pp-cli member user@example.com --json
```

Member 360 view for a specific client by email; collapses 4-5 dashboard clicks into one call.

### Daily KPI cron line

```bash
pushpress-pp-cli kpi today --json
```

One-shot JSON for the business-dashboard nightly cron.

### Programming retro

```bash
pushpress-pp-cli class-mix --days 30 --json
```

Histogram of class popularity for the last 30 days — which classes are pulling, which are bleeding.

## Auth Setup

Authenticates with a PushPress API key. Set it once with `pushpress-pp-cli auth set-token <key>` or `export PUSHPRESS_API_KEY=<key>`. Every request sends `Authorization: Bearer <key>`. Some endpoints also accept an optional `companyId` HEADER for tenant scoping.

Run `pushpress-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pushpress-pp-cli apps list --agent --select id,name,status
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
pushpress-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pushpress-pp-cli feedback --stdin < notes.txt
pushpress-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pushpress-pp-cli/feedback.jsonl`. They are never POSTed unless `PUSHPRESS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PUSHPRESS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pushpress-pp-cli profile save briefing --json
pushpress-pp-cli --profile briefing apps list
pushpress-pp-cli profile list --json
pushpress-pp-cli profile show briefing
pushpress-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pushpress-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pushpress-pp-mcp -- pushpress-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pushpress-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pushpress-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pushpress-pp-cli <command> --help`.
