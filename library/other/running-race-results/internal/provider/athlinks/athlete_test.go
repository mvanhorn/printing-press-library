package athlinks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// serveStripped writes the fixture's real body. For an object body it strips a
// top-level "_meta" key; the search fixture's real body is the object itself.
func serveStripped(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	delete(m, "_meta")
	out, _ := json.Marshal(m)
	return out
}

func athleteServer(t *testing.T) *httptest.Server {
	t.Helper()
	search := serveStripped(t, "../../../testdata/fixtures/athlinks/athlete-search.json")
	races := serveStripped(t, "../../../testdata/fixtures/athlinks/athlete-results.json")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/athletes/api/find"):
			w.Write(search)
		case strings.Contains(r.URL.Path, "/Races"):
			w.Write(races)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestFindAthletes(t *testing.T) {
	srv := athleteServer(t)
	defer srv.Close()
	c := New()
	c.AlaskaURL = srv.URL
	c.Token = "Bearer test"
	got, err := c.FindAthletes(context.Background(), "Sample")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected athletes")
	}
	if got[0].ID != "338681853" || got[0].Name != "Sample Athlete" || got[0].Provider != "athlinks" {
		t.Fatalf("bad first athlete: %+v", got[0])
	}
}

func TestAthleteHistory(t *testing.T) {
	srv := athleteServer(t)
	defer srv.Close()
	c := New()
	c.AlaskaURL = srv.URL
	c.Token = "Bearer test"
	got, err := c.AthleteHistory(context.Background(), "338681853")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected race history")
	}
	r := got[0]
	// Exact values from athlete-results.json List[0].
	if r.Provider != "athlinks" {
		t.Errorf("Provider: got %q", r.Provider)
	}
	if r.RaceName != "Mohican 100 Trail Run" {
		t.Errorf("RaceName: got %q", r.RaceName)
	}
	if r.Date != "2023-06-03" {
		t.Errorf("Date: got %q want 2023-06-03 (must come from Race.RaceDate, not the zero EventDate)", r.Date)
	}
	if r.Distance != "Trail Marathon" {
		t.Errorf("Distance: got %q want Trail Marathon (from Race.Courses[0].CourseName)", r.Distance)
	}
	if r.NetTime != "6:21:25" {
		t.Errorf("NetTime: got %q want 6:21:25", r.NetTime)
	}
}
