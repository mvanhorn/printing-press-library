# Google Analytics 4 Printing Press CLI

`google-analytics-pp-cli` is an agent-first GA4 CLI for live analytics work. It covers the GA4 Data API, the GA4 Admin API — reads *and* writes — and curated novel commands that answer the questions agents actually ask without stitching multiple raw API calls together.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install / build

```bash
PP_LIBRARY_REPO=/path/to/printing-press-library pp-sync build google-analytics-pp-cli
```

## Auth

Two credential shapes are accepted, and the CLI detects which one you handed it:

- **Service-account JSON key** (`client_email` + `private_key`) — the classic path. Signs an RS256 JWT and requests the scope the command needs.
- **ADC `authorized_user` JSON** (`client_id` + `client_secret` + `refresh_token`) — what `gcloud auth application-default login` writes. Exchanges the refresh token for an access token.

Resolution order: `--credentials`, then `GOOGLE_ANALYTICS_ADC`, then `GOOGLE_APPLICATION_CREDENTIALS`. A leading `~/` is expanded. There is no implicit developer-local credential fallback.

Scopes: reads request `analytics.readonly`; writes request `analytics.edit`. A service account that only holds readonly will mint fine for reads and fail at mint time for writes. An `authorized_user` token carries whatever the original grant allowed — the CLI cannot narrow it, so `health` reports `scope_requested`, not scope granted.

GA4 property resolution for data commands: `--property`, then `GA4_PROPERTY_ID`. The CLI does not hard-code the first authorized property or the second authorized property property IDs. For fleet health checks, pass explicit properties or set `GA4_PROPERTY_IDS`.

Important gotcha: Google Cloud API access is not GA4 property access. The service account must also be granted Viewer access inside each GA4 property. `health` / `doctor` distinguishes invalid credentials from property not shared / permission denied.

Writes need more than the scope: the account must be **Editor or Administrator** on the property. A `403` on a write means the token carried the scope but the role was missing — the CLI says so in the error.

## Global agent flags

Every command inherits:

- `--agent` = `--json --compact --no-input --yes`
- `--json`
- `--compact`
- `--no-input`
- `--yes` — also the explicit authorization for destructive Admin writes (see below); `--agent` alone does not satisfy that gate
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
- `admin` — escape hatch for any Admin API path/method (see Admin write surface)

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

## Admin write surface

Everything below mutates your GA4 configuration. Reads inside these groups (`list`, `get`) use the readonly scope; mutations use `analytics.edit`.

### The destructive-action gate

Destructive calls — every `delete`, plus `custom-dimensions archive` and `custom-metrics archive` — require a **typed** `--yes`:

```bash
# refused: --agent implies --yes, and that is deliberately not enough
google-analytics-pp-cli key-events delete --name properties/123/keyEvents/abc --agent

# allowed: --yes was actually typed
google-analytics-pp-cli key-events delete --name properties/123/keyEvents/abc --agent --yes
```

The gate inspects whether the `--yes` flag was set on the command line, not the resolved value, so an agent running in `--agent` mode can read and create but cannot destroy without a human putting `--yes` in the invocation.

### key-events

```bash
google-analytics-pp-cli key-events list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli key-events create --property "$GA4_PROPERTY_ID" --event generate_lead --agent
google-analytics-pp-cli key-events create --property "$GA4_PROPERTY_ID" --event purchase --counting ONCE_PER_SESSION --agent
google-analytics-pp-cli key-events patch --name properties/123/keyEvents/abc --counting ONCE_PER_EVENT --agent
google-analytics-pp-cli key-events delete --name properties/123/keyEvents/abc --agent --yes
```

Creating a key event that already exists returns `409 ALREADY_EXISTS` — which is the cheapest way to prove your credential can write without actually changing anything.

### custom-dimensions / custom-metrics

```bash
google-analytics-pp-cli custom-dimensions list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli custom-dimensions create --property "$GA4_PROPERTY_ID" \
  --parameter plan --display-name Plan --scope EVENT --agent
google-analytics-pp-cli custom-dimensions archive --name properties/123/customDimensions/456 --agent --yes

google-analytics-pp-cli custom-metrics list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli custom-metrics create --property "$GA4_PROPERTY_ID" \
  --parameter cart_value --display-name "Cart Value" --measurement-unit CURRENCY --agent
```

Archiving frees the parameter slot and is not reversible from the API, so it is gated like a delete.

### data-streams

`streams` remains the read shorthand; `data-streams` adds the full lifecycle.

```bash
google-analytics-pp-cli data-streams list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli data-streams create --property "$GA4_PROPERTY_ID" \
  --display-name "Marketing site" --type WEB_DATA_STREAM --uri https://example.com --agent
google-analytics-pp-cli data-streams patch --name properties/123/dataStreams/456 --display-name "Renamed" --agent
google-analytics-pp-cli data-streams delete --name properties/123/dataStreams/456 --agent --yes
```

`patch` builds its `updateMask` from the flags you actually passed, so it never clobbers a field you did not mention.

### admin — the escape hatch

Not every Admin resource has a typed command, and some live only in `v1alpha`. `admin` reaches any of them:

```bash
google-analytics-pp-cli admin GET properties/123/customDimensions --agent
google-analytics-pp-cli admin POST properties/123/keyEvents \
  --body '{"eventName":"signup","countingMethod":"ONCE_PER_EVENT"}' --agent
google-analytics-pp-cli admin PATCH properties/123/keyEvents/abc \
  --update-mask countingMethod --body '{"countingMethod":"ONCE_PER_SESSION"}' --agent
google-analytics-pp-cli admin POST properties/123/customDimensions --body @dimension.json --agent
google-analytics-pp-cli admin DELETE properties/123/customDimensions/456 --agent --yes
```

**`accessBindings` only exists in `v1alpha`.** Ask `v1beta` for it and Google answers `404` with an HTML error page rather than JSON:

```bash
google-analytics-pp-cli admin GET properties/123/accessBindings --api v1alpha --agent
```

`GET` paginates and merges the repeated list field by default; pass `--no-paginate` for a single raw page. `--body` takes inline JSON or `@file.json`. A `404` reminds you to retry with `--api v1alpha`.

## Examples

```bash
google-analytics-pp-cli agent-context --agent
google-analytics-pp-cli health --properties "$GA4_PROPERTY_IDS" --agent
google-analytics-pp-cli channels --property "$GA4_PROPERTY_ID" --start 28daysAgo --end yesterday --agent
google-analytics-pp-cli compare --property "$GA4_PROPERTY_ID" --metric sessions,totalRevenue --period wow --agent
google-analytics-pp-cli whats-changed --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli funnel --property "$GA4_PROPERTY_ID" --steps view_item,add_to_cart,begin_checkout,purchase --agent
```
