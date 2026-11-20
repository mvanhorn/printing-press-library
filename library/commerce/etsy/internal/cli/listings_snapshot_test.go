package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestListingsSnapshotLiveStoresRowAndListShowsHistory(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	db := s2FixtureDB(t, nil)
	for i := 0; i < 2; i++ {
		out, _, err := s2Run(t, "listings", "snapshot", "1000000001", "--db", db, "--json", "--no-cache", "--rate-limit", "1000")
		s2Must(t, err)
		var res s2SnapshotResult
		s2Decode(t, out, &res)
		if res.Mode != "saved" || res.SnapshotID == 0 || res.Source != "live" || !res.HasInv || res.HasPers {
			t.Fatalf("snapshot = %s", out)
		}
	}
	if srv.count("GET", "/listings/1000000001/personalization") != 2 {
		t.Fatal("personalization not read")
	}
	// verify straight against the DB
	st, err := store.Open(db)
	s2Must(t, err)
	defer st.Close()
	var n int
	var listingJSON, invJSON string
	s2Must(t, st.DB().QueryRow(`SELECT COUNT(*), MAX(listing_json), MAX(inventory_json) FROM listing_snapshots WHERE listing_id = 1000000001`).Scan(&n, &listingJSON, &invJSON))
	if n != 2 || !strings.Contains(listingJSON, "Hand-glazed ceramic mug") || !strings.Contains(invJSON, "MUG-SPECKLE-L") {
		t.Fatalf("rows=%d listing=%q inv=%q", n, listingJSON, invJSON)
	}
	out, _, err := s2Run(t, "listings", "snapshot", "--list", "--db", db, "--json")
	s2Must(t, err)
	var rows []store.ListingSnapshotRow
	s2Decode(t, out, &rows)
	if len(rows) != 2 || rows[0].ID < rows[1].ID || rows[0].ListingJSON != "" {
		t.Fatalf("history = %s", out)
	}
	out, _, _ = s2Run(t, "listings", "snapshot", "1000000001", "--list", "--bodies", "--db", db, "--json")
	s2Decode(t, out, &rows)
	if !strings.Contains(rows[0].ListingJSON, "ceramic") {
		t.Fatalf("--bodies missing JSON: %s", out)
	}
	out, _, _ = s2Run(t, "listings", "snapshot", "1999999999", "--list", "--db", db, "--json")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("history for unknown listing = %q", out)
	}
}

func TestListingsSnapshotFromStoreMakesNoAPICalls(t *testing.T) {
	srv := s2NewServer(t, nil)
	db := s2FixtureDB(t, func(db *store.Store) {
		s2Upsert(t, db.UpsertListings, s2Listing1)
		s2Must(t, db.Upsert("listings_inventory", "1000000001", json.RawMessage(`{"listing_id":1000000001,"parent_id":1000000001,"products":[{"product_id":1,"sku":"A","offerings":[{"offering_id":2,"quantity":1}]}]}`)))
	})
	out, _, err := s2Run(t, "listings", "snapshot", "1000000001", "--from-store", "--db", db, "--json")
	s2Must(t, err)
	var res s2SnapshotResult
	s2Decode(t, out, &res)
	if res.Source != "store" || !res.HasInv || len(srv.all()) != 0 {
		t.Fatalf("res=%s http=%d", out, len(srv.all()))
	}
	_, _, err = s2Run(t, "listings", "snapshot", "1000000077", "--from-store", "--db", db)
	if err == nil || ExitCode(err) != 3 {
		t.Fatalf("unknown listing: %v", err)
	}
}

func TestListingsSnapshotListOnMissingDBAndUsage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.db")
	out, errOut, err := s2Run(t, "listings", "snapshot", "--list", "--db", missing, "--json")
	s2Must(t, err)
	var rows []any
	s2Decode(t, out, &rows)
	if len(rows) != 0 || errOut == "" {
		t.Fatalf("out=%q err=%q", out, errOut)
	}
	if _, _, err := s2Run(t, "listings", "snapshot", "--db", missing); err == nil || ExitCode(err) != 2 {
		t.Fatalf("no id: %v", err)
	}
	out, _, err = s2Run(t, "listings", "snapshot", "--dry-run", "--json")
	s2Must(t, err)
	var env map[string]any
	s2Decode(t, out, &env)
	if env["dry_run"] != true {
		t.Fatalf("dry-run = %s", out)
	}
	_ = json.Valid
}
