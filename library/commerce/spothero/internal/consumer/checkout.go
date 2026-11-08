package consumer

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/exitcode"
)

const defaultRentalSourceTitle = "web"

// BuildCheckout assembles a consumer checkout body from a fresh Craig quote and account data.
func (c *Client) BuildCheckout(in BookPlaceInput) (CheckoutRequest, RateQuote, error) {
	quote, err := c.GetFacilityRates(FacilityRateQuery{
		FacilityID: in.FacilityID,
		Starts:     in.Starts,
		Ends:       in.Ends,
		CitySlug:   in.CitySlug,
	})
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, err
	}
	if len(quote.Rates) == 0 {
		return CheckoutRequest{}, RateQuote{}, exitcode.NotFoundf("no rates for facility %d", in.FacilityID)
	}
	chosen := quote.Rates[0]
	if in.SelectRateID != "" {
		found := false
		for _, r := range quote.Rates {
			if r.RateID == in.SelectRateID || r.QuoteToken == in.SelectRateID {
				chosen = r
				found = true
				break
			}
		}
		if !found {
			return CheckoutRequest{}, RateQuote{}, exitcode.NotFoundf("rate %q not found for facility %d", in.SelectRateID, in.FacilityID)
		}
	}
	if !chosen.Available {
		return CheckoutRequest{}, RateQuote{}, exitcode.NotFoundf("%s", formatFacilityUnavailable(chosen.UnavailableReasons, in.FacilityID))
	}

	me, err := c.GetMe()
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("account profile required for checkout: %v", err)
	}
	me, err = c.EnrichCreditCards(me)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("payment cards unavailable: %v", err)
	}
	email, err := c.ResolveEmail(in.Email)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("--email required when account profile unavailable: %v", err)
	}
	phone, err := resolvePhoneNumber(me, in.PhoneNumber)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("%v", err)
	}
	cardExternalID, err := resolveCardExternalID(me, in.CardExternalID, in.CardID)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("%v", err)
	}

	rentalSource := in.RentalSourceTitle
	if rentalSource == "" {
		rentalSource = defaultRentalSourceTitle
	}

	ctx := CheckoutItemContext{
		Facility:          in.FacilityID,
		Starts:            chosen.ContextStarts,
		Ends:              chosen.ContextEnds,
		PhoneNumber:       phone,
		RentalSourceTitle: rentalSource,
		SearchID:          quote.Tracking.SearchID,
		ActionID:          quote.Tracking.ActionID,
	}
	// The site mints its own action id client side and Craig returns an empty one, so
	// an empty value here would differ from what a browser sends.
	if ctx.ActionID == "" {
		ctx.ActionID = newRandomUUID()
	}
	if ctx.Starts == "" || ctx.Ends == "" {
		params := SearchParams{CitySlug: in.CitySlug}
		startLocal, err := formatCheckoutInstant(in.Starts, params)
		if err != nil {
			return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("starts: %v", err)
		}
		endLocal, err := formatCheckoutInstant(in.Ends, params)
		if err != nil {
			return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("ends: %v", err)
		}
		ctx.Starts = startLocal
		ctx.Ends = endLocal
	}

	if in.VehicleProfileID > 0 {
		ctx.VehicleProfileID = in.VehicleProfileID
	} else if in.LicensePlateStr != "" {
		ctx.LicensePlateStr = in.LicensePlateStr
		ctx.LicensePlateState = in.LicensePlateState
	} else {
		vehicles, verr := c.ListVehicles(me.ID)
		if verr != nil && chosen.LicensePlateRequired {
			// Saying "no plate" here would send the owner hunting for the wrong problem.
			return CheckoutRequest{}, RateQuote{}, exitcode.APIf("could not read saved vehicles: %v", verr)
		}
		if vehicle, ok := pickDefaultVehicle(vehicles); ok && vehicle.ID > 0 {
			ctx.VehicleProfileID = vehicle.ID
		}
	}
	if chosen.LicensePlateRequired && ctx.VehicleProfileID == 0 && ctx.LicensePlateStr == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("license plate required; add a saved vehicle or pass --vehicle-profile-id / --license-plate")
	}

	if chosen.RateID == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.APIf("quote missing rate_id")
	}
	if chosen.QuoteToken == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.APIf("quote missing quote_token")
	}
	ctx.QuoteToken = chosen.QuoteToken
	ctx.IsEventRate = chosen.IsEventRate
	if ctx.VehicleProfileID == 0 && ctx.LicensePlateStr == "" {
		// The site sends this explicitly when no plate is chosen on a facility that
		// does not require one.
		ctx.LicensePlateUnknown = true
	}

	item := CheckoutItem{
		ItemType:    "rental",
		Price:       chosen.PriceCents,
		Key:         newRandomUUID(),
		AddOns:      []any{},
		ItemContext: ctx,
	}

	req := CheckoutRequest{
		TotalPrice: chosen.PriceCents,
		Currency:   "usd",
		Email:      email,
		Payment: CheckoutPayment{
			UseSpotHeroCredit: false,
			Cards:             []CheckoutCard{{CardExternalID: cardExternalID}},
		},
		Items: []CheckoutItem{item},
	}
	return req, chosen, nil
}

// formatFacilityUnavailable builds a user-facing unavailability message from Craig reasons.
func formatFacilityUnavailable(reasons []string, facilityID int) string {
	if len(reasons) > 0 {
		return "facility not available: " + strings.Join(reasons, ", ")
	}
	return fmt.Sprintf("facility %d not available for selected window", facilityID)
}

// ApplyBookPreviewFromRate copies quote fields into preview output.
func ApplyBookPreviewFromRate(p *BookPreview, rate RateQuote) {
	p.RateID = rate.RateID
	p.QuoteValidUntil = rate.QuoteValidUntil
	p.PriceCents = rate.PriceCents
	p.Price = rate.Price
	p.Available = rate.Available
	p.UnavailableReasons = append([]string(nil), rate.UnavailableReasons...)
	p.LicensePlateRequired = rate.LicensePlateRequired
}

func validateCheckoutItem(item CheckoutItem) error {
	if item.ItemType != "rental" {
		return fmt.Errorf("item_type must be rental")
	}
	if item.Price <= 0 || item.ItemContext.QuoteToken == "" || item.Key == "" {
		return fmt.Errorf("incomplete checkout item")
	}
	if item.ItemContext.Facility == 0 || item.ItemContext.Starts == "" || item.ItemContext.Ends == "" {
		return fmt.Errorf("incomplete item_context")
	}
	if item.ItemContext.PhoneNumber == "" {
		return fmt.Errorf("item_context.phone_number required")
	}
	if item.ItemContext.RentalSourceTitle == "" {
		return fmt.Errorf("item_context.rental_source_title required")
	}
	return nil
}
