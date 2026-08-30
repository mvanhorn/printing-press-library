// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live
// pp:client-call

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type lodgingSearchOptions struct {
	request                 string
	rawResponse             bool
	limit                   int
	geoID                   int
	bounds                  []float64
	startDate               string
	endDate                 string
	adultCount              int
	roomCount               int
	childrenAges            []int
	sortBy                  string
	propertyName            string
	hotelOrVacationRental   string
	minGuestRating          float64
	hotelClasses            []int
	lodgingTypes            []string
	accommodationTypes      []string
	amenities               []string
	vacationRentalAmenities []string
	minBedsInRoom           int
	sources                 []string
	tripPlanKey             string
	referer                 string
	anonymousID             string
}

func newLodgingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lodging",
		Short:       "Search Wanderlog lodging and hotel candidates",
		Hidden:      true,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLodgingSearchCmd(flags))
	return cmd
}

func newLodgingSearchCmd(flags *rootFlags) *cobra.Command {
	opts := lodgingSearchOptions{
		adultCount:            2,
		roomCount:             1,
		limit:                 25,
		sortBy:                "ratings",
		hotelOrVacationRental: "both",
		sources:               []string{"airbnb", "expedia", "google", "kayak"},
	}
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Wanderlog lodgings using the itinerary Lodging button endpoint",
		Example: "  wanderlog-pp-cli lodging search --geo-id 50 --bounds 127.63045,26.17561,127.73895,26.24614 --start-date 2026-08-30 --end-date 2026-09-06 --adult-count 2 --room-count 1 --min-guest-rating 8 --agent",
		Annotations: map[string]string{
			"pp:endpoint":   "lodging.search",
			"pp:method":     "POST",
			"pp:path":       "/api/lodging/searchLodgings",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			body, err := buildLodgingSearchRequest(cmd, opts)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			headers := lodgingSearchHeaders(c.RequestBaseURL(), opts)
			data, _, err := c.PostQueryWithParamsAndHeaders(cmd.Context(), "/api/lodging/searchLodgings", nil, body, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !opts.rawResponse {
				var raw map[string]any
				if err := json.Unmarshal(data, &raw); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), summarizeLodgingSearchResponse(raw, opts.limit), flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&opts.request, "request", "", "Exact JSON request body; overrides individual search flags")
	cmd.Flags().BoolVar(&opts.rawResponse, "raw-response", false, "Return the full Wanderlog lodging search response instead of compact candidate summaries")
	cmd.Flags().IntVar(&opts.limit, "limit", 25, "Maximum summarized offers to return; 0 means all")
	cmd.Flags().IntVar(&opts.geoID, "geo-id", 0, "Wanderlog geo id, e.g. 50 for Okinawa")
	cmd.Flags().Float64SliceVar(&opts.bounds, "bounds", nil, "Search bounds west,south,east,north")
	cmd.Flags().StringVar(&opts.startDate, "start-date", "", "Check-in/start date YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.endDate, "end-date", "", "Check-out/end date YYYY-MM-DD")
	cmd.Flags().IntVar(&opts.adultCount, "adult-count", 2, "Adult guest count")
	cmd.Flags().IntVar(&opts.roomCount, "room-count", 1, "Room count")
	cmd.Flags().IntSliceVar(&opts.childrenAges, "children-age", nil, "Child age; repeatable or comma-separated")
	cmd.Flags().StringVar(&opts.sortBy, "sort-by", "ratings", "Sort order, e.g. ratings")
	cmd.Flags().StringVar(&opts.propertyName, "property-name", "", "Property name filter")
	cmd.Flags().StringVar(&opts.hotelOrVacationRental, "hotel-or-vacation-rental", "both", "Filter: hotel, vacationRental, or both")
	cmd.Flags().Float64Var(&opts.minGuestRating, "min-guest-rating", 0, "Minimum guest rating; omitted when 0")
	cmd.Flags().IntSliceVar(&opts.hotelClasses, "hotel-class", nil, "Hotel star class; repeatable or comma-separated")
	cmd.Flags().StringSliceVar(&opts.lodgingTypes, "lodging-type", nil, "Lodging type filter; repeatable or comma-separated")
	cmd.Flags().StringSliceVar(&opts.accommodationTypes, "accommodation-type", nil, "Accommodation type filter; repeatable or comma-separated")
	cmd.Flags().StringSliceVar(&opts.amenities, "amenity", nil, "Hotel amenity filter; repeatable or comma-separated")
	cmd.Flags().StringSliceVar(&opts.vacationRentalAmenities, "vacation-rental-amenity", nil, "Vacation rental amenity filter; repeatable or comma-separated")
	cmd.Flags().IntVar(&opts.minBedsInRoom, "min-beds-in-room", 0, "Minimum beds in room; omitted when 0")
	cmd.Flags().StringSliceVar(&opts.sources, "sources", opts.sources, "Search sources; comma-separated, defaults to airbnb,expedia,google,kayak")
	cmd.Flags().StringVar(&opts.tripPlanKey, "trip-plan-key", "", "Optional trip plan key for browser-like Referer context")
	cmd.Flags().StringVar(&opts.referer, "referer", "", "Optional explicit Referer header")
	cmd.Flags().StringVar(&opts.anonymousID, "anonymous-id", "", "Optional X-WL-Anonymous-ID header")
	return cmd
}

func buildLodgingSearchRequest(cmd *cobra.Command, opts lodgingSearchOptions) (any, error) {
	if strings.TrimSpace(opts.request) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(opts.request), &parsed); err != nil {
			return nil, fmt.Errorf("parse --request: %w", err)
		}
		return parsed, nil
	}
	if opts.geoID == 0 {
		return nil, errors.New("--geo-id is required unless --request is supplied")
	}
	if len(opts.bounds) != 4 {
		return nil, errors.New("--bounds must contain four numbers: west,south,east,north")
	}
	if err := validateDateFlag("--start-date", opts.startDate); err != nil {
		return nil, err
	}
	if err := validateDateFlag("--end-date", opts.endDate); err != nil {
		return nil, err
	}
	if opts.adultCount <= 0 {
		return nil, errors.New("--adult-count must be positive")
	}
	if opts.roomCount <= 0 {
		return nil, errors.New("--room-count must be positive")
	}
	if opts.hotelOrVacationRental == "" {
		opts.hotelOrVacationRental = "both"
	}
	if len(opts.sources) == 0 {
		opts.sources = []string{"airbnb", "expedia", "google", "kayak"}
	}
	childrenAges := opts.childrenAges
	if childrenAges == nil {
		childrenAges = []int{}
	}

	filters := map[string]any{
		"priceRange":            nil,
		"hotelClasses":          nil,
		"minGuestRating":        nil,
		"propertyTypes":         map[string]any{"lodgingTypes": nil, "accommodationTypes": nil},
		"hotelOrVacationRental": opts.hotelOrVacationRental,
		"amenities":             nil,
		"minBedsInRoom":         nil,
		"propertyName":          opts.propertyName,
		"vacationRentalFilters": map[string]any{"amenities": []any{}},
	}
	if cmd.Flags().Changed("min-guest-rating") {
		filters["minGuestRating"] = opts.minGuestRating
	}
	if len(opts.hotelClasses) > 0 {
		filters["hotelClasses"] = opts.hotelClasses
	}
	propertyTypes := filters["propertyTypes"].(map[string]any)
	if len(opts.lodgingTypes) > 0 {
		propertyTypes["lodgingTypes"] = opts.lodgingTypes
	}
	if len(opts.accommodationTypes) > 0 {
		propertyTypes["accommodationTypes"] = opts.accommodationTypes
	}
	if len(opts.amenities) > 0 {
		filters["amenities"] = opts.amenities
	}
	if cmd.Flags().Changed("min-beds-in-room") {
		filters["minBedsInRoom"] = opts.minBedsInRoom
	}
	if len(opts.vacationRentalAmenities) > 0 {
		filters["vacationRentalFilters"] = map[string]any{"amenities": opts.vacationRentalAmenities}
	}

	return map[string]any{
		"geoId":  opts.geoID,
		"bounds": opts.bounds,
		"dates": map[string]any{
			"startDate": opts.startDate,
			"endDate":   opts.endDate,
		},
		"guests": map[string]any{
			"adultCount":   opts.adultCount,
			"roomCount":    opts.roomCount,
			"childrenAges": childrenAges,
		},
		"sortBy":  opts.sortBy,
		"filters": filters,
		"sources": opts.sources,
	}, nil
}

func validateDateFlag(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required unless --request is supplied", name)
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("%s must be YYYY-MM-DD: %w", name, err)
	}
	return nil
}

func lodgingSearchHeaders(baseURL string, opts lodgingSearchOptions) map[string]string {
	headers := map[string]string{
		"Accept": "application/json, text/plain, */*",
		"Origin": strings.TrimRight(baseURL, "/"),
	}
	if opts.anonymousID != "" {
		headers["X-WL-Anonymous-ID"] = opts.anonymousID
	}
	if opts.referer != "" {
		headers["Referer"] = opts.referer
		return headers
	}
	if opts.tripPlanKey != "" && opts.geoID != 0 && len(opts.bounds) == 4 && opts.startDate != "" && opts.endDate != "" {
		headers["Referer"] = fmt.Sprintf("%s/hotels/search?geoId=%d&adultCount=%d&roomCount=%d&startDate=%s&endDate=%s&propertyName=%s&bounds=%g%%2C%g%%2C%g%%2C%g&sources=%s&tripPlanKey=%s",
			strings.TrimRight(baseURL, "/"),
			opts.geoID,
			opts.adultCount,
			opts.roomCount,
			opts.startDate,
			opts.endDate,
			opts.propertyName,
			opts.bounds[0], opts.bounds[1], opts.bounds[2], opts.bounds[3],
			strings.Join(opts.sources, "%2C"),
			opts.tripPlanKey,
		)
	}
	return headers
}

func summarizeLodgingSearchResponse(raw map[string]any, limit int) map[string]any {
	data := mapField(raw, "data")
	offers, _ := data["offers"].([]any)
	outOffers := []any{}
	for i, rawOffer := range offers {
		if limit > 0 && i >= limit {
			break
		}
		offer, _ := rawOffer.(map[string]any)
		if offer == nil {
			continue
		}
		outOffers = append(outOffers, summarizeLodgingOfferForSearch(offer))
	}
	return map[string]any{
		"success":          raw["success"],
		"is_complete":      data["isComplete"],
		"offer_count":      len(offers),
		"returned_count":   len(outOffers),
		"offers":           outOffers,
		"unfiltered_stats": data["unfilteredStats"],
		"nearby_geos":      data["nearbyGeos"],
	}
}

func summarizeLodgingOfferForSearch(offer map[string]any) map[string]any {
	lodging := mapField(offer, "lodging")
	priceRate := mapField(offer, "priceRate")
	rating := mapField(lodging, "rating")
	location := mapField(lodging, "location")
	out := map[string]any{
		"offer_id":         stringField(offer, "offerId"),
		"source":           firstNonEmpty(stringField(offer, "source"), stringField(priceRate, "source")),
		"lodging_id":       mapField(lodging, "id"),
		"name":             stringField(lodging, "name"),
		"wanderlog_rating": lodging["wanderlogRating"],
		"rating_count":     lodging["ratingCount"],
	}
	if location != nil {
		out["lat"] = firstNonZeroFloat(floatAny(location["latitude"]), floatAny(location["lat"]))
		out["lng"] = firstNonZeroFloat(floatAny(location["longitude"]), floatAny(location["lng"]))
	}
	if rating != nil {
		out["rating_source"] = stringField(rating, "source")
		out["rating"] = rating["value"]
	}
	if priceRate != nil {
		out["site"] = stringField(priceRate, "site")
		out["nightly_amount"] = priceRate["amount"]
		out["currency"] = stringField(priceRate, "currencyCode")
		out["total"] = mapField(priceRate, "total")
		out["frequency"] = stringField(priceRate, "frequency")
		out["booking_url"] = stringField(priceRate, "bookingUrl")
		out["free_cancellation"] = priceRate["hasFreeCancellation"]
	}
	if photos := lodgingOfferPhotoURLs(lodging); len(photos) > 0 {
		out["image_url"] = photos[0]
	}
	return out
}
