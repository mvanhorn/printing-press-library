package cli

import (
	"strings"
	"testing"
	"time"
)

func TestMarketEmptyStoreHintRawBytes(t *testing.T) {
	mktTestEnv(t)
	for _, command := range []string{"tags", "trend", "shops", "demand", "price"} {
		t.Run(command, func(t *testing.T) {
			out, err := mktRunCLI(t, "market", command)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "SEARCH TERM") || strings.Contains(out, "<") || strings.Contains(out, `\u003c`) {
				t.Fatalf("HINT_ESCAPING_BROKEN: %s", out)
			}
			if !mktNotesContain(mktDecode(t, out), mktNoSnapshotHint) {
				t.Fatalf("EMPTY_HINT_NOT_IN_NOTES: %s", out)
			}
		})
	}
}

func TestMarketShopsPrivacyAllowlist(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	allowedColumns := strings.Fields("snapshot_id shop_id shop_name transaction_sold_count listing_active_count create_date review_average review_count is_vacation")
	columns := map[string]bool{}
	rows, err := db.DB().Query("SELECT name FROM pragma_table_info('market_shops')")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, key := range allowedColumns {
		if !columns[key] {
			t.Fatalf("SHOP_ALLOWLIST_VIOLATED: missing column %s", key)
		}
		delete(columns, key)
	}
	if len(columns) != 0 {
		t.Fatalf("SHOP_ALLOWLIST_VIOLATED: extra columns %v", columns)
	}
	now := time.Now()
	snap := mktFxSnapshot(t, db.DB(), now, []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001})
	mktFxShop(t, db.DB(), snap, 9000001, 1, 20, now.Unix())
	out, err := mktRunCLI(t, "market", "shops")
	if err != nil {
		t.Fatal(err)
	}
	results, ok := mktDecode(t, out)["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("SHOP_ALLOWLIST_VIOLATED: missing shop row: %s", out)
	}
	row := results[0].(map[string]any)
	allowedKeys := strings.Fields("shop_id shop_name transaction_sold_count listing_active_count age_years review_average review_count top_n_appearances appearances new_shop new_shop_reasons")
	for _, key := range allowedKeys {
		if _, ok := row[key]; !ok {
			t.Fatalf("SHOP_ALLOWLIST_VIOLATED: missing JSON key %s", key)
		}
		delete(row, key)
	}
	if len(row) != 0 {
		t.Fatalf("SHOP_ALLOWLIST_VIOLATED: extra JSON keys %v", row)
	}
}
