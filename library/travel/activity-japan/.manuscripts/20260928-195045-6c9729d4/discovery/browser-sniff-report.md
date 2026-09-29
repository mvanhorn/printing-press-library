# Activity Japan browser discovery report

## User goal flow

Goal: find a Kyoto cultural workshop, inspect one plan, then inspect a future date's sessions without making a booking.

1. A fresh temporary headless browser opened EN Kyoto search, JA Kyoto search and EN plan 21688; all showed CloudFront 403.
2. Codex's in-app browser was unavailable. A fresh tab in the user's existing Firefox loaded EN Kyoto search and displayed live plan cards with IDs, operator labels, headline prices, age bands and broad durations. The agent clicked plan 62375.
3. The plan page displayed operator link/ID 9326, base and optional prices, fees, age 2–100, group 1–50, duration 60 minutes, meeting lead of 5 minutes, meeting point, address, restrictions, and cancellation text. Its calendar legend distinguished immediate and request bookings.
4. The agent opened the October 2026 calendar and selected October 8. The page displayed 10 selectable sessions marked immediate booking and an unselectable 18:30 session. No session was submitted for booking.
5. A fresh private Firefox window showed Human Verification on the same EN Kyoto search page. No challenge was solved. A full HAR export was attempted but automatic approval review rejected it because it might persist browser credentials. No HAR or session cookie was captured.

Completed: search-to-detail-to-date flow in existing Firefox; anonymous replay validation of data routes. Not completed: replay of server-rendered search HTML.

## Replayable endpoints

| Route | Purpose | Direct anonymous replay |
|---|---|---|
| `GET /get_plan_price_info` | Plan/partner fields and named price options, localized by `lang_flag` | 200 JSON; EN and JA verified |
| `GET /plan/get_plan_data` | Plan fields and nested plan price data | 200 JSON |
| `GET /plan/get_calendar` | Date-level status and headline price for a month | 200 JSON; multiple plans |
| `GET /plan/get_plan_price` | Selected-date price by `price_id` | 200 JSON; two dates |
| `GET /select_plan_course` | Selected-date course IDs, times and course status | 200 JSON; immediate/request/unselectable observed |
| `GET /plan/check_calendar_data` | Read-only selected course and party stock recheck | 200 JSON; valid and over-limit count |

All requests used `gd.activityjapan.com` without Cookie or Authorization headers. Public JSON responses were assembled into a credential-free enriched capture; Printing Press generated a six-endpoint spec and traffic analysis. A plan ID not found on the source returned an HTML fallback with HTTP 200, so the source client must validate content type/schema and returned ID.

## Source interpretation

`plan_id`, `partner_id`, `plan_price_id` and course ID are separate source identities. The site displays a headline `from` price; `get_plan_price` gives a date/option amount. `get_calendar` price is a date-level headline, not a selected-party total. `select_plan_course` statuses are observations. Site calendar script maps code 1 to booking OK and 3 to request, while 4 is closed and 5 is unselectable; code 2 remains unknown. The Firefox page legend says booking OK means immediate booking and request booking requires provider response. The source's own price text says amounts may vary by schedule.

`support_language` varies by requested locale but does not expose a list of instructor languages. Keep it separate from site language. Plan 64974's `price_suffix=pair` shows why per-group/per-person basis cannot be inferred from a common JPY amount.

## Reachability and limits

`en.activityjapan.com` and `activityjapan.com` search/plan HTML require a clearance browser session from this environment. Direct stdlib HTTP returned 403; Surf Chrome-like HTTP and a Firefox User-Agent received AWS WAF 202 challenge. The user's existing Firefox succeeded; a new private window showed Human Verification. The CLI cannot depend on a resident browser. A browser clearance import might make HTML replay possible, but this was not tested because site-scoped cookie capture permission is pending. The official partner information API remains contract-gated.

No availability observation is a reservation. Discovery requests were read-only, paced near one request per second, and limited to bounded future dates. Raw browser credentials, cookies, HAR files and account data were not saved.
