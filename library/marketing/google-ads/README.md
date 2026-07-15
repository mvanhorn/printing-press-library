# Google Ads CLI

Google Ads API for account discovery, GAQL reporting, campaigns, budgets, assets, conversions, audiences, planning, and billing operations.

Learn more at [Google Ads](https://developers.google.com/google-ads/api/).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@bobeglz](https://github.com/bobeglz) (bobe).

## Install

The recommended path installs both the `google-ads-pp-cli` binary and the `pp-google-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install google-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-ads/cmd/google-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-ads --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-ads --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-ads --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-ads --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_ADS_ACCESS_TOKEN` and `GOOGLE_ADS_DEVELOPER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-ads/cmd/google-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-ads": {
      "command": "google-ads-pp-mcp",
      "env": {
        "GOOGLE_ADS_ACCESS_TOKEN": "<your-token>",
        "GOOGLE_ADS_DEVELOPER_TOKEN": "<your-developer-token>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Google Ads requires an OAuth2 access token with the `https://www.googleapis.com/auth/adwords` scope and a Google Ads developer token. Authenticate via the OAuth2 flow — `auth login` opens a browser, runs the consent flow, and stores a refresh token so access tokens are minted automatically:

```bash
google-ads-pp-cli auth login --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET
```

`--client-id` / `--client-secret` default to `$GOOGLE_ADS_CLIENT_ID` / `$GOOGLE_ADS_CLIENT_SECRET` when set. Inspect or clear the stored grant with `google-ads-pp-cli auth status` and `google-ads-pp-cli auth logout`.

Or supply a pre-obtained access token (plus the developer token) directly via environment variables:

```bash
export GOOGLE_ADS_ACCESS_TOKEN="your-token-here"
export GOOGLE_ADS_DEVELOPER_TOKEN="your-developer-token-here"
# Optional, for manager-account calls:
export GOOGLE_ADS_LOGIN_CUSTOMER_ID="1234567890"
```

### 3. Verify Setup

```bash
google-ads-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
google-ads-pp-cli customers-google-ads search mock-value
```

## Usage

Run `google-ads-pp-cli --help` for the full command reference and flag list.

## Commands

### audience_insights

Google Ads audience insights operations

- **`google-ads-pp-cli audience-insights list-insights-eligible-dates`** - Lists date ranges for which audience insights data can be requested.

### customers

Google Ads customers operations

- **`google-ads-pp-cli customers create-customer-client`** - Creates a new client under manager. The new client customer is returned.
- **`google-ads-pp-cli customers generate-ad-group-themes`** - Returns a list of suggested AdGroups and suggested modifications (text, match type) for the given keywords.
- **`google-ads-pp-cli customers generate-audience-composition-insights`** - Returns a collection of attributes that are represented in an audience of interest, with metrics that compare each attribute's share of the audience with its share of a baseline audience.
- **`google-ads-pp-cli customers generate-audience-overlap-insights`** - Returns a collection of audience attributes along with estimates of the overlap between their potential YouTube reach and that of a given input attribute.
- **`google-ads-pp-cli customers generate-creator-insights`** - Returns insights for a collection of YouTube Creators and Channels.
- **`google-ads-pp-cli customers generate-insights-finder-report`** - Creates a saved report that can be viewed in the Insights Finder tool.
- **`google-ads-pp-cli customers generate-keyword-forecast-metrics`** - Returns metrics (such as impressions, clicks, total cost) of a keyword forecast for the given campaign.
- **`google-ads-pp-cli customers generate-keyword-historical-metrics`** - Returns a list of keyword historical metrics.
- **`google-ads-pp-cli customers generate-keyword-ideas`** - Returns a list of keyword ideas.
- **`google-ads-pp-cli customers generate-reach-forecast`** - Generates a reach forecast for a given targeting / product mix.
- **`google-ads-pp-cli customers generate-shareable-previews`** - Returns the requested Shareable Preview.
- **`google-ads-pp-cli customers generate-suggested-targeting-insights`** - Returns a collection of targeting insights (e.g. targetable audiences) that are relevant to the requested audience.
- **`google-ads-pp-cli customers generate-targeting-suggestion-metrics`** - Returns potential reach metrics for targetable audiences. This method helps answer questions like "How many Men aged 18+ interested in Camping can be reached on YouTube?"
- **`google-ads-pp-cli customers generate-trending-insights`** - Returns insights for trending content on YouTube.
- **`google-ads-pp-cli customers get-identity-verification`** - Returns Identity Verification information.
- **`google-ads-pp-cli customers list-accessible-customers`** - Returns resource names of customers directly accessible by the user authenticating the call.
- **`google-ads-pp-cli customers mutate`** - Updates a customer. Operation statuses are returned.
- **`google-ads-pp-cli customers remove-campaign-automatically-created-asset`** - Removes automatically created assets from a campaign.
- **`google-ads-pp-cli customers search-audience-insights-attributes`** - Searches for audience attributes that can be used to generate insights.
- **`google-ads-pp-cli customers start-identity-verification`** - Starts Identity Verification for a given verification program type. Statuses are returned.
- **`google-ads-pp-cli customers suggest-brands`** - Rpc to return a list of matching brands based on a prefix for this customer.
- **`google-ads-pp-cli customers suggest-keyword-themes`** - Suggests keyword themes to advertise on.
- **`google-ads-pp-cli customers suggest-smart-campaign-ad`** - Suggests a Smart campaign ad compatible with the Ad family of resources, based on data points such as targeting and the business to advertise.
- **`google-ads-pp-cli customers suggest-smart-campaign-budget-options`** - Returns BudgetOption suggestions.
- **`google-ads-pp-cli customers suggest-travel-assets`** - Returns Travel Asset suggestions. Asset suggestions are returned on a best-effort basis. There are no guarantees that all possible asset types will be returned for any given hotel property.
- **`google-ads-pp-cli customers upload-call-conversions`** - Processes the given call conversions.
- **`google-ads-pp-cli customers upload-click-conversions`** - Processes the given click conversions.
- **`google-ads-pp-cli customers upload-conversion-adjustments`** - Processes the given conversion adjustments.
- **`google-ads-pp-cli customers upload-user-data`** - Uploads the given user data.

### customers_account_budget_proposals

Google Ads customers account budget proposals operations

- **`google-ads-pp-cli customers-account-budget-proposals mutate`** - Creates, updates, or removes account budget proposals. Operation statuses are returned.

### customers_account_links

Google Ads customers account links operations

- **`google-ads-pp-cli customers-account-links create`** - Creates an account link.
- **`google-ads-pp-cli customers-account-links mutate`** - Creates or removes an account link. From V5, create is not supported through AccountLinkService.MutateAccountLink. Use AccountLinkService.CreateAccountLink instead.

### customers_ad_group_ad_labels

Google Ads customers ad group ad labels operations

- **`google-ads-pp-cli customers-ad-group-ad-labels mutate`** - Creates and removes ad group ad labels. Operation statuses are returned.

### customers_ad_group_ads

Google Ads customers ad group ads operations

- **`google-ads-pp-cli customers-ad-group-ads mutate`** - Creates, updates, or removes ads. Operation statuses are returned.
- **`google-ads-pp-cli customers-ad-group-ads remove-automatically-created-assets`** - Remove automatically created assets from an ad.

### customers_ad_group_asset_sets

Google Ads customers ad group asset sets operations

- **`google-ads-pp-cli customers-ad-group-asset-sets mutate`** - Creates, or removes ad group asset sets. Operation statuses are returned.

### customers_ad_group_assets

Google Ads customers ad group assets operations

- **`google-ads-pp-cli customers-ad-group-assets mutate`** - Creates, updates, or removes ad group assets. Operation statuses are returned.

### customers_ad_group_bid_modifiers

Google Ads customers ad group bid modifiers operations

- **`google-ads-pp-cli customers-ad-group-bid-modifiers mutate`** - Creates, updates, or removes ad group bid modifiers. Operation statuses are returned.

### customers_ad_group_criteria

Google Ads customers ad group criteria operations

- **`google-ads-pp-cli customers-ad-group-criteria mutate`** - Creates, updates, or removes criteria. Operation statuses are returned.

### customers_ad_group_criterion_customizers

Google Ads customers ad group criterion customizers operations

- **`google-ads-pp-cli customers-ad-group-criterion-customizers mutate`** - Creates, updates or removes ad group criterion customizers. Operation statuses are returned.

### customers_ad_group_criterion_labels

Google Ads customers ad group criterion labels operations

- **`google-ads-pp-cli customers-ad-group-criterion-labels mutate`** - Creates and removes ad group criterion labels. Operation statuses are returned.

### customers_ad_group_customizers

Google Ads customers ad group customizers operations

- **`google-ads-pp-cli customers-ad-group-customizers mutate`** - Creates, updates or removes ad group customizers. Operation statuses are returned.

### customers_ad_group_labels

Google Ads customers ad group labels operations

- **`google-ads-pp-cli customers-ad-group-labels mutate`** - Creates and removes ad group labels. Operation statuses are returned.

### customers_ad_groups

Google Ads customers ad groups operations

- **`google-ads-pp-cli customers-ad-groups mutate`** - Creates, updates, or removes ad groups. Operation statuses are returned.

### customers_ad_parameters

Google Ads customers ad parameters operations

- **`google-ads-pp-cli customers-ad-parameters mutate`** - Creates, updates, or removes ad parameters. Operation statuses are returned.

### customers_ads

Google Ads customers ads operations

- **`google-ads-pp-cli customers-ads mutate`** - Updates ads. Operation statuses are returned. Updating ads is not supported for TextAd, ExpandedDynamicSearchAd, GmailAd and ImageAd.

### customers_asset_generations

Google Ads customers asset generations operations

- **`google-ads-pp-cli customers-asset-generations generate-images`** - Uses generative AI to generate images that can be used as assets in a campaign.
- **`google-ads-pp-cli customers-asset-generations generate-text`** - Uses generative AI to generate text that can be used as assets in a campaign.

### customers_asset_group_assets

Google Ads customers asset group assets operations

- **`google-ads-pp-cli customers-asset-group-assets mutate`** - Creates, updates or removes asset group assets. Operation statuses are returned.

### customers_asset_group_listing_group_filters

Google Ads customers asset group listing group filters operations

- **`google-ads-pp-cli customers-asset-group-listing-group-filters mutate`** - Creates, updates or removes asset group listing group filters. Operation statuses are returned.

### customers_asset_group_signals

Google Ads customers asset group signals operations

- **`google-ads-pp-cli customers-asset-group-signals mutate`** - Creates or removes asset group signals. Operation statuses are returned.

### customers_asset_groups

Google Ads customers asset groups operations

- **`google-ads-pp-cli customers-asset-groups mutate`** - Creates, updates or removes asset groups. Operation statuses are returned.

### customers_asset_set_assets

Google Ads customers asset set assets operations

- **`google-ads-pp-cli customers-asset-set-assets mutate`** - Creates, updates or removes asset set assets. Operation statuses are returned.

### customers_asset_sets

Google Ads customers asset sets operations

- **`google-ads-pp-cli customers-asset-sets mutate`** - Creates, updates or removes asset sets. Operation statuses are returned.

### customers_assets

Google Ads customers assets operations

- **`google-ads-pp-cli customers-assets mutate`** - Creates assets. Operation statuses are returned.

### customers_audiences

Google Ads customers audiences operations

- **`google-ads-pp-cli customers-audiences mutate`** - Creates audiences. Operation statuses are returned.

### customers_batch_jobs

Google Ads customers batch jobs operations

- **`google-ads-pp-cli customers-batch-jobs add-operations`** - Add operations to the batch job.
- **`google-ads-pp-cli customers-batch-jobs list-results`** - Returns the results of the batch job. The job must be done. Supports standard list paging.
- **`google-ads-pp-cli customers-batch-jobs mutate`** - Mutates a batch job.
- **`google-ads-pp-cli customers-batch-jobs run`** - Runs the batch job. The Operation.metadata field type is BatchJobMetadata. When finished, the long running operation will not contain errors or a response. Instead, use ListBatchJobResults to get the results of the job.

### customers_bidding_data_exclusions

Google Ads customers bidding data exclusions operations

- **`google-ads-pp-cli customers-bidding-data-exclusions mutate`** - Creates, updates, or removes data exclusions. Operation statuses are returned.

### customers_bidding_seasonality_adjustments

Google Ads customers bidding seasonality adjustments operations

- **`google-ads-pp-cli customers-bidding-seasonality-adjustments mutate`** - Creates, updates, or removes seasonality adjustments. Operation statuses are returned.

### customers_bidding_strategies

Google Ads customers bidding strategies operations

- **`google-ads-pp-cli customers-bidding-strategies mutate`** - Creates, updates, or removes bidding strategies. Operation statuses are returned.

### customers_billing_setups

Google Ads customers billing setups operations

- **`google-ads-pp-cli customers-billing-setups mutate`** - Creates a billing setup, or cancels an existing billing setup.

### customers_campaign_asset_sets

Google Ads customers campaign asset sets operations

- **`google-ads-pp-cli customers-campaign-asset-sets mutate`** - Creates, updates or removes campaign asset sets. Operation statuses are returned.

### customers_campaign_assets

Google Ads customers campaign assets operations

- **`google-ads-pp-cli customers-campaign-assets mutate`** - Creates, updates, or removes campaign assets. Operation statuses are returned.

### customers_campaign_bid_modifiers

Google Ads customers campaign bid modifiers operations

- **`google-ads-pp-cli customers-campaign-bid-modifiers mutate`** - Creates, updates, or removes campaign bid modifiers. Operation statuses are returned.

### customers_campaign_budgets

Google Ads customers campaign budgets operations

- **`google-ads-pp-cli customers-campaign-budgets mutate`** - Creates, updates, or removes campaign budgets. Operation statuses are returned.

### customers_campaign_conversion_goals

Google Ads customers campaign conversion goals operations

- **`google-ads-pp-cli customers-campaign-conversion-goals mutate`** - Creates, updates or removes campaign conversion goals. Operation statuses are returned.

### customers_campaign_criteria

Google Ads customers campaign criteria operations

- **`google-ads-pp-cli customers-campaign-criteria mutate`** - Creates, updates, or removes criteria. Operation statuses are returned.

### customers_campaign_customizers

Google Ads customers campaign customizers operations

- **`google-ads-pp-cli customers-campaign-customizers mutate`** - Creates, updates or removes campaign customizers. Operation statuses are returned.

### customers_campaign_drafts

Google Ads customers campaign drafts operations

- **`google-ads-pp-cli customers-campaign-drafts list-async-errors`** - Returns all errors that occurred during CampaignDraft promote. Throws an error if called before campaign draft is promoted. Supports standard list paging.
- **`google-ads-pp-cli customers-campaign-drafts mutate`** - Creates, updates, or removes campaign drafts. Operation statuses are returned.
- **`google-ads-pp-cli customers-campaign-drafts promote`** - Promotes the changes in a draft back to the base campaign. This method returns a Long Running Operation (LRO) indicating if the Promote is done. Use google.longrunning.Operations.GetOperation to poll the LRO until it is done. Only a done status is returned in the response. See the status in the Campaign Draft resource 

### customers_campaign_goal_configs

Google Ads customers campaign goal configs operations

- **`google-ads-pp-cli customers-campaign-goal-configs mutate`** - Create or update campaign goal configs.

### customers_campaign_groups

Google Ads customers campaign groups operations

- **`google-ads-pp-cli customers-campaign-groups mutate`** - Creates, updates, or removes campaign groups. Operation statuses are returned.

### customers_campaign_labels

Google Ads customers campaign labels operations

- **`google-ads-pp-cli customers-campaign-labels mutate`** - Creates and removes campaign-label relationships. Operation statuses are returned.

### customers_campaign_lifecycle_goal

Google Ads customers campaign lifecycle goal operations

- **`google-ads-pp-cli customers-campaign-lifecycle-goal configure-campaign-lifecycle-goals`** - Process the given campaign lifecycle configurations.

### customers_campaign_shared_sets

Google Ads customers campaign shared sets operations

- **`google-ads-pp-cli customers-campaign-shared-sets mutate`** - Creates or removes campaign shared sets. Operation statuses are returned.

### customers_campaigns

Google Ads customers campaigns operations

- **`google-ads-pp-cli customers-campaigns enable-pmax-brand-guidelines`** - Enables Brand Guidelines for Performance Max campaigns.
- **`google-ads-pp-cli customers-campaigns mutate`** - Creates, updates, or removes campaigns. Operation statuses are returned.

### customers_conversion_actions

Google Ads customers conversion actions operations

- **`google-ads-pp-cli customers-conversion-actions mutate`** - Creates, updates or removes conversion actions. Operation statuses are returned.

### customers_conversion_custom_variables

Google Ads customers conversion custom variables operations

- **`google-ads-pp-cli customers-conversion-custom-variables mutate`** - Creates or updates conversion custom variables. Operation statuses are returned.

### customers_conversion_goal_campaign_configs

Google Ads customers conversion goal campaign configs operations

- **`google-ads-pp-cli customers-conversion-goal-campaign-configs mutate`** - Creates, updates or removes conversion goal campaign config. Operation statuses are returned.

### customers_conversion_value_rule_sets

Google Ads customers conversion value rule sets operations

- **`google-ads-pp-cli customers-conversion-value-rule-sets mutate`** - Creates, updates or removes conversion value rule sets. Operation statuses are returned.

### customers_conversion_value_rules

Google Ads customers conversion value rules operations

- **`google-ads-pp-cli customers-conversion-value-rules mutate`** - Creates, updates, or removes conversion value rules. Operation statuses are returned.

### customers_custom_audiences

Google Ads customers custom audiences operations

- **`google-ads-pp-cli customers-custom-audiences mutate`** - Creates or updates custom audiences. Operation statuses are returned.

### customers_custom_conversion_goals

Google Ads customers custom conversion goals operations

- **`google-ads-pp-cli customers-custom-conversion-goals mutate`** - Creates, updates or removes custom conversion goals. Operation statuses are returned.

### customers_custom_interests

Google Ads customers custom interests operations

- **`google-ads-pp-cli customers-custom-interests mutate`** - Creates or updates custom interests. Operation statuses are returned.

### customers_customer_asset_sets

Google Ads customers customer asset sets operations

- **`google-ads-pp-cli customers-customer-asset-sets mutate`** - Creates, or removes customer asset sets. Operation statuses are returned.

### customers_customer_assets

Google Ads customers customer assets operations

- **`google-ads-pp-cli customers-customer-assets mutate`** - Creates, updates, or removes customer assets. Operation statuses are returned.

### customers_customer_client_links

Google Ads customers customer client links operations

- **`google-ads-pp-cli customers-customer-client-links mutate`** - Creates or updates a customer client link. Operation statuses are returned.

### customers_customer_conversion_goals

Google Ads customers customer conversion goals operations

- **`google-ads-pp-cli customers-customer-conversion-goals mutate`** - Creates, updates or removes customer conversion goals. Operation statuses are returned.

### customers_customer_customizers

Google Ads customers customer customizers operations

- **`google-ads-pp-cli customers-customer-customizers mutate`** - Creates, updates or removes customer customizers. Operation statuses are returned.

### customers_customer_labels

Google Ads customers customer labels operations

- **`google-ads-pp-cli customers-customer-labels mutate`** - Creates and removes customer-label relationships. Operation statuses are returned.

### customers_customer_lifecycle_goal

Google Ads customers customer lifecycle goal operations

- **`google-ads-pp-cli customers-customer-lifecycle-goal configure-customer-lifecycle-goals`** - Process the given customer lifecycle configurations.

### customers_customer_manager_links

Google Ads customers customer manager links operations

- **`google-ads-pp-cli customers-customer-manager-links move-manager-link`** - Moves a client customer to a new manager customer. This simplifies the complex request that requires two operations to move a client customer to a new manager, for example: 1. Update operation with Status INACTIVE (previous manager) and, 2. Update operation with Status ACTIVE (new manager).
- **`google-ads-pp-cli customers-customer-manager-links mutate`** - Updates customer manager links. Operation statuses are returned.

### customers_customer_negative_criteria

Google Ads customers customer negative criteria operations

- **`google-ads-pp-cli customers-customer-negative-criteria mutate`** - Creates or removes criteria. Operation statuses are returned.

### customers_customer_sk_ad_network_conversion_value_schemas

Google Ads customers customer sk ad network conversion value schemas operations

- **`google-ads-pp-cli customers-customer-sk-ad-network-conversion-value-schemas mutate`** - Creates or updates the CustomerSkAdNetworkConversionValueSchema.

### customers_customer_user_access_invitations

Google Ads customers customer user access invitations operations

- **`google-ads-pp-cli customers-customer-user-access-invitations mutate`** - Creates or removes an access invitation.

### customers_customer_user_accesses

Google Ads customers customer user accesses operations

- **`google-ads-pp-cli customers-customer-user-accesses mutate`** - Updates, removes permission of a user on a given customer. Operation statuses are returned.

### customers_customizer_attributes

Google Ads customers customizer attributes operations

- **`google-ads-pp-cli customers-customizer-attributes mutate`** - Creates, updates or removes customizer attributes. Operation statuses are returned.

### customers_data_links

Google Ads customers data links operations

- **`google-ads-pp-cli customers-data-links create`** - Creates a data link. The requesting Google Ads account name and account ID will be shared with the third party (such as YouTube creators for video links) to whom you are creating the link with.
- **`google-ads-pp-cli customers-data-links remove`** - Remove a data link.
- **`google-ads-pp-cli customers-data-links update`** - Update a data link.

### customers_experiment_arms

Google Ads customers experiment arms operations

- **`google-ads-pp-cli customers-experiment-arms mutate`** - Creates, updates, or removes experiment arms. Operation statuses are returned.

### customers_experiments

Google Ads customers experiments operations

- **`google-ads-pp-cli customers-experiments end-experiment`** - Immediately ends an experiment, changing the experiment's scheduled end date and without waiting for end of day. End date is updated to be the time of the request.
- **`google-ads-pp-cli customers-experiments graduate-experiment`** - Graduates an experiment to a full campaign.
- **`google-ads-pp-cli customers-experiments list-experiment-async-errors`** - Returns all errors that occurred during the last Experiment update (either scheduling or promotion). Supports standard list paging.
- **`google-ads-pp-cli customers-experiments mutate`** - Creates, updates, or removes experiments. Operation statuses are returned.
- **`google-ads-pp-cli customers-experiments promote-experiment`** - Promotes the trial campaign thus applying changes in the trial campaign to the base campaign. This method returns a long running operation that tracks the promotion of the experiment campaign. If it fails, a list of errors can be retrieved using the ListExperimentAsyncErrors method. The operation's metadata will be a s
- **`google-ads-pp-cli customers-experiments schedule-experiment`** - Schedule an experiment. The in design campaign will be converted into a real campaign (called the experiment campaign) that will begin serving ads if successfully created. The experiment is scheduled immediately with status INITIALIZING. This method returns a long running operation that tracks the forking of the in des

### customers_goals

Google Ads customers goals operations

- **`google-ads-pp-cli customers-goals mutate`** - Create or update goals.

### customers-google-ads

Google Ads customers google ads operations

- **`google-ads-pp-cli customers-google-ads mutate`** - Creates, updates, or removes resources. This method supports atomic transactions with multiple types of resources. For example, you can atomically create a campaign and a campaign budget, or perform up to thousands of mutates atomically. This method is essentially a wrapper around a series of mutate methods. The only f
- **`google-ads-pp-cli customers-google-ads search`** - Returns all rows that match the search query.
- **`google-ads-pp-cli customers-google-ads search-stream`** - Returns all rows that match the search stream query.

### customers_invoices

Google Ads customers invoices operations

- **`google-ads-pp-cli customers-invoices list`** - Returns all invoices associated with a billing setup, for a given month.

### customers_keyword_plan_ad_group_keywords

Google Ads customers keyword plan ad group keywords operations

- **`google-ads-pp-cli customers-keyword-plan-ad-group-keywords mutate`** - Creates, updates, or removes Keyword Plan ad group keywords. Operation statuses are returned.

### customers_keyword_plan_ad_groups

Google Ads customers keyword plan ad groups operations

- **`google-ads-pp-cli customers-keyword-plan-ad-groups mutate`** - Creates, updates, or removes Keyword Plan ad groups. Operation statuses are returned.

### customers_keyword_plan_campaign_keywords

Google Ads customers keyword plan campaign keywords operations

- **`google-ads-pp-cli customers-keyword-plan-campaign-keywords mutate`** - Creates, updates, or removes Keyword Plan campaign keywords. Operation statuses are returned.

### customers_keyword_plan_campaigns

Google Ads customers keyword plan campaigns operations

- **`google-ads-pp-cli customers-keyword-plan-campaigns mutate`** - Creates, updates, or removes Keyword Plan campaigns. Operation statuses are returned.

### customers_keyword_plans

Google Ads customers keyword plans operations

- **`google-ads-pp-cli customers-keyword-plans mutate`** - Creates, updates, or removes keyword plans. Operation statuses are returned.

### customers_labels

Google Ads customers labels operations

- **`google-ads-pp-cli customers-labels mutate`** - Creates, updates, or removes labels. Operation statuses are returned.

### customers_local_services

Google Ads customers local services operations

- **`google-ads-pp-cli customers-local-services append-lead-conversation`** - RPC to append Local Services Lead Conversation resources to Local Services Lead resources.

### customers_local_services_leads

Google Ads customers local services leads operations

- **`google-ads-pp-cli customers-local-services-leads provide-lead-feedback`** - RPC to provide feedback on Local Services Lead resources.

### customers_offline_user_data_jobs

Google Ads customers offline user data jobs operations

- **`google-ads-pp-cli customers-offline-user-data-jobs add-operations`** - Adds operations to the offline user data job.
- **`google-ads-pp-cli customers-offline-user-data-jobs create`** - Creates an offline user data job.
- **`google-ads-pp-cli customers-offline-user-data-jobs run`** - Runs the offline user data job. When finished, the long running operation will contain the processing result or failure information, if any.

### customers_operations

Google Ads customers operations operations

- **`google-ads-pp-cli customers-operations cancel`** - Starts asynchronous cancellation on a long-running operation. The server makes a best effort to cancel the operation, but success is not guaranteed. If the server doesn't support this method, it returns `google.rpc.Code.UNIMPLEMENTED`. Clients can use Operations.GetOperation or other methods to check whether the cancel
- **`google-ads-pp-cli customers-operations delete`** - Deletes a long-running operation. This method indicates that the client is no longer interested in the operation result. It does not cancel the operation. If the server doesn't support this method, it returns `google.rpc.Code.UNIMPLEMENTED`.
- **`google-ads-pp-cli customers-operations get`** - Gets the latest state of a long-running operation. Clients can use this method to poll the operation result at intervals as recommended by the API service.
- **`google-ads-pp-cli customers-operations list`** - Lists operations that match the specified filter in the request. If the server doesn't support this method, it returns `UNIMPLEMENTED`.
- **`google-ads-pp-cli customers-operations wait`** - Waits until the specified long-running operation is done or reaches at most a specified timeout, returning the latest state. If the operation is already done, the latest state is immediately returned. If the timeout specified is greater than the default HTTP/RPC timeout, the HTTP/RPC timeout is used. If the server does

### customers_payments_accounts

Google Ads customers payments accounts operations

- **`google-ads-pp-cli customers-payments-accounts list`** - Returns all payments accounts associated with all managers between the login customer ID and specified serving customer in the hierarchy, inclusive.

### customers_product_link_invitations

Google Ads customers product link invitations operations

- **`google-ads-pp-cli customers-product-link-invitations create`** - Creates a product link invitation.
- **`google-ads-pp-cli customers-product-link-invitations remove`** - Remove a product link invitation.
- **`google-ads-pp-cli customers-product-link-invitations update`** - Update a product link invitation.

### customers_product_links

Google Ads customers product links operations

- **`google-ads-pp-cli customers-product-links create`** - Creates a product link.
- **`google-ads-pp-cli customers-product-links remove`** - Removes a product link.

### customers_recommendation_subscriptions

Google Ads customers recommendation subscriptions operations

- **`google-ads-pp-cli customers-recommendation-subscriptions mutate-recommendation-subscription`** - Mutates given subscription with corresponding apply parameters.

### customers_recommendations

Google Ads customers recommendations operations

- **`google-ads-pp-cli customers-recommendations apply`** - Applies given recommendations with corresponding apply parameters.
- **`google-ads-pp-cli customers-recommendations dismiss`** - Dismisses given recommendations.
- **`google-ads-pp-cli customers-recommendations generate`** - Generates Recommendations based off the requested recommendation_types.

### customers_remarketing_actions

Google Ads customers remarketing actions operations

- **`google-ads-pp-cli customers-remarketing-actions mutate`** - Creates or updates remarketing actions. Operation statuses are returned.

### customers_shared_criteria

Google Ads customers shared criteria operations

- **`google-ads-pp-cli customers-shared-criteria mutate`** - Creates or removes shared criteria. Operation statuses are returned.

### customers_shared_sets

Google Ads customers shared sets operations

- **`google-ads-pp-cli customers-shared-sets mutate`** - Creates, updates, or removes shared sets. Operation statuses are returned.

### customers_smart_campaign_settings

Google Ads customers smart campaign settings operations

- **`google-ads-pp-cli customers-smart-campaign-settings get-smart-campaign-status`** - Returns the status of the requested Smart campaign.
- **`google-ads-pp-cli customers-smart-campaign-settings mutate`** - Updates Smart campaign settings for campaigns.

### customers_third_party_app_analytics_links

Google Ads customers third party app analytics links operations

- **`google-ads-pp-cli customers-third-party-app-analytics-links regenerate-shareable-link-id`** - Regenerate ThirdPartyAppAnalyticsLink.shareable_link_id that should be provided to the third party when setting up app analytics.

### customers_user_list_customer_types

Google Ads customers user list customer types operations

- **`google-ads-pp-cli customers-user-list-customer-types mutate`** - Attach or remove user list customer types. Operation statuses are returned.

### customers_user_lists

Google Ads customers user lists operations

- **`google-ads-pp-cli customers-user-lists mutate`** - Creates or updates user lists. Operation statuses are returned.

### geo_target_constants

Google Ads geo target constants operations

- **`google-ads-pp-cli geo-target-constants suggest`** - Returns GeoTargetConstant suggestions by location name or by resource name.

### google_ads

Google Ads google ads operations

- **`google-ads-pp-cli google-ads generate-conversion-rates`** - Returns a collection of conversion rate suggestions for supported plannable products.
- **`google-ads-pp-cli google-ads list-plannable-locations`** - Returns the list of plannable locations (for example, countries).
- **`google-ads-pp-cli google-ads list-plannable-products`** - Returns the list of per-location plannable YouTube ad formats with allowed targeting.
- **`google-ads-pp-cli google-ads list-plannable-user-interests`** - Returns the list of plannable user interests. A plannable user interest is one that can be targeted in a reach forecast using ReachPlanService.GenerateReachForecast.
- **`google-ads-pp-cli google-ads list-plannable-user-lists`** - Returns the list of plannable user lists with their plannable status. User lists may not be plannable for a number of reasons, including: - They are less than 10 days old. - They have a membership lifespan that is less than 30 days - They have less than 10,000 or more than 700,000 users.

### google_ads_fields

Google Ads google ads fields operations

- **`google-ads-pp-cli google-ads-fields get`** - Returns just the requested field.
- **`google-ads-pp-cli google-ads-fields search`** - Returns all fields that match the search query.

### keyword_theme_constants

Google Ads keyword theme constants operations

- **`google-ads-pp-cli keyword-theme-constants suggest`** - Returns KeywordThemeConstant suggestions by keyword themes.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-ads-pp-cli customers-google-ads search mock-value

# JSON for scripting and agents
google-ads-pp-cli customers-google-ads search mock-value --json

# Filter to specific fields
google-ads-pp-cli customers-google-ads search mock-value --json --select id,name,status

# Dry run — show the request without sending
google-ads-pp-cli customers-google-ads search mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-ads-pp-cli customers-google-ads search mock-value --agent
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
google-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-ads-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_ADS_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |
| `GOOGLE_ADS_DEVELOPER_TOKEN` | per_call | Yes | Set to your API credential. |
| `GOOGLE_ADS_LOGIN_CUSTOMER_ID` | per_call | No | Optional manager account ID sent as the login-customer-id header. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-ads-pp-cli doctor` to check credentials
- Verify the environment variables are set: `echo $GOOGLE_ADS_ACCESS_TOKEN` and `echo $GOOGLE_ADS_DEVELOPER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
