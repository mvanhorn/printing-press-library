package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/ads-intel/internal/store"
)

func run(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--home", home}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestAgentContextAndCommandSmoke(t *testing.T) {
	home := t.TempDir()
	if got, err := run(t, home, "--agent", "agent-context"); err != nil || !strings.Contains(got, "ads-intel.agent-context/v1") {
		t.Fatalf("agent context failed: %v %s", err, got)
	}
	if got, err := run(t, home, "--profile", "demo", "sync"); err != nil || !strings.Contains(got, "synced") {
		t.Fatalf("sync failed: %v %s", err, got)
	}
	for _, args := range [][]string{{"account-status"}, {"confidence"}, {"audit"}, {"quick-wins"}, {"budget-shift"}} {
		got, err := run(t, home, append([]string{"--profile", "demo", "--agent"}, args...)...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, got)
		}
		if !strings.Contains(got, `"account_status"`) {
			t.Fatalf("%v missing account status header: %s", args, got)
		}
	}
}

func TestAuditCatalogCoverageAndDeterminism(t *testing.T) {
	catalog, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for platform, weights := range catalog.Categories {
		total := 0.0
		for _, weight := range weights {
			total += weight
		}
		if total != 100 {
			t.Fatalf("%s weights must sum to 100, got %.1f", platform, total)
		}
	}
	ids := catalogCheckIDs()
	for _, want := range []string{"google_wasted_spend_terms", "google_zero_conversion_keywords", "meta_learning_phase_guard", "tracking_cost_rows_present"} {
		if !ids[want] {
			t.Fatalf("catalog missing expected check %s", want)
		}
	}
	d := store.Fixture("demo")
	first, err := runAudit(d, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	second, err := runAudit(d, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if first.Score != second.Score || first.Status != second.Status || len(first.Findings) != len(second.Findings) {
		t.Fatalf("audit scoring is not deterministic: %#v %#v", first, second)
	}
	if len(first.QuickWins) == 0 {
		t.Fatalf("expected quick wins from fixture: %#v", first)
	}
}

func TestHealthScoreIsBoundedAndWeighted(t *testing.T) {
	catalog, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// All-pass over evaluated checks => 100.
	allPass := map[string]string{
		"google_wasted_spend_terms":       "pass",
		"amazon_wasted_spend_terms":       "pass",
		"google_zero_conversion_keywords": "pass",
		"meta_frequency_fatigue":          "pass",
		"tracking_cost_rows_present":      "pass",
		"google_broad_smart_bidding":      "na",
		"google_shared_negative_lists":    "na",
		"meta_learning_phase_guard":       "na",
	}
	if got := healthScore(catalog, allPass); got != 100 {
		t.Fatalf("all-pass health score should be 100, got %.2f", got)
	}
	// A critical fail must bound the score in [0,100) and below all-pass.
	withFail := map[string]string{}
	for k, v := range allPass {
		withFail[k] = v
	}
	withFail["tracking_cost_rows_present"] = "fail"
	got := healthScore(catalog, withFail)
	if got < 0 || got >= 100 {
		t.Fatalf("score with a fail must be in [0,100), got %.2f", got)
	}
	// Category weights must actually move the score: a fail on a heavier-weighted
	// check (critical) should not be silently ignored.
	if got == 100 {
		t.Fatalf("category/severity weights are not consumed by scoring")
	}
}

func TestNegativeKeywordDraftArtifactsAreLocalOnly(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "--profile", "demo", "sync"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--profile", "demo", "--agent", "audit")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("audit JSON parse failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"negative_keyword_drafts"`) {
		t.Fatalf("audit missing draft paths: %s", got)
	}
	entries, err := os.ReadDir(home + "/drafts/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected local draft artifacts")
	}
}

type mockGoogleAdsApplyClient struct {
	existing      []negativeCriterion
	added         []resolvedNegativeTarget
	addErrForTerm string
	verifyMissing bool
}

func (m *mockGoogleAdsApplyClient) ResolveNegativeTarget(d negativeDraft, dset store.DataSet) (resolvedNegativeTarget, error) {
	t := resolvedNegativeTarget{
		CustomerID:   d.Target.CustomerID,
		Scope:        "ad_group",
		CampaignID:   d.Target.CampaignID,
		CampaignName: d.Target.CampaignName,
		AdGroupID:    d.Target.AdGroupID,
		AdGroupName:  d.Target.AdGroupName,
		KeywordText:  d.Target.KeywordText,
		MatchType:    "EXACT",
	}
	if t.CustomerID == "" {
		t.CustomerID = dset.Account.AccountID
	}
	if t.AdGroupID == "" {
		t.Scope = "campaign"
	}
	return t, nil
}

func (m *mockGoogleAdsApplyClient) ListExactNegatives(target resolvedNegativeTarget) ([]negativeCriterion, error) {
	if m.verifyMissing && len(m.added) > 0 {
		return nil, nil
	}
	out := append([]negativeCriterion{}, m.existing...)
	for _, added := range m.added {
		out = append(out, negativeCriterion{ResourceName: "customers/" + added.CustomerID + "/adGroupCriteria/mock-" + added.KeywordText, Scope: added.Scope, CampaignID: added.CampaignID, AdGroupID: added.AdGroupID, KeywordText: added.KeywordText, MatchType: "EXACT"})
	}
	return out, nil
}

func (m *mockGoogleAdsApplyClient) AddExactNegative(target resolvedNegativeTarget) (mutationResult, error) {
	if target.KeywordText == m.addErrForTerm {
		return mutationResult{}, os.ErrPermission
	}
	m.added = append(m.added, target)
	return mutationResult{ResourceName: "customers/" + target.CustomerID + "/adGroupCriteria/mock-" + target.KeywordText}, nil
}

func (m *mockGoogleAdsApplyClient) RemoveNegative(customerID, scope, resourceName string) (mutationResult, error) {
	return mutationResult{ResourceName: resourceName}, nil
}

func withMockGoogleAdsClient(t *testing.T, m *mockGoogleAdsApplyClient) {
	t.Helper()
	old := newGoogleAdsApplyClient
	newGoogleAdsApplyClient = func(binary string) googleAdsApplyClient { return m }
	t.Cleanup(func() { newGoogleAdsApplyClient = old })
}

func syncedDraft(t *testing.T, home string) string {
	t.Helper()
	if _, err := run(t, home, "--profile", "demo", "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "demo", "audit"); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(home, "drafts", "demo", "google_negative_keyword_candidate.json")
}

func TestApplyNegativeKeywordDryRunPrintsPlanAndWritesNothing(t *testing.T) {
	home := t.TempDir()
	draft := syncedDraft(t, home)
	mock := &mockGoogleAdsApplyClient{}
	withMockGoogleAdsClient(t, mock)

	got, err := run(t, home, "--profile", "demo", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account")
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, got)
	}
	for _, want := range []string{"mode: dry-run", "customers-ad-group-criteria", "keyword=[free hiking boots]", "inverse:", "DRY-RUN: no writes executed."} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, got)
		}
	}
	if len(mock.added) != 0 {
		t.Fatalf("dry-run executed a mutation: %#v", mock.added)
	}
	if _, err := os.Stat(filepath.Join(home, "audit", "apply.log")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write audit log, stat err=%v", err)
	}
}

func TestApplyNegativeKeywordIdempotencySkipsExisting(t *testing.T) {
	home := t.TempDir()
	draft := syncedDraft(t, home)
	mock := &mockGoogleAdsApplyClient{existing: []negativeCriterion{{ResourceName: "customers/demo-account/adGroupCriteria/existing", Scope: "ad_group", AdGroupID: "ag-22", KeywordText: "free hiking boots", MatchType: "EXACT"}}}
	withMockGoogleAdsClient(t, mock)

	got, err := run(t, home, "--profile", "demo", "--agent", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account", "--live-approved", "--confirm", applyConfirmPrefix+"demo-account")
	if err != nil {
		t.Fatalf("idempotent apply failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"status":"skipped"`) {
		t.Fatalf("expected skipped status: %s", got)
	}
	if len(mock.added) != 0 {
		t.Fatalf("idempotent apply should not mutate: %#v", mock.added)
	}
}

func TestApplyNegativeKeywordRefusesLowConfidence(t *testing.T) {
	home := t.TempDir()
	low := store.Fixture("low")
	for i := range low.Campaigns {
		low.Campaigns[i].Conversions = 0
		low.Campaigns[i].Revenue = 0
		low.Campaigns[i].Impressions = 0
	}
	low.Account.AccountID = "demo-account"
	if err := store.New(home).SaveData(low); err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(home, "drafts", "low", "draft.json")
	if err := os.MkdirAll(filepath.Dir(draft), 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"draft_type": "negative_keyword_candidate", "read_only": true, "finding": finding{ID: "google_negative_keyword_candidate", Platform: "google", Title: "Negative keyword draft: free hiking boots"}, "target": negativeDraftTarget{Platform: "google", CustomerID: "demo-account", CampaignID: "g-2", AdGroupID: "ag-22", KeywordText: "free hiking boots", MatchType: "EXACT"}, "evidence": negativeDraftEvidence{Spend: 18.5, Conversions: 0}}
	b, _ := json.Marshal(body)
	if err := os.WriteFile(draft, b, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := run(t, home, "--profile", "low", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account")
	if err == nil || !strings.Contains(err.Error(), "tracking confidence") {
		t.Fatalf("expected confidence refusal, err=%v output=%s", err, got)
	}
}

func TestApplyNegativeKeywordRefusesAllowlistAndCap(t *testing.T) {
	home := t.TempDir()
	draft := syncedDraft(t, home)
	withMockGoogleAdsClient(t, &mockGoogleAdsApplyClient{})

	if got, err := run(t, home, "--profile", "demo", "apply", "negative-keyword", "--draft", draft); err == nil || !strings.Contains(err.Error(), "not in the apply allowlist") {
		t.Fatalf("expected allowlist refusal, err=%v output=%s", err, got)
	}
	if got, err := run(t, home, "--profile", "demo", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account", "--max-changes-per-run", "0"); err == nil || !strings.Contains(err.Error(), "max-changes-per-run") {
		t.Fatalf("expected cap refusal, err=%v output=%s", err, got)
	}
}

func TestApplyNegativeKeywordRefusesLiveApprovedWithoutConfirm(t *testing.T) {
	home := t.TempDir()
	draft := syncedDraft(t, home)
	mock := &mockGoogleAdsApplyClient{}
	withMockGoogleAdsClient(t, mock)

	got, err := run(t, home, "--profile", "demo", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account", "--live-approved")
	if err == nil || !strings.Contains(err.Error(), "typed confirmation") {
		t.Fatalf("expected typed confirmation refusal, err=%v output=%s", err, got)
	}
	if len(mock.added) != 0 {
		t.Fatalf("live-approved without confirm should not mutate: %#v", mock.added)
	}
}

func TestApplyNegativeKeywordPartialFailureContinues(t *testing.T) {
	home := t.TempDir()
	first := syncedDraft(t, home)
	stored, err := store.New(home).LoadData("demo")
	if err != nil {
		t.Fatal(err)
	}
	stored.SearchTerms = append(stored.SearchTerms, store.SearchTerm{Platform: "google", CampaignID: "g-2", CampaignName: "Nonbrand Prospecting", AdGroupID: "ag-22", AdGroupName: "Prospecting Terms", Term: "bad free boots", Spend: 21, Conversions: 0, Clicks: 19})
	if err := store.New(home).SaveData(stored); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(home, "drafts", "demo", "second.json")
	b, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	doc["target"].(map[string]any)["keyword_text"] = "bad free boots"
	doc["finding"].(map[string]any)["title"] = "Negative keyword draft: bad free boots"
	b, _ = json.Marshal(doc)
	if err := os.WriteFile(second, b, 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &mockGoogleAdsApplyClient{addErrForTerm: "bad free boots"}
	withMockGoogleAdsClient(t, mock)

	got, err := run(t, home, "--profile", "demo", "--agent", "apply", "negative-keyword", "--draft", first, "--draft", second, "--allow-account", "demo-account", "--max-changes-per-run", "2", "--live-approved", "--confirm", applyConfirmPrefix+"demo-account")
	if err != nil {
		t.Fatalf("partial failure command should return summarized results, not abort: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"status":"applied"`) || !strings.Contains(got, `"status":"failed"`) {
		t.Fatalf("expected applied and failed statuses: %s", got)
	}
	if len(mock.added) != 1 {
		t.Fatalf("expected one successful mutation, got %#v", mock.added)
	}
}

func TestApplyNegativeKeywordReadAfterWriteVerificationFailure(t *testing.T) {
	home := t.TempDir()
	draft := syncedDraft(t, home)
	mock := &mockGoogleAdsApplyClient{verifyMissing: true}
	withMockGoogleAdsClient(t, mock)

	got, err := run(t, home, "--profile", "demo", "--agent", "apply", "negative-keyword", "--draft", draft, "--allow-account", "demo-account", "--live-approved", "--confirm", applyConfirmPrefix+"demo-account")
	if err != nil {
		t.Fatalf("verification failure should be reported in result body, not abort: %v\n%s", err, got)
	}
	if !strings.Contains(got, "read-after-write verification failed") {
		t.Fatalf("expected verification failure: %s", got)
	}
}
