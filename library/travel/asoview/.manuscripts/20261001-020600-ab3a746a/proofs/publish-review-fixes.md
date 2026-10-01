# Publish review fixes

Greptile review of the original publish head identified three domain issues. All are fixed in the canonical source, local library and staging tree:

- Dated availability preserves a uniform closed, sold-out, deadline or quantity-limit slot state. Mixed unavailable states remain explicitly mixed; available/request-only/unknown precedence remains conservative. Complete synthetic availability requests test these cases.
- Recommendation presence cannot discard a matching-page continuation. Exhaustion uses matched venues rather than product-card count. Synthetic discovery requests test later-page matches and exclusion of recommendations, plus multiple products belonging to one venue.
- Optional response-cache persistence errors preserve valid live responses and source timestamps, with `meta.metrics.cache_write_failures` exposing the failure count. Fresh-local misses still fail without network; explicit taxonomy refresh retains its required persistence contract.

Deterministic regressions and full Go tests/vet/build passed. All seven fresh Press shipcheck legs passed. The independent semantic runner now passes 23 real-source assertions with no fixtures. The closed-slot test compares actual public activity stock and verifies the headline when every source slot is closed. A separate live product read is tested with a synthetic invalid cache destination; its source response is real and only the local storage failure is synthetic.

Fresh source checks completed at 2026-10-01T02:09:32.485835+00:00. New source-bound live dogfood acceptance and publish validation are rerun before packaging/push.
