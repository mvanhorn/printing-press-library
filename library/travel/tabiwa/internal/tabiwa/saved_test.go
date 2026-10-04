package tabiwa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSavedIsBoundedAndMissingReadsDoNotCreateFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache ?#", "saved.db")
	rows, err := Saved(ctx, path, "10", 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("missing rows %v %v", rows, err)
	}
	if _, err = os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("read created parent")
	}
	for i := 0; i < 52; i++ {
		p := Normalize(RawProduct{ID: fmt.Sprintf("J%07d", i), Name: "券", Price: str("1,000円")}, "10", "", Origin+"/ticketList/search", time.Date(2026, 10, 4, 0, 0, i, 0, time.UTC).Format(time.RFC3339))
		if err = Save(ctx, path, []Product{p}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := os.Stat(path)
	rows, err = Saved(ctx, path, "10", 50)
	if err != nil || len(rows) != 50 {
		t.Fatalf("saved rows=%d err=%v", len(rows), err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() {
		t.Fatal("read changed canonical file")
	}
	if rows[0].ID != "J0000051" {
		t.Fatal("newest observation missing")
	}
	rows, err = Saved(ctx, path, "20", 5)
	if err != nil || len(rows) != 0 {
		t.Fatal("region leaked")
	}
}

func TestSavedNewestObservationWinsAcrossClockForms(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "saved.db")
	product := func(name, clock string) Product {
		return Normalize(RawProduct{ID: "J0001900", Name: name, Price: str("2,000P")}, "10", "", Origin+"/ticketList/search", clock)
	}
	newer := product("newer facts", "2026-10-04T12:20:00.123456789Z")
	if err := Save(ctx, path, []Product{newer}); err != nil {
		t.Fatal(err)
	}
	for _, olderOrEqual := range []string{"2026-10-04T12:19:59.999999999Z", "2026-10-04T21:20:00.1+09:00", "2026-10-04T12:20:00.123456788Z", "2026-10-04T21:20:00.123456789+09:00"} {
		if err := Save(ctx, path, []Product{product("delayed facts", olderOrEqual)}); err != nil {
			t.Fatal(err)
		}
		rows, err := Saved(ctx, path, "10", 1)
		if err != nil || len(rows) != 1 {
			t.Fatalf("read %v %+v", err, rows)
		}
		if rows[0].Name != newer.Name || rows[0].ObservedAt != newer.ObservedAt {
			t.Fatalf("delayed/equal %s replaced newest evidence: %+v", olderOrEqual, rows[0])
		}
	}
	newest := product("latest facts", "2026-10-04T08:20:00.12345679-04:00")
	if err := Save(ctx, path, []Product{newest}); err != nil {
		t.Fatal(err)
	}
	rows, err := Saved(ctx, path, "10", 1)
	if err != nil || rows[0].Name != newest.Name || rows[0].ObservedAt != newest.ObservedAt {
		t.Fatalf("newer instant did not update: %+v %v", rows, err)
	}
	if err = Save(ctx, path, []Product{product("invalid facts", "not-a-clock")}); err == nil {
		t.Fatal("invalid clock accepted")
	}
	rows, err = Saved(ctx, path, "10", 1)
	if err != nil || rows[0].Name != newest.Name {
		t.Fatal("invalid save changed evidence")
	}
}

func TestSavedOrderingAndRetentionUseInstants(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "saved.db")
	old := Normalize(RawProduct{ID: "J0001900", Name: "old with lexically late offset"}, "10", "", Origin, "2026-10-04T21:19:00+09:00")
	if err := Save(ctx, path, []Product{old}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		clock := time.Date(2026, 10, 4, 12, 20, i, 123456789, time.UTC).Format(time.RFC3339Nano)
		item := Normalize(RawProduct{ID: fmt.Sprintf("J%07d", i), Name: "newer"}, "10", "", Origin, clock)
		if err := Save(ctx, path, []Product{item}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := Saved(ctx, path, "10", 50)
	if err != nil || len(rows) != 50 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, r := range rows {
		if r.ID == old.ID {
			t.Fatal("lexically late older instant survived retention")
		}
	}
	if rows[0].ID != "J0000049" {
		t.Fatal("saved ordering is not instant aware")
	}
}
