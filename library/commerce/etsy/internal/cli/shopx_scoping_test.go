package cli

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestShopxForeignListingsExcluded(t *testing.T) {
	for _, args := range [][]string{{"listings", "audit"}, {"listings", "expiring"}, {"listings", "dead-weight"}, {"inventory", "audit"}, {"shop", "attention"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := shopxNewFixture(t)
			for _, tc := range []struct{ id, shop int }{{1000000001, 555001}, {1000000002, 999002}} {
				f.put("listings", map[string]any{"listing_id": tc.id, "shop_id": tc.shop, "state": "active", "title": "short", "tags": []any{}, "ending_timestamp": shopxIn(5 * shopxDay)})
				f.put("listings_inventory", map[string]any{"id": tc.id, "listing_id": tc.id, "product_id": tc.id})
			}
			f.synced("transactions", time.Hour)
			out, stderr, err := shopxRun(t, f, args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr, "hint: 1 listing(s) from other shops ignored") {
				t.Fatal(stderr)
			}
			if args[0] == "shop" {
				obj := shopxMustObject(t, out)
				if obj["expiring_30d"] != float64(1) || obj["audit_findings"].(map[string]any)["tags_lt_13"] != float64(1) {
					t.Fatal(out)
				}
			} else {
				rows := shopxMustList(t, out)
				if len(rows) == 0 {
					t.Fatal(out)
				}
				own := false
				for _, row := range rows {
					id, _ := shopxInt(row["listing_id"])
					if id == 1000000002 {
						t.Fatal(out)
					}
					own = own || id == 1000000001
				}
				if !own {
					t.Fatal(out)
				}
			}
		})
	}
}

func TestShopxShopUnsetEmptyWithNote(t *testing.T) {
	for _, args := range [][]string{{"listings", "audit"}, {"listings", "expiring"}, {"listings", "dead-weight"}, {"inventory", "audit"}, {"shop", "attention"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active", "tags": []any{}, "ending_timestamp": shopxIn(shopxDay)})
			t.Setenv("ETSY_SHOP_ID", "")
			out, stderr, err := shopxRun(t, f, args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr, "hint: "+shopxNoteShopUnset) {
				t.Fatal(stderr)
			}
			if args[0] == "shop" {
				obj := shopxMustObject(t, out)
				if obj["expiring_30d"] != nil || obj["audit_findings"] != nil {
					t.Fatal(out)
				}
			} else if len(shopxMustList(t, out)) != 0 {
				t.Fatal(out)
			}
		})
	}
}

func TestShopxFreshestDuplicateWins(t *testing.T) {
	for _, newer := range []string{"listings", "listings-featured"} {
		t.Run(newer, func(t *testing.T) {
			f := shopxNewFixture(t)
			for _, typ := range []string{"listings", "listings-featured"} {
				title, ts := "old", shopxAgo(2*shopxDay)
				if typ == newer {
					title, ts = "new", shopxAgo(shopxDay)
				}
				f.put(typ, map[string]any{"listing_id": 1000000001, "title": title, "last_modified_timestamp": ts})
			}
			rows, byID, dropped := shopxListingsByID(f.db, shopxTestShopID)
			if len(rows) != 1 || byID[1000000001]["title"] != "new" || dropped != 0 {
				t.Fatalf("%v %v %d", rows, byID, dropped)
			}
		})
	}
}

func TestShopxSortByListingIDScales(t *testing.T) {
	ids := rand.New(rand.NewSource(41)).Perm(50000)
	rows := make([]map[string]any, len(ids))
	for i, id := range ids {
		rows[i] = map[string]any{"listing_id": id + 1}
	}
	start := time.Now()
	shopxSortByListingID(rows)
	elapsed := time.Since(start)
	for i, row := range rows {
		if shopxListingID(row) != int64(i+1) {
			t.Fatalf("row %d: %v", i, row)
		}
	}
	if elapsed >= time.Second {
		t.Fatalf("sort took %s", elapsed)
	}
}

func TestShopxMemoDecodesOnce(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001})
	shopxWithMemo(func() {
		a := shopxRows(f.db, shopxListingTypes...)
		f.put("listings", map[string]any{"listing_id": 1000000002})
		b := shopxRows(f.db, shopxListingTypes...)
		if len(a) != 1 || len(b) != 1 || &a[0] != &b[0] {
			t.Fatalf("memo slices differ: %v %v", a, b)
		}
	})
	if shopxMemo != nil {
		t.Fatal("memo leaked")
	}
	f.put("listings", map[string]any{"listing_id": 1000000003})
	if rows := shopxRows(f.db, shopxListingTypes...); len(rows) != 3 {
		t.Fatal(rows)
	}
}

func TestShopxShopMatchesRequiresOwnership(t *testing.T) {
	for _, row := range []map[string]any{{}, {"shop_id": nil}, {"shop_id": 999002}} {
		if shopxShopMatches(row, shopxTestShopID) {
			t.Fatal(row)
		}
	}
	for _, v := range []any{555001, int64(555001), float64(555001), " 555001 "} {
		if !shopxShopMatches(map[string]any{"shop_id": v}, shopxTestShopID) {
			t.Fatal(v)
		}
	}
}

func TestShopxFreshnessTiesMissingAndDistinctForeign(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001, "title": "first", "updated_timestamp": shopxAgo(shopxDay)}, map[string]any{"listing_id": 1000000002, "shop_id": nil})
	f.put("listings-active", map[string]any{"listing_id": 1000000001, "title": "tie", "updated_timestamp": shopxAgo(shopxDay)}, map[string]any{"listing_id": 1000000002, "shop_id": 999002})
	f.put("shops_listings", map[string]any{"listing_id": 1000000001, "title": "undated"})
	_, byID, dropped := shopxListingsByID(f.db, shopxTestShopID)
	if byID[1000000001]["title"] != "first" || dropped != 1 {
		t.Fatalf("%v dropped %d", byID, dropped)
	}
}

func TestShopxInventoryWithoutListings(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings_inventory", map[string]any{"id": 1000000001, "listing_id": 1000000001, "product_id": 1})
	rows, notes := shopxInventoryFindings(f.db, shopxTestShopID, 0.5)
	if rows == nil || len(rows) != 0 || len(notes) != 1 || notes[0] != "listings not synced; cannot attribute inventory rows to your shop" {
		t.Fatalf("%v %v", rows, notes)
	}
}
