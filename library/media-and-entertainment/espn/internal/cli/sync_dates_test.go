package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
)

type fakeScoreboard struct {
	calls []map[string]string
	fail  map[string]bool // keyed by the dates param
}

func (f *fakeScoreboard) RateLimit() float64 { return 0 }
func (f *fakeScoreboard) Get(_ context.Context, _ string, p map[string]string) (json.RawMessage, error) {
	f.calls = append(f.calls, p)
	if f.fail[p["dates"]] {
		return nil, errors.New(`HTTP 400: {"code":400,"message":"Failed to get events endpoint."}`)
	}
	return json.RawMessage(`{"events":[{"id":"` + p["dates"] + `1","date":"2026-09-27T17:00Z","name":"x"}]}`), nil
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ESPN 400s every dates=START-END scoreboard query since 2026-09-15; each chunk must ask by month.
func TestSyncDatesRangeRequestsMonths(t *testing.T) {
	f := &fakeScoreboard{}
	if err := syncDatesRange(context.Background(), f, openTestStore(t), "football", "nfl", "20260924-20261111", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []string
	for _, c := range f.calls {
		if strings.Contains(c["dates"], "-") {
			t.Fatalf("sent a date range %q; ESPN rejects ranges", c["dates"])
		}
		got = append(got, c["dates"])
	}
	if strings.Join(got, ",") != "202609,202610,202611" {
		t.Fatalf("month requests = %v, want [202609 202610 202611]", got)
	}
}

// A failed month must fail the command, not exit 0 with "0 events".
func TestSyncDatesRangeReportsFailures(t *testing.T) {
	f := &fakeScoreboard{fail: map[string]bool{"202610": true}}
	err := syncDatesRange(context.Background(), f, openTestStore(t), "football", "nfl", "20260924-20261111", "")
	if err == nil || !strings.Contains(err.Error(), "1 of 3 month requests") {
		t.Fatalf("want a failure naming 1 failed month, got %v", err)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// Progress lines name the month actually fetched and stay valid JSON whatever the sport/league text.
func TestSyncDatesRangeProgressLinesAreJSON(t *testing.T) {
	f := &fakeScoreboard{fail: map[string]bool{"202610": true}}
	db := openTestStore(t)
	out := captureStderr(t, func() {
		_ = syncDatesRange(context.Background(), f, db, `foot"ball`, `n\fl`, "20260924-20261111", "")
	})
	var got []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSON progress line %q: %v", line, err)
		}
		if m["sport"] != `foot"ball` || m["league"] != `n\fl` {
			t.Fatalf("sport/league not round-tripped: %q", line)
		}
		got = append(got, fmt.Sprintf("%v:%v", m["event"], m["dates"]))
	}
	want := "sync_dates:202609,sync_dates_error:202610,sync_dates:202611"
	if strings.Join(got, ",") != want {
		t.Fatalf("progress lines = %v, want %s", got, want)
	}
}
