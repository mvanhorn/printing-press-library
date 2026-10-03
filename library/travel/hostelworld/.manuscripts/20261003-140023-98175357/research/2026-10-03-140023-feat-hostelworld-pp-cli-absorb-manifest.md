# Hostelworld absorb manifest

User-approved scope comes from travel-styles-build-brief.md and Hostelworld preflight entry. Routine feature choices and website discovery were preauthorized. The supplied travel goals replace a broad speculative brainstorm; direct builder plus exactly one fresh reviewer is the delegated team contract. No stubs and no write/account/chat/booking features.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Resolve cities and properties | Native autocomplete; trvl | hostelworld-pp-cli destinations search | Preserve ambiguity and source IDs |
| 2 | Dated city search | Native Hostelworld; Hostelworld-Finder | hostelworld-pp-cli hostels search | Bounded page and currency; from-price labels |
| 3 | Property rules/facilities | Native property listing | hostelworld-pp-cli hostels inspect | Source rules with timestamp; no atmosphere guarantees |
| 4 | Dated dorm and private plans | Native room plans | hostelworld-pp-cli hostels offers | Exact source units, inventory and terms |
| 5 | Structured reusable source surface | Observed HTTP capture | (generated endpoint) source locations | Four typed read-only contracts and dry run |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description | Score |
|---|---|---|---|---|---|---|
| 1 | Comparable party costs | hostels compare | hand-code | Join actual room plans and quantity/unit rules; keep currencies separate | Compare up to five property IDs for the same stay and party. Dorm source amounts are per bed; private source totals are per room. Derived party amounts are estimates. | 10 |
| 2 | Flexible stay windows | hostels dates | hand-code | Re-request each explicit alternative; never reuse one dated quote | Check up to five explicit check-in dates at one property with the same nights and guests. | 9 |
| 3 | Nightly price basis | hostels offers | hand-code | Verify source nightly sum against whole-room/bed totals and expose ambiguous basis | Show room plans and per-night source amounts; preserve occupancy-slot versus room amounts. | 10 |
| 4 | Payment and cancellation evidence | hostels offers | hand-code | Keep deposit, deadline/timezone, rate type and contradictory source descriptions together | Show terms; free-cancellation filtering requires source availability and an unexpired deadline. | 9 |
| 5 | Saved planning evidence | hostels saved | hand-code | Persist a bounded snapshot for offline re-reading with explicit stale status | Read saved property/offer planning snapshots. Prices are stale until refreshed live. | 8 |

## Deliberate limits
Source search ranking includes promotions/commissions; we preserve source order and do not claim best hostels. Mobile-only deals, scraping full reviews, personal profiles, AI property chat and cross-provider booking workflows are outside the explicit single-source read-only task. SEO discovery alone is not advertised as a dated quote. API application config is fetched transiently from its public first-party literal; schema drift fails clearly.
