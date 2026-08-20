---
name: pp-peerspace
description: "Tech-event venue scout for Peerspace: search, shortlist, hydrate full listing details (about/rules/parking), check calendar availability, message hosts (inquiry-send), verify guest status, and export Luma/Eventbrite packs. Trigger phrases: `find tech meetup venues`, `hydrate Peerspace shortlist`, `message Peerspace host`, `contact listing owner`, `venue availability`, `export Peerspace shortlist for Luma`, `search Peerspace venues`, `use peerspace`, `run peerspace-pp-cli`."
author: "nspage"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - peerspace-pp-cli
    install:
      - kind: go
        bins: [peerspace-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/peerspace/cmd/peerspace-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/peerspace/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Peerspace — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `peerspace-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install peerspace --cli-only
   ```
2. Verify: `peerspace-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/peerspace/cmd/peerspace-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Peerspace has no public API. peerspace-pp-cli wraps live venue search and listing detail behind Surf transport, saves favorites boards, hydrates full listing page blocks into SQLite (about, rules, parking, amenities), checks calendar availability, prices and sends host inquiries, and exports shortlists for Eventbrite/Luma/Slack.

## When to Use This CLI

Use when planning tech meetups, workshops, or community events: scout venues by headcount/budget/date, build a favorites board, hydrate deep listing detail, check day/month availability, message hosts about staffing or fit, score amenity gaps, and hand off shortlists to Luma/Eventbrite workflows.

## Anti-triggers

Do not use this CLI for:
- Do not place or pay for bookings — checkout/payment is not supported
- Do not manage host-side listings or host calendar tools beyond guest availability reads
- Do not auto-hydrate every search hit — call `shortlist hydrate` only after shortlisting
- Do not send host inquiries without explicit user approval (`inquiry-send` requires `--yes`)
- Do not expect a stable official API contract; endpoints are reverse-engineered from site traffic

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Market intelligence
- **`scout budget`** — Band market listings by hourly/day rate so you can see what a city+activity actually costs.

  _Use when planning a budget before deep-diving individual venues._

  ```bash
  peerspace-pp-cli scout budget --city Paris --activity meetup --band 50 --json
  ```
- **`scout capacity`** — Histogram of guest capacity for a synced market so you can re-cut when headcount changes.

  _Use when guest count is the binding constraint, not price._

  ```bash
  peerspace-pp-cli scout capacity --city Paris --activity meetup --band 10 --json
  ```
- **`markets pulse`** — Cross-city rollup of median price, listing density, instant-book share, and capacity quantiles.

  _Use when comparing host cities for the same event type._

  ```bash
  peerspace-pp-cli markets pulse --city Paris --city Lyon --activity meetup --json
  ```
- **`markets neighborhoods`** — Per-neighborhood listing stats plus optional --vibe tech keyword signals from descriptions (WiFi, projector, transit) and a built-in tech-cluster map.

  _Use to pick tech-friendly neighborhoods before refining a shortlist._

  ```bash
  peerspace-pp-cli markets neighborhoods --city Paris --activity meetup --vibe tech --json
  ```

### Shortlist intelligence
- **`shortlist compare`** — Join your favorites board to listing attributes for a sortable offline comparison table.

  _Use when deciding among saved venues without reopening browser tabs._

  ```bash
  peerspace-pp-cli shortlist compare --sort price --json
  ```
- **`shortlist create-board`** — Create a Peerspace favorite board and attach a listing (cookie auth).

  _Use when starting a new shortlist board from a space you want to save._

  ```bash
  peerspace-pp-cli shortlist create-board --name "Q3 meetup shortlist" --listing-id 68d468bb44492187e415d4a6 --activity Meetup --location "Paris, France" --json
  ```
- **`shortlist add`** — Favorite a space by attaching its listing id to an existing favorite board (cookie auth).

  _Use when saving a listing onto a board you already have without opening the website._

  ```bash
  peerspace-pp-cli shortlist add --listing-id 68d468bb44492187e415d4a6 --board-id 669152994300a86e4a943da5 --json
  ```
- **`shortlist hydrate`** — Fetch full listing page details (about, rules, parking, amenities, format_fit) for board or listing IDs into SQLite. Pure HTTP GET `/v1/listings/{id}` — not part of search.

  _Use after shortlisting, before export/compare/host outreach, when you need deeper page blocks than search cards._

  ```bash
  peerspace-pp-cli shortlist hydrate --board-id 6a590f4497e3495c4f756ee8 --collaborator-id 66915212d22cc89e3402c745 --force --json
  ```
- **`shortlist delta`** — Show favorites added, removed, or changed since the last sync snapshot.

  _Use to see what changed on your board without scrolling hearts._

  ```bash
  peerspace-pp-cli shortlist delta --json
  ```
- **`shortlist similar`** — Find local listings near a favorite on capacity, price, and neighborhood.

  _Use when a favorite is booked or too expensive and you need substitutes._

  ```bash
  peerspace-pp-cli shortlist similar --within-pct 20 --json
  ```
- **`projects scout`** — Combine project location metadata with fitting listings from the local store.

  _Use when a saved project needs a ready market pack._

  ```bash
  peerspace-pp-cli projects scout --json
  ```
- **`shortlist export`** — Export favorites as clean JSON plus a markdown block (price, capacity, amenities, fit notes) for Eventbrite/Luma/Slack.

  _Use when turning a Peerspace shortlist into a Luma/Eventbrite draft or Slack approval message._

  ```bash
  peerspace-pp-cli shortlist export --format markdown --json
  ```
- **`shortlist drift`** — Track price and availability-field changes on favorites over time (Luma-style watch).

  _Use to catch rate spikes or lost instant-book before you pitch a venue._

  ```bash
  peerspace-pp-cli shortlist drift --since 7d --json
  ```

### Local state that compounds
- **`sync coverage`** — Report which market query keys are present, row counts, and last-synced times.

  _Use before scout/markets/sql so agents fail closed on empty markets._

  ```bash
  peerspace-pp-cli sync coverage --json
  ```

### Agent-native plumbing
- **`pulse`** — One cookie-auth shot: message thread count, favorites summary, profile identity, and optional recent shortlist activity.

  _Use for a daily inbox + shortlist health check without opening Peerspace._

  ```bash
  peerspace-pp-cli pulse --include-shortlist --json
  ```

### Tech event planning
- **`venues recommend`** — Rank synced venues for tech meetups/workshops by headcount, date window, budget, and vibe keywords.

  _Use when an agent or planner needs a ranked shortlist for a tech event, not a raw search scroll._

  ```bash
  peerspace-pp-cli venues recommend --guests 40 --budget-max 180 --vibe projector,wifi --json
  ```
- **`venues get`** — Lookup or live-fetch one full listing detail document (GET `/v1/listings/{id}`) including rules, parking, cleaning, cancellation; writes through to SQLite.

  _Use when you need deep page context for one venue without hydrating a whole board._

  ```bash
  peerspace-pp-cli venues get 68d468bb44492187e415d4a6 --json
  ```
- **`scout gaps`** — Surface missing tech-event must-haves (WiFi, AV/projector, late access, flexible seating, transit) on any shortlist.

  _Use before finalizing a venue to see what each shortlist option lacks for a tech event._

  ```bash
  peerspace-pp-cli scout gaps --checklist tech-meetup --json
  ```
- **`scout multi-city`** — Cross-city comparison with top venue options per city under shared tech-event constraints.

  _Use for multi-city meetup tours or choosing a host city under the same constraints._

  ```bash
  peerspace-pp-cli scout multi-city --city Paris --city Lyon --guests 30 --budget-max 200 --top 3 --json
  ```

### Booking readiness & host contact
- **`calendar availability-start`** / **`availability-end`** / **`availability-month`** — Guest-side availability for a space (day start slots, end slots after a start index, month overview).

  _Use when checking whether a shortlisted venue is free for your event day/window._

  ```bash
  peerspace-pp-cli calendar availability-start --space-id 68d458dba45ae0878156d4b6 --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 7 --day 23 --json
  peerspace-pp-cli calendar availability-end --space-id 68d458dba45ae0878156d4b6 --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 7 --day 23 --time-index 22 --json
  peerspace-pp-cli calendar availability-month --space-id 68d458dba45ae0878156d4b6 --year 2026 --month 7 --json
  ```
- **`contracts inquiry-quote`** — Price the message-host inquiry form without submitting (POST `/v1/contracts/inquiry/guest/quote`).

  _Use while composing outreach to estimate cost for dates + guest count._

  ```bash
  peerspace-pp-cli contracts inquiry-quote --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --json
  ```
- **`contracts inquiry-send`** — Send a host inquiry with message (POST `/v1/contracts/inquiry/guest`). **Requires `--yes`.** Contacts the host.

  _Use only when the user explicitly wants to message hosts._

  ```bash
  peerspace-pp-cli contracts inquiry-send --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --message "Hi — interested in a 50-person meetup." --yes --json
  ```
- **`contracts guest-quote`** — Price a request-to-book window (POST `/v1/contracts/request/guest/quote`, often prepare-only).

  _Use when estimating a formal booking request (not the inquiry message form)._

  ```bash
  peerspace-pp-cli contracts guest-quote --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --rate 85 --prepare-only --json
  ```
- **`verification listing`** — Check whether the guest needs ID verification for a listing.

  ```bash
  peerspace-pp-cli verification listing --listing-id 68d468bb44492187e415d4a6 --json
  ```
- **`messages unread`** — Inbox unread thread count (GET `/v2/messaging/messages/inbox/unread-count`).

  ```bash
  peerspace-pp-cli messages unread --json
  ```
- **`spaces faqs-event`** — Event FAQs for a space.

  ```bash
  peerspace-pp-cli spaces faqs-event --space-id 68d458dba45ae0878156d4b6 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 122 API entries from 702 total network entries
- Protocols: websocket (95% confidence), rpc_envelope (80% confidence), rest_json (75% confidence), html_scrape (55% confidence)
- Auth signals: api_key — headers: statsig-api-key, x-goog-api-key; api_key — query: key, token
- Generation hints: has_rpc_envelope, weak_schema_confidence, browser_http_transport
- Candidate command ideas: create_GetViewportInfo — Derived from observed POST /$rpc/google.internal.maps.mapsjs.v1.MapsJsInternalService/GetViewportInfo traffic.; create_initialize — Derived from observed POST /v1/initialize traffic.; create_m — Derived from observed POST /v1/m traffic.; create_p — Derived from observed POST /v1/p traffic.; create_pageview — Derived from observed POST /v1/pageview traffic.; create_prop.json — Derived from observed POST /prop.json traffic.; create_rgstr — Derived from observed POST /v1/rgstr traffic.; create_rum — Derived from observed POST /cdn-cgi/rum traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**entitlements** — Profile entitlement attachments

- `peerspace-pp-cli entitlements` — Entitlement attachments for a profile id

**env** — Public client environment config and keyword dictionaries

- `peerspace-pp-cli env keywords` — Localized keyword dictionary (fr-FR sample from capture)
- `peerspace-pp-cli env web` — Fetch Peerspace SPA web environment config and feature flags

**messages** — Messaging inbox counts

- `peerspace-pp-cli messages` — Unread or total message thread count for current user (v1)
- `peerspace-pp-cli messages unread` — Inbox unread thread count (v2)

**profiles** — Authenticated user profile

- `peerspace-pp-cli profiles experience` — Profile experience / onboarding signals for current user
- `peerspace-pp-cli profiles me` — Current authenticated profile (requires browser session cookie)

**projects** — Saved projects and favorites boards

- `peerspace-pp-cli projects details` — Project details for a collaborator (favorites project metadata)
- `peerspace-pp-cli projects fav-board` — Favorite board attachments (saved listing ids) for a collaborator

**shortlist** — Offline shortlist intelligence + live favorite writes + detail hydrate

- `peerspace-pp-cli shortlist add` — Favorite a space onto an existing board (POST /v1/projects/attachments; body uses `project` board id string)
- `peerspace-pp-cli shortlist create-board` — Create a favorite board and attach a listing
- `peerspace-pp-cli shortlist compare` — Join favorites to listing attributes
- `peerspace-pp-cli shortlist delta` — Favorites added/removed since last snapshot
- `peerspace-pp-cli shortlist drift` — Price/availability changes on favorites
- `peerspace-pp-cli shortlist export` — Markdown/JSON handoff (About/Rules/Parking after hydrate); optional `--collaborator-id` / `--board-id`
- `peerspace-pp-cli shortlist hydrate` — Fetch full listing detail for board or listing IDs into SQLite (GET /v1/listings/{id})
- `peerspace-pp-cli shortlist similar` — Find local substitutes near a favorite

**venues** — Venue search, full detail, filters, and default booking times

- `peerspace-pp-cli venues default-times` — Default booking time windows by day-of-week for meetup activity
- `peerspace-pp-cli venues filters` — List amenity, space-type, style, and feature filters for an activity
- `peerspace-pp-cli venues get` — Local or live full listing detail (GET /v1/listings/{id}); caches to SQLite
- `peerspace-pp-cli venues list` — Search Peerspace listings by location, activity, space type, guests, price, and availability
- `peerspace-pp-cli venues natural` — Natural-language venue search (nl_query body). Discovered from search-app JS; enrich with same query params as list.
- `peerspace-pp-cli venues prices` — Batch total-price calculation for listing ids + booking windows (from search-app getTotalPrices)
- `peerspace-pp-cli venues recommend` — Rank synced venues for tech events

**calendar** — Guest-side booking availability

- `peerspace-pp-cli calendar availability-start` — Start time slots for a space/day
- `peerspace-pp-cli calendar availability-end` — End time slots after a start `time_index`
- `peerspace-pp-cli calendar availability-month` — Month-level availability for a space

**contracts** — Quotes and host inquiries

- `peerspace-pp-cli contracts guest-quote` — Request-to-book pricing quote (prepare-only supported)
- `peerspace-pp-cli contracts inquiry-quote` — Message-host form pricing (type=INQUIRY)
- `peerspace-pp-cli contracts inquiry-send` — Submit host inquiry + message (requires `--yes`)

**verification** — Guest checks

- `peerspace-pp-cli verification listing` — Whether guest needs verification for a listing

**spaces** — Space-level resources

- `peerspace-pp-cli spaces faqs-event` — Event FAQs for a space


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `PEERSPACE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `peerspace-pp-cli venues`
- `peerspace-pp-cli venues get`
- `peerspace-pp-cli venues list`
- `peerspace-pp-cli venues search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
peerspace-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Scout Paris meetups under budget

```bash
peerspace-pp-cli venues list --activity meetup --viewport-location paris--france --size 24 --json --select hits.hits.title,hits.hits.city,hits.hits.number_guests
```

Live search with field narrowing for agent context

### Price bands after hydrate

```bash
peerspace-pp-cli scout budget --city Paris --activity meetup --band 50 --json
```

Offline price histogram from local market snapshot

### Compare favorites

```bash
peerspace-pp-cli shortlist compare --sort price --agent
```

Cookie session + joined shortlist table

### Create a favorite board (save a space)

```bash
peerspace-pp-cli shortlist create-board --name "Q3 meetup shortlist" --listing-id 68d468bb44492187e415d4a6 --activity Meetup --location "Paris, France" --agent
```

Cookie-auth write: creates board and attaches the listing in one POST

### Favorite a space onto an existing board

```bash
peerspace-pp-cli shortlist add --listing-id 68d468bb44492187e415d4a6 --board-id 669152994300a86e4a943da5 --agent
```

Cookie-auth write: attaches listing to board (`project` = board id string)

### Hydrate full listing details after shortlisting

```bash
peerspace-pp-cli shortlist hydrate --board-id 6a590f4497e3495c4f756ee8 --collaborator-id 66915212d22cc89e3402c745 --force --agent
```

Pure HTTP detail fetch (not search). Stores about/rules/parking into SQLite for export and format_fit (talk/wellness/fb/production)

### Message a host about staffing / fit

```bash
# 1) Pick start/end slots
peerspace-pp-cli calendar availability-start --space-id <space_id> --listing-id <listing_id> --year 2026 --month 7 --day 23 --agent
peerspace-pp-cli calendar availability-end --space-id <space_id> --listing-id <listing_id> --year 2026 --month 7 --day 23 --time-index 22 --agent

# 2) Optional quote, then send (contacts host — requires --yes)
peerspace-pp-cli contracts inquiry-quote --listing-id <listing_id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --agent
peerspace-pp-cli contracts inquiry-send --listing-id <listing_id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --message "Hi — interested in a 50-person meetup." --yes --agent
```

`space_id` and `host_id` come from `venues get <listing_id>` (`parentSpaceId` / `ownerId`). Rate-limit if contacting multiple hosts.

### Multi-city pulse

```bash
peerspace-pp-cli markets pulse --city Paris --city Lyon --activity meetup --json
```

Cross-city aggregates over synced keys

### Daily coordination pulse

```bash
peerspace-pp-cli pulse --json --select thread_count,favorites_count,profile.first_name
```

Inbox + shortlist + identity in one call

### Rank tech meetup fit

```bash
peerspace-pp-cli venues recommend --guests 40 --budget-max 180 --vibe wifi,projector --json --select id,title,score,gaps
```

Ranked venues with token-efficient fields for agent planning

### Export shortlist for Luma (after hydrate)

```bash
peerspace-pp-cli shortlist export --format markdown --collaborator-id <psuser> --board-id <board_id> --agent
```

Markdown + JSON handoff with About / Included / Host rules / Parking when listings are hydrated

### Amenity gaps on favorites

```bash
peerspace-pp-cli scout gaps --checklist tech-meetup --json
```

Missing tech-event must-haves on the shortlist

## Auth Setup

Public search and filters work without login (Surf clears Cloudflare). Favorites, profile, messages, projects, listing detail, calendar, quotes, verification, and host inquiries need a browser session:

```bash
peerspace-pp-cli auth login --chrome
```

While logged into peerspace.com in Chrome (cookies include `PSUser` + `PSAccess`; writes and many guest mutations also send `Authorization: Bearer <PSAccess>`).

Run `peerspace-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  peerspace-pp-cli venues list --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `PEERSPACE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PEERSPACE_CONFIG_DIR`, `PEERSPACE_DATA_DIR`, `PEERSPACE_STATE_DIR`, `PEERSPACE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PEERSPACE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `peerspace-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "peerspace": {
        "command": "peerspace-pp-mcp",
        "env": {
          "PEERSPACE_HOME": "/srv/peerspace"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PEERSPACE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PEERSPACE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
peerspace-pp-cli recall "<user's question>" --agent
```

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
      "next_action": ["<trial command>", "peerspace-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `peerspace-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `peerspace-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `peerspace-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
peerspace-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
peerspace-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
peerspace-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
peerspace-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
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

`peerspace-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PEERSPACE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
peerspace-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
peerspace-pp-cli feedback --stdin < notes.txt
peerspace-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PEERSPACE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PEERSPACE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
peerspace-pp-cli profile save briefing --json
peerspace-pp-cli --profile briefing venues list
peerspace-pp-cli profile list --json
peerspace-pp-cli profile show briefing
peerspace-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `peerspace-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/peerspace/cmd/peerspace-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add peerspace-pp-mcp -- peerspace-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which peerspace-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   peerspace-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `peerspace-pp-cli <command> --help`.
