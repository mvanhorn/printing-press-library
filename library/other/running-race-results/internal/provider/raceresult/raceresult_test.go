// internal/provider/raceresult/raceresult_test.go
package raceresult

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

// loadFixture reads a fixture file, strips the top-level "_meta" key,
// and returns the resulting JSON bytes (the real API response).
func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loadFixture: read %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("loadFixture: unmarshal %s: %v", path, err)
	}
	delete(m, "_meta")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("loadFixture: re-marshal %s: %v", path, err)
	}
	return out
}

// teamListJSON is a minimal list that has a BIB column (bib 1286 present) but
// NO AnzeigeName and NO TIME1 columns — exactly what a team list looks like.
// The adapter must skip it and proceed to the individual list.
const teamListJSON = `{"DataFields":["BIB","ID","Team"],"data":{"#g":[["1286","1286","Some Team"]]}}`

func TestLookup(t *testing.T) {
	configFixture := loadFixture(t, "../../../testdata/fixtures/raceresult/config.json")
	resultsFixture := loadFixture(t, "../../../testdata/fixtures/raceresult/results.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/results/config"):
			w.Write(configFixture)
		case strings.Contains(r.URL.Path, "/results/list"):
			if r.URL.Query().Get("term") == "" {
				t.Errorf("results/list request must include a server-side term filter: %s", r.URL.RawQuery)
			}
			// Route by the listname query parameter: team lists get the team
			// fixture (no name/time columns); individual lists get the real data.
			listname := r.URL.Query().Get("listname")
			if strings.Contains(listname, "teams") || strings.Contains(listname, "internet-teams") {
				w.Write([]byte(teamListJSON))
			} else {
				w.Write(resultsFixture)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	c.DataBaseURL = srv.URL

	ev := domain.Event{
		Provider: "raceresult",
		ID:       "390537",
		Name:     "17. REWE Team Challenge Dresden",
		Year:     2026,
	}

	t.Run("hit", func(t *testing.T) {
		// The config fixture lists two team lists ("internet-teams - *") BEFORE
		// the individual lists ("Internet-einzel - *"). The team lists contain
		// bib 1286 but lack AnzeigeName/TIME1, so the adapter must skip them
		// (ok=true, found=false) and find the result in the individual list.
		result, err := c.Lookup(context.Background(), ev, "1286")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Runner != "Runner Alpha" {
			t.Errorf("Runner: got %q, want %q", result.Runner, "Runner Alpha")
		}
		if result.Bib != "1286" {
			t.Errorf("Bib: got %q, want %q", result.Bib, "1286")
		}
		if result.NetTime != "0:17:43" {
			t.Errorf("NetTime: got %q, want %q", result.NetTime, "0:17:43")
		}
		if result.OverallPlace != 1 {
			t.Errorf("OverallPlace: got %d, want %d", result.OverallPlace, 1)
		}
		if result.Provider != "raceresult" {
			t.Errorf("Provider: got %q, want %q", result.Provider, "raceresult")
		}
	})

	t.Run("miss", func(t *testing.T) {
		_, err := c.Lookup(context.Background(), ev, "000000")
		if !errors.Is(err, provider.ErrBibNotFound) {
			t.Errorf("expected ErrBibNotFound, got %v", err)
		}
	})

	t.Run("SearchByName", func(t *testing.T) {
		// "Alpha" appears in an anonymized runner row in the results fixture.
		got, err := c.SearchByName(context.Background(), ev, "Alpha")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) == 0 {
			t.Fatal("expected at least one result for 'Alpha'")
		}
		// The config exposes two individual lists (einzel Frauen + Männer),
		// both served the same fixture here — a runner must not be duplicated.
		seen := map[string]int{}
		for _, r := range got {
			if !strings.Contains(strings.ToLower(r.Runner), "alpha") {
				t.Errorf("Runner %q does not contain 'alpha'", r.Runner)
			}
			if r.Bib == "" {
				t.Errorf("result has empty Bib: %+v", r)
			}
			seen[r.Bib]++
		}
		for bib, n := range seen {
			if n > 1 {
				t.Errorf("bib %q appears %d times; SearchByName must dedup across lists", bib, n)
			}
		}
	})
}
