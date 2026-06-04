// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"testing"
)

func chartFromValues(rows ...string) chartData {
	cd := chartData{Resolution: "month"}
	for _, r := range rows {
		cd.Values = append(cd.Values, json.RawMessage(r))
	}
	return cd
}

func TestConvertedTotal(t *testing.T) {
	// Count shape (RevenueCat's live-verified behavior): sum the per-period
	// counts.
	count := chartFromValues(
		`{"cohort":1746057600,"measure":0,"value":3}`,
		`{"cohort":1748736000,"measure":0,"value":5}`,
	)
	if got := convertedTotal(count, 100); !approx(got, 8) {
		t.Fatalf("count-shape converted = %v, want 8", got)
	}
	// Ratio shape (defensive guard): fractional values in (0,1) -> derive
	// newTrials * mean(ratio). mean(0.2,0.4)=0.3; 100*0.3 = 30.
	ratio := chartFromValues(
		`{"cohort":1746057600,"measure":0,"value":0.2}`,
		`{"cohort":1748736000,"measure":0,"value":0.4}`,
	)
	if got := convertedTotal(ratio, 100); !approx(got, 30) {
		t.Fatalf("ratio-shape converted = %v, want 30", got)
	}
	// Empty chart -> 0.
	if got := convertedTotal(chartData{}, 100); got != 0 {
		t.Fatalf("empty converted = %v, want 0", got)
	}
}

func TestBuildFunnel(t *testing.T) {
	cases := []struct {
		name        string
		newTrials   float64
		converted   float64
		wantConv    float64 // stage-2 conversion ratio
		wantDrop    float64
		wantOverall float64
		wantNote    bool
	}{
		{"half convert", 100, 40, 0.4, 0.6, 40, false},
		{"all convert", 50, 50, 1.0, 0.0, 100, false},
		{"none", 0, 0, 0, 0, 0, false},
		// Degenerate: more converts than new trials in the window means the
		// conversions counted include trials that started before the window,
		// so no clean stage-to-stage rate exists — expect a note, not a >100%.
		{"no trials but converts (degenerate)", 0, 5, 0, 0, 0, true},
		{"more converts than trials", 10, 12, 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stages, overall, note := buildFunnel(tc.newTrials, tc.converted)
			if len(stages) != 2 {
				t.Fatalf("stages = %d, want 2", len(stages))
			}
			if stages[0].Stage != "trials_new" || stages[0].Count != tc.newTrials {
				t.Fatalf("stage0 = %+v", stages[0])
			}
			// The leading stage has no prior stage; its conversion_from_prev
			// must be 0, never a misleading 100%.
			if stages[0].ConversionRate != 0 {
				t.Fatalf("stage0 conversion = %v, want 0", stages[0].ConversionRate)
			}
			conv := stages[1]
			if !approx(conv.ConversionRate, tc.wantConv) {
				t.Fatalf("conversion = %v, want %v", conv.ConversionRate, tc.wantConv)
			}
			if !approx(conv.DropOff, tc.wantDrop) {
				t.Fatalf("dropoff = %v, want %v", conv.DropOff, tc.wantDrop)
			}
			if !approx(overall, tc.wantOverall) {
				t.Fatalf("overall = %v, want %v", overall, tc.wantOverall)
			}
			if (note != "") != tc.wantNote {
				t.Fatalf("note = %q, wantNote = %v", note, tc.wantNote)
			}
		})
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
