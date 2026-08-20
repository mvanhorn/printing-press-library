package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/store"
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

func TestWeeklyReviewVerificationUsesTopLevelFields(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"metadata":{"bid":1.20,"campaignId":"wrong"},"bid":1.50,"campaignId":"c1"}`)
	if !jsonNumberMatches(raw, "bid", 1.50) {
		t.Fatalf("jsonNumberMatches did not use top-level bid")
	}
	if jsonNumberMatches(raw, "bid", 1.20) {
		t.Fatalf("jsonNumberMatches matched nested bid")
	}
	if !jsonStringMatches(raw, "campaignId", "c1") {
		t.Fatalf("jsonStringMatches did not use top-level campaignId")
	}
	if jsonStringMatches(raw, "campaignId", "wrong") {
		t.Fatalf("jsonStringMatches matched nested campaignId")
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

func TestWeeklyTargetSourcePrefersExplicitTargetACOS(t *testing.T) {
	t.Parallel()
	if got := weeklyTargetSource(30, 25, ""); got != "target_acos" {
		t.Fatalf("weeklyTargetSource explicit target with gross margin = %q, want target_acos", got)
	}
	if got := weeklyTargetSource(0, 25, ""); got != "gross_margin_pct" {
		t.Fatalf("weeklyTargetSource gross margin fallback = %q, want gross_margin_pct", got)
	}
	if got := weeklyTargetSource(0, 0, "cogs.csv"); got != "cogs_file_average_break_even_acos" {
		t.Fatalf("weeklyTargetSource COGS fallback = %q, want cogs_file_average_break_even_acos", got)
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

func TestWeeklyReviewApplyReturnsPartialResultOnLaterBatchFailure(t *testing.T) {
	clearAmazonAdsEnvForCLITest(t)
	var keywordMutations int
	var negativeMutations int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/sp/keywords/k1":
			_, _ = w.Write([]byte(`{"keywordId":"k1","bid":1.20}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/sp/adGroups/a1":
			_, _ = w.Write([]byte(`{"adGroupId":"a1","campaignId":"c1"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v2/sp/keywords":
			keywordMutations++
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sp/negativeKeywords":
			negativeMutations++
			http.Error(w, `{"error":"failed negative batch"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	configPath := writeWeeklyReviewTestConfig(t, srv.URL)
	cmd := RootCmd()
	cmd.SetContext(context.Background())
	out, err := applyWeeklyReviewActions(cmd, &rootFlags{configPath: configPath}, []adsanalytics.WeeklyReviewAction{
		{
			Type:        "lower_bid",
			Entity:      adsanalytics.ReviewEntity{Level: "keyword", KeywordID: "k1"},
			CurrentBid:  1.20,
			ProposedBid: 0.90,
		},
		{
			Type:   "create_negative_keyword",
			Entity: adsanalytics.ReviewEntity{Level: "search_term", CampaignID: "c1", AdGroupID: "a1", Text: "bad query", MatchType: "negativeExact"},
		},
	}, weeklyApplyOptions{MaxChanges: 10})
	if err == nil {
		t.Fatalf("applyWeeklyReviewActions returned nil error")
	}
	if keywordMutations != 1 || negativeMutations == 0 {
		t.Fatalf("mutation counts keyword=%d negative=%d", keywordMutations, negativeMutations)
	}
	if out == nil || out["partial_failure"] != true {
		t.Fatalf("partial result = %+v, want partial_failure true", out)
	}
	responses, ok := out["responses"].([]map[string]any)
	if !ok || len(responses) != 1 {
		t.Fatalf("responses = %#v, want one successful response", out["responses"])
	}
}

func TestAttachAutomationAuditPayloadIncludesResult(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	cmd := RootCmd()
	cmd.SetContext(context.Background())
	out := map[string]any{
		"partial_failure": true,
		"responses": []map[string]any{
			{"path": "/v2/sp/keywords", "success": true},
		},
	}
	if err := attachAutomationAudit(cmd, out, "weekly-review", "keyword.json,search.json", "apply", []adsanalytics.WeeklyReviewAction{
		{Type: "lower_bid", Entity: adsanalytics.ReviewEntity{KeywordID: "k1"}},
	}, dbPath); err != nil {
		t.Fatalf("attachAutomationAudit returned error: %v", err)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open audit store: %v", err)
	}
	defer db.Close()
	audits, err := db.ListAutomationAudits(context.Background(), 1)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audits = %+v, want one audit", audits)
	}
	var payload map[string]any
	if err := json.Unmarshal(audits[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal audit payload: %v", err)
	}
	result, ok := payload["result"].(map[string]any)
	if !ok || result["partial_failure"] != true {
		t.Fatalf("audit payload result = %#v", payload["result"])
	}
}

func writeWeeklyReviewTestConfig(t *testing.T, baseURL string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	configData := []byte(`base_url = "` + baseURL + `"
access_token = "access-token"
token_expiry = ` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `
ads_api_client_id = "client-id"
ads_profile_id = "profile-id"
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}
