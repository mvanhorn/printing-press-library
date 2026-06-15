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
	checks := [][]string{{"money-pages"}, {"query-revenue", "jackets"}, {"explain-drop"}, {"refresh-queue"}, {"digest", "weekly"}}
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
{"pages":[{"url":"/p1","title":"Page One","clicks":"12","impressions":120,"ctr":0.1,"avg_position":3.5,"previous_clicks":20,"query_sample":"blue widgets"}]}
JSON`)
	installFakeChild("google-analytics-pp-cli", `cat <<'JSON'
{"rows":[{"landing_page":"/p1","sessions":30,"transactions":2,"total_revenue":"199.50","previous_sessions":42,"previous_revenue":250},{"landing_page":"/p2","sessions":7,"revenue":15}]}
JSON`)
	installFakeChild("ahrefs-pp-cli", `cat <<'JSON'
{"data":[{"page":"/p1","backlinks":8,"referring_domains":4,"top_keyword":"widgets"}]}
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
