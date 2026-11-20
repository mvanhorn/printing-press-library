// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const inlineTestRules = `{"targets":[{"name":"linen apron","include":["linen apron"]},{"name":"wool scarf","include":["wool scarf"]}]}`

func TestMarketInlineParity(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	snap := mktFxSnapshot(t, db.DB(), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "Linen Apron Natural", price: 2400})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 2, id: 1000000002, shop: 555001, title: "Ceramic Mug Speckled", price: 2800})
	rules := filepath.Join(t.TempDir(), "rules.json")
	control := filepath.Join(t.TempDir(), "control.txt")
	if err := os.WriteFile(rules, []byte(inlineTestRules), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(control, []byte("1000000001\n1000000003\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name               string
		base, file, inline []string
	}{
		{"Gaps", []string{"market", "gaps"}, []string{"--rules", rules}, []string{"--rules-json", inlineTestRules}},
		{"Price", []string{"market", "price", "--group-by", "class"}, []string{"--rules", rules}, []string{"--rules-json", inlineTestRules}},
		{"Control", []string{"market", "gaps", "--rules-json", inlineTestRules}, []string{"--control", control}, []string{"--control-ids", "1000000001, ,1000000003"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, e := mktRunCLI(t, append(append([]string{}, tc.base...), tc.file...)...)
			if e != nil {
				t.Fatal(e)
			}
			b, e := mktRunCLI(t, append(append([]string{}, tc.base...), tc.inline...)...)
			if e != nil {
				t.Fatal(e)
			}
			if a != b {
				t.Fatalf("file=%s inline=%s", a, b)
			}
			if tc.name == "Control" && mktDecode(t, b)["gap_claims_downgraded"] != true {
				t.Fatal(b)
			}
		})
	}
}

func TestMarketInlineUsage(t *testing.T) {
	mktTestEnv(t)
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"RulesFile", []string{"market", "gaps", "--rules-json", inlineTestRules, "--rules", "unused"}, "conflicts"},
		{"RulesStdin", []string{"market", "gaps", "--rules-json", inlineTestRules, "--stdin"}, "conflicts"},
		{"ControlFile", []string{"market", "gaps", "--rules-json", inlineTestRules, "--control-ids", "1000000001", "--control", "unused"}, "conflicts"},
		{"PriceFile", []string{"market", "price", "--group-by", "class", "--rules-json", inlineTestRules, "--rules", "unused"}, "conflicts"},
		{"PriceGroup", []string{"market", "price", "--rules-json", inlineTestRules}, "--rules-json applies only with --group-by class"},
		{"BadJSON", []string{"market", "gaps", "--rules-json", "{"}, "must be JSON"},
		{"YAML", []string{"market", "gaps", "--rules-json", "targets: []"}, "must be JSON"},
		{"BadID", []string{"market", "gaps", "--rules-json", inlineTestRules, "--control-ids", "[1000000001]"}, "positive integer"},
		{"NegativeID", []string{"market", "gaps", "--rules-json", inlineTestRules, "--control-ids", "-1"}, "positive integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mktRunCLI(t, tc.args...)
			if tc.name == "PriceGroup" && err != nil && err.Error() != tc.message {
				t.Fatalf("message=%q", err.Error())
			}
			if ExitCode(err) != 2 || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("exit=%d err=%v", ExitCode(err), err)
			}
		})
	}
	for _, n := range []int{98304, 98305} {
		rules := inlineTestRules + strings.Repeat(" ", n-len(inlineTestRules))
		ids := "1000000001" + strings.Repeat(",", n-10)
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"Rules", []string{"market", "gaps", "--rules-json", rules}},
			{"PriceRules", []string{"market", "price", "--group-by", "class", "--rules-json", rules}},
			{"IDs", []string{"market", "gaps", "--rules-json", inlineTestRules, "--control-ids", ids}},
		} {
			_, err := mktRunCLI(t, tc.args...)
			if n == 98304 && err != nil {
				t.Fatalf("%s at limit: %v", tc.name, err)
			}
			if n == 98305 && ExitCode(err) != 2 {
				t.Fatalf("%s over limit: %v", tc.name, err)
			}
		}
	}
	for _, value := range []string{strings.Repeat("é", 5000), `{"targets":[{"name":"linen apron","include":["` + strings.Repeat("a", 5000) + `["]}]}`} {
		_, err := mktRunCLI(t, "market", "gaps", "--rules-json", value)
		if ExitCode(err) != 2 || len(err.Error()) > 260 || strings.Contains(err.Error(), strings.Repeat("a", 5000)) || !utf8.ValidString(err.Error()) {
			t.Fatalf("unbounded error: %v", err)
		}
	}
	_, err := mktCompileTargets([]*mktRawTarget{{name: strings.Repeat("é", 5000), include: []string{"["}}})
	if err == nil || len(err.Error()) > 210 || !utf8.ValidString(err.Error()) {
		t.Fatalf("regexp error: %v", err)
	}
	_, badControl := mktRunCLI(t, "market", "gaps", "--rules-json", inlineTestRules, "--control-ids", strings.Repeat("é", 5000))
	if ExitCode(badControl) != 2 || len(badControl.Error()) > 260 || !utf8.ValidString(badControl.Error()) {
		t.Fatalf("control diagnostic: %v", badControl)
	}
	_, badExclude := mktCompileTargets([]*mktRawTarget{{name: "linen apron", exclude: []string{strings.Repeat("a", 5000) + "["}}})
	if badExclude == nil || len(badExclude.Error()) > 210 {
		t.Fatalf("exclude diagnostic: %v", badExclude)
	}
	for _, value := range []string{strings.Repeat("a", 201), strings.Repeat("é", 101)} {
		if echo := mktEcho(value); len(echo) > 200 || !utf8.ValidString(echo) || !strings.HasSuffix(echo, "…") {
			t.Fatalf("echo=%q", echo)
		}
	}
	for _, ids := range []string{"", " , ", "1000000001,1000000001"} {
		out, err := mktInlineControl(ids)
		if err != nil {
			t.Fatal(err)
		}
		report := mktGapsReport(&mktSnapshot{Status: "complete"}, nil, nil, out, true)
		if _, ok := report["control_ids"]; !ok {
			t.Fatal("supplied empty control lost")
		}
		if ids == "1000000001,1000000001" && len(out) != 2 {
			t.Fatal("duplicate ids lost")
		}
	}
	for _, name := range []string{"gaps", "price"} {
		cmd, _, err := RootCmd().Find([]string{"market", name})
		if err != nil {
			t.Fatal(err)
		}
		flags := []string{"rules-json"}
		if name == "gaps" {
			flags = append(flags, "control-ids")
		}
		for _, flag := range flags {
			if cmd.Flags().Lookup(flag) == nil || strings.Contains(cmd.Annotations["mcp:read-flags"], flag) {
				t.Fatalf("%s %s not exposed", name, flag)
			}
		}
	}
}

func TestMarketInlineDocs(t *testing.T) {
	var doc map[string]json.RawMessage
	type docRow struct {
		Command string `json:"command"`
		Why     string `json:"why_it_matters"`
	}
	raw, err := os.ReadFile("../../.manuscripts/20261116-133635-274efd3c/research.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"market gaps":  "Use to find product ideas with little competition, stated as what was searched. Needs a prior market snapshot and a --rules YAML or JSON file of targets. Over MCP, pass rules inline with --rules-json and control ids with --control-ids (JSON and ids each up to 96 KiB; on Windows the whole command must fit ~31 KiB); each page re-sends the inline content (stateless paging).",
		"market price": "Use when pricing a new product against the market. Reads the latest snapshot; the price-vs-demand correlation is null unless the snapshot used --with-reviews. Over MCP, --group-by class takes its rules inline with --rules-json (JSON, ≤96 KiB; on Windows the whole command must fit ~31 KiB); each page re-sends the inline content (stateless paging).",
	}
	for _, array := range []string{"novel_features", "novel_features_built"} {
		count := 0
		var rows []docRow
		if err := json.Unmarshal(doc[array], &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if want, ok := expected[row.Command]; ok {
				count++
				if row.Why != want {
					t.Fatalf("%s %s drift", array, row.Command)
				}
			}
		}
		if count != 2 {
			t.Fatalf("%s found %d", array, count)
		}
	}
	for _, file := range []string{"../../README.md", "../../SKILL.md", "which.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range expected {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s missing guidance", file)
			}
		}
	}
}

func TestMCPPagingDocs(t *testing.T) {
	for _, file := range []string{"README.md", "SKILL.md", "CHANGELOG.md"} {
		t.Run(strings.ReplaceAll(file, ".", "_"), func(t *testing.T) {
			data, err := os.ReadFile("../../" + file)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"For mirror tools, check top-level `truncated`",
				"Typed local pages resume via `_pp_cursor`, not `cursor`",
				"Treat EITHER marker as incomplete",
				"with `agent: true` the guidance is always top-level `note`",
				"pass `snapshot` set to the page-1 `snapshot_id`", "`results.snapshot_id`", "without `select`", "`sql` signals it with `truncated: true` and a note", "`money_reconcile`, `reviews_digest`, `shop_attention`", "16777216", "64 KiB",
				"256 patterns of at most 512 bytes each", "about 1 MiB on macOS",
				"`money_summary`", "no `next_cursor`",
			} {
				if !strings.Contains(string(data), want) {
					t.Errorf("missing paging contract %q", want)
				}
			}
			if file == "CHANGELOG.md" {
				for _, want := range []string{"Typed single-array GET object (`{count,results}`)", "Partial page marked `_pp_truncated`", "`{items,...,truncated}` list envelope", "Paged envelope when list-shaped", "single-block failure now uses the mirror Preview shape", "`receipt_id`, `transaction_id`, `payment_id`, `entry_id`"} {
					if !strings.Contains(string(data), want) {
						t.Errorf("missing release behaviour %q", want)
					}
				}
			}
			if file != "CHANGELOG.md" {
				for _, paragraph := range strings.Split(string(data), "\n\n") {
					if strings.Contains(paragraph, "ETSY_MCP_ALLOW_BUYER_PII") && !strings.Contains(paragraph, "`limit` still caps rows") {
						t.Error("buyer opt-in paragraph missing paging disclosure")
					}
				}
			}
		})
	}
	cmd := newNovelMarketPriceCmd(&rootFlags{})
	if got := cmd.Flags().Lookup("group-by").Usage; !strings.Contains(got, "class needs --rules or --rules-json") {
		t.Fatalf("group-by help missing inline rules: %s", got)
	}
}
