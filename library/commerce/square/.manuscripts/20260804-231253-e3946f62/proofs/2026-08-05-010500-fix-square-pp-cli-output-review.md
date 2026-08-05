# Independent semantic output review

Status: WARN, with no blocking findings. All 6 eligible custom samples were assessed.

## Findings sent to polish

1. Request check reports `valid: true` even when runtime schema validation is unavailable, the API version is missing, and mutation approval failed. Keep `safe_to_send: false`; either narrow the positive field to specific parse/operation facts or make aggregate validity false when readiness checks fail.
2. Empty `change_events`, `drifted_resources`, and `subscription_changes` values serialize as `null` while populated values are arrays. Initialize them as empty arrays for a stable agent-facing JSON shape.

## Passed semantics

- Local/computed provenance is explicit.
- Empty-state counts and limitations are honest.
- Webhook health does not claim a detected gap proves an event was lost.
- Service review labels workload as a proxy and does not overclaim utilization or payment proof.
- Request check clearly states that no request was sent and keeps `safe_to_send: false`.
- No aggregation, ranking, URL, mojibake, or source-label problems appeared.

## Focused re-review after polish

PASS. Request check now emits `valid: false` alongside failed readiness, unavailable schema validation, and `safe_to_send: false`. Inventory drift emits `change_events: []` and `drifted_resources: []`; webhook health emits `subscription_changes: []`. The custom scorecard probe remains 6/6. No WARN or FAIL remains.
