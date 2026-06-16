package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/traffic-intel/internal/store"
)

func run(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	base := []string{"--home", home}
	cmd.SetArgs(append(base, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestAgentModeContextIsJSON(t *testing.T) {
	home := t.TempDir()
	got, err := run(t, home, "--agent", "agent-context")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("not json: %v\n%s", err, got)
	}
	if doc["name"] != "traffic-intel-pp-cli" || doc["external_api_calls"] != false || doc["schema_version"] != "traffic-intel.agent-context/v1" {
		t.Fatalf("unexpected context: %#v", doc)
	}
	if _, ok := doc["source_plan"].([]any); !ok {
		t.Fatalf("missing source_plan: %#v", doc)
	}
	envRows, ok := doc["env"].([]any)
	if !ok {
		t.Fatalf("missing env rows: %#v", doc)
	}
	foundAhrefsTarget := false
	for _, row := range envRows {
		m, _ := row.(map[string]any)
		if m["name"] == "AHREFS_TARGET" {
			foundAhrefsTarget = true
			break
		}
	}
	if !foundAhrefsTarget {
		t.Fatalf("agent context missing AHREFS_TARGET env row: %#v", doc["env"])
	}
}

func TestSourcesDoctorShowsPresenceWithoutSecrets(t *testing.T) {
	t.Setenv("GA4_PROPERTY_ID", "secret-property")
	got, err := run(t, t.TempDir(), "sources", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "GA4_PROPERTY_ID:present") || strings.Contains(got, "secret-property") {
		t.Fatalf("doctor leaked secret or missed presence: %s", got)
	}

	got, err = run(t, t.TempDir(), "--agent", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "secret-property") || !strings.Contains(got, "\"sources\"") {
		t.Fatalf("json doctor leaked secret or missed sources: %s", got)
	}
}

func TestProfileLifecycle(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "profile", "save", "--name", "acme", "--site", "https://example.com", "--ga-property", "123"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "profile", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "acme") {
		t.Fatalf("list missing profile: %s", got)
	}
	got, err = run(t, home, "profile", "show", "acme")
	if err != nil || !strings.Contains(got, "https://example.com") {
		t.Fatalf("show failed: %v %s", err, got)
	}
	if _, err := run(t, home, "profile", "delete", "acme"); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAndAnalysisCommands(t *testing.T) {
	home := t.TempDir()
	if got, err := run(t, home, "--profile", "demo", "sync"); err != nil || !strings.Contains(got, "synced 4 pages") {
		t.Fatalf("sync failed: %v %s", err, got)
	}
	checks := [][]string{{"movers"}, {"confidence"}, {"money-pages"}, {"query-revenue", "jackets"}, {"explain-drop"}, {"refresh-queue"}, {"opportunity-gap"}, {"quick-wins"}, {"revenue-at-risk"}, {"refresh-brief", "jackets"}, {"topic-clusters"}, {"source-coverage"}, {"internal-link-plan"}, {"experiment-plan", "jackets"}, {"forecast-impact"}, {"stale-winners"}, {"digest", "weekly"}}
	for _, args := range checks {
		got, err := run(t, home, append([]string{"--profile", "demo"}, args...)...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, got)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%v produced empty output", args)
		}
	}
	if _, err := run(t, filepath.Join(home, "missing"), "money-pages"); err == nil {
		t.Fatal("expected missing data error")
	}
}

func TestSyncWritesProvenanceSnapshotsAndMovers(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(home, "first.json")
	second := filepath.Join(home, "second.json")
	if err := os.WriteFile(first, []byte(`{
	  "profile": "custom",
	  "synced_at": "2026-06-01T00:00:00Z",
	  "source": "test-first",
	  "pages": [
	    {"url":"/collections/jackets","title":"Jackets","clicks":80,"impressions":8000,"ctr":0.01,"position":22,"sessions":500,"conversions":10,"revenue":5000,"previous_clicks":90,"previous_sessions":520,"previous_revenue":5200,"sources":{"gsc":{"query_sample":"jackets"}}},
	    {"url":"/blog/boots","title":"Boots","clicks":120,"impressions":9000,"ctr":0.013,"position":4,"sessions":600,"conversions":8,"revenue":3000,"previous_clicks":130,"previous_sessions":620,"previous_revenue":3400,"sources":{"gsc":{"query_sample":"boots"}}}
	  ]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(`{
	  "profile": "custom",
	  "synced_at": "2026-06-08T00:00:00Z",
	  "source": "test-second",
	  "pages": [
	    {"url":"/collections/jackets","title":"Jackets","clicks":160,"impressions":9000,"ctr":0.017,"position":12,"sessions":700,"conversions":12,"revenue":6200,"previous_clicks":80,"previous_sessions":500,"previous_revenue":5000,"sources":{"gsc":{"query_sample":"jackets"}}},
	    {"url":"/blog/boots","title":"Boots","clicks":70,"impressions":8500,"ctr":0.008,"position":8,"sessions":420,"conversions":5,"revenue":2100,"previous_clicks":120,"previous_sessions":600,"previous_revenue":3000,"sources":{"gsc":{"query_sample":"boots"}}}
	  ]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "custom", "sync", "--import", first); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "custom", "sync", "--import", second); err != nil {
		t.Fatal(err)
	}
	snaps, err := store.New(home).LatestSnapshots("custom", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected two snapshots, got %#v", snaps)
	}
	if snaps[0].SchemaVersion != store.SnapshotSchemaVersion || snaps[0].InputHashes["import_file"] == "" || snaps[0].DateRange.StartDate == "" {
		t.Fatalf("snapshot missing provenance: %#v", snaps[0])
	}
	got, err := run(t, home, "--profile", "custom", "--agent", "movers")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("movers did not emit JSON: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"new_strike_zone_entrants"`) || !strings.Contains(got, `"new_revenue_at_risk"`) {
		t.Fatalf("movers missing sections: %s", got)
	}
	learning, err := os.ReadFile(store.New(home).LearningsPath("custom"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(learning), "Movers snapshot diff") {
		t.Fatalf("movers did not append learnings: %s", learning)
	}
	got, err = run(t, home, "--profile", "custom", "--agent", "digest", "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"movers"`) || !strings.Contains(got, `"recommended_next_command":"traffic-intel-pp-cli movers --profile custom"`) {
		t.Fatalf("digest did not lead with movers: %s", got)
	}
}

func TestNovelCommandsUseCrossSourceSignals(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(home, "pages.json")
	body := `{
	  "profile": "custom",
	  "source": "test",
	  "pages": [
	    {
	      "url": "/collections/winter-jackets",
	      "title": "Winter Jackets",
	      "clicks": 90,
	      "impressions": 9000,
	      "ctr": 0.01,
	      "position": 6,
	      "sessions": 500,
	      "conversions": 8,
	      "revenue": 4000,
	      "previous_clicks": 150,
	      "previous_sessions": 620,
	      "previous_revenue": 5200,
	      "ref_domains": 12,
	      "sources": {
	        "gsc": {"query_sample": "winter jackets"},
	        "ahrefs": {"top_keyword": "winter jackets"}
	      }
	    },
	    {
	      "url": "/blog/best-winter-jackets",
	      "title": "Best Winter Jackets",
	      "clicks": 70,
	      "impressions": 7000,
	      "ctr": 0.01,
	      "position": 8,
	      "sessions": 300,
	      "conversions": 2,
	      "revenue": 900,
	      "previous_clicks": 95,
	      "previous_sessions": 330,
	      "previous_revenue": 1200,
	      "ref_domains": 6,
	      "sources": {
	        "gsc": {"query_sample": "winter jackets"},
	        "ahrefs": {"top_keyword": "winter jackets"}
	      }
	    },
	    {
	      "url": "/products/trail-shoe",
	      "title": "Trail Shoe",
	      "clicks": 40,
	      "impressions": 5000,
	      "ctr": 0.008,
	      "position": 9,
	      "sessions": 220,
	      "conversions": 5,
	      "revenue": 2000,
	      "previous_clicks": 35,
	      "previous_sessions": 210,
	      "previous_revenue": 1800,
	      "ref_domains": 4,
	      "sources": {
	        "gsc": {"query_sample": "trail shoe"},
	        "ahrefs": {"top_keyword": "trail shoe"}
	      }
	    }
	  ]
	}`
	if err := os.WriteFile(fixture, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "custom", "sync", "--import", fixture); err != nil {
		t.Fatal(err)
	}

	got, err := run(t, home, "--profile", "custom", "--agent", "cannibalization")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"topic":"winter jackets"`) || !strings.Contains(got, `"page_count":2`) {
		t.Fatalf("cannibalization missed competing pages: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "refresh-brief", "winter-jackets")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"likely_issue"`) || !strings.Contains(got, `"recommended_actions"`) {
		t.Fatalf("refresh brief missing agent fields: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "topic-clusters")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"topic":"winter jackets"`) || !strings.Contains(got, `"lost_revenue":1500`) {
		t.Fatalf("topic clusters missed aggregate risk: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "source-coverage")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"complete":3`) || !strings.Contains(got, `"coverage_score":1`) {
		t.Fatalf("source coverage missed complete rows: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "internal-link-plan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"from_url"`) || !strings.Contains(got, `"to_url"`) || !strings.Contains(got, `"anchor":"winter jackets"`) {
		t.Fatalf("internal link plan missing link recommendation: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "experiment-plan", "winter-jackets")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"title_tests"`) || !strings.Contains(got, `"primary_success_metric"`) {
		t.Fatalf("experiment plan missing test fields: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "forecast-impact")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"estimated_click_gain"`) || !strings.Contains(got, `"estimated_revenue_gain"`) {
		t.Fatalf("forecast impact missing estimates: %s", got)
	}

	got, err = run(t, home, "--profile", "custom", "--agent", "stale-winners")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"preventive_action"`) || !strings.Contains(got, `"score"`) {
		t.Fatalf("stale winners missing preventive fields: %s", got)
	}
}

func TestConfidenceGateRefusesThinDerivedMetrics(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(home, "thin.json")
	body := `{"profile":"thin","source":"test","pages":[{"url":"/thin","title":"Thin","clicks":4,"sessions":10,"conversions":0,"revenue":0}]}`
	if err := os.WriteFile(fixture, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "thin", "sync", "--import", fixture); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--profile", "thin", "--agent", "confidence")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"level":"Broken"`) && !strings.Contains(got, `"level":"Low"`) {
		t.Fatalf("expected low/broken confidence: %s", got)
	}
	got, err = run(t, home, "--profile", "thin", "--agent", "forecast-impact")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"refused":true`) || !strings.Contains(got, `"fix_tracking_first"`) {
		t.Fatalf("forecast did not refuse thin tracking: %s", got)
	}
}

func TestSyncLiveChildCLIsNormalizesAndMergesSources(t *testing.T) {
	home := t.TempDir()
	binDir := t.TempDir()
	logPath := filepath.Join(home, "child-args.log")
	installFakeChild := func(name, body string) {
		t.Helper()
		path := filepath.Join(binDir, name)
		script := "#!/bin/sh\n" +
			"printf '%s %s\\n' \"$(basename $0)\" \"$*\" >> \"$CHILD_LOG\"\n" +
			body + "\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installFakeChild("google-search-console-pp-cli", `cat <<'JSON'
{"schema_version":"google-search-console.search-analytics/v1","pages":[{"url":"/p1","title":"Page One","clicks":"12","impressions":120,"ctr":0.1,"avg_position":3.5,"previous_clicks":20,"query_sample":"blue widgets"}]}
JSON`)
	installFakeChild("google-analytics-pp-cli", `cat <<'JSON'
{"schema_version":"google-analytics.top-pages/v1","rows":[{"landing_page":"/p1","sessions":30,"transactions":2,"total_revenue":"199.50","previous_sessions":42,"previous_revenue":250},{"landing_page":"/p2","sessions":7,"revenue":15}]}
JSON`)
	installFakeChild("ahrefs-pp-cli", `cat <<'JSON'
{"schema_version":"ahrefs.top-pages/v1","results":{"pages":[{"page":"/p1","backlinks":8,"referring_domains":4,"top_keyword":"widgets"}]}}
JSON`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CHILD_LOG", logPath)

	got, err := run(t, home, "--profile", "live", "--agent", "sync", "--source", "all", "--site", "sc-domain:example.com", "--ga-property", "123", "--ahrefs-target", "example.com", "--start-date", "2026-06-01", "--end-date", "2026-06-07")
	if err != nil {
		t.Fatalf("sync --source all failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"pages":2`) || !strings.Contains(got, `"source":"child-cli:gsc+ga4+ahrefs"`) {
		t.Fatalf("unexpected sync summary: %s", got)
	}

	d, err := store.New(home).LoadData("live")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Pages) != 2 {
		t.Fatalf("expected merged pages, got %#v", d.Pages)
	}
	byURL := map[string]store.PageMetrics{}
	for _, p := range d.Pages {
		byURL[p.URL] = p
	}
	p1 := byURL["/p1"]
	if p1.Title != "Page One" || p1.Clicks != 12 || p1.PreviousClicks != 20 || p1.Sessions != 30 || p1.Conversions != 2 || p1.Revenue != 199.50 || p1.Backlinks != 8 || p1.RefDomains != 4 {
		t.Fatalf("p1 not normalized/merged: %#v", p1)
	}
	if !strings.Contains(p1.Sources.GSC.ChildCLICommand, "google-search-console-pp-cli webmasters query-search-analytics sc-domain:example.com --agent") || !strings.Contains(p1.Sources.GA4.ChildCLICommand, "google-analytics-pp-cli top-pages --agent --property 123") || !strings.Contains(p1.Sources.Ahrefs.ChildCLICommand, "ahrefs-pp-cli site-explorer top-pages --agent --target example.com") {
		t.Fatalf("missing child CLI provenance: %#v", p1.Sources)
	}
	if p2 := byURL["/p2"]; p2.Sessions != 7 || p2.Revenue != 15 || p2.Clicks != 0 || p2.Backlinks != 0 {
		t.Fatalf("single-source page not preserved: %#v", p2)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	for _, want := range []string{
		"google-search-console-pp-cli webmasters query-search-analytics sc-domain:example.com --agent",
		"google-analytics-pp-cli top-pages --agent --property 123",
		"ahrefs-pp-cli site-explorer top-pages --agent --target example.com",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("child command %q missing from log:\n%s", want, log)
		}
	}
}

func TestSyncAllRequiresEverySourceConfigured(t *testing.T) {
	home := t.TempDir()
	got, err := run(t, home, "--profile", "partial", "--agent", "sync", "--source", "all", "--site", "sc-domain:example.com")
	if err == nil {
		t.Fatalf("sync --source all should require every source, got success: %s", got)
	}
	if !strings.Contains(err.Error(), "missing ga4, ahrefs") {
		t.Fatalf("unexpected error: %v\n%s", err, got)
	}
	if _, loadErr := store.New(home).LoadData("partial"); loadErr == nil {
		t.Fatal("partial all-source sync should not save a dataset")
	}
}

func TestSyncSingleConfiguredSourceIsAllowed(t *testing.T) {
	home := t.TempDir()
	binDir := t.TempDir()
	script := "#!/bin/sh\ncat <<'JSON'\n{\"schema_version\":\"google-search-console.search-analytics/v1\",\"pages\":[{\"url\":\"/only-gsc\",\"clicks\":3,\"impressions\":30}]}\nJSON\n"
	if err := os.WriteFile(filepath.Join(binDir, "google-search-console-pp-cli"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := run(t, home, "--profile", "gsc-only", "--agent", "sync", "--source", "gsc", "--site", "sc-domain:example.com")
	if err != nil {
		t.Fatalf("sync --source gsc failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"source":"child-cli:gsc"`) || !strings.Contains(got, `"pages":1`) {
		t.Fatalf("unexpected single-source summary: %s", got)
	}
}

func TestSyncKnownSourceMissingConfigHasActionableError(t *testing.T) {
	got, err := run(t, t.TempDir(), "--agent", "sync", "--source", "ga4")
	if err == nil {
		t.Fatalf("sync --source ga4 should require GA4 config, got success: %s", got)
	}
	if !strings.Contains(err.Error(), `source "ga4" is not configured`) || !strings.Contains(err.Error(), "GA4_PROPERTY_ID") {
		t.Fatalf("unexpected error: %v\n%s", err, got)
	}
}

func TestSyncDefaultFixtureDoesNotRequireChildCLIs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	got, err := run(t, home, "--profile", "fixture", "sync")
	if err != nil {
		t.Fatalf("fixture sync should not shell out: %v\n%s", err, got)
	}
	if !strings.Contains(got, "embedded-fixture") {
		t.Fatalf("default sync did not preserve fixture mode: %s", got)
	}
}

func TestDigestHandlesEmptyDataset(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(home, "empty.json")
	if err := os.WriteFile(fixture, []byte(`{"profile":"demo","source":"empty-test","pages":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "demo", "sync", "--import", fixture); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--profile", "demo", "--agent", "digest", "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"pages":0`) || !strings.Contains(got, `"top_money_page":""`) {
		t.Fatalf("empty digest not guarded: %s", got)
	}
}
