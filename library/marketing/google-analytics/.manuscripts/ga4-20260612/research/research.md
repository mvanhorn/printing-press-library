# Google Analytics 4 CLI research

Run id: `ga4-20260612`
Target CLI: `google-analytics-pp-cli`
API family: GA4 Data API v1beta, GA4 Data API v1alpha funnel endpoint, and Google Analytics Admin API v1beta.

## Scope and command surface

This CLI is GA4-only. Search Console stays in `google-search-console-pp-cli`; Universal Analytics is intentionally excluded because the public UA Reporting API is sunset and would create misleading commands for agents.

The publishable command surface is split into:

- Raw Data API wrappers: `report`, `pivot`, `batch`, `realtime`, `metadata`, `compatibility`.
- Raw Admin API wrappers: `properties`, `property`, `streams`.
- Novel agent reports: `channels`, `sources`, `top-pages`, `events`, `conversions`, `funnel`, `compare`, `whats-changed`, `revenue`, `audience`, `cohort`.
- Agent/operator utilities: `agent-context`, `health`, `doctor`.

All data commands resolve property IDs as `--property` first, then `GA4_PROPERTY_ID`. Fleet checks can pass `health --properties` or set `GA4_PROPERTY_IDS`. Brand IDs such as two authorized GA4 properties are proof inputs only, not implementation defaults.

## Auth model

GA4 Data/Admin reads work with a Google service account JWT bearer flow using scope:

`https://www.googleapis.com/auth/analytics.readonly`

The CLI reads credentials in this order:

1. `--credentials <service-account-json>`
2. `GOOGLE_APPLICATION_CREDENTIALS`

The service account JSON must include `client_email`, `private_key`, and `token_uri` (defaulting to `https://oauth2.googleapis.com/token` when omitted). The CLI signs an RS256 JWT assertion, exchanges it for an access token, and sends an `Authorization: Bearer <token>` header on every Google API request. Tokens and key material are never printed in proofs.

## Property-level grant gotcha

Enabling the Google Analytics Data API in Google Cloud is not enough. GA4 property access is controlled inside Google Analytics Admin. A service account can mint a valid token and still receive 403/404 for a property if the service account email was not granted Viewer access on that GA4 property.

`health`/`doctor` are designed around this gotcha:

- token/key failures are reported as `creds_invalid` or `api_or_token_error`;
- Admin API visibility is reported via account summaries;
- each requested property is checked with a tiny `runReport` for `sessions` over `7daysAgo..yesterday`;
- 403 is classified as `api_enabled_but_property_not_shared_or_permission_denied`;
- 404 is classified as `property_not_found_or_not_shared`.

## GA4 Data API v1beta endpoints

`runReport` posts to `/v1beta/properties/{property}:runReport`. The typed request includes date ranges, dimensions, metrics, limit, dimension filter, and order-bys. GA4 accepts relative date tokens such as `today`, `yesterday`, `NdaysAgo` (for example `28daysAgo`) as well as ISO dates.

`runPivotReport` posts to `/v1beta/properties/{property}:runPivotReport`. The top-level report request is similar to `runReport`, but row limits belong inside each pivot object. A live proof caught and fixed the invalid top-level `limit` draft shape.

`batchRunReports` posts to `/v1beta/properties/{property}:batchRunReports` with a typed array of `RunReportRequest` objects. The property is supplied by the URL, not the body.

`runRealtimeReport` posts to `/v1beta/properties/{property}:runRealtimeReport` and uses realtime-compatible metrics/dimensions such as `activeUsers` and `unifiedScreenName`.

`getMetadata` reads `/v1beta/properties/{property}/metadata` and returns the current property-specific dimension and metric catalog.

`checkCompatibility` posts to `/v1beta/properties/{property}:checkCompatibility` and validates metric/dimension combinations before agents build larger reports.

## GA4 Data API v1alpha funnel endpoint

`runFunnelReport` posts to `/v1alpha/properties/{property}:runFunnelReport`. Funnel steps are typed as event-name filters. The novel `funnel` command turns a comma-separated step list such as `view_item,add_to_cart,begin_checkout,purchase` into a GA4 funnel request.

## Google Analytics Admin API v1beta endpoints

`accountSummaries` reads `/v1beta/accountSummaries?pageSize=200` and is the safest discovery endpoint for service-account visibility.

`property` reads `/v1beta/properties/{property}` for display metadata, timezone, currency, and parent account.

`streams` reads `/v1beta/properties/{property}/dataStreams` and surfaces web/app stream metadata.

## Quotas, limits, and operational notes

GA4 Data API quotas are property-scoped and token-budgeted. Requests consume quota based on complexity; high-cardinality dimensions, many metrics, pivots, funnel reports, and broad date ranges cost more than narrow summaries. Agents should default to modest `--limit` values, use novel commands for common questions, and run `compatibility` before unusual metric/dimension combinations.

Common safe defaults used by this CLI:

- `30daysAgo..yesterday` for most summary commands;
- `7daysAgo..yesterday` for health and short event trends;
- `28daysAgo..yesterday` in proofs to avoid partial same-day reporting;
- `--limit 25` for novel tables and smaller limits in smoke tests.

## Implementation structure

The draft single-file CLI was decomposed into the same shape as modern Printing Press CLIs:

- `internal/ga4/types.go`, `client.go`, `auth.go` — typed Google API layer and service-account JWT flow.
- `internal/cli/root.go`, `agent_context.go`, `health.go` — root flags and operator utilities.
- `internal/cli/raw_commands.go`, `admin_commands.go` — raw endpoint wrappers.
- `internal/cli/novel_*.go` — one file per novel command family.
- `internal/cli/builders.go`, `transform.go`, `render.go` — shared request builders, response shaping, and output helpers.
- Unit tests cover typed request building, global flag behavior, response flattening/enrichment, compare deltas, anomaly ranking math, and API-client HTTP/error paths.
