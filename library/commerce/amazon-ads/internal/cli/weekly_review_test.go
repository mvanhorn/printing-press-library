package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/config"
)

func TestWeeklyReviewVerifySkipsStaleKeywordBid(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/sp/keywords/k1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keywordId":"k1","bid":1.50}`))
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL, AuthHeaderVal: "Bearer test-token"}, time.Second, 0)
	cmd := RootCmd()
	cmd.SetContext(context.Background())
	verified, skipped := verifyWeeklyReviewCurrentState(cmd, c, []adsanalytics.WeeklyReviewAction{
		{
			Type:        "lower_bid",
			Entity:      adsanalytics.ReviewEntity{Level: "keyword", KeywordID: "k1"},
			CurrentBid:  1.20,
			ProposedBid: 0.90,
		},
	})
	if len(verified) != 0 {
		t.Fatalf("verified stale action: %+v", verified)
	}
	if len(skipped) != 1 || skipped[0].Reason != "current bid no longer matches report" {
		t.Fatalf("skipped = %+v", skipped)
	}
}
