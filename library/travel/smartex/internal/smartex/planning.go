package smartex

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, JST)
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, fmt.Errorf("--date must be a valid YYYY-MM-DD calendar date in JST")
	}
	return t, nil
}
func ParseNow(s string) (time.Time, error) {
	if s == "" {
		return time.Now().In(JST), nil
	}
	t, e := time.Parse(time.RFC3339, s)
	if e != nil {
		return t, fmt.Errorf("--now must be RFC3339 with an explicit UTC offset")
	}
	return t.In(JST), nil
}
func ValidateParty(adults, children int) error {
	if adults < 0 || children < 0 || adults+children < 1 || adults+children > 6 {
		return fmt.Errorf("--adults and --children must be nonnegative and total 1–6 per booking operation")
	}
	return nil
}
func ValidClass(s string) bool { return s == "reserved" || s == "unreserved" || s == "green" }
func ValidTrain(s string) bool {
	for _, v := range []string{"nozomi", "hikari", "kodama", "mizuho", "sakura", "tsubame"} {
		if s == v {
			return true
		}
	}
	return false
}

// JR's missing corresponding day rule is first day of the travel month,
// e.g. May31 opens May1, rather than clamping to April30.
func MonthBefore(t time.Time) time.Time {
	p := time.Date(t.Year(), t.Month()-1, 1, 0, 0, 0, 0, JST)
	last := time.Date(p.Year(), p.Month()+1, 0, 0, 0, 0, 0, JST).Day()
	if t.Day() > last {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, JST)
	}
	return time.Date(p.Year(), p.Month(), t.Day(), 0, 0, 0, 0, JST)
}
func at(t time.Time, h, m int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, JST)
}
func ts(t time.Time) string { return t.In(JST).Format(time.RFC3339) }

type Product struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	AdvanceDays          int      `json:"advance_days"`
	MinParty             int      `json:"minimum_party"`
	MaxParty             int      `json:"maximum_party"`
	Corridors            []string `json:"corridors"`
	Facilities           []string `json:"facilities"`
	TrainRules           []string `json:"train_rules"`
	ChangeRule           string   `json:"change_rule"`
	ChildPricesSupported bool     `json:"child_prices_exist"`
	PriceJPY             any      `json:"price_jpy"`
	SourceURL            string   `json:"source_url"`
}

var Products = []Product{
	{"smart-ex", "smartEX Service", 0, 1, 6, []string{"tokaido", "sanyo", "kyushu"}, []string{"reserved", "unreserved", "green"}, []string{"Nozomi/Hikari/Kodama/Mizuho/Sakura/Tsubame; actual stops and class availability vary"}, "Changes before reserved departure, ticket pickup or gate entry; fare differences and date limits apply", true, nil, Sources[0].URL},
	{"hayatoku1", "EX Hayatoku 1", 1, 1, 6, []string{"tokaido"}, []string{"unreserved"}, []string{"Hikari or Kodama; limited listed Tokaido sections"}, "Change only to a currently purchasable product; fare difference applies; consult product exceptions", true, nil, "https://smart-ex.jp/en/product/hayatoku1/"},
	{"hayatoku3", "EX Hayatoku 3", 3, 1, 6, []string{"tokaido", "sanyo", "kyushu"}, []string{"green", "reserved_in_kyushu", "unreserved_tsubame_in_hakata_transfer"}, []string{"Tokaido direct: Nozomi/Hikari/Kodama Green; Tokaido–Sanyo/Sanyo: direct Nozomi Green", "Sanyo–Kyushu direct: Mizuho/Sakura Green; Hakata transfer has product-specific train/class rules", "Kyushu direct: Mizuho/Sakura/Tsubame ordinary reserved or Green"}, "Usually change while advance conditions remain satisfied; same-day exceptions are product-specific", true, nil, "https://smart-ex.jp/en/product/hayatoku3/"},
	{"hayatoku7", "EX Hayatoku 7", 7, 1, 6, []string{"tokaido", "sanyo", "kyushu"}, []string{"reserved", "unreserved_tsubame_in_hakata_transfer"}, []string{"Tokaido direct: Hikari/Kodama ordinary reserved; Tokaido–Sanyo/Sanyo: direct Nozomi ordinary reserved", "Sanyo–Kyushu: Mizuho/Sakura ordinary reserved; Hakata transfer has product-specific rules"}, "Usually change while advance conditions remain satisfied; same-day exceptions are product-specific", true, nil, "https://smart-ex.jp/en/product/hayatoku7/"},
	{"hayatoku21", "EX Hayatoku 21", 21, 1, 6, []string{"tokaido", "sanyo", "kyushu"}, []string{"reserved", "unreserved_tsubame_in_hakata_transfer"}, []string{"Tokaido–Sanyo: direct Nozomi ordinary reserved", "Sanyo–Kyushu: direct Mizuho/Sakura ordinary reserved; Hakata-transfer rules expanded March13,2026"}, "Refund and rebook to change train/date/route; seat position may change on same train/facility/price", true, nil, "https://smart-ex.jp/en/product/hayatoku21/"},
	{"family7", "EX Family Hayatoku 7", 7, 2, 6, []string{"tokaido"}, []string{"reserved"}, []string{"Direct Hikari/Kodama ordinary reserved; no transfers; at least two travelers"}, "Advance and same-day restrictions apply; party cannot be reduced below two", true, nil, "https://smart-ex.jp/en/product/hayatoku_family7/"},
}

func ProductByID(id string) (Product, error) {
	for _, p := range Products {
		if p.ID == id {
			return p, nil
		}
	}
	return Product{}, fmt.Errorf("--product must be smart-ex, hayatoku1, hayatoku3, hayatoku7, hayatoku21 or family7; round-trip ended 2026-03-31")
}

type Window struct {
	ProductID              string            `json:"product_id"`
	BoardingDate           string            `json:"boarding_date_jst"`
	Now                    string            `json:"now_jst"`
	Opens                  any               `json:"opens_jst"`
	Closes                 any               `json:"closes_jst"`
	WindowStatus           string            `json:"window_status"`
	StandardSeatSales      string            `json:"standard_seat_sales_open_jst"`
	AdvanceRequestEnds     string            `json:"advance_request_ends_jst"`
	Confirmation           any               `json:"train_and_seat_confirmation_jst"`
	ConfirmationSources    map[string]string `json:"confirmation_sources"`
	AfterHours             bool              `json:"after_hours"`
	MaxPartyNow            int               `json:"maximum_party_per_operation_now"`
	PartyFits              any               `json:"party_fits_one_operation_now"`
	SeatMapAvailable       bool              `json:"seat_map_available_now"`
	OneYearRequestEligible any               `json:"one_year_request_eligible"`
	Class                  string            `json:"class"`
	ReservationPermitted   any               `json:"reservation_operations_permitted_by_calendar"`
	OversizedSourceRules   map[string]string `json:"oversized_source_rules,omitempty"`
	Availability           any               `json:"availability"`
	Notes                  []string          `json:"notes"`
	Sources                []Source          `json:"sources"`
}

func BookingWindow(date, product, departure string, now time.Time, adults, children int, oversized bool, class string) (Window, error) {
	if e := ValidateParty(adults, children); e != nil {
		return Window{}, e
	}
	if class == "" {
		class = "reserved"
	}
	if !ValidClass(class) || (oversized && class == "unreserved") {
		return Window{}, fmt.Errorf("--class must be reserved, unreserved or green; oversized-area seating requires reserved or green")
	}
	d, e := ParseDate(date)
	if e != nil {
		return Window{}, e
	}
	p, e := ProductByID(product)
	if e != nil {
		return Window{}, e
	}
	month := MonthBefore(d)
	sale := at(month, 10, 0)
	w := Window{ProductID: p.ID, Class: class, BoardingDate: date, Now: ts(now), StandardSeatSales: ts(sale), AdvanceRequestEnds: ts(at(month, 7, 30)), MaxPartyNow: 6, OneYearRequestEligible: p.ID == "smart-ex", WindowStatus: "unknown", Sources: []Source{Sources[1], Sources[2]}, Notes: []string{"Window status checks calendar rules, not product route eligibility or available seats", "Maintenance, card authorization and product exclusions can prevent a reservation", "Seat map availability here means calendar/class permission only, not a live seat map or available seat"}, ConfirmationSources: map[string]string{"current_japanese_advance_page": ts(at(month, 8, 0)), "english_accept_time_page": ts(at(month, 14, 0))}}
	w.Notes = append(w.Notes, "Official Japanese and English pages disagree on confirmation time (08:00 versus14:00 JST); exact completion time is unknown; check booking email/My Trips")
	if oversized {
		w.MaxPartyNow = 5
		if class == "green" {
			w.MaxPartyNow = 4
		}
		w.OversizedSourceRules = map[string]string{
			"english":          "Oversized seats are excluded from one-year, pre-sale and after-hours requests",
			"current_japanese": "Advance guidance excludes some train sections/facilities; current oversized guidance does not categorically exclude after-hours requests",
		}
		w.Sources = append(w.Sources, Sources[6], Sources[11], Sources[12])
		w.Notes = append(w.Notes, "Oversized-area seats allow at most five ordinary or four Green passengers in one operation; no cross-car grouping, and train-specific exceptions require confirmation")
	}
	mins := now.In(JST).Hour()*60 + now.In(JST).Minute()
	w.AfterHours = mins >= 23*60+30 || mins < 5*60+30
	if w.AfterHours {
		w.MaxPartyNow = 3
	}
	w.PartyFits = adults+children <= w.MaxPartyNow
	var open, close time.Time
	if p.ID == "smart-ex" {
		open = at(d.AddDate(-1, 0, 0), 5, 30)
		if d.Month() == time.February && d.Day() == 29 {
			w.Opens = nil
			w.Notes = append(w.Notes, "One-year opening for February29 lacks a documented corresponding-day rule; confirm with smartEX")
			open = time.Time{}
		}
		if oversized {
			open = time.Time{}
			w.OneYearRequestEligible = nil
			w.Notes = append(w.Notes, "Official Japanese and English guidance differs on oversized one-year eligibility; actual eligibility and earliest opening are unknown. Standard one-month seat sales are the known fallback")
		}
		if departure != "" {
			clock, e := time.Parse("15:04", departure)
			if e != nil {
				return Window{}, fmt.Errorf("--departure must be HH:MM in JST")
			}
			close = at(d, clock.Hour(), clock.Minute()).Add(-4 * time.Minute)
		} else {
			w.Notes = append(w.Notes, "Provide --departure HH:MM for the four-minute purchase cutoff")
		}
	} else if p.ID == "hayatoku1" {
		open = at(d.AddDate(0, 0, -6), 0, 0)
		close = at(d.AddDate(0, 0, -1), 23, 30)
	} else {
		open = sale
		close = at(d.AddDate(0, 0, -p.AdvanceDays), 23, 30)
	}
	if !open.IsZero() {
		w.Opens = ts(open)
	}
	if !close.IsZero() {
		w.Closes = ts(close)
	}
	if now.After(at(d, 23, 59).Add(time.Minute - time.Nanosecond)) {
		w.WindowStatus = "travel_date_passed"
	} else if !close.IsZero() && now.After(close) {
		w.WindowStatus = "closed"
	} else if !open.IsZero() && now.Before(open) {
		w.WindowStatus = "not_open"
	} else if !open.IsZero() {
		w.WindowStatus = "within_calendar_window"
	}
	if p.ID == "smart-ex" && w.WindowStatus == "within_calendar_window" && now.Before(sale) && !now.Before(at(month, 7, 30)) {
		w.WindowStatus = "confirmation_processing_gap"
	}
	if p.ID == "smart-ex" && w.WindowStatus == "unknown" {
		if oversized {
			w.WindowStatus = "advance_eligibility_requires_confirmation"
		}
		if !now.Before(sale) {
			w.WindowStatus = "within_calendar_window"
		} else if !now.Before(at(month, 7, 30)) {
			w.WindowStatus = "confirmation_processing_gap"
		}
	}
	w.ReservationPermitted = w.WindowStatus == "within_calendar_window"
	if w.WindowStatus == "unknown" || w.WindowStatus == "advance_eligibility_requires_confirmation" {
		w.ReservationPermitted = nil
	}
	if oversized && w.AfterHours && w.WindowStatus != "closed" && w.WindowStatus != "travel_date_passed" {
		w.WindowStatus = "after_hours_eligibility_requires_confirmation"
		w.ReservationPermitted = nil
		w.PartyFits = nil
		w.Notes = append(w.Notes, "English guidance prohibits after-hours oversized requests; current Japanese guidance is less explicit. Actual overnight eligibility is unknown; use daytime booking confirmation")
	} else if oversized && w.WindowStatus == "advance_eligibility_requires_confirmation" {
		w.ReservationPermitted = nil
	}
	w.SeatMapAvailable = !w.AfterHours && !now.Before(sale) && w.WindowStatus == "within_calendar_window" && class != "unreserved" && p.ID != "hayatoku1"
	return w, nil
}

type ProductView struct {
	Product
	Window               *Window  `json:"window,omitempty"`
	Assessment           string   `json:"assessment"`
	ExclusionReasons     []string `json:"exclusion_reasons"`
	RequiresConfirmation []string `json:"requires_confirmation"`
}

func CompareProducts(date, from, to, class, train string, now time.Time, adults, children int) ([]ProductView, error) {
	if e := ValidateParty(adults, children); e != nil {
		return nil, e
	}
	if class != "" && !ValidClass(class) {
		return nil, fmt.Errorf("--class must be reserved, unreserved or green")
	}
	if train != "" && !ValidTrain(train) {
		return nil, fmt.Errorf("--train must be nozomi, hikari, kodama, mizuho, sakura or tsubame")
	}
	var r Route
	var e error
	if from != "" || to != "" {
		r, e = PlanRoute(from, to, false)
		if e != nil {
			return nil, e
		}
	}
	out := []ProductView{}
	for _, p := range Products {
		v := ProductView{Product: p, Assessment: "requires_confirmation", ExclusionReasons: []string{}, RequiresConfirmation: []string{"actual_train_and_seat_availability"}}
		if p.ID != "smart-ex" {
			v.RequiresConfirmation = append(v.RequiresConfirmation, "listed_route_price_matrix", "exclusion_dates", "limited_product_seat_allocation", "direct_or_hakata_transfer_train_rules")
		}
		if adults+children < p.MinParty {
			v.ExclusionReasons = append(v.ExclusionReasons, "party_below_product_minimum")
		}
		tokaido := len(r.Corridors) == 1 && r.Corridors[0] == "tokaido"
		if p.ID == "hayatoku1" || p.ID == "family7" {
			if len(r.Corridors) > 0 && !tokaido {
				v.ExclusionReasons = append(v.ExclusionReasons, "tokaido_only")
			}
			if train != "" && train != "hikari" && train != "kodama" {
				v.ExclusionReasons = append(v.ExclusionReasons, "hikari_or_kodama_only")
			}
			if class != "" && ((p.ID == "hayatoku1" && class != "unreserved") || (p.ID == "family7" && class != "reserved")) {
				v.ExclusionReasons = append(v.ExclusionReasons, "unsupported_class_for_product")
			}
		}
		if len(r.Corridors) > 0 && !slices.Contains(r.Corridors, "kyushu") && p.ID == "hayatoku3" && class != "" && class != "green" {
			reason := "green_only_on_tokaido_sanyo"
			if tokaido {
				reason = "green_only_on_tokaido"
			}
			v.ExclusionReasons = append(v.ExclusionReasons, reason)
		}
		if tokaido && p.ID == "hayatoku7" && train == "nozomi" {
			v.ExclusionReasons = append(v.ExclusionReasons, "tokaido_hayatoku7_requires_hikari_or_kodama")
		}
		if len(r.Corridors) > 0 && train != "" {
			found := false
			for _, name := range r.TrainCategories {
				if name == train {
					found = true
				}
			}
			if !found {
				v.ExclusionReasons = append(v.ExclusionReasons, "train_category_outside_requested_corridor")
			}
		}
		if tokaido && (p.ID == "hayatoku7" || p.ID == "hayatoku21") && class == "unreserved" {
			v.ExclusionReasons = append(v.ExclusionReasons, "ordinary_reserved_only_on_tokaido")
		}
		if tokaido && p.ID == "hayatoku21" && train != "" && train != "nozomi" {
			v.ExclusionReasons = append(v.ExclusionReasons, "tokaido_hayatoku21_requires_nozomi")
		}
		if (p.ID == "hayatoku7" || p.ID == "hayatoku21") && class == "green" {
			v.ExclusionReasons = append(v.ExclusionReasons, "ordinary_car_product")
		}
		if date != "" {
			w, e := BookingWindow(date, p.ID, "", now, adults, children, false, class)
			if e != nil {
				return nil, e
			}
			v.Window = &w
			if w.WindowStatus == "closed" || w.WindowStatus == "not_open" || w.WindowStatus == "travel_date_passed" {
				v.ExclusionReasons = append(v.ExclusionReasons, w.WindowStatus)
			}
		}
		if len(v.ExclusionReasons) > 0 {
			v.Assessment = "excluded_by_checked_conditions"
		}
		out = append(out, v)
	}
	return out, nil
}

type BaggageInput struct {
	Length, Width, Height, Weight float64
	Pieces                        int
	Class                         string
	Special                       string
}

func CheckBaggage(b BaggageInput) (map[string]any, error) {
	for _, n := range []float64{b.Length, b.Width, b.Height, b.Weight} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
			return nil, fmt.Errorf("dimensions in cm and weight in kg must be finite positive numbers")
		}
	}
	if b.Pieces < 1 || b.Pieces > 10 {
		return nil, fmt.Errorf("--pieces must be1–10; normal carry-on allowance is at most two")
	}
	if !ValidClass(b.Class) {
		return nil, fmt.Errorf("--class must be reserved, unreserved or green")
	}
	if b.Special != "" && b.Special != "stroller" && b.Special != "sports" && b.Special != "instrument" {
		return nil, fmt.Errorf("--special must be stroller, sports or instrument")
	}
	sum := b.Length + b.Width + b.Height
	longest := math.Max(b.Length, math.Max(b.Width, b.Height))
	oversized := sum > 160
	allowed := sum <= 250 && longest <= 200 && b.Weight <= 30 && b.Pieces <= 2
	normalLimits := allowed
	decision := "ordinary_baggage"
	reasons := []string{}
	requires := oversized
	if !allowed {
		decision = "outside_normal_carry_on_limits"
		if sum > 250 {
			reasons = append(reasons, "three_sides_over_250_cm")
		}
		if longest > 200 {
			reasons = append(reasons, "longest_side_over_200_cm")
		}
		if b.Weight > 30 {
			reasons = append(reasons, "weight_over_30_kg")
		}
		if b.Pieces > 2 {
			reasons = append(reasons, "more_than_two_counted_pieces")
		}
	}
	if allowed && oversized {
		decision = "reserve_oversized_baggage_area"
		if b.Class == "unreserved" {
			decision = "unreserved_class_incompatible"
			allowed = false
			reasons = append(reasons, "oversized_baggage_requires_reserved_area_seat")
		}
	}
	notes := []string{"Dimensions/weight describe each identical piece; umbrellas, walking sticks and handbags are not counted as the two normal pieces", "No surcharge for a properly reserved oversized area; unreserved carry-on may incur1000JPY and conductor instructions", "Current JR summary uses more than160cm; some older smartEX English wording differs at exactly160cm", "An overall train seat availability result does not prove an oversized area is free"}
	if b.Special != "" {
		decision = "special_equipment_requires_operator_confirmation"
		requires = false
		notes = append(notes, "Strollers, sports equipment and musical instruments have exceptions to oversized-size rules; storage area use still requires reservation; this result does not approve carriage")
	}
	return map[string]any{"dimensions_cm": []float64{b.Length, b.Width, b.Height}, "total_dimensions_cm": sum, "weight_kg_per_piece": b.Weight, "pieces": b.Pieces, "class": b.Class, "special": b.Special, "decision": decision, "within_normal_limits": normalLimits, "selected_class_compatible_under_normal_rules": allowed, "oversized_area_reservation_required": requires, "reasons": reasons, "availability": nil, "notes": notes, "sources": []Source{Sources[5], Sources[6]}}, nil
}

func Policies(topic string) (map[string]any, error) {
	topics := map[string]map[string]any{
		"boarding": {"source": Sources[9], "methods": []string{"QR ticket for each passenger", "A registered transport IC card assigned to each passenger", "Paper tickets collected using the official pickup procedure"}, "notes": []string{"One IC card cannot be shared by multiple travelers in one operation", "smartEX combines Shinkansen basic and express fare; local/conventional connections are paid separately", "Ticket pickup or entering the Shinkansen gate ends online self-service change/refund eligibility"}},
		"change":   {"source": Sources[7], "notes": []string{"Change before reserved train departure, gate entry or ticket pickup", "Choose a new train at least four timetable minutes before departure; fare differences apply", "Date changes have an initial-travel-date three-month limit and one-month/one-year advance branch rules; verify reservation-specific conditions", "EX Hayatoku21 generally requires refund/rebook; same train/facility/price seat-position exception", "Hayatoku products and same-day train changes have product-specific conditions"}},
		"refund":   {"source": Sources[8], "pre_departure_fee_jpy_per_person": 320, "post_departure_fee_jpy": nil, "notes": []string{"320JPY applies to eligible unused smartEX/discount self-service refunds before departure/pickup/gate entry", "Unreserved and Hayatoku1 products use their23:30 travel-date rules; after that, self-service refund closes", "After departure, automatic refund timing and deduction depend on product/section; consult source tables", "Picked-up paper tickets require station staff; never assume the online workflow remains available"}},
		"baggage":  {"source": Sources[5], "notes": []string{"Total dimensions over160cm up to250cm need an oversized area reservation", "At most two normal pieces, each up to30kg,250cm sum and200cm maximum length", "Non-reserved seating cannot carry normal oversized baggage; special equipment exceptions need operator confirmation"}},
		"windows":  {"sources": []Source{Sources[1], Sources[2], Sources[6]}, "notes": []string{"Basic smartEX may accept requests one year in advance from05:30JST; timetable/seat details are tentative", "Reserved seat sales normally open10:00JST one month before; missing previous-month day becomes first of travel month", "After-hours23:30–05:30 has a general three-person limit and no seat map; oversized overnight eligibility is unknown because English guidance prohibits it and current Japanese guidance is less explicit", "Current Japanese and English sources disagree on08:00/14:00 confirmation and oversized one-year eligibility; check booking email/My Trips"}},
		"products": {"source": Sources[0], "notes": []string{"Hayatoku products have limited sections, excluded dates and allocated seats", "Hayatoku21 changes generally require refund and rebooking", "Round-trip smartEX discounts ended March31,2026; plan two one-way journeys"}},
	}
	if topic == "all" {
		return map[string]any{"as_of": AsOf, "topics": topics}, nil
	}
	p, ok := topics[topic]
	if !ok {
		return nil, fmt.Errorf("--topic must be boarding, change, refund, baggage, windows, products or all")
	}
	p["as_of"] = AsOf
	p["topic"] = topic
	return p, nil
}

func Handoff(from, to, date, class string, adults, children int) (map[string]any, error) {
	r, e := PlanRoute(from, to, false)
	if e != nil {
		return nil, e
	}
	if _, e = ParseDate(date); e != nil {
		return nil, e
	}
	if !ValidClass(class) {
		return nil, fmt.Errorf("--class must be reserved, unreserved or green")
	}
	if e = ValidateParty(adults, children); e != nil {
		return nil, e
	}
	return map[string]any{"route": r, "date_jst": date, "class": class, "adults": adults, "children": children, "booking_url": BookingURL, "guide_url": "https://smart-ex.jp/en/reservation/reserve_smart/sp/", "fare_navigator_url": "https://unchin-navi.jp/", "prefilled": false, "required_action": "Log in to official smartEX and enter the checklist values; this CLI does not reserve", "checklist": []string{r.From.Japanese + " → " + r.To.Japanese, date + " JST", fmt.Sprintf("adults: %d, children: %d; %s class", adults, children, class), "Confirm train/date, total fare and each passenger seat", "Choose oversized baggage area seats if required", "Confirm product exclusion dates and change/refund rules", "Choose QR/assigned IC/paper boarding method"}, "inventory": nil, "mutation_performed": false}, nil
}

func ContainsCorridor(r Route, c string) bool {
	return strings.Contains(strings.Join(r.Corridors, ","), c)
}
