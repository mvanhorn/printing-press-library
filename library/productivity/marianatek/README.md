# Mariana Tek CLI

**Every Mariana Tek booking feature, plus multi-tenant search, cancellation watching, and an offline class catalog no other tool has.**

marianatek is the first CLI and MCP server for the Mariana Tek booking platform that powers hundreds of boutique-fitness, yoga, sauna, and wellness studios. Beyond mirroring every Customer API endpoint, it adds a local SQLite catalog, FTS5 search across class sessions, cancellation watching (the API exposes no waitlist signal), and joins across tenants that the per-studio iframe widget can't perform.

## Install

The recommended path installs both the `marianatek-pp-cli` binary and the `pp-marianatek` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install marianatek
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install marianatek --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/marianatek-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-marianatek --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-marianatek --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-marianatek skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-marianatek. The skill defines how its required CLI can be installed.
```

## Authentication

Each tenant is its own brand subdomain ({tenant}.marianatek.com), so authentication is per-tenant. Run `marianatek login --tenant kolmkontrast` to launch the OAuth2 Authorization Code flow in your browser; the refresh token lands at `~/.config/marianatek/<tenant>.token.json` (0600). The HTTP client transparently refreshes on 401. For headless / CI use, `marianatek login --tenant <slug> --email --password` performs an OAuth2 password grant. Logout with `marianatek logout --tenant <slug>` deletes the token file.

## Quick Start

```bash
# Open the OAuth flow in your browser; refresh token is saved to disk so subsequent commands run without prompts.
marianatek login --tenant kolmkontrast


# Cache the next two weeks of class sessions + your reservations into the local store. This populates every transcendence command.
marianatek sync --tenant kolmkontrast --window 14d


# Find candidate classes via FTS5 over the synced catalog. --agent emits structured JSON ready for an LLM to read.
marianatek search "vinyasa morning" --agent


# Poll a sold-out class and book the spot the instant it opens. NDJSON output streams events as they happen.
marianatek watch 84212 --interval 60s --auto-book --agent


# Surface credit packs about to lapse so you don't waste them. Suitable as a weekly cron.
marianatek expiring --within 720h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cancellation hunting
- **`watch`** — Poll a sold-out class (or a filter) and emit a structured event the moment a spot opens — optionally auto-book in the same tick.

  _Reach for this whenever the user wants a sold-out class. NDJSON output lets the agent decide what to do on cancellation events._

  ```bash
  marianatek watch 84212 --interval 60s --auto-book --agent
  ```
- **`book-regular`** — Resolve a natural slot key ('tue-7am-vinyasa') against the regulars table, find the next matching upcoming class, and book it.

  _The agent shorthand for repeat-booking workflows. Pair with regulars to discover the slot keys this user has._

  ```bash
  marianatek book-regular --slot "tue-7am-vinyasa" --auto --agent
  ```

### Multi-tenant power
- **`schedule`** — Query the merged class catalog across every tenant you have logged into, with structured filters that the per-tenant iframe widget cannot offer.

  _When a user belongs to multiple studios, this is the only way to see all options at once. Always pair with --select to keep the response narrow._

  ```bash
  marianatek schedule --any-tenant --type vinyasa --before 07:00 --window 7d --agent --select tenant,location,start_time,instructor
  ```
- **`conflicts`** — Read reservations across all logged-in tenants, optionally pull an exported ICS calendar, and flag overlapping intervals or insufficient buffer.

  _Critical for multi-tenant users with packed calendars. Use --buffer to enforce minimum gap between back-to-back sessions._

  ```bash
  marianatek conflicts 2026-05-15 --ics ~/Downloads/calendar.ics --buffer 30m --agent
  ```
- **`search`** — Full-text search across cached class sessions, instructors, locations, and class types — ranks by BM25 and respects --any-tenant.

  _Use when the user's request is fuzzy. Always emit --select-narrowed output so you can fit several results in context._

  ```bash
  marianatek search "vinyasa soho morning" --agent --select tenant,start_time,instructor,location,spots_left
  ```

### Personal insights
- **`regulars`** — Group your local reservation history by instructor, class type, time-of-day, day-of-week, or location and rank the dimensions you actually use.

  _Use this to ground recommendations in what the user actually does, rather than what the API thinks they like._

  ```bash
  marianatek regulars --by instructor --top 5 --agent
  ```
- **`expiring`** — Surface credit packs and memberships about to lapse, with remaining balance and the classes that would consume them best.

  _Pre-empt wasted credits. Run as a weekly cron; if non-empty, the user has action to take._

  ```bash
  marianatek expiring --within 720h --agent
  ```

### Operations
- **`doctor`** — Per-tenant token expiry, last-sync timestamp, row counts, and a live class-probe for reachability.

  _Run before any cron job or watch session. If any tenant is in 'expired' state, refresh tokens before scheduling._

  ```bash
  marianatek doctor --agent
  ```

## Usage

Run `marianatek-pp-cli --help` for the full command reference and flag list.

## Commands

### app-version-metadatas

Manage app version metadatas

- **`marianatek-pp-cli app-version-metadatas list`** - View a list of the latest version details for Mariana Tek mobile applications, including if an update is mandatory for a given application.
- **`marianatek-pp-cli app-version-metadatas retrieve`** - Get version metadata details about a single customer application, including version number.

### appointments

Manage appointments

- **`marianatek-pp-cli appointments categories-list`** - View a list of appointment service categories available for booking.
- **`marianatek-pp-cli appointments services-payment-options-retrieve`** - View all available payment options (credits) that can be used to book this service at the specified location. Returns separate lists for options that can be used for user bookings versus guest bookings. Each payment option includes detailed information about the credit, including any restrictions or limitations.
- **`marianatek-pp-cli appointments services-schedule-filters-retrieve`** - Returns the available filter choices (service names, instructors, classrooms) for appointment services at the specified location(s). Location scope is set by providing exactly one of location_id (one or more locations) or region_id (all locations in that region). Filters are based on location only (not date), so all services, instructors, and classrooms configured at the selected location(s) are returned regardless of current availability.
- **`marianatek-pp-cli appointments services-slots-retrieve`** - Retrieve computed availability slots for one or more services at one or more locations within a date range. Returns availability data organized by date, service, provider, and time slots. Each slot includes booking and availability counts. Each provider includes enriched details: instructor_details (for provider_type employee) with id, name, bio, photo_urls, and social links, or classroom_details (for provider_type classroom) with id and name, plus location_name.  Employee` providers are omitted when the instructor is inactive or not found in this tenant. Either service_id or service_category_id must be provided. Provide exactly one of location_id or region_id (mutually exclusive).

### classes

Manage classes

- **`marianatek-pp-cli classes list`** - View a paginated list of classes. Does not include private classes. Filter the class list using a chain of one or more of the following query parameters `min_start_date`, `max_start_date`, `min_start_time`, `max_start_time`, `region`, `location`, `is_live_stream`, `classroom`, `instructor`. Eg: **api/customer/v1/classes?location={location_id}&max_start_date={datetime_string}**
- **`marianatek-pp-cli classes retrieve`** - View details about a single class. If this is a pick-a-spot class, the response will include layout data that can be used to render a map of the class.

### config

Manage config

- **`marianatek-pp-cli config retrieve`** - Retrieve brand configuration settings including appearance, contact info, and business settings.

### countries

Manage countries

- **`marianatek-pp-cli countries list`** - View a list of countries that can be associated with user addresses.
- **`marianatek-pp-cli countries retrieve`** - View information about a single country.

### customer-feedback

Manage customer feedback

- **`marianatek-pp-cli customer-feedback show-csat-retrieve`** - Check if the user should be shown a CSAT survey for an instructor. Returns whether the user has an active survey available for the specified instructor.
- **`marianatek-pp-cli customer-feedback submit-csat-create`** - Submit a CSAT (Customer Satisfaction) survey response for an instructor.

### legal

Manage legal

- **`marianatek-pp-cli legal retrieve`** - Retrieve legal configuration information for a brand including terms of service, privacy policy, and other legal documents.

### locations

Manage locations

- **`marianatek-pp-cli locations list`** - View a paginated list of available locations.
- **`marianatek-pp-cli locations retrieve`** - View detailed information about a specific location.

### me

Manage me

- **`marianatek-pp-cli me account-create`** - After a new user is created, your application should use OAuth2.0 to obtain an access token for this user. To make sure this user does not have to enter their credentials during the OAuth flow, a short-lived token is returned in the response headers, for example `X-Ephemeral-Token={EPHEMERAL}`. This can be used with the query parameter `ephemeral_token` in the `/o/authorize` request to bypass the login step. Ephemeral tokens only remain active for a few minutes and can only be used once.
- **`marianatek-pp-cli me account-destroy`** - Delete (archive) the user's account. Requires the user to submit password to continue.
- **`marianatek-pp-cli me account-partial-update`** - Update profile information for the current user.
- **`marianatek-pp-cli me account-redeem-giftcard-create`** - Redeeming a gift card will add the full value of the card with the matching redemption code to the user's account balance.
- **`marianatek-pp-cli me account-retrieve`** - View account details for the current user.
- **`marianatek-pp-cli me account-update-communications-preferences-create`** - Opt a user in or out of SMS communications.
- **`marianatek-pp-cli me account-upload-profile-image-create`** - Uploading a profile image to a user's account `(as multipart/form-data)`. Image file size should be no larger than 1MB. Supported file types - ico, gif, jpg, jpeg, png, svg.
- **`marianatek-pp-cli me achievements-retrieve`** - View achievement details for the current user.
- **`marianatek-pp-cli me appointments-bookings-create`** - Book a new appointment for the authenticated customer. The customer is derived from the auth token; do not pass customer_id in the request body. The service duration is resolved automatically from the service_id.
- **`marianatek-pp-cli me appointments-bookings-list`** - View a list of a user's appointments. Filter the bookings using `start_date`, `end_date`, and `is_upcoming` query parameters. If `is_upcoming` is true, defaults to upcoming appointments (today to 30 days later). If `is_upcoming` is false, defaults to historical appointments (30 days ago to today). is_upcoming will further filter the results for today's appointments based on the current time.If dates are not provided and `is_upcoming` is not specified, defaults to today and 30 days later.
- **`marianatek-pp-cli me appointments-bookings-retrieve`** - Retrieve details of a specific user appointment using a composite ID. The ID parameter should be in numeric format: {booking_id}{YYYYMMDDHHMM} where the last 12 digits are always the datetime in UTC (YYYYMMDDHHMM format), and everything before that is the booking_id. Example: 315202509021455
- **`marianatek-pp-cli me credit-cards-create`** - Credit cards create
- **`marianatek-pp-cli me credit-cards-destroy`** - Credit cards destroy
- **`marianatek-pp-cli me credit-cards-destroy-2`** - Credit cards destroy 2
- **`marianatek-pp-cli me credit-cards-list`** - Credit cards list
- **`marianatek-pp-cli me credit-cards-partial-update`** - Credit cards partial update
- **`marianatek-pp-cli me credit-cards-partial-update-2`** - Credit cards partial update 2
- **`marianatek-pp-cli me credit-cards-retrieve`** - Credit cards retrieve
- **`marianatek-pp-cli me credit-cards-update`** - Credit cards update
- **`marianatek-pp-cli me credits-list`** - View all credit packages that the user has purchased. To filter out credits that can no longer be used, use the query parameter `is_active=True`. Similarly, the query parameter `is_active=False` will return only packages that have been completely used or have expired.
- **`marianatek-pp-cli me credits-retrieve`** - Retrieve details of a specific user credit.
- **`marianatek-pp-cli me get-failed-authentication-renewals`** - Get memberships that are currently in payment failure status where the basket attempting the renewal is associated with a payment transaction error with code 'authentication_required'.
- **`marianatek-pp-cli me memberships-cancel-membership-create`** - Cancel a membership_instance.

This cancels a membership instance, but allows it to be used until the end of
the current payment interval.

Valid values for the (optional) cancellation_reason param, as well as customer friendly display strings,
 can be obtained from the membership_cancellation_reasons endpoint.

{
    "cancellation_reason": "injury" (optional)
    "cancellation_note": "I fell off the reformer machine and twisted my ankle" (optional)
}
- **`marianatek-pp-cli me memberships-list`** - View all memberships that the current user has purchased. To filter out memberships that can no longer be used, use the query parameter `is_active=True`. Similarly, the query parameter `is_active=False` will return only memberships that have expired.
- **`marianatek-pp-cli me memberships-membership-cancellation-reasons-retrieve`** - Get the list of cancellation reasons to be displayed in customer facing applications.
These are the valid reasons which can be used as cancellation_reason on the cancel_membership endpoint
Example response:
{
    "cancellation_reasons": [
        {
            "value": "injury",
            "display": "Injury"
        },
        {
            "value": "moving",
            "display": "Moving"
        },
        {
            "value": "cost",
            "display": "Cost"
        },
        {
            "value": "other",
            "display": "Other"
        }
    ]
}
- **`marianatek-pp-cli me memberships-renewal-intent-create`** - Creates a renewal basket and payment intent for a membership instance in payment failure status. Optionally pass a bankcard_id to use for the renewal payment; if not provided, the system will search for an available payment method. Returns the Stripe PaymentIntent ID, client secret, and other relevant information to complete the payment on-session.
- **`marianatek-pp-cli me memberships-retrieve`** - View details about a single membership belonging to the current user.
- **`marianatek-pp-cli me metrics-class-count-retrieve`** - Calculates the number of classes taken for a user within a specified date range. Also returns the user's percentile band (Top 1%, Top 5%, Top 10%, Top 25%, Top 50%, or Below 50%) based on pre-calculated thresholds stored in Constance. The percentile indicates where the user stands relative to all other users in terms of classes taken.
- **`marianatek-pp-cli me metrics-longest-weekly-streak-retrieve`** - Calculates the longest weekly class streak for a user within a specified date range.
- **`marianatek-pp-cli me metrics-most-active-month-retrieve`** - Calculates the month when the user was most active in attending classes within a specified date range.
- **`marianatek-pp-cli me metrics-most-popular-weekday-retrieve`** - Calculates the weekday when the user most frequently attends classes within a specified date range.
- **`marianatek-pp-cli me metrics-most-recent-completed-challenge-retrieve`** - Calculates the most recent challenge completed by the user within a specified date range.
- **`marianatek-pp-cli me metrics-top-instructors-retrieve`** - Calculates the top instructors based on the user's class attendance within a specified date range.
- **`marianatek-pp-cli me metrics-top-time-of-day-retrieve`** - Calculates the time of day when the user most frequently attends classes within a specified date range. Morning threshold: class start date (inclusively) in range `03:00:00 - 11:59:59`. Afternoon threshold: class start date (inclusively) in range `12:00:00 - 16:59:59`. Night Owl threshold: class start date (inclusively) in range `17:00:00-02:59:59`.
- **`marianatek-pp-cli me metrics-total-minutes-in-class-retrieve`** - Calculates the total number of minutes the user has spent in classes within a specified date range.
- **`marianatek-pp-cli me orders-cancel-create`** - Cancel a specific user order. This action is irreversible.
- **`marianatek-pp-cli me orders-list`** - View the order history for the current user. The following query parameters can be used to assist in filtering orders, `status`, `exclude_statuses`, and `reservation`.
- **`marianatek-pp-cli me orders-retrieve`** - View details about a single order.
- **`marianatek-pp-cli me reservations-assign-to-spot-create`** - Move waitlist or standby reservation to class
This endpoint should be used to move customers from the waitlist or on standby into class after Autofill
cutoff window and before Reservation cutoff. Optionally specify the specific spot to move.
otherwise a random spot will be chosen.

Example:

    {
        "data": {
            "spot": 123
        }
    }
- **`marianatek-pp-cli me reservations-cancel-create`** - Cancel a reservation previously booked by the current user.
- **`marianatek-pp-cli me reservations-cancel-penalty-retrieve`** - Check whether the user will incur a penalty if they choose to cancel at the current time. Users should always be notified if there is a cancellation fee.
- **`marianatek-pp-cli me reservations-cart-add-product-listing-create`** - Add a product listing to the current user's add-ons cart.
- **`marianatek-pp-cli me reservations-cart-adjust-quantity-create`** - Increase or decrease the quantity of a product within an open add-ons cart.
- **`marianatek-pp-cli me reservations-cart-apply-discount-code-create`** - Apply a discount code to an add-ons cart.
- **`marianatek-pp-cli me reservations-cart-checkout-create`** - Submit payment for an add-ons cart, resulting in the creation of an add-ons order.
- **`marianatek-pp-cli me reservations-cart-clear-create`** - Remove all items from an add-ons cart.
- **`marianatek-pp-cli me reservations-cart-clear-discount-codes-create`** - Remove a discount code that has been applied to an add-ons cart.
- **`marianatek-pp-cli me reservations-cart-list`** - Retrieve an open cart for an add-ons order for the given reservation.
- **`marianatek-pp-cli me reservations-cart-retrieve`** - Retrieve the Cart associated with a specific reservation. This cart can be used to add products and services related to a class reservation. If no cart exists for the given reservation, a new one will be created.
- **`marianatek-pp-cli me reservations-check-in-create`** - Check-in the user for a previously booked reservation. If geo check-in is enabled for a tenant, check-in actions are validated against the associated class session's geo check-in window.
- **`marianatek-pp-cli me reservations-create`** - Create a new reservation for the authenticated user. reservation_type accepts one of three defined values: `standard`, `standby`, and `waitlist`. `guest_email` is optional and should only be sent when `is_booked_for_me` is set to `true`. A spot object is only needed when creating a reservation for a pick-a-spot class.
- **`marianatek-pp-cli me reservations-list`** - View details about this user's reservation history.
- **`marianatek-pp-cli me reservations-retrieve`** - View details about one of the current user's reservations.
- **`marianatek-pp-cli me reservations-swap-spots-create`** - For a given reservation, swap its current spot with a different spot.

### regions

Manage regions

- **`marianatek-pp-cli regions list`** - View the class types, classrooms, and instructors for all classes in this region.
- **`marianatek-pp-cli regions retrieve`** - View the class types, classrooms, and instructors for all classes in this region.

### schedule-locations

Manage schedule locations

- **`marianatek-pp-cli schedule-locations list`** - View a list of active locations along with information relevant to their class schedules.
- **`marianatek-pp-cli schedule-locations retrieve`** - Get details about a single location along with information needed to display its class schedule, including class types, classrooms, and instructors.

### schedule-regions

Manage schedule regions

- **`marianatek-pp-cli schedule-regions list`** - View a list of active regions along with information relevant to their class schedules.
- **`marianatek-pp-cli schedule-regions retrieve`** - Get details about a single region along with information needed to display its class schedule, including class types, classrooms, and instructors.

### theme

Manage theme

- **`marianatek-pp-cli theme retrieve`** - View branding information that can be used for styling applications.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
marianatek-pp-cli app-version-metadatas list

# JSON for scripting and agents
marianatek-pp-cli app-version-metadatas list --json

# Filter to specific fields
marianatek-pp-cli app-version-metadatas list --json --select id,name,status

# Dry run — show the request without sending
marianatek-pp-cli app-version-metadatas list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
marianatek-pp-cli app-version-metadatas list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-marianatek -g
```

Then invoke `/pp-marianatek <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add marianatek marianatek-pp-mcp -e CUSTOMER_OAUTH_AUTHORIZATION=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/marianatek-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CUSTOMER_OAUTH_AUTHORIZATION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "marianatek": {
      "command": "marianatek-pp-mcp",
      "env": {
        "CUSTOMER_OAUTH_AUTHORIZATION": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
marianatek-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/customer-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CUSTOMER_OAUTH_AUTHORIZATION` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `marianatek-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CUSTOMER_OAUTH_AUTHORIZATION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized from any /me endpoint** — Refresh tokens expired. Run `marianatek login --tenant <slug>` again, or check `marianatek doctor` to see which tenant is expired.
- **watch never fires even though a spot opened in the browser** — Increase polling frequency with `--interval 30s` and confirm the class id is in the local cache: `marianatek classes get <id>`.
- **schedule returns empty under --any-tenant** — Sync hasn't run for one or more tenants. Run `marianatek sync` per tenant, or `marianatek sync --all-tenants`.
- **book / book-regular returns 402 payment required** — No applicable credit, membership, or default credit card. Run `marianatek classes payment-options <id>` to see what payment paths exist, then `marianatek cards list` to confirm a default card is on file.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bigkraig/marianatek**](https://github.com/bigkraig/marianatek) — Go
- [**Bitlancer/mariana_api-gem**](https://github.com/Bitlancer/mariana_api-gem) — Ruby
- [**bfitzsimmons/marianatek_movies**](https://github.com/bfitzsimmons/marianatek_movies) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
