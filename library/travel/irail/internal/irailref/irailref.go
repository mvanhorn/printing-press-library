// Package irailref serves the open iRail station reference datasets that the
// live API does not expose: telegraph codes, four-language station names,
// official minimum transfer times, and per-station facilities.
//
// pp:novel-static-reference
//
// Source: https://github.com/iRail/stations (stations.csv, facilities.csv).
// The data is embedded rather than fetched so lookups cost no HTTP request and
// stay inside iRail's 3 req/s budget. Refresh by re-copying both CSVs from that
// repository; the schema is the upstream header row verbatim.
package irailref

import (
	_ "embed"
	"encoding/csv"
	"strconv"
	"strings"
	"sync"
)

//go:embed stations.csv
var stationsCSV string

//go:embed facilities.csv
var facilitiesCSV string

// Station is one row of the upstream stations.csv.
type Station struct {
	URI       string `json:"uri"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	NameFR    string `json:"name_fr,omitempty"`
	NameNL    string `json:"name_nl,omitempty"`
	NameDE    string `json:"name_de,omitempty"`
	NameEN    string `json:"name_en,omitempty"`
	TafTap    string `json:"taf_tap_code,omitempty"`
	Telegraph string `json:"telegraph_code,omitempty"`
	Country   string `json:"country_code,omitempty"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	// AvgStopTimes is a busyness proxy: the average number of stop times per day.
	AvgStopTimes float64 `json:"avg_stop_times"`
	// TransferSeconds is the official minimum transfer time at this station.
	// HasTransfer reports whether the upstream row supplied a value at all,
	// so callers can distinguish "no data" from a genuine zero.
	TransferSeconds int  `json:"transfer_seconds"`
	HasTransfer     bool `json:"has_transfer_time"`
}

// SalesWindow is a ticket-desk opening window for one weekday.
type SalesWindow struct {
	Day   string `json:"day"`
	Open  string `json:"open"`
	Close string `json:"close"`
}

// Facilities is one row of the upstream facilities.csv.
type Facilities struct {
	URI                  string        `json:"uri"`
	Name                 string        `json:"name"`
	Street               string        `json:"street,omitempty"`
	Zip                  string        `json:"zip,omitempty"`
	City                 string        `json:"city,omitempty"`
	TicketVendingMachine bool          `json:"ticket_vending_machine"`
	LuggageLockers       bool          `json:"luggage_lockers"`
	FreeParking          bool          `json:"free_parking"`
	Taxi                 bool          `json:"taxi"`
	BicycleSpots         bool          `json:"bicycle_spots"`
	BlueBike             bool          `json:"blue_bike"`
	Bus                  bool          `json:"bus"`
	Tram                 bool          `json:"tram"`
	Metro                bool          `json:"metro"`
	WheelchairAvailable  bool          `json:"wheelchair_available"`
	Ramp                 bool          `json:"ramp"`
	DisabledParkingSpots int           `json:"disabled_parking_spots"`
	ElevatedPlatform     bool          `json:"elevated_platform"`
	EscalatorUp          bool          `json:"escalator_up"`
	EscalatorDown        bool          `json:"escalator_down"`
	ElevatorPlatform     bool          `json:"elevator_platform"`
	AudioInductionLoop   bool          `json:"audio_induction_loop"`
	SalesHours           []SalesWindow `json:"sales_hours,omitempty"`
}

// StepFree reports whether the station has at least one step-free boarding aid.
func (f *Facilities) StepFree() bool {
	if f == nil {
		return false
	}
	return f.WheelchairAvailable || f.Ramp || f.ElevatorPlatform
}

var (
	once       sync.Once
	stations   []*Station
	byAlias    map[string]*Station
	byURI      map[string]*Station
	facilities map[string]*Facilities
)

// Fold normalizes a station name or code for lookup: lowercase, accents
// removed, and every non-alphanumeric run collapsed away. "Liège-Guillemins",
// "liege guillemins" and "LIEGEGUILLEMINS" all fold to the same key.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'à' && r <= 'å', r == 'ä', r == 'â':
			b.WriteRune('a')
		case r >= 'è' && r <= 'ë':
			b.WriteRune('e')
		case r >= 'ì' && r <= 'ï':
			b.WriteRune('i')
		case r >= 'ò' && r <= 'ö':
			b.WriteRune('o')
		case r >= 'ù' && r <= 'ü':
			b.WriteRune('u')
		case r == 'ç':
			b.WriteRune('c')
		case r == 'ñ':
			b.WriteRune('n')
		case r == 'ß':
			b.WriteString("ss")
		}
	}
	return b.String()
}

func parseBool(s string) bool { return strings.TrimSpace(s) == "1" }

func parseInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// idFromURI extracts the numeric station id from an iRail station URI.
func idFromURI(uri string) string {
	if i := strings.LastIndex(uri, "/"); i >= 0 && i+1 < len(uri) {
		return uri[i+1:]
	}
	return ""
}

func load() {
	byAlias = make(map[string]*Station, 4096)
	byURI = make(map[string]*Station, 1024)
	facilities = make(map[string]*Facilities, 1024)

	if rows, err := csv.NewReader(strings.NewReader(stationsCSV)).ReadAll(); err == nil && len(rows) > 1 {
		for _, rec := range rows[1:] {
			if len(rec) < 13 || strings.TrimSpace(rec[0]) == "" {
				continue
			}
			st := &Station{
				URI:          rec[0],
				ID:           idFromURI(rec[0]),
				Name:         rec[1],
				NameFR:       rec[2],
				NameNL:       rec[3],
				NameDE:       rec[4],
				NameEN:       rec[5],
				TafTap:       rec[6],
				Telegraph:    rec[7],
				Country:      rec[8],
				Longitude:    parseFloat(rec[9]),
				Latitude:     parseFloat(rec[10]),
				AvgStopTimes: parseFloat(rec[11]),
			}
			if v := strings.TrimSpace(rec[12]); v != "" {
				st.TransferSeconds = parseInt(v)
				st.HasTransfer = true
			}
			stations = append(stations, st)
			byURI[st.URI] = st
			if st.ID != "" {
				register(st, st.ID)
				register(st, "be.nmbs."+st.ID)
			}
			for _, alias := range st.aliasStrings() {
				register(st, alias)
			}
		}
	}

	if rows, err := csv.NewReader(strings.NewReader(facilitiesCSV)).ReadAll(); err == nil && len(rows) > 1 {
		days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
		for _, rec := range rows[1:] {
			if len(rec) < 36 || strings.TrimSpace(rec[0]) == "" {
				continue
			}
			f := &Facilities{
				URI:                  rec[0],
				Name:                 rec[1],
				Street:               rec[2],
				Zip:                  rec[3],
				City:                 rec[4],
				TicketVendingMachine: parseBool(rec[5]),
				LuggageLockers:       parseBool(rec[6]),
				FreeParking:          parseBool(rec[7]),
				Taxi:                 parseBool(rec[8]),
				BicycleSpots:         parseBool(rec[9]),
				BlueBike:             parseBool(rec[10]),
				Bus:                  parseBool(rec[11]),
				Tram:                 parseBool(rec[12]),
				Metro:                parseBool(rec[13]),
				WheelchairAvailable:  parseBool(rec[14]),
				Ramp:                 parseBool(rec[15]),
				DisabledParkingSpots: parseInt(rec[16]),
				ElevatedPlatform:     parseBool(rec[17]),
				EscalatorUp:          parseBool(rec[18]),
				EscalatorDown:        parseBool(rec[19]),
				ElevatorPlatform:     parseBool(rec[20]),
				AudioInductionLoop:   parseBool(rec[21]),
			}
			for i, day := range days {
				open, closed := strings.TrimSpace(rec[22+i*2]), strings.TrimSpace(rec[23+i*2])
				if open == "" && closed == "" {
					continue
				}
				f.SalesHours = append(f.SalesHours, SalesWindow{Day: day, Open: open, Close: closed})
			}
			facilities[f.URI] = f
		}
	}
}

// aliasStrings returns every spelling that should resolve to this station,
// including each half of slash-joined bilingual names such as
// "Brussel-Zuid/Bruxelles-Midi".
func (s *Station) aliasStrings() []string {
	raw := []string{s.Name, s.NameFR, s.NameNL, s.NameDE, s.NameEN, s.Telegraph, s.TafTap}
	out := make([]string, 0, len(raw)*2)
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
		if strings.Contains(v, "/") {
			out = append(out, strings.Split(v, "/")...)
		}
	}
	return out
}

// register indexes one alias. First writer wins so that a busy station never
// loses its alias to a quieter namesake; ties are broken by AvgStopTimes.
func register(s *Station, alias string) {
	key := Fold(alias)
	if key == "" {
		return
	}
	if prev, ok := byAlias[key]; ok {
		if prev.AvgStopTimes >= s.AvgStopTimes {
			return
		}
	}
	byAlias[key] = s
}

// All returns every known station.
func All() []*Station {
	once.Do(load)
	return stations
}

// Lookup resolves a user-supplied station name, telegraph code, TAF/TAP code,
// numeric id or BE.NMBS id to a station. It reports false when nothing matches.
func Lookup(query string) (*Station, bool) {
	once.Do(load)
	key := Fold(query)
	if key == "" {
		return nil, false
	}
	if st, ok := byAlias[key]; ok {
		return st, true
	}
	return nil, false
}

// Search returns stations whose folded aliases contain the query, busiest
// first, capped at limit.
func Search(query string, limit int) []*Station {
	once.Do(load)
	key := Fold(query)
	if key == "" || limit <= 0 {
		return nil
	}
	var out []*Station
	seen := make(map[string]bool)
	for _, st := range stations {
		if seen[st.URI] {
			continue
		}
		for _, alias := range st.aliasStrings() {
			if strings.Contains(Fold(alias), key) {
				out = append(out, st)
				seen[st.URI] = true
				break
			}
		}
	}
	// Busiest first so "brussels" surfaces Brussels-South before a minor halt.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].AvgStopTimes > out[j-1].AvgStopTimes; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// FacilitiesFor returns the facilities row for a station, if one exists.
func FacilitiesFor(st *Station) (*Facilities, bool) {
	once.Do(load)
	if st == nil {
		return nil, false
	}
	f, ok := facilities[st.URI]
	return f, ok
}

// TransferSecondsFor returns the official minimum transfer time for a station
// name. The second result is false when the station is unknown or the upstream
// dataset carries no transfer time for it, so callers never treat missing data
// as a zero-second transfer.
func TransferSecondsFor(name string) (int, bool) {
	st, ok := Lookup(name)
	if !ok || !st.HasTransfer {
		return 0, false
	}
	return st.TransferSeconds, true
}
