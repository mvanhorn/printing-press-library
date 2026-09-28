package navitime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var regexpLocalID = regexp.MustCompile(`^local_[a-f0-9]{24}$`)
var shapePattern = regexp.MustCompile(`\bvar\s+shapeParams\s*=\s*`)
var moneyPattern = regexp.MustCompile(`(?:JPY\s*|¥\s*)([0-9][0-9,]*)`)
var numberPattern = regexp.MustCompile(`\d+`)
var clockPattern = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
var distancePattern = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*(km|m)\s*$`)

func ptr[T any](v T) *T { return &v }
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, key string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == key {
			return true
		}
	}
	return false
}
func nodes(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	var walk func(*html.Node)
	walk = func(v *html.Node) {
		if v == nil {
			return
		}
		if pred(v) {
			out = append(out, v)
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func classNodes(n *html.Node, key string) []*html.Node {
	return nodes(n, func(v *html.Node) bool { return hasClass(v, key) })
}
func firstClass(n *html.Node, key string) *html.Node {
	v := classNodes(n, key)
	if len(v) == 0 {
		return nil
	}
	return v[0]
}
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	parts := []string{}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		parts = append(parts, text(c))
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
func ct(n *html.Node, key string) string { return text(firstClass(n, key)) }
func unique(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range values {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func texts(n *html.Node, key string) []string {
	out := []string{}
	for _, v := range classNodes(n, key) {
		out = append(out, text(v))
	}
	return unique(out)
}
func money(s string) *int {
	m := moneyPattern.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	v, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if e != nil {
		return nil
	}
	return &v
}
func icMoney(s string) *int {
	i := strings.Index(strings.ToUpper(s), "IC")
	if i < 0 {
		return nil
	}
	return money(s[i:])
}
func number(s string) *int {
	v := numberPattern.FindString(s)
	if v == "" {
		return nil
	}
	n, e := strconv.Atoi(v)
	if e != nil {
		return nil
	}
	return &n
}
func distance(s string) *int {
	m := distancePattern.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	v, e := strconv.ParseFloat(m[1], 64)
	if e != nil {
		return nil
	}
	if strings.EqualFold(m[2], "km") {
		v *= 1000
	}
	if v > 1e7 {
		return nil
	}
	return ptr(int(math.Round(v)))
}
func sourceErr(u, msg string) error { return &SourceError{Message: msg, URL: u} }

func parsePlaces(body []byte, u string) ([]Place, error) {
	var raw []struct {
		Code     string            `json:"code"`
		Name     map[string]string `json:"name"`
		Address  map[string]string `json:"addressName"`
		Category any               `json:"categoryName"`
		Kind     string            `json:"mainType"`
		Coord    *struct {
			Lat *float64 `json:"lat"`
			Lon *float64 `json:"lon"`
			Alt *float64 `json:"alt"`
		} `json:"coord"`
	}
	if json.Unmarshal(body, &raw) != nil || raw == nil {
		return nil, sourceErr(u, "NAVITIME autocomplete response is not a candidate array")
	}
	out := []Place{}
	for _, v := range raw {
		refKind := "station"
		if v.Kind == "spot" {
			refKind = "spot"
		}
		if _, _, err := parseRef(refKind + ":" + v.Code); err != nil {
			return nil, sourceErr(u, "NAVITIME candidate has an invalid source identifier")
		}
		var coords *Coordinates
		if v.Coord != nil {
			if v.Coord.Lat != nil && (*v.Coord.Lat < -90 || *v.Coord.Lat > 90) || v.Coord.Lon != nil && (*v.Coord.Lon < -180 || *v.Coord.Lon > 180) {
				return nil, sourceErr(u, "NAVITIME candidate coordinates are invalid")
			}
			coords = &Coordinates{v.Coord.Lat, v.Coord.Lon, v.Coord.Alt, "degrees", "m"}
		}
		out = append(out, Place{v.Code, refKind + ":" + v.Code, v.Kind, v.Name, v.Address, v.Category, coords, u})
	}
	return out, nil
}

func effectiveShape(body []byte, u string) (map[string]json.RawMessage, error) {
	loc := shapePattern.FindIndex(body)
	if loc == nil {
		return nil, sourceErr(u, "NAVITIME page has no effective route parameters")
	}
	var shape map[string]json.RawMessage
	if json.NewDecoder(bytes.NewReader(body[loc[1]:])).Decode(&shape) != nil {
		return nil, sourceErr(u, "NAVITIME effective route parameters are malformed")
	}
	return shape, nil
}
func shapeString(shape map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(shape[key], &s)
	return s
}
func validateShape(shape map[string]json.RawMessage, q Query, u string) error {
	params, _, err := queryParams(q)
	if err != nil {
		return err
	}
	for _, entry := range []struct{ key, want string }{{"dateTime", params.Get("date_time")}, {"searchTimeMode", params.Get("search_time_mode")}, {"searchType", "transit"}} {
		if shapeString(shape, entry.key) != entry.want {
			return sourceErr(u, "NAVITIME ignored or changed the requested "+entry.key)
		}
	}
	for _, side := range []string{"start", "goal"} {
		want := params.Get(side)
		if want != "" {
			if shapeString(shape, side) != want {
				return sourceErr(u, "NAVITIME route endpoint identifier does not match request")
			}
		} else {
			var point struct {
				Spot string `json:"spot"`
			}
			if json.Unmarshal([]byte(shapeString(shape, side)), &point) != nil || point.Spot != params.Get(side+"Code") {
				return sourceErr(u, "NAVITIME POI resolution does not match the requested spot code")
			}
		}
	}
	var passes []string
	if json.Unmarshal(shape["passList"], &passes) != nil {
		return sourceErr(u, "NAVITIME effective pass parameters are missing")
	}
	if q.Pass == "" && len(passes) != 0 || q.Pass != "" && (len(passes) != 1 || passes[0] != q.Pass) {
		return sourceErr(u, "NAVITIME ignored or changed the requested pass")
	}
	return nil
}
func parseCatalog(root *html.Node) []Pass {
	containers := nodes(root, func(n *html.Node) bool { return attr(n, "id") == "pass-list" })
	out := []Pass{}
	if len(containers) == 0 {
		return out
	}
	labels := map[string]string{}
	for _, n := range nodes(root, func(n *html.Node) bool { return n.Data == "label" && attr(n, "for") != "" }) {
		labels[attr(n, "for")] = text(n)
	}
	seen := map[string]bool{}
	for _, n := range nodes(containers[0], func(n *html.Node) bool { return n.Data == "input" && attr(n, "type") == "checkbox" }) {
		id := attr(n, "name")
		name := labels[attr(n, "id")]
		if passPattern.MatchString(id) && name != "" && !seen[id] {
			seen[id] = true
			out = append(out, Pass{id, name, "source_advertised", id == "japan_rail_pass", 1})
		}
	}
	return out
}
func parseRoutes(body []byte, q Query, u string) ([]Route, []Pass, error) {
	lower := bytes.ToLower(body)
	if bytes.Contains(lower, []byte("token.awswaf.com")) || bytes.Contains(lower, []byte("gokuprops")) {
		return nil, nil, sourceErr(u, "NAVITIME returned a challenge rather than route data")
	}
	shape, err := effectiveShape(body, u)
	if err != nil {
		return nil, nil, err
	}
	if err = validateShape(shape, q, u); err != nil {
		return nil, nil, err
	}
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, nil, sourceErr(u, "NAVITIME route HTML could not be parsed")
	}
	passes := parseCatalog(root)
	if q.Pass != "" {
		found := false
		for _, p := range passes {
			if p.ID == q.Pass {
				found = true
			}
		}
		if !found {
			return nil, nil, sourceErr(u, "requested pass is not advertised by the returned source selector")
		}
	}
	out := []Route{}
	for _, item := range nodes(root, func(n *html.Node) bool { return n.Data == "li" && hasClass(n, "route") }) {
		if firstClass(item, "route-summary") == nil {
			continue
		}
		r, err := parseRoute(item, q, u)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, nil, sourceErr(u, "NAVITIME route page has no populated alternatives")
	}
	return out, passes, nil
}
func calendarSpan(item *html.Node, u string) (time.Time, time.Time, error) {
	var firstStart, firstEnd time.Time
	for _, n := range classNodes(item, "route-section-point__calendar") {
		parsed, err := url.Parse(attr(n, "href"))
		if err != nil {
			continue
		}
		span := parsed.Query().Get("dates")
		if span == "" {
			continue
		}
		pair := strings.Split(span, "/")
		if len(pair) != 2 {
			return time.Time{}, time.Time{}, sourceErr(u, "NAVITIME calendar span is invalid")
		}
		start, e1 := time.Parse("20060102T150405Z", pair[0])
		end, e2 := time.Parse("20060102T150405Z", pair[1])
		if e1 != nil || e2 != nil || end.Before(start) || end.Sub(start) > 7*24*time.Hour {
			return start, end, sourceErr(u, "NAVITIME calendar dates are inconsistent")
		}
		if !firstStart.IsZero() && (!start.Equal(firstStart) || !end.Equal(firstEnd)) {
			return start, end, sourceErr(u, "NAVITIME route contains contradictory calendar spans")
		}
		firstStart, firstEnd = start, end
	}
	if firstStart.IsZero() {
		return firstStart, firstEnd, sourceErr(u, "NAVITIME route has no calendar dates to anchor its timetable")
	}
	return firstStart.In(japan), firstEnd.In(japan), nil
}
func point(n *html.Node) Point {
	p := Point{Name: ct(n, "point-name"), Kind: nullable(strings.ToLower(ct(n, "point-type")))}
	vals := map[string]*float64{}
	for _, in := range nodes(n, func(v *html.Node) bool { return v.Data == "input" }) {
		key := attr(in, "name")
		if key == "point-lat" || key == "point-lon" {
			if f, e := strconv.ParseFloat(attr(in, "value"), 64); e == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && ((key == "point-lat" && f >= -90 && f <= 90) || (key == "point-lon" && f >= -180 && f <= 180)) {
				vals[key] = &f
			}
		}
		switch key {
		case "node", "node_id", "nodeId", "point-id", "point-node-id", "point-code":
			id := attr(in, "value")
			if stationPattern.MatchString(id) || spotPattern.MatchString(id) {
				p.SourceID = &id
			}
		}
	}
	if vals["point-lat"] != nil || vals["point-lon"] != nil {
		p.Coordinates = &Coordinates{vals["point-lat"], vals["point-lon"], nil, "degrees", "m"}
	}
	for _, link := range nodes(n, func(v *html.Node) bool { return v.Data == "a" && hasClass(v, "point-name") }) {
		u, err := url.Parse(attr(link, "href"))
		if err != nil {
			continue
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			if spotPattern.MatchString(last) {
				p.SpotID = &last
				if p.Kind != nil && *p.Kind == "spot" {
					p.SourceID = &last
				}
			} else if stationPattern.MatchString(last) {
				p.SourceID = &last
			}
		}
		for _, key := range []string{"node", "node_id", "nodeId", "station_id"} {
			id := u.Query().Get(key)
			if stationPattern.MatchString(id) {
				p.SourceID = &id
			}
		}
	}
	return p
}
func nextClock(raw string, last, end time.Time) (*string, time.Time, error) {
	if raw == "" {
		return nil, last, nil
	}
	m := clockPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return nil, last, fmt.Errorf("invalid source clock")
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if h > 47 || min > 59 {
		return nil, last, fmt.Errorf("invalid source clock")
	}
	t := time.Date(last.Year(), last.Month(), last.Day(), h%24, min, 0, 0, japan)
	if t.Before(last.Truncate(time.Minute)) {
		t = t.AddDate(0, 0, 1)
	}
	if t.After(end.Truncate(time.Minute)) {
		return nil, last, fmt.Errorf("source clock falls outside its route calendar span")
	}
	return ptr(t.Format(time.RFC3339)), t, nil
}
func parseRoute(item *html.Node, q Query, u string) (Route, error) {
	summary := firstClass(item, "route-summary")
	detail := firstClass(item, "route-detail")
	if detail == nil {
		return Route{}, sourceErr(u, "NAVITIME alternative has no leg detail")
	}
	dep, arr, err := calendarSpan(item, u)
	if err != nil {
		return Route{}, err
	}
	params, mode, _ := queryParams(q)
	requested, _ := time.ParseInLocation("2006-01-02T15:04", params.Get("date_time"), japan)
	if mode == "start-time" && dep.Before(requested) {
		return Route{}, sourceErr(u, "NAVITIME route departs before the requested departure bound")
	}
	if mode == "goal-time" && arr.After(requested) {
		return Route{}, sourceErr(u, "NAVITIME route arrives after the requested arrival bound")
	}
	fareText := ct(summary, "basic-fare")
	ic := money(ct(summary, "ic-fare"))
	transfers := number(ct(summary, "transfer-num"))
	if strings.EqualFold(ct(summary, "transfer-num"), "No Transfers") {
		transfers = ptr(0)
	}
	index := number(attr(summary, "data-route-num"))
	if index == nil {
		return Route{}, sourceErr(u, "NAVITIME alternative has no source position")
	}
	r := Route{IDProvenance: "local_content_hash", SourceIndex: *index, SourceURL: u, DepartureAt: ptr(dep.Format(time.RFC3339)), ArrivalAt: ptr(arr.Format(time.RFC3339)), TimestampProvenance: "source_calendar_utc_span", DurationMinutes: ptr(int(arr.Sub(dep).Minutes())), DurationSeconds: ptr(int(arr.Sub(dep).Seconds())), DurationMinutesBasis: "whole_minutes_floor_from_calendar_seconds", TransportKinds: []string{}, Transfers: transfers,
		Fare: Fare{TotalJPY: money(fareText), ICJPY: ic, Currency: "JPY", Basis: "published_cash_fare", SourceText: nullable(fareText)}, Legs: []Leg{}, FareGroups: []FareGroup{}, Pass: PassInfo{RequestedID: nullable(q.Pass), Coverage: "unknown", SourceTexts: []string{}}}
	sequence := nodes(detail, func(n *html.Node) bool {
		return hasClass(n, "route-section-point") || hasClass(n, "route-section-move")
	})
	last := dep
	groupNodes := []*html.Node{}
	groupIDs := map[*html.Node]string{}
	var previous *html.Node
	walking := 0
	walkKnown := true
	for i, n := range sequence {
		if hasClass(n, "route-section-point") {
			previous = n
			continue
		}
		var after *html.Node
		for _, v := range sequence[i+1:] {
			if hasClass(v, "route-section-point") {
				after = v
				break
			}
		}
		if previous == nil || after == nil || ct(previous, "point-name") == "" || ct(after, "point-name") == "" {
			return Route{}, sourceErr(u, "NAVITIME leg endpoints are missing")
		}
		line := ct(n, "line-name")
		kind := classifyMove(n, line)
		sourceDep, sourceArr := ct(n, "departure-time"), ct(n, "arrival-time")
		var departAt, arriveAt *string
		var legDep, legArr time.Time
		departAt, legDep, err = nextClock(sourceDep, last, arr)
		if err != nil {
			return Route{}, sourceErr(u, "NAVITIME departure clock contradicts calendar dates")
		}
		arriveAt, legArr, err = nextClock(sourceArr, legDep, arr)
		if err != nil {
			return Route{}, sourceErr(u, "NAVITIME arrival clock contradicts calendar dates")
		}
		last = legArr
		leg := Leg{Index: len(r.Legs) + 1, Kind: kind, From: point(previous), To: point(after), LineName: nullable(line), Service: nullable(ct(n, "destinations")), DepartureAt: departAt, ArrivalAt: arriveAt, TimestampProvenance: "source_clock_minute_precision_calendar_anchored", SourceDepartureText: nullable(sourceDep), SourceArrivalText: nullable(sourceArr), SeatFareJPY: money(ct(n, "express-fare__amount")), SeatOptions: []SeatOption{}}
		if departAt != nil && arriveAt != nil {
			leg.DurationMinutes = ptr(int(legArr.Sub(legDep).Minutes()))
		}
		for _, row := range nodes(n, func(v *html.Node) bool { return v.Data == "tr" }) {
			label := ct(row, "section-detail__label")
			value := ct(row, "section-detail__value")
			switch label {
			case "To":
				leg.Destination = nullable(value)
			case "Platform":
				leg.Platform = nullable(value)
			}
		}
		if kind == "unknown" {
			walkKnown = false
		}
		if kind == "walking" {
			leg.WalkingMeters = distance(ct(n, "distance"))
			if leg.WalkingMeters == nil {
				walkKnown = false
			} else {
				walking += *leg.WalkingMeters
			}
		}
		for _, selectNode := range classNodes(n, "seat-pick-row__select") {
			for _, option := range nodes(selectNode, func(v *html.Node) bool { return v.Data == "option" }) {
				selected := false
				for _, a := range option.Attr {
					if a.Key == "selected" {
						selected = true
					}
				}
				leg.SeatOptions = append(leg.SeatOptions, SeatOption{text(option), money(text(option)), selected})
			}
		}
		for parent := n.Parent; parent != nil && parent != detail; parent = parent.Parent {
			if hasClass(parent, "fare-group") {
				id, ok := groupIDs[parent]
				if !ok {
					id = fmt.Sprintf("fare-group-%d", len(groupNodes)+1)
					groupNodes = append(groupNodes, parent)
					groupIDs[parent] = id
				}
				leg.FareGroupID = ptr(id)
				break
			}
		}
		r.Legs = append(r.Legs, leg)
	}
	if len(r.Legs) == 0 {
		return Route{}, sourceErr(u, "NAVITIME alternative has no populated moves")
	}
	r.From = r.Legs[0].From
	r.To = r.Legs[len(r.Legs)-1].To
	fk, fi, _ := parseRef(q.From)
	tk, ti, _ := parseRef(q.To)
	r.From.SourceID = &fi
	r.To.SourceID = &ti
	if fk == "spot" {
		r.From.Kind = ptr("spot")
	}
	if tk == "spot" {
		r.To.Kind = ptr("spot")
	}
	r.Legs[0].From = r.From
	r.Legs[len(r.Legs)-1].To = r.To
	if r.Legs[0].DepartureAt != nil && *r.Legs[0].DepartureAt != dep.Truncate(time.Minute).Format(time.RFC3339) {
		return Route{}, sourceErr(u, "NAVITIME first leg contradicts route departure")
	}
	lastLeg := r.Legs[len(r.Legs)-1]
	if lastLeg.ArrivalAt != nil && *lastLeg.ArrivalAt != arr.Truncate(time.Minute).Format(time.RFC3339) {
		return Route{}, sourceErr(u, "NAVITIME final leg contradicts route arrival")
	}
	if walkKnown {
		r.WalkingMeters = ptr(walking)
	}
	for _, l := range r.Legs {
		r.TransportKinds = append(r.TransportKinds, l.Kind)
	}
	r.TransportKinds = unique(r.TransportKinds)
	r.TimingBasis = routeTimingBasis(r.TransportKinds)
	for _, kind := range r.TransportKinds {
		if kind == "car_taxi" {
			if r.Fare.TotalJPY != nil {
				r.Fare.Basis = "source_estimated_taxi_fare"
				r.Fare.SourceCaveat = ptr("Source marks this as an approximate taxi amount; actual taxi pricing and passenger assumptions are unknown.")
			} else {
				r.Fare.Basis = "unknown"
			}
			break
		}
	}
	for _, gn := range groupNodes {
		group := FareGroup{ID: groupIDs[gn], LegIndexes: []int{}, BaseFareJPY: oneMoney(texts(gn, "fare-group__fare-amount")), DisplayedTotalJPY: oneMoney(texts(gn, "section-total-row__fare")), ICJPY: oneMoney(texts(gn, "ic-fare")), SourceLabels: unique(append(texts(gn, "fare-group__fare-label"), texts(gn, "express-fare__label")...)), SourceTexts: unique(append(texts(gn, "fare-group__fare-amount"), texts(gn, "section-total-row__fare")...)), Basis: "source_through_fare_group"}
		seatTotal := 0
		seatKnown := true
		for _, leg := range r.Legs {
			if leg.FareGroupID != nil && *leg.FareGroupID == group.ID {
				group.LegIndexes = append(group.LegIndexes, leg.Index)
				if leg.SeatFareJPY == nil {
					seatKnown = false
				} else {
					seatTotal += *leg.SeatFareJPY
				}
			}
		}
		if seatKnown {
			group.DefaultSeatFareJPY = ptr(seatTotal)
		}
		r.FareGroups = append(r.FareGroups, group)
	}
	coverageLabels := []string{}
	for _, label := range classNodes(item, "special-pass-label") {
		coverageLabels = append(coverageLabels, passLabelText(label))
	}
	for _, n := range nodes(item, func(v *html.Node) bool { return v.Type == html.ElementNode }) {
		// Dedicated labels include image-only pass names. Ancestor/descendant
		// fragments are excluded from the generic warning collection.
		if firstClass(n, "special-pass-label") != nil || withinPassLabel(n) {
			continue
		}
		t := text(n)
		lower := strings.ToLower(t)
		classes := attr(n, "class")
		if len(t) > 0 && len(t) < 200 && ((strings.Contains(classes, "pass") && (strings.Contains(lower, "covered") || strings.Contains(lower, "supplement") || strings.Contains(lower, "not valid"))) || ((strings.Contains(lower, "nozomi") || strings.Contains(lower, "mizuho")) && (strings.Contains(lower, "extra fare") || strings.Contains(lower, "supplement") || strings.Contains(lower, "not covered")))) {
			r.Pass.SourceTexts = append(r.Pass.SourceTexts, t)
		}
	}
	r.Pass.SourceTexts = unique(append(minimalTexts(unique(r.Pass.SourceTexts)), coverageLabels...))
	if q.Pass != "" && len(r.Pass.SourceTexts) > 0 {
		r.Pass.Coverage = "source_coverage_labels_present"
	}
	payload, _ := json.Marshal(r)
	hash := sha256.Sum256(payload)
	r.ID = "local_" + hex.EncodeToString(hash[:12])
	return r, nil
}
func withinPassLabel(n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if hasClass(p, "special-pass-label") {
			return true
		}
	}
	return false
}

// Image alternatives are pass names only in this dedicated source block.
// Generic text extraction intentionally ignores promotional/decorative images.
func passLabelText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Data == "img" {
		return strings.TrimSpace(attr(n, "alt"))
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	parts := []string{}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, passLabelText(child))
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
func oneMoney(values []string) *int {
	var out *int
	for _, s := range values {
		v := money(s)
		if v == nil {
			continue
		}
		if out != nil && *out != *v {
			return nil
		}
		out = v
	}
	return out
}

var railLabel = regexp.MustCompile(`(?i)\b(?:nozomi|hikari|kodama|shinkansen|monorail|railway|subway|metro)\b|^JR\s`)
var busLabel = regexp.MustCompile(`(?i)\bbus\b`)
var ferryLabel = regexp.MustCompile(`(?i)\bferry\b`)

func classifyMove(n *html.Node, line string) string {
	if firstClass(n, "walk-summary") != nil || strings.EqualFold(line, "walk") {
		return "walking"
	}
	for _, img := range nodes(n, func(v *html.Node) bool { return v.Data == "img" }) {
		u, e := url.Parse(attr(img, "src"))
		if e == nil && u.Hostname() == "railroad-icon.common.navitime.jp" {
			return "rail"
		}
	}
	if busLabel.MatchString(line) {
		return "bus"
	}
	if railLabel.MatchString(line) {
		return "rail"
	}
	if ferryLabel.MatchString(line) {
		return "ferry"
	}
	switch strings.TrimSpace(strings.ToUpper(line)) {
	case "CAR/TAXI":
		return "car_taxi"
	case "ANA", "JAL", "ALL NIPPON AIRWAYS", "JAPAN AIRLINES":
		return "flight"
	}
	return "unknown"
}
func routeTimingBasis(kinds []string) string {
	road, walk, scheduled, unknown := false, false, false, false
	for _, k := range kinds {
		switch k {
		case "car_taxi":
			road = true
		case "walking":
			walk = true
		case "rail", "bus", "flight", "ferry":
			scheduled = true
		default:
			unknown = true
		}
	}
	switch {
	case unknown:
		return "mixed_or_unknown"
	case road && scheduled:
		return "mixed_scheduled_and_estimated"
	case road:
		return "estimated_road_travel"
	case scheduled && walk:
		return "scheduled_timetable_with_estimated_walking"
	case scheduled:
		return "scheduled_timetable"
	case walk:
		return "estimated_walking"
	default:
		return "unknown"
	}
}
func minimalTexts(values []string) []string {
	out := []string{}
	for _, s := range values {
		parent := false
		for _, other := range values {
			if s != other && strings.Contains(s, other) {
				parent = true
				break
			}
		}
		if !parent {
			out = append(out, s)
		}
	}
	return out
}
