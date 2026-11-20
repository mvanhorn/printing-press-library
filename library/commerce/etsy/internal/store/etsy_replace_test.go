package store

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"path/filepath"
	"testing"
)

func TestEtsyReplaceParentPartition(t *testing.T) {
	testenv.Isolate(t)
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := json.RawMessage(`{"product_id":7000000001,"parent_id":9000001,"sku":"ghostword"}`)
	if _, _, _, err = db.UpsertBatchDetailed("listings_inventory", []json.RawMessage{old}); err != nil {
		t.Fatal(err)
	}
	// A real insertion error must roll back deletion, unlike an extraction miss.
	if _, err = db.db.Exec(`CREATE TRIGGER etsy_fail BEFORE INSERT ON resources WHEN NEW.resource_type='listings_inventory' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	_, _, _, deleted, err := db.ReplaceParentPartition("listings_inventory", "", []string{"9000001"}, []json.RawMessage{json.RawMessage(`{"product_id":7000000999,"parent_id":"9000001"}`)})
	if err == nil || deleted != 0 {
		t.Fatalf("rollback: %d %v", deleted, err)
	}
	n, _ := db.Count("listings_inventory")
	if n != 1 {
		t.Fatal(n)
	}
	if _, err = db.db.Exec(`DROP TRIGGER etsy_fail`); err != nil {
		t.Fatal(err)
	}
	// Extraction misses retain normal UpsertBatchDetailed semantics: skip and commit.
	_, miss, _, deleted, err := db.ReplaceParentPartition("listings_inventory", "", []string{"9000001"}, []json.RawMessage{json.RawMessage(`{"sku":"missing"}`)})
	if err != nil || miss != 1 || deleted != 1 {
		t.Fatalf("replace: %d %d %v", miss, deleted, err)
	}
	n, _ = db.Count("listings_inventory")
	if n != 0 {
		t.Fatal(n)
	}
	var fts int
	if err = db.db.QueryRow(`SELECT count(*) FROM resources_fts WHERE resources_fts MATCH 'ghostword'`).Scan(&fts); err != nil || fts != 0 {
		t.Fatalf("fts: %d %v", fts, err)
	}
	_, _, _, deleted, err = db.ReplaceParentPartition("listings_inventory", "", nil, []json.RawMessage{old})
	if err != nil || deleted != 0 {
		t.Fatal(err)
	}
}

func TestEtsyReplaceImagesEmptyPartition(t *testing.T) {
	testenv.Isolate(t)
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := []json.RawMessage{
		json.RawMessage(`{"listing_image_id":71,"listing_id":"9","parent_id":"9","alt_text":"obsoletepicture"}`),
		json.RawMessage(`{"listing_image_id":72,"listing_id":"10","parent_id":"10"}`),
	}
	_, _, typed, err := db.UpsertBatchDetailed("listing_images", rows)
	if err != nil || typed != 0 {
		t.Fatalf("seed %d %v", typed, err)
	}
	_, _, _, deleted, err := db.ReplaceParentPartition("listing_images", "listing_images", []string{"9"}, nil)
	if err != nil || deleted != 1 {
		t.Fatalf("replace %d %v", deleted, err)
	}
	var count int
	if err = db.db.QueryRow(`SELECT count(*) FROM listing_images`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("typed %d %v", count, err)
	}
	n, err := db.Count("listing_images")
	if err != nil || n != 1 {
		t.Fatalf("generic %d %v", n, err)
	}
	if err = db.db.QueryRow(`SELECT count(*) FROM resources_fts WHERE resources_fts MATCH 'obsoletepicture'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("fts %d %v", count, err)
	}
}

func TestEtsyReplaceParentPartitionSQLCandidates(t *testing.T) {
	testenv.Isolate(t)
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	items := []json.RawMessage{
		json.RawMessage(`{"product_id":71,"parent_id":"9000001","sku":"removedstring"}`),
		json.RawMessage(`{"product_id":72,"parent_id":9000001,"sku":"removednumber"}`),
		json.RawMessage(`{"product_id":73,"parent_id":"9000002","sku":"survivor"}`),
	}
	if _, misses, _, err := db.UpsertBatchDetailed("listings_inventory", items); err != nil || misses != 0 {
		t.Fatalf("seed: %d %v", misses, err)
	}
	if _, err := db.db.Exec(`INSERT INTO resources(id,resource_type,data) VALUES('malformed','listings_inventory','{not json')`); err != nil {
		t.Fatal(err)
	}
	_, _, _, deleted, err := db.ReplaceParentPartition("listings_inventory", "", []string{"9000001"}, nil)
	if err != nil || deleted != 2 {
		t.Fatalf("replace: %d %v", deleted, err)
	}
	var count int
	if err := db.db.QueryRow(`SELECT count(*) FROM resources WHERE resource_type='listings_inventory' AND (id='malformed' OR json_valid(data))`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("survivors: %d %v", count, err)
	}
	if err := db.db.QueryRow(`SELECT count(*) FROM resources_fts WHERE resources_fts MATCH 'removedstring OR removednumber'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("fts: %d %v", count, err)
	}
	if err := db.db.QueryRow(`SELECT count(*) FROM resources_fts WHERE resources_fts MATCH 'survivor'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("surviving fts: %d %v", count, err)
	}
}

func TestEtsyReplaceParentPartitionChunks(t *testing.T) {
	testenv.Isolate(t)
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	parents := make([]string, 1200)
	for i := range parents {
		parents[i] = fmt.Sprint(1000000001 + i)
	}
	items := []json.RawMessage{
		json.RawMessage(fmt.Sprintf(`{"product_id":81,"parent_id":%q}`, parents[0])),
		json.RawMessage(fmt.Sprintf(`{"product_id":82,"parent_id":%q}`, parents[1199])),
		json.RawMessage(`{"product_id":83,"parent_id":"outside"}`),
	}
	if _, misses, _, err := db.UpsertBatchDetailed("listings_inventory", items); err != nil || misses != 0 {
		t.Fatalf("seed: %d %v", misses, err)
	}
	// Repeat a parent across chunks to verify victims are deduplicated.
	parents[600] = parents[0]
	_, _, _, deleted, err := db.ReplaceParentPartition("listings_inventory", "", parents, nil)
	if err != nil || deleted != 2 {
		t.Fatalf("replace: %d %v", deleted, err)
	}
	if count, err := db.Count("listings_inventory"); err != nil || count != 1 {
		t.Fatalf("survivors: %d %v", count, err)
	}
}

func TestEtsyReplaceParentPartitionNUL(t *testing.T) {
	testenv.Isolate(t)
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	items := []json.RawMessage{
		json.RawMessage(`{"product_id":91,"parent_id":"7\u00000"}`),
		json.RawMessage(`{"product_id":92,"parent_id":"7"}`),
	}
	if _, misses, _, err := db.UpsertBatchDetailed("listings_inventory", items); err != nil || misses != 0 {
		t.Fatalf("seed: %d %v", misses, err)
	}
	_, _, _, deleted, err := db.ReplaceParentPartition("listings_inventory", "", []string{"7\x000"}, nil)
	if err != nil || deleted != 1 {
		t.Fatalf("replace: %d %v", deleted, err)
	}
	var parent string
	if err := db.db.QueryRow(`SELECT json_extract(data,'$.parent_id') FROM resources WHERE resource_type='listings_inventory'`).Scan(&parent); err != nil || parent != "7" {
		t.Fatalf("survivor: %q %v", parent, err)
	}
}
