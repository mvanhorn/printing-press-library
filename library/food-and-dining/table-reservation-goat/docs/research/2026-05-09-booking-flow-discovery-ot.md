# OpenTable booking flow — chrome-MCP discovery (2026-05-09)

Captured live via chrome-MCP against `opentable.com` with Pejman's session cookies. One real test booking made (Water Grill Bellevue, party 2, 2026-05-09 23:00) and immediately cancelled. No persistent state on user's account.

## Endpoint summary

| Operation | Type | Path / Op | Auth |
|-----------|------|-----------|------|
| List upcoming reservations | SSR scrape | `GET /user/dining-dashboard` → `__INITIAL_STATE__.diningDashboard.upcomingReservations[]` | Session cookies |
| Validate slot before book | GraphQL persisted | `POST /dapi/fe/gql?optype=query&opname=BookingValidateSlotStatus` | Session cookies + `x-csrf-token` |
| Reservation detail (with cancel deadline) | GraphQL persisted | `POST /dapi/fe/gql?optype=query&opname=BookingConfirmationPageInFlow` | Session cookies + `x-csrf-token` |
| **Book** | REST | `POST /dapi/booking/make-reservation` | Session cookies + `x-csrf-token` |
| **Cancel** | GraphQL persisted | `POST /dapi/fe/gql?optype=mutation&opname=CancelReservation` | Session cookies + `x-csrf-token` |

## Persisted-query hashes (capture timestamp 2026-05-09)

```
BookingValidateSlotStatus:    4ae88a52122d183cdc38279476a80926da037b30b74ee2b62988020e454301d8
BookingConfirmationPageInFlow: 7c4fe1d7786e25085199bddc46cb1525ebf90ac18621ceb3021074fda52b6...  // truncated in capture; refresh on first build
CancelReservation:             4ee53a006030f602bdeb1d751fa90ddc4240d9e17d015fb7976f8efcb80a026e
RestaurantsAvailability:       cbcf4838a9b399f742e3741785df64560a826d8d3cc2828aa01ab09a8455e29e  // already in opentable/client.go
```

## Discovery gates evaluated

| Gate | Status |
|------|--------|
| 1. Lock+commit required for free? | **PASS** — single POST `make-reservation` with no separate slot-lock POST. (A `slotLockId` is in the body, but it appears to be a server-side allocation tied to the slot, not a separate client-driven step.) |
| 2. CSRF / JWT requires JS execution? | **PASS** — `x-csrf-token` header is required, but it can be obtained from the existing `Bootstrap()` flow that already runs for `RestaurantsAvailability`. No JS engine required. |
| 3. OT WAF blocks booking opname? | **PASS** — `make-reservation` returned HTTP 200 cleanly. Test booking succeeded end-to-end with the kooky-imported cookies. No chromedp fallback needed for booking. |
| 4. Slot-token TTL too short? | **PASS-with-note** — observed slot-lock TTL of ~5 minutes ("We're holding this table for 4:55 minutes"). Plenty of headroom for the agent → CLI → network round-trip. |

## Slot-lock URL parameters (from `/booking/details` URL)

When the user clicks a slot on a venue page, the page navigates to `/booking/details?...` with these params:

```
availabilityToken      base64-encoded JSON {"v":3,"m":0,"p":0,"c":6,"s":0,"n":0}  // schema version + flags
correlationId          UUID v4 (client-generated)
creditCardRequired     false (for free reservations)
dateTime               2026-05-09T23:00:00 (ISO local)
partySize              2
points                 100
pointsType             Standard
resoAttribute          default
rid                    1255093 (restaurantId)
slotHash               3635024472 (32-bit slot ID)
isModify               false
isMandatory            false
cfe                    true
st                     Standard
```

These params are echoed back into the `make-reservation` request body. The book client must obtain them via `BookingValidateSlotStatus` (which returns `slotHash` and `creditCardRequired`) — but `slotLockId` is generated server-side during the page navigation; the CLI book client must replicate the slot-lock by either replaying the URL pattern OR calling whatever XHR generates the `slotLockId`.

**Implementation note:** The browser flow is: click slot → navigate to `/booking/details?...&slotLockId=NNN&slotAvailabilityToken=...` — the slot-lock-id is allocated server-side as part of that page render. The CLI will need to either:
- (a) GET `/booking/details?...` with the URL params and parse `slotLockId`/`slotAvailabilityToken` from the rendered HTML or its `__INITIAL_STATE__`, then POST `make-reservation` with those values; OR
- (b) Reverse-engineer the lock-allocation XHR if there's a separate one. In the captures I reviewed, no separate slot-lock XHR fired before page navigation — the lock is allocated at the SSR render step.

Approach (a) matches the existing `Bootstrap()` SSR-scrape pattern and is the recommended path.

## `make-reservation` request body (sanitized shape)

```json
{
  "additionalServiceFees": [],
  "attributionToken": "<sensitive: base64-ish, from ad attribution; can be empty string>",
  "correlationId": "<UUID v4 from /booking/details URL>",
  "country": "US",
  "diningAreaId": 1,
  "email": "<from user profile>",
  "fbp": "fb.1.1777091576025.6989748418527359",  // Facebook browser ID; can be empty
  "firstName": "<from user profile>",
  "isModify": false,
  "katakanaFirstName": "",
  "katakanaLastName": "",
  "lastName": "<from user profile>",
  "nonBookableExperiences": [],
  "optInEmailRestaurant": false,
  "partySize": 2,
  "phoneNumber": "<from user profile>",
  "phoneNumberCountryId": "US",
  "points": 100,
  "pointsType": "Standard",
  "reservationAttribute": "default",
  "reservationDateTime": "2026-05-09T23:00",  // ISO local, no Z
  "reservationType": "Standard",
  "restaurantId": 1255093,
  "slotAvailabilityToken": "<base64 JWT-ish from /booking/details URL>",
  "slotHash": "3635024472",  // string in body, but a number in BookingValidateSlotStatus response
  "slotLockId": 1074646572,  // int — server-allocated at /booking/details render
  "tipAmount": 0,
  "tipPercent": 0
}
```

Required headers: `Content-Type: application/json`, `Accept: application/json`, `x-csrf-token: <from Bootstrap>`.

## `make-reservation` response body (verbatim, with PII flagged)

```json
{
  "reservationId": 2075207711,
  "restaurantId": 1255093,
  "reservationDateTime": "2026-05-09T23:00",
  "partySize": 2,
  "confirmationNumber": 114309,
  "points": 0,
  "reservationStateId": 1,
  "securityToken": "01Ozsdas9H1Yx6dXtW5dizza-Y9m2uxbTqbeXGqP_2IhQ1",  // <sensitive>
  "gpid": 140030454104,
  "isRestRef": false,
  "reservationHash": "geBpiF6w796h8laSK+vK3N5z+C08VDwBM04JLKMXrkU=",
  "reservationType": "Standard",
  "reservationSource": "Online",
  "creditCardLastFour": null,
  "order": null,
  "nonBookableExperiences": null,
  "experience": null,
  "redemptionToken": null,
  "redemptionError": false,
  "userType": 1,
  "diningAreaId": 1,
  "environment": "Indoor",
  "deposit": null,
  "partnerScaRequired": false,
  "partnerScaRedirectUrl": null,
  "privilegedAccess": null,
  "success": true
}
```

Notable: **no `cancelCutoffDate` in the book response.** Cancellation deadline must be fetched via a follow-up `BookingConfirmationPageInFlow` GraphQL query (or surfaced as `null` in CLI output).

## `BookingConfirmationPageInFlow` GraphQL — fetches cancellation deadline + full reservation detail

**Variables:**
```json
{
  "gpid": 0,
  "diningHistoryLimit": 4,
  "popularDishesCount": 3,
  "popularDishesReviewCount": 5,
  "showPopularDishes": true,
  "usefallBackCancellationPolicyMessage": false,
  "enableTicketedExperiences": false,
  "useCBR": false,
  "enablePrivateDiningExperiences": false,
  "rid": 1255093,
  "tld": "com",
  "confirmationNumber": 114309,
  "databaseRegion": "NA",
  "securityToken": "<from book response>",
  "countryId": "US",
  "isLoggedIn": true
}
```

**Response (relevant subset):**
```json
{
  "data": {
    "reservation": {
      "dateMade": "2026-05-10T05:06Z",
      "localDateTime": "2026-05-09T23:00",
      "dateTime": "2026-05-10T06:00Z",  // UTC
      "partySize": 2,
      "confirmationNumber": 114309,
      "diningAreaId": 1,
      "attribute": "Default",
      "hash": "geBpiF6w796h8laSK+vK3N5z+C08VDwBM04JLKMXrkU=",
      "cancelCutoffDate": "2026-05-10T05:55Z",  // <-- THIS IS THE CANCELLATION DEADLINE
      "diner": {"firstName": "...", "lastName": "...", "phone": {...}},
      "notes": "",
      "occasion": null,
      "source": "Online",
      "points": 0,
      "pointsRule": "LOYALTY_SUPPRESSED_POINTS",
      "experience": null,
      "type": "Standard",
      "state": {"name": "Pending", "reservationStateId": 1},
      "isRestRef": false,
      "privilegedAccess": null,
      "__typename": "Reservation"
    }
  }
}
```

## `CancelReservation` GraphQL

**Variables:**
```json
{
  "input": {
    "restaurantId": 1255093,
    "confirmationNumber": 114309,
    "securityToken": "<from book response>",
    "databaseRegion": "NA",
    "reservationSource": "Online"
  }
}
```

**Response:**
```json
{
  "data": {
    "cancelReservation": {
      "statusCode": 200,
      "errors": null,
      "data": {
        "restaurantId": 1255093,
        "reservationId": 2075207711,
        "reservationStateId": 3,
        "reservationState": "CancelledWeb",
        "confirmationNumber": 114309,
        "refundStatus": null
      }
    }
  }
}
```

## `__INITIAL_STATE__.diningDashboard.upcomingReservations[]` shape

For `ListUpcomingReservations`. SSR-hydrated on `/user/dining-dashboard`. No XHR needed.

```
UserTransaction {
  __typename: "UserTransaction"
  isUpcoming: bool
  reservationState: enum  // "PENDING" observed
  reservationType: string  // "Experience" observed
  confirmationNumber: int  // public-ish display number
  confirmationId: int      // internal
  securityToken: string    // <sensitive> — used in cancel
  restaurantId: int
  restaurantName: string
  dateTime: string         // ISO local "2026-05-10T11:15:00"
  partySize: int
  isPrivateDining: bool
  isForPrimaryDiner: bool
  dinerFirstName, dinerLastName: string  // PII; sanitize at CLI boundary
  points: int              // loyalty
  restaurant: { __typename: "Restaurant", photos: ... }
}
```

## Discriminator pattern for typed errors

Not yet captured (no error case triggered live). Expected pattern based on response shapes seen:

- `make-reservation`: REST POST. Likely returns 4xx with `{"success": false, "errorCode": "..."}` for slot-taken / payment-required. Implementation note: U2 must capture an actual slot-taken response to validate.
- `CancelReservation`: GraphQL. Returns `{"data": {"cancelReservation": {"statusCode": <code>, "errors": [...], "data": null}}}` — `errors` array is the discriminator. statusCode 200 = success; non-200 = check errors.

## Idempotency-Key

Not observed in any captured request. **Idempotency-Key header is NOT supported** by these endpoints (or at minimum, the browser flow doesn't use it). Filesystem advisory lock in U4 is the primary cross-process safety net.

## Implementation roadmap for U2

1. New file `internal/source/opentable/booking.go`
2. `Book(ctx, slug, date, time, party int, lat, lng float64) (*BookResponse, error)`:
   - GET `/r/<slug>?covers=N&dateTime=YYYY-MM-DDTHH:MM` → parse SSR for restaurantId
   - POST `BookingValidateSlotStatus` (GraphQL persisted) → confirm slot still available
   - GET `/booking/details?...` (with all the URL params from validation result) → parse SSR for `slotLockId` and `slotAvailabilityToken`
   - POST `/dapi/booking/make-reservation` (REST) with the full body
   - On success, optionally POST `BookingConfirmationPageInFlow` to fetch `cancelCutoffDate`
   - Return `*BookResponse` with `ConfirmationNumber`, `SecurityToken`, `ReservationID`, `CancelCutoffDate`
3. `Cancel(ctx, reservationID string) (*CancelResponse, error)`:
   - reservation ID is `confirmationNumber` (public form)
   - Need to also pass `securityToken` and `restaurantId` — implies either: (a) `Cancel` takes a triple `{confirmationNumber, securityToken, restaurantId}` instead of a single ID, OR (b) `Cancel` first does `ListUpcomingReservations` to look up the security token by confirmation number.
   - Recommend (a): `Cancel(ctx, ResID{ConfirmationNumber, SecurityToken, RestaurantID}) (*CancelResponse, error)`. The CLI's `book` returns these fields, so the agent passes them on cancel.
   - POST `CancelReservation` (GraphQL persisted) with the input
4. `ListUpcomingReservations(ctx) ([]UpcomingReservation, error)`:
   - GET `/user/dining-dashboard`
   - Parse `__INITIAL_STATE__.diningDashboard.upcomingReservations[]`
   - Map to `UpcomingReservation` Go struct

**Plan refinement needed:** The `cancel <network>:<reservation-id>` CLI shape needs to handle the triple `{confirmationNumber, securityToken, restaurantId}` for OT. Either encode all three in a compound ID, or change the CLI to take `--security-token` and `--rid` flags alongside the confirmation number. Decision: encode as `opentable:<rid>:<confirmationNumber>:<securityToken>` (three colon-separated fields), or just preserve the booking-view URL format.

This refinement should be carried back to the plan before U2 implementation begins.
