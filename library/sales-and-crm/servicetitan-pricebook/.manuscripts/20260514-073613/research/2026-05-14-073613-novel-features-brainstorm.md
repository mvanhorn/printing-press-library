# servicetitan-pricebook — Novel Features Brainstorm (audit trail)

Subagent: general-purpose, novel-features brainstorm + adversarial cut. First print (prior research = none).

## Customer model

**Pierce — JKA ops/automation owner (primary persona)**

**Today (without this CLI):** Pierce maintains JKA Well Drilling's entire ServiceTitan pricebook by hand in the ST web UI. When a 2M vendor quote lands, he opens each Material and Equipment record, clicks into the cost field, types the new cost, checks the markup math in his head against the tier ladder, then re-types the price. He keeps the 2M Part # discipline alive by remembering to paste it into the primary vendor slot every single time. He runs Claude Code on Windows and has already printed four sibling ST CLIs, so he expects this one to behave identically.

**Weekly ritual:** Every week he reconciles vendor quotes against live pricebook costs, sweeps for SKUs whose price drifted off the `MaterialsMarkup` tier ladder after a cost change, and audits warranty text so every entry is prefixed `Manufacturer's` with JKA's 1-year parts & labor offering sitting alongside it. He also spot-checks the category tree for orphaned or inactive-category SKUs.

**Frustration:** The UI gives him no way to ask "which SKUs are now mispriced for their tier?" or "which Materials are missing a 2M Part #?" — those are eyeball-and-spreadsheet jobs. Cost history is invisible: once he overwrites a cost, the prior value is gone, so he can't see drift over time. And the general ST MCP loads 600+ tools when all he wants is pricebook work.

**The JKA pricebook agent (Claude Code / Claude Desktop, code-orchestrated MCP)**

**Today (without this CLI):** The agent does pricebook work through the heavy general ST MCP, paying a 600+-tool token tax on every turn even for a single material lookup. It has no local state — every "did this cost change?" question is a fresh API round-trip, and it can't diff anything because nothing is cached.

**Weekly ritual:** It runs the documented JKA workflows on Pierce's behalf — ingest a vendor quote, reconcile part numbers, check markup discipline, audit warranty attribution — each currently a multi-call UI-mimicking sequence.

**Frustration:** Without a local SQLite snapshot it cannot answer drift/staleness questions in one shot; without ST-pricebook-shape-aware commands it has to reconstruct markup-ladder logic and warranty-prefix rules from scratch every session.

**Dana — JKA bookkeeper / margin reviewer (secondary persona)**

**Today (without this CLI):** Dana cares that margin is intact. She has no direct pricebook access habit; she asks Pierce "are our prices still right?" and waits. When a vendor raises costs, she has no view into whether prices followed.

**Weekly ritual:** Reviews whether recent vendor cost increases were passed through to price, and whether any SKU is selling below its tier-implied markup.

**Frustration:** There's no report. Margin erosion from un-followed cost increases is invisible until it shows up in the books a month later.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source |
|---|------|---------|-------------|---------|--------|
| 1 | Markup drift audit | `markup-audit` | For every synced material/equipment, compute actual `(price−cost)/cost`, find the `MaterialsMarkup` tier the cost falls in, flag SKUs whose actual markup deviates from the tier `percent`. | Pierce, Dana | (b) content pattern + (e) vision |
| 2 | Cost-drift report | `cost-drift` | Joins `sku_cost_history` snapshots to show every SKU whose cost moved since a given date, with old→new cost, old→new price, and whether price followed. | Dana, Pierce | (c) cross-entity + (b) snapshot table |
| 3 | Missing vendor part audit | `vendor-part-gaps` | Lists synced materials/equipment where `primaryVendor.vendorPart` is empty or null — the "missing 2M Part #" sweep. | Pierce | (a) frustration + (e) vision |
| 4 | Warranty attribution lint | `warranty-lint` | Scans equipment `manufacturerWarranty.description` + `serviceProviderWarranty` + `description` text; flags entries not prefixed `Manufacturer's` and equipment missing JKA's 1-yr parts & labor line. | Pierce | (a) frustration + (b) warranty shape |
| 5 | Stale price detector | `stale-price` | Flags SKUs whose `cost` changed in `sku_cost_history` but whose `price` `modifiedOn` is older than the cost change — price never followed the cost. | Dana, Pierce | (c) cross-entity + (b) snapshot |
| 6 | Orphan SKU finder | `orphan-skus` | Joins materials/equipment/services against `categories` to list SKUs assigned to inactive categories or to category IDs that don't exist. | Pierce | (a) frustration + (b) category tree |
| 7 | Vendor quote reconcile | `quote-reconcile <file>` | Takes a CSV of `vendorPart,cost`, self-healing-matches against synced SKUs by `primaryVendor.vendorPart`/`otherVendors[].vendorPart`, prints a diff of proposed cost changes (no write). | Pierce, agent | (a) frustration + (e) vision |
| 8 | Hold-markup repricer | `reprice --apply` | For SKUs flagged by `markup-audit`, computes the tier-correct price and emits the exact `material update`/`equipment update` payloads (`--dry-run` default). | Pierce | (b) content pattern + (e) vision |
| 9 | Category tree view | `tree` | Renders `categories` `parentId`/`subcategories`/`position` as an indented hierarchy with SKU counts per node. | Pierce | (b) category tree |
| 10 | Member-price gap audit | `member-price-audit` | Lists SKUs where `memberPrice` >= `price`, is zero, or null — member pricing that gives members no discount. | Dana | (b) member pricing |
| 11 | Multi-vendor cost compare | `vendor-compare` | For SKUs with `otherVendors[]`, flags any where an `otherVendors` cost is lower than `primaryVendor.cost` — primary vendor isn't the cheapest. | Pierce | (c) cross-entity + (b) vendor shape |
| 12 | Price change history | `history <sku>` | Prints the full `sku_cost_history` timeline for one SKU — every cost/price/vendorPart snapshot with dates. | Pierce, Dana | (b) snapshot table |
| 13 | Export feed snapshot diff | `feed-diff` | Pulls an export feed twice (continuation token) and diffs against the last cached feed run to show adds/removes/changes. | Pierce | (b) export feeds |
| 14 | Discount/fee usage audit | `discount-audit` | Lists `discounts-and-fees` that are inactive, expired, or have zero amount. | Pierce | (b) content pattern |
| 15 | Pricebook health summary | `health` | One-shot rollup: count of markup-drift, vendor-part-gaps, stale-price, orphan-skus, warranty-lint failures. | Pierce, agent | (c) cross-entity + (f) sibling pattern |
| 16 | Bulk-write planner | `bulk-plan <file>` | Turns a reviewed `quote-reconcile` diff into a single `pricebook bulk-update` payload instead of N individual updates. | Pierce, agent | (f) sibling intelligence + (b) bulk ops |

Inline kill/keep: #13 killed in Pass 2 (export feeds not sync targets, scope creep). #9, #14 on probation to Pass 3. All others pass kill/keep checks (local-SQLite or cross-join, no LLM dependency, no external service, read-only or `--dry-run`-gated, dogfood-verifiable).

## Survivors and kills

Pass 3 force-answers: #1 markup-audit SURVIVE (cross-join + tier ladder pattern). #2 cost-drift SURVIVE (local snapshot, killed #12 history). #3 vendor-part-gaps SURVIVE (null-filter API can't query). #4 warranty-lint SURVIVE (service-specific content pattern). #5 stale-price SOFT KILL — folded into #2 cost-drift. #6 orphan-skus SURVIVE (cross-join). #7 quote-reconcile SURVIVE (local join + multi-vendor matching). #8 reprice SURVIVE (write-arm of #1, distinct audit-vs-apply). #9 tree KILL (occasional, thin wrapper). #10 member-price-audit KILL (not in JKA workflows). #11 vendor-compare SOFT KILL (sub-weekly, partially served by quote-reconcile). #12 history KILL (single-SKU soft-kill). #13 feed-diff KILL (scope creep). #14 discount-audit KILL (tangential). #15 health SURVIVE (agent-shaped rollup). #16 bulk-plan SURVIVE (bulk-ops + agent-shaped output).

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Markup drift audit | `markup-audit` | 9/10 | Joins synced `materials`+`equipment` against the `materials-markup` tier ladder in local SQLite, computes actual vs tier-expected markup per SKU — no API call returns this | Brief Top Workflow #3, Product Thesis, User Vision |
| 2 | Cost-drift report | `cost-drift` | 8/10 | Reads `sku_cost_history` snapshots from local SQLite, diffs cost+price between dates, shows whether price followed cost — API exposes no history | Brief Data Layer, Top Workflow #2 |
| 3 | Missing vendor part audit | `vendor-part-gaps` | 8/10 | Null-filters synced `materials`/`equipment` on `primaryVendor.vendorPart` in local SQLite | Brief Top Workflow #1, Product Thesis |
| 4 | Warranty attribution lint | `warranty-lint` | 8/10 | Text-pattern lints synced `equipment` warranty + description fields against the `Manufacturer's`-prefix rule and JKA 1-yr parts & labor rule | Brief Top Workflow #4, memories `feedback_warranty_attribution` + `project_jka_service_warranty` |
| 5 | Orphan SKU finder | `orphan-skus` | 7/10 | Joins synced `materials`/`equipment`/`services` against `categories` to find SKUs in inactive or non-existent categories | Brief Top Workflow #5 |
| 6 | Vendor quote reconcile | `quote-reconcile <file>` | 8/10 | Self-healing-matches a `vendorPart,cost` CSV against synced `primaryVendor.vendorPart` + `otherVendors[].vendorPart`, prints a no-write cost diff | Brief Top Workflow #2, memory `project_vendor_quote_ingestion` |
| 7 | Hold-markup repricer | `reprice` | 7/10 | Takes `markup-audit` output, computes tier-correct price from the `materials-markup` ladder, emits exact update payloads (`--dry-run` default) | Brief Top Workflow #1 + #3, memory `project_pricebook_2m_workflow` |
| 8 | Pricebook health summary | `health` | 8/10 | Aggregates markup-drift, vendor-part-gaps, warranty-lint, cost-drift, orphan-skus counts from local SQLite into one agent-shaped rollup | Brief Codebase Intelligence, User Vision |
| 9 | Bulk-write planner | `bulk-plan <file>` | 7/10 | Transforms a reviewed `quote-reconcile`/`reprice` diff into a single `pricebook bulk-update` payload instead of N calls | Brief Codebase Intelligence, Top Workflow #2 |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Stale price detector (`stale-price`) | Splits one weekly question into a second command; `cost-drift` already reports whether price followed. | Cost-drift report (`cost-drift`) |
| Category tree view (`tree`) | Occasional-use and a thin wrapper over `categories list` with cosmetic indentation. | Orphan SKU finder (`orphan-skus`) |
| Member-price gap audit (`member-price-audit`) | Member pricing appears in no documented JKA workflow; monthly-at-best usage. | Markup drift audit (`markup-audit`) |
| Multi-vendor cost compare (`vendor-compare`) | Real but sub-weekly; vendor cost visibility already surfaced during `quote-reconcile`. | Vendor quote reconcile (`quote-reconcile`) |
| Price change history (`history`) | Single-SKU lookup is a soft-kill frequency; same `sku_cost_history` data served fleet-wide by `cost-drift`. | Cost-drift report (`cost-drift`) |
| Export feed snapshot diff (`feed-diff`) | Export feeds are explicitly not sync targets; would need separate caching infrastructure (scope creep). | Cost-drift report (`cost-drift`) |
| Discount/fee usage audit (`discount-audit`) | Discounts-and-fees are tangential to the documented margin/pricebook workflows; not a weekly ritual. | Pricebook health summary (`health`) |
