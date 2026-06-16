package cli

import (
	"bytes"
	"encoding/json"
	"os"
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
