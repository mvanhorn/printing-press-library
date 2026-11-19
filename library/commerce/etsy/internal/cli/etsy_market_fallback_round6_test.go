package cli

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
)

func mktRound6Fallback(t *testing.T, mktQuota bool) *atomic.Int64 {
	t.Helper()
	mktTestEnv(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	mktFake := newMktFakeEtsy(t)
	mktRequests := &atomic.Int64{}
	mktFake.Config.Handler = http.HandlerFunc(func(mktWriter http.ResponseWriter, mktRequest *http.Request) {
		mktWriter.Header().Set("Content-Type", "application/json")
		switch mktRequest.URL.Path {
		case "/listings/active":
			_, _ = mktWriter.Write([]byte(`{"count":2,"results":[{"listing_id":1000000001,"shop_id":555001,"title":"linen apron"},{"listing_id":1000000002,"shop_id":555002,"title":"linen apron"}]}`))
		case "/listings/batch":
			_, _ = mktWriter.Write([]byte(`{"results":[{"listing_id":1000000001,"shop_id":555001,"Shop":{"shop_id":555001,"transaction_sold_count":7,"listing_active_count":3}}]}`))
		case "/shops/555002":
			mktRequests.Add(1)
			if mktQuota {
				mktWriter.Header().Set("X-Limit-Per-Day", "5000")
				mktWriter.Header().Set("X-Remaining-Today", "0")
				mktWriter.WriteHeader(http.StatusTooManyRequests)
				_, _ = mktWriter.Write([]byte(`{"error":"daily quota exhausted"}`))
				return
			}
			_, _ = mktWriter.Write([]byte(`{"shop_id":555002,"transaction_sold_count":3,"listing_active_count":4}`))
		default:
			t.Errorf("unexpected request %s", mktRequest.URL.Path)
			mktWriter.WriteHeader(http.StatusNotFound)
		}
	})
	mktPointAtFake(t, mktFake)
	return mktRequests
}

func mktRound6Partial(t *testing.T, mktOut string) map[string]any {
	t.Helper()
	mktSummary := mktDecode(t, mktOut)
	if mktSummary["status"] != "partial" || mktSummary["resume_hint"] == nil || mktSummary["resume_hint"] == "" {
		t.Fatalf("expected partial snapshot with resume hint: %v", mktSummary)
	}
	if _, mktExists := mktSummary["unresolved_shops"]; mktExists {
		t.Fatalf("unexpected unresolved_shops: %v", mktSummary)
	}
	return mktSummary
}

func TestEtsyMarketFallbackBudgetExhausted(t *testing.T) {
	mktRequests := mktRound6Fallback(t, false)
	mktOut, mktErr := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "2")...)
	if mktErr != nil {
		t.Fatalf("snapshot: %v\n%s", mktErr, mktOut)
	}
	mktRound6Partial(t, mktOut)
	if mktRequests.Load() != 0 {
		t.Fatalf("shop requests=%d, want 0", mktRequests.Load())
	}
}

func TestEtsyMarketFallbackDailyQuota(t *testing.T) {
	mktRequests := mktRound6Fallback(t, true)
	mktOut, mktErr := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "10")...)
	if !client.IsDailyQuotaExhausted(mktErr) {
		t.Fatalf("expected daily quota error, got %v\n%s", mktErr, mktOut)
	}
	mktRound6Partial(t, mktOut)
	if mktRequests.Load() != 1 {
		t.Fatalf("shop requests=%d, want 1", mktRequests.Load())
	}
}

func TestEtsyMarketFallbackStoreWriteErrorIsReturned(t *testing.T) {
	mktRound6Fallback(t, false)
	mktOut, mktErr := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "2")...)
	if mktErr != nil {
		t.Fatalf("snapshot: %v\n%s", mktErr, mktOut)
	}
	mktSummary := mktRound6Partial(t, mktOut)
	mktDB := mktOpenFixtureStore(t)
	_, mktErr = mktDB.DB().Exec(`CREATE TRIGGER etsy_qzx_fail BEFORE INSERT ON market_shops WHEN NEW.shop_id = 555002 BEGIN SELECT RAISE(ABORT, 'qzx fixture write failure'); END;`)
	mktDB.Close()
	if mktErr != nil {
		t.Fatal(mktErr)
	}
	mktOut, mktErr = mktRunCLI(t, mktSnapArgs("--resume", fmt.Sprint(int64(mktSummary["snapshot_id"].(float64))), "--max-calls", "10")...)
	if mktErr == nil || !strings.Contains(mktErr.Error(), "qzx fixture write failure") {
		t.Fatalf("expected store write error, got %v\n%s", mktErr, mktOut)
	}
	mktRound6Partial(t, mktOut)
}
