# Minimal Walkerplus acceptance fixtures

These templates retain only the public HTML structures needed to exercise the real client and parser. They contain no advertisements, images, tracking, or full copied pages.

- `card.tmpl` and `listing.tmpl` preserve the list/card classes, exact-title and venue JSON-LD join, area/category link shapes, native year text, and pager from `event-list.html` and `kyoto-oct-festival.html` in this run's discovery captures. Kyoto's festival cards use the leaf tag `eg0135` and nonpadded `eg126`; the bread festival displays `2026年10月10日(土)・11日(日)` and an admission-free card fact.
- `detail.tmpl` preserves Event JSON-LD, the header/menu, and `tr.m-infotable__row` rows from `detail-data.html`, `detail-price.html`, `firework-data.html`, and `indoor-data.html`. The captured Tokyo price is paid with a children-free caveat and `offers.price: "None"`; reservation is explicitly required. The fireworks weather row is `雨天決行(強風中止)`, not an announcement of current cancellation. Osaka's schedule says `雨天決行（屋内会場）`.
- Test cases mutate dates and source facts deliberately to isolate recurrence, exclusions, edition boundaries, ambiguous joins, and safety against false-positive free/indoor/cancellation classification. These are deterministic synthetic events, not claims about live listings.

The injected transport uses `httptest.ResponseRecorder` and records every request. It never contacts the network. Expectations exercise exported Search, Shortlist, Event, and NormalizeQuery behavior; no test calls internal parsing or matching helpers.

`tokyo-city-catalog.tmpl` preserves city navigation anchor shape and the live-verified Shinjuku path `/event_list/ar0313104/shinjuku/`. It includes a foreign-prefecture city as a deliberate negative fact. The catalog has explicit zero event cards; its purpose is route resolution, with positive events supplied independently on the resolved monthly city route.
