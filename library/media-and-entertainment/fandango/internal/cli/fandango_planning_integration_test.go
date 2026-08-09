// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestFandangoPlanningCommandsAgainstContractServer(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Fandango/Showtimes" {
			t.Errorf("request path = %q, want /Fandango/Showtimes", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("subscription-Key"); got != "licensed-test-key" {
			t.Errorf("subscription-Key = %q, want configured credential", got)
		}
		if r.URL.Query().Get("Limit") != "100" {
			t.Errorf("Limit = %q, want 100", r.URL.Query().Get("Limit"))
		}

		start := "2026-08-10T19:30:00"
		if r.URL.Query().Get("StartDateTime") != "" {
			start = now.Add(30 * time.Minute).Format(time.RFC3339)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":{"showtimes":[{"id":"show-1","movieId":"movie-1","movieTitle":"Example Movie","theaterId":"%s","theaterName":"Example Cinema","displayDate":"2026-08-10","formatName":"IMAX","distance":2.5,"dateTime":{"local":"%s"},"links":[{"rel":"purchase","href":"https://tickets.example/show-1"}]}]}}`, defaultString(r.URL.Query().Get("TheaterId"), "theater-1"), start)
	}))
	defer server.Close()

	t.Setenv("FANDANGO_BASE_URL", server.URL)
	t.Setenv("FANDANGO_SUBSCRIPTION_KEY", "licensed-test-key")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	tests := []struct {
		name string
		new  func(*rootFlags) *cobra.Command
		args []string
		want []string
	}{
		{
			name: "movie plan filters and returns checkout link",
			new:  newNovelMoviePlanCmd,
			args: []string{"--zip-code", "10001", "--date", "2026-08-10", "--after", "18:00", "--before", "22:00"},
			want: []string{`"count": 1`, `https://tickets.example/show-1`},
		},
		{
			name: "starting soon uses bounded window",
			new:  newNovelStartingSoonCmd,
			args: []string{"--zip-code", "10001", "--within", "90m"},
			want: []string{`"count": 1`, `"within": "1h30m0s"`},
		},
		{
			name: "format find groups presentation formats",
			new:  newNovelFormatFindCmd,
			args: []string{"--movie-id", "movie-1", "--id-provider", "fandangoApi", "--zip-code", "10001"},
			want: []string{`"formats"`, `"IMAX"`},
		},
		{
			name: "theater compare aggregates each requested theater",
			new:  newNovelTheaterCompareCmd,
			args: []string{"--theater-ids", "theater-1,theater-2", "--date", "2026-08-10"},
			want: []string{`"theater_id": "theater-1"`, `"theater_id": "theater-2"`},
		},
		{
			name: "movie availability groups theater and date",
			new:  newNovelMovieAvailabilityCmd,
			args: []string{"--movie-id", "movie-1", "--id-provider", "fandangoApi", "--zip-code", "10001"},
			want: []string{`"availability"`, `2026-08-10|Example Cinema`},
		},
		{
			name: "watchlist matches movie titles",
			new:  newNovelWatchlistShowtimesCmd,
			args: []string{"--zip-code", "10001", "--movies", "Example Movie,Other Movie"},
			want: []string{`"count": 1`, `"title": "Example Movie"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, noCache: true, noLearn: true}
			cmd := tc.new(flags)
			cmd.SetArgs(tc.args)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v\n%s", err, output.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("output missing %q:\n%s", want, output.String())
				}
			}
		})
	}
}
