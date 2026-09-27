# Parser fixture provenance

These UTF-8 fixtures are reduced DOM excerpts from anonymous public Jalan pages
captured on 2026-09-27. They keep Japanese lodging/room/plan names and only the
fields exercised by parser tests. Scripts, advertising modals, personal reviews,
calendar listings, navigation and unrelated recommendations are removed.

- `search-pagination.html`: Hakone area 141600, 2026-11-10, one night,
  two adults, one room. It keeps the two source-sponsored cards and 30 organic
  cards in source order to protect native pagination.
- `offers.html`: property 385995, same date and party. It retains all 54
  room/plan tuples under 12 plans; the source's plan count is not an offer count.
  Coupon summaries and expected points remain separately scoped to their tuple.
- `property-385995.html` and `property-371898.html`: property facilities,
  room-wide declarations, amenity presence/absence and aggregate Japanese review
  categories. The latter explicitly says room outdoor baths are not hot spring
  baths while the property's shared baths are onsen.
- `plan.html`: property 385995, plan 03912759, room 0576806, 2026-11-10,
  one night, two adults, one room. Base quote 74,800 JPY; conditional coupon
  example 68,800 JPY; separate points and bathing tax.
- `two-night-plan.html`: same exact plan/room, two nights. Base whole-stay
  quote 149,600 JPY, with both nightly breakdown rows retained.
- `family-plan.html`: property 385995, plan 03806855, room 0546600,
  2026-11-10, one night, two rooms, each two adults and one elementary child.
  Whole-stay quote 124,740 JPY retains both adult and child room breakdowns.
- `newyear-plan.html`: same exact plan/room as `plan.html`, requested
  2026-12-31. HTTP 200 silently replaces the date with 日付未定 and an undated
  minimum 68,200 JPY. The source rejection is retained; this is not a dated quote.
- `no-offers.html`: property 385995, requested 2027-09-20. Explicit source
  statement says no matching plan or the property has stopped taking Jalan
  reservations. Tests preserve that ambiguity rather than reporting sold out.

Source routes: `https://www.jalan.net/140000/LRG_141600/`,
`https://www.jalan.net/yad385995/`, `https://www.jalan.net/yad371898/`,
`https://www.jalan.net/yad385995/plan/`, and the exact-plan route
`https://www.jalan.net/uw/uwp3200/uww3201init.do` with the IDs above.
