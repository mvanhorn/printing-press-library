package malhtml

import (
	"strings"
	"testing"
)

func consistencyEpisodesFromPolls(polls ...float64) []Episode {
	eps := make([]Episode, 0, len(polls))
	for i, poll := range polls {
		eps = append(eps, Episode{Number: i + 1, PollAverage: poll})
	}
	return eps
}

// TestConsistencySmallSamplesAreInsufficient covers the 1-, 2-, 3- and
// 5-episode cases: with fewer than six rated episodes the first-3 and last-3
// windows overlap (or are identical), so no direction can be supported. The
// verdict must say so and the slope must be marked as not computed rather than
// reported as a measured zero.
func TestConsistencySmallSamplesAreInsufficient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		polls []float64
	}{
		{"one rated episode", []float64{4.0}},
		{"two rated episodes descending", []float64{1.0, 5.0}},
		{"two rated episodes ascending", []float64{5.0, 1.0}},
		{"three rated episodes", []float64{1.0, 3.0, 5.0}},
		{"five rated episodes", []float64{1.0, 1.0, 1.0, 5.0, 5.0}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Consistency(1, consistencyEpisodesFromPolls(tc.polls...))
			if err != nil {
				t.Fatalf("Consistency: %v", err)
			}
			if got.SlopeComputed {
				t.Fatalf("slope_computed = true with %d rated episode(s); want false", len(tc.polls))
			}
			if got.Slope != 0 {
				t.Fatalf("slope = %v with %d rated episode(s); want 0 when not computed", got.Slope, len(tc.polls))
			}
			if !strings.Contains(got.Verdict, "insufficient data") {
				t.Fatalf("verdict = %q with %d rated episode(s); want an explicit insufficient-data verdict", got.Verdict, len(tc.polls))
			}
		})
	}
}

// TestConsistencyTwoEpisodeOrderCannotLookStable is the regression guard for
// the original bug: two rated episodes in either order produced slope 0 and the
// verdict "holds steady", which read as a directional finding.
func TestConsistencyTwoEpisodeOrderCannotLookStable(t *testing.T) {
	t.Parallel()

	ascending, err := Consistency(1, consistencyEpisodesFromPolls(5.0, 1.0))
	if err != nil {
		t.Fatalf("Consistency(ascending): %v", err)
	}
	descending, err := Consistency(1, consistencyEpisodesFromPolls(1.0, 5.0))
	if err != nil {
		t.Fatalf("Consistency(descending): %v", err)
	}
	if ascending.Verdict != descending.Verdict {
		t.Fatalf("verdicts differ by episode order: %q vs %q", ascending.Verdict, descending.Verdict)
	}
	if ascending.Verdict == "holds steady" {
		t.Fatal(`two rated episodes still report "holds steady"; want an insufficient-data verdict`)
	}
}

// TestConsistencySlopeWithNonOverlappingWindows covers the 6+ rated-episode
// cases, where the windows genuinely do not overlap and a direction is
// supported.
func TestConsistencySlopeWithNonOverlappingWindows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		polls       []float64
		wantSlope   string // "positive", "negative", "zero"
		wantVerdict string
	}{
		{"six episodes improving", []float64{3, 3, 3, 5, 5, 5}, "positive", "improves"},
		{"six episodes falling off", []float64{5, 5, 5, 3, 3, 3}, "negative", "falls off"},
		{"six episodes flat", []float64{4, 4, 4, 4, 4, 4}, "zero", "holds steady"},
		{"seven episodes improving", []float64{2, 2, 2, 3, 4, 5, 6}, "positive", "improves"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Consistency(1, consistencyEpisodesFromPolls(tc.polls...))
			if err != nil {
				t.Fatalf("Consistency: %v", err)
			}
			if !got.SlopeComputed {
				t.Fatalf("slope_computed = false with %d rated episode(s); want true", len(tc.polls))
			}
			if got.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
			switch tc.wantSlope {
			case "positive":
				if got.Slope <= 0 {
					t.Fatalf("slope = %v, want > 0", got.Slope)
				}
			case "negative":
				if got.Slope >= 0 {
					t.Fatalf("slope = %v, want < 0", got.Slope)
				}
			case "zero":
				if got.Slope != 0 {
					t.Fatalf("slope = %v, want 0", got.Slope)
				}
			}
		})
	}
}

// TestConsistencyIgnoresUnratedEpisodes keeps the rating filter honest: an
// unrated episode must not pad the sample size into the trend branch.
func TestConsistencyIgnoresUnratedEpisodes(t *testing.T) {
	t.Parallel()

	got, err := Consistency(1, []Episode{
		{Number: 1, PollAverage: 3},
		{Number: 2, PollAverage: 3},
		{Number: 3, PollAverage: 3},
		{Number: 4}, // unrated
		{Number: 5},
		{Number: 6},
		{Number: 7, PollAverage: 5},
		{Number: 8, PollAverage: 5},
		{Number: 9, PollAverage: 5},
	})
	if err != nil {
		t.Fatalf("Consistency: %v", err)
	}
	if got.RatedEpisodes != 6 {
		t.Fatalf("rated_episodes = %d, want 6", got.RatedEpisodes)
	}
	if !got.SlopeComputed {
		t.Fatal("slope_computed = false with six rated episodes; want true")
	}
}
