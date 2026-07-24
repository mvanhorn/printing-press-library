// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChooseFilmPrefersExactThenContains(t *testing.T) {
	films := []movieGluFilm{{FilmID: 1, FilmName: "Superman"}, {FilmID: 2, FilmName: "Superman Returns"}}
	if got, _ := chooseFilm(films, "Superman Returns"); got.FilmID != 2 {
		t.Fatalf("exact match film = %d, want 2", got.FilmID)
	}
	if got, _ := chooseFilm(films, "returns"); got.FilmID != 2 {
		t.Fatalf("contains match film = %d, want 2", got.FilmID)
	}
}

func TestFlattenMovieNightOptionsFiltersAndRanks(t *testing.T) {
	cinemas := []movieGluCinema{
		{CinemaID: 2, CinemaName: "Far", Distance: 5, Showings: map[string]struct {
			Times []movieGluTime `json:"times"`
		}{"IMAX": {Times: []movieGluTime{{StartTime: "20:00"}}}}},
		{CinemaID: 1, CinemaName: "Near", Distance: 1, Showings: map[string]struct {
			Times []movieGluTime `json:"times"`
		}{"Standard": {Times: []movieGluTime{{StartTime: "18:00"}, {StartTime: "21:00"}}}}},
	}
	got := flattenMovieNightOptions(cinemas, 19*60)
	if len(got) != 2 || got[0].CinemaID != 1 || got[0].StartTime != "21:00" || got[1].CinemaID != 2 {
		t.Fatalf("flattenMovieNightOptions() = %#v", got)
	}
}

func TestMovieNightBookingWorkflowHTTPContract(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		for key, want := range map[string]string{
			"x-api-key":     "api-key",
			"client":        "evaluation-user",
			"authorization": "Basic abc123",
			"territory":     "US",
			"api-version":   "v200",
			"geolocation":   "40.7128;-74.0060",
		} {
			if got := r.Header.Get(key); got != want {
				t.Errorf("header %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/filmsNowShowing/":
			json.NewEncoder(w).Encode(map[string]any{"films": []map[string]any{{"film_id": 77, "film_name": "The Test Movie"}}})
		case "/filmShowTimes/":
			json.NewEncoder(w).Encode(map[string]any{"cinemas": []map[string]any{{
				"cinema_id": 9, "cinema_name": "Downtown Cinema", "distance": 1.2,
				"showings": map[string]any{"IMAX": map[string]any{"times": []map[string]any{{"start_time": "20:15", "end_time": "22:30"}}}},
			}}})
		case "/purchaseConfirmation/":
			json.NewEncoder(w).Encode(map[string]any{"url": "https://cinema.example/checkout/abc"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("MOVIEGLU_BASE_URL", server.URL)
	t.Setenv("MOVIEGLU_CREDENTIALS", "api-key")
	t.Setenv("MOVIEGLU_CLIENT", "evaluation-user")
	t.Setenv("MOVIEGLU_AUTHORIZATION", "Basic abc123")
	t.Setenv("MOVIEGLU_TERRITORY", "US")
	t.Setenv("MOVIEGLU_GEOLOCATION", "40.7128;-74.0060")
	t.Setenv("MOVIEGLU_HOME", t.TempDir())

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"movie-night", "Test Movie", "--date", "2026-07-24", "--after", "19:00", "--booking-link", "--json", "--no-cache"})
	if err := root.Execute(); err != nil {
		t.Fatalf("movie-night execution error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"booking_url": "https://cinema.example/checkout/abc"`) {
		t.Fatalf("output missing booking URL: %s", stdout.String())
	}
	wantPaths := []string{
		"/filmsNowShowing/?n=25",
		"/filmShowTimes/?date=2026-07-24&film_id=77&n=25",
		"/purchaseConfirmation/?cinema_id=9&date=2026-07-24&film_id=77&time=20%3A15",
	}
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("request paths = %#v, want %#v", paths, wantPaths)
	}
}
