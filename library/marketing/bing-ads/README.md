# Bing Ads CLI

Reconstructed from the official BingAds-Python-SDK openapi_client (openapi-generator output) -- Microsoft does not publish a raw spec file. Covers all 6 Microsoft Advertising REST services (287 operations).

## Install

The recommended path installs both the `bing-ads-pp-cli` binary and the `pp-bing-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bing-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bing-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bing-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bing-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install bing-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/cmd/bing-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bing-ads --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bing-ads --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bing-ads --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bing-ads --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BING_ADS_CUSTOMER_ACCOUNT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/cmd/bing-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bing-ads": {
      "command": "bing-ads-pp-mcp",
      "env": {
        "BING_ADS_CUSTOMER_ACCOUNT_ID": "<your-key>"
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

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export BING_ADS_CUSTOMER_ACCOUNT_ID="<paste-your-key>"
```
To persist credentials, use `echo "$TOKEN" | bing-ads-pp-cli auth set-token`. Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
bing-ads-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
bing-ads-pp-cli ad-insight get-auction-insight-data
```

## Usage

Run `bing-ads-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BING_ADS_CONFIG_DIR`, `BING_ADS_DATA_DIR`, `BING_ADS_STATE_DIR`, or `BING_ADS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BING_ADS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BING_ADS_HOME=/srv/bing-ads
bing-ads-pp-cli doctor
```

Under `BING_ADS_HOME=/srv/bing-ads`, the four dirs resolve to `/srv/bing-ads/config`, `/srv/bing-ads/data`, `/srv/bing-ads/state`, and `/srv/bing-ads/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `BING_ADS_DATA_DIR` overrides an explicit `--home` for that kind. Use `BING_ADS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BING_ADS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `bing-ads-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### ad-insight

Manage ad insight

- **`bing-ads-pp-cli ad-insight apply-recommendations`** - apply_recommendations
- **`bing-ads-pp-cli ad-insight dismiss-recommendations`** - dismiss_recommendations
- **`bing-ads-pp-cli ad-insight get-auction-insight-data`** - get_auction_insight_data
- **`bing-ads-pp-cli ad-insight get-audience-breakdown`** - get_audience_breakdown
- **`bing-ads-pp-cli ad-insight get-audience-full-estimation`** - get_audience_full_estimation
- **`bing-ads-pp-cli ad-insight get-auto-apply-opt-in-status`** - get_auto_apply_opt_in_status
- **`bing-ads-pp-cli ad-insight get-bid-landscape-by-ad-group-ids`** - get_bid_landscape_by_ad_group_ids
- **`bing-ads-pp-cli ad-insight get-bid-landscape-by-campaign-ids`** - get_bid_landscape_by_campaign_ids
- **`bing-ads-pp-cli ad-insight get-bid-landscape-by-keyword-ids`** - get_bid_landscape_by_keyword_ids
- **`bing-ads-pp-cli ad-insight get-bid-opportunities`** - get_bid_opportunities
- **`bing-ads-pp-cli ad-insight get-budget-opportunities`** - get_budget_opportunities
- **`bing-ads-pp-cli ad-insight get-domain-categories`** - get_domain_categories
- **`bing-ads-pp-cli ad-insight get-estimated-bid-by-keyword-ids`** - get_estimated_bid_by_keyword_ids
- **`bing-ads-pp-cli ad-insight get-estimated-bid-by-keywords`** - get_estimated_bid_by_keywords
- **`bing-ads-pp-cli ad-insight get-estimated-position-by-keyword-ids`** - get_estimated_position_by_keyword_ids
- **`bing-ads-pp-cli ad-insight get-estimated-position-by-keywords`** - get_estimated_position_by_keywords
- **`bing-ads-pp-cli ad-insight get-historical-keyword-performance`** - get_historical_keyword_performance
- **`bing-ads-pp-cli ad-insight get-historical-search-count`** - get_historical_search_count
- **`bing-ads-pp-cli ad-insight get-keyword-categories`** - get_keyword_categories
- **`bing-ads-pp-cli ad-insight get-keyword-demographics`** - get_keyword_demographics
- **`bing-ads-pp-cli ad-insight get-keyword-idea-categories`** - get_keyword_idea_categories
- **`bing-ads-pp-cli ad-insight get-keyword-ideas`** - get_keyword_ideas
- **`bing-ads-pp-cli ad-insight get-keyword-locations`** - get_keyword_locations
- **`bing-ads-pp-cli ad-insight get-keyword-opportunities`** - get_keyword_opportunities
- **`bing-ads-pp-cli ad-insight get-keyword-traffic-estimates`** - get_keyword_traffic_estimates
- **`bing-ads-pp-cli ad-insight get-performance-insights-detail-data-by-account-id`** - get_performance_insights_detail_data_by_account_id
- **`bing-ads-pp-cli ad-insight get-recommendations`** - get_recommendations
- **`bing-ads-pp-cli ad-insight get-text-asset-suggestions-by-final-urls`** - get_text_asset_suggestions_by_final_urls
- **`bing-ads-pp-cli ad-insight retrieve-recommendations`** - retrieve_recommendations
- **`bing-ads-pp-cli ad-insight set-auto-apply-opt-in-status`** - set_auto_apply_opt_in_status
- **`bing-ads-pp-cli ad-insight suggest-keywords-for-url`** - suggest_keywords_for_url
- **`bing-ads-pp-cli ad-insight suggest-keywords-from-existing-keywords`** - suggest_keywords_from_existing_keywords
- **`bing-ads-pp-cli ad-insight tag-recommendations`** - tag_recommendations

### bulk

Manage bulk

- **`bing-ads-pp-cli bulk download-campaigns-by-account-ids`** - download_campaigns_by_account_ids
- **`bing-ads-pp-cli bulk download-campaigns-by-campaign-ids`** - download_campaigns_by_campaign_ids
- **`bing-ads-pp-cli bulk get-download-status`** - get_bulk_download_status
- **`bing-ads-pp-cli bulk get-upload-status`** - get_bulk_upload_status
- **`bing-ads-pp-cli bulk get-upload-url`** - get_bulk_upload_url
- **`bing-ads-pp-cli bulk upload-entity-records`** - upload_entity_records

### campaign-management

Manage campaign management

- **`bing-ads-pp-cli campaign-management add-ad-extensions`** - add_ad_extensions
- **`bing-ads-pp-cli campaign-management add-ad-group-criterions`** - add_ad_group_criterions
- **`bing-ads-pp-cli campaign-management add-ad-groups`** - add_ad_groups
- **`bing-ads-pp-cli campaign-management add-ads`** - add_ads
- **`bing-ads-pp-cli campaign-management add-asset-groups`** - add_asset_groups
- **`bing-ads-pp-cli campaign-management add-audience-groups`** - add_audience_groups
- **`bing-ads-pp-cli campaign-management add-audiences`** - add_audiences
- **`bing-ads-pp-cli campaign-management add-bid-strategies`** - add_bid_strategies
- **`bing-ads-pp-cli campaign-management add-brand-kits`** - add_brand_kits
- **`bing-ads-pp-cli campaign-management add-budgets`** - add_budgets
- **`bing-ads-pp-cli campaign-management add-campaign-conversion-goals`** - add_campaign_conversion_goals
- **`bing-ads-pp-cli campaign-management add-campaign-criterions`** - add_campaign_criterions
- **`bing-ads-pp-cli campaign-management add-campaigns`** - add_campaigns
- **`bing-ads-pp-cli campaign-management add-conversion-goals`** - add_conversion_goals
- **`bing-ads-pp-cli campaign-management add-conversion-value-rules`** - add_conversion_value_rules
- **`bing-ads-pp-cli campaign-management add-data-exclusions`** - add_data_exclusions
- **`bing-ads-pp-cli campaign-management add-experiments`** - add_experiments
- **`bing-ads-pp-cli campaign-management add-html5s`** - add_html5s
- **`bing-ads-pp-cli campaign-management add-import-jobs`** - add_import_jobs
- **`bing-ads-pp-cli campaign-management add-keywords`** - add_keywords
- **`bing-ads-pp-cli campaign-management add-labels`** - add_labels
- **`bing-ads-pp-cli campaign-management add-linked-in-segments`** - add_linked_in_segments
- **`bing-ads-pp-cli campaign-management add-list-items-to-shared-list`** - add_list_items_to_shared_list
- **`bing-ads-pp-cli campaign-management add-media`** - add_media
- **`bing-ads-pp-cli campaign-management add-negative-keywords-to-entities`** - add_negative_keywords_to_entities
- **`bing-ads-pp-cli campaign-management add-new-customer-acquisition-goals`** - add_new_customer_acquisition_goals
- **`bing-ads-pp-cli campaign-management add-seasonality-adjustments`** - add_seasonality_adjustments
- **`bing-ads-pp-cli campaign-management add-shared-entity`** - add_shared_entity
- **`bing-ads-pp-cli campaign-management add-uet-tags`** - add_uet_tags
- **`bing-ads-pp-cli campaign-management add-videos`** - add_videos
- **`bing-ads-pp-cli campaign-management appeal-editorial-rejections`** - appeal_editorial_rejections
- **`bing-ads-pp-cli campaign-management apply-asset-group-listing-group-actions`** - apply_asset_group_listing_group_actions
- **`bing-ads-pp-cli campaign-management apply-customer-list-items`** - apply_customer_list_items
- **`bing-ads-pp-cli campaign-management apply-customer-list-user-data`** - apply_customer_list_user_data
- **`bing-ads-pp-cli campaign-management apply-hotel-group-actions`** - apply_hotel_group_actions
- **`bing-ads-pp-cli campaign-management apply-offline-conversion-adjustments`** - apply_offline_conversion_adjustments
- **`bing-ads-pp-cli campaign-management apply-offline-conversions`** - apply_offline_conversions
- **`bing-ads-pp-cli campaign-management apply-online-conversion-adjustments`** - apply_online_conversion_adjustments
- **`bing-ads-pp-cli campaign-management apply-product-partition-actions`** - apply_product_partition_actions
- **`bing-ads-pp-cli campaign-management create-asset-group-recommendation`** - create_asset_group_recommendation
- **`bing-ads-pp-cli campaign-management create-brand-kit-recommendation`** - create_brand_kit_recommendation
- **`bing-ads-pp-cli campaign-management create-responsive-ad-recommendation`** - create_responsive_ad_recommendation
- **`bing-ads-pp-cli campaign-management create-responsive-search-ad-recommendation`** - create_responsive_search_ad_recommendation
- **`bing-ads-pp-cli campaign-management delete-ad-extensions`** - delete_ad_extensions
- **`bing-ads-pp-cli campaign-management delete-ad-extensions-associations`** - delete_ad_extensions_associations
- **`bing-ads-pp-cli campaign-management delete-ad-group-criterions`** - delete_ad_group_criterions
- **`bing-ads-pp-cli campaign-management delete-ad-groups`** - delete_ad_groups
- **`bing-ads-pp-cli campaign-management delete-ads`** - delete_ads
- **`bing-ads-pp-cli campaign-management delete-asset-groups`** - delete_asset_groups
- **`bing-ads-pp-cli campaign-management delete-audience-group-asset-group-associations`** - delete_audience_group_asset_group_associations
- **`bing-ads-pp-cli campaign-management delete-audience-groups`** - delete_audience_groups
- **`bing-ads-pp-cli campaign-management delete-audiences`** - delete_audiences
- **`bing-ads-pp-cli campaign-management delete-bid-strategies`** - delete_bid_strategies
- **`bing-ads-pp-cli campaign-management delete-brand-kits`** - delete_brand_kits
- **`bing-ads-pp-cli campaign-management delete-budgets`** - delete_budgets
- **`bing-ads-pp-cli campaign-management delete-campaign-conversion-goals`** - delete_campaign_conversion_goals
- **`bing-ads-pp-cli campaign-management delete-campaign-criterions`** - delete_campaign_criterions
- **`bing-ads-pp-cli campaign-management delete-campaigns`** - delete_campaigns
- **`bing-ads-pp-cli campaign-management delete-data-exclusions`** - delete_data_exclusions
- **`bing-ads-pp-cli campaign-management delete-experiments`** - delete_experiments
- **`bing-ads-pp-cli campaign-management delete-html5s`** - delete_html5s
- **`bing-ads-pp-cli campaign-management delete-import-jobs`** - delete_import_jobs
- **`bing-ads-pp-cli campaign-management delete-keywords`** - delete_keywords
- **`bing-ads-pp-cli campaign-management delete-label-associations`** - delete_label_associations
- **`bing-ads-pp-cli campaign-management delete-labels`** - delete_labels
- **`bing-ads-pp-cli campaign-management delete-linked-in-segments`** - delete_linked_in_segments
- **`bing-ads-pp-cli campaign-management delete-list-items-from-shared-list`** - delete_list_items_from_shared_list
- **`bing-ads-pp-cli campaign-management delete-media`** - delete_media
- **`bing-ads-pp-cli campaign-management delete-negative-keywords-from-entities`** - delete_negative_keywords_from_entities
- **`bing-ads-pp-cli campaign-management delete-seasonality-adjustments`** - delete_seasonality_adjustments
- **`bing-ads-pp-cli campaign-management delete-shared-entities`** - delete_shared_entities
- **`bing-ads-pp-cli campaign-management delete-shared-entity-associations`** - delete_shared_entity_associations
- **`bing-ads-pp-cli campaign-management delete-videos`** - delete_videos
- **`bing-ads-pp-cli campaign-management get-account-migration-statuses`** - get_account_migration_statuses
- **`bing-ads-pp-cli campaign-management get-account-properties`** - get_account_properties
- **`bing-ads-pp-cli campaign-management get-ad-extension-ids-by-account-id`** - get_ad_extension_ids_by_account_id
- **`bing-ads-pp-cli campaign-management get-ad-extensions-associations`** - get_ad_extensions_associations
- **`bing-ads-pp-cli campaign-management get-ad-extensions-by-ids`** - get_ad_extensions_by_ids
- **`bing-ads-pp-cli campaign-management get-ad-extensions-editorial-reasons`** - get_ad_extensions_editorial_reasons
- **`bing-ads-pp-cli campaign-management get-ad-group-criterions-by-ids`** - get_ad_group_criterions_by_ids
- **`bing-ads-pp-cli campaign-management get-ad-groups-by-campaign-id`** - get_ad_groups_by_campaign_id
- **`bing-ads-pp-cli campaign-management get-ad-groups-by-ids`** - get_ad_groups_by_ids
- **`bing-ads-pp-cli campaign-management get-ads-by-ad-group-id`** - get_ads_by_ad_group_id
- **`bing-ads-pp-cli campaign-management get-ads-by-editorial-status`** - get_ads_by_editorial_status
- **`bing-ads-pp-cli campaign-management get-ads-by-ids`** - get_ads_by_ids
- **`bing-ads-pp-cli campaign-management get-annotation-opt-out`** - get_annotation_opt_out
- **`bing-ads-pp-cli campaign-management get-asset-group-listing-groups-by-ids`** - get_asset_group_listing_groups_by_ids
- **`bing-ads-pp-cli campaign-management get-asset-groups-by-campaign-id`** - get_asset_groups_by_campaign_id
- **`bing-ads-pp-cli campaign-management get-asset-groups-by-ids`** - get_asset_groups_by_ids
- **`bing-ads-pp-cli campaign-management get-asset-groups-editorial-reasons`** - get_asset_groups_editorial_reasons
- **`bing-ads-pp-cli campaign-management get-audience-group-asset-group-associations-by-asset-group-ids`** - get_audience_group_asset_group_associations_by_asset_group_ids
- **`bing-ads-pp-cli campaign-management get-audience-group-asset-group-associations-by-audience-group-ids`** - get_audience_group_asset_group_associations_by_audience_group_ids
- **`bing-ads-pp-cli campaign-management get-audience-groups-by-ids`** - get_audience_groups_by_ids
- **`bing-ads-pp-cli campaign-management get-audiences-by-ids`** - get_audiences_by_ids
- **`bing-ads-pp-cli campaign-management get-bid-strategies-by-ids`** - get_bid_strategies_by_ids
- **`bing-ads-pp-cli campaign-management get-bmc-stores-by-customer-id`** - get_bmc_stores_by_customer_id
- **`bing-ads-pp-cli campaign-management get-brand-kits-by-account-id`** - get_brand_kits_by_account_id
- **`bing-ads-pp-cli campaign-management get-brand-kits-by-ids`** - get_brand_kits_by_ids
- **`bing-ads-pp-cli campaign-management get-bsc-countries`** - get_bsc_countries
- **`bing-ads-pp-cli campaign-management get-budgets-by-ids`** - get_budgets_by_ids
- **`bing-ads-pp-cli campaign-management get-campaign-criterions-by-ids`** - get_campaign_criterions_by_ids
- **`bing-ads-pp-cli campaign-management get-campaign-ids-by-bid-strategy-ids`** - get_campaign_ids_by_bid_strategy_ids
- **`bing-ads-pp-cli campaign-management get-campaign-ids-by-budget-ids`** - get_campaign_ids_by_budget_ids
- **`bing-ads-pp-cli campaign-management get-campaign-sizes-by-account-id`** - get_campaign_sizes_by_account_id
- **`bing-ads-pp-cli campaign-management get-campaigns-by-account-id`** - get_campaigns_by_account_id
- **`bing-ads-pp-cli campaign-management get-campaigns-by-ids`** - get_campaigns_by_ids
- **`bing-ads-pp-cli campaign-management get-clipchamp-templates`** - get_clipchamp_templates
- **`bing-ads-pp-cli campaign-management get-config-value`** - get_config_value
- **`bing-ads-pp-cli campaign-management get-conversion-goals-by-ids`** - get_conversion_goals_by_ids
- **`bing-ads-pp-cli campaign-management get-conversion-goals-by-tag-ids`** - get_conversion_goals_by_tag_ids
- **`bing-ads-pp-cli campaign-management get-conversion-value-rules-by-account-id`** - get_conversion_value_rules_by_account_id
- **`bing-ads-pp-cli campaign-management get-conversion-value-rules-by-ids`** - get_conversion_value_rules_by_ids
- **`bing-ads-pp-cli campaign-management get-data-exclusions-by-account-id`** - get_data_exclusions_by_account_id
- **`bing-ads-pp-cli campaign-management get-data-exclusions-by-ids`** - get_data_exclusions_by_ids
- **`bing-ads-pp-cli campaign-management get-diagnostics`** - get_diagnostics
- **`bing-ads-pp-cli campaign-management get-editorial-reasons-by-ids`** - get_editorial_reasons_by_ids
- **`bing-ads-pp-cli campaign-management get-experiments-by-ids`** - get_experiments_by_ids
- **`bing-ads-pp-cli campaign-management get-file-import-upload-url`** - get_file_import_upload_url
- **`bing-ads-pp-cli campaign-management get-geo-locations-file-url`** - get_geo_locations_file_url
- **`bing-ads-pp-cli campaign-management get-health-check`** - get_health_check
- **`bing-ads-pp-cli campaign-management get-html5s-by-ids`** - get_html5s_by_ids
- **`bing-ads-pp-cli campaign-management get-import-entity-ids-mapping`** - get_import_entity_ids_mapping
- **`bing-ads-pp-cli campaign-management get-import-jobs-by-ids`** - get_import_jobs_by_ids
- **`bing-ads-pp-cli campaign-management get-import-results`** - get_import_results
- **`bing-ads-pp-cli campaign-management get-keywords-by-ad-group-id`** - get_keywords_by_ad_group_id
- **`bing-ads-pp-cli campaign-management get-keywords-by-editorial-status`** - get_keywords_by_editorial_status
- **`bing-ads-pp-cli campaign-management get-keywords-by-ids`** - get_keywords_by_ids
- **`bing-ads-pp-cli campaign-management get-label-associations-by-entity-ids`** - get_label_associations_by_entity_ids
- **`bing-ads-pp-cli campaign-management get-label-associations-by-label-ids`** - get_label_associations_by_label_ids
- **`bing-ads-pp-cli campaign-management get-labels-by-ids`** - get_labels_by_ids
- **`bing-ads-pp-cli campaign-management get-list-items-by-shared-list`** - get_list_items_by_shared_list
- **`bing-ads-pp-cli campaign-management get-media-associations`** - get_media_associations
- **`bing-ads-pp-cli campaign-management get-media-meta-data-by-account-id`** - get_media_meta_data_by_account_id
- **`bing-ads-pp-cli campaign-management get-media-meta-data-by-ids`** - get_media_meta_data_by_ids
- **`bing-ads-pp-cli campaign-management get-negative-keywords-by-entity-ids`** - get_negative_keywords_by_entity_ids
- **`bing-ads-pp-cli campaign-management get-negative-sites-by-ad-group-ids`** - get_negative_sites_by_ad_group_ids
- **`bing-ads-pp-cli campaign-management get-negative-sites-by-campaign-ids`** - get_negative_sites_by_campaign_ids
- **`bing-ads-pp-cli campaign-management get-new-customer-acquisition-goals-by-account-id`** - get_new_customer_acquisition_goals_by_account_id
- **`bing-ads-pp-cli campaign-management get-offline-conversion-report-by-goal-ids`** - get_offline_conversion_report_by_goal_ids
- **`bing-ads-pp-cli campaign-management get-offline-conversion-reports`** - get_offline_conversion_reports
- **`bing-ads-pp-cli campaign-management get-profile-data-file-url`** - get_profile_data_file_url
- **`bing-ads-pp-cli campaign-management get-responsive-ad-recommendation-job`** - get_responsive_ad_recommendation_job
- **`bing-ads-pp-cli campaign-management get-seasonality-adjustments-by-account-id`** - get_seasonality_adjustments_by_account_id
- **`bing-ads-pp-cli campaign-management get-seasonality-adjustments-by-ids`** - get_seasonality_adjustments_by_ids
- **`bing-ads-pp-cli campaign-management get-shared-entities`** - get_shared_entities
- **`bing-ads-pp-cli campaign-management get-shared-entities-by-account-id`** - get_shared_entities_by_account_id
- **`bing-ads-pp-cli campaign-management get-shared-entity-associations-by-entity-ids`** - get_shared_entity_associations_by_entity_ids
- **`bing-ads-pp-cli campaign-management get-shared-entity-associations-by-shared-entity-ids`** - get_shared_entity_associations_by_shared_entity_ids
- **`bing-ads-pp-cli campaign-management get-supported-clipchamp-audio`** - get_supported_clipchamp_audio
- **`bing-ads-pp-cli campaign-management get-supported-fonts`** - get_supported_fonts
- **`bing-ads-pp-cli campaign-management get-uet-tag-auth-key`** - get_uet_tag_auth_key
- **`bing-ads-pp-cli campaign-management get-uet-tags-by-ids`** - get_uet_tags_by_ids
- **`bing-ads-pp-cli campaign-management get-videos-by-ids`** - get_videos_by_ids
- **`bing-ads-pp-cli campaign-management refine-asset-group-recommendation`** - refine_asset_group_recommendation
- **`bing-ads-pp-cli campaign-management refine-responsive-ad-recommendation`** - refine_responsive_ad_recommendation
- **`bing-ads-pp-cli campaign-management refine-responsive-search-ad-recommendation`** - refine_responsive_search_ad_recommendation
- **`bing-ads-pp-cli campaign-management search-companies`** - search_companies
- **`bing-ads-pp-cli campaign-management set-account-properties`** - set_account_properties
- **`bing-ads-pp-cli campaign-management set-ad-extensions-associations`** - set_ad_extensions_associations
- **`bing-ads-pp-cli campaign-management set-audience-group-asset-group-associations`** - set_audience_group_asset_group_associations
- **`bing-ads-pp-cli campaign-management set-label-associations`** - set_label_associations
- **`bing-ads-pp-cli campaign-management set-negative-sites-to-ad-groups`** - set_negative_sites_to_ad_groups
- **`bing-ads-pp-cli campaign-management set-negative-sites-to-campaigns`** - set_negative_sites_to_campaigns
- **`bing-ads-pp-cli campaign-management set-shared-entity-associations`** - set_shared_entity_associations
- **`bing-ads-pp-cli campaign-management update-ad-extensions`** - update_ad_extensions
- **`bing-ads-pp-cli campaign-management update-ad-group-criterions`** - update_ad_group_criterions
- **`bing-ads-pp-cli campaign-management update-ad-groups`** - update_ad_groups
- **`bing-ads-pp-cli campaign-management update-ads`** - update_ads
- **`bing-ads-pp-cli campaign-management update-annotation-opt-out`** - update_annotation_opt_out
- **`bing-ads-pp-cli campaign-management update-asset-groups`** - update_asset_groups
- **`bing-ads-pp-cli campaign-management update-audience-groups`** - update_audience_groups
- **`bing-ads-pp-cli campaign-management update-audiences`** - update_audiences
- **`bing-ads-pp-cli campaign-management update-bid-strategies`** - update_bid_strategies
- **`bing-ads-pp-cli campaign-management update-brand-kits`** - update_brand_kits
- **`bing-ads-pp-cli campaign-management update-budgets`** - update_budgets
- **`bing-ads-pp-cli campaign-management update-campaign-criterions`** - update_campaign_criterions
- **`bing-ads-pp-cli campaign-management update-campaigns`** - update_campaigns
- **`bing-ads-pp-cli campaign-management update-conversion-goals`** - update_conversion_goals
- **`bing-ads-pp-cli campaign-management update-conversion-value-rules`** - update_conversion_value_rules
- **`bing-ads-pp-cli campaign-management update-conversion-value-rules-status`** - update_conversion_value_rules_status
- **`bing-ads-pp-cli campaign-management update-data-exclusions`** - update_data_exclusions
- **`bing-ads-pp-cli campaign-management update-experiments`** - update_experiments
- **`bing-ads-pp-cli campaign-management update-import-jobs`** - update_import_jobs
- **`bing-ads-pp-cli campaign-management update-keywords`** - update_keywords
- **`bing-ads-pp-cli campaign-management update-labels`** - update_labels
- **`bing-ads-pp-cli campaign-management update-linked-in-segments`** - update_linked_in_segments
- **`bing-ads-pp-cli campaign-management update-new-customer-acquisition-goals`** - update_new_customer_acquisition_goals
- **`bing-ads-pp-cli campaign-management update-seasonality-adjustments`** - update_seasonality_adjustments
- **`bing-ads-pp-cli campaign-management update-shared-entities`** - update_shared_entities
- **`bing-ads-pp-cli campaign-management update-uet-tags`** - update_uet_tags
- **`bing-ads-pp-cli campaign-management update-videos`** - update_videos

### customer-billing

Manage customer billing

- **`bing-ads-pp-cli customer-billing add-insertion-order`** - add_insertion_order
- **`bing-ads-pp-cli customer-billing check-feature-adoption-coupon-eligibility`** - check_feature_adoption_coupon_eligibility
- **`bing-ads-pp-cli customer-billing claim-feature-adoption-coupons`** - claim_feature_adoption_coupons
- **`bing-ads-pp-cli customer-billing dispatch-coupons`** - dispatch_coupons
- **`bing-ads-pp-cli customer-billing distribute-coupons`** - distribute_coupons
- **`bing-ads-pp-cli customer-billing get-account-monthly-spend`** - get_account_monthly_spend
- **`bing-ads-pp-cli customer-billing get-billing-documents`** - get_billing_documents
- **`bing-ads-pp-cli customer-billing get-billing-documents-info`** - get_billing_documents_info
- **`bing-ads-pp-cli customer-billing get-billing-groups`** - get_billing_groups
- **`bing-ads-pp-cli customer-billing get-coupon-info`** - get_coupon_info
- **`bing-ads-pp-cli customer-billing get-ungrouped-accounts`** - get_ungrouped_accounts
- **`bing-ads-pp-cli customer-billing redeem-coupon`** - redeem_coupon
- **`bing-ads-pp-cli customer-billing search-coupons`** - search_coupons
- **`bing-ads-pp-cli customer-billing search-insertion-orders`** - search_insertion_orders
- **`bing-ads-pp-cli customer-billing update-billing-group-accounts`** - update_billing_group_accounts
- **`bing-ads-pp-cli customer-billing update-insertion-order`** - update_insertion_order

### customer-management

Manage customer management

- **`bing-ads-pp-cli customer-management add-account`** - add_account
- **`bing-ads-pp-cli customer-management add-client-links`** - add_client_links
- **`bing-ads-pp-cli customer-management add-prepay-account`** - add_prepay_account
- **`bing-ads-pp-cli customer-management delete-account`** - delete_account
- **`bing-ads-pp-cli customer-management delete-customer`** - delete_customer
- **`bing-ads-pp-cli customer-management delete-user`** - delete_user
- **`bing-ads-pp-cli customer-management dismiss-notifications`** - dismiss_notifications
- **`bing-ads-pp-cli customer-management find-accounts`** - find_accounts
- **`bing-ads-pp-cli customer-management find-accounts-or-customers-info`** - find_accounts_or_customers_info
- **`bing-ads-pp-cli customer-management get-accessible-customer`** - get_accessible_customer
- **`bing-ads-pp-cli customer-management get-account`** - get_account
- **`bing-ads-pp-cli customer-management get-account-pilot-features`** - get_account_pilot_features
- **`bing-ads-pp-cli customer-management get-accounts-info`** - get_accounts_info
- **`bing-ads-pp-cli customer-management get-current-user`** - get_current_user
- **`bing-ads-pp-cli customer-management get-customer`** - get_customer
- **`bing-ads-pp-cli customer-management get-customer-pilot-features`** - get_customer_pilot_features
- **`bing-ads-pp-cli customer-management get-customers-info`** - get_customers_info
- **`bing-ads-pp-cli customer-management get-linked-accounts-and-customers-info`** - get_linked_accounts_and_customers_info
- **`bing-ads-pp-cli customer-management get-notifications`** - get_notifications
- **`bing-ads-pp-cli customer-management get-pilot-features-countries`** - get_pilot_features_countries
- **`bing-ads-pp-cli customer-management get-user`** - get_user
- **`bing-ads-pp-cli customer-management get-user-mfa-status`** - get_user_mfa_status
- **`bing-ads-pp-cli customer-management get-users-info`** - get_users_info
- **`bing-ads-pp-cli customer-management map-account-id-to-external-account-ids`** - map_account_id_to_external_account_ids
- **`bing-ads-pp-cli customer-management map-customer-id-to-external-customer-id`** - map_customer_id_to_external_customer_id
- **`bing-ads-pp-cli customer-management search-accounts`** - search_accounts
- **`bing-ads-pp-cli customer-management search-client-links`** - search_client_links
- **`bing-ads-pp-cli customer-management search-customers`** - search_customers
- **`bing-ads-pp-cli customer-management search-user-invitations`** - search_user_invitations
- **`bing-ads-pp-cli customer-management send-user-invitation`** - send_user_invitation
- **`bing-ads-pp-cli customer-management signup-customer`** - signup_customer
- **`bing-ads-pp-cli customer-management update-account`** - update_account
- **`bing-ads-pp-cli customer-management update-client-links`** - update_client_links
- **`bing-ads-pp-cli customer-management update-customer`** - update_customer
- **`bing-ads-pp-cli customer-management update-prepay-account`** - update_prepay_account
- **`bing-ads-pp-cli customer-management update-user`** - update_user
- **`bing-ads-pp-cli customer-management update-user-roles`** - update_user_roles
- **`bing-ads-pp-cli customer-management upgrade-customer-to-agency`** - upgrade_customer_to_agency
- **`bing-ads-pp-cli customer-management validate-address`** - validate_address

### reporting

Manage reporting

- **`bing-ads-pp-cli reporting poll-generate-report`** - poll_generate_report
- **`bing-ads-pp-cli reporting submit-generate-report`** - submit_generate_report


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bing-ads-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bing-ads-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bing-ads-pp-cli learnings list`** - Inspect taught rows
- **`bing-ads-pp-cli learnings forget <query>`** - Undo a teach
- **`bing-ads-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bing-ads-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bing-ads-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bing-ads-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BING_ADS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bing-ads-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bing-ads-pp-cli ad-insight get-auction-insight-data

# JSON for scripting and agents
bing-ads-pp-cli ad-insight get-auction-insight-data --json
# Filter to specific fields
bing-ads-pp-cli ad-insight get-auction-insight-data --json --select Result

# Dry run — show the request without sending
bing-ads-pp-cli ad-insight get-auction-insight-data --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bing-ads-pp-cli ad-insight get-auction-insight-data --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
bing-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `bing-ads-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/microsoft-advertising-bing-pp-cli/config.toml`; `--home`, `BING_ADS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BING_ADS_CUSTOMER_ACCOUNT_ID` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bing-ads-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bing-ads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BING_ADS_CUSTOMER_ACCOUNT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
