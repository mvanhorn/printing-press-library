// Read actual dated inventory, retaining whole-stay per-room quote evidence.
// pp:data-source live
// pp:client-call publicTravelOptions.client constructs travel.NewClient; API.Offers performs the real public HTTP read.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
)

type publicOfferHit struct {
	HotelID            string            `json:"hotel_id"`
	PlanID             string            `json:"plan_id"`
	RoomID             string            `json:"room_id"`
	PlanName           *string           `json:"plan_name"`
	RoomName           *string           `json:"room_name"`
	Price              travel.Price      `json:"price"`
	Meals              travel.Meals      `json:"meals"`
	CancellationPolicy *travel.Policy    `json:"plan_cancellation_policy"`
	SourceCaveat       *string           `json:"source_caveat"`
	SourceURL          string            `json:"source_url"`
	BookingURL         string            `json:"booking_url"`
	RoomAnchor         string            `json:"room_anchor"`
	Query              travel.OfferQuery `json:"query"`
}

func compactOffer(offer travel.Offer) publicOfferHit {
	return publicOfferHit{offer.HotelID, offer.PlanID, offer.RoomID, offer.PlanName, offer.RoomName, offer.Price, offer.Meals, offer.CancellationPolicy, offer.SourceCaveat, offer.SourceURL, offer.BookingURL, offer.RoomAnchor, offer.Query}
}

func bindTravelParty(cmd *cobra.Command, query *travel.OfferQuery) {
	cmd.Flags().IntVar(&query.Rooms, "rooms", 1, "Required: number of rooms with the same party in every room")
	cmd.Flags().IntVar(&query.AdultsPerRoom, "adults-per-room", 2, "Required: adults in each room (at least one)")
	cmd.Flags().IntVar(&query.Children.Upper, "child-upper", 0, "Upper elementary school children in each room")
	cmd.Flags().IntVar(&query.Children.Lower, "child-lower", 0, "Lower elementary school children in each room")
	cmd.Flags().IntVar(&query.Children.InfantMealBed, "infant-meal-bed", 0, "Infants with meals and bedding in each room")
	cmd.Flags().IntVar(&query.Children.InfantMeal, "infant-meal", 0, "Infants with meals only in each room")
	cmd.Flags().IntVar(&query.Children.InfantBed, "infant-bed", 0, "Infants with bedding only in each room")
	cmd.Flags().IntVar(&query.Children.InfantNone, "infant-none", 0, "Infants without meals or bedding in each room")
}

func bindTravelOfferQuery(cmd *cobra.Command, query *travel.OfferQuery, search bool) {
	cmd.Flags().StringVar(&query.HotelID, "hotel", "", "Required: numeric property ID")
	cmd.Flags().StringVar(&query.Checkin, "checkin", "", "Required: check-in date YYYY-MM-DD in Japan time")
	cmd.Flags().StringVar(&query.Checkout, "checkout", "", "Required: later check-out date YYYY-MM-DD in Japan time")
	cmd.Flags().IntVar(&query.Page, "page", 1, "Source plan page to inspect")
	if search {
		cmd.Flags().IntVar(&query.Offset, "offset", 0, "Skip this many room-offer rows within the source plan page")
		cmd.Flags().IntVar(&query.Limit, "limit", 5, "Maximum emitted room offers (1–100); source plans may have multiple rooms")
	} else {
		query.Limit = 100
	}
}

func validateTravelOfferQuery(cmd *cobra.Command, query travel.OfferQuery) error {
	if err := requireTravelFlags(cmd, "hotel", "checkin", "checkout", "rooms", "adults-per-room"); err != nil {
		return err
	}
	if err := validateTravelWindow(query.Page, query.Offset, query.Limit); err != nil {
		return err
	}
	if err := query.Validate(); err != nil {
		return usageErr(err)
	}
	return nil
}

func newNovelOffersSearchCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var query travel.OfferQuery
	cmd := &cobra.Command{
		Use: "search", Short: "Read bookable room/plan tuples for explicit dates and a uniform per-room party",
		Example:     "  rakuten-travel-pp-cli offers search --hotel 51870 --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2 --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--hotel=51870;--checkin=2026-11-08;--checkout=2026-11-10;--rooms=1;--adults-per-room=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := validateTravelOfferQuery(cmd, query); err != nil {
				return err
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result, err := api.Offers(ctx, query)
			if err != nil {
				return publicTravelError(err)
			}
			offers := make([]publicOfferHit, 0, len(result.Offers))
			for _, offer := range result.Offers {
				offers = append(offers, compactOffer(offer))
			}
			return options.output(cmd, flags, publicTravelMeta(result.Status, result.Source, &result.Page, result.Query, api), offers)
		},
	}
	bindTravelOfferQuery(cmd, &query, true)
	bindTravelParty(cmd, &query)
	options.bind(cmd)
	return cmd
}
