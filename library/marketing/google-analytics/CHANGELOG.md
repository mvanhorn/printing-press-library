# Changelog

## 1.1.0 - 2026-06-12

- Rebuilds the GA4 CLI to publish-grade structure with a typed `internal/ga4` Data/Admin/Funnel API layer and per-command CLI files.
- Adds meaningful unit tests for request builders, global flag behavior, response shaping, compare delta/% math, anomaly ranking, and HTTP error paths.
- Replaces draft research/proofs with real GA4 API research and live smoke JSON for every raw and novel command against BestSelf `280199692` and LittleMight `540652239`.
- Fixes live-discovered request-shape bugs for pivot limits and dimension order-bys.

## 1.0.0 - 2026-06-12

- Initial private GA4-only Printing Press CLI.
- Adds raw Data/Admin API wrappers.
- Adds novel agent commands for channels, sources, top pages, events/conversions, funnels, compare, whats-changed, revenue, audience/cohort, and health/doctor.
