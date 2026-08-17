# Ikon Pass — Discovery Findings (verified 2026-06-14)

Base: `https://account.ikonpass.com`  |  Backend: Ruby on Rails  |  WAF: Imperva Incapsula
Login: Auth0 SSO (`login.alterramtnco.com`, OAuth2 + PKCE) → mints `.ikonpass.com` session cookie.
Runtime auth model: **cookie session** (import `.ikonpass.com` cookies; no API key / OAuth-in-CLI).
Envelope: every endpoint returns `{ "data": …, "errors": [...] }`.

## Verified endpoints

| Endpoint | Method | Auth | Notes |
|---|---|---|---|
| `/api/v2/resorts` | GET | **none (public)** | 78 resorts; metadata + `reservations_enabled`. Powers `resorts` + name→id resolution. |
| `/api/v2/reservation-availability/{resortId}` | GET | cookie | Per-pass availability array (shape below). `{resortId}` = numeric `id`. |
| `/api/v2/me` | GET | cookie | Profile/identity — name, dob, email, linked logins, addresses. **PII.** Powers `whoami`. |
| `/api/v2/my-products` | GET | cookie | The account's passes/products (ids match availability array). Holder/type info → **PII**. Powers `passes`. |
| `/api/v2/my-products/{productId}/shared-vouchers-summary` | GET | cookie | Friends & Family shared-voucher summary (remaining/used). Powers `friends-family`. |
| `/api/v2/my-products/{productId}/benefits` | GET | cookie | Per-pass benefits. Powers `benefits`. |
| `/api/v2/my-products/{productId}/pass-usage` | GET | cookie | CONFIRMED shape: `data[]` of `{display_name:"<Resort>, Winter YYYY-YYYY", used_days, max_days(int|null), redemptions[]:{resort_name, redemption_date(ISO), is_return(bool)}}`. Each product = one season (ids rotate yearly); the products list spans seasons → iterate my-products → pass-usage = full multi-year history. Powers `usage`/`most-visited`. Empty `{data:[]}` for current/unskied products in off-season. |
| `/api/v2/reservations` | GET | cookie | User's own reservations. **PII.** Optional `my-reservations`. |

### `reservation-availability` shape (confirmed live, redacted)
`{ "data": [ <one object per pass on the account>, … ] }`, each:
`id` (pass/product id — account-specific, NOT stored), `has_access` (bool),
`reservations` (array of current bookings), `reservations_available` (int),
`current_resort_upcoming_reservation_count` (int), `unavailable_dates` (ISO dates — full),
`blackout_dates` (ISO dates), `closed_dates` (ISO dates), `max_reservation_date` (season horizon).
Bookable = `[today … max_reservation_date]` minus `unavailable ∪ blackout ∪ closed`,
gated by `has_access` and `reservations_available > 0`. (Off-season: date arrays empty.)

## Confirmed NON-endpoints (SPA catch-all `text/html` 0 bytes)
No API for: usage, scans, redemptions, history, reservation-history, days, visits,
activity, transactions, entitlements, profile, account, members, passes, products,
destinations, closures, friends-and-family (top-level), guest/discount-tickets.
→ Those exact names 404, BUT historical usage IS exposed via
`/api/v2/my-products/{productId}/pass-usage` (found on the second probe pass).
"Most visited over time" = aggregate `pass-usage` across the account's passes/seasons
— real history from the API, not just forward-accumulation. (Earlier negative finding corrected.)

## Reachability / WAF
- Plain `curl` (Chrome UA, `Accept: application/json`) clears Incapsula cleanly → printed CLI's standard HTTP client works.
- Automation browsers (agent-browser/Playwright) are Incapsula-BLOCKED → no resident-browser runtime; standard HTTP only.
- Risk: Incapsula can escalate (it killed a 2020 community notifier). Read-only + low volume keeps risk low, not zero.

## Scope
- Single source: Ikon. Flights = separate CLI (out). Deals = pricing calendar, not an API (out).
- v1 = read-only. Booking (POST + CSRF) and F&F voucher *sharing* (mutation) are OUT of v1; show remaining/used only.
