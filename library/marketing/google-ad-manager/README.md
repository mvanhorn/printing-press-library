# Google Ad Manager CLI

**The only REST-native Google Ad Manager CLI: report, search, and manage publisher inventory from the terminal, backed by a local mirror the web console can't match.**

Wraps the Ad Manager REST v1 API with a local SQLite mirror, offline full-text search across inventory, targeting, and orders, and one-shot async report orchestration where `report run` chains create, run, poll, and fetch into a single command. Built for publisher ad-ops who want to observe and manage a GAM360 network without living in the console. Covers reporting, inventory and targeting management (including the writes the REST API supports), and orders, line-item, and PMP visibility; trafficking, forecasting, and creative creation remain in the legacy SOAP API and are intentionally out of scope.

Learn more at [Google Ad Manager](https://google.com).

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `google-ad-manager-pp-cli` binary and the `pp-google-ad-manager` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager --agent claude-code
npx -y @mvanhorn/printing-press-library install google-ad-manager --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/cmd/google-ad-manager-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-ad-manager-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-ad-manager --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-ad-manager --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-ad-manager --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-ad-manager-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_AD_MANAGER_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/cmd/google-ad-manager-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-ad-manager": {
      "command": "google-ad-manager-pp-mcp",
      "env": {
        "GOOGLE_AD_MANAGER_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a Google OAuth2 access token for the admanager scope (or admanager.readonly for read-only use). Set GOOGLE_AD_MANAGER_ACCESS_TOKEN, for example `export GOOGLE_AD_MANAGER_ACCESS_TOKEN=$(gcloud auth print-access-token --scopes=https://www.googleapis.com/auth/admanager.readonly)`, or mint one from a service account reused from your SOAP setup. Every command is scoped to a network code: set GOOGLE_AD_MANAGER_NETWORK_CODE or pass --network <code>. Access tokens are short-lived (about an hour); re-export when you see a 401.

## Quick Start

```bash
# Verify your token, scope, network code, and API reachability before anything else.
google-ad-manager-pp-cli doctor

# Browse the ad-unit hierarchy; the first run fetches live and seeds the local mirror.
google-ad-manager-pp-cli adunits tree --network 123456 --json

# Grep across the seeded local mirror at once, no per-resource list calls.
google-ad-manager-pp-cli search "holiday" --json

# Pull yesterday's revenue by ad unit in one shot instead of the four-step UI wizard.
google-ad-manager-pp-cli report run --dimensions AD_UNIT_NAME,DATE --metrics IMPRESSIONS,REVENUE --date-range YESTERDAY --network 123456 --json

# Flag active ad units with no placement coverage.
google-ad-manager-pp-cli inventory orphans --network 123456 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Async report orchestration
- **`report run`** — Create, run, poll, and fetch every row of a GAM report in a single blocking command instead of the four-step UI slog.

  _Reach for this whenever an agent needs actual GAM revenue or delivery numbers; it turns the hardest API workflow into one deterministic step._

  ```bash
  google-ad-manager-pp-cli report run --dimensions AD_UNIT_NAME,DATE --metrics IMPRESSIONS,REVENUE --date-range YESTERDAY --network 123456 --json
  ```
- **`report rerun`** — Re-execute an existing saved report definition by ID and stream fresh rows, so the morning revenue pull is one short command.

  _Use when the user references a named or standing report they run repeatedly; it replays a known-good definition without rebuilding it._

  ```bash
  google-ad-manager-pp-cli report rerun 988877 --network 123456 --agent
  ```
- **`report watch`** — Re-run a saved report and diff it against the last cached run to surface what moved in revenue, impressions, or eCPM since yesterday.

  _Reach for this to answer what changed or dropped overnight without an analyst manually eyeballing two exports._

  ```bash
  google-ad-manager-pp-cli report watch 988877 --metric 0 --threshold 15 --network 123456 --json
  ```
- **`lineitem pace`** — Join read-only line-item goals against a delivery report to show each line item's pace versus goal, ranked by risk.

  _Reach for this to let an agent triage which campaigns are at delivery risk without opening the delivery UI._

  ```bash
  google-ad-manager-pp-cli lineitem pace --order 554433 --date-range MONTH_TO_DATE --network 123456 --agent
  ```

### Offline cross-entity intelligence
- **`order graph`** — Expand one order into its line items (goal, type, flight dates) in one structured object.

  _Reach for this to give an agent the complete picture of what a campaign actually touches in one structured object._

  ```bash
  google-ad-manager-pp-cli order graph 554433 --agent --select order.lineItems.id,order.lineItems.name --network 123456
  ```
- **`since`** — Show which mirrored entities were added or changed (by updateTime) since a cutoff — a changelog the platform does not keep.

  _Use to catch what trafficking or another team changed out from under you between two points in time._

  ```bash
  google-ad-manager-pp-cli since --since last-sync --network 123456 --json
  ```

### Inventory hygiene
- **`adunits tree`** — Render the full hierarchical ad-unit tree (or any subtree) from cache, with status and depth, in one offline call.

  _Reach for this to orient an agent in the inventory hierarchy before it reasons about placements or subtree revenue._

  ```bash
  google-ad-manager-pp-cli adunits tree --root 21700000 --status ACTIVE --network 123456
  ```
- **`inventory orphans`** — Tree-walk the ad-unit hierarchy to flag active units with no placement coverage.

  _Use during inventory cleanups to hand an agent a concrete list of misconfigured or dead units to investigate._

  ```bash
  google-ad-manager-pp-cli inventory orphans --root 21700000 --network 123456 --json
  ```

## Recipes


### Yesterday's revenue by ad unit

```bash
google-ad-manager-pp-cli report run --dimensions AD_UNIT_NAME,DATE --metrics IMPRESSIONS,REVENUE --date-range YESTERDAY --network 123456 --json
```

Builds, runs, polls, and fetches the report in one call and emits machine-readable rows.

### Expand an order for an agent, narrowed

```bash
google-ad-manager-pp-cli order graph 554433 --agent --select order.lineItems.id,order.lineItems.name --network 123456
```

Returns the order and its line items (goal, type, dates); --select narrows the nested response to the fields an agent needs.

### What changed since a cutoff

```bash
google-ad-manager-pp-cli since --since 7d --network 123456 --json
```

Surfaces entities added or changed (by updateTime) since the cutoff; refreshes live when --network is set. It cannot detect removals.

## Usage

Run `google-ad-manager-pp-cli --help` for the full command reference and flag list.

## Commands

### ad-breaks

Manage ad breaks

- **`google-ad-manager-pp-cli ad-breaks create`** - API to create an `AdBreak` object. Informs DAI of an upcoming ad break for a live stream event, with an optional expected start time. DAI will begin decisioning ads for the break shortly before the expected start time, if provided. Each live stream event can only have one incomplete ad break at any given time. The next ad break can be scheduled after the previous ad break has started serving, indicated by its state being `COMPLETE`, or it has been deleted. This method cannot be used if the `LiveStreamEvent` has [prefetching ad breaks enabled](https://developers.google.com/ad-manager/api/reference/latest/LiveStreamEventService.LiveStreamEvent#prefetchenabled) or the event is not active. If a `LiveStreamEvent` is deactivated after creating an ad break and before the ad break is complete, the ad break is discarded. An ad break's state is complete when the following occurs: - Full service DAI: after a matching ad break shows in the `LiveStreamEvent` manifest only when the ad break has started decisioning. - Pod Serving: after the ad break is requested using the ad break ID or break sequence.
- **`google-ad-manager-pp-cli ad-breaks list`** - API to retrieve a list of `AdBreak` objects. By default, when no `orderBy` query parameter is specified, ad breaks are ordered reverse chronologically. However, ad breaks with a 'breakState' of 'SCHEDULED' or 'DECISIONED' are prioritized and appear first.

### ad-review-center-ads-batch-allow

Manage ad review center ads batch allow

- **`google-ad-manager-pp-cli ad-review-center-ads-batch-allow <parent>`** - API to batch allow AdReviewCenterAds. This method supports partial success. Some operations may succeed while others fail. Callers should check the failedRequests field in the response to determine which operations failed.

### ad-review-center-ads-batch-block

Manage ad review center ads batch block

- **`google-ad-manager-pp-cli ad-review-center-ads-batch-block <parent>`** - API to batch block AdReviewCenterAds. This method supports partial success. Some operations may succeed while others fail. Callers should check the failedRequests field in the response to determine which operations failed.

### ad-review-center-ads-search

Manage ad review center ads search

- **`google-ad-manager-pp-cli ad-review-center-ads-search <parent>`** - API to search for AdReviewCenterAds.

### ad-spots

Manage ad spots

- **`google-ad-manager-pp-cli ad-spots create`** - API to create an `AdSpot` object.
- **`google-ad-manager-pp-cli ad-spots list`** - API to retrieve a list of `AdSpot` objects.

### ad-spots-batch-create

Manage ad spots batch create

- **`google-ad-manager-pp-cli ad-spots-batch-create <parent>`** - API to batch create `AdSpot` objects.

### ad-spots-batch-update

Manage ad spots batch update

- **`google-ad-manager-pp-cli ad-spots-batch-update <parent>`** - API to batch update `AdSpot` objects.

### ad-unit-sizes

Manage ad unit sizes

- **`google-ad-manager-pp-cli ad-unit-sizes <parent>`** - API to retrieve a list of AdUnitSize objects.

### ad-units

Manage ad units

- **`google-ad-manager-pp-cli ad-units create`** - API to create an `AdUnit` object.
- **`google-ad-manager-pp-cli ad-units list`** - API to retrieve a list of AdUnit objects.

### ad-units-batch-activate

Manage ad units batch activate

- **`google-ad-manager-pp-cli ad-units-batch-activate <parent>`** - API to batch activate `AdUnit` objects.

### ad-units-batch-archive

Manage ad units batch archive

- **`google-ad-manager-pp-cli ad-units-batch-archive <parent>`** - Archives a list of `AdUnit` objects.

### ad-units-batch-create

Manage ad units batch create

- **`google-ad-manager-pp-cli ad-units-batch-create <parent>`** - API to batch create `AdUnit` objects.

### ad-units-batch-deactivate

Manage ad units batch deactivate

- **`google-ad-manager-pp-cli ad-units-batch-deactivate <parent>`** - Deactivates a list of `AdUnit` objects.

### ad-units-batch-update

Manage ad units batch update

- **`google-ad-manager-pp-cli ad-units-batch-update <parent>`** - API to batch update `AdUnit` objects.

### applications

Manage applications

- **`google-ad-manager-pp-cli applications create`** - API to create a `Application` object.
- **`google-ad-manager-pp-cli applications list`** - API to retrieve a list of `Application` objects.

### applications-batch-archive

Manage applications batch archive

- **`google-ad-manager-pp-cli applications-batch-archive <parent>`** - / API to batch archive `Application` objects.

### applications-batch-create

Manage applications batch create

- **`google-ad-manager-pp-cli applications-batch-create <parent>`** - API to batch create `Application` objects.

### applications-batch-unarchive

Manage applications batch unarchive

- **`google-ad-manager-pp-cli applications-batch-unarchive <parent>`** - / API to batch unarchive `Application` objects.

### applications-batch-update

Manage applications batch update

- **`google-ad-manager-pp-cli applications-batch-update <parent>`** - API to batch update `Application` objects.

### audience-segments

Manage audience segments

- **`google-ad-manager-pp-cli audience-segments <parent>`** - API to retrieve a list of `AudienceSegment` objects.

### bandwidth-groups

Manage bandwidth groups

- **`google-ad-manager-pp-cli bandwidth-groups <parent>`** - API to retrieve a list of `BandwidthGroup` objects.

### browser-languages

Manage browser languages

- **`google-ad-manager-pp-cli browser-languages <parent>`** - API to retrieve a list of `BrowserLanguage` objects.

### browsers

Manage browsers

- **`google-ad-manager-pp-cli browsers <parent>`** - API to retrieve a list of `Browser` objects.

### cms-metadata-keys

Manage cms metadata keys

- **`google-ad-manager-pp-cli cms-metadata-keys <parent>`** - API to retrieve a list of `CmsMetadataKey` objects.

### cms-metadata-keys-batch-activate

Manage cms metadata keys batch activate

- **`google-ad-manager-pp-cli cms-metadata-keys-batch-activate <parent>`** - API to activate a list of `CmsMetadataKey` objects.

### cms-metadata-keys-batch-deactivate

Manage cms metadata keys batch deactivate

- **`google-ad-manager-pp-cli cms-metadata-keys-batch-deactivate <parent>`** - API to deactivate a list of `CmsMetadataKey` objects.

### cms-metadata-values

Manage cms metadata values

- **`google-ad-manager-pp-cli cms-metadata-values <parent>`** - API to retrieve a list of `CmsMetadataValue` objects.

### cms-metadata-values-batch-activate

Manage cms metadata values batch activate

- **`google-ad-manager-pp-cli cms-metadata-values-batch-activate <parent>`** - API to activate a list of `CmsMetadataValue` objects.

### cms-metadata-values-batch-deactivate

Manage cms metadata values batch deactivate

- **`google-ad-manager-pp-cli cms-metadata-values-batch-deactivate <parent>`** - API to deactivate a list of `CmsMetadataValue` objects.

### companies

Manage companies

- **`google-ad-manager-pp-cli companies <parent>`** - API to retrieve a list of `Company` objects.

### contacts

Manage contacts

- **`google-ad-manager-pp-cli contacts create`** - API to create a `Contact` object.
- **`google-ad-manager-pp-cli contacts list`** - API to retrieve a list of `Contact` objects.

### contacts-batch-create

Manage contacts batch create

- **`google-ad-manager-pp-cli contacts-batch-create <parent>`** - API to batch create `Contact` objects.

### contacts-batch-update

Manage contacts batch update

- **`google-ad-manager-pp-cli contacts-batch-update <parent>`** - API to batch update `Contact` objects.

### content

Manage content

- **`google-ad-manager-pp-cli content <parent>`** - API to retrieve a list of `Content` objects.

### content-bundles

Manage content bundles

- **`google-ad-manager-pp-cli content-bundles <parent>`** - API to retrieve a list of `ContentBundle` objects.

### content-labels

Manage content labels

- **`google-ad-manager-pp-cli content-labels <parent>`** - API to retrieve a list of `ContentLabel` objects.

### creative-templates

Manage creative templates

- **`google-ad-manager-pp-cli creative-templates <parent>`** - API to retrieve a list of `CreativeTemplate` objects.

### custom-fields

Manage custom fields

- **`google-ad-manager-pp-cli custom-fields create`** - API to create a `CustomField` object.
- **`google-ad-manager-pp-cli custom-fields list`** - API to retrieve a list of `CustomField` objects.

### custom-fields-batch-activate

Manage custom fields batch activate

- **`google-ad-manager-pp-cli custom-fields-batch-activate <parent>`** - Activates a list of `CustomField` objects.

### custom-fields-batch-create

Manage custom fields batch create

- **`google-ad-manager-pp-cli custom-fields-batch-create <parent>`** - API to batch create `CustomField` objects.

### custom-fields-batch-deactivate

Manage custom fields batch deactivate

- **`google-ad-manager-pp-cli custom-fields-batch-deactivate <parent>`** - Deactivates a list of `CustomField` objects.

### custom-fields-batch-update

Manage custom fields batch update

- **`google-ad-manager-pp-cli custom-fields-batch-update <parent>`** - API to batch update `CustomField` objects.

### custom-targeting-keys

Manage custom targeting keys

- **`google-ad-manager-pp-cli custom-targeting-keys create`** - API to create a `CustomTargetingKey` object.
- **`google-ad-manager-pp-cli custom-targeting-keys list`** - API to retrieve a list of `CustomTargetingKey` objects.

### custom-targeting-keys-batch-activate

Manage custom targeting keys batch activate

- **`google-ad-manager-pp-cli custom-targeting-keys-batch-activate <parent>`** - API to batch activate `CustomTargetingKey` objects.

### custom-targeting-keys-batch-create

Manage custom targeting keys batch create

- **`google-ad-manager-pp-cli custom-targeting-keys-batch-create <parent>`** - API to batch create `CustomTargetingKey` objects.

### custom-targeting-keys-batch-deactivate

Manage custom targeting keys batch deactivate

- **`google-ad-manager-pp-cli custom-targeting-keys-batch-deactivate <parent>`** - Deactivates a list of `CustomTargetingKey` objects.

### custom-targeting-keys-batch-update

Manage custom targeting keys batch update

- **`google-ad-manager-pp-cli custom-targeting-keys-batch-update <parent>`** - API to batch update `CustomTargetingKey` objects.

### custom-targeting-values

Manage custom targeting values

- **`google-ad-manager-pp-cli custom-targeting-values <parent>`** - API to retrieve a list of `CustomTargetingValue` objects.

### device-capabilities

Manage device capabilities

- **`google-ad-manager-pp-cli device-capabilities <parent>`** - API to retrieve a list of `DeviceCapability` objects.

### device-categories

Manage device categories

- **`google-ad-manager-pp-cli device-categories <parent>`** - API to retrieve a list of `DeviceCategory` objects.

### device-manufacturers

Manage device manufacturers

- **`google-ad-manager-pp-cli device-manufacturers <parent>`** - API to retrieve a list of `DeviceManufacturer` objects.

### entity-signals-mappings

Manage entity signals mappings

- **`google-ad-manager-pp-cli entity-signals-mappings create`** - API to create an `EntitySignalsMapping` object.
- **`google-ad-manager-pp-cli entity-signals-mappings list`** - API to retrieve a list of `EntitySignalsMapping` objects.

### entity-signals-mappings-batch-create

Manage entity signals mappings batch create

- **`google-ad-manager-pp-cli entity-signals-mappings-batch-create <parent>`** - API to batch create `EntitySignalsMapping` objects.

### entity-signals-mappings-batch-update

Manage entity signals mappings batch update

- **`google-ad-manager-pp-cli entity-signals-mappings-batch-update <parent>`** - API to batch update `EntitySignalsMapping` objects.

### geo-targets

Manage geo targets

- **`google-ad-manager-pp-cli geo-targets <parent>`** - API to retrieve a list of `GeoTarget` objects.

### labels

Manage labels

- **`google-ad-manager-pp-cli labels create`** - API to create a `Label` object.
- **`google-ad-manager-pp-cli labels list`** - API to retrieve a list of `Label` objects.

### labels-batch-activate

Manage labels batch activate

- **`google-ad-manager-pp-cli labels-batch-activate <parent>`** - API to activate `Label` objects.

### labels-batch-create

Manage labels batch create

- **`google-ad-manager-pp-cli labels-batch-create <parent>`** - API to batch create `Label` objects.

### labels-batch-deactivate

Manage labels batch deactivate

- **`google-ad-manager-pp-cli labels-batch-deactivate <parent>`** - API to deactivate `Label` objects.

### labels-batch-update

Manage labels batch update

- **`google-ad-manager-pp-cli labels-batch-update <parent>`** - API to batch update `Label` objects.

### line-items

Manage line items

- **`google-ad-manager-pp-cli line-items <parent>`** - API to retrieve a list of `LineItem` objects.

### linked-devices

Manage linked devices

- **`google-ad-manager-pp-cli linked-devices <parent>`** - Lists `LinkedDevice` objects.

### mcm-earnings-fetch

Manage mcm earnings fetch

- **`google-ad-manager-pp-cli mcm-earnings-fetch <parent>`** - API to retrieve a list of `McmEarnings` objects.

### mobile-carriers

Manage mobile carriers

- **`google-ad-manager-pp-cli mobile-carriers <parent>`** - API to retrieve a list of `MobileCarrier` objects.

### mobile-device-submodels

Manage mobile device submodels

- **`google-ad-manager-pp-cli mobile-device-submodels <parent>`** - API to retrieve a list of `MobileDeviceSubmodel` objects.

### mobile-devices

Manage mobile devices

- **`google-ad-manager-pp-cli mobile-devices <parent>`** - API to retrieve a list of `MobileDevice` objects.

### name-cancel

Manage name cancel

- **`google-ad-manager-pp-cli name-cancel <name>`** - Starts asynchronous cancellation on a long-running operation. The server makes a best effort to cancel the operation, but success is not guaranteed. If the server doesn't support this method, it returns `google.rpc.Code.UNIMPLEMENTED`. Clients can use Operations.GetOperation or other methods to check whether the cancellation succeeded or whether the operation completed despite cancellation. On successful cancellation, the operation is not deleted; instead, it becomes an operation with an Operation.error value with a google.rpc.Status.code of `1`, corresponding to `Code.CANCELLED`.

### name-fetch-rows

Manage name fetch rows

- **`google-ad-manager-pp-cli name-fetch-rows <name>`** - Returns the result rows from a completed report. The caller must have previously called `RunReport` and waited for that operation to complete. The rows will be returned according to the order specified by the `sorts` member of the report definition.

### name-run

Manage name run

- **`google-ad-manager-pp-cli name-run <name>`** - Initiates the execution of an existing report asynchronously. Users can get the report by polling this operation using `OperationsService.GetOperation`. Poll every 5 seconds initially, with an exponential backoff. Once a report is complete, the operation will contain a `RunReportResponse` in its response field containing a report_result that can be passed to the `FetchReportResultRows` method to retrieve the report data.

### networks

Manage networks

- **`google-ad-manager-pp-cli networks`** - API to retrieve all the networks the current user has access to.

### operating-system-versions

Manage operating system versions

- **`google-ad-manager-pp-cli operating-system-versions <parent>`** - API to retrieve a list of `OperatingSystemVersion` objects.

### operating-systems

Manage operating systems

- **`google-ad-manager-pp-cli operating-systems <parent>`** - API to retrieve a list of `OperatingSystem` objects.

### orders

Manage orders

- **`google-ad-manager-pp-cli orders <parent>`** - API to retrieve a list of `Order` objects. Fields used for literal matching in filter string: * `order_id` * `display_name` * `external_order_id`

### placements

Manage placements

- **`google-ad-manager-pp-cli placements create`** - API to create an `Placement` object.
- **`google-ad-manager-pp-cli placements list`** - API to retrieve a list of `Placement` objects.

### placements-batch-activate

Manage placements batch activate

- **`google-ad-manager-pp-cli placements-batch-activate <parent>`** - Activates a list of `Placement` objects.

### placements-batch-archive

Manage placements batch archive

- **`google-ad-manager-pp-cli placements-batch-archive <parent>`** - Archives a list of `Placement` objects.

### placements-batch-create

Manage placements batch create

- **`google-ad-manager-pp-cli placements-batch-create <parent>`** - API to batch create `Placement` objects.

### placements-batch-deactivate

Manage placements batch deactivate

- **`google-ad-manager-pp-cli placements-batch-deactivate <parent>`** - Deactivates a list of `Placement` objects.

### placements-batch-update

Manage placements batch update

- **`google-ad-manager-pp-cli placements-batch-update <parent>`** - API to batch update `Placement` objects.

### private-auction-deals

Manage private auction deals

- **`google-ad-manager-pp-cli private-auction-deals create`** - API to create a `PrivateAuctionDeal` object.
- **`google-ad-manager-pp-cli private-auction-deals list`** - API to retrieve a list of `PrivateAuctionDeal` objects.

### private-auctions

Manage private auctions

- **`google-ad-manager-pp-cli private-auctions create`** - API to create a `PrivateAuction` object.
- **`google-ad-manager-pp-cli private-auctions list`** - API to retrieve a list of `PrivateAuction` objects.

### programmatic-buyers

Manage programmatic buyers

- **`google-ad-manager-pp-cli programmatic-buyers <parent>`** - API to retrieve a list of `ProgrammaticBuyer` objects.

### reports

Manage reports

- **`google-ad-manager-pp-cli reports create`** - API to create a `Report` object.
- **`google-ad-manager-pp-cli reports list`** - API to retrieve a list of `Report` objects.

### rich-media-ads-companies

Manage rich media ads companies

- **`google-ad-manager-pp-cli rich-media-ads-companies <parent>`** - API to retrieve a list of `RichMediaAdsCompany` objects.

### roles

Manage roles

- **`google-ad-manager-pp-cli roles <parent>`** - API to retrieve a list of `Role` objects.

### sites

Manage sites

- **`google-ad-manager-pp-cli sites create`** - API to create a `Site` object.
- **`google-ad-manager-pp-cli sites list`** - API to retrieve a list of `Site` objects.

### sites-batch-create

Manage sites batch create

- **`google-ad-manager-pp-cli sites-batch-create <parent>`** - API to batch create `Site` objects.

### sites-batch-deactivate

Manage sites batch deactivate

- **`google-ad-manager-pp-cli sites-batch-deactivate <parent>`** - Deactivates a list of `Site` objects.

### sites-batch-submit-for-approval

Manage sites batch submit for approval

- **`google-ad-manager-pp-cli sites-batch-submit-for-approval <parent>`** - Submits a list of `Site` objects for approval.

### sites-batch-update

Manage sites batch update

- **`google-ad-manager-pp-cli sites-batch-update <parent>`** - API to batch update `Site` objects.

### taxonomy-categories

Manage taxonomy categories

- **`google-ad-manager-pp-cli taxonomy-categories <parent>`** - API to retrieve a list of `TaxonomyCategory` objects.

### teams

Manage teams

- **`google-ad-manager-pp-cli teams create`** - API to create a `Team` object.
- **`google-ad-manager-pp-cli teams list`** - API to retrieve a list of `Team` objects.

### teams-batch-activate

Manage teams batch activate

- **`google-ad-manager-pp-cli teams-batch-activate <parent>`** - API to batch activate `Team` objects.

### teams-batch-create

Manage teams batch create

- **`google-ad-manager-pp-cli teams-batch-create <parent>`** - API to batch create `Team` objects.

### teams-batch-deactivate

Manage teams batch deactivate

- **`google-ad-manager-pp-cli teams-batch-deactivate <parent>`** - API to batch deactivate `Team` objects.

### teams-batch-update

Manage teams batch update

- **`google-ad-manager-pp-cli teams-batch-update <parent>`** - API to batch update `Team` objects.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-ad-manager-pp-cli ad-breaks list mock-value

# JSON for scripting and agents
google-ad-manager-pp-cli ad-breaks list mock-value --json

# Filter to specific fields
google-ad-manager-pp-cli ad-breaks list mock-value --json --select id,name,status

# Dry run — show the request without sending
google-ad-manager-pp-cli ad-breaks list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-ad-manager-pp-cli ad-breaks list mock-value --agent
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

## Health Check

```bash
google-ad-manager-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-ad-manager-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_AD_MANAGER_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `google-ad-manager-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-ad-manager-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_AD_MANAGER_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Your access token expired (they last about an hour). Re-export GOOGLE_AD_MANAGER_ACCESS_TOKEN with a fresh `gcloud auth print-access-token`.
- **403 PERMISSION_DENIED or wrong network** — Confirm the token's Google account has access to that network and the API is enabled in its Cloud project; check the code with `google-ad-manager-pp-cli networks list`.
- **Command needs a network code** — Set GOOGLE_AD_MANAGER_NETWORK_CODE or pass --network <code>; every resource path is scoped to a network.
- **Report run seems to hang** — Reports are async; run keeps polling the operation until rows are ready. Add --timeout to bound the wait or run without --wait to get the operation name and fetch later.
- **search returns nothing** — Run `sync` first; search reads the local mirror, not the live API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**googleads-python-lib**](https://github.com/googleads/googleads-python-lib) — Python (742 stars)
- [**dfp-api**](https://github.com/publica-project/dfp-api) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
