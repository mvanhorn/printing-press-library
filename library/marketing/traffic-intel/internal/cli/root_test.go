package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
