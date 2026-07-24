// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type atomOption struct {
	ID                 string   `json:"id,omitempty"`
	ProductionID       string   `json:"production_id,omitempty"`
	Production         string   `json:"production,omitempty"`
	VenueID            string   `json:"venue_id,omitempty"`
	Venue              string   `json:"venue,omitempty"`
	Start              string   `json:"start,omitempty"`
	DistanceKM         float64  `json:"distance_km,omitempty"`
	Price              float64  `json:"price,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	Rating             string   `json:"rating,omitempty"`
	RuntimeMinutes     int      `json:"runtime_minutes,omitempty"`
	AvailableInventory int      `json:"available_inventory,omitempty"`
	Attributes         []string `json:"attributes,omitempty"`
	CheckoutURL        string   `json:"checkout_url,omitempty"`
}

type atomInventory struct {
	Options   []atomOption
	Preorders []map[string]any
}

func fetchAtomInventory(cmd *cobra.Command, flags *rootFlags, lat, lon, radius float64, start, end time.Time) (atomInventory, error) {
	c, err := flags.newClient()
	if err != nil {
		return atomInventory{}, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	venueRaw, err := c.GetWithHeadersNoCache(ctx, "/partner/v1/venue/details/byLocation", map[string]string{
		"lat": strconv.FormatFloat(lat, 'f', -1, 64), "lon": strconv.FormatFloat(lon, 'f', -1, 64),
		"radius": strconv.FormatFloat(radius, 'f', -1, 64), "pageSize": "100",
	}, nil)
	if err != nil {
		return atomInventory{}, classifyAPIError(err, flags)
	}
	venueMaps := findObjectSlice(venueRaw, "venues", "venueDetails", "items", "data")
	if len(venueMaps) == 0 {
		return atomInventory{Options: []atomOption{}}, nil
	}
	venueNames := map[string]string{}
	venueDistances := map[string]float64{}
	venueIDs := make([]string, 0, len(venueMaps))
	for _, venue := range venueMaps {
		id := anyString(venue, "id", "venueId", "venueID")
		if id == "" {
			continue
		}
		venueIDs = append(venueIDs, id)
		venueNames[id] = anyString(venue, "name", "venueName")
		venueDistances[id] = anyFloat(venue, "kmDistance", "distanceKm", "distance")
	}
	if len(venueIDs) == 0 {
		return atomInventory{Options: []atomOption{}}, nil
	}
	showRaw, _, err := c.Post(ctx, "/partner/v1/showtime/details/forVenues", map[string]any{
		"venueIds": venueIDs,
		"isoDateBounds": map[string]string{
			"isoStartDate": start.UTC().Format(time.RFC3339),
			"isoEndDate":   end.UTC().Format(time.RFC3339),
		},
		"includeProductionDetails": true,
	})
	if err != nil {
		return atomInventory{}, classifyAPIError(err, flags)
	}
	var root any
	if err := json.Unmarshal(showRaw, &root); err != nil {
		return atomInventory{}, fmt.Errorf("decode Atom showtimes: %w", err)
	}
	productions := map[string]map[string]any{}
	collectNamedObjects(root, []string{"productions", "productionDetails"}, func(item map[string]any) {
		if id := anyString(item, "id", "productionId", "productionID"); id != "" {
			productions[id] = item
		}
	})
	var options []atomOption
	seen := map[string]bool{}
	collectNamedObjects(root, []string{"showtimes", "showtimeDetails"}, func(item map[string]any) {
		id := anyString(item, "id", "showtimeId", "showtimeID")
		startText := anyString(item, "isoStartTime", "startTimeUtc", "startTime", "localStartTime")
		if id == "" || startText == "" || seen[id] {
			return
		}
		seen[id] = true
		venueID := anyString(item, "venueId", "venueID")
		productionID := anyString(item, "productionId", "productionID")
		product := productions[productionID]
		price, currency := lowestOffer(item)
		option := atomOption{
			ID: id, ProductionID: productionID, Production: anyString(product, "name", "title"),
			VenueID: venueID, Venue: venueNames[venueID], Start: startText,
			DistanceKM: venueDistances[venueID], Price: price, Currency: currency,
			Rating:             anyString(product, "advisoryRating", "rating"),
			RuntimeMinutes:     anyInt(product, "runtimeMinutes", "runtime"),
			AvailableInventory: anyInt(item, "availableInventory", "inventory"),
			Attributes:         anyStrings(item, "attributes", "attributeIds"),
			CheckoutURL:        anyString(item, "checkoutUrl", "checkoutURL"),
		}
		if option.Production == "" {
			option.Production = anyString(item, "productionName", "title")
		}
		options = append(options, option)
	})
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Start != options[j].Start {
			return options[i].Start < options[j].Start
		}
		return options[i].DistanceKM < options[j].DistanceKM
	})
	var preorders []map[string]any
	collectNamedObjects(root, []string{"preOrderDetails", "preorderDetails"}, func(item map[string]any) {
		copyItem := map[string]any{}
		for key, value := range item {
			copyItem[key] = value
		}
		productionID := anyString(item, "productionId", "productionID")
		venueID := anyString(item, "venueId", "venueID")
		copyItem["production"] = anyString(productions[productionID], "name", "title")
		copyItem["venue"] = venueNames[venueID]
		preorders = append(preorders, copyItem)
	})
	return atomInventory{Options: options, Preorders: preorders}, nil
}

func parseAtomCoordinates(latitude, longitude string) (float64, float64, error) {
	if strings.TrimSpace(latitude) == "" || strings.TrimSpace(longitude) == "" {
		return 0, 0, usageErr(fmt.Errorf("--latitude and --longitude are required"))
	}
	lat, err := strconv.ParseFloat(latitude, 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, usageErr(fmt.Errorf("--latitude must be between -90 and 90"))
	}
	lon, err := strconv.ParseFloat(longitude, 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, usageErr(fmt.Errorf("--longitude must be between -180 and 180"))
	}
	return lat, lon, nil
}

func atomTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func findObjectSlice(raw json.RawMessage, keys ...string) []map[string]any {
	var root any
	if json.Unmarshal(raw, &root) != nil {
		return nil
	}
	var out []map[string]any
	collectNamedObjects(root, keys, func(item map[string]any) { out = append(out, item) })
	return out
}

func collectNamedObjects(value any, keys []string, visit func(map[string]any)) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if containsFold(keys, key) {
				switch list := child.(type) {
				case []any:
					for _, entry := range list {
						if item, ok := entry.(map[string]any); ok {
							visit(item)
						}
					}
				}
			}
			collectNamedObjects(child, keys, visit)
		}
	case []any:
		for _, child := range current {
			collectNamedObjects(child, keys, visit)
		}
	}
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func anyString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := item[key].(type) {
		case string:
			return value
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func anyFloat(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := item[key].(type) {
		case float64:
			return value
		case string:
			parsed, _ := strconv.ParseFloat(value, 64)
			return parsed
		}
	}
	return 0
}

func anyInt(item map[string]any, keys ...string) int {
	return int(anyFloat(item, keys...))
}

func anyStrings(item map[string]any, keys ...string) []string {
	for _, key := range keys {
		if values, ok := item[key].([]any); ok {
			out := make([]string, 0, len(values))
			for _, value := range values {
				switch typed := value.(type) {
				case string:
					out = append(out, typed)
				case map[string]any:
					if name := anyString(typed, "name", "value", "id"); name != "" {
						out = append(out, name)
					}
				}
			}
			return out
		}
	}
	return nil
}

func lowestOffer(item map[string]any) (float64, string) {
	var best float64
	var currency string
	offers, _ := item["offers"].([]any)
	for _, rawOffer := range offers {
		offer, _ := rawOffer.(map[string]any)
		priceMap, _ := offer["price"].(map[string]any)
		price := anyFloat(priceMap, "amount", "value")
		if price > 0 && (best == 0 || price < best) {
			best, currency = price, anyString(priceMap, "currency", "currencyCode")
		}
	}
	return best, currency
}

func atomClockOn(day time.Time, clock string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", day.Format("2006-01-02")+" "+clock, time.Local)
}

func splitUpper(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out[strings.ToUpper(part)] = true
		}
	}
	return out
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
