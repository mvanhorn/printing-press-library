# Judge.me Absorb Manifest

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Product review widget | Official OpenAPI / Hydrogen wrappers | (generated endpoint) widgets product-review | Agent JSON/select output and MCP exposure |
| 2 | Product preview badge | Official OpenAPI / Hydrogen wrappers | (generated endpoint) widgets preview-badge | Agent JSON/select output and MCP exposure |
| 3 | Featured review carousel | Official OpenAPI | (generated endpoint) widgets featured-carousel | Agent JSON/select output |
| 4 | Floating reviews tab | Official OpenAPI | (generated endpoint) widgets reviews-tab | Agent JSON/select output |
| 5 | All-reviews page | Official OpenAPI | (generated endpoint) widgets all-reviews-page | Agent JSON/select output |
| 6 | Verified badge | Official OpenAPI | (generated endpoint) widgets verified-badge | Agent JSON/select output |
| 7 | All-reviews count | Official OpenAPI | (generated endpoint) widgets all-reviews-count | Structured output |
| 8 | All-reviews rating | Official OpenAPI | (generated endpoint) widgets all-reviews-rating | Structured output |
| 9 | Shop-review count | Official OpenAPI | (generated endpoint) widgets shop-reviews-count | Structured output |
| 10 | Shop-review rating | Official OpenAPI | (generated endpoint) widgets shop-reviews-rating | Structured output |
| 11 | Widget settings | Official OpenAPI | (generated endpoint) widgets settings | Selectable agent output |
| 12 | Miracle HTML | Official OpenAPI | (generated endpoint) widgets html-miracle | Safe structured wrapper |
| 13 | Checkout-comments widget | Official OpenAPI | (generated endpoint) widgets checkout-comments | Agent JSON/select output |
| 14 | List reviews | Official OpenAPI / Pipedream | (behavior in judgeme-pp-cli reviews index) requires explicit population labeling and never claims full-corpus completeness | Population safety and loop warning |
| 15 | Create review | Official OpenAPI / Pipedream | (behavior in judgeme-pp-cli reviews create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 16 | Update review curation | Official OpenAPI / Pipedream | (behavior in judgeme-pp-cli reviews update) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 17 | Count reviews | Official OpenAPI | (generated endpoint) reviews reviewers-count | Structured output |
| 18 | Get review | Official OpenAPI | (generated endpoint) reviews get | Structured output |
| 19 | Get reviewer | Official OpenAPI | (generated endpoint) reviewers get | Structured output |
| 20 | Update reviewer | Official OpenAPI | (behavior in judgeme-pp-cli reviewers update) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 21 | Reviewer data request | Official OpenAPI | (behavior in judgeme-pp-cli reviewers data-request) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 22 | List webhooks | Official OpenAPI | (generated endpoint) webhooks index | Structured output |
| 23 | Delete webhook | Official OpenAPI | (behavior in judgeme-pp-cli webhooks destroy) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 24 | Create webhook | Official OpenAPI | (behavior in judgeme-pp-cli webhooks create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 25 | Get webhook | Official OpenAPI | (generated endpoint) webhooks get | Structured output |
| 26 | Update webhook | Official OpenAPI | (behavior in judgeme-pp-cli webhooks update) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 27 | Bulk-create webhooks | Official OpenAPI | (behavior in judgeme-pp-cli webhooks bulk-create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 28 | Shop info | Official OpenAPI | (generated endpoint) shops info | Structured output |
| 29 | Update shop | Official OpenAPI | (behavior in judgeme-pp-cli shops update) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 30 | Uninstall shop | Official OpenAPI | (behavior in judgeme-pp-cli shops destroy) explicit `--apply` gate plus `--dry-run` | Read-only default and destructive disclosure |
| 31 | Create shop/order payload | Official OpenAPI | (behavior in judgeme-pp-cli shops comments-create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 32 | Read settings | Official OpenAPI | (generated endpoint) settings index | Structured output |
| 33 | Create public reply | Official OpenAPI | (behavior in judgeme-pp-cli replies create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 34 | Create private reply | Official OpenAPI | (behavior in judgeme-pp-cli private-replies create) explicit `--apply` gate plus `--dry-run` | Read-only default |
| 35 | Reputation summary | Public `judge-me` printed CLI | judgeme-pp-cli reputation summary | Compatibility plus verified mirror provenance |
| 36 | Product reputation hotspots | Public `judge-me` printed CLI | judgeme-pp-cli reputation products | Compatibility plus body-dedup metrics |
| 37 | Moderation attention queue | Public `judge-me` printed CLI | judgeme-pp-cli reputation moderation-queue | Compatibility plus population labels |
| 38 | Trust settings audit | Public `judge-me` printed CLI | judgeme-pp-cli reputation settings-audit | Compatibility |
| 39 | Product evidence view | Public `judge-me` printed CLI | judgeme-pp-cli reputation product | Compatibility |
| 40 | Standard agent/CLI framework | House conventions | (behavior in judgeme-pp-cli doctor) doctor, feedback, profiles, MCP, JSON/CSV/select, exit codes | Consistent operator and agent surface |

## Transcendence

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Completeness-safe mirror | `sync --resources reviews --full` | 10/10 | hand-code | Calls `/reviews/count` and rating-partitioned `/reviews`, detects all-seen page loops, deduplicates IDs, and commits only on exact equality. | Live 12,871-row defect receipt; user requirement; official count endpoint. | Use this command to refresh the canonical local corpus. Do NOT use `reviews index` to claim corpus completeness. |
| 2 | Filtered corpus export | `reviews export` | 9/10 | hand-code | Reads the verified SQLite review table and emits date/rating/product/population filters as JSON or CSV. | User requirement; downstream commerce-intel plan; official review fields. | none |
| 3 | Population census | `reviews populations` | 9/10 | hand-code | Aggregates explicit moderation/storefront populations from local published, curated, and hidden fields. | 9,638 published vs 3,229 hidden live evidence; Judge.me four-status documentation. | Use this command for published/hidden/pending/archive counts. Do NOT use `reviews unique-bodies` for moderation populations. |
| 4 | Syndication clusters | `reviews syndication` | 9/10 | hand-code | Groups the local mirror by normalized SHA-256 body hash and reports bodies attached to multiple product identifiers. | User requirement; 12,871 rows vs 9,552 unique non-empty bodies in receipt. | Use this command to find the same body attached to multiple products. Do NOT use it for a deduplicated export; use `reviews unique-bodies` or `reviews export --unique-bodies`. |
| 5 | Unique-body view | `reviews unique-bodies` | 8/10 | hand-code | Selects one deterministic representative per body hash from the verified local review table while preserving source row counts. | User requirement; receipt demonstrates syndicated duplicates. | Use this command for one representative row per normalized body. Do NOT use it for syndication cluster diagnostics; use `reviews syndication`. |

## Scope Approval

The user's initial request explicitly requires the five transcendence rows, full endpoint coverage, private RonanRx publication, and live completeness proof. It is treated as approval to generate without a redundant mid-run prompt.
