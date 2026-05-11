package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "store.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertSitesIdempotent(t *testing.T) {
	s := openTempStore(t)
	sites := []omnilogic.Site{
		{MspSystemID: 1, BackyardName: "Main Pool"},
		{MspSystemID: 2, BackyardName: "Vacation Home"},
	}
	if err := s.UpsertSites(sites); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Run again; should overwrite cleanly
	sites[0].BackyardName = "Main Pool Renamed"
	if err := s.UpsertSites(sites); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := s.ListSites()
	if err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(got))
	}
	if got[0].BackyardName != "Main Pool Renamed" {
		t.Errorf("rename didn't apply: got %q", got[0].BackyardName)
	}
}

func TestAppendTelemetryAndQuery(t *testing.T) {
	s := openTempStore(t)
	air := 75
	water := 82
	ph := 7.6
	orp := 700
	salt := 3000
	tele := &omnilogic.Telemetry{
		MspSystemID: 1,
		AirTemp:     &air,
		SampledAt:   time.Now().UTC(),
		BodiesOfWater: []omnilogic.TelemetryBOW{{
			SystemID: "10", WaterTemp: &water, PH: &ph, ORP: &orp, SaltPPM: &salt,
		}},
	}
	count, err := s.AppendTelemetry(tele)
	if err != nil {
		t.Fatalf("AppendTelemetry: %v", err)
	}
	if count < 5 {
		t.Errorf("expected >=5 samples appended, got %d", count)
	}
	samples, err := s.QueryTelemetry(1, "ph", "", 0)
	if err != nil {
		t.Fatalf("QueryTelemetry: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 ph sample, got %d", len(samples))
	}
	if !samples[0].ValueReal.Valid || samples[0].ValueReal.Float64 != 7.6 {
		t.Errorf("ph value wrong: %+v", samples[0])
	}
	salts, err := s.QueryTelemetry(1, "salt_ppm", "", 0)
	if err != nil {
		t.Fatalf("QueryTelemetry salt: %v", err)
	}
	if len(salts) != 1 || !salts[0].ValueInt.Valid || salts[0].ValueInt.Int64 != 3000 {
		t.Errorf("salt value wrong: %+v", salts)
	}
}

func TestUpsertAlarmsClearsStale(t *testing.T) {
	s := openTempStore(t)
	first := []omnilogic.Alarm{
		{EquipmentID: "10", Code: "PH_LOW", Message: "pH low"},
		{EquipmentID: "11", Code: "PUMP_FAIL", Message: "pump failed"},
	}
	if err := s.UpsertAlarms(1, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Second sync: PH_LOW gone, PUMP_FAIL still present
	second := []omnilogic.Alarm{
		{EquipmentID: "11", Code: "PUMP_FAIL", Message: "pump failed"},
	}
	if err := s.UpsertAlarms(1, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	// PH_LOW should be cleared (cleared_at NOT NULL)
	row := s.DB.QueryRow(`SELECT cleared_at FROM alarms WHERE code = 'PH_LOW'`)
	var cleared string
	if err := row.Scan(&cleared); err != nil {
		t.Fatalf("scan cleared: %v", err)
	}
	if cleared == "" {
		t.Errorf("PH_LOW should be cleared, got empty")
	}
}

func TestCommandLog(t *testing.T) {
	s := openTempStore(t)
	id, err := s.LogCommand(CommandLogEntry{
		Op:     "SetHeaterEnable",
		Target: "heater 5",
		Params: map[string]any{"enable": true},
		Status: "ok",
	})
	if err != nil {
		t.Fatalf("LogCommand: %v", err)
	}
	if id == 0 {
		t.Errorf("expected non-zero id")
	}
}
