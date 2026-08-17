// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the stateful commands' pure decision logic (2f): watch's
// target/exit rule, drift's trend aggregates, compare's delta verdict, saved's
// nights parsing, plus the booking-URL invariant.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
)

// watch: no target always hits; a target hits only on a real total at/below it;
// a missing or zero quote must never read as "target reached".
func TestWatchTargetHit(t *testing.T) {
	cases := []struct {
		name    string
		target  float64
		total   float64
		haveOff bool
		want    bool
	}{
		{"no target always hits", 0, 300, true, true},
		{"no target, no offer does NOT hit", 0, 0, false, false},
		{"below target hits", 250, 200, true, true},
		{"exactly at target hits", 250, 250, true, true},
		{"above target misses", 250, 300, true, false},
		{"no offer misses positive target", 250, 0, false, false},
		{"zero price misses positive target", 250, 0, true, false},
	}
	for _, c := range cases {
		if got := watchTargetHit(c.target, c.total, c.haveOff); got != c.want {
			t.Errorf("%s: watchTargetHit(%v,%v,%v)=%v, want %v", c.name, c.target, c.total, c.haveOff, got, c.want)
		}
	}
}

func pt(total float64) driftPoint { return driftPoint{Total: total} }

// drift: aggregates track the series; direction respects a one-cent deadband.
func TestSummarizeDrift(t *testing.T) {
	// Empty series.
	empty := summarizeDrift(nil)
	if empty.Direction != "flat" || empty.First != 0 || empty.Last != 0 {
		t.Errorf("empty drift should be flat/zero, got %+v", empty)
	}

	// Rising series: 100 → 130, dipping to 90 in the middle.
	up := summarizeDrift([]driftPoint{pt(100), pt(90), pt(130)})
	if up.First != 100 || up.Last != 130 || up.Min != 90 || up.Max != 130 {
		t.Errorf("rising aggregates wrong: %+v", up)
	}
	if up.Change != 30 || up.Direction != "up" {
		t.Errorf("rising direction/change wrong: change=%v dir=%s", up.Change, up.Direction)
	}

	// Falling series.
	down := summarizeDrift([]driftPoint{pt(200), pt(150)})
	if down.Direction != "down" || down.Change != -50 {
		t.Errorf("falling should be down/-50, got dir=%s change=%v", down.Direction, down.Change)
	}

	// Within the deadband → flat, not a spurious move.
	flat := summarizeDrift([]driftPoint{pt(100), pt(100.005)})
	if flat.Direction != "flat" {
		t.Errorf("sub-cent change should stay flat, got %s (change=%v)", flat.Direction, flat.Change)
	}
}

func offerURL(total float64, u string) *carsource.Offer {
	return &carsource.Offer{Total: total, Currency: "EUR", URL: u}
}

// compare: a pricier direct quote means the aggregator is cheaper; a cent-level
// gap is a tie; a missing side is unknown.
func TestCompareDelta(t *testing.T) {
	// Direct pricier → aggregator cheaper, positive delta.
	if d, c := compareDelta(offerURL(180, "https://a"), offerURL(200, "https://b")); c != "aggregator" || d != 20 {
		t.Errorf("direct pricier should be aggregator/+20, got %s/%v", c, d)
	}
	// Direct cheaper → negative delta.
	if d, c := compareDelta(offerURL(200, "https://a"), offerURL(180, "https://b")); c != "direct" || d != -20 {
		t.Errorf("direct cheaper should be direct/-20, got %s/%v", c, d)
	}
	// Cent-level gap → tie.
	if _, c := compareDelta(offerURL(200, ""), offerURL(200.005, "")); c != "tie" {
		t.Errorf("sub-cent gap should tie, got %s", c)
	}
	// Missing side → unknown.
	if _, c := compareDelta(nil, offerURL(200, "")); c != "unknown" {
		t.Errorf("nil aggregator should be unknown, got %s", c)
	}
}

// saved: nights parsing defaults to a week and rejects non-positive input.
func TestParseNightsFlag(t *testing.T) {
	if n, err := parseNightsFlag(""); err != nil || n != 7 {
		t.Errorf("empty nights should default to 7, got %d/%v", n, err)
	}
	if n, err := parseNightsFlag("3"); err != nil || n != 3 {
		t.Errorf("\"3\" should parse to 3, got %d/%v", n, err)
	}
	for _, bad := range []string{"0", "-1", "abc", "3.5"} {
		if _, err := parseNightsFlag(bad); err == nil {
			t.Errorf("nights %q should error", bad)
		}
	}
}

// booking-URL invariant: every direct offer must carry a valid https link, or
// the `direct` "Book at:" footer silently blanks.
func TestCheckBookingURLs(t *testing.T) {
	good := airportProbe{IATA: "AGP", Offers: map[string][]carsource.Offer{
		"Clickrent": {{Total: 171, URL: "https://clickrent.es"}},
		"Centauro":  {{Total: 215, URL: "https://www.centauro.net"}},
	}}
	if r := checkBookingURLs(good); r.Status != selftestPass {
		t.Errorf("valid https URLs should pass, got %s: %s", r.Status, r.Detail)
	}

	// A dropped URL fails.
	missing := airportProbe{IATA: "AGP", Offers: map[string][]carsource.Offer{
		"Goldcar": {{Total: 247, URL: ""}},
	}}
	if r := checkBookingURLs(missing); r.Status != selftestFail {
		t.Errorf("empty URL should fail, got %s", r.Status)
	}

	// A non-https scheme fails.
	insecure := airportProbe{IATA: "AGP", Offers: map[string][]carsource.Offer{
		"Delpaso": {{Total: 176, URL: "ftp://delpasocarhire.com"}},
	}}
	if r := checkBookingURLs(insecure); r.Status != selftestFail {
		t.Errorf("non-https URL should fail, got %s", r.Status)
	}

	// No offers → skip (reachability owns that failure).
	if r := checkBookingURLs(airportProbe{IATA: "AGP", Offers: map[string][]carsource.Offer{}}); r.Status != selftestSkip {
		t.Errorf("no offers should skip, got %s", r.Status)
	}
}

// searchKey must treat the drop-off location as part of a route's identity: a
// one-way rental (different drop-off) must never share a price-history key with
// the round-trip. Regression for the snapshot-collision bug.
func TestSearchKeyDistinguishesDropoff(t *testing.T) {
	roundTrip := searchKey("AGP", "", "20/08/2026", "27/08/2026", 35)
	oneWay := searchKey("AGP", "MAD", "20/08/2026", "27/08/2026", 35)
	if roundTrip == oneWay {
		t.Errorf("round-trip and one-way must not share a key (both %q)", roundTrip)
	}
	if searchKey("AGP", "BCN", "20/08/2026", "27/08/2026", 35) == oneWay {
		t.Error("different drop-off must produce a different key")
	}
	if searchKey("AGP", "MAD", "20/08/2026", "27/08/2026", 35) != oneWay {
		t.Error("searchKey must be stable for identical inputs")
	}
}
