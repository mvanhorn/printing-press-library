---
name: pp-google-ad-manager
description: "The only REST-native Google Ad Manager CLI: report, search, and manage publisher inventory from the terminal Trigger phrases: `run a GAM report`, `google ad manager revenue by ad unit`, `check line item delivery in ad manager`, `search my ad manager inventory`, `what targeting does this order use`, `use google-ad-manager`, `run google ad manager`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-ad-manager-pp-cli
    install:
      - kind: go
        bins: [google-ad-manager-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/cmd/google-ad-manager-pp-cli
---

# Google Ad Manager — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-ad-manager-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-ad-manager --cli-only
   ```
2. Verify: `google-ad-manager-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/cmd/google-ad-manager-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the Ad Manager REST v1 API with a local SQLite mirror, offline full-text search across inventory, targeting, and orders, and one-shot async report orchestration where `report run` chains create, run, poll, and fetch into a single command. Built for publisher ad-ops who want to observe and manage a GAM360 network without living in the console. Covers reporting, inventory and targeting management (including the writes the REST API supports), and orders, line-item, and PMP visibility; trafficking, forecasting, and creative creation remain in the legacy SOAP API and are intentionally out of scope.

## When to Use This CLI

Use for observing and reporting on a Google Ad Manager (publisher) network: pulling revenue and delivery reports, browsing and searching inventory and custom targeting, checking order and line-item status, and inspecting PMP deals, plus the inventory and targeting writes the REST API supports. Ideal when an agent needs real GAM numbers or inventory context without navigating the web console, or when a question spans multiple entity types the UI keeps on separate screens.

## Anti-triggers

Do not use this CLI for:
- Google Ads / AdWords advertiser campaigns: that is a different product (the advertiser side) with a different API; use a Google Ads tool instead.
- Booking or editing line items, orders, or creatives, or creating creative associations: those writes are SOAP-only and not supported here.
- Availability or delivery forecasting: the forecast service is SOAP-only and out of scope.
- Header-bidding or real-time-bidding configuration: not part of the Ad Manager REST API.

## Unique Capabilities

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

## Command Reference

**ad-breaks** — Manage ad breaks

- `google-ad-manager-pp-cli ad-breaks create` — API to create an `AdBreak` object.
- `google-ad-manager-pp-cli ad-breaks list` — API to retrieve a list of `AdBreak` objects.

**ad-review-center-ads-batch-allow** — Manage ad review center ads batch allow

- `google-ad-manager-pp-cli ad-review-center-ads-batch-allow <parent>` — API to batch allow AdReviewCenterAds. This method supports partial success.

**ad-review-center-ads-batch-block** — Manage ad review center ads batch block

- `google-ad-manager-pp-cli ad-review-center-ads-batch-block <parent>` — API to batch block AdReviewCenterAds. This method supports partial success.

**ad-review-center-ads-search** — Manage ad review center ads search

- `google-ad-manager-pp-cli ad-review-center-ads-search <parent>` — API to search for AdReviewCenterAds.

**ad-spots** — Manage ad spots

- `google-ad-manager-pp-cli ad-spots create` — API to create an `AdSpot` object.
- `google-ad-manager-pp-cli ad-spots list` — API to retrieve a list of `AdSpot` objects.

**ad-spots-batch-create** — Manage ad spots batch create

- `google-ad-manager-pp-cli ad-spots-batch-create <parent>` — API to batch create `AdSpot` objects.

**ad-spots-batch-update** — Manage ad spots batch update

- `google-ad-manager-pp-cli ad-spots-batch-update <parent>` — API to batch update `AdSpot` objects.

**ad-unit-sizes** — Manage ad unit sizes

- `google-ad-manager-pp-cli ad-unit-sizes <parent>` — API to retrieve a list of AdUnitSize objects.

**ad-units** — Manage ad units

- `google-ad-manager-pp-cli ad-units create` — API to create an `AdUnit` object.
- `google-ad-manager-pp-cli ad-units list` — API to retrieve a list of AdUnit objects.

**ad-units-batch-activate** — Manage ad units batch activate

- `google-ad-manager-pp-cli ad-units-batch-activate <parent>` — API to batch activate `AdUnit` objects.

**ad-units-batch-archive** — Manage ad units batch archive

- `google-ad-manager-pp-cli ad-units-batch-archive <parent>` — Archives a list of `AdUnit` objects.

**ad-units-batch-create** — Manage ad units batch create

- `google-ad-manager-pp-cli ad-units-batch-create <parent>` — API to batch create `AdUnit` objects.

**ad-units-batch-deactivate** — Manage ad units batch deactivate

- `google-ad-manager-pp-cli ad-units-batch-deactivate <parent>` — Deactivates a list of `AdUnit` objects.

**ad-units-batch-update** — Manage ad units batch update

- `google-ad-manager-pp-cli ad-units-batch-update <parent>` — API to batch update `AdUnit` objects.

**applications** — Manage applications

- `google-ad-manager-pp-cli applications create` — API to create a `Application` object.
- `google-ad-manager-pp-cli applications list` — API to retrieve a list of `Application` objects.

**applications-batch-archive** — Manage applications batch archive

- `google-ad-manager-pp-cli applications-batch-archive <parent>` — / API to batch archive `Application` objects.

**applications-batch-create** — Manage applications batch create

- `google-ad-manager-pp-cli applications-batch-create <parent>` — API to batch create `Application` objects.

**applications-batch-unarchive** — Manage applications batch unarchive

- `google-ad-manager-pp-cli applications-batch-unarchive <parent>` — / API to batch unarchive `Application` objects.

**applications-batch-update** — Manage applications batch update

- `google-ad-manager-pp-cli applications-batch-update <parent>` — API to batch update `Application` objects.

**audience-segments** — Manage audience segments

- `google-ad-manager-pp-cli audience-segments <parent>` — API to retrieve a list of `AudienceSegment` objects.

**bandwidth-groups** — Manage bandwidth groups

- `google-ad-manager-pp-cli bandwidth-groups <parent>` — API to retrieve a list of `BandwidthGroup` objects.

**browser-languages** — Manage browser languages

- `google-ad-manager-pp-cli browser-languages <parent>` — API to retrieve a list of `BrowserLanguage` objects.

**browsers** — Manage browsers

- `google-ad-manager-pp-cli browsers <parent>` — API to retrieve a list of `Browser` objects.

**cms-metadata-keys** — Manage cms metadata keys

- `google-ad-manager-pp-cli cms-metadata-keys <parent>` — API to retrieve a list of `CmsMetadataKey` objects.

**cms-metadata-keys-batch-activate** — Manage cms metadata keys batch activate

- `google-ad-manager-pp-cli cms-metadata-keys-batch-activate <parent>` — API to activate a list of `CmsMetadataKey` objects.

**cms-metadata-keys-batch-deactivate** — Manage cms metadata keys batch deactivate

- `google-ad-manager-pp-cli cms-metadata-keys-batch-deactivate <parent>` — API to deactivate a list of `CmsMetadataKey` objects.

**cms-metadata-values** — Manage cms metadata values

- `google-ad-manager-pp-cli cms-metadata-values <parent>` — API to retrieve a list of `CmsMetadataValue` objects.

**cms-metadata-values-batch-activate** — Manage cms metadata values batch activate

- `google-ad-manager-pp-cli cms-metadata-values-batch-activate <parent>` — API to activate a list of `CmsMetadataValue` objects.

**cms-metadata-values-batch-deactivate** — Manage cms metadata values batch deactivate

- `google-ad-manager-pp-cli cms-metadata-values-batch-deactivate <parent>` — API to deactivate a list of `CmsMetadataValue` objects.

**companies** — Manage companies

- `google-ad-manager-pp-cli companies <parent>` — API to retrieve a list of `Company` objects.

**contacts** — Manage contacts

- `google-ad-manager-pp-cli contacts create` — API to create a `Contact` object.
- `google-ad-manager-pp-cli contacts list` — API to retrieve a list of `Contact` objects.

**contacts-batch-create** — Manage contacts batch create

- `google-ad-manager-pp-cli contacts-batch-create <parent>` — API to batch create `Contact` objects.

**contacts-batch-update** — Manage contacts batch update

- `google-ad-manager-pp-cli contacts-batch-update <parent>` — API to batch update `Contact` objects.

**content** — Manage content

- `google-ad-manager-pp-cli content <parent>` — API to retrieve a list of `Content` objects.

**content-bundles** — Manage content bundles

- `google-ad-manager-pp-cli content-bundles <parent>` — API to retrieve a list of `ContentBundle` objects.

**content-labels** — Manage content labels

- `google-ad-manager-pp-cli content-labels <parent>` — API to retrieve a list of `ContentLabel` objects.

**creative-templates** — Manage creative templates

- `google-ad-manager-pp-cli creative-templates <parent>` — API to retrieve a list of `CreativeTemplate` objects.

**custom-fields** — Manage custom fields

- `google-ad-manager-pp-cli custom-fields create` — API to create a `CustomField` object.
- `google-ad-manager-pp-cli custom-fields list` — API to retrieve a list of `CustomField` objects.

**custom-fields-batch-activate** — Manage custom fields batch activate

- `google-ad-manager-pp-cli custom-fields-batch-activate <parent>` — Activates a list of `CustomField` objects.

**custom-fields-batch-create** — Manage custom fields batch create

- `google-ad-manager-pp-cli custom-fields-batch-create <parent>` — API to batch create `CustomField` objects.

**custom-fields-batch-deactivate** — Manage custom fields batch deactivate

- `google-ad-manager-pp-cli custom-fields-batch-deactivate <parent>` — Deactivates a list of `CustomField` objects.

**custom-fields-batch-update** — Manage custom fields batch update

- `google-ad-manager-pp-cli custom-fields-batch-update <parent>` — API to batch update `CustomField` objects.

**custom-targeting-keys** — Manage custom targeting keys

- `google-ad-manager-pp-cli custom-targeting-keys create` — API to create a `CustomTargetingKey` object.
- `google-ad-manager-pp-cli custom-targeting-keys list` — API to retrieve a list of `CustomTargetingKey` objects.

**custom-targeting-keys-batch-activate** — Manage custom targeting keys batch activate

- `google-ad-manager-pp-cli custom-targeting-keys-batch-activate <parent>` — API to batch activate `CustomTargetingKey` objects.

**custom-targeting-keys-batch-create** — Manage custom targeting keys batch create

- `google-ad-manager-pp-cli custom-targeting-keys-batch-create <parent>` — API to batch create `CustomTargetingKey` objects.

**custom-targeting-keys-batch-deactivate** — Manage custom targeting keys batch deactivate

- `google-ad-manager-pp-cli custom-targeting-keys-batch-deactivate <parent>` — Deactivates a list of `CustomTargetingKey` objects.

**custom-targeting-keys-batch-update** — Manage custom targeting keys batch update

- `google-ad-manager-pp-cli custom-targeting-keys-batch-update <parent>` — API to batch update `CustomTargetingKey` objects.

**custom-targeting-values** — Manage custom targeting values

- `google-ad-manager-pp-cli custom-targeting-values <parent>` — API to retrieve a list of `CustomTargetingValue` objects.

**device-capabilities** — Manage device capabilities

- `google-ad-manager-pp-cli device-capabilities <parent>` — API to retrieve a list of `DeviceCapability` objects.

**device-categories** — Manage device categories

- `google-ad-manager-pp-cli device-categories <parent>` — API to retrieve a list of `DeviceCategory` objects.

**device-manufacturers** — Manage device manufacturers

- `google-ad-manager-pp-cli device-manufacturers <parent>` — API to retrieve a list of `DeviceManufacturer` objects.

**entity-signals-mappings** — Manage entity signals mappings

- `google-ad-manager-pp-cli entity-signals-mappings create` — API to create an `EntitySignalsMapping` object.
- `google-ad-manager-pp-cli entity-signals-mappings list` — API to retrieve a list of `EntitySignalsMapping` objects.

**entity-signals-mappings-batch-create** — Manage entity signals mappings batch create

- `google-ad-manager-pp-cli entity-signals-mappings-batch-create <parent>` — API to batch create `EntitySignalsMapping` objects.

**entity-signals-mappings-batch-update** — Manage entity signals mappings batch update

- `google-ad-manager-pp-cli entity-signals-mappings-batch-update <parent>` — API to batch update `EntitySignalsMapping` objects.

**geo-targets** — Manage geo targets

- `google-ad-manager-pp-cli geo-targets <parent>` — API to retrieve a list of `GeoTarget` objects.

**labels** — Manage labels

- `google-ad-manager-pp-cli labels create` — API to create a `Label` object.
- `google-ad-manager-pp-cli labels list` — API to retrieve a list of `Label` objects.

**labels-batch-activate** — Manage labels batch activate

- `google-ad-manager-pp-cli labels-batch-activate <parent>` — API to activate `Label` objects.

**labels-batch-create** — Manage labels batch create

- `google-ad-manager-pp-cli labels-batch-create <parent>` — API to batch create `Label` objects.

**labels-batch-deactivate** — Manage labels batch deactivate

- `google-ad-manager-pp-cli labels-batch-deactivate <parent>` — API to deactivate `Label` objects.

**labels-batch-update** — Manage labels batch update

- `google-ad-manager-pp-cli labels-batch-update <parent>` — API to batch update `Label` objects.

**line-items** — Manage line items

- `google-ad-manager-pp-cli line-items <parent>` — API to retrieve a list of `LineItem` objects.

**linked-devices** — Manage linked devices

- `google-ad-manager-pp-cli linked-devices <parent>` — Lists `LinkedDevice` objects.

**mcm-earnings-fetch** — Manage mcm earnings fetch

- `google-ad-manager-pp-cli mcm-earnings-fetch <parent>` — API to retrieve a list of `McmEarnings` objects.

**mobile-carriers** — Manage mobile carriers

- `google-ad-manager-pp-cli mobile-carriers <parent>` — API to retrieve a list of `MobileCarrier` objects.

**mobile-device-submodels** — Manage mobile device submodels

- `google-ad-manager-pp-cli mobile-device-submodels <parent>` — API to retrieve a list of `MobileDeviceSubmodel` objects.

**mobile-devices** — Manage mobile devices

- `google-ad-manager-pp-cli mobile-devices <parent>` — API to retrieve a list of `MobileDevice` objects.

**name-cancel** — Manage name cancel

- `google-ad-manager-pp-cli name-cancel <name>` — Starts asynchronous cancellation on a long-running operation.

**name-fetch-rows** — Manage name fetch rows

- `google-ad-manager-pp-cli name-fetch-rows <name>` — Returns the result rows from a completed report.

**name-run** — Manage name run

- `google-ad-manager-pp-cli name-run <name>` — Initiates the execution of an existing report asynchronously.

**networks** — Manage networks

- `google-ad-manager-pp-cli networks` — API to retrieve all the networks the current user has access to.

**operating-system-versions** — Manage operating system versions

- `google-ad-manager-pp-cli operating-system-versions <parent>` — API to retrieve a list of `OperatingSystemVersion` objects.

**operating-systems** — Manage operating systems

- `google-ad-manager-pp-cli operating-systems <parent>` — API to retrieve a list of `OperatingSystem` objects.

**orders** — Manage orders

- `google-ad-manager-pp-cli orders <parent>` — API to retrieve a list of `Order` objects.

**placements** — Manage placements

- `google-ad-manager-pp-cli placements create` — API to create an `Placement` object.
- `google-ad-manager-pp-cli placements list` — API to retrieve a list of `Placement` objects.

**placements-batch-activate** — Manage placements batch activate

- `google-ad-manager-pp-cli placements-batch-activate <parent>` — Activates a list of `Placement` objects.

**placements-batch-archive** — Manage placements batch archive

- `google-ad-manager-pp-cli placements-batch-archive <parent>` — Archives a list of `Placement` objects.

**placements-batch-create** — Manage placements batch create

- `google-ad-manager-pp-cli placements-batch-create <parent>` — API to batch create `Placement` objects.

**placements-batch-deactivate** — Manage placements batch deactivate

- `google-ad-manager-pp-cli placements-batch-deactivate <parent>` — Deactivates a list of `Placement` objects.

**placements-batch-update** — Manage placements batch update

- `google-ad-manager-pp-cli placements-batch-update <parent>` — API to batch update `Placement` objects.

**private-auction-deals** — Manage private auction deals

- `google-ad-manager-pp-cli private-auction-deals create` — API to create a `PrivateAuctionDeal` object.
- `google-ad-manager-pp-cli private-auction-deals list` — API to retrieve a list of `PrivateAuctionDeal` objects.

**private-auctions** — Manage private auctions

- `google-ad-manager-pp-cli private-auctions create` — API to create a `PrivateAuction` object.
- `google-ad-manager-pp-cli private-auctions list` — API to retrieve a list of `PrivateAuction` objects.

**programmatic-buyers** — Manage programmatic buyers

- `google-ad-manager-pp-cli programmatic-buyers <parent>` — API to retrieve a list of `ProgrammaticBuyer` objects.

**reports** — Manage reports

- `google-ad-manager-pp-cli reports create` — API to create a `Report` object.
- `google-ad-manager-pp-cli reports list` — API to retrieve a list of `Report` objects.

**rich-media-ads-companies** — Manage rich media ads companies

- `google-ad-manager-pp-cli rich-media-ads-companies <parent>` — API to retrieve a list of `RichMediaAdsCompany` objects.

**roles** — Manage roles

- `google-ad-manager-pp-cli roles <parent>` — API to retrieve a list of `Role` objects.

**sites** — Manage sites

- `google-ad-manager-pp-cli sites create` — API to create a `Site` object.
- `google-ad-manager-pp-cli sites list` — API to retrieve a list of `Site` objects.

**sites-batch-create** — Manage sites batch create

- `google-ad-manager-pp-cli sites-batch-create <parent>` — API to batch create `Site` objects.

**sites-batch-deactivate** — Manage sites batch deactivate

- `google-ad-manager-pp-cli sites-batch-deactivate <parent>` — Deactivates a list of `Site` objects.

**sites-batch-submit-for-approval** — Manage sites batch submit for approval

- `google-ad-manager-pp-cli sites-batch-submit-for-approval <parent>` — Submits a list of `Site` objects for approval.

**sites-batch-update** — Manage sites batch update

- `google-ad-manager-pp-cli sites-batch-update <parent>` — API to batch update `Site` objects.

**taxonomy-categories** — Manage taxonomy categories

- `google-ad-manager-pp-cli taxonomy-categories <parent>` — API to retrieve a list of `TaxonomyCategory` objects.

**teams** — Manage teams

- `google-ad-manager-pp-cli teams create` — API to create a `Team` object.
- `google-ad-manager-pp-cli teams list` — API to retrieve a list of `Team` objects.

**teams-batch-activate** — Manage teams batch activate

- `google-ad-manager-pp-cli teams-batch-activate <parent>` — API to batch activate `Team` objects.

**teams-batch-create** — Manage teams batch create

- `google-ad-manager-pp-cli teams-batch-create <parent>` — API to batch create `Team` objects.

**teams-batch-deactivate** — Manage teams batch deactivate

- `google-ad-manager-pp-cli teams-batch-deactivate <parent>` — API to batch deactivate `Team` objects.

**teams-batch-update** — Manage teams batch update

- `google-ad-manager-pp-cli teams-batch-update <parent>` — API to batch update `Team` objects.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-ad-manager-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

### What changed since the last sync

```bash
google-ad-manager-pp-cli since --since last-sync --network 123456 --json
```

Surfaces entities added or changed (by updateTime) since the cutoff; refreshes live when --network is set. It cannot detect removals.

## Auth Setup

Authenticate with a Google OAuth2 access token for the admanager scope (or admanager.readonly for read-only use). Set GOOGLE_AD_MANAGER_ACCESS_TOKEN, for example `export GOOGLE_AD_MANAGER_ACCESS_TOKEN=$(gcloud auth print-access-token --scopes=https://www.googleapis.com/auth/admanager.readonly)`, or mint one from a service account reused from your SOAP setup. Every command is scoped to a network code: set GOOGLE_AD_MANAGER_NETWORK_CODE or pass --network <code>. Access tokens are short-lived (about an hour); re-export when you see a 401.

Run `google-ad-manager-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-ad-manager-pp-cli ad-breaks list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
google-ad-manager-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-ad-manager-pp-cli feedback --stdin < notes.txt
google-ad-manager-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/google-ad-manager-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_AD_MANAGER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_AD_MANAGER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-ad-manager-pp-cli profile save briefing --json
google-ad-manager-pp-cli --profile briefing ad-breaks list mock-value
google-ad-manager-pp-cli profile list --json
google-ad-manager-pp-cli profile show briefing
google-ad-manager-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-ad-manager-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/cmd/google-ad-manager-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-ad-manager-pp-mcp -- google-ad-manager-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-ad-manager-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-ad-manager-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-ad-manager-pp-cli <command> --help`.
