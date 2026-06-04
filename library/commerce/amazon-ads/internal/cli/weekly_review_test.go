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

func TestWeeklyReviewCurrencyFlag(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	cmd, remaining, err := root.Find([]string{"weekly-review"})
	if err != nil {
		t.Fatalf("Find weekly-review returned error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("Find weekly-review returned remaining args %v", remaining)
	}
	flag := cmd.Flags().Lookup("currency")
	if flag == nil {
		t.Fatalf("weekly-review --currency flag missing")
	}
	if flag.DefValue != "USD" {
		t.Fatalf("--currency default = %q, want USD", flag.DefValue)
	}
}

func TestWeeklyReviewMutationBatchesDeduplicateEntities(t *testing.T) {
	t.Parallel()
	batches := weeklyReviewMutationBatches([]adsanalytics.WeeklyReviewAction{
		{Type: "lower_bid", Entity: adsanalytics.ReviewEntity{KeywordID: "k1"}, ProposedBid: 1.10},
		{Type: "raise_bid", Entity: adsanalytics.ReviewEntity{KeywordID: "k1"}, ProposedBid: 1.20},
		{Type: "adjust_budget", Entity: adsanalytics.ReviewEntity{CampaignID: "c1"}, ProposedBudget: 50},
		{Type: "adjust_budget", Entity: adsanalytics.ReviewEntity{CampaignID: "c1"}, ProposedBudget: 60},
		{Type: "create_negative_keyword", Entity: adsanalytics.ReviewEntity{CampaignID: "c1", AdGroupID: "a1", Text: "bad query", MatchType: "negativeExact"}},
		{Type: "create_negative_keyword", Entity: adsanalytics.ReviewEntity{CampaignID: "c1", AdGroupID: "a1", Text: "bad query", MatchType: "negativeExact"}},
	})
	if len(batches) != 3 {
		t.Fatalf("batches = %+v, want 3", batches)
	}
	for _, batch := range batches {
		if len(batch.Body) != 1 {
			t.Fatalf("batch %s %s body = %+v, want one deduped row", batch.Method, batch.Path, batch.Body)
		}
	}
}
