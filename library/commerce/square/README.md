# Square CLI

**The current Square v2 API surface, plus local reconciliation and history the remote API cannot provide by itself.**

Use one scriptable CLI for payments, orders, catalog, inventory, customers, bookings, staff, and developer operations. Sync selected resources locally, then run close reconciliation, inventory drift, customer timelines, webhook health checks, and service reviews with compact agent-ready output.

Learn more at [Square](https://squareup.com/developers).

Created by [@wirelesstkd](https://github.com/wirelesstkd) (matthew.martin).

## Install

The recommended path installs both the `square-pp-cli` binary and the `pp-square` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install square
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install square --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install square --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install square --agent claude-code
npx -y @mvanhorn/printing-press-library install square --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/square/cmd/square-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/square-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install square --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-square --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-square --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install square --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/square-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SQUARE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/square/cmd/square-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "square": {
      "command": "square-pp-mcp",
      "env": {
        "SQUARE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Use a Square sandbox token while learning or testing. Set SQUARE_ACCESS_TOKEN outside your shell history and set SQUARE_BASE_URL=https://connect.squareupsandbox.com for sandbox calls. Inspect mutation help and use --dry-run before explicitly approving any write. Production and sandbox tokens are not interchangeable.

## Quick Start

```bash
# Check configuration and local prerequisites without credentials or network calls.
square-pp-cli doctor --dry-run

# Build a bounded local operating snapshot after sandbox authentication is configured.
square-pp-cli sync --resources locations,catalog,inventory,customers,orders,payments,refunds,disputes,payouts,bookings,invoices,loyalty,team-members,events,webhooks-subscriptions --since 7d --max-pages 5

# Explain the latest gross-to-net settlement differences by location.
square-pp-cli reconcile close --since 24h --agent

# Find catalog and stock changes since the earlier snapshot.
square-pp-cli inventory drift --since 7d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Operational reconciliation
- **`reconcile close`** — Explain the difference between sales, collected payments, refunds, disputes, and payouts for each location.

  _Use this when an operator needs an explainable close instead of several unrelated API responses._

  ```bash
  square-pp-cli reconcile close --since 24h --agent
  ```
- **`inventory drift`** — See which prices, variations, availability settings, and location counts changed since an earlier snapshot.

  _Use this before opening or publishing a menu when current-state API calls cannot explain what changed._

  ```bash
  square-pp-cli inventory drift --since 7d --agent
  ```

### Connected customer operations
- **`customer timeline`** — Follow one customer's orders, payments, refunds, loyalty activity, invoices, and bookings in chronological order.

  _Use this for customer-service investigations that otherwise require carrying IDs between services._

  ```bash
  square-pp-cli customer timeline CUSTOMER_ABC123 --agent
  ```
- **`service review`** — Review completed, unpaid, and missed appointments alongside staff and location utilization.

  _Use this for a service business's weekly operating review instead of assembling separate reports._

  ```bash
  square-pp-cli service review --since 7d --agent
  ```

### Integration confidence
- **`webhook health`** — Measure duplicate locally ingested webhook deliveries, ordering problems, delivery lag, gaps, and subscription changes.

  _Use this before releases or during incidents when a subscription listing cannot reveal delivery behavior._

  ```bash
  square-pp-cli webhook health --since 24h --agent
  ```
- **`request check`** — Validate a planned Square request, environment, API version, mutation policy, and idempotency before anything is sent.

  _Use this before a risky sandbox or production call when schema validity alone is not enough._

  ```bash
  square-pp-cli request check --method POST --path /v2/payments --body payment-request.json --agent
  ```

## Recipes

### Explain yesterday's close

```bash
square-pp-cli reconcile close --since 24h --agent --select locations.location_name,locations.gross_sales,locations.gross_payments,locations.refunds,locations.net_collected,locations.payouts
```

Return only the location and settlement fields an agent needs to explain a discrepancy.

### Review stock drift

```bash
square-pp-cli inventory drift --since 7d --agent
```

Compare retained catalog and inventory snapshots across locations.

### Trace a customer

```bash
square-pp-cli customer timeline CUSTOMER_ABC123 --agent
```

Assemble connected customer activity into one chronological record.

### Check webhook delivery

```bash
square-pp-cli webhook health --since 24h --agent
```

Analyze real webhook receipts previously captured with webhook ingest, plus retained subscription drift.

### Validate a payment request offline

```bash
square-pp-cli request check --method POST --path /v2/payments --body payment-request.json --agent
```

Check the method and path against the shipped API contract, then inspect JSON readability, environment, API version, explicit mutation approval, and idempotency before sending anything.

## Usage

Run `square-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SQUARE_CONFIG_DIR`, `SQUARE_DATA_DIR`, `SQUARE_STATE_DIR`, or `SQUARE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SQUARE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SQUARE_HOME=/srv/square
square-pp-cli doctor
```

Under `SQUARE_HOME=/srv/square`, the four dirs resolve to `/srv/square/config`, `/srv/square/data`, `/srv/square/state`, and `/srv/square/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "square": {
      "command": "square-pp-mcp",
      "env": {
        "SQUARE_HOME": "/srv/square"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SQUARE_DATA_DIR` overrides an explicit `--home` for that kind. Use `SQUARE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SQUARE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `square-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### apple-pay

Manage apple pay

- **`square-pp-cli apple-pay`** - Activates a domain for use with Apple Pay on the Web and Square. A validation
is performed on this domain by Apple to ensure that it is properly set up as
an Apple Pay enabled domain.

This endpoint provides an easy way for platform developers to bulk activate
Apple Pay on the Web with Square for merchants using their platform.

Note: You will need to host a valid domain verification file on your domain to support Apple Pay.  The
current version of this file is always available at https://app.squareup.com/digital-wallets/apple-pay/apple-developer-merchantid-domain-association,
and should be hosted at `.well_known/apple-developer-merchantid-domain-association` on your
domain.  This file is subject to change; we strongly recommend checking for updates regularly and avoiding
long-lived caches that might not keep in sync with the correct file version.

To learn more about the Web Payments SDK and how to add Apple Pay, see [Take an Apple Pay Payment](https://developer.squareup.com/docs/web-payments/apple-pay).

### bank-accounts

Manage bank accounts

- **`square-pp-cli bank-accounts create`** - Store a bank account on file for a square account
- **`square-pp-cli bank-accounts get`** - Retrieve details of a [BankAccount](entity:BankAccount) bank account linked to a Square account.
- **`square-pp-cli bank-accounts get-by-v1-id`** - Returns details of a [BankAccount](entity:BankAccount) identified by V1 bank account ID.
- **`square-pp-cli bank-accounts list`** - Returns a list of [BankAccount](entity:BankAccount) objects linked to a Square account.

### bookings

Manage bookings

- **`square-pp-cli bookings bulk-delete-custom-attributes`** - Bulk deletes bookings custom attributes.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings bulk-retrieve`** - Bulk-Retrieves a list of bookings by booking IDs.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings bulk-retrieve-team-member-profiles`** - Retrieves one or more team members' booking profiles.
- **`square-pp-cli bookings bulk-upsert-custom-attributes`** - Bulk upserts bookings custom attributes.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings create`** - Creates a booking.

The required input must include the following:
- `Booking.location_id`
- `Booking.start_at`
- `Booking.AppointmentSegment.team_member_id`
- `Booking.AppointmentSegment.service_variation_id`
- `Booking.AppointmentSegment.service_variation_version`

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings create-custom-attribute-definition`** - Creates a bookings custom attribute definition.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings delete-custom-attribute-definition`** - Deletes a bookings custom attribute definition.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings list`** - Retrieve a collection of bookings.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings list-custom-attribute-definitions`** - Get all bookings custom attribute definitions.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings list-location-profiles`** - Lists location booking profiles of a seller.
- **`square-pp-cli bookings list-team-member-profiles`** - Lists booking profiles for team members.
- **`square-pp-cli bookings retrieve`** - Retrieves a booking.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings retrieve-business-profile`** - Retrieves a seller's booking profile.
- **`square-pp-cli bookings retrieve-custom-attribute-definition`** - Retrieves a bookings custom attribute definition.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings retrieve-location-profile`** - Retrieves a seller's location booking profile.
- **`square-pp-cli bookings retrieve-team-member-profile`** - Retrieves a team member's booking profile.
- **`square-pp-cli bookings search-availability`** - Searches for availabilities for booking.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_READ` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_READ` and `APPOINTMENTS_READ` for the OAuth scope.
- **`square-pp-cli bookings update`** - Updates a booking.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.
- **`square-pp-cli bookings update-custom-attribute-definition`** - Updates a bookings custom attribute definition.

To call this endpoint with buyer-level permissions, set `APPOINTMENTS_WRITE` for the OAuth scope.
To call this endpoint with seller-level permissions, set `APPOINTMENTS_ALL_WRITE` and `APPOINTMENTS_WRITE` for the OAuth scope.

For calls to this endpoint with seller-level permissions to succeed, the seller must have subscribed to *Appointments Plus*
or *Appointments Premium*.

### cards

Manage cards

- **`square-pp-cli cards create`** - Adds a card on file to an existing merchant.
- **`square-pp-cli cards list`** - Retrieves a list of cards owned by the account making the request.
A max of 25 cards will be returned.
- **`square-pp-cli cards retrieve`** - Retrieves details for a specific Card.

### cash-drawers

Manage cash drawers

- **`square-pp-cli cash-drawers list-shift-events`** - Provides a paginated list of events for a single cash drawer shift.
- **`square-pp-cli cash-drawers list-shifts`** - Provides the details for all of the cash drawer shifts for a location
in a date range.
- **`square-pp-cli cash-drawers retrieve-shift`** - Provides the summary details for a single cash drawer shift. See
[ListCashDrawerShiftEvents](api-endpoint:CashDrawers-ListCashDrawerShiftEvents) for a list of cash drawer shift events.

### catalog

Manage catalog

- **`square-pp-cli catalog batch-delete-objects`** - Deletes a set of [CatalogItem](entity:CatalogItem)s based on the
provided list of target IDs and returns a set of successfully deleted IDs in
the response. Deletion is a cascading event such that all children of the
targeted object are also deleted. For example, deleting a CatalogItem will
also delete all of its [CatalogItemVariation](entity:CatalogItemVariation)
children.

`BatchDeleteCatalogObjects` succeeds even if only a portion of the targeted
IDs can be deleted. The response will only include IDs that were
actually deleted.

To ensure consistency, only one delete request is processed at a time per seller account.
While one (batch or non-batch) delete request is being processed, other (batched and non-batched)
delete requests are rejected with the `429` error code.
- **`square-pp-cli catalog batch-retrieve-objects`** - Returns a set of objects based on the provided ID.
Each [CatalogItem](entity:CatalogItem) returned in the set includes all of its
child information including: all of its
[CatalogItemVariation](entity:CatalogItemVariation) objects, references to
its [CatalogModifierList](entity:CatalogModifierList) objects, and the ids of
any [CatalogTax](entity:CatalogTax) objects that apply to it.
- **`square-pp-cli catalog batch-upsert-objects`** - Creates or updates up to 10,000 target objects based on the provided
list of objects. The target objects are grouped into batches and each batch is
inserted/updated in an all-or-nothing manner. If an object within a batch is
malformed in some way, or violates a database constraint, the entire batch
containing that item will be disregarded. However, other batches in the same
request may still succeed. Each batch may contain up to 1,000 objects, and
batches will be processed in order as long as the total object count for the
request (items, variations, modifier lists, discounts, and taxes) is no more
than 10,000.

This endpoint uses full-replacement semantics. The client must send the complete object, and any
field absent from the request is interpreted as an intentional clear. This logic applies to
nested objects as well. For example, omitting inlined children like variations will delete them.

To ensure consistency, only one update request is processed at a time per seller account.
While one (batch or non-batch) update request is being processed, other (batched and non-batched)
update requests are rejected with the `429` error code. Prefer batching related changes into a
single call rather than issuing many small writes, since each write acquires the lock separately
and parallel writes to the same seller will contend with each other, producing `429` errors.
- **`square-pp-cli catalog create-image`** - Uploads an image file to be represented by a [CatalogImage](entity:CatalogImage) object that can be linked to an existing
[CatalogObject](entity:CatalogObject) instance. The resulting `CatalogImage` is unattached to any `CatalogObject` if the `object_id`
is not specified.

This `CreateCatalogImage` endpoint accepts HTTP multipart/form-data requests with a JSON part and an image file part in
JPEG, PJPEG, PNG, or GIF format. The maximum file size is 15MB.
- **`square-pp-cli catalog delete-object`** - Deletes a single [CatalogObject](entity:CatalogObject) based on the
provided ID and returns the set of successfully deleted IDs in the response.
Deletion is a cascading event such that all children of the targeted object
are also deleted. For example, deleting a [CatalogItem](entity:CatalogItem)
will also delete all of its
[CatalogItemVariation](entity:CatalogItemVariation) children.

To ensure consistency, only one delete request is processed at a time per seller account.
While one (batch or non-batch) delete request is being processed, other (batched and non-batched)
delete requests are rejected with the `429` error code.
- **`square-pp-cli catalog get`** - Retrieves information about the Square Catalog API, such as batch size
limits that can be used by the `BatchUpsertCatalogObjects` endpoint.
- **`square-pp-cli catalog list`** - Returns a list of all [CatalogObject](entity:CatalogObject)s of the specified types in the catalog.

The `types` parameter is specified as a comma-separated list of the [CatalogObjectType](entity:CatalogObjectType) values,
for example, "`ITEM`, `ITEM_VARIATION`, `MODIFIER`, `MODIFIER_LIST`, `CATEGORY`, `DISCOUNT`, `TAX`, `IMAGE`".
Always specify `types` explicitly. When upgrading to a newer API version, omitting `types` may
cause new object types to appear in results that were not returned under the previous version.

__Important:__ ListCatalog does not return deleted catalog items. To retrieve
deleted catalog items, use [SearchCatalogObjects](api-endpoint:Catalog-SearchCatalogObjects)
and set the `include_deleted_objects` attribute value to `true`.
- **`square-pp-cli catalog retrieve-object`** - Returns a single [CatalogItem](entity:CatalogItem) as a
[CatalogObject](entity:CatalogObject) based on the provided ID. The returned
object includes all of the relevant [CatalogItem](entity:CatalogItem)
information including: [CatalogItemVariation](entity:CatalogItemVariation)
children, references to its
[CatalogModifierList](entity:CatalogModifierList) objects, and the ids of
any [CatalogTax](entity:CatalogTax) objects that apply to it.
- **`square-pp-cli catalog search-items`** - Searches for catalog items or item variations by matching supported search attribute values, including
custom attribute values, against one or more of the specified query filters.

This (`SearchCatalogItems`) endpoint differs from the [SearchCatalogObjects](api-endpoint:Catalog-SearchCatalogObjects)
endpoint in the following aspects:

- `SearchCatalogItems` can only search for items or item variations, whereas `SearchCatalogObjects` can search for any type of catalog objects.
- `SearchCatalogItems` supports the custom attribute query filters to return items or item variations that contain custom attribute values, where `SearchCatalogObjects` does not.
- `SearchCatalogItems` does not support the `include_deleted_objects` filter to search for deleted items or item variations, whereas `SearchCatalogObjects` does.
- The both endpoints use different call conventions, including the query filter formats.
- **`square-pp-cli catalog search-objects`** - Searches for [CatalogObject](entity:CatalogObject) of any type by matching supported search attribute values,
excluding custom attribute values on items or item variations, against one or more of the specified query filters.

This (`SearchCatalogObjects`) endpoint differs from the [SearchCatalogItems](api-endpoint:Catalog-SearchCatalogItems)
endpoint in the following aspects:

- `SearchCatalogItems` can only search for items or item variations, whereas `SearchCatalogObjects` can search for any type of catalog objects.
- `SearchCatalogItems` supports the custom attribute query filters to return items or item variations that contain custom attribute values, where `SearchCatalogObjects` does not.
- `SearchCatalogItems` does not support the `include_deleted_objects` filter to search for deleted items or item variations, whereas `SearchCatalogObjects` does.
- The both endpoints have different call conventions, including the query filter formats.

The `object_types` parameter is specified as a list of [CatalogObjectType](entity:CatalogObjectType) values.
Always specify `object_types` explicitly. When upgrading to a newer API version, omitting
`object_types` may cause new object types to appear in results that were not returned under
the previous version.
- **`square-pp-cli catalog update-image`** - Uploads a new image file to replace the existing one in the specified [CatalogImage](entity:CatalogImage) object.

This `UpdateCatalogImage` endpoint accepts HTTP multipart/form-data requests with a JSON part and an image file part in
JPEG, PJPEG, PNG, or GIF format. The maximum file size is 15MB.
- **`square-pp-cli catalog update-item-modifier-lists`** - Updates the [CatalogModifierList](entity:CatalogModifierList) objects
that apply to the targeted [CatalogItem](entity:CatalogItem) without having
to perform an upsert on the entire item.
- **`square-pp-cli catalog update-item-taxes`** - Updates the [CatalogTax](entity:CatalogTax) objects that apply to the
targeted [CatalogItem](entity:CatalogItem) without having to perform an
upsert on the entire item.
- **`square-pp-cli catalog upsert-object`** - Creates a new or updates the specified [CatalogObject](entity:CatalogObject).

This endpoint uses full-replacement semantics. The client must send the complete object, and any
field absent from the request is interpreted as an intentional clear. This logic applies to
nested objects as well. For example, omitting inlined children like variations will delete them.

To ensure consistency, only one update request is processed at a time per seller account.
While one (batch or non-batch) update request is being processed, other (batched and non-batched)
update requests are rejected with the `429` error code.

### channels

Manage channels

- **`square-pp-cli channels bulk-retrieve`** - Bulk retrieve channels
- **`square-pp-cli channels list`** - List channels
- **`square-pp-cli channels retrieve`** - Retrieve channel

### customers

Manage customers

- **`square-pp-cli customers bulk-create`** - Creates multiple [customer profiles](entity:Customer) for a business.

This endpoint takes a map of individual create requests and returns a map of responses.

You must provide at least one of the following values in each create request:

- `given_name`
- `family_name`
- `company_name`
- `email_address`
- `phone_number`
- **`square-pp-cli customers bulk-delete`** - Deletes multiple customer profiles.

The endpoint takes a list of customer IDs and returns a map of responses.
- **`square-pp-cli customers bulk-retrieve`** - Retrieves multiple customer profiles.

This endpoint takes a list of customer IDs and returns a map of responses.
- **`square-pp-cli customers bulk-update`** - Updates multiple customer profiles.

This endpoint takes a map of individual update requests and returns a map of responses.
- **`square-pp-cli customers bulk-upsert-custom-attributes`** - Creates or updates [custom attributes](entity:CustomAttribute) for customer profiles as a bulk operation.

Use this endpoint to set the value of one or more custom attributes for one or more customer profiles.
A custom attribute is based on a custom attribute definition in a Square seller account, which is
created using the [CreateCustomerCustomAttributeDefinition](api-endpoint:CustomerCustomAttributes-CreateCustomerCustomAttributeDefinition) endpoint.

This `BulkUpsertCustomerCustomAttributes` endpoint accepts a map of 1 to 25 individual upsert
requests and returns a map of individual upsert responses. Each upsert request has a unique ID
and provides a customer ID and custom attribute. Each upsert response is returned with the ID
of the corresponding request.

To create or update a custom attribute owned by another application, the `visibility` setting
must be `VISIBILITY_READ_WRITE_VALUES`. Note that seller-defined custom attributes
(also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli customers create`** - Creates a new customer for a business.

You must provide at least one of the following values in your request to this
endpoint:

- `given_name`
- `family_name`
- `company_name`
- `email_address`
- `phone_number`
- **`square-pp-cli customers create-custom-attribute-definition`** - Creates a customer-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
Use this endpoint to define a custom attribute that can be associated with customer profiles.

A custom attribute definition specifies the `key`, `visibility`, `schema`, and other properties
for a custom attribute. After the definition is created, you can call
[UpsertCustomerCustomAttribute](api-endpoint:CustomerCustomAttributes-UpsertCustomerCustomAttribute) or
[BulkUpsertCustomerCustomAttributes](api-endpoint:CustomerCustomAttributes-BulkUpsertCustomerCustomAttributes)
to set the custom attribute for customer profiles in the seller's Customer Directory.

Sellers can view all custom attributes in exported customer data, including those set to
`VISIBILITY_HIDDEN`.
- **`square-pp-cli customers create-group`** - Creates a new customer group for a business.

The request must include the `name` value of the group.
- **`square-pp-cli customers delete`** - Deletes a customer profile from a business.

To delete a customer profile that was created by merging existing profiles, you must use the ID of the newly created profile.
- **`square-pp-cli customers delete-custom-attribute-definition`** - Deletes a customer-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.

Deleting a custom attribute definition also deletes the corresponding custom attribute from
all customer profiles in the seller's Customer Directory.

Only the definition owner can delete a custom attribute definition.
- **`square-pp-cli customers delete-group`** - Deletes a customer group as identified by the `group_id` value.
- **`square-pp-cli customers list`** - Lists customer profiles associated with a Square account.

Under normal operating conditions, newly created or updated customer profiles become available
for the listing operation in well under 30 seconds. Occasionally, propagation of the new or updated
profiles can take closer to one minute or longer, especially during network incidents and outages.
- **`square-pp-cli customers list-custom-attribute-definitions`** - Lists the customer-related [custom attribute definitions](entity:CustomAttributeDefinition) that belong to a Square seller account.

When all response pages are retrieved, the results include all custom attribute definitions
that are visible to the requesting application, including those that are created by other
applications and set to `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`. Note that
seller-defined custom attributes (also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli customers list-groups`** - Retrieves the list of customer groups of a business.
- **`square-pp-cli customers list-segments`** - Retrieves the list of customer segments of a business.
- **`square-pp-cli customers retrieve`** - Returns details for a single customer.
- **`square-pp-cli customers retrieve-custom-attribute-definition`** - Retrieves a customer-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.

To retrieve a custom attribute definition created by another application, the `visibility`
setting must be `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`. Note that seller-defined custom attributes
(also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli customers retrieve-group`** - Retrieves a specific customer group as identified by the `group_id` value.
- **`square-pp-cli customers retrieve-segment`** - Retrieves a specific customer segment as identified by the `segment_id` value.
- **`square-pp-cli customers search`** - Searches the customer profiles associated with a Square account using one or more supported query filters.

Calling `SearchCustomers` without any explicit query filter returns all
customer profiles ordered alphabetically based on `given_name` and
`family_name`.

Under normal operating conditions, newly created or updated customer profiles become available
for the search operation in well under 30 seconds. Occasionally, propagation of the new or updated
profiles can take closer to one minute or longer, especially during network incidents and outages.
- **`square-pp-cli customers update`** - Updates a customer profile. This endpoint supports sparse updates, so only new or changed fields are required in the request.
To add or update a field, specify the new value. To remove a field, specify `null`.

To update a customer profile that was created by merging existing profiles, you must use the ID of the newly created profile.
- **`square-pp-cli customers update-custom-attribute-definition`** - Updates a customer-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.

Use this endpoint to update the following fields: `name`, `description`, `visibility`, or the
`schema` for a `Selection` data type.

Only the definition owner can update a custom attribute definition. Note that sellers can view
all custom attributes in exported customer data, including those set to `VISIBILITY_HIDDEN`.
- **`square-pp-cli customers update-group`** - Updates a customer group as identified by the `group_id` value.

### devices

Manage devices

- **`square-pp-cli devices create-code`** - Creates a DeviceCode that can be used to login to a Square Terminal device to enter the connected
terminal mode.
- **`square-pp-cli devices get`** - Retrieves Device with the associated `device_id`.
- **`square-pp-cli devices get-code`** - Retrieves DeviceCode with the associated ID.
- **`square-pp-cli devices list`** - List devices associated with the merchant. Currently, only Terminal API
devices are supported.
- **`square-pp-cli devices list-codes`** - Lists all DeviceCodes associated with the merchant.

### disputes

Manage disputes

- **`square-pp-cli disputes list`** - Returns a list of disputes associated with a particular account.
- **`square-pp-cli disputes retrieve`** - Returns details about a specific dispute.

### employees

Manage employees

- **`square-pp-cli employees list`** - List employees
- **`square-pp-cli employees retrieve`** - Retrieve employee

### events

Manage events

- **`square-pp-cli events disable`** - Disables events to prevent them from being searchable.
All events are disabled by default. You must enable events to make them searchable.
Disabling events for a specific time period prevents them from being searchable, even if you re-enable them later.
- **`square-pp-cli events enable`** - Enables events to make them searchable. Only events that occur while in the enabled state are searchable.
- **`square-pp-cli events list-types`** - Lists all event types that you can subscribe to as webhooks or query using the Events API.
- **`square-pp-cli events search`** - Search for Square API events that occur within a 28-day timeframe.

### gift-cards

Manage gift cards

- **`square-pp-cli gift-cards create`** - Creates a digital gift card or registers a physical (plastic) gift card. The resulting gift card
has a `PENDING` state. To activate a gift card so that it can be redeemed for purchases, call
[CreateGiftCardActivity](api-endpoint:GiftCardActivities-CreateGiftCardActivity) and create an `ACTIVATE`
activity with the initial balance. Alternatively, you can use [RefundPayment](api-endpoint:Refunds-RefundPayment)
to refund a payment to the new gift card.
- **`square-pp-cli gift-cards create-activity`** - Creates a gift card activity to manage the balance or state of a [gift card](entity:GiftCard).
For example, create an `ACTIVATE` activity to activate a gift card with an initial balance before first use.
- **`square-pp-cli gift-cards list`** - Lists all gift cards. You can specify optional filters to retrieve 
a subset of the gift cards. Results are sorted by `created_at` in ascending order.
- **`square-pp-cli gift-cards list-activities`** - Lists gift card activities. By default, you get gift card activities for all
gift cards in the seller's account. You can optionally specify query parameters to
filter the list. For example, you can get a list of gift card activities for a gift card,
for all gift cards in a specific region, or for activities within a time window.
- **`square-pp-cli gift-cards retrieve`** - Retrieves a gift card using the gift card ID.
- **`square-pp-cli gift-cards retrieve-from-gan`** - Retrieves a gift card using the gift card account number (GAN).
- **`square-pp-cli gift-cards retrieve-from-nonce`** - Retrieves a gift card using a secure payment token that represents the gift card.

### inventory

Manage inventory

- **`square-pp-cli inventory batch-change`** - Applies adjustments and counts to the provided item quantities.

On success: returns the current calculated counts for all objects
referenced in the request.
On failure: returns a list of related errors.
- **`square-pp-cli inventory batch-retrieve-changes`** - Returns historical physical counts and adjustments based on the
provided filter criteria.

Results are paginated and sorted in ascending order according their
`occurred_at` timestamp (oldest first).

BatchRetrieveInventoryChanges is a catch-all query endpoint for queries
that cannot be handled by other, simpler endpoints.
- **`square-pp-cli inventory batch-retrieve-counts`** - Returns current counts for the provided
[CatalogObject](entity:CatalogObject)s at the requested
[Location](entity:Location)s.

Results are paginated and sorted in descending order according to their
`calculated_at` timestamp (newest first).

When `updated_after` is specified, only counts that have changed since that
time (based on the server timestamp for the most recent change) are
returned. This allows clients to perform a "sync" operation, for example
in response to receiving a Webhook notification.
- **`square-pp-cli inventory create-adjustment-reason`** - Creates a custom inventory adjustment reason.
- **`square-pp-cli inventory delete-adjustment-reason`** - Soft deletes a custom inventory adjustment reason.
- **`square-pp-cli inventory deprecated-batch-change`** - Deprecated version of [BatchChangeInventory](api-endpoint:Inventory-BatchChangeInventory) after the endpoint URL
is updated to conform to the standard convention.
- **`square-pp-cli inventory deprecated-batch-retrieve-changes`** - Deprecated version of [BatchRetrieveInventoryChanges](api-endpoint:Inventory-BatchRetrieveInventoryChanges) after the endpoint URL
is updated to conform to the standard convention.
- **`square-pp-cli inventory deprecated-batch-retrieve-counts`** - Deprecated version of [BatchRetrieveInventoryCounts](api-endpoint:Inventory-BatchRetrieveInventoryCounts) after the endpoint URL
is updated to conform to the standard convention.
- **`square-pp-cli inventory deprecated-retrieve-adjustment`** - Deprecated version of [RetrieveInventoryAdjustment](api-endpoint:Inventory-RetrieveInventoryAdjustment) after the endpoint URL
is updated to conform to the standard convention.
- **`square-pp-cli inventory deprecated-retrieve-physical-count`** - Deprecated version of [RetrieveInventoryPhysicalCount](api-endpoint:Inventory-RetrieveInventoryPhysicalCount) after the endpoint URL
is updated to conform to the standard convention.
- **`square-pp-cli inventory list-adjustment-reasons`** - Returns the standard and custom inventory adjustment reasons available
to the seller.
- **`square-pp-cli inventory restore-adjustment-reason`** - Restores a soft-deleted custom inventory adjustment reason.
- **`square-pp-cli inventory retrieve-adjustment`** - Returns the [InventoryAdjustment](entity:InventoryAdjustment) object
with the provided `adjustment_id`.
- **`square-pp-cli inventory retrieve-adjustment-reason`** - Returns the inventory adjustment reason identified by the provided
`reason_id`. Deleted custom reasons can be retrieved by ID.
- **`square-pp-cli inventory retrieve-count`** - Retrieves the current calculated stock count for a given
[CatalogObject](entity:CatalogObject) at a given set of
[Location](entity:Location)s. Responses are paginated and unsorted.
For more sophisticated queries, use a batch endpoint.
- **`square-pp-cli inventory retrieve-physical-count`** - Returns the [InventoryPhysicalCount](entity:InventoryPhysicalCount)
object with the provided `physical_count_id`.
- **`square-pp-cli inventory update-adjustment`** - Applies an update to the provided adjustment.

On success: returns the newly updated adjustment.
On failure: returns a list of related errors.
- **`square-pp-cli inventory update-adjustment-reason`** - Updates a custom inventory adjustment reason.

### invoices

Manage invoices

- **`square-pp-cli invoices create`** - Creates a draft [invoice](entity:Invoice) 
for an order created using the Orders API.

A draft invoice remains in your account and no action is taken. 
You must publish the invoice before Square can process it (send it to the customer's email address or charge the customer’s card on file).
- **`square-pp-cli invoices delete`** - Deletes the specified invoice. When an invoice is deleted, the 
associated order status changes to CANCELED. You can only delete a draft 
invoice (you cannot delete a published invoice, including one that is scheduled for processing).
- **`square-pp-cli invoices get`** - Retrieves an invoice by invoice ID.
- **`square-pp-cli invoices list`** - Returns a list of invoices for a given location. The response 
is paginated. If truncated, the response includes a `cursor` that you    
use in a subsequent request to retrieve the next set of invoices.
- **`square-pp-cli invoices search`** - Searches for invoices from a location specified in 
the filter. You can optionally specify customers in the filter for whom to 
retrieve invoices. In the current implementation, you can only specify one location and 
optionally one customer.

The response is paginated. If truncated, the response includes a `cursor` 
that you use in a subsequent request to retrieve the next set of invoices.
- **`square-pp-cli invoices update`** - Updates an invoice. This endpoint supports sparse updates, so you only need
to specify the fields you want to change along with the required `version` field.
Some restrictions apply to updating invoices. For example, you cannot change the
`order_id` or `location_id` field.

### labor

Manage labor

- **`square-pp-cli labor bulk-publish-scheduled-shifts`** - Publishes 1 - 100 scheduled shifts. This endpoint takes a map of individual publish
requests and returns a map of responses. When a scheduled shift is published, Square keeps
the `draft_shift_details` field as is and copies it to the `published_shift_details` field.

The minimum `start_at` and maximum `end_at` timestamps of all shifts in a
`BulkPublishScheduledShifts` request must fall within a two-week period.
- **`square-pp-cli labor create-break-type`** - Creates a new `BreakType`.

A `BreakType` is a template for creating `Break` objects.
You must provide the following values in your request to this
endpoint:

- `location_id`
- `break_name`
- `expected_duration`
- `is_paid`

You can only have three `BreakType` instances per location. If you attempt to add a fourth
`BreakType` for a location, an `INVALID_REQUEST_ERROR` "Exceeded limit of 3 breaks per location."
is returned.
- **`square-pp-cli labor create-scheduled-shift`** - Creates a scheduled shift by providing draft shift details such as job ID,
team member assignment, and start and end times.

The following `draft_shift_details` fields are required:
- `location_id`
- `job_id`
- `start_at`
- `end_at`
- **`square-pp-cli labor create-shift`** - Creates a new `Shift`.

A `Shift` represents a complete workday for a single team member.
You must provide the following values in your request to this
endpoint:

- `location_id`
- `team_member_id`
- `start_at`

An attempt to create a new `Shift` can result in a `BAD_REQUEST` error when:
- The `status` of the new `Shift` is `OPEN` and the team member has another
shift with an `OPEN` status.
- The `start_at` date is in the future.
- The `start_at` or `end_at` date overlaps another shift for the same team member.
- The `Break` instances are set in the request and a break `start_at`
is before the `Shift.start_at`, a break `end_at` is after
the `Shift.end_at`, or both.
- **`square-pp-cli labor create-timecard`** - Creates a new `Timecard`.

A `Timecard` represents a complete workday for a single team member.
You must provide the following values in your request to this
endpoint:

- `location_id`
- `team_member_id`
- `start_at`

An attempt to create a new `Timecard` can result in a `BAD_REQUEST` error when:
- The `status` of the new `Timecard` is `OPEN` and the team member has another
timecard with an `OPEN` status.
- The `start_at` date is in the future.
- The `start_at` or `end_at` date overlaps another timecard for the same team member.
- The `Break` instances are set in the request and a break `start_at`
is before the `Timecard.start_at`, a break `end_at` is after
the `Timecard.end_at`, or both.
- **`square-pp-cli labor delete-break-type`** - Deletes an existing `BreakType`.

A `BreakType` can be deleted even if it is referenced from a `Shift`.
- **`square-pp-cli labor delete-shift`** - Deletes a `Shift`.
- **`square-pp-cli labor delete-timecard`** - Deletes a `Timecard`.
- **`square-pp-cli labor get-break-type`** - Returns a single `BreakType` specified by `id`.
- **`square-pp-cli labor get-employee-wage`** - Returns a single `EmployeeWage` specified by `id`.
- **`square-pp-cli labor get-shift`** - Returns a single `Shift` specified by `id`.
- **`square-pp-cli labor get-team-member-wage`** - Returns a single `TeamMemberWage` specified by `id`.
- **`square-pp-cli labor list-break-types`** - Returns a paginated list of `BreakType` instances for a business.
- **`square-pp-cli labor list-employee-wages`** - Returns a paginated list of `EmployeeWage` instances for a business.
- **`square-pp-cli labor list-team-member-wages`** - Returns a paginated list of `TeamMemberWage` instances for a business.
- **`square-pp-cli labor list-workweek-configs`** - Returns a list of `WorkweekConfig` instances for a business.
- **`square-pp-cli labor publish-scheduled-shift`** - Publishes a scheduled shift. When a scheduled shift is published, Square keeps the
`draft_shift_details` field as is and copies it to the `published_shift_details` field.
- **`square-pp-cli labor retrieve-scheduled-shift`** - Retrieves a scheduled shift by ID.
- **`square-pp-cli labor retrieve-timecard`** - Returns a single `Timecard` specified by `id`.
- **`square-pp-cli labor search-scheduled-shifts`** - Returns a paginated list of scheduled shifts, with optional filter and sort settings.
By default, results are sorted by `start_at` in ascending order.
- **`square-pp-cli labor search-shifts`** - Returns a paginated list of `Shift` records for a business.
The list to be returned can be filtered by:
- Location IDs
- Team member IDs
- Shift status (`OPEN` or `CLOSED`)
- Shift start
- Shift end
- Workday details

The list can be sorted by:
- `START_AT`
- `END_AT`
- `CREATED_AT`
- `UPDATED_AT`
- **`square-pp-cli labor search-timecards`** - Returns a paginated list of `Timecard` records for a business.
The list to be returned can be filtered by:
- Location IDs
- Team member IDs
- Timecard status (`OPEN` or `CLOSED`)
- Timecard start
- Timecard end
- Workday details

The list can be sorted by:
- `START_AT`
- `END_AT`
- `CREATED_AT`
- `UPDATED_AT`
- **`square-pp-cli labor update-break-type`** - Updates an existing `BreakType`.
- **`square-pp-cli labor update-scheduled-shift`** - Updates the draft shift details for a scheduled shift. This endpoint supports
sparse updates, so only new, changed, or removed fields are required in the request.
You must publish the shift to make updates public.

You can make the following updates to `draft_shift_details`:
- Change the `location_id`, `job_id`, `start_at`, and `end_at` fields.
- Add, change, or clear the `team_member_id` and `notes` fields. To clear these fields,
set the value to null.
- Change the `is_deleted` field. To delete a scheduled shift, set `is_deleted` to true
and then publish the shift.
- **`square-pp-cli labor update-shift`** - Updates an existing `Shift`.

When adding a `Break` to a `Shift`, any earlier `Break` instances in the `Shift` have
the `end_at` property set to a valid RFC-3339 datetime string.

When closing a `Shift`, all `Break` instances in the `Shift` must be complete with `end_at`
set on each `Break`.
- **`square-pp-cli labor update-timecard`** - Updates an existing `Timecard`.

When adding a `Break` to a `Timecard`, any earlier `Break` instances in the `Timecard` have
the `end_at` property set to a valid RFC-3339 datetime string.

When closing a `Timecard`, all `Break` instances in the `Timecard` must be complete with `end_at`
set on each `Break`.
- **`square-pp-cli labor update-workweek-config`** - Updates a `WorkweekConfig`.

### locations

Manage locations

- **`square-pp-cli locations bulk-delete-custom-attributes`** - Deletes [custom attributes](entity:CustomAttribute) for locations as a bulk operation.
To delete a custom attribute owned by another application, the `visibility` setting must be
`VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli locations bulk-upsert-custom-attributes`** - Creates or updates [custom attributes](entity:CustomAttribute) for locations as a bulk operation.
Use this endpoint to set the value of one or more custom attributes for one or more locations.
A custom attribute is based on a custom attribute definition in a Square seller account, which is
created using the [CreateLocationCustomAttributeDefinition](api-endpoint:LocationCustomAttributes-CreateLocationCustomAttributeDefinition) endpoint.
This `BulkUpsertLocationCustomAttributes` endpoint accepts a map of 1 to 25 individual upsert
requests and returns a map of individual upsert responses. Each upsert request has a unique ID
and provides a location ID and custom attribute. Each upsert response is returned with the ID
of the corresponding request.
To create or update a custom attribute owned by another application, the `visibility` setting
must be `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli locations create`** - Creates a [location](https://developer.squareup.com/docs/locations-api).
Creating new locations allows for separate configuration of receipt layouts, item prices,
and sales reports. Developers can use locations to separate sales activity through applications
that integrate with Square from sales activity elsewhere in a seller's account.
Locations created programmatically with the Locations API last forever and
are visible to the seller for their own management. Therefore, ensure that
each location has a sensible and unique name.
- **`square-pp-cli locations create-custom-attribute-definition`** - Creates a location-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
Use this endpoint to define a custom attribute that can be associated with locations.
A custom attribute definition specifies the `key`, `visibility`, `schema`, and other properties
for a custom attribute. After the definition is created, you can call
[UpsertLocationCustomAttribute](api-endpoint:LocationCustomAttributes-UpsertLocationCustomAttribute) or
[BulkUpsertLocationCustomAttributes](api-endpoint:LocationCustomAttributes-BulkUpsertLocationCustomAttributes)
to set the custom attribute for locations.
- **`square-pp-cli locations delete-custom-attribute-definition`** - Deletes a location-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
Deleting a custom attribute definition also deletes the corresponding custom attribute from
all locations.
Only the definition owner can delete a custom attribute definition.
- **`square-pp-cli locations list`** - Provides details about all of the seller's [locations](https://developer.squareup.com/docs/locations-api),
including those with an inactive status. Locations are listed alphabetically by `name`.
- **`square-pp-cli locations list-custom-attribute-definitions`** - Lists the location-related [custom attribute definitions](entity:CustomAttributeDefinition) that belong to a Square seller account.
When all response pages are retrieved, the results include all custom attribute definitions
that are visible to the requesting application, including those that are created by other
applications and set to `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli locations retrieve`** - Retrieves details of a single location. Specify "main"
as the location ID to retrieve details of the [main location](https://developer.squareup.com/docs/locations-api#about-the-main-location).
- **`square-pp-cli locations retrieve-custom-attribute-definition`** - Retrieves a location-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
To retrieve a custom attribute definition created by another application, the `visibility`
setting must be `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli locations update`** - Updates a [location](https://developer.squareup.com/docs/locations-api).
- **`square-pp-cli locations update-custom-attribute-definition`** - Updates a location-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
Use this endpoint to update the following fields: `name`, `description`, `visibility`, or the
`schema` for a `Selection` data type.
Only the definition owner can update a custom attribute definition.

### loyalty

Manage loyalty

- **`square-pp-cli loyalty accumulate-points`** - Adds points earned from a purchase to a [loyalty account](entity:LoyaltyAccount).

- If you are using the Orders API to manage orders, provide the `order_id`. Square reads the order
to compute the points earned from both the base loyalty program and an associated
[loyalty promotion](entity:LoyaltyPromotion). For purchases that qualify for multiple accrual
rules, Square computes points based on the accrual rule that grants the most points.
For purchases that qualify for multiple promotions, Square computes points based on the most
recently created promotion. A purchase must first qualify for program points to be eligible for promotion points.

- If you are not using the Orders API to manage orders, provide `points` with the number of points to add.
You must first perform a client-side computation of the points earned from the loyalty program and
loyalty promotion. For spend-based and visit-based programs, you can call [CalculateLoyaltyPoints](api-endpoint:Loyalty-CalculateLoyaltyPoints)
to compute the points earned from the base loyalty program. For information about computing points earned from a loyalty promotion, see
[Calculating promotion points](https://developer.squareup.com/docs/loyalty-api/loyalty-promotions#calculate-promotion-points).
- **`square-pp-cli loyalty adjust-points`** - Adds points to or subtracts points from a buyer's account.

Use this endpoint only when you need to manually adjust points. Otherwise, in your application flow, you call
[AccumulateLoyaltyPoints](api-endpoint:Loyalty-AccumulateLoyaltyPoints)
to add points when a buyer pays for the purchase.
- **`square-pp-cli loyalty calculate-points`** - Calculates the number of points a buyer can earn from a purchase. Applications might call this endpoint
to display the points to the buyer.

- If you are using the Orders API to manage orders, provide the `order_id` and (optional) `loyalty_account_id`.
Square reads the order to compute the points earned from the base loyalty program and an associated
[loyalty promotion](entity:LoyaltyPromotion).

- If you are not using the Orders API to manage orders, provide `transaction_amount_money` with the
purchase amount. Square uses this amount to calculate the points earned from the base loyalty program,
but not points earned from a loyalty promotion. For spend-based and visit-based programs, the `tax_mode`
setting of the accrual rule indicates how taxes should be treated for loyalty points accrual.
If the purchase qualifies for program points, call
[ListLoyaltyPromotions](api-endpoint:Loyalty-ListLoyaltyPromotions) and perform a client-side computation
to calculate whether the purchase also qualifies for promotion points. For more information, see
[Calculating promotion points](https://developer.squareup.com/docs/loyalty-api/loyalty-promotions#calculate-promotion-points).
- **`square-pp-cli loyalty cancel-promotion`** - Cancels a loyalty promotion. Use this endpoint to cancel an `ACTIVE` promotion earlier than the
end date, cancel an `ACTIVE` promotion when an end date is not specified, or cancel a `SCHEDULED` promotion.
Because updating a promotion is not supported, you can also use this endpoint to cancel a promotion before
you create a new one.

This endpoint sets the loyalty promotion to the `CANCELED` state
- **`square-pp-cli loyalty create-account`** - Creates a loyalty account. To create a loyalty account, you must provide the `program_id` and a `mapping` with the `phone_number` of the buyer.
- **`square-pp-cli loyalty create-promotion`** - Creates a loyalty promotion for a [loyalty program](entity:LoyaltyProgram). A loyalty promotion
enables buyers to earn points in addition to those earned from the base loyalty program.

This endpoint sets the loyalty promotion to the `ACTIVE` or `SCHEDULED` status, depending on the
`available_time` setting. A loyalty program can have a maximum of 10 loyalty promotions with an
`ACTIVE` or `SCHEDULED` status.
- **`square-pp-cli loyalty create-reward`** - Creates a loyalty reward. In the process, the endpoint does following:

- Uses the `reward_tier_id` in the request to determine the number of points
to lock for this reward.
- If the request includes `order_id`, it adds the reward and related discount to the order.

After a reward is created, the points are locked and
not available for the buyer to redeem another reward.
- **`square-pp-cli loyalty delete-reward`** - Deletes a loyalty reward by doing the following:

- Returns the loyalty points back to the loyalty account.
- If an order ID was specified when the reward was created
(see [CreateLoyaltyReward](api-endpoint:Loyalty-CreateLoyaltyReward)),
it updates the order by removing the reward and related
discounts.

You cannot delete a reward that has reached the terminal state (REDEEMED).
- **`square-pp-cli loyalty list-programs`** - Returns a list of loyalty programs in the seller's account.
Loyalty programs define how buyers can earn points and redeem points for rewards. Square sellers can have only one loyalty program, which is created and managed from the Seller Dashboard. For more information, see [Loyalty Program Overview](https://developer.squareup.com/docs/loyalty/overview).


Replaced with [RetrieveLoyaltyProgram](api-endpoint:Loyalty-RetrieveLoyaltyProgram) when used with the keyword `main`.
- **`square-pp-cli loyalty list-promotions`** - Lists the loyalty promotions associated with a [loyalty program](entity:LoyaltyProgram).
Results are sorted by the `created_at` date in descending order (newest to oldest).
- **`square-pp-cli loyalty redeem-reward`** - Redeems a loyalty reward.

The endpoint sets the reward to the `REDEEMED` terminal state.

If you are using your own order processing system (not using the
Orders API), you call this endpoint after the buyer paid for the
purchase.

After the reward reaches the terminal state, it cannot be deleted.
In other words, points used for the reward cannot be returned
to the account.
- **`square-pp-cli loyalty retrieve-account`** - Retrieves a loyalty account.
- **`square-pp-cli loyalty retrieve-program`** - Retrieves the loyalty program in a seller's account, specified by the program ID or the keyword `main`.

Loyalty programs define how buyers can earn points and redeem points for rewards. Square sellers can have only one loyalty program, which is created and managed from the Seller Dashboard. For more information, see [Loyalty Program Overview](https://developer.squareup.com/docs/loyalty/overview).
- **`square-pp-cli loyalty retrieve-promotion`** - Retrieves a loyalty promotion.
- **`square-pp-cli loyalty retrieve-reward`** - Retrieves a loyalty reward.
- **`square-pp-cli loyalty search-accounts`** - Searches for loyalty accounts in a loyalty program.

You can search for a loyalty account using the phone number or customer ID associated with the account. To return all loyalty accounts, specify an empty `query` object or omit it entirely.

Search results are sorted by `created_at` in ascending order.
- **`square-pp-cli loyalty search-events`** - Searches for loyalty events.

A Square loyalty program maintains a ledger of events that occur during the lifetime of a
buyer's loyalty account. Each change in the point balance
(for example, points earned, points redeemed, and points expired) is
recorded in the ledger. Using this endpoint, you can search the ledger for events.

Search results are sorted by `created_at` in descending order.
- **`square-pp-cli loyalty search-rewards`** - Searches for loyalty rewards. This endpoint accepts a request with no query filters and returns results for all loyalty accounts.
If you include a `query` object, `loyalty_account_id` is required and `status` is  optional.

If you know a reward ID, use the
[RetrieveLoyaltyReward](api-endpoint:Loyalty-RetrieveLoyaltyReward) endpoint.

Search results are sorted by `updated_at` in descending order.

### merchants

Manage merchants

- **`square-pp-cli merchants bulk-delete-custom-attributes`** - Deletes [custom attributes](entity:CustomAttribute) for a merchant as a bulk operation.
To delete a custom attribute owned by another application, the `visibility` setting must be
`VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli merchants bulk-upsert-custom-attributes`** - Creates or updates [custom attributes](entity:CustomAttribute) for a merchant as a bulk operation.
Use this endpoint to set the value of one or more custom attributes for a merchant.
A custom attribute is based on a custom attribute definition in a Square seller account, which is
created using the [CreateMerchantCustomAttributeDefinition](api-endpoint:MerchantCustomAttributes-CreateMerchantCustomAttributeDefinition) endpoint.
This `BulkUpsertMerchantCustomAttributes` endpoint accepts a map of 1 to 25 individual upsert
requests and returns a map of individual upsert responses. Each upsert request has a unique ID
and provides a merchant ID and custom attribute. Each upsert response is returned with the ID
of the corresponding request.
To create or update a custom attribute owned by another application, the `visibility` setting
must be `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli merchants create-custom-attribute-definition`** - Creates a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
Use this endpoint to define a custom attribute that can be associated with a merchant connecting to your application.
A custom attribute definition specifies the `key`, `visibility`, `schema`, and other properties
for a custom attribute. After the definition is created, you can call
[UpsertMerchantCustomAttribute](api-endpoint:MerchantCustomAttributes-UpsertMerchantCustomAttribute) or
[BulkUpsertMerchantCustomAttributes](api-endpoint:MerchantCustomAttributes-BulkUpsertMerchantCustomAttributes)
to set the custom attribute for a merchant.
- **`square-pp-cli merchants delete-custom-attribute-definition`** - Deletes a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
Deleting a custom attribute definition also deletes the corresponding custom attribute from
the merchant.
Only the definition owner can delete a custom attribute definition.
- **`square-pp-cli merchants list`** - Provides details about the merchant associated with a given access token.

The access token used to connect your application to a Square seller is associated
with a single merchant. That means that `ListMerchants` returns a list
with a single `Merchant` object. You can specify your personal access token
to get your own merchant information or specify an OAuth token to get the
information for the merchant that granted your application access.

If you know the merchant ID, you can also use the [RetrieveMerchant](api-endpoint:Merchants-RetrieveMerchant)
endpoint to retrieve the merchant information.
- **`square-pp-cli merchants list-custom-attribute-definitions`** - Lists the merchant-related [custom attribute definitions](entity:CustomAttributeDefinition) that belong to a Square seller account.
When all response pages are retrieved, the results include all custom attribute definitions
that are visible to the requesting application, including those that are created by other
applications and set to `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli merchants retrieve`** - Retrieves the `Merchant` object for the given `merchant_id`.
- **`square-pp-cli merchants retrieve-custom-attribute-definition`** - Retrieves a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.
To retrieve a custom attribute definition created by another application, the `visibility`
setting must be `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli merchants update-custom-attribute-definition`** - Updates a merchant-related [custom attribute definition](entity:CustomAttributeDefinition) for a Square seller account.
Use this endpoint to update the following fields: `name`, `description`, `visibility`, or the
`schema` for a `Selection` data type.
Only the definition owner can update a custom attribute definition.

### oauth2

Manage oauth2

- **`square-pp-cli oauth2 obtain-token`** - Returns an OAuth access token and refresh token using the `authorization_code`
or `refresh_token` grant type.

When `grant_type` is `authorization_code`:
- With the [code flow](https://developer.squareup.com/docs/oauth-api/overview#code-flow),
provide `code`, `client_id`, and `client_secret`.
- With the [PKCE flow](https://developer.squareup.com/docs/oauth-api/overview#pkce-flow),
provide `code`, `client_id`, and `code_verifier`. 

When `grant_type` is `refresh_token`:
- With the code flow, provide `refresh_token`, `client_id`, and `client_secret`.
The response returns the same refresh token provided in the request.
- With the PKCE flow, provide `refresh_token` and `client_id`. The response returns
a new refresh token.

You can use the `scopes` parameter to limit the set of permissions authorized by the
access token. You can use the `short_lived` parameter to create an access token that
expires in 24 hours.

__Important:__ OAuth tokens should be encrypted and stored on a secure server.
Application clients should never interact directly with OAuth tokens.
- **`square-pp-cli oauth2 retrieve-token-status`** - Returns information about an [OAuth access token](https://developer.squareup.com/docs/build-basics/access-tokens#get-an-oauth-access-token) or an application’s [personal access token](https://developer.squareup.com/docs/build-basics/access-tokens#get-a-personal-access-token).

Add the access token to the Authorization header of the request.

__Important:__ The `Authorization` header you provide to this endpoint must have the following format:

```
Authorization: Bearer ACCESS_TOKEN
```

where `ACCESS_TOKEN` is a
[valid production authorization credential](https://developer.squareup.com/docs/build-basics/access-tokens).

If the access token is expired or not a valid access token, the endpoint returns an `UNAUTHORIZED` error.
- **`square-pp-cli oauth2 revoke-token`** - Revokes an access token generated with the OAuth flow.

If an account has more than one OAuth access token for your application, this
endpoint revokes all of them, regardless of which token you specify. 

__Important:__ The `Authorization` header for this endpoint must have the
following format:

```
Authorization: Client APPLICATION_SECRET
```

Replace `APPLICATION_SECRET` with the application secret on the **OAuth**
page for your application in the Developer Dashboard.

### online-checkout

Manage online checkout

- **`square-pp-cli online-checkout create-payment-link`** - Creates a Square-hosted checkout page. Applications can share the resulting payment link with their buyer to pay for goods and services.
- **`square-pp-cli online-checkout delete-payment-link`** - Deletes a payment link.
- **`square-pp-cli online-checkout list-payment-links`** - Lists all payment links.
- **`square-pp-cli online-checkout retrieve-location-settings`** - Retrieves the location-level settings for a Square-hosted checkout page.
- **`square-pp-cli online-checkout retrieve-merchant-settings`** - Retrieves the merchant-level settings for a Square-hosted checkout page.
- **`square-pp-cli online-checkout retrieve-payment-link`** - Retrieves a payment link.
- **`square-pp-cli online-checkout update-location-settings`** - Updates the location-level settings for a Square-hosted checkout page.
- **`square-pp-cli online-checkout update-merchant-settings`** - Updates the merchant-level settings for a Square-hosted checkout page.
- **`square-pp-cli online-checkout update-payment-link`** - Updates a payment link. You can update the `payment_link` fields such as
`description`, `checkout_options`, and  `pre_populated_data`.
You cannot update other fields such as the `order_id`, `version`, `URL`, or `timestamp` field.

### orders

Manage orders

- **`square-pp-cli orders batch-retrieve`** - Retrieves a set of [orders](entity:Order) by their IDs.

If a given order ID does not exist, the ID is ignored instead of generating an error.
- **`square-pp-cli orders bulk-delete-custom-attributes`** - Deletes order [custom attributes](entity:CustomAttribute) as a bulk operation.

Use this endpoint to delete one or more custom attributes from one or more orders.
A custom attribute is based on a custom attribute definition in a Square seller account.  (To create a
custom attribute definition, use the [CreateOrderCustomAttributeDefinition](api-endpoint:OrderCustomAttributes-CreateOrderCustomAttributeDefinition) endpoint.)

This `BulkDeleteOrderCustomAttributes` endpoint accepts a map of 1 to 25 individual delete
requests and returns a map of individual delete responses. Each delete request has a unique ID
and provides an order ID and custom attribute. Each delete response is returned with the ID
of the corresponding request.

To delete a custom attribute owned by another application, the `visibility` setting
must be `VISIBILITY_READ_WRITE_VALUES`. Note that seller-defined custom attributes
(also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli orders bulk-upsert-custom-attributes`** - Creates or updates order [custom attributes](entity:CustomAttribute) as a bulk operation.

Use this endpoint to delete one or more custom attributes from one or more orders.
A custom attribute is based on a custom attribute definition in a Square seller account.  (To create a
custom attribute definition, use the [CreateOrderCustomAttributeDefinition](api-endpoint:OrderCustomAttributes-CreateOrderCustomAttributeDefinition) endpoint.)

This `BulkUpsertOrderCustomAttributes` endpoint accepts a map of 1 to 25 individual upsert
requests and returns a map of individual upsert responses. Each upsert request has a unique ID
and provides an order ID and custom attribute. Each upsert response is returned with the ID
of the corresponding request.

To create or update a custom attribute owned by another application, the `visibility` setting
must be `VISIBILITY_READ_WRITE_VALUES`. Note that seller-defined custom attributes
(also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli orders calculate`** - Enables applications to preview order pricing without creating an order.
- **`square-pp-cli orders clone`** - Creates a new order, in the `DRAFT` state, by duplicating an existing order. The newly created order has
only the core fields (such as line items, taxes, and discounts) copied from the original order.
- **`square-pp-cli orders create`** - Creates a new [order](entity:Order) that can include information about products for
purchase and settings to apply to the purchase.

To pay for a created order, see
[Pay for Orders](https://developer.squareup.com/docs/orders-api/pay-for-orders).

You can modify open orders using the [UpdateOrder](api-endpoint:Orders-UpdateOrder) endpoint.
- **`square-pp-cli orders create-custom-attribute-definition`** - Creates an order-related custom attribute definition.  Use this endpoint to
define a custom attribute that can be associated with orders.

After creating a custom attribute definition, you can set the custom attribute for orders
in the Square seller account.
- **`square-pp-cli orders delete-custom-attribute-definition`** - Deletes an order-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.

Only the definition owner can delete a custom attribute definition.
- **`square-pp-cli orders list-custom-attribute-definitions`** - Lists the order-related [custom attribute definitions](entity:CustomAttributeDefinition) that belong to a Square seller account.

When all response pages are retrieved, the results include all custom attribute definitions
that are visible to the requesting application, including those that are created by other
applications and set to `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`. Note that
seller-defined custom attributes (also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli orders retrieve`** - Retrieves an [Order](entity:Order) by ID.
- **`square-pp-cli orders retrieve-custom-attribute-definition`** - Retrieves an order-related [custom attribute definition](entity:CustomAttributeDefinition) from a Square seller account.

To retrieve a custom attribute definition created by another application, the `visibility`
setting must be `VISIBILITY_READ_ONLY` or `VISIBILITY_READ_WRITE_VALUES`. Note that seller-defined custom attributes
(also known as custom fields) are always set to `VISIBILITY_READ_WRITE_VALUES`.
- **`square-pp-cli orders search`** - Search all orders for one or more locations. Orders include all sales,
returns, and exchanges regardless of how or when they entered the Square
ecosystem (such as Point of Sale, Invoices, and Connect APIs).

`SearchOrders` requests need to specify which locations to search and define a
[SearchOrdersQuery](entity:SearchOrdersQuery) object that controls
how to sort or filter the results. Your `SearchOrdersQuery` can:

  Set filter criteria.
  Set the sort order.
  Determine whether to return results as complete `Order` objects or as
[OrderEntry](entity:OrderEntry) objects.

Note that details for orders processed with Square Point of Sale while in
offline mode might not be transmitted to Square for up to 72 hours. Offline
orders have a `created_at` value that reflects the time the order was created,
not the time it was subsequently transmitted to Square.
- **`square-pp-cli orders update`** - Updates an open [order](entity:Order) by adding, replacing, or deleting
fields. Orders with a `COMPLETED` or `CANCELED` state cannot be updated.

An `UpdateOrder` request requires the following:

- The `order_id` in the endpoint path, identifying the order to update.
- The latest `version` of the order to update.
- The [sparse order](https://developer.squareup.com/docs/orders-api/manage-orders/update-orders#sparse-order-objects)
containing only the fields to update and the version to which the update is
being applied.
- If deleting fields, the [dot notation paths](https://developer.squareup.com/docs/orders-api/manage-orders/update-orders#identifying-fields-to-delete)
identifying the fields to clear.

To pay for an order, see
[Pay for Orders](https://developer.squareup.com/docs/orders-api/pay-for-orders).
- **`square-pp-cli orders update-custom-attribute-definition`** - Updates an order-related custom attribute definition for a Square seller account.

Only the definition owner can update a custom attribute definition. Note that sellers can view all custom attributes in exported customer data, including those set to `VISIBILITY_HIDDEN`.

### payments

Manage payments

- **`square-pp-cli payments cancel-by-idempotency-key`** - Cancels (voids) a payment identified by the idempotency key that is specified in the
request.

Use this method when the status of a `CreatePayment` request is unknown (for example, after you send a
`CreatePayment` request, a network error occurs and you do not get a response). In this case, you can
direct Square to cancel the payment using this endpoint. In the request, you provide the same
idempotency key that you provided in your `CreatePayment` request that you want to cancel. After
canceling the payment, you can submit your `CreatePayment` request again.

Note that if no payment with the specified idempotency key is found, no action is taken and the endpoint
returns successfully.
- **`square-pp-cli payments create`** - Creates a payment using the provided source. You can use this endpoint 
to charge a card (credit/debit card or    
Square gift card) or record a payment that the seller received outside of Square 
(cash payment from a buyer or a payment that an external entity 
processed on behalf of the seller).

The endpoint creates a 
`Payment` object and returns it in the response.
- **`square-pp-cli payments get`** - Retrieves details for a specific payment.
- **`square-pp-cli payments list`** - Retrieves a list of payments taken by the account making the request.

Results are eventually consistent, and new payments or changes to payments might take several
seconds to appear.

The maximum results per page is 100.
- **`square-pp-cli payments update`** - Updates a payment with the APPROVED status.
You can update the `amount_money` and `tip_money` using this endpoint.

### payouts

Manage payouts

- **`square-pp-cli payouts get`** - Retrieves details of a specific payout identified by a payout ID.
To call this endpoint, set `PAYOUTS_READ` for the OAuth scope.
- **`square-pp-cli payouts list`** - Retrieves a list of all payouts for the default location.
You can filter payouts by location ID, status, time range, and order them in ascending or descending order.
To call this endpoint, set `PAYOUTS_READ` for the OAuth scope.

### refunds

Manage refunds

- **`square-pp-cli refunds get-payment`** - Retrieves a specific refund using the `refund_id`.
- **`square-pp-cli refunds list-payment`** - Retrieves a list of refunds for the account making the request.

Results are eventually consistent, and new refunds or changes to refunds might take several
seconds to appear.

The maximum results per page is 100.
- **`square-pp-cli refunds payment`** - Refunds a payment. You can refund the entire payment amount or a
portion of it. You can use this endpoint to refund a card payment or record a 
refund of a cash or external payment. For more information, see
[Refund Payment](https://developer.squareup.com/docs/payments-api/refund-payments).

### reporting

Manage reporting

- **`square-pp-cli reporting get-metadata`** - Returns available reporting cubes, measures, and dimensions.
- **`square-pp-cli reporting load-query`** - Runs a reporting query. The API can return an error value of Continue wait while processing; clients should resend the identical request with bounded exponential backoff.

### sites

Manage sites

- **`square-pp-cli sites`** - Lists the Square Online sites that belong to a seller. Sites are listed in descending order by the `created_at` date.


__Note:__ Square Online APIs are publicly available as part of an early access program. For more information, see [Early access program for Square Online APIs](https://developer.squareup.com/docs/online-api#early-access-program-for-square-online-apis).

### subscriptions

Manage subscriptions

- **`square-pp-cli subscriptions bulk-swap-plan`** - Schedules a plan variation change for all active subscriptions under a given plan
variation. For more information, see [Swap Subscription Plan Variations](https://developer.squareup.com/docs/subscriptions-api/swap-plan-variations).
- **`square-pp-cli subscriptions create`** - Enrolls a customer in a subscription.

If you provide a card on file in the request, Square charges the card for
the subscription. Otherwise, Square sends an invoice to the customer's email
address. The subscription starts immediately, unless the request includes
the optional `start_date`. Each individual subscription is associated with a particular location.

For more information, see [Create a subscription](https://developer.squareup.com/docs/subscriptions-api/manage-subscriptions#create-a-subscription).
- **`square-pp-cli subscriptions retrieve`** - Retrieves a specific subscription.
- **`square-pp-cli subscriptions search`** - Searches for subscriptions.

Results are ordered chronologically by subscription creation date. If
the request specifies more than one location ID,
the endpoint orders the result
by location ID, and then by creation date within each location. If no locations are given
in the query, all locations are searched.

You can also optionally specify `customer_ids` to search by customer.
If left unset, all customers
associated with the specified locations are returned.
If the request specifies customer IDs, the endpoint orders results
first by location, within location by customer ID, and within
customer by subscription creation date.
- **`square-pp-cli subscriptions update`** - Updates a subscription by modifying or clearing `subscription` field values.
To clear a field, set its value to `null`.

### team-members

Manage team members

- **`square-pp-cli team-members bulk-create`** - Creates multiple `TeamMember` objects. The created `TeamMember` objects are returned on successful creates.
This process is non-transactional and processes as much of the request as possible. If one of the creates in
the request cannot be successfully processed, the request is not marked as failed, but the body of the response
contains explicit error information for the failed create.

Learn about [Troubleshooting the Team API](https://developer.squareup.com/docs/team/troubleshooting#bulk-create-team-members).
- **`square-pp-cli team-members bulk-update`** - Updates multiple `TeamMember` objects. The updated `TeamMember` objects are returned on successful updates.
This process is non-transactional and processes as much of the request as possible. If one of the updates in
the request cannot be successfully processed, the request is not marked as failed, but the body of the response
contains explicit error information for the failed update.
Learn about [Troubleshooting the Team API](https://developer.squareup.com/docs/team/troubleshooting#bulk-update-team-members).
- **`square-pp-cli team-members create`** - Creates a single `TeamMember` object. The `TeamMember` object is returned on successful creates.
You must provide the following values in your request to this endpoint:
- `given_name`
- `family_name`

Learn about [Troubleshooting the Team API](https://developer.squareup.com/docs/team/troubleshooting#createteammember).
- **`square-pp-cli team-members create-job`** - Creates a job in a seller account. A job defines a title and tip eligibility. Note that
compensation is defined in a [job assignment](entity:JobAssignment) in a team member's wage setting.
- **`square-pp-cli team-members list-jobs`** - Lists jobs in a seller account. Results are sorted by title in ascending order.
- **`square-pp-cli team-members retrieve`** - Retrieves a `TeamMember` object for the given `TeamMember.id`.
Learn about [Troubleshooting the Team API](https://developer.squareup.com/docs/team/troubleshooting#retrieve-a-team-member).
- **`square-pp-cli team-members retrieve-job`** - Retrieves a specified job.
- **`square-pp-cli team-members search`** - Returns a paginated list of `TeamMember` objects for a business. 
The list can be filtered by location IDs, `ACTIVE` or `INACTIVE` status, or whether
the team member is the Square account owner.
- **`square-pp-cli team-members update`** - Updates a single `TeamMember` object. The `TeamMember` object is returned on successful updates.
Learn about [Troubleshooting the Team API](https://developer.squareup.com/docs/team/troubleshooting#update-a-team-member).
- **`square-pp-cli team-members update-job`** - Updates the title or tip eligibility of a job. Changes to the title propagate to all
`JobAssignment`, `Shift`, and `TeamMemberWage` objects that reference the job ID. Changes to
tip eligibility propagate to all `TeamMemberWage` objects that reference the job ID.

### terminals

Manage terminals

- **`square-pp-cli terminals cancel-action`** - Cancels a Terminal action request if the status of the request permits it.
- **`square-pp-cli terminals cancel-checkout`** - Cancels a Terminal checkout request if the status of the request permits it.
- **`square-pp-cli terminals cancel-refund`** - Cancels an Interac Terminal refund request by refund request ID if the status of the request permits it.
- **`square-pp-cli terminals create-action`** - Creates a Terminal action request and sends it to the specified device.
- **`square-pp-cli terminals create-checkout`** - Creates a Terminal checkout request and sends it to the specified device to take a payment
for the requested amount.
- **`square-pp-cli terminals create-refund`** - Creates a request to refund an Interac payment completed on a Square Terminal. Refunds for Interac payments on a Square Terminal are supported only for Interac debit cards in Canada. Other refunds for Terminal payments should use the Refunds API. For more information, see [Refunds API](api:Refunds).
- **`square-pp-cli terminals dismiss-action`** - Dismisses a Terminal action request if the status and type of the request permits it.

See [Link and Dismiss Actions](https://developer.squareup.com/docs/terminal-api/advanced-features/custom-workflows/link-and-dismiss-actions) for more details.
- **`square-pp-cli terminals dismiss-checkout`** - Dismisses a Terminal checkout request if the status and type of the request permits it.
- **`square-pp-cli terminals dismiss-refund`** - Dismisses a Terminal refund request if the status and type of the request permits it.
- **`square-pp-cli terminals get-action`** - Retrieves a Terminal action request by `action_id`. Terminal action requests are available for 30 days.
- **`square-pp-cli terminals get-checkout`** - Retrieves a Terminal checkout request by `checkout_id`. Terminal checkout requests are available for 30 days.
- **`square-pp-cli terminals get-refund`** - Retrieves an Interac Terminal refund object by ID. Terminal refund objects are available for 30 days.
- **`square-pp-cli terminals search-actions`** - Retrieves a filtered list of Terminal action requests created by the account making the request. Terminal action requests are available for 30 days.
- **`square-pp-cli terminals search-checkouts`** - Returns a filtered list of Terminal checkout requests created by the application making the request. Only Terminal checkout requests created for the merchant scoped to the OAuth token are returned. Terminal checkout requests are available for 30 days.
- **`square-pp-cli terminals search-refunds`** - Retrieves a filtered list of Interac Terminal refund requests created by the seller making the request. Terminal refund requests are available for 30 days.

### transfer-orders

Manage transfer orders

- **`square-pp-cli transfer-orders create`** - Creates a new transfer order in [DRAFT](entity:TransferOrderStatus) status. A transfer order represents the intent 
to move [CatalogItemVariation](entity:CatalogItemVariation)s from one [Location](entity:Location) to another. 
The source and destination locations must be different and must belong to your Square account.

In [DRAFT](entity:TransferOrderStatus) status, you can:
- Add or remove items
- Modify quantities
- Update shipping information
- Delete the entire order via [DeleteTransferOrder](api-endpoint:TransferOrders-DeleteTransferOrder)

The request requires source_location_id and destination_location_id.
Inventory levels are not affected until the order is started via 
[StartTransferOrder](api-endpoint:TransferOrders-StartTransferOrder).

Common integration points:
- Sync with warehouse management systems
- Automate regular stock transfers
- Initialize transfers from inventory optimization systems

Creates a [transfer_order.created](webhook:transfer_order.created) webhook event.
- **`square-pp-cli transfer-orders delete`** - Deletes a transfer order in [DRAFT](entity:TransferOrderStatus) status.
Only draft orders can be deleted. Once an order is started via 
[StartTransferOrder](api-endpoint:TransferOrders-StartTransferOrder), it can no longer be deleted.

Creates a [transfer_order.deleted](webhook:transfer_order.deleted) webhook event.
- **`square-pp-cli transfer-orders retrieve`** - Retrieves a specific [TransferOrder](entity:TransferOrder) by ID. Returns the complete
order details including:

- Basic information (status, dates, notes)
- Line items with ordered and received quantities
- Source and destination [Location](entity:Location)s
- Tracking information (if available)
- **`square-pp-cli transfer-orders search`** - Searches for transfer orders using filters. Returns a paginated list of matching
[TransferOrder](entity:TransferOrder)s sorted by creation date.

Common search scenarios:
- Find orders for a source [Location](entity:Location)
- Find orders for a destination [Location](entity:Location)
- Find orders in a particular [TransferOrderStatus](entity:TransferOrderStatus)
- **`square-pp-cli transfer-orders update`** - Updates an existing transfer order. This endpoint supports sparse updates,
allowing you to modify specific fields without affecting others.

Creates a [transfer_order.updated](webhook:transfer_order.updated) webhook event.

### vendors

Manage vendors

- **`square-pp-cli vendors bulk-create`** - Creates one or more [Vendor](entity:Vendor) objects to represent suppliers to a seller.
- **`square-pp-cli vendors bulk-retrieve`** - Retrieves one or more vendors of specified [Vendor](entity:Vendor) IDs.
- **`square-pp-cli vendors bulk-update`** - Updates one or more of existing [Vendor](entity:Vendor) objects as suppliers to a seller.
- **`square-pp-cli vendors create`** - Creates a single [Vendor](entity:Vendor) object to represent a supplier to a seller.
- **`square-pp-cli vendors retrieve`** - Retrieves the vendor of a specified [Vendor](entity:Vendor) ID.
- **`square-pp-cli vendors search`** - Searches for vendors using a filter against supported [Vendor](entity:Vendor) properties and a supported sorter.
- **`square-pp-cli vendors update`** - Updates an existing [Vendor](entity:Vendor) object as a supplier to a seller.

### webhooks

Manage webhooks

- **`square-pp-cli webhooks create-subscription`** - Creates a webhook subscription.
- **`square-pp-cli webhooks delete-subscription`** - Deletes a webhook subscription.
- **`square-pp-cli webhooks list-event-types`** - Lists all webhook event types that can be subscribed to.
- **`square-pp-cli webhooks list-subscriptions`** - Lists all webhook subscriptions owned by your application.
- **`square-pp-cli webhooks retrieve-subscription`** - Retrieves a webhook subscription identified by its ID.
- **`square-pp-cli webhooks test-subscription`** - Tests a webhook subscription by sending a test event to the notification URL.
- **`square-pp-cli webhooks update-subscription`** - Updates a webhook subscription.
- **`square-pp-cli webhooks update-subscription-signature-key`** - Updates a webhook subscription by replacing the existing signature key with a new one.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`square-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`square-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`square-pp-cli learnings list`** - Inspect taught rows
- **`square-pp-cli learnings forget <query>`** - Undo a teach
- **`square-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`square-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`square-pp-cli teach-pattern`** - Install a query/resource template up front
- **`square-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SQUARE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `square-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
square-pp-cli bank-accounts list

# JSON for scripting and agents
square-pp-cli bank-accounts list --json

# Filter to specific fields
square-pp-cli bank-accounts list --json --select id,name,status

# Dry run — show the request without sending
square-pp-cli bank-accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
square-pp-cli bank-accounts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
square-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `square-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/square-pp-cli/config.toml`; `--home`, `SQUARE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SQUARE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `square-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `square-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SQUARE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Square returns UNAUTHORIZED or an authentication error.** — Confirm SQUARE_ACCESS_TOKEN is set for the selected environment; sandbox and production tokens cannot be mixed.
- **Square returns 429 RATE_LIMITED.** — Reduce --max-pages or batch size and let the built-in exponential backoff complete before retrying.
- **A local compound command reports missing or stale data.** — Run square-pp-cli sync --resources locations,catalog,inventory,customers,orders,payments,refunds,disputes,payouts,bookings,invoices,loyalty,team-members,events,webhooks-subscriptions --since 7d, then repeat the command.
- **A request works in one environment but fails in the other.** — Check the configured Square environment, token, location IDs, and Square-Version together; each environment has separate data.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Square Node.js SDK**](https://github.com/square/square-nodejs-sdk) — TypeScript (116 stars)
- [**Square Python SDK**](https://github.com/square/square-python-sdk) — Python (113 stars)
- [**Square MCP Server**](https://github.com/square/square-mcp-server) — TypeScript (104 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
