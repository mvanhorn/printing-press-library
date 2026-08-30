// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

package pricehist

import (
	"math"
	"reflect"
	"testing"
)

const day = int64(86400)

func at(dayN int) int64 { return int64(dayN) * day }

func flat(price float64, n int, halfPrice bool) []Point {
	out := make([]Point, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Point{ObservedAt: at(i), Price: price, IsHalfPrice: halfPrice})
	}
	return out
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{4.5}, 4.5},
		{"odd", []float64{3, 1, 2}, 2},
		{"even", []float64{4, 1, 3, 2}, 2.5},
		{"duplicates", []float64{3, 3, 3, 3}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Median(tc.in); got != tc.want {
				t.Fatalf("Median(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMedianDoesNotMutateInput(t *testing.T) {
	in := []float64{3, 1, 2}
	_ = Median(in)
	if !reflect.DeepEqual(in, []float64{3, 1, 2}) {
		t.Fatalf("Median mutated its input: %v", in)
	}
}

func TestMinMax(t *testing.T) {
	tests := []struct {
		name             string
		in               []float64
		wantMin, wantMax float64
	}{
		{"empty", nil, 0, 0},
		{"single", []float64{2.5}, 2.5, 2.5},
		{"many", []float64{5, 1.5, 9, 3}, 1.5, 9},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Min(tc.in); got != tc.wantMin {
				t.Fatalf("Min(%v) = %v, want %v", tc.in, got, tc.wantMin)
			}
			if got := Max(tc.in); got != tc.wantMax {
				t.Fatalf("Max(%v) = %v, want %v", tc.in, got, tc.wantMax)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	points := []Point{
		{ObservedAt: at(10), Price: 6},
		{ObservedAt: at(0), Price: 4},
		{ObservedAt: at(5), Price: 5},
	}
	got := Summarize(points)
	want := Summary{Count: 3, Min: 4, Median: 5, Max: 6, FirstAt: at(0), LastAt: at(10), DaysOfHistory: 10}
	if got != want {
		t.Fatalf("Summarize = %+v, want %+v", got, want)
	}
	if empty := Summarize(nil); empty.Count != 0 || empty.Median != 0 {
		t.Fatalf("Summarize(nil) = %+v, want zero value", empty)
	}
}

func TestClassify(t *testing.T) {
	// A history that drifts around $5.50-$6.00 and never dips below $5.00.
	varied := []Point{
		{ObservedAt: at(0), Price: 5.00},
		{ObservedAt: at(1), Price: 5.50},
		{ObservedAt: at(2), Price: 6.00},
		{ObservedAt: at(3), Price: 5.50},
		{ObservedAt: at(4), Price: 6.00},
		{ObservedAt: at(5), Price: 5.75},
	}

	tests := []struct {
		name string
		in   ClassifyInput
		want string
	}{
		{
			name: "no prior observations at all",
			in:   ClassifyInput{CurrentPrice: 3, WasPrice: 6, Prior: nil},
			want: VerdictNoHistory,
		},
		{
			name: "one prior observation is still not history",
			in:   ClassifyInput{CurrentPrice: 3, WasPrice: 6, Prior: flat(6, 1, false)},
			want: VerdictNoHistory,
		},
		{
			name: "two prior observations are still not history",
			in:   ClassifyInput{CurrentPrice: 3, WasPrice: 6, Prior: flat(6, 2, false)},
			want: VerdictNoHistory,
		},
		{
			name: "parked at one price with a doubled was-price",
			in:   ClassifyInput{CurrentPrice: 3, WasPrice: 6, Prior: flat(3, 20, false)},
			want: VerdictWasInflated,
		},
		{
			name: "new low well under the median",
			in:   ClassifyInput{CurrentPrice: 3.50, WasPrice: 5.50, Prior: varied},
			want: VerdictGenuine,
		},
		{
			name: "habitual cycle low",
			in: ClassifyInput{CurrentPrice: 5.00, WasPrice: 6.00, Prior: []Point{
				{Price: 5.00}, {Price: 8.00}, {Price: 5.00}, {Price: 8.00}, {Price: 5.00}, {Price: 8.00},
			}},
			want: VerdictRecycled,
		},
		{
			name: "mid-range price with a plausible was-price",
			in:   ClassifyInput{CurrentPrice: 5.75, WasPrice: 6.00, Prior: varied},
			want: VerdictOrdinary,
		},
		{
			// The was-price here is real: the product sat at $8 four times
			// out of five, so $8 is not an inflated reference and the drop to
			// $5 is a genuine low rather than a manufactured saving.
			name: "high was-price the product genuinely sold at is not inflation",
			in: ClassifyInput{CurrentPrice: 5.00, WasPrice: 8.00, Prior: []Point{
				{Price: 8.00}, {Price: 8.00}, {Price: 8.00}, {Price: 8.00}, {Price: 5.00},
			}},
			want: VerdictGenuine,
		},
		{
			name: "zero was-price never triggers inflation",
			in:   ClassifyInput{CurrentPrice: 3, WasPrice: 0, Prior: flat(3, 20, false)},
			want: VerdictRecycled,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.in)
			if got.Label != tc.want {
				t.Fatalf("Classify(%+v).Label = %q (reason %q), want %q", tc.in, got.Label, got.Reason, tc.want)
			}
			if got.Reason == "" {
				t.Fatalf("Classify returned verdict %q with no reason", got.Label)
			}
		})
	}
}

func TestClassifyNoHistoryIsNeverConfident(t *testing.T) {
	for n := 0; n < MinObservationsForVerdict; n++ {
		got := Classify(ClassifyInput{CurrentPrice: 1, WasPrice: 99, Prior: flat(50, n, false)})
		if got.Label != VerdictNoHistory {
			t.Fatalf("with %d prior observations Classify = %q, want %q", n, got.Label, VerdictNoHistory)
		}
	}
	got := Classify(ClassifyInput{CurrentPrice: 1, WasPrice: 99, Prior: flat(50, MinObservationsForVerdict, false)})
	if got.Label == VerdictNoHistory {
		t.Fatalf("with %d prior observations Classify still refused to judge", MinObservationsForVerdict)
	}
}

func TestHalfPriceEpisodes(t *testing.T) {
	tests := []struct {
		name   string
		points []Point
		want   []Episode
	}{
		{"empty", nil, []Episode{}},
		{"never half price", flat(6, 5, false), []Episode{}},
		{
			name: "single observation episode",
			points: []Point{
				{ObservedAt: at(0), Price: 6, IsHalfPrice: false},
				{ObservedAt: at(1), Price: 3, IsHalfPrice: true},
				{ObservedAt: at(2), Price: 6, IsHalfPrice: false},
			},
			want: []Episode{{StartAt: at(1), EndAt: at(1), Observations: 1, RunDays: 0, LowPrice: 3}},
		},
		{
			name: "two episodes split by a full price sighting",
			points: []Point{
				{ObservedAt: at(0), Price: 3, IsHalfPrice: true},
				{ObservedAt: at(1), Price: 3, IsHalfPrice: true},
				{ObservedAt: at(2), Price: 6, IsHalfPrice: false},
				{ObservedAt: at(9), Price: 3, IsHalfPrice: true},
			},
			want: []Episode{
				{StartAt: at(0), EndAt: at(1), Observations: 2, RunDays: 1, LowPrice: 3},
				{StartAt: at(9), EndAt: at(9), Observations: 1, RunDays: 0, LowPrice: 3},
			},
		},
		{
			name: "unsorted input is sorted first",
			points: []Point{
				{ObservedAt: at(2), Price: 6, IsHalfPrice: false},
				{ObservedAt: at(1), Price: 3, IsHalfPrice: true},
				{ObservedAt: at(0), Price: 3, IsHalfPrice: true},
			},
			want: []Episode{{StartAt: at(0), EndAt: at(1), Observations: 2, RunDays: 1, LowPrice: 3}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HalfPriceEpisodes(tc.points)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("HalfPriceEpisodes = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// threeEpisodes builds daily observations across 3 half-price episodes of 3
// days each, with episode starts exactly 28 days apart.
func threeEpisodes() []Point {
	points := make([]Point, 0, 60)
	for d := 0; d < 60; d++ {
		half := (d >= 0 && d <= 2) || (d >= 28 && d <= 30) || (d >= 56 && d <= 58)
		price := 6.0
		if half {
			price = 3.0
		}
		points = append(points, Point{ObservedAt: at(d), Price: price, IsHalfPrice: half})
	}
	return points
}

func TestAnalyzeCycle(t *testing.T) {
	t.Run("no episodes yields no forecast", func(t *testing.T) {
		got := AnalyzeCycle(flat(6, 30, false), at(30))
		if got.Episodes != 0 || got.Forecast || got.NextWindowStartAt != 0 {
			t.Fatalf("AnalyzeCycle on a never-discounted history fabricated a forecast: %+v", got)
		}
		if got.Confidence != ConfidenceNone {
			t.Fatalf("confidence = %q, want %q", got.Confidence, ConfidenceNone)
		}
		if got.Note == "" {
			t.Fatalf("no-episode result carried no explanatory note")
		}
	})

	t.Run("single episode cannot measure a gap", func(t *testing.T) {
		points := []Point{
			{ObservedAt: at(0), Price: 6},
			{ObservedAt: at(1), Price: 3, IsHalfPrice: true},
			{ObservedAt: at(2), Price: 3, IsHalfPrice: true},
			{ObservedAt: at(3), Price: 6},
		}
		got := AnalyzeCycle(points, at(10))
		if got.Forecast || got.NextWindowStartAt != 0 {
			t.Fatalf("single episode produced a forecast: %+v", got)
		}
		if got.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want %q", got.Confidence, ConfidenceLow)
		}
	})

	t.Run("three regular episodes", func(t *testing.T) {
		got := AnalyzeCycle(threeEpisodes(), at(60))
		if got.Episodes != 3 {
			t.Fatalf("episodes = %d, want 3", got.Episodes)
		}
		if math.Abs(got.MedianGapDays-28) > 0.05 {
			t.Fatalf("median gap = %v days, want ~28", got.MedianGapDays)
		}
		if math.Abs(got.MedianRunDays-2) > 0.05 {
			t.Fatalf("median run = %v days, want ~2", got.MedianRunDays)
		}
		if got.Confidence == ConfidenceLow || got.Confidence == ConfidenceNone {
			t.Fatalf("confidence = %q, want better than low for 3 regular episodes", got.Confidence)
		}
		if !got.Forecast || got.NextWindowStartAt <= at(60) {
			t.Fatalf("expected a forward-looking window, got %+v", got)
		}
		if got.NextWindowEndAt <= got.NextWindowStartAt {
			t.Fatalf("window end %d not after start %d", got.NextWindowEndAt, got.NextWindowStartAt)
		}
		if math.Abs(got.DaysSinceLastEnd-2) > 0.05 {
			t.Fatalf("days since last end = %v, want ~2", got.DaysSinceLastEnd)
		}
	})

	t.Run("irregular gaps downgrade confidence", func(t *testing.T) {
		points := make([]Point, 0, 120)
		half := map[int]bool{0: true, 5: true, 90: true}
		for d := 0; d < 120; d++ {
			points = append(points, Point{ObservedAt: at(d), Price: 6, IsHalfPrice: half[d]})
		}
		got := AnalyzeCycle(points, at(120))
		if got.Episodes != 3 {
			t.Fatalf("episodes = %d, want 3", got.Episodes)
		}
		if got.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want %q for gaps of 5 and 85 days", got.Confidence, ConfidenceLow)
		}
	})
}

func TestTermRelevant(t *testing.T) {
	tests := []struct {
		name        string
		term        string
		productName string
		brand       string
		want        bool
	}{
		{"exact phrase", "tim tam", "Arnott's Tim Tam Original Chocolate Biscuits", "Arnott's", true},
		{"one token missing", "tim tam", "Arnott's Shapes Original", "Arnott's", false},
		{"nonsense term", "zzqqxx nonsense", "Arnott's Tim Tam Original", "Arnott's", false},
		{"brand only match", "arnotts", "Shapes Barbecue", "Arnotts", true},
		{"short tokens ignored", "og milk", "Milk 1L", "Woolworths", true},
		{"empty term matches", "", "Anything", "", true},
		{"case insensitive", "TIM TAM", "arnott's tim tam", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TermRelevant(tc.term, tc.productName, tc.brand); got != tc.want {
				t.Fatalf("TermRelevant(%q, %q, %q) = %v, want %v", tc.term, tc.productName, tc.brand, got, tc.want)
			}
		})
	}
}

func TestTokenizeAndIsStockcode(t *testing.T) {
	if got, want := Tokenize("Tim-Tam 200g!"), []string{"tim", "tam", "200g"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokenize = %v, want %v", got, want)
	}
	stockcodes := map[string]bool{"6035173": true, "36066": true, "": false, "tim tam": false, "60351a": false, " 123": true}
	for in, want := range stockcodes {
		if got := IsStockcode(in); got != want {
			t.Fatalf("IsStockcode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRounding(t *testing.T) {
	if got := Round2(3.0000000000000004); got != 3 {
		t.Fatalf("Round2 = %v, want 3", got)
	}
	if got := Round1(27.96); got != 28 {
		t.Fatalf("Round1 = %v, want 28", got)
	}
}
