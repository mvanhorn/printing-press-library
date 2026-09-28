package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil"
)

func validateCalendar(raw []byte) error {
	var w calendarWire
	if e := json.Unmarshal(raw, &w); e != nil {
		return e
	}
	cal := w.Calendar
	if cal == nil {
		return fmt.Errorf("missing availability_calendar")
	}
	if cal.Type != "success" {
		return &SourceStatusError{Status: cal.Type}
	}
	loc, e := time.LoadLocation(cal.TimeZone)
	if e != nil || cal.TimeZone == "" {
		return fmt.Errorf("invalid calendar time_zone %q", cal.TimeZone)
	}
	if cal.Data == nil {
		return fmt.Errorf("missing calendar data")
	}
	for date, slots := range cal.Data {
		if e := ValidateDate(date); e != nil {
			return fmt.Errorf("invalid calendar local date: %w", e)
		}
		for stamp := range slots {
			t, e := time.Parse(time.RFC3339Nano, stamp)
			if e != nil {
				return fmt.Errorf("invalid calendar slot timestamp %q", stamp)
			}
			_ = t.In(loc)
		}
	}
	for _, date := range cal.Closed {
		if e := ValidateDate(date); e != nil {
			return fmt.Errorf("invalid closed date: %w", e)
		}
	}
	return nil
}
func (c *Client) loadCalendar(ctx context.Context, v venueWire, date, requested string, party int) (calendarWire, observation, error) {
	loc, e := venueLocation(v)
	if e != nil {
		return calendarWire{}, observation{}, e
	}
	clock := queryAnchorTime(requested)
	start, _ := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
	body := map[string]any{"shop_id": v.ID, "locale": "en", "start_at": start.UTC().Format(time.RFC3339Nano), "num_people": party}
	raw, obs, e := c.fetch(ctx, "POST", "/v2/hub/availability_calendar_v2", nil, body, 30*time.Second, func(raw []byte) error {
		if e := validateCalendar(raw); e != nil {
			return e
		}
		var w calendarWire
		_ = json.Unmarshal(raw, &w)
		if w.Calendar.TimeZone != loc.String() {
			return fmt.Errorf("calendar time_zone %q differs from venue time_zone %q", w.Calendar.TimeZone, loc.String())
		}
		return nil
	})
	if e != nil {
		return calendarWire{}, obs, e
	}
	var w calendarWire
	_ = json.Unmarshal(raw, &w)
	return w, obs, nil
}
func (c *Client) Check(ctx context.Context, o CheckOptions) (Result, error) {
	if e := ValidateCheck(o); e != nil {
		return nil, e
	}
	limit, _ := resultLimit(o.Limit)
	v, venueObs, e := c.loadVenue(ctx, o.Venue)
	if e != nil {
		return c.failedChecks(o.Venue, []string{o.Date}, o.Party, o.Time, e), e
	}
	w, obs, e := c.loadCalendar(ctx, v, o.Date, o.Time, o.Party)
	if e != nil {
		return c.failedChecksForVenue(v, []string{o.Date}, o.Party, o.Time, e), e
	}
	row := calendarDay(v, w, o.Date, o.Time, o.Party, limit, o.IncludeUnavailable, obs, venueObs)
	return c.result(Result{"venue": venueSummary(v), "checks": []map[string]any{row}}, &obs), nil
}
func (c *Client) failedChecks(slug string, dates []string, party int, requested string, e error) Result {
	rows := make([]map[string]any, 0, len(dates))
	for _, date := range dates {
		rows = append(rows, failedDay(slug, "", nil, date, party, requested, e))
	}
	return c.result(Result{"venue": unknownVenue(slug), "checks": rows, "fetch_failures": []map[string]any{{"slug": slug, "error": e.Error()}}}, nil)
}
func (c *Client) failedChecksForVenue(v venueWire, dates []string, party int, requested string, e error) Result {
	rows := make([]map[string]any, 0, len(dates))
	for _, date := range dates {
		rows = append(rows, failedDay(v.Slug, v.ID, optionalString(v.TimeZone), date, party, requested, e))
	}
	return c.result(Result{"venue": venueSummary(v), "checks": rows, "fetch_failures": []map[string]any{{"slug": v.Slug, "error": e.Error()}}}, nil)
}

// The verified consumer calendar returns a time window around its anchor.
// A missing exact preference defaults to dinner; it never means full-day coverage.
func queryAnchorTime(requested string) string {
	if requested != "" {
		return requested
	}
	return "18:00"
}

func failedDay(slug, id string, zone any, date string, party int, requested string, e error) map[string]any {
	status := "failed"
	var source any
	var sourceErr *SourceStatusError
	if errors.As(e, &sourceErr) {
		source = sourceErr.Status
		if strings.ToLower(sourceErr.Status) != "error" && strings.ToLower(sourceErr.Status) != "failed" && strings.ToLower(sourceErr.Status) != "failure" {
			status = "unknown"
		}
	}
	return map[string]any{"venue_id": emptyNull(id), "slug": slug, "date": date, "party": party, "time_zone": zone, "scope": "venue", "requested_time": emptyNull(requested), "query_anchor_time": queryAnchorTime(requested), "requested_time_available": nil, "status": status, "source_status": source, "available_times": []string{}, "alternative_times": []string{}, "slots": []map[string]any{}, "coverage": nil, "freshness": nil, "error": e.Error()}
}
func calendarDay(v venueWire, w calendarWire, date, requested string, party, limit int, includeUnavailable bool, obs, venueObs observation) map[string]any {
	cal := w.Calendar
	loc, _ := time.LoadLocation(cal.TimeZone)
	day, present := cal.Data[date]
	closed := false
	for _, d := range cal.Closed {
		if d == date {
			closed = true
		}
	}
	dates := make([]string, 0, len(cal.Data))
	for d := range cal.Data {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	var from, to any
	if len(dates) > 0 {
		from = dates[0]
		to = dates[len(dates)-1]
	}
	type candidate struct {
		stamp string
		local time.Time
		slot  slotWire
	}
	matched := make([]candidate, 0, len(day))
	filtered := 0
	for stamp, slot := range day {
		t, e := time.Parse(time.RFC3339Nano, stamp)
		if e != nil {
			continue
		}
		local := t.In(loc)
		if local.Format("2006-01-02") != date {
			filtered++
			continue
		}
		matched = append(matched, candidate{t.UTC().Format(time.RFC3339Nano), local, slot})
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].local.Before(matched[j].local) })
	var timeFrom, timeTo any
	if len(matched) > 0 {
		timeFrom = matched[0].local.Format("15:04")
		timeTo = matched[len(matched)-1].local.Format("15:04")
	}
	available := make([]string, 0)
	alternatives := make([]string, 0)
	slots := make([]map[string]any, 0)
	evidence := make([]map[string]any, 0)
	var requestedAvailable any
	falseCount, unknownCount := 0, 0
	for _, s := range matched {
		hm := s.local.Format("15:04")
		isRequested := requested != "" && hm == requested && s.local.Second() == 0 && s.local.Nanosecond() == 0
		status := "unknown"
		var isAvailable any
		if s.slot.Available != nil {
			isAvailable = *s.slot.Available
			if *s.slot.Available {
				status = "available"
				available = append(available, hm)
				if requested != "" && !isRequested {
					alternatives = append(alternatives, hm)
				}
			} else {
				status = "unavailable"
				falseCount++
			}
		} else {
			unknownCount++
		}
		if isRequested {
			requestedAvailable = isAvailable
			evidence = append(evidence, map[string]any{"starts_at": s.stamp, "local_time": hm, "is_available": isAvailable})
		}
		if includeUnavailable || status == "available" || isRequested {
			relation := "offered_time"
			if requested != "" {
				relation = "alternative"
				if isRequested {
					relation = "requested"
				}
			}
			slot := map[string]any{"starts_at": s.stamp, "local_start_at": s.local.Format(time.RFC3339Nano), "local_time": hm, "is_available": isAvailable, "status": status, "relation": relation, "scope": "venue", "tc_points_rate": rawField(s.slot.raw, "tc_points_rate"), "fixed_amount": rawField(s.slot.raw, "fixed_amount"), "is_variable_rate": rawField(s.slot.raw, "is_variable_rate")}
			if isRequested {
				slots = append([]map[string]any{slot}, slots...)
			} else {
				slots = append(slots, slot)
			}
		}
	}
	status := "unknown"
	switch {
	case closed:
		status = "closed"
	case !present:
		status = "unknown"
	case requested != "":
		if b, ok := requestedAvailable.(bool); ok {
			if b {
				status = "available"
			} else {
				status = "unavailable"
			}
		}
	case len(available) > 0:
		status = "available"
	case falseCount > 0 && unknownCount == 0:
		status = "unavailable"
	}
	allAvailable, allSlots := len(available), len(slots)
	if len(available) > limit {
		available = available[:limit]
	}
	if len(alternatives) > limit {
		alternatives = alternatives[:limit]
	}
	if len(slots) > limit {
		slots = slots[:limit]
	}
	var err any
	return map[string]any{"venue_id": v.ID, "slug": v.Slug, "date": date, "party": party, "time_zone": cal.TimeZone, "scope": "venue", "requested_time": emptyNull(requested), "query_anchor_time": queryAnchorTime(requested), "requested_time_available": requestedAvailable, "status": status, "source_status": cal.Type, "available_times": available, "alternative_times": alternatives, "available_count": allAvailable, "slots": slots, "slots_total": allSlots, "has_more_slots": allSlots > limit, "truncated": allSlots > limit || allAvailable > limit, "coverage": map[string]any{"time_scope": "source_time_window", "full_day": false, "returned_time_from": timeFrom, "returned_time_to": timeTo, "date_present": present, "closed": closed, "returned_from": from, "returned_to": to, "timeslots": len(matched), "filtered_timestamp_count": filtered}, "freshness": obs.result(), "venue_freshness": venueObs.result(), "source": map[string]any{"endpoint": "/v2/hub/availability_calendar_v2", "method": "POST", "evidence": "explicit_is_available_boolean", "course_linked": false}, "source_evidence": evidence, "error": err}
}

func calendarCoversDate(w calendarWire, date string) bool {
	if w.Calendar == nil {
		return false
	}
	if _, present := w.Calendar.Data[date]; present {
		return true // An explicit empty slot map still covers the date.
	}
	for _, closed := range w.Calendar.Closed {
		if closed == date {
			return true
		}
	}
	return false
}

func (c *Client) Scan(ctx context.Context, o ScanOptions) (Result, error) {
	if e := ValidateScan(o); e != nil {
		return nil, e
	}
	limit, _ := resultLimit(o.Limit)
	start, _ := time.Parse("2006-01-02", o.From)
	end, _ := time.Parse("2006-01-02", o.To)
	dates := make([]string, 0, 14)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	type outcome struct {
		venue    map[string]any
		rows     []map[string]any
		failures []map[string]any
		err      error
	}
	type calendarSnapshot struct {
		calendar calendarWire
		observed observation
	}
	type calendarAttempt struct {
		snapshot calendarSnapshot
		err      error
	}
	outcomes := make([]outcome, len(o.Venues))
	jobs := make(chan int, len(o.Venues))
	for i := range o.Venues {
		jobs <- i
	}
	close(jobs)
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for worker := 0; worker < c.opts.Concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				slug := o.Venues[i]
				outcomes[i].venue = unknownVenue(slug)
				v, venueObs, e := c.loadVenue(scanCtx, slug)
				if e != nil {
					outcomes[i].err = e
					for _, date := range dates {
						outcomes[i].rows = append(outcomes[i].rows, failedDay(slug, "", nil, date, o.Party, o.Time, e))
					}
					var throttle *cliutil.RateLimitError
					if errors.As(e, &throttle) {
						cancel()
					}
					continue
				}
				outcomes[i].venue = venueSummary(v)
				covered := make(map[string]calendarSnapshot, len(dates))
				attempts := make(map[string]calendarAttempt, len(dates))
				var stopErr error
				for _, date := range dates {
					if _, ok := covered[date]; ok {
						continue
					}
					if stopErr != nil {
						attempts[date] = calendarAttempt{err: stopErr}
						continue
					}
					w, obs, err := c.loadCalendar(scanCtx, v, date, o.Time, o.Party)
					if err != nil {
						attempts[date] = calendarAttempt{err: err}
						var throttle *cliutil.RateLimitError
						if errors.As(err, &throttle) {
							stopErr = err
							cancel()
						} else if scanCtx.Err() != nil {
							stopErr = err
						}
						continue
					}
					// Reuse only explicit machine coverage. A min/max date span is not
					// proof that dates omitted from data and closed_dates were checked.
					for _, requestedDate := range dates {
						if _, alreadyCovered := covered[requestedDate]; !alreadyCovered && calendarCoversDate(w, requestedDate) {
							covered[requestedDate] = calendarSnapshot{w, obs}
						}
					}
					// A successful response can omit its anchor; retain that unknown
					// fallback in case a later window never explicitly covers it.
					attempts[date] = calendarAttempt{snapshot: calendarSnapshot{w, obs}}
				}
				// Materialize after all bounded reads so later explicit coverage can
				// recover an earlier failed or missing-anchor date. The covered map
				// retains each date's first explicit observation and timestamp.
				for _, date := range dates {
					if snapshot, ok := covered[date]; ok {
						outcomes[i].rows = append(outcomes[i].rows, calendarDay(v, snapshot.calendar, date, o.Time, o.Party, limit, o.IncludeUnavailable, snapshot.observed, venueObs))
						continue
					}
					attempt := attempts[date]
					if attempt.err == nil {
						outcomes[i].rows = append(outcomes[i].rows, calendarDay(v, attempt.snapshot.calendar, date, o.Time, o.Party, limit, o.IncludeUnavailable, attempt.snapshot.observed, venueObs))
						continue
					}
					outcomes[i].rows = append(outcomes[i].rows, failedDay(slug, v.ID, optionalString(v.TimeZone), date, o.Party, o.Time, attempt.err))
					outcomes[i].failures = append(outcomes[i].failures, map[string]any{"slug": slug, "date": date, "error": attempt.err.Error()})
					if outcomes[i].err == nil {
						outcomes[i].err = attempt.err
					}
					var throttle *cliutil.RateLimitError
					if errors.As(attempt.err, &throttle) {
						outcomes[i].err = attempt.err // Preserve typed 429 after an earlier partial failure.
					}
				}
			}
		}()
	}
	wg.Wait()
	venues := make([]map[string]any, 0, len(o.Venues))
	rows := make([]map[string]any, 0, len(o.Venues)*len(dates))
	failures := make([]map[string]any, 0)
	var throttle error
	failedVenues := 0
	for i, out := range outcomes {
		venues = append(venues, out.venue)
		rows = append(rows, out.rows...)
		if out.err != nil {
			failedVenues++
			if len(out.failures) == 0 {
				failures = append(failures, map[string]any{"slug": o.Venues[i], "error": out.err.Error()})
			} else {
				failures = append(failures, out.failures...)
			}
			var rateErr *cliutil.RateLimitError
			if errors.As(out.err, &rateErr) {
				throttle = out.err
			}
		}
	}
	result := c.result(Result{"venues": venues, "checks": rows, "fetch_failures": failures, "window": map[string]any{"from": o.From, "to": o.To, "days": len(dates), "venues": len(o.Venues)}}, nil)
	result["meta"].(map[string]any)["partial_failure"] = len(failures) > 0
	result["meta"].(map[string]any)["failed_venues"] = failedVenues
	if throttle != nil {
		return result, throttle
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if len(failures) > 0 {
		return result, &PartialError{failedVenues, len(o.Venues)}
	}
	return result, nil
}
