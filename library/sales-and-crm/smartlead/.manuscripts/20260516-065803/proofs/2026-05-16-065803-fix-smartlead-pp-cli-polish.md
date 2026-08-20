# SmartLead CLI — Phase 5.5 Polish

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 79/100 (B) | 80/100 (A) |
| Verify | 100% | 100% |
| Dogfood | WARN | PASS |
| Go vet | 0 | 0 |
| Tools-audit | 20 pending | 0 pending |

## Fixes applied by polish
- Fixed campaign_id rendering as scientific-notation float ("3.093733e+06")
  instead of an integer ID — added store.FormatIDValue, applied at both
  UpsertBatch ID-extraction sites and both sync.go extractID sites. Campaign
  IDs in health/dupes output are now usable as input to other commands.
- Added store.coalesceFKValue (falls back to parent_id when the typed FK
  column is absent), applied to all 11 typed-table inserts — a more general
  form of the dependent-resource FK fix.
- Removed dead function extractResponseData from helpers.go.
- Accepted all 20 tools-audit findings in the ledger (generated DO-NOT-EDIT
  parent groupers carrying parentNoSubcommandRunE; surfaced as a retro item).

## Skipped (structural, not defects)
- auth_protocol 4/10 — SmartLead uses query-param api_key auth; the scorecard
  dimension structurally penalizes non-header schemes. The CLI implements the
  API's actual auth correctly.
- type_fidelity 1/5 — the authored spec carries no typed response schemas
  (SmartLead does not publish them); output is generic JSON.

ship_recommendation: ship
further_polish_recommended: no
