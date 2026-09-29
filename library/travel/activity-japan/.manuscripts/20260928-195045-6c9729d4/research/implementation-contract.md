# Activity Japan implementation contract (pending source access)

## CLI surface

- `experience search`: bounded destination/category/date/party/budget/preferences shortlist; return only source-supported filters and echo requested vs returned conditions.
- `experience detail <plan-id>`: one plan, operator identity, venue/meeting/pickup separately, eligibility, inclusions, equipment, restrictions, cancellation, language evidence and canonical URLs.
- `experience sessions <plan-id> --date YYYY-MM-DD --adults N ...`: observed sessions/option prices only when date/party source data is available; never equate operating period with seat availability.
- `experience compare <ids...>`: at most five details; explicit constraint verdicts `match`, `mismatch`, `unknown`, and item-level errors. No automatic cross-currency or inferred total.
- `experience handoff <plan-id>`: canonical source URL with locale, no booking mutation.
- `inventory coverage`: language-specific sitemap counts and ID presence, clearly labelled as index coverage.

## Output contract

Compact JSON objects carry response-level `source=activity-japan`, `site_language`, `observed_at` (RFC3339 Asia/Tokyo), `cache` (`hit`, age, TTL), `coverage` (source paths/pages scanned, partial/truncated), `pagination`, `data`, and `errors`. Stable IDs are source strings. Missing consequential fields are `null` plus a `missing` list where useful. A source fact and a derived interpretation are separate fields; each quote and availability observation has its own timestamp.

Each plan retains `plan_id`, `operator_id` if exposed, `operator_name_original`, `plan_name_original`, localized name if supplied, source URL and canonical URL. Options and sessions have their own source IDs when present; never synthesize IDs without tagging them as derived keys. Price objects separate headline-from amount, selected quote, currency, basis (`per_person`, `per_group`, `unknown`), age band, mandatory fees, optional extras and tax inclusion. Availability enum: `instant_confirmable`, `request`, `waitlist`, `sold_out`, `unchecked`, with unknown for unrecognized source wording. A session is an observation, not a reservation.

Duration carries total experience vs activity minutes; schedule carries session start, meeting time and check-in lead separately. Guide languages carry source evidence independent of website locale. Locations carry separate venue, meeting and pickup records. Date and time parsing uses Asia/Tokyo; timezone-less source text remains source text until context establishes an instant.

## Resource limits

Default search 5 items, max 20; at most 3 upstream pages and 5 compare items. Session scan at most 7 explicit dates, max two concurrent reads, 10-second per-request timeout, 25-second command deadline, 1 bounded retry for 429/5xx with context cancellation, response body cap 2 MiB, bounded disk cache with explicit refresh and maximum age. Source denial is a typed error; item-level failures remain in `errors`. Dry runs stop before network/cache writes. stdout is one JSON object; diagnostics on stderr.

## Verification gate

Before claiming a behavior, run source-backed live tests across at least Kyoto culture, Osaka food and Okinawa outdoors, plus search-to-detail identity, plan names, source headline price vs exact quote, date/party echoes, availability states and one explicit failure response. Use deterministic tests for pricing/age bands, language interpretation, timezone and partial failures. Measure bytes, upstream requests, elapsed time and peak RSS for cached and uncached representative commands. Fixture-only cases are labelled separately. If WAF/partner credentials remain unavailable, none of these live cases can be reported as passed.
