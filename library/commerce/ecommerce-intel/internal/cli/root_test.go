package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
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

func TestAgentModeContextIsJSON(t *testing.T) {
	got, err := run(t, t.TempDir(), "--agent", "agent-context")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("not json: %v\n%s", err, got)
	}
	if doc["name"] != "ecommerce-intel-pp-cli" || doc["external_api_calls"] != false || doc["schema_version"] != "ecommerce-intel.agent-context/v1" {
		t.Fatalf("unexpected context: %#v", doc)
	}
	if !strings.Contains(got, "ChatGPT") || !strings.Contains(got, "Perplexity") || !strings.Contains(got, "Google AI Overviews") {
		t.Fatalf("missing GEO answer engines: %s", got)
	}
}

func TestSourcesDoctorShowsPresenceWithoutSecrets(t *testing.T) {
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "secret-token")
	got, err := run(t, t.TempDir(), "sources", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "SHOPIFY_ACCESS_TOKEN:present") || strings.Contains(got, "secret-token") {
		t.Fatalf("doctor leaked secret or missed presence: %s", got)
	}
	got, err = run(t, t.TempDir(), "--agent", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "secret-token") || !strings.Contains(got, `"sources"`) {
		t.Fatalf("json doctor leaked secret or missed sources: %s", got)
	}
}

func TestProfileLifecycle(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "profile", "save", "--name", "acme", "--shopify-shop", "acme.myshopify.com", "--ga-property", "123", "--klaviyo-account", "k1"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "profile", "list")
	if err != nil || !strings.Contains(got, "acme") {
		t.Fatalf("list failed: %v %s", err, got)
	}
	got, err = run(t, home, "profile", "show", "acme")
	if err != nil || !strings.Contains(got, "acme.myshopify.com") || !strings.Contains(got, "k1") {
		t.Fatalf("show failed: %v %s", err, got)
	}
	if _, err := run(t, home, "profile", "delete", "acme"); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAndCommerceCommands(t *testing.T) {
	home := t.TempDir()
	got, err := run(t, home, "--profile", "demo", "sync")
	if err != nil || !strings.Contains(got, "synced 3 products") {
		t.Fatalf("sync failed: %v %s", err, got)
	}
	commands := [][]string{{"dashboard"}, {"opportunities"}, {"action-plan"}, {"money-pages"}, {"money-products"}, {"query-revenue", "boot"}, {"explain-drop"}, {"product-actions"}, {"category-actions"}, {"email-actions"}, {"inventory-risk"}, {"digest", "weekly"}, {"geo-audit"}}
	for _, args := range commands {
		got, err := run(t, home, append([]string{"--profile", "demo"}, args...)...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, got)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%v produced empty output", args)
		}
	}
	opps, err := run(t, home, "--profile", "demo", "opportunities")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(opps, "%!s(<nil>)") || !strings.Contains(opps, "inventory	") {
		t.Fatalf("opportunities formatting/type regression: %s", opps)
	}
	if _, err := run(t, filepath.Join(home, "missing"), "money-products"); err == nil {
		t.Fatal("expected missing data error")
	}
}

func TestImportDataset(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(home, "dataset.json")
	d := store.Fixture("imported")
	d.Products = d.Products[:1]
	b, _ := json.Marshal(d)
	if err := os.WriteFile(fixture, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--profile", "imported", "--agent", "sync", "--import", fixture)
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"products":1`) {
		t.Fatalf("unexpected import summary: %s", got)
	}
}

func TestLivePlanDoesNotRequireChildCLIs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := run(t, t.TempDir(), "--profile", "plan", "sync", "--source", "all")
	if err != nil {
		t.Fatalf("planned sync should not shell out: %v\n%s", err, got)
	}
	if !strings.Contains(got, "child-cli-plan:all") {
		t.Fatalf("source plan not preserved: %s", got)
	}
	if got, err := run(t, t.TempDir(), "sync", "--source", "nonsense"); err == nil || !strings.Contains(got, "invalid --source") {
		t.Fatalf("expected invalid source error, got err=%v output=%s", err, got)
	}
}

func TestGEOAuditJSON(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "sync"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--agent", "geo-audit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"answer_engines"`) || !strings.Contains(got, "llms.txt") {
		t.Fatalf("missing GEO fields: %s", got)
	}
}
