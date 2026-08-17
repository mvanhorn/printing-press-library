package cli

import (
	"encoding/json"
	"testing"
)

func TestDiffListings(t *testing.T) {
	prev := json.RawMessage(`{"title":"Game","version":"1.0","price":0,"offersIAP":true,"containsAds":false,"score":4.5,"screenshots":["a","b"]}`)
	latest := json.RawMessage(`{"title":"Game Deluxe","version":"1.1","price":0,"offersIAP":true,"containsAds":true,"score":4.6,"screenshots":["a","b","c"]}`)
	changes := diffListings(prev, latest)
	got := map[string]bool{}
	for _, c := range changes {
		got[c.Field] = true
	}
	for _, want := range []string{"title", "version", "containsAds", "score", "screenshotCount"} {
		if !got[want] {
			t.Errorf("expected change in %q, changes=%+v", want, changes)
		}
	}
	if got["price"] || got["offersIAP"] {
		t.Errorf("unchanged fields should not appear: %+v", changes)
	}
}

func TestDiffListingsNoChange(t *testing.T) {
	same := json.RawMessage(`{"title":"Game","version":"1.0","score":4.5}`)
	if changes := diffListings(same, same); len(changes) != 0 {
		t.Errorf("identical listings should yield no changes, got %+v", changes)
	}
}

func TestNewWatchListingCmd(t *testing.T) {
	cmd := newNovelWatchListingCmd(&rootFlags{})
	if cmd.Use != "watch-listing <appId>" {
		t.Errorf("Use = %q", cmd.Use)
	}
}
