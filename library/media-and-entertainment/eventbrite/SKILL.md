---
name: pp-eventbrite
description: "Every Eventbrite organizer endpoint, plus a local SQLite mirror of your events, orders Trigger phrases: `sync my eventbrite events`, `which of my events are selling slowest`, `find repeat attendees on eventbrite`, `check in roster for my event`, `how did my discount code perform`, `use eventbrite`, `run eventbrite`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - eventbrite-pp-cli
    install:
      - kind: go
        bins: [eventbrite-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/cmd/eventbrite-pp-cli
---

# Eventbrite — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `eventbrite-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install eventbrite --cli-only
   ```
2. Verify: `eventbrite-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/cmd/eventbrite-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Existing Eventbrite tools wrap a handful of event CRUD calls one event at a time. This CLI mirrors the full v3 organizer API and syncs your events, orders, attendees, ticket classes, and discounts into a local SQLite store, then layers cross-event analytics on top: sales-velocity ranks your live events by sell rate, repeat-attendees surfaces returning fans across your whole history, and org-rollup gives agencies a single pane across every client organization.

## When to Use This CLI

Use this CLI when an agent manages an organizer's Eventbrite presence: creating and publishing events, pulling order and attendee history, and answering cross-event questions (which events sell slowest, which fans return, who the top buyers are, how a discount code performed) that the per-event dashboard and existing single-org MCP servers cannot answer. It is the right tool whenever the task needs the whole order/attendee history in one place rather than a single live API lookup. When the same promoter also sells on DICE, both CLIs emit JSON and a local SQLite store, so an agent can join the two datasets by event (name and date) and by buyer email for a cross-platform performance and loyalty picture.

## Unique Capabilities

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

## Command Reference

**balance** — Manage balance


**categories** — <a name="categories_object"></a>

## Category Object

An overarching category that an event falls into (vertical). Examples are “Music”, and “Endurance”.

- `eventbrite-pp-cli categories category-by-id` — Gets a `category` by ID as ``category``.
- `eventbrite-pp-cli categories list-of` — Returns a list of Category as categories, including subcategories nested. Returns a paginated response.

**discounts** — <a name="discount_object"></a>

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

- `eventbrite-pp-cli discounts delete-a` — Delete a Discount. Only unused Discounts can be deleted.
- `eventbrite-pp-cli discounts retrieve-a` — Retrieve a Discount by Discount ID.
- `eventbrite-pp-cli discounts update-a` — Update a Discount by Discount ID.

**event** — <a name="event_object"></a>

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


**events** — <a name="event_object"></a>

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

- `eventbrite-pp-cli events delete-an` — Delete an Event if the delete is permitted. Returns a boolean indicating the success or failure of the delete action.
- `eventbrite-pp-cli events retrieve-an` — Retrieve an Event by Event ID.
- `eventbrite-pp-cli events update-an` — Update Event by Event ID.

**formats** — <a name="formats_object"></a>

## Format Object

The Format object represents an [Event](#event_object) type, for example seminar, workshop or concert. Specifying a Format helps website visitors discover a certain type of Event.

- `eventbrite-pp-cli formats list` — List all available Formats. Returns a paginated response.
- `eventbrite-pp-cli formats retrieve-a` — Retrieve a Format by Format ID.

**media** — <!-- Here is a tutorial on [Media Upload in EventBrite](https://www.eventbrite.com/developer/v3/resources/uploads/). -->

<a name="media_object"></a>

## Media Object

The Media object represents an image that can be included with an [Event](#event_object) listing, for example to provide branding or further information on the Event.

- `eventbrite-pp-cli media retrieve` — Retrieve Media by Media ID.
- `eventbrite-pp-cli media retrieve-a-upload` — Retrieve information on a Media image upload.
- `eventbrite-pp-cli media upload-a-file` — Upload a Media image file.

**orders** — <a name="order_object"></a>

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

- `eventbrite-pp-cli orders <order_id>` — Retrieve an Order by Order ID.

**organizations** — <a name="organization_object"></a>

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


**pricing** — <a name="Pricing_object"></a>

## Pricing Object

The Pricing object represents all the available fee rates for different currencies, countries, [Assortments](#assortments_object) and sales channels.

- `eventbrite-pp-cli pricing calculate-items` — Calculates the Fees that Eventbrite would charge for a given price as it’s shown on the ticket authoring flow.
- `eventbrite-pp-cli pricing list` — List all available Pricing rates. Returns a paginated response.

**reports** — <a name="reports_object"></a>

## Report Object

The Report object represents the Reports that you can retrieve using the API. This includes Reports on:

+ Sales activity

+ [Attendees](#attendee_object) for an [Event](#event_object).

- `eventbrite-pp-cli reports retrieve-a-attendee` — Retrieve an Attendee Report by Event ID or Event status.
- `eventbrite-pp-cli reports retrieve-a-sales` — Retrieve a sales Report by Event ID or Event status.

**series** — Manage series

- `eventbrite-pp-cli series <event_series_id>` — Retrieve the parent Event Series by Event Series ID.

**subcategories** — Manage subcategories

- `eventbrite-pp-cli subcategories list-of` — List all available Subcategories. Returns a paginated response.
- `eventbrite-pp-cli subcategories subcategory-by-id` — Retrieve a Subcategory by Subcategory ID.

**ticket-groups** — <a name="ticket_group_object"></a>

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

- `eventbrite-pp-cli ticket-groups delete-a` — Delete a Ticket Group. The status of the Ticket Group is changed to `deleted`.
- `eventbrite-pp-cli ticket-groups retrieve-a` — Retrieve a Ticket Group by Ticket Group ID.
- `eventbrite-pp-cli ticket-groups update-a` — Update Ticket Group by Ticket Group ID.

**users** — >**Note:** Note These URLs will accept “me” in place of a user ID in URLs - for example, /users/me/orders/ will return orders placed by the current user.

<a name="user_object"></a>

## User Object

User is an object representing an Eventbrite account. Users are [Members](#members_object) of an [Organization](#organization_object).

- `eventbrite-pp-cli users list-your-organizations` — List the Organizations to which you are a Member. Returns a paginated response.
- `eventbrite-pp-cli users retrieve-information-about-a-account` — Returns a user for the specified `user` as user.
- `eventbrite-pp-cli users retrieve-information-about-your-account` — Retrieve Information About Your User Account

**venues** — <a name="venue_object"></a>

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

- `eventbrite-pp-cli venues retrieve-a` — Retrieve a Venue by Venue ID.
- `eventbrite-pp-cli venues update-a` — Update a Venue by Venue ID.

**webhooks** — <a name="webhooks_object"></a>

## Webhook Object

An object representing a webhook associated with the Organization.

- `eventbrite-pp-cli webhooks create-deprecated` — Create a Webhook. > Warning: Access to this API will be no longer usable on June 1st, 2020.
- `eventbrite-pp-cli webhooks delete-by-id` — Delete a Webhook by ID.
- `eventbrite-pp-cli webhooks list-of-deprecation` — List Webhooks. > Warning: Access to this API will be no longer usable on June 1st, 2020.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
eventbrite-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Eventbrite uses an OAuth2 private token sent as a Bearer header. For your own account you do not need the full OAuth dance — copy a private token from eventbrite.com/platform/api-keys and set EVENTBRITE_API_KEY. Run `eventbrite-pp-cli doctor` to confirm it is picked up.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  eventbrite-pp-cli formats list --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
eventbrite-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
eventbrite-pp-cli feedback --stdin < notes.txt
eventbrite-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.eventbrite-pp-cli/feedback.jsonl`. They are never POSTed unless `EVENTBRITE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EVENTBRITE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
eventbrite-pp-cli profile save briefing --json
eventbrite-pp-cli --profile briefing formats list
eventbrite-pp-cli profile list --json
eventbrite-pp-cli profile show briefing
eventbrite-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `eventbrite-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/cmd/eventbrite-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add eventbrite-pp-mcp -- eventbrite-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which eventbrite-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   eventbrite-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `eventbrite-pp-cli <command> --help`.
