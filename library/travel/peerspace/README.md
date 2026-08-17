# Peerspace CLI

**Tech-event venue scout for Peerspace: search, shortlist, hydrate full listing details, check availability, message hosts, and export proposal packs for Luma/Eventbrite/Slack.**

Peerspace has no public API. peerspace-pp-cli wraps live venue search and listing detail behind Surf transport, saves favorites boards, hydrates full page blocks (about, rules, parking, amenities) into SQLite, checks calendar availability, prices and sends host inquiries, and exports shortlists for Eventbrite/Luma/Slack.

**Typical flow:** `venues list` → `shortlist create-board` / `add` → `shortlist hydrate` → `calendar availability-*` → `contracts inquiry-send` → `shortlist export`.

Learn more at [Peerspace](https://www.peerspace.com).

## Install

The recommended path installs both the `peerspace-pp-cli` binary and the `pp-peerspace` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install peerspace
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install peerspace --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install peerspace --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install peerspace --agent claude-code
npx -y @mvanhorn/printing-press-library install peerspace --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/peerspace/cmd/peerspace-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peerspace-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install peerspace --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-peerspace --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-peerspace --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install peerspace --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
peerspace-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peerspace-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/peerspace/cmd/peerspace-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "peerspace": {
      "command": "peerspace-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public search and filters work without login (Surf clears Cloudflare). Favorites, profile, messages, projects, **listing detail**, **calendar**, **quotes**, **verification**, and **host inquiries** need a browser session:

```bash
peerspace-pp-cli auth login --chrome
```

While logged into [peerspace.com](https://www.peerspace.com) in Chrome. Cookies include `PSUser` and `PSAccess`; many guest writes also send `Authorization: Bearer <PSAccess>`.

## Quick Start

```bash
# Verify the CLI boots and transport is wired
peerspace-pp-cli doctor

# Import Chrome cookies (needed for boards, detail, outreach)
peerspace-pp-cli auth login --chrome

# Pull a live market snapshot (search cards — not full page detail)
peerspace-pp-cli venues list --activity meetup --viewport-location paris--france --size 24 --json

# Save a board + favorite spaces
peerspace-pp-cli shortlist create-board --name "Paris meetup ≤50" --listing-id <id> --activity Meetup --location "Paris, France" --json
peerspace-pp-cli shortlist add --listing-id <id2> --board-id <board_id> --json

# Hydrate full listing pages into local SQLite (about / rules / parking)
peerspace-pp-cli shortlist hydrate --board-id <board_id> --collaborator-id <psuser> --force --json

# Export stakeholder markdown (richer after hydrate)
peerspace-pp-cli shortlist export --format markdown --collaborator-id <psuser> --board-id <board_id>

# Check day availability, then message a host (inquiry-send requires --yes)
peerspace-pp-cli calendar availability-start --space-id <space_id> --listing-id <id> --year 2026 --month 7 --day 23 --json
peerspace-pp-cli contracts inquiry-send --listing-id <id> --year 2026 --month 7 --day 23 \
  --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 \
  --activity meetup --message "Hi — interested in a 50-person meetup." --yes --json
```

## Unique Features

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
- **`shortlist hydrate`** — Fetch full listing page details (about, rules, parking, amenities, `format_fit`) for a board or listing IDs into SQLite. Pure HTTP `GET /v1/listings/{id}` — **not** run during search.

  _Use after shortlisting, before export or host outreach._

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
- **`venues get`** — Local or live full listing detail (`GET /v1/listings/{id}`) with write-through to SQLite.

  ```bash
  peerspace-pp-cli venues get 68d468bb44492187e415d4a6 --json
  ```

### Booking readiness & host contact
- **`calendar availability-start` / `availability-end` / `availability-month`** — Guest-side day and month availability for a space.

  ```bash
  peerspace-pp-cli calendar availability-start --space-id <space_id> --listing-id <id> --year 2026 --month 7 --day 23 --json
  ```
- **`contracts inquiry-quote`** — Price the message-host form without sending.

  ```bash
  peerspace-pp-cli contracts inquiry-quote --listing-id <id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --json
  ```
- **`contracts inquiry-send`** — Submit a host inquiry + message (**requires `--yes`**; contacts the host).

  ```bash
  peerspace-pp-cli contracts inquiry-send --listing-id <id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --message "Hello…" --yes --json
  ```
- **`contracts guest-quote`** — Request-to-book pricing quote (prepare-only supported).
- **`verification listing`** — Whether the guest needs ID verification for a listing.
- **`messages unread`** — Inbox unread count (v2).
- **`spaces faqs-event`** — Event FAQs for a space.

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

Cookie-auth write: attaches listing to board (`project` = board id string; also sends `Authorization: Bearer <PSAccess>`)

### Hydrate full listing details after shortlisting

```bash
peerspace-pp-cli shortlist hydrate --board-id 6a590f4497e3495c4f756ee8 --collaborator-id 66915212d22cc89e3402c745 --force --agent
```

Pure HTTP detail fetch (not search). Stores about/rules/parking into SQLite for export and `format_fit` (talk/wellness/fb/production)

### Message a host about staffing / fit

```bash
# 1) Pick start/end slots
peerspace-pp-cli calendar availability-start --space-id <space_id> --listing-id <listing_id> --year 2026 --month 7 --day 23 --agent
peerspace-pp-cli calendar availability-end --space-id <space_id> --listing-id <listing_id> --year 2026 --month 7 --day 23 --time-index 22 --agent

# 2) Optional quote, then send (contacts host — requires --yes)
peerspace-pp-cli contracts inquiry-quote --listing-id <listing_id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --agent
peerspace-pp-cli contracts inquiry-send --listing-id <listing_id> --year 2026 --month 7 --day 23 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --message "Hi — interested in a 50-person meetup." --yes --agent
```

`space_id` and `host_id` come from `venues get <listing_id>` (`parentSpaceId` / `ownerId`). Rate-limit if contacting multiple hosts (exit code 7).

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

Markdown + JSON handoff with About / Included / Host rules / Parking when listings are hydrated. Pass `--collaborator-id` / `--board-id` so export is not empty.

### Amenity gaps on favorites

```bash
peerspace-pp-cli scout gaps --checklist tech-meetup --json
```

Missing tech-event must-haves on the shortlist

## Usage

Run `peerspace-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PEERSPACE_CONFIG_DIR`, `PEERSPACE_DATA_DIR`, `PEERSPACE_STATE_DIR`, or `PEERSPACE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PEERSPACE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PEERSPACE_HOME=/srv/peerspace
peerspace-pp-cli doctor
```

Under `PEERSPACE_HOME=/srv/peerspace`, the four dirs resolve to `/srv/peerspace/config`, `/srv/peerspace/data`, `/srv/peerspace/state`, and `/srv/peerspace/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `PEERSPACE_DATA_DIR` overrides an explicit `--home` for that kind. Use `PEERSPACE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PEERSPACE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `peerspace-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### entitlements

Profile entitlement attachments

- **`peerspace-pp-cli entitlements`** - Entitlement attachments for a profile id

### env

Public client environment config and keyword dictionaries

- **`peerspace-pp-cli env keywords`** - Localized keyword dictionary (fr-FR sample from capture)
- **`peerspace-pp-cli env web`** - Fetch Peerspace SPA web environment config and feature flags

### messages

Messaging inbox counts

- **`peerspace-pp-cli messages`** - Unread or total message thread count for current user (v1)
- **`peerspace-pp-cli messages unread`** - Inbox unread thread count (v2)

### profiles

Authenticated user profile

- **`peerspace-pp-cli profiles experience`** - Profile experience / onboarding signals for current user
- **`peerspace-pp-cli profiles me`** - Current authenticated profile (requires browser session cookie)

### projects

Saved projects and favorites boards

- **`peerspace-pp-cli projects details`** - Project details for a collaborator (favorites project metadata)
- **`peerspace-pp-cli projects fav-board`** - Favorite board attachments (saved listing ids) for a collaborator

### shortlist

Offline shortlist intelligence + live favorite writes + detail hydrate

- **`peerspace-pp-cli shortlist add`** - Favorite a space onto an existing board (POST /v1/projects/attachments)
- **`peerspace-pp-cli shortlist create-board`** - Create a favorite board and attach a listing
- **`peerspace-pp-cli shortlist compare`** - Join favorites to listing attributes
- **`peerspace-pp-cli shortlist delta`** - Favorites added/removed since last snapshot
- **`peerspace-pp-cli shortlist drift`** - Price/availability changes on favorites
- **`peerspace-pp-cli shortlist export`** - Markdown/JSON handoff (About/Rules/Parking after hydrate); optional `--collaborator-id` / `--board-id`
- **`peerspace-pp-cli shortlist hydrate`** - Fetch full listing detail for board or IDs into SQLite
- **`peerspace-pp-cli shortlist similar`** - Find local substitutes near a favorite

### venues

Venue search, full detail, filters, and default booking times

- **`peerspace-pp-cli venues default-times`** - Default booking time windows by day-of-week for meetup activity
- **`peerspace-pp-cli venues filters`** - List amenity, space-type, style, and feature filters for an activity
- **`peerspace-pp-cli venues get`** - Local or live full listing detail (GET /v1/listings/{id}); caches to SQLite
- **`peerspace-pp-cli venues list`** - Search Peerspace listings by location, activity, space type, guests, price, and availability
- **`peerspace-pp-cli venues natural`** - Natural-language venue search (nl_query body). Discovered from search-app JS; enrich with same query params as list.
- **`peerspace-pp-cli venues prices`** - Batch total-price calculation for listing ids + booking windows (from search-app getTotalPrices)
- **`peerspace-pp-cli venues recommend`** - Rank synced venues for tech events

### calendar

Guest-side booking availability

- **`peerspace-pp-cli calendar availability-start`** - Start time slots for a space/day
- **`peerspace-pp-cli calendar availability-end`** - End time slots after a start `time_index`
- **`peerspace-pp-cli calendar availability-month`** - Month-level availability for a space

### contracts

Quotes and host inquiries

- **`peerspace-pp-cli contracts guest-quote`** - Request-to-book pricing quote (prepare-only supported)
- **`peerspace-pp-cli contracts inquiry-quote`** - Message-host form pricing (type=INQUIRY)
- **`peerspace-pp-cli contracts inquiry-send`** - Submit host inquiry + message (requires `--yes`)

### verification

Guest checks

- **`peerspace-pp-cli verification listing`** - Whether guest needs verification for a listing

### spaces

Space-level resources

- **`peerspace-pp-cli spaces faqs-event`** - Event FAQs for a space


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`peerspace-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`peerspace-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`peerspace-pp-cli learnings list`** - Inspect taught rows
- **`peerspace-pp-cli learnings forget <query>`** - Undo a teach
- **`peerspace-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`peerspace-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`peerspace-pp-cli teach-pattern`** - Install a query/resource template up front
- **`peerspace-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PEERSPACE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `peerspace-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
peerspace-pp-cli venues list

# JSON for scripting and agents
peerspace-pp-cli venues list --json

# Filter to specific fields
peerspace-pp-cli venues list --json --select id,name,status

# Dry run — show the request without sending
peerspace-pp-cli venues list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
peerspace-pp-cli venues list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `PEERSPACE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `peerspace-pp-cli venues`
- `peerspace-pp-cli venues get`
- `peerspace-pp-cli venues list`
- `peerspace-pp-cli venues search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
peerspace-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `peerspace-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/peerspace-pp-cli/config.toml`; `--home`, `PEERSPACE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `peerspace-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 or Cloudflare challenge HTML on venues list** — Browser-Chrome transport is default; retry; if still blocked, run auth login --chrome for clearance cookies
- **profiles me or fav-board returns 401** — Run peerspace-pp-cli auth login --chrome while logged into peerspace.com
- **scout/markets returns empty** — Run venues list for that city+activity first, then peerspace-pp-cli sync coverage --json
- **shortlist export is empty** — Pass `--collaborator-id` (PSUser) and `--board-id`; run `shortlist hydrate` first for About/Rules/Parking blocks
- **shortlist add / create-board fails** — Needs cookie session + `PSAccess` bearer; body field is `project` (board id string), not nested `project_id`
- **inquiry-send rate limited (exit 7)** — Wait and retry; do not blast many hosts without delays
- **calendar / contracts need space_id** — Resolve via `venues get <listing_id>` (`parentSpaceId`); listing id alone is not enough for availability paths

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://img.peerspace.com/image/upload/f_auto,q_auto,dpr_auto,w_3840/Rebrand%20Imagery/Hero/04_-_HERO
- Capture coverage: 122 API entries from 702 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: websocket (95% confidence), rpc_envelope (80% confidence), rest_json (75% confidence), html_scrape (55% confidence)
- Auth signals: api_key — headers: statsig-api-key, x-goog-api-key; api_key — query: key, token
- Protection signals: cloudflare (90% confidence), captcha (85% confidence)
- Generation hints: has_rpc_envelope, weak_schema_confidence, browser_http_transport
- Candidate command ideas: create_GetViewportInfo — Derived from observed POST /$rpc/google.internal.maps.mapsjs.v1.MapsJsInternalService/GetViewportInfo traffic.; create_initialize — Derived from observed POST /v1/initialize traffic.; create_m — Derived from observed POST /v1/m traffic.; create_p — Derived from observed POST /v1/p traffic.; create_pageview — Derived from observed POST /v1/pageview traffic.; create_prop.json — Derived from observed POST /prop.json traffic.; create_rgstr — Derived from observed POST /v1/rgstr traffic.; create_rum — Derived from observed POST /cdn-cgi/rum traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
