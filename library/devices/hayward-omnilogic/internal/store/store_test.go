package store

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestOpenPermissions locks the user-only file/dir permissions on the
// SQLite store. Regression for the P1 security review: previously the
// store dir was 0o755 and the SQLite file 0o644 (umask-defaulted), which
// let any local user `sqlite3 ~/.local/share/.../store.sqlite` and dump
// telemetry, alarms, the audit log, etc. Both must be user-only.
//
// Skips on Windows because POSIX-mode bits don't apply there.
func TestOpenPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	// Use a nested path whose parent doesn't exist yet so we actually
	// exercise the MkdirAll(0o700) path (TempDir creates with 0o700-ish
	// on most macs but we can't rely on that across platforms).
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "haystore")
	path := filepath.Join(storeDir, "perms.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Errorf("store dir perms: want 0700, got %#o", mode)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Errorf("store file perms: want 0600, got %#o", mode)
	}
}

// TestAppendTelemetry_FiltersSentinels regression test for Greptile P1
// #3216464198: Hayward's -1 sentinel must never reach the time-series
// store. Pre-fix, AppendTelemetry wrote -1 readings, which corrupted
// drift baselines and chemistry log accuracy.
func TestAppendTelemetry_FiltersSentinels(t *testing.T) {
	s := openTempStore(t)
	bad := -1
	good := 78
	badPH := -1.0
	goodPH := 7.5
	badORP := -1
	goodORP := 700
	badSalt := -1
	goodSalt := 3000

	tele := &omnilogic.Telemetry{
		MspSystemID: 1,
		AirTemp:     &bad, // should be filtered
		SampledAt:   time.Now().UTC(),
		BodiesOfWater: []omnilogic.TelemetryBOW{
			{
				SystemID:  "10",
				WaterTemp: &bad, // filtered
				PH:        &badPH, // filtered
				ORP:       &badORP, // filtered
				SaltPPM:   &badSalt, // filtered
			},
			{
				SystemID:  "11",
				WaterTemp: &good,
				PH:        &goodPH,
				ORP:       &goodORP,
				SaltPPM:   &goodSalt,
			},
		},
	}
	count, err := s.AppendTelemetry(tele)
	if err != nil {
		t.Fatalf("AppendTelemetry: %v", err)
	}
	// Expect only the 4 good readings appended (water_temp, ph, orp, salt
	// from BoW 11), not the 5 sentinels from air + BoW 10.
	if count != 4 {
		t.Errorf("expected 4 good samples appended, got %d", count)
	}

	// Confirm the -1 readings never made it into the store.
	for _, metric := range []string{"air_temp", "water_temp", "ph", "orp", "salt_ppm"} {
		samples, _ := s.QueryTelemetry(1, metric, "", 0)
		for _, smp := range samples {
			v := smp.FormatValue()
			if v == "-1" || v == "-1.0" {
				t.Errorf("metric %q still has a -1 sentinel in the store: %+v", metric, smp)
			}
		}
	}
}
