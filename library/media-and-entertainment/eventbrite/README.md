# Eventbrite CLI

**Every Eventbrite organizer endpoint, plus a local SQLite mirror of your events, orders, and attendees you can search offline — the cross-event search Eventbrite removed, restored over your own data.**

Existing Eventbrite tools wrap a handful of event CRUD calls one event at a time. This CLI mirrors the full v3 organizer API and syncs your events, orders, attendees, ticket classes, and discounts into a local SQLite store, then layers cross-event analytics on top: sales-velocity ranks your live events by sell rate, repeat-attendees surfaces returning fans across your whole history, and org-rollup gives agencies a single pane across every client organization.

Created by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `eventbrite-pp-cli` binary and the `pp-eventbrite` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install eventbrite
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install eventbrite --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install eventbrite --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install eventbrite --agent claude-code
npx -y @mvanhorn/printing-press-library install eventbrite --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/cmd/eventbrite-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eventbrite-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-eventbrite --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-eventbrite --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install eventbrite --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eventbrite-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EVENTBRITE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/cmd/eventbrite-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "eventbrite": {
      "command": "eventbrite-pp-mcp",
      "env": {
        "EVENTBRITE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Eventbrite uses an OAuth2 private token sent as a Bearer header. For your own account you do not need the full OAuth dance — copy a private token from eventbrite.com/platform/api-keys and set EVENTBRITE_API_KEY. Run `eventbrite-pp-cli doctor` to confirm it is picked up.

## Quick Start

```bash
# Confirm EVENTBRITE_API_KEY is set and the API is reachable before anything else.
eventbrite-pp-cli doctor

# Pull everything your token can access into local SQLite; Eventbrite paginates with continuation tokens, which sync follows automatically.
eventbrite-pp-cli sync

# Rank your live events by tickets sold per day so you can see laggards at a glance.
eventbrite-pp-cli sales-velocity --json

# Find fans who attended multiple events — the cross-event view Eventbrite's public search no longer provides.
eventbrite-pp-cli repeat-attendees --min 2 --json

# Full-text search across everything you have synced offline.
eventbrite-pp-cli search "VIP"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-event sales analytics
- **`sales-velocity`** — Ranks all your live events by tickets sold per day since on-sale and projects a sell-out date.

  _Reach for this when an agent needs to know which of an organizer's events are underperforming right now, not just current totals._

  ```bash
  eventbrite-pp-cli sales-velocity --json
  ```
- **`discount-performance`** — Per discount code: redemptions, type, and the redemption rate (share of the code's allotment used).

  _Pick this to measure whether a promo code actually drove sales before renewing or killing it._

  ```bash
  eventbrite-pp-cli discount-performance --json
  ```
- **`capacity`** — Sold vs total capacity and percent remaining across all your live events at once.

  _Use this to spot which events are near sell-out or badly under-sold in a single call._

  ```bash
  eventbrite-pp-cli capacity --json
  ```
- **`refund-rate`** — Refunded and cancelled order counts, refunded revenue, and the rate per event or across the org.

  _Pick this to flag events with abnormal refund rates that signal a pricing, date, or fulfillment problem._

  ```bash
  eventbrite-pp-cli refund-rate --json
  ```
- **`top-buyers`** — Ranks ticket buyers by total spend across all your events, not just by how many events they attended.

  _Reach for this to find an organizer's highest-value customers for VIP or pre-sale outreach._

  ```bash
  eventbrite-pp-cli top-buyers --limit 25 --json
  ```

### Search your own data offline
- **`repeat-attendees`** — Finds fans who bought into two or more of your events, across your whole synced history.

  _Use this for loyalty and re-marketing questions an agent cannot answer from any one event's attendee list._

  ```bash
  eventbrite-pp-cli repeat-attendees --min 2 --json
  ```
- **`roster`** — Offline attendee roster for one event with checked-in vs not-yet, VIP and comp flags, door-sorted.

  _Reach for this at the door when an agent needs a fast who-has-not-checked-in list without a live API round-trip._

  ```bash
  eventbrite-pp-cli roster 1234567890 --csv
  ```
- **`fan-export`** — Exports unique attendee contacts (email, name, events attended, check-in status), deduped across all your events and filterable by event.

  _Use this to build a clean, deduped contact list for an email campaign across an organizer's whole audience._

  ```bash
  eventbrite-pp-cli fan-export --csv
  ```

### Agency multi-org view
- **`org-rollup`** — Single pane across every organization your token can see: events count, tickets sold, gross, and top event per org.

  _Use this for an agency or multi-brand operator who needs one client roll-up instead of logging into each org._

  ```bash
  eventbrite-pp-cli org-rollup --json
  ```

## Recipes

### Find your slowest-selling live events

```bash
eventbrite-pp-cli sales-velocity --json
```

Ranks every live event by tickets sold per day since on-sale with a projected sell-out date, computed from the local order store.

### Surface returning fans for re-marketing

```bash
eventbrite-pp-cli repeat-attendees --min 3 --agent --select email,name,events_count
```

Cross-event attendee join; --agent emits structured rows and --select narrows to just the contact fields an email tool needs.

### Door check-in roster, offline

```bash
eventbrite-pp-cli roster 1234567890 --csv
```

Prints the synced attendee roster for one event with checked-in status as CSV for a will-call sheet — no live API call required.

### Agency Friday client roll-up

```bash
eventbrite-pp-cli org-rollup --json
```

One pane across every organization the token can see: events, tickets sold, gross, and top event per client org.

### Measure a promo code's ROI

```bash
eventbrite-pp-cli discount-performance --json
```

Ranks discount codes by redemptions and redemption rate (share of each code's allotment used) from the synced discount objects.

### Combine Eventbrite + DICE for full event performance

```bash
eventbrite-pp-cli fan-export --json
```

Export your Eventbrite buyer/attendee list as JSON; a promoter selling the same show on DICE exports the DICE side too, then an agent joins events by name and date and buyers by normalized email for a cross-platform performance and loyalty view no single platform shows.

## Usage

Run `eventbrite-pp-cli --help` for the full command reference and flag list.

## Commands

### balance

Manage balance

### categories

<a name="categories_object"></a>

## Category Object

An overarching category that an event falls into (vertical). Examples are “Music”, and “Endurance”.

- **`eventbrite-pp-cli categories category-by-id`** - Gets a `category` by ID as ``category``.
- **`eventbrite-pp-cli categories list-of`** - Returns a list of Category as categories, including subcategories nested. Returns a paginated response.

### discounts

<a name="discount_object"></a>

## Discount Object

The Discount object represents a discount that an Order owner can use when purchasing tickets to an [Event](#event_object).

A Discount can be used to a single [Ticket Class](#ticket_class_object) or across multiple Ticket Classes for multiple Events simultaneously (known as a cross event Discount).

There are four types of Discounts:

+ **Public Discount.** Publically displays Discount to Order owner on the Event Listing and Checkout pages. Only used with a single Event.

+ **Coded Discount.** Requires Order owner to use a secret code to access the Discount.

+ **Access Code.** Requires Order owner to use a secret code to access hidden tickets. Access codes can also optionally contain a discount amount.

+ **Hold Discount.** Allows Order owner to apply or unlock Discount for seats on hold.

The display price of a ticket is calculated as: \
`price_before_discount` - `discount_amount` = `display_price`

Notes:

* Public and Coded Discounts can specify either an amount off or a percentage off, but not both types discounts.

* Public Discounts should not contain apostrophes or non-alphanumeric characters (except “-���, “_”, ” ”, “(”, ”)”, “/”, and “”).

* Coded Discounts and Access Codes should not contain spaces, apostrophes or non-alphanumeric characters (except “-”, “_”, “(”, ”)”, “/”, and “”).

#### Fields

Use these fields to specify information about a Discount.

|         Field         |       Type       |                                                                                              Description                                                                                              |
| :-------------------- | :--------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `code`                | `string`         | Discount name for a Public Discount, or the code for a Coded Discount and Access Code.                                                                                                                |
| `type`                | `string`         | Discount type. Can be `access`, `coded`, `public` or `hold`.                                                                                                                                          |
| `end_date`            | `datetime`       | Date until which the Discount code is usable. Date is naive and assumed relative to the timezone of an Event. If null or empty, the discount is usable until the Event end_date. ISO 8601 notation: `YYYY-MM-DDThh:mm:ss`.|
| `end_date_relative`   | `integer`        | End time in seconds before the start of the Event until which the Discount code is usable. If null or empty, the discount is usable until the Event end_date.                                         |
| `amount_off`          | `decimal`        | Fixed amount applied as a Discount. This amount is not expressed with a currency; instead uses the Event currency from 0.01 to 99999.99. Only two decimals are allowed. The default is `null` for an Access Code. |
| `percent_off`         | `decimal`        | Percentage amount applied as a Discount. Displayed in the ticket price during checkout, from 1.00 to 100.00. Only two decimals are allowed. The default is `null` for an Access Code.                             |
| `quantity_available`  | `integer`        | Number of times this Discount can be used; `0` indicates unlimited use.                                                                                                                               |
| `quantity_sold`       | `integer`        | Number of times this Discount has been used. This is a read only field.                                                                                                                               |
| `start_date`          | `local datetime` | Date from which the Discount code is usable. If null or empty, the Discount is usable effective immediately.                                                                                          |
| `start_date_relative` | `integer`        | Start time in seconds before the start of the Event from which the Discount code is usable. If null or empty, the Discount is usable effective immediately.                                           |
| `ticket_class_ids`    | `list`           | List of discounted Ticket Class IDs for a single Event. Leave empty if you want to see all the tickets for the Event.                                                                                 |
| `event_id`            | `string`         | Single Event ID to which the Discount can be used. Leave empty for Discounts.                                                                                                                         |
| `ticket_group_id`     | `string`         | [Ticket Group](#ticket_group_object) ID to which the Discount can be used.                                                                                                                            |
| `hold_ids`            | `list`           | List of hold IDs this discount can unlock. Null if this discount does not unlock a hold.                                                                                                              |

The following conditions define the extend of the Discount:

* If `event_id` is provided and `ticket_class_ids` are not provided, a single Event Discount is created for all Event tickets.

* If both `event_id` and `ticket_class_ids` are provided, a single Event Discount is created for the specific Event  tickets.

* If `ticket_group_id` is provided, a Discount is created for the Ticket Group.

* If neither `event_id` nor `ticket_group_id` are provided, a Discount is created that applies to all tickets for an [Organization's](#organization_object) Events, including future Events.

#### Expansions

Information from expansions fields are not normally returned when requesting information. To receive this information in a request, expand the request.

|     Expansion      |               Source               |                                                                           Description                                                                            |
| :----------------- | :--------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `event`            | `event_id`                         | Single Event to which the Discount can be used.                                                                                                                  |
| `ticket_group`     | `ticket_group_id`                  | Ticket Group to which the Discount can be used.                                                                                                                  |
| `reserved_seating` | `ticket-reserved-seating-settings` | Reserved seating settings for the [Ticket Class](#ticket_class_object). This expansion is not returned for Ticket Classes that do not support reserved seasting. |

- **`eventbrite-pp-cli discounts delete-a`** - Delete a Discount. Only unused Discounts can be deleted.

<b>Warning:</b> A Discount cannot be restored after being deleted.
- **`eventbrite-pp-cli discounts retrieve-a`** - Retrieve a Discount by Discount ID.
- **`eventbrite-pp-cli discounts update-a`** - Update a Discount by Discount ID.

### event

<a name="event_object"></a>

## Event Object

The Event object represents an Eventbrite Event. An Event is owned by one [Organization](#organization_object).

#### Public Fields

Use these fields to specify information about an Event. For publicly listed Events, this information can be retrieved by all Eventbrite [Users](#user_object) and Eventbrite applications.

|       Field       |       Type       |                                               Description                                               |
| :---------------- | :--------------- | ------------------------------------------------------------------------------------------------------- |
| `name`            | `multipart-text` | Event name.                                                                                             |
| `summary`         | `string`         | (Optional) Event summary. Short summary describing the event and its purpose.                           |
| `description`     | `multipart-text` | (*DEPRECATED*) (Optional) Event description. Description can be lengthy and have significant formatting.               |
| `url`             | `string`         | URL of the Event's Listing page on eventbrite.com.                                                      |
| `start`           | `datetime-tz`    | Event start date and time.                                                                              |
| `end`             | `datetime-tz`    | Event end date and time.                                                                                |
| `created`         | `datetime`       | Event creation date and time.                                                                           |
| `changed`         | `datetime`       | Date and time of most recent changes to the Event.                                                      |
| `published`       | `datetime`       | Event publication date and time.                                                                        |
| `status`          | `string`         | Event status. Can be `draft`, `live`, `started`, `ended`, `completed` and `canceled`.                   |
| `currency`        | `string`         | Event [ISO 4217](https://en.wikipedia.org/wiki/ISO_4217) currency code.                                 |
| `online_event`    | `boolean`        | true = Specifies that the Event is online only (i.e. the Event does not have a [Venue](#venue_object)). |
| `hide_start_date` | `boolean`        | If true, the event's start date should never be displayed to attendees.                                     |
| `hide_end_date`   | `boolean`        | If true, the event's end date should never be displayed to attendees.                                       |

#### Private Fields

Use these fields to specify properties of an Event that are only available to the [User](#user_object).

|        Field         |   Type    |                                                                                                Description                                                                                                |
| :------------------- | :-------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `listed`             | `boolean` | true = Allows the Event to be publicly searchable on the Eventbrite website.                                                                                                                              |
| `shareable`          | `boolean` | true = Event is shareable, by including social sharing buttons for the Event to Eventbrite applications.                                                                                                  |
| `invite_only`        | `boolean` | true = Only invitees who have received an email inviting them to the Event are able to see Eventbrite applications.                                                                                       |
| `show_remaining`     | `boolean` | true = Provides, to Eventbrite applications, the total number of remaining tickets for the Event.                                                                                                         |
| `password`           | `string`  | Event password used by visitors to access the details of the Event.                                                                                                                                       |
| `capacity`           | `integer` | Maximum number of tickets for the Event that can be sold to [Attendees](#attendee_object). The total capacity is calculated by the sum of the quantity_total of the [Ticket Class](#ticket_class_object). |
| `capacity_is_custom` | `boolean` | true = Use custom capacity value to specify the maximum number of Attendees for the Event. False = Calculate the maximum number of Attendees for the Event from the total of all Ticket Class capacities. |

<a name="music_properties_object"></a>

#### Music Properties

The Music Properties object includes a few attributes of an event for Music clients. To retrieve Music Properties by Event ID, use the `music_properties` expansion.

|       Field       |   Type   |                                                                                Description                                                                                 |
| :---------------- | :------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `age_restriction` | `enum`   | Minimum age requirement of event attendees.                                                                                                                                |
| `presented_by`    | `string` | Main music event sponsor.                                                                                                                                                  |
| `door_time`       | `string` | Time relative to UTC that the doors are opened to allow people in the the day of the event. When not set, the event will not have any door time set. 2019-05-12T-19:00:00Z |

<a name="event_expansions" />

#### Expansions

Information from expansions fields are not normally returned when requesting information. To receive this information in a request, expand the request.

|       Expansion           |        Source         |                                                                                                                       Description                                                                                                                        |
| :------------------------ | :-------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `logo`                    | `logo_id`             | Event image logo.                                                                                                                                                                                                                                        |
| `venue`                   | `venue_id`            | Event [Venue](#venue_object).                                                                                                                                                                                                                            |
| `organizer`               | `organizer_id`        | Event [Organizer](#organizer_object).                                                                                                                                                                                                                    |
| `format`                  | `format_id`           | Event [Format](#formats_object).                                                                                                                                                                                                                         |
| `category`                | `category_id`         | Event [Category](#categories_object).                                                                                                                                                                                                                    |
| `subcategory`             | `subcategory_id`      | Event [Subcategory](#subcategories_object).                                                                                                                                                                                                              |
| `bookmark_info`           | `bookmark_info`       | Indicates whether a user has saved the Event as a bookmark. Returns false if there are no bookmarks. If there are bookmarks, returns a a dictionary specifying the number of end-users who have bookmarked the Event as a count object like `{count:3}`. |
| `refund_policy`           | `refund_policy`       | Event [Refund Policy](#refund_policy_object).                                                                                                                                                                                                           |
| `ticket_availability`     | `ticket_availability` | Overview of availability of all Ticket Classes                                                                                                                                                                           |
| `external_ticketing`      | `external_ticketing`  | External ticketing data for the Event.                                                                                                                                                                                                                   |
| `music_properties`        | `music_properties`    | Event [Music Properties](#music_properties_object)                                                                                                                                                                                                       |
| `publish_settings`        | `publish_settings`    | Event publish settings.                                                                                                                                                                                                                                  |
| `basic_inventory_info`    | `basic_inventory_info`| Indicates whether the event has Ticket Classes, Inventory Tiers, Donation Ticket Classes, Ticket Rules, Inventory Add-Ons, and/or Admission Inventory Tiers.                                                                                             |
| `event_sales_status`      | `event_sales_status`  | Event’s sales status details                                                                                                                                                                                                                             |
| `checkout_settings   `    | `checkout_settings`   | Event checkout and payment settings.                                                                                                                                                                                                                     |
| `listing_properties`      | `listing_properties`  | Display/listing details about the event                                                                                                                                                                                                                  |
| `has_digital_content`     | `has_digital_content` | Whether or not an event [Has Digital Content](#has_digital_content_object)                                                                                                                                                                                                   |

### events

<a name="event_object"></a>

## Event Object

The Event object represents an Eventbrite Event. An Event is owned by one [Organization](#organization_object).

#### Public Fields

Use these fields to specify information about an Event. For publicly listed Events, this information can be retrieved by all Eventbrite [Users](#user_object) and Eventbrite applications.

|       Field       |       Type       |                                               Description                                               |
| :---------------- | :--------------- | ------------------------------------------------------------------------------------------------------- |
| `name`            | `multipart-text` | Event name.                                                                                             |
| `summary`         | `string`         | (Optional) Event summary. Short summary describing the event and its purpose.                           |
| `description`     | `multipart-text` | (*DEPRECATED*) (Optional) Event description. Description can be lengthy and have significant formatting.               |
| `url`             | `string`         | URL of the Event's Listing page on eventbrite.com.                                                      |
| `start`           | `datetime-tz`    | Event start date and time.                                                                              |
| `end`             | `datetime-tz`    | Event end date and time.                                                                                |
| `created`         | `datetime`       | Event creation date and time.                                                                           |
| `changed`         | `datetime`       | Date and time of most recent changes to the Event.                                                      |
| `published`       | `datetime`       | Event publication date and time.                                                                        |
| `status`          | `string`         | Event status. Can be `draft`, `live`, `started`, `ended`, `completed` and `canceled`.                   |
| `currency`        | `string`         | Event [ISO 4217](https://en.wikipedia.org/wiki/ISO_4217) currency code.                                 |
| `online_event`    | `boolean`        | true = Specifies that the Event is online only (i.e. the Event does not have a [Venue](#venue_object)). |
| `hide_start_date` | `boolean`        | If true, the event's start date should never be displayed to attendees.                                     |
| `hide_end_date`   | `boolean`        | If true, the event's end date should never be displayed to attendees.                                       |

#### Private Fields

Use these fields to specify properties of an Event that are only available to the [User](#user_object).

|        Field         |   Type    |                                                                                                Description                                                                                                |
| :------------------- | :-------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `listed`             | `boolean` | true = Allows the Event to be publicly searchable on the Eventbrite website.                                                                                                                              |
| `shareable`          | `boolean` | true = Event is shareable, by including social sharing buttons for the Event to Eventbrite applications.                                                                                                  |
| `invite_only`        | `boolean` | true = Only invitees who have received an email inviting them to the Event are able to see Eventbrite applications.                                                                                       |
| `show_remaining`     | `boolean` | true = Provides, to Eventbrite applications, the total number of remaining tickets for the Event.                                                                                                         |
| `password`           | `string`  | Event password used by visitors to access the details of the Event.                                                                                                                                       |
| `capacity`           | `integer` | Maximum number of tickets for the Event that can be sold to [Attendees](#attendee_object). The total capacity is calculated by the sum of the quantity_total of the [Ticket Class](#ticket_class_object). |
| `capacity_is_custom` | `boolean` | true = Use custom capacity value to specify the maximum number of Attendees for the Event. False = Calculate the maximum number of Attendees for the Event from the total of all Ticket Class capacities. |

<a name="music_properties_object"></a>

#### Music Properties

The Music Properties object includes a few attributes of an event for Music clients. To retrieve Music Properties by Event ID, use the `music_properties` expansion.

|       Field       |   Type   |                                                                                Description                                                                                 |
| :---------------- | :------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `age_restriction` | `enum`   | Minimum age requirement of event attendees.                                                                                                                                |
| `presented_by`    | `string` | Main music event sponsor.                                                                                                                                                  |
| `door_time`       | `string` | Time relative to UTC that the doors are opened to allow people in the the day of the event. When not set, the event will not have any door time set. 2019-05-12T-19:00:00Z |

<a name="event_expansions" />

#### Expansions

Information from expansions fields are not normally returned when requesting information. To receive this information in a request, expand the request.

|       Expansion           |        Source         |                                                                                                                       Description                                                                                                                        |
| :------------------------ | :-------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `logo`                    | `logo_id`             | Event image logo.                                                                                                                                                                                                                                        |
| `venue`                   | `venue_id`            | Event [Venue](#venue_object).                                                                                                                                                                                                                            |
| `organizer`               | `organizer_id`        | Event [Organizer](#organizer_object).                                                                                                                                                                                                                    |
| `format`                  | `format_id`           | Event [Format](#formats_object).                                                                                                                                                                                                                         |
| `category`                | `category_id`         | Event [Category](#categories_object).                                                                                                                                                                                                                    |
| `subcategory`             | `subcategory_id`      | Event [Subcategory](#subcategories_object).                                                                                                                                                                                                              |
| `bookmark_info`           | `bookmark_info`       | Indicates whether a user has saved the Event as a bookmark. Returns false if there are no bookmarks. If there are bookmarks, returns a a dictionary specifying the number of end-users who have bookmarked the Event as a count object like `{count:3}`. |
| `refund_policy`           | `refund_policy`       | Event [Refund Policy](#refund_policy_object).                                                                                                                                                                                                           |
| `ticket_availability`     | `ticket_availability` | Overview of availability of all Ticket Classes                                                                                                                                                                           |
| `external_ticketing`      | `external_ticketing`  | External ticketing data for the Event.                                                                                                                                                                                                                   |
| `music_properties`        | `music_properties`    | Event [Music Properties](#music_properties_object)                                                                                                                                                                                                       |
| `publish_settings`        | `publish_settings`    | Event publish settings.                                                                                                                                                                                                                                  |
| `basic_inventory_info`    | `basic_inventory_info`| Indicates whether the event has Ticket Classes, Inventory Tiers, Donation Ticket Classes, Ticket Rules, Inventory Add-Ons, and/or Admission Inventory Tiers.                                                                                             |
| `event_sales_status`      | `event_sales_status`  | Event’s sales status details                                                                                                                                                                                                                             |
| `checkout_settings   `    | `checkout_settings`   | Event checkout and payment settings.                                                                                                                                                                                                                     |
| `listing_properties`      | `listing_properties`  | Display/listing details about the event                                                                                                                                                                                                                  |
| `has_digital_content`     | `has_digital_content` | Whether or not an event [Has Digital Content](#has_digital_content_object)                                                                                                                                                                                                   |

- **`eventbrite-pp-cli events delete-an`** - Delete an Event if the delete is permitted. Returns a boolean indicating the success or failure of the delete action.
To delete an Event, the Event must not have any pending or completed orders.

If the event is a series parent, all series occurrences must be in a valid state to be deleted. Deleting the series parent will delete all series occurrences.
- **`eventbrite-pp-cli events retrieve-an`** - Retrieve an Event by Event ID.

> **Note**: If the Event being retrieved was created using the new version of Create, then you may notice that the Event’s description field is now being used to hold the event summary. To retrieve your event’s fully-rendered HTML description, you will need to make an additional API call to retrieve the Event's full HTML description.
- **`eventbrite-pp-cli events update-an`** - Update Event by Event ID.

Note that if the event is a series parent, updating `name`, `description`, `hide_start_date`, `hide_end_date`, `currency`, `show_remaining`, `password`, `capacity`, or `source` on the series parent will update these fields on all occurrences in the series.

### formats

<a name="formats_object"></a>

## Format Object

The Format object represents an [Event](#event_object) type, for example seminar, workshop or concert. Specifying a Format helps website visitors discover a certain type of Event.

- **`eventbrite-pp-cli formats list`** - List all available Formats. Returns a paginated response.
- **`eventbrite-pp-cli formats retrieve-a`** - Retrieve a Format by Format ID.

### media

<!-- Here is a tutorial on [Media Upload in EventBrite](https://www.eventbrite.com/developer/v3/resources/uploads/). -->

<a name="media_object"></a>

## Media Object

The Media object represents an image that can be included with an [Event](#event_object) listing, for example to provide branding or further information on the Event.

- **`eventbrite-pp-cli media retrieve`** - Retrieve Media by Media ID.
- **`eventbrite-pp-cli media retrieve-a-upload`** - Retrieve information on a Media image upload.
- **`eventbrite-pp-cli media upload-a-file`** - Upload a Media image file.

### orders

<a name="order_object"></a>

## Order object

The Order object represents an order made against Eventbrite for one or more [Ticket Classes](#ticket_class_object). In other words, a single Order can be made up of multiple tickets. The object contains an Order's financial and transactional information; use the [Attendee](#attendee_object) object to return information on Attendees.

Order objects are considered private; meaning that all Order information is only available to the Eventbrite [User](#user_object) and Order owner.

#### Order Fields

|      Field       |                Type                 |                                                          Description                                                          |
| :--------------- | :---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `created`        | `datetime`                          | Date and time the Order was placed and the Attendee created.                                                                  |
| `changed`        | `datetime`                          | Date and time of the last change to Attendee.                                                                                 |
| `name`           | `string`                            | Order owner name. To ensure forward compatibility with non-Western names, use this field instead of `first_name`/`last_name`. |
| `first_name`     | `string`                            | Order owner first name. Use `name` field instead.                                                                             |
| `last_name`      | `string`                            | Order owner last name. Use `name` field instead.                                                                              |
| `email`          | `string`                            | Order owner email address.                                                                                                    |
| `costs`          | [order-costs](#order_cost)          | Cost breakdown of the Order.                                                                                                  |
| `event_id`       | `string`                            | Order's [Event](#event_object) ID.                                                                                            |
| `time_remaining` | `number`                            | Time remaining to complete Order (in seconds).                                                                                |
| `questions`      | [order-questions](#order_questions) | (Optional) Custom questions shown to Order's owner.                                                                           |
| `answers`        | [order-answers](#order_answers)     | (Optional) Answers to custom questions shown to Order's owner.                                                                |
| `promo_code`     | `string`                            | (Optional) [Discount](#discount_object) code applied to Order.                                                                |
| `status`         | `string`                            | Order status.                                                                                                                 |

<a name="order_cost"></a>

#### Order Costs Fields

Contains a breakdown of Order costs.

|          Field          |                  Type                   |                                                                                                 Description                                                                                                 |
| :---------------------- | :-------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `base_price`            | `currency`                              | Order amount without fees and tax. Use instead the `display_price` field if the [Ticket Class](#ticket_class_object) `include_fee` field is used; otherwise an incorrect value is shown to the Order owner. |
| `display_price`         | `currency`                              | Order amount without fees and tax. This field shows the correct value to the Order owner when the Ticket Class `include_fee` field is used.                                                                 |
| `display_fee`           | `currency`                              | Order amount with fees and tax included (absorbed) in the price as displayed.                                                                                                                               |
| `gross`                 | `currency`                              | Total amount of Order.                                                                                                                                                                                      |
| `eventbrite_fee`        | `currency`                              | Eventbrite fee as portion of Order `gross` amount. Do not expose this field to Order owner.                                                                                                                 |
| `payment_fee`           | `currency`                              | Payment processor fee as portion of Order `gross` amount.                                                                                                                                                   |
| `tax`                   | `currency`                              | Tax as portion of Order `gross` amount passed to Event [Organization](#organization_object).                                                                                                                |
| `display_tax`           | [order-display-tax](#order_display_tax) | Order tax. Same value as `tax` field, but also includes the tax name.                                                                                                                                       |
| `price_before_discount` | `currency`                              | Order price before a [Discount](#discount_object) code is applied. If no discount code is applied, value should be equal to `display_price`.                                                                |
| `discount_amount`       | `currency`                              | Order total Discount. If no discount code is applied, discount_amount will not be returned.                                                                                                                 |
| `discount_type`         | `string`                                | Type of Discount applied to Order. Can be `null` or `coded`, `access`, `public` or `hold`. If no discount code is applied, discount_type will not be returned.                                              |
| `fee_components`        | `Cost Component` (list)                 | List of price costs components that belong to the fee display group.                                                                                                                                        |
| `tax_components`        | `Cost Component` (list)                 | List of price costs components that belong to the tax display group.                                                                                                                                        |
| `shipping_components`   | `Cost Component` (list)                 | List of price costs components that belong to the shippig display group.                                                                                                                                    |
| `has_gts_tax`           | `boolean`                               | Indicates if any of the tax_components is a gts tax.                                                                                                                                                        |
| `tax_name`              | `string`                                | The name of the tax that applies, if any.                                                                                                                                                                   |

<a name="order_display_tax"></a>

#### Display Tax Fields

| Field  |    Type    | Description |
| :----- | :--------- | ----------- |
| `name` | `string`   | Tax name.   |
| `tax`  | `currency` | Tax amount. |

#### Refund Request Fields

The Order includes a refund request.

|     Field      |          Type          |                             Description                             |
| :------------- | :--------------------- | ------------------------------------------------------------------- |
| `from_email`   | `string`               | Email used to create the refund request.                            |
| `from_name`    | `string`               | Refund request name.                                                |
| `status`       | `string`               | Refund request status.                                              |
| `message`      | `string`               | Message associated with the refund request.                         |
| `reason`       | `string`               | Refund request reason code.                                         |
| `last_message` | `string`               | Last message associated with the last status of the refund request. |
| `last_reason`  | `string`               | Last reason code of the refund request.                             |
| `items`        | list of [refund_item](#refund_item) | Requested refunded items of the refund request.        |

<a name="refund_item"></a>

#### Refund Item Fields

A Refund Request contains a refund item.

|        Field             |    Type    |                                                                                                                        Description                                                                                                                                                                              |
| :----------------------- | :--------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `event_id`               | `string`   | Refund item Event.                                                                                                                                                                                                                                                                                              |
| `order_id`               | `string`   | Refund item Order. Field can be `null`.                                                                                                                                                                                                                                                                         |
| `processed_date`         | `datetime` | (Optional) The date and time this refund item was processed, if it has been processed.                                                                                                                                                                                                                          |
| `item_type`              | `string`   | Refund item Order type. Use `order` for full refund, `attendee` for partial refund for the Attendee, or `merchandise` for partial refund as merchandise.                                                                                                                                                        |
| `amount_processed`       | `currency` | (Optional) The amount of money refunded. This will be absent if the refund has not been processed.                                                                                                                                                                                                              |
| `amount_requested`       | `currency` | (Optional) The amount of money requested for refund. Only appears for attendee-initiated refunds.                                                                                                                                                                                                               |
| `quantity_processed`     | `number`   | (Optional) Quantity refunded. If the `item_type` field value is `order`, `quantity_processed` is always 1. If the `item_type` field value is `attendee` or `merchandise`, then the `quantity_processed` value displays the number of items processed. This will be absent if the refund has not been processed. |
| `quantity_requested`     | `number`   | (Optional) Quantity requested to be refunded. If the `item_type` is `order`, `quantity_requested` is always 1. If the `item_type` is `attendee` or `merchandise`, then the `quantity_requested` value displays the number of items requested for a refund. Only appears for attendee-initiated refund items.    |
| `refund_reason_code`     | `string`   | A descriptive code for the refund reason                                                                                                                                                                                                                                                                        |
| `status`                 | `string`   | Refund item status, one of `pending`, `processed`, or `error`                                                                                                                                                                                                                                                   |

<a name="order_questions"></a>

#### Order Questions Fields

Use to present Custom Questions to an Attendee.

|   Field    |   Type    |                                Description                                |
| :--------- | :-------- | ------------------------------------------------------------------------- |
| `id`       | `string`  | Custom Question ID.                                                       |
| `label`    | `string`  | Custom Question Label.                                                    |
| `type`     | `string`  | Can be `text`, `url`, `email`, `date`, `number`, `address`, or `dropdown` |
| `required` | `boolean` | true = Answer to custom question is required.                             |

<a name="order_answers"></a>

#### Order Answers Fields

Contains information on an Attendee's answers to custom questions.

|     Field     |   Type   |                                                   Description                                                    |
| :------------ | :------- | ---------------------------------------------------------------------------------------------------------------- |
| `question_id` | `string` | Custom Question ID.                                                                                              |
| `attendee_id` | `string` | Attendee ID.                                                                                                     |
| `question`    | `string` | Custom Question text.                                                                                            |
| `type`        | `string` | Can be `text`, `url`, `email`, `date`, `number`, `address`, or `dropdown`.                                       |
| `answer`      | varies   | Answer type. Generally use the `string` value; except when an answer of `address` or `date` is more appropriate. |

#### Order Notes Fields

Order Notes is free-form text related to an Order.

|     Field     |    Type    |                            Description                             |
| :------------ | :--------- | ------------------------------------------------------------------ |
| `created`     | `datetime` | Order note creation date and time.                                 |
| `text`        | `string`   | Order note content up to 2000 characters.                          |
| `type`        | `string`   | Type of Order associated with order note, always and only `order`. |
| `event_id`    | `event`    | ID of Event associated with Order.                                 |
| `order_id`    | `order`    | ID of Order associated with order note.                            |
| `author_name` | `string`   | First and last name Order owner associated with order note.        |

#### Expansions

Information from expansions fields are not normally returned when requesting information.
To receive this information in a request, expand the request.

|     Expansion                |       Source                                            |                                        Description                                         |
| :--------------------------- | :------------------------------------------------------ | ------------------------------------------------------------------------------------------ |
| `event`                      | `event_id`                                              | Order's associated Event.                                                                  |
| `attendees`                  | `attendee`(list)                                        | Order's Attendees.                                                                         |
| `merchandise`                | `merchandise`(list)                                     | Merchandise included in this Order.                                                        |
| `concierge`                  | `concierge`                                             | Order's concierge.                                                                         |
| `refund_requests`            | `refund_request`                                        | Order's refund request.                                                                    |
| `survey`                     | `order-questions`                                       | (Optional) Order's custom questions.                                                       |
| `survey_responses`           | `order-survey-responses`(object)                        | (Optional) Order's responses to survey questions.                                          |
| `answers`                    | `order-answers`                                         | (Optional) Order's answers to custom questions.                                            |
| `ticket_buyer_settings`      | `ticket_buyer_settings`                                 | (Optional) Include information relevant to the purchaser, including confirmation messages. |
| `contact_list_preferences`   | `contact_list_preferences`                              | (Optional) Opt-in preferences for the email address associated with the Order.             |

- **`eventbrite-pp-cli orders <order_id>`** - Retrieve an Order by Order ID.

### organizations

<a name="organization_object"></a>

## Organization Object

An object representing a business structure (like a Marketing department) in which [Events](#event_object) are created and managed. Organizations are owned by one [User](#users_object) and can have multiple [Members](#members_object).

The Organization object is used to group Members, [Roles](#roles_object), [Venues](#venue_object) and [Assortments](#assortments_object).

#### Public Fields

Use these fields to specify information about an Organization.

|   Field    |   Type   |                                                                       Description                                                                        |
| :--------- | :------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`       | `string` | Organization ID. Must be obtained via an API request, such as a List your Organizations request. The `organization_id` is NOT equal to an `organizer_id` (the string in an Organizer Profile URL). |
| `name`     | `string` | Organization Name.                                                                                                                                       |
| `image_id` | `string` | (Optional) ID of the image for an Organization.                                                                                                          |
| `vertical` | `string` | Type of business vertical within which this Organization operates. Currently, the only values are `default` and `music`. If not specified, the value is `default`. |

### pricing

<a name="Pricing_object"></a>

## Pricing Object

The Pricing object represents all the available fee rates for different currencies, countries, [Assortments](#assortments_object) and sales channels.

- **`eventbrite-pp-cli pricing calculate-items`** - Calculates the Fees that Eventbrite would charge for a given price as it’s shown on the ticket authoring flow. This price would be hypothetical, as the pricing calculation would be based on the passed parameters instead of facts as it happens when an order is created.

This price is a simplified view. The price reported can’t be used to calculate the price of an order. Its used to get fees, taxes and total price depending from scope parameter. The scope can be as one of the members: `organization`, `event`, `ticket_class` or `assortment_plan`.

Depending on the scope type, the scope identifier has different meanings:
For scope.type ``organization`` scope.identifier represents an organization id.
For scope.type ``event`` scope.identifier represents an event id.
For scope.type ``ticket_class`` scope.identifier represents a ticket class id.
For scope.type ``assortment_plan`` scope.identifier can take a value of either 'package1' or 'package2'

Returns a `item_pricing` according to the provided base price and scope.
- **`eventbrite-pp-cli pricing list`** - List all available Pricing rates. Returns a paginated response.

### reports

<a name="reports_object"></a>

## Report Object

The Report object represents the Reports that you can retrieve using the API. This includes Reports on:

+ Sales activity

+ [Attendees](#attendee_object) for an [Event](#event_object).

- **`eventbrite-pp-cli reports retrieve-a-attendee`** - Retrieve an Attendee Report by Event ID or Event status.
- **`eventbrite-pp-cli reports retrieve-a-sales`** - Retrieve a sales Report by Event ID or Event status.

### series

Manage series

- **`eventbrite-pp-cli series <event_series_id>`** - Retrieve the parent Event Series by Event Series ID.

### subcategories

Manage subcategories

- **`eventbrite-pp-cli subcategories list-of`** - List all available Subcategories. Returns a paginated response.
- **`eventbrite-pp-cli subcategories subcategory-by-id`** - Retrieve a Subcategory by Subcategory ID.

### ticket-groups

<a name="ticket_group_object"></a>

## Ticket Group Object

The Ticket Group object is used to group [Ticket Classes](#ticket_class_object).

Most commonly used to apply a [Cross-Event Discount](#discount_object) to multiple Ticket Classes.

#### Fields

|       Field        |     Type     |                                                                                             Description                                                                                             |
| :----------------- | :----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`             | `string`     | Ticket Group name. A name containing more than 20 characters is automatically truncated.                                                                                                            |
| `status`           | `string`     | Ticket Group status. Can be `transfer`, `live`, `deleted` or `archived`. By default, the status is `live`.                                                                                          |
| `event_ticket_ids` | `dict`       | Dictionary showing the Ticket Class IDs associated with a specific Event ID.                                                                                                                        |
| `tickets`          | `objectlist` | List of Ticket Class. Includes for each Ticket Class `id`, `event_id`, `sales_channels`, `variants` and `name`. By default this field is empty, unless the Ticket Class Expansions fields are used. |

- **`eventbrite-pp-cli ticket-groups delete-a`** - Delete a Ticket Group. The status of the Ticket Group is changed to `deleted`.
- **`eventbrite-pp-cli ticket-groups retrieve-a`** - Retrieve a Ticket Group by Ticket Group ID.
- **`eventbrite-pp-cli ticket-groups update-a`** - Update Ticket Group by Ticket Group ID.

### users

>**Note:** Note These URLs will accept “me” in place of a user ID in URLs - for example, /users/me/orders/ will return orders placed by the current user.

<a name="user_object"></a>

## User Object

User is an object representing an Eventbrite account. Users are [Members](#members_object) of an [Organization](#organization_object).

- **`eventbrite-pp-cli users list-your-organizations`** - List the Organizations to which you are a Member. Returns a paginated response.
- **`eventbrite-pp-cli users retrieve-information-about-a-account`** - Returns a user for the specified `user` as user. If you want to get details about the currently authenticated user, use /users/me/.
To include the User’s assortment package in the response, add the assortment expansion parameter: /users/me/?expand=assortment
- **`eventbrite-pp-cli users retrieve-information-about-your-account`** - Retrieve Information About Your User Account

### venues

<a name="venue_object"></a>

## Venue Object

The Venue object represents the location of an [Event](#event_object) (i.e. where an Event takes place).

Venues are grouped together by the [Organization](#organization_object) object.

#### Venue Fields

|       Field       |   Type    |                        Description                         |
| :---------------- | :-------- | ---------------------------------------------------------- |
| `address`         | `address` | Venue address.                                             |
| `id`              | `string`  | Venue ID.                                                  |
| `age_restriction` | `string`  | Age restriction of the Venue.                              |
| `capacity`        | `number`  | Maximum number of tickets that can be sold for the Venue.  |
| `name`            | `string`  | Venue name.                                                |
| `latitude`        | `string`  | Latitude coordinates of the Venue address.                 |
| `longitude`       | `string`  | Longitude coordinates of the Venue address.                |

- **`eventbrite-pp-cli venues retrieve-a`** - Retrieve a Venue by Venue ID.
- **`eventbrite-pp-cli venues update-a`** - Update a Venue by Venue ID.

### webhooks

<a name="webhooks_object"></a>

## Webhook Object

An object representing a webhook associated with the Organization.

- **`eventbrite-pp-cli webhooks create-deprecated`** - Create a Webhook.

> Warning: Access to this API will be no longer usable on June 1st, 2020.

For more information regarding deprecated APIs, refer to our [changelog](https://www.eventbrite.com/platform/docs/changelog).
- **`eventbrite-pp-cli webhooks delete-by-id`** - Delete a Webhook by ID.
- **`eventbrite-pp-cli webhooks list-of-deprecation`** - List Webhooks.

> Warning: Access to this API will be no longer usable on June 1st, 2020.

For more information regarding deprecated APIs, refer to our [changelog](https://www.eventbrite.com/platform/docs/changelog).

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
eventbrite-pp-cli formats list

# JSON for scripting and agents
eventbrite-pp-cli formats list --json

# Filter to specific fields
eventbrite-pp-cli formats list --json --select id,name,status

# Dry run — show the request without sending
eventbrite-pp-cli formats list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
eventbrite-pp-cli formats list --agent
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
eventbrite-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/eventbrite-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EVENTBRITE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `eventbrite-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EVENTBRITE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Set EVENTBRITE_API_KEY to a private token from eventbrite.com/platform/api-keys, then run `eventbrite-pp-cli doctor`.
- **Public event search returns nothing or 404** — Eventbrite removed public event search in 2020; sync your org first with `sync` and search the local store with `search` or `repeat-attendees`.
- **sync stops before all records load** — List endpoints page with continuation tokens; re-run `eventbrite-pp-cli sync --full` to walk every page.
- **sales-velocity or capacity returns empty** — These read the local store — run `eventbrite-pp-cli sync` first.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**eventbrite-sdk-python**](https://github.com/eventbrite/eventbrite-sdk-python) — Python
- [**eventbrite-sdk-javascript**](https://github.com/eventbrite/eventbrite-sdk-javascript) — JavaScript
- [**eventbrite-sdk-php**](https://github.com/eventbrite/eventbrite-sdk-php) — PHP
- [**GearPlug/eventbrite-python**](https://github.com/GearPlug/eventbrite-python) — Python
- [**ibraheem4/eventbrite-mcp**](https://github.com/ibraheem4/eventbrite-mcp) — TypeScript
- [**joshuachestang/eventbrite-mcp-server**](https://github.com/joshuachestang/eventbrite-mcp-server) — TypeScript
- [**punkpeye/eventbrite-mcp**](https://github.com/punkpeye/eventbrite-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
