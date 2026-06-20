// internal/provider/nyrr/nyrr_test.go
package nyrr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

var testEvent = domain.Event{
	Provider: "nyrr",
	ID:       "26MINI",
	Name:     "Mastercard Mini 10K",
	Year:     2026,
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	fixture, err := os.ReadFile("../../../testdata/fixtures/nyrr/search.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/runners/finishers-filter" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	return srv
}

func TestLookup_Hit(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	got, err := c.Lookup(context.Background(), testEvent, "19")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Runner != "Sample Runner" {
		t.Errorf("Runner: got %q, want %q", got.Runner, "Sample Runner")
	}
	if got.Bib != "19" {
		t.Errorf("Bib: got %q, want %q", got.Bib, "19")
	}
	if got.OverallPlace != 20 {
		t.Errorf("OverallPlace: got %d, want %d", got.OverallPlace, 20)
	}
	if got.NetTime != "0:33:48" {
		t.Errorf("NetTime: got %q, want %q", got.NetTime, "0:33:48")
	}
	if got.Provider != "nyrr" {
		t.Errorf("Provider: got %q, want %q", got.Provider, "nyrr")
	}
}

func TestLookup_Miss(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	_, err := c.Lookup(context.Background(), testEvent, "999999")
	if !errors.Is(err, provider.ErrBibNotFound) {
		t.Errorf("expected ErrBibNotFound, got: %v", err)
	}
}

func TestSearchByName(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	got, err := c.SearchByName(context.Background(), testEvent, "Runner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one result for 'Runner'")
	}
	for _, r := range got {
		if !strings.Contains(strings.ToLower(r.Runner), "runner") {
			t.Errorf("result Runner %q does not contain 'runner'", r.Runner)
		}
		if r.Bib == "" {
			t.Errorf("result has empty Bib: %+v", r)
		}
	}
}
