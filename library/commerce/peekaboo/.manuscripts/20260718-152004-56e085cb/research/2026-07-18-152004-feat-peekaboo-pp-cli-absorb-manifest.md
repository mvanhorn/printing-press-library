# Peekaboo Guru CLI — Absorb Manifest

## Ecosystem scan
No CLI, SDK, MCP server, or community wrapper exists for Peekaboo Guru. "Peekaboo Connect"
is a contact-only B2B data service (no public docs). The only reference surface is the
Peekaboo web app + Android/iOS app themselves. Absorb target = full feature parity with
the site/app, beaten with offline SQLite, --json/--agent output, and coordinate export.

## Absorbed (match every capability the Peekaboo site/app offers)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse cities/locations | Peekaboo city picker | (generated endpoint) locations list | 982 cities offline, sortable by entityCount, coords included |
| 2 | Browse categories | Peekaboo category nav | (generated endpoint) categories list | 13 categories w/ dealCount, offline |
| 3 | Browse merchants by city+category | Peekaboo places page | (generated endpoint) places list | --json, pagination, nearestBranch coords |
| 4 | Merchant detail | Peekaboo detail page | (generated endpoint) places detail | full record: social, menu, gallery, rating, stats |
| 5 | List branches w/ coordinates | Peekaboo branches page + Direction CTA | (generated endpoint) branches list | lat/long per branch, timings, address; offline export |
| 6 | List deals/offers for a merchant | Peekaboo card-offers page | (generated endpoint) deals list | title, %, validity window, terms; --json |
| 7 | Card sources per merchant (bank deals) | Peekaboo "Cards & Offers" | (generated endpoint) cards list | which banks give deals here, dealCount/maxDiscount |
| 8 | Browse brands / card-deal sources | Peekaboo brands nav | (generated endpoint) brands list | brand catalog w/ categories, offline |
| 9 | Merchant amenities | Peekaboo amenities widget | (generated endpoint) amenities list | dine-in/delivery/etc. flags |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Branch directions URLs | directions | hand-code | Composes the site's Google Maps daddr URL from local branch coords for ALL branches at once (the site makes you click each Direction button) | Use this to list/export a Maps directions URL for every branch of a merchant. Do NOT use it to pick only the single closest branch; use 'nearest'. |
| 2 | Nearest branch + directions | nearest | hand-code | Haversine over branches x local city->coord index; no single API call ranks branches by distance | Use this to find the single closest branch of a merchant to a city or coordinates, with its directions URL. Do NOT use it to list all branches; use 'directions'. |
| 3 | Card -> merchants reverse index | wallet | hand-code | Inverts merchant->cards into card->merchants via a local join across deals+brands+entities; impossible on the site | Use this to list merchants honoring a specific bank/card's deal (card -> merchants). Do NOT use it to rank a city's deals regardless of card; use 'top-deals'. |
| 4 | Cross-merchant deal ranking | top-deals | hand-code | Ranks a whole city's deals by percentageValue in local SQLite; no cross-merchant aggregation endpoint exists | Use this to rank a city's merchants by discount percentage. Do NOT use it to filter by a specific bank card; use 'wallet'. |
| 5 | Deals expiring soon | expiring | hand-code | Cross-merchant endDate window filter over the local deal mirror | Use this to list deals whose validity window ends within N days. Do NOT use it to filter by open-now; use 'open-now'. |
| 6 | Open-right-now filter | open-now | hand-code | Compares synced branch timings against the current local time across merchants | Use this to list merchants open at the current time based on branch timings. Do NOT use it to find deals expiring soon; use 'expiring'. |

## Hand-code commitment
6 transcendence features, all `hand-code` (~50-150 LoC each + root.go wiring). 0 spec-emits. 0 stubs.
