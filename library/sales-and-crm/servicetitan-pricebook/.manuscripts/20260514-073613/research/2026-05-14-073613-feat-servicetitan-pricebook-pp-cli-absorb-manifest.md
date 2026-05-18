# servicetitan-pricebook Absorb Manifest

ServiceTitan is a closed/proprietary ERP API; the competitive landscape is narrow. The only "competitors" are ST's own web UI/mobile, the giant general ST MCP that this per-module split replaces (web search confirms only general 60–100+-tool ST MCP servers exist — `glassdoc/servicetitan-mcp`, `BusyBee3333/servicetitan-mcp-2026-complete`, `JordanDalton/ServiceTitanMcpServer` — each bundling pricebook inside a huge suite; no focused pricebook CLI/MCP exists), and the four sibling generated CLIs in the user's library (`servicetitan-crm`, `servicetitan-dispatch`, `servicetitan-inventory`, `servicetitan-jpm`). The absorb manifest matches every endpoint the ST UI exposes, then transcends with cross-entity SQLite queries and JKA-workflow-grounded commands the UI cannot answer.

## Absorbed (match or beat everything that exists)

40 operations across 10 resources. All ship as Cobra commands; none as stubs.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | List client-specific rate sheets | ST UI / general MCP | `client-specific-pricing list --json` | Offline cache, FTS, agent-native |
| 2 | Update rate sheet | ST UI | `client-specific-pricing update <id> --dry-run` | Dry-run, idempotent |
| 3 | List categories | ST UI | `categories list --json --select id,name,parentId,active` | Offline cache |
| 4 | Create category | ST UI | `categories create --dry-run` | Dry-run, --stdin batch |
| 5 | Get category | ST UI | `categories get <id>` | --select compact output |
| 6 | Update category | ST UI | `categories update <id> --dry-run` | Dry-run |
| 7 | Delete category | ST UI | `categories delete <id> --dry-run` | Dry-run |
| 8 | List discounts & fees | ST UI | `discounts-and-fees list --json` | Offline cache |
| 9 | Create discount/fee | ST UI | `discounts-and-fees create --dry-run` | Dry-run |
| 10 | Get discount/fee | ST UI | `discounts-and-fees get <id>` | --select |
| 11 | Update discount/fee | ST UI | `discounts-and-fees update <id> --dry-run` | Dry-run |
| 12 | Delete discount/fee | ST UI | `discounts-and-fees delete <id> --dry-run` | Dry-run |
| 13 | List equipment | ST UI / general MCP | `equipment list --active True --json` | Offline cache, FTS, --select |
| 14 | Create equipment | ST UI | `equipment create --dry-run` | Dry-run, --stdin batch |
| 15 | Get equipment | ST UI | `equipment get <id>` | --select |
| 16 | Update equipment | ST UI | `equipment update <id> --dry-run` | Dry-run |
| 17 | Delete equipment | ST UI | `equipment delete <id> --dry-run` | Dry-run |
| 18 | Export categories feed | ST general MCP | `export categories --since <iso>` | Continuation-token state, --json |
| 19 | Export equipment feed | ST general MCP | `export equipment --since <iso>` | Continuation-token state |
| 20 | Export services feed | ST general MCP | `export services --since <iso>` | Continuation-token state |
| 21 | Export materials feed | ST general MCP | `export materials --since <iso>` | Continuation-token state |
| 22 | Get pricebook images | ST UI | `images get` | Offline cache |
| 23 | Upload pricebook image | ST UI | `images upload --dry-run` | Dry-run |
| 24 | List materials | ST UI / general MCP | `materials list --active True --json` | Offline cache, FTS, --select |
| 25 | Create material | ST UI | `materials create --dry-run` | Dry-run, --stdin batch |
| 26 | Get material | ST UI | `materials get <id>` | --select |
| 27 | Update material | ST UI | `materials update <id> --dry-run` | Dry-run |
| 28 | Delete material | ST UI | `materials delete <id> --dry-run` | Dry-run |
| 29 | List material cost types | ST UI | `materials cost-types --json` | Offline cache (small lookup) |
| 30 | List materials markup tiers | ST UI | `materials-markup list --json` | Offline cache (the tier ladder) |
| 31 | Create markup tier | ST UI | `materials-markup create --dry-run` | Dry-run |
| 32 | Get markup tier | ST UI | `materials-markup get <id>` | --select |
| 33 | Update markup tier | ST UI | `materials-markup update <id> --dry-run` | Dry-run |
| 34 | Bulk-create pricebook | ST UI / general MCP | `pricebook bulk-create --stdin --dry-run` | Batch stdin, dry-run |
| 35 | Bulk-update pricebook | ST UI / general MCP | `pricebook bulk-update --stdin --dry-run` | Batch stdin, dry-run |
| 36 | List services | ST UI / general MCP | `services list --active True --json` | Offline cache, FTS, --select |
| 37 | Create service | ST UI | `services create --dry-run` | Dry-run, --stdin batch |
| 38 | Get service | ST UI | `services get <id>` | --select |
| 39 | Update service | ST UI | `services update <id> --dry-run` | Dry-run |
| 40 | Delete service | ST UI | `services delete <id> --dry-run` | Dry-run |

All 40 ship as full implementations. No stubs. Plus framework-provided: `sync` (pulls cacheable entities into local SQLite + snapshots cost/price history), FTS5 `search` across cached SKUs, `sql`, `doctor`, `auth`, `reconcile`, `stale`, `context`.

## Transcendence (only possible with our approach)

12 features, all scored ≥5/10. Features 1–9 came from the customer-grounded subagent; features 10–12 were added by the user at the Phase 1.5 gate (de-duplication, sales-copy description audit, natural-language part finder). Each has a persona and a buildability proof. None ship as stubs.

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Markup drift audit | `markup-audit --json` | 9/10 | Joins synced `materials`+`equipment` against the `materials-markup` tier ladder in local SQLite; computes actual `(price−cost)/cost` vs the tier-expected `percent` per SKU; flags deviations. No single API call returns this. | Pierce, Dana |
| 2 | Cost-drift report | `cost-drift --since <iso>` | 8/10 | Reads `sku_cost_history` snapshots from local SQLite, diffs cost+price between dates, shows whether price followed the cost change. The ST API exposes no history. | Dana, Pierce |
| 3 | Missing vendor-part audit | `vendor-part-gaps --json` | 8/10 | Null-filters synced `materials`/`equipment` on `primaryVendor.vendorPart` in local SQLite — the "missing 2M Part #" sweep. The API has no "where vendorPart is empty" query. | Pierce |
| 4 | Warranty attribution lint | `warranty-lint --json` | 8/10 | Text-pattern lints synced `equipment` `manufacturerWarranty`/`serviceProviderWarranty`/`description` fields against the `Manufacturer's`-prefix rule and the JKA 1-yr parts & labor rule. | Pierce |
| 5 | Orphan SKU finder | `orphan-skus --json` | 7/10 | Joins synced `materials`/`equipment`/`services` against `categories` in local SQLite to list SKUs assigned to inactive or non-existent categories. | Pierce |
| 6 | Vendor quote reconcile | `quote-reconcile <file>` | 8/10 | Self-healing-matches a vendor-doc cost file against synced `primaryVendor.vendorPart` + `otherVendors[].vendorPart` in local SQLite; prints a no-write cost diff. Accepts `--format csv` or `--format json` so Claude can hand it the structured extraction of a quote / order confirmation / invoice PDF. `--apply` routes the accepted diff through `bulk-plan`. | Pierce, agent |
| 7 | Hold-markup repricer | `reprice [--apply]` | 7/10 | Takes `markup-audit` output, computes the tier-correct price from the `materials-markup` ladder, emits exact `materials update`/`equipment update` payloads (`--dry-run` default; `--apply` to write). | Pierce |
| 8 | Pricebook health summary | `health --json` | 8/10 | Aggregates markup-drift, vendor-part-gaps, warranty-lint, cost-drift, orphan-skus, dedupe, copy-audit counts from local SQLite into one compact agent-shaped rollup — sized for agent priming. | Pierce, agent |
| 9 | Bulk-write planner | `bulk-plan <file>` | 7/10 | Transforms a reviewed `quote-reconcile`/`reprice`/`copy-audit` diff into a single `pricebook bulk-update` payload instead of N individual update calls (matters under ST's ~7k/hr rate limit). | Pierce, agent |
| 10 | Duplicate-SKU detector | `dedupe --json` | 8/10 | Fuzzy-matches synced `materials`/`equipment` against each other on normalized `code` + `displayName` + `description` + `vendorPart`; clusters near-duplicates with a similarity score so Pierce can collapse excess pricebook growth. Pure local SQLite — the ST API has no "find SKUs like this one" query. | Pierce |
| 11 | Sales-copy description audit | `copy-audit --json` | 7/10 | Flags synced SKUs whose `displayName`/`description` are empty, too short, ALL-CAPS, a bare part number, or otherwise not customer-facing. A sales-copy agent rewrites the flagged entries; `materials update` / `equipment update` / `bulk-plan` writes them back. Service-specific content-quality lint. | Pierce, sales-copy agent |
| 12 | Natural-language part finder | `find <description>` | 6/10 | Forgiving multi-field ranked search over synced SKUs (`code` + `displayName` + `description` + `category` + `vendorPart`) tuned for "describe the part, I don't know the code" — returns suggested SKUs with the fields a tech needs (code, price, vendor part, category, active). More than the framework `search`: domain-tuned ranking and tech-facing output shape. | JKA field tech, Pierce |

### Dropped prior features
First print — no prior research to reconcile.

### User additions at Phase 1.5 gate
Features 10–12 plus the `quote-reconcile --format json` / `--apply` enhancement (feature 6) were added by the user at the absorb gate. All four trace to documented JKA pain: pricebook bloat, non-customer-facing descriptions, techs unable to find parts, and the vendor-document ingestion loop. The LLM work in each (digesting a vendor PDF, writing sales copy, translating a natural-language part request) is done by Claude *calling* the CLI — the CLI surfaces stay deterministic and dogfood-verifiable.

## Why this is the GOAT for ServiceTitan Pricebook

- **Coverage:** 40/40 ST Pricebook v2 operations, no stubs. The general ST MCP advertises these too, but inside a 600+-tool bundle that dominates an agent's context window.
- **Local store + cross-entity joins + cost history:** None of the 9 transcendence features can be answered from a single ST API call. The `sku_cost_history` snapshot table is what makes cost-drift, markup-drift, and stale detection one-shot.
- **Grounded in JKA's documented workflows:** markup discipline, 2M Part # discipline, warranty attribution, vendor-quote reconcile — every transcendence feature traces to a Top Workflow in the brief or an auto-memory entry.
- **Composed auth done right:** Four sibling CLIs already prove the `ST_APP_KEY` + OAuth2 bearer + tenant-injection flow against the live ST API. The whitespace-stripping defense prevents the known JKA `invalid_client` gotcha.
- **Token-efficient MCP surface:** `x-mcp` enrichment (stdio+http transport, code orchestration, hidden endpoint tools) means the agent sees a thin search+execute pair over ~65 tools, not 600+.
