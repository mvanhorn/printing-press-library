# Google Analytics 4 Printing Press CLI

`google-analytics-pp-cli` is an agent-first GA4 CLI for live analytics work. It covers the GA4 Data API, the GA4 Admin API discovery calls, and curated novel commands that answer the questions agents actually ask without stitching multiple raw API calls together.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install / build

```bash
PP_LIBRARY_REPO=/path/to/printing-press-library pp-sync build google-analytics-pp-cli
```

## Auth

Use a service-account JSON key with Analytics readonly scope. Set `GOOGLE_APPLICATION_CREDENTIALS` to your service-account JSON path, or pass `--credentials` explicitly.

Resolution order: `--credentials`, then `GOOGLE_APPLICATION_CREDENTIALS`. There is no implicit developer-local credential fallback.

GA4 property resolution for data commands: `--property`, then `GA4_PROPERTY_ID`. The CLI does not hard-code the first authorized property or the second authorized property property IDs. For fleet health checks, pass explicit properties or set `GA4_PROPERTY_IDS`.

Important gotcha: Google Cloud API access is not GA4 property access. The service account must also be granted Viewer access inside each GA4 property. `health` / `doctor` distinguishes invalid credentials from property not shared / permission denied.

## Global agent flags

Every command inherits:

- `--agent` = `--json --compact --no-input --yes`
- `--json`
- `--compact`
- `--no-input`
- `--yes`
- `--property`
- `--credentials`
- `--timeout`

## Raw API wrappers

- `report` — GA4 Data API `runReport` (`--metrics`, `--dimensions`, `--start`, `--end`, `--filter`, `--order`, `--limit`)
- `pivot` — `runPivotReport`
- `batch` — `batchRunReports`
- `realtime` — `runRealtimeReport`
- `metadata` — list valid dimensions/metrics for a property
- `compatibility` — `checkCompatibility`
- `properties` — Admin API `accountSummaries.list`
- `property` — Admin API `properties.get`
- `streams` — Admin API `properties.dataStreams.list`

## Novel commands

- `channels` — sessions/users/conversions/revenue by default channel group.
- `sources` — source/medium acquisition breakdown with computed conversion rate.
- `top-pages` — landing pages by sessions, engagement, conversions, revenue.
- `events` / `conversions` — events or conversions over time with trend summary.
- `funnel` — v1alpha `runFunnelReport` for a named event sequence.
- `compare` — period-over-period metric deltas and percent changes.
- `whats-changed` — anomaly-style mover scan across key dimensions.
- `revenue` — ecommerce revenue, AOV, transactions by channel/source.
- `audience` / `cohort` — cheap audience and retention snapshots.
- `health` / `doctor` — token mint, Admin visibility, and per-property access grants.

## Examples

```bash
google-analytics-pp-cli agent-context --agent
google-analytics-pp-cli health --properties $GA4_PROPERTY_IDS --agent
google-analytics-pp-cli channels --property $GA4_PROPERTY_ID --start 28daysAgo --end yesterday --agent
google-analytics-pp-cli compare --property $GA4_PROPERTY_ID --metric sessions,totalRevenue --period wow --agent
google-analytics-pp-cli whats-changed --property $GA4_PROPERTY_ID --agent
google-analytics-pp-cli funnel --property $GA4_PROPERTY_ID --steps view_item,add_to_cart,begin_checkout,purchase --agent
```
