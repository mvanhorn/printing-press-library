// internal/provider/mika/mika_test.go
package mika

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

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	searchHTML := readFixture(t, "../../../testdata/fixtures/mika/search.html")
	detailHTML := readFixture(t, "../../../testdata/fixtures/mika/detail.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("content") == "detail" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(detailHTML)
			return
		}
		// Default: search page (pid=search).
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(searchHTML)
	}))
	return srv
}

func TestLookup_Hit(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	ev := domain.Event{
		Provider: "mika",
		ID:       "BML_HCH3C0OH2F2",
		Name:     "BMW Berlin Marathon",
		Year:     2025,
	}

	res, err := c.Lookup(context.Background(), ev, "73664")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"Runner", res.Runner, "Sample Runner"},
		{"Bib", res.Bib, "73664"},
		{"NetTime", res.NetTime, "04:21:19"},
		{"GunTime", res.GunTime, "04:29:35"},
		{"Provider", res.Provider, "mika"},
	}
	for _, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.field, tc.got, tc.want)
		}
	}

	if res.OverallPlace != 24556 {
		t.Errorf("OverallPlace: got %d, want 24556", res.OverallPlace)
	}
	if res.GenderPlace != 17968 {
		t.Errorf("GenderPlace: got %d, want 17968", res.GenderPlace)
	}
	if !strings.Contains(res.SourceURL, "content=detail") {
		t.Errorf("SourceURL missing 'content=detail': %s", res.SourceURL)
	}
}

func TestLookup_Miss(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	ev := domain.Event{
		Provider: "mika",
		ID:       "BML_HCH3C0OH2F2",
		Name:     "BMW Berlin Marathon",
		Year:     2025,
	}

	// The served detail.html is bib 73664; requesting 00000 triggers the bib guard.
	_, err := c.Lookup(context.Background(), ev, "00000")
	if !errors.Is(err, provider.ErrBibNotFound) {
		t.Errorf("expected ErrBibNotFound, got: %v", err)
	}
}

func TestSearchByName(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL

	ev := domain.Event{
		Provider: "mika",
		ID:       "BML_HCH3C0OH2F2",
		Name:     "BMW Berlin Marathon",
		Year:     2025,
	}

	// search.html was captured for "Runner" - result rows contain anonymized Runner variants.
	got, err := c.SearchByName(context.Background(), ev, "Runner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one result for 'Runner'")
	}
	// All rows must have Runner and Bib populated.
	for _, r := range got {
		if r.Runner == "" {
			t.Errorf("result has empty Runner: %+v", r)
		}
		if r.Bib == "" {
			t.Errorf("result has empty Bib: %+v", r)
		}
	}
	// At least one result must contain the anonymized runner surname.
	found := false
	for _, r := range got {
		if strings.Contains(strings.ToLower(r.Runner), "runner") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no result Runner contains 'runner'; got: %v", got)
	}
}
