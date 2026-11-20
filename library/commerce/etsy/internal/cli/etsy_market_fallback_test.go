package cli

import (
	"database/sql"
	"net/http"
	"testing"
)

func TestEtsyMarketPersistentlyOmittedFallsBackToShop(t *testing.T) {
	mktCheckOmittedFallback(t, http.StatusOK)
}

func TestEtsyMarketOmittedShop404IsTerminal(t *testing.T) {
	mktCheckOmittedFallback(t, http.StatusNotFound)
}

func mktCheckOmittedFallback(t *testing.T, mktStatus int) {
	t.Helper()
	mktTag := "MARKET_404_CHECK"
	if mktStatus == http.StatusOK {
		mktTag = "PROBE_MARKET_FALLBACK_MISSING"
	}
	mktTestEnv(t)
	mktFake := newMktFakeEtsy(t)
	mktShopRequests := 0
	mktFake.Config.Handler = http.HandlerFunc(func(mktWriter http.ResponseWriter, mktRequest *http.Request) {
		mktWriter.Header().Set("Content-Type", "application/json")
		switch mktRequest.URL.Path {
		case "/listings/active":
			_, _ = mktWriter.Write([]byte(`{"count":2,"results":[{"listing_id":1000000001,"shop_id":555001,"title":"linen apron"},{"listing_id":1000000002,"shop_id":555002,"title":"Juniper Ceramics"}]}`))
		case "/listings/batch":
			_, _ = mktWriter.Write([]byte(`{"results":[{"listing_id":1000000001,"shop_id":555001,"Shop":{"shop_id":555001,"transaction_sold_count":7,"listing_active_count":3}}]}`))
		case "/shops/555002":
			mktShopRequests++
			mktWriter.WriteHeader(mktStatus)
			if mktStatus == http.StatusOK {
				_, _ = mktWriter.Write([]byte(`{"shop_id":555002,"transaction_sold_count":42,"listing_active_count":3}`))
			}
		default:
			t.Errorf("unexpected request %s", mktRequest.URL.Path)
			mktWriter.WriteHeader(http.StatusNotFound)
		}
	})
	mktPointAtFake(t, mktFake)
	mktOut, mktErr := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "10")...)
	if mktErr != nil {
		t.Fatalf("snapshot: %v\n%s", mktErr, mktOut)
	}
	mktSummary := mktDecode(t, mktOut)
	if mktSummary["status"] != "complete" {
		t.Fatalf(mktTag+": summary=%v", mktSummary)
	}
	if _, mktExists := mktSummary["unresolved_shops"]; mktExists {
		t.Fatalf("unexpected unresolved_shops: %v", mktSummary)
	}
	mktDB := mktOpenFixtureStore(t)
	defer mktDB.Close()
	var mktSold, mktActive sql.NullInt64
	mktErr = mktDB.DB().QueryRow("SELECT transaction_sold_count, listing_active_count FROM market_shops WHERE snapshot_id=? AND shop_id=?", int64(mktSummary["snapshot_id"].(float64)), 555002).Scan(&mktSold, &mktActive)
	if mktErr != nil {
		t.Fatalf(mktTag+": shop row: %v", mktErr)
	}
	if mktStatus == http.StatusOK {
		if !mktSold.Valid || mktSold.Int64 != 42 || !mktActive.Valid || mktActive.Int64 != 3 {
			t.Fatalf(mktTag+": sold=%v active=%v", mktSold, mktActive)
		}
	} else if mktSold.Valid || mktActive.Valid {
		t.Fatalf("expected NULL metrics: sold=%v active=%v", mktSold, mktActive)
	}
	if mktShopRequests != 1 {
		t.Fatalf("shop requests=%d, want 1", mktShopRequests)
	}
}
