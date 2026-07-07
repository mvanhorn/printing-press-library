# Bento CLI

**Every Bento feature, plus the ones their own CLI does not have, plus a local SQLite store that turns 'who are my top spenders by tag' into a one-liner.**

A Go binary CLI for Bento email marketing that covers all 39 features from Bento's official CLI and MCP, brings in transactional send + spam validation + segments that are SDK-only or docs-only today, and layers 12 novel commands on top -- snapshot diff, hygiene pipeline, Vendure purchase replay, churn-risk scoring, broadcast what-if. Offline FTS over your subscribers via SQLite, --json on every command, MCP tools auto-derived from the Cobra tree.

Created by [@bossriceshark](https://github.com/bossriceshark) (bossriceshark).

## Install

The recommended path installs both the `bento-pp-cli` binary and the `pp-bento` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bento
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bento --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bento --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bento --agent claude-code
npx -y @mvanhorn/printing-press-library install bento --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bento/cmd/bento-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bento-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bento --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bento --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bento --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bento --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bento-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BENTO_PUBLISHABLE_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bento": {
      "command": "bento-pp-mcp",
      "env": {
        "BENTO_PUBLISHABLE_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Bento uses HTTP Basic with publishable_key:secret_key, plus a site_uuid attached as `?site_uuid=` on GETs and as a body field on POSTs. Export the three env vars below in your shell (or your `.envrc`/`.env` file) — every command reads them automatically:

```bash
export BENTO_PUBLISHABLE_KEY=pk_...
export BENTO_SECRET_KEY=sk_...
export BENTO_SITE_UUID=...
```

Get the keys from the Bento dashboard under Settings -> API Keys, and the site UUID under Account -> Site Settings. Verify with `bento doctor`. The CLI sets a descriptive User-Agent automatically — generic UAs are 403'd by Bento's Cloudflare edge.

## Quick Start

```bash
# Verify auth, base URL, User-Agent, and live API reachability.
bento doctor

# Snapshot Bento into the local SQLite store. Required before any novel command.
bento sync --resources subscribers,tags,fields,broadcasts

# Local FTS5 search across notes/fields with structured filters.
bento subscribers find "vip" --tagged customer --json --select email,tags

# Run the full validation pipeline before any cold import.
bento hygiene scrub --in leads.csv --out-clean clean.csv --out-rejected rejected.csv

# Map a Vendure order export to Bento $purchase events; preview before sending.
bento events purchase-replay --from vendure-orders.json --dry-run --json
```

## Unique Features

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

## Usage

Run `bento-pp-cli --help` for the full command reference and flag list.

## Commands

Run `bento-pp-cli <command> --help` for full flag and argument details on any command below.

### Subscribers and Tags

| Command | Purpose |
| --- | --- |
| `bento subscribers find <query>` | Local FTS5 search across subscriber notes/fields with structured filters (`--tagged`, `--purchased-after`, `--csv`) |
| `bento subscribers churn-risk` | Rank subscribers by days-since-last-engagement against your cohort baseline |
| `bento subscribers winback` | Build a CSV of one-time buyers who lapsed N days and stopped opening |
| `bento subscribers pre-delete` | Preview tags, events, revenue, and active automations before a GDPR delete |
| `bento subscribers fetch-batch` | Pull subscribers by email and populate the local store (non-Enterprise path) |
| `bento subscribers import-csv` | Import a Bento dashboard CSV export into the local subscriber store |
| `bento tags drift` | Tags whose subscriber counts swung more than N% in the window |

### Sync, Search, Analytics (local store)

| Command | Purpose |
| --- | --- |
| `bento sync --resources <list>` | Snapshot Bento into local SQLite (subscribers, tags, fields, broadcasts, sequences, workflows, templates, segments) |
| `bento sync diff --since <ts>` | Diff two local snapshots — Bento has no `since` filter, so this is the only way to detect drift |
| `bento search <query>` | FTS5 across all synced resources (live API fallback when available) |
| `bento analytics --type <resource> --group-by <field>` | Count and group-by over synced data |
| `bento tail [resource] --interval 10s` | Poll the API and stream changes as NDJSON to stdout |

### Events (Vendure bridge + hygiene)

| Command | Purpose |
| --- | --- |
| `bento events purchase-replay --from vendure-orders.json` | Map a Vendure order export to Bento `$purchase` events; preview before sending |
| `bento events review-window --shipped-csv orders.csv --delay 10d` | Emit `$review_request` events for orders shipped N days ago, with `--skip-stamped` deduping |
| `bento events lint --file events.jsonl` | Catch payloads that use `/fetch/commands subscribe` when the intent needs `/batch/events` to fire automations |

### Email Hygiene and Validation

| Command | Purpose |
| --- | --- |
| `bento hygiene scrub --in leads.csv --out-clean clean.csv --out-rejected rejected.csv` | Chain validation + jesses_ruleset + blacklist + content_moderation in one pass |
| `bento experimental create-validation` | Email validity heuristics for a single address |
| `bento experimental list <domain-or-ip>` | Check blacklists (domain or IP, not both) |
| `bento experimental create` | Content moderation score for a short string |
| `bento experimental create-gender` | Guess gender from first + last name (US Census data) |
| `bento experimental list-geolocation` | Geolocate an IP address |

### Broadcasts, Fields, and Campaign Planning

| Command | Purpose |
| --- | --- |
| `bento broadcasts whatif --segment <id>` | Estimate audience size, predicted opens, and hygiene-risk count for a draft |
| `bento fields lint --against vendure-schema.json` | Diff local custom fields against a Vendure schema and flag missing, renamed, or type-mismatched fields |

### Live API (fetch and batch)

| Command | Purpose |
| --- | --- |
| `bento fetch list` | List broadcasts |
| `bento fetch list-subscribers` | Fetch one or more subscribers by email |
| `bento fetch list-fields` / `list-tags` | List custom fields / tags |
| `bento fetch list-search` | Enterprise-only advanced subscriber search |
| `bento fetch create-subscribers` / `create-fields` / `create-tags` | Create individual resources |
| `bento fetch create` | Execute subscriber commands: `add_tag`, `remove_tag`, `add_field`, `remove_field`, `subscribe`, `unsubscribe`, `change_email`, `add_tag_via_event` |
| `bento batch create` | Up to 100 broadcasts per request |
| `bento batch create-emails` | Up to 60 priority-queued transactional emails per request |
| `bento batch create-events` | Up to 1,000 events per request (auto-creates users) |
| `bento batch create-subscribers` | Bulk subscriber upserts via Bento's import queues (no Flow side effects) |

### Stats

| Command | Purpose |
| --- | --- |
| `bento stats list-site` | User / subscriber / unsubscriber counts for the site |
| `bento stats list` | Same counts for a specific segment (server-side only — never client-side) |

### Account and Workflow

| Command | Purpose |
| --- | --- |
| `bento auth setup` / `set-token` / `status` / `logout` | Manage credentials |
| `bento doctor` | Verify auth, base URL, User-Agent, and live API reachability |
| `bento workflow archive` / `status` | One-shot sync of every resource for offline use; show archive state |
| `bento profile save` / `list` / `use` / `show` / `delete` | Save and reuse named flag sets |
| `bento import <resource> --input data.jsonl` | Import data from a JSONL file by issuing per-record POSTs |

### Discovery and Utilities

| Command | Purpose |
| --- | --- |
| `bento which "<capability>"` | Natural-language lookup of the command that implements a capability |
| `bento agent-context` | Machine-readable JSON describing commands, flags, and auth (for runtime agent introspection) |
| `bento feedback "<note>"` | Capture friction locally at `~/.bento-pp-cli/feedback.jsonl` (opt-in upstream POST via `--send`) |
| `bento version` | Print version |
| `bento completion <shell>` | Generate shell completion scripts |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bento-pp-cli experimental list

# JSON for scripting and agents
bento-pp-cli experimental list --json

# Filter to specific fields
bento-pp-cli experimental list --json --select id,name,status

# Dry run — show the request without sending
bento-pp-cli experimental list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bento-pp-cli experimental list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Cookbook

Verified-flag recipes that combine the CLI's offline store, agent-mode, and Vendure bridge with shell tools an operator already uses.

### 1. Daily refresh

```bash
# Pull every domain table into the local SQLite store, then sanity-check the archive.
bento sync --resources subscribers,tags,fields,broadcasts,sequences,workflows,templates,segments
bento workflow status --json | jq '{ts, resources: [.resources[] | {name, count, last_synced}]}'
```

### 2. Weekly drift check before a broadcast

```bash
# Catch tags whose subscriber counts shifted >25% week-over-week (silently-broken automations).
bento tags drift --window 7d --threshold 25 --json | jq '.tags[] | select(.delta_pct | fabs > 50)'

# Audience size + hygiene-risk preview for the broadcast you're about to queue.
bento broadcasts whatif --segment vip-loyalty --json --select audience_size,predicted_opens,hygiene_risk_count
```

### 3. Cold-list onboarding pipeline

```bash
# 1. Hygiene scrub. Clean rows go forward; rejected get logged.
bento hygiene scrub --in raw-leads.csv --out-clean clean.csv --out-rejected rejected.csv --rate 1.5

# 2. Convert the clean CSV to JSONL (one subscriber object per line).
tail -n +2 clean.csv | awk -F, '{printf "{\"email\":\"%s\"}\n", $1}' > clean.jsonl

# 3. Stream into Bento via the per-record import path.
bento import subscribers --input clean.jsonl
```

### 4. Vendure -> Bento purchase events with preview

```bash
# Inspect the mapping shape and dedupe keys without sending anything.
bento events purchase-replay --from vendure-orders.json --json \
  --select 'events[].email,events[].details.unique.key'

# Once happy, flip the switch.
bento events purchase-replay --from vendure-orders.json --send
```

### 5. Vendure-side review trigger (10 days after ship)

```bash
# Daily cron: emit $review_request only for orders shipped 10 days ago today;
# --skip-stamped honors the local dedupe table so your existing review platform's mail never overlaps.
bento events review-window --shipped-csv orders.csv --delay 10d --skip-stamped --send
```

### 6. Win-back cohort export

```bash
# One-time buyers who lapsed 180d and stopped opening — straight to CSV for ad platforms.
bento subscribers winback --lapsed 180d --last-purchased 90d --csv > winback.csv
wc -l winback.csv
```

### 7. GDPR pre-delete audit

```bash
# Show tags, events, revenue, and active automations before queuing a data-deletion batch.
bento subscribers pre-delete --emails-from gdpr-batch.txt --json | jq '.subjects[] | {email, revenue, active_automations}'
```

### 8. Schema lint after a Vendure customer-schema change

```bash
# Catch renamed or type-mismatched custom fields before the next purchase event misfires.
bento fields lint --against vendure-schema.json --json
```

### 9. Search and ad-hoc cohorting

```bash
# Local FTS across notes and custom fields with structured filters.
bento subscribers find "lifetime warranty" --tagged customer --purchased-after 365d --csv > cohort.csv

# Run analytics over the local store without touching the API.
bento analytics --type subscribers --group-by source --limit 20 --json
```

### 10. Reusable invocations via profiles

```bash
# Save the agent-mode flag set as a named profile (flags on the save invocation are captured).
bento profile save agent --json --compact --no-color --no-input
# Apply with --profile on any future call.
bento subscribers find "vip" --profile agent
```

### 11. Agent discovery

```bash
# Resolve natural-language intent to the right command.
bento which "find lapsed customers"
bento which "import a CSV"
# Emit machine-readable command map for runtime introspection.
bento agent-context --pretty
```

## Health Check

```bash
bento-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bento-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BENTO_PUBLISHABLE_KEY` | per_call | Yes | Basic auth username. Starts with pk_. Available in your Bento dashboard under Settings -> API Keys. |
| `BENTO_SECRET_KEY` | per_call | Yes | Basic auth password. Starts with sk_. Available in your Bento dashboard under Settings -> API Keys. Never expose client-side. |
| `BENTO_SITE_UUID` | per_call | Yes | Bento site identifier. Attached to every request as ?site_uuid= on GETs and in the JSON body on POSTs. Available in your Bento dashboard under Account -> Site Settings. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bento-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BENTO_PUBLISHABLE_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 with Cloudflare HTML body** — User-Agent header is missing or generic. `bento doctor` verifies UA is set correctly; the CLI hard-sets it automatically.
- **400 'site_uuid required'** — Run `bento auth status` to confirm BENTO_SITE_UUID is set. The CLI attaches site_uuid to every request automatically; this error means auth is misconfigured.
- **401 'Author not authorized'** — The from-email on `bento emails send` or `bento broadcasts create` is not a pre-authorized Author. Add the address in the Bento dashboard under Account -> Authors.
- **429 rate-limited** — Bento has no Retry-After header. The CLI implements client-side exponential backoff automatically. For batch ops, lower --batch-size or add --pace 60/min.
- **subscriber import returns null then is missing** — Imports are async (1-5 min). Use `bento subscribers get <email> --wait 5m` to poll, or `bento sync --resources subscribers` and re-search locally.

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

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bento-cli**](https://github.com/bentonow/bento-cli) — TypeScript
- [**bento-mcp**](https://github.com/bentonow/bento-mcp) — TypeScript
- [**bento-node-sdk**](https://github.com/bentonow/bento-node-sdk) — TypeScript
- [**bento-ruby-sdk**](https://github.com/bentonow/bento-ruby-sdk) — Ruby
- [**bento-python-sdk**](https://github.com/bentonow/bento-python-sdk) — Python
- [**bento-php-sdk**](https://github.com/bentonow/bento-php-sdk) — PHP
- [**n8n-nodes-bento**](https://www.npmjs.com/package/n8n-nodes-bento) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
