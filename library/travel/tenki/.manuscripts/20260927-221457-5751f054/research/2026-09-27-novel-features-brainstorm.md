## Customer model

Root completed this audit after the user explicitly corrected topology: root is the sole Astra orchestrator/planner; redundant Astra children were interrupted. Existing architecture findings retained. This explicit instruction supersedes the generic skill's child-planner requirement.

**Itinerary agent.** Today: opens municipal daily and hourly pages for several Japan day trips. Ritual: repeatedly revise outing dates during a trip. Frustration: different horizons, rain periods and missing values make ad hoc comparisons unreliable.

**Seasonal-outing agent.** Today: checks seasonal spot status then its linked municipality weather, including mountain destinations. Ritual: repeatedly shortlist seasonal outings while travelling. Frustration: old season text, municipal weather and altitude guidance can look deceptively current or site-specific.

## Candidates (pre-cut)

| Candidate | Source | Decision | Long Description |
|---|---|---|---|
| Date/destination criteria matrix | User brief/persona | Keep | Compare bounded places and dates using explicit criteria; use forecast hourly for individual source intervals. |
| Daytime window evaluation | Hourly product semantics | Keep | Evaluate a specified activity window; daily outlook remains a whole-day product. |
| Seasonal weather pairing | Seasonal content pattern | Keep | Pair a current seasonal spot report with separately dated municipal weather; use seasonal show for report details. |
| Evidence coverage gate | User correctness brief | Keep | Explain unavailable criteria and source-level mismatches rather than selecting a false winner. |
| Universal sightseeing score | Persona | Kill: unexplained weighting and missing evidence manufacture precision | none |
| Summit ascent safety rating | Mountain content | Kill: foothill/model values cannot establish summit or route safety | none |
| Automatic 2027 blossom forecast | Seasonal content | Kill: source product unavailable, would manufacture a forecast | none |
| Nationwide background monitor | Local queries | Kill: unbounded source load and unnecessary daemon | none |
| Cross-provider consensus | Persona | Kill: user permits tenki.jp only | none |
| Trip route optimizer | Persona | Kill: transport/routing data outside source scope | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Criteria matrix | compare | 10/10 (3+3+2+2) | hand-code | Uses bounded municipal daily/hourly responses to evaluate explicit thresholds per destination/date, without external services. | User brief and official forecast guide | Itinerary agent | Compare selected places/dates; inspect individual source periods with forecast hourly. |
| 2 | Activity-window evaluation | compare --hours | 9/10 (3+3+2+1) | hand-code | Uses complete preceding-hour rain intervals and instantaneous wind/temperature to evaluate a requested JST window. | Official hourly semantics and requested outdoor planning | Itinerary agent | Hourly criteria apply only where complete source coverage exists. |
| 3 | Seasonal weather pairing | compare --season | 9/10 (3+3+1+2) | hand-code | Uses a seasonal spot report and its explicitly linked municipality forecast, retaining independent source years/times. | Current foliage detail and ended Sakura with fresh weather | Seasonal-outing agent | Pair seasonal evidence with weather; seasonal show exposes source seasonal detail. |
| 4 | Evidence coverage gate | compare | 10/10 (3+3+2+2) | hand-code | Checks date coverage, missing values, freshness and forecast reference before emitting meets/fails/insufficient-data results. | Official town-hall/foothill FAQ and user correctness requirements | Both | Unknown or stale evidence never becomes a positive suitability result. |

All four serve the repeated planning ritual; they add normalization and cross-product evidence rather than rename an endpoint. They share one command and deterministic comparison engine. These are four behaviors, not four separate services or extra management layers. Closest killed siblings: universal score (1,4), route optimizer (2), automatic future blossom forecast (3). All buildability tags are hand-code. No generator-only custom behaviors.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Universal sightseeing score | Unsupported weight/precision assumptions | Criteria matrix |
| Summit ascent safety rating | Wrong reference/forecast level | Evidence coverage gate |
| Future-year blossom invention | Unavailable source product | Seasonal weather pairing |
| Background nationwide monitor | Unbounded load/scope | Bounded matrix |
| Provider consensus | Violates tenki.jp-only requirement | Evidence coverage gate |
| Route optimizer | Missing route/travel-time source | Activity-window evaluation |
