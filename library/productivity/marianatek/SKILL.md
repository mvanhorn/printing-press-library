---
name: pp-marianatek
description: "Every Mariana Tek booking feature, plus multi-tenant search, cancellation watching, and an offline class catalog no... Trigger phrases: `book a class at kolmkontrast`, `watch this class for cancellations`, `find me a vinyasa morning class`, `show my Mariana Tek reservations`, `what credits do I have expiring`, `use marianatek`, `run marianatek`."
author: "salmonumbrella"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - marianatek-pp-cli
---

# Mariana Tek — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `marianatek-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install marianatek --cli-only
   ```
2. Verify: `marianatek-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/marianatek/cmd/marianatek-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

marianatek is the first CLI and MCP server for the Mariana Tek booking platform that powers hundreds of boutique-fitness, yoga, sauna, and wellness studios. Beyond mirroring every Customer API endpoint, it adds a local SQLite catalog, FTS5 search across class sessions, cancellation watching (the API exposes no waitlist signal), and joins across tenants that the per-studio iframe widget can't perform.

## When to Use This CLI

Use marianatek whenever a user wants to discover, book, or track classes at any Mariana Tek-powered studio. It is especially valuable for multi-tenant members who book across several brands, for cancellation hunters who need a real waitlist mechanism, and for agentic schedulers that need a machine-readable booking surface. The MCP server exposes every read-only command for agent use; mutating commands are guarded with --dry-run.

## Unique Capabilities

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

## Command Reference

**app-version-metadatas** — Manage app version metadatas

- `marianatek-pp-cli app-version-metadatas list` — View a list of the latest version details for Mariana Tek mobile applications, including if an update is mandatory...
- `marianatek-pp-cli app-version-metadatas retrieve` — Get version metadata details about a single customer application, including version number.

**appointments** — Manage appointments

- `marianatek-pp-cli appointments categories-list` — View a list of appointment service categories available for booking.
- `marianatek-pp-cli appointments services-payment-options-retrieve` — View all available payment options (credits) that can be used to book this service at the specified location....
- `marianatek-pp-cli appointments services-schedule-filters-retrieve` — Returns the available filter choices (service names, instructors, classrooms) for appointment services at the...
- `marianatek-pp-cli appointments services-slots-retrieve` — Retrieve computed availability slots for one or more services at one or more locations within a date range. Returns...

**classes** — Manage classes

- `marianatek-pp-cli classes list` — View a paginated list of classes. Does not include private classes. Filter the class list using a chain of one or...
- `marianatek-pp-cli classes retrieve` — View details about a single class. If this is a pick-a-spot class, the response will include layout data that can be...

**config** — Manage config

- `marianatek-pp-cli config` — Retrieve brand configuration settings including appearance, contact info, and business settings.

**countries** — Manage countries

- `marianatek-pp-cli countries list` — View a list of countries that can be associated with user addresses.
- `marianatek-pp-cli countries retrieve` — View information about a single country.

**customer-feedback** — Manage customer feedback

- `marianatek-pp-cli customer-feedback show-csat-retrieve` — Check if the user should be shown a CSAT survey for an instructor. Returns whether the user has an active survey...
- `marianatek-pp-cli customer-feedback submit-csat-create` — Submit a CSAT (Customer Satisfaction) survey response for an instructor.

**legal** — Manage legal

- `marianatek-pp-cli legal` — Retrieve legal configuration information for a brand including terms of service, privacy policy, and other legal...

**locations** — Manage locations

- `marianatek-pp-cli locations list` — View a paginated list of available locations.
- `marianatek-pp-cli locations retrieve` — View detailed information about a specific location.

**me** — Manage me

- `marianatek-pp-cli me account-create` — After a new user is created, your application should use OAuth2.0 to obtain an access token for this user. To make...
- `marianatek-pp-cli me account-destroy` — Delete (archive) the user's account. Requires the user to submit password to continue.
- `marianatek-pp-cli me account-partial-update` — Update profile information for the current user.
- `marianatek-pp-cli me account-redeem-giftcard-create` — Redeeming a gift card will add the full value of the card with the matching redemption code to the user's account...
- `marianatek-pp-cli me account-retrieve` — View account details for the current user.
- `marianatek-pp-cli me account-update-communications-preferences-create` — Opt a user in or out of SMS communications.
- `marianatek-pp-cli me account-upload-profile-image-create` — Uploading a profile image to a user's account `(as multipart/form-data)`. Image file size should be no larger than...
- `marianatek-pp-cli me achievements-retrieve` — View achievement details for the current user.
- `marianatek-pp-cli me appointments-bookings-create` — Book a new appointment for the authenticated customer. The customer is derived from the auth token; do not pass...
- `marianatek-pp-cli me appointments-bookings-list` — View a list of a user's appointments. Filter the bookings using `start_date`, `end_date`, and `is_upcoming` query...
- `marianatek-pp-cli me appointments-bookings-retrieve` — Retrieve details of a specific user appointment using a composite ID. The ID parameter should be in numeric format:...
- `marianatek-pp-cli me credit-cards-create` — Credit cards create
- `marianatek-pp-cli me credit-cards-destroy` — Credit cards destroy
- `marianatek-pp-cli me credit-cards-destroy-2` — Credit cards destroy 2
- `marianatek-pp-cli me credit-cards-list` — Credit cards list
- `marianatek-pp-cli me credit-cards-partial-update` — Credit cards partial update
- `marianatek-pp-cli me credit-cards-partial-update-2` — Credit cards partial update 2
- `marianatek-pp-cli me credit-cards-retrieve` — Credit cards retrieve
- `marianatek-pp-cli me credit-cards-update` — Credit cards update
- `marianatek-pp-cli me credits-list` — View all credit packages that the user has purchased. To filter out credits that can no longer be used, use the...
- `marianatek-pp-cli me credits-retrieve` — Retrieve details of a specific user credit.
- `marianatek-pp-cli me get-failed-authentication-renewals` — Get memberships that are currently in payment failure status where the basket attempting the renewal is associated...
- `marianatek-pp-cli me memberships-cancel-membership-create` — Cancel a membership_instance. This cancels a membership instance, but allows it to be used until the end of the...
- `marianatek-pp-cli me memberships-list` — View all memberships that the current user has purchased. To filter out memberships that can no longer be used, use...
- `marianatek-pp-cli me memberships-membership-cancellation-reasons-retrieve` — Get the list of cancellation reasons to be displayed in customer facing applications. These are the valid reasons...
- `marianatek-pp-cli me memberships-renewal-intent-create` — Creates a renewal basket and payment intent for a membership instance in payment failure status. Optionally pass a...
- `marianatek-pp-cli me memberships-retrieve` — View details about a single membership belonging to the current user.
- `marianatek-pp-cli me metrics-class-count-retrieve` — Calculates the number of classes taken for a user within a specified date range. Also returns the user's percentile...
- `marianatek-pp-cli me metrics-longest-weekly-streak-retrieve` — Calculates the longest weekly class streak for a user within a specified date range.
- `marianatek-pp-cli me metrics-most-active-month-retrieve` — Calculates the month when the user was most active in attending classes within a specified date range.
- `marianatek-pp-cli me metrics-most-popular-weekday-retrieve` — Calculates the weekday when the user most frequently attends classes within a specified date range.
- `marianatek-pp-cli me metrics-most-recent-completed-challenge-retrieve` — Calculates the most recent challenge completed by the user within a specified date range.
- `marianatek-pp-cli me metrics-top-instructors-retrieve` — Calculates the top instructors based on the user's class attendance within a specified date range.
- `marianatek-pp-cli me metrics-top-time-of-day-retrieve` — Calculates the time of day when the user most frequently attends classes within a specified date range. Morning...
- `marianatek-pp-cli me metrics-total-minutes-in-class-retrieve` — Calculates the total number of minutes the user has spent in classes within a specified date range.
- `marianatek-pp-cli me orders-cancel-create` — Cancel a specific user order. This action is irreversible.
- `marianatek-pp-cli me orders-list` — View the order history for the current user. The following query parameters can be used to assist in filtering...
- `marianatek-pp-cli me orders-retrieve` — View details about a single order.
- `marianatek-pp-cli me reservations-assign-to-spot-create` — Move waitlist or standby reservation to class This endpoint should be used to move customers from the waitlist or on...
- `marianatek-pp-cli me reservations-cancel-create` — Cancel a reservation previously booked by the current user.
- `marianatek-pp-cli me reservations-cancel-penalty-retrieve` — Check whether the user will incur a penalty if they choose to cancel at the current time. Users should always be...
- `marianatek-pp-cli me reservations-cart-add-product-listing-create` — Add a product listing to the current user's add-ons cart.
- `marianatek-pp-cli me reservations-cart-adjust-quantity-create` — Increase or decrease the quantity of a product within an open add-ons cart.
- `marianatek-pp-cli me reservations-cart-apply-discount-code-create` — Apply a discount code to an add-ons cart.
- `marianatek-pp-cli me reservations-cart-checkout-create` — Submit payment for an add-ons cart, resulting in the creation of an add-ons order.
- `marianatek-pp-cli me reservations-cart-clear-create` — Remove all items from an add-ons cart.
- `marianatek-pp-cli me reservations-cart-clear-discount-codes-create` — Remove a discount code that has been applied to an add-ons cart.
- `marianatek-pp-cli me reservations-cart-list` — Retrieve an open cart for an add-ons order for the given reservation.
- `marianatek-pp-cli me reservations-cart-retrieve` — Retrieve the Cart associated with a specific reservation. This cart can be used to add products and services related...
- `marianatek-pp-cli me reservations-check-in-create` — Check-in the user for a previously booked reservation. If geo check-in is enabled for a tenant, check-in actions are...
- `marianatek-pp-cli me reservations-create` — Create a new reservation for the authenticated user. reservation_type accepts one of three defined values:...
- `marianatek-pp-cli me reservations-list` — View details about this user's reservation history.
- `marianatek-pp-cli me reservations-retrieve` — View details about one of the current user's reservations.
- `marianatek-pp-cli me reservations-swap-spots-create` — For a given reservation, swap its current spot with a different spot.

**regions** — Manage regions

- `marianatek-pp-cli regions list` — View the class types, classrooms, and instructors for all classes in this region.
- `marianatek-pp-cli regions retrieve` — View the class types, classrooms, and instructors for all classes in this region.

**schedule-locations** — Manage schedule locations

- `marianatek-pp-cli schedule-locations list` — View a list of active locations along with information relevant to their class schedules.
- `marianatek-pp-cli schedule-locations retrieve` — Get details about a single location along with information needed to display its class schedule, including class...

**schedule-regions** — Manage schedule regions

- `marianatek-pp-cli schedule-regions list` — View a list of active regions along with information relevant to their class schedules.
- `marianatek-pp-cli schedule-regions retrieve` — Get details about a single region along with information needed to display its class schedule, including class...

**theme** — Manage theme

- `marianatek-pp-cli theme` — View branding information that can be used for styling applications.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
marianatek-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find an open vinyasa morning slot across every studio you belong to

```bash
marianatek search "vinyasa morning" --any-tenant --agent --select tenant,start_time,location,instructor,spots_left
```

FTS5 ranks matches across the local cache; --select keeps the response shape narrow so the agent can fit several results in context.

### Watch a sold-out class and auto-book on cancellation

```bash
marianatek watch 84212 --interval 60s --auto-book --agent
```

Streams NDJSON events. The first --auto-book success emits a confirmed-reservation event and the command exits 0.

### List credit packs that expire in the next 30 days

```bash
marianatek expiring --within 720h --agent --select tenant,expires_at,remaining,suggested_classes
```

Joins balance + expiry + candidate class costs. --select trims to the fields an agent uses to recommend an action.

### Find your top 5 instructors by reservation count

```bash
marianatek regulars --by instructor --top 5 --agent
```

Groups local reservations by instructor; outputs counts, last-booked date, and the most common location/time pairing.

### Detect calendar conflicts across studios for a given day

```bash
marianatek conflicts 2026-05-15 --ics ~/Downloads/calendar.ics --buffer 30m --agent
```

Reads reservations across every logged-in tenant, optionally an exported ICS, flags overlaps + sub-buffer pairs. Run before confirming back-to-back bookings.

## Auth Setup

Each tenant is its own brand subdomain ({tenant}.marianatek.com), so authentication is per-tenant. Run `marianatek login --tenant kolmkontrast` to launch the OAuth2 Authorization Code flow in your browser; the refresh token lands at `~/.config/marianatek/<tenant>.token.json` (0600). The HTTP client transparently refreshes on 401. For headless / CI use, `marianatek login --tenant <slug> --email --password` performs an OAuth2 password grant. Logout with `marianatek logout --tenant <slug>` deletes the token file.

Run `marianatek-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  marianatek-pp-cli app-version-metadatas list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
marianatek-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
marianatek-pp-cli feedback --stdin < notes.txt
marianatek-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.marianatek-pp-cli/feedback.jsonl`. They are never POSTed unless `MARIANATEK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MARIANATEK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
marianatek-pp-cli profile save briefing --json
marianatek-pp-cli --profile briefing app-version-metadatas list
marianatek-pp-cli profile list --json
marianatek-pp-cli profile show briefing
marianatek-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `marianatek-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add marianatek-pp-mcp -- marianatek-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which marianatek-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   marianatek-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `marianatek-pp-cli <command> --help`.
