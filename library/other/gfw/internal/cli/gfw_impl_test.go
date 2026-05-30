// Hand-authored — NOT generated. Tests for the GFW novel-feature pure logic
// (risk scoring, watch-window parsing, vessel-identity flattening, encounter
// counterpart extraction). No network.
package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestScoreRisk(t *testing.T) {
	cases := []struct {
		name      string
		counts    map[string]int
		wantLevel string
	}{
		{"empty", map[string]int{}, "low"},
		{"low-fishing", map[string]int{"fishing": 3, "port_visit": 2}, "low"},
		{"medium-encounters", map[string]int{"encounter": 3}, "medium"},
		{"high-gaps", map[string]int{"gap": 4, "encounter": 2}, "high"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			score, level, signals := scoreRisk(c.counts)
			if level != c.wantLevel {
				t.Errorf("level = %q, want %q (score %d)", level, c.wantLevel, score)
			}
			if score < 0 || score > 100 {
				t.Errorf("score %d out of range", score)
			}
			if len(signals) == 0 {
				t.Error("signals must never be empty")
			}
		})
	}
}

func TestParseWatchWindow(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 7 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"bogus", 0, true},
		{"-5d", 0, true},
	}
	for _, c := range cases {
		got, err := parseWatchWindow(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseWatchWindow(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseWatchWindow(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseWatchWindow(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestExtractVesselIdentity(t *testing.T) {
	raw := json.RawMessage(`{"entries":[{
		"selfReportedInfo":[{"id":"V1","shipname":"PROGRESS 10","flag":"COK","ssvid":"518100675","imo":"9213208","callsign":"E5U3586"}],
		"combinedSourcesInfo":[{"shiptypes":["CARGO"]}]
	}]}`)
	got := extractVesselIdentity(raw, "fallback")
	if got.ID != "V1" || got.Name != "PROGRESS 10" || got.Flag != "COK" || got.SSVID != "518100675" || got.IMO != "9213208" || got.CallSign != "E5U3586" {
		t.Errorf("identity mismatch: %+v", got)
	}
	if got.ShipType != "CARGO" {
		t.Errorf("ship_type = %q, want CARGO", got.ShipType)
	}

	// Bare object with no identity falls back to the requested id.
	empty := extractVesselIdentity(json.RawMessage(`{}`), "req-id")
	if empty.ID != "req-id" {
		t.Errorf("fallback id = %q, want req-id", empty.ID)
	}
}

func TestEncounterCounterpart(t *testing.T) {
	var ev map[string]any
	_ = json.Unmarshal([]byte(`{"id":"E1","encounter":{"vessel":{"id":"C1","name":"PARTNER","flag":"PAN"}}}`), &ev)
	id, name, flag, encRaw := encounterCounterpart(ev)
	if id != "C1" || name != "PARTNER" || flag != "PAN" {
		t.Errorf("counterpart = (%q,%q,%q), want (C1,PARTNER,PAN)", id, name, flag)
	}
	if len(encRaw) == 0 {
		t.Error("encRaw should carry the raw encounter object")
	}

	// No encounter object → empty counterpart, nil raw.
	id2, _, _, raw2 := encounterCounterpart(map[string]any{"id": "E2"})
	if id2 != "" || raw2 != nil {
		t.Errorf("expected empty counterpart for no-encounter event, got id=%q raw=%v", id2, raw2)
	}
}

func TestEntriesOf(t *testing.T) {
	// List envelope with entries → returns them.
	if got := entriesOf(json.RawMessage(`{"total":2,"entries":[{"id":"a"},{"id":"b"}]}`)); len(got) != 2 {
		t.Errorf("populated envelope: got %d entries, want 2", len(got))
	}
	// Empty-entries list envelope → empty, NOT a phantom entry (the bug).
	if got := entriesOf(json.RawMessage(`{"total":0,"limit":2,"offset":0,"entries":[]}`)); len(got) != 0 {
		t.Errorf("empty envelope: got %d entries, want 0 (phantom-entry regression)", len(got))
	}
	// Bare object with no entries key (get-by-id) → wrapped as one entry.
	if got := entriesOf(json.RawMessage(`{"selfReportedInfo":[{"id":"V1"}]}`)); len(got) != 1 {
		t.Errorf("bare object: got %d entries, want 1", len(got))
	}
}
