package asoview

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func calendarStatus(d Object) string {
	switch {
	case flag(d["isPast"]):
		return "past"
	case flag(d["isPassedDeadline"]):
		return "deadline_passed"
	case flag(d["isNotDoing"]):
		return "not_operating"
	case flag(d["isOutOfPeriod"]):
		return "outside_sales_or_operation_period"
	case flag(d["isRequest"]):
		return "request_only"
	case flag(d["isFull"]):
		return "sold_out"
	case flag(d["isNotReservable"]):
		return "not_reservable"
	case flag(d["isFewRemaining"]):
		return "few_remaining"
	case flag(d["isRemaining"]):
		return "available"
	default:
		return "unknown"
	}
}
func slotStatus(s Object, quantity int) string {
	if flag(s["isClosed"]) {
		return "closed"
	}
	if flag(s["isRequestDeadlinePassed"]) {
		return "deadline_passed"
	}
	min, max := integer(s["minimumReservableQuantity"]), integer(s["maximumReservableQuantity"])
	if (min != nil && int64(quantity) < *min) || (max != nil && *max > 0 && int64(quantity) > *max) {
		return "party_outside_limits"
	}
	if flag(s["isRequest"]) {
		return "request_only"
	}
	remain := integer(s["remainReserveNumber"])
	if remain != nil && *remain < int64(quantity) {
		if flag(s["canRequestAfterFull"]) {
			return "request_only"
		}
		if *remain == 0 {
			return "sold_out"
		}
		return "insufficient_quantity"
	}
	if remain != nil && *remain >= int64(quantity) {
		return "available"
	}
	switch text(s["stockStatus"]) {
	case "STOCK_OUT":
		return "sold_out"
	case "NOT_OPEN":
		return "not_operating"
	case "ON_REQUEST", "REQUEST":
		return "request_only"
	}
	return "unknown"
}

func aggregateSlotStatus(slots []Object) string {
	status := "unknown"
	requestOnly, unknown := false, false
	for i, slot := range slots {
		st := text(slot["status"])
		if st == "available" {
			return st
		}
		if i == 0 {
			status = st
		} else if status != st {
			status = "unavailable"
		}
		requestOnly = requestOnly || st == "request_only"
		unknown = unknown || st == "unknown"
	}
	if requestOnly {
		return "request_only"
	}
	if unknown {
		return "unknown"
	}
	return status
}
func timeMeaning(p Object, slot Object) string {
	if p["kind"] == "activity" {
		return "experience_start"
	}
	title := text(p["name_ja"])
	if nullable(slot["timeTicketScheduleId"]) != nil {
		return "reserved_entry_window"
	}
	if strings.Contains(title, "日付指定") {
		return "admission_window_on_selected_date"
	}
	return "source_time_window_unclassified"
}
func (c *Client) calendar(ctx context.Context, p Object, month string) ([]Object, error) {
	path := "/stocks/calendars"
	q := url.Values{"planCode": {text(p["id"])}, "yearMonth": {month}, "limit": {"1"}}
	if p["kind"] == "ticket" {
		path = "/stocks/ticket/calendars"
		q.Set("channelCode", "asoview")
	}
	raw, err := c.Get(ctx, path, q, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var ds Object
	if err = decodeJSON(raw, &ds); err != nil {
		return nil, err
	}
	if _, ok := ds["years"].([]any); !ok {
		return nil, fail(5, "Asoview calendar unavailable or schema changed for %s", p["id"])
	}
	days := []Object{}
	seen := map[string]bool{}
	for _, year := range list(ds["years"]) {
		for _, mon := range list(object(year)["months"]) {
			for _, week := range list(object(mon)["weeks"]) {
				for _, v := range list(object(week)["days"]) {
					d := object(v)
					date := text(d["date"])
					if date == "" || !strings.HasPrefix(date, month+"-") || seen[date] {
						continue
					}
					if _, err = ParseDate(date); err != nil {
						return nil, fail(5, "source calendar date is invalid")
					}
					seen[date] = true
					days = append(days, Object{"date": date, "status": calendarStatus(d), "price": price(d["fee"], nil, "date_specific_calendar_from", date), "source_stock_status": nullable(d["stockStatus"]), "signals": Object{"past": d["isPast"], "deadline_passed": d["isPassedDeadline"], "out_of_period": d["isOutOfPeriod"], "not_operating": d["isNotDoing"], "not_reservable": d["isNotReservable"], "full": d["isFull"], "remaining": d["isRemaining"], "few_remaining": d["isFewRemaining"], "request_only": d["isRequest"]}})
				}
			}
		}
	}
	return days, nil
}
func (c *Client) courses(ctx context.Context, p Object, date string, quantity int) ([]Object, []Object, error) {
	path := "/stocks/courses"
	q := url.Values{"planCode": {text(p["id"])}, "date": {date}}
	if p["kind"] == "ticket" {
		path = "/stocks/ticket/courses"
		q.Set("channelCode", "asoview")
	}
	raw, err := c.Get(ctx, path, q, 30*time.Second)
	if err != nil {
		return nil, nil, err
	}
	var rows []any
	if err = decodeJSON(raw, &rows); err != nil {
		return nil, nil, err
	}
	if rows == nil {
		return nil, nil, fail(5, "Asoview slot response is null")
	}
	if len(rows) > 100 {
		return nil, nil, fail(5, "source returned more than 100 slots; unsupported inventory breadth")
	}
	normalized := []Object{}
	source := []Object{}
	for _, v := range rows {
		s := object(v)
		id := idString(s["id"])
		if id == "" {
			return nil, nil, fail(5, "Asoview slot ID missing")
		}
		source = append(source, s)
		normalized = append(normalized, Object{"id": id, "date": date, "status": slotStatus(s, quantity), "start_time": nullable(s["startTimeLabel"]), "end_time": nullable(s["endTimeLabel"]), "label": nullable(s["label"]), "time_meaning": timeMeaning(p, s), "schedule_id": nullable(s["timeTicketScheduleId"]), "quantity_limits_confirmed": integer(s["maximumReservableQuantity"]) != nil && *integer(s["maximumReservableQuantity"]) > 0, "price": price(s["sellingFee"], s["unit"], "date_specific_slot_from", date), "minimum_quantity": integer(s["minimumReservableQuantity"]), "maximum_quantity": integer(s["maximumReservableQuantity"]), "remaining_quantity": integer(s["remainReserveNumber"]), "source_stock_status": nullable(s["stockStatus"]), "source_stock_label": nullable(s["stockStatusLabel"]), "request_only": s["isRequest"], "can_request_after_full": s["canRequestAfterFull"], "quantity_checked": quantity, "reserved": false})
	}
	return normalized, source, nil
}
func dateEligibility(p Object, date string) string {
	v := object(p["validity"])
	for _, r := range list(v["unavailable_periods"]) {
		m := object(r)
		from, to := text(m["from"]), text(m["to"])
		if from != "" && to != "" && date >= from && date <= to {
			return "not_usable"
		}
	}
	if flag(v["is_date_period"]) {
		if end := text(v["end_date"]); end != "" {
			if endDate, err := ParseDate(end); err == nil && date > endDate.Format("2006-01-02") {
				return "validity_expired"
			}
		}
	}
	return "unknown"
}
func (c *Client) Availability(ctx context.Context, input, date, month string, quantity int) (Object, error) {
	if err := validateQuantity(quantity); err != nil {
		return nil, err
	}
	if date != "" && month != "" {
		return nil, fail(2, "use exactly one of --date or --month")
	}
	if date == "" && month == "" {
		return nil, fail(2, "--date YYYY-MM-DD or --month YYYY-MM is required")
	}
	if date != "" {
		t, err := ParseDate(date)
		if err != nil {
			return nil, err
		}
		month = t.Format("2006-01")
	} else {
		if _, err := ParseMonth(month); err != nil {
			return nil, err
		}
	}
	p, err := c.Product(ctx, input, false)
	if err != nil {
		return nil, err
	}
	header := Object{"id": p["id"], "name_ja": p["name_ja"], "booking_url": p["booking_url"], "entry": p["entry"], "validity": p["validity"], "requested_date": strptr(date), "requested_month": month, "requested_quantity": quantity, "date_party_total": nil, "stock_is_snapshot": true, "reserved": false}
	if p["kind"] == "ticket" && object(p["entry"])["selection"] == "no_reserved_slot_in_product" {
		status := "unknown"
		if date != "" {
			status = dateEligibility(p, date)
		}
		header["days"] = []Object{}
		header["slots"] = []Object{}
		header["status"] = status
		header["coverage"] = Object{"public_dated_stock": false, "reason": "general_admission_validity_has_no_reserved_slot_inventory", "date_eligibility": status}
		return header, nil
	}
	days, err := c.calendar(ctx, p, month)
	if err != nil {
		return nil, err
	}
	header["days"] = days
	header["slots"] = []Object{}
	header["coverage"] = Object{"public_dated_stock": true, "party_eligibility_confirmed": false, "price_total_confirmed": false}
	header["status"] = "calendar_only"
	if date != "" {
		day := Object{"date": date, "status": "unknown", "price": nil}
		found := false
		for _, d := range days {
			if d["date"] == date {
				day = d
				found = true
				break
			}
		}
		header["days"] = []Object{day}
		header["status"] = day["status"]
		if !found {
			header["coverage"] = Object{"public_dated_stock": false, "reason": "date_absent_from_source_calendar"}
			return header, nil
		}
		slots, _, e := c.courses(ctx, p, date, quantity)
		if e != nil {
			return nil, e
		}
		header["slots"] = slots
		if dateEligibility(p, date) == "not_usable" {
			header["status"] = "not_usable"
		} else if len(slots) > 0 {
			header["status"] = aggregateSlotStatus(slots)
		}
	}
	return header, nil
}
func (c *Client) Options(ctx context.Context, input, date, slot string, quantity int, party string) (Object, error) {
	if err := validateQuantity(quantity); err != nil {
		return nil, err
	}
	if date != "" {
		if _, err := ParseDate(date); err != nil {
			return nil, err
		}
	}
	if (slot != "" || party != "") && date == "" {
		return nil, fail(2, "--slot and --party require --date")
	}
	if party != "" {
		var e error
		quantity, e = PartyQuantity(party)
		if e != nil {
			return nil, e
		}
	}
	p, err := c.Product(ctx, input, false)
	if err != nil {
		return nil, err
	}
	result := Object{"id": p["id"], "name_ja": p["name_ja"], "booking_url": p["booking_url"], "date": strptr(date), "selected_slot": nil, "options": p["options"], "slots": []Object{}, "product_age_band": p["age_band"], "party_subtotal": nil, "date_party_total": nil, "coverage": "advertised_bands_only"}
	if date == "" {
		return result, nil
	}
	var raw []any
	if p["kind"] == "activity" {
		if slot != "" {
			slots, src, e := c.courses(ctx, p, date, quantity)
			if e != nil {
				return nil, e
			}
			_, selected, e := chooseSlot(slots, src, slot)
			if e != nil {
				return nil, e
			}
			result["selected_slot"] = selected
			result["slots"] = slots
		}
		body, e := c.Get(ctx, "/reservations/plans/"+text(p["id"])+"/dates/"+date+"/basicfees", url.Values{}, 30*time.Second)
		if e != nil {
			return nil, e
		}
		if e = decodeJSON(body, &raw); e != nil {
			return nil, e
		}
	} else {
		if object(p["entry"])["selection"] == "no_reserved_slot_in_product" {
			result["coverage"] = "general_admission_advertised_bands_no_dated_band_surface"
			if party != "" {
				return nil, fail(5, "dated party subtotal unavailable for general-admission ticket; inspect advertised options instead")
			}
			return result, nil
		}
		slots, src, e := c.courses(ctx, p, date, quantity)
		if e != nil {
			return nil, e
		}
		result["slots"] = slots
		if slot == "" && len(slots) != 1 {
			if party != "" {
				return nil, fail(2, "--party requires a selected --slot when multiple or no source slots exist")
			}
			result["options"] = []Object{}
			result["coverage"] = "select_slot_to_fetch_dated_bands"
			return result, nil
		}
		chosen, selected, e := chooseSlot(slots, src, slot)
		if e != nil {
			return nil, e
		}
		result["selected_slot"] = selected
		start := NormalizeTime(text(chosen["startTimeLabel"]))
		if start == "" {
			return nil, fail(5, "source slot has no time key for dated band retrieval")
		}
		q := url.Values{"ticketTypeCode": {text(p["id"])}, "channelCode": {"asoview"}, "date": {date}, "time": {start}}
		body, e := c.Get(ctx, "/item/category-sales-situations/", q, 30*time.Second)
		if e != nil {
			return nil, e
		}
		var ds Object
		if e = decodeJSON(body, &ds); e != nil {
			return nil, e
		}
		a, ok := ds["categories"].([]any)
		if !ok {
			return nil, fail(5, "Asoview dated price bands unavailable or schema changed")
		}
		raw = a
	}
	if raw == nil {
		return nil, fail(5, "Asoview returned null dated bands")
	}
	if len(raw) > 100 {
		return nil, fail(5, "source returned more than 100 fee bands")
	}
	bands, err := optionsFrom(raw, true, date)
	if err != nil {
		return nil, err
	}
	result["options"] = bands
	result["coverage"] = "date_specific_bands_not_checkout_quote"
	if party != "" {
		sub, e := PartySubtotal(bands, party)
		if e != nil {
			return nil, e
		}
		result["party_subtotal"] = sub
	}
	return result, nil
}
func chooseSlot(slots, raw []Object, requested string) (Object, Object, error) {
	for i, s := range slots {
		if (requested == "" && len(slots) == 1) || text(s["id"]) == requested || NormalizeTime(text(s["start_time"])) == NormalizeTime(requested) || text(s["label"]) == requested {
			return raw[i], s, nil
		}
	}
	return nil, nil, fail(2, "--slot %q is not an available source slot ID/time; inspect availability --date first", requested)
}
func PartySubtotal(bands []Object, party string) (Object, error) {
	values := strings.Split(party, ",")
	if len(values) > 20 {
		return nil, fail(2, "--party supports up to 20 bands")
	}
	seen := map[string]bool{}
	parts := []Object{}
	var sum int64
	total := 0
	for _, entry := range values {
		x := strings.Split(entry, ":")
		if len(x) != 2 || seen[x[0]] {
			return nil, fail(2, "--party must be distinct option-id:quantity pairs separated by commas")
		}
		n, e := strconv.Atoi(x[1])
		if e != nil || n < 1 || n > 50 {
			return nil, fail(2, "--party band quantity must be 1..50")
		}
		total += n
		if total > 50 {
			return nil, fail(2, "--party total quantity exceeds 50")
		}
		seen[x[0]] = true
		var band Object
		for _, b := range bands {
			if text(b["id"]) == x[0] || text(b["code"]) == x[0] {
				band = b
				break
			}
		}
		if band == nil {
			return nil, fail(2, "--party option %q is absent from source bands", x[0])
		}
		canonical := "band:" + text(band["id"])
		if seen[canonical] {
			return nil, fail(2, "--party contains the same source option by ID and code")
		}
		seen[canonical] = true
		bp := object(band["price"])
		amount, ok := bp["amount"].(*int64)
		if !ok || amount == nil || bp["unit"] == nil {
			return nil, fail(5, "source band amount/unit missing; party subtotal cannot be calculated")
		}
		if *amount < 0 || *amount > 100000000 {
			return nil, fail(5, "source band amount outside bounded calculation range")
		}
		alloc, _ := band["allocation"].(*int64)
		if alloc != nil && *alloc != 1 {
			return nil, fail(5, "source allocation differs from one unit; party subtotal unsupported")
		}
		line := *amount * int64(n)
		sum += line
		parts = append(parts, Object{"option_id": band["id"], "quantity": n, "unit": bp["unit"], "amount": line})
	}
	return Object{"amount": sum, "currency": "JPY", "basis": "calculated_dated_band_subtotal", "quantity": total, "lines": parts, "quote_confirmed": false, "excludes": []string{"additional_charges", "group_or_coupon_discounts", "age_and_option_dependencies", "checkout_validation"}}, nil
}

// PartyQuantity checks syntax before any source request and supplies stock-check units.
func PartyQuantity(party string) (int, error) {
	entries := strings.Split(party, ",")
	if len(entries) > 20 {
		return 0, fail(2, "--party supports up to 20 bands")
	}
	total := 0
	seen := map[string]bool{}
	for _, entry := range entries {
		x := strings.Split(entry, ":")
		if len(x) != 2 || x[0] == "" || seen[x[0]] {
			return 0, fail(2, "--party must be distinct option-id:quantity pairs")
		}
		seen[x[0]] = true
		n, e := strconv.Atoi(x[1])
		if e != nil || n < 1 || n > 50 {
			return 0, fail(2, "--party quantity must be 1..50")
		}
		total += n
	}
	if total > 50 {
		return 0, fail(2, "--party total quantity exceeds 50")
	}
	return total, nil
}
