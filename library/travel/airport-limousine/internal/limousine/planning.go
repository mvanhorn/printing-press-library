// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FareUnits struct {
	AdultJPY *int   `json:"adult_jpy"`
	ChildJPY *int   `json:"child_jpy"`
	Currency string `json:"currency"`
}
type Station struct {
	ID           string    `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	JapaneseName string    `json:"japanese_name"`
	Kind         string    `json:"source_kind"`
	Role         string    `json:"role"`
	Fare         FareUnits `json:"published_fare"`
	SourceURL    string    `json:"source_url"`
}
type Clock struct {
	Raw              string `json:"source_hhmm"`
	SourceType       string `json:"source_time_type"`
	Time             string `json:"time"`
	Date             string `json:"date_jst"`
	DateTime         string `json:"datetime_jst"`
	DayOffset        int    `json:"day_offset"`
	RolloverInferred bool   `json:"rollover_inferred"`
	Minutes          int    `json:"-"`
}
type Call struct {
	StopID        string    `json:"stop_id"`
	Name          string    `json:"name"`
	JapaneseName  string    `json:"japanese_name"`
	Role          string    `json:"role"`
	Arrival       *Clock    `json:"arrival"`
	Departure     *Clock    `json:"departure"`
	CallState     string    `json:"call_state"`
	Platform      string    `json:"platform_japanese"`
	Fare          FareUnits `json:"published_fare"`
	ArrivalRaw    string    `json:"arrival_source_value"`
	DepartureRaw  string    `json:"departure_source_value"`
	ArrivalType   string    `json:"arrival_source_type"`
	DepartureType string    `json:"departure_source_type"`
}
type Journey struct {
	ID                       string `json:"id"`
	RouteCode                string `json:"source_route_code"`
	ServiceDate              string `json:"service_date_jst"`
	Operator                 string `json:"operator"`
	NameJapanese             string `json:"line_name_japanese"`
	Calls                    []Call `json:"calls"`
	Origin                   *Call  `json:"origin,omitempty"`
	Destination              *Call  `json:"destination,omitempty"`
	ScheduledDurationMinutes *int   `json:"scheduled_duration_minutes,omitempty"`
	OperationState           *int   `json:"source_operation_state,omitempty"`
	SourceStatus             string `json:"source_status,omitempty"`
	SourceRecordCreatedAt    string `json:"source_record_created_at,omitempty"`
}
type Schedule struct {
	RouteID         string         `json:"route_id"`
	RouteName       string         `json:"route_name"`
	Airport         string         `json:"airport_japanese"`
	Direction       string         `json:"direction"`
	SourceDirection string         `json:"source_direction"`
	SelectedDate    string         `json:"selected_date_jst"`
	FirstSourceDate string         `json:"first_source_date_jst"`
	LastSourceDate  string         `json:"last_source_date_jst"`
	Suspended       bool           `json:"suspended"`
	SourceURL       string         `json:"source_url"`
	Stations        []Station      `json:"stations"`
	Journeys        []Journey      `json:"journeys"`
	ScannedTrains   int            `json:"scanned_trains"`
	Alerts          map[string]any `json:"source_alerts"`
}

var hhmmPattern = regexp.MustCompile(`^\d{1,4}$`)

func ClockFromSource(raw, kind, date string, previous int) (*Clock, error) {
	if raw == "" || strings.HasPrefix(raw, "-") || kind != "0" {
		return nil, nil
	}
	if !hhmmPattern.MatchString(raw) {
		return nil, fmt.Errorf("unrecognized provider time %q", raw)
	}
	n, e := strconv.Atoi(raw)
	if e != nil {
		return nil, e
	}
	hour, minute := n/100, n%100
	if hour > 47 || minute > 59 {
		return nil, fmt.Errorf("invalid provider HHMM %q", raw)
	}
	total := hour*60 + minute
	inferred := false
	if total < previous && hour >= 24 {
		return nil, fmt.Errorf("provider extended-hour times are not chronological")
	}
	if total < previous && hour < 24 {
		if previous-total < 12*60 {
			return nil, fmt.Errorf("provider stop times are not chronological (%s)", raw)
		}
		for total < previous {
			total += 24 * 60
		}
		inferred = true
	}
	d, e := time.ParseInLocation("2006-01-02", date, JST)
	if e != nil {
		return nil, e
	}
	absolute := d.Add(time.Duration(total) * time.Minute)
	return &Clock{Raw: raw, SourceType: kind, Time: absolute.Format("15:04"), Date: absolute.Format("2006-01-02"), DateTime: absolute.Format(time.RFC3339), DayOffset: total / (24 * 60), RolloverInferred: inferred, Minutes: total}, nil
}

func (p *Provider) Timetable(ctx context.Context, route, date, direction string) (Schedule, error) {
	if err := ValidateID(route); err != nil {
		return Schedule{}, err
	}
	wire := "1"
	if direction == "to-airport" {
		wire = "2"
	} else if direction != "from-airport" {
		return Schedule{}, fmt.Errorf("--direction must be from-airport or to-airport")
	}
	query := url.Values{"dir": {wire}, "d": {date}}
	page := "/en/timetable/detail/" + route
	data, err := p.Data(ctx, DataPath(page, query))
	if err != nil {
		return Schedule{}, err
	}
	return ParseSchedule(data, route, date, direction, PageURL(page, query))
}
func ParseSchedule(data map[string]any, route, date, direction, sourceURL string) (Schedule, error) {
	area := M(data["area"])
	if area == nil || S(area["area_id"]) != route {
		return Schedule{}, fmt.Errorf("provider did not resolve route %q", route)
	}
	selected := S(data["selectedDate"])
	if selected != date {
		return Schedule{}, fmt.Errorf("provider returned service date %q instead of %q", selected, date)
	}
	first, last := S(data["nowDate"]), S(data["maxDate"])
	if first != "" && date < first || last != "" && date > last {
		return Schedule{}, fmt.Errorf("--date %s is outside the provider window %s through %s", date, first, last)
	}
	sourceDir := S(data["direction"])
	if DirectionName(sourceDir) != direction {
		return Schedule{}, fmt.Errorf("provider timetable direction does not match --direction")
	}
	airportKind := ""
	if strings.HasPrefix(route, "Haneda-") {
		airportKind = "H"
	}
	if strings.HasPrefix(route, "Narita-") {
		airportKind = "N"
	}
	if airportKind == "" {
		return Schedule{}, fmt.Errorf("unsupported route airport prefix: select an ID from routes")
	}
	out := Schedule{RouteID: route, RouteName: S(area["name"]), Airport: S(data["airport"]), Direction: direction, SourceDirection: sourceDir, SelectedDate: selected, FirstSourceDate: first, LastSourceDate: last, Suspended: B(area["unkyu"]), SourceURL: sourceURL, Stations: []Station{}, Journeys: []Journey{}, Alerts: M(data["alert"])}
	groups, ok := data["displayTimeTables"]
	if !ok {
		return Schedule{}, fmt.Errorf("provider timetable has no displayTimeTables")
	}
	seen := map[string]bool{}
	for _, rawGroup := range A(groups) {
		group := M(rawGroup)
		stationList := []Station{}
		for _, raw := range A(group["stations"]) {
			st := M(raw)
			id := S(st["id"])
			if err := ValidateID(id); err != nil {
				return Schedule{}, err
			}
			role := "alighting"
			same := S(st["kind"]) == airportKind
			if direction == "from-airport" && same || direction == "to-airport" && !same {
				role = "boarding"
			}
			fare := M(st["fare"])
			x := Station{ID: id, Code: S(st["code"]), Name: S(st["display_name"]), JapaneseName: S(st["name"]), Kind: S(st["kind"]), Role: role, Fare: FareUnits{AdultJPY: Num(fare["adult"]), ChildJPY: Num(fare["child"]), Currency: "JPY"}, SourceURL: PageURL("/en/busstop/detail/"+id, nil)}
			stationList = append(stationList, x)
			if !seen[id] {
				out.Stations = append(out.Stations, x)
				seen[id] = true
			}
		}
		for _, raw := range A(group["trains"]) {
			train := M(raw)
			out.ScannedTrains++
			times := A(train["time"])
			if len(times) != len(stationList) {
				return Schedule{}, fmt.Errorf("provider train/station column count differs")
			}
			info := M(train["additionalInfo"])
			serviceDate := S(info["service_date"])
			if serviceDate == "" {
				serviceDate = date
			}
			if serviceDate != date {
				return Schedule{}, fmt.Errorf("provider trip belongs to a different service date")
			}
			id := S(train["uniqueId"])
			if id == "" {
				id = S(train["lineNumber"])
			}
			if id == "" {
				return Schedule{}, fmt.Errorf("provider trip has no stable identifier")
			}
			trip := Journey{ID: id, RouteCode: S(info["route_code"]), ServiceDate: serviceDate, Operator: S(info["company_name"]), NameJapanese: S(train["lineName"]), Calls: []Call{}, OperationState: Num(info["operation_state"]), SourceStatus: S(info["status"]), SourceRecordCreatedAt: S(info["created_at"])}
			previous := -1
			for i, rawTime := range times {
				tm := M(rawTime)
				st := stationList[i]
				ar, dp := S(tm["arrivalTime"]), S(tm["departureTime"])
				art, dpt := S(tm["arrivalTimeType"]), S(tm["departureTimeType"])
				arrival, err := ClockFromSource(ar, art, date, previous)
				if err != nil {
					return Schedule{}, err
				}
				if arrival != nil {
					previous = arrival.Minutes
				}
				departure, err := ClockFromSource(dp, dpt, date, previous)
				if err != nil {
					return Schedule{}, err
				}
				if departure != nil {
					previous = departure.Minutes
				}
				state := "scheduled"
				if arrival == nil && departure == nil {
					state = "unknown"
					if art == "-2" || dpt == "-2" {
						state = "passes"
					} else if art == "-1" && dpt == "-1" {
						state = "not-served"
					}
				}
				trip.Calls = append(trip.Calls, Call{StopID: st.ID, Name: st.Name, JapaneseName: st.JapaneseName, Role: st.Role, Arrival: arrival, Departure: departure, CallState: state, Platform: S(tm["platform"]), Fare: st.Fare, ArrivalRaw: ar, DepartureRaw: dp, ArrivalType: art, DepartureType: dpt})
			}
			out.Journeys = append(out.Journeys, trip)
		}
	}
	return out, nil
}

func FilterJourneys(schedule Schedule, from, to, after string) ([]Journey, error) {
	if after != "" && from == "" {
		return nil, fmt.Errorf("--after requires --from-stop so the departure terminal is unambiguous")
	}
	if from != "" || to != "" {
		find := func(id, role string) error {
			if id == "" {
				return nil
			}
			for _, s := range schedule.Stations {
				if s.ID == id {
					if s.Role != role {
						return fmt.Errorf("stop %s is not a %s stop in this direction", id, role)
					}
					return nil
				}
			}
			return fmt.Errorf("stop %s is absent from this route and direction", id)
		}
		if err := find(from, "boarding"); err != nil {
			return nil, err
		}
		if err := find(to, "alighting"); err != nil {
			return nil, err
		}
	}
	afterMinute := -1
	if after != "" {
		parts := strings.Split(after, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("--after must be HH:MM in the JST service day (00:00–47:59)")
		}
		h, e := strconv.Atoi(parts[0])
		m, e2 := strconv.Atoi(parts[1])
		if e != nil || e2 != nil || len(parts[1]) != 2 || h < 0 || h > 47 || m < 0 || m > 59 {
			return nil, fmt.Errorf("--after must be HH:MM in the JST service day (00:00–47:59)")
		}
		afterMinute = h*60 + m
	}
	out := []Journey{}
	for _, j := range schedule.Journeys {
		var origin, dest *Call
		for i := range j.Calls {
			c := &j.Calls[i]
			if c.StopID == from && c.Departure != nil {
				origin = c
			}
			if c.StopID == to && c.Arrival != nil {
				dest = c
			}
		}
		if from != "" && origin == nil || to != "" && dest == nil {
			continue
		}
		if origin != nil && afterMinute >= 0 && origin.Departure.Minutes < afterMinute {
			continue
		}
		if origin != nil && dest != nil {
			d := dest.Arrival.Minutes - origin.Departure.Minutes
			if d < 0 {
				return nil, fmt.Errorf("provider selected-pair times are not chronological")
			}
			j.ScheduledDurationMinutes = &d
		}
		j.Origin = origin
		j.Destination = dest
		out = append(out, j)
	}
	return out, nil
}

type FareQuote struct {
	RouteID     string   `json:"route_id"`
	ServiceDate string   `json:"service_date_jst"`
	Direction   string   `json:"direction"`
	Origin      Station  `json:"origin"`
	Destination Station  `json:"destination"`
	AdultJPY    *int     `json:"adult_unit_jpy"`
	ChildJPY    *int     `json:"child_unit_jpy"`
	Adults      int      `json:"adults"`
	Children    int      `json:"children"`
	TotalJPY    *int     `json:"total_jpy"`
	Currency    string   `json:"currency"`
	ServedTrips int      `json:"served_trips"`
	QuoteState  string   `json:"quote_state"`
	Assumptions []string `json:"assumptions"`
	BookingURL  string   `json:"booking_url"`
}

func Quote(schedule Schedule, from, to string, adults, children int, tripID string) (FareQuote, error) {
	if from == "" || to == "" {
		return FareQuote{}, fmt.Errorf("fare requires --from-stop and --to-stop from the timetable station IDs")
	}
	if adults < 0 || children < 0 || adults+children < 1 || adults+children > 100 {
		return FareQuote{}, fmt.Errorf("--adults and --children must total 1–100, with neither negative")
	}
	trips, err := FilterJourneys(schedule, from, to, "")
	if err != nil {
		return FareQuote{}, err
	}
	if len(trips) == 0 {
		return FareQuote{}, fmt.Errorf("no dated trip serves the selected boarding and alighting stops")
	}
	if tripID != "" {
		selected := []Journey{}
		for _, j := range trips {
			if j.ID == tripID {
				selected = append(selected, j)
			}
		}
		if len(selected) == 0 {
			return FareQuote{}, fmt.Errorf("--trip-id is not a served trip for this pair and date")
		}
		trips = selected
	}
	var origin, dest Station
	for _, s := range schedule.Stations {
		if s.ID == from {
			origin = s
		}
		if s.ID == to {
			dest = s
		}
	}
	fare := trips[0].Destination.Fare
	if schedule.Direction == "to-airport" {
		fare = trips[0].Origin.Fare
	}
	same := func(a, b *int) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}
	for _, j := range trips {
		f := j.Destination.Fare
		if schedule.Direction == "to-airport" {
			f = j.Origin.Fare
		}
		if !same(f.AdultJPY, fare.AdultJPY) || !same(f.ChildJPY, fare.ChildJPY) {
			return FareQuote{}, fmt.Errorf("published fares differ across served trips; select --trip-id from timetable for an exact fare")
		}
	}
	origin.Fare = trips[0].Origin.Fare
	dest.Fare = trips[0].Destination.Fare
	out := FareQuote{RouteID: schedule.RouteID, ServiceDate: schedule.SelectedDate, Direction: schedule.Direction, Origin: origin, Destination: dest, AdultJPY: fare.AdultJPY, ChildJPY: fare.ChildJPY, Adults: adults, Children: children, Currency: "JPY", ServedTrips: len(trips), QuoteState: "unknown", BookingURL: schedule.SourceURL, Assumptions: []string{"One-way published station-column fare for this served pair and direction; discounts and surcharges are not inferred.", "Children means passengers eligible for the published child fare; under-six passengers using a seat generally need child fare. See conditions --topic boarding.", "Arithmetic planning total, not a fare guarantee, reservation or seat-availability check."}}
	if (adults == 0 || fare.AdultJPY != nil) && (children == 0 || fare.ChildJPY != nil) {
		total := 0
		if adults > 0 {
			total += adults * *fare.AdultJPY
		}
		if children > 0 {
			total += children * *fare.ChildJPY
		}
		out.TotalJPY = &total
		out.QuoteState = "published-fare-arithmetic"
	}
	return out, nil
}
