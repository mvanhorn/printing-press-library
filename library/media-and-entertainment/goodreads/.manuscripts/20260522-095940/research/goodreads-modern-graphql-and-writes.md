# Goodreads Modern GraphQL + Write-Payload Capture

Captured: 2026-05-28 via Chrome DevTools Protocol (live, authenticated session).
Supersedes the "Not Yet Covered" gaps in `api-map-status.md` (rating write, review write/edit,
spoiler toggle, search). Closes the completeness gap that caused PR #894 to be deferred.

## Architecture: HYBRID (key finding)

The original map treated Goodreads as a legacy Rails app. The modern book/shelf/rating UI is a
**Next.js + Apollo client talking to an AWS AppSync GraphQL API**, while review bodies still post to
**legacy Rails form endpoints**. Both surfaces are live simultaneously; a complete CLI must speak both.

- **GraphQL endpoint:** `https://kxbwmqov6jgg3daaamb744ycu4.appsync-api.us-east-1.amazonaws.com/graphql` (POST)
  - AppSync host (us-east-1). Auth rides on the Apollo client's bound headers (captured at protocol level via CDP;
    a page-context `fetch` is blocked by CORS, and `window.fetch` interception fails because Apollo binds fetch at init).
- **Legacy host:** `https://www.goodreads.com/...` (Rails form POSTs, Rails CSRF `authenticity_token`).

## GraphQL operations (captured live, full bodies in run proofs)

| Op | Type | Variables | Notes |
|----|------|-----------|-------|
| `RateBook` | mutation | `$input: RateBookInput!` → `input.rating` (int 1-5) | **PROVEN**: rating=4 fired and persisted |
| `UnrateBook` | mutation | (book identity in input) | **PROVEN**: clears an existing rating |
| `getReviews` | query | book id | book-page reviews list |
| `getSimilarBooks` | query | book id | recommendations rail |
| `myReviewCard` | query | `$id` | user's own review/rating card (refetched after RateBook) |
| `GetAdsTargeting` | query | — | ads targeting (omit from CLI) |

Rating contract: `RateBook` with `input.rating` to set 1-5 stars; `UnrateBook` to clear. The legacy
`/review/update` form does NOT carry the star rating — rating is GraphQL-only on the modern UI.

## Legacy review-write + publicize form (captured from `/review/edit/:book_id` DOM)

**`POST /review/update/:book_id`** (Rails form, `application/x-www-form-urlencoded`). Fields:

| Field | Meaning |
|-------|---------|
| `authenticity_token` | Rails CSRF (extract from the edit page per-request) |
| `review[review]` | the review body text |
| `review[spoiler_flag]` | spoiler toggle (checkbox) |
| `review[notes]` | private notes (not the public review) |
| `add_update` | **publicize: share this review/update to your feed** ("publicize my comments") |
| `add_to_blog` | cross-post to external blog |
| `shelfChooser` | shelf to file the book under |
| `readingSessionDatePicker…[start|end][year|month|day]` | reading dates |
| `review[sell_flag]` | sell-copy flag |
| `next` / `source` | submit + provenance |

This maps the full "write a review + set spoiler + publicize" flow. No public review was posted during
capture — the field set was read structurally from the live editor form.

## Search (was "Not Yet Covered")
- `GET /search?q=<q>&search_type=books` — confirmed live (returns the legacy results page with `/book/show/:id` links).

## CLI implications
- Rating commands must hit AppSync GraphQL (`RateBook`/`UnrateBook`), not a legacy route.
- Review write/publicize/spoiler must POST the legacy `/review/update/:book_id` form with the fields above.
- The earlier batch-form-key question (`messages[<id>]` value) is legacy-form array-index style, consistent with this form's `review[...]` bracket-key convention.
- Auth: GraphQL needs the AppSync auth header (capture via CDP); legacy needs the session cookie + CSRF token.

## Reproduction
CDP: `session.use(<book target>)`, `Network.enable`, subscribe `Network.requestWillBeSent`, then click the
rating star / open `/review/edit/:id`. Full captured GraphQL bodies archived at `/tmp/gr-mutations.json`
(move into run proofs before publish).
