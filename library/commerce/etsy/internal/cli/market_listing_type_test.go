package cli

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMktDigitalFromListingType(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	for _, tc := range []struct {
		name  string
		input *string
		want  any
	}{
		{"download", strPtr("download"), 1},
		{"both", strPtr("both"), 1},
		{"physical", strPtr("physical"), 0},
		{"trim and fold", strPtr(" Download "), 1},
		{"nil", nil, nil},
		{"empty", strPtr(""), nil},
		{"other", strPtr("other"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mktDigitalFromListingType(tc.input); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMarketSnapshotListingTypeDigital(t *testing.T) {
	mktTestEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/listings/active":
			// Real listing responses supply listing_type, never is_digital.
			json.NewEncoder(w).Encode(map[string]any{"count": 5, "results": []map[string]any{
				{"listing_id": 1000000001, "shop_id": 9000001, "title": "linen apron pattern", "listing_type": "download"},
				{"listing_id": 1000000002, "shop_id": 9000001, "title": "ceramic mug", "listing_type": "physical"},
				{"listing_id": 1000000003, "shop_id": 9000001, "title": "linen apron"},
				{"listing_id": 1000000004, "shop_id": 9000001, "title": "linen apron and pattern", "listing_type": "both"},
				{"listing_id": 1000000005, "shop_id": 9000001, "title": "ceramic mug set", "listing_type": "PHYSICAL"},
			}})
		case "/listings/batch":
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
				{"listing_id": 1000000001, "shop_id": 9000001, "shop": map[string]any{"shop_id": 9000001, "shop_name": "Juniper Ceramics"}},
			}})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("ETSY_BASE_URL", srv.URL)
	t.Setenv("ETSY_API_KEY", "fake:key")
	out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "5")...)
	if err != nil {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	db := mktOpenFixtureStore(t)
	defer db.Close()
	rows, err := db.DB().Query("SELECT listing_id, is_digital FROM market_results ORDER BY listing_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []sql.NullInt64{{Int64: 1, Valid: true}, {Int64: 0, Valid: true}, {}, {Int64: 1, Valid: true}, {Int64: 0, Valid: true}}
	i := 0
	for rows.Next() {
		var id int64
		var got sql.NullInt64
		if err := rows.Scan(&id, &got); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) {
			t.Fatal("unexpected extra listing")
		}
		if id != int64(1000000001+i) || got != want[i] {
			t.Errorf("listing %d: got %+v, want id %d value %+v", id, got, 1000000001+i, want[i])
		}
		i++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if i != len(want) {
		t.Fatalf("got %d listings, want %d", i, len(want))
	}
}
