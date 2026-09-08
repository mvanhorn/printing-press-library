# Google Keyword Planner CLI

**Turn Google Keyword Planner reads into an immutable local evidence portfolio for offline research and safe warehouse handoff.**

The CLI collects keyword ideas and historical monthly metrics through Google's read-only v25 REST methods, preserving every raw response, request scope, coverage state, and supplied bid field in SQLite. Use the portfolio commands to trace a number to its receipt, compare preserved snapshots, keep close-variant lineage visible, and calculate policy-safe descriptive statistics offline. Planner search-volume and advertiser-bid fields are vendor estimates for subject research, not measured YouTube demand, YouTube RPM, or a revenue forecast.

## Install

Install the CLI from the public catalogue:

```bash
npx -y @mvanhorn/printing-press-library install keyword-planner --cli-only
keyword-planner-pp-cli --version
keyword-planner-pp-cli doctor --json
```

The catalogue's binary is named `keyword-planner-pp-cli`. Ensure the install directory is on PATH. To build a checkout from source, use Go 1.26.6 or newer:

```bash
go build -o build/keyword-planner-pp-cli ./cmd/keyword-planner-pp-cli
./build/keyword-planner-pp-cli --version
```

## Quick Start

Configure your Google Ads credentials as described below before collecting. The database starts empty. After your first collection, use `portfolio list` to obtain snapshot IDs; replace the example IDs in analytical recipes. Dated examples illustrate historical requests—choose a closed month available in your snapshot.

```bash
# Verify the local configuration and portfolio schema without network activity or credentials.
keyword-planner-pp-cli doctor --json

# Collect subject-level ideas with explicit language and geography while preserving every result page.
keyword-planner-pp-cli ideas --seed "beef steak" --seed "ribeye steak" --seed "beef brisket" --seed "pork chops" --seed "meat cutting" --seed "butchery" --language languageConstants/1000 --geo geoTargetConstants/2840 --agent

# Retrieve the chosen wording's monthly history and keep submitted, returned, and variant relationships.
keyword-planner-pp-cli historical --keyword "beef steak" --keyword "beef brisket" --language languageConstants/1000 --geo geoTargetConstants/2840 --agent

# Check returned and missing coverage before using a saved collection.
keyword-planner-pp-cli portfolio coverage latest --agent

# Export a narrow, traceable relation for the read-only DuckDB consumer.
keyword-planner-pp-cli portfolio export --snapshot latest --agent --select keyword,month,monthly_searches

```

## Unique Features

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

## Authentication

Authentication uses the OAuth refresh-token flow with GOOGLE_ADS_CLIENT_ID, GOOGLE_ADS_CLIENT_SECRET, GOOGLE_ADS_REFRESH_TOKEN, and GOOGLE_ADS_DEVELOPER_TOKEN loaded from ~/.env. GOOGLE_ADS_LOGIN_CUSTOMER_ID is the optional manager login header, while GOOGLE_ADS_CUSTOMER_ID is the separately resolved operating account; the approved OAuth grant carries the required AdWords scope, the CLI refreshes a short-lived access token in memory, and sends Authorization: Bearer plus developer-token and, when applicable, login-customer-id. Token responses are never stored in the portfolio, and an invalid grant, permission or developer-token error, or customer routing failure is an account-setup problem rather than a reason to loop retries.

## Configuration

Planner credentials default to `~/.env`, a private regular file. Environment bindings override values in that file; explicit command flags override the corresponding path or customer binding.

| Binding | Purpose |
|---|---|
| `GOOGLE_ADS_CLIENT_ID` | OAuth application's client ID |
| `GOOGLE_ADS_CLIENT_SECRET` | OAuth application's protected client secret |
| `GOOGLE_ADS_REFRESH_TOKEN` | Approved grant used to obtain an in-memory access token |
| `GOOGLE_ADS_DEVELOPER_TOKEN` | Google Ads developer access token |
| `GOOGLE_ADS_CUSTOMER_ID` | Operating customer; `--customer-id` overrides it |
| `GOOGLE_ADS_LOGIN_CUSTOMER_ID` | Optional manager login context, resolved separately |
| `KEYWORD_PLANNER_ENV_FILE` | Override credential-file location; `--env-file` takes precedence |
| `KEYWORD_PLANNER_DB` | Override portfolio location; `--db` takes precedence |
| `KEYWORD_PLANNER_CALL_LEDGER` | Optional private local file recording Ads request attempts |
| `KEYWORD_PLANNER_CONFIG` | Generated framework's separate `config.json` location; not the Planner credential file |
| `KEYWORD_PLANNER_BASE_URL` | Retained generated test-client setting; it does not redirect the curated Planner transport |

The old `KEYWORD_PLANNER_ACCESS_TOKEN` environment reader is retired. Use the approved refresh-token bindings. Shared Printing Press learning and profile state is separate from the Planner portfolio; its runtime locations are available through `agent-context` and command help.

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

### API-specific
- **OAuth returns invalid_grant** — Inspect the approved OAuth grant and protected bindings in `~/.env`; after correcting the grant, run `keyword-planner-pp-cli doctor --live`. Do not retry the same invalid grant.
- **USER_PERMISSION_DENIED or CUSTOMER_NOT_ENABLED** — Verify that GOOGLE_ADS_CUSTOMER_ID names the operating account and GOOGLE_ADS_LOGIN_CUSTOMER_ID is only the manager context, then rerun `keyword-planner-pp-cli doctor --live` after access is corrected.
- **DEVELOPER_TOKEN_NOT_APPROVED or DEVELOPER_TOKEN_PROHIBITED** — Use a developer token approved for the operating account and rerun `keyword-planner-pp-cli doctor --live`; do not switch providers or retry a permanently rejected token.
- **A request uses an invalid language or geo resource name** — Pass a verified `languageConstants/<id>` and `geoTargetConstants/<id>` value; resolve labels before the live call.
- **The Planner request is rate-limited or daily quota is exhausted** — Keep Planner calls at no more than one request per second per operating customer, allow bounded transient backoff, and stop on persistent daily-quota errors.
- **The requested range ends in the current or a future month** — Set the end month to a closed UTC calendar month; the CLI rejects open and future ranges before network activity.

## Commands

| Command | Result |
|---|---|
| `ideas` | Full keyword-idea collection, including every result page |
| `historical` | Chosen-keyword monthly metrics, preserving input/variant linkage |
| `portfolio list` | Snapshot inventory |
| `portfolio show` | Request context, raw receipts, rows, and coverage |
| `portfolio search` | Filtered local evidence |
| `portfolio export` | Offline JSON or CSV rows |
| `doctor` | Offline local readiness; explicit live mode checks Google |

Use `keyword-planner-pp-cli --help` and each command's help for flags. The source also retains local Printing Press learning/profile helpers; their store is separate from the evidence portfolio.

`--home` relocates the default portfolio and `.env` paths, including quoted `~/...` paths; explicit `--db`, `--env-file`, and their environment overrides take precedence. Planner pacing files remain in the shared user cache so separate portfolio homes cannot independently exceed the customer rate limit. Planner calls use a positive `--timeout` (default 60 seconds). `--rate-limit 0.5` slows calls to one every two seconds; the default is one per second, and disabling or exceeding that ceiling is rejected. Seed and keyword files above 8 MiB fail explicitly before collection. CSV exports honor `--select` even when no rows match.

## Agent Usage

Add `--agent` for structured non-interactive output, or `--json` for complete JSON. Use `--select` to retain specific fields and `--csv` for tabular export. Diagnostics go to stderr; list-shaped empty JSON is `[]`. Use `--dry-run` on a complete collection request to inspect its resolved nonsecret body without a Google call.

Evidence commands retain metric values, eligibility flags and provenance in agent and compact output. Use their explicit `--limit` and `--select` controls to narrow the rendered result. Normal terminal collection and inventory output uses readable tables; piped output remains JSON.

```bash
keyword-planner-pp-cli agent-context --pretty
keyword-planner-pp-cli portfolio export --snapshot latest --json --select keyword,month,monthly_searches
```

The generated MCP command mirror uses the same curated CLI paths and evidence policy. Raw endpoint mirrors are disabled. The local validation does not deploy an MCP server. `tools-manifest.json` retains two hidden upstream endpoint references in the Printing Press schema; `mcp-runtime-tools.json` preserves the 28-tool runtime inventory and full input schemas. The running stdio server is authoritative for tool discovery.

The verified MCP transport is stdio. The optional generated HTTP server still needs an upstream header-read timeout before a later HTTP deployment; this generator issue is recorded in the archived review.

## Health Check

```bash
keyword-planner-pp-cli doctor --json
keyword-planner-pp-cli doctor --live --json
```

The first command is offline. The second intentionally uses the configured OAuth grant and Google Ads read methods.

Illustrative fields from the publication-time check; your local readiness may differ:

```json
{
  "mode": "offline",
  "database_status": "ok",
  "network": "not contacted",
  "auth": "not checked"
}
```

## Portfolio and DuckDB

The evidence database is `~/.local/share/keyword-planner/snapshots.db`. [sql/keyword_monthly.sql](sql/keyword_monthly.sql) provides the standalone DuckDB read-only attachment and compatibility relation. It preserves original warehouse columns plus provenance and casts dates explicitly. Combined geographies and unresolved mappings do not enter the single-country relation. The warehouse's registry and loaders remain separate.

Run DuckDB from the directory containing `snapshots.db`, then load the SQL recipe with `.read /absolute/path/to/keyword_monthly.sql`. Alternatively, edit only the attachment path in the recipe. Keep `READ_ONLY` enabled.

## Sources & Inspiration

- [Google Ads v25 Discovery](https://googleads.googleapis.com/$discovery/rest?version=v25)
- [Google historical metrics guide](https://developers.google.com/google-ads/api/docs/keyword-planning/generate-historical-metrics)
- [Google Ads Printing Press](https://github.com/mvanhorn/printing-press-library/tree/60351630b3d94c016faa7bfb65102e109da086c9/library/marketing/google-ads): read-command and header reference.
- [google-ads-python](https://github.com/googleads/google-ads-python): official request/response behavior reference.

Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), with scoped Google auth, evidence, and command-policy extensions recorded in `.printing-press-patches/`. Apache-2.0; retain LICENSE and NOTICE.

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
