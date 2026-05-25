# Jinko Travel CLI

Industry-grade stable API for flights **and** hotels — with multi-product cart booking. Search flights, search hotels, put both in one trip, pay once.

Every other travel CLI in this library books flights **or** hotels. Jinko books them together in a single Stripe payment.

```bash
# Search a flight, search a hotel, build a multi-product trip, check out
flight_token=$(jinko flight-search --from PAR --to NYC --date 2026-06-15 --return 2026-06-22 \
  --format json | jq -r '.offers[0].trip_item_token')
hotel_token=$(jinko hotel-search --city "New York" --checkin 2026-06-15 --checkout 2026-06-22 \
  --format json | jq -r '.hotels[0].rooms[0].rates[0].trip_item_token')

trip=$(jinko trip --trip-item-token "$flight_token" --format json | jq -r '.trip_id')
jinko trip --trip-id "$trip" --trip-item-token "$hotel_token"
jinko trip --trip-id "$trip" --travelers '[{"first_name":"Jane","last_name":"Doe","date_of_birth":"1990-01-15","gender":"FEMALE","passenger_type":"ADULT"}]' \
                            --contact '{"email":"jane@example.com","phone":"+33612345678"}'

jinko book --trip-id "$trip"   # → checkout_url (Stripe-hosted, one page for the whole cart)
jinko trip-status --trip-id "$trip"   # → PNRs once fulfilled
```

## Install

```bash
npx -y @mvanhorn/printing-press-library install jinko
```

Installs the `jinko-pp-cli` Go binary **and** the focused `/pp-jinko` skill in one shot.

CLI only:
```bash
npx -y @mvanhorn/printing-press-library install jinko --cli-only
```

Skill only:
```bash
npx -y @mvanhorn/printing-press-library install jinko --skill-only
```

### Without Node

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-cli@latest
```

## Authenticate

Get an API key at [app.gojinko.com/devplatform](https://app.gojinko.com/devplatform), then:

```bash
jinko auth login --key jnk_...
# or
export JINKO_API_KEY=jnk_...
```

The same credential works with [`@gojinko/cli`](https://www.npmjs.com/package/@gojinko/cli) (the Node CLI) and Jinko's MCP server — pick whichever transport you prefer; switching between them does not require re-auth.

Inspect or clear:
```bash
jinko auth status     # masked token + source
jinko auth logout     # clears ~/.jinko/config.yaml
```

## Token flow

Named tokens compose left-to-right — no hidden state.

```
find-flight / find-destination  →  offer_token       (cached, repeatable)
flight-search   --offer-token   →  trip_item_token   (live, bookable)
hotel-search    --city ...       →  trip_item_token   (live)
trip            --trip-item-token (flight or hotel)  →  trip_id  (multi-product cart)
book            --trip-id        →  checkout_url     (Stripe-hosted page)
trip-status     --trip-id        →  PNRs + lifecycle (poll until fulfilled)
```

Quote is automatic — it runs inside `book`. There is no separate quote command to remember.

## Commands

### Discovery

| Command | What it does |
|---|---|
| `find-flight --from PAR --to NYC --date 2026-06-15` | Cached search by route+date — fast, returns `offer_token`s |
| `find-destination --from PAR --date 2026-06-15` | "Cheapest-from" — what destinations are affordable from PAR? |
| `flight-search --from PAR --to NYC --date 2026-06-15 --return 2026-06-22 --passengers 2` | Live prices + `trip_item_token` |
| `flight-search --offer-token <token>` | Live price-check one cached offer |
| `hotel-search --city "Paris" --checkin 2026-06-15 --checkout 2026-06-18 --adults 2` | Live hotel inventory + per-rate `trip_item_token` |
| `hotel-details --hotel-id <id>` | Gallery, facilities, policies, room details |

### Trip building + checkout

| Command | What it does |
|---|---|
| `trip --trip-item-token <token>` | Create a new trip with one item |
| `trip --trip-id <id> --trip-item-token <token>` | Add another item to an existing trip (flight + hotel coexist) |
| `trip --trip-id <id> --remove-item-id <item_id>` | Remove an item — or swap one by adding `--trip-item-token` |
| `trip --trip-id <id> --travelers '[...]' --contact '{...}'` | Set travelers + contact |
| `book --trip-id <id>` | Schedule checkout — returns `checkout_url` + items + ancillaries |
| `trip-status --trip-id <id>` | Cart + quote + payment + fulfillment + PNRs |

### Auth

| Command | What it does |
|---|---|
| `auth login --key jnk_...` | Save API key to `~/.jinko/config.yaml` |
| `auth status` | Show masked token + source (flag / env / config) |
| `auth logout` | Clear credentials, rotate correlation id |

## What makes Jinko different

| | flight-goat | booking-com | airbnb | **jinko** |
|---|---|---|---|---|
| Flights | scraped (Kayak + Google Flights) | — | — | ✓ stable BFF (Sabre, Travelfusion, ...) |
| Hotels | — | scraped (cookies) | scraped (cookies) | ✓ stable BFF (Nuitee + suppliers) |
| Bookable end-to-end | — | — | — | ✓ (Stripe Checkout in one URL) |
| Multi-product cart | — | — | — | ✓ flight + hotel in one trip |
| API stability SLA | no | no | no | ✓ contracted |

Reverse-engineered scrapers break when the upstream changes their HTML or cookies. Jinko is a contracted supplier with an SLA — the same API powers production bookings on [gojinko.com](https://app.gojinko.com).

## Output format

Default is JSON (pipe-friendly). Pass `--format table` for human-readable tables on a subset of commands (extending).

Errors are emitted to **stderr** as structured JSON with a stable error `code` so agents can switch on it without parsing English. Exit codes:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | API error (4xx / 5xx from the BFF) |
| 2 | authentication required |
| 3 | invalid user input |

## Observability

Every request carries:
- `X-Source: cli`, `X-Client-Kind: jinko-pp-cli/<version>` — for Datadog filtering
- `X-Session-ID` — stable for 24h per CLI install (`~/.jinko/.session`)
- `X-Request-ID` — fresh per HTTP call
- `traceparent` — W3C trace-context, one trace per CLI invocation

This is the same instrumentation as the official Node CLI, so a multi-tool session (CLI + MCP) clusters under one logical user in dashboards.

## Related

- [`@gojinko/cli`](https://www.npmjs.com/package/@gojinko/cli) — the Node CLI; identical surface, same auth
- [`@gojinko/api-client`](https://www.npmjs.com/package/@gojinko/api-client) — typed TypeScript SDK
- [Jinko MCP server](https://mcp.gojinko.com) — direct MCP transport

## License

Apache 2.0 — see [LICENSE](LICENSE).

Printed by [@dyzsasd](https://github.com/dyzsasd).
