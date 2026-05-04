package omdb

import (
	"testing"
)

func TestFetchEmptyAPIKey(t *testing.T) {
	cases := []struct {
		name   string
		imdbID string
		apiKey string
	}{
		{"empty key", "tt0137523", ""},
		{"empty key + empty id", "", ""},
		{"key set, empty id", "", "anykey"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Fetch(tc.imdbID, tc.apiKey)
			if err != nil {
				t.Fatalf("Fetch(%q,%q) err = %v, want nil", tc.imdbID, tc.apiKey, err)
			}
			if res != nil {
				t.Fatalf("Fetch(%q,%q) result = %+v, want nil for graceful degradation", tc.imdbID, tc.apiKey, res)
			}
		})
	}
}

func TestRatingBySource(t *testing.T) {
	r := &Result{
		Ratings: []Rating{
			{Source: "Internet Movie Database", Value: "8.8/10"},
			{Source: "Rotten Tomatoes", Value: "79%"},
			{Source: "Metacritic", Value: "66/100"},
		},
	}
	cases := []struct {
		source string
		want   string
	}{
		{"Internet Movie Database", "8.8/10"},
		{"Rotten Tomatoes", "79%"},
		{"Metacritic", "66/100"},
		{"Missing", ""},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := r.RatingBySource(tc.source)
			if got != tc.want {
				t.Errorf("RatingBySource(%q) = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}

func TestRatingBySourceNilReceiver(t *testing.T) {
	var r *Result
	if got := r.RatingBySource("Anything"); got != "" {
		t.Errorf("nil.RatingBySource(...) = %q, want empty", got)
	}
}
