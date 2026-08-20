package safari

import (
	"math"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func TestSafariEpochRoundTrip(t *testing.T) {
	cases := []time.Time{
		time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC),
		time.Date(2010, 6, 15, 12, 30, 0, 999000000, time.UTC),
	}
	for _, tc := range cases {
		raw := timeToSafariSeconds(tc)
		got := source.SafariSecondsToTime(raw)
		delta := got.Sub(tc)
		if math.Abs(delta.Seconds()) > 1e-6 {
			t.Fatalf("round-trip mismatch: got %v want %v", got, tc)
		}
	}
}
