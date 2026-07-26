# Etsy Seller Dashboard CLI — Novel Features Brainstorm

Run: `20260725-212952-c7905991`

## Customer Model

- **POD catalog operator:** researches niche phrases, checks which listings deserve paid visibility, and identifies inventory that may justify a promotion. Today this requires four tabs and a spreadsheet.
- **Seasonal physical-goods planner:** compares keyword/category momentum with ad orders, offsite traffic, and campaign dates before purchasing materials or scheduling production.
- **Multi-shop growth operator:** normalizes listing IDs and date ranges, then rebuilds recurring comparisons for each shop.
- **Profit-conscious seller:** compares Etsy Ads spend/revenue, Offsite Ads fees/attributed revenue, and promotion performance without accidentally combining incompatible attribution totals.

## Candidates Before Adversarial Cut

| # | Candidate | Command | Description | Verdict |
|---:|---|---|---|---|
| 1 | Listing action queue | `listing action-queue` | Joins demand, onsite ads, offsite acquisition, and promotion observations into deterministic review priorities. | Keep |
| 2 | Economics reconciliation | `economics reconcile` | Separates onsite spend, offsite fees, attributed revenue, and promotion revenue without claiming unobserved net profit. | Keep |
| 3 | Observed promotion lift | `promotion observed-lift <promotion-id>` | Compares promotion dates with equal-length prior periods across promotion, onsite-ad, and offsite metrics. | Keep |
| 4 | Acquisition channel gap | `acquisition channel-gap` | Finds listings strong in Etsy Ads but weak in Offsite Ads, or vice versa, using normalized efficiency signals. | Keep |
| 5 | Research-quota allocation | `quota allocate` | Prioritizes scarce keyword searches for listings with high marketing exposure but stale or missing demand evidence. | Keep |
| 6 | Visibility-performance gap | `listing visibility-gap` | Compares observed keyword rank/demand with onsite and offsite performance for explicit listing-keyword mappings. | Keep |
| 7 | Cross-surface anomalies | `growth anomalies` | Detects unusually large weekly changes and coincident movements across all four synchronized surfaces. | Keep |
| 8 | Weekly scorecard | `growth weekly` | Prints totals from all four surfaces in one report. | Kill: concatenation without analytical leverage |
| 9 | Keyword revenue attribution | `keyword revenue <term>` | Assigns onsite-ad revenue to Marketplace Insights keywords. | Kill: ad search-term contract unavailable |
| 10 | Promotion fatigue | `promotion fatigue <listing-id>` | Detects diminishing response across repeated promotions. | Kill: requires substantial repeated history |
| 11 | Inventory buy recommendation | `inventory buy-plan` | Recommends purchase quantities from demand and marketing performance. | Kill: inventory, cost, and lead-time data absent |
| 12 | Spend leakage report | `ads leakage` | Lists advertised listings with spend but no attributed orders. | Kill: thin single-surface filter |
| 13 | Automatic growth optimizer | `growth optimize --apply` | Changes ad, promotion, or offsite settings from local rules. | Kill: excluded mutations |
| 14 | AI growth explainer | `growth explain` | Generates a narrative explanation for weekly movements. | Kill: LLM dependency; deterministic output preferred |

## Survivors

| # | Name | Command | Mechanism and Evidence | Score | Buildability | Required Data | Acceptance Criteria |
|---:|---|---|---|---:|---|---|---|
| 1 | Listing action queue | `listing action-queue` | Drain-first SQLite joins listing IDs across keyword demand/rank, Etsy Ads, Offsite Ads, and promotions; emits deterministic action codes with timestamps and reasons. | 10/10 | Hand-code, local | Keyword metrics and rank observations; Ads and Offsite listing stats; promotion definitions/performance | Fixtures produce stable `research`, `review-ads`, `review-promotion`, and `hold` codes; missing surfaces are reported rather than zero-filled; no network call or mutation |
| 2 | Economics reconciliation | `economics reconcile` | Joins channel-specific financial observations while preserving attribution boundaries and refusing to invent profit. | 10/10 | Hand-code, local | Ads spend/revenue; Offsite fees and direct/indirect revenue; promotion performance | Exact source subtotals; overlapping revenues never summed into profit; exclusions such as COGS, shipping, and base Etsy fees labeled; zero and missing remain distinct |
| 3 | Observed promotion lift | `promotion observed-lift <promotion-id>` | Aligns promotion dates with equal-length prior windows in Ads and Offsite snapshots; labels results as observed change, not causation. | 9/10 | Hand-code, local | Promotion definitions/listing sets; Ads and Offsite listing/date snapshots | Baseline equals promotion duration; exact deltas; typed insufficient-history result; stale resources emit sync hints |
| 4 | Acquisition channel gap | `acquisition channel-gap` | Normalizes onsite and offsite listing measures into separately labeled efficiency signals and exposes channel asymmetry. | 9/10 | Hand-code, local | Ads listing stats; Offsite listing/channel/economics data; comparable windows | Classifies `onsite-strong`, `offsite-strong`, `balanced`, and `insufficient-data`; handles zero denominators; exposes component metrics and date coverage |
| 5 | Research-quota allocation | `quota allocate` | Joins Marketplace Insights quota/cache state with listing marketing exposure to rank where fresh demand evidence would reduce decision uncertainty. | 10/10 | Hand-code, local | Quota and keyword cache; Ads, Offsite, and promotion listing observations | Never enqueues a keyword search; fresh cache entries are not recommended; visible deterministic score; remaining quota caps recommendation count |
| 6 | Visibility-performance gap | `listing visibility-gap` | Joins explicit or observed listing-keyword rank/demand mappings with onsite and offsite listing performance without fabricating keyword revenue. | 8/10 | Hand-code, local | Keyword metrics, listing preview/rank history, explicit mappings, Ads and Offsite listing stats | No title-token inference; identifies high-demand/low-visibility and high-visibility/weak-paid cases; reports unmapped/stale observations |
| 7 | Cross-surface anomalies | `growth anomalies` | Compares historical snapshots across all four sources with deterministic median/deviation rules and emits source-linked exceptions. | 8/10 | Hand-code, local | Historical snapshots for all four modules | Minimum-history rule documented; stable direction/magnitude/source rows; constant series not anomalous; coincident movements never labeled causal; works with one absent surface |

## Scope Redirects

- `listing action-queue`: listing-level review priorities. Use `economics reconcile` for channel-cost reconciliation.
- `economics reconcile`: attributed financial observations. Use `listing action-queue` for operational priorities.
- `promotion observed-lift`: one promotion’s during-versus-baseline observation. Use `growth anomalies` for portfolio-wide outliers.
- `listing visibility-gap`: explicit search-visibility and paid-performance mappings. Use `listing action-queue` for the broader four-surface queue.
- `growth anomalies`: recurring portfolio exceptions. Use `promotion observed-lift` for one promotion window.

## Rejected Candidates

| Candidate | Rejection Reason | Closest Survivor |
|---|---|---|
| Weekly scorecard | Concatenates totals without a decision or analytical transformation | Cross-surface anomalies |
| Keyword revenue attribution | Missing Ads search-term contract makes term-level revenue attribution fabricated | Visibility-performance gap |
| Promotion fatigue | Requires substantial repeated history and is less verifiable than direct window comparison | Observed promotion lift |
| Inventory buy recommendation | Inventory, COGS, lead time, and fulfillment risk are absent | Listing action queue |
| Spend leakage report | Thin filter over Ads listing metrics and ignores Offsite acquisition | Acquisition channel gap |
| Automatic growth optimizer | Requires excluded mutations and unproven replay/idempotency safeguards | Listing action queue |
| AI growth explainer | Requires LLM interpretation; deterministic anomaly evidence is more agent-friendly | Cross-surface anomalies |
