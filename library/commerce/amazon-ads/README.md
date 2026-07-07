# Amazon Ads CLI

Amazon Ads CLI for OAuth profile setup, report normalization, profitability analytics, keyword optimization, and guarded campaign automation.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `amazon-ads-pp-cli` binary and the `pp-amazon-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install amazon-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/cmd/amazon-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-ads --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-ads --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install amazon-ads --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth2 refresh-token credentials. Configure the CLI first if you have not already:

```bash
mkdir -p ~/.config/amazon-ads-pp-cli
chmod 700 ~/.config/amazon-ads-pp-cli
$EDITOR ~/.config/amazon-ads-pp-cli/.env
```

Set `AMAZON_ADS_CLIENT_ID` and `AMAZON_ADS_CLIENT_SECRET` in that file, then run `amazon-ads-pp-cli auth login --port 8085` with `http://localhost:8085/callback` registered as the Amazon redirect URL. The login flow saves `AMAZON_ADS_REFRESH_TOKEN` and `AMAZON_ADS_PROFILE_ID`. Process environment variables with the same names still override `.env` values.

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. If Claude Desktop prompts for credentials, fill in the same `AMAZON_ADS_CLIENT_ID`, `AMAZON_ADS_CLIENT_SECRET`, `AMAZON_ADS_REFRESH_TOKEN`, and `AMAZON_ADS_PROFILE_ID` values from your local `.env`. Leave `AMAZON_ADS_API_CLIENT_ID` blank unless it differs from `AMAZON_ADS_CLIENT_ID`.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/cmd/amazon-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "amazon-ads": {
      "command": "amazon-ads-pp-mcp",
      "env": {
        "AMAZON_ADS_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up OAuth2 Refresh Credentials

This CLI uses Login with Amazon OAuth2 with refresh-token rotation. Access tokens are refreshed automatically.

For local OAuth login, register this exact Allowed Return URL / Redirect URI in your Amazon developer app:

```text
http://localhost:8085/callback
```

Put your client credentials in `~/.config/amazon-ads-pp-cli/.env`:

```bash
mkdir -p ~/.config/amazon-ads-pp-cli
chmod 700 ~/.config/amazon-ads-pp-cli
cat > ~/.config/amazon-ads-pp-cli/.env <<'EOF'
AMAZON_ADS_CLIENT_ID=your-client-id
AMAZON_ADS_CLIENT_SECRET=your-client-secret
AMAZON_ADS_REFRESH_TOKEN=
AMAZON_ADS_PROFILE_ID=
EOF
chmod 600 ~/.config/amazon-ads-pp-cli/.env
```

Then run `amazon-ads-pp-cli auth login --port 8085` to authorize `advertising::campaign_management` and save the refresh token. The login flow also fetches advertising profiles and selects the only profile automatically, or prompts you when multiple profiles are available. If you already have a refresh token, paste it into `.env` instead.

### 3. Verify Setup

```bash
amazon-ads-pp-cli doctor
```

This checks your configuration and credentials.

List or change the selected advertising profile:

```bash
amazon-ads-pp-cli profiles list --json
amazon-ads-pp-cli profiles select <profile-id>
amazon-ads-pp-cli profiles current --json
```

### 4. Try Your First Command

```bash
amazon-ads-pp-cli amazon-ads-dsp-dsp list
```

## Usage

Run `amazon-ads-pp-cli --help` for the full command reference and flag list.

### Profitability and Optimization Commands

These novel commands operate on exported Amazon Ads reports and optional COGS data at `~/.config/amazon-ads-pp-cli/cogs.toml`.
`dayparting-analysis` and `budget-pacing` require a report export with `hour`, `hourOfDay`, or timestamped rows; the pinned DSP reporting schema exposes `SUMMARY`/`DAILY` time units, while Amazon Ads Insights timing data carries `hourOfDay`.

```bash
amazon-ads-pp-cli break-even-acos --asin B0XXXX --price 32.99 --cogs 8.50 --fees 30 --json
amazon-ads-pp-cli true-profit --asin B0XXXX --price 32.99 --cogs 8.50 --fees 30 --ad-spend 4.20 --json
amazon-ads-pp-cli acos-vs-tacos --report product-performance.csv --seller-store ~/.config/amazon-seller-pp-cli/store.db --asin B0XXXX --json
amazon-ads-pp-cli portfolio-dashboard --report campaign-performance.csv --json
amazon-ads-pp-cli campaign-comparison --report campaign-performance.csv --json
amazon-ads-pp-cli product-ad-profitability --report product-performance.csv --cogs-file ~/.config/amazon-ads-pp-cli/cogs.toml --json
amazon-ads-pp-cli reports wait <report-id> --wait-timeout 20m --json
amazon-ads-pp-cli normalize-report --input downloaded-report.csv.gz --kind performance --output normalized.jsonl --format jsonl --store --json
amazon-ads-pp-cli placement-analysis --report placement-performance.csv --json
amazon-ads-pp-cli competitor-asin-mining --report product-targeting-performance.csv --asin B0XXXX --json
amazon-ads-pp-cli seasonal-planner --report historical-performance.csv --budget-multiplier 1.25 --json
amazon-ads-pp-cli dayparting-analysis --report hourly-performance.csv --json
amazon-ads-pp-cli budget-pacing --report hourly-campaign-performance.csv --threshold 0.90 --early-hour 18 --json
amazon-ads-pp-cli share-of-voice --report share-of-voice.csv --asin B0XXXX --keywords "self journal,daily planner" --json
amazon-ads-pp-cli search-term-mining --report search-terms.csv --target-acos 25 --json
amazon-ads-pp-cli wasted-spend --report search-terms.csv --threshold 10 --json
amazon-ads-pp-cli keyword-cannibalization --report search-terms.csv --json
amazon-ads-pp-cli negative-keyword-generator --report search-terms.csv --threshold 10 --json
amazon-ads-pp-cli new-keyword-opportunities --report search-terms.csv --target-acos 25 --json
amazon-ads-pp-cli bid-optimizer --report keyword-performance.csv --target-acos 25 --json
amazon-ads-pp-cli keyword-decay --baseline keyword-performance-previous.csv --current keyword-performance-current.csv --degradation-threshold 30 --min-spend 25 --json
amazon-ads-pp-cli keyword-lifecycle --report keyword-performance.csv --target-acos 25 --json
amazon-ads-pp-cli keyword-snapshots import --report keyword-performance.csv --snapshot-at 2026-02-01 --json
amazon-ads-pp-cli keyword-snapshots list --json
amazon-ads-pp-cli bid-history --keyword "self journal" --json
amazon-ads-pp-cli auto-negate --report search-terms.csv --threshold 15 --min-clicks 20 --json
amazon-ads-pp-cli auto-promote --report search-terms.csv --min-conversions 3 --max-acos 30 --json
amazon-ads-pp-cli budget-rebalance --report campaign-performance.csv --total-budget 500 --json
amazon-ads-pp-cli bid-rules apply --report keyword-performance.csv --file rules.json --json
amazon-ads-pp-cli automation-audit --limit 20 --json
```

Automation commands print plans by default. Pass `--apply` only when the report includes the Amazon IDs needed for mutation: `campaign_id` and `ad_group_id` for `auto-negate` and `auto-promote`, `campaign_id` for `budget-rebalance`, and `keyword_id` for `bid-rules apply`. Apply mode de-duplicates repeated remote mutation keys before sending, is capped by `--max-changes`, bid mutations are capped by `--max-bid`, and budget rebalance is capped by `--max-daily-budget` when set. `--apply --dry-run` previews the mutation request without sending it. `auto-promote --apply` also requires `--bid`.

## Commands

### amazon-ads-dsp-dsp

Manage amazon ads dsp dsp

- **`amazon-ads-pp-cli amazon-ads-dsp-dsp get`** - Returns advertiser information based on given advertiser id.
- **`amazon-ads-pp-cli amazon-ads-dsp-dsp list`** - Returns a list of advertisers with information which satisfy the filtering criteria.

### amazon-ads-profiles-profiles

Manage amazon ads profiles profiles

- **`amazon-ads-pp-cli amazon-ads-profiles-profiles get-by-id`** - This operation does not return a response unless the current account has created at least one campaign using the advertising console.
- **`amazon-ads-pp-cli amazon-ads-profiles-profiles list`** - Note that this operation does not return a response unless the current account has created at least one campaign using the advertising console.
- **`amazon-ads-pp-cli amazon-ads-profiles-profiles update`** - Note that this operation is only used for Sellers using Sponsored Products. This operation is not enabled for vendor type accounts.

### amazon-ads-sponsored-reports

Manage amazon ads sponsored reports

- **`amazon-ads-pp-cli amazon-ads-sponsored-reports <reportId>`** - Uses the `reportId` value from the response of a report previously requested via `POST` method of the `/sd/{recordType}/report` operation.

**To understand the call flow for asynchronous reports, see [Getting started with sponsored ads reports](/API/docs/en-us/reporting/v2/sponsored-ads-reports).**

### assets

Manage assets

- **`amazon-ads-pp-cli assets get`** - Retrieves an asset along with the metadata
- **`amazon-ads-pp-cli assets get-upload-location`** - Creates an ephemeral resource (upload location) to upload Assets to Creative Assets tool. The upload location is short lived and expires in 15 minutes.The upload location only supports PUT HTTP Method to upload the asset content. If the upload location expires, API user will get `403` Forbidden response.
* All ad specs - sizes and policies can be found [here](https://advertising.amazon.com/resources/ad-specs/?ref_=a20m_us_hnav_spcs)

* Program specific links
1. **Stores** - [here](https://advertising.amazon.com/resources/ad-specs/stores?ref_=a20m_us_spcs_stcrgd)
2. **SB/SBV/sponsored ads** - [here](https://advertising.amazon.com/resources/ad-policy/sponsored-ads-policies?ref_=a20m_us_spcs_sbv_spcs_spadcap)
- **`amazon-ads-pp-cli assets register`** - The API should be called once the asset is uploaded to the location
provided by the /asset/upload API endpoint.
- **`amazon-ads-pp-cli assets search`** - Search assets

### attribution

Manage attribution

- **`amazon-ads-pp-cli attribution get-advertisers-by-profile`** - For sellers, an attribution profile has one associated advertiser. For vendors, an attribution profile may have more than one associated advertiser.
- **`amazon-ads-pp-cli attribution get-publisher-macro-tag`** - Some third-party publishers do not support tags that include macro parameters. In this case, the attribution tag includes a set of '**insertValue**' placeholder values. Replace these placeholder values with your campaign, ad group, and ad identifiers to create unique ad-level tags.<br/><br/> For example: "?maas=maas_adg_api_123456789_static_9_99&ref_=aa_maas&tag=maas&aa_campaignid={**insertCampaignId**}&aa_adgroupid={**insertAdGroupId**}&aa_creativeid={**insertAdiD**}"<br/><br/> An example of an integrator nonMacro tag with filled campaign, ad group, and ad ID values is "?maas=maas_adg_api_123456789_static_9_99&ref_=aa_maas&tag=maas&aa_campaignid=**12345**&aa_adgroupid=**5678**&aa_creativeid=**1357**"
- **`amazon-ads-pp-cli attribution get-publisher-tag-template`** - Third-party publishers, such as Google Ads, Facebook, Microsoft Ads, and Pinterest support tags that include macro parameters. Using macro parameters, campaign tracking information is dynamically inserted into the click-through URL when an ad is clicked. This resource is a tag pre-populated with campaign, ad group, and ad level publisher macros with the values associated with your campaign. <br/><br/> For example, a Google Ads macro tag is "?maas=maas_adg_api_123456789_1_99&ref_=aa_maas&tag=maas&aa_campaignid={campaignid}&aa_adgroupid={adgroupid}&aa_creativeid=ad-{creative}_{targetid}_dev-{device}_ext-{feeditemid}"
- **`amazon-ads-pp-cli attribution get-publishers`** - Use the response to determine whether to use either the macroTags or nonMacroTemplateTags resource to get tags for a certain publisher.
- **`amazon-ads-pp-cli attribution get-tags-by-campaign`** - Gets an attribution report for a specified list of advertisers.

### audiences

Manage audiences

- **`amazon-ads-pp-cli audiences fetch-taxonomy`** - Returns a list of audience categories for a given category path

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli audiences list`** - Returns a list of audience segments for an advertiser. The result set can be filtered by providing an array of Filter objects. Each item in the resulting set will match all specified filters.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### benchmarks

Manage benchmarks

- **`amazon-ads-pp-cli benchmarks get-brands`** - Gets a list of brands that the advertising account has promoted in their SB campaigns

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli benchmarks get-report-data`** - Provides overview of metrics for all brands and categories that the entity has access to.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli benchmarks get-time-series`** - Provides time series data for the specified brand and category filtered by optional parameters

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### billing

Manage billing

- **`amazon-ads-pp-cli billing bulk-get-notifications`** - Gets an array of all currently valid billing notifications associated for each advertising account.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli billing bulk-get-status`** - Gets the current billing status associated for each advertising account.

**Requires one of these permissions**:
["advertiser_campaign_edit"]

### brands

Manage brands

- **`amazon-ads-pp-cli brands`** - Gets an array of Brand data objects for the Brand associated with the profile ID passed in the header. For more information about Brands, see [Brand Services](https://brandservices.amazon.com/).

### currencies

Manage currencies

- **`amazon-ads-pp-cli currencies`** - Gets an array of localized currencies in their target marketplaces using advertiser identifier in header, and source marketplace ID (via marketplaceId or countryCode) in the body

### dp

Manage dp

- **`amazon-ads-pp-cli dp create`** - Creates a new data provider audience. Note that the API call rate is limited to 1 transaction per second (TPS). Calls exceeding this rate are throttled.
- **`amazon-ads-pp-cli dp get`** - Gets metadata for an audience specified by identifier. Note that the API call rate is limited to 1 transaction per second (TPS). Calls exceeding this rate are throttled.
- **`amazon-ads-pp-cli dp update`** - Associates or disassociates a record with an audience. Note that the API call rate is limited to 100 transactions per second (TPS). Calls exceeding this rate are throttled. Payload size is limited to 1MB. Calls with a payload larger than 1MB receive a 413 response.
- **`amazon-ads-pp-cli dp update-audiencemetadata`** - Updates metadata of an existing audience specified by identifier. Note that the API call rate is limited to 1 transaction per second (TPS). Calls exceeding this rate are throttled.
- **`amazon-ads-pp-cli dp update-users`** - Deletes user data originally sourced from the client. The API call rate is limited to 1 transactions per second (TPS). Calls exceeding this rate are throttled. Payload size is limited to 1000 users or 1MB. Calls with a more than 1000 users or 1MB will receive a 413 response.

### dsp

Manage dsp

- **`amazon-ads-pp-cli dsp associate-line-items-to-creatives`** - Create/delete association between line item and creative.

Callout -  Do not pass in startDate, endDate and weight. Use the PUT operation instead to populate these fields. We will add support in POST in a future update. A future update will also include support for multiple at a time.
- **`amazon-ads-pp-cli dsp create-file-uploads-policy`** - Create file upload policy that used to upload file to AWS S3. File upload policy will expire in 15 minutes.
- **`amazon-ads-pp-cli dsp create-image-creative`** - Create an image creative.

Callout - A future update will add support to create multiple Image creatives at a time.
- **`amazon-ads-pp-cli dsp create-line-items`** - Create line item.

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp create-orders`** - Create an order.

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp create-rec-creatives`** - Create a new Responsive eCommerce Creatives(REC).

Callout - A future update will add support to create multiple REC creatives at a time.
- **`amazon-ads-pp-cli dsp create-third-party-creative`** - Create a third party creative.

Note that a future update will add support to create multiple third party creatives at a time.
- **`amazon-ads-pp-cli dsp create-video-creatives`** - Create a video creative

Callout - A future update will add support to create multiple Video creatives at a time.
- **`amazon-ads-pp-cli dsp export-products-by-order-id`** - Export conversion tracking products as a file by identifier. The file URL will expire in 15 minutes.
- **`amazon-ads-pp-cli dsp get-apps`** - Gets apps based on app Ids or text querys. Either one of app Ids or text query may be supplied, but not both.
- **`amazon-ads-pp-cli dsp get-conversion-trackings`** - Get conversion tracking information for given order.
- **`amazon-ads-pp-cli dsp get-creative-moderation`** - Get creative moderation summary by creativeId.
- **`amazon-ads-pp-cli dsp get-creatives`** - Gets one or more creatives.
- **`amazon-ads-pp-cli dsp get-domain-targeting`** - Gets one or more line items domain targeting information.
- **`amazon-ads-pp-cli dsp get-domains`** - Gets the list of domain lists for inclusion/exclusion based on entity. Lists are sorted by creation time.
- **`amazon-ads-pp-cli dsp get-dv-custom-contextual-segments`** - Retrieves custom contextual segments pre-bid targeting data for an account that is already linked to Double Verify. If an account is not linked to the provider, no data will be returned.
- **`amazon-ads-pp-cli dsp get-geo-locations`** - Gets locationTargeting objects based on locationTargetingId or text query, such as city name, zip code, or other address text. Either one of locationTargetingId or text query may be supplied, but not both.
- **`amazon-ads-pp-cli dsp get-goal-configurations`** - Gets a list of configurations that can be applied to orders to optimize for a desired campaign goal, sorted by goal name.
- **`amazon-ads-pp-cli dsp get-iab-content-categories`** - Gets the hierarchy of IAB content categories as a list sorted by ID in ascending order.
- **`amazon-ads-pp-cli dsp get-image-creatives`** - Get an image creative matching criteria provided in request.

Callout - A future update will add support to get multiple Image creatives at a time.
- **`amazon-ads-pp-cli dsp get-line-item`** - Gets line item with complete information specified by identifier.
- **`amazon-ads-pp-cli dsp get-line-items`** - Gets one or more line items with basic information.
- **`amazon-ads-pp-cli dsp get-odc-custom-predicts`** - Retrieves custom predict pre-bid targeting data for an account that is already linked to Oracle Data Cloud. If an account is not linked to the provider, no data will be returned.
- **`amazon-ads-pp-cli dsp get-odc-standard-predicts`** - Gets Oracle Data Cloud provided standard predicts for pre-bid targeting.
- **`amazon-ads-pp-cli dsp get-order`** - Gets an order with complete information specified by an identifier.
- **`amazon-ads-pp-cli dsp get-orders`** - Gets one or more orders with basic information.
- **`amazon-ads-pp-cli dsp get-pixels`** - Gets a list of pixels based on filters. AdvertiserIdFilter must be provided. Results are sorted by create time in ascending order (earliest first).
- **`amazon-ads-pp-cli dsp get-pixels-by-order-id`** - Get conversion tracking pixels by identifier.
- **`amazon-ads-pp-cli dsp get-product-categories`** - Gets the hierarchy of product category objects as a list sorted by ID in ascending order.
- **`amazon-ads-pp-cli dsp get-products-by-order-id`** - Get conversion tracking products by identifier. If the order was previously updated by list of products, response will be a list of products. The maximum size of list will be 2000. If the order was previously updated by product file, please use '/dsp/orders/{orderId}/conversionTracking/products/export' to export as a file.
- **`amazon-ads-pp-cli dsp get-rec-creatives`** - Get an Responsive eCommerce Creative (REC) matching criteria provided in request.

Callout - A future update will add support to get multiple REC creatives at a time.
- **`amazon-ads-pp-cli dsp get-supply-sources`** - Gets the supply sources based on line item type, advertiser and supply source type. When the supply source type is deal, orderId must be supplied. The returned list of deal supply sources will be filtered to include only those valid for the advertiser that owns the order and are running during the order dates.
- **`amazon-ads-pp-cli dsp get-third-party-creatives`** - Get a third party creative matching criteria provided in request.

Note that a future update will add support to get multiple third party creatives at a time.
- **`amazon-ads-pp-cli dsp get-video-creatives`** - Get a video creative matching criteria provided in request.

Callout - A future update will add support to get multiple Video creatives at a time.
- **`amazon-ads-pp-cli dsp list-line-item-creative-associations`** - Gets an array of creative associations, filtered by specified criteria.
- **`amazon-ads-pp-cli dsp preview-image-creative`** - Preview an image creative.
- **`amazon-ads-pp-cli dsp preview-rec-creative`** - Preview a Responsive eCommerce Creative(REC).
- **`amazon-ads-pp-cli dsp preview-third-party-creative`** - Preview a third party creative.
- **`amazon-ads-pp-cli dsp preview-video-creative`** - Preview a video creative
- **`amazon-ads-pp-cli dsp set-line-item-status`** - Setting delivery activation status for the given line item id. Version 2.x line items accept `application/vnd.dsplineitems.v2+json` for this API. Version 3.x line items accept `application/vnd.dsplineitemsdeliveryactivationstatus.v3+json` for this API.
- **`amazon-ads-pp-cli dsp set-order-status`** - Setting delivery activation status for the given order id.
- **`amazon-ads-pp-cli dsp update`** - Add or remove conversion tracking products from the order. It can be updated by either providing values for productList or productFile field. For productList, up to 2,000 ProductTrackingItems can be added, including up to 20 ProductTrackingItems per domain if FEATURED_WITH_VARIATIONS is specified in productAssociation. For productFile, up to 50,000 Products can be used. Check out our tutorial for more details.
- **`amazon-ads-pp-cli dsp update-conversion-tracking`** - Add or remove conversion tracking information from the order.
- **`amazon-ads-pp-cli dsp update-domain-targeting`** - Replaces the DomainTargeting for the specified line items with the ones provided in the request body.
- **`amazon-ads-pp-cli dsp update-image-creative`** - Update an image creative.

Callout - A future update will add support to update multiple Image creatives at a time.
- **`amazon-ads-pp-cli dsp update-line-item-creative-associations`** - Update association details. This API will be used to update startDate, endDate and weight parameters. Weight field can be updated only if creativeRotationType is `WEIGHTED`. CreativeRotationType field is under line item setting.

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp update-line-items`** - This is a full update, not partial patch. All the fields/data returned by GET LineItem by Id must be provided(including read-only fields). Any field that is changed/removed would be updated as provided in the request.
To update `deliveryActivationStatus` use POST deliveryActivationStatus by LineItem Id instead.

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp update-orders`** - This is a full update, not partial patch. All the fields/data returned by GET Order by Id must be provided(including read-only fields). Any field that is changed/removed would be updated as provided in the request.
To update `deliveryActivationStatus` use POST deliveryActivationStatus by Order Id instead.

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp update-pixels-by-order-id`** - Add or remove conversion tracking pixels from the order. The maximum size of pixel list is 100.
- **`amazon-ads-pp-cli dsp update-rec-creatives`** - Update existing Responsive eCommerce Creatives(REC).

Callout - A future update will add support for multiple at a time.
- **`amazon-ads-pp-cli dsp update-third-party-creative`** - Update a third party creative.

Note that a future update will add support to update multiple third party creatives at a time.
- **`amazon-ads-pp-cli dsp update-video-creatives`** - Update a video creative

Callout - A future update will add support to update multiple Video creatives at a time.

### dsp-reports-dsp

Manage dsp reports dsp

- **`amazon-ads-pp-cli dsp-reports-dsp create-report-v3`** - Use this operation to request creation of a report that includes metrics about your Amazon DSP campaigns. Specify the `type` of report and the `metrics` you'd like to include. Note that the value specified for the `dimensions` field affects the metrics included in the report. See the `dimensions` field description for more information.

**Authorized resource type**:
DSP Rodeo Entity ID, DSP Advertiser Account ID

**Parameter name**:
accountId

**Parameter in**:
path

**Requires one of these permissions**:
["view_performance_dashboard"]
- **`amazon-ads-pp-cli dsp-reports-dsp get-campaign-report-v3`** - Pass the identifier of a previously requested report in the `reportId` field to get the current status of the report. While the report is pending, `status` is set to `IN_PROGRESS`. When a response with `status` set to `SUCCESS` is returned, the report is available for download at the address specified in the `location` field.

**Authorized resource type**:
DSP Rodeo Entity ID, DSP Advertiser Account ID

**Parameter name**:
accountId

**Parameter in**:
path

**Requires one of these permissions**:
["view_performance_dashboard"]

### eligibility

Manage eligibility

- **`amazon-ads-pp-cli eligibility product`** - Gets a list of advertising eligibility objects for a set of products. Requests are permitted only for products sold by the merchant associated with the profile. Note that the request object is a list of ASINs, but multiple SKUs are returned if there is more than one SKU associated with an ASIN. If a product is not eligible for advertising, the response includes an object describing the reasons for ineligibility.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli eligibility program`** - Checks the advertiser's eligibility to ad programs.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### history

Manage history

- **`amazon-ads-pp-cli history`** - Returns history of changes for provided event sources that match the filters and time ranges specified. Only events that belong to the authenticated Advertiser can be queried. All times will be in UTC Epoch format. This API accepts identifiers in either the alphamumeric format (default), or the numeric format. If numeric IDs are supplied, then numeric IDs will be returned otherwise, alphanumeric IDs are returned.

### hsa

Manage hsa

### insights

Manage insights

- **`amazon-ads-pp-cli insights generate-brand-metrics-report`** - Generates the Brand Metrics report in CSV or JSON format. Customize the report by passing a specific categoryNodeTreeName, categoryNodePath, brandName, reportStartDate, reportEndDate, lookbackPeriod, format or a list of metrics from the available metrics in the metrics field. If an empty request body is passed, report for the latest available report date in JSON format will get generated with all the available brands and metrics for an advertiser. The report may or may not contain the Brand Metrics data for one or more brands depending on data availability.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli insights get-brand-metrics-report`** - Fetch the location and status of the report for the brands for which the metrics are available. The URL to the report is only available when the status of the report is SUCCESSFUL

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### invoices

Manage invoices

- **`amazon-ads-pp-cli invoices get`** - **Requires one of these permissions**:
["nemo_transactions_view","nemo_transactions_edit"]
- **`amazon-ads-pp-cli invoices get-advertiser`** - **Requires one of these permissions**:
["nemo_transactions_view","nemo_transactions_edit"]

### keywords

Manage keywords

- **`amazon-ads-pp-cli keywords`** - Returns localized keywords within specified marketplaces or locales.

### manager-accounts

Manage manager accounts

- **`amazon-ads-pp-cli manager-accounts create`** - Creates a new Amazon Advertising [Manager account](https://advertising.amazon.com/help?ref_=a20m_us_blog_whtsnewfb2020_040120#GU3YDB26FR7XT3C8).
- **`amazon-ads-pp-cli manager-accounts get-for-user`** - Returns all [Manager accounts](https://advertising.amazon.com/help?ref_=a20m_us_blog_whtsnewfb2020_040120#GU3YDB26FR7XT3C8) that a user has access to, along with metadata for all of the Amazon Advertising accounts that are linked to the Manager account.

### measurement

Manage measurement

- **`amazon-ads-pp-cli measurement cancel-studies`** - Cancel existing studies. Once a study is cancelled it can not be resumed again.
- **`amazon-ads-pp-cli measurement check-planning-eligibility`** - Checks eligibility against all vendor products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement create-surveys`** - Create new study surveys.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement get-dspbrand-lift-study-result`** - Get result of a DSP BRAND_LIFT study. Returns 200 successful response if json resource is requested in Accept header. Returns a 307 Temporary Redirect response if any of the file types is requested and response includes a location header with the value set to an AWS S3 path where the result is located. The path expires after 60 seconds.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement get-studies`** - Gets base study objects given a list of studyIds or a list of advertiserIds.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement get-surveys`** - Gets one or more study surveys with requested survey identifiers or a study identifier.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement omnichannel-metrics-brand-search`** - Search for brands to be used in the OMNICHANNEL_METRICS vendor product.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement update-surveys`** - Update measurement surveys. This will be a full update.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement vendor-product`** - Lists the supported measurement vendors products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement vendor-product-policy`** - Gets the policies for the specific vendor product(s).

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement vendor-product-survey-question-templates`** - Gets the survey question templates for the specific vendor product(s).

**Requires one of these permissions**:
[]

### measurement-dsp

Manage measurement dsp

- **`amazon-ads-pp-cli measurement-dsp check-dspaudience-research-eligibility`** - Checks the DSP AUDIENCE_RESEARCH study type eligibility status against vendor products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp check-dspbrand-lift-eligibility`** - Checks the DSP BRAND_LIFT study type eligibility status against vendor products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp check-dspcreative-testing-eligibility`** - Checks the DSP CREATIVE_TESTING study type eligibility status against vendor products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp check-dspomnichannel-metrics-eligibility`** - Checks the DSP OMNICHANNEL_METRICS study type eligibility status against vendor products.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp create-dspaudience-research-study`** - Create new DSP AUDIENCE_RESEARCH study.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp create-dspbrand-lift-studies`** - Create new DSP BRAND_LIFT studies.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp create-dspcreative-testing-study`** - Create new DSP CREATIVE_TESTING study.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp create-dspomnichannel-metrics-studies`** - Create new DSP OMNICHANNEL_METRICS studies.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspaudience-research-studies`** - Gets one or more DSP AUDIENCE_RESEARCH studies with requested study identifiers or an advertiser identifier.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspaudience-research-study-result`** - Get result of a DSP AUDIENCE_RESEARCH study. Returns 200 successful response if json resource is requested in Accept header. Returns a 307 Temporary Redirect response if any of the file types is requested and response includes a location header with the value set to an AWS S3 path where the result is located. The path expires after 60 seconds.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspbrand-lift-studies`** - Gets one or more DSP BRAND_LIFT studies with requested study identifiers or an advertiser identifier.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspcreative-testing-studies`** - Gets one or more DSP CREATIVE_TESTING studies with requested study identifiers or an advertiser identifier.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspcreative-testing-study-result`** - Get result of a DSP CREATIVE_TESTING study. Returns 200 successful response if json resource is requested in Accept header. Returns a 307 Temporary Redirect response if any of the file types is requested and response includes a location header with the value set to an AWS S3 path where the result is located. The path expires after 60 seconds.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspomnichannel-metrics-studies`** - Gets one or more DSP OMNICHANNEL_METRICS studies with requested study identifiers or an advertiser identifier.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp get-dspomnichannel-metrics-study-result`** - Get result of a DSP OMNICHANNEL_METRICS study. Returns a 307 Temporary Redirect response if any of the file types is requested and response includes a location header with the value set to an AWS S3 path where the result is located. The path expires after 60 seconds. Accept header does not support json for OMNICHANNEL_METRICS study type.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp update-dspaudience-research-study`** - Update DSP AUDIENCE_RESEARCH study. This will be a full update.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp update-dspbrand-lift-studies`** - Update DSP BRAND_LIFT studies. This will be a full update.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp update-dspcreative-testing-study`** - Update DSP CREATIVE_TESTING study. This will be a full update.

**Requires one of these permissions**:
[]
- **`amazon-ads-pp-cli measurement-dsp update-dspomnichannel-metrics-studies`** - Update DSP OMNICHANNEL_METRICS studies. This will be a full update.

**Requires one of these permissions**:
[]

### media

Manage media

- **`amazon-ads-pp-cli media complete-upload`** - The API should be called once the media is uploaded to the location provided by the /media/upload API endpoint. The API creates a Media resource for the uploaded media. Media resource is comprised of Media Identifier. The Media Identifier can be used to attach media to Ad Program (Sponsored Brands).

The API internally kicks off the asynchronous validation and processing workflow of the uploaded media. As a result, Media may not be immediately available for usage (to create Sponsored Brands Video Campaign) as soon as the response is received. See /media/describe API doc for instructions on when media is ready for campaign creation.
- **`amazon-ads-pp-cli media create-upload-resource`** - Creates an ephemeral resource (upload location) to upload Media for an Ad Program. The upload location is short lived and expires in 15 minutes. Once the upload is complete, /media/complete API should be used to notify that the upload is complete. <p> The upload location only supports `PUT` HTTP Method to upload the media content. If the upload location expires, API user will get `403 Forbidden` response. </p>
- **`amazon-ads-pp-cli media describe`** - API to poll for media status.
In order to attach media to campaign, media should be in either `PendingDeepValidation` or `Available` status.

`Available` status guarantees that media has completed processing and published for usage.

Though media can be attached to campaign once the status of the media transitions to `PendingDeepValidation`, media could still fail additional validation and transition to `Failed` status. For example in the context of SBV, SBV campaign can be created when status transitions to `PendingDeepValidation`, it could result in SBV campaign to be rejected later if media transitions to `Failed` status.

### overlapping-audiences

Manage overlapping audiences

- **`amazon-ads-pp-cli overlapping-audiences <audienceId>`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### page-asins

Manage page asins

- **`amazon-ads-pp-cli page-asins`** - Note that for sellers, the addresss must be a Store page. Vendors may also specify a custom landing page address.

### pre-moderation

Manage pre moderation

- **`amazon-ads-pp-cli pre-moderation`** - This API will be accepting different components of the ad/page and will be automatically validating the components and send back the policy violations if any. We recommend to send all components of the same entity to be sent together. It will make us better detect any policy violation if present. This will increase the Time to go live for the entity. In one request please don't send the components of more than one entity.

### product

Manage product

- **`amazon-ads-pp-cli product`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]

### products

Manage products

- **`amazon-ads-pp-cli products`** - Localizes (maps) products from a source marketplace to one or more target marketplaces. The localization process succeeds for a given target marketplace if a product matching the source product can be found there and the advertiser is eligible to advertise it. Seller requests have an additional condition: the SKU of a localized product must match the SKU of the source product.

### profiles

List Amazon Ads profiles available to the authorized application.

- **`amazon-ads-pp-cli profiles`** - List advertising profiles for the current Login with Amazon credentials.

### reports

Manage reports

- **`amazon-ads-pp-cli reports <reportId>`** - To understand the call flow for asynchronous reports, see [Getting started with sponsored ads reports](/API/docs/en-us/reporting/v2/sponsored-ads-reports).
- **`amazon-ads-pp-cli reports wait <reportId>`** - Poll `/v2/reports/{reportId}` until Amazon returns a terminal report status such as `SUCCESS` or `FAILURE`.

### sb

Manage sb

- **`amazon-ads-pp-cli sb archive-campaign`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/v2/guides/developer_notes) for more information.
- **`amazon-ads-pp-cli sb archive-keyword`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/v2/guides/developer_notes) for more information.
- **`amazon-ads-pp-cli sb archive-negative-keyword`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/v2/guides/developer_notes) for more information.
- **`amazon-ads-pp-cli sb archive-negative-target`** - Archives a negative target specified by identifier. Note that archiving is permanent, and once a negative target has been archived it can't be made active again.
- **`amazon-ads-pp-cli sb archive-target`** - Archives a target specified by identifier. Note that archiving is permanent, and once a target has been archived it can't be made active again.
- **`amazon-ads-pp-cli sb create-campaigns`** - **Note:** To create multi-ad group campaigns use the [version 4 POST campaigns](https://advertising.amazon.com/API/docs/en-us/sponsored-brands/3-0/openapi/prod#/Campaigns/CreateSponsoredBrandsCampaigns) endpoint. 

See the [create a Sponsored Brands campaign](https://advertising.amazon.com/help#GQFZA83P55P747BZ) topic in the Amazon Ads Support Center for more information about the campaign review process. **Note** to retrieve the state of a campaign submitted for creation, use the listCampaign operation and the campaign identifier from this operation. On SB creation, the state field is read-only. 
<br>**To create a video campaign specify adFormat as 'video'. If adFormat is not specified a Product Collection campaign will be created. Only a single video campaign can be created at a time.**
<br>**Note** each campaign in campaign creation operation supports adding keywords or negative keywords with maximum list size of 100. Additional keywords or negative keywords can be added using [createKeywords](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Keywords) or [createNegativeKeywords](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Negative_Keywords).
<br>**Note** each campaign in campaign creation operation supports adding targets or negative targets with maximum list size of 100. Additional targets or negative targets can be added using [createTargets](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Product%20targeting) or [createNegativeTargets](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Negative%20product%20targeting).
<br>**Note** that keywords or expressions *can not* be recreated for a campaign if the keyword or expression has previously been associated with a campaign and subsequently archived.
- **`amazon-ads-pp-cli sb create-draft-campaigns`** - Creates sponsored brands draft campaigns. <br>**To create a video campaign specify adFormat as 'video'. If adFormat is not specified then a product collection draft is created.**
<br>Note each draft campaign can have keywords, negative keywords, targets and negative targets with batch size of upto 100.
<br>**Note** each draft campaign in this operation supports adding keywords or negative keywords with maximum list size of 100. Additional keywords or negative keywords can be added using [createKeywords](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Keywords) or [createNegativeKeywords](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Negative_Keywords).
<br>**Note** each draft campaign in this operation supports adding targets or negative targets with maximum list size of 100. Additional targets or negative targets can be added using [createTargets](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Product%20targeting) or [createNegativeTargets](https://advertising.amazon.com/API/docs/v3/reference/SponsoredBrands/Negative%20product%20targeting).
- **`amazon-ads-pp-cli sb create-keywords`** - Note that `state` can't be set at keyword creation. Keywords submitted for creation have state set to `pending` while under moderation review. Moderation review may take up to 72 hours. <br/>Note that keywords can be created on campaigns where serving status is not one of `archived`, `terminated`, `rejected`, or `ended`. <br/>Note that this operation supports a maximum list size of 100 keywords.
- **`amazon-ads-pp-cli sb create-negative-keywords`** - Note that `bid` and `state` can't be set at negative keyword creation. <br/>Note that Negative keywords submitted for creation have state set to `pending` while under moderation review. Moderation review may take up to 72 hours. <br/>Note that negative keywords can be created on campaigns one where serving status is not one of `archived`, `terminated`, `rejected`, or `ended`. <br/>Note that this operation supports a maximum list size of 100 negative keywords.
<br>**Note** that negative keywords *can not* be recreated for a campaign if the negative keyword has previously been associated with a campaign and subsequently archived.
- **`amazon-ads-pp-cli sb create-negative-targets`** - Create one or more negative targets.
- **`amazon-ads-pp-cli sb create-targets`** - Create one or more targets.
- **`amazon-ads-pp-cli sb delete-draft-campaign`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/v2/guides/developer_notes) for more information.
- **`amazon-ads-pp-cli sb get`** - Note that this resource is only available for campaigns in the US marketplace.
- **`amazon-ads-pp-cli sb get-ad-group`** - Gets an ad group specified by identifier.
- **`amazon-ads-pp-cli sb get-bids-recommendations`** - Get a list of bid recommendation objects for a specified list of keywords or products.
- **`amazon-ads-pp-cli sb get-brand-recommendations`** - The Brand suggestions are based on a list of either category identifiers or keywords passed in the request. It is not valid to specify both category identifiers and keywords in the request.
- **`amazon-ads-pp-cli sb get-campaign`** - Gets a campaign specified by identifier.
- **`amazon-ads-pp-cli sb get-draft-campaign`** - Gets a draft campaign specified by identifier.
- **`amazon-ads-pp-cli sb get-keyword`** - Gets a keyword specified by identifier.
- **`amazon-ads-pp-cli sb get-negative-keyword`** - Gets a negative keyword specified by identifier.
- **`amazon-ads-pp-cli sb get-negative-target`** - Gets a negative target specified by identifier.
- **`amazon-ads-pp-cli sb get-product-recommendations`** - Recommendations are based on the ASINs that are passed in the request.
- **`amazon-ads-pp-cli sb get-target`** - Gets a target specified by identifier.
- **`amazon-ads-pp-cli sb get-targeting-categories`** - Recommendations are based on the ASINs that are passed in the request.
- **`amazon-ads-pp-cli sb list-ad-groups`** - Gets an array of ad groups associated with the client identifier passed in the authorization header, filtered by specified criteria.
- **`amazon-ads-pp-cli sb list-campaigns`** - **Note**: To ensure you are getting all campaign data, use the [version 4 list campaigns endpoint](https://advertising.amazon.com/API/docs/en-us/sponsored-brands/3-0/openapi/prod#/Campaigns/ListSponsoredBrandsCampaigns) instead.

To return Gets an array of all campaigns associated with the client identifier passed in the authorization header, filtered by specified criteria. Returns both productCollection and video campaigns. Use either `adFormatFilter` or `creativeType` to filter campaigns by ad formats such as `productCollection` or `video`. <br>**Note:** An advertiser that has lost brand eligibility will not be able to use any write operations such as `POST`, `PUT`, and `DELETE`. This includes the `GET` operation `/pageAsins`. However, the rest of the `GET` operations such as `/sb/campaigns` will be usable regardless of advertiser's eligibility status.
- **`amazon-ads-pp-cli sb list-draft-campaigns`** - Gets an array of all draft campaigns associated with the client identifier passed in the authorization header, filtered by specified criteria. <br>**Returns both productCollection and video draft campaigns by default. Use adFormatFilter to filter drafts by ad formats.**
- **`amazon-ads-pp-cli sb list-keywords`** - Gets an array of keywords, filtered by optional criteria.
- **`amazon-ads-pp-cli sb list-negative-keywords`** - Gets an array of negative keywords, filtered by optional criteria.
- **`amazon-ads-pp-cli sb list-negative-targets`** - Gets a list of product negative targets associated with the client identifier passed in the authorization header, filtered by specified criteria.
- **`amazon-ads-pp-cli sb list-targets`** - Gets a list of product targets associated with the client identifier passed in the authorization header, filtered by specified criteria.
- **`amazon-ads-pp-cli sb submit-draft-campaign`** - On successful submission, a campaign is created with an identifier that could be different from the original draft campaign identifier. The new identifier is returned in the response. Note that when a draft campaign is approved, the 'status' and 'servingStatus' fields are changed to values associated with an active campaign.
- **`amazon-ads-pp-cli sb update-campaigns`** - Mutable fields:
* `name` 
* `state`
* `portfolioId`
* `budget`
* `bidOptimization`
* `bidMultiplier`
* `bidAdjustments`
* `endDate`
- **`amazon-ads-pp-cli sb update-draft-campaigns`** - Updates one or more draft campaigns.
- **`amazon-ads-pp-cli sb update-keywords`** - Keywords submitted for update may have state set to `pending` for moderation review. Moderation may take up to 72 hours. <br/>Note that keywords can be updated on campaigns where serving status is not one of `archived`, `terminated`, `rejected`, or `ended`. <br/>Note that this operation supports a maximum list size of 100 keywords.
- **`amazon-ads-pp-cli sb update-negative-keywords`** - Negative keywords submitted for update may have state set to `pending` for moderation review. Moderation may take up to 72 hours. <br/>Note that negative keywords can be updated on campaigns where serving status is not one of `archived`, `terminated`, `rejected`, or `ended`. <br/>Note that this operation supports a maximum list size of 100 negative keywords.
- **`amazon-ads-pp-cli sb update-negative-targets`** - Updates one or more negative targets.
- **`amazon-ads-pp-cli sb update-targets`** - Updates one or more targets.

### sd

Manage sd

- **`amazon-ads-pp-cli sd archive-ad-group`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/en-us/info/developer-notes#archiving) for more information.
- **`amazon-ads-pp-cli sd archive-campaign`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/en-us/info/developer-notes#archiving) for more information.
- **`amazon-ads-pp-cli sd archive-negative-targeting-clause`** - Equivalent to using the updateNegativeTargetingClauses operation to set the `state` property of a targeting clause to `archived`. See [Developer Notes](http://advertising.amazon.com/API/docs/guides/developer_notes#Archiving) for more information.
- **`amazon-ads-pp-cli sd archive-product-ad`** - This operation is equivalent to an update operation that sets the status field to 'archived'. Note that setting the status field to 'archived' is permanent and can't be undone. See [Developer Notes](https://advertising.amazon.com/API/docs/en-us/info/developer-notes#archiving) for more information.
- **`amazon-ads-pp-cli sd archive-targeting-clause`** - Equivalent to using the `updateTargetingClauses` operation to set the `state` property of a targeting clause to `archived`. See [Developer
Notes](http://advertising.amazon.com/API/docs/guides/developer_notes#Archiving) for more information.
- **`amazon-ads-pp-cli sd associate-optimization-rules-with-ad-group`** - Associate one or more optimization rules to an ad group specified by identifier. Only one optimization rule can be associated per adGroup. This note will be removed when multiple rules are supported per adGroup.
- **`amazon-ads-pp-cli sd create-ad-groups`** - Creates one or more ad groups.
- **`amazon-ads-pp-cli sd create-brand-safety-deny-list-domains`** - Creates one or more domains to add to a Brand Safety Deny List. The Brand Safety Deny List is at the advertiser level. It can take up to 15 minutes from the time a domain is added to the time it is reflected in the deny list.
- **`amazon-ads-pp-cli sd create-campaigns`** - Creates one or more campaigns.
- **`amazon-ads-pp-cli sd create-creatives`** - A POST request of one or more creatives.
- **`amazon-ads-pp-cli sd create-negative-targeting-clauses`** - Successfully created negative targeting clauses associated with an ad group are assigned a unique target identifier.
Product negative targeting clause examples:
| Negative targeting clause | Description |
|---------------------------|-------------|
| asinSameAs=B0123456789 | Negatively target this product.|
| asinBrandSameAs=12345 | Negatively target products in the brand.|
- **`amazon-ads-pp-cli sd create-optimization-rules`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- **`amazon-ads-pp-cli sd create-product-ads`** - Creates one or more product ads.
- **`amazon-ads-pp-cli sd create-sdforecast`** - Returns forecasts for a given ad group specified in SD forecast request.
- **`amazon-ads-pp-cli sd create-targeting-clauses`** - Successfully created targeting clauses are assigned a unique `targetId` value.

Create new targeting clauses for campaigns with tactic 'T00020' using the following:
| Contextual targeting clause | Description |
|------------------|-------------|
| similarProduct | Dynamic segment to target products that are similar to the advertised asin. We recommend using 'similarProduct' targeting for all adGroups. |
| asinSameAs=B0123456789 | Target this product. |
| asinCategorySameAs=12345 | Target products in the category. |
| asinCategorySameAs=12345 asinBrandSameAs=45678 | Target products in the category and brand. |

**Refinements:**
- asinBrandSameAs
- asinPriceBetween
- asinPriceGreaterThan
- asinPriceLessThan
- asinReviewRatingLessThan
- asinReviewRatingGreaterThan
- asinReviewRatingBetween
- asinIsPrimeShippingEligible
- asinAgeRangeSameAs
- asinGenreSameAs

**Refinement Notes:**
* Brand, price, and review predicates are optional and may only be specified if category is also specified.
* Review predicates accept numbers between 0 and 5 and are inclusive.
* When using either of the 'between' strings to construct a targeting expression the format of the string is 'double-double' where the first double must be smaller than the second double. Prices are not inclusive.
* 'similarProduct' has no expression value or refinements.

Create new targeting clauses for campaigns with tactic 'T00030' using the following:
| Audience targeting clause | Description |
|------------------|-------------|
| views(exactProduct lookback=30) | Target an audience that has viewed the advertised asins in the past 7,14,30,60, or 90 days. |
| views(similarProduct lookback=60) | Target an audience that has viewed similar products to the advertised asins in the past 7,14,30,60, or 90 days. |
| views(asinCategorySameAs=12345 lookback=90) | Target an audience that has viewed products in the given category in the past 7,14,30,60, or 90 days. |
| views(asinCategorySameAs=12345 asinBrandSameAs=45678 asinPriceBetween=50-100 lookback=60) | Target an audience that has viewed products in the given category, brand, and price range in the past 7,14,30,60, or 90 days. |
| purchases(relatedProduct lookback=180) | Target an audience that has purchased a related product in the past 7,14,30,60,90,180 or 365 days|
| purchases(exactProduct lookback=365) | Target an audience that has purchased the advertised asins in the past 7,14,30,60,90,180 or 365 days|
| purchases(asinCategorySameAs=12345 asinBrandSameAs=45678 asinPriceBetween=50-100 lookback=90) | Target an audience that has purchased products in the given category, brand, and price range in the past 7,14,30,60,90,180 or 365 days |

Note:
1. There is a limit of 20 targeting clauses per request for T00030.
2. There is a limit of 100 targeting clauses per request for T00020.
3. If you receive the error of "Cannot create targeting clause: audience size is too small", please expand or broaden your targeting clause to increase the audience size.
- **`amazon-ads-pp-cli sd delete-brand-safety-deny-list`** - Archives all of the domains in the Brand Safety Deny List. It can take several hours from the time a domain is deleted to the time it is reflected in the deny list. You can check the status of the delete request by calling GET /sd/brandSafety/{requestId}/status. If the status is "COMPLETED", you can call GET /sd/brandSafety/deny to validate that your deny list has been successfully deleted.
- **`amazon-ads-pp-cli sd disassociate-optimization-rule`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- **`amazon-ads-pp-cli sd download-snapshot`** - **To understand the call flow for asynchronous snapshots, see [Getting started with sponsored ads snapshots](/API/docs/en-us/concepts/snapshots/sponsored-ads).**
- **`amazon-ads-pp-cli sd get`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available. Gets an OptimizationRule object for a requested Sponsored Display optimization rule.
- **`amazon-ads-pp-cli sd get-ad-group`** - Returns an AdGroup object for a requested campaign. Note that the AdGroup object is designed for performance, with a small set of commonly used ad group fields to reduce size. If the extended set of fields is required, use the campaign operations that return the AdGroupResponseEx object.
- **`amazon-ads-pp-cli sd get-ad-group-response-ex`** - Gets extended information for a requested ad group.
- **`amazon-ads-pp-cli sd get-adgroups`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available. Gets an OptimizationRule object for a requested Sponsored Display optimization rule.
- **`amazon-ads-pp-cli sd get-campaign`** - Returns a Campaign object for a requested campaign. Note that the Campaign object is designed for performance, with a small set of commonly used campaign fields to reduce size. If the extended set of fields is required, use the campaign operations that return the CampaignResponseEx object.
- **`amazon-ads-pp-cli sd get-campaign-response-ex`** - Returns a CampaignResponseEx object for a requested campaign. The CampaignResponseEx includes the extended set of available fields.
- **`amazon-ads-pp-cli sd get-negative-targets`** - This call returns the minimal set of negative targeting clause fields, but is more efficient than getNegativeTargetsEx.
- **`amazon-ads-pp-cli sd get-negative-targets-ex`** - Gets a negative targeting clause with extended fields. Note that this call returns the full set of negative targeting clause extended fields, but is less efficient than getNegativeTarget.
- **`amazon-ads-pp-cli sd get-product-ad`** - Note that the ProductAd object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use a product ad operations that returns the ProductAdResponseEx object.
- **`amazon-ads-pp-cli sd get-product-ad-response-ex`** - Gets extended information for a product ad.
- **`amazon-ads-pp-cli sd get-request-results`** - When a user adds domains to their Brand Safety Deny List, the request is processed asynchronously, and a requestId is provided to the user. This requestId can be used to view the request results for up to 90 days from when the request was submitted. The results provide the status of each domain in the given request. Request results may contain multiple pages. This endpoint will only be available once the request has completed processing. To see the status of the request you can call GET /sd/brandSafety/{requestId}/status. Note that this endpoint only lists the results of POST requests to /sd/brandSafety/deny - it does not reflect the results of DELETE requests.
- **`amazon-ads-pp-cli sd get-request-status`** - When a user modifies their Brand Safety Deny List, the request is processed asynchronously, and a requestId is provided to the user. This requestId can be used to check the status of the request for up to 90 days from when the request was submitted.
- **`amazon-ads-pp-cli sd get-snapshot`** - **To understand the call flow for asynchronous snapshots, see [Getting started with sponsored ads snapshots](/API/docs/en-us/tutorials/sponsored-ads-snapshots).**
- **`amazon-ads-pp-cli sd get-target-bid-recommendations`** - Provides a list of bid recommendations based on the list of input advertised ASINs and targeting clauses in the same format as the targeting API. For each targeting clause in the request a corresponding bid recommendation will be returned in the response. Currently the API will accept up to 100 targeting clauses.

The recommended bids are derrived from the last 7 days of winning auction bids for the related targeting clause.

Receive bid recommendations using the following:
Contextual targeting clause|Description|
|-----------|----|
|asinSameAs=B0123456789|Receive a bid recommendation for this target product
|asinCategorySameAs=12345|Receive a bid recommendation for this target category
|similarProduct|Receive a bid recommendation for targets that are similar to the advertised asins.

Audience targeting clause|Description|
|-----------|----|
|views(asinCategorySameAs=12345 lookback=30)|Receive a bid recommendation for a target audience that has viewed products in the given category
|views(similarProduct lookback=30)|Receive a bid recommendation for a target audience that has viewed similar products to the advertised asins
|views(exactProduct lookback=30)|Receive a bid recommendation for a target audience that has viewed the advertised asins

#### Notes:
- Bid recommendations for purchases and audiences are **not currently supported**. This note will be removed when these operations are available.
- Refinements are currently not supported and if included will not impact the bid recommendation for the target.

#### Advertised ASIN Notes:
- For asinSameAs targets the advertised asins will not impact the bid recommendation
- For asinCategrySameAs targets the advertised asins are optional, but including them will provide a more refined bid recommendation
- For similarProduct & exactProduct targets the advertised asins are required
- **`amazon-ads-pp-cli sd get-target-recommendations`** - This API provides product and category recommendations to target based on the list of input ASINs.
Allow 1 week for our systems to process data for any new ASINs listed on Amazon before using this service.

For API v3.0, the API returns up to 100 recommendations for contextual targeting.

For API v3.1, the API returns up to 100 recommendations for both product and category targeting.

For API v3.2, the API introduces contextual targeting themes in the request and returns product recommendations based on different targeting themes.

The currently available tactic identifiers are:

|Tactic Name|Type|Description|
|-----------|----|-----------|
|T00020&nbsp;|Contextual Targeting|Products: Choose individual products to show your ads in placements related to those products.|
|T00030&nbsp;|Audience Targeting|Audiences: Select individual audiences to show your ads.|
- **`amazon-ads-pp-cli sd get-targets`** - This call returns the minimal set of targeting clause fields.
- **`amazon-ads-pp-cli sd get-targets-ex`** - Gets a targeting clause object with extended fields. Note that this call returns the full set of targeting clause extended fields, but is less efficient than getTarget.
- **`amazon-ads-pp-cli sd list-ad-groups`** - Gets an array of AdGroup objects for a requested set of Sponsored Display ad groups. Note that the AdGroup object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use the ad group operations that return the AdGroupResponseEx object.
- **`amazon-ads-pp-cli sd list-ad-groups-ex`** - Gets an array of AdGroupResponseEx objects for a set of requested ad groups.
- **`amazon-ads-pp-cli sd list-campaigns`** - Gets an array of Campaign objects for a requested set of Sponsored Display campaigns. Note that the Campaign object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use the campaign operations that return the CampaignResponseEx object.
- **`amazon-ads-pp-cli sd list-campaigns-ex`** - Gets an array of CampaignResponseEx objects for a set of requested campaigns.
- **`amazon-ads-pp-cli sd list-creative-moderations`** - Gets a list of creative moderations
- **`amazon-ads-pp-cli sd list-creatives`** - Gets a list of creatives
- **`amazon-ads-pp-cli sd list-domains`** - Gets an array of websites/apps that are on the advertiser's Brand Safety Deny List. It can take up to 15 minutes
from the time a domain is added/deleted to the time it is reflected in the deny list.
- **`amazon-ads-pp-cli sd list-negative-targeting-clauses`** - Gets a list of negative targeting clauses objects for a requested set of Sponsored Display negative targets. Note that the Negative Targeting Clause object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use the negative target operations that return the NegativeTargetingClauseEx object.
- **`amazon-ads-pp-cli sd list-negative-targeting-clauses-ex`** - Gets an array of NegativeTargetingClauseEx objects for a set of requested negative targets. Note that this call returns the full set of negative targeting clause extended fields, but is less efficient than getNegativeTargets.
- **`amazon-ads-pp-cli sd list-optimization-rules`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available. Gets an array of OptimizationRule objects for a requested set of Sponsored Display optimization rules.
- **`amazon-ads-pp-cli sd list-product-ads`** - Gets an array of ProductAd objects for a requested set of Sponsored Display product ads. Note that the ProductAd object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use a product ad operation that returns the ProductAdResponseEx object.
- **`amazon-ads-pp-cli sd list-product-ads-ex`** - Gets an array of ProductAdResponseEx objects for a set of requested ad groups. The ProductAdResponseEx object includes the extended set of available fields.
- **`amazon-ads-pp-cli sd list-request-status`** - List status of all Brand Safety List requests. The list will contain requests that were submitted in the past 90 days.
- **`amazon-ads-pp-cli sd list-targeting-clauses`** - Gets a list of targeting clauses objects for a requested set of Sponsored Display targets. Note that the Targeting Clause object is designed for performance, and includes a small set of commonly used fields to reduce size. If the extended set of fields is required, use the target operations that return the TargetingClauseEx object.
- **`amazon-ads-pp-cli sd list-targeting-clauses-ex`** - Gets an array of TargetingClauseEx objects for a set of requested targets. Note that this call returns the full set of targeting clause extended fields, but is less efficient than getTargets.
- **`amazon-ads-pp-cli sd post-creative-preview`** - Gets creative preview HTML.
- **`amazon-ads-pp-cli sd update-ad-groups`** - Updates on or more ad groups.
- **`amazon-ads-pp-cli sd update-campaigns`** - Updates one or more campaigns.
- **`amazon-ads-pp-cli sd update-creatives`** - Updates one or more creatives.
- **`amazon-ads-pp-cli sd update-negative-targeting-clauses`** - Updates one or more negative targeting clauses. Negative targeting clauses are identified using their targetId. The mutable field is `state`. Maximum length of the array is 100 objects.
- **`amazon-ads-pp-cli sd update-optimization-rules`** - This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- **`amazon-ads-pp-cli sd update-product-ads`** - Updates one or more product ads.
- **`amazon-ads-pp-cli sd update-targeting-clauses`** - Updates one or more targeting clauses. Targeting clauses are identified using their targetId. The mutable fields are `bid` and `state`. Maximum length of the array is 100 objects.

### sp

Manage sp

- **`amazon-ads-pp-cli sp archive-ad-group`** - Sets the ad group status to `archived`. Archived entities cannot be made active again. See developer notes for more information.
- **`amazon-ads-pp-cli sp archive-campaign`** - Sets the campaign status to `archived`. Archived entities cannot be made active again. See [developer notes](https://advertising.amazon.com/API/docs/en-us/get-started/developer-notes#Archiving) for more information.
- **`amazon-ads-pp-cli sp archive-campaign-negative-keyword`** - Set the status of the specified campaign negative keyword to `archived`. Note that once the status for a keyword is set to `archived` it cannot be changed.
- **`amazon-ads-pp-cli sp archive-keyword`** - Set the status of the specified keyword to `archived`. Note that once the status for a keyword is set to `archived` it cannot be changed.
- **`amazon-ads-pp-cli sp archive-negative-keyword`** - Set the status of the specified negative keyword to `archived`. Note that once the status for a keyword is set to `archived` it cannot be changed.
- **`amazon-ads-pp-cli sp archive-negative-targeting-clause`** - Set the `status` of a negative targeting clause to `archived`. Note that once a negative targeting clause `status` is set to `archived`, it cannot be changed.
- **`amazon-ads-pp-cli sp archive-product-ad`** - Sets the state of a specified product ad to `archived`. Note that once the state is set to `archived` it cannot be changed.
- **`amazon-ads-pp-cli sp archive-targeting-clause`** - Set the `status` of a targeting clause to `archived`. Note that once a targeting clause `status` is set to `archived`, it cannot be changed.
- **`amazon-ads-pp-cli sp bulk-get-asin-suggested-keywords`** - Suggested keywords are returned in an array ordered by descending effectiveness.
- **`amazon-ads-pp-cli sp create-ad-groups`** - Creates one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-campaign-negative-keywords`** - Creates one or more campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-campaigns`** - Creates one or more campaigns. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-keyword-bid-recommendations`** - **Deprecation notice: This endpoint will be deprecated on December 31, 2022. Use [theme-based bid recommendations](/API/docs/en-us/sponsored-products/3-0/openapi/prod#/ThemeBasedBidRecommendation/GetThemeBasedBidRecommendationForAdGroup_v1) going forward.**
- **`amazon-ads-pp-cli sp create-keywords`** - Creates one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-negative-keywords`** - Creates one or more negative keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-negative-targeting-clauses`** - Creates one ore more negative targeting expressions. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-product-ads`** - Creates one or more product ads. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp create-target-recommendations`** - **Deprecation notice: This endpoint will be deprecated on February 28, 2023. Use [version 3 targeting recommendations](/API/docs/en-us/sponsored-products/3-0/openapi/prod#/Product%20Recommendation%20Service/getProductRecommendations) going forward.**
- **`amazon-ads-pp-cli sp create-targeting-clauses`** - Creates one or more targeting expressions. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp download-snapshot`** - **To understand the call flow for asynchronous snapshots, see [Getting started with sponsored ads snapshots](/API/docs/en-us/concepts/snapshots/sponsored-ads).**
- **`amazon-ads-pp-cli sp get-ad-group`** - Gets an ad group specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-ad-group-bid-recommendations`** - **Deprecation notice: This endpoint will be deprecated on December 31, 2022. Use [theme-based bid recommendations](/API/docs/en-us/sponsored-products/3-0/openapi/prod#/ThemeBasedBidRecommendation/GetThemeBasedBidRecommendationForAdGroup_v1) going forward.**
- **`amazon-ads-pp-cli sp get-ad-group-ex`** - Gets an ad group that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-ad-group-suggested-keywords`** - Gets suggested keywords for the specified ad group.
- **`amazon-ads-pp-cli sp get-ad-group-suggested-keywords-ex`** - Gets suggested keywords with extended data for the specified ad group.
- **`amazon-ads-pp-cli sp get-ad-groups`** - Gets one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-ad-groups-ex`** - Gets ad groups that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-asin-suggested-keywords`** - Suggested keywords are returned in an array ordered by descending effectiveness.
- **`amazon-ads-pp-cli sp get-bid-recommendations`** - Gets a list of bid recommendations for keyword, product, or auto targeting expressions.
- **`amazon-ads-pp-cli sp get-campaign`** - Gets a campaign specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-campaign-ex`** - Gets a campaign with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-campaign-negative-keyword`** - Gets a campaign negative keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-campaign-negative-keyword-ex`** - Gets a campaign negative keyword that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-keyword`** - Gets a keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-keyword-bid-recommendations`** - **Deprecation notice: This endpoint will be deprecated on December 31, 2022. Use [theme-based bid recommendations](/API/docs/en-us/sponsored-products/3-0/openapi/prod#/ThemeBasedBidRecommendation/GetThemeBasedBidRecommendationForAdGroup_v1) going forward.**
- **`amazon-ads-pp-cli sp get-keyword-ex`** - Gets a keyword with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-negative-keyword`** - Gets a negative keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-negative-keyword-ex`** - Gets a negative keyword that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-negative-targeting-clause`** - Get a negative targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-negative-targeting-clause-ex`** - Get a negative targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-product-ad`** - Gets a product ad specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-product-ad-ex`** - Gets extended data for a product ad specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-snapshot-status`** - **To understand the call flow for asynchronous snapshots, see [Getting started with sponsored ads snapshots](/API/docs/en-us/concepts/snapshots/sponsored-ads).**
- **`amazon-ads-pp-cli sp get-targeting-clause`** - Get a targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp get-targeting-clause-ex`** - Get a targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-campaign-negative-keywords`** - Gets a list of campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-campaign-negative-keywords-ex`** - Gets a list of campaign negative keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-campaigns`** - Gets an array of campaigns. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-campaigns-ex`** - Gets an array of campaigns with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-keywords`** - Gets one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-keywords-ex`** - Gets a list of keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-negative-keywords`** - Gets a list of negative keyword objects. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-negative-keywords-ex`** - Gets a list of negative keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-negative-targeting-clauses`** - Gets a list of negative targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-negative-targeting-clauses-ex`** - Gets a list of negative targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-product-ads`** - Gets a list of product ads filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-product-ads-ex`** - Gets extended data for a list of product ads filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-targeting-clauses`** - Gets a list of targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp list-targeting-clauses-ex`** - Gets a list of targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-ad-groups`** - Updates one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-campaign-negative-keywords`** - Updates one or more campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-campaigns`** - Updates one or more campaigns. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-keywords`** - Updates one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-negative-keywords`** - Updates one or more negative keywords. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-negative-targeting-clause`** - Updates one or more negative targeting clauses. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-product-ads`** - Updates one or more product ads specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- **`amazon-ads-pp-cli sp update-targeting-clause`** - Updates one or more targeting clauses. [PLANNED DEPRECATION 6/30/2023]

### sponsored-brands-sb

Manage sponsored brands sb

- **`amazon-ads-pp-cli sponsored-brands-sb campaigns-budget-usage`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-associated-budget-rules-for-sbcampaigns`** - A maximum of 250 rules can be associated to a campaign. Note that the name of each rule associated to a campaign is required to be unique.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-brand-video-creative`** - This API creates a new version of an existing creative for given [Sponsored Brands Brand Video Ad](https://devportal-internal-beta.demand-tools.advertising.a2z.com/API/docs/en-us/sponsored-brands-beta-1p#/Ads/CreateSponsoredBrandsBrandVideoAds) by supplying brand video creative content

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-budget-rules-for-sbcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-product-collection-creative`** - This API creates a new version of creative for given Sponsored Brands ad by supplying product collection creative content

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brand-store-spotlight-ads`** - Creates Sponsored Brands store spotlight ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-ad-groups`** - Creates Sponsored Brands ad groups.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-brand-video-ads`** - Creates Sponsored Brands brand video ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-campaigns`** - Creates Sponsored Brands campaigns.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-product-collection-ads`** - Creates Sponsored Brands product collection ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-video-ads`** - Creates Sponsored Brands video ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-store-spotlight-creative`** - This API creates a new version of creative for given Sponsored Brands ad by supplying store spotlight creative content

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb create-video-creative`** - This API creates a new version of an existing creative for given Sponsored Brands ad by supplying video creative content

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb delete-sponsored-brands-ad-groups`** - Deletes Sponsored Brands ad groups.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb delete-sponsored-brands-ads`** - Deletes Sponsored Brands ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb disassociate-associated-budget-rule-for-sbcampaigns`** - Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- **`amazon-ads-pp-cli sponsored-brands-sb get-budget-recommendations`** - Provides daily budget recomemndations for a list of requested Sponsored Brands campaigns, with context on estimated historical missed opportunities.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb get-budget-rule-by-rule-id-for-sbcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb get-campaign-shopper-segment-forecast`** - Gets shopper segment bidding campaign performance forecasts.

**Requires one of these permissions**:
["advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb get-campaigns-associated-with-sbbudget-rule`** - Gets all the campaigns associated with a budget rule
- **`amazon-ads-pp-cli sponsored-brands-sb get-headline-recommendations`** - API to receive creative headline suggestions.
- **`amazon-ads-pp-cli sponsored-brands-sb get-keyword-recommendations`** - Gets an array of keyword recommendation objects for a set of ASINs included either on a landing page or a Stores page. Vendors may also specify a custom landing page.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb get-rule-based-budget-history-for-sbcampaigns`** - The budget history is returned for the time period specified in the required startDate and endDate parameters. The maximum time period is 90 days.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb get-sbbudget-rules-for-advertiser`** - Get all budget rules created by an advertiser
- **`amazon-ads-pp-cli sponsored-brands-sb list-associated-budget-rules-for-sbcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb list-creatives`** - This API gets an array of all Sponsored Brands creatives that qualify the given resource identifiers and filters

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-ad-groups`** - Lists Sponsored Brands ad groups.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-ads`** - Lists Sponsored Brands ads.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-campaigns`** - Lists Sponsored Brands campaigns.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb sbget-budget-rules-recommendation`** - A rule enables an automatic budget increase for a specified date range or for a special event. The response also includes a suggested budget increase for each special event.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-negative-brands`** - Returns brands recommended for negative targeting. Only available for Sellers and Vendors. These recommendations include your own brands because targeting your own brands usually results in lower performance than targeting competitors' brands. 

Only available in the following marketplaces: US, CA, MX, UK, DE, FR, ES, IT, IN, JP

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-refinements-for-category`** - Returns refinements according to category input. 

Only available in the following marketplaces: US, CA, MX, UK, DE, FR, ES, IT, IN, JP

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-targetable-asincounts`** - Get number of targetable asins based on refinements provided by the user.

Use `/sb/targets/categories` or `/sb/recommendations/targets/category` to retrieve the category ID. Use `/sb/targets/categories/{categoryRefinementId}/refinements` to retrieve refinements data for a category.

Only available in the following marketplaces: US, CA, MX, UK, DE, FR, ES, IT, IN, JP

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-targetable-categories`** - Returns all targetable categories by default in a list. List of categories can be used to build and traverse category tree.
Set query parameter `includeOnlyRootCategories=true` to return only the root categories, or set `parentCategoryRefinementId` to return children of a specific parent category.
Each category node has the fields - category name, category refinement id, parent category refinement id, isTargetable flag, and ASIN count range. 

Only available in the following marketplaces: US, CA, MX, UK, DE, FR, ES, IT, IN, JP

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-brands-sb update-budget-rules-for-sbcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb update-sponsored-brands-ad-groups`** - Updates Sponsored Brands ad groups.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-brands-sb update-sponsored-brands-ads`** - Updates Sponsored Brands ads.

**Requires one of these permissions**:
["advertiser_campaign_edit"]

### sponsored-display-sd

Manage sponsored display sd

- **`amazon-ads-pp-cli sponsored-display-sd campaigns-budget-usage`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd create-associated-budget-rules-for-sdcampaigns`** - A maximum of 250 rules can be associated to a campaign. Note that the name of each rule associated to a campaign is required to be unique.
- **`amazon-ads-pp-cli sponsored-display-sd create-brand-safety-deny-list-domains`** - Creates one or more domains to add to a Brand Safety Deny List. The Brand Safety Deny List is at the advertiser level. It can take up to 15 minutes from the time a domain is added to the time it is reflected in the deny list.
- **`amazon-ads-pp-cli sponsored-display-sd create-budget-rules-for-sdcampaigns`** - Creates one or more budget rules.
- **`amazon-ads-pp-cli sponsored-display-sd delete-brand-safety-deny-list`** - Archives all of the domains in the Brand Safety Deny List. It can take several hours from the time a domain is deleted to the time it is reflected in the deny list. You can check the status of the delete request by calling GET /sd/brandSafety/{requestId}/status. If the status is "COMPLETED", you can call GET /sd/brandSafety/deny to validate that your deny list has been successfully deleted.
- **`amazon-ads-pp-cli sponsored-display-sd disassociate-associated-budget-rule-for-sdcampaigns`** - Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- **`amazon-ads-pp-cli sponsored-display-sd download-snapshot-by-id`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-budget-rule-by-rule-id-for-sdcampaigns`** - Gets a budget rule specified by identifier.
- **`amazon-ads-pp-cli sponsored-display-sd get-campaigns-associated-with-sdbudget-rule`** - Gets all the campaigns associated with a budget rule
- **`amazon-ads-pp-cli sponsored-display-sd get-headline-recommendations-for`** - You can use this Sponsored Display API to retrieve creative headline recommendations from an array of ASINs.

**Requires one of these permissions**:
["advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-request-results`** - When a user adds domains to their Brand Safety Deny List, the request is processed asynchronously, and a requestId is provided to the user. This requestId can be used to view the request results for up to 90 days from when the request was submitted. The results provide the status of each domain in the given request. Request results may contain multiple pages. This endpoint will only be available once the request has completed processing. To see the status of the request you can call GET /sd/brandSafety/{requestId}/status. Note that this endpoint only lists the results of POST requests to /sd/brandSafety/deny - it does not reflect the results of DELETE requests.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-request-status`** - When a user modifies their Brand Safety Deny List, the request is processed asynchronously, and a requestId is provided to the user. This requestId can be used to check the status of the request for up to 90 days from when the request was submitted

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-rule-based-budget-history-for-sdcampaigns`** - The budget history is returned for the time period specified in the required startDate and endDate parameters. The maximum time period is 90 days.
- **`amazon-ads-pp-cli sponsored-display-sd get-sdbudget-rules-for-advertiser`** - Get all budget rules created by an advertiser
- **`amazon-ads-pp-cli sponsored-display-sd get-snapshot-by-id`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-target-bid-recommendations`** - Provides a list of bid recommendations based on the list of input advertised ASINs and targeting clauses in the same format as the targeting API. For each targeting clause in the request a corresponding bid recommendation will be returned in the response. Currently the API will accept up to 100 targeting clauses.

The recommended bids are derrived from the last 7 days of winning auction bids for the related targeting clause.

Receive bid recommendations using the following:
Product targeting clause|Description|
|-----------|----|
|asinSameAs=B0123456789|Receive a bid recommendation for this target product
|asinCategorySameAs=12345|Receive a bid recommendation for this target category
|similarProduct|Receive a bid recommendation for targets that are similar to the advertised asins.

Audience targeting clause|Description|
|-----------|----|
|views(asinCategorySameAs=12345 lookback=30)|Receive a bid recommendation for a target audience that has viewed products in the given category
|views(similarProduct lookback=30)|Receive a bid recommendation for a target audience that has viewed similar products to the advertised asins
|views(exactProduct lookback=30)|Receive a bid recommendation for a target audience that has viewed the advertised asins

#### Refinement Notes:
- Refinements are currently not supported and if included will not impact the bid recommendation for the target

#### Advertised ASIN Notes:
- For asinSameAs targets the advertised asins will not impact the bid recommendation
- For asinCategrySameAs targets the advertised asins are optional, but including them will provide a more refined bid recommendation
- For similarProduct & exactProduct targets the advertised asins are required

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd get-target-recommendations`** - Provides a list of products to target based on the list of input ASINs. Currently the API will return up to 100 recommended products and categories.
The currently available tactic identifiers are:

|Tactic Name|Type|Description|
|-----------|----|-----------|
|T00020&nbsp;|Product Targeting|Products: Choose individual products to show your ads in placements related to those products.|
|T00030&nbsp;|Audience Targeting|Audiences: Select individual audiences to show your ads.|

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd list-associated-budget-rules-for-sdcampaigns`** - Gets a list of budget rules associated to a campaign specified by identifier.
- **`amazon-ads-pp-cli sponsored-display-sd list-domains`** - Gets an array of websites/apps that are on the advertiser's Brand Safety Deny List. It can take up to 15 minutes from the time a domain is added/deleted to the time it is reflected in the deny list.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd list-request-status`** - List status of all Brand Safety List requests. The list will contain requests that were submitted in the past 90 days.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-display-sd update-budget-rules-for-sdcampaigns`** - Update one or more budget rules.

### sponsored-products-sp

Manage sponsored products sp

- **`amazon-ads-pp-cli sponsored-products-sp campaigns-budget-usage`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp create-associated-budget-rules-for-spcampaigns`** - A maximum of 250 rules can be associated to a campaign. Note that the name of each rule associated to a campaign is required to be unique.

**Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-budget-rules-for-spcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-optimization-rule`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-ad-groups`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaign-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaign-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-product-ads`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-campaign-optimization-rule`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-ad-groups`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaign-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaign-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-product-ads`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp disassociate-associated-budget-rule-for-spcampaigns`** - Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- **`amazon-ads-pp-cli sponsored-products-sp get-budget-recommendation`** - Creates daily budget recommendation along with benchmark metrics when creating a new campaign.
- **`amazon-ads-pp-cli sponsored-products-sp get-budget-recommendations`** - Given a list of campaigns as input, this API provides the following metrics -  <br> <b>1. Recommended daily budget - </b> Estimated budget needed to keep the campaign in budget for the full 24-hour period. Consider this budget to minimize your campaign's chances of running out of budget. <br> <b>2. Percent time in budget </b> - The share of time the campaign was in budget during the past 7 days. <br> <b>3. Estimated missed impressions, clicks and sales </b> - for all campaigns. These are the estimated additional impressions, clicks and sales the campaign might have generated had it adopt the recommended budget. These are estimates based on previous website traffic and campaign's historical performance - and not a guarantee of actual impressions, clicks and sales. Consider using these metrics to further inform your budget allocation decisions. Note: the API only supports NA region currently and when you send the requst, please make sure the campaign belongs to the corresponding marketplace.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-budget-rule-by-rule-id-for-spcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-campaign-optimization-rule`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-campaign-recommendations`** - Gets the top consolidated recommendations across bid, budget, targeting for SP campaigns given an advertiser profile id. The recommendations are refreshed everyday.

**Requires one of these permissions**:
["advertiser_campaign_view","advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp get-campaigns-associated-with-spbudget-rule`** - Gets all the campaigns associated with a budget rule
- **`amazon-ads-pp-cli sponsored-products-sp get-category-recommendations-for-asins`** - Returns a list of category recommendations for the input list of ASINs. Use this API to discover relevant categories to target. To find ASINs, either use the Product Metadata API or browse the Amazon Retail Website. <br> <ul><li>Response can be requested in different versions with the help of accept header. Please review the response mediaTypes for more information.</li><ul>

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-negative-brands`** - Returns brands recommended for negative targeting. Only available for Sellers and Vendors. These recommendations include your own brands because targeting your own brands usually results in lower performance than targeting competitors' brands.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-optimization-rule-eligibility`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-product-recommendations`** - Given an advertised ASIN as input, this API returns suggested ASINs to target in a product targeting campaign. We use various methods to generate these suggestions. These include using historical performance of your ad, items that shoppers they frequently view and purchase together, etc. The suggested targets can be retrieved either as a single list, or grouped by ‘theme' – i.e. an accompanying context for why we recommend the items. You can pick the desired format using the Accepts header, please see the response mediaTypes for more information. </br>
<h4>Pagination Behavior</h4> The API supports cursor based pagination using encoded cursor values to return next set of records or previously served records. The <b>count</b> parameter in the request body will be used to determine the size of results when requesting the previous page or next page. If no value for <b>count</b> is passed in the request, a default value is assumed. Please refer the range and defaults for these values in the request schema under GetProductRecommendationsRequest. </br> <i><b>Note:</b> The clients should never cache pagination cursor values locally as these values will expire after a certain time period. However a cursor value can be reused to perform retries in case of failures as long as the value has not expired.
</br></br> <h4>Themes </h4> Themes provide additional context for why we are recommending a product as a target. See below for an overall list of themes currently available –  </br><b>- Top converting targets</b> – These ASINs generated conversions for the input ASIN in the past 30 days (e.g. your product appeared as an ad on the detail page of these items, and a shopper clicked and purchased your item). The suggested ASINs under this theme are sorted in decreasing order of sales generated for your promoted item. </br><b>- Similar items (frequently viewed together)</b> – Items that shoppers frequently view and click along with your advertised item during a shopping session.
</br><b>- Complements</b> – Items that are frequently purchased together as complements. For example, if you are promoting a tennis racquet, you may see tennis balls recommended under this theme.
</br><b>- Similar items with low ratings and reviews</b> – Subset of the ‘similar items’ theme containing items that are rated lower than 3 stars and/or with fewer than 5 reviews.
</br><b>- Other books read by your readers</b> – Items that shoppers frequently view and click along with your advertised item during a shopping session. </br></br><i><b>Note:</b> Availability of themes differs by input ASIN - some ASINs may not have all above themes available</i>
- **`amazon-ads-pp-cli sponsored-products-sp get-ranked-keyword-recommendation`** - The <b> POST /sp/targets/keywords/recommendations </b> endpoint returns recommended keyword targets given either A) a list of ad ASINs or B) a campaign ID and ad group ID. Please use the recommendationType field to specify if you want to use option A or option B. This endpoint will also return recommended bids along with each recommendation keyword target.<br><br> <b> Ranking </b> <br> The keyword recommendations will be ranked in descending order of clicks or impressions, depending on the <b>sortDimension</b> field provided by the user. You may also input your own keyword targets to be ranked alongside the keyword recommendations by using the <b>targets</b> array. <br><br> <b> Localization </b> <br> Use the <b> locale </b> field to get keywords in your specified locale. Supported marketplace to locale mappings can be found at the <a href='https://advertising.amazon.com/API/docs/en-us/localization/#/Keyword%20Localization'>POST /keywords/localize</a> endpoint. <h1> Version 5.0 </h1>  <h2> New Features </h2> Version 5.0 utilizes the new theme-based bid recommendations, which can be retrieved at the endpoint <b>/sp/targets/bid/recommendations</b>, to return improved bid recommendations for each keyword. Theme-based bid recommendations provide \\\"themes\\\" and \\\"impact metrics\\\" along with each bid suggestion to help you choose the right bid for your keyword target.<br><br><b>Themes</b><br> We now may return multiple bid suggestions for each keyword target. Each suggestion will have a theme to express the business objective of the bid. Available themes are: <ul> <li> CONVERSION_OPPORTUNITIES - The default theme which aims to maximize number of conversions. </li> <li> SPECIAL_DAYS - A theme available during high sales events such as Prime Day, to anticipate an increase in sales and competition.</li></ul><b>Impact Metrics</b><br>We have added impact metrics which provide insight on the number of clicks and conversions you will receive for targeting a keyword at a certain bid. <br><br><b>Bidding Strategy</b><br> You may now specify your bidding strategy in the KEYWORDS_BY_ASINS request to get bid suggestions tailored to your bidding strategy. For KEYWORDS_BY_ADGROUP requests, you will not specify a bidding strategy, because the bidding strategy of the ad group is used. The three bidding strategies are: <ul> <li> LEGACY_FOR_SALES - Dynamic bids (down only) </li> <li> AUTO_FOR_SALES - Dynamic bids (up and down) </li> <li> MANUAL - Fixed bids </li> </ul> <h3> Availability </h3> Version 5.0 is only available in the following marketplaces: US, CA, UK, DE, FR, ES, IN, JP. <h1> Version 4.0 </h1> <h2> New features </h2> Version 4.0 allows users to retrieve recommended keyword targets which are sorted in descending order of clicks or conversions. The default sort dimension, if not specified, ranks recommendations by our interal ranking mechanism. We have also have added search term metrics. <b> Search term impression share </b> indicates the percentage share of all ad-attributed impressions you received on that keyword in the last 30 days. This metric helps advertisers identify potential opportunities based on their share on relevant keywords. <b> Search term impression rank </b> indicates your ranking among all advertisers for the keyword by ad impressions in a marketplace. It tells an advertiser how many advertisers had higher share of ad impressions. <i> Search term information is only available for keywords the advertiser targeted with ad impressions. </i> <h3> Availability </h3> Version 4.0 is available in all marketplaces.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-refinements-for-category`** - Returns refinements according to category input.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-rule-based-budget-history-for-spcampaigns`** - The budget history is returned for the time period specified in the required startDate and endDate parameters. The maximum time period is 90 days.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-rule-notification`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-spbudget-rules-for-advertiser`** - Get all budget rules created by an advertiser
- **`amazon-ads-pp-cli sponsored-products-sp get-targetable-asincounts`** - Get number of targetable asins based on refinements provided by the user. Please use the GetTargetableCategories API or the GetCategoryRecommendationsForASINs API to retrieve the category ID. Please use the GetRefinementsByCategory API to retrieve refinements data for a category.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-targetable-categories`** - Returns all targetable categories. This API returns a large JSON string containing a tree of category nodes. Each category node has the fields - category id, category name, and child categories.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp get-theme-based-bid-recommendation-for-ad-group-v1`** - The current version of the theme-based bid recommendation service supports auto-targeting and keyword targeting expressions only. Note that the currency for bid recommendations are in local currency units.
- **`amazon-ads-pp-cli sponsored-products-sp list-associated-budget-rules-for-spcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-ad-groups`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaign-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaign-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-product-ads`** - **Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp search-brands`** - Returns up to 100 brands related to keyword input for negative targeting.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp spget-budget-rules-recommendation`** - A rule enables an automatic budget increase for a specified date range or for a special event. The response also includes a suggested budget increase for each special event.

**Requires one of these permissions**:
["advertiser_campaign_edit","advertiser_campaign_view"]
- **`amazon-ads-pp-cli sponsored-products-sp update-budget-rules-for-spcampaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-optimization-rule`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-ad-groups`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaign-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaign-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaigns`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-negative-keywords`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-negative-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-product-ads`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]
- **`amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-targeting-clauses`** - **Requires one of these permissions**:
["advertiser_campaign_edit"]

### stores

(Not available for video campaigns)

- **`amazon-ads-pp-cli stores create-asset`** - Image assets are stored in the Store Assets Library. Note that there may be a delay before the image is displayed in the console.
- **`amazon-ads-pp-cli stores list-assets`** - For sellers or vendors, gets an array of assets associated with the specified brand entity identifier. Vendors are not required to specify a brand entity identifier, and in this case all assets associated with the vendor are returned.

### targeting-expression

Manage targeting expression

- **`amazon-ads-pp-cli targeting-expression`** - Localizes (maps) targeting expressions from a source marketplace to one or more target marketplaces. V3: Providing locales in your request's source details or target details, will now return in &lt;sourceField&gt; and &lt;targetField&gt; respectively the translations of your targeting expressions.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
amazon-ads-pp-cli amazon-ads-dsp-dsp list

# JSON for scripting and agents
amazon-ads-pp-cli amazon-ads-dsp-dsp list --json

# Filter to specific fields
amazon-ads-pp-cli amazon-ads-dsp-dsp list --json --select id,name,status

# Dry run — show the request without sending
amazon-ads-pp-cli amazon-ads-dsp-dsp list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
amazon-ads-pp-cli amazon-ads-dsp-dsp list --agent
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
amazon-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/amazon-ads-pp-cli/config.toml`

Local credential file: `~/.config/amazon-ads-pp-cli/.env`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AMAZON_ADS_CLIENT_ID` | auth_flow_input | Yes | Set during initial auth setup. |
| `AMAZON_ADS_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |
| `AMAZON_ADS_REFRESH_TOKEN` | auth_flow_input | Yes | Set during initial auth setup. |
| `AMAZON_ADS_PROFILE_ID` | request_scope | Yes for scoped endpoints | Selected Amazon Ads advertising profile ID. |

The CLI reads these values from the process environment first, then from `~/.config/amazon-ads-pp-cli/.env`.

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `amazon-ads-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `amazon-ads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AMAZON_ADS_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
