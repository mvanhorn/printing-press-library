package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/store"
	"github.com/spf13/cobra"
)

func TestAutoNegateApplyPostsNegativeKeywords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnvForCLITest(t)

	var gotPath string
	var gotAuth string
	var gotClientID string
	var gotScope string
	var gotBody []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotClientID = r.Header.Get("Amazon-Advertising-API-ClientId")
		gotScope = r.Header.Get("Amazon-Advertising-API-Scope")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	configData := []byte(`base_url = "` + srv.URL + `"
access_token = "access-token"
token_expiry = ` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `
ads_api_client_id = "client-id"
ads_profile_id = "profile-id"
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reportPath := filepath.Join(t.TempDir(), "search_terms.json")
	reportData := []byte(`[{
  "campaign_id": "111",
  "campaign": "Discovery",
  "ad_group_id": "222",
  "ad_group": "Auto",
  "search_term": "bad query",
  "spend": 25,
  "clicks": 30,
  "conversions": 0
}]`)
	if err := os.WriteFile(reportPath, reportData, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "--json", "auto-negate", "--report", reportPath, "--threshold", "15", "--min-clicks", "20", "--apply"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auto-negate --apply returned error: %v\noutput:\n%s", err, out.String())
	}

	if gotPath != "/v2/sp/negativeKeywords" {
		t.Fatalf("path = %q, want /v2/sp/negativeKeywords", gotPath)
	}
	if gotAuth != "Bearer access-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotClientID != "client-id" {
		t.Fatalf("client ID header = %q, want client-id", gotClientID)
	}
	if gotScope != "profile-id" {
		t.Fatalf("scope header = %q, want profile-id", gotScope)
	}
	if len(gotBody) != 1 {
		t.Fatalf("len(body) = %d, want 1: %+v", len(gotBody), gotBody)
	}
	if gotBody[0]["campaignId"] != "111" || gotBody[0]["adGroupId"] != "222" || gotBody[0]["keywordText"] != "bad query" || gotBody[0]["matchType"] != "negativeExact" {
		t.Fatalf("request body = %+v", gotBody[0])
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if envelope["dry_run"] != false || envelope["applied"] != true {
		t.Fatalf("output apply flags = %v", envelope)
	}
}

func TestAutoNegateApplyDryRunDoesNotClaimApplied(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnvForCLITest(t)

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "dry run should not reach server", http.StatusInternalServerError)
	}))
	defer srv.Close()

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	configData := []byte(`base_url = "` + srv.URL + `"
access_token = "access-token"
token_expiry = ` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `
ads_api_client_id = "client-id"
ads_profile_id = "profile-id"
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reportPath := filepath.Join(t.TempDir(), "search_terms.json")
	reportData := []byte(`[{
  "campaign_id": "111",
  "campaign": "Discovery",
  "ad_group_id": "222",
  "ad_group": "Auto",
  "search_term": "bad query",
  "spend": 25,
  "clicks": 30,
  "conversions": 0
}]`)
	if err := os.WriteFile(reportPath, reportData, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "--json", "--dry-run", "auto-negate", "--report", reportPath, "--threshold", "15", "--min-clicks", "20", "--apply"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auto-negate --apply --dry-run returned error: %v\noutput:\n%s", err, out.String())
	}
	if requests != 0 {
		t.Fatalf("server saw %d requests, want 0", requests)
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if envelope["dry_run"] != true || envelope["applied"] != false || envelope["success"] != false {
		t.Fatalf("output apply preview flags = %v", envelope)
	}
	if envelope["status"] != float64(0) || envelope["sent_count"] != float64(1) {
		t.Fatalf("output preview status/count = %v", envelope)
	}
}

func TestAutoNegateApplyVerifyShortCircuitDoesNotClaimApplied(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	clearAmazonAdsEnvForCLITest(t)

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "verify short-circuit should not reach server", http.StatusInternalServerError)
	}))
	defer srv.Close()

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	configData := []byte(`base_url = "` + srv.URL + `"
ads_api_client_id = "client-id"
ads_profile_id = "profile-id"
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reportPath := filepath.Join(t.TempDir(), "search_terms.json")
	reportData := []byte(`[{
  "campaign_id": "111",
  "campaign": "Discovery",
  "ad_group_id": "222",
  "ad_group": "Auto",
  "search_term": "bad query",
  "spend": 25,
  "clicks": 30,
  "conversions": 0
}]`)
	if err := os.WriteFile(reportPath, reportData, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "--json", "auto-negate", "--report", reportPath, "--threshold", "15", "--min-clicks", "20", "--apply"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auto-negate verify --apply returned error: %v\noutput:\n%s", err, out.String())
	}
	if requests != 0 {
		t.Fatalf("server saw %d requests, want 0", requests)
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if envelope["dry_run"] != true || envelope["applied"] != false || envelope["success"] != false || envelope["verify_short_circuit"] != true {
		t.Fatalf("verify short-circuit output flags = %v", envelope)
	}

	db, err := store.OpenWithContext(context.Background(), defaultDBPath("amazon-ads-pp-cli"))
	if err != nil {
		t.Fatalf("open audit store: %v", err)
	}
	defer db.Close()
	audits, err := db.ListAutomationAudits(context.Background(), 1)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 1 || audits[0].Mode != "verify_short_circuit" {
		t.Fatalf("audit = %+v, want one verify_short_circuit record", audits)
	}
}

func TestAutomationApplyBodyGuardrails(t *testing.T) {
	t.Parallel()

	if _, err := autoNegateApplyBody([]adsanalytics.AutoNegatePlan{{SearchTerm: "missing ids"}}); err == nil {
		t.Fatalf("autoNegateApplyBody without IDs returned nil error")
	}
	negativeBody, err := autoNegateApplyBody([]adsanalytics.AutoNegatePlan{
		{CampaignID: "1", AdGroupID: "2", SearchTerm: "Duplicate Term"},
		{CampaignID: "1", AdGroupID: "2", SearchTerm: " duplicate term "},
		{CampaignID: "1", AdGroupID: "2", SearchTerm: "Different Term"},
	})
	if err != nil {
		t.Fatalf("autoNegateApplyBody duplicate case returned error: %v", err)
	}
	if len(negativeBody) != 2 {
		t.Fatalf("autoNegateApplyBody duplicate len = %d, want 2: %+v", len(negativeBody), negativeBody)
	}
	if _, err := autoPromoteApplyBody([]adsanalytics.AutoPromotePlan{{CampaignID: "1", AdGroupID: "2", SearchTerm: "term"}}, 0, 5); err == nil {
		t.Fatalf("autoPromoteApplyBody without bid returned nil error")
	}
	if _, err := autoPromoteApplyBody([]adsanalytics.AutoPromotePlan{{CampaignID: "1", AdGroupID: "2", SearchTerm: "term"}}, 6, 5); err == nil {
		t.Fatalf("autoPromoteApplyBody above max bid returned nil error")
	}
	budgetBody, err := budgetRebalanceApplyBody([]adsanalytics.BudgetRebalancePlan{{CampaignID: "1", Recommended: 42}}, 100)
	if err != nil {
		t.Fatalf("budgetRebalanceApplyBody returned error: %v", err)
	}
	if budgetBody[0]["campaignId"] != "1" || budgetBody[0]["dailyBudget"] != 42.0 {
		t.Fatalf("budget body = %+v", budgetBody[0])
	}
	budgetDeduped, err := budgetRebalanceApplyBody([]adsanalytics.BudgetRebalancePlan{
		{CampaignID: "1", Recommended: 42},
		{CampaignID: "1", Recommended: 45},
	}, 100)
	if err != nil {
		t.Fatalf("budgetRebalanceApplyBody duplicate case returned error: %v", err)
	}
	if len(budgetDeduped) != 1 || budgetDeduped[0]["dailyBudget"] != 42.0 {
		t.Fatalf("budget duplicate body = %+v, want first recommendation only", budgetDeduped)
	}
	if _, err := budgetRebalanceApplyBody([]adsanalytics.BudgetRebalancePlan{{CampaignID: "1", Campaign: "Expensive", Recommended: 125}}, 100); err == nil {
		t.Fatalf("budgetRebalanceApplyBody above max daily budget returned nil error")
	}
	bidBody, err := bidRulesApplyBody([]adsanalytics.BidRulePlan{{KeywordID: "kw-1", RecommendedBid: 1.23}}, 10)
	if err != nil {
		t.Fatalf("bidRulesApplyBody returned error: %v", err)
	}
	if bidBody[0]["keywordId"] != "kw-1" || bidBody[0]["bid"] != 1.23 {
		t.Fatalf("bid body = %+v", bidBody[0])
	}
	bidDeduped, err := bidRulesApplyBody([]adsanalytics.BidRulePlan{
		{KeywordID: "kw-1", Keyword: "term", RecommendedBid: 1.23},
		{KeywordID: "kw-1", Keyword: "term", RecommendedBid: 1.45},
	}, 10)
	if err != nil {
		t.Fatalf("bidRulesApplyBody duplicate case returned error: %v", err)
	}
	if len(bidDeduped) != 1 || bidDeduped[0]["bid"] != 1.23 {
		t.Fatalf("bid duplicate body = %+v, want first bid only", bidDeduped)
	}
	if _, err := bidRulesApplyBody([]adsanalytics.BidRulePlan{{KeywordID: "kw-1", Keyword: "term", RecommendedBid: 11}}, 10); err == nil {
		t.Fatalf("bidRulesApplyBody above max bid returned nil error")
	}
	if err := enforceMaxChanges(2, 1); err == nil {
		t.Fatalf("enforceMaxChanges above max returned nil error")
	}
	out, err := applyAutomationMutation(&cobra.Command{}, &rootFlags{}, nil, "POST", "/v2/sp/keywords", []map[string]any{}, map[string]any{"dry_run": false, "applied": true})
	if err != nil {
		t.Fatalf("empty apply mutation returned error: %v", err)
	}
	if out["skipped"] != true || out["sent_count"] != 0 || out["noop"] != true || out["applied"] != false {
		t.Fatalf("empty apply mutation output = %+v", out)
	}
	if _, hasSuccess := out["success"]; hasSuccess {
		t.Fatalf("empty apply mutation should not claim success: %+v", out)
	}
}
