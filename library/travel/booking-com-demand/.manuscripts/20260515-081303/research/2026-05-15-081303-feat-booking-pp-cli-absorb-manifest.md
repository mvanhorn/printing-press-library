# Booking.com CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search destinations | hotels_mcp_server | accommodations search | Offline FTS, date/price filters, SQLite cache |
| 2 | Get hotels by destination | hotels_mcp_server | accommodations search --destination | Price history tracking, bulk compare |
| 3 | Hotel details | azaelcodes/bookingcomclient | accommodations details | Full property data with offline access |
| 4 | Hotel descriptions | rybakit/bookingcom-client | accommodations details --descriptions | Multi-language, offline searchable |
| 5 | Hotel facilities | rybakit/bookingcom-client | accommodations details --facilities | Filterable, offline lookup |
| 6 | Changed hotels tracking | rybakit/bookingcom-client | accommodations changes | Local diff, change notifications |
| 7 | Hotel reviews | Demand API | reviews list | FTS on review text |
| 8 | Review scores | Demand API | reviews scores | Score breakdown with historical tracking |
| 9 | Check availability | Demand API | availability check | Price breakdown, charge details |
| 10 | Bulk availability | Demand API | availability bulk | Compare up to 50 properties at once |
| 11 | Accommodation chains | Demand API | chains list | Offline reference, brand filtering |
| 12 | Constants/reference data | Demand API | constants list | Offline lookup for facilities, room types, bed types |
| 13 | Car rental search | Demand API | cars search | Date/location filters, price compare |
| 14 | Car depots | Demand API | cars depots | Location-based depot finder |
| 15 | Car depot reviews | Demand API | cars depots reviews | Score breakdown |
| 16 | Car details | Demand API | cars details | Specs, capacity, brand info |
| 17 | Car suppliers | Demand API | cars suppliers | Supplier directory |
| 18 | Car constants | Demand API | cars constants | Reference data |
| 19 | Location airports | Demand API | locations airports | Offline airport code lookup |
| 20 | Location cities | Demand API | locations cities | Offline city reference |
| 21 | Location countries | Demand API | locations countries | Country/region data |
| 22 | Location districts | Demand API | locations districts | Neighborhood-level data |
| 23 | Location landmarks | Demand API | locations landmarks | POI reference |
| 24 | Location regions | Demand API | locations regions | Regional grouping |
| 25 | Payment cards | Demand API | payments cards | Supported card types |
| 26 | Payment currencies | Demand API | payments currencies | Currency reference |
| 27 | Languages | Demand API | languages list | Supported languages |
| 28 | Order preview | Demand API | orders preview | Final pricing before commit |
| 29 | Order create | Demand API | orders create | Booking creation with --dry-run |
| 30 | Order details | Demand API | orders details | Order lookup with filters |
| 31 | Order accommodation details | Demand API | orders details --type accommodation | Full accommodation booking info |
| 32 | Order car details | Demand API | orders details --type car | Car booking info |
| 33 | Order flight details | Demand API | orders details --type flight | Flight booking info |
| 34 | Order modify | Demand API | orders modify | Date/card/room changes |
| 35 | Order cancel | Demand API | orders cancel --dry-run | Cancellation with preview |
| 36 | Send message | Demand API | messages send | Text + attachment support |
| 37 | Latest messages | Demand API | messages latest | Conversation history |
| 38 | Confirm messages | Demand API | messages confirm | Receipt confirmation |
| 39 | Conversations | Demand API | messages conversations | Thread view |
| 40 | Upload attachment | Demand API | messages attachments upload | File sharing |
| 41 | Download attachment | Demand API | messages attachments download | File retrieval |
| 42 | Attachment metadata | Demand API | messages attachments metadata | File info |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Price drift detector | accommodations drift --since 7d --threshold 10% | 8/10 | Joins current availability/bulk API response against local SQLite availability_history to compute per-property price deltas | Kenji persona: weekly bulk-availability checks are a 45-minute manual spreadsheet ordeal |
| 2 | Review category ranker | reviews rank --destination "Barcelona" --min-cleanliness 8 --sort price | 9/10 | Joins local review_scores + accommodations + availability to filter by per-category thresholds and sort by price | Mariana persona: multi-category review filtering is impossible in the web UI |
| 3 | Upcoming orders with messages | orders upcoming --days 14 --with-messages | 8/10 | Joins local orders (filtered by check-in date) with messages_latest (unresolved threads) | Diego persona: no single view of orders + message status, requires manual cross-referencing |
| 4 | Property change digest | accommodations changelog --since 7d --fields price,facilities | 8/10 | Calls accommodations/details/changes for changed IDs, diffs current details against local table | Kenji persona: change tracking + diff is a first-class API feature with no existing tooling |
| 5 | Facility comparison matrix | accommodations compare --ids 123,456,789 --show facilities,scores,price | 8/10 | Joins local accommodations + facilities + review_scores + availability into side-by-side matrix | Mariana persona: property comparison across facilities, scores, and pricing is manual |
| 6 | Destination intelligence | locations intel --city "Barcelona" | 7/10 | Aggregates 6 local tables (cities, districts, landmarks, accommodations, review_scores, availability) | Mariana persona: destination-level analytics require browsing dozens of pages |
