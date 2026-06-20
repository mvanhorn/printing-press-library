// internal/provider/athlinks/athlinks_test.go
package athlinks

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

// loadFixture reads a JSON fixture file, strips the top-level "_meta" key,
// and returns the re-marshalled bytes representing the real API response.
func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}
	delete(m, "_meta")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-marshal fixture %s: %v", path, err)
	}
	return out
}

// loadFixtureField reads a JSON fixture and returns the raw bytes of a single
// top-level field — used for the search fixture whose real API response is the
// bare array stored under "data" by the capture wrapper.
func loadFixtureField(t *testing.T, path, field string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}
	v, ok := m[field]
	if !ok {
		t.Fatalf("fixture %s missing field %q", path, field)
	}
	return v
}

var testEvent = domain.Event{
	Provider: "athlinks",
	ID:       "1094411",
	Name:     "Paraguay Multisport Challenge",
	Year:     2024,
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	searchFixture := loadFixtureField(t, "../../../testdata/fixtures/athlinks/search.json", "data")
	detailFixture := loadFixture(t, "../../../testdata/fixtures/athlinks/detail.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/results/search"):
			w.Write(searchFixture)
		case strings.Contains(path, "/bib/") && strings.Contains(path, "/result"):
			w.Write(detailFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestLookup_Hit(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	c.Token = "Bearer test"

	got, err := c.Lookup(context.Background(), testEvent, "8420")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Values from detail.json (with _meta stripped):
	// displayName: "Sample Runner"
	// chipTimeInMillis: 535000 → 535s → "0:08:55"
	// gunTimeInMillis:  540000 → 540s → "0:09:00"
	// intervals[0] full=true:
	//   divisions: overall rank=1, gender rank=1, M30-39 rank=1
	if got.Runner != "Sample Runner" {
		t.Errorf("Runner: got %q, want %q", got.Runner, "Sample Runner")
	}
	if got.Bib != "8420" {
		t.Errorf("Bib: got %q, want %q", got.Bib, "8420")
	}
	if got.NetTime != "0:08:55" {
		t.Errorf("NetTime: got %q, want %q", got.NetTime, "0:08:55")
	}
	if got.GunTime != "0:09:00" {
		t.Errorf("GunTime: got %q, want %q", got.GunTime, "0:09:00")
	}
	if got.OverallPlace != 1 {
		t.Errorf("OverallPlace: got %d, want %d", got.OverallPlace, 1)
	}
	if got.GenderPlace != 1 {
		t.Errorf("GenderPlace: got %d, want %d", got.GenderPlace, 1)
	}
	if got.Provider != "athlinks" {
		t.Errorf("Provider: got %q, want %q", got.Provider, "athlinks")
	}
	if got.SourceURL != "https://www.athlinks.com/events/1094411/results" {
		t.Errorf("SourceURL: got %q", got.SourceURL)
	}
}

func TestLookup_Miss(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	c.Token = "Bearer test"

	_, err := c.Lookup(context.Background(), testEvent, "00000")
	if !errors.Is(err, provider.ErrBibNotFound) {
		t.Errorf("expected ErrBibNotFound, got: %v", err)
	}
}

// TestLookup_NoToken verifies the token is optional: with no token set, the
// public search + detail endpoints are queried anonymously and the lookup
// succeeds.
func TestLookup_NoToken(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c2 := New()
	c2.BaseURL = srv.URL
	c2.Token = ""

	got, err := c2.Lookup(context.Background(), testEvent, "8420")
	if err != nil {
		t.Fatalf("anonymous lookup should succeed, got: %v", err)
	}
	if got.Runner != "Sample Runner" {
		t.Errorf("Runner: got %q, want %q", got.Runner, "Sample Runner")
	}
}

// TestLookup_AuthGated verifies the fallback: when an endpoint returns 401/403
// and no token is set, the error tells the user to set ATHLINKS_TOKEN.
func TestLookup_AuthGated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	c.Token = ""

	_, err := c.Lookup(context.Background(), testEvent, "8420")
	if err == nil {
		t.Fatal("expected error on 403 with no token, got nil")
	}
	if !strings.Contains(err.Error(), "ATHLINKS_TOKEN") {
		t.Errorf("expected error mentioning ATHLINKS_TOKEN, got: %v", err)
	}
}

func TestSearchByName(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	c.Token = "Bearer test"

	// The fixture contains anonymized runner rows; search for "RUNNER".
	got, err := c.SearchByName(context.Background(), testEvent, "RUNNER")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one result for 'RUNNER'")
	}
	for _, r := range got {
		if !strings.Contains(strings.ToUpper(r.Runner), "RUNNER") {
			t.Errorf("Runner %q does not contain 'RUNNER'", r.Runner)
		}
		if r.Bib == "" {
			t.Errorf("result has empty Bib: %+v", r)
		}
		if r.SourceURL != "https://www.athlinks.com/events/1094411/results" {
			t.Errorf("SourceURL: got %q", r.SourceURL)
		}
	}
}
