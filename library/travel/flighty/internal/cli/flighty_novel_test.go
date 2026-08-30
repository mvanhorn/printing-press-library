// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the Flighty novel-command pure logic and the RSC
// extraction glue. Fixtures mirror real SSR payload shapes captured from
// flighty.com/airports.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func flightyTestAirport(iata, name, city, region, status string, delay int, lat, lon float64) flightyCatalogAirport {
	loc, _ := json.Marshal(flightyLatLon{Latitude: lat, Longitude: lon})
	return flightyCatalogAirport{
		IATA:            iata,
		Name:            name,
		City:            city,
		Region:          region,
		Status:          status,
		CumulativeDelay: delay,
		Location:        loc,
	}
}

func TestFlightyRankWorstOrdersByCumulativeDelay(t *testing.T) {
	airports := []flightyCatalogAirport{
		flightyTestAirport("AAA", "Alpha", "Alpha City", "Europe", "MINOR_ISSUES", 120, 50, 8),
		flightyTestAirport("BBB", "Bravo", "Bravo City", "Europe", "MAJOR_ISSUES", 400, 51, 9),
		flightyTestAirport("CCC", "Charlie", "Charlie City", "Asia", "NORMAL_OPERATIONS", 0, 35, 139),
	}
	rows := flightyRankWorst(airports, "", "", 0)
	if len(rows) != 3 {
		t.Fatalf("expected all rows, got %d", len(rows))
	}
	if rows[0].IATA != "BBB" || rows[2].IATA != "CCC" {
		t.Fatalf("unexpected order: %s, %s, %s", rows[0].IATA, rows[1].IATA, rows[2].IATA)
	}
	// Region filter
	rows = flightyRankWorst(airports, "asia", "", 0)
	if len(rows) != 1 || rows[0].IATA != "CCC" {
		t.Fatalf("region filter failed: %+v", rows)
	}
	// Status filter
	rows = flightyRankWorst(airports, "", "MAJOR_ISSUES", 0)
	if len(rows) != 1 || rows[0].IATA != "BBB" {
		t.Fatalf("status filter failed: %+v", rows)
	}
	// Limit
	rows = flightyRankWorst(airports, "", "", 2)
	if len(rows) != 2 {
		t.Fatalf("limit failed: got %d rows", len(rows))
	}
	// Negative test: filter that matches nothing must return empty, not all.
	rows = flightyRankWorst(airports, "pacific", "", 0)
	if len(rows) != 0 {
		t.Fatalf("mismatching region filter leaked rows: %+v", rows)
	}
}

func TestFlightyRankNearbyDistanceAndHealth(t *testing.T) {
	// SFO at 37.62,-122.38; OAK ~16km away; SJC ~42km; FAR ~2400km.
	airports := []flightyCatalogAirport{
		flightyTestAirport("SFO", "San Francisco Intl.", "San Francisco", "North America", "MAJOR_ISSUES", 900, 37.6188, -122.375),
		flightyTestAirport("OAK", "Oakland Intl.", "Oakland", "North America", "NORMAL_OPERATIONS", 10, 37.7126, -122.2197),
		flightyTestAirport("SJC", "San Jose Intl.", "San Jose", "North America", "MINOR_ISSUES", 60, 37.3639, -121.9289),
		flightyTestAirport("FAR", "Hector Intl.", "Fargo", "North America", "NORMAL_OPERATIONS", 0, 46.9207, -96.8176),
	}
	origin := flightyLatLon{Latitude: 37.6188, Longitude: -122.375}
	// Healthy-only: OAK only (SJC is MINOR_ISSUES; FAR beyond max-km default 500).
	rows := flightyRankNearby(airports, "SFO", origin, true, 500, 0)
	if len(rows) != 1 {
		t.Fatalf("healthy-only filter failed: %+v", rows)
	}
	if rows[0].IATA != "OAK" {
		t.Fatalf("nearest healthy should be OAK, got %s (%.1f km)", rows[0].IATA, rows[0].DistanceKm)
	}
	if rows[0].DistanceKm > 25 || rows[0].DistanceKm < 5 {
		t.Fatalf("OAK distance implausible: %.1f km", rows[0].DistanceKm)
	}
	// Without health filter but with tight max-km, MAJOR_ISSUES SFO origin excluded, others within range.
	rows = flightyRankNearby(airports, "SFO", origin, false, 100, 0)
	if len(rows) != 2 {
		t.Fatalf("max-km filter failed: %+v", rows)
	}
	// Origin must never appear in its own nearby list.
	for _, r := range rows {
		if r.IATA == "SFO" {
			t.Fatal("origin leaked into nearby results")
		}
	}
	// Absence-of-correctness: no airports in range -> empty, not fabricated rows.
	rows = flightyRankNearby(airports, "SFO", origin, true, 5, 0)
	if len(rows) != 0 {
		t.Fatalf("expected empty result inside 5km, got %+v", rows)
	}
}

func TestFlightySplitFlightNumber(t *testing.T) {
	tests := []struct{ in, num, airline string }{
		{"UA5072", "5072", "UA"},
		{"5072", "5072", ""},
		{"dl10", "10", "DL"},
		{"LH1234", "1234", "LH"},
	}
	for _, tt := range tests {
		num, airline := flightySplitFlightNumber(tt.in)
		if num != tt.num || airline != tt.airline {
			t.Fatalf("split(%q) = (%q, %q), want (%q, %q)", tt.in, num, airline, tt.num, tt.airline)
		}
	}
}

func TestFlightyDiffSnapshots(t *testing.T) {
	mk := func(iata, status string, delay int, warnings ...string) json.RawMessage {
		raw, _ := json.Marshal(flightySnapshotAirport{IATA: iata, Name: "Test " + iata, Status: status, CumulativeDelay: delay, Warnings: warnings})
		return raw
	}
	prev := map[string]json.RawMessage{
		"DEN": mk("DEN", "NORMAL_OPERATIONS", 0),
		"EWR": mk("EWR", "MINOR_ISSUES", 120, "Ground Delay"),
		"LGA": mk("LGA", "MINOR_ISSUES", 60),
	}
	cur := map[string]json.RawMessage{
		"DEN": mk("DEN", "MAJOR_ISSUES", 240, "Ground Delay", "High Cancellations"),
		"EWR": mk("EWR", "MINOR_ISSUES", 150), // warning cleared, delay delta
		"LGA": mk("LGA", "MINOR_ISSUES", 60),  // unchanged -> no change row
	}
	changes := flightyDiffSnapshots(prev, cur)
	byIATA := map[string]flightyStatusChange{}
	for _, ch := range changes {
		byIATA[ch.IATA] = ch
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes (DEN, EWR), got %d: %+v", len(changes), changes)
	}
	den := byIATA["DEN"]
	if den.From != "NORMAL_OPERATIONS" || den.To != "MAJOR_ISSUES" {
		t.Fatalf("DEN status transition wrong: %+v", den)
	}
	if len(den.Added) != 2 {
		t.Fatalf("DEN should have 2 added warnings: %+v", den.Added)
	}
	if den.DelayDelta == nil || *den.DelayDelta != 240 {
		t.Fatalf("DEN delay delta wrong: %+v", den.DelayDelta)
	}
	ewr := byIATA["EWR"]
	if len(ewr.Cleared) != 1 || ewr.Cleared[0] != "Ground Delay" {
		t.Fatalf("EWR cleared warnings wrong: %+v", ewr.Cleared)
	}
	if ewr.From != "" {
		t.Fatalf("EWR status did not change but From is set: %+v", ewr)
	}
	// Absence-of-correctness: identical snapshots must produce zero changes.
	same := flightyDiffSnapshots(cur, cur)
	if len(same) != 0 {
		t.Fatalf("identical snapshots produced changes: %+v", same)
	}
}

// TestExtractFlightyCatalogResolvesReferences exercises the RSC reference
// resolution: specific regions reference the All array by position.
func TestExtractFlightyCatalogResolvesReferences(t *testing.T) {
	// Simulated RSC chunks: All holds inline objects; Europe references them.
	push := func(s string) string {
		// Minimal escaping: our payloads contain no quotes needing escape
		// beyond the ones we add here.
		return `<script>self.__next_f.push([1,"` + strings.ReplaceAll(s, `"`, `\"`) + `"])</script>`
	}
	all := `"All":{"airports":[` +
		`{"iata":"HAM","slug":"hamburg-ham","name":"Hamburg","city":"Hamburg","status":"MINOR_ISSUES","cumulativeDelay":90,"location":{"latitude":53.63,"longitude":9.99}},` +
		`{"iata":"LGW","slug":"london-gatwick-lgw","name":"London Gatwick","city":"London","status":"MAJOR_ISSUES","cumulativeDelay":300,"location":{"latitude":51.15,"longitude":-0.18}},` +
		`{"iata":"SFO","slug":"san-francisco-intl-sfo","name":"San Francisco Intl.","city":"San Francisco","status":"NORMAL_OPERATIONS","cumulativeDelay":0,"location":{"latitude":37.62,"longitude":-122.38}}]}` +
		`,"Europe":{"airports":["$f:props:children:props:regions:All:airports:0","$f:props:children:props:regions:All:airports:1"]}` +
		`,"North America":{"airports":["$f:props:children:props:regions:All:airports:2"]}`
	html := push(`2:{"regionNames":["All","Europe","North America"],"regions":{` + all + `}}`)
	out, err := extractFlightyRSC("catalog", []byte(html))
	if err != nil {
		t.Fatalf("catalog extraction failed: %v", err)
	}
	var rows []flightyCatalogAirport
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("catalog is not valid JSON: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 unique airports, got %d: %+v", len(rows), rows)
	}
	byIATA := map[string]flightyCatalogAirport{}
	for _, r := range rows {
		byIATA[r.IATA] = r
	}
	if got := byIATA["HAM"].Region; got != "Europe" {
		t.Fatalf("HAM region = %q, want Europe", got)
	}
	if got := byIATA["SFO"].Region; got != "North America" {
		t.Fatalf("SFO region = %q, want North America", got)
	}
	// Reference resolution must carry the full row, not a stub.
	if byIATA["LGW"].CumulativeDelay != 300 || byIATA["LGW"].Status != "MAJOR_ISSUES" {
		t.Fatalf("LGW reference did not resolve to full row: %+v", byIATA["LGW"])
	}
}

func TestExtractFlightyDetailMergesPerformance(t *testing.T) {
	push := func(s string) string {
		return `<script>self.__next_f.push([1,"` + strings.ReplaceAll(s, `"`, `\"`) + `"])</script>`
	}
	html := push(`1f:[["$","$L21",null,{"iata":"DEN","name":"Denver Intl.","city":"Denver","timezone":"America/Denver","airportWeather":{"temperature":22.5,"flightRules":"VFR","rawMetar":"METAR KDEN"}}],` +
		`["$","$L33",null,{"data":{"today":{"departurePerformance":{"numOperations":1000,"onTime":{"absolute":900,"percentage":"90%"}},"arrivalPerformance":{"numOperations":980}}}}]]`)
	out, err := extractFlightyRSC("detail", []byte(html))
	if err != nil {
		t.Fatalf("detail extraction failed: %v", err)
	}
	var d struct {
		IATA           string `json:"iata"`
		AirportWeather struct {
			RawMetar string `json:"rawMetar"`
		} `json:"airportWeather"`
		Today struct {
			DeparturePerformance struct {
				NumOperations int `json:"numOperations"`
			} `json:"departurePerformance"`
		} `json:"today"`
	}
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatalf("detail is not valid JSON: %v", err)
	}
	if d.IATA != "DEN" || d.AirportWeather.RawMetar != "METAR KDEN" {
		t.Fatalf("identity/weather fields lost: %+v", d)
	}
	if d.Today.DeparturePerformance.NumOperations != 1000 {
		t.Fatalf("performance not merged: %+v", d.Today)
	}
}

func TestExtractFlightyBoard(t *testing.T) {
	push := func(s string) string {
		return `<script>self.__next_f.push([1,"` + strings.ReplaceAll(s, `"`, `\"`) + `"])</script>`
	}
	html := push(`2:["$","$L27",null,{"initialFlights":[{"id":"f1","city":"Riverton","status":[{"type":"text","text":"4h 1m Late","style":"RED"}],"originalTime":{"text":"06:36","style":"GRAY_STRIKETHROUGH"},"newTime":{"text":"10:37","style":"RED"},"secondaryCorner":"Belt 18","airline":{"iata":"UA","name":"United"},"flightNumber":"5072","departure":{"iata":"RIW"},"arrival":{"iata":"DEN","gate":"B11","belt":"18"}}]}]`)
	out, err := extractFlightyRSC("board", []byte(html))
	if err != nil {
		t.Fatalf("board extraction failed: %v", err)
	}
	var flights []flightyBoardFlight
	if err := json.Unmarshal(out, &flights); err != nil {
		t.Fatalf("board is not valid JSON: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
	f := flights[0]
	if f.FlightNumber != "5072" || f.Airline.IATA != "UA" || f.Arrival.Gate != "B11" {
		t.Fatalf("board fields lost: %+v", f)
	}
	if got := f.StatusText(); got != "4h 1m Late" {
		t.Fatalf("StatusText() = %q, want \"4h 1m Late\"", got)
	}
}
