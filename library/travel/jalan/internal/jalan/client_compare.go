package jalan

import (
	"context"
	"fmt"
	"sort"
)

// Compare evaluates dates OR exact plan/room pairs serially under one budget.
// It preserves source order and supplies sorted prices only inside comparable
// currency/basis groups; reference or unknown quotes are never zero prices.
func (c *Client) Compare(ctx context.Context, propertyID string, q Query, dates []string, plans []PlanRef) (Response, error) {
	if err := validateID(propertyID, "property"); err != nil {
		return Response{}, err
	}
	if (len(dates) == 0) == (len(plans) == 0) {
		return Response{}, usage("invalid_query", "compare requires dates or plan/room pairs, exclusively", "Pass --dates YYYY-MM-DD,... or --check-in YYYY-MM-DD with --plans plan_id:room_id,... .")
	}
	count := len(dates) + len(plans)
	if count > 5 {
		return Response{}, usage("invalid_query", "compare accepts at most five alternatives", "Use no more than five dates or exact plan/room pairs.")
	}
	ctx, cancel, err := c.commandContext(ctx)
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	queryDate := q.CheckIn
	if len(dates) > 0 {
		q.CheckIn = dates[0]
	}
	q, err = normalizeQuery(q, false, c.now())
	if err != nil {
		return Response{}, err
	}
	if len(dates) > 0 {
		if err := validateOfferFilters(q); err != nil {
			return Response{}, err
		}
		seen := map[string]bool{}
		for _, date := range dates {
			if seen[date] {
				return Response{}, usage("invalid_query", "comparison dates must be distinct", "Remove duplicate dates.")
			}
			seen[date] = true
			candidate := q
			candidate.CheckIn = date
			if _, err := normalizeQuery(candidate, false, c.now()); err != nil {
				return Response{}, err
			}
		}
	} else {
		if err := validatePlanFilters(q); err != nil {
			return Response{}, err
		}
		seen := map[string]bool{}
		for _, plan := range plans {
			if err := validateID(plan.PlanID, "plan"); err != nil {
				return Response{}, err
			}
			if err := validateID(plan.RoomID, "room"); err != nil {
				return Response{}, err
			}
			key := plan.PlanID + ":" + plan.RoomID
			if seen[key] {
				return Response{}, usage("invalid_query", "comparison plan/room pairs must be distinct", "Remove duplicate plan/room pairs.")
			}
			seen[key] = true
		}
	}
	session := c.newSession()
	results := []any{}
	failures := []map[string]any{}
	warnings := []string{"Alternative observations are fetched serially, so source inventory is not an atomic snapshot.", "Price order covers only fetched offers with the same currency and price basis; there is no cheapest or exhaustive-inventory guarantee."}
	var firstFailure error
	for i := 0; i < count; i++ {
		candidate := q
		var observed Response
		var cell map[string]any
		var cellErr error
		var requestURL string
		if len(dates) > 0 {
			candidate.CheckIn = dates[i]
			requestURL = c.sourceURL("/yad"+propertyID+"/plan/", candidate.values())
			observed, cellErr = c.offers(ctx, propertyID, candidate, session)
			cell = map[string]any{"alternative_index": i, "check_in": dates[i], "query": candidate}
		} else {
			plan := plans[i]
			values := candidate.values()
			values.Set("yadNo", propertyID)
			values.Set("planCd", plan.PlanID)
			values.Set("roomTypeCd", plan.RoomID)
			values.Set("screenId", "UWW3101")
			requestURL = c.sourceURL("/uw/uwp3200/uww3201init.do", values)
			observed, cellErr = c.plan(ctx, propertyID, plan.PlanID, plan.RoomID, candidate, session)
			cell = map[string]any{"alternative_index": i, "check_in": candidate.CheckIn, "plan_id": plan.PlanID, "room_id": plan.RoomID, "query": candidate}
		}
		if cellErr != nil {
			if firstFailure == nil {
				firstFailure = cellErr
			}
			failed := failure(cellErr, requestURL)
			failed["alternative_index"] = i
			if len(dates) > 0 {
				failed["check_in"] = dates[i]
			} else {
				failed["plan_id"] = plans[i].PlanID
				failed["room_id"] = plans[i].RoomID
				failed["check_in"] = candidate.CheckIn
			}
			failures = append(failures, failed)
			continue
		}
		cell["status"] = observed.Meta["status"]
		cell["results"] = observed.Results
		cell["pagination"] = observed.Pagination
		cell["source_url"] = requestURL
		cell["observations"] = observationsForURL(session, requestURL)
		cell["comparable_price_groups"] = comparablePriceGroups(observed.Results)
		results = append(results, cell)
		if sourceWarnings, ok := observed.Meta["warnings"].([]string); ok {
			warnings = append(warnings, sourceWarnings...)
		}
	}
	baseStay := q
	if len(dates) > 0 {
		baseStay.CheckIn = queryDate
	}
	query := map[string]any{"property_id": propertyID, "stay": baseStay, "dates": dates, "plans": plans, "requested_check_in": queryDate}
	if dates == nil {
		query["dates"] = []string{}
	}
	if plans == nil {
		query["plans"] = []PlanRef{}
	}
	response := c.response(session, query, results, map[string]any{"requested_alternatives": count, "returned_count": len(results), "failed_count": len(failures), "has_more": false}, warnings, false)
	response.Meta["comparison_mode"] = "plans"
	if len(dates) > 0 {
		response.Meta["comparison_mode"] = "dates"
	}
	response.FetchFailures = failures
	if len(failures) > 0 && len(results) == 0 {
		response.Meta["status"] = "failed"
		code := failures[0]["code"].(string)
		for _, failed := range failures[1:] {
			if failed["code"] != code {
				code = "fetch_failure"
				break
			}
		}
		return response, &Error{Code: code, Message: fmt.Sprintf("all %d comparison alternatives failed", count), Hint: "Inspect fetch_failures and retry only after resolving the source failure.", URL: failures[0]["url"].(string), Cause: firstFailure, FetchFailures: failures}
	}
	if len(failures) > 0 {
		response.Meta["status"] = "partial"
		response.Meta["coverage"] = "partial observed alternatives; not exhaustive; failed alternatives are retained in fetch_failures"
		return response, &PartialError{Failures: failures, Cause: firstFailure}
	}
	return response, nil
}

func observationsForURL(session *requestSession, sourceURL string) []observation {
	out := []observation{}
	for _, entry := range session.observations {
		if entry.URL == sourceURL {
			out = append(out, entry)
		}
	}
	return out
}

func comparablePriceGroups(items []any) []any {
	groups := map[string][]map[string]any{}
	for index, item := range items {
		var price Price
		available := false
		switch typed := item.(type) {
		case Offer:
			price = typed.Price
			available = typed.Availability == "available"
		case Plan:
			price = typed.Price
			available = typed.Availability == "available"
		}
		if !available || price.Amount == nil || *price.Amount < 0 || price.Currency == "" || price.Currency == "unknown" || price.Basis == "" || price.Basis == "unknown" {
			continue
		}
		key := price.Currency + "/" + price.Basis
		groups[key] = append(groups[key], map[string]any{"result_index": index, "amount": *price.Amount, "currency": price.Currency, "basis": price.Basis})
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []any{}
	for _, key := range keys {
		prices := groups[key]
		sort.SliceStable(prices, func(i, j int) bool { return prices[i]["amount"].(int64) < prices[j]["amount"].(int64) })
		out = append(out, map[string]any{"scope": "fetched comparable base quotes only", "currency_basis": key, "ordered_observations": prices, "observed_count": len(prices)})
	}
	return out
}
