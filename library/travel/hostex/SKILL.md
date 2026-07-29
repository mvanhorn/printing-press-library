---
name: pp-hostex
description: "Every Hostex v3 operation, plus a local SQLite mirror that answers cross-property questions no single Hostex call can — occupancy gaps, revenue rollups, and guest-message SLA, offline. Trigger phrases: `check my Hostex reservations`, `which guests check in this week`, `update listing prices on Hostex`, `reply to a guest message`, `revenue by property this month`, `use hostex`, `run hostex`."
author: "bust011r"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hostex-pp-cli
    install:
      - kind: go
        bins: [hostex-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/hostex/cmd/hostex-pp-cli
---

# Hostex — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hostex-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install hostex --cli-only
   ```
2. Verify: `hostex-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostex/cmd/hostex-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Hostex has a REST API and an official MCP server but no first-class CLI. This one mirrors all 86 endpoints as typed commands with --json/--select/--dry-run, then syncs reservations, properties, listings, conversations, reviews, tasks and transactions into local SQLite so you can run joins the API can't: which occupied stays have no cleaning task, revenue by property this month, threads unanswered past your SLA, and price-parity gaps across channels. It gets the tricky Hostex plumbing right once: every response is HTTP 200 with error_code in the body, and the multi-layer rate limits return Retry-After.

## When to Use This CLI

Reach for this CLI when an agent or script needs to read or mutate a Hostex host's data: triaging the guest inbox, querying reservations by date/property/status, pushing price/inventory/restriction updates across channels, scheduling cleaning tasks, or rolling up revenue. It is the right tool when the question spans multiple entities (stays + tasks + transactions) because the local SQLite mirror answers those joins offline; a single REST call cannot.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to build a multi-tenant SaaS that acts on behalf of many hosts — that needs the OAuth 2.0 partner flow, not a single access token.
- Do not use it as a real-time webhook receiver; it manages webhook subscriptions but does not host an inbound endpoint.
- Do not use it to bulk-message guests faster than the per-thread throttle allows; channel OTAs will suspend the account.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Operations: stop problems before guests do
- **`ops-gaps`** — Find imminent or occupied stays with no cleaning task or missing check-in details.

  _Reach for this to catch turnover and check-in problems before a guest complains, instead of eyeballing the calendar._

  ```bash
  hostex-pp-cli ops-gaps --within 7d --agent
  ```
- **`stay-brief`** — One dossier for a single stay: reservation, guest, thread state, tasks, transactions, and review.

  _Use when you need the full picture of one stay; to scan many stays for problems use ops-gaps._

  ```bash
  hostex-pp-cli stay-brief HMABC123 --agent
  ```

### Inbox and automation SLA
- **`inbox-sla`** — Rank open guest conversations by age since the last guest message and flag threads past your SLA.

  _OTAs penalize slow replies; use this to triage which threads are about to breach SLA._

  ```bash
  hostex-pp-cli inbox-sla --breach 6h --agent
  ```
- **`automation-preview`** — List pending automated messages and reviews and flag any whose thread a human has already handled.

  _Use to vet queued bot actions before they fire on a human-handled thread; for slow human threads use inbox-sla._

  ```bash
  hostex-pp-cli automation-preview --day tomorrow --agent
  ```

### Channel parity and revenue (portfolio)
- **`price-parity`** — Flag property-dates where per-channel listing price or min-stay diverges across channels.

  _Use to catch silent revenue loss from channel price drift; for availability mismatch use oversell-watch._

  ```bash
  hostex-pp-cli price-parity --property 12345 --days 30 --agent
  ```
- **`oversell-watch`** — Flag dates a channel still shows bookable on a property that is blocked or booked on the master calendar.

  _Use to catch double-sell risk; for price or min-stay drift use price-parity._

  ```bash
  hostex-pp-cli oversell-watch --days 30 --agent
  ```
- **`revenue-rollup`** — Net income minus expense by property or month over a date range, from the live ledger.

  _Use for the Monday portfolio revenue review in one command._

  ```bash
  hostex-pp-cli revenue-rollup --by property --month 2026-06 --agent
  ```

  The scan stops after `--max-pages` pages of 100 ledger entries (default 50, so 5,000). When the range holds more than that, the output sets `truncated: true` and a warning goes to stderr rather than quietly understating the totals — re-run with a larger `--max-pages` or a narrower range.

## Command Reference

**automation** — Scheduled automation actions (e.g. automated guest messages and scheduled host reviews).

- `hostex-pp-cli automation delete-action` — Removes a **waiting** message or review automation plan without running it (same as deleting an upcoming action in the
- `hostex-pp-cli automation execute-action` — Dispatches a **waiting** message or review automation plan immediately (same behavior as executing an upcoming action
- `hostex-pp-cli automation query-actions` — Returns scheduled automation **actions** that are waiting to run: either automated **messages** (`type=message`

**availabilities** — Manage availabilities

- `hostex-pp-cli availabilities query` — By sending a request to this endpoint, you can retrieve the availabilities of the properties.
- `hostex-pp-cli availabilities update` — Use this endpoint to update property availabilities.

**calendar-share-links** — Public share links that expose a read-only calendar (and reservation list) of all or selected properties to anyone holding the link.

- `hostex-pp-cli calendar-share-links create` — Create a new public calendar share link.
- `hostex-pp-cli calendar-share-links delete` — Permanently invalidate a calendar share link.
- `hostex-pp-cli calendar-share-links query` — List the operator's public calendar share links.

**channel-accounts** — Manage channel accounts

- `hostex-pp-cli channel-accounts` — Query the third-party channel accounts (Airbnb, Booking.com, etc.) that the operator has connected.

**conversations** — Manage conversations

- `hostex-pp-cli conversations get-details` — This endpoint is used to retrieve the messages and details of a conversation.
- `hostex-pp-cli conversations query` — This endpoint is used to query the list of conversations regarding guest inquiries.
- `hostex-pp-cli conversations send-message` — Send a text or image message to the guest.

**custom-channels** — Manage custom channels

- `hostex-pp-cli custom-channels` — Query custom channels created from the [Custom Options Page](https://hostex.io/app/settings/custom-options).

**expense-items** — Manage expense items

- `hostex-pp-cli expense-items` — Query the dictionary of expense item categorizations available to the operator.

**expense-methods** — Manage expense methods

- `hostex-pp-cli expense-methods` — Query the dictionary of payment methods available to the operator for expense entries.

**groups** — Manage groups

- `hostex-pp-cli groups create` — Create a new property group. Optionally pre-attach properties at creation time via `property_ids`.
- `hostex-pp-cli groups delete` — Delete a property group.
- `hostex-pp-cli groups query` — You can query property groups by making a request to this endpoint.
- `hostex-pp-cli groups update` — Update a property group.

**income-items** — Manage income items

- `hostex-pp-cli income-items` — Query the dictionary of income item categorizations available to the operator.

**income-methods** — Manage income methods

- `hostex-pp-cli income-methods` — Query the dictionary of payment methods available to the operator for income entries.

**knowledge-bases** — AI knowledge base entries for the HostGPT automation assistant. Each entry defines content and the scope (properties/channels) where it applies.

- `hostex-pp-cli knowledge-bases create` — Create a new knowledge base entry for the HostGPT automation assistant.
- `hostex-pp-cli knowledge-bases delete` — Delete a knowledge base entry by its ID.
- `hostex-pp-cli knowledge-bases get` — Retrieve the full details of a single knowledge base entry by its ID.
- `hostex-pp-cli knowledge-bases query` — You can query knowledge base entries by making a request to this endpoint.
- `hostex-pp-cli knowledge-bases update` — Replace an existing knowledge base entry.

**listings** — Manage listings

- `hostex-pp-cli listings get-airbnb-price-and-rules` — Fetch the current pricing, availability rules and booking settings of an Airbnb listing in real time from Airbnb.
- `hostex-pp-cli listings get-vrbo-price-and-rules` — Get the pricing and rules of a Vrbo listing as currently recorded by Hostex.
- `hostex-pp-cli listings query` — Query the listings (third-party properties) synced from the operator's connected channel accounts.
- `hostex-pp-cli listings query-calendars` — By sending a request to this endpoint, you can retrieve calendar information for multiple listings.
- `hostex-pp-cli listings update-airbnb-price-and-rules` — Update the listing-level pricing, fees, booking settings and availability rules of an Airbnb listing.
- `hostex-pp-cli listings update-inventories` — Update the inventories of channel listings.
- `hostex-pp-cli listings update-prices` — Update the prices of channel listings.
- `hostex-pp-cli listings update-restrictions` — Update the restrictions of channel listings.
- `hostex-pp-cli listings update-vrbo-price-and-rules` — Update the listing-level pricing, fees and booking rules of a Vrbo listing.

**oauth** — Manage oauth

- `hostex-pp-cli oauth obtain-token` — This endpoint is used to obtain a new access token using various OAuth 2.0 grant types or refresh an existing token.
- `hostex-pp-cli oauth revoke-token` — This endpoint allows clients to revoke an access or refresh token.

**pricing-ratios** — Manage pricing ratios

- `hostex-pp-cli pricing-ratios` — Return the per-channel pricing ratio of each OTA listing linked to a property (`property_id`)

**properties** — Manage properties

- `hostex-pp-cli properties create-property` — Create a new property (room) under the current operator.
- `hostex-pp-cli properties query` — You can query properties by making a request to this endpoint.

**reservation-tags** — Manage the operator's reservation tag dictionary (the tags that can be applied to reservations).

- `hostex-pp-cli reservation-tags create` — Create a new reservation tag in the operator's dictionary. Color is auto-assigned from the Hostex palette.
- `hostex-pp-cli reservation-tags delete` — Delete one of the operator's custom reservation tags.
- `hostex-pp-cli reservation-tags query` — List the operator's reservation tag dictionary.

**reservations** — Manage reservations

- `hostex-pp-cli reservations cancel` — Cancel a direct booking reservation in Hostex.
- `hostex-pp-cli reservations create` — Create a reservation (Direct Booking) in Hostex.
- `hostex-pp-cli reservations query` — You can query reservations by making a request to this endpoint.
- `hostex-pp-cli reservations update-basic-info` — Update basic information of a stay including guest details, dates, pricing, and other attributes.

**reviews** — Manage reviews

- `hostex-pp-cli reviews create` — Create review or reply for a reservation.
- `hostex-pp-cli reviews query` — Query reviews like the [Reviews Page](https://hostex.io/app/reviews).

**room-types** — Manage room types

- `hostex-pp-cli room-types create` — Create a new room type under the current operator.
- `hostex-pp-cli room-types query` — You can query room types by making a request to this endpoint.

**staffs** — Manage staffs

- `hostex-pp-cli staffs create` — Create a schedule staff. The staff is created as active by default.
- `hostex-pp-cli staffs delete` — Delete a staff permanently along with their property assignments.
- `hostex-pp-cli staffs query` — You can query schedule staffs (cleaners / operators / receptionists, etc.) by making a request to this endpoint.
- `hostex-pp-cli staffs update` — Update an existing staff. All fields are optional; only the supplied fields are changed.

**tags** — Manage tags

- `hostex-pp-cli tags create` — Create a new property tag. Optionally pre-attach properties via `property_ids` and / or room types via `room_type_ids`.
- `hostex-pp-cli tags delete` — Delete a property tag.
- `hostex-pp-cli tags query` — You can query tags by making a request to this endpoint.
- `hostex-pp-cli tags update` — Update a property tag.

**tasks** — Schedule tasks such as cleaning, maintenance, reception, housekeeping and others.

- `hostex-pp-cli tasks create` — Create a schedule task.
- `hostex-pp-cli tasks delete` — Delete a task permanently. Returns 404 if the task does not exist or is not accessible to the current operator.
- `hostex-pp-cli tasks query` — You can query schedule tasks (cleaning / maintain / reception / housekeeping / others)
- `hostex-pp-cli tasks update` — Update an existing task. All fields are optional; only the supplied fields are changed.

**transactions** — Manage transactions

- `hostex-pp-cli transactions create` — Record a new income or expense entry.
- `hostex-pp-cli transactions delete` — Delete a transaction entry. The operation is irreversible.
- `hostex-pp-cli transactions query` — Query income and expense entries (also known as `transactions`) recorded against the operator
- `hostex-pp-cli transactions update` — Update an existing transaction entry. Only the fields listed below can be modified.

**webhooks** — Manage webhooks

- `hostex-pp-cli webhooks create` — Create a webhook.
- `hostex-pp-cli webhooks delete` — You can only delete webhooks created by your own app if they are manageable.
- `hostex-pp-cli webhooks query` — Query Webhooks like the [Webhooks Page](https://hostex.io/app/api/web-hooks).
- `hostex-pp-cli webhooks update` — Update the url or event subscriptions for a webhook. You can only update webhooks created by your own app.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hostex-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Project only what you need from a verbose reservation list

```bash
hostex-pp-cli reservations query --agent --select data.reservations.stay_code,data.reservations.guest_name,data.reservations.check_in_date,data.reservations.status
```

Reservation payloads are large and deeply nested; --select narrows to the fields an agent needs so it doesn't burn context.

### Dry-run a price push before sending it

```bash
hostex-pp-cli listings update-prices --dry-run
```

Shows the request body that would be sent to the async price endpoint without mutating any channel.

### Offline search across synced guest threads

```bash
hostex-pp-cli search "refund" --type conversations --db ./hostex.db
```

After sync, full-text search runs locally with no API call and no rate-limit cost.

### Confirm a revenue rollup is complete before reporting it

```bash
hostex-pp-cli revenue-rollup --by month --from 2026-01-01 --to 2026-12-31 --max-pages 200 --agent
```

The ledger scan is capped so a high-volume account can't pin the CLI. Read `truncated` before trusting the numbers: `true` means the range outran the cap and income, expense and net are all understated. Raise `--max-pages` or narrow the range until it reads `false`.

## Auth Setup

Hostex authenticates with a Hostex-Access-Token header. Create one in the Host Portal (OpenAPI Settings) with read-only or writable scope; tokens do not expire. Set it as HOSTEX_ACCESS_TOKEN. The server also accepts Authorization: Bearer, but prefer the dedicated header. A read-only token rejects every write with error_code 401.

Run `hostex-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `HOSTEX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `HOSTEX_CONFIG_DIR`, `HOSTEX_DATA_DIR`, `HOSTEX_STATE_DIR`, `HOSTEX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `HOSTEX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `hostex-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "hostex": {
        "command": "hostex-pp-mcp",
        "env": {
          "HOSTEX_HOME": "/srv/hostex"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `HOSTEX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `HOSTEX_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
hostex-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hostex-pp-cli feedback --stdin < notes.txt
hostex-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `HOSTEX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOSTEX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hostex-pp-cli profile save briefing --json
hostex-pp-cli --profile briefing availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15
hostex-pp-cli profile list --json
hostex-pp-cli profile show briefing
hostex-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `hostex-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/hostex/cmd/hostex-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hostex-pp-mcp -- hostex-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hostex-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hostex-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hostex-pp-cli <command> --help`.
