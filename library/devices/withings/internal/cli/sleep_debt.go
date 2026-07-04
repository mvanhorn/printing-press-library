// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `sleep debt` — cumulative sleep deficit vs target over a rolling window.
// pp:data-source local
//
// Hand-authored implementation (RunE body replaced from the generated stub).
// Registered under the `sleep` parent in sleep.go.

package cli

import (
	"encoding/json"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// sleepDebtResult is the JSON shape emitted by `sleep debt`.
type sleepDebtResult struct {
	Window         string  `json:"window"`
	Nights         int     `json:"nights"`
	TargetHours    float64 `json:"target_hours"`
	AvgSleepHours  float64 `json:"avg_sleep_hours"`
	CumulativeDebt float64 `json:"cumulative_debt_hours"`
}

func newNovelSleepDebtCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagTarget string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "debt",
		Short:       "Cumulative sleep deficit against your target over a rolling window",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Sums (target - actual) sleep per night over a rolling window from the local
sleep-summary mirror and reports the cumulative deficit — the number the
per-night summary never adds up for you.

Reads local data only. Sync first with:
  withings-pp-cli sync --resources sleep`,
		Example: "  withings-pp-cli sleep debt\n" +
			"  withings-pp-cli sleep debt --window 2w --target 7h30m\n" +
			"  withings-pp-cli sleep debt --window 30d --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			window, err := parseSinceFlag(flagWindow, 14*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			target := 8 * time.Hour
			if flagTarget != "" {
				t, terr := cliutil.ParseDurationLoose(flagTarget)
				if terr != nil {
					return usageErr(terr)
				}
				if t > 0 {
					target = t
				}
			}
			cutoff := time.Now().Add(-window)

			db, handled, err := openLocalForAnalytics(cmd, flags, dbPath, "sleep", false)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			res, err := computeSleepDebt(db, cutoff, target, flagSinceLabel(flagWindow, "14d"))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "14d", "Rolling window (e.g. 14d, 2w)")
	cmd.Flags().StringVar(&flagTarget, "target", "8h", "Nightly sleep target (e.g. 8h, 7h30m)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	return cmd
}

// computeSleepDebt sums per-night deficits against target over the window.
// Nights without a total_sleep_time are excluded from the denominator.
func computeSleepDebt(db *store.Store, cutoff time.Time, target time.Duration, windowLabel string) (sleepDebtResult, error) {
	targetHours := target.Hours()
	res := sleepDebtResult{Window: windowLabel, TargetHours: roundN(targetHours, 2)}

	rows, err := localRows(db, "sleep")
	if err != nil {
		return res, err
	}
	cutYMD := cutoff.UTC().Format("2006-01-02")

	var totalSleepHours, totalDebt float64
	nights := 0
	for _, raw := range rows {
		var s struct {
			Date      string `json:"date"`
			StartDate int64  `json:"startdate"`
			Data      struct {
				TotalSleepTime float64 `json:"total_sleep_time"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		day := s.Date
		if day == "" {
			day = epochToYMD(s.StartDate)
		}
		if day == "" || day < cutYMD {
			continue
		}
		if s.Data.TotalSleepTime <= 0 {
			continue
		}
		sleepHours := s.Data.TotalSleepTime / 3600.0
		totalSleepHours += sleepHours
		totalDebt += targetHours - sleepHours
		nights++
	}

	res.Nights = nights
	if nights > 0 {
		res.AvgSleepHours = roundN(totalSleepHours/float64(nights), 2)
	}
	res.CumulativeDebt = roundN(totalDebt, 2)
	return res, nil
}
