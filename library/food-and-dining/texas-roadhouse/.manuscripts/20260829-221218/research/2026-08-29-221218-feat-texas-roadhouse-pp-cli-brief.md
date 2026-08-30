---
api_name: texas-roadhouse
spec_source: sniffed
generated_at: 2026-08-29
---

# Texas Roadhouse CLI Brief

Waitlist CLI for Texas Roadhouse call-ahead seating. Find a nearby store, read the quote, join the list, check in at the host stand, or cancel.

## API Identity

- **Domain:** restaurant call-ahead waitlist (not table reservations, not online ordering).
- **Users:** diners who want on a nearby store list; agents acting only after the human names a store and party size.
- **Data profile:** nearby stores (`extref`, name, address, distance), per-party-size quote buckets (`MinQuote` / `MaxQuote` for sizes 1–6), waitlist request status, check-in, cancel.

## Source

Internal spec derived from a sanitized browser sniff of `www.texasroadhouse.com`. There is no public OpenAPI. Auth type is none. Research notes record contract facts only — no customer phones, emails, tokens, cookies, raw HAR, or traffic dumps.

## Verified contract facts

- Store waitlist paths use store **extref**, not the internal store id. Example: Springfield MO extref is `218`.
- Email is required on join (`EmailAddress`).
- Party size max is **6**.
- `WaitMinutes` on submit comes from that party size's quote **MinQuote**.
- Cancel body field `waitlistRequestId` is a **JSON number**. A string value 400s.
- Cancel `siteId` is the store extref. Query `clientid` must be `texasroadhouse`.
- Check-in is **HERE** / `POST /api/texasroadhouse/waitlist/{extref}/checkin`.
- Join is `POST /api/texasroadhouse/waitlist/{extref}/submit`.
- Quote is `GET /api/texasroadhouse/waitlist/{extref}/quote`.
- Status is `GET /api/texasroadhouse/waitlist/{extref}/requests/{request_id}/status`.

## Top workflows

1. Geocode a named place (`mapbox`), list nearby stores (`stores --lat --long`), quote each extref.
2. Present stores. Stop. Wait for the human to name one store and a party size (1–6).
3. Dry-run submit. Live join only with `--yes`.
4. Status while waiting. Check-in (HERE) at the host stand only with `--yes`.
5. Cancel only with `--yes`. `waitlistRequestId` stays a JSON number.

## Safety

Live join, check-in, and cancel mutate a guest waitlist. Default (no `--yes`, no `--dry-run`) must refuse. `--dry-run` prints the body and does not POST. MCP mirrors of those three tools are destructive and use the same confirmation meaning.
