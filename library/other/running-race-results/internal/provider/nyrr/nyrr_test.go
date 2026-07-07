// internal/provider/nyrr/nyrr_test.go
package nyrr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestLookup_PaginatesBibSearch(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/runners/finishers-filter" {
			http.NotFound(w, r)
			return
		}
		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.SearchString != "100" {
			t.Errorf("SearchString: got %q, want %q", req.SearchString, "100")
		}
		if req.PageSize != 100 {
			t.Errorf("PageSize: got %d, want %d", req.PageSize, 100)
		}

		requests++
		w.Header().Set("Content-Type", "application/json")
		switch req.PageIndex {
		case 1:
			items := make([]item, 100)
			for i := range items {
				items[i] = item{
					FirstName:    "Nearby",
					LastName:     fmt.Sprintf("Runner%d", i),
					Bib:          fmt.Sprintf("100%d", i),
					OverallTime:  "4:00:00",
					OverallPlace: i + 1,
				}
			}
			json.NewEncoder(w).Encode(searchResponse{TotalItems: 101, Items: items})
		case 2:
			json.NewEncoder(w).Encode(searchResponse{
				TotalItems: 101,
				Items: []item{{
					FirstName:    "Exact",
					LastName:     "Runner",
					Bib:          "100",
					OverallTime:  "5:00:00",
					OverallPlace: 101,
				}},
			})
		default:
			t.Errorf("unexpected PageIndex %d", req.PageIndex)
			json.NewEncoder(w).Encode(searchResponse{TotalItems: 101})
		}
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	got, err := c.Lookup(context.Background(), testEvent, "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runner != "Exact Runner" {
		t.Errorf("Runner: got %q, want %q", got.Runner, "Exact Runner")
	}
	if got.OverallPlace != 101 {
		t.Errorf("OverallPlace: got %d, want %d", got.OverallPlace, 101)
	}
	if requests != 2 {
		t.Errorf("requests: got %d, want %d", requests, 2)
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
