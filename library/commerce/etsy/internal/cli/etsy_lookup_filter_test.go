package cli

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestEtsyLookupRefreshBuyerBoundary(t *testing.T) {
	etsyDB, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer etsyDB.Close()
	etsyCtx := context.Background()
	for etsyType, etsyData := range map[string]string{"receipts": `{"receipt_id":910001,"name":"Quinby Testbuyer"}`, "listings": `{"listing_id":1000000001,"title":"Juniper Ceramics mug"}`} {
		if err := etsyDB.Upsert(etsyType, "910001", []byte(etsyData)); err != nil {
			t.Fatal(err)
		}
	}
	if err := lookups.RecordMisses(etsyCtx, etsyDB.DB(), []string{"Quinby Testbuyer", "Juniper Ceramics mug", "Juniper Ceramics", "Juniper", "Ceramics", "Quinby", "Testbuyer"}); err != nil {
		t.Fatal(err)
	}
	refreshLookupsFromSyncedStore(etsyCtx, etsyDB.DB(), io.Discard)
	var etsyGood, etsyBad int
	if err := etsyDB.DB().QueryRow("SELECT count(*) FROM entity_lookups WHERE kind='listings' AND canonical IN ('Juniper','Ceramics')").Scan(&etsyGood); err != nil {
		t.Fatal(err)
	}
	if err := etsyDB.DB().QueryRow("SELECT count(*) FROM entity_lookups WHERE kind='receipts' OR canonical LIKE '%Quinby Testbuyer%' OR value LIKE '%Quinby Testbuyer%'").Scan(&etsyBad); err != nil {
		t.Fatal(err)
	}
	if etsyGood != 2 || etsyBad != 0 {
		t.Fatalf("PROBE_LOOKUP_EXTRACT_BUYER_KIND: listings=%d buyer=%d", etsyGood, etsyBad)
	}
}

func TestEtsyLookupRefreshPurgesExistingBuyerRows(t *testing.T) {
	etsyDB, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer etsyDB.Close()
	if _, err := etsyDB.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES ('receipts','Quinby Testbuyer','Quinby Testbuyer','synced'), ('users','Test Buyer','Test Buyer','synced'), ('listings','Juniper Ceramics','Juniper Ceramics','synced'), ('receipts','Seed Row','Seed Row','taught')`); err != nil {
		t.Fatal(err)
	}
	refreshLookupsFromSyncedStore(context.Background(), etsyDB.DB(), io.Discard)
	var etsyBad, etsyListing, etsyTaught int
	for etsyQuery, etsyCount := range map[string]*int{
		"SELECT count(*) FROM entity_lookups WHERE source='synced' AND kind IN ('receipts','users')": &etsyBad,
		"SELECT count(*) FROM entity_lookups WHERE source='synced' AND kind='listings'":              &etsyListing,
		"SELECT count(*) FROM entity_lookups WHERE source='taught' AND canonical='Seed Row'":         &etsyTaught,
	} {
		if err := etsyDB.DB().QueryRow(etsyQuery).Scan(etsyCount); err != nil {
			t.Fatal(err)
		}
	}
	if etsyBad != 0 || etsyListing != 1 || etsyTaught != 1 {
		t.Fatalf("PROBE_LOOKUP_PURGE_MISSING: buyer=%d listing=%d taught=%d", etsyBad, etsyListing, etsyTaught)
	}
}
