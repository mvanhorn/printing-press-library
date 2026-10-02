// Package travelokacompare calculates explicitly scoped comparisons of public source quotes.
package travelokacompare

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"math/big"
	"sort"
	"strings"
)

func invalid(message string) error {
	return &traveloka.APIError{Code: "INVALID_INPUT", Message: message}
}

// ExactTotal returns a rational source total without currency conversion or floating point.
// An absent amount/scale is unknown; inconsistent known currency or malformed units is invalid.
func ExactTotal(m *traveloka.Money, currency string) (*big.Rat, error) {
	if m == nil {
		return nil, nil
	}
	if m.Currency != "" && m.Currency != currency {
		return nil, invalid("source total currency differs from query currency; conversion is unsupported")
	}
	if m.Currency == "" || m.Decimals == nil || m.MinorUnits == "" {
		return nil, nil
	}
	if *m.Decimals < 0 || *m.Decimals > 18 {
		return nil, invalid("source total decimal scale must be between 0 and 18")
	}
	rendered, e := traveloka.FormatMinorUnits(m.MinorUnits, *m.Decimals)
	if e != nil {
		return nil, invalid("source total has invalid exact minor units")
	}
	if m.Amount != "" && m.Amount != rendered {
		return nil, invalid("source total amount conflicts with its minor units and decimal scale")
	}
	n, _ := new(big.Int).SetString(m.MinorUnits, 10)
	return new(big.Rat).SetFrac(n, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(*m.Decimals)), nil)), nil
}

// Difference returns after minus before with the larger original decimal scale preserved.
func Difference(before, after *traveloka.Money, currency string) (*traveloka.Money, error) {
	a, e := ExactTotal(before, currency)
	if e != nil {
		return nil, e
	}
	b, e := ExactTotal(after, currency)
	if e != nil {
		return nil, e
	}
	if a == nil || b == nil {
		return nil, nil
	}
	scale := *before.Decimals
	if *after.Decimals > scale {
		scale = *after.Decimals
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	delta := new(big.Rat).Sub(b, a)
	delta.Mul(delta, new(big.Rat).SetInt(factor))
	if !delta.IsInt() {
		return nil, invalid("exact price difference cannot be represented at source scale")
	}
	raw := delta.Num().String()
	amount, e := traveloka.FormatMinorUnits(raw, scale)
	return &traveloka.Money{Currency: currency, MinorUnits: raw, Amount: amount, Decimals: &scale}, e
}

type OfferIssue struct {
	Offer  traveloka.Offer `json:"offer"`
	Reason string          `json:"reason"`
}
type FrontierResult struct {
	Offers        []traveloka.Offer `json:"offers"`
	Unknown       []OfferIssue      `json:"unknown_dimensions"`
	Scanned       int               `json:"scanned_offers"`
	FrontierCount int               `json:"frontier_count"`
	Dominated     int               `json:"dominated_offers"`
	Truncated     bool              `json:"truncated"`
	Note          string            `json:"note"`
}

func basis(kind string) string {
	if kind == "flights" {
		return "party_trip_total"
	}
	return "party_stay_total"
}
func checkBasis(o traveloka.Offer, kind string) error {
	if v, ok := o.Details["price_basis"].(string); ok && v != "" && v != basis(kind) {
		return invalid("source price_basis differs from the expected trip/stay total unit")
	}
	return nil
}
func bounds(limit, scan int) error {
	if limit < 1 || limit > 100 || scan < 1 || scan > 1000 {
		return invalid("--limit must be 1..100 and --max-scan-records must be 1..1000")
	}
	return nil
}

// Frontier keeps every nondominated tie across price, stops and known source duration.
// The frontier is scoped to the scanned snapshot prefix, never to all upstream inventory.
func Frontier(s *traveloka.Snapshot, limit, scan int) (FrontierResult, error) {
	r := FrontierResult{Offers: []traveloka.Offer{}, Unknown: []OfferIssue{}}
	if e := bounds(limit, scan); e != nil {
		return r, e
	}
	if s == nil || s.Kind != "flights" || s.Query.Kind != "flights" {
		return r, invalid("shortlist requires a flight snapshot")
	}
	n := len(s.Offers)
	if n > scan {
		n = scan
		r.Truncated = true
	}
	known := []traveloka.Offer{}
	prices := []*big.Rat{}
	for _, o := range s.Offers[:n] {
		if e := checkBasis(o, s.Kind); e != nil {
			return r, e
		}
		p, e := ExactTotal(o.Price.Total, s.Query.Currency)
		if e != nil {
			return r, e
		}
		reasons := []string{}
		if p == nil {
			reasons = append(reasons, "unknown_exact_total")
		}
		if o.Details["price_basis"] != basis(s.Kind) {
			reasons = append(reasons, "unknown_price_basis")
		}
		if o.Stops == nil || *o.Stops < 0 {
			reasons = append(reasons, "unknown_stops")
		}
		if o.DurationMinutes == nil || *o.DurationMinutes <= 0 {
			reasons = append(reasons, "unknown_elapsed_duration")
		}
		if o.Kind != "flight" {
			reasons = append(reasons, "unknown_product_kind")
		}
		if len(reasons) > 0 {
			r.Unknown = append(r.Unknown, OfferIssue{o, strings.Join(reasons, ",")})
			continue
		}
		known = append(known, o)
		prices = append(prices, p)
	}
	for i, a := range known {
		dominated := false
		for j, b := range known {
			if i == j {
				continue
			}
			cmp := prices[j].Cmp(prices[i])
			if cmp <= 0 && *b.Stops <= *a.Stops && *b.DurationMinutes <= *a.DurationMinutes && (cmp < 0 || *b.Stops < *a.Stops || *b.DurationMinutes < *a.DurationMinutes) {
				dominated = true
				break
			}
		}
		if dominated {
			r.Dominated++
		} else {
			r.Offers = append(r.Offers, a)
		}
	}
	r.Scanned = n
	r.FrontierCount = len(r.Offers)
	if len(r.Offers) > limit {
		r.Offers = r.Offers[:limit]
		r.Truncated = true
	}
	if len(r.Unknown) > limit {
		r.Unknown = r.Unknown[:limit]
		r.Truncated = true
	}
	r.Note = "Frontier covers only scanned retrieved offers; ties are retained before output limits. Missing dimensions are not ranked. Increase --max-scan-records or --limit to widen local output."
	return r, nil
}

type FlexPair struct {
	Refundable    traveloka.Offer  `json:"refundable"`
	Nonrefundable traveloka.Offer  `json:"nonrefundable"`
	Difference    *traveloka.Money `json:"refundable_minus_nonrefundable"`
}
type FlexResult struct {
	Pairs     []FlexPair   `json:"pairs"`
	Unpaired  []OfferIssue `json:"unpaired"`
	Scanned   int          `json:"scanned_offers"`
	PairCount int          `json:"pair_count"`
	Truncated bool         `json:"truncated"`
	Note      string       `json:"note"`
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func canonical(v any) string      { b, _ := json.Marshal(v); return string(b) }
func known(v any) bool            { return v != nil && v != "" }
func sourceInteger(v any) (*big.Int, bool) {
	var raw string
	switch n := v.(type) {
	case json.Number:
		raw = string(n)
	case string:
		raw = n
	case int:
		raw = fmt.Sprint(n)
	case int64:
		raw = fmt.Sprint(n)
	default:
		return nil, false
	}
	if raw == "" || strings.TrimSpace(raw) != raw {
		return nil, false
	}
	for i, r := range raw {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r < '0' || r > '9' {
			return nil, false
		}
	}
	n, ok := new(big.Int).SetString(raw, 10)
	return n, ok
}
func flexKey(q traveloka.Query, o traveloka.Offer) (string, string) {
	if o.Kind != "room" || o.PropertyID == "" || o.RoomID == "" || o.PropertyID != q.PropertyID {
		return "", "missing_or_mismatched_room_identity"
	}
	if q.Adults <= 0 || q.Rooms <= 0 || q.Children < 0 || len(q.ChildAges) != q.Children || q.CheckIn == "" || q.CheckOut == "" {
		return "", "unknown_query_occupancy_or_stay"
	}
	if o.OccupancyMatch == nil {
		return "", "unknown_occupancy_match"
	}
	if !*o.OccupancyMatch {
		return "", "mismatched_occupancy"
	}
	if o.Refundable == nil {
		return "", "unknown_refundable_classification"
	}
	rate := object(o.Details["rate"])
	for _, field := range []string{"maxOccupancy", "maxChildOccupancy", "ccGuaranteeRequirement", "bookingPolicy"} {
		if !known(rate[field]) {
			return "", "unknown_" + field
		}
	}
	capacity, capKnown := sourceInteger(rate["maxOccupancy"])
	childCapacity, childKnown := sourceInteger(rate["maxChildOccupancy"])
	if !capKnown || capacity.Sign() <= 0 {
		return "", "unknown_maxOccupancy"
	}
	if !childKnown || childCapacity.Sign() < 0 {
		return "", "unknown_maxChildOccupancy"
	}
	chargedValue, chargedPresent := rate["numChargedRooms"]
	if !chargedPresent {
		chargedValue = object(o.Details["room"])["numChargedRooms"]
	}
	rooms, roomsKnown := sourceInteger(chargedValue)
	if !roomsKnown {
		return "", "unknown_numChargedRooms"
	}
	if rooms.Cmp(big.NewInt(int64(q.Rooms))) != 0 {
		return "", "mismatched_charged_rooms"
	}
	if o.MealPlan == "" {
		return "", "unknown_meal_plan"
	}
	if o.Payment == "" {
		return "", "unknown_payment_type"
	}
	if o.Details["price_basis"] != "party_stay_total" {
		return "", "unknown_price_basis"
	}
	terms := map[string]any{}
	// Include all exposed meal/payment/occupancy terms. Missing required terms above
	// remain unknown; missing optional terms are retained as presence markers.
	for _, field := range []string{"maxOccupancy", "maxChildOccupancy", "numChargedRooms", "mealPlanDisplay", "includedMealTypes", "displayNumBreakfastIncluded", "numBreakfastIncluded", "isBreakfastIncluded", "extraBedIsIncluded", "childOccupancyPolicyDisplay", "ccGuaranteeRequirement", "bookingPolicy", "paymentInfo", "paymentDisplay", "creditCardGuarantee", "ccGuaranteeOptions", "hotelRoomCCGuaranteeRequirementDisplay", "flexiPaySummary"} {
		v, present := rate[field]
		terms[field] = map[string]any{"present": present, "value": v}
	}
	// Explicit source integer strings and JSON numbers represent the same exact
	// occupancy quantity. Original representations remain in the returned offers.
	terms["maxOccupancy"] = capacity.String()
	terms["maxChildOccupancy"] = childCapacity.String()
	terms["numChargedRooms"] = rooms.String()
	return canonical([]any{q.ContextKey(), o.PropertyID, o.RoomID, o.MealPlan, o.Payment, terms, o.Price.TaxInclusion}), ""
}

// Flexibility pairs only explicit refundable opposites under exact occupancy/meal/payment keys.
func Flexibility(s *traveloka.Snapshot, limit, scan int) (FlexResult, error) {
	r := FlexResult{Pairs: []FlexPair{}, Unpaired: []OfferIssue{}}
	if e := bounds(limit, scan); e != nil {
		return r, e
	}
	if s == nil || s.Kind != "rooms" || s.Query.Kind != "rooms" {
		return r, invalid("flexibility requires a room/rate-plan snapshot")
	}
	n := len(s.Offers)
	if n > scan {
		n = scan
		r.Truncated = true
	}
	groups := map[string][]traveloka.Offer{}
	keys := []string{}
	for _, o := range s.Offers[:n] {
		if e := checkBasis(o, s.Kind); e != nil {
			return r, e
		}
		p, e := ExactTotal(o.Price.Total, s.Query.Currency)
		if e != nil {
			return r, e
		}
		key, reason := flexKey(s.Query, o)
		if p == nil {
			reason = "unknown_exact_stay_total"
		}
		if reason != "" {
			r.Unpaired = append(r.Unpaired, OfferIssue{o, reason})
			continue
		}
		if _, exists := groups[key]; !exists {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], o)
	}
	sort.Strings(keys)
	for _, key := range keys {
		yes, no := []traveloka.Offer{}, []traveloka.Offer{}
		for _, o := range groups[key] {
			if *o.Refundable {
				yes = append(yes, o)
			} else {
				no = append(no, o)
			}
		}
		if len(yes) == 0 || len(no) == 0 {
			for _, o := range groups[key] {
				r.Unpaired = append(r.Unpaired, OfferIssue{o, "no_opposite_refundability_with_exact_matching_terms"})
			}
			continue
		}
		for _, a := range yes {
			for _, b := range no {
				r.PairCount++
				if len(r.Pairs) < limit {
					d, e := Difference(b.Price.Total, a.Price.Total, s.Query.Currency)
					if e != nil {
						return r, e
					}
					r.Pairs = append(r.Pairs, FlexPair{a, b, d})
				}
			}
		}
	}
	r.Scanned = n
	if r.PairCount > limit {
		r.Truncated = true
	}
	if len(r.Unpaired) > limit {
		r.Unpaired = r.Unpaired[:limit]
		r.Truncated = true
	}
	r.Note = "Only source-confirmed opposite cancellation classifications with exact matching terms are paired. Source cancellation deadlines and unknown timezones remain unchanged; differences are source stay totals, not a refund guarantee."
	return r, nil
}

type FieldChange struct {
	Before any `json:"before"`
	After  any `json:"after"`
}
type Change struct {
	Identity                    string                 `json:"identity"`
	Before                      traveloka.Offer        `json:"before"`
	After                       traveloka.Offer        `json:"after"`
	Fields                      map[string]FieldChange `json:"fields"`
	Difference                  *traveloka.Money       `json:"after_minus_before"`
	DifferenceUnavailableReason string                 `json:"difference_unavailable_reason,omitempty"`
}
type DiffResult struct {
	Changes         []Change          `json:"changes"`
	NotReturned     []traveloka.Offer `json:"not_returned"`
	NewReturned     []traveloka.Offer `json:"new_returned"`
	UnmatchedBefore []OfferIssue      `json:"unmatched_before"`
	UnmatchedAfter  []OfferIssue      `json:"unmatched_after"`
	ScannedBefore   int               `json:"scanned_before"`
	ScannedAfter    int               `json:"scanned_after"`
	Matched         int               `json:"matched_offers"`
	Unchanged       int               `json:"unchanged_offers"`
	ChangeCount     int               `json:"change_count"`
	Truncated       bool              `json:"truncated"`
	Note            string            `json:"note"`
}

func identity(o traveloka.Offer, kind string) (string, string) {
	if o.ID == "" {
		return "", "missing_source_offer_id"
	}
	if o.Details["price_basis"] != basis(kind) {
		return "", "unknown_price_basis"
	}
	if kind == "flights" {
		if o.Kind != "flight" || len(o.Legs) == 0 {
			return "", "incomplete_flight_identity"
		}
		ids := []any{o.ID}
		for _, l := range o.Legs {
			id, ok := l.Details["id"].(string)
			if !ok || id == "" || len(l.Segments) == 0 {
				return "", "incomplete_leg_identity"
			}
			segments := []any{}
			for _, s := range l.Segments {
				if s.Origin == "" || s.Destination == "" || s.DepartureDate == "" || s.ArrivalDate == "" || s.DepartureTime == "" || s.ArrivalTime == "" || s.DepartureUTCOffsetMinutes == nil || s.ArrivalUTCOffsetMinutes == nil || s.MarketingAirline == "" || s.FlightNumber == "" || s.Cabin == "" {
					return "", "incomplete_segment_identity"
				}
				segments = append(segments, []any{s.Origin, s.Destination, s.DepartureDate, s.DepartureTime, s.ArrivalDate, s.ArrivalTime, *s.DepartureUTCOffsetMinutes, *s.ArrivalUTCOffsetMinutes, s.MarketingAirline, s.OperatingAirline, s.FlightNumber, s.Cabin})
			}
			ids = append(ids, []any{id, segments})
		}
		return canonical(ids), ""
	}
	if kind == "rooms" {
		if o.Kind != "room" || o.PropertyID == "" || o.RoomID == "" {
			return "", "incomplete_room_inventory_identity"
		}
		return canonical([]string{o.ID, o.PropertyID, o.RoomID}), ""
	}
	if kind == "hotels" {
		if o.Kind != "hotel" || o.PropertyID == "" {
			return "", "incomplete_property_identity"
		}
		return canonical([]string{o.ID, o.PropertyID}), ""
	}
	return "", "unsupported_product_kind"
}
func policyFields(v any, fields ...string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for _, key := range fields {
		if value, present := m[key]; present {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
func baggagePolicy(v any) any {
	if _, ok := v.(map[string]any); !ok {
		return v
	}
	return policyFields(v, "quantity", "weight", "unitOfMeasure", "height", "width", "length", "copyText", "copyDescription", "availableToBuy", "allowance", "allowanceText")
}
func farePolicies(v map[string]any) any {
	routes, ok := v["routes"].([]any)
	if !ok {
		return nil
	}
	out := []any{}
	for _, raw := range routes {
		route := object(raw)
		out = append(out, []any{
			policyFields(route["flightRefundInfo"], "refundableStatus", "refundInfoSummary", "refundInfoDetail", "policyDetails"),
			policyFields(route["flightRescheduleInfo"], "rescheduleStatus", "rescheduleInfoSummary", "rescheduleInfoDetail", "rescheduleUnknownReason", "policyDetails"),
		})
	}
	return out
}
func policy(o traveloka.Offer) any {
	legs := []any{}
	for _, l := range o.Legs {
		segs := []any{}
		for _, s := range l.Segments {
			facilities := object(s.Details["facilities"])
			segs = append(segs, []any{baggagePolicy(s.CheckedBaggage), baggagePolicy(s.CabinBaggage), policyFields(s.Details, "mayReCheckIn", "visaRequired"),
				policyFields(facilities, "isPersonalItemIncluded", "personalItemIncluded"), baggagePolicy(facilities["personalItem"])})
		}
		metadata := object(l.Details["flight_metadata"])
		terms := map[string]any{}
		for _, k := range []string{"isRefundable", "isReschedulable", "isSelfTransfer"} {
			v, present := metadata[k]
			terms[k] = map[string]any{"present": present, "value": v}
		}
		legs = append(legs, []any{farePolicies(l.FareRules), terms, segs})
	}
	rate := object(o.Details["rate"])
	terms := map[string]any{}
	for _, k := range []string{"maxOccupancy", "maxChildOccupancy", "numChargedRooms", "mealPlanDisplay", "includedMealTypes", "isBreakfastIncluded", "displayNumBreakfastIncluded", "ccGuaranteeRequirement", "bookingPolicy", "paymentDisplay", "paymentInfo", "creditCardGuarantee", "ccGuaranteeOptions", "flexiPaySummary", "originalCancellationPolicy", "roomInventoryGroupCancellationPolicy", "cancellationPolicyDisplay", "childOccupancyPolicyDisplay"} {
		v, present := rate[k]
		terms[k] = map[string]any{"present": present, "value": v}
	}
	var catalogTerms map[string]any
	if o.Kind == "hotel" {
		catalogTerms = map[string]any{}
		inventory := object(o.Details["inventory"])
		for _, k := range []string{"matchSearchOccupancy", "numChargedRooms", "availableRateTypes", "hasFreeCancellationRooms", "hotelRoomCCGuaranteeRequirementDisplay"} {
			v, present := inventory[k]
			catalogTerms[k] = map[string]any{"present": present, "value": v}
		}
	}
	return []any{o.Refundable, o.Reschedulable, o.Cancellation, o.MealPlan, o.Payment, o.OccupancyMatch, legs, terms, catalogTerms}
}

// Diff compares exact identities only. UUIDs that change across room retrievals stay unmatched.
// It never guesses equivalent offers from names and never equates omission with sold-out inventory.
func Diff(before, after *traveloka.Snapshot, limit, scan int) (DiffResult, error) {
	r := DiffResult{Changes: []Change{}, NotReturned: []traveloka.Offer{}, NewReturned: []traveloka.Offer{}, UnmatchedBefore: []OfferIssue{}, UnmatchedAfter: []OfferIssue{}}
	if e := bounds(limit, scan); e != nil {
		return r, e
	}
	if before == nil || after == nil || before.Kind != after.Kind || before.Query.Kind != before.Kind || after.Query.Kind != after.Kind || !before.Query.SameContext(after.Query) {
		return r, invalid("before/after snapshots must have identical kind, market, locale, currency, dates, party and travel context")
	}
	if before.Kind != "flights" && before.Kind != "rooms" && before.Kind != "hotels" {
		return r, invalid("unsupported snapshot kind for quote diff")
	}
	sides := []*traveloka.Snapshot{before, after}
	maps := []map[string][]traveloka.Offer{{}, {}}
	for i, s := range sides {
		n := len(s.Offers)
		if n > scan {
			n = scan
			r.Truncated = true
		}
		if i == 0 {
			r.ScannedBefore = n
		} else {
			r.ScannedAfter = n
		}
		for _, o := range s.Offers[:n] {
			if e := checkBasis(o, s.Kind); e != nil {
				return r, e
			}
			if _, e := ExactTotal(o.Price.Total, s.Query.Currency); e != nil {
				return r, e
			}
			key, reason := identity(o, s.Kind)
			if reason != "" {
				issue := OfferIssue{o, reason}
				if i == 0 {
					r.UnmatchedBefore = append(r.UnmatchedBefore, issue)
				} else {
					r.UnmatchedAfter = append(r.UnmatchedAfter, issue)
				}
				continue
			}
			maps[i][key] = append(maps[i][key], o)
		}
	}
	keys := []string{}
	seen := map[string]bool{}
	for _, m := range maps {
		for key := range m {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		a, b := maps[0][key], maps[1][key]
		if len(a) > 1 || len(b) > 1 {
			for _, o := range a {
				r.UnmatchedBefore = append(r.UnmatchedBefore, OfferIssue{o, "duplicate_exact_identity"})
			}
			for _, o := range b {
				r.UnmatchedAfter = append(r.UnmatchedAfter, OfferIssue{o, "duplicate_exact_identity"})
			}
			continue
		}
		if len(a) == 0 {
			r.NewReturned = append(r.NewReturned, b[0])
			continue
		}
		if len(b) == 0 {
			r.NotReturned = append(r.NotReturned, a[0])
			continue
		}
		r.Matched++
		fields := map[string]FieldChange{}
		if canonical(a[0].Price) != canonical(b[0].Price) {
			fields["price"] = FieldChange{a[0].Price, b[0].Price}
		}
		if canonical(policy(a[0])) != canonical(policy(b[0])) {
			fields["policies"] = FieldChange{policy(a[0]), policy(b[0])}
		}
		if len(fields) == 0 {
			r.Unchanged++
			continue
		}
		change := Change{Identity: key, Before: a[0], After: b[0], Fields: fields}
		if a[0].Price.TaxInclusion != b[0].Price.TaxInclusion {
			change.DifferenceUnavailableReason = "incompatible_tax_inclusion"
		} else {
			d, e := Difference(a[0].Price.Total, b[0].Price.Total, before.Query.Currency)
			if e != nil {
				return r, e
			}
			change.Difference = d
		}
		r.Changes = append(r.Changes, change)
	}
	r.ChangeCount = len(r.Changes)
	// Each explicitly named result section is independently bounded.
	if len(r.Changes) > limit {
		r.Changes = r.Changes[:limit]
		r.Truncated = true
	}
	if len(r.NotReturned) > limit {
		r.NotReturned = r.NotReturned[:limit]
		r.Truncated = true
	}
	if len(r.NewReturned) > limit {
		r.NewReturned = r.NewReturned[:limit]
		r.Truncated = true
	}
	if len(r.UnmatchedBefore) > limit {
		r.UnmatchedBefore = r.UnmatchedBefore[:limit]
		r.Truncated = true
	}
	if len(r.UnmatchedAfter) > limit {
		r.UnmatchedAfter = r.UnmatchedAfter[:limit]
		r.Truncated = true
	}
	r.Note = "Exact source identities only; changed inventory IDs are not matched by names. Not returned indicates absence from this retrieval; no availability inference. Each result section and the scanned snapshot prefix are bounded."
	return r, nil
}
