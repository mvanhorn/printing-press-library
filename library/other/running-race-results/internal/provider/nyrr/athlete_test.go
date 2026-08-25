package nyrr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// stripMeta removes the "_meta" key from a fixture file and returns the
// cleaned JSON bytes, mirroring what the real API might send.
func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	delete(m, "_meta")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-marshal fixture %s: %v", path, err)
	}
	return out
}

func newAthleteTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	searchFixture := loadFixture(t, "../../../testdata/fixtures/nyrr/runner-search.json")
	historyFixture := loadFixture(t, "../../../testdata/fixtures/nyrr/runner-history.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/runners/search":
			w.Write(searchFixture)
		case "/api/v2/runners/races":
			w.Write(historyFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestFindAthletes(t *testing.T) {
	srv := newAthleteTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	athletes, err := c.FindAthletes(context.Background(), "Runner")
	if err != nil {
		t.Fatalf("FindAthletes: %v", err)
	}
	if len(athletes) == 0 {
		t.Fatal("expected at least one athlete")
	}

	first := athletes[0]
	if first.ID != "10206629" {
		t.Errorf("first athlete ID: got %q, want %q", first.ID, "10206629")
	}
	if first.Name != "Sample Runner" {
		t.Errorf("first athlete Name: got %q, want %q", first.Name, "Sample Runner")
	}
	if first.Provider != "nyrr" {
		t.Errorf("first athlete Provider: got %q, want %q", first.Provider, "nyrr")
	}
}

func TestFindAthletes_Dedup(t *testing.T) {
	srv := newAthleteTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	athletes, err := c.FindAthletes(context.Background(), "Runner")
	if err != nil {
		t.Fatalf("FindAthletes: %v", err)
	}

	seen := make(map[string]struct{})
	for _, a := range athletes {
		if _, dup := seen[a.ID]; dup {
			t.Errorf("duplicate athlete ID %q in FindAthletes results", a.ID)
		}
		seen[a.ID] = struct{}{}
	}
}

func TestAthleteHistory(t *testing.T) {
	srv := newAthleteTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	results, err := c.AthleteHistory(context.Background(), "2969961")
	if err != nil {
		t.Fatalf("AthleteHistory: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	r := results[0]
	if r.RaceName != "NYRR Cross Country Champs." {
		t.Errorf("RaceName: got %q, want %q", r.RaceName, "NYRR Cross Country Champs.")
	}
	if r.Date != "2005-11-13" {
		t.Errorf("Date: got %q, want %q", r.Date, "2005-11-13")
	}
	if r.Distance != "5 kilometers" {
		t.Errorf("Distance: got %q, want %q", r.Distance, "5 kilometers")
	}
	if r.NetTime != "0:21:40" {
		t.Errorf("NetTime: got %q, want %q", r.NetTime, "0:21:40")
	}
	if r.Bib != "7629" {
		t.Errorf("Bib: got %q, want %q", r.Bib, "7629")
	}
	if r.SourceURL != "https://results.nyrr.org/races/a51113/results" {
		t.Errorf("SourceURL: got %q, want %q", r.SourceURL, "https://results.nyrr.org/races/a51113/results")
	}
	if r.Provider != "nyrr" {
		t.Errorf("Provider: got %q, want %q", r.Provider, "nyrr")
	}
}
