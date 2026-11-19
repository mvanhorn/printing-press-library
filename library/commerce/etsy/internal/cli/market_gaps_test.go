// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketGapsHelpWires smoke-tests that the market gaps command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketGapsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market gaps --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "gaps"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market gaps --help missing %q in output:\n%s", want, help)
		}
	}
}

const mktTestRules = `# synthetic classes
targets:
  - name: "linen apron"
    include: ["apron", 'pinafore']
    exclude:
      - "toddler"
  - name: "ceramic planter"
    include: ["planter"]
  - name: "wool scarf"
    include: ["scarf", "shawl"] # trailing comment
`

func mktGapsFixture(t *testing.T) string {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	snap := mktFxSnapshot(t, d, time.Now(), []string{"linen apron", "ceramic mug"}, 2, "complete", mktFxProg{})
	mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 5001, title: "Natural LINEN Apron", price: 2400, tags: []string{"kitchen"}})
	mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: 2, id: 1000000002, shop: 5001, title: "Toddler Apron", price: 2000, tags: []string{"kids"}})
	mktFxResult(t, d, snap, mktFxListing{term: "ceramic mug", pos: 1, id: 1000000003, shop: 5002, title: "Speckled Mug", price: 1800, tags: []string{"Pinafore Style"}})
	db.Close()
	dir := t.TempDir()
	p := filepath.Join(dir, "classes.yaml")
	if err := os.WriteFile(p, []byte(mktTestRules), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func mktGapByName(t *testing.T, out, name string) map[string]any {
	t.Helper()
	for _, x := range mktDecode(t, out)["results"].([]any) {
		if x.(map[string]any)["name"] == name {
			return x.(map[string]any)
		}
	}
	t.Fatalf("target %q missing: %s", name, out)
	return nil
}

func TestMarketGapsExactZeroStatementAndMatching(t *testing.T) {
	rules := mktGapsFixture(t)
	out, err := mktRunCLI(t, "market", "gaps", "--rules", rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	zero := mktGapByName(t, out, "wool scarf")
	if zero["statement"] != "no listing found in 2 pages of 2 terms" {
		t.Errorf("statement = %q", zero["statement"])
	}
	if zero["matching_listings"].(float64) != 0 {
		t.Errorf("matching = %v", zero["matching_listings"])
	}
	if strings.Contains(strings.ToLower(out), "no competition") {
		t.Error("output must never claim 'no competition'")
	}
	// Case-insensitive title match; "Toddler Apron" excluded; tag text matches too (pinafore in a mug's tags).
	apron := mktGapByName(t, out, "linen apron")
	if apron["matching_listings"].(float64) != 2 || apron["distinct_shops"].(float64) != 2 {
		t.Errorf("apron = %v, want 2 listings (title + tag match, toddler excluded) from 2 shops", apron)
	}
	if _, has := mktDecode(t, out)["gap_claims_downgraded"]; has {
		t.Error("no --control given, so no downgrade field expected")
	}
}

func TestMarketGapsControlDowngrade(t *testing.T) {
	rules := mktGapsFixture(t)
	control := filepath.Join(t.TempDir(), "ids.txt")
	if err := os.WriteFile(control, []byte("# known visible\n1000000001\n1000000003\n1000000777\n1000000888\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := mktRunCLI(t, "market", "gaps", "--rules", rules, "--control", control)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	m := mktDecode(t, out)
	if m["control_reached"].(float64) != 0.5 {
		t.Errorf("control_reached = %v, want 0.5", m["control_reached"])
	}
	if m["gap_claims_downgraded"] != true || m["note"] == nil {
		t.Errorf("want downgrade with a note: %v", m)
	}
	// Full coverage: no downgrade.
	full := filepath.Join(t.TempDir(), "full.txt")
	_ = os.WriteFile(full, []byte("1000000001\n1000000002\n"), 0o600)
	out, err = mktRunCLI(t, "market", "gaps", "--rules", rules, "--control", full)
	if err != nil {
		t.Fatal(err)
	}
	m = mktDecode(t, out)
	if m["control_reached"].(float64) != 1.0 || m["gap_claims_downgraded"] != false {
		t.Errorf("full control: %v", m)
	}
}

func TestMarketGapsRulesErrorsAndEmptyStore(t *testing.T) {
	mktTestEnv(t)
	if _, err := mktRunCLI(t, "market", "gaps"); err == nil {
		t.Error("missing --rules should be a usage error")
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	_ = os.WriteFile(bad, []byte("targets:\n  - name: x\n    include: [\"(unclosed\"]\n"), 0o600)
	if _, err := mktRunCLI(t, "market", "gaps", "--rules", bad); err == nil {
		t.Error("invalid regex should be rejected")
	}
	good := filepath.Join(t.TempDir(), "good.yaml")
	_ = os.WriteFile(good, []byte(mktTestRules), 0o600)
	out, err := mktRunCLI(t, "market", "gaps", "--rules", good)
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	if len(mktDecode(t, out)["results"].([]any)) != 0 {
		t.Errorf("want empty results: %s", out)
	}
}

func TestMktParseRulesShapes(t *testing.T) {
	ts, err := mktParseRules(mktTestRules)
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 3 || ts[0].Name != "linen apron" || len(ts[0].Include) != 2 || len(ts[0].Exclude) != 1 || len(ts[2].Include) != 2 {
		t.Fatalf("parsed %+v", ts)
	}
	if !ts[0].mktMatches("Handmade PINAFORE dress") || ts[0].mktMatches("toddler apron") {
		t.Error("include/exclude matching wrong")
	}
	if _, err := mktParseRules("nonsense: 1\n"); err == nil {
		t.Error("expected error without targets:")
	}
	if _, err := mktParseRules("targets:\n  - include: [a]\n"); err == nil {
		t.Error("expected error for a target without name")
	}
}

func TestMarketGapsRulesFromStdin(t *testing.T) {
	_ = mktGapsFixture(t)
	for name, rules := range map[string]string{"yaml": mktTestRules, "json": mktGapsHappyStdin} {
		cmd := RootCmd()
		cmd.SetArgs([]string{"market", "gaps", "--rules", "-", "--json"})
		cmd.SetIn(strings.NewReader(rules))
		var out, errb bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errb)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s: market gaps --rules - error = %v (%s)", name, err, errb.String())
		}
		apron := mktGapByName(t, out.String(), "linen apron")
		if apron["matching_listings"].(float64) < 1 {
			t.Errorf("%s: stdin rules not applied: %v", name, apron)
		}
	}
}

func TestMarketGapsStdinRulesInvalid(t *testing.T) {
	mktTestEnv(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--rules", "-", "--json"})
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("empty stdin rules must be a usage error, not success")
	}
}

func TestMarketGapsEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t) // fresh sandbox: no store, no snapshots
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--rules=-", "--json"})
	cmd.SetIn(strings.NewReader(mktGapsHappyStdin))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty store must exit 0, got %v (%s)", err, errb.String())
	}
	m := mktDecode(t, out.String())
	if res, ok := m["results"].([]any); !ok || len(res) != 0 {
		t.Errorf("results = %v, want empty array", m["results"])
	}
	if n, _ := m["note"].(string); n == "" {
		t.Errorf("expected a note on an empty store: %s", out.String())
	}
}

func TestMarketGapsStdinFlagReadsStdin(t *testing.T) {
	_ = mktGapsFixture(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--stdin", "--json"})
	cmd.SetIn(strings.NewReader(mktTestRules))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market gaps --stdin error = %v (%s)", err, errb.String())
	}
	if apron := mktGapByName(t, out.String(), "linen apron"); apron["matching_listings"].(float64) < 1 {
		t.Errorf("--stdin rules not applied: %v", apron)
	}
}

func TestMarketGapsStdinConflictsWithRulesFile(t *testing.T) {
	rules := mktGapsFixture(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--stdin", "--rules", rules, "--json"})
	cmd.SetIn(strings.NewReader(mktTestRules))
	var errb bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errb)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("--stdin with a --rules file path must be a usage error")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 2 {
		t.Errorf("want usage error (exit 2), got %v", err)
	}
	if !strings.Contains(err.Error(), "--stdin") || !strings.Contains(err.Error(), "--rules") {
		t.Errorf("error should name both flags: %v", err)
	}
}

// The exact argv the live-dogfood runner builds: base args from the first
// Example line, then --rules - from happy-args, then the appended --stdin.
func TestMarketGapsRunnerArgvEmptySandbox(t *testing.T) {
	mktTestEnv(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "gaps", "--rules", "-", "--stdin", "--json"})
	cmd.SetIn(strings.NewReader(mktGapsHappyStdin))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("runner argv must exit 0, got %v (%s)", err, errb.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %q", out.String())
	}
	if _, ok := mktDecode(t, out.String())["results"].([]any); !ok {
		t.Errorf("missing results array: %s", out.String())
	}
}

func TestMarketGapsFirstExampleHasNoControl(t *testing.T) {
	first := strings.SplitN(strings.TrimSpace(newNovelMarketGapsCmd(&rootFlags{}).Example), "\n", 2)[0]
	if strings.Contains(first, "--control") || !strings.Contains(first, "--rules") {
		t.Errorf("first example must be rules-only (the live-dogfood runner derives args from it): %q", first)
	}
}

func TestMktGapStatementCounts(t *testing.T) {
	for _, tc := range []struct {
		n, shops, pages, terms int
		want                   string
	}{
		{0, 0, 1, 2, "no listing found in 1 page of 2 terms"},
		{0, 0, 2, 1, "no listing found in 2 pages of 1 term"},
		{1, 1, 1, 1, "1 listing from 1 shop found in 1 page of 1 term"},
		{2, 2, 2, 2, "2 listings from 2 shops found in 2 pages of 2 terms"},
	} {
		if got := mktGapStatement(tc.n, tc.shops, tc.pages, tc.terms); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
