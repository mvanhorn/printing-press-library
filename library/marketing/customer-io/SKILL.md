---
name: pp-customer-io
description: "Every Customer.io action a marketer or ops engineer takes — campaigns, broadcasts, segments, deliveries, exports, suppressions, Reverse-ETL — wrapped in named verbs, backed by a local SQLite cache, and served through a bundled MCP server. Trigger phrases: `use customer-io`, `run customer-io`, `trigger a customer.io broadcast`, `send a customer.io transactional message`, `export a customer.io segment`, `check customer.io delivery health`, `audit customer.io suppressions`, `what fraction of segment X opened journey Y in customer.io`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - customer-io-pp-cli
    install:
      - kind: go
        bins: [customer-io-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-cli
---

# Customer.io — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `customer-io-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install customer-io --cli-only
   ```
2. Verify: `customer-io-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official customerio/cli exposes the API as raw passthrough. This CLI gives you typed commands for every meaningful workflow, plus eight commands no other tool has — journey funnels cross-cut by segment, multi-segment overlap, customer 360 timelines, broadcast pre-flight, suppression audit trails, Reverse-ETL health, bulk suppress with provenance, and incident-ready delivery triage bundles. It is also the only Customer.io tool that ships an MCP server.

## When to Use This CLI

Reach for this CLI when you need to do anything in Customer.io that's tedious in the web UI: bulk suppressions with an audit trail, journey funnels cross-cut by segment, customer-360 timelines for incident triage, broadcast pre-flight before a high-stakes send, or Reverse-ETL health checks. It is also the right tool when an agent needs to drive Customer.io — the bundled MCP server exposes the same verbs over stdio and HTTP transports. For raw event ingestion (the Track API) or per-source CDP write keys, prefer one of the official server-side SDKs.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`campaigns funnel`** — Render a step-by-step journey funnel (sent → delivered → opened → clicked → converted) for one campaign, optionally cross-cut by segment.

  _Reach for this when an agent is asked 'what fraction of segment X completed journey Y' — the answer is one query against synced data, not two exports and a spreadsheet._

  ```bash
  customer-io campaigns funnel cmp_482 --segment seg_19 --since 30d --json
  ```
- **`segments overlap`** — Compute pairwise and multi-way Venn diagrams of segment memberships from the local store.

  _Use when validating audience design — 'are my churned-risk and high-value segments overlapping?' — without exporting twice._

  ```bash
  customer-io segments overlap seg_19 seg_42 seg_88 --json
  ```
- **`customers timeline`** — Chronological per-customer event stream merging identifies, deliveries, suppressions, and segment-membership events from the local store.

  _Reach for this on incident triage or 'did the welcome SMS go out to this user?' — one command instead of a UI scroll-fest._

  ```bash
  customer-io customers timeline alice@example.com --since 30d --json
  ```

### Send-time safety
- **`broadcasts preflight`** — Before triggering a broadcast, check target segment size, suppression overlap, and last-sent recency from the local deliveries cache; emit a green/yellow/red verdict with structured reasons.

  _Use before any broadcast to avoid double-sending, accidental over-mailing, or 429 storms._

  ```bash
  customer-io broadcasts preflight 123457 bcr_77 --segment seg_19 --json
  ```
- **`suppressions audit`** — Attribute every suppression in a window to the triggering bounce or complaint delivery (or 'manual' if no preceding event).

  _Use when an ops engineer needs to explain why a customer is suppressed, or when auditing complaint-driven churn._

  ```bash
  customer-io suppressions audit --since 30d --reason bounce --json
  ```
- **`cdp-reverse-etl health`** — Named verb over Reverse-ETL run history with status, row counts, error reasons, and an optional --watch poll mode.

  _Use as the daily standup question 'are warehouse syncs healthy?' — replaces three separate api calls + jq filters._

  ```bash
  customer-io cdp-reverse-etl health --since 24h --watch
  ```

### Audit and provenance
- **`suppressions bulk add`** — Read a CSV or JSONL of email/customer-id, fan out real suppress calls with adaptive throttle, append every call to a local JSONL audit log keyed by date.

  _Use for compliance-driven bulk actions where you need to defend later 'who got suppressed when, by whom, with what status code'._

  ```bash
  customer-io suppressions bulk add --from-csv complaints.csv --reason complaint --dry-run
  ```
- **`deliveries triage`** — Filter live + local deliveries by template + status + window, write a self-contained bundle (summary.md with SQL group-by error reasons, deliveries.jsonl, recipients.txt) ready to paste into an incident doc.

  _Pipe the bundle into Claude or a Notion doc for one-shot incident handoff; replaces 30+ minutes of UI scrolling._

  ```bash
  customer-io deliveries triage --template tx_91 --status bounced --since 1h --bundle ./incident-2026-05-07
  ```

## Command Reference

**broadcasts** — List, inspect, and trigger one-off broadcasts (1 req / 10 s rate-limited)

- `customer-io-pp-cli broadcasts get` — Get one broadcast
- `customer-io-pp-cli broadcasts list` — List broadcasts in an environment
- `customer-io-pp-cli broadcasts metrics` — Read metrics for one broadcast
- `customer-io-pp-cli broadcasts trigger` — Trigger a broadcast (rate-limited to 1 req / 10 s)

**campaigns** — List campaigns and read campaign + journey metrics

- `customer-io-pp-cli campaigns get` — Get one campaign
- `customer-io-pp-cli campaigns journey_metrics` — Read step-by-step journey funnel metrics
- `customer-io-pp-cli campaigns list` — List campaigns in an environment
- `customer-io-pp-cli campaigns metrics` — Read aggregate metrics for a campaign

**cdp_destinations** — List CDP destinations (Premium feature)

- `customer-io-pp-cli cdp_destinations` — List CDP destinations

**cdp_reverse_etl** — List Reverse-ETL syncs (Premium feature)

- `customer-io-pp-cli cdp_reverse_etl` — List Reverse-ETL syncs

**cdp_sources** — List CDP data sources (Premium feature)

- `customer-io-pp-cli cdp_sources` — List CDP sources

**customers** — Manage Customer.io customer profiles within an environment (workspace)

- `customer-io-pp-cli customers get` — Get a customer's attributes
- `customer-io-pp-cli customers list_activities` — List activity events for a customer
- `customer-io-pp-cli customers list_messages` — List messages sent to a customer
- `customer-io-pp-cli customers list_segments` — List the segments a customer belongs to

**deliveries** — Inspect delivery events (sends, opens, clicks, bounces, complaints)

- `customer-io-pp-cli deliveries get` — Get one delivery
- `customer-io-pp-cli deliveries list` — List recent deliveries

**exports** — Start, monitor, and download data exports (segment members, deliveries, customers, etc.)

- `customer-io-pp-cli exports download` — Get the signed download URL for a finished export
- `customer-io-pp-cli exports get` — Get the status of an export
- `customer-io-pp-cli exports list` — List recent exports

**segments** — List segments and inspect segment membership

- `customer-io-pp-cli segments customer_count` — Get the customer count for a segment
- `customer-io-pp-cli segments get` — Get one segment
- `customer-io-pp-cli segments list` — List segments in an environment
- `customer-io-pp-cli segments members` — List customer IDs in a segment

**suppressions** — Suppress and unsuppress customers; the official audit surface for compliance actions

- `customer-io-pp-cli suppressions add` — Suppress a customer
- `customer-io-pp-cli suppressions count` — Get the count of suppressed customers in an environment (Customer.io has no list endpoint; use 'exports...
- `customer-io-pp-cli suppressions remove` — Remove a suppression

**transactional** — Inspect transactional templates and metrics

- `customer-io-pp-cli transactional get_template` — Get one transactional template
- `customer-io-pp-cli transactional list_templates` — List transactional templates
- `customer-io-pp-cli transactional template_metrics` — Read metrics for one transactional template

**webhooks** — Manage Reporting Webhooks for delivery + engagement events

- `customer-io-pp-cli webhooks get` — Get one Reporting Webhook
- `customer-io-pp-cli webhooks list` — List Reporting Webhooks

**workspaces** — List the environments (workspaces) visible to the Service Account

- `customer-io-pp-cli workspaces account` — Get the current account details
- `customer-io-pp-cli workspaces list` — Read the current account, including environment_ids visible to the SA token


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
customer-io-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find which high-value customers haven't engaged with the welcome journey

```bash
customer-io campaigns funnel cmp_welcome --segment seg_high_value --since 90d --agent --select steps.delivered.count,steps.opened.count,steps.clicked.count,non_engaged.recipients
```

The funnel command joins synced deliveries with segment members; --agent + --select narrows the deeply-nested funnel response to just the counts and the non-engaged recipient list, keeping the agent's context lean.

### Draft an incident bundle for a transactional bounce spike

```bash
customer-io deliveries triage --template tx_password_reset --status bounced --since 2h --bundle ./incident-bounce-2026-05-07
```

Writes summary.md with grouped error reasons, deliveries.jsonl with full delivery objects, and recipients.txt — paste-ready for a Notion incident doc or a Claude summarize prompt.

### Prove that a broadcast is safe to trigger

```bash
customer-io broadcasts preflight bcr_summer_promo --segment seg_active_30d --json
```

Returns a green/yellow/red verdict with structured reasons (segment size, suppression overlap, last-sent recency from synced deliveries). Run before every broadcast at non-trivial scale.

### Bulk-suppress a complaint list with provenance

```bash
customer-io suppressions bulk add --from-csv complaints-2026-05.csv --reason complaint --dry-run
```

Drop --dry-run to commit. Every call lands in `~/.customer-io/audit/suppressions-2026-05-07.jsonl` with timestamp, recipient, status, and HTTP code — defensible later.

### Watch Reverse-ETL syncs in real time

```bash
customer-io cdp-reverse-etl health --watch --since 24h
```

Polls every 60 s and prints a status row per sync; pipe to `tee` to keep a log.

## Auth Setup

Customer.io uses Service Account tokens (`sa_live_*` prefix). The CLI exchanges the token for a short-lived JWT via the OAuth client-credentials endpoint at `https://us.fly.customer.io/v1/service_accounts/oauth/token` (or `eu.fly.customer.io` for EU workspaces) and uses the JWT as the Bearer for both the Journeys UI API (`/v1/...`) and the CDP control plane (`/cdp/api/...`). Run `customer-io auth login --sa-token $CIO_TOKEN --region us` once; the cached JWT auto-refreshes. The Track API (separate Site ID + API Key auth) is intentionally out of scope for v1 — the SA token is the unified credential.

Run `customer-io-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  customer-io-pp-cli broadcasts list mock-value --agent --select id,name,status
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
customer-io-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
customer-io-pp-cli feedback --stdin < notes.txt
customer-io-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.customer-io-pp-cli/feedback.jsonl`. They are never POSTed unless `CUSTOMER_IO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CUSTOMER_IO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
customer-io-pp-cli profile save briefing --json
customer-io-pp-cli --profile briefing broadcasts list mock-value
customer-io-pp-cli profile list --json
customer-io-pp-cli profile show briefing
customer-io-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `customer-io-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/customer-io/cmd/customer-io-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add customer-io-pp-mcp -- customer-io-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which customer-io-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   customer-io-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `customer-io-pp-cli <command> --help`.
