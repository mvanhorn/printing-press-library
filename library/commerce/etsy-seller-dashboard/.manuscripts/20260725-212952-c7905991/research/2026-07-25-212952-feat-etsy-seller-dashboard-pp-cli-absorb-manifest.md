# Etsy Seller Dashboard CLI — Absorb Manifest

Run: `20260725-212952-c7905991`

## Source Catalog

| Source | Role | Access | Primary evidence |
|---|---|---|---|
| Marketplace Insights | Peer source; README lead only | Authenticated browser session | Captured Etsy AJAX endpoints plus Etsy Help |
| Etsy Ads | Peer source | Authenticated browser session | Captured listing-performance endpoint plus Etsy Help |
| Offsite Ads | Peer source | Authenticated browser session | Captured summary, traffic, channel, and listing endpoints plus Etsy Help |
| Sales & Discounts | Peer source | Authenticated browser session | Captured promotion and revenue-stat endpoint plus Etsy Help |

All commands are read-only. The CLI may cache Etsy responses and compute derived metrics locally in SQLite. Etsy-side mutations are excluded until separately approved and backed by replayable, idempotent contracts.

## Absorbed Feature Inventory

| ID | Source | CLI feature | Proposed command or behavior | Delivery | Evidence / constraint |
|---:|---|---|---|---|---|
| MI-01 | Marketplace Insights | Keyword snapshot | `insights keyword show TERM` | Spec-emits | Searches and listing counts from captured data endpoint |
| MI-02 | Marketplace Insights | Related keywords | `insights keyword related TERM` | Spec-emits | Captured exploratory-keyword search workflow |
| MI-03 | Marketplace Insights | Trend view | `insights trends show TERM` | Spec-emits | Captured trending-search-terms endpoint |
| MI-04 | Marketplace Insights | Trending categories | `insights categories list` | Spec-emits | Captured trending-categories endpoint |
| MI-05 | Marketplace Insights | Saved searches | `insights saved list` | Spec-emits | Captured saved-search-terms endpoint; read only |
| MI-06 | Marketplace Insights | Quota visibility | `insights quota show` | Hand-code | Weekly search allowance derived from response/account state |
| MI-07 | Marketplace Insights | Cache-first repeat lookup | `--cache-ttl`, offline reuse | Hand-code | Avoids consuming scarce searches and supports deterministic replay |
| MI-08 | Marketplace Insights | Sort and export | `--sort`, `--format table,json,csv` | Hand-code | Local presentation over captured metrics |
| MI-09 | Marketplace Insights | Research lists and notes | `insights list add/show`, `insights note set` | Hand-code | Local SQLite only |
| MI-10 | Marketplace Insights | Snapshot history | `insights sync`, `insights history` | Hand-code | Timestamped local snapshots |
| MI-11 | Marketplace Insights | Delta analysis | `insights delta TERM` | Hand-code | Compares stored snapshots |
| MI-12 | Marketplace Insights | Opportunity score | `insights opportunities` | Hand-code | Deterministic demand-versus-competition formula |
| MI-13 | Marketplace Insights | Listing preview | `insights listings preview TERM` | Spec-emits | Listing previews present in Marketplace Insights data |
| MI-14 | Marketplace Insights | Listing rank and audit | `insights listing rank`, `insights listing audit` | Hand-code | Joins keyword result positions and listing metadata |
| MI-15 | Marketplace Insights | Listing comparison | `insights listing compare` | Hand-code | Deterministic comparison of cached listing metrics |
| MI-16 | Marketplace Insights | Research health | `insights health` | Hand-code | Cache freshness, quota, coverage, and missing-field checks |
| MI-17 | Marketplace Insights | Keyword suggestions | `insights suggest` | Hand-code | Ranks related/cached terms without extra Etsy calls |
| MI-18 | Marketplace Insights | Optional-field tolerant output | Stable null-safe schema | Hand-code | Captured responses vary by term and account |
| MI-19 | Marketplace Insights | Competition score | `insights competition TERM` | Hand-code | Normalized listings-to-searches calculation |
| AD-01 | Etsy Ads | Account performance summary | `ads summary` | Spec-emits | Views, clicks, orders, revenue, spend, and ROAS |
| AD-02 | Etsy Ads | Listing performance table | `ads listings` | Spec-emits | Captured paginated listing stats endpoint |
| AD-03 | Etsy Ads | Listing sorting and filtering | `--sort`, `--advertised`, `--section` | Hand-code | Endpoint supports sort/pagination; remaining filters local |
| AD-04 | Etsy Ads | Period selection | `--from`, `--to`, preset ranges | Spec-emits | Dashboard and endpoint accept date windows |
| AD-05 | Etsy Ads | Previous-period comparison | `ads compare` | Hand-code | Dashboard exposes comparison mode |
| AD-06 | Etsy Ads | Click-rate and ROAS diagnostics | `ads diagnose` | Hand-code | Uses captured click rate, spend, revenue, and ROAS |
| AD-07 | Etsy Ads | Search-term report | `ads search-terms` | Stub pending contract | Official dashboard documents last-30-day terms; endpoint not captured |
| AD-08 | Etsy Ads | Advertised-state visibility | `ads listings --advertised` | Spec-emits | Captured `is_promoted` state; read only |
| AD-09 | Etsy Ads | Export | `ads ... --format table,json,csv` | Hand-code | Local presentation |
| OA-01 | Offsite Ads | Attribution summary | `offsite summary` | Spec-emits | Captured total/direct/indirect revenue, fees, orders, and new buyers |
| OA-02 | Offsite Ads | Traffic series | `offsite traffic` | Spec-emits | Captured ad-traffic endpoint |
| OA-03 | Offsite Ads | Period comparison | `offsite compare` | Hand-code | Captured current and comparison stats |
| OA-04 | Offsite Ads | Channel performance | `offsite channels` | Spec-emits | Captured channel-performance endpoint |
| OA-05 | Offsite Ads | Listing attribution | `offsite listings` | Spec-emits | Captured listing clicks, orders, and revenue |
| OA-06 | Offsite Ads | Fee economics | `offsite economics` | Hand-code | Effective fee rate, attributed margin input, and Etsy policy tiers |
| OA-07 | Offsite Ads | Attribution-window annotation | Output metadata | Hand-code | Etsy documents a 30-day post-click attribution window |
| OA-08 | Offsite Ads | Enrollment visibility | `offsite status` | Hand-code | Read-only policy/account-state presentation |
| OA-09 | Offsite Ads | Export | `offsite ... --format table,json,csv` | Hand-code | Local presentation |
| SD-01 | Sales & Discounts | Promotion inventory | `promotions list` | Spec-emits | Captured combined promotions endpoint |
| SD-02 | Sales & Discounts | Promotion detail | `promotions show ID` | Hand-code | Local selection from captured definitions |
| SD-03 | Sales & Discounts | Promotion type coverage | Sale, promo code, bundle, targeted offer | Spec-emits | Etsy Help and captured type fields |
| SD-04 | Sales & Discounts | Status and schedule filters | `--status`, `--from`, `--to` | Hand-code | Captured dates and status |
| SD-05 | Sales & Discounts | Rules and conditions | Detail output for thresholds and eligibility | Spec-emits | Captured condition and discount fields |
| SD-06 | Sales & Discounts | Listing-set coverage | `promotions listings ID` | Hand-code | Captured listing-set references |
| SD-07 | Sales & Discounts | Targeted-offer sends | `promotions targeted` | Spec-emits | Captured targeted-offer state and send counts |
| SD-08 | Sales & Discounts | Revenue performance | `promotions performance` | Spec-emits | Captured revenue statistics |
| SD-09 | Sales & Discounts | Order, item, and AOV metrics | Performance columns when available | Stub pending field confirmation | Official dashboard documents discounted orders, items sold, AOV, and revenue |
| SD-10 | Sales & Discounts | Promotion comparison | `promotions compare ID...` | Hand-code | Deterministic local comparison |
| SD-11 | Sales & Discounts | Stacking-rule annotation | Detail warnings | Hand-code | Etsy documents bundle and shipping/discount stacking rules |
| SD-12 | Sales & Discounts | Export | `promotions ... --format table,json,csv` | Hand-code | Local presentation |

## Explicit Gaps and Safety Boundaries

| Gap | Treatment |
|---|---|
| Country-filtered Marketplace Insights metrics | Stub; no captured field or endpoint |
| Marketplace keyword clicks or CTR | Stub; Marketplace Insights exposes searches/listing counts, not engagement |
| Etsy Ads search-term contract | Stub until a sanitized endpoint contract is captured |
| Promotion order/item/AOV fields | Optional/stub until live response fields are confirmed |
| Cookie replay outside active Etsy browser session | `doctor --dry-run` must detect and explain browser-session requirement |
| Etsy Ads listing toggles or budget changes | Excluded mutation |
| Offsite Ads opt-out or enrollment changes | Excluded mutation |
| Promotion create, edit, pause, or delete | Excluded mutation |

## Novel Feature Survivors

| ID | Feature | Command | Delivery | Evidence and boundary |
|---:|---|---|---|---|
| NV-01 | Listing action queue | `listing action-queue` | Hand-code | Joins explicit listing IDs across all four sources; deterministic reason codes; missing data remains missing |
| NV-02 | Economics reconciliation | `economics reconcile` | Hand-code | Preserves source attribution boundaries; never reports net profit or double-counted revenue |
| NV-03 | Observed promotion lift | `promotion observed-lift ID` | Hand-code | Equal-length prior-window comparison; reports observed association, not causation |
| NV-04 | Acquisition channel gap | `acquisition channel-gap` | Hand-code | Compares separately labeled onsite/offsite efficiency signals over aligned periods |
| NV-05 | Research-quota allocation | `quota allocate` | Hand-code | Ranks stale demand-evidence gaps without spending Marketplace Insights quota |
| NV-06 | Visibility-performance gap | `listing visibility-gap` | Hand-code | Uses explicit/observed listing-keyword mappings; never fabricates keyword revenue |
| NV-07 | Cross-surface anomalies | `growth anomalies` | Hand-code | Deterministic historical outliers and coincident movements across synchronized sources |

Full scoring, acceptance criteria, scope redirects, and rejection reasons are recorded in the companion novel-features brainstorm.
