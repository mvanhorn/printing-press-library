# Wanderlog CLI

**Clone shared Wanderlog plans into editable trips, then mine guides and places for agent-ready planning.**

Use anonymous Wanderlog endpoints to read shared/public plans, guides, geos, places, and category lists. With WANDERLOG_COOKIE, plan clone/fill and fine-grained plan editor commands create or update private trips using ShareDB-backed writes; dry-run and preview modes make every mutation inspectable before apply.

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

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

Public guide, geo, place, category, shared-plan preview, and shared itinerary reads work without credentials. Creating, filling, or fine-grained editing of a target trip requires WANDERLOG_COOKIE, including the current connect.sid cookie value in Cookie-header format. ShareDB apply mode is gated behind --apply and should be dogfooded only against an approved disposable target trip.

## Quick Start

```bash
# Verify the generated CLI and config path before making live requests.
wanderlog-pp-cli doctor --dry-run

# Inspect the source plan shape before copying anything.
wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --agent

# Preview the new-trip clone operation without writing to Wanderlog.
wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --dry-run --agent

# Resolve a destination geo id for guide and place workflows.
wanderlog-pp-cli geos autocomplete Paris --agent --select id,name,fullName,slug

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Plan cloning and fill
- **`plan clone`** — Create a new Wanderlog trip from a shared or public source plan, then fill it with the source plan template.

  _Use this when the user wants to turn an existing shared Wanderlog plan into their own new editable trip._

  ```bash
  wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --dry-run --agent
  ```
- **`plan fill`** — Fill an existing Wanderlog trip from a shared or public source plan with dry-run and force safeguards.

  _Use this when the user already created a target trip and wants to populate it from a shared template._

  ```bash
  wanderlog-pp-cli plan fill --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --target-key exampletarget --dry-run --agent
  ```
- **`plan preview`** — Inspect a shared plan and report dates, sections, blocks, resources, and clone warnings before any write.

  _Use this before clone/fill to confirm what will be copied and whether credentials are needed._

  ```bash
  wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --agent
  ```

### Agentic plan editing
- **`plan sections`** — List editable section indexes, day numbers, section ids, dates, and block counts for a plan.

  _Use this before editing so an agent targets the right day or section._

  ```bash
  wanderlog-pp-cli plan sections --target-key exampletargetkey --agent
  ```
- **`plan note add`** — Add a note block to a selected day or section through ShareDB.

  _Use this to add reminders, reservations, constraints, or planning notes._

  ```bash
  wanderlog-pp-cli plan note add --target-key exampletargetkey --day 1 --text 'Book ferry tickets' --dry-run --agent
  ```
- **`plan place add`** — Add a real place block to a selected day or section from a Google/Wanderlog place id, or from a query with location bias.

  _Use this to build an itinerary stop by stop._

  ```bash
  wanderlog-pp-cli plan place add --target-key exampletargetkey --day 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --text 'Sunset photos' --dry-run --agent
  ```
- **`plan block move`** — Move a note or place block within or across days.

  _Use this to reorder an itinerary after adding candidate stops._

  ```bash
  wanderlog-pp-cli plan block move --target-key exampletargetkey --day 1 --block-id 123456789 --to-day 2 --to-position 0 --dry-run --agent
  ```
- **`plan block delete`** — Delete a note or place block from a selected day or section.

  _Use this to clean up test blocks or remove rejected candidates._

  ```bash
  wanderlog-pp-cli plan block delete --target-key exampletargetkey --day 1 --block-id 123456789 --dry-run --agent
  ```
- **`plan block edit-text`** — Replace the rich-text note attached to an existing block.

  _Use this to revise stop notes, reservation notes, or reminders after a block exists._

  ```bash
  wanderlog-pp-cli plan block edit-text --target-key exampletargetkey --day 2 --block-id 123456789 --text 'Arrive by 09:30' --dry-run --agent
  ```
- **`plan block set-field`** — Set or remove a non-protected field on a block, including newly observed fields from live plan data.

  _Use this for schedule or metadata fields once you have inspected the plan shape; use `--json-value` for numbers, booleans, arrays, or objects._

  ```bash
  wanderlog-pp-cli plan block set-field --target-key exampletargetkey --day 2 --block-id 123456789 --field startTime --value '09:30' --dry-run --agent
  ```
- **`plan block schedule`** — Set or clear first-class schedule fields on an existing block.

  _Use this to turn loose stops into a timed itinerary._

  ```bash
  wanderlog-pp-cli plan block schedule --target-key exampletargetkey --day 2 --block-id 123456789 --start 09:30 --duration-minutes 90 --dry-run --agent
  ```
- **`plan block attachment list/add/remove`** — List, add, or remove attachment metadata on a block.

  _Use this for tickets, booking links, PDFs, and other planning artifacts._

  ```bash
  wanderlog-pp-cli plan block attachment add --target-key exampletargetkey --day 2 --block-id 123456789 --title Tickets --url https://example.com/tickets.pdf --dry-run --agent
  ```
- **`plan checklist`** — Add checklist blocks and add/check/remove checklist items.

  _Use this for packing lists, booking tasks, and shared planning todos._

  ```bash
  wanderlog-pp-cli plan checklist add --target-key exampletargetkey --day 1 --title Packing --item Passport --item Sunscreen --dry-run --agent
  ```
- **`plan comments`** — List, add, edit, delete, or vote on Wanderlog plan comments using the confirmed comments API.

  _Use this to read friend discussion, ask questions, and explain agent-made edits._

  ```bash
  wanderlog-pp-cli plan comments add --target-key exampletargetkey --text 'I added a timed draft for day 2; please review.' --dry-run --agent
  ```
- **`plan collaborators`** — Inspect collaborator/share metadata, list pending invites, send email/user invites, add/remove collaborators by user id, and create share keys.

  _Use this for account-level collaboration tasks around the shared plan. Keep invite sends on `--dry-run` until the recipient list is explicit._

  ```bash
  wanderlog-pp-cli plan collaborators --target-key exampletargetkey --agent
  wanderlog-pp-cli plan collaborators invites --target-key exampletargetkey --agent
  wanderlog-pp-cli plan collaborators invite --target-key exampletargetkey --email friend@example.com --message 'Want to help tune Okinawa?' --dry-run --agent
  ```
- **`plan budget`** — Summarize, export, and edit Wanderlog trip budget expenses and settlement payments.

  _Use this to set the trip budget, add costs with categories/splits/payers, link expenses to itinerary blocks, and record settlement payments. Mutating budget commands use ShareDB and are covered by `plan undo`/`plan redo`._

  ```bash
  wanderlog-pp-cli plan budget summary --target-key exampletargetkey --agent
  ```
- **`plan route`** — Build route optimization request bodies or call Wanderlog's optimizeRoute endpoint.

  _Use this to compute a better order/travel path, then apply block moves deliberately._

  ```bash
  wanderlog-pp-cli plan route day-body --target-key exampletargetkey --day 2 --agent
  ```
- **`plan history`, `plan undo`, `plan redo`** — List, preview, undo, and redo local-journaled ShareDB itinerary edits.

  _Use this as the safety net after applied itinerary edits. Undo/redo defaults to preview; pass `--apply` to mutate the plan._

  ```bash
  wanderlog-pp-cli plan history --target-key exampletargetkey --agent
  wanderlog-pp-cli plan undo --target-key exampletargetkey --apply --agent
  wanderlog-pp-cli plan redo --target-key exampletargetkey --apply --agent
  ```
- **`plan section add-day`** — Insert a new day section and update trip day/date bounds.

  _Use this when the itinerary needs another travel day._

  ```bash
  wanderlog-pp-cli plan section add-day --target-key exampletargetkey --date 2026-09-07 --heading 'Bonus day' --dry-run --agent
  ```
- **`plan section set-field`** — Set or remove a field on a section, including day headings or rich text.

  _Use this to label days or adjust section-level notes._

  ```bash
  wanderlog-pp-cli plan section set-field --target-key exampletargetkey --day 2 --field heading --value 'Northern Okinawa' --dry-run --agent
  ```
- **`plan section delete`** — Delete an empty section and update trip day/date bounds.

  _Use this to remove unused days or empty sections after reviewing the target._

  ```bash
  wanderlog-pp-cli plan section delete --target-key exampletargetkey --day 9 --dry-run --agent
  ```
- **`plan raw op`** — Preview or apply an explicit ShareDB JSON0 operation array.

  _Use this only as an escape hatch for fields not yet covered by a named command, after inspecting live plan shape and dry-run output._

  ```bash
  wanderlog-pp-cli plan raw op --target-key exampletargetkey --op '[{"p":["title"],"od":"Old title","oi":"New title"}]' --dry-run --agent
  ```

## Recipes


### Preview a shared Okinawa plan

```bash
wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --agent
```

Check dates, sections, blocks, and copy warnings without credentials.

### Dry-run a new-trip clone

```bash
wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --dry-run --agent
```

Show the new trip that would be created and filled before writing to Wanderlog.

### Dry-run filling an existing trip

```bash
wanderlog-pp-cli plan fill --source-url https://wanderlog.com/plan/omxsrbpstldoniqa/trip-to-okinawa-prefecture/shared --target-key exampletarget --dry-run --agent
```

Compare source and target copy actions before applying to an existing trip.

### Agentically add and adjust stops in a private clone

```bash
wanderlog-pp-cli plan sections --target-key exampletargetkey --agent
wanderlog-pp-cli plan note add --target-key exampletargetkey --day 1 --text 'Book ferry tickets' --dry-run --agent
wanderlog-pp-cli plan place add --target-key exampletargetkey --day 1 --place-id ChIJLU7jZClu5kcR4PcOOO6p3I0 --text 'Sunset photos' --dry-run --agent
wanderlog-pp-cli plan block move --target-key exampletargetkey --day 1 --block-id 123456789 --to-day 2 --to-position 0 --dry-run --agent
wanderlog-pp-cli plan block edit-text --target-key exampletargetkey --day 2 --block-id 123456789 --text 'Arrive by 09:30' --dry-run --agent
wanderlog-pp-cli plan block schedule --target-key exampletargetkey --day 2 --block-id 123456789 --start 09:30 --duration-minutes 90 --dry-run --agent
wanderlog-pp-cli plan checklist add --target-key exampletargetkey --day 1 --title Packing --item Passport --item Sunscreen --dry-run --agent
wanderlog-pp-cli plan comments list --target-key exampletargetkey --agent
wanderlog-pp-cli plan route day-body --target-key exampletargetkey --day 2 --agent
wanderlog-pp-cli plan section set-field --target-key exampletargetkey --day 2 --field heading --value 'Northern Okinawa' --dry-run --agent
```

Inspect every edit first. Add `--apply` only after the user approves the target plan and operation. Use `plan budget` for costs and settlements. Use `plan raw op` only when named commands cannot express an observed Wanderlog field.

### Extract a public guide for agent planning

```bash
wanderlog-pp-cli guides get omxsrbpstldoniqa --client-schema-version 2 --agent --select tripPlan.title,tripPlan.itinerary.sections,resources.placeMetadata
```

Pull only the guide/plan structure and place metadata an agent needs.

## Usage

Run `wanderlog-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Read cookie-backed account metadata

- **`wanderlog-pp-cli account`** - Get the current Wanderlog account for WANDERLOG_COOKIE

### geos

Search Wanderlog destinations and guide-rich geos

- **`wanderlog-pp-cli geos autocomplete`** - Search destinations by name
- **`wanderlog-pp-cli geos good-guides`** - List destinations known to have good public guides

### guides

Browse and inspect public Wanderlog guides

- **`wanderlog-pp-cli guides comments`** - List comments for a public trip or guide key
- **`wanderlog-pp-cli guides distinction`** - Get distinction metadata for a public trip or guide key
- **`wanderlog-pp-cli guides get`** - Get a public guide or shared trip by view key
- **`wanderlog-pp-cli guides list-for-geo`** - List public guides for a destination geo id

### pages

Extract useful HTML fallback pages

- **`wanderlog-pp-cli pages explore`** - Fetch a destination explore page
- **`wanderlog-pp-cli pages geo-category-page`** - Fetch a public geo category list page
- **`wanderlog-pp-cli pages shared-view`** - Fetch a public shared itinerary page

### place_lists

Read destination category and list pages

- **`wanderlog-pp-cli place-lists <geo_category_id>`** - Get places in a destination category list

### places

Search places and retrieve Wanderlog place card details

- **`wanderlog-pp-cli places autocomplete`** - Search places with Wanderlog's Places API autocomplete request envelope
- **`wanderlog-pp-cli places card`** - Get rich place details and card data
- **`wanderlog-pp-cli places details`** - Get details for a Google/Wanderlog place id

### session

Read anonymous Wanderlog session preferences

- **`wanderlog-pp-cli session`** - Get current anonymous session preferences

### trips

Read and manage cookie-backed Wanderlog trips

- **`wanderlog-pp-cli trips create`** - Create a Wanderlog trip for the authenticated account
- **`wanderlog-pp-cli trips delete`** - Delete an authenticated Wanderlog trip
- **`wanderlog-pp-cli trips get`** - Get an authenticated trip with resources
- **`wanderlog-pp-cli trips home`** - List home trips for the authenticated Wanderlog account


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
