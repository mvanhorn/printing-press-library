package consumer

// Consumer session API (reservations, checkout, geocode helper).
const DefaultBaseURL = "https://spothero.com/api/v1"

// Endpoint paths on the consumer session API.
const (
	PathSearchParams = "/search-params/"
	// PathFacilityRates is legacy ratesSearch on spothero.com (404/dead); book preview uses Craig instead.
	PathFacilityRates   = "/facilities/%d/rates/"
	PathUser            = "/user/"
	PathUsersMe         = "/users/me/"
	PathUserVehicles    = "/users/%d/vehicles/"
	PathUserCreditCards = "/users/%d/credit-cards/" // #nosec G101 -- URL path template; holds no credential
	PathReservations    = "/reservations/"
	PathReservation     = "/reservations/%s/"
	PathCheckout        = "/checkout/"
	// PathReservationRefund is what the site posts to cancel; see RefundRequest. Not yet
	// exercised against the real API.
	PathReservationRefund = "/reservations/%s/refund/"
	// PathReservationRefundEligibility is the read the site's cancel dialog makes
	// first; it decides whether a refund is offered at all.
	PathReservationRefundEligibility = "/reservations/%s/refund-eligibility/"
)

// ConsumerCheckoutVersion is SpotHero-Version on POST /checkout/. Read from the
// consumer-checkout bundle on 2026-11-07; earlier builds of this CLI sent 2025-04-28.
const ConsumerCheckoutVersion = "2026-10-06"

// ConsumerAccountVersion is SpotHero-Version on reservation reads such as
// refund-eligibility; the site pins these separately from checkout.
const ConsumerAccountVersion = "2025-04-28"

// Craig consumer search paths (live confirmed Sep 2026 HAR).
const (
	PathCraigTransientSearch     = "/search/transient"
	PathCraigBulkTransientSearch = "/search/bulk/transient"
	// PathCraigTransientFacility is GET quote/rates for one facility (book preview).
	PathCraigTransientFacility = "/search/transient/%d"
)
