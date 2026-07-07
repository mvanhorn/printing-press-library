// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the Tripadvisor transcendence helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestTAParseDetail(t *testing.T) {
	raw := json.RawMessage(`{
		"location_id": "93450",
		"name": "The Plaza",
		"rating": "4.5",
		"num_reviews": "1,058",
		"price_level": "$$$$",
		"web_url": "https://example.com/x",
		"ranking_data": {"ranking_string": "#4 of 82 hotels in New York City", "ranking": "4", "ranking_out_of": 82},
		"address_obj": {"address_string": "768 5th Ave, New York City, NY"},
		"trip_types": [
			{"name": "families", "localized_name": "Families", "value": "120"},
			{"name": "couples", "localized_name": "Couples", "value": "300"}
		],
		"subratings": {"0": {"name": "service", "localized_name": "Service", "value": "4.7"}}
	}`)
	d := taParseDetail(raw)
	if d.Rating != 4.5 {
		t.Errorf("rating: got %v want 4.5", d.Rating)
	}
	if d.NumReviews != 1058 {
		t.Errorf("num_reviews: got %d want 1058 (comma-stripped)", d.NumReviews)
	}
	if d.Ranking != 4 {
		t.Errorf("ranking: got %d want 4 (parsed from quoted string)", d.Ranking)
	}
	if d.RankingOutOf != 82 {
		t.Errorf("ranking_out_of: got %d want 82 (parsed from number)", d.RankingOutOf)
	}
	if d.TripTypes["families"] != 120 || d.TripTypes["couples"] != 300 {
		t.Errorf("trip_types: got %v", d.TripTypes)
	}
	if d.Subratings["Service"] != 4.7 {
		t.Errorf("subratings: got %v", d.Subratings)
	}
}

func TestTAParseStubs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"data-wrapper", `{"data":[{"location_id":"1","name":"A"},{"location_id":"2","name":"B"}]}`, 2},
		{"bare-array", `[{"location_id":"9","name":"Z"}]`, 1},
		{"drops-empty-id", `{"data":[{"name":"no id"},{"location_id":"3","name":"C"}]}`, 1},
		{"empty", `{"data":[]}`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := taParseStubs(json.RawMessage(c.raw))
			if len(got) != c.want {
				t.Errorf("got %d stubs want %d", len(got), c.want)
			}
		})
	}
}

func TestTAParseIntRaw(t *testing.T) {
	cases := map[string]int{`4`: 4, `"7"`: 7, `"1,234"`: 1234, `null`: 0, `"x"`: 0}
	for in, want := range cases {
		if got := taParseIntRaw(json.RawMessage(in)); got != want {
			t.Errorf("taParseIntRaw(%s): got %d want %d", in, got, want)
		}
	}
}

func TestTASortDetails(t *testing.T) {
	mk := func(id string, rating float64, reviews, ranking int) taDetail {
		return taDetail{LocationID: id, Rating: rating, NumReviews: reviews, Ranking: ranking}
	}
	t.Run("rating-desc", func(t *testing.T) {
		items := []taDetail{mk("a", 4.0, 10, 5), mk("b", 4.8, 2, 9), mk("c", 4.5, 50, 1)}
		taSortDetails(items, "rating")
		if items[0].LocationID != "b" || items[2].LocationID != "a" {
			t.Errorf("rating order wrong: %v", ids(items))
		}
	})
	t.Run("reviews-desc", func(t *testing.T) {
		items := []taDetail{mk("a", 4.0, 10, 5), mk("b", 4.8, 2, 9), mk("c", 4.5, 50, 1)}
		taSortDetails(items, "reviews")
		if items[0].LocationID != "c" {
			t.Errorf("reviews order wrong: %v", ids(items))
		}
	})
	t.Run("ranking-asc-zero-last", func(t *testing.T) {
		items := []taDetail{mk("a", 4.0, 10, 0), mk("b", 4.8, 2, 9), mk("c", 4.5, 50, 1)}
		taSortDetails(items, "ranking")
		if items[0].LocationID != "c" || items[2].LocationID != "a" {
			t.Errorf("ranking order wrong (0 should sort last): %v", ids(items))
		}
	})
}

func TestTAExtractDataArray(t *testing.T) {
	if got := taExtractDataArray(json.RawMessage(`{"data":[1,2,3]}`)); len(got) != 3 {
		t.Errorf("data-wrapper: got %d want 3", len(got))
	}
	if got := taExtractDataArray(json.RawMessage(`garbage`)); got == nil || len(got) != 0 {
		t.Errorf("garbage should yield non-nil empty slice, got %v", got)
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{1.234: 1.23, 1.235: 1.24, -0.126: -0.13, 0: 0}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v): got %v want %v", in, got, want)
		}
	}
}

func ids(items []taDetail) []string {
	out := make([]string, len(items))
	for i, d := range items {
		out[i] = d.LocationID
	}
	return out
}
