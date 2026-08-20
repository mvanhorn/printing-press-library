// Table-driven tests for the pure-logic helpers behind the Tesla novel features.
package cli

import (
	"testing"
	"time"
)

func TestEvaluateReady(t *testing.T) {
	cases := []struct {
		name      string
		snap      *vehicleStateSnapshot
		tripMi    float64
		threshold int
		wantReady bool
		wantBlk   int
		wantWarn  int
	}{
		{
			name:      "nil snapshot is not ready",
			snap:      nil,
			wantReady: false,
			wantBlk:   1,
		},
		{
			name: "happy path: charged, locked, no sentry",
			snap: &vehicleStateSnapshot{
				BatteryLevel:       80,
				EstBatteryRangeMi:  220,
				ChargingState:      "Disconnected",
				Locked:             true,
				SentryMode:         false,
				DriverTempSettingC: 22,
				InsideTempC:        21,
				OnlineState:        "online",
			},
			threshold: 50,
			wantReady: true,
		},
		{
			name: "battery critical below 20",
			snap: &vehicleStateSnapshot{
				BatteryLevel: 15,
				Locked:       true,
			},
			threshold: 50,
			wantReady: false,
			wantBlk:   1,
		},
		{
			name: "battery low + not charging blocks ready",
			snap: &vehicleStateSnapshot{
				BatteryLevel:  40,
				ChargingState: "Disconnected",
				Locked:        true,
			},
			threshold: 50,
			wantReady: false,
			wantBlk:   1,
		},
		{
			name: "battery low but charging is OK",
			snap: &vehicleStateSnapshot{
				BatteryLevel:  40,
				ChargingState: "Charging",
				Locked:        true,
			},
			threshold: 50,
			wantReady: true,
		},
		{
			name: "OTA installing blocks ready",
			snap: &vehicleStateSnapshot{
				BatteryLevel:         80,
				Locked:               true,
				SoftwareUpdateStatus: "installing",
			},
			threshold: 50,
			wantReady: false,
			wantBlk:   1,
		},
		{
			name: "trip too long for range",
			snap: &vehicleStateSnapshot{
				BatteryLevel:      80,
				EstBatteryRangeMi: 100,
				Locked:            true,
			},
			tripMi:    150,
			threshold: 50,
			wantReady: false,
			wantBlk:   1,
		},
		{
			name: "unlocked car is warning not blocker",
			snap: &vehicleStateSnapshot{
				BatteryLevel: 80,
				Locked:       false,
			},
			threshold: 50,
			wantReady: true,
			wantWarn:  1,
		},
		{
			name: "sentry on is warning not blocker",
			snap: &vehicleStateSnapshot{
				BatteryLevel: 80,
				Locked:       true,
				SentryMode:   true,
			},
			threshold: 50,
			wantReady: true,
			wantWarn:  1,
		},
		{
			name: "cabin temp delta > 3C is warning",
			snap: &vehicleStateSnapshot{
				BatteryLevel:       80,
				Locked:             true,
				DriverTempSettingC: 22,
				InsideTempC:        10,
			},
			threshold: 50,
			wantReady: true,
			wantWarn:  1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := evaluateReady(tc.snap, tc.tripMi, tc.threshold)
			if r.Ready != tc.wantReady {
				t.Errorf("ready: got %v want %v (blockers=%v warnings=%v)", r.Ready, tc.wantReady, r.Blockers, r.Warnings)
			}
			if len(r.Blockers) != tc.wantBlk {
				t.Errorf("blockers count: got %d want %d (%v)", len(r.Blockers), tc.wantBlk, r.Blockers)
			}
			if len(r.Warnings) != tc.wantWarn {
				t.Errorf("warnings count: got %d want %d (%v)", len(r.Warnings), tc.wantWarn, r.Warnings)
			}
		})
	}
}

func TestStitchVehicleStates(t *testing.T) {
	t0 := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	states := []stateRow{
		{VIN: "5Y", CapturedAt: t0, ShiftState: "P", ChargingState: "Disconnected", BatteryLevel: 80},
		{VIN: "5Y", CapturedAt: t0.Add(5 * time.Minute), ShiftState: "D", ChargingState: "Disconnected", BatteryLevel: 79, Latitude: 47.6, Longitude: -122.3},
		{VIN: "5Y", CapturedAt: t0.Add(20 * time.Minute), ShiftState: "D", ChargingState: "Disconnected", BatteryLevel: 75, Latitude: 47.7, Longitude: -122.4},
		{VIN: "5Y", CapturedAt: t0.Add(30 * time.Minute), ShiftState: "P", ChargingState: "Disconnected", BatteryLevel: 74, Latitude: 47.7, Longitude: -122.4},
		{VIN: "5Y", CapturedAt: t0.Add(35 * time.Minute), ShiftState: "P", ChargingState: "Charging", BatteryLevel: 74, FastChargerType: "Tesla"},
		{VIN: "5Y", CapturedAt: t0.Add(60 * time.Minute), ShiftState: "P", ChargingState: "Charging", BatteryLevel: 85, FastChargerType: "Tesla"},
		{VIN: "5Y", CapturedAt: t0.Add(70 * time.Minute), ShiftState: "P", ChargingState: "Complete", BatteryLevel: 85},
	}
	drives, charges := stitchVehicleStates("5Y", states)
	if len(drives) != 1 {
		t.Fatalf("drives: got %d want 1; %+v", len(drives), drives)
	}
	if drives[0].StartBatteryLevel != 79 || drives[0].EndBatteryLevel != 75 {
		t.Errorf("drive battery: got %d->%d want 79->75 (end is last D-row, not the trailing P-row)", drives[0].StartBatteryLevel, drives[0].EndBatteryLevel)
	}
	if drives[0].EnergyUsedKwh <= 0 {
		t.Errorf("drive energy_used_kwh should be > 0, got %f", drives[0].EnergyUsedKwh)
	}
	if len(charges) != 1 {
		t.Fatalf("charges: got %d want 1; %+v", len(charges), charges)
	}
	if charges[0].FastChargerType != "Tesla" {
		t.Errorf("charge fast_charger_type: got %q want Tesla", charges[0].FastChargerType)
	}
	if charges[0].EnergyAddedKwh <= 0 {
		t.Errorf("charge energy_added_kwh should be > 0, got %f", charges[0].EnergyAddedKwh)
	}
}

func TestParseDurationDayAware(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"90d", 90 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"6mo", 6 * 30 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"30m", 30 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseDurationDayAware(c.in)
			if err != nil {
				t.Fatalf("parse %q: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("parse %q: got %v want %v", c.in, got, c.want)
			}
		})
	}
}

func TestContainsSignedCmdSignal(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`{"response":{"result":true}}`, false},
		{`{"error":"vehicle_command_protocol_required"}`, true},
		{`{"error":"command authentication is required for this vehicle"}`, true},
		{`{"signed_command_enforced":true}`, true},
		{`{"response":{"battery_level":80}}`, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := containsSignedCmdSignal([]byte(c.in))
			if got != c.want {
				t.Errorf("got %v want %v for %q", got, c.want, c.in)
			}
		})
	}
}
