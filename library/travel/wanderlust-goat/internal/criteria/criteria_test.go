package criteria

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
)

func TestParseKissaten(t *testing.T) {
	m := Parse("vintage jazz kissaten, no tourists, great pour-over")
	if m.Intent != sources.IntentDrinks && m.Intent != sources.IntentCoffee {
		t.Errorf("expected coffee or drinks intent, got %q", m.Intent)
	}
	hasJazzTag := false
	hasCafeTag := false
	for _, tag := range m.OSMTags {
		if tag.Key == "music:jazz" {
			hasJazzTag = true
		}
		if tag.Key == "amenity" && tag.Value == "cafe" {
			hasCafeTag = true
		}
	}
	if !hasJazzTag && !hasCafeTag {
		t.Error("expected music:jazz or amenity=cafe tag")
	}
	hasNoTourists := false
	for _, neg := range m.NegationKW {
		if neg == "no tourists" {
			hasNoTourists = true
		}
	}
	if !hasNoTourists {
		t.Error("expected 'no tourists' negation")
	}
}

func TestParseViewpoint(t *testing.T) {
	m := Parse("hidden viewpoint photographers know about, blue hour")
	if m.Intent != sources.IntentViewpoint {
		t.Errorf("expected viewpoint intent, got %q", m.Intent)
	}
}

func TestParseFoodFallback(t *testing.T) {
	m := Parse("something tasty for lunch")
	if m.Intent != sources.IntentFood {
		t.Errorf("expected food (default), got %q", m.Intent)
	}
}

func TestQuietSignalsReturnsCopy(t *testing.T) {
	a := QuietSignals()
	b := QuietSignals()
	if &a[0] == &b[0] {
		t.Error("QuietSignals must return a copy, got shared backing array")
	}
}
