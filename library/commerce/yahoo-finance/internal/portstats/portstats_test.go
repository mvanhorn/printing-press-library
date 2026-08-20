package portstats

import (
	"errors"
	"math"
	"testing"
)

func TestPearson(t *testing.T) {
	cases := []struct {
		name    string
		a, b    []float64
		expect  float64
		wantErr error
	}{
		{"perfectly correlated", []float64{1, 2, 3, 4, 5}, []float64{2, 4, 6, 8, 10}, 1.0, nil},
		{"perfectly anti-correlated", []float64{1, 2, 3}, []float64{3, 2, 1}, -1.0, nil},
		{"identical series", []float64{5, 5, 5, 5}, []float64{5, 5, 5, 5}, 0.0, nil},
		{"two points up", []float64{1, 2}, []float64{1, 2}, 1.0, nil},
		{"unequal length", []float64{1, 2}, []float64{1, 2, 3}, 0, ErrLength},
		{"too short", []float64{1}, []float64{1}, 0, ErrTooShort},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Pearson(c.a, c.b)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("Pearson err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Pearson unexpected err: %v", err)
			}
			if math.Abs(got-c.expect) > 1e-4 {
				t.Errorf("Pearson = %v, want %v", got, c.expect)
			}
		})
	}
}

func TestDailyReturns(t *testing.T) {
	cases := []struct {
		name   string
		in     []float64
		expect []float64
	}{
		{"flat", []float64{100, 100, 100}, []float64{0, 0}},
		{"+10% +10%", []float64{100, 110, 121}, []float64{0.10, 0.10}},
		{"single point yields nil", []float64{100}, nil},
		{"zero prev safe", []float64{0, 100}, []float64{0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DailyReturns(c.in)
			if len(got) != len(c.expect) {
				t.Fatalf("len = %d, want %d", len(got), len(c.expect))
			}
			for i := range got {
				if math.Abs(got[i]-c.expect[i]) > 1e-9 {
					t.Errorf("DailyReturns[%d] = %v, want %v", i, got[i], c.expect[i])
				}
			}
		})
	}
}

func TestPairedReturns(t *testing.T) {
	a := []float64{100, 110, 121}
	b := []float64{50, 55, 60.5, 66.55} // truncated to len 3
	ra, rb := PairedReturns(a, b)
	if len(ra) != 2 || len(rb) != 2 {
		t.Fatalf("expected len 2/2, got %d/%d", len(ra), len(rb))
	}
	if math.Abs(ra[0]-0.10) > 1e-9 || math.Abs(rb[0]-0.10) > 1e-9 {
		t.Errorf("paired returns mismatch: ra=%v rb=%v", ra, rb)
	}
}
