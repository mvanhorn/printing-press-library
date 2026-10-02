package haneda

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func ptr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	return &s
}
func parseTimestamp(s string) *string {
	for _, layout := range []string{"2006/01/02 15:04:05", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, JST); err == nil {
			v := t.In(JST).Format(time.RFC3339)
			return &v
		}
	}
	return nil
}
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "20060102"} {
		if t, err := time.ParseInLocation(layout, s, JST); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid source date %q", s)
}
func sourceTime(date, clock string) (*string, error) {
	if ptr(clock) == nil {
		return nil, nil
	}
	d, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	if _, err = time.Parse("15:04", clock); err != nil {
		return nil, fmt.Errorf("invalid source clock %q", clock)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", d.Format("2006-01-02")+" "+clock, JST)
	if err != nil {
		return nil, err
	}
	v := t.Format(time.RFC3339)
	return &v, nil
}
func normalizeFlight(raw RawFlight, airports []Airport, airlines []Airline, stamp *string) (Flight, error) {
	f := Flight{Kind: raw.Kind, Direction: raw.Direction, SourceAreaName: raw.Area, ViaAreaName: ptr(raw.Via), Airport: findAirport(raw.Area, airports), ListedFlights: []ListedFlight{}, CodeshareFlights: []string{}, BoardingGates: []string{}, CheckinCounters: []string{}, SecurityChecks: []string{}, ArrivalExits: []string{}, Facilities: []Facility{}, UnknownFields: []string{"operating_flight", "actual_at"}, SourceReportedAt: stamp}
	if raw.Kind != "domestic" && raw.Kind != "international" {
		return f, fmt.Errorf("unknown source flight type %q", raw.Kind)
	}
	if raw.Direction != "departure" && raw.Direction != "arrival" {
		return f, fmt.Errorf("unknown source flight direction %q", raw.Direction)
	}
	d, err := parseDate(raw.Date.Key)
	if err != nil {
		return f, err
	}
	f.ServiceDate = d.Format("2006-01-02")
	for _, a := range raw.Airlines {
		n := strings.ToUpper(strings.TrimSpace(a.Number))
		if !flightNumberPattern.MatchString(n) {
			return f, fmt.Errorf("invalid/missing source flight number %q", a.Number)
		}
		meta := findAirline(a.Code, airlines)
		f.ListedFlights = append(f.ListedFlights, ListedFlight{Number: n, AirlineCode: strings.TrimSpace(a.Code), Name: meta.Name, NameJA: meta.NameJA, URL: a.URL})
	}
	if len(f.ListedFlights) == 0 {
		return f, fmt.Errorf("source group has no listed flights")
	}
	f.SourcePrimaryFlight = f.ListedFlights[0].Number
	for _, a := range f.ListedFlights[1:] {
		f.CodeshareFlights = append(f.CodeshareFlights, a.Number)
	}
	// Time and route are deliberately excluded: their changes must retain identity in diffs.
	f.ID = "hnd:" + raw.Kind + ":" + raw.Direction + ":" + raw.Date.Key + ":" + f.SourcePrimaryFlight
	f.ScheduledAt, err = sourceTime(raw.Date.Display, raw.Scheduled)
	if err != nil {
		return f, err
	}
	// The explicit changed date, rather than a nearest-midnight guess, controls rollover.
	if ptr(raw.Changed) != nil && ptr(raw.Date.Change) != nil {
		f.RevisedAt, err = sourceTime(raw.Date.Change, raw.Changed)
		if err != nil {
			return f, err
		}
	}
	if f.ScheduledAt != nil && f.RevisedAt != nil {
		a, _ := time.Parse(time.RFC3339, *f.ScheduledAt)
		b, _ := time.Parse(time.RFC3339, *f.RevisedAt)
		delta := int(b.Sub(a).Minutes())
		f.TimeChangeMinutes = &delta
	}
	if raw.Terminal != nil {
		f.Terminal = ptr(raw.Terminal.Name)
	}
	if f.Terminal != nil && *f.Terminal != "T1" && *f.Terminal != "T2" && *f.Terminal != "T3" {
		f.UnknownFields = append(f.UnknownFields, "terminal_mapping")
	}
	f.Status = Status{Category: raw.Status.Category, Text: raw.Status.Text, Reason: ptr(raw.Status.Reason), Known: ptr(raw.Status.Category) != nil || ptr(raw.Status.Text) != nil}
	if ptr(raw.Status.Category) == nil {
		f.UnknownFields = append(f.UnknownFields, "status_category")
	}
	if !f.Status.Known {
		f.UnknownFields = append(f.UnknownFields, "status")
	}
	for _, option := range raw.Options {
		for _, item := range option.Items {
			if ptr(item.Name) == nil {
				continue
			}
			f.Facilities = append(f.Facilities, Facility{Type: option.Type, Title: option.Title, Name: item.Name, MapURL: ptr(item.Map)})
			switch option.Type {
			case "gate", "gateDep":
				f.BoardingGates = append(f.BoardingGates, item.Name)
			case "checkin", "dmsCheckin":
				f.CheckinCounters = append(f.CheckinCounters, item.Name)
			case "checkpoint":
				f.SecurityChecks = append(f.SecurityChecks, item.Name)
			case "exitGate":
				f.ArrivalExits = append(f.ArrivalExits, item.Name)
			}
		}
	}
	if f.Terminal == nil {
		f.UnknownFields = append(f.UnknownFields, "terminal")
	}
	if raw.Direction == "departure" && len(f.BoardingGates) == 0 {
		f.UnknownFields = append(f.UnknownFields, "boarding_gates")
	}
	if raw.Direction == "departure" && len(f.CheckinCounters) == 0 {
		f.UnknownFields = append(f.UnknownFields, "checkin_counters")
	}
	if f.ScheduledAt == nil {
		f.UnknownFields = append(f.UnknownFields, "scheduled_at")
	}
	if ptr(raw.Changed) != nil && f.RevisedAt == nil {
		f.UnknownFields = append(f.UnknownFields, "revised_at")
	}
	if f.Airport == nil {
		f.UnknownFields = append(f.UnknownFields, "other_airport")
	}
	code := catalogCode(raw.Kind)
	f.SourceURL = Origin + "/en/flight/detail/" + code + "_" + raw.Direction + ".html?" + url.Values{"searchDt": {raw.Date.Key}, "flightNumber": {f.SourcePrimaryFlight}}.Encode()
	return f, nil
}

func boardNotes() []string {
	return []string{
		"Source groups are services, not separate seats; all listed codeshares retain their original source order. The first listed code is not proof of the operating carrier.",
		"revised_at preserves the source change_time/change_date. The airport does not explicitly distinguish estimates from actual times; actual_at stays null, including after departure/arrival.",
		"Response timestamps do not establish per-flight freshness. Search lists may include adjacent service days; blank status and missing facilities are unknown.",
		"Airport information does not establish bookable seats, guaranteed gates or guaranteed connections; consult the canonical airport/airline handoff.",
	}
}
