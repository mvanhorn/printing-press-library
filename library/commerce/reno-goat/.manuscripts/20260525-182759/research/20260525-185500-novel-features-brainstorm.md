# Home GOAT — Novel Features Brainstorm Audit Trail

## Date: 2026-05-25
## Run ID: 20260525-182759

## Customer Model (4 Personas)

1. **Maya the Renovator** — Homeowner doing a kitchen/bath remodel. Needs to compare prices across retailers, track a project budget, and know when prices drop.
2. **Darren the Designer** — Interior designer sourcing for clients. Needs spec sheets, cross-brand comparison, and project-level organization.
3. **Carlos the GC** — General contractor specifying fixtures. Needs foundational-category routing, brand cross-reference, and delivery availability by zip.
4. **Priya the Deal Hunter** — Budget-conscious shopper watching for sales and clearance. Needs price watch alerts and stale detection on saved items.

## Brainstorm Questions

1. What can ONLY a multi-source aggregator do that no single retailer can?
2. What local-first data patterns (SQLite, FTS5, price history) unlock capabilities no web app offers?
3. What workflow pain points do renovators/designers have that retailers ignore?
4. What agent-native features does a CLI enable that a browser cannot?

## All 13 Candidates Evaluated

### Survivors (shipped as Transcendence)

| # | Feature | Score | Persona | Rationale |
|---|---------|-------|---------|-----------|
| 1 | Price Watch | 8/10 | Maya, Priya | Multi-source price history is fundamentally impossible on any single retailer. SQLite stores every poll snapshot; threshold alerting is cron-native. High implementation feasibility — just a `watch` table + re-fetch loop. |
| 2 | Project Tracker | 7/10 | Maya, Darren | Cross-store project budgets are the #1 pain point for renovators. No retailer tracks spend at other retailers. Implementation: `projects` + `project_items` tables with FK to products. |
| 3 | Stale Detector | 7/10 | Darren, Maya | Saved products go stale silently — discontinued, price changed, out of stock. No retailer alerts you about products saved at *other* retailers. Implementation: re-fetch + diff against cached snapshot. |
| 4 | Spec Sheet Export | 6/10 | Carlos, Darren | Product specs are buried in unstructured HTML across retailers. Normalizing dimensions, materials, finishes into markdown/CSV is a CLI-native win. Implementation complexity is moderate — each source has different spec field shapes. |
| 5 | Brand Cross-Reference | 6/10 | Carlos, Priya | "Who else carries Kohler?" is a question no retailer answers. Cross-source brand matching with price comparison. Implementation: brand normalization + fan-out search filtered by brand. |

### Killed (with reasons)

| # | Feature | Score | Kill Reason |
|---|---------|-------|-------------|
| 6 | Visual Similarity Search | 5/10 | Requires ML model or Dupe.com API — neither available in v1. Deferred to Dupe integration phase. |
| 7 | Room Planner / Mood Board | 4/10 | Too ambitious for CLI; better as a web app feature. Scope creep. |
| 8 | AR/3D Preview Links | 3/10 | We'd just be linking to retailer AR features — no added value. |
| 9 | Price History Charts | 5/10 | Terminal charts are low-fidelity. The raw data (Price Watch) is the real value; charting is a UI concern for a web companion, not a CLI. |
| 10 | Auto-Negotiate / Coupon Apply | 2/10 | Ethically questionable, technically fragile, ToS-violating at most retailers. |
| 11 | Inventory Alert (back-in-stock) | 5/10 | Overlaps heavily with Stale Detector. Stale Detector already covers the "is this still available?" use case. |
| 12 | Multi-Unit Pricing / Bulk Calculator | 4/10 | Niche — only relevant for contractors buying multiples. Carlos persona served better by Brand Cross-Reference. |
| 13 | Style Matching / Aesthetic Classifier | 3/10 | Requires ML/embeddings for "mid-century modern" style detection. Out of scope for v1; would be a Dupe.com integration feature. |

## Decision Log

- **Price Watch beats Price History Charts**: The data layer (snapshots in SQLite) is the real transcendence. Terminal charts are a cosmetic feature that can be added later. Ship the watch polling + threshold alerting.
- **Stale Detector absorbs Inventory Alert**: Both answer "is my saved product still available/correct?" Stale Detector is the broader framing — it catches price changes AND discontinuation AND stock-outs.
- **Brand Cross-Reference beats Multi-Unit Pricing**: Brand availability across retailers serves 3 of 4 personas; bulk pricing only serves Carlos and only in narrow scenarios.
- **Spec Sheet Export kept despite moderate complexity**: Each source has different spec field shapes, but the normalization value is high for the designer/contractor personas. Worth the implementation cost.
