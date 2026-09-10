package cli

import (
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"
	"strings"
	"testing"
)

// TestAdaptationBandScalesWithReachedEpisodes is the regression guard for the
// original bug: the unfinished branch recomputed the announced episode count
// times chapters-per-episode, which is algebraically the full chapter count, so
// every still-airing anime reported ~80-100% source coverage. The band must
// track how far the adaptation actually reached.
func TestAdaptationBandScalesWithReachedEpisodes(t *testing.T) {
	t.Parallel()

	// A 100-chapter source across 25 announced episodes: 4 chapters/episode.
	cases := []struct {
		name     string
		reached  int
		wantLow  int
		wantHigh int
	}{
		{"only two episodes aired", 2, 6, 10},
		{"five episodes aired", 5, 16, 24},
		{"ten episodes aired", 10, 32, 48},
		{"every announced episode reached", 25, 80, 100},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cpe, low, high, ok := adaptationBand(100, 25, tc.reached)
			if !ok {
				t.Fatalf("adaptationBand(100, 25, %d) returned ok=false", tc.reached)
			}
			if cpe != 4 {
				t.Fatalf("chapters per episode = %v, want 4", cpe)
			}
			if low != tc.wantLow || high != tc.wantHigh {
				t.Fatalf("band = %d-%d, want %d-%d", low, high, tc.wantLow, tc.wantHigh)
			}
			if high > 100 {
				t.Fatalf("band high = %d, want the chapter total (100) as the cap", high)
			}
		})
	}
}

// TestAdaptationBandNeverAssumesCompletion is the direct falsification of the
// reported bug: a partly-reached adaptation must not report the same band as a
// fully-reached one.
func TestAdaptationBandNeverAssumesCompletion(t *testing.T) {
	t.Parallel()

	_, lowEarly, highEarly, ok := adaptationBand(100, 25, 5)
	if !ok {
		t.Fatal("adaptationBand(100, 25, 5) returned ok=false")
	}
	_, lowFull, highFull, ok := adaptationBand(100, 25, 25)
	if !ok {
		t.Fatal("adaptationBand(100, 25, 25) returned ok=false")
	}
	if lowEarly >= lowFull || highEarly >= highFull {
		t.Fatalf("reached=5 band %d-%d is not below reached=25 band %d-%d; coverage still assumes completion",
			lowEarly, highEarly, lowFull, highFull)
	}
	if highFull != 100 {
		t.Fatalf("reached=25 band high = %d, want 100", highFull)
	}
}

// TestAdaptationBandIsMonotonicInReach asserts the band grows with the number
// of episodes reached, so a partly-reached adaptation can never look better
// covered than a further-reached one.
func TestAdaptationBandIsMonotonicInReach(t *testing.T) {
	t.Parallel()

	lastHigh := -1
	for reached := 1; reached <= 25; reached++ {
		_, low, high, ok := adaptationBand(100, 25, reached)
		if !ok {
			t.Fatalf("adaptationBand(100, 25, %d) returned ok=false", reached)
		}
		if high < lastHigh {
			t.Fatalf("reached=%d band high %d is below the previous band high %d", reached, high, lastHigh)
		}
		if low > high {
			t.Fatalf("reached=%d band %d-%d is inverted", reached, low, high)
		}
		lastHigh = high
	}
}

func TestAdaptationBandUnknownCounts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                              string
		mangaChapters, announced, reached int
	}{
		{"no chapter count", 0, 25, 5},
		{"no announced episode total", 100, 0, 5},
		{"no reached episode count", 100, 25, 0},
		{"negative reached count", 100, 25, -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, _, ok := adaptationBand(tc.mangaChapters, tc.announced, tc.reached); ok {
				t.Fatalf("adaptationBand(%d, %d, %d) returned ok=true with an unknown count",
					tc.mangaChapters, tc.announced, tc.reached)
			}
		})
	}
}

func TestAiredEpisodeCountUsesFurthestEpisodeNumber(t *testing.T) {
	t.Parallel()

	// A table with gaps (a skipped special) reports the furthest episode
	// reached, not how many rows happen to be present.
	eps := []malhtml.Episode{{Number: 3}, {Number: 9}, {Number: 5}}
	if got := airedEpisodeCount(eps); got != 9 {
		t.Fatalf("airedEpisodeCount = %d, want 9 (the highest episode number)", got)
	}

	// Rows without usable numbers fall back to the row count.
	unnumbered := []malhtml.Episode{{}, {}, {}}
	if got := airedEpisodeCount(unnumbered); got != 3 {
		t.Fatalf("airedEpisodeCount = %d, want 3 (the row count)", got)
	}

	if got := airedEpisodeCount(nil); got != 0 {
		t.Fatalf("airedEpisodeCount(nil) = %d, want 0", got)
	}
}

// TestAiredSummaryNeverClaimsAFractionOfAnUnknownTotal is the regression guard
// for "aired so far: 12 of 0 announced episodes", which the detail page's
// unknown episode total used to produce.
func TestAiredSummaryNeverClaimsAFractionOfAnUnknownTotal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		announced, reached int
		want               string
	}{
		{"unknown announced total", 0, 12, "aired so far: 12 episodes (announced total unknown)"},
		{"partly aired", 19, 16, "aired so far: 16 of 19 announced episodes"},
		{"fully aired", 19, 19, ""},
		{"nothing reached", 19, 0, ""},
		{"both unknown", 0, 0, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := airedSummary(tc.announced, tc.reached)
			if got != tc.want {
				t.Fatalf("airedSummary(%d, %d) = %q, want %q", tc.announced, tc.reached, got, tc.want)
			}
			if strings.Contains(got, "of 0 ") {
				t.Fatalf("airedSummary(%d, %d) = %q claims a fraction of an unknown total", tc.announced, tc.reached, got)
			}
		})
	}
}

// TestAdaptationNoteIsHonestAboutWhatWasComputed keeps the note from claiming an
// estimate that was never computed.
func TestAdaptationNoteIsHonestAboutWhatWasComputed(t *testing.T) {
	t.Parallel()

	noBand := adaptationNote(11, false, true, false)
	if !strings.Contains(noBand, "could be estimated") {
		t.Fatalf("note = %q, want it to say the band could not be estimated", noBand)
	}
	if strings.Contains(noBand, "estimate derived") {
		t.Fatalf("note = %q still claims an estimate", noBand)
	}
	if !strings.Contains(noBand, "still airing") {
		t.Fatalf("note = %q, want the still-airing caveat", noBand)
	}

	withBand := adaptationNote(11, true, false, true)
	if !strings.Contains(withBand, "estimate derived") {
		t.Fatalf("note = %q, want the estimate rationale when a band was computed", withBand)
	}
	if !strings.Contains(withBand, "still publishing") {
		t.Fatalf("note = %q, want the still-publishing caveat", withBand)
	}
	if strings.Contains(withBand, "still airing") {
		t.Fatalf("note = %q, want no still-airing caveat for a finished anime", withBand)
	}

	unknownChapters := "the manga entry publishes no chapter count, so only the link between the two entries is known"
	if got := adaptationNote(0, false, true, true); got != unknownChapters {
		t.Fatalf("note = %q, want %q", got, unknownChapters)
	}
}
