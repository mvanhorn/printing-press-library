// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
)

const dayLayout = "2006-01-02"

// metricSpec maps a --metric flag value to the typed store column backing it.
// Only metrics with a dedicated typed table (created from explicit field
// definitions in the spec) are supported here — daily-resilience,
// daily-spo2, and daily-cardiovascular-age only ever landed in the generic
// `resources` table with no confirmed field schema, so they are deliberately
// left out rather than guessing at JSON shapes.
type metricSpec struct {
	table       string
	column      string
	description string
}

var metricSpecs = map[string]metricSpec{
	"sleep":     {table: "daily_sleep", column: "score", description: "daily sleep score (0-100)"},
	"readiness": {table: "daily_readiness", column: "score", description: "daily readiness score (0-100)"},
	"activity":  {table: "daily_activity", column: "score", description: "daily activity score (0-100)"},
	"stress":    {table: "daily_stress", column: "stress_high", description: "minutes of high stress that day"},
	"hrv":       {table: "sleep", column: "average_hrv", description: "average overnight HRV (ms)"},
}

func knownMetrics() []string {
	names := make([]string, 0, len(metricSpecs))
	for k := range metricSpecs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func resolveMetric(metric string) (metricSpec, error) {
	spec, ok := metricSpecs[metric]
	if !ok {
		return metricSpec{}, fmt.Errorf("unknown --metric %q; supported: %v", metric, knownMetrics())
	}
	return spec, nil
}

// resolveSinceDay turns a --since value into a start day string. Accepts a
// relative duration shorthand ("7d", "30d", "2w") or an absolute YYYY-MM-DD
// date. Empty input falls back to defaultDays days before today.
func resolveSinceDay(since string, defaultDays int) (string, error) {
	if since == "" {
		return time.Now().UTC().AddDate(0, 0, -defaultDays).Format(dayLayout), nil
	}
	if d, err := time.Parse(dayLayout, since); err == nil {
		return d.Format(dayLayout), nil
	}
	dur, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return "", fmt.Errorf("invalid --since %q: use a duration like 7d/30d or a date YYYY-MM-DD", since)
	}
	return time.Now().UTC().Add(-dur).Format(dayLayout), nil
}

func today() string { return time.Now().UTC().Format(dayLayout) }

func addDays(day string, n int) string {
	d, err := time.Parse(dayLayout, day)
	if err != nil {
		return day
	}
	return d.AddDate(0, 0, n).Format(dayLayout)
}

func daysBetween(start, end string) int {
	s, err1 := time.Parse(dayLayout, start)
	e, err2 := time.Parse(dayLayout, end)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(e.Sub(s).Hours() / 24)
}

func meanStdDev(vals []float64) (mean, stdDev float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	if len(vals) < 2 {
		return mean, 0
	}
	var sq float64
	for _, v := range vals {
		sq += (v - mean) * (v - mean)
	}
	stdDev = math.Sqrt(sq / float64(len(vals)-1))
	return mean, stdDev
}

// metricSeries returns day -> value for the given metric across
// [startDay, endDay] inclusive, read from the metric's typed table.
func metricSeries(db *store.Store, spec metricSpec, startDay, endDay string) (map[string]float64, error) {
	query := fmt.Sprintf(
		`SELECT day, %s FROM %s WHERE day >= ? AND day <= ? AND %s IS NOT NULL AND day IS NOT NULL`,
		spec.column, spec.table, spec.column,
	)
	rows, err := db.DB().Query(query, startDay, endDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var day sql.NullString
		var val sql.NullFloat64
		if err := rows.Scan(&day, &val); err != nil {
			continue
		}
		if day.Valid && val.Valid {
			result[day.String] = val.Float64
		}
	}
	return result, rows.Err()
}

func sortedDays(m map[string]float64) []string {
	days := make([]string, 0, len(m))
	for d := range m {
		days = append(days, d)
	}
	sort.Strings(days)
	return days
}

// missingMirrorMessage is the standard hint printed when the local SQLite
// mirror does not exist yet.
func missingMirrorMessage(dbPath string) string {
	return fmt.Sprintf("no local mirror at %s\nrun: oura-pp-cli sync\n", dbPath)
}
