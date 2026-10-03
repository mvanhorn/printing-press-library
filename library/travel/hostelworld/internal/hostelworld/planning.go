// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hostelworld

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil"
)

type Query struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Nights   int    `json:"nights"`
	Guests   int    `json:"guests"`
}

var numericID = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
var htmlTags = regexp.MustCompile(`<[^>]*>`)
var decimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,6})?$`)

func ValidID(id string) bool { return numericID.MatchString(id) }
func NewQuery(start, end string, nights, guests int, now time.Time) (Query, error) {
	q := Query{CheckIn: start, CheckOut: end, Nights: nights, Guests: guests}
	a, e := time.Parse("2006-01-02", start)
	if e != nil {
		return q, fmt.Errorf("--check-in must be YYYY-MM-DD")
	}
	// The destination timezone is not known yet. Reject only dates that are
	// past everywhere; the provider decides its own source-local same-day rule.
	earliest := now.In(time.FixedZone("earliest-source-date", -12*3600))
	y, m, d := earliest.Date()
	if a.Before(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)) {
		return q, fmt.Errorf("--check-in is past in every source-local timezone")
	}
	if end != "" {
		b, e := time.Parse("2006-01-02", end)
		if e != nil {
			return q, fmt.Errorf("--check-out must be YYYY-MM-DD")
		}
		n := int(b.Sub(a) / (24 * time.Hour))
		if nights > 0 && nights != n {
			return q, fmt.Errorf("--nights and --check-out disagree")
		}
		q.Nights = n
	}
	if q.Nights < 1 || q.Nights > 30 {
		return q, fmt.Errorf("stay length must be 1–30 nights; supply --check-out or --nights")
	}
	if guests < 1 || guests > 10 {
		return q, fmt.Errorf("--guests must be 1–10; source property rules may impose lower limits")
	}
	q.CheckOut = a.AddDate(0, 0, q.Nights).Format("2006-01-02")
	return q, nil
}
func (q Query) Params() map[string]string {
	return map[string]string{"date-start": q.CheckIn, "num-nights": strconv.Itoa(q.Nights), "guests": strconv.Itoa(q.Guests), "application": "web", "show-rate-restrictions": "true"}
}
func Decode(data []byte) (map[string]any, error) {
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if e := d.Decode(&v); e != nil || v == nil {
		return nil, fmt.Errorf("source returned an invalid JSON object")
	}
	return v, nil
}
func Text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	}
	return ""
}
func object(v any) map[string]any { x, _ := v.(map[string]any); return x }
func array(v any) []any           { x, _ := v.([]any); return x }
func integer(v any) int           { n, _ := strconv.Atoi(Text(v)); return n }
func Clean(v any, cap int) string {
	s := strings.Join(strings.Fields(cliutil.CleanText(htmlTags.ReplaceAllString(Text(v), " "))), " ")
	r := []rune(s)
	if len(r) > cap {
		return string(r[:cap]) + "…"
	}
	return s
}
func money(v any, unit string) map[string]any {
	m := object(v)
	value, currency := Text(m["value"]), Text(m["currency"])
	if !decimal.MatchString(value) || len(currency) != 3 {
		return nil
	}
	return map[string]any{"value": value, "currency": currency, "unit": unit}
}
func rat(m map[string]any) *big.Rat {
	if m == nil {
		return nil
	}
	r, ok := new(big.Rat).SetString(Text(m["value"]))
	if !ok {
		return nil
	}
	return r
}
func multiply(m map[string]any, n int, unit string) map[string]any {
	r := rat(m)
	if r == nil || n < 1 {
		return nil
	}
	r.Mul(r, big.NewRat(int64(n), 1))
	return map[string]any{"value": r.FloatString(2), "currency": m["currency"], "unit": unit}
}
func sourcePrices(v any, plan string, unit string) map[string]any {
	for _, x := range array(v) {
		m := object(x)
		if Text(m["ratePlan"]) == plan {
			return money(m["price"], unit)
		}
	}
	return nil
}

var listingLabel = regexp.MustCompile(`[^a-z0-9]+`)

func canonical(id, name string) string {
	// The observed public SEO route requires a nonempty label. Hostelworld
	// resolves the stable numeric ID and redirects an alternate label to its
	// canonical one; the bare /p/id/ route returns 404.
	label := strings.ToLower(strings.ReplaceAll(Clean(name, 160), "&", " and "))
	label = strings.Trim(listingLabel.ReplaceAllString(label, "-"), "-")
	if label == "" {
		label = "property"
	}
	return "https://www.hostelworld.com/hostels/p/" + id + "/" + label + "/"
}
func BookingURL(id, name, city string, q Query) string {
	// The stable ID is authoritative; source names are escaped route labels.
	p := "https://www.hostelworld.com/pwa/hosteldetails.php/" + url.PathEscape(name) + "/" + url.PathEscape(city) + "/" + id
	values := url.Values{"from": {q.CheckIn}, "to": {q.CheckOut}, "guests": {strconv.Itoa(q.Guests)}}
	return p + "?" + values.Encode()
}
func Property(v map[string]any, now time.Time) (map[string]any, error) {
	id := Text(v["id"])
	if !ValidID(id) || Text(v["name"]) == "" {
		return nil, fmt.Errorf("property response schema changed: id/name missing")
	}
	facilities := []any{}
	for _, cat := range array(v["facilities"]) {
		for _, f := range array(object(cat)["facilities"]) {
			m := object(f)
			facilities = append(facilities, map[string]any{"id": m["id"], "name": m["name"], "category": object(cat)["name"]})
			if len(facilities) >= 100 {
				break
			}
		}
	}
	notes := []string{}
	for _, x := range array(v["thingsToNote"]) {
		if len(notes) >= 8 {
			break
		}
		notes = append(notes, Clean(x, 4000))
	}
	return map[string]any{"id": id, "name": v["name"], "source_name": v["transName"], "city": v["city"], "active": v["isActive"], "type": v["type"], "currency": v["currency"], "check_in": v["checkIn"], "check_out": v["latestCheckOut"], "maximum_guests_per_booking": v["maxNumberOfGuestsPerBooking"], "facilities": facilities, "policies": v["policies"], "source_rules": notes, "tax_info": v["taxInfo"], "fee_info": v["feeInfo"], "canonical_url": canonical(id, Text(v["name"])), "observed_at": now.UTC().Format(time.RFC3339), "evidence": "source listing; rules and facilities are not guarantees"}, nil
}
func Availability(v, property map[string]any, q Query, kind string, onlyFree bool, now time.Time) (map[string]any, error) {
	id := Text(v["id"])
	rooms := object(v["rooms"])
	if !ValidID(id) || rooms == nil {
		return nil, fmt.Errorf("availability response schema changed: id/rooms missing")
	}
	for _, key := range []string{"dorms", "privates"} {
		if _, ok := rooms[key].([]any); !ok {
			return nil, fmt.Errorf("availability response schema changed: %s is not an array", key)
		}
	}
	deadline := Text(v["freeCancellationAvailableUntil"])
	freeStatus := "unknown"
	flag, hasFlag := v["freeCancellationAvailable"].(bool)
	if hasFlag && !flag {
		freeStatus = "unavailable"
	} else if flag {
		d, e := time.Parse(time.RFC3339, deadline)
		if e == nil {
			if d.After(now) {
				freeStatus = "available"
			} else {
				freeStatus = "expired"
			}
		}
	}
	offers := []any{}
	for _, group := range []string{"dorms", "privates"} {
		category := "dorm"
		basis := "per_bed_per_stay"
		avgBasis := "per_bed_per_night"
		if group == "privates" {
			category = "private"
			basis = "per_room_per_stay"
			avgBasis = "per_room_per_night"
		}
		if kind != "all" && kind != category {
			continue
		}
		for _, r := range array(rooms[group]) {
			room := object(r)
			for _, p := range array(room["ratePlans"]) {
				plan := object(p)
				pid := Text(plan["id"])
				if pid == "" {
					return nil, fmt.Errorf("availability rate plan has no id")
				}
				total := sourcePrices(room["totalPrice"], pid, basis)
				avg := sourcePrices(room["averagePricePerNight"], pid, avgBasis)
				if total == nil {
					return nil, fmt.Errorf("rate plan %s has no valid source stay price", pid)
				}
				occupancy := integer(room["numberOfGuestsPerRoom"])
				capacity := integer(room["capacity"])
				qty := q.Guests
				inventory := integer(room["totalBedsAvailable"])
				quantityUnit := "beds"
				known := room["totalBedsAvailable"] != nil
				if category == "private" {
					inventory = integer(room["totalRoomsAvailable"])
					known = room["totalRoomsAvailable"] != nil
					quantityUnit = "rooms"
					qty = 0
					if occupancy > 0 {
						qty = (q.Guests + occupancy - 1) / occupancy
					}
				}
				fit := "unknown"
				if known && qty > 0 {
					if inventory >= qty {
						fit = "available"
					} else {
						fit = "insufficient_inventory"
					}
				}
				if len(array(plan["rateRuleViolations"])) > 0 {
					fit = "restricted"
				}
				if max := integer(property["maxNumberOfGuestsPerBooking"]); max > 0 && q.Guests > max {
					fit = "restricted"
				}
				if active, known := property["isActive"].(bool); known && !active {
					fit = "inactive_listing"
				}
				planFree, cancellationBasis := rateCancellationStatus(plan, freeStatus, deadline, now)
				if onlyFree && planFree != "available" {
					continue
				}
				daily := []any{}
				sum := new(big.Rat)
				dates := map[string]bool{}
				validDates := true
				a, _ := time.Parse("2006-01-02", q.CheckIn)
				expected := map[string]bool{}
				for day := 0; day < q.Nights; day++ {
					expected[a.AddDate(0, 0, day).Format("2006-01-02")] = true
				}
				for _, row := range array(room["priceBreakdown"]) {
					m := object(row)
					if Text(m["ratePlan"]) != pid {
						continue
					}
					date := Text(m["date"])
					price := money(m["price"], "unknown")
					if price == nil || price["currency"] != total["currency"] || !expected[date] || dates[date] {
						validDates = false
						continue
					}
					dates[date] = true
					sum.Add(sum, rat(price))
					daily = append(daily, map[string]any{"date": date, "price": price})
				}
				nightlyBasis := "unknown"
				if validDates && len(dates) == q.Nights && sum.Sign() > 0 {
					if sum.Cmp(rat(total)) == 0 {
						if category == "private" {
							nightlyBasis = "per_room_per_night"
						} else {
							nightlyBasis = "per_bed_per_night"
						}
					}
					if category == "private" && occupancy > 0 && new(big.Rat).Mul(new(big.Rat).Set(sum), big.NewRat(int64(occupancy), 1)).Cmp(rat(total)) == 0 {
						nightlyBasis = "per_occupancy_slot_per_night"
					}
				}
				for _, row := range daily {
					object(object(row)["price"])["unit"] = nightlyBasis
				}
				estimate := multiply(total, qty, "whole_party_per_stay")
				if fit != "available" {
					estimate = nil
				}
				terms := object(plan["paymentProcedure"])
				payment := map[string]any{"id": terms["id"], "label": terms["label"], "description": Clean(terms["description"], 1600)}
				offers = append(offers, map[string]any{"room_id": Text(room["id"]), "room_name": room["name"], "category": category, "source_dorm_type": room["basicType"], "room_capacity": capacity, "guests_per_private_room": room["numberOfGuestsPerRoom"], "ensuite": room["ensuite"], "available_beds": room["totalBedsAvailable"], "available_rooms": room["totalRoomsAvailable"], "rate_plan_id": pid, "rate_plan_type": plan["ratePlanType"], "meal_plan": room["mealPlan"], "room_conditions": room["conditions"], "rate_rule_violations": plan["rateRuleViolations"], "party_fit": fit, "required_quantity": qty, "quantity_unit": quantityUnit, "source_stay_amount": total, "source_average_nightly": avg, "source_nightly_breakdown": daily, "nightly_price_basis": nightlyBasis, "derived_party_estimate": estimate, "estimate_notice": "derived from source unit price and quantity; not a checkout quote", "guest_eligibility": "source dorm type and property rules require confirmation; same-room allocation is not guaranteed", "payment": payment, "free_cancellation_status": planFree, "free_cancellation_basis": cancellationBasis, "free_cancellation_deadline": deadline, "free_cancellation_deadline_scope": "availability_response", "rate_free_cancellation_deadline": plan["freeCancellationAvailableUntil"], "deposit_percentage": v["depositPercentage"]})
				if len(offers) > 150 {
					return nil, fmt.Errorf("source offers exceed 150-plan bound")
				}
			}
		}
	}
	state := "offers_present"
	if len(offers) == 0 {
		state = "no_matching_offers"
	}
	return map[string]any{"property_id": id, "property_name": property["name"], "property_rules": property["thingsToNote"], "tax_policy": property["policies"], "query": q, "status": state, "offers": offers, "cancellation_policies": v["cancellationPolicies"], "source_availability_free_cancellation_available": v["freeCancellationAvailable"], "source_availability_cancellation_status": freeStatus, "special_event_conditions": v["specialEventConditions"], "observed_at": now.UTC().Format(time.RFC3339), "canonical_url": canonical(id, Text(property["name"])), "booking_url": BookingURL(id, Text(property["name"]), Text(object(property["city"])["name"]), q), "coverage": "fresh source response for these dates and guests; not a booking guarantee"}, nil
}

// An availability-wide indication can describe an optional flexible rate. It
// does not establish that a deposit-only rate is refundable. Keep that source
// signal separate and require affirmative rate-level evidence for the filter.
func rateCancellationStatus(plan map[string]any, availabilityStatus, deadline string, now time.Time) (string, string) {
	rateType := strings.ReplaceAll(strings.ToUpper(Text(plan["ratePlanType"])), "_", "")
	if strings.Contains(rateType, "NONREFUND") {
		return "unavailable", "nonrefundable_rate_type"
	}
	description := Text(object(plan["paymentProcedure"])["description"])
	terms := strings.ToLower(Clean(description, len(description)))
	compact := strings.NewReplacer("-", "", " ", "").Replace(terms)
	if strings.Contains(compact, "nonrefundable") {
		if strings.Contains(terms, "unless") || strings.Contains(terms, "if you select") {
			return "conditional", "payment_requires_optional_flexible_booking"
		}
		return "unavailable", "nonrefundable_payment_terms"
	}
	if flag, present := plan["freeCancellationAvailable"].(bool); present {
		if !flag {
			return "unavailable", "rate_source_flag"
		}
		if availabilityStatus == "unavailable" {
			return "unknown", "conflicting_rate_and_availability_flags"
		}
		if own := Text(plan["freeCancellationAvailableUntil"]); own != "" {
			deadline = own
		}
		d, err := time.Parse(time.RFC3339, deadline)
		if err != nil {
			return "unknown", "rate_source_flag_without_valid_deadline"
		}
		if !d.After(now) {
			return "expired", "rate_source_flag_and_deadline"
		}
		return "available", "rate_source_flag_and_deadline"
	}
	if availabilityStatus == "unavailable" || availabilityStatus == "expired" {
		return availabilityStatus, "availability_response"
	}
	return "unknown", "availability_signal_does_not_establish_rate_refund"
}

// CompareAmounts keeps currencies separate and compares only valid party estimates.
func CompareAmounts(a, b map[string]any) bool {
	am, bm := object(a["derived_party_estimate"]), object(b["derived_party_estimate"])
	if am == nil {
		return false
	}
	if bm == nil {
		return true
	}
	if am["currency"] != bm["currency"] {
		return Text(am["currency"]) < Text(bm["currency"])
	}
	return rat(am).Cmp(rat(bm)) < 0
}
