# Changelog

## 1.2.0 - 2026-08-06

- Adds a GA4 Admin API write surface: `key-events` (list/create/patch/delete), `custom-dimensions` and `custom-metrics` (list/create/archive), and `data-streams` (list/get/create/patch/delete).
- Adds `admin <GET|POST|PATCH|PUT|DELETE> <path>` as a generic Admin escape hatch with `--body` (inline or `@file`), `--update-mask`, `--api v1beta|v1alpha`, and `--no-paginate`. `--api v1alpha` reaches resources such as `accessBindings` that v1beta answers with a 404 HTML page.
- Accepts ADC `authorized_user` credentials (`client_id` + `client_secret` + `refresh_token`) alongside service-account JSON keys, and adds `GOOGLE_ANALYTICS_ADC` to the resolution order. Previously `--credentials` assumed a service account, which blocked the whole write path.
- Requests `analytics.edit` for writes while reads keep `analytics.readonly`, caching one client per scope.
- Gates every destructive mutation behind a typed `--yes`. The check inspects whether the flag appeared on the command line, so `--agent` (which implies `--yes`) cannot authorize a delete or archive on its own.
- Surfaces actionable hints on Admin failures: 403 distinguishes a missing Editor/Administrator role from a missing scope, and 404 suggests retrying with `--api v1alpha`.

## 2026.7.1 - 2026-07-08

- fix(catalog): require Go 1.26.5 across published modules (#1467).

## 2026.6.3 - 2026-06-21

- fix(catalog): require Go 1.26.4 across published modules (#1308).

## 2026.6.2 - 2026-06-16

- Improve catalog descriptions (#1222).

## 2026.6.1 - 2026-06-12

- Baseline release metadata added for this published CLI.

## 1.1.0 - 2026-06-12

- Rebuilds the GA4 CLI to publish-grade structure with a typed `internal/ga4` Data/Admin/Funnel API layer and per-command CLI files.
- Adds meaningful unit tests for request builders, global flag behavior, response shaping, compare delta/% math, anomaly ranking, and HTTP error paths.
- Replaces draft research/proofs with real GA4 API research and live smoke JSON for every raw and novel command against two authorized GA4 properties.
- Fixes live-discovered request-shape bugs for pivot limits and dimension order-bys.

## 1.0.0 - 2026-06-12

- Initial private GA4-only Printing Press CLI.
- Adds raw Data/Admin API wrappers.
- Adds novel agent commands for channels, sources, top pages, events/conversions, funnels, compare, whats-changed, revenue, audience/cohort, and health/doctor.
