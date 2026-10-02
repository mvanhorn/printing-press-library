package mcp

import (
	"strings"
	"testing"
)

func TestAirportPagePathsRejectAmbiguousOrInjectedInputs(t *testing.T) {
	for _, args := range []map[string]any{{"route-id": "../secret"}, {"route-id": "Haneda-Narita", "direction": 1.5}, {"route-id": "Haneda-Narita", "date": "2026-99-01"}, {"route-id": "Haneda-Narita", "seats": 1}} {
		if _, _, err := airportPagePath("timetable", args); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
	path, _, err := airportPagePath("timetable", map[string]any{"route-id": "Haneda-Narita", "date": "2026-10-03", "direction": float64(2)})
	if err != nil || !strings.Contains(path, "d=2026-10-03") || !strings.Contains(path, "dir=2") {
		t.Fatalf("%s %v", path, err)
	}
}
