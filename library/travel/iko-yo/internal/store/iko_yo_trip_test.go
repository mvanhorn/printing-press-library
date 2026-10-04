package store

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"path/filepath"
	"testing"
	"time"
)

func TestTripCacheAtomicSnapshotsProvenanceAndSearch(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "cache.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	raw := []byte(`<div class="p-shared-basic_info"><table><tr><th>スポット名</th><td>Family place</td></tr><tr><th>住所</th><td>埼玉県草加市</td></tr></table></div><p class="p-shared-paragraph">授乳室があります。</p>`)
	r, e := trip.ParseDetail(raw, "spots/8220", observedTripTime())
	if e != nil {
		t.Fatal(e)
	}
	if e := db.SaveTripRecords(ctx, []trip.Record{r}); e != nil {
		t.Fatal(e)
	}
	saved, e := db.TripRecord(ctx, r.Ref)
	if e != nil || saved.Amenities["nursing"].Status != "reported_present" || saved.DataSource != "local" {
		t.Fatalf("%+v %v", saved, e)
	}
	count, e := db.TripCacheCount(ctx, "spots")
	if e != nil || count != 1 {
		t.Fatalf("count %d %v", count, e)
	}
	records, cov, e := db.TripRecords(ctx, "all", 10)
	if e != nil || len(records) != 1 || len(cov) != 1 {
		t.Fatalf("read/join %d %d %v", len(records), len(cov), e)
	}
	found, e := db.Search("Family", 10, "spots")
	if e != nil || len(found) != 1 {
		t.Fatalf("generic search %d %v", len(found), e)
	}
	// A basic observation adds collection provenance without overwriting detail age.
	basic := r
	basic.Detail = false
	basic.Name = "new list title"
	basic.ObservedAt = "2026-10-04T00:00:00Z"
	basic.Collection.PagesScanned = 1
	basic.Collection.Region = 6
	basic.Collection.Prefecture = 11
	if e := db.SaveTripRecords(ctx, []trip.Record{basic}); e != nil {
		t.Fatal(e)
	}
	saved, e = db.TripRecord(ctx, r.Ref)
	if e != nil || !saved.Detail || saved.Name != "Family place" || saved.ObservedAt != r.ObservedAt {
		t.Fatalf("detail relabeled %+v %v", saved, e)
	}
	_, cov, e = db.TripRecords(ctx, "all", 10)
	if e != nil || len(cov) != 2 {
		t.Fatalf("provenance %d %v", len(cov), e)
	}
	// A fresh detail replaces stale positive evidence instead of merging it.
	fresh, e := trip.ParseDetail([]byte(`<div class="p-shared-basic_info"><table><tr><th>スポット名</th><td>Family place</td></tr></table></div>`), r.Ref, observedTripTime())
	if e != nil {
		t.Fatal(e)
	}
	if e := db.SaveTripRecords(ctx, []trip.Record{fresh}); e != nil {
		t.Fatal(e)
	}
	saved, e = db.TripRecord(ctx, r.Ref)
	if e != nil || saved.Amenities["nursing"].Status != "unknown" || len(saved.Evidence) != 0 {
		t.Fatalf("stale evidence retained %+v %v", saved, e)
	}
	generic, e := db.Get("spots", "8220")
	if e != nil {
		t.Fatal(e)
	}
	var mirrored trip.Record
	if e := json.Unmarshal(generic, &mirrored); e != nil || mirrored.Amenities["nursing"].Status != "unknown" {
		t.Fatalf("stale mirror %s %v", generic, e)
	}
}
func TestTripCacheRollbackAndEmptyStore(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "cache.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	records, cov, e := db.TripRecords(ctx, "all", 10)
	if e != nil || len(records) != 0 || len(cov) != 0 {
		t.Fatal("empty store", e)
	}
	r, e := trip.ParseDetail([]byte(`<div class="p-shared-basic_info"><table><tr><th>イベント名</th><td>Event</td></tr></table></div>`), "events/8412", observedTripTime())
	if e != nil {
		t.Fatal(e)
	}
	bad := r
	bad.Ref = "wrong"
	if db.SaveTripRecords(ctx, []trip.Record{r, bad}) == nil {
		t.Fatal("invalid source reference accepted")
	}
	count, e := db.TripCacheCount(ctx, "all")
	if e != nil || count != 0 {
		t.Fatalf("rollback %d %v", count, e)
	}
	if _, _, e := db.TripRecords(ctx, "all", 1001); e == nil {
		t.Fatal("unbounded local scan")
	}
}

func observedTripTime() time.Time { return time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC) }
