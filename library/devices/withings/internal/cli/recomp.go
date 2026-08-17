// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `recomp` — body-recomposition verdict from the local measure mirror.
// pp:data-source local
//
// Hand-authored implementation (RunE body replaced from the generated stub,
// which only returned a TODO error). The constructor name and registration in
// root.go are unchanged.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/cliutil"
	"github.com/spf13/cobra"
)

// recompResult is the JSON shape emitted by `recomp`.
type recompResult struct {
	Since          string  `json:"since"`
	Samples        int     `json:"samples"`
	WeightStart    float64 `json:"weight_start"`
	WeightEnd      float64 `json:"weight_end"`
	WeightChange   float64 `json:"weight_change"`
	WeightRollAvg  float64 `json:"weight_rolling_avg"`
	FatMassChange  float64 `json:"fat_mass_change"`
	LeanMassChange float64 `json:"lean_mass_change"`
	Verdict        string  `json:"verdict"`
}

func newNovelRecompCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "recomp",
		Short:       "See whether you're actually recomposing — fat mass down while lean mass holds — on a de-noised rolling-average weight",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Reads the local measure mirror and reports whether body composition is
trending toward recomposition: fat mass down while lean (fat-free) mass holds
or rises. Weight is reported as a 7-day rolling average so day-to-day scale
noise does not drive the verdict.

Reads local data only. Sync first with:
  withings-pp-cli sync --resources measure`,
		Example: "  withings-pp-cli recomp\n" +
			"  withings-pp-cli recomp --since 12w\n" +
			"  withings-pp-cli recomp --since 180d --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseSinceFlag(flagSince, 90*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-since)

			db, handled, err := openLocalForAnalytics(cmd, flags, dbPath, "measure", false)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			groups, err := loadMeasureGroups(db, cutoff)
			if err != nil {
				return err
			}

			res := computeRecomp(groups, flagSinceLabel(flagSince, "90d"))
			return flags.printJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Lookback window (e.g. 30d, 12w, 720h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	return cmd
}

// computeRecomp derives the recomposition verdict from measure groups that are
// already filtered to the window and sorted ascending by date.
func computeRecomp(groups []measureGroup, sinceLabel string) recompResult {
	res := recompResult{Since: sinceLabel, Verdict: "insufficient data"}

	var weights, fatMasses, leanMasses []float64
	for _, g := range groups {
		if w, ok := g.scaledOfType(1); ok {
			weights = append(weights, w)
		}
		// fat_mass: prefer type 8 (fat mass weight); derive from weight*ratio
		// when only fat_ratio (type 6) is present.
		if fm, ok := g.scaledOfType(8); ok {
			fatMasses = append(fatMasses, fm)
		} else if fr, ok := g.scaledOfType(6); ok {
			if w, okw := g.scaledOfType(1); okw {
				fatMasses = append(fatMasses, w*fr/100.0)
			}
		}
		if lm, ok := g.scaledOfType(5); ok {
			leanMasses = append(leanMasses, lm)
		}
	}

	res.Samples = len(weights)
	if len(weights) > 0 {
		res.WeightStart = roundN(weights[0], 3)
		res.WeightEnd = roundN(weights[len(weights)-1], 3)
		res.WeightChange = roundN(weights[len(weights)-1]-weights[0], 3)
		res.WeightRollAvg = roundN(rollingAvgTail(weights, 7), 3)
	}

	var fatChange, leanChange float64
	haveFat := len(fatMasses) >= 2
	haveLean := len(leanMasses) >= 2
	if haveFat {
		fatChange = fatMasses[len(fatMasses)-1] - fatMasses[0]
		res.FatMassChange = roundN(fatChange, 3)
	}
	if haveLean {
		leanChange = leanMasses[len(leanMasses)-1] - leanMasses[0]
		res.LeanMassChange = roundN(leanChange, 3)
	}

	res.Verdict = recompVerdict(weights, fatMasses, leanMasses, fatChange, leanChange, haveFat, haveLean)
	return res
}

// recompVerdict classifies the trend. "recomposing" requires fat mass falling
// while lean mass is held or rising. Otherwise we fall back to a weight-trend
// verdict (losing/gaining), or "insufficient data" when there isn't enough.
func recompVerdict(weights, fatMasses, leanMasses []float64, fatChange, leanChange float64, haveFat, haveLean bool) string {
	// Tolerance: treat |change| <= holdTol kg as "held" rather than a real move.
	const holdTol = 0.2

	if haveFat && haveLean {
		fatDown := fatChange < -holdTol
		leanHeldOrUp := leanChange >= -holdTol
		leanDown := leanChange < -holdTol
		fatUp := fatChange > holdTol
		switch {
		case fatDown && leanHeldOrUp:
			return "recomposing"
		case fatDown && leanDown:
			return "losing"
		case fatUp && leanHeldOrUp:
			return "gaining"
		}
	}

	// Weight-only fallback.
	if len(weights) >= 2 {
		change := weights[len(weights)-1] - weights[0]
		switch {
		case change < -holdTol:
			return "losing"
		case change > holdTol:
			return "gaining"
		default:
			return "holding"
		}
	}
	return "insufficient data"
}

// rollingAvgTail returns the mean of the last `window` values of xs (or of all
// values when fewer than `window` are present). Returns 0 for an empty slice.
func rollingAvgTail(xs []float64, window int) float64 {
	if len(xs) == 0 {
		return 0
	}
	start := len(xs) - window
	if start < 0 {
		start = 0
	}
	var sum float64
	for _, v := range xs[start:] {
		sum += v
	}
	return sum / float64(len(xs)-start)
}

// parseSinceFlag parses a loose-duration --since value, applying def when the
// flag is empty.
func parseSinceFlag(value string, def time.Duration) (time.Duration, error) {
	if value == "" {
		return def, nil
	}
	d, err := cliutil.ParseDurationLoose(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", value, err)
	}
	if d <= 0 {
		return def, nil
	}
	return d, nil
}

// flagSinceLabel returns the user-supplied window label, or def when empty, for
// echoing back in output.
func flagSinceLabel(value, def string) string {
	if value == "" {
		return def
	}
	return value
}
