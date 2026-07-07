package roadside

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildNominatimURL(t *testing.T) {
	u := BuildNominatimURL("Austin, TX")
	if !strings.HasPrefix(u, NominatimBaseURL) {
		t.Errorf("url should start with base, got %q", u)
	}
	for _, want := range []string{"format=jsonv2", "countrycodes=us%2Cca", "limit=1", "Austin"} {
		if !strings.Contains(u, want) {
			t.Errorf("url %q missing %q", u, want)
		}
	}
}

func TestParseNominatimResponse(t *testing.T) {
	body := []byte(`[{"lat":"30.2711286","lon":"-97.7436995","display_name":"Austin, Travis County, Texas, United States"}]`)
	lat, lng, display, err := ParseNominatimResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lat < 30.2 || lat > 30.3 {
		t.Errorf("lat out of range: %v", lat)
	}
	if lng > -97.7 || lng < -97.8 {
		t.Errorf("lng out of range: %v", lng)
	}
	if !strings.Contains(display, "Austin") {
		t.Errorf("display: got %q", display)
	}
}

func TestParseNominatimResponseEmpty(t *testing.T) {
	_, _, _, err := ParseNominatimResponse([]byte(`[]`))
	if !errors.Is(err, ErrPlaceNotFound) {
		t.Errorf("empty array should return ErrPlaceNotFound, got %v", err)
	}
}

func TestMilesToDelta(t *testing.T) {
	// 25 mi should produce a small positive delta well under the 5-degree cap.
	d := MilesToDelta(25)
	if d <= 0 || d > 1 {
		t.Errorf("MilesToDelta(25) = %v, want a small positive value", d)
	}
	// Non-positive radius falls back to the 25-mile default.
	if MilesToDelta(0) != MilesToDelta(25) {
		t.Errorf("MilesToDelta(0) should equal MilesToDelta(25)")
	}
}
