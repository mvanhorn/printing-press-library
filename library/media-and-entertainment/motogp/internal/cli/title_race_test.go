// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNovelTitleRaceHelpWires smoke-tests that the title-race command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTitleRaceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"title-race", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("title-race --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "title-race"} {
		if !strings.Contains(help, want) {
			t.Fatalf("title-race --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelTitleRaceIncludesSprintAndRacePoints(t *testing.T) {
	withTempLearnHome(t)
	var sessionsRequests int
	var sprintRequests int
	var raceRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/results/seasons":
			fmt.Fprint(w, `[{"id":"season-2024","year":2024}]`)
		case "/results/categories":
			fmt.Fprint(w, `[{"id":"motogp","name":"MotoGP™"}]`)
		case "/results/events":
			fmt.Fprint(w, `[`+
				`{"id":"opening-no-race","name":"Opening Round","date_start":"2024-05-01"},`+
				`{"id":"mugello","name":"Italian GP","date_start":"2024-06-01"}`+
				`]`)
		case "/results/sessions":
			sessionsRequests++
			if r.URL.Query().Get("eventUuid") == "opening-no-race" {
				fmt.Fprint(w, `[{"id":"opening-fp1","type":"FP","number":1}]`)
			} else {
				fmt.Fprint(w, `[{"id":"mugello-sprint","type":"SPR"},{"id":"mugello-race","type":"RAC"}]`)
			}
		case "/results/session/mugello-sprint/classification":
			sprintRequests++
			fmt.Fprint(w, `{"classification":[{"position":1,"points":12,"rider":{"id":"pecco","full_name":"Francesco Bagnaia"}}]}`)
		case "/results/session/mugello-race/classification":
			raceRequests++
			fmt.Fprint(w, `{"classification":[{"position":1,"points":25,"rider":{"id":"pecco","full_name":"Francesco Bagnaia"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MOTOGP_BASE_URL", server.URL)

	stdout, stderr, err := runRootArgs(t, "--no-cache", "title-race", "2024", "motogp")
	if err != nil {
		t.Fatalf("title-race: %v (stderr=%q)", err, stderr)
	}
	var out struct {
		Rounds []struct {
			Round        int            `json:"round"`
			Winner       string         `json:"winner"`
			LeaderPoints int            `json:"leader_points"`
			Standings    map[string]int `json:"standings"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("decode output: %v (stdout=%q)", err, stdout)
	}
	if len(out.Rounds) != 1 {
		t.Fatalf("round count = %d, want 1", len(out.Rounds))
	}
	round := out.Rounds[0]
	if round.Round != 1 || round.Winner != "Francesco Bagnaia" {
		t.Errorf("round identity = %#v, want round 1 won by Francesco Bagnaia", round)
	}
	if round.LeaderPoints != 37 || round.Standings["Francesco Bagnaia"] != 37 {
		t.Errorf("combined sprint+race points = %#v, want 37", round)
	}
	if sessionsRequests != 2 || sprintRequests != 1 || raceRequests != 1 {
		t.Errorf("request counts sessions/sprint/race = %d/%d/%d, want 2/1/1", sessionsRequests, sprintRequests, raceRequests)
	}
}

func TestNovelTitleRaceReturnsClassificationFailure(t *testing.T) {
	withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/results/seasons":
			fmt.Fprint(w, `[{"id":"season-2024","year":2024}]`)
		case "/results/categories":
			fmt.Fprint(w, `[{"id":"motogp","name":"MotoGP™"}]`)
		case "/results/events":
			fmt.Fprint(w, `[`+
				`{"id":"round-1","name":"Round One","date_start":"2024-01-01"},`+
				`{"id":"round-2","name":"Round Two","date_start":"2024-02-01"}`+
				`]`)
		case "/results/sessions":
			eventID := r.URL.Query().Get("eventUuid")
			fmt.Fprintf(w, `[{"id":"%s-race","type":"RAC"}]`, eventID)
		case "/results/session/round-1-race/classification":
			fmt.Fprint(w, `{"classification":[{"position":1,"points":25,"rider":{"id":"rider-1","full_name":"Rider One"}}]}`)
		case "/results/session/round-2-race/classification":
			http.Error(w, `classification unavailable`, http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MOTOGP_BASE_URL", server.URL)

	_, stderr, err := runRootArgs(t, "--no-cache", "title-race", "2024", "motogp", "--rounds", "2")
	if err == nil {
		t.Fatal("title-race returned success after an intermediate classification failure")
	}
	if !strings.Contains(err.Error(), "fetching race classification for Round Two") {
		t.Fatalf("error = %q, want Round Two classification context (stderr=%q)", err, stderr)
	}
}
