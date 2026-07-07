---
name: pp-bento
description: "Every Bento feature, plus the ones their own CLI does not have. Trigger phrases: `import this list to Bento`, `scrub these emails before sending`, `show my Bento subscribers`, `build a win-back cohort`, `preview this Bento broadcast`, `sync Bento`, `use bento`, `run bento`."
author: "bossriceshark"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bento-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/bento/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Bento — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bento-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bento --cli-only
   ```
2. Verify: `bento-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bento/cmd/bento-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for bento-pp-cli when you need a fast offline view over your Bento account (FTS, JOINs, cohort builds), when you're scripting CI/CD imports, when you need transactional send from a Go process without dragging in Node, or when you're bridging Vendure / non-native ecom platforms to Bento via webhook + $purchase events. Pair with Claude Desktop / Cursor via MCP for natural-language access to every command.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Sync Integrity
- **`sync diff`** — Show exactly which subscribers, tags, fields, or templates changed between two local snapshots since Bento has no since filter.

  _Use this when you need to detect drift between Bento and your local source of truth, or audit what changed since the last sync._

  ```bash
  bento sync diff --since 2026-05-19 --resource subscribers --json
  ```
- **`tags drift`** — Surface tags whose subscriber counts swung more than N% week-over-week, catching misfiring workflows.

  _Run weekly to catch silently-broken automations adding or removing tags unexpectedly._

  ```bash
  bento tags drift --window 30d --threshold 25 --json
  ```

### List Hygiene
- **`hygiene scrub`** — Run validation + jesses_ruleset + blacklist + content_moderation in one pass and emit a clean CSV plus a rejected CSV with reasons.

  _Reach for this before importing any list you didn't generate yourself. Bad list = ISP penalty._

  ```bash
  bento hygiene scrub --in leads.csv --out-clean clean.csv --out-rejected rejected.csv
  ```

### Vendure Bridge
- **`events purchase-replay`** — Dry-run a Vendure order export against the Bento $purchase mapping and show the exact batch payload before sending.

  _Use before any Vendure -> Bento purchase sync to catch shape mismatches without polluting subscriber history._

  ```bash
  bento events purchase-replay --from vendure-orders.json --dry-run --json
  ```
- **`events review-window`** — Emit a dated Bento event stream for orders shipped N days ago, skipping any order your existing third-party review platform already triggered.

  _Use when running a Bento-side review trigger alongside a third-party review platform without double-mailing._

  ```bash
  bento events review-window --shipped-csv orders.csv --delay 10d --skip-stamped --dry-run
  ```
- **`fields lint`** — Diff local custom fields against a Vendure customer-schema file and flag missing, renamed, or type-mismatched fields.

  _Use after any Vendure schema change to verify Bento custom fields still align._

  ```bash
  bento fields lint --against vendure-schema.json --json
  ```

### Safety Rails
- **`events lint`** — Warn when a payload uses /fetch/commands subscribe but the intent (welcome flow, tag-triggered automation) requires /batch/events to fire automations.

  _Run this on any payload bound for /fetch/commands or /batch/events to catch silent automation misfires._

  ```bash
  bento events lint --file events.jsonl --json
  ```
- **`subscribers pre-delete`** — Show tags, events, revenue, and active automations a subscriber set carries before you file a GDPR data_deletion_request.

  _Always run before any GDPR/CCPA batch deletion to confirm the right cohort and surface revenue impact._

  ```bash
  bento subscribers pre-delete --emails-from gdpr-batch.txt --json
  ```

### Retention
- **`subscribers churn-risk`** — Rank subscribers by days-since-last-open/click against your cohort baseline before the next broadcast goes out.

  _Use before queuing a broadcast to identify the cohort most likely to mark as spam._

  ```bash
  bento subscribers churn-risk --threshold high --limit 100 --json
  ```
- **`subscribers winback`** — Build a CSV of customers who bought once, lapsed 180+ days, and never opened the last 3 broadcasts.

  _Reach for this when scoping a reactivation campaign with a custom decay window._

  ```bash
  bento subscribers winback --lapsed 180d --last-purchased 90d --csv
  ```

### Campaign Planning
- **`broadcasts whatif`** — Estimate audience size, predicted opens, and hygiene-risk count for a draft broadcast before you queue it in the dashboard.

  _Use before any broadcast send to sanity-check audience size and deliverability risk._

  ```bash
  bento broadcasts whatif --segment seg_abc --json
  ```

### Discovery
- **`subscribers find`** — Full-text search subscriber notes/fields plus structured filters in one query, exportable as CSV.

  _Reach for this when building any ad-hoc cohort that isn't worth a segment in the dashboard._

  ```bash
  bento subscribers find "first order" --tagged customer --purchased-after 365d --csv
  ```

## Command Reference

**batch** — Manage batch

- `bento-pp-cli batch create` — This endpoint allows you to create multiple email broadcasts efficiently.
- `bento-pp-cli batch create-emails` — This endpoint allows you to create and send multiple emails efficiently.
- `bento-pp-cli batch create-events` — This endpoint allows you to send multiple events to your site.
- `bento-pp-cli batch create-subscribers` — This endpoint allows for efficient bulk creation and updating of subscribers.

**experimental** — Access experimental APIs designed for specific use cases. Use with caution as these may change.

- `bento-pp-cli experimental create` — This endpoint will return an opinionated moderation score based on the content provided.
- `bento-pp-cli experimental create-gender` — This endpoint allows you to guess the gender of a subscriber based on their first name and last name.
- `bento-pp-cli experimental create-validation` — This experimental endpoint provides basic validation for email addresses and associated user information.
- `bento-pp-cli experimental list` — This experimental endpoint allows you to check if a domain or IP address is present on various blacklists.
- `bento-pp-cli experimental list-geolocation` — This endpoint allows you to geolocate an IP address.

**fetch** — Manage fetch

- `bento-pp-cli fetch create` — This endpoint allows you to execute various commands to modify subscriber data.
- `bento-pp-cli fetch create-fields` — Creates a new custom field in your account. Custom fields allow you to store additional data about your subscribers.
- `bento-pp-cli fetch create-subscribers` — Creates a new subscriber in your account and queues them for indexing.
- `bento-pp-cli fetch create-tags` — Creates a new tag in your account for subscriber segmentation.
- `bento-pp-cli fetch list` — Fetches a list of all broadcasts in your account, providing an overview of your email campaigns.
- `bento-pp-cli fetch list-fields` — Fetches a list of all custom fields defined in your account.
- `bento-pp-cli fetch list-search` — This endpoint allows for advanced searching of your subscriber base.
- `bento-pp-cli fetch list-subscribers` — Fetch detailed information about one or multiple subscribers in your account.
- `bento-pp-cli fetch list-tags` — Fetches a list of all tags in your account. Tags are useful for segmenting your subscriber base.

**stats** — Retrieve statistical data about your account. IMPORTANT — Never use these endpoints in client-side code.

- `bento-pp-cli stats list` — Fetches statistics for a specific segment in your account.
- `bento-pp-cli stats list-site` — Fetches overall statistics for your site. This includes counts of users, subscribers, and unsubscribers.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bento-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pre-import list hygiene before any cold outreach

```bash
bento hygiene scrub --in cold-leads.csv --out-clean clean.csv --out-rejected rejected.csv --json
```

Chains validation + jesses_ruleset + blacklist + content_moderation in one pipeline so a dirty list never tanks deliverability.

### Vendure -> Bento purchase event with dedupe

```bash
bento events purchase-replay --from vendure-orders.json --skip-already-tracked --json --select 'events[].email,events[].details.unique.key'
```

Maps Vendure order JSON to Bento $purchase shape, validates dedupe keys, and emits the batch payload. --select narrows the deep nested response to just the fields agents need to confirm.

### Find lapsed VIPs for a win-back broadcast

```bash
bento subscribers winback --lapsed 180d --last-purchased 90d --tagged vip --csv > winback.csv
```

Three-way local JOIN that no Bento segment builder supports.

### Audit before a GDPR batch deletion

```bash
bento subscribers pre-delete --emails-from gdpr-batch.txt --json
```

Shows revenue, active automations, and tag exposure for each email before you queue irreversible anonymization.

### Catch silently-broken workflows via tag drift

```bash
bento tags drift --window 7d --threshold 30 --json
```

Surfaces tags whose subscriber counts shifted more than 30% week-over-week.

## Auth Setup

Bento uses HTTP Basic with publishable_key:secret_key, plus a site_uuid attached as ?site_uuid= on GETs and as a body field on POSTs. Set BENTO_PUBLISHABLE_KEY, BENTO_SECRET_KEY, and BENTO_SITE_UUID, or run `bento auth set` to store them in ~/.config/bento. The CLI sets a descriptive User-Agent automatically -- generic UAs are 403'd by Cloudflare.

Run `bento-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bento-pp-cli experimental list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bento-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bento-pp-cli feedback --stdin < notes.txt
bento-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bento-pp-cli/feedback.jsonl`. They are never POSTed unless `BENTO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BENTO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bento-pp-cli profile save briefing --json
bento-pp-cli --profile briefing experimental list
bento-pp-cli profile list --json
bento-pp-cli profile show briefing
bento-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bento-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add bento-pp-mcp -- bento-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bento-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bento-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bento-pp-cli <command> --help`.

## Differences from the official Bento CLI

This CLI is an independent reimplementation. Compared to Bento's own `@bentonow/bento-cli` and `bento-mcp`, it adds:

- Transactional email send (Ruby/PHP SDK only — not in the official CLI or MCP)
- Batch commands API (`/fetch/commands` with `subscribe`/`unsubscribe`/`add_tag`) — only in the Python SDK upstream
- Segments list/get endpoints — docs-only upstream
- Every experimental endpoint (validation, jesses_ruleset, blacklist, content_moderation, gender, geolocation)
- Data deletion requests endpoint (GDPR/CCPA)
- A local SQLite store with FTS5 search across subscribers/tags/fields/broadcasts
- 12 novel commands (snapshot diff, hygiene pipeline, purchase replay, churn-risk scoring, broadcast what-if, etc.) that have no upstream equivalent
- Single Go binary — no Node runtime required
- MCP server auto-derived from the Cobra command tree (every CLI command is reachable to agents)

## Known Gaps

These are limits of the Bento API itself, not of this CLI. None of them can be solved by an API wrapper:

- **No broadcast send/schedule/stop endpoint.** Broadcasts are dashboard-only — you can list and read them, but cannot trigger them via API.
- **No workflow create/edit/pause endpoint.** Same — dashboard-only.
- **No sequence email reorder/delete endpoint.** Edit individual emails inside a sequence in the dashboard.
- **No standalone email-template listing.** Templates are only accessible nested under broadcasts and sequences.
- **No event history endpoint.** Bento has `POST /batch/events` to write but no list endpoint to read event history back.
- **Bulk subscriber listing requires the Enterprise tier.** Mitigated locally via `subscribers fetch-batch` (per-email pull) and `subscribers import-csv` (load a dashboard CSV export into the local store).
- **No subscriber DELETE.** Use the GDPR `/data_deletion_requests` endpoint to anonymize.
- **No documented webhook signature verification.** Bento does not publish a signing scheme for inbound webhooks.
