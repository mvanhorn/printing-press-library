# Data contract

## Identity and scope

Hotel IDs, plan IDs and room IDs are source strings. Room IDs may contain hyphens. An offer is that tuple plus dates and party; a name is never a substitute for an ID. Area IDs are public website paths, such as `tokyo/E`, and are not guessed official API district codes.

Dates are explicit calendar dates for a Japan stay. Occupancy is uniform per room: `rooms` multiplies the requested identical rooms, while adult and child categories describe each room. The client validates the source query echo before accepting inventory. Unequal allocations are unsupported.

## Prices, meals and policies

Offer amounts use integer JPY. A labelled whole-stay room total is **per room for the requested nights**. A per-person amount is preserved only if the source provides it. No nightly or all-rooms amount is fabricated by multiplication or division. Missing or zero values for unavailable variants do not mean a free room.

Consumption-tax inclusion, accommodation tax, other taxes and optional fees are separate states. A source label equivalent to “tax included” does not prove that every charge is included. Property notes can identify exclusions; retain the source wording and scope.

Breakfast and dinner are nullable booleans. Property cancellation policy is labelled property-level. Plan cancellation is exposed only when its association is reliable; otherwise it is unknown. Source caveats and final booking-stage conditions still apply.

Coordinates require a declared datum and units. Unverified values remain null; Tokyo Datum arcseconds must never be relabelled WGS84 degrees.

## Coverage and freshness

Search output is bounded. Source pagination units can be plans, while emitted rows are room-plan offers. `page` and `offset` distinguish the source page from position within its parsed rows. Follow the returned continuation and retain coverage metadata; a bounded-page minimum is not a claim of the globally cheapest offer.

Metadata cache defaults to 24 hours. Inventory uses a live request by default; explicit inventory caching is capped at 60 seconds. Source timestamps identify when data was fetched and observed. An invocation may reuse its last offer-page response while looking through offsets, preserving one consistent snapshot and avoiding duplicate requests; this is not persistent inventory caching.

Cache persistence is best-effort. A failed write emits a warning on stderr while preserving a successfully parsed source result. `--no-cache` skips persistence and its warnings; source fetch and parsing failures still return errors.

## Failure meanings

A verified empty source search is a successful `no_matches` or `no_availability` result. Authentication/challenge pages, HTTP failures, throttling, invalid requests, malformed HTML and mismatched query echoes are errors. A missing offer within a bounded scan does not prove that the tuple never exists.

Comparison preserves every requested cell and marks errors separately. Failed fetches are not zero-price offers and do not participate in minimum calculations. Freshness is per source result, including cache age when applicable.

The focused Travel commands do not support framework receipt files or audit directories. They reject `--receipt`, `--receipt-file` and `--audit-dir` with a usage error before fetching data. Their JSON output retains source and freshness metadata.

## Booking handoff

The returned booking link is the canonical dated source plan page with its observed room anchor. The hotel/plan/room tuple and query accompany it. The CLI does not turn Rakuten's reservation POST into an unverified GET and never submits a booking.
