# Tokyo Art Beat CLI build brief

## User Vision
Focused single-source Japan exhibition, gallery and museum discovery for itinerary dates. Public read-only access. Preserve Japanese and English names, event editions, venue identity, source URLs, missing values and JST. No ticket-availability inference, account writes, paid membership automation or global installation.

## API Identity and Access
Tokyo Art Beat covers Japan nationwide: 65 local areas in the live catalog, including Tokyo neighborhoods and prefectures. The website uses a published Contentful-compatible JSON CDN at `https://cdn.prod.tabdev.net/api/cda/published/spaces/j05yk38inose/environments/master/entries`. Anonymous GET without access_token follows a 307 redirect to a cached JSON response and returns 200. No credentials are needed for the approved scope. This is an undocumented website surface, not an official public API. The content_types route returned 401 and is excluded. Anonymous browser capture verified event listings; full responses reverified by direct HTTP. No wrapper/CLI/MCP found in targeted GitHub/npm/PyPI searches; no wrapper issue tracker to inspect.

Membership boundaries: https://www.tokyoartbeat.com/en/aboutSubscription and the homepage describe MuPon redemption, personal lists/bookmarks/reviews and maps as premium benefits. This CLI reads public names, coordinates and coupon indicators; it neither redeems coupons nor imitates paid account features. Proximity is computed from public venue coordinates and never described as walking time. Official/ticket links are only carried when the source supplies them.

## Top Workflows and Pain Points
1. Search a trip window by area, category, artist or venue; constrain an exact event edition by source ID.
2. Inspect event detail and separate venue location/hours/admission from event overrides, date span and closure notes.
3. Find starts, ends and overlapping date spans; avoid yearless date ambiguity and archived editions.
4. Bounded nearby shortlist based on straight-line distance with explicit candidate truncation.
5. Reuse fresh local response cache and select just the fields an agent needs.
Pain points: enormous nested website payloads; English omissions while Japanese has reservation detail; weekly closures do not determine exceptional/holiday opening; an event listing is not ticket inventory.

## Build Priorities and Table Stakes
Public bilingual event/venue search and detail; finite ID catalogs; inclusive dates with ISO years; source attribution/freshness; null unknowns; compact JSON, bounded pagination, explicit partial and truncation; lazy details; structured stdout errors with stderr diagnostics. Explainable chronological/distance sorting, no invented popularity score.

## Data Layer and Product Thesis
Use a bounded private file response cache, TTL 1 hour and explicit --fresh/--offline. No account state or bulk mirror. The CLI is a small Go process with no browser runtime. Catalogs resolve names to source IDs, and linked venues are fetched in one batch. Search lists concise cards; details fetch larger fields lazily. Compare cache misses/hits via request counts, latency, output bytes and peak RSS.

## Decisions
User preapproved focused read-only scope and routine gates. Sole builder instruction supersedes the Printing Press novel-feature brainstorming subagent; exactly one fresh-context reviewer is reserved for code review. Global skill updater is skipped under the no-global-change instruction. No publication, PR or service purchase.

## Sources
- https://www.tokyoartbeat.com/en and https://www.tokyoartbeat.com/
- https://www.tokyoartbeat.com/en/events/filter/open
- https://www.tokyoartbeat.com/en/venues
- https://www.tokyoartbeat.com/en/aboutSubscription
- Public browser scripts and sanitized HAR under discovery; bounded JSON response samples under research.
