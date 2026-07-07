# Browser-Sniff Discovery Report: KDP Niche Finder

## Source
- Target: https://kdpnichefinder.com/dashboard (Laravel + Inertia.js SPA; brand front for an Artistly-built app)
- Method: manual DevTools HAR export (273 entries), processed by `cli-printing-press browser-sniff`
- Reachability: **standard_http** (confidence 0.65; no browser-only signals) — printed CLI ships plain HTTP

## Auth model
- **Session-cookie (Laravel)**. Unauthenticated `/dashboard` 302→ login. Authenticated requests carry HttpOnly session cookies (stripped from HAR) + `X-XSRF-TOKEN` header (encrypted Laravel CSRF value, = `XSRF-TOKEN` cookie).
- GET endpoints: session cookie only (Laravel exempts GET/HEAD/OPTIONS from CSRF). Inertia GETs also send `X-Inertia: true` + `X-Inertia-Version`.
- POST endpoints: session cookie **and** `X-XSRF-TOKEN` header.
- CLI auth plan: `auth login --chrome` captures the full kdpnichefinder.com cookie jar; compose `X-XSRF-TOKEN` from the `XSRF-TOKEN` cookie for writes. Session cookie name TBD at live capture (Laravel default family `*_session`).

## Endpoints discovered (replayable)
| Method | Path | Returns | Auth | Notes |
|--------|------|---------|------|-------|
| GET | /app/category/{type} | Inertia JSON | session | type ∈ {evergreen, fresh_money, hidden_gems, high_ticket}; `?search=<term>`, `?page=N`; books at `props.books.data[]`, Laravel-paginated |
| GET | /api/categories | JSON array | session | category metadata: {key, name, description} |
| GET | /api/folders | JSON array | session | user folders: {id, name} |
| POST | /api/folders | JSON | session+CSRF | body {name} → creates folder |
| GET | /api/user | JSON object | session | {name, email, image_url} |
| POST | /api/books/{book_id}/toggle-save | JSON {saved} | session+CSRF | body {folder_id: int|null} |
| GET | /app/saved-books | Inertia/HTML | session | saved books view |

## Core entity: Book (a "niche")
Fields observed in `props.books.data[]`:
- id (int), title (string, keyword-rich), amazon_url (string → ASIN at /dp/XXXX)
- image_url (string), price (string), publisher (string)
- **estimated_monthly_sales (int)**, **estimated_monthly_revenue (float)** ← demand signal

## Pagination
Laravel paginator on category lists: `current_page, data, last_page, per_page, total, next_page_url, prev_page_url, links`. Page-based via `?page=N`.

## Third-party / out of scope
- `api.everbee.com/prompts` (20 calls): Everbee powers the underlying data; `/prompts` is Artistly AI-prompt suggestions, NOT niche data. Excluded from CLI scope (different auth `x-access-token`, third-party host).
- `images-na.ssl-images-amazon.com` (72): Amazon cover images (referenced via `image_url`, not called by CLI).

## Replayability verdict
PASS — clean REST + Inertia JSON routes, session-cookie replay, standard HTTP. No resident browser needed at runtime.
