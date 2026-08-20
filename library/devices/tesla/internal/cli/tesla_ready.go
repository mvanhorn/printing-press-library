// tesla ready <vin> — composite "can I leave in 5 minutes?" yes/no.
// Hand-coded novel feature; lives outside the generator-emitted files so it
// survives `printing-press generate --force` regens.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

func newReadyCmd(flags *rootFlags) *cobra.Command {
	var (
		tripMi            float64
		readyThresholdPct int
		forceRefresh      bool
	)
	cmd := &cobra.Command{
		Use:   "ready [vehicle_id]",
		Short: "Composite \"can I leave in 5 minutes?\" yes/no with blockers list",
		Long: `Reads the most-recent vehicle_states snapshot (or refreshes live with --force-refresh)
and evaluates SOC vs trip distance, plugged-in, doors locked, sentry off, cabin
warmed-up, and OTA progress. Returns a single JSON object with a boolean ready
and a list of blockers and warnings.`,
		Example:     "  tesla-pp-cli ready 5YJ3E1EA6XXXXXXXX --json\n  tesla-pp-cli ready 5YJ3E1EA6XXXXXXXX --trip-mi 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return printReadyJSON(cmd, flags, &readyResult{Ready: true, VIN: args[0], VerifyNoop: true})
			}
			if dryRunOK(flags) {
				return printReadyJSON(cmd, flags, &readyResult{Ready: true, VIN: args[0], DryRun: true})
			}
			vin := args[0]
			ctx := cmd.Context()

			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}

			snap, snapErr := loadLatestVehicleSnapshot(ctx, s, vin)
			if forceRefresh || snap == nil || snapErr != nil {
				if refreshed, ferr := refreshAndStoreVehicleData(ctx, s, flags, vin); ferr == nil {
					snap = refreshed
				} else if snap == nil {
					return fmt.Errorf("no cached state and live refresh failed: %w", ferr)
				}
			}

			result := evaluateReady(snap, tripMi, readyThresholdPct)
			result.VIN = vin
			return printReadyJSON(cmd, flags, result)
		},
	}
	cmd.Flags().Float64Var(&tripMi, "trip-mi", 0, "Planned trip distance in miles (raises required SOC)")
	cmd.Flags().IntVar(&readyThresholdPct, "ready-threshold-pct", 50, "Minimum battery SOC required to be ready (default 50)")
	cmd.Flags().BoolVar(&forceRefresh, "force-refresh", false, "Skip cache; hit the live API to refresh state first")
	return cmd
}

type readyResult struct {
	Ready             bool           `json:"ready"`
	VIN               string         `json:"vin"`
	BatteryLevel      int            `json:"battery_level,omitempty"`
	BatteryRangeMi    float64        `json:"battery_range_mi,omitempty"`
	Blockers          []readyBlocker `json:"blockers"`
	Warnings          []readyBlocker `json:"warnings"`
	StateCapturedAt   string         `json:"state_captured_at,omitempty"`
	StaleSeconds      int64          `json:"stale_seconds,omitempty"`
	OnlineState       string         `json:"online_state,omitempty"`
	ChargingState     string         `json:"charging_state,omitempty"`
	InsideTempC       float64        `json:"inside_temp_c,omitempty"`
	DriverTargetTempC float64        `json:"driver_target_temp_c,omitempty"`
	DryRun            bool           `json:"dry_run,omitempty"`
	VerifyNoop        bool           `json:"verify_noop,omitempty"`
}

type readyBlocker struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

type vehicleStateSnapshot struct {
	VIN                  string
	CapturedAt           time.Time
	OnlineState          string
	ShiftState           string
	ChargingState        string
	BatteryLevel         int
	EstBatteryRangeMi    float64
	InsideTempC          float64
	OutsideTempC         float64
	DriverTempSettingC   float64
	Locked               bool
	SentryMode           bool
	SoftwareUpdateStatus string
	ChargeLimitSoc       int
}

func loadLatestVehicleSnapshot(ctx context.Context, s *store.Store, vin string) (*vehicleStateSnapshot, error) {
	row := s.DB().QueryRowContext(ctx, `
        SELECT vin, captured_at, online_state, shift_state, charging_state,
               battery_level, est_battery_range_mi,
               inside_temp_c, outside_temp_c, driver_temp_setting_c,
               locked, sentry_mode, software_update_status, charge_limit_soc
        FROM tesla_vehicle_states
        WHERE vin = ?
        ORDER BY captured_at DESC LIMIT 1`, vin)
	var (
		snap                             = &vehicleStateSnapshot{}
		capturedAt                       string
		online, shift, chg, sw           sql.NullString
		bl, climitSoc                    sql.NullInt64
		erng, insideT, outsideT, driverT sql.NullFloat64
		locked, sentry                   sql.NullBool
	)
	if err := row.Scan(&snap.VIN, &capturedAt, &online, &shift, &chg, &bl, &erng,
		&insideT, &outsideT, &driverT, &locked, &sentry, &sw, &climitSoc); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	snap.OnlineState = online.String
	snap.ShiftState = shift.String
	snap.ChargingState = chg.String
	snap.BatteryLevel = int(bl.Int64)
	snap.EstBatteryRangeMi = erng.Float64
	snap.InsideTempC = insideT.Float64
	snap.OutsideTempC = outsideT.Float64
	snap.DriverTempSettingC = driverT.Float64
	snap.Locked = locked.Bool
	snap.SentryMode = sentry.Bool
	snap.SoftwareUpdateStatus = sw.String
	snap.ChargeLimitSoc = int(climitSoc.Int64)
	if t, perr := time.Parse(time.RFC3339, capturedAt); perr == nil {
		snap.CapturedAt = t
	}
	return snap, nil
}

func refreshAndStoreVehicleData(ctx context.Context, s *store.Store, flags *rootFlags, vehicleID string) (*vehicleStateSnapshot, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := strings.ReplaceAll("/api/1/vehicles/{vehicle_id}/vehicle_data", "{vehicle_id}", vehicleID)
	raw, err := c.Get(path, nil)
	if err != nil {
		return nil, fmt.Errorf("vehicle_data: %w", err)
	}
	if err := store.InsertTeslaVehicleState(ctx, s, vehicleID, vehicleID, raw); err != nil {
		return nil, err
	}
	return loadLatestVehicleSnapshot(ctx, s, vehicleID)
}

func evaluateReady(snap *vehicleStateSnapshot, tripMi float64, threshold int) *readyResult {
	r := &readyResult{Blockers: []readyBlocker{}, Warnings: []readyBlocker{}}
	if snap == nil {
		r.Blockers = append(r.Blockers, readyBlocker{Name: "no_state", Detail: "no vehicle state available"})
		return r
	}
	r.BatteryLevel = snap.BatteryLevel
	r.BatteryRangeMi = snap.EstBatteryRangeMi
	r.OnlineState = snap.OnlineState
	r.ChargingState = snap.ChargingState
	r.InsideTempC = snap.InsideTempC
	r.DriverTargetTempC = snap.DriverTempSettingC
	if !snap.CapturedAt.IsZero() {
		r.StateCapturedAt = snap.CapturedAt.UTC().Format(time.RFC3339)
		r.StaleSeconds = int64(time.Since(snap.CapturedAt).Seconds())
	}

	if snap.BatteryLevel < 20 {
		r.Blockers = append(r.Blockers, readyBlocker{Name: "battery_critical", Detail: fmt.Sprintf("SOC %d%% below 20%% floor", snap.BatteryLevel)})
	} else if snap.BatteryLevel < threshold && snap.ChargingState != "Charging" {
		r.Blockers = append(r.Blockers, readyBlocker{Name: "battery_low", Detail: fmt.Sprintf("SOC %d%% below ready threshold %d%%", snap.BatteryLevel, threshold)})
	}
	if tripMi > 0 && snap.EstBatteryRangeMi > 0 && snap.EstBatteryRangeMi < tripMi*1.20 {
		r.Blockers = append(r.Blockers, readyBlocker{Name: "battery_below_trip", Detail: fmt.Sprintf("range %.0f mi < trip %.0f mi + 20%% buffer", snap.EstBatteryRangeMi, tripMi)})
	}
	if !snap.Locked {
		r.Warnings = append(r.Warnings, readyBlocker{Name: "doors_unlocked", Detail: "vehicle unlocked - lock before leaving"})
	}
	if snap.SentryMode {
		r.Warnings = append(r.Warnings, readyBlocker{Name: "sentry_on", Detail: "sentry mode active - drains battery"})
	}
	if snap.SoftwareUpdateStatus == "installing" || snap.SoftwareUpdateStatus == "downloading" {
		r.Blockers = append(r.Blockers, readyBlocker{Name: "ota_in_progress", Detail: fmt.Sprintf("software update status: %s", snap.SoftwareUpdateStatus)})
	}
	if snap.DriverTempSettingC > 0 && snap.InsideTempC > 0 {
		delta := snap.DriverTempSettingC - snap.InsideTempC
		if math.Abs(delta) > 3 {
			r.Warnings = append(r.Warnings, readyBlocker{Name: "cabin_not_warmed", Detail: fmt.Sprintf("inside %.0fC, target %.0fC", snap.InsideTempC, snap.DriverTempSettingC)})
		}
	}
	if snap.OnlineState == "offline" {
		r.Warnings = append(r.Warnings, readyBlocker{Name: "offline", Detail: "vehicle currently offline; cached state may be stale"})
	}
	r.Ready = len(r.Blockers) == 0
	return r
}

func printReadyJSON(cmd *cobra.Command, flags *rootFlags, r *readyResult) error {
	w := cmd.OutOrStdout()
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return printOutputWithFlags(w, json.RawMessage(raw), flags)
}
