package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newNCCPLTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := EnsureNCCPLSchema(context.Background(), s); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

func TestSaveNCCPLDateRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)
	rows := []NCCPLRow{
		{Key: "FI|EQUITY", Payload: `{"client_type":"FI","net_value":100}`},
		{Key: "FC|EQUITY", Payload: `{"client_type":"FC","net_value":-30}`},
	}
	if err := SaveNCCPLDate(ctx, s, "fipi", "2026-09-04", rows, time.Now()); err != nil {
		t.Fatalf("save: %v", err)
	}
	obs, err := NCCPLObservations(ctx, s, "fipi", "", "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d observations, want 2", len(obs))
	}
	if obs[0].ObservedAt == "" {
		t.Error("observed_at must be stamped")
	}
}

// A date that legitimately returned nothing must still be recorded, or a later gap
// audit cannot tell "fetched and empty" from "never fetched".
func TestSaveNCCPLDateRecordsEmptyDates(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)
	if err := SaveNCCPLDate(ctx, s, "slb", "2026-09-04", nil, time.Now()); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	cov, err := NCCPLCoveredDates(ctx, s, "slb")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if len(cov) != 1 {
		t.Fatalf("got %d coverage rows, want 1 -- an empty fetch must still be recorded", len(cov))
	}
	if cov[0].RowCount != 0 {
		t.Errorf("row_count = %d, want 0", cov[0].RowCount)
	}
}

// Re-syncing must refresh the payload but must NOT move observed_at forward: the
// vintage records when a value was first seen, which is what establishes ex-ante
// availability.
func TestSaveNCCPLDatePreservesFirstObservedAt(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)
	first := time.Now().Add(-48 * time.Hour)
	if err := SaveNCCPLDate(ctx, s, "fipi", "2026-09-04",
		[]NCCPLRow{{Key: "k", Payload: `{"v":1}`}}, first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	before, err := NCCPLObservations(ctx, s, "fipi", "", "")
	if err != nil || len(before) != 1 {
		t.Fatalf("read back: %v", err)
	}
	if err := SaveNCCPLDate(ctx, s, "fipi", "2026-09-04",
		[]NCCPLRow{{Key: "k", Payload: `{"v":2}`}}, time.Now()); err != nil {
		t.Fatalf("second save: %v", err)
	}
	after, err := NCCPLObservations(ctx, s, "fipi", "", "")
	if err != nil || len(after) != 1 {
		t.Fatalf("read back 2: %v", err)
	}
	if after[0].Payload != `{"v":2}` {
		t.Errorf("payload = %s, want refreshed", after[0].Payload)
	}
	if after[0].ObservedAt != before[0].ObservedAt {
		t.Errorf("observed_at moved from %q to %q; the vintage must record first observation",
			before[0].ObservedAt, after[0].ObservedAt)
	}
}

func TestNCCPLObservationsDateRangeFilter(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)
	for _, d := range []string{"2026-09-01", "2026-09-02", "2026-09-03"} {
		if err := SaveNCCPLDate(ctx, s, "fipi", d,
			[]NCCPLRow{{Key: "k", Payload: `{"v":1}`}}, time.Now()); err != nil {
			t.Fatalf("save %s: %v", d, err)
		}
	}
	got, err := NCCPLObservations(ctx, s, "fipi", "2026-09-02", "2026-09-03")
	if err != nil {
		t.Fatalf("range read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows in range, want 2", len(got))
	}
	res, err := NCCPLStoredResources(ctx, s)
	if err != nil || len(res) != 1 || res[0] != "fipi" {
		t.Fatalf("stored resources = %v, %v", res, err)
	}
}
