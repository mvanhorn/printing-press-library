# WeWork CLI — End-to-End Booking Validation (live)

## Outcome: booking placed successfully; CLI book contract corrected

A real desk booking was completed via the authenticated browser to validate the
booking flow and capture the true endpoint contract.

- **Booking:** Austin test location — Tuesday Aug 18 2026, 8:30 AM–5:00 PM
- **Charge:** $47.00 to saved Visa (card on file) — user pre-authorized this specific booking
- **Result:** "Your desk is confirmed" — POST /common-booking/ returned BookingStatus + ReservationID
- **Refundable:** full refund until 24h before start (approximately Aug 17 at 08:30 Central Time)
- Invoice/billing address field required by checkout; filled with the building's own
  address per user instruction (saved to account billing via POST /payments/set-billing-address).

## Key finding: inferred book endpoint was WRONG
Pre-capture inference was `common-booking/reserveworkspace`. The REAL flow is:
1. POST /common-booking/quote (price quote, no charge)
2. POST /payments/set-billing-address (Line1, City, State, Zip, Country)
3. POST /common-booking/ (create booking; body incl. CardUuid, CreditCharged, SpaceID,
   LocationID, WeWorkSpaceID, SpaceTypeID, StartTime, EndTime, Currency, UTCOffset, ...)
   -> returns BookingStatus, ReservationID, WeworkUUID, Errors

## Applied to the CLI
- Spec corrected; CLI regenerated. `common-booking book-desk` now targets POST /common-booking/
  with the verified payload; new `common-booking quote-booking` (POST /common-booking/quote);
  `cancel-booking` takes ReservationID.
- Hand-authored novel commands (`desks` city->bbox + --sort/--available-only, friendly
  `cities`/`bookings`) were clobbered by the regen and reapplied durably via a markerless
  self-registering file (internal/cli/wework_novel.go, registerNovelCommand init hook).
- Re-verified: verify/validate-narrative/dogfood/workflow-verify/verify-skill PASS;
  scorecard Grade A; only live_api_verification HOLD (no token in shell).

## Still not exercised
- `cancel-booking` payload not run live (would cancel the real booking). Path from JS,
  input is ReservationID.

## Cancel flow — VERIFIED (real cancellation 2026-08-12, full $47 refund)
Cancel modal stated "you will get refunded the full $47.00"; after confirm, Upcoming was empty.
Real endpoint: `POST /workplaceone/api/common-booking/cancel` (NOT the JS-inferred
`user-booking-cancellation`). Payload: bookingId, reservationId, bookingExternaluuid,
bookingType, bookingLocationType, locationId, spaceId, reservableId, startTime, endTime,
creditsUsed, isBookingApprovalOn, cancellationNote, mailParams. Spec + CLI corrected and
regenerated; `common-booking cancel-booking` now targets /common-booking/cancel.

## Regen gotcha (retro candidate)
`generate --force` quarantined the markerless self-registering hand file
(internal/cli/wework_novel.go) into a `.preserve-*` snapshot instead of merging it back into
the live tree, despite the registerNovelCommand hook pattern. Manual restore from the snapshot
was required after each regen. Also must re-bump golang.org/x/text to v0.39.0 after every regen.
