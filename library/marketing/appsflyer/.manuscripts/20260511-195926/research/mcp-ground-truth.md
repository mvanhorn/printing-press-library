# AppsFlyer BETA MCP Ground Truth

Captured from the loaded `mcp__claude_ai_AppsFlyer_BETA__*` tool schemas.
These represent the maintained, agent-facing API shape.

## Tool surface (16 tools)

### Read aggregated dashboard
- `fetch_aggregated_data` — primary metrics pull
  - 1-10 app_ids per call
  - 1-25 metrics per call
  - Up to 4 groupings
  - Sort by metrics (multi)
  - Default sort: Total attributions desc
  - row_count: 1-300 (default 25)
  - Critical: **percentage metrics returned as decimals** (0.50 = 50%)
  - Returns CSV in Data section + JSON metadata (timezone, currency, per-metric partial-data flag, error, filters)

### SKAdNetwork
- `skan_get_app_data` — SKAN attribution per app
  - One app_id at a time
  - date_type: install | arrival
  - Note: **skip the most recent 2 days of SKAN data**
  - canonical media_source names: googleadwords_int, facebook
  - postback level KPIs: pb_0, pb_1, pb_2 + modeled variants

### App / account configuration
- `get_apps` — list apps in the account
- `get_app_settings` — per-app settings
- `get_users` — list account users

### Integrations
- `get_active_cost_integrations` — which cost connectors are live
- `get_active_adrevenue_integrations` — which ad-rev connectors are live
- `list_cost_supported_media_sources` (NO AUTH) — full catalog of cost-supported sources (147 entries)
- `list_adrevenue_supported_media_sources` (NO AUTH) — full catalog of ad-rev-supported sources (24 entries with integration_types)

### Audiences
- `list_active_audiences` — audiences active in the account
- `list_audiences_connections` — audience connection catalog
- `get_audience_connections` — connections for a specific audience

### OneLink (deep links)
- `get_onelink_templates`
- `get_onelink_template_links`
- `get_onelink_details`

### Knowledge
- `get_public_knowledge` — search KB / dev hub (Zendesk + DevHub)

## Canonical enums

### Metrics (40)
Impressions, CPM, DAU/MAU, Installs, eCPA, IPM, First visits, Unique users,
Count per unique user, Count, Active users, Revenue, SKAN duplicates,
Click-throughs, Gross profit, Uninstalls rate, Revenue per unique user,
Re-engagements, Cost, mau-30, Sessions, Re-attributions, Assisted installs,
Total attributions, Platform activations, Clicks, Average count, ARPU,
View-throughs, Click-to-install rate, ROAS, Uninstalls, User acquisitions,
eCPI, CTR, Conversion rate, Retention rate, Re-visits, CPC, ROI

### Aggregation type
cumulative | on-period (key cohort-vs-on-day distinction)

### Attribution sources
ssot | cross-platform | appsflyer | skan

### Groupings (30) — aggregated data
Ad, Ad ID, Adset, Adset ID, App, App version, Attributed touch type,
Attribution type, Campaign, Campaign ID, Channel, Conversion type, Geo,
Date, Engagement Destination, Engagement type, Event name, Install app store,
Keywords, Media source, Monetization type, OS version, Agency, Platform,
Platform group, Product line, Site ID, Source, Store product page, Sub param 1

### Filters (29) — aggregated data (no "App" or "Product line" since those route via app_ids)
same as groupings minus App, Date, Product line

### SKAN KPIs (40)
installs, unique_users_pb_0, impressions, revenue, roas, conv_rate,
event_count_modeled_pb_0, unique_users_pb_1, click_through_installs, clicks,
unique_users_modeled, direct_first_installs_ratio, revenue_modeled,
unique_users_pb_2, converted_users, revenue_pb_1, install_cost,
null_cv_installs, event_count_modeled, unique_users_modeled_pb_0, roas_modeled,
direct_click_through_ratio, avg_ecpi, arpu, null_cv_rate, unique_users,
non_organic_installs, revenue_modeled_pb_0, direct_redownloads_ratio,
roi_modeled, event_count, arpu_modeled, view_through_installs, revenue_pb_0,
revenue_pb_2, roi, direct_view_through_ratio, direct_installs, ctr,
converted_users_installs

### SKAN groupings (22)
date, campaign_id, mode, af_attribution_flag, adset_id, channel,
source_app_name, install_type, source, creative, adset_name, ad_id,
postback_sequence_index, campaign, inapps, ad_name, site_id, version,
media_source, attributed_touch_type, country, fidelity_type, event_name

## Media-source canonical IDs (sample)

- Meta ads / Facebook → facebook_int
- Google Ads → googleadwords_int
- TikTok For Business → tiktokglobal_int
- Apple Search Ads → iossearchads_int
- ironSource → ironsource_int
- AppLovin → applovin_int
- Snapchat → snapchat_int
- X Ads (Twitter) → twitter_int
- reddit → reddit_int
- Pinterest → pinterest_int
- Yahoo → yahoogemini_int
- ByteDance China → bytedance_int
- VK Ads (myTarget) → mail.ru_int
- AppLovin MAX (ad rev) → applovinmax_int
- Google AdMob (ad rev) → googleadmob_int
- Unity ads mediation → unityadsmediation_int

(147 total cost-supported, 24 ad-rev with integration_types)

## Implementation signals for our CLI

1. The CLI's `--source` flag must accept the `_int` canonical IDs (matches API)
   but also accept friendly names like "facebook" or "google" — we ship a
   resolver that maps display → canonical.
2. The cohort/aggregated distinction in the API is "aggregation_type":
   `cumulative` (ltv-like) vs `on-period` (daily snapshot). Our `--ltv` flag
   should map cumulative.
3. SKAN data lags 2 days — our `skan` command should default `--end-date` to
   `yesterday - 2d` instead of `yesterday`.
4. Aggregated data CSV percentage trap: ROAS=0.50 means 50%. Our pretty-table
   formatter must multiply ratio metrics by 100 (and tag as %).
5. partial_data is a per-metric flag in the metadata. Our `--exclude-partial`
   default-true behavior tracks this.
6. Up to 10 app_ids per call — for accounts with >10 apps, we need to fan out
   and merge.
7. SKAN is one app at a time — fan-out helper applies here too.

