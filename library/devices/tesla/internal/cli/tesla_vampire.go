// tesla vampire — SOC delta over idle time, flags suspicious vampire drain.
// Hand-coded; out-of-tree from generator.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

func newVampireCmd(flags *rootFlags) *cobra.Command {
	var (
		vin       string
		threshold float64
		since     string
	)
	cmd := &cobra.Command{
		Use:   "vampire",
		Short: "Flag suspicious vampire drain (SOC dropping while parked + disconnected)",
		Long: `Walks tesla_vehicle_states for windows where the car was parked (shift_state NULL or P)
and unplugged (charging_state == Disconnected), then computes the SOC-loss rate
per 24h. Windows exceeding the threshold are flagged with a diagnosis (sentry,
cold weather, or unexplained).`,
		Example:     "  tesla-pp-cli vampire --threshold 1.5 --since 14d --json",
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
			report, err := computeVampire(ctx, s, vin, sinceT, threshold)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&vin, "vin", "", "Filter to a single VIN (default: all)")
	cmd.Flags().Float64Var(&threshold, "threshold", 1.5, "Flag windows draining > N pct per 24h")
	cmd.Flags().StringVar(&since, "since", "14d", "Window start")
	return cmd
}

type vampireWindow struct {
	VIN             string  `json:"vin"`
	StartedAt       string  `json:"started_at"`
	EndedAt         string  `json:"ended_at"`
	StartBattery    int     `json:"start_battery_level"`
	EndBattery      int     `json:"end_battery_level"`
	SOCDeltaPct     int     `json:"soc_delta_pct"`
	HoursIdle       float64 `json:"hours_idle"`
	RatePctPer24h   float64 `json:"rate_pct_per_24h"`
	SentryMode      bool    `json:"sentry_mode"`
	AvgOutsideTempC float64 `json:"avg_outside_temp_c,omitempty"`
	Diagnosis       string  `json:"diagnosis"`
}

type vampireReport struct {
	Threshold      float64         `json:"threshold_pct_per_24h"`
	WindowsFlagged int             `json:"windows_flagged"`
	WindowsScanned int             `json:"windows_scanned"`
	Note           string          `json:"note,omitempty"`
	Windows        []vampireWindow `json:"windows"`
}

func computeVampire(ctx context.Context, s *store.Store, vin string, since time.Time, threshold float64) (*vampireReport, error) {
	out := &vampireReport{Threshold: threshold, Windows: []vampireWindow{}}
	q := `SELECT vin, captured_at, shift_state, charging_state, battery_level,
                  sentry_mode, outside_temp_c
            FROM tesla_vehicle_states
            WHERE captured_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339)}
	if vin != "" {
		q += " AND vin = ?"
		args = append(args, vin)
	}
	q += " ORDER BY vin, captured_at ASC"
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	type sample struct {
		vin        string
		t          time.Time
		shift, chg sql.NullString
		bl         int
		sentry     bool
		outsideT   sql.NullFloat64
	}
	byVIN := map[string][]sample{}
	for rows.Next() {
		var (
			v, cap     string
			shift, chg sql.NullString
			bl         sql.NullInt64
			sentry     sql.NullBool
			outsideT   sql.NullFloat64
		)
		if err := rows.Scan(&v, &cap, &shift, &chg, &bl, &sentry, &outsideT); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, cap)
		byVIN[v] = append(byVIN[v], sample{
			vin: v, t: t,
			shift: shift, chg: chg, bl: int(bl.Int64),
			sentry: sentry.Bool, outsideT: outsideT,
		})
	}
	for _, samples := range byVIN {
		var start *sample
		var tempSum float64
		var tempN int
		var sentrySeen bool
		flush := func(end *sample) {
			if start == nil || end == nil || start == end {
				return
			}
			out.WindowsScanned++
			delta := start.bl - end.bl
			hours := end.t.Sub(start.t).Hours()
			if hours < 1 || delta <= 0 {
				return
			}
			rate := (float64(delta) / hours) * 24.0
			if rate < threshold {
				return
			}
			window := vampireWindow{
				VIN:           start.vin,
				StartedAt:     start.t.UTC().Format(time.RFC3339),
				EndedAt:       end.t.UTC().Format(time.RFC3339),
				StartBattery:  start.bl,
				EndBattery:    end.bl,
				SOCDeltaPct:   delta,
				HoursIdle:     hours,
				RatePctPer24h: rate,
				SentryMode:    sentrySeen,
			}
			if tempN > 0 {
				window.AvgOutsideTempC = tempSum / float64(tempN)
			}
			switch {
			case sentrySeen:
				window.Diagnosis = "sentry on"
			case window.AvgOutsideTempC < 0:
				window.Diagnosis = "cold weather"
			default:
				window.Diagnosis = "phantom drain"
			}
			out.Windows = append(out.Windows, window)
		}
		for i := range samples {
			ss := &samples[i]
			idle := (ss.shift.String == "" || ss.shift.String == "P") && ss.chg.String == "Disconnected"
			if idle {
				if start == nil {
					start = ss
					tempSum = 0
					tempN = 0
					sentrySeen = false
				}
				if ss.outsideT.Valid {
					tempSum += ss.outsideT.Float64
					tempN++
				}
				if ss.sentry {
					sentrySeen = true
				}
			} else if start != nil {
				flush(&samples[i-1])
				start = nil
			}
		}
		if start != nil && len(samples) > 0 {
			flush(&samples[len(samples)-1])
		}
	}
	out.WindowsFlagged = len(out.Windows)
	if out.WindowsScanned == 0 {
		out.Note = "no idle windows in range; sync more vehicle_states first"
	}
	_ = fmt.Sprintf // appease imports
	return out, nil
}
