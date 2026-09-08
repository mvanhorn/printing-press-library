---
name: pp-keyword-planner
description: Collect Google Keyword Planner ideas and historical monthly metrics, inspect immutable snapshots, and export receipt-linked evidence offline. Use for Google Keyword Planner or keyword-planner-pp-cli requests.
author: Max Michel
license: Apache-2.0
argument-hint: "ideas | historical | portfolio | doctor"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - keyword-planner-pp-cli
---

# Google Keyword Planner — Printing Press CLI

## Installation and first check

Use `keyword-planner-pp-cli` from PATH. Its default evidence portfolio is `~/.local/share/keyword-planner/snapshots.db`. Follow **Prerequisites: Install the CLI** below.

```bash
keyword-planner-pp-cli --version
keyword-planner-pp-cli doctor --json
```

## When to Use This CLI

Use this CLI when an agent needs Google Keyword Planner ideas or historical monthly metrics with explicit language, geography, range, and network context. Use the local portfolio commands to reopen evidence offline, trace a value to its raw receipt, compare saved collections, preserve variant lineage, or prepare a safe DuckDB handoff. Treat the returned volumes and bids as Planner signals for YouTube subject research, not as measured YouTube demand, RPM, or revenue.

## Anti-triggers

Do not use this press for campaign or bid changes, account administration, Google Trends, automatic scoring, forecasts, schedules, or revenue predictions. Planner estimates are not measured YouTube demand or RPM.

## Workflow

1. Choose six to eight seeds at the subject's level. Avoid replacing the subject with narrow title wording.
2. Specify language and geo resources. Inspect the complete request with `--dry-run` if needed.
3. Use `ideas` to discover related terms, then `historical` for selected terms.
4. Inspect saved coverage before interpretation. Follow snapshot IDs and preserve short-window, null, and ambiguity flags.
5. Use local portfolio commands for trace, comparison, export, and safe descriptive statistics. Reach Google again only for an explicitly requested fresh collection.

## Auth Setup

Authentication uses the OAuth refresh-token flow with GOOGLE_ADS_CLIENT_ID, GOOGLE_ADS_CLIENT_SECRET, GOOGLE_ADS_REFRESH_TOKEN, and GOOGLE_ADS_DEVELOPER_TOKEN loaded from ~/.env. GOOGLE_ADS_LOGIN_CUSTOMER_ID is the optional manager login header, while GOOGLE_ADS_CUSTOMER_ID is the separately resolved operating account; the approved OAuth grant carries the required AdWords scope, the CLI refreshes a short-lived access token in memory, and sends Authorization: Bearer plus developer-token and, when applicable, login-customer-id. Token responses are never stored in the portfolio, and an invalid grant, permission or developer-token error, or customer routing failure is an account-setup problem rather than a reason to loop retries.

Run `keyword-planner-pp-cli doctor` to verify setup.

## Command Reference

| Command | Use |
|---|---|
| `keyword-planner-pp-cli ideas` | Collect all idea pages |
| `keyword-planner-pp-cli historical` | Collect chosen terms in bounded batches |
| `keyword-planner-pp-cli portfolio list` | List saved snapshots |
| `keyword-planner-pp-cli portfolio show` | Inspect saved requests and receipts |
| `keyword-planner-pp-cli portfolio search` | Filter stored monthly evidence |
| `keyword-planner-pp-cli portfolio export` | Emit local JSON or CSV |
| `keyword-planner-pp-cli doctor` | Inspect local readiness; live checks require the explicit live flag |

Snapshot examples below use example IDs that must be replaced with IDs from your own portfolio. Run `keyword-planner-pp-cli portfolio list --agent` to choose compatible IDs in another portfolio; `latest` and `previous` can refer to different endpoints.

## Unique Capabilities

These local evidence views extend the direct API with receipt, coverage, lineage, comparison, integrity, and descriptive-statistics views.

### Evidence that compounds
- **`portfolio trace`** — Trace one stored monthly value back to the exact request and raw receipt that produced it.

  _Reach for this when a number needs a receipt-backed explanation that a direct Planner call cannot provide._

  ```bash
  keyword-planner-pp-cli portfolio trace 2c2f465c-6253-430a-a533-aa30fabbfbff --keyword "beef steak" --month 2026-07 --agent
  ```
- **`portfolio coverage`** — See requested, returned, missing, unavailable, raw-only, and incomplete coverage for a saved collection.

  _Use this before consuming a collection when the API response alone cannot explain what was absent or incomplete._

  ```bash
  keyword-planner-pp-cli portfolio coverage latest --agent
  ```
- **`portfolio diff`** — Compare two preserved collections to see changed terms, monthly values, flags, and coverage.

  _Use this when the question is what changed between saved collections rather than what one live call returns._

  ```bash
  keyword-planner-pp-cli portfolio diff 2c2f465c-6253-430a-a533-aa30fabbfbff c6ea4780-190b-42f4-8f4d-a8b7207a2395 --agent
  ```

### Safe warehouse handoff
- **`portfolio variants`** — Follow submitted keywords through returned terms and close variants without double-counting demand.

  _Pick this when a Planner result must be interpreted with its returned-term and close-variant relationships intact._

  ```bash
  keyword-planner-pp-cli portfolio variants latest --limit 10 --agent --json
  ```
- **`portfolio integrity`** — Verify that raw receipts, hashes, normalized rows, and coverage still reconcile.

  _Run this before a warehouse handoff when evidence integrity matters more than a fresh remote request._

  ```bash
  keyword-planner-pp-cli portfolio integrity latest --agent
  ```
- **`portfolio safe-stats`** — Compute descriptive monthly aggregates while excluding unavailable, ambiguous, anomalous, and incomplete rows.

  _Choose this for mechanical local summaries that must preserve warnings and avoid turning Planner signals into forecasts._

  ```bash
  keyword-planner-pp-cli portfolio safe-stats --snapshot latest --agent
  ```

## Recipes

### Check local readiness

```bash
keyword-planner-pp-cli doctor --json
```

Confirm local setup and portfolio schema without a network call.

### Collect subject-level ideas

```bash
keyword-planner-pp-cli ideas --seed "beef steak" --seed "ribeye steak" --seed "beef brisket" --seed "pork chops" --seed "meat cutting" --seed "butchery" --language languageConstants/1000 --geo geoTargetConstants/2840 --agent
```

Use explicit resource names and preserve all pages for the six-to-eight-seed research pass.

### Retrieve historical wording

```bash
keyword-planner-pp-cli historical --keyword "beef steak" --keyword "beef brisket" --language languageConstants/1000 --geo geoTargetConstants/2840 --agent
```

Keep the submitted terms, returned terms, variant links, and every monthly value.

### Compare preserved collections

```bash
keyword-planner-pp-cli portfolio diff 2c2f465c-6253-430a-a533-aa30fabbfbff c6ea4780-190b-42f4-8f4d-a8b7207a2395 --agent
```

Compare two compatible snapshots selected from `portfolio list` without replacing either or calling Google again.

### Export a DuckDB-ready slice

```bash
keyword-planner-pp-cli portfolio export --snapshot latest --agent --select keyword,month,monthly_searches
```

Emit a narrow, traceable local relation for the read-only warehouse consumer.

### Preview a complete collection request

```bash
keyword-planner-pp-cli historical --keyword "beef steak" --keyword "beef brisket" --language languageConstants/1000 --geo geoTargetConstants/2840 --dry-run --json
```

Inspect resolved targeting and the closed-month range without reading credentials, contacting Google, or writing the portfolio.

### Export one closed month as CSV

```bash
keyword-planner-pp-cli portfolio export --snapshot 2c2f465c-6253-430a-a533-aa30fabbfbff --start 2026-07 --end 2026-07 --format csv
```

Keep the month bound explicit and export local evidence for a spreadsheet.

### Inspect a small available search slice

```bash
keyword-planner-pp-cli portfolio search "beef steak" --snapshot 2c2f465c-6253-430a-a533-aa30fabbfbff --available-only --limit 10 --json
```

Show available saved monthly values with their flags and provenance; availability alone is not the safe-aggregate policy.

## Authentication

Live collection loads only the Google Ads bindings from `~/.env`, with explicit environment overrides. Keep that file private. Access tokens exist only in memory; token responses and credential values never enter the portfolio or command output.

| Binding | Purpose |
|---|---|
| `GOOGLE_ADS_CLIENT_ID` | OAuth client for the approved grant |
| `GOOGLE_ADS_CLIENT_SECRET` | OAuth client secret |
| `GOOGLE_ADS_REFRESH_TOKEN` | Approved refresh grant carrying the AdWords scope |
| `GOOGLE_ADS_DEVELOPER_TOKEN` | Google Ads developer-token header |
| `GOOGLE_ADS_CUSTOMER_ID` | Explicit operating customer target |
| `GOOGLE_ADS_LOGIN_CUSTOMER_ID` | Optional manager login context |

The operating target and manager context are separate bindings. Equal values are allowed after the target has been verified; manager status alone does not establish eligibility. Currency comes from a live account lookup and retains its provenance. Use `doctor --live` to check the configured target and both Planner methods. The default doctor is offline.

## Data policy

Use six to eight seeds at the subject's level. The API accepts one to twenty, so other valid counts produce a warning. Language and geography must be explicit official resource names. The only network is `GOOGLE_SEARCH`. Empty geography needs explicit all-geographies acknowledgement.

The default range is the latest twelve closed UTC months; explicit ranges override it. Missing months and missing CPC are unavailable, never inferred zero. A short returned window remains usable. Current/future request months are rejected; unexpected open months remain raw-only. Google's trailing twelve-month average and snapshot-level bid micros retain their original meaning.

Every response is saved before decoding. Every repeat creates a new snapshot. Errors and interrupted collections remain visible and incomplete. Local output limits do not reduce collection scope. Ambiguous zero/UNSPECIFIED metrics remain inspectable with `PLANNER-BLIND` and are excluded from safe aggregates. See [DATA-CONTRACT.md](DATA-CONTRACT.md) for null, month, variant, currency, and coverage rules.

## Troubleshooting

- `invalid_grant`: the grant needs inspection or renewed authorization; repeating the same invalid grant will not repair it.
- Permission, developer-token, or customer errors: inspect the separately configured target and manager context, and the provider's specific error code. Do not create or link accounts automatically.
- Quota errors: use bounded retries only for transient throttling. Local pacing coordinates processes on this host; it does not coordinate other hosts or represent account-wide remaining quota.
- Empty data: check coverage and exact request scope. A valid empty response has no inferred suppression cause.
- No August data in an early September response: inspect coverage; a closed month need not already be available from Google.

## Agent Mode

Use `--agent` for structured output or `--json` for complete JSON. Use `--select` for a narrow field set. Empty lists remain `[]`. Do not interpret absent values as zero. Collection limits and display limits have different meanings; an incomplete collection is never accepted as a complete result.

```bash
keyword-planner-pp-cli portfolio export --snapshot latest --json --select keyword,month,monthly_searches
keyword-planner-pp-cli portfolio coverage latest --agent
```

Read `agent-context` or command help for runtime truth. Local learning/profile helpers use their own generated store; they do not replace a fresh collection or alter the evidence portfolio. Keep credentials out of taught queries, command text, logs, and reports.

## Public distribution reference

Use the Printing Press installer below, or build a checkout from source as described in README.md.

<details>
<summary>Generator-owned public installer reference</summary>

## Prerequisites: Install the CLI

This skill drives the `keyword-planner-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install keyword-planner --cli-only
   ```
2. Verify: `keyword-planner-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/cmd/keyword-planner-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

</details>

The CLI collects keyword ideas and historical monthly metrics through Google's read-only v25 REST methods, preserving every raw response, request scope, coverage state, and supplied bid field in SQLite. Use the portfolio commands to trace a number to its receipt, compare preserved snapshots, keep close-variant lineage visible, and calculate policy-safe descriptive statistics offline. Planner search-volume and advertiser-bid fields are vendor estimates for subject research, not measured YouTube demand, YouTube RPM, or a revenue forecast.
