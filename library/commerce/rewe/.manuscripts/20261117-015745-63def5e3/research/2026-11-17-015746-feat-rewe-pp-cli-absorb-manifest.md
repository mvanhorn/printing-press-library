# REWE CLI Absorb Manifest (approved by the user 2026-11-17)

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Market search by zip / text | korb, rewerse (app API) | markets pickup/services/search | no APK certificate |
| 2 | Select market + service | korb | markets select / current | scriptable, dry-run |
| 3 | Product search with price | korb, rewerse | products search | listing ids for basket, JSON |
| 4 | Basket add/set/remove | korb | basket set/remove/add-many | versioned basket returned |
| 5 | Pickup timeslots | korb | slots list/overview/reserve | fee + earliest |
| 6 | Payment options | korb | payment options | MARKET_PAYMENT detection |
| 7 | Favourites | korb | favorites list/add/remove | |
| 8 | Bought before | korb | purchased list | |
| 9 | Recipes search/detail | rewerse | recipes search/tiles, recipe get | schema.org ingredients |

## Transcendence
| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|-------------------------|
| 1 | Self-renewing session | auth renew | Keycloak SSO cookie replay over plain HTTP; others need APK cert or manual token copy |
| 2 | Structured market offers | offers | parses the offers page into rows and stores them per ISO week |
| 3 | Deals on my products | deals mine | joins offers with purchased products and favourites |
| 4 | Earliest pickup slot | slots earliest | picks earliest free slot, optional reserve |
| 5 | Full checkout with gate | order place | basket refresh, slot, pay-in-market, confirmation submit, --confirm gate |
| 6 | Recipe to basket | recipe basket | REWE ingredient mapper + add recommended products |
| 7 | Basket by text | basket add | resolves basket id and best product match |
| 8 | Shopping list to basket | list add | one line per item -> best match -> basket |
| 9 | Price history | price history | local snapshots of product and offer prices |
