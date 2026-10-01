package cli

import (
	"testing"
)

func TestEvidenceProjectionRejectsUnknownAroundEmptyAndNull(t *testing.T) {
	cases := []struct {
		path   string
		v      map[string]any
		fields string
	}{
		{"weather forecast", map[string]any{"daily": []any{}, "hourly": []any{}, "location": map[string]any{"id": "jcode:1"}}, "definitely_missing"},
		{"season show", map[string]any{"location": map[string]any{"id": "384"}, "predictions": []any{}}, "location,definitely_missing"},
		{"weather forecast", map[string]any{"daily": []any{}, "observation": nil}, "daily.made_up"},
	}
	for _, c := range cases {
		if _, e := projectEvidence(c.v, c.fields, c.path); e == nil {
			t.Fatal("accepted unknown path", c.fields)
		}
	}
	v := map[string]any{"daily": []any{}, "observation": nil, "status": "out_of_horizon"}
	p, e := projectEvidence(v, "daily.valid_date,observation.temperature_c,status", "weather forecast")
	if e != nil {
		t.Fatal(e)
	}
	if p["observation"] != nil || len(p["daily"].([]any)) != 0 || p["status"] != "out_of_horizon" {
		t.Fatal(p)
	}
}
func TestEvidenceProjectionKeepsRequestedUnitsAndNames(t *testing.T) {
	v := map[string]any{"hourly": []any{map[string]any{"valid_at": "2026-10-01T01:00:00+09:00", "temperature_c": 18.0, "kind": "forecast"}}, "location": map[string]any{"name_ja": "京都", "elevation_m": nil}}
	p, e := projectEvidence(v, "hourly.valid_at,hourly.temperature_c,location.name_ja,location.elevation_m", "weather forecast")
	if e != nil {
		t.Fatal(e)
	}
	r := p["hourly"].([]any)[0].(map[string]any)
	if len(r) != 2 || r["temperature_c"] != 18.0 {
		t.Fatal(r)
	}
	if p["location"].(map[string]any)["name_ja"] != "京都" {
		t.Fatal(p)
	}
}

func TestProjectionSchemasDoNotInventCrossProductFields(t *testing.T) {
	if _, e := projectEvidence(map[string]any{"location": map[string]any{"id": "384"}}, "location.requested_name", "season show"); e == nil {
		t.Fatal("weather-only field accepted for season")
	}
	p, e := projectEvidence(map[string]any{"items": []any{map[string]any{"name": "Kyoto", "status": "error", "error": "timeout"}}}, "items.location,items.status", "weather compare")
	if e != nil {
		t.Fatal(e)
	}
	r := p["items"].([]any)[0].(map[string]any)
	if r["location"] != nil || r["status"] != "error" {
		t.Fatal(r)
	}
}
