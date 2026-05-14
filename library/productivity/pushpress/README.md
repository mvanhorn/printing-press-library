# PushPress CLI

**Every PushPress /v3 endpoint that exists, plus the going-dark report and the trainer-dashboard one-liner the API itself can't answer.**

Wraps PushPress's /v3 Platform API (members, check-ins, app installs, webhooks, messages) with a Go binary that caches every entity in local SQLite and adds the cross-entity views the API doesn't: `going-dark` for daily churn audits, `roster` for the trainer dashboard, `kpi today` for the business cockpit, `member` for coach prep. Five categories absent from /v3 (plans/billing, classes, leads, signup-source, tasks/notes, cancellations) ship as honest gap-flagged commands that point at the documented /v2 follow-up rather than silently dropping.

Printed by [@i2Fitness](https://github.com/i2Fitness) (Alex Puckhaber).

## Install

The recommended path installs both the `pushpress-pp-cli` binary and the `pp-pushpress` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pushpress
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pushpress --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pushpress-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pushpress --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pushpress --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pushpress skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pushpress. The skill defines how its required CLI can be installed.
```

## Authentication

Authenticates with a PushPress API key. Set it once with `pushpress-pp-cli auth set-token <key>` or `export PUSHPRESS_API_KEY=<key>`. Every request sends `Authorization: Bearer <key>`. Some endpoints also accept an optional `companyId` HEADER for tenant scoping.

## Quick Start

```bash
# Save your PushPress API key once.
pushpress-pp-cli auth set-token <your-api-key>


# Verify the key reaches the API and identify which company the key is scoped to.
pushpress-pp-cli doctor


# Populate the local store so going-dark and roster have data.
pushpress-pp-cli sync --full


# The headline churn report — members who haven't checked in for 14 days.
pushpress-pp-cli going-dark --days 14 --json


# One-line metric ticker for a business-dashboard cron.
pushpress-pp-cli kpi today --json

```

## Unique Features

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

## Usage

Run `pushpress-pp-cli --help` for the full command reference and flag list.

## Commands

### apps

manage PushPress appl ecosystem

- **`pushpress-pp-cli apps get`** - Get details of a specific app
- **`pushpress-pp-cli apps list`** - List all available apps

### checkins

Manage checkins

- **`pushpress-pp-cli checkins get`** - Get a check-in by ID
- **`pushpress-pp-cli checkins list`** - Get a list of all check-ins

### company

Manage company

- **`pushpress-pp-cli company get`** - Get company details associated with the API key

### customers

Manage customers

- **`pushpress-pp-cli customers get`** - Get a customer by ID
- **`pushpress-pp-cli customers list`** - Get a list of all customers in a

### keys

Manage keys

- **`pushpress-pp-cli keys create-api`** - Create a new API key for a company.
- **`pushpress-pp-cli keys delete-api`** - Permanently delete an API key from the system.
- **`pushpress-pp-cli keys get-api`** - Retrieve a single API key.
- **`pushpress-pp-cli keys list-api`** - List all active API keys for a client.

### messages

Manage messages

- **`pushpress-pp-cli messages send-email`** - Send an email
- **`pushpress-pp-cli messages send-ping`** - Send a ping notification via Ably Realtime
- **`pushpress-pp-cli messages send-push`** - Send a push notification

### webhooks

create and configure webhooks for PushPress events

- **`pushpress-pp-cli webhooks create`** - Create a new webhook to subscribe to one or more events
- **`pushpress-pp-cli webhooks delete`** - Delete a specific webhook
- **`pushpress-pp-cli webhooks get`** - Get details of a specific webhook
- **`pushpress-pp-cli webhooks list`** - List all registered webhooks
- **`pushpress-pp-cli webhooks update`** - Update the URL or events for an existing webhook


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pushpress-pp-cli apps list

# JSON for scripting and agents
pushpress-pp-cli apps list --json

# Filter to specific fields
pushpress-pp-cli apps list --json --select id,name,status

# Dry run — show the request without sending
pushpress-pp-cli apps list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pushpress-pp-cli apps list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pushpress -g
```

Then invoke `/pp-pushpress <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add pushpress pushpress-pp-mcp -e PUSHPRESS_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pushpress-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PUSHPRESS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pushpress": {
      "command": "pushpress-pp-mcp",
      "env": {
        "PUSHPRESS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
pushpress-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pushpress-platform-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PUSHPRESS_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pushpress-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PUSHPRESS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — Regenerate the API key in the PushPress developer portal and re-run `pushpress-pp-cli auth set-token`.
- **`plans list` / `mrr today` / `cancellations recent` print 'not supported by /v3'** — Expected — these categories aren't in the public /v3 API. Run the documented /v2 browser-sniff follow-up to unlock them.
- **Empty going-dark or recency output** — Run `pushpress-pp-cli sync --full` (or the `pushpress-hydrate` companion script) to populate the local store. Note: `/v3/checkins` returns 404 in production, so check-in-driven transcendence stays empty until that's resolved — see Known Gaps below.

---

## Known Gaps

Documented limitations from this generator run. Verified live against the real /v3 API.

**1. Five must-have categories aren't in `/v3`.** Per the build briefing's gap-flag protocol, these ship as explicit stub commands that print "not supported by /v3" and document the /v2 follow-up:

| Stub command | What's missing | Follow-up |
|---|---|---|
| `plans list`, `plans members`, `mrr today` | Plans + billing + MRR | Browser-sniff `/v2/plans`, `/v2/billing`, `/v2/subscription` |
| `signups recent` | Documented Customer schema lacks `dateAdded` (real schema includes `membershipDetails.initialMembershipStartDate` — see #3) | Re-derivable from rich customer payload OR /v2/activity |
| `cancellations recent` | No cancellation/freeze surface | Browser-sniff `/v2/billing` Churn report |
| `classes list`, `classes roster` | No class-definition endpoint | Browser-sniff `/v2/calendar` |
| `leads list` | No lead surface | Browser-sniff `/v2/client` filtered by lead status |
| `tasks list`, `notes list` | No task/notes surface | Browser-sniff `/v2/task`, `/v2/communications` |
| `cohort` | Originally a transcendence row; downgraded to stub when discovered the spec lacks `dateAdded` | Re-elevable once `/v2` lands or schema-drift is exploited |

**2. `/v3/checkins` returns 404 on production.** Verified live: `curl -H "API-KEY: <key>" https://api.pushpress.com/v3/checkins` → `404 Cannot GET /checkins`. The Speakeasy spec documents this endpoint family but it doesn't actually serve. Effect: the CLI exposes `checkins list`/`checkins get` and the entire check-in-driven transcendence layer (`going-dark`, `recency`, `kpi today` check-in counts, `class-mix`, `member 360` cadence trend), but all those reads return empty against real data. Workaround: wait for `/v3/checkins` to come back, OR pivot to `/v2/calendar/report/no-show/*` via the documented /v2 browser-sniff follow-up.

**3. Real `/v3/customers` schema is richer than the Speakeasy spec admits.** Live `customers list --json` reveals fields the spec doesn't document: `membershipDetails.initialMembershipStartDate` (signup date), `assignedToStaffId` (coach assignment), `role` (status indicator), structured `name` (first/last/nickname), `gender`, `dob`, `emergencyContact`. The CLI exposes the raw JSON; agents can query via `json_extract(data, '$.field.path')` or `--select`. The bundled `pushpress-hydrate` script handles the dict→string conversion for the promoted `name` and `address` columns.

**4. `sync --full` only enumerates a subset of resources.** Same press-side gap as [Printing Press issue #1355](https://github.com/mvanhorn/cli-printing-press/issues/1355). Workaround: the `pushpress-hydrate` script (Python, installed alongside this binary) hits `/v3/customers` directly and populates the local store so the transcendence commands have data. Will retire once #1355 lands.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**speakeasy-sdks/pushpress-typescript-sdk**](https://github.com/speakeasy-sdks/pushpress-typescript-sdk) — TypeScript
- [**PushPress/pushpress-ts**](https://github.com/PushPress/pushpress-ts) — TypeScript
- [**@pushpress/pushpress (npm)**](https://www.npmjs.com/package/@pushpress/pushpress) — TypeScript
- [**Zapier PushPress integration**](https://zapier.com/apps/pushpress/integrations) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
