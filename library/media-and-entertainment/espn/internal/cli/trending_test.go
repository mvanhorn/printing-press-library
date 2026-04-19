package cli

import "testing"

func TestParseTrendingNowShape(t *testing.T) {
	body := []byte(`{"headlines":[
		{"headline":"Lakers stun Celtics","section":"NBA","published":"2026-04-19T22:00:00Z"},
		{"headline":"NFL draft preview","section":"NFL","published":"2026-04-18T12:00:00Z"}
	]}`)
	got := parseTrending(body)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Name != "Lakers stun Celtics" || got[0].League != "NBA" || got[0].EntityType != "story" {
		t.Errorf("unexpected first entry: %+v", got[0])
	}
}

func TestParseTrendingItemsShape(t *testing.T) {
	body := []byte(`{"items":[{"name":"Trout","entity_type":"athlete","league":"mlb"}]}`)
	got := parseTrending(body)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

func TestParseTrendingArrayFallback(t *testing.T) {
	body := []byte(`[{"entity_type":"athlete","name":"Mahomes","league":"nfl"}]`)
	got := parseTrending(body)
	if len(got) != 1 || got[0].Name != "Mahomes" {
		t.Fatalf("unexpected fallback parse: %+v", got)
	}
}

func TestParseTrendingMalformed(t *testing.T) {
	body := []byte(`<html>not json</html>`)
	got := parseTrending(body)
	if got != nil {
		t.Errorf("malformed input should yield nil, got %v", got)
	}
}

func TestParseTrendingDisplayNameFallback(t *testing.T) {
	body := []byte(`[{"displayName":"LeBron","type":"athlete"}]`)
	got := parseTrending(body)
	if len(got) != 1 || got[0].Name != "LeBron" || got[0].EntityType != "athlete" {
		t.Errorf("unexpected fallback parse: %+v", got)
	}
}
