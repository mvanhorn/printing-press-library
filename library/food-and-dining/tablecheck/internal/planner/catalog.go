package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) Search(ctx context.Context, o SearchOptions) (Result, error) {
	if e := ValidateSearch(o); e != nil {
		return nil, e
	}
	limit, _ := resultLimit(o.Limit)
	q := url.Values{"geo_latitude": {strconv.FormatFloat(o.Latitude, 'f', -1, 64)}, "geo_longitude": {strconv.FormatFloat(o.Longitude, 'f', -1, 64)}, "geo_distance": {strconv.FormatFloat(o.Radius, 'f', -1, 64)}, "shop_universe_id": {"57e0b91744aea12988000001"}, "per_page": {strconv.Itoa(limit)}, "include_ids": {"true"}, "sort_by": {"distance"}, "venue_type": {"tc"}, "service_mode": {"dining"}, "locale": {"en"}}
	for k, v := range map[string]string{"cuisines[]": o.Cuisine, "budget_dinner_avg_min": o.BudgetMin, "budget_dinner_avg_max": o.BudgetMax, "date": o.Date, "time": o.Time, "search_after": o.Cursor} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if o.Party != 0 {
		q.Set("num_people", strconv.Itoa(o.Party))
	}
	if o.Date != "" || o.Time != "" || o.Party != 0 {
		q.Set("availability_mode", "same_meal_time")
	}
	if o.Date != "" {
		q.Set("availability_format", "datetime")
	}
	raw, obs, e := c.fetch(ctx, "GET", "/v2/shop_search", q, nil, 5*time.Minute, validateSearch)
	if e != nil {
		return nil, e
	}
	var w searchWire
	_ = json.Unmarshal(raw, &w)
	items := make([]map[string]any, 0, limit)
	for i, v := range w.Shops {
		if i >= limit {
			break
		}
		s := venueSummary(v)
		s["discovery_availability"] = rawField(v.raw, "availability")
		s["availability_scope"] = "discovery_summary"
		items = append(items, s)
	}
	var next, more, count any
	if w.Meta.Cursor != nil && *w.Meta.Cursor != "" {
		next = *w.Meta.Cursor
	}
	if w.Meta.LastPage != nil {
		more = !*w.Meta.LastPage
		if *w.Meta.LastPage {
			next = nil
		}
	}
	if w.Meta.RecordCount != nil {
		count = *w.Meta.RecordCount
	}
	truncated := len(w.Shops) > limit
	return c.result(Result{"items": items, "pagination": map[string]any{"limit": limit, "has_more": more, "next_cursor": next, "record_count": count, "truncated": truncated}, "query": map[string]any{"latitude": o.Latitude, "longitude": o.Longitude, "radius_m": o.Radius, "cuisine": emptyNull(o.Cuisine), "budget_min": emptyNull(o.BudgetMin), "budget_max": emptyNull(o.BudgetMax), "budget_basis": "venue_dinner_average", "date": emptyNull(o.Date), "time": emptyNull(o.Time), "party": zeroNull(o.Party), "availability_scope": "discovery_summary"}}, &obs), nil
}
func emptyNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func zeroNull(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
func (c *Client) loadVenue(ctx context.Context, slug string) (venueWire, observation, error) {
	if e := ValidateVenue(slug); e != nil {
		return venueWire{}, observation{}, e
	}
	raw, obs, e := c.fetch(ctx, "GET", "/v2/shops/"+url.PathEscape(slug), url.Values{"locale": {"en"}}, nil, time.Hour, validateVenue)
	if e != nil {
		return venueWire{}, obs, e
	}
	var w searchWire
	_ = json.Unmarshal(raw, &w)
	for _, v := range w.Shops {
		if v.Slug == slug {
			return v, obs, nil
		}
	}
	return venueWire{}, obs, &NotFoundError{fmt.Sprintf("venue %q was not returned by TableCheck", slug)}
}
func (c *Client) Venue(ctx context.Context, slug string) (Result, error) {
	v, obs, e := c.loadVenue(ctx, slug)
	if e != nil {
		return nil, e
	}
	out := venueSummary(v)
	for _, key := range []string{"address", "alt_address", "phone", "url", "service_categories", "request_policy_translations", "request_policy_require_confirm", "request_policy_placement", "enable_request_any_datetime", "content_body_translations", "tc_input_service_category", "service_modes"} {
		out[key] = rawField(v.raw, key)
	}
	out["request_policy"] = translationField(v.raw, "request_policy_translations", "en")
	out["request_policy_ja"] = translationField(v.raw, "request_policy_translations", "ja")
	out["policy_scope"] = "static_venue_conditions"
	return c.result(Result{"venue": out}, &obs), nil
}
func translationField(raw map[string]json.RawMessage, key, locale string) any {
	var t []translation
	if json.Unmarshal(raw[key], &t) != nil {
		return nil
	}
	return text(t, locale)
}
func (c *Client) Cuisines(ctx context.Context, query string, limit, offset int) (Result, error) {
	limit, e := resultLimit(limit)
	if e != nil {
		return nil, e
	}
	if offset < 0 || offset > 100000 {
		return nil, invalid("offset must be between 0 and 100000")
	}
	if len(query) > 200 {
		return nil, invalid("query must be at most 200 characters")
	}
	raw, obs, e := c.fetch(ctx, "GET", "/v2/cuisines", nil, nil, 24*time.Hour, validateCuisines)
	if e != nil {
		return nil, e
	}
	var w cuisinesWire
	_ = json.Unmarshal(raw, &w)
	all := make([]map[string]any, 0)
	q := strings.ToLower(strings.TrimSpace(query))
	for _, v := range w.Cuisines {
		match := q == "" || strings.Contains(strings.ToLower(v.Field), q)
		for _, t := range v.Texts {
			if strings.Contains(strings.ToLower(t.Translation), q) {
				match = true
			}
		}
		if match {
			all = append(all, map[string]any{"id": v.Field, "key": v.Field, "name": displayName(v.Texts), "name_en": text(v.Texts, "en"), "name_ja": text(v.Texts, "ja")})
		}
	}
	items, page := paginate(all, limit, offset)
	return c.result(Result{"items": items, "pagination": page}, &obs), nil
}
func paginate(all []map[string]any, limit, offset int) ([]map[string]any, map[string]any) {
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	start := offset
	if start > len(all) {
		start = len(all)
	}
	items := append(make([]map[string]any, 0), all[start:end]...)
	more := end < len(all)
	var next any
	if more {
		next = end
	}
	return items, map[string]any{"limit": limit, "offset": offset, "has_more": more, "next_offset": next, "total": len(all)}
}
func (c *Client) Courses(ctx context.Context, o CourseOptions) (Result, error) {
	if e := ValidateCourses(o); e != nil {
		return nil, e
	}
	limit, _ := resultLimit(o.Limit)
	v, venueObs, e := c.loadVenue(ctx, o.Venue)
	if e != nil {
		return nil, e
	}
	loc, e := venueLocation(v)
	if e != nil {
		return nil, e
	}
	from := o.From
	if from == "" {
		from = c.opts.Now().In(loc).Format("2006-01-02")
	}
	start, _ := time.ParseInLocation("2006-01-02", from, loc)
	to := o.To
	if to == "" {
		to = start.AddDate(0, 0, 50).Format("2006-01-02")
	}
	if to < from {
		return nil, invalid("to must not be before from")
	}
	end, _ := time.ParseInLocation("2006-01-02", to, loc)
	body := map[string]any{"shop_id": v.ID, "locale": "en", "date_min": start.UTC().Format(time.RFC3339Nano), "date_max": end.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano), "has_price": true}
	raw, obs, e := c.fetch(ctx, "POST", "/v2/hub/menu_items", nil, body, time.Hour, validateMenus)
	if e != nil {
		return nil, e
	}
	var w menusWire
	_ = json.Unmarshal(raw, &w)
	r := Result{"venue": venueSummary(v), "window": map[string]any{"from": from, "to": to, "time_zone": loc.String(), "boundary_semantics": "source_date_window"}}
	if o.CourseID != "" {
		for _, m := range w.Items {
			if m.ID != o.CourseID {
				continue
			}
			detail := make(map[string]any, len(m.raw))
			for k, value := range m.raw {
				detail[k] = decodeAny(value)
			}
			for k, value := range courseSummary(m, v) {
				detail[k] = value
			}
			for _, key := range []string{"tagline", "description", "fine_print", "how_to_redeem"} {
				field := key + "_translations"
				detail[key] = translationField(m.raw, field, "en")
				detail[key+"_ja"] = translationField(m.raw, field, "ja")
			}
			for _, key := range []string{"valid_date_ranges", "valid_time_ranges", "available_times", "min_time_cutoff_at", "max_time_cutoff_at", "days", "meals", "min_order_qty", "max_order_qty", "duration", "payment_type", "payment_require_intl", "payment_require_group_size", "payment_partial_order_unit_amt", "payment_partial_order_pct", "payment_partial_group_fee", "payment_partial_person_fee", "service_category_ids", "questions", "cancel_fee_rules", "pax_prices"} {
				detail[key] = rawField(m.raw, key)
			}
			detail["computed_total"] = nil
			detail["venue_freshness"] = venueObs.result()
			r["course"] = detail
			return c.result(r, &obs), nil
		}
		return nil, &NotFoundError{fmt.Sprintf("course %q was not returned in venue %q menu window", o.CourseID, o.Venue)}
	}
	all := make([]map[string]any, 0, len(w.Items))
	for _, m := range w.Items {
		all = append(all, courseSummary(m, v))
	}
	r["items"], r["pagination"] = paginate(all, limit, o.Offset)
	return c.result(r, &obs), nil
}
func venueLocation(v venueWire) (*time.Location, error) {
	if v.TimeZone == nil || *v.TimeZone == "" {
		return nil, fmt.Errorf("venue %s has no time_zone; local-date planning is unknown", v.Slug)
	}
	loc, e := time.LoadLocation(*v.TimeZone)
	if e != nil {
		return nil, fmt.Errorf("venue %s has invalid time_zone: %w", v.Slug, e)
	}
	return loc, nil
}
func (c *Client) BookingURL(ctx context.Context, o HandoffOptions) (Result, error) {
	if e := ValidateHandoff(o); e != nil {
		return nil, e
	}
	v, obs, e := c.loadVenue(ctx, o.Venue)
	if e != nil {
		return nil, e
	}
	mode := ""
	if v.BookingMode != nil {
		mode = *v.BookingMode
	}
	_, target := canonical(v.Slug, mode)
	if target == "" {
		return nil, fmt.Errorf("venue %s has unknown booking_page_mode; cannot construct a canonical booking handoff", v.Slug)
	}
	u, _ := url.Parse(target)
	q := u.Query()
	if o.Date != "" {
		q.Set("start_date", o.Date)
	}
	if o.Time != "" {
		q.Set("start_time", o.Time)
	}
	if o.Party != 0 {
		q.Set("num_people", strconv.Itoa(o.Party))
	}
	u.RawQuery = q.Encode()
	return c.result(Result{"venue_id": v.ID, "slug": v.Slug, "booking_url": u.String(), "booking_page_mode": mode, "date": emptyNull(o.Date), "time": emptyNull(o.Time), "party": zeroNull(o.Party), "availability_checked": false, "reservation_created": false}, &obs), nil
}
