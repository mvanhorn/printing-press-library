package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestEnsureMarketSchemaCreatesTermPagesIdempotently(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 2; i++ {
		if err := EnsureMarketSchema(s.DB()); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err := s.DB().Exec(`INSERT INTO market_term_pages (snapshot_id, term, page, result_count, listings) VALUES (1, 'linen apron', 1, 0, 0)`); err != nil {
				t.Fatal(err)
			}
		}
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM market_term_pages WHERE snapshot_id=1 AND term='linen apron' AND page=1 AND result_count=0 AND listings=0`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}

func TestEnsureMarketSchemaAddsShopName(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "legacy-market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.DB().Exec(`CREATE TABLE market_shops (
		snapshot_id INTEGER NOT NULL, shop_id INTEGER NOT NULL,
		transaction_sold_count INTEGER, listing_active_count INTEGER,
		create_date INTEGER, review_average REAL, review_count INTEGER,
		is_vacation INTEGER, PRIMARY KEY(snapshot_id, shop_id))`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO market_shops(snapshot_id, shop_id, transaction_sold_count) VALUES(1, 555001, 7)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := EnsureMarketSchema(s.DB()); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := s.DB().QueryRow(`SELECT count(*) FROM pragma_table_info('market_shops') WHERE name='shop_name'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("column count=%d err=%v", count, err)
		}
		var name sql.NullString
		var sold int
		if err := s.DB().QueryRow(`SELECT shop_name, transaction_sold_count FROM market_shops WHERE snapshot_id=1 AND shop_id=555001`).Scan(&name, &sold); err != nil || name.Valid || sold != 7 {
			t.Fatalf("name=%v sold=%d err=%v", name, sold, err)
		}
	}
}
