package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-operator-intel/internal/store"
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

func TestAgentContextUsesDescriptorShape(t *testing.T) {
	got, err := run(t, t.TempDir(), "--agent", "agent-context")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("not json: %v\n%s", err, got)
	}
	if doc["schema_version"] != "amazon-operator-intel.agent-context/v1" || doc["local_first"] != true || doc["external_api_calls"] != false {
		t.Fatalf("unexpected context: %#v", doc)
	}
	cmds, ok := doc["commands"].([]any)
	if !ok || len(cmds) < 20 {
		t.Fatalf("missing descriptor commands: %#v", doc["commands"])
	}
	first := cmds[0].(map[string]any)
	if _, ok := first["safe_for_agents"]; !ok || first["description"] == "" {
		t.Fatalf("commands are not descriptor-shaped: %#v", first)
	}
}

func TestDoctorAndSourcesDoNotLeakSecrets(t *testing.T) {
	t.Setenv("AMAZON_ADS_PROFILE_ID", "secret-profile")
	got, err := run(t, t.TempDir(), "sources", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "secret-profile") || !strings.Contains(got, "amazon-seller-pp-cli") {
		t.Fatalf("sources doctor leaked secret or missed source: %s", got)
	}
	got, err = run(t, t.TempDir(), "--agent", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "secret-profile") || !strings.Contains(got, `"profiles"`) {
		t.Fatalf("doctor leaked secret or missed profiles: %s", got)
	}
}

func TestProfileLifecycle(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "profile", "save", "--name", "shop", "--marketplace-id", "ATVPDKIKX0DER", "--seller-id", "SELLER1", "--ads-profile-id", "999", "--days", "45", "--target-acos", "0.25"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "profile", "list")
	if err != nil || !strings.Contains(got, "shop") {
		t.Fatalf("list failed: %v %s", err, got)
	}
	got, err = run(t, home, "profile", "show", "shop")
	if err != nil || !strings.Contains(got, "SELLER1") || strings.Contains(got, "SP_API_REFRESH_TOKEN") {
		t.Fatalf("show failed or leaked credential names as values: %v %s", err, got)
	}
	if _, err := run(t, home, "profile", "delete", "shop"); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultAndImportSyncDoNotExecChildren(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	calls := filepath.Join(home, "calls.log")
	writeStub(t, bin, "amazon-seller-pp-cli", calls, "exit 13")
	writeStub(t, bin, "amazon-ads-pp-cli", calls, "exit 13")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := run(t, home, "sync")
	if err != nil || !strings.Contains(got, "embedded-fixture") {
		t.Fatalf("fixture sync failed: %v %s", err, got)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("default sync executed child CLI")
	}
	importPath := filepath.Join(home, "dataset.json")
	if err := os.WriteFile(importPath, []byte(`{"profile":"demo","source":"local","skus":[{"sku":"LOCAL","asin":"BLOCAL","revenue":10}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "demo", "sync", "--import", importPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("import sync executed child CLI")
	}
}

func TestSourceAllMissingConfigErrorsBeforeChildExec(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	calls := filepath.Join(home, "calls.log")
	writeStub(t, bin, "amazon-seller-pp-cli", calls, "echo '[]'")
	writeStub(t, bin, "amazon-ads-pp-cli", calls, "echo '[]'")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AMAZON_ADS_PROFILE_ID", "")
	got, err := run(t, home, "sync", "--source", "all")
	if err == nil || !strings.Contains(got+err.Error(), "ads_profile_id") {
		t.Fatalf("expected missing config error, got %v %s", err, got)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("source all ran child before config validation")
	}
}

func TestSingleSourceSyncMergesAndPreservesOrphans(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "sync"); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	calls := filepath.Join(home, "calls.log")
	writeStub(t, bin, "amazon-seller-pp-cli", calls, `printf '%s\n' '{"rows":[{"sku":"SELLER-ONLY","asin":"BSELLER","available_quantity":12,"days_of_cover":18,"revenue":1000,"profit":220,"listing_score":88},{"search_term":"seller brand term","sku":"SELLER-ONLY","organic_rank":7}]}'`)
	writeStub(t, bin, "amazon-ads-pp-cli", calls, `printf '%s\n' '{"rows":[{"campaign_id":"cmp-orphan","campaign_name":"Orphan ads","advertised_sku":"ADS-ORPHAN","advertised_asin":"BORPHAN","spend":321,"sales":0,"acos":0},{"search_term":"waste query","advertised_sku":"ADS-ORPHAN","spend":99,"orders":0}]}'`)
	for _, name := range []string{"campaign-performance.csv", "product-performance.csv", "search-term-report.csv"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("header\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := run(t, home, "--profile", "default", "sync", "--source", "seller", "--marketplace-id", "ATVPDKIKX0DER"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, home, "--profile", "default", "sync", "--source", "ads", "--ads-profile-id", "123", "--ads-report-dir", home); err != nil {
		t.Fatal(err)
	}
	d, err := store.New(home).LoadData("default")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.SKUs) < 12 {
		t.Fatalf("single-source sync did not merge with existing fixture: %d", len(d.SKUs))
	}
	foundSeller, foundOrphan := false, false
	for _, s := range d.SKUs {
		if s.SKU == "SELLER-ONLY" && s.Source.Seller.Present {
			foundSeller = true
		}
		if s.SKU == "ADS-ORPHAN" && s.Source.Ads.Present {
			foundOrphan = true
		}
	}
	if !foundSeller || !foundOrphan {
		t.Fatalf("missing merged seller or orphan ads rows: %#v", d.SKUs)
	}
	callBytes, _ := os.ReadFile(calls)
	callsText := string(callBytes)
	if !strings.Contains(callsText, "amazon-seller-pp-cli fba-inventory --agent --marketplace-ids ATVPDKIKX0DER --granularity-type Marketplace --granularity-id ATVPDKIKX0DER") || !strings.Contains(callsText, "amazon-ads-pp-cli search-term-mining --agent --report "+home+"/search-term-report.csv") {
		t.Fatalf("unexpected child args:\n%s", callsText)
	}
}

func TestChildNonZeroSurfacesStderr(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	calls := filepath.Join(home, "calls.log")
	writeStub(t, bin, "amazon-seller-pp-cli", calls, `echo "bad child" >&2; exit 12`)
	t.Setenv("PATH", bin)
	got, err := run(t, home, "sync", "--source", "seller", "--marketplace-id", "ATVPDKIKX0DER")
	if err == nil || !strings.Contains(got+err.Error(), "bad child") {
		t.Fatalf("missing child stderr: %v %s", err, got)
	}
}

func TestRepresentativeParsers(t *testing.T) {
	seller := []map[string]any{{"seller_sku": "SKU1", "asin": "B1", "available_quantity": json.Number("8"), "days_of_cover": json.Number("6.5"), "contribution_margin": json.Number("0.22")}}
	skus := parseSellerSKUs(seller, "seller cmd")
	if len(skus) != 1 || skus[0].SKU != "SKU1" || !skus[0].Source.Seller.Present {
		t.Fatalf("seller parser failed: %#v", skus)
	}
	account := store.Fixture("x").Account
	if account.Score == 0 || !account.Source.Seller.Present {
		t.Fatalf("seller account fixture evidence missing: %#v", account)
	}
	ads := []map[string]any{{"campaign_id": "C1", "campaign_name": "Camp", "advertised_sku": "SKU1", "spend": json.Number("12.5"), "sales": json.Number("50"), "clicks": json.Number("10")}}
	if rows := parseAdsCampaigns(ads, "ads cmd"); len(rows) != 1 || rows[0].CampaignID != "C1" || !rows[0].Source.Ads.Present {
		t.Fatalf("ads campaign parser failed: %#v", rows)
	}
	terms := parseAdsSearchTerms([]map[string]any{{"search_term": "query", "spend": json.Number("9"), "orders": json.Number("0")}}, "terms cmd")
	if len(terms) != 1 || terms[0].Term != "query" {
		t.Fatalf("ads term parser failed: %#v", terms)
	}
}

func TestDateRangeComputesInclusiveDays(t *testing.T) {
	if got := daysFromRange("2026-06-01", "2026-06-07"); got != 7 {
		t.Fatalf("daysFromRange = %d, want 7", got)
	}
	if got := daysFromRange("", "2026-06-07"); got != 0 {
		t.Fatalf("empty start should not override days: %d", got)
	}
}

func TestLocalFileParsers(t *testing.T) {
	dir := t.TempDir()
	cogs := filepath.Join(dir, "cogs.csv")
	if err := os.WriteFile(cogs, []byte("sku,cogs\nDAYPACK-BLK,3.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadCOGS(cogs)
	if err != nil || m["DAYPACK-BLK"] != 3.25 {
		t.Fatalf("bad cogs parse: %v %#v", err, m)
	}
	po := filepath.Join(dir, "po.csv")
	if err := os.WriteFile(po, []byte("po_id,sku,units,unit_cost,expected_ship_date,status\nPOX,SKU1,10,2.50,2026-06-18,ship_window_at_risk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pos, err := loadPurchaseOrders(po)
	if err != nil || len(pos) != 1 || !pos[0].Source.VendorFiles.Present {
		t.Fatalf("bad po parse: %v %#v", err, pos)
	}
	ded := filepath.Join(dir, "deductions.csv")
	if err := os.WriteFile(ded, []byte("id,type,sku,amount,reason,dispute_by,confidence\nD1,chargeback,SKU1,44.5,label,2026-06-30,0.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ds, err := loadVendorDeductions(ded)
	if err != nil || len(ds) != 1 || !ds[0].Source.VendorFiles.Present {
		t.Fatalf("bad deduction parse: %v %#v", err, ds)
	}
	keywords := filepath.Join(dir, "keywords.txt")
	if err := os.WriteFile(keywords, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := loadLines(keywords)
	if err != nil || len(lines) != 2 {
		t.Fatalf("bad keyword parse: %v %#v", err, lines)
	}
}

func TestRequiredCommandsReturnMeaningfulFixtureOutput(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "sync"); err != nil {
		t.Fatal(err)
	}
	checks := [][]string{{"war-room"}, {"restock-or-kill"}, {"ad-spend-guardrail"}, {"sku-profit-truth"}, {"listing-triage"}, {"cash-leaks"}, {"search-term-actions"}, {"digest", "daily"}, {"digest", "weekly"}, {"operator-plan"}, {"cash-calendar"}, {"launch-readiness", "--asin", "B00FIXTURE", "--sku", "FIXTURE-SKU", "--target-acos", "0.25", "--launch-budget", "500", "--inventory-units", "250", "--cogs", "12.50"}, {"rank-defense"}, {"bundle-opportunities"}, {"vendor-ops", "readiness"}, {"vendor-ops", "deductions", "--fixture"}, {"vendor-ops", "po-watch", "--fixture"}, {"vendor-ops", "scorecard", "--fixture"}}
	for _, args := range checks {
		got, err := run(t, home, append([]string{"--agent"}, args...)...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, got)
		}
		if strings.TrimSpace(got) == "" || got == "[]\n" || got == "{}\n" {
			t.Fatalf("%v produced empty output: %s", args, got)
		}
	}
}

func TestAdvancedWorkflowAssertions(t *testing.T) {
	home := t.TempDir()
	if _, err := run(t, home, "sync"); err != nil {
		t.Fatal(err)
	}
	got, err := run(t, home, "--agent", "operator-plan", "--owner", "founder,ops,ads", "--max-actions", "6")
	if err != nil || !strings.Contains(got, `"owner":"founder"`) || !strings.Contains(got, `"today"`) || !strings.Contains(got, `"source_commands"`) {
		t.Fatalf("operator-plan missing required fields: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "cash-calendar")
	if err != nil || !strings.Contains(got, `"cash_crunch_dates"`) || !strings.Contains(got, `"recommended_tradeoffs"`) {
		t.Fatalf("cash-calendar missing pressure/tradeoffs: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "launch-readiness", "--asin", "B00FIXTURE", "--sku", "FIXTURE-SKU", "--target-acos", "0.25", "--launch-budget", "500", "--inventory-units", "250", "--cogs", "12.50")
	if err != nil || (!strings.Contains(got, `"decision":"ready_with_risks"`) && !strings.Contains(got, `"decision":"not_ready_`)) {
		t.Fatalf("launch-readiness missing decision: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "rank-defense")
	if err != nil || !strings.Contains(got, `"defend"`) || !strings.Contains(got, `"reduce"`) || !strings.Contains(got, `"do_not_defend"`) {
		t.Fatalf("rank-defense missing buckets: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "bundle-opportunities")
	if err != nil || !strings.Contains(got, `"decision":"test_bundle"`) || !strings.Contains(got, `"decision":"reject"`) {
		t.Fatalf("bundle opportunities missing accept/reject: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "vendor-ops", "deductions", "--fixture")
	if err != nil || !strings.Contains(got, `"recommendation"`) {
		t.Fatalf("deductions missing dispute ranking: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "vendor-ops", "po-watch", "--fixture")
	if err != nil || !strings.Contains(got, `"risk"`) {
		t.Fatalf("po-watch missing risk: %v %s", err, got)
	}
	got, err = run(t, home, "--agent", "vendor-ops", "scorecard", "--fixture")
	if err != nil || !strings.Contains(got, `"source":"local-import"`) {
		t.Fatalf("scorecard missing local provenance: %v %s", err, got)
	}
}

func writeStub(t *testing.T, dir, name, calls, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + "echo \"" + name + " $*\" >> " + shellQuote(calls) + "\n" + body + "\n"
	if runtime.GOOS == "windows" {
		t.Fatal("shell stubs require unix")
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
