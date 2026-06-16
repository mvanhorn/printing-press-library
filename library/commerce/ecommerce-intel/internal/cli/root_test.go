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
	commands := [][]string{{"dashboard"}, {"opportunities"}, {"action-plan"}, {"money-pages"}, {"money-products"}, {"query-revenue", "boot"}, {"explain-drop"}, {"product-actions"}, {"category-actions"}, {"email-actions"}, {"inventory-risk"}, {"source-coverage"}, {"merchandising-link-plan"}, {"experiment-plan", "boot"}, {"forecast-impact"}, {"restock-winners"}, {"cannibalization"}, {"category-clusters"}, {"digest", "weekly"}, {"geo-audit"}}
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

func TestSyncAllRequiresEverySourceConfigured(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := run(t, t.TempDir(), "--profile", "plan", "sync", "--source", "all")
	if err == nil {
		t.Fatalf("sync --source all should require config, got success: %s", got)
	}
	if !strings.Contains(err.Error(), "missing shopify") || !strings.Contains(err.Error(), "ga4") || !strings.Contains(err.Error(), "gsc") || !strings.Contains(err.Error(), "ahrefs") {
		t.Fatalf("unexpected missing-config error: %v\n%s", err, got)
	}
	if got, err := run(t, t.TempDir(), "sync", "--source", "nonsense"); err == nil || !strings.Contains(got, "invalid --source") {
		t.Fatalf("expected invalid source error, got err=%v output=%s", err, got)
	}
}

func TestSyncDefaultFixtureDoesNotExec(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	got, err := run(t, home, "--profile", "fixture", "sync")
	if err != nil {
		t.Fatalf("default sync should stay offline: %v\n%s", err, got)
	}
	if !strings.Contains(got, "embedded-shopify-commerce-fixture") {
		t.Fatalf("fixture source not preserved: %s", got)
	}
}

func TestSyncLiveChildCLIsMergesSources(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "--profile", "live", "sync"); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	logPath := filepath.Join(home, "child-args.log")
	installFakeChild := func(name, body string) {
		t.Helper()
		path := filepath.Join(binDir, name)
		script := "#!/bin/sh\nprintf '%s %s\\n' \"$(basename $0)\" \"$*\" >> \"$CHILD_LOG\"\n" + body + "\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installFakeChild("shopify-pp-cli", `cat <<'JSON'
{"products":[{"id":"gid://shopify/Product/1","handle":"waterproof-hiking-boot","title":"Waterproof Hiking Boot","url":"/products/waterproof-hiking-boot","vendor":"Trail","product_type":"boots","price":"199","inventory_quantity":77},{"handle":"new-sock","title":"New Sock","url":"/products/new-sock","price":12,"inventory":40}]}
JSON`)
	installFakeChild("klaviyo-pp-cli", `cat <<'JSON'
{"flows":[{"name":"Abandoned Checkout","revenue":9300,"recipients":1000,"open_rate":0.5,"click_rate":0.1,"conversion_rate":0.04,"attributed_sales":90},{"product_handle":"waterproof-hiking-boot","email_revenue":333,"email_clicks":22}]}
JSON`)
	installFakeChild("google-analytics-pp-cli", `cat <<'JSON'
{"rows":[{"landing_page":"/products/waterproof-hiking-boot","sessions":111,"total_revenue":2222,"previous_sessions":90,"previous_revenue":1800},{"landing_page":"/collections/jackets","sessions":22,"revenue":44}]}
JSON`)
	installFakeChild("google-search-console-pp-cli", `cat <<'JSON'
{"rows":[{"keys":["/products/waterproof-hiking-boot"],"clicks":55,"previous_clicks":44,"position":3.2}]}
JSON`)
	installFakeChild("ahrefs-pp-cli", `cat <<'JSON'
{"results":{"pages":[{"url":"/products/waterproof-hiking-boot","backlinks":7,"referring_domains":3}]}}
JSON`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CHILD_LOG", logPath)

	got, err := run(t, home, "--profile", "live", "--agent", "sync", "--source", "all", "--shop", "demo.myshopify.com", "--klaviyo-account", "acct", "--ga-property", "123", "--site", "sc-domain:example.com", "--ahrefs-target", "example.com", "--start-date", "2026-06-01", "--end-date", "2026-06-07")
	if err != nil {
		t.Fatalf("sync --source all failed: %v\n%s", err, got)
	}
	if !strings.Contains(got, `"source":"child-cli:shopify+klaviyo+ga4+gsc+ahrefs"`) {
		t.Fatalf("unexpected live sync summary: %s", got)
	}
	d, err := store.New(home).LoadData("live")
	if err != nil {
		t.Fatal(err)
	}
	var boot store.Product
	for _, p := range d.Products {
		if p.Handle == "waterproof-hiking-boot" {
			boot = p
		}
	}
	if boot.Price != 199 || boot.Inventory != 77 || boot.Sessions != 111 || boot.Revenue != 2222 || boot.SearchClicks != 55 || boot.Backlinks != 7 || boot.EmailRevenue != 333 || !boot.Source.Shopify.Synced || !boot.Source.Klaviyo.Synced || !boot.Source.GA4.Synced || !boot.Source.GSC.Synced || !boot.Source.Ahrefs.Synced {
		t.Fatalf("boot not merged with provenance: %#v", boot)
	}
	if len(d.Pages) < 3 {
		t.Fatalf("expected orphan/non-product pages to be preserved, got %#v", d.Pages)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	for _, want := range []string{"shopify-pp-cli products list --agent --all --data-source live", "klaviyo-pp-cli report flow-comparison --agent --data-source live", "google-analytics-pp-cli top-pages --agent --property 123", "google-search-console-pp-cli webmasters query-search-analytics sc-domain:example.com --agent", "ahrefs-pp-cli site-explorer top-pages --agent --target example.com"} {
		if !strings.Contains(log, want) {
			t.Fatalf("child command %q missing from log:\n%s", want, log)
		}
	}
}

func TestParseChildEntityAliases(t *testing.T) {
	cases := []struct {
		source string
		body   string
		check  func(t *testing.T, got childEntities)
	}{
		{"shopify", `{"products":[{"handle":"alpha","title":"Alpha","variants":[{"price":"42","inventory_quantity":5}]}]}`, func(t *testing.T, got childEntities) {
			if len(got.Products) != 1 || got.Products[0].Handle != "alpha" || got.Products[0].Price != 42 {
				t.Fatalf("bad shopify parse: %#v", got)
			}
		}},
		{"klaviyo", `{"rows":[{"flow_name":"Welcome","attributed_revenue":"120.50","click_rate":"0.04"}]}`, func(t *testing.T, got childEntities) {
			if len(got.Emails) != 1 || got.Emails[0].Name != "Welcome" || got.Emails[0].Revenue != 120.50 {
				t.Fatalf("bad klaviyo parse: %#v", got)
			}
		}},
		{"ga4", `{"rows":[{"landingPagePlusQueryString":"/products/alpha","sessions":"9","totalRevenue":"81"}]}`, func(t *testing.T, got childEntities) {
			if len(got.Pages) != 1 || got.Pages[0].Sessions != 9 || got.Products[0].Handle != "alpha" {
				t.Fatalf("bad ga4 parse: %#v", got)
			}
		}},
		{"gsc", `{"rows":[{"keys":["/products/alpha"],"clicks":7,"avg_position":4.5}]}`, func(t *testing.T, got childEntities) {
			if len(got.Pages) != 1 || got.Pages[0].SearchClicks != 7 || got.Pages[0].SearchPosition != 4.5 {
				t.Fatalf("bad gsc parse: %#v", got)
			}
		}},
		{"ahrefs", `{"results":{"pages":[{"page":"/products/alpha","backlinks":8,"ref_domains":3}]}}`, func(t *testing.T, got childEntities) {
			if len(got.Pages) != 1 || got.Pages[0].Backlinks != 8 || got.Pages[0].RefDomains != 3 {
				t.Fatalf("bad ahrefs parse: %#v", got)
			}
		}},
	}
	for _, tc := range cases {
		got, err := parseChildEntities(tc.source, []byte(tc.body))
		if err != nil {
			t.Fatalf("%s parse failed: %v", tc.source, err)
		}
		tc.check(t, got)
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
