// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package jbo

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/cliutil"
)

type Stop struct {
	ID         string `json:"stop_id"`
	Name       string `json:"name"`
	NameJA     any    `json:"name_ja"`
	Role       string `json:"role"`
	SourceTime string `json:"source_time"`
	Timestamp  string `json:"timestamp_jst"`
	MapURL     string `json:"map_url,omitempty"`
}

func ParseStops(doc *html.Node, s Service) ([]Stop, error) {
	out := []Stop{}
	for _, n := range all(doc, func(n *html.Node) bool {
		return n.Data == "input" && (attr(n, "name") == "DepBusStop" || attr(n, "name") == "ArvBusStop")
	}) {
		v := strings.Split(attr(n, "value"), ",")
		if len(v) < 3 {
			continue
		}
		id := v[0]
		clock := v[len(v)-1]
		name := strings.TrimSpace(strings.Join(v[1:len(v)-1], ","))
		role := "departure"
		day := s.DepDate
		if attr(n, "name") == "ArvBusStop" {
			role = "arrival"
			day = s.ArrDate
		}
		m := timeRE.FindStringSubmatch(clock)
		if len(m) > 0 && intVal(m[1]) >= 24 {
			day = s.DepDate
		}
		ts, e := Timestamp(day, clock)
		if e != nil {
			return nil, e
		}
		st := Stop{id, name, nameJA(name), role, clock, ts, ""}
		box := n
		for box.Parent != nil && !cls(box, "row") {
			box = box.Parent
		}
		for _, a := range all(box, func(n *html.Node) bool { return n.Data == "a" }) {
			if strings.Contains(attr(a, "href"), "maps") {
				st.MapURL = attr(a, "href")
				break
			}
		}
		out = append(out, st)
	}
	if len(out) == 0 {
		return nil, errors.New("source boarding-stop selection unavailable or layout changed")
	}
	return out, nil
}
func ParseFares(doc *html.Node) (map[string]any, int, error) {
	fares := map[string]any{}
	for _, n := range all(doc, func(n *html.Node) bool {
		return n.Data == "input" && strings.HasPrefix(attr(n, "id"), "Fare_PassengerName")
	}) {
		name := attr(n, "value")
		if name == "" {
			continue
		}
		idx := strings.TrimPrefix(attr(n, "id"), "Fare_PassengerName")
		f := byID(doc, "Fare_Passenger"+idx)
		if f == nil {
			continue
		}
		unit, err := strconv.Atoi(attr(f, "value"))
		if err != nil || unit < 0 {
			return nil, 0, fmt.Errorf("invalid source unit fare for %s", name)
		}
		label := strings.ToLower(name)
		if label == "adult" {
			fares["adult"] = map[string]any{"label": name, "unit_jpy": unit}
		} else if strings.HasPrefix(label, "child") {
			fares["child"] = map[string]any{"label": name, "unit_jpy": unit}
		}
	}
	m := regexp.MustCompile(`Max number of tickets per transaction\s*:\s*([0-9]+)`).FindStringSubmatch(text(doc))
	limit := 0
	if len(m) > 0 {
		limit = intVal(m[1])
	}
	if len(fares) == 0 {
		return nil, 0, errors.New("source adult/child fare labels unavailable; cannot infer fares")
	}
	return fares, limit, nil
}

// The selected fare table is fresher and scoped to the selected stop pair and
// plan. Bare positive counts are lower bounds: the provider's numeric display
// does not document whether it caps the count. Zero explicitly reports no seats.
func ParseFareAvailability(doc *html.Node) Availability {
	m := regexp.MustCompile(`(?i)Seat Availability\s*:\s*([0-9]+)\b`).FindStringSubmatch(text(doc))
	if len(m) == 0 {
		return Availability{Status: "unknown", Raw: "Selected fare table does not report a numeric seat count"}
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return Availability{Status: "unknown", Raw: m[0]}
	}
	a := Availability{Status: "available", Raw: m[0], LowerBound: n}
	if n == 0 {
		a.Status, a.Exact, a.LowerBound = "sold_out", 0, nil
	}
	return a
}
func PartyEvidence(a Availability, total, limit int) map[string]any {
	out := map[string]any{"status": "unknown", "requested_seats": total, "availability": a, "max_tickets_per_transaction": nil, "seat_adjacency": "unknown", "gender_or_seat_plan_feasibility": "unknown"}
	if limit > 0 {
		out["max_tickets_per_transaction"] = limit
		if total > limit {
			out["status"] = "exceeds_transaction_limit"
			return out
		}
	}
	if a.Exact != nil {
		if a.Exact.(int) >= total {
			out["status"] = "capacity_sufficient"
		} else {
			out["status"] = "capacity_insufficient"
		}
	} else if a.LowerBound != nil && a.LowerBound.(int) >= total {
		out["status"] = "capacity_sufficient"
	}
	if a.Status == "sold_out" {
		out["status"] = "capacity_insufficient"
	}
	return out
}
func (c *Client) Quote(ctx context.Context, route string, dir int, date, service, dep, arr string, adults, children, plan int) (map[string]any, error) {
	if adults < 0 || children < 0 || adults+children < 1 || adults+children > 20 {
		return nil, errors.New("party must request 1-20 seats with nonnegative adult and child counts")
	}
	if plan < 1 || plan > 20 {
		return nil, errors.New("fare-plan must be 1-20")
	}
	out, sv, path, e := c.Services(ctx, route, dir, date)
	if e != nil {
		return nil, e
	}
	if basis, ok := out["fare_basis"]; ok {
		out["headline_fare_basis"] = basis
		delete(out, "fare_basis")
	}
	if len(sv) == 0 {
		out["quote_status"] = "no_inventory_for_requested_date"
		out["party"] = PartyEvidence(Availability{Status: "unknown"}, adults+children, 0)
		return out, nil
	}
	delete(out, "services")
	var s *Service
	for i := range sv {
		if service == "" && sv[i].Availability.Status == "available" || service != "" && sv[i].ID == service {
			s = &sv[i]
			break
		}
	}
	if s == nil {
		if service == "" {
			out["quote_status"] = "no_confirmed_available_service"
			out["party"] = PartyEvidence(Availability{Status: "unknown"}, adults+children, 0)
			return out, nil
		}
		return nil, fmt.Errorf("service %s not offered on requested day", service)
	}
	out["service"] = s
	if s.Availability.Status != "available" {
		out["quote_status"] = "service_not_confirmed_available"
		out["party"] = PartyEvidence(Availability{Status: "unknown", Raw: "Selected stop-pair capacity not checked; service headline is unavailable"}, adults+children, 0)
		return out, nil
	}
	dates := func(x string) string { return strings.ReplaceAll(x, "-", "") }
	times := func(x string) string { return strings.ReplaceAll(x, ":", "") }
	suffix := route + "/" + s.ID + "/" + dates(s.DepDate) + "/" + times(s.DepTime) + "/" + strconv.Itoa(dir) + "/" + dates(s.ArrDate) + "/" + times(s.ArrTime)
	plans, _, e := c.Get(ctx, "/"+c.Language+"/DetailAjax/SelectRoute/"+suffix+"?time=99")
	if e != nil {
		return nil, e
	}
	inputs := all(plans, func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == "radioBtn" })
	if plan > len(inputs) {
		return nil, fmt.Errorf("fare-plan %d unavailable; source offers %d plans", plan, len(inputs))
	}
	radio := attr(inputs[plan-1], "value")
	if !regexp.MustCompile(`^[0-9]+/[0-9]+/[0-9]+/$`).MatchString(radio) {
		return nil, errors.New("source fare-plan format changed")
	}
	out["fare_plan"] = plan
	out["fare_plan_id"] = strings.TrimSuffix(radio, "/")
	out["fare_plan_choices"] = len(inputs)
	asof := regexp.MustCompile(`Seat Availability as of\s*([0-9/]+\s+[0-9:]+)`).FindStringSubmatch(text(plans))
	if len(asof) > 0 {
		out["source_availability_as_of_jst"] = asof[1]
	}
	doc, _, e := c.Get(ctx, "/"+c.Language+"/DetailAjax/SelectFABN/"+radio+"/"+strconv.Itoa(dir)+"?time=00")
	if e != nil {
		return nil, e
	}
	stops, e := ParseStops(doc, *s)
	if e != nil {
		return nil, e
	}
	out["stops"] = stops
	out["stop_id_scope"] = "route, direction and service; not a global bus-stop ID"
	var ds, as *Stop
	for i := range stops {
		st := &stops[i]
		if st.Role == "departure" && (dep == st.ID || dep == "" && ds == nil) {
			ds = st
		}
		if st.Role == "arrival" && (arr == st.ID || arr == "") {
			as = st
		}
	}
	if ds == nil || as == nil {
		return nil, errors.New("requested departure/arrival stop ID is not offered by this service")
	}
	dts, _ := time.Parse(time.RFC3339, ds.Timestamp)
	ats, _ := time.Parse(time.RFC3339, as.Timestamp)
	if !ats.After(dts) {
		return nil, errors.New("arrival stop time must follow boarding stop time")
	}
	out["boarding"] = ds
	out["alighting"] = as
	fareDoc, raw, e := c.Get(ctx, "/"+c.Language+"/DetailAjax/GetFareTable/"+ds.ID+"/"+as.ID+"/"+strconv.Itoa(dir)+"?time=88")
	if e != nil {
		return nil, e
	}
	fares, limit, e := ParseFares(fareDoc)
	if e != nil {
		return nil, e
	}
	out["fares"] = fares
	out["currency"] = "JPY"
	out["party"] = PartyEvidence(ParseFareAvailability(fareDoc), adults+children, limit)
	out["capacity_basis"] = "Selected stop pair and fare plan; positive numeric display conservatively treated as a lower bound, booking confirmation required"
	out["fare_source_url"] = canonical("/" + c.Language + "/DetailAjax/GetFareTable/" + ds.ID + "/" + as.ID + "/" + strconv.Itoa(dir))
	out["adults"] = adults
	out["children"] = children
	out["trip_type"] = "one_way"
	out["group_discount"] = "unknown_not_assumed"
	out["roundtrip_fare"] = "unknown_not_assumed"
	out["price_basis"] = "Source labeled unit fares for selected stops; arithmetic estimate, booking confirmation required"
	out["estimated_total_jpy"] = nil
	total := 0
	known := true
	for kind, count := range map[string]int{"adult": adults, "child": children} {
		f, ok := fares[kind]
		if count > 0 && !ok {
			known = false
		}
		if ok {
			total += count * f.(map[string]any)["unit_jpy"].(int)
		}
	}
	if known {
		out["estimated_total_jpy"] = total
	}
	out["quote_status"] = "fare_evidence_reported"
	out["booking_url"] = canonical(path)
	out["cancellation_fee_conditions"] = nil
	// Cancellation endpoint parameters are read from this source fare response.
	m := regexp.MustCompile(`(?s)\$\.get\('(/(?:en|zh-tw|zh-cn|ko)/DetailAjax/GetCancelFee/)'\s*\+(.+?),\s*function`).FindStringSubmatch(raw)
	if len(m) > 0 {
		parts := regexp.MustCompile(`["']([0-9]+)["']`).FindAllStringSubmatch(m[2], -1)
		if len(parts) == 6 {
			ps := []string{}
			for _, p := range parts {
				ps = append(ps, p[1])
			}
			cancelDoc, _, ce := c.Get(ctx, m[1]+strings.Join(ps, "/"))
			if ce == nil {
				out["cancellation_fee_conditions"] = text(cancelDoc)
			} else {
				// Every c.Get request uses the Client's shared AdaptiveLimiter.
				// A throttled optional fetch still surfaces the typed source failure.
				var rateErr *cliutil.RateLimitError
				if errors.As(ce, &rateErr) {
					return nil, ce
				}
				out["partial_failures"] = []string{"Cancellation fee fetch failed: " + ce.Error()}
			}
		}
	}
	out["upstream_requests"] = c.Requests
	out["response_bytes"] = c.Bytes
	out["elapsed_ms"] = time.Since(c.Started).Milliseconds()
	return out, nil
}
func (c *Client) Conditions(ctx context.Context, route string, dir int, date string) (map[string]any, error) {
	_, ds, e := c.Route(ctx, route)
	if e != nil {
		return nil, e
	}
	t, e := ParseDate(date)
	if e != nil {
		return nil, e
	}
	var d *Direction
	for i := range ds {
		if ds[i].Direction == dir {
			d = &ds[i]
		}
	}
	if d == nil {
		return nil, errors.New("direction not offered")
	}
	path := strings.TrimPrefix(d.URL, Origin) + t.Format("20060102")
	doc, _, e := c.Get(ctx, path)
	if e != nil {
		return nil, e
	}
	n := byID(doc, "TextArea")
	if n == nil {
		return nil, errors.New("route/operator conditions unavailable or layout changed")
	}
	out := c.Metadata(path)
	out["route_id"] = route
	out["direction"] = dir
	out["conditions"] = text(n)
	out["general_policy_urls"] = map[string]string{"luggage": canonical("/" + c.Language + "/Luggage"), "boarding": canonical("/" + c.Language + "/Boarding"), "faq": canonical("/" + c.Language + "/FAQ"), "terms": canonical("/" + c.Language + "/TermsOfUse")}
	out["cancellation_note"] = "Exact service/plan cancellation fees are returned by bus quote when source provides them"
	return out, nil
}
