// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Flighty Airports RSC extraction glue. This file is not
// generator-emitted; keep it in its own file so regen-merge preserves it.
//
// flighty.com/airports embeds all page data as Next.js RSC flight chunks.
// The generated endpoint commands call extractFlightyRSC(kind, raw) instead
// of the generic embedded-json extractor, which cannot see RSC payloads.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/flighty/internal/rsc"
)

// flightyCatalogRegionMarker anchors the regions object on the homepage and
// TV dashboard payloads. The marker ends with the opening brace.
const flightyCatalogRegionMarker = `"regions":{`

// flightyDetailMarker anchors the airport props object on detail pages. The
// marker begins with the opening brace.
const flightyDetailMarker = `{"iata":"`

// flightyPerformanceMarker anchors the today-performance object.
const flightyPerformanceMarker = `{"today":{"departurePerformance"`

// flightyBoardMarker anchors the flight-board array.
const flightyFlightBoardMarker = `"initialFlights":[`

// extractFlightyRSC extracts the typed JSON surface for one page kind from
// raw SSR HTML. Kinds:
//   - "catalog": array of airport summaries (homepage + TV dashboard), each
//     annotated with the region it was listed under.
//   - "detail": one airport object with weather and today-performance merged.
//   - "board": array of flight-board entries (arrivals or departures).
func extractFlightyRSC(kind string, raw []byte) (json.RawMessage, error) {
	chunks, err := rsc.ExtractChunks(raw)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "catalog":
		return extractFlightyCatalog(chunks)
	case "detail":
		return extractFlightyDetail(chunks)
	case "board":
		return extractFlightyBoard(chunks)
	default:
		return nil, fmt.Errorf("flighty: unknown extraction kind %q", kind)
	}
}

// flightyCatalogAirport is one catalog row as embedded on the meltdown map.
type flightyCatalogAirport struct {
	ID              string          `json:"id"`
	Slug            string          `json:"slug"`
	Name            string          `json:"name"`
	IATA            string          `json:"iata"`
	City            string          `json:"city"`
	Location        json.RawMessage `json:"location,omitempty"`
	Status          string          `json:"status,omitempty"`
	Arrival         json.RawMessage `json:"arrival,omitempty"`
	Departure       json.RawMessage `json:"departure,omitempty"`
	Warnings        []string        `json:"warnings,omitempty"`
	CumulativeDelay int             `json:"cumulativeDelay,omitempty"`
	// Region is CLI-added: which map region the airport was listed under.
	Region string `json:"region,omitempty"`
}

// extractFlightyCatalog flattens the region-grouped catalog into one array.
// The "All" region carries the inline airport objects; specific-region arrays
// hold RSC references of the form
// "$f:props:children:props:regions:All:airports:N" pointing at All's array
// positions. De-duplication is by IATA; the specific region wins over "All".
func extractFlightyCatalog(chunks string) (json.RawMessage, error) {
	obj, err := rsc.FindObject(chunks, flightyCatalogRegionMarker)
	if err != nil {
		return nil, fmt.Errorf("flighty: extracting catalog regions: %w", err)
	}
	// The anchored fragment IS the regions map: {"<Region>":{"airports":[...]}}.
	var catalog map[string]struct {
		Airports []json.RawMessage `json:"airports"`
	}
	if err := json.Unmarshal(obj, &catalog); err != nil {
		return nil, fmt.Errorf("flighty: parsing catalog regions: %w", err)
	}

	// Position-indexed rows from "All": position N is what references point at.
	allRaw := catalog["All"].Airports
	allRows := make([]*flightyCatalogAirport, len(allRaw))
	for i, raw := range allRaw {
		var ap flightyCatalogAirport
		if err := json.Unmarshal(raw, &ap); err != nil {
			continue // reference or non-object element; position stays nil
		}
		allRows[i] = &ap
	}

	byIATA := map[string]flightyCatalogAirport{}
	order := []string{}
	add := func(ap flightyCatalogAirport) {
		key := strings.ToUpper(strings.TrimSpace(ap.IATA))
		if key == "" {
			key = ap.Slug
		}
		if _, seen := byIATA[key]; !seen {
			order = append(order, key)
		}
		byIATA[key] = ap
	}

	// Specific regions first so their region assignment wins over "All".
	regionNames := make([]string, 0, len(catalog))
	for region := range catalog {
		if region != "All" {
			regionNames = append(regionNames, region)
		}
	}
	sort.Strings(regionNames)
	for _, region := range regionNames {
		for _, raw := range catalog[region].Airports {
			if ap, ok := flightyParseCatalogEntry(raw, allRows); ok {
				ap.Region = region
				add(*ap)
			}
		}
	}
	// Any All-only airports the specific regions did not reference.
	for _, ap := range allRows {
		if ap == nil {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(ap.IATA))
		if key == "" {
			key = ap.Slug
		}
		if _, seen := byIATA[key]; !seen {
			cp := *ap
			cp.Region = "All"
			add(cp)
		}
	}

	out := make([]flightyCatalogAirport, 0, len(order))
	for _, key := range order {
		out = append(out, byIATA[key])
	}
	// Deterministic order for stable output and diffs.
	sort.Slice(out, func(i, j int) bool { return out[i].IATA < out[j].IATA })
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("flighty: marshaling catalog: %w", err)
	}
	return data, nil
}

// flightyRefRe matches RSC array-position references into the All region's
// airports array: "$f:props:children:props:regions:All:airports:N".
var flightyRefRe = regexp.MustCompile(`\$f:props:children:props:regions:All:airports:(\d+)$`)

// flightyParseCatalogEntry decodes one regions-array element: either an inline
// airport object or a reference into the All array. ok is false for
// unparseable elements.
func flightyParseCatalogEntry(raw json.RawMessage, allRows []*flightyCatalogAirport) (*flightyCatalogAirport, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, false
	}
	if trimmed[0] == '"' {
		var ref string
		if err := json.Unmarshal(raw, &ref); err != nil {
			return nil, false
		}
		m := flightyRefRe.FindStringSubmatch(strings.TrimSpace(ref))
		if m == nil {
			return nil, false
		}
		idx := 0
		fmt.Sscanf(m[1], "%d", &idx)
		if idx < 0 || idx >= len(allRows) || allRows[idx] == nil {
			return nil, false
		}
		cp := *allRows[idx]
		return &cp, true
	}
	var ap flightyCatalogAirport
	if err := json.Unmarshal(raw, &ap); err != nil {
		return nil, false
	}
	return &ap, true
}

// extractFlightyDetail merges the airport props object (identity, weather)
// with the today-performance object into one AirportDetail JSON value.
func extractFlightyDetail(chunks string) (json.RawMessage, error) {
	props, err := rsc.FindObject(chunks, flightyDetailMarker)
	if err != nil {
		return nil, fmt.Errorf("flighty: extracting airport detail: %w", err)
	}
	var detail map[string]any
	if err := json.Unmarshal(props, &detail); err != nil {
		return nil, fmt.Errorf("flighty: parsing airport detail: %w", err)
	}
	// The props object ends before share-link siblings; performance lives in
	// a separate payload island whose root object wraps the real "today" map.
	today, todayErr := rsc.FindObject(chunks, flightyPerformanceMarker)
	if todayErr == nil {
		var perf map[string]json.RawMessage
		if err := json.Unmarshal(today, &perf); err == nil {
			if inner, ok := perf["today"]; ok {
				detail["today"] = json.RawMessage(inner)
			} else {
				detail["today"] = json.RawMessage(today)
			}
		}
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("flighty: marshaling airport detail: %w", err)
	}
	return data, nil
}

// extractFlightyBoard returns the initialFlights array as JSON.
func extractFlightyBoard(chunks string) (json.RawMessage, error) {
	arr, err := rsc.FindArray(chunks, flightyFlightBoardMarker)
	if err != nil {
		return nil, fmt.Errorf("flighty: extracting flight board: %w", err)
	}
	var flights []json.RawMessage
	if err := json.Unmarshal(arr, &flights); err != nil {
		return nil, fmt.Errorf("flighty: parsing flight board: %w", err)
	}
	if flights == nil {
		flights = []json.RawMessage{}
	}
	data, err := json.Marshal(flights)
	if err != nil {
		return nil, fmt.Errorf("flighty: marshaling flight board: %w", err)
	}
	return data, nil
}

// flightyPageKinds maps sync resource names and page paths to extraction kinds.
func flightyPageKindForPath(path string) string {
	switch {
	case path == "/airports" || path == "/airports/tv":
		return "catalog"
	case strings.HasPrefix(path, "/airports/") && strings.HasSuffix(path, "/arrivals"),
		strings.HasPrefix(path, "/airports/") && strings.HasSuffix(path, "/departures"):
		return "board"
	case strings.HasPrefix(path, "/airports/"):
		return "detail"
	default:
		return ""
	}
}
