---
name: pp-google-analytics
description: Google Analytics 4 Printing Press CLI for GA4 raw reports, funnels, acquisition/revenue insights, period comparisons, anomaly scans, property-access health checks, and gated GA4 Admin writes (key events, custom dimensions/metrics, data streams).
tags: [printing-press, ga4, google-analytics, analytics, marketing]
---

# pp-google-analytics

## Prerequisites: Install the CLI

This skill drives the `google-analytics-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-analytics --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `google-analytics-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/cmd/google-analytics-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs live GA4 property data, acquisition/revenue summaries, ecommerce diagnostics, funnels, period comparisons, anomaly scans, or property-access diagnostics. It also administers GA4 configuration: key events, custom dimensions and metrics, and data streams. Use `google-search-console-pp-cli` for Search Console.


Use `google-analytics-pp-cli` for GA4-only analytics work. Search Console is covered by `google-search-console-pp-cli`; do not use the legacy combined GSC/GA4 CLI for new work.

## Auth and property selection

- Credentials: `--credentials`, else `GOOGLE_ANALYTICS_ADC`, else `GOOGLE_APPLICATION_CREDENTIALS`. Both a service-account JSON key and an ADC `authorized_user` JSON (`client_id` + `client_secret` + `refresh_token`, what `gcloud auth application-default login` writes) are accepted; the shape is detected automatically.
- Scopes: reads request `analytics.readonly`, writes request `analytics.edit`. An `authorized_user` token carries whatever its original grant allowed, so `health` reports `scope_requested`, not scope granted.
- Property: pass `--property`, or set `GA4_PROPERTY_ID`.
- Fleet health checks: `health --properties "$GA4_PROPERTY_IDS" --agent` or set `GA4_PROPERTY_IDS`.

Durable gotcha: Google Cloud API access is not GA4 property access. The service account must be granted Viewer access inside the GA4 property. If `health` shows token/admin OK but a property check is 403/404, fix the GA4 property grant rather than rotating credentials.

Durable gotcha for writes: the scope is necessary but not sufficient — the account must also be **Editor or Administrator** on the property. A `403` on a write means the role is missing, not the scope.

## Best commands for agents

```bash
google-analytics-pp-cli agent-context --agent
google-analytics-pp-cli health --properties "$GA4_PROPERTY_IDS" --agent
google-analytics-pp-cli channels --property "$GA4_PROPERTY_ID" --start 28daysAgo --end yesterday --agent
google-analytics-pp-cli sources --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli top-pages --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli compare --property "$GA4_PROPERTY_ID" --metric sessions,totalRevenue --period wow --agent
google-analytics-pp-cli whats-changed --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli revenue --property "$GA4_PROPERTY_ID" --by channel --agent
google-analytics-pp-cli funnel --property "$GA4_PROPERTY_ID" --steps view_item,add_to_cart,begin_checkout,purchase --agent
```

## Admin writes (gated)

Destructive calls — every `delete`, plus `custom-dimensions archive` and `custom-metrics archive` — require a **typed** `--yes`. `--agent` expands to include `--yes`, and that deliberately does **not** satisfy the gate: the CLI checks whether `--yes` appeared on the command line. An agent can list and create on its own; destroying needs a human to add `--yes`.

```bash
google-analytics-pp-cli key-events list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli key-events create --property "$GA4_PROPERTY_ID" --event generate_lead --agent
google-analytics-pp-cli key-events patch --name properties/123/keyEvents/abc --counting ONCE_PER_SESSION --agent
google-analytics-pp-cli key-events delete --name properties/123/keyEvents/abc --agent --yes

google-analytics-pp-cli custom-dimensions create --property "$GA4_PROPERTY_ID" --parameter plan --display-name Plan --scope EVENT --agent
google-analytics-pp-cli custom-metrics create --property "$GA4_PROPERTY_ID" --parameter cart_value --display-name "Cart Value" --measurement-unit CURRENCY --agent
google-analytics-pp-cli custom-dimensions archive --name properties/123/customDimensions/456 --agent --yes

google-analytics-pp-cli data-streams list --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli data-streams create --property "$GA4_PROPERTY_ID" --display-name "Marketing site" --type WEB_DATA_STREAM --uri https://example.com --agent
```

To prove a credential can write without changing anything, create a key event that already exists and expect `409 ALREADY_EXISTS`.

## Admin escape hatch

`admin <GET|POST|PATCH|PUT|DELETE> <path>` reaches any Admin API resource, including those without a typed command:

```bash
google-analytics-pp-cli admin GET properties/123/customDimensions --agent
google-analytics-pp-cli admin PATCH properties/123/keyEvents/abc --update-mask countingMethod --body '{"countingMethod":"ONCE_PER_SESSION"}' --agent
google-analytics-pp-cli admin POST properties/123/customDimensions --body @dimension.json --agent
google-analytics-pp-cli admin GET properties/123/accessBindings --api v1alpha --agent
```

Durable gotcha: **`accessBindings` lives only in `v1alpha`.** Asking `v1beta` returns `404` with an HTML error page instead of JSON. `GET` paginates and merges the list field unless you pass `--no-paginate`; `DELETE` is gated by the typed `--yes` rule above.

## Raw wrappers

`report`, `pivot`, `batch`, `realtime`, `metadata`, `compatibility`, `properties`, `property`, and `streams` are available for low-level escape hatches. Prefer novel commands when answering business questions.
