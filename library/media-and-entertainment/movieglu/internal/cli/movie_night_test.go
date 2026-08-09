// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/store"
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

func TestMovieNightLocalSourceNeverCallsMovieGlu(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "unexpected live request", http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("MOVIEGLU_BASE_URL", server.URL)
	t.Setenv("MOVIEGLU_CREDENTIALS", "api-key")
	t.Setenv("MOVIEGLU_CLIENT", "evaluation-user")
	t.Setenv("MOVIEGLU_AUTHORIZATION", "Basic abc123")
	t.Setenv("MOVIEGLU_TERRITORY", "US")
	t.Setenv("MOVIEGLU_GEOLOCATION", "")
	t.Setenv("MOVIEGLU_HOME", t.TempDir())

	root := RootCmd()
	root.SetArgs([]string{"movie-night", "Test Movie", "--date", "2026-07-24", "--data-source", "local", "--json"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no local data") {
		t.Fatalf("local movie-night error = %v, want missing local-data guidance", err)
	}
	if calls != 0 {
		t.Fatalf("local movie-night made %d live request(s), want 0", calls)
	}
}

func TestMovieNightAutoLocalFallbackPreservesResultWhenBookingLinkRequested(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "unexpected live request", http.StatusInternalServerError)
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("MOVIEGLU_HOME", home)
	t.Setenv("MOVIEGLU_BASE_URL", server.URL)
	t.Setenv("MOVIEGLU_CREDENTIALS", "")
	t.Setenv("MOVIEGLU_CLIENT", "")
	t.Setenv("MOVIEGLU_AUTHORIZATION", "")
	t.Setenv("MOVIEGLU_TERRITORY", "")
	t.Setenv("MOVIEGLU_GEOLOCATION", "")

	db, err := store.Open(defaultDBPath("movieglu-pp-cli"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	if err := db.Upsert("films-now-showing", "77", json.RawMessage(`{"film_id":77,"film_name":"The Local Movie"}`)); err != nil {
		db.Close()
		t.Fatalf("Upsert(film) error = %v", err)
	}
	showtimes := json.RawMessage(`{"cinema_id":9,"cinema_name":"Local Cinema","distance":1.2,"_pp_film_id":"77","_pp_show_date":"2026-07-24","showings":{"IMAX":{"times":[{"start_time":"20:15","end_time":"22:30"}]}}}`)
	if err := db.Upsert("film-show-times", "9", showtimes); err != nil {
		db.Close()
		t.Fatalf("Upsert(showtimes) error = %v", err)
	}
	for id, unrelated := range map[string]json.RawMessage{
		"10": json.RawMessage(`{"cinema_id":10,"cinema_name":"Wrong Film Cinema","distance":0.1,"_pp_film_id":"88","_pp_show_date":"2026-07-24","showings":{"IMAX":{"times":[{"start_time":"19:00"}]}}}`),
		"11": json.RawMessage(`{"cinema_id":11,"cinema_name":"Wrong Date Cinema","distance":0.2,"_pp_film_id":"77","_pp_show_date":"2026-07-25","showings":{"IMAX":{"times":[{"start_time":"19:15"}]}}}`),
	} {
		if err := db.Upsert("film-show-times", id, unrelated); err != nil {
			db.Close()
			t.Fatalf("Upsert(unrelated showtimes %s) error = %v", id, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"movie-night", "Local Movie", "--date", "2026-07-24", "--booking-link", "--json", "--no-cache"})
	if err := root.Execute(); err != nil {
		t.Fatalf("movie-night local fallback error = %v, stderr = %s", err, stderr.String())
	}
	if calls != 0 {
		t.Fatalf("local fallback made %d live request(s), want 0", calls)
	}
	for _, want := range []string{`"source": "local"`, `"film_name": "The Local Movie"`, `"booking_link_unavailable"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %s: %s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `"booking_url"`) {
		t.Fatalf("local fallback unexpectedly returned booking URL: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Wrong Film Cinema") || strings.Contains(stdout.String(), "Wrong Date Cinema") {
		t.Fatalf("local fallback mixed unrelated showtimes: %s", stdout.String())
	}
}

func TestFilterLocalMovieNightCinemasUsesExactFilmAndDateScope(t *testing.T) {
	rows := []movieGluCinema{
		{CinemaID: 1, CinemaName: "Correct", FilmID: "77", ShowDate: "2026-07-24"},
		{CinemaID: 2, CinemaName: "Wrong Film", FilmID: "88", ShowDate: "2026-07-24"},
		{CinemaID: 3, CinemaName: "Wrong Date", FilmID: "77", ShowDate: "2026-07-25"},
		{CinemaID: 4, CinemaName: "Legacy Unscoped"},
	}
	got := filterLocalMovieNightCinemas(rows, 77, "2026-07-24")
	if len(got) != 1 || got[0].CinemaID != 1 {
		t.Fatalf("filterLocalMovieNightCinemas() = %#v, want only exact film/date row", got)
	}
}
