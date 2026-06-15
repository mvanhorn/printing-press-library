# Squire CLI Absorb Manifest

No existing Squire CLI/MCP/SDK exists (confirmed via search). "Absorbed" = the Squire discovery website's own capabilities, each emitted by the generator from the sniffed spec. Transcendence = cross-shop / over-time / local-SQLite commands the site never built.

## Absorbed (match the website; generated endpoint commands)
| # | Feature | Source | Our Implementation | Added Value |
|---|---------|--------|--------------------|-------------|
| 1 | Search shops/barbers by term | Squire web search | (generated endpoint) search list_public | offline cache, --json, --select |
| 2 | View shop profile | Squire web shop page | (generated endpoint) shop get_details | --select, offline, slug or uuid |
| 3 | List a shop's services + prices | Squire web shop page | (generated endpoint) shop get_service | --json, sortable |
| 4 | List barbers/staff | Squire web shop page | (generated endpoint) shop get_professional | offline, --json |
| 5 | A barber's services | Squire web barber page | (generated endpoint) shop get_service_2 | offline |
| 6 | Next available time for a barber | Squire web booking | (generated endpoint) shop get_next_available_time | scriptable |
| 7 | Reviews + AI summary + rating | Squire web reviews | (generated endpoint) reviews get_shop | limit/skip paginate, --json |
| 8 | Discover shops by city/location | Squire web city pages | (generated endpoint) discover list_shops | offline |
| 9 | City lookup | Squire web | (generated endpoint) discover list_city | — |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Soonest-available barber across shops | soonest --near <city\|shops> [--service "Haircut"] | hand-code | Sorts live nextAvailableTime ISO across N shops; site shows one shop at a time, no cross-shop availability view | none |
| 2 | Cross-shop compare (price/rating/staff) | compare <shopA> <shopB> [...] | hand-code | SQLite join across services/reviews/shops the API exposes only per-shop | Use to put 2+ named shops side by side on price, rating, and staff. Do NOT use for ranking an unknown set of shops in an area — use 'roster' (rating) or 'cheapest' (single-service price). |
| 3 | Cheapest service in an area | cheapest "<service>" --near <city\|shop> [--limit N] | hand-code | Local FTS on service name/category + sort on cost cents across shops; low end of costRange flagged | Use to find the lowest price for ONE service category across an area. Do NOT use to compare named shops — use 'compare'. Do NOT use when rating matters more than price — use 'roster'. |
| 4 | Watch a shop for price/staff drift | watch <shop> | hand-code | Stateful snapshot diff (cents-level price moves, barber add/remove, rating change) impossible on the stateless site | Use to detect what changed at ONE shop since last run. Do NOT use for point-in-time multi-shop comparison — use 'compare'. |
| 5 | City shop ranking by rating confidence | roster --near <city> [--min-reviews N] [--limit N] | hand-code | Ranks shops by rating × log(numberOfRatings); attaches Squire's AI summary verbatim (no LLM) | Use to rank the best shops in a city by rating quality and review volume. Do NOT use to compare named shops — use 'compare'. Do NOT use when price decides — use 'cheapest'. |

Internal helper (not a novel feature): `resolve <slug>` → shop UUID (foundational; every shop command resolves slug→UUID since /v2/shop/{id}/service requires the UUID).

Stubs: none. All 5 transcendence rows are shipping scope (hand-code).
