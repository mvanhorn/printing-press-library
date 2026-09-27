package jalan

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const paginationConsistencyWarning = "Fresh calls and different native source pages may reorder results. Use --max-age 5m to reuse each cached source page for repeatable local slices; this does not create a snapshot across native pages."

func (c *Client) sourceURL(path string, values url.Values) string {
	if len(values) == 0 {
		return c.baseURL + path
	}
	return c.baseURL + path + "?" + values.Encode()
}
func parseFailure(err error, sourceURL string) error {
	return &Error{Code: "parse_failure", Message: "source HTML could not be interpreted as the requested page", Hint: "Open the source URL to inspect changed markup or an unavailable detail page.", URL: sourceURL, Cause: err}
}

func (c *Client) response(session *requestSession, query any, items []any, pagination map[string]any, warnings []string, noResults bool) Response {
	if items == nil {
		items = []any{}
	}
	if pagination == nil {
		pagination = map[string]any{}
	}
	allWarnings := append(append([]string{}, session.warnings...), warnings...)
	urls := []string{}
	status := "ok"
	if noResults {
		status = "no_matches"
	}
	cacheStatus := "live"
	var observedAt any
	var earliest time.Time
	var age int64
	for i, observation := range session.observations {
		urls = append(urls, observation.URL)
		if i == 0 {
			cacheStatus = observation.Cache
		} else if cacheStatus != observation.Cache {
			cacheStatus = "mixed"
		}
		if earliest.IsZero() || observation.ObservedAt.Before(earliest) {
			earliest = observation.ObservedAt
		}
		if observation.CacheAgeMS > age {
			age = observation.CacheAgeMS
		}
	}
	if !earliest.IsZero() {
		observedAt = earliest.Format(time.RFC3339Nano)
	}
	meta := map[string]any{"source": "Jalan anonymous Japanese public HTML", "source_urls": urls, "observed_at": observedAt, "timezone": "Asia/Tokyo", "query": query, "cache_status": cacheStatus, "cache_age_ms": age, "upstream_requests": session.requests, "elapsed_ms": c.now().Sub(session.started).Milliseconds(), "coverage": "bounded observed source subset; not exhaustive; source pages may change between observations", "status": status, "warnings": allWarnings, "observations": session.observations}
	if len(urls) == 1 {
		meta["source_url"] = urls[0]
	}
	return Response{Meta: meta, Results: items, Pagination: pagination, FetchFailures: []map[string]any{}}
}

// Search maps a logical page into native 30-item source pages, then slices the
// requested window. Native idx values that are not multiples of 30 are rounded.
func (c *Client) Search(ctx context.Context, q Query) (Response, error) {
	ctx, cancel, err := c.commandContext(ctx)
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	q, err = normalizeQuery(q, true, c.now())
	if err != nil {
		return Response{}, err
	}
	session := c.newSession()
	logicalStart := (q.Page - 1) * q.Limit
	sourceOffset := (logicalStart / 30) * 30
	skip := logicalStart % 30
	items := []any{}
	warnings := []string{paginationConsistencyWarning}
	offsets := []int{}
	sourceCount := 0
	var total *int
	hasNext := false
	noResults := false
	var partialCause error
	failures := []map[string]any{}
	for sourcePage := 0; sourcePage < 2; sourcePage++ {
		values := q.values()
		values.Set("idx", strconv.Itoa(sourceOffset+sourcePage*30))
		sourceURL := c.sourceURL("/uw/uwp1400/uww1400.do", values)
		body, err := c.fetch(ctx, sourceURL, session)
		if err != nil {
			if len(items) == 0 {
				return Response{}, err
			}
			partialCause = err
			failures = append(failures, failure(err, sourceURL))
			break
		}
		page, err := ParseSearch(body, sourceURL)
		if err != nil {
			wrapped := parseFailure(err, sourceURL)
			if len(items) == 0 {
				return Response{}, wrapped
			}
			partialCause = wrapped
			failures = append(failures, failure(wrapped, sourceURL))
			break
		}
		if err := verifyQueryEcho(body, q, sourceURL); err != nil {
			if len(items) == 0 {
				return Response{}, err
			}
			partialCause = err
			failures = append(failures, failure(err, sourceURL))
			break
		}
		offsets = append(offsets, sourceOffset+sourcePage*30)
		sourceCount += len(page.Items)
		warnings = append(warnings, page.Warnings...)
		if page.Total != nil {
			total = page.Total
		}
		hasNext = page.HasNext
		noResults = page.NoResults && len(items) == 0
		start := 0
		if sourcePage == 0 {
			start = skip
		}
		if start > len(page.Items) {
			start = len(page.Items)
		}
		remaining := q.Limit - len(items)
		end := min(len(page.Items), start+remaining)
		for _, item := range page.Items[start:end] {
			items = append(items, item)
		}
		if end < len(page.Items) {
			hasNext = true
		}
		if len(items) >= q.Limit || !page.HasNext || len(page.Items) == 0 {
			break
		}
	}
	hasMore := hasNext
	if total != nil {
		hasMore = logicalStart+len(items) < *total
	}
	nextAction := any(nil)
	if hasMore {
		nextAction = fmt.Sprintf("Repeat with --page %d --limit %d and the same query; add --max-age 5m for repeatable slices within each cached source page (no snapshot across native pages).", q.Page+1, q.Limit)
	}
	pagination := map[string]any{"page": q.Page, "requested_limit": q.Limit, "returned_count": len(items), "source_page_size": 30, "source_returned_count": sourceCount, "source_offsets": offsets, "total": total, "has_more": hasMore, "next_action": nextAction}
	response := c.response(session, q, items, pagination, warnings, noResults)
	if len(failures) > 0 {
		response.FetchFailures = failures
		response.Meta["status"] = "partial"
		response.Meta["coverage"] = "partial observed subset; not exhaustive; a required source page failed"
		return response, &PartialError{Failures: failures, Cause: partialCause}
	}
	return response, nil
}

func (c *Client) Property(ctx context.Context, propertyID string) (Response, error) {
	if err := validateID(propertyID, "property"); err != nil {
		return Response{}, err
	}
	ctx, cancel, err := c.commandContext(ctx)
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	session := c.newSession()
	sourceURL := c.sourceURL("/yad"+propertyID+"/", nil)
	body, err := c.fetch(ctx, sourceURL, session)
	if err != nil {
		return Response{}, err
	}
	property, err := ParseProperty(body, sourceURL, propertyID)
	if err != nil {
		return Response{}, parseFailure(err, sourceURL)
	}
	return c.response(session, map[string]any{"property_id": propertyID}, []any{property}, map[string]any{"returned_count": 1, "has_more": false}, nil, false), nil
}

func (c *Client) Offers(ctx context.Context, propertyID string, q Query) (Response, error) {
	if err := validateID(propertyID, "property"); err != nil {
		return Response{}, err
	}
	ctx, cancel, err := c.commandContext(ctx)
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	q, err = normalizeQuery(q, false, c.now())
	if err != nil {
		return Response{}, err
	}
	if err := validateOfferFilters(q); err != nil {
		return Response{}, err
	}
	return c.offers(ctx, propertyID, q, c.newSession())
}
func (c *Client) offers(ctx context.Context, propertyID string, q Query, session *requestSession) (Response, error) {
	sourceURL := c.sourceURL("/yad"+propertyID+"/plan/", q.values())
	body, err := c.fetch(ctx, sourceURL, session)
	if err != nil {
		return Response{}, err
	}
	page, err := ParseOffers(body, sourceURL, propertyID)
	if err != nil {
		return Response{}, parseFailure(err, sourceURL)
	}
	if err := verifyQueryEcho(body, q, sourceURL); err != nil {
		return Response{}, err
	}
	// The source counts plans, while one plan can contain many room tuples. Slice
	// the observed tuple list instead of treating the plan count as a tuple total.
	start := (q.Page - 1) * q.Limit
	end := min(len(page.Items), start+q.Limit)
	if start > len(page.Items) {
		start = len(page.Items)
	}
	items := []any{}
	for _, item := range page.Items[start:end] {
		items = append(items, item)
	}
	hasMore := end < len(page.Items) || page.HasNext
	warnings := append([]string{paginationConsistencyWarning}, page.Warnings...)
	var total any
	if !page.HasNext {
		total = len(page.Items)
	}
	nextAction := any(nil)
	if hasMore {
		nextAction = fmt.Sprintf("Repeat with --page %d --limit %d and the same query; add --max-age 5m for repeatable slices within each cached source page (no snapshot across native pages).", q.Page+1, q.Limit)
	}
	if page.HasNext {
		warnings = append(warnings, "Source advertises additional offers beyond this observed page; native continuation is not verified.")
		if end >= len(page.Items) {
			nextAction = "Open the source URL for additional offer pages."
		}
	}
	pagination := map[string]any{"page": q.Page, "requested_limit": q.Limit, "returned_count": len(items), "source_page_size": nil, "source_returned_count": len(page.Items), "total": total, "source_plan_count": page.PlanTotal, "has_more": hasMore, "next_action": nextAction}
	if start >= len(page.Items) && page.HasNext {
		return Response{}, &Error{Code: "unsupported", Message: "requested offer page is outside the observed source tuples", Hint: "Open the source URL; continuation beyond this source page is not verified.", URL: sourceURL}
	}
	return c.response(session, map[string]any{"property_id": propertyID, "stay": q}, items, pagination, warnings, page.NoResults), nil
}

func (c *Client) Plan(ctx context.Context, propertyID, planID, roomID string, q Query) (Response, error) {
	for _, entry := range []struct{ value, kind string }{{propertyID, "property"}, {planID, "plan"}, {roomID, "room"}} {
		if err := validateID(entry.value, entry.kind); err != nil {
			return Response{}, err
		}
	}
	ctx, cancel, err := c.commandContext(ctx)
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	q, err = normalizeQuery(q, false, c.now())
	if err != nil {
		return Response{}, err
	}
	if err := validatePlanFilters(q); err != nil {
		return Response{}, err
	}
	return c.plan(ctx, propertyID, planID, roomID, q, c.newSession())
}
func (c *Client) plan(ctx context.Context, propertyID, planID, roomID string, q Query, session *requestSession) (Response, error) {
	values := q.values()
	values.Set("yadNo", propertyID)
	values.Set("planCd", planID)
	values.Set("roomTypeCd", roomID)
	values.Set("screenId", "UWW3101")
	sourceURL := c.sourceURL("/uw/uwp3200/uww3201init.do", values)
	body, err := c.fetch(ctx, sourceURL, session)
	if err != nil {
		return Response{}, err
	}
	plan, err := ParsePlan(body, sourceURL, propertyID, planID, roomID)
	if err != nil {
		return Response{}, parseFailure(err, sourceURL)
	}
	if plan.Availability != "unavailable" {
		if err := verifyQueryEcho(body, q, sourceURL); err != nil {
			return Response{}, err
		}
	}
	response := c.response(session, map[string]any{"property_id": propertyID, "plan_id": planID, "room_id": roomID, "stay": q}, []any{plan}, map[string]any{"returned_count": 1, "has_more": false}, nil, false)
	if plan.Availability == "unavailable" {
		response.Meta["status"] = "unavailable"
		response.Meta["warnings"] = append(response.Meta["warnings"].([]string), "The source rejected the requested dated plan/room; reference prices are excluded from dated quotes.")
	}
	return response, nil
}
