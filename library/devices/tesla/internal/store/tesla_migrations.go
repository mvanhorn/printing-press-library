// Tesla-specific typed tables for novel-feature analytics.
// Lives outside the generator-emitted store.go so it survives --force regens.
// Lazy-init: novel commands call EnsureTeslaSchema at the top of RunE.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const teslaSchemaDDL = `
CREATE TABLE IF NOT EXISTS tesla_vehicle_states (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vin TEXT NOT NULL,
  vehicle_id TEXT NOT NULL,
  captured_at TIMESTAMP NOT NULL,
  online_state TEXT,
  shift_state TEXT,
  charging_state TEXT,
  battery_level INTEGER,
  est_battery_range_mi REAL,
  ideal_battery_range_mi REAL,
  inside_temp_c REAL,
  outside_temp_c REAL,
  driver_temp_setting_c REAL,
  locked BOOLEAN,
  sentry_mode BOOLEAN,
  charge_limit_soc INTEGER,
  charger_voltage INTEGER,
  charger_actual_current INTEGER,
  charger_phases INTEGER,
  charge_amps INTEGER,
  charger_power INTEGER,
  fast_charger_type TEXT,
  charger_session_kwh REAL,
  latitude REAL,
  longitude REAL,
  odometer_mi REAL,
  software_version TEXT,
  software_update_status TEXT,
  raw_json TEXT NOT NULL,
  UNIQUE(vin, captured_at)
);
CREATE INDEX IF NOT EXISTS tesla_vehicle_states_vin_ts ON tesla_vehicle_states(vin, captured_at);

CREATE TABLE IF NOT EXISTS tesla_drives (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vin TEXT NOT NULL,
  started_at TIMESTAMP NOT NULL,
  ended_at TIMESTAMP NOT NULL,
  start_lat REAL,
  start_lng REAL,
  end_lat REAL,
  end_lng REAL,
  distance_mi REAL,
  start_battery_level INTEGER,
  end_battery_level INTEGER,
  energy_used_kwh REAL,
  efficiency_wh_per_mi REAL,
  notes TEXT,
  UNIQUE(vin, started_at)
);
CREATE INDEX IF NOT EXISTS tesla_drives_vin_started ON tesla_drives(vin, started_at);

CREATE TABLE IF NOT EXISTS tesla_charges (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vin TEXT NOT NULL,
  started_at TIMESTAMP NOT NULL,
  ended_at TIMESTAMP NOT NULL,
  fast_charger_type TEXT,
  location_lat REAL,
  location_lng REAL,
  location_label TEXT,
  energy_added_kwh REAL,
  start_battery_level INTEGER,
  end_battery_level INTEGER,
  max_charger_power_kw REAL,
  cost_usd REAL,
  cost_per_kwh REAL,
  tariff_window TEXT,
  raw_json TEXT,
  UNIQUE(vin, started_at)
);
CREATE INDEX IF NOT EXISTS tesla_charges_vin_started ON tesla_charges(vin, started_at);

CREATE TABLE IF NOT EXISTS tesla_tariffs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  location_label TEXT NOT NULL,
  time_start TEXT NOT NULL,
  time_end TEXT NOT NULL,
  days_mask TEXT,
  window_label TEXT NOT NULL,
  cost_per_kwh REAL NOT NULL,
  source TEXT,
  UNIQUE(location_label, time_start, time_end, window_label)
);

CREATE TABLE IF NOT EXISTS tesla_keys_enrolled (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vin TEXT NOT NULL,
  pubkey_hash TEXT NOT NULL,
  role TEXT,
  form_factor TEXT,
  display_name TEXT,
  added_at TIMESTAMP,
  last_seen TIMESTAMP,
  UNIQUE(vin, pubkey_hash)
);

CREATE TABLE IF NOT EXISTS tesla_commands_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vin TEXT,
  command TEXT NOT NULL,
  args_json TEXT,
  ts TIMESTAMP NOT NULL,
  latency_ms INTEGER,
  http_status INTEGER,
  error TEXT,
  used_pubkey_hash TEXT
);
CREATE INDEX IF NOT EXISTS tesla_commands_log_vin_ts ON tesla_commands_log(vin, ts);
`

// EnsureTeslaSchema creates the Tesla-specific typed tables if missing.
// Safe to call from any novel command at RunE entry.
func EnsureTeslaSchema(ctx context.Context, s *Store) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("tesla schema: store not initialized")
	}
	_, err := s.db.ExecContext(ctx, teslaSchemaDDL)
	if err != nil {
		return fmt.Errorf("tesla schema: %w", err)
	}
	return nil
}

// InsertTeslaVehicleState upserts a snapshot from a vehicle_data response.
// Caller is responsible for parsing the raw JSON; this fn just records columns
// it can extract via json.Unmarshal into a permissive shape.
func InsertTeslaVehicleState(ctx context.Context, s *Store, vin, vehicleID string, raw json.RawMessage) error {
	if err := EnsureTeslaSchema(ctx, s); err != nil {
		return err
	}
	var v vehicleDataShape
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("parse vehicle_data: %w", err)
	}
	row := v.Response
	if row.VIN == "" {
		row.VIN = vin
	}
	if row.VehicleID == 0 {
		// vehicle_id may be string-encoded in some responses
	}
	now := nowUTC()
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO tesla_vehicle_states (
        vin, vehicle_id, captured_at, online_state, shift_state, charging_state,
        battery_level, est_battery_range_mi, ideal_battery_range_mi,
        inside_temp_c, outside_temp_c, driver_temp_setting_c,
        locked, sentry_mode, charge_limit_soc,
        charger_voltage, charger_actual_current, charger_phases, charge_amps, charger_power,
        fast_charger_type, charger_session_kwh,
        latitude, longitude, odometer_mi, software_version, software_update_status, raw_json
      ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		row.VIN, vehicleID, now,
		row.State, row.DriveState.ShiftState, row.ChargeState.ChargingState,
		row.ChargeState.BatteryLevel, row.ChargeState.EstBatteryRange, row.ChargeState.IdealBatteryRange,
		row.ClimateState.InsideTemp, row.ClimateState.OutsideTemp, row.ClimateState.DriverTempSetting,
		row.VehicleState.Locked, row.VehicleState.SentryMode, row.ChargeState.ChargeLimitSoc,
		row.ChargeState.ChargerVoltage, row.ChargeState.ChargerActualCurrent, row.ChargeState.ChargerPhases, row.ChargeState.ChargeAmps, row.ChargeState.ChargerPower,
		row.ChargeState.FastChargerType, row.ChargeState.ChargeEnergyAdded,
		row.DriveState.Latitude, row.DriveState.Longitude, row.VehicleState.Odometer,
		row.VehicleState.CarVersion, row.VehicleState.SoftwareUpdate.Status, string(raw),
	)
	return err
}

// permissive partial shape of the vehicle_data envelope. Missing fields scan as zero.
type vehicleDataShape struct {
	Response struct {
		VIN         string `json:"vin"`
		VehicleID   int64  `json:"vehicle_id"`
		State       string `json:"state"`
		ChargeState struct {
			BatteryLevel         int     `json:"battery_level"`
			ChargingState        string  `json:"charging_state"`
			ChargeLimitSoc       int     `json:"charge_limit_soc"`
			ChargeAmps           int     `json:"charge_amps"`
			ChargerVoltage       int     `json:"charger_voltage"`
			ChargerActualCurrent int     `json:"charger_actual_current"`
			ChargerPhases        int     `json:"charger_phases"`
			ChargerPower         int     `json:"charger_power"`
			EstBatteryRange      float64 `json:"est_battery_range"`
			IdealBatteryRange    float64 `json:"ideal_battery_range"`
			FastChargerType      string  `json:"fast_charger_type"`
			ChargeEnergyAdded    float64 `json:"charge_energy_added"`
		} `json:"charge_state"`
		ClimateState struct {
			InsideTemp        float64 `json:"inside_temp"`
			OutsideTemp       float64 `json:"outside_temp"`
			DriverTempSetting float64 `json:"driver_temp_setting"`
		} `json:"climate_state"`
		DriveState struct {
			Latitude   float64 `json:"latitude"`
			Longitude  float64 `json:"longitude"`
			ShiftState string  `json:"shift_state"`
		} `json:"drive_state"`
		VehicleState struct {
			Locked         bool    `json:"locked"`
			SentryMode     bool    `json:"sentry_mode"`
			Odometer       float64 `json:"odometer"`
			CarVersion     string  `json:"car_version"`
			SoftwareUpdate struct {
				Status string `json:"status"`
			} `json:"software_update"`
		} `json:"vehicle_state"`
	} `json:"response"`
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
