# Changelog

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
