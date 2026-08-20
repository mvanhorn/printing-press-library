// tesla timeline — stitched drives + charges from local vehicle_states polls.
// Hand-coded; out-of-tree from generator.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

const teslaNominalBatteryKwh = 75.0 // ballpark; real value depends on model + battery age

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var (
		vin      string
		since    string
		typ      string
		saveOnly bool
	)
	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Stitched drives + charges over the local vehicle_states history",
		Long: `Walks tesla_vehicle_states rows in chronological order and reconstructs
discrete drives (shift_state in D/R/N bracketed by P) and charges
(charging_state == Charging bracketed by other states). Output is sorted
chronologically with type=drive | type=charge entries.`,
		Example:     "  tesla-pp-cli timeline --since 7d --json\n  tesla-pp-cli timeline --vin 5YJ3E1EA6XXXXXXXX --type charges --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			ctx := cmd.Context()
			sinceT, err := parseSinceDuration(since)
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}
			drives, charges, err := stitchTimeline(ctx, s, vin, sinceT)
			if err != nil {
				return err
			}
			// Persist stitched rows
			if err := saveTimeline(ctx, s, drives, charges); err != nil {
				return fmt.Errorf("save: %w", err)
			}
			if saveOnly {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"drives_saved":  len(drives),
					"charges_saved": len(charges),
				}, flags)
			}
			entries := combineTimeline(drives, charges, typ)
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
	cmd.Flags().StringVar(&vin, "vin", "", "Filter to a single VIN (default: all)")
	cmd.Flags().StringVar(&since, "since", "7d", "Window start")
	cmd.Flags().StringVar(&typ, "type", "all", "Filter: all | drives | charges")
	cmd.Flags().BoolVar(&saveOnly, "save-only", false, "Write to store but don't emit JSON")
	return cmd
}

type driveRow struct {
	Type              string  `json:"type"`
	VIN               string  `json:"vin"`
	StartedAt         string  `json:"started_at"`
	EndedAt           string  `json:"ended_at"`
	DistanceMi        float64 `json:"distance_mi,omitempty"`
	StartBatteryLevel int     `json:"start_battery_level"`
	EndBatteryLevel   int     `json:"end_battery_level"`
	EnergyUsedKwh     float64 `json:"energy_used_kwh,omitempty"`
	EfficiencyWhPerMi float64 `json:"efficiency_wh_per_mi,omitempty"`
	StartLat          float64 `json:"start_lat,omitempty"`
	StartLng          float64 `json:"start_lng,omitempty"`
	EndLat            float64 `json:"end_lat,omitempty"`
	EndLng            float64 `json:"end_lng,omitempty"`
}

type chargeRow struct {
	Type              string  `json:"type"`
	VIN               string  `json:"vin"`
	StartedAt         string  `json:"started_at"`
	EndedAt           string  `json:"ended_at"`
	FastChargerType   string  `json:"fast_charger_type,omitempty"`
	EnergyAddedKwh    float64 `json:"energy_added_kwh,omitempty"`
	StartBatteryLevel int     `json:"start_battery_level"`
	EndBatteryLevel   int     `json:"end_battery_level"`
	LocationLat       float64 `json:"location_lat,omitempty"`
	LocationLng       float64 `json:"location_lng,omitempty"`
}

type stateRow struct {
	VIN             string
	CapturedAt      time.Time
	ShiftState      string
	ChargingState   string
	BatteryLevel    int
	Latitude        float64
	Longitude       float64
	FastChargerType string
	OdometerMi      float64
}

func stitchTimeline(ctx context.Context, s *store.Store, vin string, since time.Time) ([]driveRow, []chargeRow, error) {
	q := `SELECT vin, captured_at, shift_state, charging_state, battery_level,
                  latitude, longitude, fast_charger_type, odometer_mi
           FROM tesla_vehicle_states WHERE captured_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339)}
	if vin != "" {
		q += " AND vin = ?"
		args = append(args, vin)
	}
	q += " ORDER BY vin, captured_at ASC"
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byVIN := map[string][]stateRow{}
	for rows.Next() {
		var (
			r                    stateRow
			cap                  string
			shift, chgState, fct sql.NullString
			bl                   sql.NullInt64
			lat, lng, odo        sql.NullFloat64
		)
		if err := rows.Scan(&r.VIN, &cap, &shift, &chgState, &bl, &lat, &lng, &fct, &odo); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, cap)
		r.CapturedAt = t
		r.ShiftState = shift.String
		r.ChargingState = chgState.String
		r.BatteryLevel = int(bl.Int64)
		r.Latitude = lat.Float64
		r.Longitude = lng.Float64
		r.FastChargerType = fct.String
		r.OdometerMi = odo.Float64
		byVIN[r.VIN] = append(byVIN[r.VIN], r)
	}
	var drives []driveRow
	var charges []chargeRow
	for v, states := range byVIN {
		d, c := stitchVehicleStates(v, states)
		drives = append(drives, d...)
		charges = append(charges, c...)
	}
	return drives, charges, nil
}

func stitchVehicleStates(vin string, states []stateRow) ([]driveRow, []chargeRow) {
	var drives []driveRow
	var charges []chargeRow
	var curDrive *driveRow
	var curCharge *chargeRow
	for _, s := range states {
		driving := s.ShiftState == "D" || s.ShiftState == "R" || s.ShiftState == "N"
		charging := s.ChargingState == "Charging"
		if driving {
			if curDrive == nil {
				curDrive = &driveRow{
					Type: "drive", VIN: vin,
					StartedAt:         s.CapturedAt.UTC().Format(time.RFC3339),
					StartBatteryLevel: s.BatteryLevel,
					StartLat:          s.Latitude, StartLng: s.Longitude,
				}
			}
			curDrive.EndedAt = s.CapturedAt.UTC().Format(time.RFC3339)
			curDrive.EndBatteryLevel = s.BatteryLevel
			curDrive.EndLat = s.Latitude
			curDrive.EndLng = s.Longitude
		} else if curDrive != nil {
			delta := float64(curDrive.StartBatteryLevel - curDrive.EndBatteryLevel)
			if delta > 0 {
				curDrive.EnergyUsedKwh = (delta / 100.0) * teslaNominalBatteryKwh
			}
			drives = append(drives, *curDrive)
			curDrive = nil
		}
		if charging {
			if curCharge == nil {
				curCharge = &chargeRow{
					Type: "charge", VIN: vin,
					StartedAt:         s.CapturedAt.UTC().Format(time.RFC3339),
					StartBatteryLevel: s.BatteryLevel,
					FastChargerType:   s.FastChargerType,
					LocationLat:       s.Latitude, LocationLng: s.Longitude,
				}
			}
			curCharge.EndedAt = s.CapturedAt.UTC().Format(time.RFC3339)
			curCharge.EndBatteryLevel = s.BatteryLevel
		} else if curCharge != nil {
			delta := float64(curCharge.EndBatteryLevel - curCharge.StartBatteryLevel)
			if delta > 0 {
				curCharge.EnergyAddedKwh = (delta / 100.0) * teslaNominalBatteryKwh
			}
			charges = append(charges, *curCharge)
			curCharge = nil
		}
	}
	if curDrive != nil {
		drives = append(drives, *curDrive)
	}
	if curCharge != nil {
		charges = append(charges, *curCharge)
	}
	return drives, charges
}

func saveTimeline(ctx context.Context, s *store.Store, drives []driveRow, charges []chargeRow) error {
	for _, d := range drives {
		_, _ = s.DB().ExecContext(ctx, `INSERT OR REPLACE INTO tesla_drives (
            vin, started_at, ended_at, start_lat, start_lng, end_lat, end_lng,
            start_battery_level, end_battery_level, energy_used_kwh
          ) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			d.VIN, d.StartedAt, d.EndedAt, d.StartLat, d.StartLng, d.EndLat, d.EndLng,
			d.StartBatteryLevel, d.EndBatteryLevel, d.EnergyUsedKwh)
	}
	for _, c := range charges {
		_, _ = s.DB().ExecContext(ctx, `INSERT OR REPLACE INTO tesla_charges (
            vin, started_at, ended_at, fast_charger_type, location_lat, location_lng,
            energy_added_kwh, start_battery_level, end_battery_level
          ) VALUES (?,?,?,?,?,?,?,?,?)`,
			c.VIN, c.StartedAt, c.EndedAt, c.FastChargerType, c.LocationLat, c.LocationLng,
			c.EnergyAddedKwh, c.StartBatteryLevel, c.EndBatteryLevel)
	}
	return nil
}

func combineTimeline(drives []driveRow, charges []chargeRow, typ string) []json.RawMessage {
	out := []json.RawMessage{}
	type item struct {
		t   string
		raw json.RawMessage
	}
	var all []item
	if typ == "all" || typ == "drives" {
		for _, d := range drives {
			b, _ := json.Marshal(d)
			all = append(all, item{t: d.StartedAt, raw: b})
		}
	}
	if typ == "all" || typ == "charges" {
		for _, c := range charges {
			b, _ := json.Marshal(c)
			all = append(all, item{t: c.StartedAt, raw: b})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].t < all[j].t })
	for _, it := range all {
		out = append(out, it.raw)
	}
	return out
}
