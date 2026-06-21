// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package health

import (
	"math"
	"testing"
)

// stepsRow builds a synced `steps` DataPoint JSON row for a given day and
// count, using the interval time wrapper Google emits for interval types.
func stepsRow(day string, count int) []byte {
	return []byte(`{
		"name": "users/me/dataTypes/steps/dataPoints/` + day + `",
		"steps": {
			"count": "` + itoa(count) + `",
			"interval": {"startTime": "` + day + `T00:00:00Z", "endTime": "` + day + `T23:59:59Z"}
		}
	}`)
}

// rhrRow builds a daily-resting-heart-rate row, which uses the daily date
// wrapper and a string-encoded int64 value.
func rhrRow(year, month, dayNum, bpm int) []byte {
	return []byte(`{
		"name": "users/me/dataTypes/daily-resting-heart-rate/dataPoints/x",
		"dailyRestingHeartRate": {
			"beatsPerMinute": "` + itoa(bpm) + `",
			"date": {"year": ` + itoa(year) + `, "month": ` + itoa(month) + `, "day": ` + itoa(dayNum) + `}
		}
	}`)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestExtractPoint(t *testing.T) {
	t.Run("interval steps with string int64", func(t *testing.T) {
		p, ok := ExtractPoint(stepsRow("2026-06-01", 8421))
		if !ok {
			t.Fatal("expected steps row to extract")
		}
		if p.Metric != "steps" {
			t.Errorf("metric = %q, want steps", p.Metric)
		}
		if p.Value != 8421 {
			t.Errorf("value = %v, want 8421", p.Value)
		}
		if p.Day != "2026-06-01" {
			t.Errorf("day = %q, want 2026-06-01", p.Day)
		}
	})

	t.Run("daily date wrapper", func(t *testing.T) {
		p, ok := ExtractPoint(rhrRow(2026, 6, 2, 58))
		if !ok {
			t.Fatal("expected rhr row to extract")
		}
		if p.Metric != "daily-resting-heart-rate" {
			t.Errorf("metric = %q", p.Metric)
		}
		if p.Value != 58 {
			t.Errorf("value = %v, want 58", p.Value)
		}
		if p.Day != "2026-06-02" {
			t.Errorf("day = %q, want 2026-06-02", p.Day)
		}
	})

	t.Run("weight sample with native double", func(t *testing.T) {
		row := []byte(`{"name":"users/me/dataTypes/weight/dataPoints/y","weight":{"weightGrams":81600.5,"sampleTime":{"physicalTime":"2026-06-03T07:15:00Z"}}}`)
		p, ok := ExtractPoint(row)
		if !ok {
			t.Fatal("expected weight row to extract")
		}
		if p.Metric != "weight" || p.Value != 81600.5 {
			t.Errorf("got metric=%q value=%v", p.Metric, p.Value)
		}
	})

	t.Run("nanosecond-precision timestamps extract", func(t *testing.T) {
		// Google Cloud APIs serialize timestamps with fractional seconds;
		// these must parse via RFC3339Nano, not plain RFC3339.
		rows := [][]byte{
			[]byte(`{"name":"users/me/dataTypes/steps/dataPoints/a","steps":{"count":8421,"interval":{"startTime":"2026-06-01T00:00:00.000000000Z","endTime":"2026-06-01T23:59:59.999999999Z"}}}`),
			[]byte(`{"name":"users/me/dataTypes/weight/dataPoints/b","weight":{"weightGrams":81600.5,"sampleTime":{"physicalTime":"2026-06-03T07:15:00.123456789Z"}}}`),
		}
		for _, row := range rows {
			p, ok := ExtractPoint(row)
			if !ok {
				t.Fatalf("expected fractional-second row to extract: %s", row)
			}
			if p.Day == "" {
				t.Errorf("expected non-empty Day for %s", row)
			}
		}
	})

	t.Run("non-point rows rejected", func(t *testing.T) {
		for _, row := range [][]byte{
			[]byte(`{"name":"users/me","healthUserId":"abc"}`),       // identity
			[]byte(`{"displayName":"Ryan","timezone":"UTC"}`),        // profile
			[]byte(`not json`),                                       // garbage
			[]byte(`{"steps":{"interval":{"startTime":"2026-06-01T00:00:00Z"}}}`), // no value
		} {
			if _, ok := ExtractPoint(row); ok {
				t.Errorf("expected rejection of %s", row)
			}
		}
	})
}

func TestTrends(t *testing.T) {
	rows := [][]byte{
		stepsRow("2026-06-01", 10000),
		stepsRow("2026-06-02", 2000),
		stepsRow("2026-06-03", 12000),
		stepsRow("2026-06-04", 4000),
	}
	points := ExtractPoints(rows)
	if len(points) != 4 {
		t.Fatalf("extracted %d points, want 4", len(points))
	}

	trends := Trends(points, "steps", 2)
	if len(trends) != 1 {
		t.Fatalf("got %d trends, want 1", len(trends))
	}
	tr := trends[0]
	if tr.Metric != "steps" || tr.Days != 4 {
		t.Errorf("metric=%q days=%d", tr.Metric, tr.Days)
	}
	// Day 4 rolling avg over window 2 = mean(12000, 4000) = 8000.
	last := tr.Points[len(tr.Points)-1]
	if last.Rolling != 8000 {
		t.Errorf("last rolling = %v, want 8000", last.Rolling)
	}
	// Day 1 rolling = raw 10000; delta = 8000 - 10000 = -2000.
	if tr.Delta != -2000 {
		t.Errorf("delta = %v, want -2000", tr.Delta)
	}
}

func TestTrendsDailyMeanAggregation(t *testing.T) {
	// Two intraday step readings on the same day should average, not sum,
	// into the daily series.
	rows := [][]byte{
		stepsRow("2026-06-01", 4000),
		stepsRow("2026-06-01", 6000),
	}
	// Both rows share an id suffix; force distinct content is unnecessary
	// here because Trends groups by Day regardless of id.
	points := ExtractPoints(rows)
	trends := Trends(points, "steps", 1)
	if len(trends) != 1 {
		t.Fatalf("got %d trends", len(trends))
	}
	if got := trends[0].Points[0].Value; got != 5000 {
		t.Errorf("daily mean = %v, want 5000", got)
	}
}

func TestStreaks(t *testing.T) {
	// 10k-step goal: hit, hit, miss, hit, hit, hit → longest 3, current 3.
	rows := [][]byte{
		stepsRow("2026-06-01", 11000),
		stepsRow("2026-06-02", 10500),
		stepsRow("2026-06-03", 3000),
		stepsRow("2026-06-04", 12000),
		stepsRow("2026-06-05", 10000),
		stepsRow("2026-06-06", 15000),
	}
	points := ExtractPoints(rows)
	res, err := Streaks(points, "steps", 10000, ">=")
	if err != nil {
		t.Fatal(err)
	}
	if res.Longest != 3 {
		t.Errorf("longest = %d, want 3", res.Longest)
	}
	if res.Current != 3 {
		t.Errorf("current = %d, want 3", res.Current)
	}
	if res.QualifyingDays != 5 {
		t.Errorf("qualifying = %d, want 5", res.QualifyingDays)
	}
	if res.LongestEnd != "2026-06-06" {
		t.Errorf("longest end = %q", res.LongestEnd)
	}
	// The longest streak (Day4–Day6) follows a non-qualifying day, so this
	// guards the LongestStart reset-after-gap bug.
	if res.LongestStart != "2026-06-04" {
		t.Errorf("longest start = %q, want 2026-06-04", res.LongestStart)
	}
}

func TestStreaksGapBreaksRun(t *testing.T) {
	// A calendar gap (missing 06-03) breaks the streak even though both
	// surrounding days qualify.
	rows := [][]byte{
		stepsRow("2026-06-01", 11000),
		stepsRow("2026-06-02", 11000),
		stepsRow("2026-06-04", 11000),
	}
	points := ExtractPoints(rows)
	res, err := Streaks(points, "steps", 10000, ">=")
	if err != nil {
		t.Fatal(err)
	}
	if res.Longest != 2 {
		t.Errorf("longest = %d, want 2 (gap breaks)", res.Longest)
	}
	if res.Current != 1 {
		t.Errorf("current = %d, want 1 (only the last day)", res.Current)
	}
}

func TestStreaksErrors(t *testing.T) {
	if _, err := Streaks(nil, "", 1, ">="); err == nil {
		t.Error("expected error for empty metric")
	}
	if _, err := Streaks(nil, "steps", 1, "≥"); err == nil {
		t.Error("expected error for bad op")
	}
}

func TestCorrelate(t *testing.T) {
	// Perfect positive: steps and a synthetic metric move together.
	var rows [][]byte
	for i := 1; i <= 6; i++ {
		rows = append(rows, stepsRow(dayStr(i), i*1000))
		rows = append(rows, distanceRow(dayStr(i), float64(i)*800))
	}
	points := ExtractPoints(rows)
	res, err := Correlate(points, "steps", "distance", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.OverlapDays != 6 {
		t.Errorf("overlap = %d, want 6", res.OverlapDays)
	}
	if math.Abs(res.R-1.0) > 1e-9 {
		t.Errorf("pearson r = %v, want ~1.0", res.R)
	}
}

func TestCorrelateBestLag(t *testing.T) {
	// metric B lags A by 1 day: B[d] == A[d-1]. Best lag should be +1
	// (A's day d pairs with B's day d+1) with r ~ 1.0.
	aVals := []int{1000, 5000, 2000, 8000, 3000, 9000}
	var rows [][]byte
	for i, v := range aVals {
		rows = append(rows, stepsRow(dayStr(i+1), v))
	}
	for i, v := range aVals {
		rows = append(rows, distanceRow(dayStr(i+2), float64(v))) // shifted +1 day
	}
	points := ExtractPoints(rows)
	res, err := Correlate(points, "steps", "distance", 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.BestLagDays != 1 {
		t.Errorf("best lag = %d, want 1", res.BestLagDays)
	}
	if math.Abs(res.BestLagR-1.0) > 1e-9 {
		t.Errorf("best lag r = %v, want ~1.0", res.BestLagR)
	}
}

func TestCorrelateErrors(t *testing.T) {
	if _, err := Correlate(nil, "steps", "steps", 0); err == nil {
		t.Error("expected error for identical metrics")
	}
	if _, err := Correlate(ExtractPoints([][]byte{stepsRow("2026-06-01", 1000)}), "steps", "distance", 0); err == nil {
		t.Error("expected error for too few overlapping days")
	}
}

func TestPearson(t *testing.T) {
	if r := pearson([]float64{1, 2, 3}, []float64{2, 4, 6}); math.Abs(r-1.0) > 1e-9 {
		t.Errorf("perfect positive r = %v", r)
	}
	if r := pearson([]float64{1, 2, 3}, []float64{6, 4, 2}); math.Abs(r+1.0) > 1e-9 {
		t.Errorf("perfect negative r = %v", r)
	}
	if r := pearson([]float64{1, 2, 3}, []float64{5, 5, 5}); r != 0 {
		t.Errorf("flat series r = %v, want 0", r)
	}
}

func TestTopMetrics(t *testing.T) {
	rows := [][]byte{
		stepsRow("2026-06-01", 1000),
		stepsRow("2026-06-02", 1000),
		stepsRow("2026-06-03", 1000),
		distanceRow("2026-06-01", 500),
		distanceRow("2026-06-02", 500),
		rhrRow(2026, 6, 1, 60),
	}
	points := ExtractPoints(rows)
	top := TopMetrics(points, 2)
	if len(top) != 2 {
		t.Fatalf("got %d metrics, want 2", len(top))
	}
	if top[0] != "steps" {
		t.Errorf("top metric = %q, want steps (most points)", top[0])
	}
	if top[1] != "distance" {
		t.Errorf("second metric = %q, want distance", top[1])
	}
	if got := TopMetrics(nil, 2); len(got) != 0 {
		t.Errorf("empty points should yield no metrics, got %v", got)
	}
}

func dayStr(d int) string {
	return "2026-06-0" + itoa(d)
}

func distanceRow(day string, meters float64) []byte {
	return []byte(`{
		"name": "users/me/dataTypes/distance/dataPoints/` + day + `",
		"distance": {
			"distanceMeters": ` + ftoa(meters) + `,
			"interval": {"startTime": "` + day + `T00:00:00Z"}
		}
	}`)
}

func ftoa(f float64) string {
	i := int(f)
	frac := int(math.Round((f - float64(i)) * 100))
	if frac == 0 {
		return itoa(i)
	}
	return itoa(i) + "." + itoa(frac)
}
