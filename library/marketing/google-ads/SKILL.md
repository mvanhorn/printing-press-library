---
name: pp-google-ads
description: "Google Ads API for account discovery, GAQL reporting, campaigns, budgets, assets, conversions, audiences, planning, and billing operations. Trigger phrases: `pull a Google Ads report`, `GAQL query`, `check campaign performance`, `use google-ads`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      env: ["GOOGLE_ADS_ACCESS_TOKEN", "GOOGLE_ADS_DEVELOPER_TOKEN"]
      bins:
        - google-ads-pp-cli
    envVars:
      - name: GOOGLE_ADS_ACCESS_TOKEN
        required: true
        description: "OAuth2 access token with the https://www.googleapis.com/auth/adwords scope."
      - name: GOOGLE_ADS_DEVELOPER_TOKEN
        required: true
        description: "Google Ads developer token sent as the developer-token header."
      - name: GOOGLE_ADS_LOGIN_CUSTOMER_ID
        required: false
        description: "Optional manager account ID sent as the login-customer-id header."
    install:
      - kind: go
        bins: [google-ads-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-ads/cmd/google-ads-pp-cli
---

# Google Ads — Printing Press CLI

Google Ads API for account discovery, GAQL reporting, campaigns, budgets, assets, conversions, audiences, planning, and billing operations.

## Command Reference

**audience_insights** — Google Ads audience insights operations

- `google-ads-pp-cli audience_insights` — Lists date ranges for which audience insights data can be requested.

**customers** — Google Ads customers operations

- `google-ads-pp-cli customers create_customer_client` — Creates a new client under manager. The new client customer is returned.
- `google-ads-pp-cli customers generate_ad_group_themes` — Returns a list of suggested AdGroups and suggested modifications (text, match type) for the given keywords.
- `google-ads-pp-cli customers generate_audience_composition_insights` — Returns a collection of attributes that are represented in an audience of interest, with metrics that compare each...
- `google-ads-pp-cli customers generate_audience_overlap_insights` — Returns a collection of audience attributes along with estimates of the overlap between their potential YouTube...
- `google-ads-pp-cli customers generate_creator_insights` — Returns insights for a collection of YouTube Creators and Channels.
- `google-ads-pp-cli customers generate_insights_finder_report` — Creates a saved report that can be viewed in the Insights Finder tool.
- `google-ads-pp-cli customers generate_keyword_forecast_metrics` — Returns metrics (such as impressions, clicks, total cost) of a keyword forecast for the given campaign.
- `google-ads-pp-cli customers generate_keyword_historical_metrics` — Returns a list of keyword historical metrics.
- `google-ads-pp-cli customers generate_keyword_ideas` — Returns a list of keyword ideas.
- `google-ads-pp-cli customers generate_reach_forecast` — Generates a reach forecast for a given targeting / product mix.
- `google-ads-pp-cli customers generate_shareable_previews` — Returns the requested Shareable Preview.
- `google-ads-pp-cli customers generate_suggested_targeting_insights` — Returns a collection of targeting insights (e.g. targetable audiences) that are relevant to the requested audience.
- `google-ads-pp-cli customers generate_targeting_suggestion_metrics` — Returns potential reach metrics for targetable audiences. This method helps answer questions like 'How many Men aged...
- `google-ads-pp-cli customers generate_trending_insights` — Returns insights for trending content on YouTube.
- `google-ads-pp-cli customers get_identity_verification` — Returns Identity Verification information.
- `google-ads-pp-cli customers list_accessible_customers` — Returns resource names of customers directly accessible by the user authenticating the call.
- `google-ads-pp-cli customers mutate` — Updates a customer. Operation statuses are returned.
- `google-ads-pp-cli customers remove_campaign_automatically_created_asset` — Removes automatically created assets from a campaign.
- `google-ads-pp-cli customers search_audience_insights_attributes` — Searches for audience attributes that can be used to generate insights.
- `google-ads-pp-cli customers start_identity_verification` — Starts Identity Verification for a given verification program type. Statuses are returned.
- `google-ads-pp-cli customers suggest_brands` — Rpc to return a list of matching brands based on a prefix for this customer.
- `google-ads-pp-cli customers suggest_keyword_themes` — Suggests keyword themes to advertise on.
- `google-ads-pp-cli customers suggest_smart_campaign_ad` — Suggests a Smart campaign ad compatible with the Ad family of resources, based on data points such as targeting and...
- `google-ads-pp-cli customers suggest_smart_campaign_budget_options` — Returns BudgetOption suggestions.
- `google-ads-pp-cli customers suggest_travel_assets` — Returns Travel Asset suggestions. Asset suggestions are returned on a best-effort basis. There are no guarantees...
- `google-ads-pp-cli customers upload_call_conversions` — Processes the given call conversions.
- `google-ads-pp-cli customers upload_click_conversions` — Processes the given click conversions.
- `google-ads-pp-cli customers upload_conversion_adjustments` — Processes the given conversion adjustments.
- `google-ads-pp-cli customers upload_user_data` — Uploads the given user data.

**customers_account_budget_proposals** — Google Ads customers account budget proposals operations

- `google-ads-pp-cli customers_account_budget_proposals <customerId>` — Creates, updates, or removes account budget proposals. Operation statuses are returned.

**customers_account_links** — Google Ads customers account links operations

- `google-ads-pp-cli customers_account_links create` — Creates an account link.
- `google-ads-pp-cli customers_account_links mutate` — Creates or removes an account link. From V5, create is not supported through AccountLinkService.MutateAccountLink....

**customers_ad_group_ad_labels** — Google Ads customers ad group ad labels operations

- `google-ads-pp-cli customers_ad_group_ad_labels <customerId>` — Creates and removes ad group ad labels. Operation statuses are returned.

**customers_ad_group_ads** — Google Ads customers ad group ads operations

- `google-ads-pp-cli customers_ad_group_ads mutate` — Creates, updates, or removes ads. Operation statuses are returned.
- `google-ads-pp-cli customers_ad_group_ads remove_automatically_created_assets` — Remove automatically created assets from an ad.

**customers_ad_group_asset_sets** — Google Ads customers ad group asset sets operations

- `google-ads-pp-cli customers_ad_group_asset_sets <customerId>` — Creates, or removes ad group asset sets. Operation statuses are returned.

**customers_ad_group_assets** — Google Ads customers ad group assets operations

- `google-ads-pp-cli customers_ad_group_assets <customerId>` — Creates, updates, or removes ad group assets. Operation statuses are returned.

**customers_ad_group_bid_modifiers** — Google Ads customers ad group bid modifiers operations

- `google-ads-pp-cli customers_ad_group_bid_modifiers <customerId>` — Creates, updates, or removes ad group bid modifiers. Operation statuses are returned.

**customers_ad_group_criteria** — Google Ads customers ad group criteria operations

- `google-ads-pp-cli customers_ad_group_criteria <customerId>` — Creates, updates, or removes criteria. Operation statuses are returned.

**customers_ad_group_criterion_customizers** — Google Ads customers ad group criterion customizers operations

- `google-ads-pp-cli customers_ad_group_criterion_customizers <customerId>` — Creates, updates or removes ad group criterion customizers. Operation statuses are returned.

**customers_ad_group_criterion_labels** — Google Ads customers ad group criterion labels operations

- `google-ads-pp-cli customers_ad_group_criterion_labels <customerId>` — Creates and removes ad group criterion labels. Operation statuses are returned.

**customers_ad_group_customizers** — Google Ads customers ad group customizers operations

- `google-ads-pp-cli customers_ad_group_customizers <customerId>` — Creates, updates or removes ad group customizers. Operation statuses are returned.

**customers_ad_group_labels** — Google Ads customers ad group labels operations

- `google-ads-pp-cli customers_ad_group_labels <customerId>` — Creates and removes ad group labels. Operation statuses are returned.

**customers_ad_groups** — Google Ads customers ad groups operations

- `google-ads-pp-cli customers_ad_groups <customerId>` — Creates, updates, or removes ad groups. Operation statuses are returned.

**customers_ad_parameters** — Google Ads customers ad parameters operations

- `google-ads-pp-cli customers_ad_parameters <customerId>` — Creates, updates, or removes ad parameters. Operation statuses are returned.

**customers_ads** — Google Ads customers ads operations

- `google-ads-pp-cli customers_ads <customerId>` — Updates ads. Operation statuses are returned. Updating ads is not supported for TextAd, ExpandedDynamicSearchAd,...

**customers_asset_generations** — Google Ads customers asset generations operations

- `google-ads-pp-cli customers_asset_generations generate_images` — Uses generative AI to generate images that can be used as assets in a campaign.
- `google-ads-pp-cli customers_asset_generations generate_text` — Uses generative AI to generate text that can be used as assets in a campaign.

**customers_asset_group_assets** — Google Ads customers asset group assets operations

- `google-ads-pp-cli customers_asset_group_assets <customerId>` — Creates, updates or removes asset group assets. Operation statuses are returned.

**customers_asset_group_listing_group_filters** — Google Ads customers asset group listing group filters operations

- `google-ads-pp-cli customers_asset_group_listing_group_filters <customerId>` — Creates, updates or removes asset group listing group filters. Operation statuses are returned.

**customers_asset_group_signals** — Google Ads customers asset group signals operations

- `google-ads-pp-cli customers_asset_group_signals <customerId>` — Creates or removes asset group signals. Operation statuses are returned.

**customers_asset_groups** — Google Ads customers asset groups operations

- `google-ads-pp-cli customers_asset_groups <customerId>` — Creates, updates or removes asset groups. Operation statuses are returned.

**customers_asset_set_assets** — Google Ads customers asset set assets operations

- `google-ads-pp-cli customers_asset_set_assets <customerId>` — Creates, updates or removes asset set assets. Operation statuses are returned.

**customers_asset_sets** — Google Ads customers asset sets operations

- `google-ads-pp-cli customers_asset_sets <customerId>` — Creates, updates or removes asset sets. Operation statuses are returned.

**customers_assets** — Google Ads customers assets operations

- `google-ads-pp-cli customers_assets <customerId>` — Creates assets. Operation statuses are returned.

**customers_audiences** — Google Ads customers audiences operations

- `google-ads-pp-cli customers_audiences <customerId>` — Creates audiences. Operation statuses are returned.

**customers_batch_jobs** — Google Ads customers batch jobs operations

- `google-ads-pp-cli customers_batch_jobs add_operations` — Add operations to the batch job.
- `google-ads-pp-cli customers_batch_jobs list_results` — Returns the results of the batch job. The job must be done. Supports standard list paging.
- `google-ads-pp-cli customers_batch_jobs mutate` — Mutates a batch job.
- `google-ads-pp-cli customers_batch_jobs run` — Runs the batch job. The Operation.metadata field type is BatchJobMetadata. When finished, the long running operation...

**customers_bidding_data_exclusions** — Google Ads customers bidding data exclusions operations

- `google-ads-pp-cli customers_bidding_data_exclusions <customerId>` — Creates, updates, or removes data exclusions. Operation statuses are returned.

**customers_bidding_seasonality_adjustments** — Google Ads customers bidding seasonality adjustments operations

- `google-ads-pp-cli customers_bidding_seasonality_adjustments <customerId>` — Creates, updates, or removes seasonality adjustments. Operation statuses are returned.

**customers_bidding_strategies** — Google Ads customers bidding strategies operations

- `google-ads-pp-cli customers_bidding_strategies <customerId>` — Creates, updates, or removes bidding strategies. Operation statuses are returned.

**customers_billing_setups** — Google Ads customers billing setups operations

- `google-ads-pp-cli customers_billing_setups <customerId>` — Creates a billing setup, or cancels an existing billing setup.

**customers_campaign_asset_sets** — Google Ads customers campaign asset sets operations

- `google-ads-pp-cli customers_campaign_asset_sets <customerId>` — Creates, updates or removes campaign asset sets. Operation statuses are returned.

**customers_campaign_assets** — Google Ads customers campaign assets operations

- `google-ads-pp-cli customers_campaign_assets <customerId>` — Creates, updates, or removes campaign assets. Operation statuses are returned.

**customers_campaign_bid_modifiers** — Google Ads customers campaign bid modifiers operations

- `google-ads-pp-cli customers_campaign_bid_modifiers <customerId>` — Creates, updates, or removes campaign bid modifiers. Operation statuses are returned.

**customers_campaign_budgets** — Google Ads customers campaign budgets operations

- `google-ads-pp-cli customers_campaign_budgets <customerId>` — Creates, updates, or removes campaign budgets. Operation statuses are returned.

**customers_campaign_conversion_goals** — Google Ads customers campaign conversion goals operations

- `google-ads-pp-cli customers_campaign_conversion_goals <customerId>` — Creates, updates or removes campaign conversion goals. Operation statuses are returned.

**customers_campaign_criteria** — Google Ads customers campaign criteria operations

- `google-ads-pp-cli customers_campaign_criteria <customerId>` — Creates, updates, or removes criteria. Operation statuses are returned.

**customers_campaign_customizers** — Google Ads customers campaign customizers operations

- `google-ads-pp-cli customers_campaign_customizers <customerId>` — Creates, updates or removes campaign customizers. Operation statuses are returned.

**customers_campaign_drafts** — Google Ads customers campaign drafts operations

- `google-ads-pp-cli customers_campaign_drafts list_async_errors` — Returns all errors that occurred during CampaignDraft promote. Throws an error if called before campaign draft is...
- `google-ads-pp-cli customers_campaign_drafts mutate` — Creates, updates, or removes campaign drafts. Operation statuses are returned.
- `google-ads-pp-cli customers_campaign_drafts promote` — Promotes the changes in a draft back to the base campaign. This method returns a Long Running Operation (LRO)...

**customers_campaign_goal_configs** — Google Ads customers campaign goal configs operations

- `google-ads-pp-cli customers_campaign_goal_configs <customerId>` — Create or update campaign goal configs.

**customers_campaign_groups** — Google Ads customers campaign groups operations

- `google-ads-pp-cli customers_campaign_groups <customerId>` — Creates, updates, or removes campaign groups. Operation statuses are returned.

**customers_campaign_labels** — Google Ads customers campaign labels operations

- `google-ads-pp-cli customers_campaign_labels <customerId>` — Creates and removes campaign-label relationships. Operation statuses are returned.

**customers_campaign_lifecycle_goal** — Google Ads customers campaign lifecycle goal operations

- `google-ads-pp-cli customers_campaign_lifecycle_goal <customerId>` — Process the given campaign lifecycle configurations.

**customers_campaign_shared_sets** — Google Ads customers campaign shared sets operations

- `google-ads-pp-cli customers_campaign_shared_sets <customerId>` — Creates or removes campaign shared sets. Operation statuses are returned.

**customers_campaigns** — Google Ads customers campaigns operations

- `google-ads-pp-cli customers_campaigns enable_pmax_brand_guidelines` — Enables Brand Guidelines for Performance Max campaigns.
- `google-ads-pp-cli customers_campaigns mutate` — Creates, updates, or removes campaigns. Operation statuses are returned.

**customers_conversion_actions** — Google Ads customers conversion actions operations

- `google-ads-pp-cli customers_conversion_actions <customerId>` — Creates, updates or removes conversion actions. Operation statuses are returned.

**customers_conversion_custom_variables** — Google Ads customers conversion custom variables operations

- `google-ads-pp-cli customers_conversion_custom_variables <customerId>` — Creates or updates conversion custom variables. Operation statuses are returned.

**customers_conversion_goal_campaign_configs** — Google Ads customers conversion goal campaign configs operations

- `google-ads-pp-cli customers_conversion_goal_campaign_configs <customerId>` — Creates, updates or removes conversion goal campaign config. Operation statuses are returned.

**customers_conversion_value_rule_sets** — Google Ads customers conversion value rule sets operations

- `google-ads-pp-cli customers_conversion_value_rule_sets <customerId>` — Creates, updates or removes conversion value rule sets. Operation statuses are returned.

**customers_conversion_value_rules** — Google Ads customers conversion value rules operations

- `google-ads-pp-cli customers_conversion_value_rules <customerId>` — Creates, updates, or removes conversion value rules. Operation statuses are returned.

**customers_custom_audiences** — Google Ads customers custom audiences operations

- `google-ads-pp-cli customers_custom_audiences <customerId>` — Creates or updates custom audiences. Operation statuses are returned.

**customers_custom_conversion_goals** — Google Ads customers custom conversion goals operations

- `google-ads-pp-cli customers_custom_conversion_goals <customerId>` — Creates, updates or removes custom conversion goals. Operation statuses are returned.

**customers_custom_interests** — Google Ads customers custom interests operations

- `google-ads-pp-cli customers_custom_interests <customerId>` — Creates or updates custom interests. Operation statuses are returned.

**customers_customer_asset_sets** — Google Ads customers customer asset sets operations

- `google-ads-pp-cli customers_customer_asset_sets <customerId>` — Creates, or removes customer asset sets. Operation statuses are returned.

**customers_customer_assets** — Google Ads customers customer assets operations

- `google-ads-pp-cli customers_customer_assets <customerId>` — Creates, updates, or removes customer assets. Operation statuses are returned.

**customers_customer_client_links** — Google Ads customers customer client links operations

- `google-ads-pp-cli customers_customer_client_links <customerId>` — Creates or updates a customer client link. Operation statuses are returned.

**customers_customer_conversion_goals** — Google Ads customers customer conversion goals operations

- `google-ads-pp-cli customers_customer_conversion_goals <customerId>` — Creates, updates or removes customer conversion goals. Operation statuses are returned.

**customers_customer_customizers** — Google Ads customers customer customizers operations

- `google-ads-pp-cli customers_customer_customizers <customerId>` — Creates, updates or removes customer customizers. Operation statuses are returned.

**customers_customer_labels** — Google Ads customers customer labels operations

- `google-ads-pp-cli customers_customer_labels <customerId>` — Creates and removes customer-label relationships. Operation statuses are returned.

**customers_customer_lifecycle_goal** — Google Ads customers customer lifecycle goal operations

- `google-ads-pp-cli customers_customer_lifecycle_goal <customerId>` — Process the given customer lifecycle configurations.

**customers_customer_manager_links** — Google Ads customers customer manager links operations

- `google-ads-pp-cli customers_customer_manager_links move_manager_link` — Moves a client customer to a new manager customer. This simplifies the complex request that requires two operations...
- `google-ads-pp-cli customers_customer_manager_links mutate` — Updates customer manager links. Operation statuses are returned.

**customers_customer_negative_criteria** — Google Ads customers customer negative criteria operations

- `google-ads-pp-cli customers_customer_negative_criteria <customerId>` — Creates or removes criteria. Operation statuses are returned.

**customers_customer_sk_ad_network_conversion_value_schemas** — Google Ads customers customer sk ad network conversion value schemas operations

- `google-ads-pp-cli customers_customer_sk_ad_network_conversion_value_schemas <customerId>` — Creates or updates the CustomerSkAdNetworkConversionValueSchema.

**customers_customer_user_access_invitations** — Google Ads customers customer user access invitations operations

- `google-ads-pp-cli customers_customer_user_access_invitations <customerId>` — Creates or removes an access invitation.

**customers_customer_user_accesses** — Google Ads customers customer user accesses operations

- `google-ads-pp-cli customers_customer_user_accesses <customerId>` — Updates, removes permission of a user on a given customer. Operation statuses are returned.

**customers_customizer_attributes** — Google Ads customers customizer attributes operations

- `google-ads-pp-cli customers_customizer_attributes <customerId>` — Creates, updates or removes customizer attributes. Operation statuses are returned.

**customers_data_links** — Google Ads customers data links operations

- `google-ads-pp-cli customers_data_links create` — Creates a data link. The requesting Google Ads account name and account ID will be shared with the third party (such...
- `google-ads-pp-cli customers_data_links remove` — Remove a data link.
- `google-ads-pp-cli customers_data_links update` — Update a data link.

**customers_experiment_arms** — Google Ads customers experiment arms operations

- `google-ads-pp-cli customers_experiment_arms <customerId>` — Creates, updates, or removes experiment arms. Operation statuses are returned.

**customers_experiments** — Google Ads customers experiments operations

- `google-ads-pp-cli customers_experiments end_experiment` — Immediately ends an experiment, changing the experiment's scheduled end date and without waiting for end of day. End...
- `google-ads-pp-cli customers_experiments graduate_experiment` — Graduates an experiment to a full campaign.
- `google-ads-pp-cli customers_experiments list_experiment_async_errors` — Returns all errors that occurred during the last Experiment update (either scheduling or promotion). Supports...
- `google-ads-pp-cli customers_experiments mutate` — Creates, updates, or removes experiments. Operation statuses are returned.
- `google-ads-pp-cli customers_experiments promote_experiment` — Promotes the trial campaign thus applying changes in the trial campaign to the base campaign. This method returns a...
- `google-ads-pp-cli customers_experiments schedule_experiment` — Schedule an experiment. The in design campaign will be converted into a real campaign (called the experiment...

**customers_goals** — Google Ads customers goals operations

- `google-ads-pp-cli customers_goals <customerId>` — Create or update goals.

**customers_google_ads** — Google Ads customers google ads operations

- `google-ads-pp-cli customers_google_ads mutate` — Creates, updates, or removes resources. This method supports atomic transactions with multiple types of resources....
- `google-ads-pp-cli customers_google_ads search` — Returns all rows that match the search query.
- `google-ads-pp-cli customers_google_ads search_stream` — Returns all rows that match the search stream query.

**customers_invoices** — Google Ads customers invoices operations

- `google-ads-pp-cli customers_invoices <customerId>` — Returns all invoices associated with a billing setup, for a given month.

**customers_keyword_plan_ad_group_keywords** — Google Ads customers keyword plan ad group keywords operations

- `google-ads-pp-cli customers_keyword_plan_ad_group_keywords <customerId>` — Creates, updates, or removes Keyword Plan ad group keywords. Operation statuses are returned.

**customers_keyword_plan_ad_groups** — Google Ads customers keyword plan ad groups operations

- `google-ads-pp-cli customers_keyword_plan_ad_groups <customerId>` — Creates, updates, or removes Keyword Plan ad groups. Operation statuses are returned.

**customers_keyword_plan_campaign_keywords** — Google Ads customers keyword plan campaign keywords operations

- `google-ads-pp-cli customers_keyword_plan_campaign_keywords <customerId>` — Creates, updates, or removes Keyword Plan campaign keywords. Operation statuses are returned.

**customers_keyword_plan_campaigns** — Google Ads customers keyword plan campaigns operations

- `google-ads-pp-cli customers_keyword_plan_campaigns <customerId>` — Creates, updates, or removes Keyword Plan campaigns. Operation statuses are returned.

**customers_keyword_plans** — Google Ads customers keyword plans operations

- `google-ads-pp-cli customers_keyword_plans <customerId>` — Creates, updates, or removes keyword plans. Operation statuses are returned.

**customers_labels** — Google Ads customers labels operations

- `google-ads-pp-cli customers_labels <customerId>` — Creates, updates, or removes labels. Operation statuses are returned.

**customers_local_services** — Google Ads customers local services operations

- `google-ads-pp-cli customers_local_services <customerId>` — RPC to append Local Services Lead Conversation resources to Local Services Lead resources.

**customers_local_services_leads** — Google Ads customers local services leads operations

- `google-ads-pp-cli customers_local_services_leads <resourceName>` — RPC to provide feedback on Local Services Lead resources.

**customers_offline_user_data_jobs** — Google Ads customers offline user data jobs operations

- `google-ads-pp-cli customers_offline_user_data_jobs add_operations` — Adds operations to the offline user data job.
- `google-ads-pp-cli customers_offline_user_data_jobs create` — Creates an offline user data job.
- `google-ads-pp-cli customers_offline_user_data_jobs run` — Runs the offline user data job. When finished, the long running operation will contain the processing result or...

**customers_operations** — Google Ads customers operations operations

- `google-ads-pp-cli customers_operations cancel` — Starts asynchronous cancellation on a long-running operation. The server makes a best effort to cancel the...
- `google-ads-pp-cli customers_operations delete` — Deletes a long-running operation. This method indicates that the client is no longer interested in the operation...
- `google-ads-pp-cli customers_operations get` — Gets the latest state of a long-running operation. Clients can use this method to poll the operation result at...
- `google-ads-pp-cli customers_operations list` — Lists operations that match the specified filter in the request. If the server doesn't support this method, it...
- `google-ads-pp-cli customers_operations wait` — Waits until the specified long-running operation is done or reaches at most a specified timeout, returning the...

**customers_payments_accounts** — Google Ads customers payments accounts operations

- `google-ads-pp-cli customers_payments_accounts <customerId>` — Returns all payments accounts associated with all managers between the login customer ID and specified serving...

**customers_product_link_invitations** — Google Ads customers product link invitations operations

- `google-ads-pp-cli customers_product_link_invitations create` — Creates a product link invitation.
- `google-ads-pp-cli customers_product_link_invitations remove` — Remove a product link invitation.
- `google-ads-pp-cli customers_product_link_invitations update` — Update a product link invitation.

**customers_product_links** — Google Ads customers product links operations

- `google-ads-pp-cli customers_product_links create` — Creates a product link.
- `google-ads-pp-cli customers_product_links remove` — Removes a product link.

**customers_recommendation_subscriptions** — Google Ads customers recommendation subscriptions operations

- `google-ads-pp-cli customers_recommendation_subscriptions <customerId>` — Mutates given subscription with corresponding apply parameters.

**customers_recommendations** — Google Ads customers recommendations operations

- `google-ads-pp-cli customers_recommendations apply` — Applies given recommendations with corresponding apply parameters.
- `google-ads-pp-cli customers_recommendations dismiss` — Dismisses given recommendations.
- `google-ads-pp-cli customers_recommendations generate` — Generates Recommendations based off the requested recommendation_types.

**customers_remarketing_actions** — Google Ads customers remarketing actions operations

- `google-ads-pp-cli customers_remarketing_actions <customerId>` — Creates or updates remarketing actions. Operation statuses are returned.

**customers_shared_criteria** — Google Ads customers shared criteria operations

- `google-ads-pp-cli customers_shared_criteria <customerId>` — Creates or removes shared criteria. Operation statuses are returned.

**customers_shared_sets** — Google Ads customers shared sets operations

- `google-ads-pp-cli customers_shared_sets <customerId>` — Creates, updates, or removes shared sets. Operation statuses are returned.

**customers_smart_campaign_settings** — Google Ads customers smart campaign settings operations

- `google-ads-pp-cli customers_smart_campaign_settings get_smart_campaign_status` — Returns the status of the requested Smart campaign.
- `google-ads-pp-cli customers_smart_campaign_settings mutate` — Updates Smart campaign settings for campaigns.

**customers_third_party_app_analytics_links** — Google Ads customers third party app analytics links operations

- `google-ads-pp-cli customers_third_party_app_analytics_links <resourceName>` — Regenerate ThirdPartyAppAnalyticsLink.shareable_link_id that should be provided to the third party when setting up...

**customers_user_list_customer_types** — Google Ads customers user list customer types operations

- `google-ads-pp-cli customers_user_list_customer_types <customerId>` — Attach or remove user list customer types. Operation statuses are returned.

**customers_user_lists** — Google Ads customers user lists operations

- `google-ads-pp-cli customers_user_lists <customerId>` — Creates or updates user lists. Operation statuses are returned.

**geo_target_constants** — Google Ads geo target constants operations

- `google-ads-pp-cli geo_target_constants` — Returns GeoTargetConstant suggestions by location name or by resource name.

**google_ads** — Google Ads google ads operations

- `google-ads-pp-cli google_ads generate_conversion_rates` — Returns a collection of conversion rate suggestions for supported plannable products.
- `google-ads-pp-cli google_ads list_plannable_locations` — Returns the list of plannable locations (for example, countries).
- `google-ads-pp-cli google_ads list_plannable_products` — Returns the list of per-location plannable YouTube ad formats with allowed targeting.
- `google-ads-pp-cli google_ads list_plannable_user_interests` — Returns the list of plannable user interests. A plannable user interest is one that can be targeted in a reach...
- `google-ads-pp-cli google_ads list_plannable_user_lists` — Returns the list of plannable user lists with their plannable status. User lists may not be plannable for a number...

**google_ads_fields** — Google Ads google ads fields operations

- `google-ads-pp-cli google_ads_fields get` — Returns just the requested field.
- `google-ads-pp-cli google_ads_fields search` — Returns all fields that match the search query.

**keyword_theme_constants** — Google Ads keyword theme constants operations

- `google-ads-pp-cli keyword_theme_constants` — Returns KeywordThemeConstant suggestions by keyword themes.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-ads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your OAuth2 access token:

```bash
google-ads-pp-cli auth set-token YOUR_TOKEN_HERE
```

Also set `GOOGLE_ADS_DEVELOPER_TOKEN`; set `GOOGLE_ADS_LOGIN_CUSTOMER_ID` when calling through a manager account.

Run `google-ads-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-ads-pp-cli customers_google_ads search mock-value --agent --select id,name,status
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
google-ads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-ads-pp-cli feedback --stdin < notes.txt
google-ads-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.google-ads-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_ADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_ADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-ads-pp-cli profile save briefing --json
google-ads-pp-cli --profile briefing customers_google_ads search mock-value
google-ads-pp-cli profile list --json
google-ads-pp-cli profile show briefing
google-ads-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-ads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.26.3 or newer)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-ads/cmd/google-ads-pp-cli@latest
   ```
3. Verify: `google-ads-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-ads/cmd/google-ads-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-ads-pp-mcp -- google-ads-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-ads-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-ads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-ads-pp-cli <command> --help`.
