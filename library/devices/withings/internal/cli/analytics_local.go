// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the local-only analytics commands (recomp, recovery,
// bp-report, sleep debt, digest, correlate). Every analytics command reads
// ONLY the local SQLite mirror — never the live API — so these helpers parse
// the stored JSON bodies (the full nested objects live in resources.data) and
// compute the daily series / group rollups the commands share.
//
// Hand-authored (no "DO NOT EDIT" header): new code for the analytics layer.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// ---------- DB open + missing-mirror handling ----------

// analyticsDBPath resolves the --db flag value to a concrete path, falling
// back to the canonical default when empty.
func analyticsDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("withings-pp-cli")
}

// openLocalForAnalytics opens the local store read-only when the mirror file
// exists. When it does NOT exist it prints the standard "no local mirror"
// hint to stderr, emits an empty machine payload to stdout (`[]` for a list
// shape, `{}` for an object shape) when --json is set, and returns
// handled=true so the caller returns nil. `resources` is the comma-separated
// resource list to suggest in the sync hint (e.g. "measure" or
// "measure,activity,sleep").
func openLocalForAnalytics(cmd *cobra.Command, flags *rootFlags, dbPath, resources string, emptyIsList bool) (db *store.Store, handled bool, err error) {
	path := analyticsDBPath(dbPath)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		fmt.Fprintf(os.Stderr, "no local mirror at %s\nrun: withings-pp-cli sync --resources %s --db %s\n", path, resources, path)
		if flags != nil && flags.asJSON {
			if emptyIsList {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "{}")
			}
		}
		return nil, true, nil
	}
	s, openErr := store.OpenReadOnly(path)
	if openErr != nil {
		return nil, false, fmt.Errorf("opening local mirror %s: %w", path, openErr)
	}
	return s, false, nil
}

// openLocalForAnalyticsRW is the read-write counterpart of
// openLocalForAnalytics, used by commands that also persist auxiliary local
// state (e.g. bp-report's annotation table). Same missing-mirror contract:
// when the mirror file is absent it prints the sync hint, emits the empty
// machine payload, and returns handled=true.
func openLocalForAnalyticsRW(cmd *cobra.Command, flags *rootFlags, dbPath, resources string, emptyIsList bool) (db *store.Store, handled bool, err error) {
	path := analyticsDBPath(dbPath)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		fmt.Fprintf(os.Stderr, "no local mirror at %s\nrun: withings-pp-cli sync --resources %s --db %s\n", path, resources, path)
		if flags != nil && flags.asJSON {
			if emptyIsList {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "{}")
			}
		}
		return nil, true, nil
	}
	s, openErr := store.OpenWithContext(cmd.Context(), path)
	if openErr != nil {
		return nil, false, fmt.Errorf("opening local mirror %s: %w", path, openErr)
	}
	return s, false, nil
}

// ---------- Measure parsing ----------

// measureValue is one entry in a measure group's measures[] array.
type measureValue struct {
	Value int `json:"value"`
	Type  int `json:"type"`
	Unit  int `json:"unit"`
}

// measureGroup is the parsed shape of a single Withings measure group as stored
// in resources.data for resource_type "measure".
type measureGroup struct {
	GrpID    int64          `json:"grpid"`
	Category int            `json:"category"`
	Date     int64          `json:"date"`
	Measures []measureValue `json:"measures"`
}

// scaledOfType returns the scaled real value of the first measure of the given
// type code in the group, and whether such a measure was present.
func (g measureGroup) scaledOfType(typeCode int) (float64, bool) {
	for _, m := range g.Measures {
		if m.Type == typeCode {
			return scaleMeasure(m.Value, m.Unit), true
		}
	}
	return 0, false
}

// loadMeasureGroups reads measure rows from the local store whose group date is
// at or after `since`, parses each, and returns them sorted ascending by date.
// category 1 (real measures) only — category 2 rows are user objectives, not
// observations. A zero `since` returns all groups.
func loadMeasureGroups(db *store.Store, since time.Time) ([]measureGroup, error) {
	rows, err := db.List("measure", 0)
	if err != nil {
		return nil, fmt.Errorf("listing measure rows: %w", err)
	}
	cutoff := since.Unix()
	out := make([]measureGroup, 0, len(rows))
	for _, raw := range rows {
		var g measureGroup
		if err := json.Unmarshal(raw, &g); err != nil {
			continue
		}
		if g.Category != 0 && g.Category != 1 {
			continue
		}
		if !since.IsZero() && g.Date < cutoff {
			continue
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// ---------- Generic local row loading ----------

// localRows returns the raw `data` JSON for every row of resourceType in the
// store. Thin wrapper over store.List(_, 0) kept here so the analytics code
// reads from one place.
func localRows(db *store.Store, resourceType string) ([]json.RawMessage, error) {
	return db.List(resourceType, 0)
}

// ---------- Date helpers ----------

// epochToYMD renders a unix epoch (seconds) as a UTC YYYY-MM-DD day key.
func epochToYMD(epoch int64) string {
	if epoch <= 0 {
		return ""
	}
	return time.Unix(epoch, 0).UTC().Format("2006-01-02")
}

// parseYMD parses a YYYY-MM-DD day key to a time at UTC midnight; ok=false on
// malformed input.
func parseYMD(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ---------- Statistics ----------

// pearson returns the Pearson correlation coefficient of paired samples xs, ys.
// ok=false when fewer than 2 pairs or when either series has zero variance
// (correlation is undefined). xs and ys must be the same length.
func pearson(xs, ys []float64) (r float64, ok bool) {
	n := len(xs)
	if n != len(ys) || n < 2 {
		return 0, false
	}
	var sx, sy float64
	for i := 0; i < n; i++ {
		sx += xs[i]
		sy += ys[i]
	}
	mx := sx / float64(n)
	my := sy / float64(n)
	var cov, vx, vy float64
	for i := 0; i < n; i++ {
		dx := xs[i] - mx
		dy := ys[i] - my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx == 0 || vy == 0 {
		return 0, false
	}
	return cov / math.Sqrt(vx*vy), true
}

// roundN rounds x to prec decimal places. Used to keep JSON output stable and
// readable rather than emitting full float64 noise.
func roundN(x float64, prec int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	p := math.Pow10(prec)
	return math.Round(x*p) / p
}

// ---------- Daily metric series (for correlate) ----------

// correlateMetrics is the set of metric keys correlate accepts, in a stable
// order suitable for an error message.
var correlateMetrics = []string{
	"weight", "fat_ratio", "steps", "calories",
	"sleep_score", "resting_hr", "systolic", "diastolic",
}

// isKnownMetric reports whether key is a supported correlate metric.
func isKnownMetric(key string) bool {
	for _, m := range correlateMetrics {
		if m == key {
			return true
		}
	}
	return false
}

// buildDailySeries constructs a day -> value map for a single metric key from
// the local store, considering only days at/after cutoff. The aggregation per
// day depends on the metric: measure-derived scalars take the last reading of
// the day, resting_hr takes the day minimum, and activity steps/calories sum
// over the day. Unknown keys return an empty map.
func buildDailySeries(db *store.Store, key string, cutoff time.Time) (map[string]float64, error) {
	cutYMD := cutoff.UTC().Format("2006-01-02")
	switch key {
	case "weight", "fat_ratio", "systolic", "diastolic", "resting_hr":
		return measureSeries(db, key, cutoff)
	case "steps", "calories":
		return activitySeries(db, key, cutYMD)
	case "sleep_score":
		return sleepScoreSeries(db, cutYMD)
	default:
		return map[string]float64{}, nil
	}
}

// measureSeries builds a daily series from measure groups for a measure-derived
// metric key.
func measureSeries(db *store.Store, key string, cutoff time.Time) (map[string]float64, error) {
	groups, err := loadMeasureGroups(db, cutoff)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, g := range groups {
		day := epochToYMD(g.Date)
		if day == "" {
			continue
		}
		var val float64
		var ok bool
		switch key {
		case "weight":
			val, ok = g.scaledOfType(1)
		case "fat_ratio":
			val, ok = g.scaledOfType(6)
		case "systolic":
			val, ok = g.scaledOfType(10)
		case "diastolic":
			val, ok = g.scaledOfType(9)
		case "resting_hr":
			val, ok = g.scaledOfType(11)
		}
		if !ok {
			continue
		}
		if key == "resting_hr" {
			if cur, exists := out[day]; !exists || val < cur {
				out[day] = val
			}
			continue
		}
		// groups are date-ascending, so the last write wins == latest of day.
		out[day] = val
	}
	return out, nil
}

// activitySeries builds a daily series summing steps or calories from activity
// rows.
func activitySeries(db *store.Store, key, cutYMD string) (map[string]float64, error) {
	rows, err := localRows(db, "activity")
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, raw := range rows {
		var a struct {
			Date     string  `json:"date"`
			Steps    float64 `json:"steps"`
			Calories float64 `json:"calories"`
		}
		if json.Unmarshal(raw, &a) != nil {
			continue
		}
		if a.Date == "" || a.Date < cutYMD {
			continue
		}
		switch key {
		case "steps":
			out[a.Date] += a.Steps
		case "calories":
			out[a.Date] += a.Calories
		}
	}
	return out, nil
}

// sleepScoreSeries builds a daily series of sleep scores from sleep summaries.
func sleepScoreSeries(db *store.Store, cutYMD string) (map[string]float64, error) {
	rows, err := localRows(db, "sleep")
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, raw := range rows {
		var s struct {
			Date      string `json:"date"`
			StartDate int64  `json:"startdate"`
			Data      struct {
				SleepScore float64 `json:"sleep_score"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		day := s.Date
		if day == "" {
			day = epochToYMD(s.StartDate)
		}
		if day == "" || day < cutYMD || s.Data.SleepScore <= 0 {
			continue
		}
		out[day] = s.Data.SleepScore
	}
	return out, nil
}

// matchedPairs returns the value pairs for days present in both series, where
// series b is shifted by lagDays relative to series a: for each day D in a, we
// look up day D+lagDays in b. Days are aligned on calendar dates.
func matchedPairs(a, b map[string]float64, lagDays int) (xs, ys []float64) {
	for day, av := range a {
		t, ok := parseYMD(day)
		if !ok {
			continue
		}
		shifted := t.AddDate(0, 0, lagDays).Format("2006-01-02")
		if bv, ok := b[shifted]; ok {
			xs = append(xs, av)
			ys = append(ys, bv)
		}
	}
	return xs, ys
}

// bestLagCorrelation scans integer lags in [minLag, maxLag], computes the
// Pearson r at each (over the days matched at that lag), and returns the lag
// whose |r| is largest along with that r and whether any lag produced a
// defined correlation.
//
// Ties on |r| (common when series move in perfect lockstep, where every lag
// that overlaps is also perfectly correlated) are broken in favor of (1) the
// lag with more matched day-pairs — a stronger, better-supported alignment —
// and then (2) the lag closest to zero, so a genuine zero-lag relationship is
// not spuriously reported as lagged.
func bestLagCorrelation(a, b map[string]float64, minLag, maxLag int) (bestLag int, bestR float64, ok bool) {
	const eps = 1e-9
	bestAbs := -1.0
	bestPairs := -1
	for lag := minLag; lag <= maxLag; lag++ {
		xs, ys := matchedPairs(a, b, lag)
		r, defined := pearson(xs, ys)
		if !defined {
			continue
		}
		abs := math.Abs(r)
		pairs := len(xs)
		better := false
		switch {
		case !ok:
			better = true
		case abs > bestAbs+eps:
			better = true
		case abs >= bestAbs-eps:
			// |r| tie: prefer more matched pairs, then the smaller |lag|.
			if pairs > bestPairs {
				better = true
			} else if pairs == bestPairs && abs2Int(lag) < abs2Int(bestLag) {
				better = true
			}
		}
		if better {
			bestAbs = abs
			bestR = r
			bestLag = lag
			bestPairs = pairs
			ok = true
		}
	}
	return bestLag, bestR, ok
}

// abs2Int returns the absolute value of an int (small local helper for the
// lag tie-break).
func abs2Int(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
