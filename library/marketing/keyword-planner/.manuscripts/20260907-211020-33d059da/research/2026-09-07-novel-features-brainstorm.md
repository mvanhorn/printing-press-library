---
type: novel-features
api: keyword-planner
created: 2026-09-08
public_projection: true
---

# Evidence workflows beyond direct API calls

| Command | Problem solved | Verification |
|---|---|---|
| portfolio trace | A reported number needs an exact request and response receipt. | Reconcile receipt identity, scope, raw hash and monthly value. |
| portfolio coverage | A request can return fewer months or terms than requested. | Compare requested, returned, missing, raw-only and incomplete coverage. |
| portfolio variants | Close variants can be confused with independent demand. | Preserve ordered inputs and returned/variant lineage without multiplying volume. |
| portfolio diff | Repeated calls must preserve history and reveal changes. | Compare compatible snapshots and retain unchanged prior rows. |
| portfolio integrity | Stored evidence and normalized tables can drift. | Check hashes, receipts, row counts and coverage agreement. |
| portfolio safe-stats | Ambiguous or incomplete data can pollute summaries. | Exclude unsafe rows and retain scope/currency provenance. |

All six workflows operate on the local portfolio without credentials or network access. Ranking, revenue predictions, campaign mutations, forecasting and scheduled collection were excluded from the product scope.
