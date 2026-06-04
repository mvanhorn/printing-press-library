# GoDaddy Operation Risk Model

Generated: 2026-05-22 10:44 UTC
Updated: 2026-05-22 13:06 UTC

## Purpose

This is the barrier model for the official GoDaddy API map. It is intentionally conservative. The CLI should expose broad surface area, but execution must make account-changing, destructive, and purchase/billing routes hard to trigger by accident.

## Current Counts

| Risk class | Operations | Default barrier |
|---|---:|---|
| `read` | 61 | `none` |
| `validation` | 8 | `none` |
| `write` | 27 | `requires_allow_writes` |
| `destructive` | 17 | `requires_allow_writes_and_explicit_confirm` |
| `purchase_or_billing` | 25 | `requires_allow_writes_and_purchase_confirm` |

Total: 138 official operations across 12 Swagger specs.

## Generated Artifacts

- Normalized JSON route index: `godaddy/api-map/normalized/official-routes.json`
- Full TSV route index: `godaddy/api-map/normalized/official-routes.tsv`
- Risk counts by API group: `godaddy/api-map/normalized/risk-by-api.tsv`
- Risk counts overall: `godaddy/api-map/normalized/risk-summary.tsv`
- Account-action subset: `godaddy/api-map/normalized/account-action-routes.tsv`
- Classifier source: `godaddy/tools/build_official_index.jq`

## Classifier Rules

The classifier reads each official Swagger operation and assigns one class:

- `read`: `GET` operations.
- `validation`: non-GET operations whose operation id, path, or summary mentions validation, availability, schema, search, suggest, or resolution.
- `destructive`: `DELETE` operations, or operations whose text mentions delete, cancel, revoke, remove, unregister, or reject.
- `purchase_or_billing`: operations whose text mentions purchase, renew, register, bid, redeem, transfer, subscription, order, auction, or subaccount.
- `write`: remaining non-GET operations.

This is a first-pass heuristic, not a substitute for command review. When it over-classifies, keep the stronger barrier until a route-specific review proves the route is lower risk.

## Notable Barrier Decisions

- DNS `PATCH /v1/domains/{domain}/records` is `write`; it adds records.
- DNS `PUT /v1/domains/{domain}/records` is `write` but needs special copy because it replaces all DNS records.
- DNS `DELETE /v1/domains/{domain}/records/{type}/{name}` is `destructive`.
- Domain register, renew, transfer, redeem, privacy purchase, and auction bids are `purchase_or_billing`.
- Subscription cancellation and shopper deletion are `destructive`.

## CLI Implementation

The local PP scaffold now applies the risk model after generated Cobra commands are assembled:

- Code: `library/developer-tools/godaddy/internal/cli/risk_annotations.go`
- Tests: `library/developer-tools/godaddy/internal/cli/risk_annotations_test.go`
- Human help proofs:
  - `godaddy/proofs/godaddy-risk-help-domains-purchase-2026-05-22.txt`
  - `godaddy/proofs/godaddy-risk-help-dns-replace-2026-05-22.txt`
- Agent-context proof: `godaddy/proofs/godaddy-risk-agent-context-2026-05-22.json`

Each endpoint command with `pp:method` and `pp:path` now receives:

- `pp:risk`: `read`, `validation`, `write`, `destructive`, or `purchase_or_billing`
- `pp:barrier`: currently `none` or `requires_GODADDY_ALLOW_WRITES`
- `pp:warning`: human/agent copy for non-read routes

The annotations are a visibility layer. The actual account-write blocker remains in the client: live mutating calls require `GODADDY_ALLOW_WRITES=1`, while `--dry-run` previews without sending.

## Next

Next risk-model work is live-read/account mapping, not more static route labeling. Keep all credential sourcing intentional and read-only unless the user explicitly approves a mutation.
