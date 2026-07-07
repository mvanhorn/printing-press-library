---
name: pp-amazon-ads
description: "Printing Press CLI for Amazon Ads reports, profitability analytics, keyword optimization, and guarded automation."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - amazon-ads-pp-cli
    install:
      - kind: go
        bins: [amazon-ads-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/cmd/amazon-ads-pp-cli
---

# Amazon Ads — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `amazon-ads-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install amazon-ads --cli-only
   ```
2. Verify: `amazon-ads-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/cmd/amazon-ads-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Start here: profitability

For a seller focused on organic + advertising profitability, start with these commands before browsing the full API surface:

1. `amazon-ads-pp-cli break-even-acos` — Calculate the maximum ACOS a product can carry before it loses money.
2. `amazon-ads-pp-cli true-profit` — Calculate product profit after COGS, estimated Amazon fees, and ad spend.
3. `amazon-ads-pp-cli acos-vs-tacos --report product-performance.csv --seller-store ~/.config/amazon-seller-pp-cli/store.db` — Join ad spend to seller-store sales for ACOS and TACOS by ASIN when the seller store has matching data.
4. `amazon-ads-pp-cli portfolio-dashboard --report campaign-performance.csv` — Summarize spend, sales, orders, ACOS, CPC, CTR, and CVR.
5. `amazon-ads-pp-cli product-ad-profitability --report product-performance.csv --cogs-file products.toml` — Estimate per-ASIN profit from ad performance and COGS.
6. `amazon-ads-pp-cli campaign-comparison --report campaign-performance.csv` — Compare campaigns by spend, sales, ACOS, CPC, CTR, and CVR.

Use `amazon-ads-pp-cli reports recipe <command>` when a command asks for `--report`; it prints the exact report kind, required columns, export path, and sample header. The schema-aware flows support console CSV/TSV exports and Ads API JSON/GZIP report artifacts.

`acos-vs-tacos` covers ad spend joined to seller-store sales and TACOS by ASIN when the seller store overlaps the ads report. It does not yet cover organic sessions, conversion rate, parent/child ASIN rollups, refunds, COGS unless `--cogs-file` is provided to profit commands, or TACOS by portfolio. For the seller-side dashboard workflow, cross-check `amazon-seller-pp-cli sales-intel dashboard`.

## Command Reference

**amazon-ads-dsp-dsp** — Manage amazon ads dsp dsp

- `amazon-ads-pp-cli amazon-ads-dsp-dsp get` — Returns advertiser information based on given advertiser id.
- `amazon-ads-pp-cli amazon-ads-dsp-dsp list` — Returns a list of advertisers with information which satisfy the filtering criteria.

**amazon-ads-profiles-profiles** — Manage amazon ads profiles profiles

- `amazon-ads-pp-cli amazon-ads-profiles-profiles get-by-id` — This operation does not return a response unless the current account has created at least one campaign using the
- `amazon-ads-pp-cli amazon-ads-profiles-profiles list` — Note that this operation does not return a response unless the current account has created at least one campaign using
- `amazon-ads-pp-cli amazon-ads-profiles-profiles update` — Note that this operation is only used for Sellers using Sponsored Products.

**amazon-ads-sponsored-reports** — Manage amazon ads sponsored reports

- `amazon-ads-pp-cli amazon-ads-sponsored-reports <reportId>` — Uses the `reportId` value from the response of a report previously requested via `POST` method of the

**assets** — Manage assets

- `amazon-ads-pp-cli assets get` — Retrieves an asset along with the metadata
- `amazon-ads-pp-cli assets get-upload-location` — Creates an ephemeral resource (upload location) to upload Assets to Creative Assets tool.
- `amazon-ads-pp-cli assets register` — The API should be called once the asset is uploaded to the location provided by the /asset/upload API endpoint.
- `amazon-ads-pp-cli assets search` — Search assets

**attribution** — Manage attribution

- `amazon-ads-pp-cli attribution get-advertisers-by-profile` — For sellers, an attribution profile has one associated advertiser.
- `amazon-ads-pp-cli attribution get-publisher-macro-tag` — Some third-party publishers do not support tags that include macro parameters.
- `amazon-ads-pp-cli attribution get-publisher-tag-template` — Third-party publishers, such as Google Ads, Facebook, Microsoft Ads
- `amazon-ads-pp-cli attribution get-publishers` — Use the response to determine whether to use either the macroTags or nonMacroTemplateTags resource to get tags for a
- `amazon-ads-pp-cli attribution get-tags-by-campaign` — Gets an attribution report for a specified list of advertisers.

**audiences** — Manage audiences

- `amazon-ads-pp-cli audiences fetch-taxonomy` — Returns a list of audience categories for a given category path **Requires one of these permissions**
- `amazon-ads-pp-cli audiences list` — Returns a list of audience segments for an advertiser.

**benchmarks** — Manage benchmarks

- `amazon-ads-pp-cli benchmarks get-brands` — Gets a list of brands that the advertising account has promoted in their SB campaigns **Requires one of these
- `amazon-ads-pp-cli benchmarks get-report-data` — Provides overview of metrics for all brands and categories that the entity has access to.
- `amazon-ads-pp-cli benchmarks get-time-series` — Provides time series data for the specified brand and category filtered by optional parameters **Requires one of these

**billing** — Manage billing

- `amazon-ads-pp-cli billing bulk-get-notifications` — Gets an array of all currently valid billing notifications associated for each advertising account.
- `amazon-ads-pp-cli billing bulk-get-status` — Gets the current billing status associated for each advertising account.

**brands** — Manage brands

- `amazon-ads-pp-cli brands` — Gets an array of Brand data objects for the Brand associated with the profile ID passed in the header.

**currencies** — Manage currencies

- `amazon-ads-pp-cli currencies` — Gets an array of localized currencies in their target marketplaces using advertiser identifier in header

**dp** — Manage dp

- `amazon-ads-pp-cli dp create` — Creates a new data provider audience. Note that the API call rate is limited to 1 transaction per second (TPS).
- `amazon-ads-pp-cli dp get` — Gets metadata for an audience specified by identifier.
- `amazon-ads-pp-cli dp update` — Associates or disassociates a record with an audience.
- `amazon-ads-pp-cli dp update-audiencemetadata` — Updates metadata of an existing audience specified by identifier.
- `amazon-ads-pp-cli dp update-users` — Deletes user data originally sourced from the client. The API call rate is limited to 1 transactions per second (TPS).

**dsp** — Manage dsp

- `amazon-ads-pp-cli dsp associate-line-items-to-creatives` — Create/delete association between line item and creative. Callout - Do not pass in startDate, endDate and weight.
- `amazon-ads-pp-cli dsp create-file-uploads-policy` — Create file upload policy that used to upload file to AWS S3. File upload policy will expire in 15 minutes.
- `amazon-ads-pp-cli dsp create-image-creative` — Create an image creative. Callout - A future update will add support to create multiple Image creatives at a time.
- `amazon-ads-pp-cli dsp create-line-items` — Create line item. Callout - A future update will add support for multiple at a time.
- `amazon-ads-pp-cli dsp create-orders` — Create an order. Callout - A future update will add support for multiple at a time.
- `amazon-ads-pp-cli dsp create-rec-creatives` — Create a new Responsive eCommerce Creatives(REC).
- `amazon-ads-pp-cli dsp create-third-party-creative` — Create a third party creative.
- `amazon-ads-pp-cli dsp create-video-creatives` — Create a video creative Callout - A future update will add support to create multiple Video creatives at a time.
- `amazon-ads-pp-cli dsp export-products-by-order-id` — Export conversion tracking products as a file by identifier. The file URL will expire in 15 minutes.
- `amazon-ads-pp-cli dsp get-apps` — Gets apps based on app Ids or text querys. Either one of app Ids or text query may be supplied, but not both.
- `amazon-ads-pp-cli dsp get-conversion-trackings` — Get conversion tracking information for given order.
- `amazon-ads-pp-cli dsp get-creative-moderation` — Get creative moderation summary by creativeId.
- `amazon-ads-pp-cli dsp get-creatives` — Gets one or more creatives.
- `amazon-ads-pp-cli dsp get-domain-targeting` — Gets one or more line items domain targeting information.
- `amazon-ads-pp-cli dsp get-domains` — Gets the list of domain lists for inclusion/exclusion based on entity. Lists are sorted by creation time.
- `amazon-ads-pp-cli dsp get-dv-custom-contextual-segments` — Retrieves custom contextual segments pre-bid targeting data for an account that is already linked to Double Verify.
- `amazon-ads-pp-cli dsp get-geo-locations` — Gets locationTargeting objects based on locationTargetingId or text query, such as city name, zip code
- `amazon-ads-pp-cli dsp get-goal-configurations` — Gets a list of configurations that can be applied to orders to optimize for a desired campaign goal
- `amazon-ads-pp-cli dsp get-iab-content-categories` — Gets the hierarchy of IAB content categories as a list sorted by ID in ascending order.
- `amazon-ads-pp-cli dsp get-image-creatives` — Get an image creative matching criteria provided in request.
- `amazon-ads-pp-cli dsp get-line-item` — Gets line item with complete information specified by identifier.
- `amazon-ads-pp-cli dsp get-line-items` — Gets one or more line items with basic information.
- `amazon-ads-pp-cli dsp get-odc-custom-predicts` — Retrieves custom predict pre-bid targeting data for an account that is already linked to Oracle Data Cloud.
- `amazon-ads-pp-cli dsp get-odc-standard-predicts` — Gets Oracle Data Cloud provided standard predicts for pre-bid targeting.
- `amazon-ads-pp-cli dsp get-order` — Gets an order with complete information specified by an identifier.
- `amazon-ads-pp-cli dsp get-orders` — Gets one or more orders with basic information.
- `amazon-ads-pp-cli dsp get-pixels` — Gets a list of pixels based on filters. AdvertiserIdFilter must be provided.
- `amazon-ads-pp-cli dsp get-pixels-by-order-id` — Get conversion tracking pixels by identifier.
- `amazon-ads-pp-cli dsp get-product-categories` — Gets the hierarchy of product category objects as a list sorted by ID in ascending order.
- `amazon-ads-pp-cli dsp get-products-by-order-id` — Get conversion tracking products by identifier.
- `amazon-ads-pp-cli dsp get-rec-creatives` — Get an Responsive eCommerce Creative (REC) matching criteria provided in request.
- `amazon-ads-pp-cli dsp get-supply-sources` — Gets the supply sources based on line item type, advertiser and supply source type.
- `amazon-ads-pp-cli dsp get-third-party-creatives` — Get a third party creative matching criteria provided in request.
- `amazon-ads-pp-cli dsp get-video-creatives` — Get a video creative matching criteria provided in request.
- `amazon-ads-pp-cli dsp list-line-item-creative-associations` — Gets an array of creative associations, filtered by specified criteria.
- `amazon-ads-pp-cli dsp preview-image-creative` — Preview an image creative.
- `amazon-ads-pp-cli dsp preview-rec-creative` — Preview a Responsive eCommerce Creative(REC).
- `amazon-ads-pp-cli dsp preview-third-party-creative` — Preview a third party creative.
- `amazon-ads-pp-cli dsp preview-video-creative` — Preview a video creative
- `amazon-ads-pp-cli dsp set-line-item-status` — Setting delivery activation status for the given line item id. Version 2.x line items accept `application/vnd.
- `amazon-ads-pp-cli dsp set-order-status` — Setting delivery activation status for the given order id.
- `amazon-ads-pp-cli dsp update` — Add or remove conversion tracking products from the order.
- `amazon-ads-pp-cli dsp update-conversion-tracking` — Add or remove conversion tracking information from the order.
- `amazon-ads-pp-cli dsp update-domain-targeting` — Replaces the DomainTargeting for the specified line items with the ones provided in the request body.
- `amazon-ads-pp-cli dsp update-image-creative` — Update an image creative. Callout - A future update will add support to update multiple Image creatives at a time.
- `amazon-ads-pp-cli dsp update-line-item-creative-associations` — Update association details. This API will be used to update startDate, endDate and weight parameters.
- `amazon-ads-pp-cli dsp update-line-items` — This is a full update, not partial patch.
- `amazon-ads-pp-cli dsp update-orders` — This is a full update, not partial patch.
- `amazon-ads-pp-cli dsp update-pixels-by-order-id` — Add or remove conversion tracking pixels from the order. The maximum size of pixel list is 100.
- `amazon-ads-pp-cli dsp update-rec-creatives` — Update existing Responsive eCommerce Creatives(REC). Callout - A future update will add support for multiple at a time.
- `amazon-ads-pp-cli dsp update-third-party-creative` — Update a third party creative.
- `amazon-ads-pp-cli dsp update-video-creatives` — Update a video creative Callout - A future update will add support to update multiple Video creatives at a time.

**dsp-reports-dsp** — Manage dsp reports dsp

- `amazon-ads-pp-cli dsp-reports-dsp create-report-v3` — Use this operation to request creation of a report that includes metrics about your Amazon DSP campaigns.
- `amazon-ads-pp-cli dsp-reports-dsp get-campaign-report-v3` — Pass the identifier of a previously requested report in the `reportId` field to get the current status of the report.

**eligibility** — Manage eligibility

- `amazon-ads-pp-cli eligibility product` — Gets a list of advertising eligibility objects for a set of products.
- `amazon-ads-pp-cli eligibility program` — Checks the advertiser's eligibility to ad programs.

**history** — Manage history

- `amazon-ads-pp-cli history` — Returns history of changes for provided event sources that match the filters and time ranges specified.

**hsa** — Manage hsa


**insights** — Manage insights

- `amazon-ads-pp-cli insights generate-brand-metrics-report` — Generates the Brand Metrics report in CSV or JSON format.
- `amazon-ads-pp-cli insights get-brand-metrics-report` — Fetch the location and status of the report for the brands for which the metrics are available.

**invoices** — Manage invoices

- `amazon-ads-pp-cli invoices get` — **Requires one of these permissions**: ['nemo_transactions_view','nemo_transactions_edit']
- `amazon-ads-pp-cli invoices get-advertiser` — **Requires one of these permissions**: ['nemo_transactions_view','nemo_transactions_edit']

**keywords** — Manage keywords

- `amazon-ads-pp-cli keywords` — Returns localized keywords within specified marketplaces or locales.

**manager-accounts** — Manage manager accounts

- `amazon-ads-pp-cli manager-accounts create` — Creates a new Amazon Advertising [Manager account](https://advertising.amazon.com/help?
- `amazon-ads-pp-cli manager-accounts get-for-user` — Returns all [Manager accounts](https://advertising.amazon.com/help?

**measurement** — Manage measurement

- `amazon-ads-pp-cli measurement cancel-studies` — Cancel existing studies. Once a study is cancelled it can not be resumed again.
- `amazon-ads-pp-cli measurement check-planning-eligibility` — Checks eligibility against all vendor products. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement create-surveys` — Create new study surveys. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement get-dspbrand-lift-study-result` — Get result of a DSP BRAND_LIFT study. Returns 200 successful response if json resource is requested in Accept header.
- `amazon-ads-pp-cli measurement get-studies` — Gets base study objects given a list of studyIds or a list of advertiserIds. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement get-surveys` — Gets one or more study surveys with requested survey identifiers or a study identifier.
- `amazon-ads-pp-cli measurement omnichannel-metrics-brand-search` — Search for brands to be used in the OMNICHANNEL_METRICS vendor product. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement update-surveys` — Update measurement surveys. This will be a full update. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement vendor-product` — Lists the supported measurement vendors products. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement vendor-product-policy` — Gets the policies for the specific vendor product(s). **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement vendor-product-survey-question-templates` — Gets the survey question templates for the specific vendor product(s). **Requires one of these permissions**: []

**measurement-dsp** — Manage measurement dsp

- `amazon-ads-pp-cli measurement-dsp check-dspaudience-research-eligibility` — Checks the DSP AUDIENCE_RESEARCH study type eligibility status against vendor products.
- `amazon-ads-pp-cli measurement-dsp check-dspbrand-lift-eligibility` — Checks the DSP BRAND_LIFT study type eligibility status against vendor products.
- `amazon-ads-pp-cli measurement-dsp check-dspcreative-testing-eligibility` — Checks the DSP CREATIVE_TESTING study type eligibility status against vendor products.
- `amazon-ads-pp-cli measurement-dsp check-dspomnichannel-metrics-eligibility` — Checks the DSP OMNICHANNEL_METRICS study type eligibility status against vendor products.
- `amazon-ads-pp-cli measurement-dsp create-dspaudience-research-study` — Create new DSP AUDIENCE_RESEARCH study. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp create-dspbrand-lift-studies` — Create new DSP BRAND_LIFT studies. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp create-dspcreative-testing-study` — Create new DSP CREATIVE_TESTING study. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp create-dspomnichannel-metrics-studies` — Create new DSP OMNICHANNEL_METRICS studies. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp get-dspaudience-research-studies` — Gets one or more DSP AUDIENCE_RESEARCH studies with requested study identifiers or an advertiser identifier.
- `amazon-ads-pp-cli measurement-dsp get-dspaudience-research-study-result` — Get result of a DSP AUDIENCE_RESEARCH study.
- `amazon-ads-pp-cli measurement-dsp get-dspbrand-lift-studies` — Gets one or more DSP BRAND_LIFT studies with requested study identifiers or an advertiser identifier.
- `amazon-ads-pp-cli measurement-dsp get-dspcreative-testing-studies` — Gets one or more DSP CREATIVE_TESTING studies with requested study identifiers or an advertiser identifier.
- `amazon-ads-pp-cli measurement-dsp get-dspcreative-testing-study-result` — Get result of a DSP CREATIVE_TESTING study.
- `amazon-ads-pp-cli measurement-dsp get-dspomnichannel-metrics-studies` — Gets one or more DSP OMNICHANNEL_METRICS studies with requested study identifiers or an advertiser identifier.
- `amazon-ads-pp-cli measurement-dsp get-dspomnichannel-metrics-study-result` — Get result of a DSP OMNICHANNEL_METRICS study.
- `amazon-ads-pp-cli measurement-dsp update-dspaudience-research-study` — Update DSP AUDIENCE_RESEARCH study. This will be a full update. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp update-dspbrand-lift-studies` — Update DSP BRAND_LIFT studies. This will be a full update. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp update-dspcreative-testing-study` — Update DSP CREATIVE_TESTING study. This will be a full update. **Requires one of these permissions**: []
- `amazon-ads-pp-cli measurement-dsp update-dspomnichannel-metrics-studies` — Update DSP OMNICHANNEL_METRICS studies. This will be a full update. **Requires one of these permissions**: []

**media** — Manage media

- `amazon-ads-pp-cli media complete-upload` — The API should be called once the media is uploaded to the location provided by the /media/upload API endpoint.
- `amazon-ads-pp-cli media create-upload-resource` — Creates an ephemeral resource (upload location) to upload Media for an Ad Program.
- `amazon-ads-pp-cli media describe` — API to poll for media status.

**overlapping-audiences** — Manage overlapping audiences

- `amazon-ads-pp-cli overlapping-audiences <audienceId>` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']

**page-asins** — Manage page asins

- `amazon-ads-pp-cli page-asins` — Note that for sellers, the addresss must be a Store page. Vendors may also specify a custom landing page address.

**pre-moderation** — Manage pre moderation

- `amazon-ads-pp-cli pre-moderation` — This API will be accepting different components of the ad/page and will be automatically validating the components and

**product** — Manage product

- `amazon-ads-pp-cli product` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']

**products** — Manage products

- `amazon-ads-pp-cli products` — Localizes (maps) products from a source marketplace to one or more target marketplaces.

**profiles** — List Amazon Ads profiles available to the authorized application.

- `amazon-ads-pp-cli profiles` — List advertising profiles for the current Login with Amazon credentials.

**reports** — Manage reports

- `amazon-ads-pp-cli reports <reportId>` — To understand the call flow for asynchronous reports

**sb** — Manage sb

- `amazon-ads-pp-cli sb archive-campaign` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sb archive-keyword` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sb archive-negative-keyword` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sb archive-negative-target` — Archives a negative target specified by identifier.
- `amazon-ads-pp-cli sb archive-target` — Archives a target specified by identifier.
- `amazon-ads-pp-cli sb create-campaigns` — **Note:** To create multi-ad group campaigns use the [version 4 POST campaigns](https://advertising.amazon.
- `amazon-ads-pp-cli sb create-draft-campaigns` — Creates sponsored brands draft campaigns. **To create a video campaign specify adFormat as 'video'.
- `amazon-ads-pp-cli sb create-keywords` — Note that `state` can't be set at keyword creation.
- `amazon-ads-pp-cli sb create-negative-keywords` — Note that `bid` and `state` can't be set at negative keyword creation.
- `amazon-ads-pp-cli sb create-negative-targets` — Create one or more negative targets.
- `amazon-ads-pp-cli sb create-targets` — Create one or more targets.
- `amazon-ads-pp-cli sb delete-draft-campaign` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sb get` — Note that this resource is only available for campaigns in the US marketplace.
- `amazon-ads-pp-cli sb get-ad-group` — Gets an ad group specified by identifier.
- `amazon-ads-pp-cli sb get-bids-recommendations` — Get a list of bid recommendation objects for a specified list of keywords or products.
- `amazon-ads-pp-cli sb get-brand-recommendations` — The Brand suggestions are based on a list of either category identifiers or keywords passed in the request.
- `amazon-ads-pp-cli sb get-campaign` — Gets a campaign specified by identifier.
- `amazon-ads-pp-cli sb get-draft-campaign` — Gets a draft campaign specified by identifier.
- `amazon-ads-pp-cli sb get-keyword` — Gets a keyword specified by identifier.
- `amazon-ads-pp-cli sb get-negative-keyword` — Gets a negative keyword specified by identifier.
- `amazon-ads-pp-cli sb get-negative-target` — Gets a negative target specified by identifier.
- `amazon-ads-pp-cli sb get-product-recommendations` — Recommendations are based on the ASINs that are passed in the request.
- `amazon-ads-pp-cli sb get-target` — Gets a target specified by identifier.
- `amazon-ads-pp-cli sb get-targeting-categories` — Recommendations are based on the ASINs that are passed in the request.
- `amazon-ads-pp-cli sb list-ad-groups` — Gets an array of ad groups associated with the client identifier passed in the authorization header
- `amazon-ads-pp-cli sb list-campaigns` — **Note**: To ensure you are getting all campaign data, use the [version 4 list campaigns endpoint](https://advertising.
- `amazon-ads-pp-cli sb list-draft-campaigns` — Gets an array of all draft campaigns associated with the client identifier passed in the authorization header
- `amazon-ads-pp-cli sb list-keywords` — Gets an array of keywords, filtered by optional criteria.
- `amazon-ads-pp-cli sb list-negative-keywords` — Gets an array of negative keywords, filtered by optional criteria.
- `amazon-ads-pp-cli sb list-negative-targets` — Gets a list of product negative targets associated with the client identifier passed in the authorization header
- `amazon-ads-pp-cli sb list-targets` — Gets a list of product targets associated with the client identifier passed in the authorization header
- `amazon-ads-pp-cli sb submit-draft-campaign` — On successful submission
- `amazon-ads-pp-cli sb update-campaigns` — Mutable fields
- `amazon-ads-pp-cli sb update-draft-campaigns` — Updates one or more draft campaigns.
- `amazon-ads-pp-cli sb update-keywords` — Keywords submitted for update may have state set to `pending` for moderation review. Moderation may take up to 72 hours.
- `amazon-ads-pp-cli sb update-negative-keywords` — Negative keywords submitted for update may have state set to `pending` for moderation review.
- `amazon-ads-pp-cli sb update-negative-targets` — Updates one or more negative targets.
- `amazon-ads-pp-cli sb update-targets` — Updates one or more targets.

**sd** — Manage sd

- `amazon-ads-pp-cli sd archive-ad-group` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sd archive-campaign` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sd archive-negative-targeting-clause` — Equivalent to using the updateNegativeTargetingClauses operation to set the `state` property of a targeting clause to
- `amazon-ads-pp-cli sd archive-product-ad` — This operation is equivalent to an update operation that sets the status field to 'archived'.
- `amazon-ads-pp-cli sd archive-targeting-clause` — Equivalent to using the `updateTargetingClauses` operation to set the `state` property of a targeting clause to
- `amazon-ads-pp-cli sd associate-optimization-rules-with-ad-group` — Associate one or more optimization rules to an ad group specified by identifier.
- `amazon-ads-pp-cli sd create-ad-groups` — Creates one or more ad groups.
- `amazon-ads-pp-cli sd create-brand-safety-deny-list-domains` — Creates one or more domains to add to a Brand Safety Deny List. The Brand Safety Deny List is at the advertiser level.
- `amazon-ads-pp-cli sd create-campaigns` — Creates one or more campaigns.
- `amazon-ads-pp-cli sd create-creatives` — A POST request of one or more creatives.
- `amazon-ads-pp-cli sd create-negative-targeting-clauses` — Successfully created negative targeting clauses associated with an ad group are assigned a unique target identifier.
- `amazon-ads-pp-cli sd create-optimization-rules` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd create-product-ads` — Creates one or more product ads.
- `amazon-ads-pp-cli sd create-sdforecast` — Returns forecasts for a given ad group specified in SD forecast request.
- `amazon-ads-pp-cli sd create-targeting-clauses` — Successfully created targeting clauses are assigned a unique `targetId` value.
- `amazon-ads-pp-cli sd delete-brand-safety-deny-list` — Archives all of the domains in the Brand Safety Deny List.
- `amazon-ads-pp-cli sd disassociate-optimization-rule` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd download-snapshot` — **To understand the call flow for asynchronous snapshots
- `amazon-ads-pp-cli sd get` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd get-ad-group` — Returns an AdGroup object for a requested campaign.
- `amazon-ads-pp-cli sd get-ad-group-response-ex` — Gets extended information for a requested ad group.
- `amazon-ads-pp-cli sd get-adgroups` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd get-campaign` — Returns a Campaign object for a requested campaign.
- `amazon-ads-pp-cli sd get-campaign-response-ex` — Returns a CampaignResponseEx object for a requested campaign.
- `amazon-ads-pp-cli sd get-negative-targets` — This call returns the minimal set of negative targeting clause fields, but is more efficient than getNegativeTargetsEx.
- `amazon-ads-pp-cli sd get-negative-targets-ex` — Gets a negative targeting clause with extended fields.
- `amazon-ads-pp-cli sd get-product-ad` — Note that the ProductAd object is designed for performance
- `amazon-ads-pp-cli sd get-product-ad-response-ex` — Gets extended information for a product ad.
- `amazon-ads-pp-cli sd get-request-results` — When a user adds domains to their Brand Safety Deny List, the request is processed asynchronously
- `amazon-ads-pp-cli sd get-request-status` — When a user modifies their Brand Safety Deny List, the request is processed asynchronously
- `amazon-ads-pp-cli sd get-snapshot` — **To understand the call flow for asynchronous snapshots
- `amazon-ads-pp-cli sd get-target-bid-recommendations` — Provides a list of bid recommendations based on the list of input advertised ASINs and targeting clauses in the same
- `amazon-ads-pp-cli sd get-target-recommendations` — This API provides product and category recommendations to target based on the list of input ASINs.
- `amazon-ads-pp-cli sd get-targets` — This call returns the minimal set of targeting clause fields.
- `amazon-ads-pp-cli sd get-targets-ex` — Gets a targeting clause object with extended fields.
- `amazon-ads-pp-cli sd list-ad-groups` — Gets an array of AdGroup objects for a requested set of Sponsored Display ad groups.
- `amazon-ads-pp-cli sd list-ad-groups-ex` — Gets an array of AdGroupResponseEx objects for a set of requested ad groups.
- `amazon-ads-pp-cli sd list-campaigns` — Gets an array of Campaign objects for a requested set of Sponsored Display campaigns.
- `amazon-ads-pp-cli sd list-campaigns-ex` — Gets an array of CampaignResponseEx objects for a set of requested campaigns.
- `amazon-ads-pp-cli sd list-creative-moderations` — Gets a list of creative moderations
- `amazon-ads-pp-cli sd list-creatives` — Gets a list of creatives
- `amazon-ads-pp-cli sd list-domains` — Gets an array of websites/apps that are on the advertiser's Brand Safety Deny List.
- `amazon-ads-pp-cli sd list-negative-targeting-clauses` — Gets a list of negative targeting clauses objects for a requested set of Sponsored Display negative targets.
- `amazon-ads-pp-cli sd list-negative-targeting-clauses-ex` — Gets an array of NegativeTargetingClauseEx objects for a set of requested negative targets.
- `amazon-ads-pp-cli sd list-optimization-rules` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd list-product-ads` — Gets an array of ProductAd objects for a requested set of Sponsored Display product ads.
- `amazon-ads-pp-cli sd list-product-ads-ex` — Gets an array of ProductAdResponseEx objects for a set of requested ad groups.
- `amazon-ads-pp-cli sd list-request-status` — List status of all Brand Safety List requests. The list will contain requests that were submitted in the past 90 days.
- `amazon-ads-pp-cli sd list-targeting-clauses` — Gets a list of targeting clauses objects for a requested set of Sponsored Display targets.
- `amazon-ads-pp-cli sd list-targeting-clauses-ex` — Gets an array of TargetingClauseEx objects for a set of requested targets.
- `amazon-ads-pp-cli sd post-creative-preview` — Gets creative preview HTML.
- `amazon-ads-pp-cli sd update-ad-groups` — Updates on or more ad groups.
- `amazon-ads-pp-cli sd update-campaigns` — Updates one or more campaigns.
- `amazon-ads-pp-cli sd update-creatives` — Updates one or more creatives.
- `amazon-ads-pp-cli sd update-negative-targeting-clauses` — Updates one or more negative targeting clauses. Negative targeting clauses are identified using their targetId.
- `amazon-ads-pp-cli sd update-optimization-rules` — This operation is a PREVIEW ONLY. This note will be removed once this functionality becomes available.
- `amazon-ads-pp-cli sd update-product-ads` — Updates one or more product ads.
- `amazon-ads-pp-cli sd update-targeting-clauses` — Updates one or more targeting clauses. Targeting clauses are identified using their targetId.

**sp** — Manage sp

- `amazon-ads-pp-cli sp archive-ad-group` — Sets the ad group status to `archived`. Archived entities cannot be made active again.
- `amazon-ads-pp-cli sp archive-campaign` — Sets the campaign status to `archived`. Archived entities cannot be made active again.
- `amazon-ads-pp-cli sp archive-campaign-negative-keyword` — Set the status of the specified campaign negative keyword to `archived`.
- `amazon-ads-pp-cli sp archive-keyword` — Set the status of the specified keyword to `archived`.
- `amazon-ads-pp-cli sp archive-negative-keyword` — Set the status of the specified negative keyword to `archived`.
- `amazon-ads-pp-cli sp archive-negative-targeting-clause` — Set the `status` of a negative targeting clause to `archived`.
- `amazon-ads-pp-cli sp archive-product-ad` — Sets the state of a specified product ad to `archived`.
- `amazon-ads-pp-cli sp archive-targeting-clause` — Set the `status` of a targeting clause to `archived`.
- `amazon-ads-pp-cli sp bulk-get-asin-suggested-keywords` — Suggested keywords are returned in an array ordered by descending effectiveness.
- `amazon-ads-pp-cli sp create-ad-groups` — Creates one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-campaign-negative-keywords` — Creates one or more campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-campaigns` — Creates one or more campaigns. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-keyword-bid-recommendations` — **Deprecation notice: This endpoint will be deprecated on December 31, 2022.
- `amazon-ads-pp-cli sp create-keywords` — Creates one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-negative-keywords` — Creates one or more negative keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-negative-targeting-clauses` — Creates one ore more negative targeting expressions. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-product-ads` — Creates one or more product ads. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp create-target-recommendations` — **Deprecation notice: This endpoint will be deprecated on February 28, 2023.
- `amazon-ads-pp-cli sp create-targeting-clauses` — Creates one or more targeting expressions. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp download-snapshot` — **To understand the call flow for asynchronous snapshots
- `amazon-ads-pp-cli sp get-ad-group` — Gets an ad group specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-ad-group-bid-recommendations` — **Deprecation notice: This endpoint will be deprecated on December 31, 2022.
- `amazon-ads-pp-cli sp get-ad-group-ex` — Gets an ad group that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-ad-group-suggested-keywords` — Gets suggested keywords for the specified ad group.
- `amazon-ads-pp-cli sp get-ad-group-suggested-keywords-ex` — Gets suggested keywords with extended data for the specified ad group.
- `amazon-ads-pp-cli sp get-ad-groups` — Gets one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-ad-groups-ex` — Gets ad groups that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-asin-suggested-keywords` — Suggested keywords are returned in an array ordered by descending effectiveness.
- `amazon-ads-pp-cli sp get-bid-recommendations` — Gets a list of bid recommendations for keyword, product, or auto targeting expressions.
- `amazon-ads-pp-cli sp get-campaign` — Gets a campaign specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-campaign-ex` — Gets a campaign with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-campaign-negative-keyword` — Gets a campaign negative keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-campaign-negative-keyword-ex` — Gets a campaign negative keyword that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-keyword` — Gets a keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-keyword-bid-recommendations` — **Deprecation notice: This endpoint will be deprecated on December 31, 2022.
- `amazon-ads-pp-cli sp get-keyword-ex` — Gets a keyword with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-negative-keyword` — Gets a negative keyword specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-negative-keyword-ex` — Gets a negative keyword that has extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-negative-targeting-clause` — Get a negative targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-negative-targeting-clause-ex` — Get a negative targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-product-ad` — Gets a product ad specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-product-ad-ex` — Gets extended data for a product ad specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-snapshot-status` — **To understand the call flow for asynchronous snapshots
- `amazon-ads-pp-cli sp get-targeting-clause` — Get a targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp get-targeting-clause-ex` — Get a targeting clause specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-campaign-negative-keywords` — Gets a list of campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-campaign-negative-keywords-ex` — Gets a list of campaign negative keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-campaigns` — Gets an array of campaigns. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-campaigns-ex` — Gets an array of campaigns with extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-keywords` — Gets one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-keywords-ex` — Gets a list of keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-negative-keywords` — Gets a list of negative keyword objects. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-negative-keywords-ex` — Gets a list of negative keywords that have extended data fields. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-negative-targeting-clauses` — Gets a list of negative targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-negative-targeting-clauses-ex` — Gets a list of negative targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-product-ads` — Gets a list of product ads filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-product-ads-ex` — Gets extended data for a list of product ads filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-targeting-clauses` — Gets a list of targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp list-targeting-clauses-ex` — Gets a list of targeting clauses filtered by specified criteria. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-ad-groups` — Updates one or more ad groups. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-campaign-negative-keywords` — Updates one or more campaign negative keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-campaigns` — Updates one or more campaigns. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-keywords` — Updates one or more keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-negative-keywords` — Updates one or more negative keywords. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-negative-targeting-clause` — Updates one or more negative targeting clauses. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-product-ads` — Updates one or more product ads specified by identifier. [PLANNED DEPRECATION 6/30/2023]
- `amazon-ads-pp-cli sp update-targeting-clause` — Updates one or more targeting clauses. [PLANNED DEPRECATION 6/30/2023]

**sponsored-brands-sb** — Manage sponsored brands sb

- `amazon-ads-pp-cli sponsored-brands-sb campaigns-budget-usage` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-brands-sb create-associated-budget-rules-for-sbcampaigns` — A maximum of 250 rules can be associated to a campaign.
- `amazon-ads-pp-cli sponsored-brands-sb create-brand-video-creative` — This API creates a new version of an existing creative for given [Sponsored Brands Brand Video Ad](https
- `amazon-ads-pp-cli sponsored-brands-sb create-budget-rules-for-sbcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-product-collection-creative` — This API creates a new version of creative for given Sponsored Brands ad by supplying product collection creative
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brand-store-spotlight-ads` — Creates Sponsored Brands store spotlight ads. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-ad-groups` — Creates Sponsored Brands ad groups. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-brand-video-ads` — Creates Sponsored Brands brand video ads. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-campaigns` — Creates Sponsored Brands campaigns. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-product-collection-ads` — Creates Sponsored Brands product collection ads. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-sponsored-brands-video-ads` — Creates Sponsored Brands video ads. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb create-store-spotlight-creative` — This API creates a new version of creative for given Sponsored Brands ad by supplying store spotlight creative content
- `amazon-ads-pp-cli sponsored-brands-sb create-video-creative` — This API creates a new version of an existing creative for given Sponsored Brands ad by supplying video creative
- `amazon-ads-pp-cli sponsored-brands-sb delete-sponsored-brands-ad-groups` — Deletes Sponsored Brands ad groups. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb delete-sponsored-brands-ads` — Deletes Sponsored Brands ads. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb disassociate-associated-budget-rule-for-sbcampaigns` — Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- `amazon-ads-pp-cli sponsored-brands-sb get-budget-recommendations` — Provides daily budget recomemndations for a list of requested Sponsored Brands campaigns
- `amazon-ads-pp-cli sponsored-brands-sb get-budget-rule-by-rule-id-for-sbcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-brands-sb get-campaign-shopper-segment-forecast` — Gets shopper segment bidding campaign performance forecasts.
- `amazon-ads-pp-cli sponsored-brands-sb get-campaigns-associated-with-sbbudget-rule` — Gets all the campaigns associated with a budget rule
- `amazon-ads-pp-cli sponsored-brands-sb get-headline-recommendations` — API to receive creative headline suggestions.
- `amazon-ads-pp-cli sponsored-brands-sb get-keyword-recommendations` — Gets an array of keyword recommendation objects for a set of ASINs included either on a landing page or a Stores page.
- `amazon-ads-pp-cli sponsored-brands-sb get-rule-based-budget-history-for-sbcampaigns` — The budget history is returned for the time period specified in the required startDate and endDate parameters.
- `amazon-ads-pp-cli sponsored-brands-sb get-sbbudget-rules-for-advertiser` — Get all budget rules created by an advertiser
- `amazon-ads-pp-cli sponsored-brands-sb list-associated-budget-rules-for-sbcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-brands-sb list-creatives` — This API gets an array of all Sponsored Brands creatives that qualify the given resource identifiers and filters
- `amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-ad-groups` — Lists Sponsored Brands ad groups.
- `amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-ads` — Lists Sponsored Brands ads.
- `amazon-ads-pp-cli sponsored-brands-sb list-sponsored-brands-campaigns` — Lists Sponsored Brands campaigns.
- `amazon-ads-pp-cli sponsored-brands-sb sbget-budget-rules-recommendation` — A rule enables an automatic budget increase for a specified date range or for a special event.
- `amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-negative-brands` — Returns brands recommended for negative targeting. Only available for Sellers and Vendors.
- `amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-refinements-for-category` — Returns refinements according to category input.
- `amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-targetable-asincounts` — Get number of targetable asins based on refinements provided by the user.
- `amazon-ads-pp-cli sponsored-brands-sb sbtargeting-get-targetable-categories` — Returns all targetable categories by default in a list.
- `amazon-ads-pp-cli sponsored-brands-sb update-budget-rules-for-sbcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb update-sponsored-brands-ad-groups` — Updates Sponsored Brands ad groups. **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-brands-sb update-sponsored-brands-ads` — Updates Sponsored Brands ads. **Requires one of these permissions**: ['advertiser_campaign_edit']

**sponsored-display-sd** — Manage sponsored display sd

- `amazon-ads-pp-cli sponsored-display-sd campaigns-budget-usage` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-display-sd create-associated-budget-rules-for-sdcampaigns` — A maximum of 250 rules can be associated to a campaign.
- `amazon-ads-pp-cli sponsored-display-sd create-brand-safety-deny-list-domains` — Creates one or more domains to add to a Brand Safety Deny List. The Brand Safety Deny List is at the advertiser level.
- `amazon-ads-pp-cli sponsored-display-sd create-budget-rules-for-sdcampaigns` — Creates one or more budget rules.
- `amazon-ads-pp-cli sponsored-display-sd delete-brand-safety-deny-list` — Archives all of the domains in the Brand Safety Deny List.
- `amazon-ads-pp-cli sponsored-display-sd disassociate-associated-budget-rule-for-sdcampaigns` — Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- `amazon-ads-pp-cli sponsored-display-sd download-snapshot-by-id` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-display-sd get-budget-rule-by-rule-id-for-sdcampaigns` — Gets a budget rule specified by identifier.
- `amazon-ads-pp-cli sponsored-display-sd get-campaigns-associated-with-sdbudget-rule` — Gets all the campaigns associated with a budget rule
- `amazon-ads-pp-cli sponsored-display-sd get-headline-recommendations-for` — You can use this Sponsored Display API to retrieve creative headline recommendations from an array of ASINs.
- `amazon-ads-pp-cli sponsored-display-sd get-request-results` — When a user adds domains to their Brand Safety Deny List, the request is processed asynchronously
- `amazon-ads-pp-cli sponsored-display-sd get-request-status` — When a user modifies their Brand Safety Deny List, the request is processed asynchronously
- `amazon-ads-pp-cli sponsored-display-sd get-rule-based-budget-history-for-sdcampaigns` — The budget history is returned for the time period specified in the required startDate and endDate parameters.
- `amazon-ads-pp-cli sponsored-display-sd get-sdbudget-rules-for-advertiser` — Get all budget rules created by an advertiser
- `amazon-ads-pp-cli sponsored-display-sd get-snapshot-by-id` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-display-sd get-target-bid-recommendations` — Provides a list of bid recommendations based on the list of input advertised ASINs and targeting clauses in the same
- `amazon-ads-pp-cli sponsored-display-sd get-target-recommendations` — Provides a list of products to target based on the list of input ASINs.
- `amazon-ads-pp-cli sponsored-display-sd list-associated-budget-rules-for-sdcampaigns` — Gets a list of budget rules associated to a campaign specified by identifier.
- `amazon-ads-pp-cli sponsored-display-sd list-domains` — Gets an array of websites/apps that are on the advertiser's Brand Safety Deny List.
- `amazon-ads-pp-cli sponsored-display-sd list-request-status` — List status of all Brand Safety List requests. The list will contain requests that were submitted in the past 90 days.
- `amazon-ads-pp-cli sponsored-display-sd update-budget-rules-for-sdcampaigns` — Update one or more budget rules.

**sponsored-products-sp** — Manage sponsored products sp

- `amazon-ads-pp-cli sponsored-products-sp campaigns-budget-usage` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp create-associated-budget-rules-for-spcampaigns` — A maximum of 250 rules can be associated to a campaign.
- `amazon-ads-pp-cli sponsored-products-sp create-budget-rules-for-spcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-optimization-rule` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-ad-groups` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaign-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaign-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-campaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-product-ads` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp create-sponsored-products-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-campaign-optimization-rule` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-ad-groups` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaign-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaign-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-campaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-product-ads` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp delete-sponsored-products-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp disassociate-associated-budget-rule-for-spcampaigns` — Disassociates a budget rule specified by identifier from a campaign specified by identifier.
- `amazon-ads-pp-cli sponsored-products-sp get-budget-recommendation` — Creates daily budget recommendation along with benchmark metrics when creating a new campaign.
- `amazon-ads-pp-cli sponsored-products-sp get-budget-recommendations` — Given a list of campaigns as input, this API provides the following metrics - <b>1.
- `amazon-ads-pp-cli sponsored-products-sp get-budget-rule-by-rule-id-for-spcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp get-campaign-optimization-rule` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp get-campaign-recommendations` — Gets the top consolidated recommendations across bid, budget, targeting for SP campaigns given an advertiser profile id.
- `amazon-ads-pp-cli sponsored-products-sp get-campaigns-associated-with-spbudget-rule` — Gets all the campaigns associated with a budget rule
- `amazon-ads-pp-cli sponsored-products-sp get-category-recommendations-for-asins` — Returns a list of category recommendations for the input list of ASINs.
- `amazon-ads-pp-cli sponsored-products-sp get-negative-brands` — Returns brands recommended for negative targeting. Only available for Sellers and Vendors.
- `amazon-ads-pp-cli sponsored-products-sp get-optimization-rule-eligibility` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp get-product-recommendations` — Given an advertised ASIN as input, this API returns suggested ASINs to target in a product targeting campaign.
- `amazon-ads-pp-cli sponsored-products-sp get-ranked-keyword-recommendation` — The <b> POST /sp/targets/keywords/recommendations </b> endpoint returns recommended keyword targets given either A)
- `amazon-ads-pp-cli sponsored-products-sp get-refinements-for-category` — Returns refinements according to category input.
- `amazon-ads-pp-cli sponsored-products-sp get-rule-based-budget-history-for-spcampaigns` — The budget history is returned for the time period specified in the required startDate and endDate parameters.
- `amazon-ads-pp-cli sponsored-products-sp get-rule-notification` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp get-spbudget-rules-for-advertiser` — Get all budget rules created by an advertiser
- `amazon-ads-pp-cli sponsored-products-sp get-targetable-asincounts` — Get number of targetable asins based on refinements provided by the user.
- `amazon-ads-pp-cli sponsored-products-sp get-targetable-categories` — Returns all targetable categories. This API returns a large JSON string containing a tree of category nodes.
- `amazon-ads-pp-cli sponsored-products-sp get-theme-based-bid-recommendation-for-ad-group-v1` — The current version of the theme-based bid recommendation service supports auto-targeting and keyword targeting
- `amazon-ads-pp-cli sponsored-products-sp list-associated-budget-rules-for-spcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-ad-groups` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaign-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaign-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-campaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-product-ads` — **Requires one of these permissions**: ['advertiser_campaign_edit','advertiser_campaign_view']
- `amazon-ads-pp-cli sponsored-products-sp list-sponsored-products-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp search-brands` — Returns up to 100 brands related to keyword input for negative targeting.
- `amazon-ads-pp-cli sponsored-products-sp spget-budget-rules-recommendation` — A rule enables an automatic budget increase for a specified date range or for a special event.
- `amazon-ads-pp-cli sponsored-products-sp update-budget-rules-for-spcampaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-optimization-rule` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-ad-groups` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaign-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaign-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-campaigns` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-negative-keywords` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-negative-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-product-ads` — **Requires one of these permissions**: ['advertiser_campaign_edit']
- `amazon-ads-pp-cli sponsored-products-sp update-sponsored-products-targeting-clauses` — **Requires one of these permissions**: ['advertiser_campaign_edit']

**stores** — (Not available for video campaigns)

- `amazon-ads-pp-cli stores create-asset` — Image assets are stored in the Store Assets Library.
- `amazon-ads-pp-cli stores list-assets` — For sellers or vendors, gets an array of assets associated with the specified brand entity identifier.

**targeting-expression** — Manage targeting expression

- `amazon-ads-pp-cli targeting-expression` — Localizes (maps) targeting expressions from a source marketplace to one or more target marketplaces.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
amazon-ads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses Login with Amazon OAuth2 with refresh-token rotation. Register this exact Allowed Return URL / Redirect URI in the Amazon developer app:

```text
http://localhost:8085/callback
```

Configure local credentials in `~/.config/amazon-ads-pp-cli/.env`:

```bash
mkdir -p ~/.config/amazon-ads-pp-cli
chmod 700 ~/.config/amazon-ads-pp-cli
$EDITOR ~/.config/amazon-ads-pp-cli/.env
```

Use:

```bash
AMAZON_ADS_CLIENT_ID=your-client-id
AMAZON_ADS_CLIENT_SECRET=your-client-secret
AMAZON_ADS_REFRESH_TOKEN=
AMAZON_ADS_PROFILE_ID=
```

Then run `amazon-ads-pp-cli auth login --port 8085` to authorize `advertising::campaign_management` and save the refresh token. The login flow also fetches advertising profiles and selects the only profile automatically, or prompts when multiple profiles are available. If you already have a refresh token, paste it into `.env` instead. Access tokens are refreshed automatically before API calls.

Run `amazon-ads-pp-cli doctor` to verify setup.

List or change the persisted Amazon Ads profile scope before running scoped endpoints:

```bash
amazon-ads-pp-cli profiles list --json
amazon-ads-pp-cli profiles select <profile-id>
amazon-ads-pp-cli profiles current --json
```

Novel seller analytics commands operate on exported Amazon Ads reports and optional COGS TOML.
`dayparting-analysis` and `budget-pacing` require a report export with `hour`, `hourOfDay`, or timestamped rows; the pinned DSP reporting schema exposes `SUMMARY`/`DAILY` time units, while Amazon Ads Insights timing data carries `hourOfDay`.

```bash
amazon-ads-pp-cli break-even-acos --price 32.99 --cogs 8.50 --fees 30 --json
amazon-ads-pp-cli true-profit --price 32.99 --cogs 8.50 --fees 30 --ad-spend 4.20 --json
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

Automation commands print plans by default. Use `--apply` only when the report includes the Amazon IDs required for mutation: `campaign_id` and `ad_group_id` for search-term automation, `campaign_id` for budget updates, and `keyword_id` for bid-rule updates. Apply mode de-duplicates repeated remote mutation keys before sending, is capped by `--max-changes`, bid mutations are capped by `--max-bid`, and budget rebalance is capped by `--max-daily-budget` when set. `--apply --dry-run` previews the mutation request without sending it. `auto-promote --apply` also requires `--bid`.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  amazon-ads-pp-cli amazon-ads-dsp-dsp list --agent --select id,name,status
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
amazon-ads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
amazon-ads-pp-cli feedback --stdin < notes.txt
amazon-ads-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/amazon-ads-pp-cli/feedback.jsonl`. They are never POSTed unless `AMAZON_ADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMAZON_ADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
amazon-ads-pp-cli profile save briefing --json
amazon-ads-pp-cli --profile briefing amazon-ads-dsp-dsp list
amazon-ads-pp-cli profile list --json
amazon-ads-pp-cli profile show briefing
amazon-ads-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

Amazon sponsored report submit endpoints in the upstream schemas do not all expose generator `--wait` flags. Use `amazon-ads-pp-cli reports wait <report-id>` after creating a report to poll `/v2/reports/{reportId}` until Amazon returns a terminal status.

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

1. **Empty, `help`, or `--help`** → show `amazon-ads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/cmd/amazon-ads-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add amazon-ads-pp-mcp -- amazon-ads-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which amazon-ads-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   amazon-ads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `amazon-ads-pp-cli <command> --help`.
