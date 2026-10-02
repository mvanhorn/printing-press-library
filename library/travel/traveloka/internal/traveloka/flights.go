// Source RPCs delegate to Client.Post in http.go, which uses cliutil.AdaptiveLimiter
// and returns typed cliutil.RateLimitError failures; these workflows preserve them.
package traveloka

import (
	"context"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"math/big"
	"sort"
	"strings"
	"time"
)

type flightInventory struct {
	rows     []map[string]any
	complete bool
	polls    int
	meta     map[string]any
	bounded  bool
}

// SearchFlights retrieves bounded candidates and source-prefetched authoritative trip prices.
func (c *Client) SearchFlights(ctx context.Context, q Query) (*Snapshot, error) {
	if q.Kind != "flights" {
		return nil, apiError("INVALID_INPUT", "SearchFlights requires kind flights", 0, false)
	}
	if err := ValidateQuery(q); err != nil {
		return nil, err
	}
	s, err := newQuoteSnapshot(q)
	if err != nil {
		return nil, err
	}
	sid, err := freshID()
	if err != nil {
		return nil, err
	}
	handoff, err := FlightURL(q)
	if err != nil {
		return nil, err
	}
	limit := boundedLimit(q.Limit, 10, 50)
	candidateCap := boundedLimit(q.MaxCandidates, 3, 10)
	d, err := c.TemplateData(flightInitialPath)
	if err != nil {
		return nil, err
	}
	journeys := []any{map[string]any{"originCode": q.Origin, "destinationCode": q.Destination, "departureDate": q.Depart}}
	tripType, sourceType, pricing := "ONE_WAY", "ONE_WAY", "INDEPENDENT"
	if q.ReturnDate != "" {
		journeys = append(journeys, map[string]any{"originCode": q.Destination, "destinationCode": q.Origin, "departureDate": q.ReturnDate})
		tripType, sourceType, pricing = "ROUND_TRIP", "ROUNDTRIP", "COMBINED"
	}
	d["journeys"] = journeys
	d["tripType"] = tripType
	d["journeyIndex"] = 0
	d["selectedFlights"] = []any{}
	d["selectedFlightsContext"] = map[string]any{}
	d["sharedFlights"] = []any{}
	d["searchId"] = sid
	d["seatPublishedClass"] = q.Cabin
	d["numSeats"] = map[string]any{"numAdults": q.Adults, "numChildren": q.Children, "numInfants": q.Infants}
	d["currency"] = q.Currency
	d["inventoryPricingDisplayType"] = pricing
	additional := sourceObject(d["additionalData"])
	if additional == nil {
		additional = map[string]any{}
	}
	additional["searchSource"] = sourceType
	additional["isBaggageFilterEnabled"] = false
	additional["isBreakSmartCombo"] = false
	additional["prefetchFlag"] = false
	d["additionalData"] = additional
	d["trackingMap"] = map[string]any{"originalUrl": handoff, "redirectedUrl": nil}
	d["filter"] = map[string]any{"standAlone": true}
	bounded, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	out, err := c.collectFlights(bounded, ctx, d, q.Shopper(), true)
	if err != nil {
		return nil, err
	}
	if err = rankFlightCandidates(out.rows, q.Currency); err != nil {
		return nil, err
	}
	s.Coverage = map[string]any{"search_id": sid, "outbound_observed": len(out.rows), "max_outbound_candidates": candidateCap, "output_limit": limit, "candidate_order": "source_display_price", "source_outbound_meta": sourceFields(out.meta, "searchCompleted", "expiryTimeStamp", "defaultSortType", "availableSortTypes", "priceDisplayType"), "polls": out.polls, "prefetch_failed": 0, "scope": "bounded_candidates"}
	retained := out.rows
	if len(retained) > candidateCap {
		retained = retained[:candidateCap]
	}
	s.Coverage["outbound_retained"] = len(retained)
	s.Coverage["outbound_candidates_truncated"] = len(out.rows) > len(retained)
	complete := out.complete && !out.bounded
	attempted, failed := 0, 0
	evaluated := 0
	outputTruncated := false
	failures := []any{}
	returnCoverage := []any{}
	var firstFailure error
	quote := func(rows ...map[string]any) error {
		attempted++
		offer, e := c.prefetchFlight(bounded, q, sid, handoff, rows)
		if e != nil {
			var throttled *cliutil.RateLimitError
			if errors.As(e, &throttled) {
				return throttled
			}
			var ae *APIError
			if errors.As(e, &ae) && ae.Code == "CURRENCY_MISMATCH" {
				return e
			}
			failed++
			if firstFailure == nil {
				firstFailure = e
			}
			failures = append(failures, map[string]any{"journey_ids": flightIDs(rows), "message": e.Error()})
			return nil
		}
		s.Offers = append(s.Offers, offer)
		return nil
	}
	for _, outbound := range retained {
		if len(s.Offers) >= limit {
			outputTruncated = true
			break
		}
		evaluated++
		if q.ReturnDate == "" {
			if err = quote(outbound); err != nil {
				return nil, err
			}
			continue
		}
		inData, e := copyMap(d)
		if e != nil {
			return nil, e
		}
		id := sourceString(outbound["id"])
		inData["journeyIndex"] = 1
		inData["selectedFlights"] = []any{id}
		inData["selectedFlightsContext"] = map[string]any{id: map[string]any{"isBaggageFilterEnabled": false}}
		in, e := c.collectFlights(bounded, ctx, inData, q.Shopper(), false)
		if e != nil {
			return nil, e
		}
		if e = rankFlightCandidates(in.rows, q.Currency); e != nil {
			return nil, e
		}
		complete = complete && in.complete && !in.bounded
		returnCoverage = append(returnCoverage, map[string]any{"outbound_id": id, "observed": len(in.rows), "search_complete": in.complete, "polls": in.polls, "source_meta": sourceFields(in.meta, "expiryTimeStamp", "defaultSortType", "priceDisplayType")})
		for _, inbound := range in.rows {
			if len(s.Offers) >= limit || attempted >= 50 {
				outputTruncated = true
				break
			}
			if err = quote(outbound, inbound); err != nil {
				return nil, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, sourceHTTPError(ctx, err, 0, "flight retrieval canceled before completion", c.secrets)
	}
	sourceComplete := complete
	if len(s.Offers) == 0 && evaluated < len(out.rows) {
		complete = false
		s.Warnings = append(s.Warnings, "No offers were found among the evaluated outbound candidates; unsearched candidates may have valid return flights.")
	}
	s.SearchComplete = complete
	s.Coverage["returns"] = returnCoverage
	s.Coverage["prefetch_attempted"] = attempted
	s.Coverage["prefetch_failed"] = failed
	s.Coverage["prefetch_failures"] = failures
	s.Coverage["returned"] = len(s.Offers)
	s.Coverage["outbound_evaluated"] = evaluated
	s.Coverage["prefetch_cap"] = 50
	s.Coverage["output_truncated"] = outputTruncated
	s.RetrievedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if failed > 0 {
		s.Warnings = append(s.Warnings, "Some source prefetches failed; coverage.prefetch_failures records the omitted candidates.")
	}
	if !sourceComplete {
		s.Warnings = append(s.Warnings, "Source search did not complete within the bounded polling window.")
	}
	if len(s.Offers) == 0 && firstFailure != nil {
		return nil, firstFailure
	}
	s.Status = "success"
	if !complete {
		s.Status = "incomplete"
	} else if len(s.Offers) == 0 {
		s.Status = "no_inventory"
	}
	return s, nil
}

func (c *Client) collectFlights(ctx, caller context.Context, data map[string]any, shop Shopper, initial bool) (flightInventory, error) {
	r := flightInventory{rows: []map[string]any{}, meta: map[string]any{}}
	index := map[string]int{}
	path := flightPollPath
	if initial {
		path = flightInitialPath
	}
	for call := 0; call <= 12; call++ {
		if call > 0 {
			if e := waitSourceRefresh(ctx, r.meta); e != nil {
				if callerErr := caller.Err(); callerErr != nil {
					return r, sourceHTTPError(caller, callerErr, 0, "flight search canceled", c.secrets)
				}
				r.bounded = true
				return r, nil
			}
			r.polls++
			path = flightPollPath
		}
		resp, e := c.Post(ctx, path, data, shop)
		if e != nil {
			if ctx.Err() != nil && caller.Err() == nil {
				r.bounded = true
				return r, nil
			}
			return r, e
		}
		d, e := responseData(resp)
		if e != nil {
			return r, e
		}
		results, ok := d["searchResults"].([]any)
		if !ok {
			return r, apiError("MALFORMED_RESPONSE", "flight search has no incremental searchResults array", 200, false)
		}
		for _, raw := range results {
			row := sourceObject(raw)
			id := sourceString(row["id"])
			if id == "" {
				return r, apiError("MALFORMED_RESPONSE", "flight candidate has no source ID", 200, false)
			}
			if i, ok := index[id]; ok {
				r.rows[i] = row
			} else {
				if len(r.rows) >= 5000 {
					r.bounded = true
					continue
				}
				index[id] = len(r.rows)
				r.rows = append(r.rows, row)
			}
		}
		r.meta = sourceObject(d["meta"])
		if r.meta == nil {
			r.meta = sourceObject(resp["meta"])
		}
		if r.meta == nil {
			r.meta = map[string]any{}
		}
		if responseID := sourceString(r.meta["searchId"]); responseID != "" && responseID != sourceString(data["searchId"]) {
			return r, apiError("MALFORMED_RESPONSE", "source returned a different flight search ID", 200, false)
		}
		if b := sourceBool(r.meta["searchCompleted"]); b != nil && *b {
			r.complete = true
			return r, nil
		}
	}
	r.bounded = true
	return r, nil
}

func flightIDs(rows []map[string]any) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = sourceString(r["id"])
	}
	return ids
}

func rankFlightCandidates(rows []map[string]any, currency string) error {
	prices := map[string]*big.Rat{}
	for _, r := range rows {
		meta, fare := sourceObject(r["flightMetadata"]), sourceObject(r["fare"])
		v := meta["totalCombinedPrice"]
		if v == nil {
			v = fare["display"]
		}
		m, e := displayMoney(v, currency)
		if e != nil {
			return e
		}
		if _, e = displayMoney(fare["display"], currency); e != nil {
			return e
		}
		if m != nil && m.Decimals != nil {
			n, _ := new(big.Int).SetString(m.MinorUnits, 10)
			den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(*m.Decimals)), nil)
			prices[sourceString(r["id"])] = new(big.Rat).SetFrac(n, den)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := prices[sourceString(rows[i]["id"])], prices[sourceString(rows[j]["id"])]
		if a == nil {
			return false
		}
		return b == nil || a.Cmp(b) < 0
	})
	return nil
}

func (c *Client) prefetchFlight(ctx context.Context, q Query, sid, handoff string, rows []map[string]any) (Offer, error) {
	d, e := c.TemplateData(flightPrefetchPath)
	if e != nil {
		return Offer{}, e
	}
	ids := flightIDs(rows)
	contexts := map[string]any{}
	for _, id := range ids {
		contexts[id] = map[string]any{"isBaggageFilterEnabled": false}
	}
	d["journeyIds"] = ids
	d["journeyContextData"] = contexts
	d["searchId"] = sid
	d["isPrefetch"] = true
	d["isBreakSmartCombo"] = false
	delete(d, "prefetchCacheKey")
	resp, e := c.Post(ctx, flightPrefetchPath, d, q.Shopper())
	if e != nil {
		return Offer{}, e
	}
	data, e := responseData(resp)
	if e != nil {
		return Offer{}, e
	}
	total, e := displayMoney(data["totalPrice"], q.Currency)
	if e != nil {
		return Offer{}, e
	}
	if total == nil {
		return Offer{}, apiError("MALFORMED_RESPONSE", "source flight prefetch has no authoritative totalPrice", 200, false)
	}
	per, e := displayMoney(data["displayedPricePerPax"], q.Currency)
	if e != nil {
		return Offer{}, e
	}
	o := Offer{ID: strings.Join(ids, "+"), Kind: "flight", BookingURL: handoff, Price: Price{Total: total, PerPassenger: per, TaxInclusion: "unknown"}, Legs: []Leg{}, Details: map[string]any{"journey_ids": ids, "display_type": data["displayType"], "price_basis": "party_trip_total"}}
	var stops, duration int
	knownStops, knownDuration := true, true
	for _, r := range rows {
		leg, e := normalizeFlightLeg(r)
		if e != nil {
			return Offer{}, e
		}
		o.Legs = append(o.Legs, leg)
		meta := sourceObject(r["flightMetadata"])
		if v := sourceInt(meta["totalNumStop"]); v != nil {
			stops += *v
		} else {
			knownStops = false
		}
		if v := sourceInt(meta["tripDuration"]); v != nil {
			duration += *v
		} else {
			knownDuration = false
		}
		o.Refundable = combinePolicy(o.Refundable, sourceBool(meta["isRefundable"]), len(o.Legs) == 1)
		o.Reschedulable = combinePolicy(o.Reschedulable, sourceBool(meta["isReschedulable"]), len(o.Legs) == 1)
	}
	if knownStops {
		o.Stops = &stops
	}
	if knownDuration {
		o.DurationMinutes = &duration
	}
	return o, nil
}
func combinePolicy(a, b *bool, first bool) *bool {
	if first {
		return b
	}
	if (a != nil && !*a) || (b != nil && !*b) {
		v := false
		return &v
	}
	if a == nil || b == nil {
		return nil
	}
	v := true
	return &v
}
func normalizeFlightLeg(r map[string]any) (Leg, error) {
	l := Leg{Segments: []Segment{}, FareRules: map[string]any{}, Details: map[string]any{"id": r["id"], "flight_metadata": sourceFields(sourceObject(r["flightMetadata"]), "totalNumStop", "tripDuration", "transitAirportCodes", "transitDurations", "seatClassInventory", "isRefundable", "isReschedulable", "isSelfTransfer", "cardFacilities"), "fare": r["fare"]}}
	rules := []any{}
	routes := []any{}
	for _, v := range sourceList(r["connectingFlightRoutes"]) {
		route := sourceObject(v)
		rules = append(rules, sourceFields(route, "flightRefundInfo", "flightRescheduleInfo"))
		routes = append(routes, sourceFields(route, "departureAirport", "arrivalAirport", "numDayOffset", "totalNumStop", "durationInMinutes"))
		for _, v := range sourceList(route["segments"]) {
			seg := sourceObject(v)
			fac := sourceObject(seg["facilities"])
			l.Segments = append(l.Segments, Segment{Origin: sourceString(seg["departureAirport"]), Destination: sourceString(seg["arrivalAirport"]), DepartureDate: sourceDate(seg["departureDate"]), DepartureTime: sourceTime(seg["departureTime"]), ArrivalDate: sourceDate(seg["arrivalDate"]), ArrivalTime: sourceTime(seg["arrivalTime"]), DepartureUTCOffsetMinutes: sourceInt(seg["tzDepartureMinuteOffset"]), ArrivalUTCOffsetMinutes: sourceInt(seg["tzArrivalMinuteOffset"]), MarketingAirline: firstString(seg["airlineCode"], seg["brandCode"]), OperatingAirline: sourceString(seg["operatingAirlineCode"]), FlightNumber: sourceString(seg["flightNumber"]), Cabin: sourceString(seg["seatClass"]), DurationMinutes: sourceInt(seg["durationMinutes"]), CheckedBaggage: fac["baggage"], CabinBaggage: fac["cabinBaggage"], Details: sourceFields(seg, "brandCode", "departureTerminalName", "arrivalTerminalName", "aircraftType", "facilities", "aircraftInformation", "flightLegInfoList", "numTransits", "transitInfoV2", "mayReCheckIn", "visaRequired")})
		}
	}
	if len(l.Segments) == 0 {
		return l, apiError("MALFORMED_RESPONSE", "source flight candidate has no segments", 200, false)
	}
	l.FareRules["routes"] = rules
	l.Details["routes"] = routes
	return l, nil
}
