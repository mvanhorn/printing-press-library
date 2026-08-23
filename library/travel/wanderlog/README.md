# Wanderlog CLI

**Clone shared Wanderlog plans into editable trips, then mine guides and places for agent-ready planning.**

Public reads — guides, geos, places, category lists, and shared plans — need no cookie; writes do. With WANDERLOG_COOKIE, `plan clone`/`plan fill` and the fine-grained plan editor write through ShareDB, and every mutation is inspectable as a dry-run before `--apply`.

Learn more at [Wanderlog](https://wanderlog.com).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `wanderlog-pp-cli` binary and the `pp-wanderlog` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wanderlog
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wanderlog --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wanderlog --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wanderlog --agent claude-code
npx -y @mvanhorn/printing-press-library install wanderlog --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlog/cmd/wanderlog-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wanderlog-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wanderlog --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wanderlog --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wanderlog --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wanderlog --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wanderlog-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WANDERLOG_COOKIE` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlog/cmd/wanderlog-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wanderlog": {
      "command": "wanderlog-pp-mcp",
      "env": {
        "WANDERLOG_COOKIE": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Public guide, geo, place, category, shared-plan preview, and shared itinerary reads work without credentials. Creating, filling, or fine-grained editing of a target trip requires WANDERLOG_COOKIE, holding the current connect.sid cookie value in Cookie-header format.

There is no `auth login`. The user runs setup in their own terminal — never paste `connect.sid` into chat, and never print the cookie:

```bash
wanderlog-pp-cli auth setup
wanderlog-pp-cli auth set-token YOUR_TOKEN_HERE
```

`--launch` opens the setup URL. After a write fails, run `auth status --agent`; a present cookie is enough even when `verified` is false. ShareDB apply mode is gated behind `--apply` and should be dogfooded only against an approved disposable target trip.

## Quick Start

```bash
# Verify the generated CLI and config path before making live requests.
wanderlog-pp-cli doctor --dry-run

# Inspect a pasted shared link first: dates, sections, counts, and clone warnings in about 1KB.
wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent

# Read the itinerary itself — days, section headings, and stop names — without dumping the full trip JSON.
wanderlog-pp-cli plan outline --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent

# Preview the new-trip clone operation without writing to Wanderlog.
wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent

```

## Known Gaps

- **`sync` populates nothing on this CLI.** Wanderlog exposes no bulk list endpoints, so the sync resource table is empty: `wanderlog-pp-cli sync` exits 0, writes zero rows, and prints `no_bulk_list_endpoints`. The local SQLite mirror is filled only as a side effect of single-fetch reads (`guides get`, `places details`, `trips get`, and friends). `workflow archive` has the same limit, because it drives the same resource list.
- **"Run sync first" advisories are inaccurate.** Because the local store is never sync-populated, hints such as `hint: local store has not been synced yet. Run 'wanderlog-pp-cli sync' ...` and the equivalent line from `workflow status` point at a command that cannot fix the condition. Read the data you need with the single-fetch commands instead; the mirror fills as you go.
- **`workflow-verify` reports `workflow-pass` on zero workflows.** The CLI ships two real `workflow` subcommands (`archive`, `status`), but no `workflow_verify.yaml`, so the verifier has nothing to check and its pass verdict carries no signal.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Plan cloning and fill
- **`plan clone`** — Create a new Wanderlog trip from a shared or public source plan, then fill it with the source plan template.

  _Use this when the user wants a private editable copy of a shared plan. A `/plan/<16-char-key>/...` URL is often already editable — do not clone just to edit it._

  ```bash
  wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent
  ```
- **`plan fill`** — Fill an existing Wanderlog trip from a shared or public source plan with dry-run and force safeguards.

  _Use this when the user already created a target trip and wants to populate it from a shared template. Substitute YOUR_TRIP_KEY with the 16-character key of a trip you own, from `trips home`._

  ```bash
  wanderlog-pp-cli plan fill --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --target-key YOUR_TRIP_KEY --dry-run --agent
  ```
- **`plan preview`** — Inspect a shared plan and report dates, sections, blocks, resources, and clone warnings before any write.

  _Use this before clone/fill to confirm what will be copied and whether credentials are needed. The report is ~1KB, so it is safe to run on any pasted link._

  ```bash
  wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```

### Plan reading and inspection
- **`trips home`** — List the authenticated account's home trips together with their 16-character plan `key`s. Also: list my trips, own itineraries.

  _Run this before any outline or edit: the numeric `tripPlan.id` is not a key, and every editing command wants the 16-char key._

  ```bash
  wanderlog-pp-cli trips home --agent
  ```
- **`plan outline`** — Show a slim itinerary outline: days, section headings, and stop names, optionally for a single day. Also: show days in my plan, slim day list.

  _Use this instead of a fat `trips get` dump when an agent needs the shape of the itinerary cheaply; pass `--day` to narrow to one day._

  ```bash
  wanderlog-pp-cli plan outline --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan inspect`** — Inspect a slim itinerary outline and, with `--check`, report counts, unformatted notes, lodging coverage, closed places, and text-vs-schedule mismatches. Also: verify an itinerary after edits.

  _Run `--check` after a batch of writes to confirm the plan is still consistent before handing it back to the user._

  ```bash
  wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --check=counts,unformatted,lodging-coverage,closed-places,text-vs-schedule --agent
  ```
- **`plan votes`** — List place and hotel block upvote counts for a Wanderlog plan. Also: who upvoted places, upvotedBy counts.

  _Use this to see what collaborators actually voted for; the comments list is not a vote tally._

  ```bash
  wanderlog-pp-cli plan votes --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`lodging search`** — Search Wanderlog's lodging aggregator across Airbnb, Expedia, Google, and Kayak for a geo and date range, and return compact candidates carrying price, rating, coordinates, and booking URL. Also: search hotels, find a place to stay, compare stays.

  _Use this to find candidates before committing to one — it answers “where could we stay?”, which `plan reservation add` does not. Feed a chosen offer straight into `plan reservation add --kind lodging --lodging-offer-json '<offer>'`, which is how stays with no Google place id become native lodging blocks. `lodging` is hidden from `--help` but callable by name. The example narrows to one source and a single night so it returns quickly; --sources defaults to airbnb,expedia,google,kayak when you omit it._

  ```bash
  wanderlog-pp-cli lodging search --geo-id 50 --bounds 127.680,26.210,127.690,26.220 --start-date 2026-10-05 --end-date 2026-10-06 --sources expedia --hotel-or-vacation-rental hotel --limit 3 --agent
  ```

### Agentic plan editing
- **`plan sections`** — List editable section indexes, day numbers, section ids, dates, and block counts for a plan.

  _Use this before editing so an agent targets the right day or section._

  ```bash
  wanderlog-pp-cli plan sections --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan note add`** — Add a note block to a selected day or section through ShareDB.

  _Use this to add reminders, reservations, constraints, or planning notes._

  ```bash
  wanderlog-pp-cli plan note add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --text 'Book ferry tickets' --dry-run --agent
  ```
- **`plan place add`** — Add a real place block to a selected day or section from a Google/Wanderlog place id, or from a query with location bias. Also: geocode a place by query, verify an address.

  _Use this to build an itinerary stop by stop._

  ```bash
  wanderlog-pp-cli plan place add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --text 'Sunset photos' --dry-run --agent
  ```
- **`plan place replace`** — Replace only the nested place on an existing itinerary block, keeping its times and notes.

  _Use this when a stop was geocoded to the wrong venue and you do not want to lose the schedule or note already attached to it._

  ```bash
  wanderlog-pp-cli plan place replace --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --dry-run --agent
  ```
- **`plan fill-day`** — Insert a batch of place stops into one Wanderlog day from a JSON array of stops with optional times and notes.

  _Use this to lay down a whole day in one apply instead of one `plan place add` per stop; `--closed-place-policy` blocks stops that are closed on that date._

  ```bash
  wanderlog-pp-cli plan fill-day --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --stops-json '[{"place_id":"ChIJxekpmbdp5TQRSqyFdGKMUJc","start":"09:00","note":"Cafe"}]' --dry-run --agent
  ```
- **`plan block move`** — Move a note or place block within or across days.

  _Use this to reorder an itinerary after adding candidate stops._

  ```bash
  wanderlog-pp-cli plan block move --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --to-day 2 --to-position 0 --dry-run --agent
  ```
- **`plan block delete`** — Delete a note or place block from a selected day or section.

  _Use this to clean up test blocks or remove rejected candidates._

  ```bash
  wanderlog-pp-cli plan block delete --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --dry-run --agent
  ```
- **`plan block edit-text`** — Replace the rich-text note attached to an existing block, as plain text or compiled markdown with `--markdown`. Also: replace a stop note as plain text.

  _Use this to revise stop notes, reservation notes, or reminders. Without `--markdown` a bulleted string becomes one flat insert; with it, `**bold**`, `-`/`*` bullets, and `#` label lines survive._

  ```bash
  wanderlog-pp-cli plan block edit-text --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --text 'Updated note' --dry-run --agent
  ```
- **`plan block edit-text --markdown`** — Compile a markdown note into Wanderlog rich text: **bold**, `-`/`*` bullets, and `#` label lines. Also: format notes with bullets, bold text, rich text on a stop.

  _Without `--markdown` a bulleted string lands as one flat insert; with it the bullets and bold runs survive._

  ```bash
  wanderlog-pp-cli plan block edit-text --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --markdown --text $'# Stop\n- item' --dry-run --agent
  ```
- **`plan block rename`** — Rename the display name on an existing place or lodging block. Also: rename a place or hotel, change a display name.

  _Use this after a bad geocode instead of `plan block set-field place.name`, which does not update the rendered stop label._

  ```bash
  wanderlog-pp-cli plan block rename --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 1 --name 'Property' --dry-run --agent
  ```
- **`plan block set-field`** — Set or remove a non-protected field on a block, including newly observed fields from live plan data.

  _Use this for schedule or metadata fields once you have inspected the plan shape; use `--json-value` for numbers, booleans, arrays, or objects._

  ```bash
  wanderlog-pp-cli plan block set-field --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --field startTime --value 09:30 --dry-run --agent
  ```
- **`plan block schedule`** — Set or clear first-class schedule fields on an existing block. Also: set stop times, duration.

  _Use this to turn loose stops into a timed itinerary._

  ```bash
  wanderlog-pp-cli plan block schedule --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 2 --block-index 0 --start 09:30 --duration-minutes 90 --dry-run --agent
  ```
- **`plan block attachment`** — List, add, or remove attachment metadata on a block. Also: tickets, booking links, PDFs.

  _Use this for tickets, booking links, PDFs, and other planning artifacts._

  ```bash
  wanderlog-pp-cli plan block attachment add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --title Tickets --url https://example.com/tickets.pdf --dry-run --agent
  ```
- **`plan reservation add`** — Add a flight, lodging, rental car, restaurant, transit, cruise, or standalone attachment block with dates, times, and confirmation details. Also: record a hotel stay, check-in and check-out dates.

  _Use `--kind lodging` to record a stay: `--span-nights` is on by default when the dates span more than one night, and `--display-name` sets the rendered place name after geocoding._

  ```bash
  wanderlog-pp-cli plan reservation add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --kind lodging --query 'Hotel Moon Beach' --lat 26.32 --lng 127.76 --start-date 2026-09-01 --end-date 2026-09-03 --display-name 'Hotel Moon Beach' --dry-run --agent
  ```
- **`plan checklist`** — Add checklist blocks and add, check, or remove checklist items. Also: packing lists, todo items.

  _Use this for packing lists, booking tasks, and shared planning todos._

  ```bash
  wanderlog-pp-cli plan checklist add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --title Packing --item Passport --item Sunscreen --dry-run --agent
  ```
- **`plan comments`** — List, add, edit, delete, or vote on Wanderlog plan comments using the confirmed comments API. Also: friend discussion, replies, comment votes.

  _Use this to read friend discussion, ask questions, and explain agent-made edits._

  ```bash
  wanderlog-pp-cli plan comments list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan collaborators`** — Inspect collaborator/share metadata, list pending invites, send email/user invites, add or remove collaborators by user id, and create share keys. Also: invite a friend, share links, share keys, pending invites, permissions.

  _Use this for account-level collaboration tasks around the shared plan. Keep invite sends on `--dry-run` until the recipient list is explicit. Related invocations: `plan collaborators invites --plan-url URL --agent` lists pending invites, and `plan collaborators invite --plan-url URL --email friend@example.com --dry-run --agent` sends one._

  ```bash
  wanderlog-pp-cli plan collaborators --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan budget`** — Summarize, export, and edit Wanderlog trip budget expenses and settlement payments. Also: add an expense, costs, splits, payers, record a payment or settlement, CSV export.

  _Use this to set the trip budget, add costs with categories/splits/payers, link expenses to itinerary blocks, and record settlement payments. Mutating budget commands use ShareDB and are covered by `plan undo`/`plan redo`._

  ```bash
  wanderlog-pp-cli plan budget summary --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan route`** — Build route optimization request bodies or call Wanderlog's optimizeRoute endpoint. Also: optimize a route, better travel order.

  _Use this to compute a better order/travel path, then apply block moves deliberately._

  ```bash
  wanderlog-pp-cli plan route day-body --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --agent
  ```
- **`plan history`** — List, preview, undo, and redo local-journaled ShareDB itinerary edits (`plan history`, `plan undo`, `plan redo`). Also: undo my last edit, redo my last edit, revert an itinerary change.

  _Use this as the safety net after applied itinerary edits. Undo/redo defaults to preview; pass `--apply` to mutate the plan. Related invocations: `plan undo --plan-url URL --apply --agent` and `plan redo --plan-url URL --apply --agent`._

  ```bash
  wanderlog-pp-cli plan history --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
  ```
- **`plan section add-day`** — Insert a new day section and update trip day/date bounds.

  _Use this when the itinerary needs another travel day._

  ```bash
  wanderlog-pp-cli plan section add-day --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --date 2026-09-07 --position 11 --dry-run --agent
  ```
- **`plan section set-field`** — Set or remove a field on a section, including day headings or rich text.

  _Use this to label days or adjust section-level notes._

  ```bash
  wanderlog-pp-cli plan section set-field --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --field heading --value 'Arrival day' --dry-run --agent
  ```
- **`plan section delete`** — Delete an empty section and update trip day/date bounds.

  _Use this to remove unused days or empty sections after reviewing the target._

  ```bash
  wanderlog-pp-cli plan section delete --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 8 --force --dry-run --agent
  ```
- **`plan section swap-days`** — Swap the block arrays of two day sections in one JSON0 batch.

  _Use this to reorder a trip by whole days without moving stops one at a time; it is a single apply._

  ```bash
  wanderlog-pp-cli plan section swap-days --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --with-day 2 --dry-run --agent
  ```
- **`plan block apply`** — Preview or apply a JSON0 operation array read from disk with --ops-file. Also: apply an ops-file batch of plan edits.

  _Use this to replay a reviewed batch of operations in one apply; `--dry-run` alone short-circuits without reading the file. A real run is `plan block apply --plan-url URL --ops-file ./ops.json --apply --agent`; the global `--dry-run` returns before the file is read, so it is safe as a smoke probe._

  ```bash
  wanderlog-pp-cli plan block apply --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --ops-file ./ops.json --dry-run --agent
  ```
- **`plan raw op`** — Preview or apply an explicit ShareDB JSON0 operation array.

  _Use this only as an escape hatch for fields not yet covered by a named command, after inspecting live plan shape and dry-run output._

  ```bash
  wanderlog-pp-cli plan raw op --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --op '[{"p":["title"],"od":"Old","oi":"New"}]' --agent
  ```

## Recipes

### Read my itinerary or a pasted wanderlog.com/plan URL

```bash
wanderlog-pp-cli plan outline --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent
```

`trips home --agent` first when the user says “my trips”: it lists the 16-character keys. For a link nobody has opened yet, `plan preview --source-url URL --agent` reports dates, sections, block counts, and copy warnings in about 1 KB and needs no credentials. Then outline the plan you are about to edit, by `--plan-url` or `--target-key`.

### Clone or fill from a shared plan

```bash
wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent
```

Clone only when the user wants a private copy, then re-run with `--apply` once they name it. To populate a trip they already created, dry-run `plan fill --source-url URL --target-key YOUR_TRIP_KEY --agent` instead and compare the source and target copy actions first.

### Format notes with bullets

```bash
wanderlog-pp-cli plan block edit-text --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 0 --markdown --text $'**Label**\n- item' --dry-run --agent
```

Address the block with `--block-index` from `plan sections`, or with `--block-id` when you already know the numeric id. What `--markdown` does and does not compile: Gotchas.

### Save lodging for multiple nights

```bash
wanderlog-pp-cli plan reservation add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --kind lodging --query 'Hotel Moon Beach' --lat 26.32 --lng 127.76 --start-date 2026-09-01 --end-date 2026-09-03 --display-name 'Hotel Moon Beach' --dry-run --agent
```

`--query` needs `--lat`/`--lng` to bias the lookup — run `places autocomplete` first when you have no coordinates. `--span-nights` is on by default once the dates span more than one night, and `--display-name` fixes the rendered place name after geocoding.

### Rename a bad geocode

```bash
wanderlog-pp-cli plan block rename --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --block-index 1 --name 'Property' --dry-run --agent
```

This is the only command that moves the rendered stop label; see Gotchas.

### Check an itinerary after a batch of edits

```bash
wanderlog-pp-cli plan inspect --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --check=unformatted,lodging-coverage --agent
```

Always write `--check=NAMES`: `--check` has a no-argument default, so the space form `--check NAMES` runs all five checks and silently discards the value. `--check=NAMES` returns the named checks plus the plan scalars (`target_key`, `title`, `start_date`, `end_date`, `section_count`, `block_count`) and omits the sections outline — 226–1,309 B for one check and 2,432 B for all five on the sample plan, against ~21 KB for the full outline. Add `--with-sections` to keep the outline; without `--check`, `plan inspect` returns the full outline, identical to `plan outline`. The checks report block counts, notes that never got markdown formatting, days with no lodging, stops closed on their dated day, and text times that disagree with the schedule fields.

## Usage

Run `wanderlog-pp-cli --help` for the full command reference and flag list.

## Commands

Everything in this section is callable. The groups under **Endpoint groups** are marked `Hidden: true` so they stay out of `--help` — that keeps the agent-facing help lean — but they behave like any other command.

### Top-level commands

| Command | What it does |
|---|---|
| `wanderlog-pp-cli account` | Get the current Wanderlog account for WANDERLOG_COOKIE |
| `wanderlog-pp-cli session` | Get current anonymous session preferences |
| `wanderlog-pp-cli place-lists <geo_category_id>` | Get places in a destination category list |
| `wanderlog-pp-cli which` | Find the command that implements a capability |
| `wanderlog-pp-cli api` | Browse all API endpoints by interface name |
| `wanderlog-pp-cli agent-context` | Emit structured JSON describing this CLI for agents |
| `wanderlog-pp-cli sync` | Sync API data to local SQLite for offline search and analysis |
| `wanderlog-pp-cli import` | Import data from JSONL file via API create/upsert calls |
| `wanderlog-pp-cli doctor` | Check CLI health |
| `wanderlog-pp-cli version` | Print version |

### `plan` — read and edit an itinerary

The `plan` tree is where this CLI earns its keep. Every mutating subcommand previews by default and only writes with `--apply`; the global `--dry-run` blocks writes outright.

| Command | What it does |
|---|---|
| `wanderlog-pp-cli plan block` | Move or delete blocks in an editable Wanderlog plan |
| `wanderlog-pp-cli plan block apply` | Preview or apply a JSON0 operation array from a file |
| `wanderlog-pp-cli plan block attachment` | List, add, or remove block attachments |
| `wanderlog-pp-cli plan block attachment add` | Add an attachment metadata object to a Wanderlog block |
| `wanderlog-pp-cli plan block attachment list` | List attachments on a Wanderlog block |
| `wanderlog-pp-cli plan block attachment remove` | Remove an attachment from a Wanderlog block |
| `wanderlog-pp-cli plan block delete` | Delete one block from a Wanderlog day or section |
| `wanderlog-pp-cli plan block edit-text` | Replace block note as plain text unless --markdown |
| `wanderlog-pp-cli plan block move` | Move one block between Wanderlog days or positions |
| `wanderlog-pp-cli plan block rename` | Rename a place or lodging block display name |
| `wanderlog-pp-cli plan block schedule` | Set or clear schedule fields on a Wanderlog block |
| `wanderlog-pp-cli plan block set-field` | Set or remove a field on a Wanderlog block |
| `wanderlog-pp-cli plan budget` | Read and edit Wanderlog trip budget expenses and payments |
| `wanderlog-pp-cli plan budget csv` | Export Wanderlog budget expenses as CSV |
| `wanderlog-pp-cli plan budget expense` | List, add, edit, or remove budget expenses |
| `wanderlog-pp-cli plan budget expense add` | Add a Wanderlog budget expense |
| `wanderlog-pp-cli plan budget expense edit` | Edit a Wanderlog budget expense |
| `wanderlog-pp-cli plan budget expense list` | List Wanderlog budget expenses |
| `wanderlog-pp-cli plan budget expense remove` | Remove a Wanderlog budget expense |
| `wanderlog-pp-cli plan budget payment` | List, add, or remove budget settlement payments |
| `wanderlog-pp-cli plan budget payment add` | Add a Wanderlog budget settlement payment |
| `wanderlog-pp-cli plan budget payment list` | List Wanderlog budget payments |
| `wanderlog-pp-cli plan budget payment remove` | Remove a Wanderlog budget payment |
| `wanderlog-pp-cli plan budget set` | Set budget total, currency, or debt simplification |
| `wanderlog-pp-cli plan budget summary` | Summarize a Wanderlog trip budget |
| `wanderlog-pp-cli plan checklist` | Add and edit checklist blocks |
| `wanderlog-pp-cli plan checklist add` | Add a checklist block to a day or section |
| `wanderlog-pp-cli plan checklist item` | Add, check, or remove checklist items |
| `wanderlog-pp-cli plan checklist item add` | Add an item to a checklist block |
| `wanderlog-pp-cli plan checklist item check` | Set checked state on a checklist item |
| `wanderlog-pp-cli plan checklist item remove` | Remove an item from a checklist block |
| `wanderlog-pp-cli plan clone` | Create a new Wanderlog trip from a shared or public source plan, then fill it with the source plan template. |
| `wanderlog-pp-cli plan collaborators` | Inspect and edit Wanderlog collaborator/share metadata |
| `wanderlog-pp-cli plan collaborators add` | Add a Wanderlog collaborator by user id |
| `wanderlog-pp-cli plan collaborators invite` | Send Wanderlog trip invites by email or user id |
| `wanderlog-pp-cli plan collaborators invites` | List pending Wanderlog trip invites |
| `wanderlog-pp-cli plan collaborators remove` | Remove a Wanderlog collaborator by user id |
| `wanderlog-pp-cli plan collaborators share-key` | Create or refresh a Wanderlog share key with permissions |
| `wanderlog-pp-cli plan comments` | List and edit Wanderlog plan comments |
| `wanderlog-pp-cli plan comments add` | Add a comment to a Wanderlog plan |
| `wanderlog-pp-cli plan comments delete` | Delete a Wanderlog comment |
| `wanderlog-pp-cli plan comments edit` | Edit a Wanderlog comment |
| `wanderlog-pp-cli plan comments list` | List comments for a Wanderlog plan |
| `wanderlog-pp-cli plan comments vote` | Vote on a Wanderlog comment |
| `wanderlog-pp-cli plan fill` | Fill an existing Wanderlog trip from a shared or public source plan with dry-run and force safeguards. |
| `wanderlog-pp-cli plan fill-day` | Insert a batch of place stops into one Wanderlog day |
| `wanderlog-pp-cli plan history` | List local undo/redo journal entries for a Wanderlog plan |
| `wanderlog-pp-cli plan inspect` | Inspect a slim itinerary outline; pass --check for counts, formatting, lodging, closures, and schedule mismatches |
| `wanderlog-pp-cli plan note` | Add note blocks to an editable Wanderlog plan |
| `wanderlog-pp-cli plan note add` | Add a note block to a day or section |
| `wanderlog-pp-cli plan outline` | Show a slim itinerary outline: days, section headings, and stop names |
| `wanderlog-pp-cli plan place` | Add place blocks to an editable Wanderlog plan |
| `wanderlog-pp-cli plan place add` | Add a place block to a day or section |
| `wanderlog-pp-cli plan place replace` | Replace only the nested place on an existing itinerary block |
| `wanderlog-pp-cli plan preview` | Inspect a shared plan and report dates, sections, blocks, resources, and clone warnings before any write. |
| `wanderlog-pp-cli plan raw` | Advanced JSON0 operations for unsupported Wanderlog plan fields |
| `wanderlog-pp-cli plan raw op` | Apply an explicit ShareDB JSON0 operation array |
| `wanderlog-pp-cli plan redo` | Redo the latest undone ShareDB edit from the local journal |
| `wanderlog-pp-cli plan reservation` | List, add, edit, or remove reservation and standalone attachment blocks |
| `wanderlog-pp-cli plan reservation add` | Add a flight, lodging, rental car, restaurant, transit, cruise, or standalone attachment block |
| `wanderlog-pp-cli plan reservation edit` | Set or remove a field on a reservation or standalone attachment block |
| `wanderlog-pp-cli plan reservation list` | List reservation and standalone attachment blocks in a Wanderlog plan |
| `wanderlog-pp-cli plan reservation remove` | Remove a reservation or standalone attachment block |
| `wanderlog-pp-cli plan route` | Build or send Wanderlog route optimization requests |
| `wanderlog-pp-cli plan route day-body` | Build a route optimization JSON body from one itinerary day |
| `wanderlog-pp-cli plan route optimize` | Call Wanderlog route optimization |
| `wanderlog-pp-cli plan section` | Add, edit, or delete Wanderlog itinerary sections |
| `wanderlog-pp-cli plan section add-day` | Insert a new day section into an editable Wanderlog plan |
| `wanderlog-pp-cli plan section delete` | Delete an empty Wanderlog section |
| `wanderlog-pp-cli plan section set-field` | Set or remove a field on a Wanderlog section |
| `wanderlog-pp-cli plan section swap-days` | Swap the blocks arrays of two day sections in one JSON0 batch |
| `wanderlog-pp-cli plan sections` | List editable sections and day indexes for a Wanderlog plan |
| `wanderlog-pp-cli plan undo` | Undo the latest applied ShareDB edit from the local journal |
| `wanderlog-pp-cli plan votes` | List place and hotel block upvote counts for a Wanderlog plan |

### `auth`

Manage authentication for Wanderlog (there is no auth login; use setup and set-token)

| Command | What it does |
|---|---|
| `wanderlog-pp-cli auth setup` | Print how to copy connect.sid and save it with auth set-token (no auth login) |
| `wanderlog-pp-cli auth set-token` | Save an API token to the config file |
| `wanderlog-pp-cli auth status` | Show authentication status |
| `wanderlog-pp-cli auth logout` | Clear stored credentials |

### `profile`

Named sets of flags saved for reuse

| Command | What it does |
|---|---|
| `wanderlog-pp-cli profile save` | Save the current invocation's non-default flags as a named profile |
| `wanderlog-pp-cli profile use` | Print the flag values a profile will apply (does not execute anything) |
| `wanderlog-pp-cli profile show` | Show a profile's values as JSON |
| `wanderlog-pp-cli profile list` | List saved profiles |
| `wanderlog-pp-cli profile delete` | Remove a profile |

### `workflow`

Compound workflows that combine multiple API operations

| Command | What it does |
|---|---|
| `wanderlog-pp-cli workflow archive` | Sync all resources to local store for offline access and search |
| `wanderlog-pp-cli workflow status` | Show local archive status and sync state for all resources |

### `feedback`

Record feedback about this CLI (local by default; upstream opt-in)

| Command | What it does |
|---|---|
| `wanderlog-pp-cli feedback list` | List recent feedback entries |

### Endpoint groups (hidden from `--help`, still callable)

Most of these mirror single Wanderlog endpoints one-for-one; `lodging search` is the exception — it is a hand-built command that summarizes the itinerary Lodging-button endpoint into agent-sized candidates (see **Unique Features**). All of them are hidden from `--help` on purpose so agents see the task-shaped surface first; run them by name, and use `wanderlog-pp-cli api` to browse every endpoint.

#### geos

Search Wanderlog destinations and guide-rich geos

| Command | What it does |
|---|---|
| `wanderlog-pp-cli geos autocomplete` | Search destinations by name |
| `wanderlog-pp-cli geos good-guides` | List destinations known to have good public guides |

#### guides

Browse and inspect public Wanderlog guides

| Command | What it does |
|---|---|
| `wanderlog-pp-cli guides comments` | List comments for a public trip or guide key |
| `wanderlog-pp-cli guides distinction` | Get distinction metadata for a public trip or guide key |
| `wanderlog-pp-cli guides get` | Get a public guide or shared trip by view key |
| `wanderlog-pp-cli guides list-for-geo` | List public guides for a destination geo id |

#### lodging

Search Wanderlog lodging and hotel candidates

| Command | What it does |
|---|---|
| `wanderlog-pp-cli lodging search` | Search Wanderlog lodgings using the itinerary Lodging button endpoint |

#### pages

Extract useful HTML fallback pages

| Command | What it does |
|---|---|
| `wanderlog-pp-cli pages explore` | Fetch a destination explore page |
| `wanderlog-pp-cli pages geo-category-page` | Fetch a public geo category list page |
| `wanderlog-pp-cli pages shared-view` | Fetch a public shared itinerary page |

#### places

Search places and retrieve Wanderlog place card details

| Command | What it does |
|---|---|
| `wanderlog-pp-cli places autocomplete` | Search places with Wanderlog's Places API autocomplete request envelope |
| `wanderlog-pp-cli places card` | Get rich place details and card data |
| `wanderlog-pp-cli places details` | Get details for a Google/Wanderlog place id |

#### trips

Read and manage cookie-backed Wanderlog trips

| Command | What it does |
|---|---|
| `wanderlog-pp-cli trips create` | Create a Wanderlog trip for the authenticated account |
| `wanderlog-pp-cli trips delete` | Delete an authenticated Wanderlog trip |
| `wanderlog-pp-cli trips get` | Get an authenticated trip with resources |
| `wanderlog-pp-cli trips home` | List home trips for the authenticated Wanderlog account |


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wanderlog-pp-cli guides get mock-value

# JSON for scripting and agents
wanderlog-pp-cli guides get mock-value --json

# Filter to specific fields
wanderlog-pp-cli guides get mock-value --json --select id,name,status

# Dry run — show the request without sending
wanderlog-pp-cli guides get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wanderlog-pp-cli guides get mock-value --agent
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

## Health Check

```bash
wanderlog-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/wanderlog-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WANDERLOG_COOKIE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wanderlog-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wanderlog-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WANDERLOG_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **plan clone or plan fill refuses to apply.** — Run the same command with --dry-run first, then set WANDERLOG_COOKIE and pass --apply only for an approved target trip.
- **Personal trip commands return an auth or HTML login response.** — Refresh WANDERLOG_COOKIE with a current connect.sid cookie from a logged-in Wanderlog browser session.
- **A public guide or plan key returns incompatibleItineraryConversion.** — Pass --client-schema-version 2; the public tripPlan endpoint requires the current client schema.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://wanderlog.com
- Capture coverage: 12 API entries from 16 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: ssr_embedded_data (85% confidence), rest_json (75% confidence), html_scrape (55% confidence)
- Auth signals: none
- Protection signals: perimeterx (80% confidence)
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: get_geoCategory — Derived from observed GET /api/placesList/geoCategory/{geocategory_id} traffic.; get_placesAPI — Derived from observed GET /api/placesAPI/{placesapi_id} traffic.; list_Paris — Derived from observed GET /api/geo/autocomplete/Paris traffic.; list_autocomplete — Derived from observed GET /api/user/autocomplete/ traffic.; list_comments — Derived from observed GET /api/tripPlans/uzyvvtuwtc/comments traffic.; list_distinction — Derived from observed GET /api/tripPlans/uzyvvtuwtc/distinction traffic.; list_likes — Derived from observed GET /api/tripPlans/likes traffic.; list_sessionStore — Derived from observed GET /api/sessionStore traffic.

Warnings from discovery:
- html_challenge_page: API-looking request returned an HTML login, challenge, or access-denied page.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wanderlog-mcp**](https://github.com/shaikhspeare/wanderlog-mcp) — TypeScript (48 stars)
- [**Wanderlog-to-KML**](https://github.com/danilden1/Wanderlog-to-KML) — Python (10 stars)
- [**wanderlog_importer**](https://github.com/devsuhh/wanderlog_importer) — JavaScript (4 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
