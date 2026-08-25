# Etsy Seller Dashboard Browser Discovery

## Scope

Authenticated discovery covered four peer Shop Manager surfaces:

1. Marketplace Insights
2. Etsy Ads
3. Offsite Ads
4. Sales & Discounts

Marketplace Insights remains the README/first-run lead only because one surface must anchor the presentation.

## User Goal Flows

### Marketplace Insights

- Research a keyword.
- Review searches, listing competition, trend points, related terms, and listing previews.
- Sort and paginate related terms.
- Return to category trends and select a category.

### Etsy Ads

- Review campaign totals by metric and date range.
- Compare with the prior period.
- Paginate listing-level ad statistics.
- Inspect listing impressions, clicks, click rate, orders, spend, revenue, conversions, and ROAS.

### Offsite Ads

- Review direct and indirect attributed performance.
- Toggle graph/table and previous-period comparison.
- Change the report range.
- Inspect traffic history, channel share, and listing performance.

### Sales & Discounts

- Review key promotions and targeted offers.
- Open the details/statistics tab.
- Change the report range.
- Sort promotion performance.
- Inspect promotion definitions, dates, discount conditions, sends, uses, and revenue.

No ad-status toggle, Offsite Ads opt-out control, promotion setup, or other mutation was used.

## Capture Configuration

- Auth source: local Chrome `Default` profile.
- Driver: `agent-browser` 0.33.0.
- Capture pacing: approximately one second after meaningful interactions; persistent analytics prevented a strict network-idle signal.
- First capture: Marketplace Insights, 722 requests.
- Expanded capture: Ads/Offsite Ads/Sales & Discounts, 747 requests.
- Analyzer: Printing Press `browser-sniff`.
- Durable raw credentials/sessions: none.
- Raw HAR files: ephemeral session directory only and deleted after sanitized extraction.

## Discovered Data Surfaces

| Surface | Method | Normalized path | Key data |
| --- | --- | --- | --- |
| Marketplace Insights | GET | `/api/v3/ajax/bespoke/shop/{shop_id}/marketplace-insights/data` | Landing data |
| Marketplace Insights | GET | `/api/v3/ajax/shop/{shop_id}/marketplace-insights/trending-categories` | Category IDs/slugs |
| Marketplace Insights | GET | `/api/v3/ajax/bespoke/shop/{shop_id}/marketplace-insights/trending-search-terms-v2` | Category search terms |
| Marketplace Insights | GET | `/api/v3/ajax/shop/{shop_id}/marketplace-insights/saved-search-terms` | Saved terms and pagination |
| Marketplace Insights | POST | `/api/v3/ajax/shop/{shop_id}/marketplace-insights/llm-exploratory-keywords/search/enqueue` | Keyword request and cached result; CSRF-protected |
| Marketplace Insights | GET | `/your/shops/me/marketplace-insights` | Authenticated landing HTML |
| Marketplace Insights | GET | `/your/shops/me/marketplace-insights/search` | Keyword metrics, trends, related terms, listing previews |
| Etsy Ads | GET | `/api/v3/ajax/shop/{shop_id}/prolist/stats/listings` | Listing metadata, impressions, clicks, spend, conversions, revenue, click rate, ROAS |
| Offsite Ads | GET | `/api/v3/ajax/shop/{shop_id}/offsite-ads-data/ad-traffic` | Current and comparison traffic series |
| Offsite Ads | GET | `/api/v3/ajax/shop/{shop_id}/offsite-ads-data/channel-performance` | Channel, clicks, share |
| Offsite Ads | GET | `/api/v3/ajax/shop/{shop_id}/offsite-ads-data/listing-performance` | Listing clicks, orders, revenue |
| Offsite Ads | GET | `/api/v3/ajax/shop/{shop_id}/offsite-ads-stats` | Direct/indirect revenue, fees, orders, new buyers |
| Sales & Discounts | GET | `/api/v3/ajax/bespoke/shop/{shop_id}/sales-coupons/combined` | Promotions, discount rules, dates, targeted offers, send counts, revenue stats |
| Shop Manager | GET | `/api/v3/ajax/shop/{shop_id}/manager/nav-badge-counts` | Navigation badges; shared shell data, not a headline command |

Support chatbot, telemetry, tag, polyfill, advertising-extension, and static bundle traffic are capture noise and must not become CLI commands.

## Request and Response Details

### Etsy Ads

`prolist/stats/listings` accepts sort type/order, offset, limit, and promoted-state filters. Each listing record includes listing metadata plus impressions, clicks, spend, conversions, revenue, click rate, and ROAS.

### Offsite Ads

- Traffic accepts current and comparison date ranges.
- Channel performance accepts a date range.
- Listing performance includes listing ID/title/URL, clicks, orders, and formatted revenue.
- Summary data includes direct and indirect revenue, fees, orders, and new buyers.

### Sales & Discounts

The combined resource includes promotion IDs/types/names, creation/start/end dates, minimum-order conditions, fixed/percentage rewards, redemption limits, listing sets, coupon URLs, active/stopped targeted-offer state, recent send counts, and revenue statistics.

### Marketplace Insights

Keyword enqueue accepts a required `keyword` body field and an ephemeral `x-csrf-token`. Trending terms accept `taxonomy_id`. The authenticated search HTML carries richer keyword metrics and related-term results.

## Authentication and Replayability

Anonymous direct HTTP receives Etsy DataDome protection. All four pages succeed within the authenticated Chrome profile and share the seller-session model.

Required runtime strategy:

1. Import Etsy session cookies from local Chrome at runtime.
2. Store cookie material only in the local auth store with restrictive permissions.
3. Use replayable HTTP/HTML for data commands; no resident browser dependency.
4. Fetch ephemeral CSRF material only for explicit mutations.
5. Never persist captured cookie, CSRF, shop, member, listing, campaign, coupon, or analytics identifiers in source or research artifacts.
6. Detect challenge/sign-in HTML and return actionable auth-refresh guidance.

Replay confidence:

- Browser-authenticated read flows: high.
- Direct HTTP with imported cookies: plausible, pending dogfood.
- Marketplace keyword enqueue outside browser: captured, pending CSRF replay proof.
- Ad, Offsite Ads, and promotion mutations: intentionally not captured and outside approved shipping scope.

## Generation Guidance

- Build a single Etsy Seller Dashboard CLI with equal-weight command groups: `insights`, `ads`, `offsite-ads`, and `promotions`.
- Default to read-only analytics.
- Normalize shared shop/listing/date keys in SQLite.
- Preserve raw JSON output alongside curated tables.
- Feature-detect optional Etsy experiment fields.
- Exclude all unrelated support, telemetry, advertising-extension, and static-asset requests.
- Keep two known data gaps explicit: Marketplace Insights country filtering and click/CTR engagement are not present in the captured keyword surface.
