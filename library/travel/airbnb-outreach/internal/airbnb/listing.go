// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Listing fetches the PDP (property detail page) sections for a listing.
// id may be numeric or an encoded global ID. Dates are optional; when given
// they drive the availability/price sections. StaysPdpSections is served only
// over POST and requires the full fragment-include map (captured from the web
// client) — a minimal variables object is rejected with a generic error.
func (c *Client) Listing(id, checkin, checkout string, adults int) (json.RawMessage, error) {
	num := NumericID(id)
	if adults <= 0 {
		adults = 1
	}
	dateRange := "null"
	if checkin != "" && checkout != "" {
		dateRange = fmt.Sprintf(`{"startDate":%q,"endDate":%q}`, checkin, checkout)
	}
	vars := strings.NewReplacer(
		"__ID__", EncodeGlobalID("StayListing", num),
		"__DEMANDID__", EncodeGlobalID("DemandStayListing", num),
		"__ADULTS__", strconv.Itoa(adults),
		"__CHECKIN__", jsonStringOrNull(checkin),
		"__CHECKOUT__", jsonStringOrNull(checkout),
		"__DATERANGE__", dateRange,
		"__P3__", fmt.Sprintf("p3_%s_pp", num),
	).Replace(pdpVariablesTemplate)

	var v map[string]any
	if err := json.Unmarshal([]byte(vars), &v); err != nil {
		return nil, fmt.Errorf("building PDP variables: %w", err)
	}
	return c.QueryPost("StaysPdpSections", v)
}

func jsonStringOrNull(s string) string {
	if s == "" {
		return "null"
	}
	return `"` + s + `"`
}

// pdpVariablesTemplate is the StaysPdpSections variables shape captured from the
// Airbnb web client. The include-fragment flags are required: omitting them
// makes the server reject the query. __PLACEHOLDER__ tokens are substituted per
// request. Dates are injected raw (a JSON string or the literal null).
const pdpVariablesTemplate = `{
"id":"__ID__","demandStayListingId":"__DEMANDID__",
"pdpSectionsRequest":{"adults":"__ADULTS__","amenityFilters":null,"bypassTargetings":false,"categoryTag":null,"causeId":null,"children":null,"disasterId":null,"discountedGuestFeeVersion":null,"federatedSearchId":null,"forceBoostPriorityMessageType":null,"hostPreview":false,"infants":null,"interactionType":null,"layouts":["SIDEBAR","SINGLE_COLUMN"],"pets":0,"pdpTypeOverride":null,"photoId":null,"preview":false,"previousStateCheckIn":null,"previousStateCheckOut":null,"priceDropSource":null,"partner":null,"partnerProgram":null,"privateBooking":false,"promotionUuid":null,"relaxedAmenityIds":null,"searchId":null,"selectedCancellationPolicyId":null,"selectedRatePlanId":null,"splitStays":null,"staysBookingMigrationEnabled":false,"translateUgc":null,"useNewSectionWrapperApi":false,"sectionIds":["POLICIES_DEFAULT","BOOK_IT_SIDEBAR","BOOK_IT_NAV","BOOK_IT_FLOATING_FOOTER","BOOK_IT_CALENDAR_SHEET","CANCELLATION_POLICY_PICKER_MODAL"],"checkIn":__CHECKIN__,"checkOut":__CHECKOUT__,"p3ImpressionId":"__P3__"},
"categoryTag":null,"federatedSearchId":null,"federatedSearchSessionId":null,"p3ImpressionId":"__P3__","photoId":null,"amenityIds":null,
"dateRange":__DATERANGE__,
"guestCounts":{"numberOfAdults":__ADULTS__},"numberOfChildren":null,"numberOfInfants":null,"numberOfPets":null,
"includePdpMigrationAccessibilityFeaturesModalFragment":false,"includeGpAccessibilityFeaturesFragment":true,"includePdpMigrationAccessibilityFeaturesPreviewCarouselFragment":false,"includePdpMigrationLuxeServicesFragment":false,"includeGpLuxeServicesFragment":true,"includeGpAdminBannerFragment":true,"includePdpMigrationBookItNavFragment":false,"includeGpBookItFragment":true,"includePdpMigrationAmenitiesFragment":false,"includeGpAmenitiesFragment":true,"includeGpCancellationPolicyPickerModalFragment":true,"includePdpMigrationAvailabilityCalendarInlineFragment":false,"includeGpAvailabilityCalendarInlineFragment":true,"includePdpMigrationAvailabilityCalendarFragment":false,"includeGpAvailabilityCalendarFragment":true,"includePdpMigrationDescriptionFragment":false,"includeGpDescriptionFragment":true,"includePdpMigrationHeroFragment":false,"includeGpHeroFragment":true,"includePdpMigrationHighlightsCompactFragment":false,"includeGpHighlightsCompactFragment":true,"includePdpMigrationHighlightsFragment":false,"includeGpHighlightsFragment":true,"includePdpMigrationLocationPdpFragment":false,"includeGpLocationPdpFragment":true,"includePdpMigrationMeetYourHostFragment":false,"includeGpMeetYourHostFragment":true,"includePdpMigrationMessageBannerFragment":false,"includeGpMessageBannerFragment":true,"includePdpMigrationNavFragment":false,"includeGpNavFragment":true,"includePdpMigrationNavMobileFragment":false,"includeGpNavMobileFragment":true,"includePdpMigrationBookItFloatingFooterFragment":false,"includePdpMigrationBookItSidebarFragment":false,"includePdpMigrationBookItCalendarSheetFragment":false,"includePdpMigrationBookItNonExperiencedGuestFragment":false,"includeGpBookItNonExperiencedGuestFragment":true,"includePdpMigrationBathroomFragment":false,"includeGpBathroomFragment":true,"includePdpMigrationOverviewV2Fragment":false,"includeGpOverviewV2Fragment":true,"includePdpMigrationPropertyAvailableRoomsFragment":false,"includeGpPropertyAvailableRoomsFragment":true,"includePdpMigrationReviewsHighlightBannerFragment":false,"includeGpReviewsHighlightBannerFragment":true,"includePdpMigrationHostOverviewDefaultFragment":false,"includeGpHostOverviewDefaultFragment":true,"includePdpMigrationNonExperiencedGuestLearnMoreModalFragment":false,"includeGpNonExperiencedGuestLearnMoreModalFragment":true,"includePdpMigrationReportToAirbnbFragment":false,"includeGpReportToAirbnbFragment":true,"includePdpMigrationReviewsFragment":false,"includeGpReviewsFragment":true,"includePdpMigrationReviewsEmptyFragment":false,"includeGpReviewsEmptyFragment":true,"includePdpMigrationSeoLinksFragment":false,"includeGpSeoLinksFragment":true,"includePdpMigrationSleepingArrangementFragment":false,"includeGpSleepingArrangementFragment":true,"includePdpMigrationSleepingArrangementImagesFragment":false,"includeGpSleepingArrangementImagesFragment":true,"includePdpMigrationTitleFragment":false,"includeGpTitleFragment":true,"includeGpUgcTranslationFragment":true,"includePdpMigrationPoliciesFragment":false,"includeGpPoliciesFragment":true,"includePdpMigrationMarqueeBookItFloatingFooterFragment":false,"includeGpMarqueeBookItFloatingFooterFragment":true,"includePdpMigrationMarqueeBookItNavFragment":false,"includeGpMarqueeBookItNavFragment":true,"includePdpMigrationMarqueeBookItSidebarFragment":false,"includeGpMarqueeBookItSidebarFragment":true,"includePdpMigrationOnlyOnBookItFragment":false,"includePdpMigrationOnlyOnBookItNavFragment":false,"includePdpMigrationPdpEducationFragment":false
}`

// QuoteParams describes a price-quote request.
type QuoteParams struct {
	ListingID string
	Checkin   string
	Checkout  string
	Adults    int
}

// Quote returns the checkout price breakdown for a listing and date range via
// Airbnb's stayCheckout operation (read-only draft checkout — no reservation,
// no payment). The variables shape is the one the web checkout sends. The
// response is the checkout sections tree; the command surfaces the price rows.
func (c *Client) Quote(p QuoteParams) (json.RawMessage, error) {
	adults := p.Adults
	if adults <= 0 {
		adults = 1
	}
	vars := map[string]any{
		"input": map[string]any{
			"businessTravel":        map[string]any{},
			"checkinDate":           p.Checkin,
			"checkoutDate":          p.Checkout,
			"guestCounts":           map[string]any{"numberOfAdults": adults, "numberOfChildren": 0, "numberOfInfants": 0, "numberOfPets": 0},
			"guestCurrencyOverride": c.currency,
			"listingDetail":         map[string]any{},
			"lux":                   map[string]any{},
			"metadata":              map[string]any{"internalFlags": []string{}},
			"org":                   map[string]any{},
			"productId":             EncodeGlobalID("StayListing", NumericID(p.ListingID)),
			"addOn":                 map[string]any{"carbonOffsetParams": map[string]any{"isSelected": false}, "guestDonationParams": map[string]any{"isSelected": false}},
			"quickPayData":          nil,
		},
	}
	return c.Query("stayCheckout", vars)
}
