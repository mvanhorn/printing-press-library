---
name: pp-jinko
description: Search flights and hotels, build a multi-product trip (flight + hotel in one cart), and check out via Stripe. Use when the user wants to book real travel from the terminal — e.g. "book a flight from Paris to NYC June 15 and a hotel in Manhattan for those nights, one payment."
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/jinko/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Jinko Travel

Jinko is the only CLI in this library that **books** flights and hotels together with a real Stripe checkout. Reverse-engineered scrapers (flight-goat, booking-com, airbnb) surface prices; Jinko issues PNRs and confirmations.

## When to pick Jinko

- The user wants to actually book, not just price-check.
- The user wants flight **and** hotel in one cart, paid once.
- The user wants reproducible programmatic bookings (CI/CD, scheduled trips, agent workflows).

Use flight-goat or booking-com first when:
- The user is browsing prices and is not ready to book.
- The user wants to compare Jinko's price against scraped sources.

## Auth

```bash
jinko auth login --key jnk_...   # or set JINKO_API_KEY
jinko auth status                # confirm before any booking command
```

Get a key at https://app.gojinko.com/devplatform.

## Token flow — read this before composing commands

```
find-flight / find-destination  →  offer_token       (cached search)
flight-search   --offer-token   →  trip_item_token   (live + bookable)
hotel-search    --city ...       →  trip_item_token   (live)
trip            --trip-item-token (flight or hotel)  →  trip_id
book            --trip-id        →  checkout_url     (Stripe-hosted)
trip-status     --trip-id        →  PNRs + lifecycle
```

- `trip_item_token`s are time-bounded — use them within minutes, not hours. If you get a "stale token" error, re-run the search.
- `quote` is **automatic** inside `book`. Do not look for a `jinko quote` command — it doesn't exist.
- The same `trip_id` can hold flight items **and** hotel items. Add as many as you need before calling `book`.

## Canonical end-to-end script

```bash
# 1. Search both products in parallel (run as background jobs in zsh / bash 4+)
flight=$(jinko flight-search \
  --from PAR --to NYC --date 2026-06-15 --return 2026-06-22 --passengers 2 \
  --format json | jq -r '.offers[0].trip_item_token')

hotel=$(jinko hotel-search \
  --city "New York" --checkin 2026-06-15 --checkout 2026-06-22 --adults 2 \
  --format json | jq -r '.hotels[0].rooms[0].rates[0].trip_item_token')

# 2. Build one trip with both
trip=$(jinko trip --trip-item-token "$flight" --format json | jq -r '.trip_id')
jinko trip --trip-id "$trip" --trip-item-token "$hotel"

# 3. Set travelers (required before book)
jinko trip --trip-id "$trip" \
  --travelers '[{"first_name":"Jane","last_name":"Doe","date_of_birth":"1990-01-15","gender":"FEMALE","passenger_type":"ADULT"},
                {"first_name":"John","last_name":"Doe","date_of_birth":"1988-04-22","gender":"MALE","passenger_type":"ADULT"}]' \
  --contact '{"email":"jane@example.com","phone":"+33612345678"}'

# 4. Book → returns a Stripe checkout URL the user pays on
jinko book --trip-id "$trip"

# 5. Poll until fulfilled (TravelFusion bookings can take up to 72h)
while true; do
  state=$(jinko trip-status --trip-id "$trip" --format json | jq -r '.state')
  echo "trip state: $state"
  [[ "$state" == "fulfilled" || "$state" == "failed" ]] && break
  sleep 60
done
```

## Anti-patterns to avoid

- **Don't add an item before the previous response returns.** Each `trip` call returns the merged state — wait for it before the next call.
- **Don't compose a `trip` with a stale `trip_item_token`.** Re-run `flight-search` / `hotel-search` if more than ~5 minutes have passed.
- **Don't poll `trip-status` faster than every 30s.** The lifecycle has natural step delays; tighter polling burns rate-limit budget without learning anything new.
- **Don't try to pay programmatically.** `book` returns a Stripe URL; the user must visit it. There is no card-token endpoint in v0.1.

## Output

Every command emits JSON to stdout by default. Errors go to stderr as `{ "ok": false, "error": { "code", "message" } }` — switch on `error.code`, not on the message string. Exit codes: 0 success, 1 API error, 2 auth required, 3 input error.
