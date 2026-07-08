// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

// Package health turns synced Google Health API data points into the
// quantified-self analytics the raw API does not provide: per-metric
// trend lines, goal streaks, and cross-metric correlation. It operates
// purely on synced JSON rows, so every function here is
// deterministic and offline.
//
// A Google Health DataPoint is a union: each point carries exactly one of
// ~50 metric fields (steps, weight, daily_resting_heart_rate, heart_rate,
// …), a single numeric magnitude inside that field, and a timestamp drawn
// from one of three shapes — an interval (interval.startTime), a sample
// (sampleTime.physicalTime), or a daily date ({year,month,day}). Extract
// normalizes those into a flat Point so the analytics never need to know
// the 50 metric schemas.
package health

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrInsufficientData signals that an analysis could not run because the
// local store does not yet hold enough overlapping observations — a soft
// "sync more first" condition, not a usage error. Callers use errors.Is
// to present it as an exit-0 hint rather than a failure.
var ErrInsufficientData = errors.New("insufficient overlapping data")

// Point is one extracted health observation: a numeric magnitude for a
// single metric at a single time. Day is the metric's calendar day
// (YYYY-MM-DD), used to align observations across metrics for streaks
// and correlation regardless of intraday timestamp differences.
type Point struct {
	Metric string    `json:"metric"`
	Value  float64   `json:"value"`
	Time   time.Time `json:"time"`
	Day    string    `json:"day"`
}

// metricValueKeys lists, per metric union field, the property that holds
// the metric's magnitude. Without this hint the generic numeric-leaf
// fallback could pick a metadata number (e.g. a confidence score) over
// the real value. Keys are the camelCase union field names exactly as
// they appear in a DataPoint.
var metricValueKeys = map[string]string{
	"steps":                  "count",
	"weight":                 "weightGrams",
	"distance":               "distanceMeters",
	"floors":                 "count",
	"activeMinutes":          "minutes",
	"activeZoneMinutes":      "minutes",
	"activeEnergyBurned":     "energyKilocalories",
	"basalEnergyBurned":      "energyKilocalories",
	"heartRate":              "beatsPerMinute",
	"dailyRestingHeartRate":  "beatsPerMinute",
	"heartRateVariability":   "heartRateVariabilityMilliseconds",
	"bloodGlucose":           "bloodGlucoseMgPerDl",
	"bodyFat":                "bodyFatPercentage",
	"height":                 "heightMeters",
	"coreBodyTemperature":    "temperatureCelsius",
	"dailyOxygenSaturation":  "averagePercentage",
	"dailyRespiratoryRate":   "averageBreathsPerMinute",
	"dailyVo2Max":            "vo2MaxKgMinMl",
	"altitude":               "altitudeMeters",
}

// timeWrapperKeys names the sub-objects that carry a point's timestamp,
// in priority order. interval/sampleTime are RFC3339 google-datetime
// strings; date is a {year,month,day} object handled separately.
var timeWrapperKeys = []string{"interval", "sampleTime", "date"}

// nonMetricKeys are DataPoint fields that are never the metric union
// member, so the union-field finder skips them.
var nonMetricKeys = map[string]bool{
	"name":       true,
	"dataSource": true,
	"createTime": true,
	"updateTime": true,
}

// ExtractPoints maps a batch of stored DataPoint JSON rows to Points,
// silently skipping any row that is not a recognizable health data point
// (profile rows, settings, malformed JSON). Order is not preserved; call
// a sort before presenting.
func ExtractPoints(rows [][]byte) []Point {
	points := make([]Point, 0, len(rows))
	for _, row := range rows {
		if p, ok := ExtractPoint(row); ok {
			points = append(points, p)
		}
	}
	return points
}

// ExtractPoint parses one stored DataPoint JSON object into a Point. It
// returns ok=false when the row has no numeric metric value or no
// resolvable timestamp — the signal callers use to filter non-point
// rows out of a mixed resources table.
func ExtractPoint(raw []byte) (Point, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return Point{}, false
	}
	field, body, ok := findMetricField(obj)
	if !ok {
		return Point{}, false
	}
	value, ok := metricValue(field, body)
	if !ok {
		return Point{}, false
	}
	ts, ok := metricTime(body)
	if !ok {
		return Point{}, false
	}
	metric := metricName(obj, field)
	return Point{
		Metric: metric,
		Value:  value,
		Time:   ts,
		Day:    ts.Format("2006-01-02"),
	}, true
}

// findMetricField returns the single populated union member of a
// DataPoint: the first object-valued field that isn't a known
// non-metric key. Returns ok=false when none is found (e.g. a profile or
// identity row that wandered into the scan).
func findMetricField(obj map[string]any) (field string, body map[string]any, ok bool) {
	// Iterate keys in sorted order so the chosen field is deterministic.
	// A well-formed DataPoint is a union with exactly one metric member, but
	// sorting guards against non-deterministic selection if a row ever
	// carries more than one object-valued field.
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if nonMetricKeys[key] {
			continue
		}
		if m, isObj := obj[key].(map[string]any); isObj {
			return key, m, true
		}
	}
	return "", nil, false
}

// metricValue pulls the numeric magnitude from a metric body, preferring
// the known value key for the field and falling back to the first numeric
// scalar leaf (skipping time wrappers and metadata sub-objects). Google
// encodes int64 values as JSON strings, so string-numbers are parsed too.
func metricValue(field string, body map[string]any) (float64, bool) {
	if key, known := metricValueKeys[field]; known {
		if v, ok := numeric(body[key]); ok {
			return v, true
		}
	}
	// Deterministic fallback: scan immediate scalar fields in key order.
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "interval" || k == "sampleTime" || k == "date" {
			continue
		}
		if v, ok := numeric(body[k]); ok {
			return v, true
		}
	}
	return 0, false
}

// numeric coerces a JSON value to float64, accepting native numbers and
// Google's string-encoded int64s. Booleans and non-numeric strings fail.
func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// metricTime resolves a point's timestamp from whichever time wrapper the
// metric uses: interval.startTime, sampleTime.physicalTime, or a daily
// date object. Returns ok=false when none parse.
func metricTime(body map[string]any) (time.Time, bool) {
	for _, wrapper := range timeWrapperKeys {
		sub, ok := body[wrapper].(map[string]any)
		if !ok {
			continue
		}
		switch wrapper {
		case "interval":
			if t, ok := parseRFC3339(sub["startTime"]); ok {
				return t, true
			}
		case "sampleTime":
			if t, ok := parseRFC3339(sub["physicalTime"]); ok {
				return t, true
			}
		case "date":
			if t, ok := parseDateObject(sub); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func parseRFC3339(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}, false
	}
	// Google Cloud APIs serialize google.protobuf.Timestamp with nanosecond
	// precision (e.g. "2026-06-01T00:00:00.000000000Z"), so RFC3339Nano must
	// be tried first; plain time.RFC3339 fails on any fractional-second value
	// and would silently drop most interval/sampleTime data points. The
	// RFC3339 fallback covers the rare whole-second timestamp.
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseDateObject builds a UTC midnight time from a Google Date
// {year,month,day}. A zero year/month/day means the field is not a usable
// calendar date, so it fails rather than producing a year-0 timestamp.
func parseDateObject(sub map[string]any) (time.Time, bool) {
	year, yok := numeric(sub["year"])
	month, mok := numeric(sub["month"])
	day, dok := numeric(sub["day"])
	if !yok || !mok || !dok || year < 1 || month < 1 || day < 1 {
		return time.Time{}, false
	}
	return time.Date(int(year), time.Month(int(month)), int(day), 0, 0, 0, 0, time.UTC), true
}

// metricName returns the kebab-case data type for a point. It prefers the
// data_type segment of the resource name (users/{u}/dataTypes/{t}/...)
// and falls back to kebab-casing the union field name.
func metricName(obj map[string]any, field string) string {
	if name, ok := obj["name"].(string); ok {
		if dt := dataTypeFromName(name); dt != "" {
			return dt
		}
	}
	return camelToKebab(field)
}

func dataTypeFromName(name string) string {
	parts := strings.Split(name, "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "dataTypes" {
			return parts[i+1]
		}
	}
	return ""
}

func camelToKebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// dailyMeans collapses a metric's points to one value per calendar day
// (the mean of that day's observations), returning days sorted ascending.
// Streaks and correlation operate on this daily series so intraday
// sampling rate never skews the result.
func dailyMeans(points []Point, metric string) ([]string, map[string]float64) {
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, p := range points {
		if metric != "" && p.Metric != metric {
			continue
		}
		sums[p.Day] += p.Value
		counts[p.Day]++
	}
	days := make([]string, 0, len(sums))
	means := make(map[string]float64, len(sums))
	for day, sum := range sums {
		days = append(days, day)
		means[day] = sum / float64(counts[day])
	}
	sort.Strings(days)
	return days, means
}

// metricsIn returns the distinct metric names present in points, sorted.
func metricsIn(points []Point) []string {
	set := map[string]bool{}
	for _, p := range points {
		set[p.Metric] = true
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// TopMetrics returns up to n metric names ordered by how many observations
// each has (descending, then alphabetically). Used to auto-select the most
// data-rich metrics when the user doesn't name one.
func TopMetrics(points []Point, n int) []string {
	counts := map[string]int{}
	for _, p := range points {
		counts[p.Metric]++
	}
	type mc struct {
		metric string
		count  int
	}
	arr := make([]mc, 0, len(counts))
	for m, c := range counts {
		arr = append(arr, mc{m, c})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].count != arr[j].count {
			return arr[i].count > arr[j].count
		}
		return arr[i].metric < arr[j].metric
	})
	out := make([]string, 0, n)
	for i := 0; i < len(arr) && i < n; i++ {
		out = append(out, arr[i].metric)
	}
	return out
}

// TrendPoint is one day on a metric's trend line: the day's mean value and
// the trailing rolling average over the configured window.
type TrendPoint struct {
	Day     string  `json:"day"`
	Value   float64 `json:"value"`
	Rolling float64 `json:"rolling_avg"`
}

// MetricTrend is a de-noised trend line for one metric: a rolling average
// series plus the net first→last change so a glance answers "is this
// going up or down?" without scale-weight whiplash.
type MetricTrend struct {
	Metric  string       `json:"metric"`
	Window  int          `json:"window_days"`
	Days    int          `json:"days"`
	First   float64      `json:"first_rolling"`
	Last    float64      `json:"last_rolling"`
	Delta   float64      `json:"delta"`
	Points  []TrendPoint `json:"points"`
}

// Trends computes a trailing rolling-average trend line for every metric
// in points (or just one when metric != ""). window is the rolling window
// in days; a window <= 1 yields the raw daily means. Metrics with no
// observations are omitted.
func Trends(points []Point, metric string, window int) []MetricTrend {
	if window < 1 {
		window = 1
	}
	var metrics []string
	if metric != "" {
		metrics = []string{metric}
	} else {
		metrics = metricsIn(points)
	}

	trends := make([]MetricTrend, 0, len(metrics))
	for _, m := range metrics {
		days, means := dailyMeans(points, m)
		if len(days) == 0 {
			continue
		}
		tp := make([]TrendPoint, 0, len(days))
		for i, day := range days {
			lo := i - window + 1
			if lo < 0 {
				lo = 0
			}
			var sum float64
			for j := lo; j <= i; j++ {
				sum += means[days[j]]
			}
			rolling := sum / float64(i-lo+1)
			tp = append(tp, TrendPoint{Day: day, Value: means[day], Rolling: rolling})
		}
		first := tp[0].Rolling
		last := tp[len(tp)-1].Rolling
		trends = append(trends, MetricTrend{
			Metric: m,
			Window: window,
			Days:   len(days),
			First:  first,
			Last:   last,
			Delta:  last - first,
			Points: tp,
		})
	}
	return trends
}

// StreakResult reports goal-streak stats for one metric: the current
// run of consecutive qualifying days ending at the most recent day, and
// the longest such run in the record.
type StreakResult struct {
	Metric        string  `json:"metric"`
	Op            string  `json:"op"`
	Threshold     float64 `json:"threshold"`
	QualifyingDays int    `json:"qualifying_days"`
	TotalDays     int     `json:"total_days"`
	Current       int     `json:"current_streak"`
	Longest       int     `json:"longest_streak"`
	LongestStart  string  `json:"longest_start,omitempty"`
	LongestEnd    string  `json:"longest_end,omitempty"`
}

// Streaks computes consecutive-calendar-day streaks where the metric's
// daily mean satisfies (op threshold). op is one of ">=", ">", "<=", "<",
// "==". A gap in calendar days breaks a streak even if the surrounding
// days qualify, so "current" reflects a genuinely unbroken run ending on
// the latest recorded day.
func Streaks(points []Point, metric string, threshold float64, op string) (StreakResult, error) {
	if metric == "" {
		return StreakResult{}, fmt.Errorf("streaks requires a --metric")
	}
	cmp, err := thresholdComparator(op, threshold)
	if err != nil {
		return StreakResult{}, err
	}
	days, means := dailyMeans(points, metric)
	res := StreakResult{Metric: metric, Op: op, Threshold: threshold, TotalDays: len(days)}
	if len(days) == 0 {
		return res, nil
	}

	var run int
	var runStart string
	var prev time.Time
	havePrev := false
	for _, day := range days {
		qualifies := cmp(means[day])
		if qualifies {
			res.QualifyingDays++
		}
		d, _ := time.Parse("2006-01-02", day)
		consecutive := havePrev && d.Sub(prev) == 24*time.Hour
		switch {
		case qualifies && consecutive && run > 0:
			// Continue the current unbroken run.
			run++
		case qualifies:
			// Start (or restart) a run on this day — either the first
			// qualifying day, a non-consecutive qualifying day, or the
			// day after a non-qualifying day reset run to 0. In every
			// case runStart must be this day, not a stale empty string.
			run = 1
			runStart = day
		default:
			run = 0
			runStart = ""
		}
		if run > res.Longest {
			res.Longest = run
			res.LongestStart = runStart
			res.LongestEnd = day
		}
		prev = d
		havePrev = true
	}
	// Current streak: the run is "current" only if it includes the latest day.
	res.Current = run
	return res, nil
}

// thresholdComparator returns a predicate testing a daily value against
// threshold using op. op is one of ">=", ">", "<=", "<", "==".
func thresholdComparator(op string, threshold float64) (func(float64) bool, error) {
	switch op {
	case ">=":
		return func(v float64) bool { return v >= threshold }, nil
	case ">":
		return func(v float64) bool { return v > threshold }, nil
	case "<=":
		return func(v float64) bool { return v <= threshold }, nil
	case "<":
		return func(v float64) bool { return v < threshold }, nil
	case "==", "=":
		return func(v float64) bool { return v == threshold }, nil
	default:
		return nil, fmt.Errorf("unknown op %q (use >=, >, <=, <, ==)", op)
	}
}

// CorrelationResult reports the relationship between two daily metric
// series: the Pearson correlation at zero lag, plus the lag (in days,
// metric B shifted relative to A) that maximizes |r|. A strong best-lag
// correlation at a non-zero lag is the interesting signal — e.g. today's
// step count predicting tomorrow's resting heart rate.
type CorrelationResult struct {
	MetricA      string  `json:"metric_a"`
	MetricB      string  `json:"metric_b"`
	OverlapDays  int     `json:"overlap_days"`
	R            float64 `json:"pearson_r"`
	BestLagDays  int     `json:"best_lag_days"`
	BestLagR     float64 `json:"best_lag_r"`
	BestLagN     int     `json:"best_lag_overlap_days"`
}

// Correlate computes the Pearson correlation between two metrics' daily
// means, aligned by calendar day, scanning lags in [-maxLag, +maxLag] to
// find the shift of B that maximizes |r|. It needs at least three
// overlapping days at zero lag to return a result.
func Correlate(points []Point, metricA, metricB string, maxLag int) (CorrelationResult, error) {
	if metricA == "" || metricB == "" {
		return CorrelationResult{}, fmt.Errorf("correlate requires two metrics")
	}
	if metricA == metricB {
		return CorrelationResult{}, fmt.Errorf("correlate requires two distinct metrics")
	}
	if maxLag < 0 {
		maxLag = 0
	}
	_, meansA := dailyMeans(points, metricA)
	daysB, meansB := dailyMeans(points, metricB)
	if len(meansA) == 0 || len(daysB) == 0 {
		return CorrelationResult{}, fmt.Errorf("no overlapping data for %s and %s: %w", metricA, metricB, ErrInsufficientData)
	}

	res := CorrelationResult{MetricA: metricA, MetricB: metricB}
	zeroR, zeroN, zeroOK := pearsonAtLag(meansA, meansB, 0)
	if !zeroOK {
		return CorrelationResult{}, fmt.Errorf("fewer than 3 overlapping days for %s and %s: %w", metricA, metricB, ErrInsufficientData)
	}
	res.R = zeroR
	res.OverlapDays = zeroN
	res.BestLagR = zeroR
	res.BestLagN = zeroN
	res.BestLagDays = 0

	for lag := -maxLag; lag <= maxLag; lag++ {
		r, n, ok := pearsonAtLag(meansA, meansB, lag)
		if !ok {
			continue
		}
		if math.Abs(r) > math.Abs(res.BestLagR) {
			res.BestLagR = r
			res.BestLagDays = lag
			res.BestLagN = n
		}
	}
	return res, nil
}

// pearsonAtLag aligns the two day→value maps with metric B shifted by lag
// days (positive lag pairs A's day d with B's day d+lag) and computes the
// Pearson correlation over the overlapping, paired days. Needs >=3 pairs.
func pearsonAtLag(a, b map[string]float64, lag int) (r float64, n int, ok bool) {
	var xs, ys []float64
	for day, av := range a {
		d, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		shifted := d.AddDate(0, 0, lag).Format("2006-01-02")
		if bv, found := b[shifted]; found {
			xs = append(xs, av)
			ys = append(ys, bv)
		}
	}
	if len(xs) < 3 {
		return 0, len(xs), false
	}
	return pearson(xs, ys), len(xs), true
}

// pearson returns the Pearson correlation coefficient of two equal-length
// slices. Returns 0 when either series has zero variance (a flat line
// has no linear relationship to anything).
func pearson(xs, ys []float64) float64 {
	n := float64(len(xs))
	var sx, sy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
	}
	mx, my := sx/n, sy/n
	var cov, vx, vy float64
	for i := range xs {
		dx := xs[i] - mx
		dy := ys[i] - my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx == 0 || vy == 0 {
		return 0
	}
	return cov / math.Sqrt(vx*vy)
}
