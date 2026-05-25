# Agent guidance for jinko-pp-cli

This file mirrors `SKILL.md` but is structured for **autonomous agents** integrating Jinko into a longer workflow (Claude Code, Codex, custom orchestrators). Read it once and link to commands by exact name in your tool descriptions.

## Surface

| Command | Endpoint | Idempotent | Cost |
|---|---|---|---|
| `auth login/logout/status` | local config | yes | free |
| `find-flight` | `POST /api/v1/flights/search` | yes | cached, free |
| `find-destination` | `POST /api/v1/flights/destination-search` | yes | cached, free |
| `flight-search` | `POST /api/v1/flights/shop` | yes | live API call |
| `hotel-search` | `POST /api/v1/hotels/shop` | yes | live API call |
| `hotel-details` | `GET /api/v1/hotels/{id}/details` | yes | live API call |
| `trip` (no `--trip-item-token`) | `POST /api/v1/shop/sync` | yes | free (cart op) |
| `trip` (with token) | `POST /api/v1/shop/sync` | **no** (creates / mutates trip) | free |
| `book` | `POST /api/v1/shop/sync/checkout` | **no** | schedules payment; user must pay on the returned URL |
| `trip-status` | `GET /api/v1/shop/sync/{id}` | yes | free |

"Idempotent" here means: re-running with the same flags does not change server state.

## Composition rules

1. **One trip per multi-product booking.** Don't create a separate trip for the flight and the hotel — put them in the same `trip_id`. The user gets one Stripe URL and one PNR-bundle email.
2. **Set travelers before `book`.** The BFF rejects `book` if any item lacks the required passenger metadata. Set it once per trip.
3. **`book` is non-destructive — the user has not paid yet.** The trip stays in `pending_payment` until they complete Stripe. Safe to re-run `book` to get a fresh URL.
4. **Long async fulfillment is normal.** TravelFusion confirmations can take up to 72 hours. Poll `trip-status` — don't assume failure based on a short timeout.

## Error handling

| `error.code` | What it means | Recommended action |
|---|---|---|
| `AUTH_REQUIRED` | No API key | Prompt the user to `jinko auth login --key jnk_...` |
| `AUTH_INVALID` | Bad / expired credential | Same as above, plus suggest checking `app.gojinko.com/devplatform` |
| `OFFER_EXPIRED` | `offer_token` / `trip_item_token` is stale | Re-run the corresponding `*-search` command |
| `TRIP_NOT_FOUND` | Wrong `trip_id` | Check the value; trips are scoped to your tenant |
| `PRICE_CHANGED` | Live price differs from cached one | Surface the diff to the user before re-attempting |
| `BOOKING_FAILED` | Fulfillment failed on the supplier side | Read `trip-status` for the rejection reason; refund is automatic |
| `VALIDATION_ERROR` | Missing or malformed field | Read the human message; fix the inputs |

`X-Request-ID` is logged with every response — include it in support tickets for fast triage.

## Versioning

`jinko --version` prints the CLI version. `printing-press-library install jinko` always pulls the latest tagged release. Pin in CI with `go install ...@vX.Y.Z`.
