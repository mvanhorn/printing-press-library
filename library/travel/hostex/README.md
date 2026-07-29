# Hostex CLI

**Every Hostex v3 operation, plus a local SQLite mirror that answers cross-property questions no single Hostex call can — occupancy gaps, revenue rollups, and guest-message SLA, offline.**

Hostex has a REST API and an official MCP server but no first-class CLI. This one mirrors all 86 endpoints as typed commands with --json/--select/--dry-run, then syncs reservations, properties, listings, conversations, reviews, tasks and transactions into local SQLite so you can run joins the API can't: which occupied stays have no cleaning task, revenue by property this month, threads unanswered past your SLA, and price-parity gaps across channels. It gets the tricky Hostex plumbing right once: every response is HTTP 200 with error_code in the body, and the multi-layer rate limits return Retry-After.

## Install

The recommended path installs both the `hostex-pp-cli` binary and the `pp-hostex` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hostex
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hostex --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hostex --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hostex --agent claude-code
npx -y @mvanhorn/printing-press-library install hostex --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostex/cmd/hostex-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostex-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hostex --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hostex --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hostex --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hostex --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostex-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HOSTEX_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostex/cmd/hostex-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hostex": {
      "command": "hostex-pp-mcp",
      "env": {
        "HOSTEX_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Hostex authenticates with a Hostex-Access-Token header. Create one in the Host Portal (OpenAPI Settings) with read-only or writable scope; tokens do not expire. Set it as HOSTEX_ACCESS_TOKEN. The server also accepts Authorization: Bearer, but prefer the dedicated header. A read-only token rejects every write with error_code 401.

## Quick Start

```bash
# Health check; confirms token presence and API reachability without spending a real call.
hostex-pp-cli doctor --dry-run

# Pull your reservations and properties into the local SQLite mirror.
hostex-pp-cli sync --resources reservations,properties --db ./hostex.db

# Offline full-text search across synced reservations.
hostex-pp-cli search "smith" --type reservations --db ./hostex.db

# Live query with field projection to keep agent context small.
hostex-pp-cli reservations query --json --select data.reservations.stay_code,data.reservations.status

```

## Unique Features

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

## Usage

Run `hostex-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HOSTEX_CONFIG_DIR`, `HOSTEX_DATA_DIR`, `HOSTEX_STATE_DIR`, or `HOSTEX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HOSTEX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HOSTEX_HOME=/srv/hostex
hostex-pp-cli doctor
```

Under `HOSTEX_HOME=/srv/hostex`, the four dirs resolve to `/srv/hostex/config`, `/srv/hostex/data`, `/srv/hostex/state`, and `/srv/hostex/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `HOSTEX_DATA_DIR` overrides an explicit `--home` for that kind. Use `HOSTEX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HOSTEX_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `hostex-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### automation

Scheduled automation actions (e.g. automated guest messages and scheduled host reviews).

- **`hostex-pp-cli automation delete-action`** - Removes a **waiting** message or review automation plan without running it (same as deleting an upcoming action in the app). Requires a **writable** access token. Only `send_message` and `review` plan types are supported.
- **`hostex-pp-cli automation execute-action`** - Dispatches a **waiting** message or review automation plan immediately (same behavior as executing an upcoming action in the app). For **review** plans, the reservation must have reached check-out date in the operator timezone. Requires a **writable** access token. Only `send_message` and `review` plan types are supported.
- **`hostex-pp-cli automation query-actions`** - Returns scheduled automation **actions** that are waiting to run: either automated **messages** (`type=message`, same scope as the in-app “upcoming message actions” list, within the next 30 days) or automated **reviews** (`type=review`). Filters mirror the internal automation plan lists (keyword, property IDs, time range, channel types, rule event filters). Pagination uses `offset` and `limit` like other v3 list endpoints.

### availabilities

Manage availabilities

- **`hostex-pp-cli availabilities query`** - By sending a request to this endpoint, you can retrieve the availabilities of the properties.
- **`hostex-pp-cli availabilities update`** - Use this endpoint to update property availabilities. <br><br>Please be aware that a successful response indicates only that we have initiated an asynchronous task to handle your submission; it <b>DOES NOT</b> ensure that the channel inventories have been modified successfully. If you wish to view detailed results of the task execution, please visit the [Host Portal](https://hostex.io/app/calendar).

### calendar-share-links

Public share links that expose a read-only calendar (and reservation list) of all or selected properties to anyone holding the link.

- **`hostex-pp-cli calendar-share-links create`** - Create a new public calendar share link. `scope = entire` exposes every property in the operator's account; `scope = partial` exposes only the properties listed in `property_ids`. Each operator can have at most one entire-scope link: requesting one when it already exists simply returns the existing link.
- **`hostex-pp-cli calendar-share-links delete`** - Permanently invalidate a calendar share link. The public URL will start returning a `share link invalid` error to anyone who tries to open it.
- **`hostex-pp-cli calendar-share-links query`** - List the operator's public calendar share links. Each link exposes a read-only calendar (and reservation list) of either every property (`scope = entire`) or a selected subset (`scope = partial`) and is reachable by anyone who has the URL.

### channel-accounts

Manage channel accounts

- **`hostex-pp-cli channel-accounts`** - Query the third-party channel accounts (Airbnb, Booking.com, etc.) that the operator has connected. Each entry exposes the account identity and current authorization status. Use `id` to fetch a single account.

### conversations

Manage conversations

- **`hostex-pp-cli conversations get-details`** - This endpoint is used to retrieve the messages and details of a conversation. <br><br>We are constantly improving our API which could mean that message schema may change. In order to maintain a healthy integration, your Application must parse and ignore unexpected parameters instead of throwing errors.
- **`hostex-pp-cli conversations query`** - This endpoint is used to query the list of conversations regarding guest inquiries.
- **`hostex-pp-cli conversations send-message`** - Send a text or image message to the guest.

### custom-channels

Manage custom channels

- **`hostex-pp-cli custom-channels`** - Query custom channels created from the [Custom Options Page](https://hostex.io/app/settings/custom-options).

### expense-items

Manage expense items

- **`hostex-pp-cli expense-items`** - Query the dictionary of expense item categorizations available to the operator. The returned `id` matches the `item_id` returned by `GET /transactions` for entries with `direction=expense`.

### expense-methods

Manage expense methods

- **`hostex-pp-cli expense-methods`** - Query the dictionary of payment methods available to the operator for expense entries. The returned `id` matches the `payment_method_id` returned by `GET /transactions` for entries with `direction=expense`.

### groups

Manage groups

- **`hostex-pp-cli groups create`** - Create a new property group. Optionally pre-attach properties at creation time via `property_ids`. The group name must be unique within the operator's account.
- **`hostex-pp-cli groups delete`** - Delete a property group. All property-group pivot rows are removed automatically; the underlying properties themselves are unaffected.
- **`hostex-pp-cli groups query`** - You can query property groups by making a request to this endpoint.
- **`hostex-pp-cli groups update`** - Update a property group. Pass `name` to rename it; pass `property_ids` to **replace** the group's full property assignment with the supplied list (pass an empty array to detach all properties). Omitted fields are left unchanged.

### income-items

Manage income items

- **`hostex-pp-cli income-items`** - Query the dictionary of income item categorizations available to the operator. The returned `id` matches the `item_id` returned by `GET /transactions` for entries with `direction=income`.

### income-methods

Manage income methods

- **`hostex-pp-cli income-methods`** - Query the dictionary of payment methods available to the operator for income entries. The returned `id` matches the `payment_method_id` returned by `GET /transactions` for entries with `direction=income`.

### knowledge-bases

AI knowledge base entries for the HostGPT automation assistant. Each entry defines content and the scope (properties/channels) where it applies.

- **`hostex-pp-cli knowledge-bases create`** - Create a new knowledge base entry for the HostGPT automation assistant. The entry defines AI content and the property/channel scope where it applies.
- **`hostex-pp-cli knowledge-bases delete`** - Delete a knowledge base entry by its ID.
- **`hostex-pp-cli knowledge-bases get`** - Retrieve the full details of a single knowledge base entry by its ID.
- **`hostex-pp-cli knowledge-bases query`** - You can query knowledge base entries by making a request to this endpoint. Results are paginated and can be filtered by property or channel.
- **`hostex-pp-cli knowledge-bases update`** - Replace an existing knowledge base entry. This endpoint performs a **full replacement** — `scope_property`, `scope_channel`, `contents` and `is_enable` must all be supplied. Use `GET /knowledge_bases/{id}` first if you only want to change one field.

### listings

Manage listings

- **`hostex-pp-cli listings get-airbnb-price-and-rules`** - Fetch the current pricing, availability rules and booking settings of an Airbnb listing in real time from Airbnb. The response fields mirror the `settings` object accepted by `POST /listings/airbnb/price_and_rules`, so values read here can be written back as-is.

This endpoint calls multiple Airbnb APIs per request and therefore shares the stricter rate limit of write operations (120 requests per minute).
- **`hostex-pp-cli listings get-vrbo-price-and-rules`** - Get the pricing and rules of a Vrbo listing as currently recorded by Hostex. The data is the snapshot Hostex keeps in sync with Vrbo (updated when changes are made through Hostex or the listing is re-synced), not a real-time read from Vrbo. The response fields mirror the `settings` object accepted by `POST /listings/vrbo/price_and_rules`.
- **`hostex-pp-cli listings query`** - Query the listings (third-party properties) synced from the operator's connected channel accounts. A listing is a property as it exists on a specific channel, identified by `listing_id` (the channel-side property id). Filter by `channel_account_id`, `listing_id` and/or `channel_type` to scope results.
- **`hostex-pp-cli listings query-calendars`** - By sending a request to this endpoint, you can retrieve calendar information for multiple listings. This endpoint will return daily details on price, inventory, and restrictions for each listing.
- **`hostex-pp-cli listings update-airbnb-price-and-rules`** - Update the listing-level pricing, fees, booking settings and availability rules of an Airbnb listing. <br><br>Unlike the calendar endpoints, this updates the listing's default settings (not a specific date range). Only the fields you provide are updated; omit a field to leave it unchanged. <br><br>This request is processed synchronously against Airbnb: a successful response means the changes were accepted by Airbnb.
- **`hostex-pp-cli listings update-inventories`** - Update the inventories of channel listings. <br><br>Please be aware that a successful response indicates only that we have initiated an asynchronous task to handle your submission; it <b>DOES NOT</b> ensure that the channel inventories have been modified successfully. If you wish to view detailed results of the task execution, please visit the [Host Portal](https://hostex.io/app/price). <br><br>Furthermore, you should be aware that this endpoint only modifies the listing's inventory and <b>DOES NOT</b> affect the property availability. If the property availability is modified, it may still result in the channel inventory being overwritten again.
- **`hostex-pp-cli listings update-prices`** - Update the prices of channel listings. <br><br>Please be aware that a successful response indicates only that we have initiated an asynchronous task to handle your submission; it <b>DOES NOT</b> ensure that the channel prices have been modified successfully. If you wish to view detailed results of the task execution, please visit the [Host Portal](https://hostex.io/app/price).
- **`hostex-pp-cli listings update-restrictions`** - Update the restrictions of channel listings. <br><br>Please be aware that a successful response indicates only that we have initiated an asynchronous task to handle your submission; it <b>DOES NOT</b> ensure that the channel restrictions have been modified successfully. If you wish to view detailed results of the task execution, please visit the [Host Portal](https://hostex.io/app/price).
- **`hostex-pp-cli listings update-vrbo-price-and-rules`** - Update the listing-level pricing, fees and booking rules of a Vrbo listing. <br><br>Unlike the calendar endpoints, this updates the listing's default settings (not a specific date range). Only the fields you provide are updated; omit a field to leave it unchanged. <br><br>This request is processed synchronously against Vrbo: a successful response means the changes were accepted by Vrbo.

### oauth

Manage oauth

- **`hostex-pp-cli oauth obtain-token`** - This endpoint is used to obtain a new access token using various OAuth 2.0 grant types or refresh an existing token.
- **`hostex-pp-cli oauth revoke-token`** - This endpoint allows clients to revoke an access or refresh token. Deleting/revoking a token will disconnect the Host from your application.

### pricing-ratios

Manage pricing ratios

- **`hostex-pp-cli pricing-ratios`** - Return the per-channel pricing ratio of each OTA listing linked to a property (`property_id`) or a room type (`room_type_id`). Pricing ratio is the **percentage** Hostex multiplies a base (property/room-type level) price by to derive the actual price pushed to each listing. Use this endpoint at the skill layer to compose a 'change price by property / room type' workflow: read the ratios, compute `target_price = round(base_price * ratio / 100)` for every non-readonly listing, then call `POST /listings/prices` once per listing. Listings marked `readonly: true` are silently controlled by the channel (e.g. Airbnb child rate plans) and cannot be repriced via `POST /listings/prices`; skip them.

### properties

Manage properties

- **`hostex-pp-cli properties create-property`** - Create a new property (room) under the current operator. Hostex properties are physical room units; OTA listings can be attached to a property afterwards via the Hostex Host Portal. The newly created property has no address, channels or pictures and starts with the operator's default check-in / check-out time settings. Subject to the property quantity limit of the operator's subscription.
- **`hostex-pp-cli properties query`** - You can query properties by making a request to this endpoint.

### reservation-tags

Manage the operator's reservation tag dictionary (the tags that can be applied to reservations).

- **`hostex-pp-cli reservation-tags create`** - Create a new reservation tag in the operator's dictionary. Color is auto-assigned from the Hostex palette. Names must be unique across both system default tags and the operator's own tags (a soft-deleted tag with the same name will be restored). Operators are capped at 500 tags.
- **`hostex-pp-cli reservation-tags delete`** - Delete one of the operator's custom reservation tags. System default tags (`is_default = true`) cannot be deleted via this API and will return 404. Deleting a tag also removes it from any reservations it had been applied to.
- **`hostex-pp-cli reservation-tags query`** - List the operator's reservation tag dictionary. Includes both system default tags (`is_default = true`, shared across all operators) and the operator's own custom tags. These are the tags that `POST /reservations/{stay_code}/tags` can attach to a reservation.

### reservations

Manage reservations

- **`hostex-pp-cli reservations cancel`** - Cancel a direct booking reservation in Hostex. Note that this endpoint does not support the cancellation of channel bookings.
- **`hostex-pp-cli reservations create`** - Create a reservation (Direct Booking) in Hostex.
- **`hostex-pp-cli reservations query`** - You can query reservations by making a request to this endpoint.
- **`hostex-pp-cli reservations update-basic-info`** - Update basic information of a stay including guest details, dates, pricing, and other attributes.

### reviews

Manage reviews

- **`hostex-pp-cli reviews create`** - Create review or reply for a reservation.
- **`hostex-pp-cli reviews query`** - Query reviews like the [Reviews Page](https://hostex.io/app/reviews).

### room-types

Manage room types

- **`hostex-pp-cli room-types create`** - Create a new room type under the current operator. A room type is a group of interchangeable properties (rooms) sold as a single inventory pool. Optionally link existing properties at creation time via `property_ids` — each linked property must not already belong to another room type. Subject to the room type quantity limit of the operator's subscription (capped by the property quantity limit) and not available on Basic editions.
- **`hostex-pp-cli room-types query`** - You can query room types by making a request to this endpoint.

### staffs

Manage staffs

- **`hostex-pp-cli staffs create`** - Create a schedule staff. The staff is created as active by default. Use `property_ids` to limit the staff to specific properties; the staff will be created with access to all properties when omitted.

For international operators, `mobile` must be in `+<country code> <number>` format (e.g. `+86 13800138000`).
- **`hostex-pp-cli staffs delete`** - Delete a staff permanently along with their property assignments. Returns 404 if the staff does not exist or is not accessible to the current operator.
- **`hostex-pp-cli staffs query`** - You can query schedule staffs (cleaners / operators / receptionists, etc.) by making a request to this endpoint.
- **`hostex-pp-cli staffs update`** - Update an existing staff. All fields are optional; only the supplied fields are changed. Passing `property_ids` replaces the staff's full property assignment list (use an empty array to clear). Use `is_active` to enable or disable the staff.

### tags

Manage tags

- **`hostex-pp-cli tags create`** - Create a new property tag. Optionally pre-attach properties via `property_ids` and / or room types via `room_type_ids`. The name must be unique within the operator's account (soft-deleted same-name tags are restored). Color is auto-assigned from the Hostex palette unless one of the allowed hex strings is supplied. Operators are capped at 500 property tags.
- **`hostex-pp-cli tags delete`** - Delete a property tag. All property-tag and room-type-tag pivot rows are removed automatically; the underlying properties / room types themselves are unaffected.
- **`hostex-pp-cli tags query`** - You can query tags by making a request to this endpoint.
- **`hostex-pp-cli tags update`** - Update a property tag. Pass `name` and / or `color` to change the tag itself; pass `property_ids` and / or `room_type_ids` to **replace** the tag's full assignment list. Pass an empty array to detach all properties / all room types. Omitted fields are left unchanged.

### tasks

Schedule tasks such as cleaning, maintenance, reception, housekeeping and others.

- **`hostex-pp-cli tasks create`** - Create a schedule task. The task may be linked to a property (`property_id`), to a specific stay (`stay_code`), and/or assigned to a staff (`staff_id`); all are optional. When `stay_code` is provided the task is linked to the reservation; `property_id` is inferred from the stay when omitted (and must match the stay's property when both are supplied). The `type` selects the task category and `level` is only meaningful for cleaning tasks.
- **`hostex-pp-cli tasks delete`** - Delete a task permanently. Returns 404 if the task does not exist or is not accessible to the current operator.
- **`hostex-pp-cli tasks query`** - You can query schedule tasks (cleaning / maintain / reception / housekeeping / others) by making a request to this endpoint.
- **`hostex-pp-cli tasks update`** - Update an existing task. All fields are optional; only the supplied fields are changed. Pass `property_id=0` or `staff_id=0` to detach the task from the related property or staff.

### transactions

Manage transactions

- **`hostex-pp-cli transactions create`** - Record a new income or expense entry. What the entry is linked to is inferred from the request:
- provide `stay_code` to record an entry against a specific stay,
- provide `property_id` to record an entry against a specific property,
- provide neither to record an operator-level entry (not tied to any specific property or stay; only available to the master operator).

`stay_code` and `property_id` are mutually exclusive.

The `direction` field decides whether the entry is an `income` or an `expense`, which in turn determines which dictionaries are used for `item_id` (`GET /income_items` or `GET /expense_items`) and `payment_method_id` (`GET /income_methods` or `GET /expense_methods`). The `amount` is always provided as a positive number; the sign is derived from `direction`. The `currency` accompanies the amount and must be supplied unless `stay_code` is provided, in which case it is inherited from the reservation order and, if you do provide it, it must match the order's currency.
- **`hostex-pp-cli transactions delete`** - Delete a transaction entry. The operation is irreversible. Returns 404 if the entry does not exist or is not accessible to the current operator.
- **`hostex-pp-cli transactions query`** - Query income and expense entries (also known as `transactions`) recorded against the operator, properties or reservations.

The response provides each entry with its categorization (`item_id` / `item_name`) and payment method (`payment_method_id` / `payment_method_name`). The values of `item_id` and `payment_method_id` reference the dictionaries returned by `GET /income_items`, `GET /expense_items`, `GET /income_methods` and `GET /expense_methods` (which dictionary applies depends on `direction`).
- **`hostex-pp-cli transactions update`** - Update an existing transaction entry. Only the fields listed below can be modified. The entry's `direction`, link target (related property / reservation / operator) and `currency` are immutable; if you need to change any of these, delete the entry and create a new one.

### webhooks

Manage webhooks

- **`hostex-pp-cli webhooks create`** - Create a webhook.
- **`hostex-pp-cli webhooks delete`** - You can only delete webhooks created by your own app if they are manageable. Attempting to delete non-manageable webhooks from other apps will result in a 403 error.
- **`hostex-pp-cli webhooks query`** - Query Webhooks like the [Webhooks Page](https://hostex.io/app/api/web-hooks).
- **`hostex-pp-cli webhooks update`** - Update the url or event subscriptions for a webhook. You can only update webhooks created by your own app.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15

# JSON for scripting and agents
hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --json

# Filter to specific fields
hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --json --select id,name,status

# Dry run — show the request without sending
hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hostex-pp-cli availabilities query --property-ids example-value --start-date 2026-01-15 --end-date 2026-01-15 --agent
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
hostex-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `hostex-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/hostex-pp-cli/config.toml`; `--home`, `HOSTEX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HOSTEX_ACCESS_TOKEN` | per_call | No | Set to your API credential. |
| `HOSTEX_HOSTEX_ACCESS_TOKEN` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `hostex-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `hostex-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HOSTEX_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Calls return HTTP 200 but nothing works** — Hostex puts the real result in error_code; branch on it, not the HTTP status. error_code 0 means success.
- **"Invalid access token" on the first call** — Check the header spelling Hostex-Access-Token (with the dash) and that HOSTEX_ACCESS_TOKEN is exported.
- **Writes fail with error_code 401 though the token is valid** — Your token is read-only scope. Create a writable token in the Host Portal; scope cannot be changed after creation.
- **error_code 429** — Rate limit hit (per-token, per-endpoint, or per-thread). Back off for the seconds in the Retry-After header.
- **error_code 420** — Account/subscription problem (expired, or Basic edition using a Pro feature). Only the host can fix it in the portal; do not retry.
