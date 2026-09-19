---
name: pp-bing-ads
description: "Printing Press CLI for Bing Ads. Reconstructed from the official BingAds-Python-SDK openapi_client (openapi-generator output)"
author: "ubuntu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bing-ads-pp-cli
    install:
      - kind: go
        bins: [bing-ads-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/cmd/bing-ads-pp-cli
---

# Bing Ads — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bing-ads-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bing-ads --cli-only
   ```
2. Verify: `bing-ads-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/cmd/bing-ads-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Reconstructed from the official BingAds-Python-SDK openapi_client (openapi-generator output) -- Microsoft does not publish a raw spec file. Covers all 6 Microsoft Advertising REST services (287 operations).

## Command Reference

**ad-insight** — Manage ad insight

- `bing-ads-pp-cli ad-insight apply-recommendations` — apply_recommendations
- `bing-ads-pp-cli ad-insight dismiss-recommendations` — dismiss_recommendations
- `bing-ads-pp-cli ad-insight get-auction-insight-data` — get_auction_insight_data
- `bing-ads-pp-cli ad-insight get-audience-breakdown` — get_audience_breakdown
- `bing-ads-pp-cli ad-insight get-audience-full-estimation` — get_audience_full_estimation
- `bing-ads-pp-cli ad-insight get-auto-apply-opt-in-status` — get_auto_apply_opt_in_status
- `bing-ads-pp-cli ad-insight get-bid-landscape-by-ad-group-ids` — get_bid_landscape_by_ad_group_ids
- `bing-ads-pp-cli ad-insight get-bid-landscape-by-campaign-ids` — get_bid_landscape_by_campaign_ids
- `bing-ads-pp-cli ad-insight get-bid-landscape-by-keyword-ids` — get_bid_landscape_by_keyword_ids
- `bing-ads-pp-cli ad-insight get-bid-opportunities` — get_bid_opportunities
- `bing-ads-pp-cli ad-insight get-budget-opportunities` — get_budget_opportunities
- `bing-ads-pp-cli ad-insight get-domain-categories` — get_domain_categories
- `bing-ads-pp-cli ad-insight get-estimated-bid-by-keyword-ids` — get_estimated_bid_by_keyword_ids
- `bing-ads-pp-cli ad-insight get-estimated-bid-by-keywords` — get_estimated_bid_by_keywords
- `bing-ads-pp-cli ad-insight get-estimated-position-by-keyword-ids` — get_estimated_position_by_keyword_ids
- `bing-ads-pp-cli ad-insight get-estimated-position-by-keywords` — get_estimated_position_by_keywords
- `bing-ads-pp-cli ad-insight get-historical-keyword-performance` — get_historical_keyword_performance
- `bing-ads-pp-cli ad-insight get-historical-search-count` — get_historical_search_count
- `bing-ads-pp-cli ad-insight get-keyword-categories` — get_keyword_categories
- `bing-ads-pp-cli ad-insight get-keyword-demographics` — get_keyword_demographics
- `bing-ads-pp-cli ad-insight get-keyword-idea-categories` — get_keyword_idea_categories
- `bing-ads-pp-cli ad-insight get-keyword-ideas` — get_keyword_ideas
- `bing-ads-pp-cli ad-insight get-keyword-locations` — get_keyword_locations
- `bing-ads-pp-cli ad-insight get-keyword-opportunities` — get_keyword_opportunities
- `bing-ads-pp-cli ad-insight get-keyword-traffic-estimates` — get_keyword_traffic_estimates
- `bing-ads-pp-cli ad-insight get-performance-insights-detail-data-by-account-id` — get_performance_insights_detail_data_by_account_id
- `bing-ads-pp-cli ad-insight get-recommendations` — get_recommendations
- `bing-ads-pp-cli ad-insight get-text-asset-suggestions-by-final-urls` — get_text_asset_suggestions_by_final_urls
- `bing-ads-pp-cli ad-insight retrieve-recommendations` — retrieve_recommendations
- `bing-ads-pp-cli ad-insight set-auto-apply-opt-in-status` — set_auto_apply_opt_in_status
- `bing-ads-pp-cli ad-insight suggest-keywords-for-url` — suggest_keywords_for_url
- `bing-ads-pp-cli ad-insight suggest-keywords-from-existing-keywords` — suggest_keywords_from_existing_keywords
- `bing-ads-pp-cli ad-insight tag-recommendations` — tag_recommendations

**bulk** — Manage bulk

- `bing-ads-pp-cli bulk download-campaigns-by-account-ids` — download_campaigns_by_account_ids
- `bing-ads-pp-cli bulk download-campaigns-by-campaign-ids` — download_campaigns_by_campaign_ids
- `bing-ads-pp-cli bulk get-download-status` — get_bulk_download_status
- `bing-ads-pp-cli bulk get-upload-status` — get_bulk_upload_status
- `bing-ads-pp-cli bulk get-upload-url` — get_bulk_upload_url
- `bing-ads-pp-cli bulk upload-entity-records` — upload_entity_records

**campaign-management** — Manage campaign management

- `bing-ads-pp-cli campaign-management add-ad-extensions` — add_ad_extensions
- `bing-ads-pp-cli campaign-management add-ad-group-criterions` — add_ad_group_criterions
- `bing-ads-pp-cli campaign-management add-ad-groups` — add_ad_groups
- `bing-ads-pp-cli campaign-management add-ads` — add_ads
- `bing-ads-pp-cli campaign-management add-asset-groups` — add_asset_groups
- `bing-ads-pp-cli campaign-management add-audience-groups` — add_audience_groups
- `bing-ads-pp-cli campaign-management add-audiences` — add_audiences
- `bing-ads-pp-cli campaign-management add-bid-strategies` — add_bid_strategies
- `bing-ads-pp-cli campaign-management add-brand-kits` — add_brand_kits
- `bing-ads-pp-cli campaign-management add-budgets` — add_budgets
- `bing-ads-pp-cli campaign-management add-campaign-conversion-goals` — add_campaign_conversion_goals
- `bing-ads-pp-cli campaign-management add-campaign-criterions` — add_campaign_criterions
- `bing-ads-pp-cli campaign-management add-campaigns` — add_campaigns
- `bing-ads-pp-cli campaign-management add-conversion-goals` — add_conversion_goals
- `bing-ads-pp-cli campaign-management add-conversion-value-rules` — add_conversion_value_rules
- `bing-ads-pp-cli campaign-management add-data-exclusions` — add_data_exclusions
- `bing-ads-pp-cli campaign-management add-experiments` — add_experiments
- `bing-ads-pp-cli campaign-management add-html5s` — add_html5s
- `bing-ads-pp-cli campaign-management add-import-jobs` — add_import_jobs
- `bing-ads-pp-cli campaign-management add-keywords` — add_keywords
- `bing-ads-pp-cli campaign-management add-labels` — add_labels
- `bing-ads-pp-cli campaign-management add-linked-in-segments` — add_linked_in_segments
- `bing-ads-pp-cli campaign-management add-list-items-to-shared-list` — add_list_items_to_shared_list
- `bing-ads-pp-cli campaign-management add-media` — add_media
- `bing-ads-pp-cli campaign-management add-negative-keywords-to-entities` — add_negative_keywords_to_entities
- `bing-ads-pp-cli campaign-management add-new-customer-acquisition-goals` — add_new_customer_acquisition_goals
- `bing-ads-pp-cli campaign-management add-seasonality-adjustments` — add_seasonality_adjustments
- `bing-ads-pp-cli campaign-management add-shared-entity` — add_shared_entity
- `bing-ads-pp-cli campaign-management add-uet-tags` — add_uet_tags
- `bing-ads-pp-cli campaign-management add-videos` — add_videos
- `bing-ads-pp-cli campaign-management appeal-editorial-rejections` — appeal_editorial_rejections
- `bing-ads-pp-cli campaign-management apply-asset-group-listing-group-actions` — apply_asset_group_listing_group_actions
- `bing-ads-pp-cli campaign-management apply-customer-list-items` — apply_customer_list_items
- `bing-ads-pp-cli campaign-management apply-customer-list-user-data` — apply_customer_list_user_data
- `bing-ads-pp-cli campaign-management apply-hotel-group-actions` — apply_hotel_group_actions
- `bing-ads-pp-cli campaign-management apply-offline-conversion-adjustments` — apply_offline_conversion_adjustments
- `bing-ads-pp-cli campaign-management apply-offline-conversions` — apply_offline_conversions
- `bing-ads-pp-cli campaign-management apply-online-conversion-adjustments` — apply_online_conversion_adjustments
- `bing-ads-pp-cli campaign-management apply-product-partition-actions` — apply_product_partition_actions
- `bing-ads-pp-cli campaign-management create-asset-group-recommendation` — create_asset_group_recommendation
- `bing-ads-pp-cli campaign-management create-brand-kit-recommendation` — create_brand_kit_recommendation
- `bing-ads-pp-cli campaign-management create-responsive-ad-recommendation` — create_responsive_ad_recommendation
- `bing-ads-pp-cli campaign-management create-responsive-search-ad-recommendation` — create_responsive_search_ad_recommendation
- `bing-ads-pp-cli campaign-management delete-ad-extensions` — delete_ad_extensions
- `bing-ads-pp-cli campaign-management delete-ad-extensions-associations` — delete_ad_extensions_associations
- `bing-ads-pp-cli campaign-management delete-ad-group-criterions` — delete_ad_group_criterions
- `bing-ads-pp-cli campaign-management delete-ad-groups` — delete_ad_groups
- `bing-ads-pp-cli campaign-management delete-ads` — delete_ads
- `bing-ads-pp-cli campaign-management delete-asset-groups` — delete_asset_groups
- `bing-ads-pp-cli campaign-management delete-audience-group-asset-group-associations` — delete_audience_group_asset_group_associations
- `bing-ads-pp-cli campaign-management delete-audience-groups` — delete_audience_groups
- `bing-ads-pp-cli campaign-management delete-audiences` — delete_audiences
- `bing-ads-pp-cli campaign-management delete-bid-strategies` — delete_bid_strategies
- `bing-ads-pp-cli campaign-management delete-brand-kits` — delete_brand_kits
- `bing-ads-pp-cli campaign-management delete-budgets` — delete_budgets
- `bing-ads-pp-cli campaign-management delete-campaign-conversion-goals` — delete_campaign_conversion_goals
- `bing-ads-pp-cli campaign-management delete-campaign-criterions` — delete_campaign_criterions
- `bing-ads-pp-cli campaign-management delete-campaigns` — delete_campaigns
- `bing-ads-pp-cli campaign-management delete-data-exclusions` — delete_data_exclusions
- `bing-ads-pp-cli campaign-management delete-experiments` — delete_experiments
- `bing-ads-pp-cli campaign-management delete-html5s` — delete_html5s
- `bing-ads-pp-cli campaign-management delete-import-jobs` — delete_import_jobs
- `bing-ads-pp-cli campaign-management delete-keywords` — delete_keywords
- `bing-ads-pp-cli campaign-management delete-label-associations` — delete_label_associations
- `bing-ads-pp-cli campaign-management delete-labels` — delete_labels
- `bing-ads-pp-cli campaign-management delete-linked-in-segments` — delete_linked_in_segments
- `bing-ads-pp-cli campaign-management delete-list-items-from-shared-list` — delete_list_items_from_shared_list
- `bing-ads-pp-cli campaign-management delete-media` — delete_media
- `bing-ads-pp-cli campaign-management delete-negative-keywords-from-entities` — delete_negative_keywords_from_entities
- `bing-ads-pp-cli campaign-management delete-seasonality-adjustments` — delete_seasonality_adjustments
- `bing-ads-pp-cli campaign-management delete-shared-entities` — delete_shared_entities
- `bing-ads-pp-cli campaign-management delete-shared-entity-associations` — delete_shared_entity_associations
- `bing-ads-pp-cli campaign-management delete-videos` — delete_videos
- `bing-ads-pp-cli campaign-management get-account-migration-statuses` — get_account_migration_statuses
- `bing-ads-pp-cli campaign-management get-account-properties` — get_account_properties
- `bing-ads-pp-cli campaign-management get-ad-extension-ids-by-account-id` — get_ad_extension_ids_by_account_id
- `bing-ads-pp-cli campaign-management get-ad-extensions-associations` — get_ad_extensions_associations
- `bing-ads-pp-cli campaign-management get-ad-extensions-by-ids` — get_ad_extensions_by_ids
- `bing-ads-pp-cli campaign-management get-ad-extensions-editorial-reasons` — get_ad_extensions_editorial_reasons
- `bing-ads-pp-cli campaign-management get-ad-group-criterions-by-ids` — get_ad_group_criterions_by_ids
- `bing-ads-pp-cli campaign-management get-ad-groups-by-campaign-id` — get_ad_groups_by_campaign_id
- `bing-ads-pp-cli campaign-management get-ad-groups-by-ids` — get_ad_groups_by_ids
- `bing-ads-pp-cli campaign-management get-ads-by-ad-group-id` — get_ads_by_ad_group_id
- `bing-ads-pp-cli campaign-management get-ads-by-editorial-status` — get_ads_by_editorial_status
- `bing-ads-pp-cli campaign-management get-ads-by-ids` — get_ads_by_ids
- `bing-ads-pp-cli campaign-management get-annotation-opt-out` — get_annotation_opt_out
- `bing-ads-pp-cli campaign-management get-asset-group-listing-groups-by-ids` — get_asset_group_listing_groups_by_ids
- `bing-ads-pp-cli campaign-management get-asset-groups-by-campaign-id` — get_asset_groups_by_campaign_id
- `bing-ads-pp-cli campaign-management get-asset-groups-by-ids` — get_asset_groups_by_ids
- `bing-ads-pp-cli campaign-management get-asset-groups-editorial-reasons` — get_asset_groups_editorial_reasons
- `bing-ads-pp-cli campaign-management get-audience-group-asset-group-associations-by-asset-group-ids` — get_audience_group_asset_group_associations_by_asset_group_ids
- `bing-ads-pp-cli campaign-management get-audience-group-asset-group-associations-by-audience-group-ids` — get_audience_group_asset_group_associations_by_audience_group_ids
- `bing-ads-pp-cli campaign-management get-audience-groups-by-ids` — get_audience_groups_by_ids
- `bing-ads-pp-cli campaign-management get-audiences-by-ids` — get_audiences_by_ids
- `bing-ads-pp-cli campaign-management get-bid-strategies-by-ids` — get_bid_strategies_by_ids
- `bing-ads-pp-cli campaign-management get-bmc-stores-by-customer-id` — get_bmc_stores_by_customer_id
- `bing-ads-pp-cli campaign-management get-brand-kits-by-account-id` — get_brand_kits_by_account_id
- `bing-ads-pp-cli campaign-management get-brand-kits-by-ids` — get_brand_kits_by_ids
- `bing-ads-pp-cli campaign-management get-bsc-countries` — get_bsc_countries
- `bing-ads-pp-cli campaign-management get-budgets-by-ids` — get_budgets_by_ids
- `bing-ads-pp-cli campaign-management get-campaign-criterions-by-ids` — get_campaign_criterions_by_ids
- `bing-ads-pp-cli campaign-management get-campaign-ids-by-bid-strategy-ids` — get_campaign_ids_by_bid_strategy_ids
- `bing-ads-pp-cli campaign-management get-campaign-ids-by-budget-ids` — get_campaign_ids_by_budget_ids
- `bing-ads-pp-cli campaign-management get-campaign-sizes-by-account-id` — get_campaign_sizes_by_account_id
- `bing-ads-pp-cli campaign-management get-campaigns-by-account-id` — get_campaigns_by_account_id
- `bing-ads-pp-cli campaign-management get-campaigns-by-ids` — get_campaigns_by_ids
- `bing-ads-pp-cli campaign-management get-clipchamp-templates` — get_clipchamp_templates
- `bing-ads-pp-cli campaign-management get-config-value` — get_config_value
- `bing-ads-pp-cli campaign-management get-conversion-goals-by-ids` — get_conversion_goals_by_ids
- `bing-ads-pp-cli campaign-management get-conversion-goals-by-tag-ids` — get_conversion_goals_by_tag_ids
- `bing-ads-pp-cli campaign-management get-conversion-value-rules-by-account-id` — get_conversion_value_rules_by_account_id
- `bing-ads-pp-cli campaign-management get-conversion-value-rules-by-ids` — get_conversion_value_rules_by_ids
- `bing-ads-pp-cli campaign-management get-data-exclusions-by-account-id` — get_data_exclusions_by_account_id
- `bing-ads-pp-cli campaign-management get-data-exclusions-by-ids` — get_data_exclusions_by_ids
- `bing-ads-pp-cli campaign-management get-diagnostics` — get_diagnostics
- `bing-ads-pp-cli campaign-management get-editorial-reasons-by-ids` — get_editorial_reasons_by_ids
- `bing-ads-pp-cli campaign-management get-experiments-by-ids` — get_experiments_by_ids
- `bing-ads-pp-cli campaign-management get-file-import-upload-url` — get_file_import_upload_url
- `bing-ads-pp-cli campaign-management get-geo-locations-file-url` — get_geo_locations_file_url
- `bing-ads-pp-cli campaign-management get-health-check` — get_health_check
- `bing-ads-pp-cli campaign-management get-html5s-by-ids` — get_html5s_by_ids
- `bing-ads-pp-cli campaign-management get-import-entity-ids-mapping` — get_import_entity_ids_mapping
- `bing-ads-pp-cli campaign-management get-import-jobs-by-ids` — get_import_jobs_by_ids
- `bing-ads-pp-cli campaign-management get-import-results` — get_import_results
- `bing-ads-pp-cli campaign-management get-keywords-by-ad-group-id` — get_keywords_by_ad_group_id
- `bing-ads-pp-cli campaign-management get-keywords-by-editorial-status` — get_keywords_by_editorial_status
- `bing-ads-pp-cli campaign-management get-keywords-by-ids` — get_keywords_by_ids
- `bing-ads-pp-cli campaign-management get-label-associations-by-entity-ids` — get_label_associations_by_entity_ids
- `bing-ads-pp-cli campaign-management get-label-associations-by-label-ids` — get_label_associations_by_label_ids
- `bing-ads-pp-cli campaign-management get-labels-by-ids` — get_labels_by_ids
- `bing-ads-pp-cli campaign-management get-list-items-by-shared-list` — get_list_items_by_shared_list
- `bing-ads-pp-cli campaign-management get-media-associations` — get_media_associations
- `bing-ads-pp-cli campaign-management get-media-meta-data-by-account-id` — get_media_meta_data_by_account_id
- `bing-ads-pp-cli campaign-management get-media-meta-data-by-ids` — get_media_meta_data_by_ids
- `bing-ads-pp-cli campaign-management get-negative-keywords-by-entity-ids` — get_negative_keywords_by_entity_ids
- `bing-ads-pp-cli campaign-management get-negative-sites-by-ad-group-ids` — get_negative_sites_by_ad_group_ids
- `bing-ads-pp-cli campaign-management get-negative-sites-by-campaign-ids` — get_negative_sites_by_campaign_ids
- `bing-ads-pp-cli campaign-management get-new-customer-acquisition-goals-by-account-id` — get_new_customer_acquisition_goals_by_account_id
- `bing-ads-pp-cli campaign-management get-offline-conversion-report-by-goal-ids` — get_offline_conversion_report_by_goal_ids
- `bing-ads-pp-cli campaign-management get-offline-conversion-reports` — get_offline_conversion_reports
- `bing-ads-pp-cli campaign-management get-profile-data-file-url` — get_profile_data_file_url
- `bing-ads-pp-cli campaign-management get-responsive-ad-recommendation-job` — get_responsive_ad_recommendation_job
- `bing-ads-pp-cli campaign-management get-seasonality-adjustments-by-account-id` — get_seasonality_adjustments_by_account_id
- `bing-ads-pp-cli campaign-management get-seasonality-adjustments-by-ids` — get_seasonality_adjustments_by_ids
- `bing-ads-pp-cli campaign-management get-shared-entities` — get_shared_entities
- `bing-ads-pp-cli campaign-management get-shared-entities-by-account-id` — get_shared_entities_by_account_id
- `bing-ads-pp-cli campaign-management get-shared-entity-associations-by-entity-ids` — get_shared_entity_associations_by_entity_ids
- `bing-ads-pp-cli campaign-management get-shared-entity-associations-by-shared-entity-ids` — get_shared_entity_associations_by_shared_entity_ids
- `bing-ads-pp-cli campaign-management get-supported-clipchamp-audio` — get_supported_clipchamp_audio
- `bing-ads-pp-cli campaign-management get-supported-fonts` — get_supported_fonts
- `bing-ads-pp-cli campaign-management get-uet-tag-auth-key` — get_uet_tag_auth_key
- `bing-ads-pp-cli campaign-management get-uet-tags-by-ids` — get_uet_tags_by_ids
- `bing-ads-pp-cli campaign-management get-videos-by-ids` — get_videos_by_ids
- `bing-ads-pp-cli campaign-management refine-asset-group-recommendation` — refine_asset_group_recommendation
- `bing-ads-pp-cli campaign-management refine-responsive-ad-recommendation` — refine_responsive_ad_recommendation
- `bing-ads-pp-cli campaign-management refine-responsive-search-ad-recommendation` — refine_responsive_search_ad_recommendation
- `bing-ads-pp-cli campaign-management search-companies` — search_companies
- `bing-ads-pp-cli campaign-management set-account-properties` — set_account_properties
- `bing-ads-pp-cli campaign-management set-ad-extensions-associations` — set_ad_extensions_associations
- `bing-ads-pp-cli campaign-management set-audience-group-asset-group-associations` — set_audience_group_asset_group_associations
- `bing-ads-pp-cli campaign-management set-label-associations` — set_label_associations
- `bing-ads-pp-cli campaign-management set-negative-sites-to-ad-groups` — set_negative_sites_to_ad_groups
- `bing-ads-pp-cli campaign-management set-negative-sites-to-campaigns` — set_negative_sites_to_campaigns
- `bing-ads-pp-cli campaign-management set-shared-entity-associations` — set_shared_entity_associations
- `bing-ads-pp-cli campaign-management update-ad-extensions` — update_ad_extensions
- `bing-ads-pp-cli campaign-management update-ad-group-criterions` — update_ad_group_criterions
- `bing-ads-pp-cli campaign-management update-ad-groups` — update_ad_groups
- `bing-ads-pp-cli campaign-management update-ads` — update_ads
- `bing-ads-pp-cli campaign-management update-annotation-opt-out` — update_annotation_opt_out
- `bing-ads-pp-cli campaign-management update-asset-groups` — update_asset_groups
- `bing-ads-pp-cli campaign-management update-audience-groups` — update_audience_groups
- `bing-ads-pp-cli campaign-management update-audiences` — update_audiences
- `bing-ads-pp-cli campaign-management update-bid-strategies` — update_bid_strategies
- `bing-ads-pp-cli campaign-management update-brand-kits` — update_brand_kits
- `bing-ads-pp-cli campaign-management update-budgets` — update_budgets
- `bing-ads-pp-cli campaign-management update-campaign-criterions` — update_campaign_criterions
- `bing-ads-pp-cli campaign-management update-campaigns` — update_campaigns
- `bing-ads-pp-cli campaign-management update-conversion-goals` — update_conversion_goals
- `bing-ads-pp-cli campaign-management update-conversion-value-rules` — update_conversion_value_rules
- `bing-ads-pp-cli campaign-management update-conversion-value-rules-status` — update_conversion_value_rules_status
- `bing-ads-pp-cli campaign-management update-data-exclusions` — update_data_exclusions
- `bing-ads-pp-cli campaign-management update-experiments` — update_experiments
- `bing-ads-pp-cli campaign-management update-import-jobs` — update_import_jobs
- `bing-ads-pp-cli campaign-management update-keywords` — update_keywords
- `bing-ads-pp-cli campaign-management update-labels` — update_labels
- `bing-ads-pp-cli campaign-management update-linked-in-segments` — update_linked_in_segments
- `bing-ads-pp-cli campaign-management update-new-customer-acquisition-goals` — update_new_customer_acquisition_goals
- `bing-ads-pp-cli campaign-management update-seasonality-adjustments` — update_seasonality_adjustments
- `bing-ads-pp-cli campaign-management update-shared-entities` — update_shared_entities
- `bing-ads-pp-cli campaign-management update-uet-tags` — update_uet_tags
- `bing-ads-pp-cli campaign-management update-videos` — update_videos

**customer-billing** — Manage customer billing

- `bing-ads-pp-cli customer-billing add-insertion-order` — add_insertion_order
- `bing-ads-pp-cli customer-billing check-feature-adoption-coupon-eligibility` — check_feature_adoption_coupon_eligibility
- `bing-ads-pp-cli customer-billing claim-feature-adoption-coupons` — claim_feature_adoption_coupons
- `bing-ads-pp-cli customer-billing dispatch-coupons` — dispatch_coupons
- `bing-ads-pp-cli customer-billing distribute-coupons` — distribute_coupons
- `bing-ads-pp-cli customer-billing get-account-monthly-spend` — get_account_monthly_spend
- `bing-ads-pp-cli customer-billing get-billing-documents` — get_billing_documents
- `bing-ads-pp-cli customer-billing get-billing-documents-info` — get_billing_documents_info
- `bing-ads-pp-cli customer-billing get-billing-groups` — get_billing_groups
- `bing-ads-pp-cli customer-billing get-coupon-info` — get_coupon_info
- `bing-ads-pp-cli customer-billing get-ungrouped-accounts` — get_ungrouped_accounts
- `bing-ads-pp-cli customer-billing redeem-coupon` — redeem_coupon
- `bing-ads-pp-cli customer-billing search-coupons` — search_coupons
- `bing-ads-pp-cli customer-billing search-insertion-orders` — search_insertion_orders
- `bing-ads-pp-cli customer-billing update-billing-group-accounts` — update_billing_group_accounts
- `bing-ads-pp-cli customer-billing update-insertion-order` — update_insertion_order

**customer-management** — Manage customer management

- `bing-ads-pp-cli customer-management add-account` — add_account
- `bing-ads-pp-cli customer-management add-client-links` — add_client_links
- `bing-ads-pp-cli customer-management add-prepay-account` — add_prepay_account
- `bing-ads-pp-cli customer-management delete-account` — delete_account
- `bing-ads-pp-cli customer-management delete-customer` — delete_customer
- `bing-ads-pp-cli customer-management delete-user` — delete_user
- `bing-ads-pp-cli customer-management dismiss-notifications` — dismiss_notifications
- `bing-ads-pp-cli customer-management find-accounts` — find_accounts
- `bing-ads-pp-cli customer-management find-accounts-or-customers-info` — find_accounts_or_customers_info
- `bing-ads-pp-cli customer-management get-accessible-customer` — get_accessible_customer
- `bing-ads-pp-cli customer-management get-account` — get_account
- `bing-ads-pp-cli customer-management get-account-pilot-features` — get_account_pilot_features
- `bing-ads-pp-cli customer-management get-accounts-info` — get_accounts_info
- `bing-ads-pp-cli customer-management get-current-user` — get_current_user
- `bing-ads-pp-cli customer-management get-customer` — get_customer
- `bing-ads-pp-cli customer-management get-customer-pilot-features` — get_customer_pilot_features
- `bing-ads-pp-cli customer-management get-customers-info` — get_customers_info
- `bing-ads-pp-cli customer-management get-linked-accounts-and-customers-info` — get_linked_accounts_and_customers_info
- `bing-ads-pp-cli customer-management get-notifications` — get_notifications
- `bing-ads-pp-cli customer-management get-pilot-features-countries` — get_pilot_features_countries
- `bing-ads-pp-cli customer-management get-user` — get_user
- `bing-ads-pp-cli customer-management get-user-mfa-status` — get_user_mfa_status
- `bing-ads-pp-cli customer-management get-users-info` — get_users_info
- `bing-ads-pp-cli customer-management map-account-id-to-external-account-ids` — map_account_id_to_external_account_ids
- `bing-ads-pp-cli customer-management map-customer-id-to-external-customer-id` — map_customer_id_to_external_customer_id
- `bing-ads-pp-cli customer-management search-accounts` — search_accounts
- `bing-ads-pp-cli customer-management search-client-links` — search_client_links
- `bing-ads-pp-cli customer-management search-customers` — search_customers
- `bing-ads-pp-cli customer-management search-user-invitations` — search_user_invitations
- `bing-ads-pp-cli customer-management send-user-invitation` — send_user_invitation
- `bing-ads-pp-cli customer-management signup-customer` — signup_customer
- `bing-ads-pp-cli customer-management update-account` — update_account
- `bing-ads-pp-cli customer-management update-client-links` — update_client_links
- `bing-ads-pp-cli customer-management update-customer` — update_customer
- `bing-ads-pp-cli customer-management update-prepay-account` — update_prepay_account
- `bing-ads-pp-cli customer-management update-user` — update_user
- `bing-ads-pp-cli customer-management update-user-roles` — update_user_roles
- `bing-ads-pp-cli customer-management upgrade-customer-to-agency` — upgrade_customer_to_agency
- `bing-ads-pp-cli customer-management validate-address` — validate_address

**reporting** — Manage reporting

- `bing-ads-pp-cli reporting poll-generate-report` — poll_generate_report
- `bing-ads-pp-cli reporting submit-generate-report` — submit_generate_report


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bing-ads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `bing-ads-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export BING_ADS_CUSTOMER_ACCOUNT_ID="<your-key>"
```
To persist credentials, use `echo "$TOKEN" | bing-ads-pp-cli auth set-token`. Stored secrets live in `credentials.toml` under the data dir, not in `config.toml`.

Run `bing-ads-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bing-ads-pp-cli ad-insight get-auction-insight-data --agent --select Result
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `BING_ADS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BING_ADS_CONFIG_DIR`, `BING_ADS_DATA_DIR`, `BING_ADS_STATE_DIR`, `BING_ADS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BING_ADS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `bing-ads-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "bing-ads": {
        "command": "bing-ads-pp-mcp",
        "env": {
          "BING_ADS_HOME": "/srv/bing-ads"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BING_ADS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BING_ADS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
bing-ads-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "bing-ads-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `bing-ads-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `bing-ads-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
bing-ads-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
bing-ads-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
bing-ads-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
bing-ads-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`bing-ads-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `BING_ADS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bing-ads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bing-ads-pp-cli feedback --stdin < notes.txt
bing-ads-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BING_ADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BING_ADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
bing-ads-pp-cli profile save briefing --json
bing-ads-pp-cli --profile briefing ad-insight get-auction-insight-data
bing-ads-pp-cli profile list --json
bing-ads-pp-cli profile show briefing
bing-ads-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bing-ads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/cmd/bing-ads-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bing-ads-pp-mcp -- bing-ads-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bing-ads-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bing-ads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bing-ads-pp-cli <command> --help`.
