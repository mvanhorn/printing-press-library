// Source RPCs delegate to Client.Post in http.go, which uses cliutil.AdaptiveLimiter
// and returns typed cliutil.RateLimitError failures; these workflows preserve them.
package traveloka

import (
	"context"
	"net/url"
	"strings"
)

// SearchHotels preserves the source popularity page and exact stay/nightly price bases.
func (c *Client) SearchHotels(ctx context.Context, q Query) (*Snapshot, error) {
	if q.Kind != "hotels" || q.GeoID == "" || q.PropertyID != "" {
		return nil, apiError("INVALID_INPUT", "SearchHotels requires kind hotels and a source geographic ID", 0, false)
	}
	if err := ValidateQuery(q); err != nil {
		return nil, err
	}
	d, err := c.TemplateData(hotelSearchPath)
	if err != nil {
		return nil, err
	}
	q, err = c.hotelSourceName(ctx, q, d, false)
	if err != nil {
		return nil, err
	}
	handoff, err := HotelURL(q)
	if err != nil {
		return nil, err
	}
	top := boundedLimit(q.Limit, 10, 50)
	applyHotelContext(d, q)
	d["sourceType"] = "HOTEL_GEO"
	d["geoId"] = q.GeoID
	d["locationName"] = q.PropertyName
	filter := map[string]any{"areaFilters": []any{}, "basicFilterIds": map[string]any{}, "rangeFilters": map[string]any{}, "brandIds": nil, "chainIds": nil}
	d["basicFilterSortSpec"] = nil
	d["criteriaFilterSortSpec"] = nil
	filter["skip"] = q.Offset
	filter["top"] = top
	filter["basicSortType"] = "POPULARITY"
	filter["ascending"] = false
	d["filterSortRequestSpec"] = filter
	contexts := sourceObject(d["contexts"])
	if contexts == nil {
		contexts = map[string]any{}
	}
	contexts["searchURL"] = handoff
	d["contexts"] = contexts
	monitor := sourceObject(d["monitoringSpec"])
	if monitor == nil {
		monitor = map[string]any{}
	}
	monitor["referrer"] = handoff
	d["monitoringSpec"] = monitor
	d["catalogContext"] = map[string]any{"catalogState": map[string]any{"displayedEntryCount": q.Offset, "displayedHotelCount": q.Offset}}
	delete(d, "highlightedHotelId")
	resp, err := c.Post(ctx, hotelSearchPath, d, q.Shopper())
	if err != nil {
		return nil, err
	}
	data, err := responseData(resp)
	if err != nil {
		return nil, err
	}
	entries, ok := data["entries"].([]any)
	if !ok {
		return nil, apiError("MALFORMED_RESPONSE", "hotel search has no entries array", 200, false)
	}
	s, err := newQuoteSnapshot(q)
	if err != nil {
		return nil, err
	}
	inventoryCount := 0
	for _, raw := range entries {
		entry := sourceObject(raw)
		if sourceString(entry["displayType"]) != "INVENTORY" || sourceString(entry["contentType"]) != "HOTEL" {
			continue
		}
		o := sourceObject(entry["data"])
		summary := sourceObject(o["hotelInventorySummary"])
		id, name := sourceString(o["id"]), firstString(o["name"], o["displayName"])
		if id == "" || name == "" || summary == nil {
			continue
		}
		inventoryCount++
		price, e := hotelFinalPrice(summary["finalPrice"], q.Currency)
		if e != nil {
			return nil, e
		}
		detailQuery := q
		detailQuery.PropertyID = id
		detailQuery.PropertyName = name
		link, e := HotelURL(detailQuery)
		if e != nil {
			return nil, e
		}
		if len(s.Offers) < top {
			s.Offers = append(s.Offers, Offer{ID: id, Kind: "hotel", PropertyID: id, PropertyName: name, BookingURL: link, Price: price, OccupancyMatch: sourceBool(summary["matchSearchOccupancy"]), Details: map[string]any{"num_charged_rooms": summary["numChargedRooms"], "price_basis": "party_stay_total", "source_price": summary["finalPrice"], "property": sourceFields(o, "displayName", "address", "region", "starRating", "latitude", "longitude", "reviewSummary", "userRating", "numReviews", "hotelFeatures", "showedFacilityTypes", "hotelFlexibilityFeatures", "hotelSeoUrl"), "inventory": sourceFields(summary, "matchSearchOccupancy", "numChargedRooms", "availableRateTypes", "hasFreeCancellationRooms", "hotelRoomCCGuaranteeRequirementDisplay")}})
		}
	}
	complete := true
	if b := sourceBool(sourceObject(data["meta"])["searchCompleted"]); b != nil {
		complete = *b
	}
	if b := sourceBool(data["searchCompleted"]); b != nil {
		complete = *b
	}
	total := sourceInt(data["numOfHotels"])
	if len(entries) == 0 && total == nil && sourceString(data["status"]) != "SUCCESS" {
		complete = false
	}
	s.SearchComplete = complete
	s.Status = "success"
	if !complete {
		s.Status = "incomplete"
		s.Warnings = append(s.Warnings, "Source hotel page did not confirm completion.")
	} else if len(s.Offers) == 0 {
		s.Status = "no_inventory"
	}
	s.Coverage = map[string]any{"source_total": data["numOfHotels"], "source_original_hotel_cards": data["numOfOriginalHotelCard"], "offset": q.Offset, "top": top, "scanned_entries": len(entries), "inventory_entries": inventoryCount, "returned": len(s.Offers), "ordering": "POPULARITY", "search_url": handoff, "scope": "source_page", "truncated": inventoryCount > top}
	advance := inventoryCount
	if n := sourceInt(data["numOfOriginalHotelCard"]); n != nil && *n >= 0 {
		advance = *n
	}
	if advance == 0 && len(entries) > 0 {
		advance = top
	}
	next := q.Offset + advance
	if total != nil && next < *total && advance > 0 {
		s.Coverage["next_offset"] = next
	} else if total == nil && inventoryCount >= top {
		s.Coverage["next_offset"] = next
	} else {
		s.Coverage["next_offset"] = nil
	}
	return s, nil
}

// HotelRooms returns each source rate's own meal, payment, occupancy and cancellation terms.
func (c *Client) HotelRooms(ctx context.Context, q Query) (*Snapshot, error) {
	if q.Kind != "rooms" || q.PropertyID == "" {
		return nil, apiError("INVALID_INPUT", "HotelRooms requires kind rooms and a source property ID", 0, false)
	}
	if err := ValidateQuery(q); err != nil {
		return nil, err
	}
	d, err := c.TemplateData(hotelRoomsPath)
	if err != nil {
		return nil, err
	}
	q, err = c.hotelSourceName(ctx, q, d, true)
	if err != nil {
		return nil, err
	}
	handoff, err := HotelURL(q)
	if err != nil {
		return nil, err
	}
	applyHotelContext(d, q)
	d["hotelId"] = q.PropertyID
	d["isReschedule"] = false
	d["prefetch"] = false
	d["preview"] = false
	delete(d, "prevSearchId")
	contexts := sourceObject(d["contexts"])
	if contexts == nil {
		contexts = map[string]any{}
	}
	contexts["hotelDetailURL"] = handoff
	contexts["bookingId"] = nil
	contexts["sourceIdentifier"] = "HOTEL_DETAIL"
	d["contexts"] = contexts
	resp, err := c.Post(ctx, hotelRoomsPath, d, q.Shopper())
	if err != nil {
		return nil, err
	}
	data, err := responseData(resp)
	if err != nil {
		return nil, err
	}
	entries, ok := data["recommendedEntries"].([]any)
	if !ok {
		return nil, apiError("MALFORMED_RESPONSE", "room search has no recommendedEntries array", 200, false)
	}
	s, err := newQuoteSnapshot(q)
	if err != nil {
		return nil, err
	}
	limit := boundedLimit(q.Limit, 20, 100)
	scanned := 0
	for _, raw := range entries {
		room := sourceObject(raw)
		roomID := sourceString(room["hotelRoomId"])
		for _, rawRate := range sourceList(room["hotelRoomInventoryList"]) {
			rate := sourceObject(rawRate)
			id := sourceString(rate["hotelRoomInventoryId"])
			if id == "" || roomID == "" {
				continue
			}
			scanned++
			price, e := hotelFinalPrice(rate["finalPrice"], q.Currency)
			if e != nil {
				return nil, e
			}
			meal := sourceLabel(rate["mealPlanDisplay"])
			if meal == "" {
				meal = sourceString(rate["displayNumBreakfastIncluded"])
			}
			match := sourceBool(rate["matchSearchOccupancy"])
			if _, exists := rate["matchSearchOccupancy"]; !exists {
				match = sourceBool(room["matchSearchOccupancy"])
			}
			cancel := sourceObject(rate["roomCancellationPolicy"])
			if cancel != nil {
				cancel = sourceFields(cancel, keysOf(cancel)...)
			}
			if cancel != nil {
				cancel["timezone_known"] = cancellationZoneKnown(cancel)
			}
			if len(s.Offers) < limit {
				s.Offers = append(s.Offers, Offer{ID: id, Kind: "room", PropertyID: q.PropertyID, PropertyName: q.PropertyName, RoomID: roomID, RoomName: sourceString(room["name"]), MealPlan: meal, Payment: sourceString(rate["rateType"]), BookingURL: handoff, Price: price, OccupancyMatch: match, Refundable: sourceBool(rate["isRefundable"]), Cancellation: cancel, Details: map[string]any{"price_basis": "party_stay_total", "source_price": rate["finalPrice"], "room": sourceFields(room, "hotelRoomId", "name", "description", "hotelBedType", "bedArrangements", "hotelRoomSizeDisplay", "baseOccupancy", "displayBaseOccupancy", "matchSearchOccupancy", "numChargedRooms", "amenitiesByCategory", "amenitiesIncluded"), "rate": sourceFields(rate, "hotelRoomInventoryId", "inventoryGroupId", "inventoryName", "maxOccupancy", "maxChildOccupancy", "isBookable", "numRemainingRooms", "numBreakfastIncluded", "displayNumBreakfastIncluded", "isBreakfastIncluded", "mealPlanDisplay", "includedMealTypes", "extraBedIsIncluded", "rateType", "isRefundable", "matchSearchOccupancy", "numChargedRooms", "hotelRoomCCGuaranteeRequirementDisplay", "paymentDisplay", "paymentInfo", "creditCardGuarantee", "ccGuaranteeOptions", "ccGuaranteeRequirement", "bookingPolicy", "flexiPaySummary", "roomReschedulePolicy", "childOccupancyPolicyDisplay", "extraChargeDisplay", "cancellationPolicyDisplay", "originalCancellationPolicy", "roomInventoryGroupCancellationPolicy", "hotelRoomTaxOrFeeBreakdownDisplays", "rateDisplay", "inventoryLabelDisplay")}})
			}
		}
	}
	complete := sourceString(data["status"]) == "SUCCESS"
	if b := sourceBool(data["searchCompleted"]); b != nil {
		complete = *b
	}
	s.SearchComplete = complete
	s.Status = "success"
	if !complete {
		s.Status = "incomplete"
		s.Warnings = append(s.Warnings, "Source room response did not confirm successful completion.")
	} else if len(s.Offers) == 0 {
		s.Status = "no_inventory"
	}
	s.Coverage = map[string]any{"source_room_entries": len(entries), "scanned_rate_plans": scanned, "limit": limit, "returned": len(s.Offers), "truncated": scanned > limit, "ordering": "source_recommended_entries", "property_url": handoff}
	return s, nil
}

func applyHotelContext(d map[string]any, q Query) {
	d["checkInDate"] = dateObject(q.CheckIn)
	d["checkOutDate"] = dateObject(q.CheckOut)
	d["numOfNights"] = queryNights(q)
	d["numAdults"] = q.Adults
	d["numChildren"] = q.Children
	d["numInfants"] = q.Infants
	d["numRooms"] = q.Rooms
	d["childAges"] = append([]int{}, q.ChildAges...)
	d["currency"] = q.Currency
	d["rateTypes"] = []string{"PAY_NOW", "PAY_AT_PROPERTY"}
	d["ccGuaranteeOptions"] = map[string]any{"ccGuaranteeRequirementOptions": []string{"CC_GUARANTEE"}, "ccInfoPreferences": []string{"CC_TOKEN", "CC_FULL_INFO"}}
	d["isJustLogin"] = false
}
func hotelFinalPrice(v any, currency string) (Price, error) {
	p := Price{TaxInclusion: "unknown"}
	o := sourceObject(v)
	if o == nil {
		if v != nil {
			return p, apiError("MALFORMED_RESPONSE", "source finalPrice must be an object", 200, false)
		}
		return p, nil
	}
	stay, night := sourceObject(o["totalPriceRateDisplay"]), sourceObject(o["perRoomPerNightDisplay"])
	var err error
	if p.Total, p.TaxInclusion, err = hotelDisplayPrice(stay, currency); err != nil {
		return p, err
	}
	if p.PerRoomPerNight, _, err = hotelDisplayPrice(night, currency); err != nil {
		return p, err
	}
	if p.BaseFare, err = checkedMoney(stay["baseFare"], stay["numOfDecimalPoint"], currency); err != nil {
		return p, err
	}
	if p.Taxes, err = checkedMoney(stay["taxes"], stay["numOfDecimalPoint"], currency); err != nil {
		return p, err
	}
	if p.Fees, err = checkedMoney(stay["fees"], stay["numOfDecimalPoint"], currency); err != nil {
		return p, err
	}
	return p, nil
}

// Empty source money objects are unknown and do not block another exposed price.
func hotelDisplayPrice(display map[string]any, currency string) (*Money, string, error) {
	for _, field := range []string{"inclusiveFinalPrice", "totalFare", "exclusiveFinalPrice"} {
		money, err := checkedMoney(display[field], display["numOfDecimalPoint"], currency)
		if err != nil {
			return nil, "unknown", err
		}
		if money == nil {
			continue
		}
		inclusion := "unknown"
		if field == "inclusiveFinalPrice" {
			inclusion = "inclusive"
		} else if field == "exclusiveFinalPrice" {
			inclusion = "exclusive"
		}
		return money, inclusion, nil
	}
	return nil, "unknown", nil
}

func sourceLabel(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	o := sourceObject(v)
	return firstString(o["displayMealPlanIncluded"], o["label"], o["text"], o["display"], o["name"], o["title"])
}
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
func cancellationZoneKnown(m map[string]any) bool {
	for _, k := range []string{"timezone", "timeZone", "timezoneId", "utcOffset"} {
		if v, ok := m[k]; ok && v != nil && sourceString(v) != "" {
			return true
		}
	}
	return false
}

func (c *Client) hotelSourceName(ctx context.Context, q Query, template map[string]any, property bool) (Query, error) {
	if q.PropertyName != "" {
		return q, nil
	}
	id, typ, urlKey := q.GeoID, "HOTEL_GEO", "searchURL"
	if property {
		id, typ, urlKey = q.PropertyID, "HOTEL", "hotelDetailURL"
	}
	if name := capturedHotelName(sourceString(sourceObject(template["contexts"])[urlKey]), typ, id); name != "" {
		q.PropertyName = name
		return q, nil
	}
	queries := []string{}
	if q.Destination != "" {
		queries = append(queries, q.Destination)
	}
	if t, e := c.TemplateData(hotelAutocompletePath); e == nil {
		if v := sourceString(t["query"]); v != "" {
			queries = append(queries, v)
		}
	}
	queries = append(queries, id)
	seen := map[string]bool{}
	for _, query := range queries {
		if seen[query] {
			continue
		}
		seen[query] = true
		d, e := c.TemplateData(hotelAutocompletePath)
		if e != nil {
			return q, e
		}
		d["query"] = query
		d["experimentContext"] = map[string]any{"mapParamKeyToVariant": map[string]any{"varAutocompleteLogic": "control"}}
		resp, e := c.Post(ctx, hotelAutocompletePath, d, q.Shopper())
		if e != nil {
			return q, e
		}
		data, e := responseData(resp)
		if e != nil {
			return q, e
		}
		for _, group := range []string{"autoCompleteContent", "geoRegionContent", "geoCityContent", "geoAreaContent", "landmarkContent", "hotelContent"} {
			for _, raw := range sourceList(sourceObject(data[group])["rows"]) {
				row := sourceObject(raw)
				if sourceString(row["id"]) != id {
					continue
				}
				isProperty := sourceString(row["type"]) == "HOTEL"
				if isProperty != property {
					continue
				}
				name := firstString(row["displayName"], row["name"])
				if name != "" {
					q.PropertyName = name
					return q, nil
				}
			}
		}
	}
	return q, apiError("MALFORMED_RESPONSE", "source destination/property name could not be resolved for this ID; use resolve and supply its exact name", 200, false)
}
func capturedHotelName(raw, typ, id string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "www.traveloka.com" {
		return ""
	}
	parts := strings.Split(u.Query().Get("spec"), ".")
	if len(parts) < 8 || parts[4] != typ || parts[5] != id {
		return ""
	}
	return strings.Join(parts[6:len(parts)-1], ".")
}
