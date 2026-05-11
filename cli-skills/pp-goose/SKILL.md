---
name: pp-goose
description: "Run your goose.pet facility from the terminal — today's roster, customer lookup, vaccine warnings, and bulk CSV... Trigger phrases: `what's the roster today at the facility`, `who is checking in today`, `look up customer at goose`, `check this pet's vaccinations`, `pull goose weekly reports`, `use goose`, `run goose`."
author: "Corey Pensky"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - goose-pp-cli
---

# Goose — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `goose-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install goose --cli-only
   ```
2. Verify: `goose-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Goose has no public API and no third-party CLI. This one wraps the admin surface the web app uses: bookings, pets, customers, schedules, and the 16 data-export endpoints behind the Reports page. Sign in once via Chrome and the CLI manages Cognito refresh on its own; novel commands like `today`, `vaccines expiring --by-visit`, and `alerts daily` join across the local store for answers the web app can't give in one click.

## When to Use This CLI

Use this CLI when you operate a goose.pet facility and want scriptable access to the admin surface. Daily-ops checks (today's roster, alerts), customer/pet lookups during off-hours support, bulk weekly CSV exports for the analyst, and cross-entity questions (vaccines × bookings, churn × vouchers) that the web app can't answer in one click.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Auth & reachability
- **`auth login --chrome`** — Sign in once by importing your goose.pet Cognito session from Chrome localStorage; subsequent commands auto-refresh the access token.

  _First-run setup. Without it, the CLI is unusable beyond an hour. After it, the CLI behaves like any tokened SaaS CLI._

  ```bash
  goose auth login --chrome
  ```

### Daily ops
- **`today`** — Today's arrivals, departures, and currently-here pets — with vaccine-expired, agreement-missing, and balance-due flags surfaced from the booking's join graph.

  _The morning huddle in one call. Agents picking 'who is at the facility right now' get a single pre-joined payload instead of orchestrating four endpoints._

  ```bash
  goose today --json --select arrivals.pet.displayName,arrivals.warnings
  ```
- **`alerts daily`** — Single command surfaces operational risk for today: incoming pets with expired vaccinations, customers without an active agreement revision, checkouts with non-zero outstanding balance, and customer vouchers expiring within 7 days.

  _The 7am pre-open check — agents or scripts can decide what needs attention before customers walk in._

  ```bash
  goose alerts daily --json
  ```

### Lookup
- **`customer`** — Search-and-resolve in one command: takes a name, phone, or email, returns the full customer record with pets, vouchers, agreements, payments, notes, balance, and recent bookings.

  _Off-hours support calls — answer 'what's the deal with this customer' from the terminal without four browser clicks._

  ```bash
  goose customer 'Pat Smith' --json
  ```
- **`pet`** — Find a pet by name and return tags, vaccinations, instructions, current room assignment, owner, and upcoming bookings in one render.

  _Quickest answer to 'is Riley current on shots?' or 'where is Riley right now?' for front-desk and on-call._

  ```bash
  goose pet Riley --json
  ```

### Local state that compounds
- **`vaccines expiring`** — Pets with vaccinations expiring within N days. With --by-visit, filter to pets that also have an upcoming booking inside the window.

  _Front-desk wants the lapses that will actually arrive this week, not the full backlog._

  ```bash
  goose vaccines expiring --within 30d --by-visit --json
  ```
- **`churn`** — Customers who haven't booked in N days, optionally restricted to those holding unused vouchers/credits.

  _Marketing re-engagement list in one command instead of Excel-stitching three CSV exports._

  ```bash
  goose churn --not-booked-since 60d --has-voucher --json
  ```

### Reporting
- **`reports run-all`** — Parallel fan-out over the 16 documented data-export endpoints (sales, customer-activity, agreements, feeding-medication, vaccinations, etc.) for a date range, written to ./reports/<week>/<slug>.csv.

  _Weekly analytics pull in one command. Agents that ingest sales data don't have to re-implement export orchestration._

  ```bash
  goose reports run-all --week 2026-W19 --json
  ```

## Command Reference

**balances** — Outstanding balances

- `goose-pp-cli balances` — Bulk lookup of outstanding balances for a set of customers

**bookings** — Bookings / reservations / invoices — the core unit of a facility visit

- `goose-pp-cli bookings get` — Get a single booking by invoice ID
- `goose-pp-cli bookings list` — List bookings with filters and rich relation includes

**contracts** — Service agreements / contracts

- `goose-pp-cli contracts` — List contracts (e.g. service agreements)

**conversations** — Goose-native conversations (separate from embedded Intercom messaging)

- `goose-pp-cli conversations` — List conversation threads

**customers** — Customers — location-user-profile (customer at this facility)

- `goose-pp-cli customers get` — Get a customer by ID with full relation graph
- `goose-pp-cli customers search` — List customers with includes (note: full-text search lives on search-api.goose.pet — see `customer` novel command)

**dashboard** — Today's roster — arrivals, departures, currently-here pets, with pet tags, room assignments, feeding instructions, and warnings

- `goose-pp-cli dashboard` — List today's bookings with arrivals, departures, in-stay, and full pet/owner detail

**notes** — Customer / pet notes

- `goose-pp-cli notes` — List notes for a customer (or facility-wide)

**payment_methods** — Stored payment methods (v1 and v2)

- `goose-pp-cli payment_methods` — List stored payment methods (v2 endpoint)

**reports** — Report catalog and CSV exports

- `goose-pp-cli reports export` — Download a single CSV/data-export report by slug (e.g. feeding-medication-export, sales-export,...
- `goose-pp-cli reports get` — Get a single report type by name slug
- `goose-pp-cli reports list` — List all report types available at this facility

**services** — Location service types — boarding, daycare, grooming, etc.

- `goose-pp-cli services` — List service types offered by this facility

**species** — Species + breeds catalog

- `goose-pp-cli species` — List species and breeds offered at this facility

**staff** — Staff and other facility resources (rooms, kennels, yards)

- `goose-pp-cli staff availability` — Get resource availability for a date range
- `goose-pp-cli staff list` — List staff / resources

**vouchers** — Customer vouchers — package credits and cash credits

- `goose-pp-cli vouchers` — List vouchers (filter by customer, type, status)


**Hand-written commands**

- `goose-pp-cli auth login` — Import Goose.pet session tokens from Chrome localStorage (Cognito refresh-token capture); use --chrome to bind to...
- `goose-pp-cli today` — Composite of today's arrivals/departures/here with vaccine, agreement, and balance warnings
- `goose-pp-cli customer` — One-shot customer lookup: search-api → detail with all includes + balance
- `goose-pp-cli pet` — One-shot pet lookup: FTS → pet detail with tags, vaccinations, instructions, owner, upcoming bookings
- `goose-pp-cli vaccines expiring` — Pets with expiring vaccinations; --by-visit filters to those with upcoming bookings
- `goose-pp-cli churn` — Customers who haven't booked in N days; optional --has-voucher overlay
- `goose-pp-cli reports run-all` — Parallel fan-out over all CSV exports for a given week
- `goose-pp-cli alerts daily` — Daily operational risk panel: expired vaccines, missing agreements, balance due, voucher expiry


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
goose-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning huddle JSON

```bash
goose today --json --select arrivals.pet.displayName,arrivals.pet.tags,arrivals.warnings,departures.pet.displayName,departures.warnings
```

Project just the names + warnings for arrivals/departures into a slim payload an agent or shell script can render.

### This-week vaccinations to call about

```bash
goose vaccines expiring --within 14d --by-visit --json --select pet.displayName,owner.displayName,owner.phone,vaccine.expirationDate
```

Names and phones for the call list — already filtered to pets with upcoming bookings.

### Analyst weekly pull

```bash
goose reports run-all --week 2026-W19
```

Fans out all 16 CSV exports for one ISO week into ./reports/2026-W19/.

### Churn re-engagement with credit overlay

```bash
goose churn --not-booked-since 60d --has-voucher --json --select displayName,email,phone,lastBookedAt,availableVouchers
```

Customers who haven't booked in 60 days but still hold unused vouchers — the highest-conversion re-engagement segment.

## Auth Setup

Goose auth is AWS Cognito (User Pool us-east-2_IqPUw1L4C). The CLI handles this with `goose auth login --chrome`: it reads your existing app.goose.pet session tokens from Chrome's localStorage, persists the refresh token, and mints fresh 1-hour access tokens via Cognito InitiateAuth as needed. If you'd rather not use Chrome, paste an access token: `export GOOSE_ACCESS_TOKEN=<jwt>` (re-paste every hour).

Run `goose-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  goose-pp-cli bookings list --agent --select id,name,status
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
goose-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
goose-pp-cli feedback --stdin < notes.txt
goose-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.goose-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOSE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOSE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
goose-pp-cli profile save briefing --json
goose-pp-cli --profile briefing bookings list
goose-pp-cli profile list --json
goose-pp-cli profile show briefing
goose-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `goose-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add goose-pp-mcp -- goose-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which goose-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   goose-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `goose-pp-cli <command> --help`.
