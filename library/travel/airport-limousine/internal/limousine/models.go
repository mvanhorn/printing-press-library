// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Meta struct {
	Context  any       `json:"context,omitempty"`
	Stations []Station `json:"stations,omitempty"`

	Source        string    `json:"source"`
	SourceURLs    []string  `json:"source_urls"`
	ObservedAt    string    `json:"observed_at"`
	Timezone      string    `json:"timezone"`
	Requests      int       `json:"requests"`
	Scanned       int       `json:"scanned_records"`
	Total         int       `json:"total_matching"`
	Returned      int       `json:"returned"`
	Truncated     bool      `json:"truncated"`
	Note          string    `json:"note,omitempty"`
	FetchFailures []Failure `json:"fetch_failures,omitempty"`
}
type Failure struct {
	SourceURL string `json:"source_url"`
	Error     string `json:"error"`
}
type Envelope struct {
	Meta     Meta      `json:"meta"`
	Results  any       `json:"results"`
	Stations []Station `json:"stations,omitempty"`
	Details  any       `json:"details,omitempty"`
}

// MarshalJSON keeps the standard provenance envelope at exactly two keys.
// Supporting station columns and context live beside their source metadata.
func (e Envelope) MarshalJSON() ([]byte, error) {
	meta := e.Meta
	meta.Context = e.Details
	meta.Stations = e.Stations
	return json.Marshal(struct {
		Meta    Meta `json:"meta"`
		Results any  `json:"results"`
	}{meta, e.Results})
}

func (p *Provider) Meta(urls []string, scanned, total, returned int, note string) Meta {
	return Meta{Source: "live", SourceURLs: urls, ObservedAt: p.ObservedAt, Timezone: "Asia/Tokyo", Requests: p.Requests, Scanned: scanned, Total: total, Returned: returned, Truncated: total > returned, Note: note}
}

type Route struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Airport        string `json:"airport"`
	Suspended      bool   `json:"suspended"`
	Note           string `json:"source_note,omitempty"`
	FromAirportURL string `json:"from_airport_url"`
	ToAirportURL   string `json:"to_airport_url"`
}

func Routes(obj map[string]any, query, airport string, includeSuspended bool) ([]Route, int, error) {
	areas := M(obj["areas"])
	if areas == nil {
		return nil, 0, fmt.Errorf("provider route list has no areas")
	}
	out := []Route{}
	scanned := 0
	for _, ap := range []string{"haneda", "narita"} {
		for _, raw := range A(areas[ap]) {
			scanned++
			m := M(raw)
			id := S(m["area_id"])
			name := S(m["name"])
			// CMS placeholder records have no real destination. They are excluded from travel planning.
			if id == "test" || strings.Contains(strings.ToLower(id), "dummy") {
				continue
			}
			if err := ValidateID(id); err != nil {
				return nil, scanned, err
			}
			if airport != "" && ap != airport {
				continue
			}
			suspended := B(m["unkyu"])
			if suspended && !includeSuspended {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(id+" "+name), strings.ToLower(query)) {
				continue
			}
			page := "/en/timetable/detail/" + id
			out = append(out, Route{ID: id, Name: name, Airport: ap, Suspended: suspended, Note: S(m["note"]), FromAirportURL: PageURL(page, url.Values{"dir": {"1"}}), ToAirportURL: PageURL(page, url.Values{"dir": {"2"}})})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, scanned, nil
}

type Stop struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	OfficialName string   `json:"official_name"`
	JapaneseName string   `json:"japanese_name"`
	Kind         string   `json:"source_kind"`
	Airports     []string `json:"airports,omitempty"`
	InboundCode  string   `json:"inbound_code"`
	OutboundCode string   `json:"outbound_code"`
	UpdatedAt    string   `json:"source_updated_at,omitempty"`
	SourceURL    string   `json:"source_url"`
}

func ReadStop(m map[string]any) (Stop, error) {
	id := S(m["busstop_id"])
	if err := ValidateID(id); err != nil {
		return Stop{}, err
	}
	airports := []string{}
	for _, ap := range A(m["airport"]) {
		airports = append(airports, S(ap))
	}
	return Stop{ID: id, Name: S(m["display_name"]), OfficialName: S(m["name"]), JapaneseName: S(m["norikae_name"]), Kind: S(m["kind"]), Airports: airports, InboundCode: S(m["inbound_busstop_code"]), OutboundCode: S(m["outbound_busstop_code"]), UpdatedAt: S(m["updatedAt"]), SourceURL: PageURL("/en/busstop/detail/"+id, nil)}, nil
}
func Stops(obj map[string]any, query, airport string) ([]Stop, int, error) {
	raw, ok := obj["busstops"]
	if !ok {
		return nil, 0, fmt.Errorf("provider stop search has no busstops field")
	}
	out := []Stop{}
	a := A(raw)
	for _, v := range a {
		s, err := ReadStop(M(v))
		if err != nil {
			return nil, len(a), err
		}
		if airport != "" {
			found := false
			for _, ap := range s.Airports {
				if ap == airport {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(s.Name+" "+s.OfficialName+" "+s.JapaneseName), strings.ToLower(query)) {
			continue
		}
		out = append(out, s)
	}
	return out, len(a), nil
}

type Connection struct {
	RouteID      string `json:"route_id"`
	LineID       string `json:"line_id"`
	Name         string `json:"name"`
	JapaneseName string `json:"japanese_name"`
	Airport      string `json:"airport"`
	Direction    string `json:"direction"`
	TimetableURL string `json:"timetable_url"`
}
type StopDetail struct {
	Stop             Stop         `json:"stop"`
	Address          string       `json:"address"`
	PostalCode       string       `json:"postal_code"`
	PickupLat        *float64     `json:"pickup_lat"`
	PickupLon        *float64     `json:"pickup_lon"`
	DropoffLat       *float64     `json:"dropoff_lat"`
	DropoffLon       *float64     `json:"dropoff_lon"`
	CoordinatesUnit  string       `json:"coordinates_unit"`
	PickupNote       string       `json:"pickup_note,omitempty"`
	DropoffNote      string       `json:"dropoff_note,omitempty"`
	MapURLs          []string     `json:"map_urls"`
	Connections      []Connection `json:"connections"`
	TotalConnections int          `json:"total_connections"`
	SourceUpdatedAt  string       `json:"location_source_updated_at,omitempty"`
}

func Detail(obj map[string]any, limit int) (StopDetail, error) {
	st, err := ReadStop(M(obj["busstop"]))
	if err != nil {
		return StopDetail{}, err
	}
	base := M(obj["baseInformation"])
	if base == nil {
		return StopDetail{}, fmt.Errorf("provider stop detail has no boarding information")
	}
	out := StopDetail{Stop: st, Address: S(base["address"]), PostalCode: S(base["postal_code"]), PickupLat: Float(base["pickup_lat"]), PickupLon: Float(base["pickup_lon"]), DropoffLat: Float(base["dropoff_lat"]), DropoffLon: Float(base["dropoff_lon"]), CoordinatesUnit: "decimal degrees", PickupNote: CleanHTML(S(base["pickup_text"])), DropoffNote: CleanHTML(S(base["dropoff_text"])), SourceUpdatedAt: S(base["updatedAt"]), MapURLs: []string{}, Connections: []Connection{}}
	seen := map[string]bool{}
	for _, key := range []string{"pickup_img", "dropoff_img"} {
		for _, v := range A(base[key]) {
			u := S(M(v)["url"])
			parsed, err := url.Parse(u)
			if err == nil && parsed.Scheme == "https" && strings.HasSuffix(parsed.Hostname(), ".limousinebus.co.jp") && !seen[u] {
				seen[u] = true
				out.MapURLs = append(out.MapURLs, u)
			}
		}
	}
	for _, key := range []string{"hanedaRosens", "naritaRosens"} {
		for _, v := range A(obj[key]) {
			m := M(v)
			a := M(m["area"])
			id := S(a["area_id"])
			if id == "" {
				continue
			}
			if err := ValidateID(id); err != nil {
				return StopDetail{}, err
			}
			dir := S(m["direction"])
			wire := "1"
			if dir == "outbound" {
				wire = "2"
			} else if dir != "inbound" {
				return StopDetail{}, fmt.Errorf("unknown provider route direction %q", dir)
			}
			out.TotalConnections++
			if len(out.Connections) >= limit {
				continue
			}
			out.Connections = append(out.Connections, Connection{RouteID: id, LineID: S(m["rosen_id"]), Name: S(m["name"]), JapaneseName: S(m["norikae_name"]), Airport: S(m["airport"]), Direction: DirectionName(dir), TimetableURL: PageURL("/en/timetable/detail/"+id, url.Values{"dir": {wire}})})
		}
	}
	return out, nil
}

type Duration struct {
	ID                string `json:"id"`
	Airport           string `json:"airport"`
	Direction         string `json:"direction"`
	Route             string `json:"route"`
	SourceClockJST    string `json:"source_clock_jst"`
	SourceDateKnown   bool   `json:"source_date_known"`
	CurrentMinutes    *int   `json:"current_minutes"`
	CurrentState      string `json:"current_state"`
	CurrentText       string `json:"current_text"`
	StandardMinutes   *int   `json:"standard_minutes"`
	StandardText      string `json:"standard_text"`
	DifferenceMinutes *int   `json:"difference_minutes"`
	ArrivalGuaranteed bool   `json:"arrival_guaranteed"`
}

func Minutes(text string) (*int, string) {
	trim := strings.TrimSpace(text)
	if strings.HasSuffix(trim, " min") {
		n, e := strconv.Atoi(strings.TrimSuffix(trim, " min"))
		if e == nil && n >= 0 {
			return &n, "estimated"
		}
	}
	if strings.Contains(strings.ToLower(trim), "adjusting") {
		return nil, "adjusting"
	}
	if strings.Contains(strings.ToLower(trim), "retrieving") {
		return nil, "retrieving"
	}
	return nil, "unknown"
}
func DirectionName(source string) string {
	if source == "inbound" {
		return "from-airport"
	}
	if source == "outbound" {
		return "to-airport"
	}
	return "unknown"
}
func Durations(obj map[string]any, query, airport, direction string) ([]Duration, int, error) {
	rt := M(obj["realtime"])
	if rt == nil {
		return nil, 0, fmt.Errorf("provider current-time response lacks realtime data")
	}
	out := []Duration{}
	scanned := 0
	for _, ap := range []string{"haneda", "narita"} {
		b := M(rt[ap])
		clock := S(b["current_time"])
		for _, raw := range A(b["table_data"]) {
			scanned++
			m := M(raw)
			dir := DirectionName(S(m["dir"]))
			route := S(m["kukan"])
			if airport != "" && ap != airport || direction != "" && dir != direction || query != "" && !strings.Contains(strings.ToLower(route), strings.ToLower(query)) {
				continue
			}
			cur, state := Minutes(S(m["realtime"]))
			standard, _ := Minutes(S(m["standard"]))
			var delta *int
			if cur != nil && standard != nil {
				n := *cur - *standard
				delta = &n
			}
			num := ""
			if n := Num(m["no"]); n != nil {
				num = strconv.Itoa(*n)
			}
			out = append(out, Duration{ID: ap + "-" + num + "-" + dir, Airport: ap, Direction: dir, Route: route, SourceClockJST: clock, CurrentMinutes: cur, CurrentState: state, CurrentText: S(m["realtime"]), StandardMinutes: standard, StandardText: S(m["standard"]), DifferenceMinutes: delta})
		}
	}
	return out, scanned, nil
}

func ServiceDate(input string, now time.Time) (string, error) {
	if input == "" {
		input = now.In(JST).Format("2006-01-02")
	}
	d, e := time.ParseInLocation("2006-01-02", input, JST)
	if e != nil || d.Format("2006-01-02") != input {
		return "", fmt.Errorf("--date must be YYYY-MM-DD in Asia/Tokyo")
	}
	return input, nil
}
