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

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

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
- `hostex-pp-cli conversations send-message` — Send a text or image message to the guest. **This endpoint is not idempotent and does not return a message ID.

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

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Project only what you need from a verbose reservation list

```bash
hostex-pp-cli reservations query --start-check-in-date 2026-11-01 --end-check-in-date 2026-11-30 --agent --select data.reservations.stay_code,data.reservations.guest_name,data.reservations.check_in_date,data.reservations.status
```

Reservation payloads are large and deeply nested; --select narrows to the fields an agent needs so it doesn't burn context. The API returns HTTP 400 unless both check-in dates are given.

### Dry-run a price push before sending it

```bash
hostex-pp-cli listings update-prices --channel-type airbnb --listing-id 1234567890 --prices '[{"start_date":"2026-11-01","end_date":"2026-11-30","price":180}]' --dry-run
```

Shows the request body that would be sent to the async price endpoint without mutating any channel. Prices are integers in the listing currency; confirm the applied result in the Host Portal afterwards.

### Offline search across synced guest threads

```bash
hostex-pp-cli search "refund" --type conversations --db ./hostex.db
```

After sync, full-text search runs locally with no API call and no rate-limit cost.

### Read channel calendars before repricing

```bash
hostex-pp-cli listings query-calendars --start-date 2026-11-01 --end-date 2026-11-30 --listings '[{"listing_id":"1234567890","channel_type":"airbnb"}]' --agent
```

A read that happens to use POST, so it is safe to run live. Compare the current per-day prices with `pricing-ratios` (per-channel percentage over the lead channel) before pushing changes; the property base price itself can only be edited in the Host Portal.

## Auth Setup

Hostex authenticates with a Hostex-Access-Token header. Create one in the Host Portal (OpenAPI Settings) with read-only or writable scope; tokens do not expire. Run `echo "$TOKEN" | hostex-pp-cli auth set-token` (the token is read from stdin) or export HOSTEX_ACCESS_TOKEN. The server also accepts Authorization: Bearer, but prefer the dedicated header. A read-only token rejects every write with error_code 401.

Run `hostex-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --agent --select data,error_code,error_msg
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
hostex-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "hostex-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `hostex-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `hostex-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `hostex-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
hostex-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
hostex-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
hostex-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
hostex-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`hostex-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `HOSTEX_NO_LEARN=true` in the environment globally disables the pipeline.

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
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

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

If `--wait` times out or polling fails after the job was accepted, the command exits `8` and prints the job ID plus a recovery command that fetches the result. The job may still be running and may already be billed, so run the recovery command instead of resubmitting.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 8 | Async job submitted but not finished (run the printed recovery command; do not resubmit) |
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
