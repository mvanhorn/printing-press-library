// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNovelCircuitHistoryHelpWires smoke-tests that the circuit-history command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCircuitHistoryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"circuit-history", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("circuit-history --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "circuit-history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("circuit-history --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelCircuitHistoryReturnsClassificationFailure(t *testing.T) {
	withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/results/seasons":
			fmt.Fprint(w, `[{"id":"season-2024","year":2024},{"id":"season-2023","year":2023}]`)
		case "/results/events":
			seasonID := r.URL.Query().Get("seasonUuid")
			fmt.Fprintf(w, `[{"id":"%s-mugello","name":"Italian GP","date_start":"2024-06-01","circuit":{"name":"Mugello"}}]`, seasonID)
		case "/results/categories":
			fmt.Fprint(w, `[{"id":"motogp","name":"MotoGP™"}]`)
		case "/results/sessions":
			eventID := r.URL.Query().Get("eventUuid")
			fmt.Fprintf(w, `[{"id":"%s-race","type":"RAC"}]`, eventID)
		case "/results/session/season-2024-mugello-race/classification":
			fmt.Fprint(w, `{"classification":[{"position":1,"rider":{"id":"rider-1","full_name":"Rider One"},"team":{"name":"Team One"}}]}`)
		case "/results/session/season-2023-mugello-race/classification":
			http.Error(w, `classification unavailable`, http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MOTOGP_BASE_URL", server.URL)

	_, stderr, err := runRootArgs(t, "--no-cache", "circuit-history", "mugello", "motogp", "--seasons", "2")
	if err == nil {
		t.Fatal("circuit-history returned partial success after a classification failure")
	}
	if !strings.Contains(err.Error(), "fetching race classification for 2023") {
		t.Fatalf("error = %q, want 2023 classification context (stderr=%q)", err, stderr)
	}
}
