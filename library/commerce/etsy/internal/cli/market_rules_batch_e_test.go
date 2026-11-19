// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBatchERulesMatching(t *testing.T) {
	for _, axis := range []string{"include", "all", "exclude", "fields", "fold"} {
		t.Run(axis, func(t *testing.T) {
			src, title, tags, want := `{"targets":[{"name":"x","include":["apron"]}]}`, "mug", []string(nil), false
			switch axis {
			case "all":
				src = `{"targets":[{"name":"x","all":["linen","apron"]}]}`
				title = "linen towel"
			case "exclude":
				src = `{"targets":[{"name":"x","exclude":["toddler"]}]}`
				title = "toddler apron"
			case "fields":
				src = `{"targets":[{"name":"x","fields":["title"],"include":["apron"]}]}`
				tags = []string{"apron"}
			case "fold":
				src = `{"targets":[{"name":"x","fold":true,"all":["linen apron"]}]}`
				title = "LINEN---APRON"
				want = true
			}
			ts, err := mktParseRules(src)
			if err != nil {
				t.Fatal(err)
			}
			if got := ts[0].matchesRow(mktRow{Title: title, Tags: tags}); got != want {
				t.Fatal(axis, got)
			}
		})
	}
	ts, err := mktParseRules(`{"targets":[{"name":"linen apron","all":["linen","apron"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !ts[0].matchesRow(mktRow{Title: "linen apron"}) || ts[0].matchesRow(mktRow{Title: "ceramic mug"}) {
		t.Fatal("include-less all constraints lost")
	}
	ts, err = mktParseRules(`{"targets":[{"name":"linen apron","include":["linen-apron"]}]}`)
	if err != nil || !ts[0].matchesRow(mktRow{Title: "LINEN-APRON"}) || ts[0].matchesRow(mktRow{Title: "linen apron"}) {
		t.Fatal("default folding changed", err)
	}
	for _, fields := range []string{"title", "[title]", "[title, tags]", "[tags, title]"} {
		ts, err := mktParseRules("targets:\n - name: linen apron\n   fields: " + fields + "\n   include: apron")
		if err != nil {
			t.Fatal(err)
		}
		if got := ts[0].matchesRow(mktRow{Title: "ceramic mug", Tags: []string{"apron"}}); got != strings.Contains(fields, "tags") {
			t.Fatal(fields, got)
		}
	}
	if got := mktFold("ÉCRU---陶器 100%__linen"); got != "écru 陶器 100% linen" {
		t.Fatal(got)
	}
	// Regex operators are not literals; escaped punctuation is.
	for _, pattern := range []string{`linen.*apron`, `linen\s+apron`, `(?:linen|apron)`, `[a-z]+`, `[[:alpha:]]`, `[\s]`, `\s+`, `\s*`, `linen\s?apron`, `linen \S+`, `\w+`, `\d+`, `\p{L}+`, `[^a]`, `\W`, `\S`, `[[:punct:]]`, `linen apron`} {
		raw, _ := json.Marshal(map[string]any{"targets": []any{map[string]any{"name": "linen apron", "include": []string{pattern}, "fold": true}}})
		if _, err := mktParseRules(string(raw)); err != nil {
			t.Fatal(pattern, err)
		}
	}
}

func TestBatchEValidationUsage(t *testing.T) {
	mktTestEnv(t)
	sources := []string{
		`{"targets":[{"name":"x","ALL":["a"]}]}`,
		`{"targets":[{"name":"x","Fold":true}]}`,
		`{"targets":[{"Name":"x"}]}`,
		`{"targets":[{"name":"x"}]} {"targets":[]}`,
		`{"targets":[{"name":"x","all":["linen(-|_)apron"],"fold":true}]}`,
		`{"targets":[{"name":"x","all":["(\\.|,)"],"fold":true}]}`,
		`{"targets":[{"name":"x","all":["linen  apron"],"fold":true}]}`,
		"targets:\n - name: x\n   include: linen-apron\n   fold: true",
		`{"targets":[{"name":"linen apron","wat":[]}]}`,
		`{"targets":[{"name":"linen apron"}],"wat":[]}`,
		`{"targets":[{"name":"linen apron","fields":[]}]}`,
		`{"targets":[{"name":"linen apron","fields":null}]}`,
		`{"targets":[{"name":"linen apron","all":["[a_]"],"fold":true}]}`,
		`{"targets":[{"name":"linen apron","all":["[-a]"],"fold":true}]}`,
		`{"targets":[{"name":"linen apron","all":["[.]"],"fold":true}]}`,
		`{"targets":[{"name":"linen apron","fold":null}]}`,
		`{"targets":[{"name":"linen apron","fields":["tags"]}]}`,
		`{"targets":[{"name":"linen apron","fields":["title","title"]}]}`,
		`{"targets":[{"name":"linen apron","fields":["title","wat"]}]}`,
		`{"targets":[{"name":"linen apron","fold":"true"}]}`,
		`{"targets":[{"name":"linen apron","all":["linen-apron"],"fold":true}]}`,
		`{"targets":[{"name":"linen apron","exclude":["linen\\-apron"],"fold":true}]}`,
		"targets:\n - name: linen apron\n   unknown:\n     - apron",
		"targets:\n - name: linen apron\n   fields: []",
		"targets:\n - name: linen apron\n   fields:\n     - tags",
		"targets:\n - name: linen apron\n   fold: yes",
	}
	for _, pattern := range []string{`linen \s apron`, `linen\s\sapron`, `a {2}b`, `(a )( b)`, `a \sb`, `a(-|x)b`, `a|-`, `[\x00_a]`} {
		raw, _ := json.Marshal(map[string]any{"targets": []any{map[string]any{"name": "x", "all": []string{pattern}, "fold": true}}})
		sources = append(sources, string(raw))
	}
	for _, src := range sources {
		path := filepath.Join(t.TempDir(), "rules")
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		for _, cmd := range [][]string{{"market", "gaps"}, {"market", "price", "--group-by", "class"}} {
			_, err := mktRunCLI(t, append(cmd, "--rules", path)...)
			if ExitCode(err) != 2 {
				t.Fatalf("%s: %v", src, err)
			}
		}
	}
	for _, src := range sources[:2] {
		_, err := mktParseRules(src)
		if !strings.Contains(err.Error(), "want name, include, exclude, all, fields, fold") {
			t.Fatal(err)
		}
	}
	_, err := mktParseRules(`{"targets":[{"name":"linen apron","include":["linen-apron"],"fold":true}]}`)
	if err == nil || !strings.Contains(err.Error(), `"linen apron"`) {
		t.Fatal(err)
	}
	for _, n := range []int{256, 257} {
		r := &mktRawTarget{name: "linen apron", all: make([]string, n)}
		for i := range r.all {
			r.all[i] = "apron"
		}
		_, err := mktCompileTargets([]*mktRawTarget{r})
		if (err != nil) != (n == 257) || (n == 257 && !strings.Contains(err.Error(), "maximum 256 patterns across include, exclude and all")) {
			t.Fatal(n, err)
		}
	}
	_, err = mktCompileTargets([]*mktRawTarget{{name: "linen apron", all: []string{strings.Repeat("a", 513)}}})
	if err == nil || !strings.Contains(err.Error(), "pattern exceeds 512 bytes") {
		t.Fatal("all length cap missing")
	}
}

func TestBatchELegacyResultsBytes(t *testing.T) {
	ts, err := mktParseRules(`{"targets":[{"name":"linen apron","include":["apron"],"exclude":["toddler"]},{"name":"ceramic mug","exclude":["toddler"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	rows := []mktRow{{ListingID: 1000000001, Title: "LINEN APRON", ShopID: sql.NullInt64{Int64: 9000001, Valid: true}}, {ListingID: 1000000002, Title: "toddler apron"}}
	report := mktGapsReport(&mktSnapshot{ID: 1, Pages: 1, Terms: []string{"linen apron"}, Status: "complete"}, rows, ts, nil, false)
	raw, _ := json.Marshal(report["results"])
	want := `[{"name":"linen apron","matching_listings":1,"distinct_shops":1,"statement":"1 listing from 1 shop found in 1 page of 1 term"},{"matches_all":true,"name":"ceramic mug","matching_listings":1,"distinct_shops":1,"statement":"1 listing from 1 shop found in 1 page of 1 term"}]`
	// matches_all is the sole allowed change to the legacy results encoding.
	if string(raw) != want {
		t.Fatalf("got %s\nwant %s", raw, want)
	}
}

func TestBatchEReportsAndInterface(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400})
	db.Close()

	for _, tc := range []struct {
		name     string
		patterns string
		broad    bool
	}{
		{"all_only", `"all":["linen","apron"]`, false},
		{"exclude_only", `"exclude":["toddler"]`, true},
		{"include_only", `"include":["apron"]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules := `{"targets":[{"name":"linen apron",` + tc.patterns + `}]}`
			for _, cmd := range [][]string{{"market", "gaps"}, {"market", "price", "--group-by", "class"}} {
				for _, output := range [][]string{nil, {"--compact"}, {"--select", "results"}} {
					args := append(append(append([]string{}, cmd...), "--rules-json", rules), output...)
					out, err := mktRunCLI(t, args...)
					if err != nil {
						t.Fatal(err)
					}
					report := mktDecode(t, out)
					if tc.broad {
						if note, ok := report["note"].(string); ok && strings.HasPrefix(note, "target ") {
							t.Fatalf("advisory leaked into note: %s", out)
						}
					}
					results := report["results"].([]any)
					if len(results) != 1 {
						t.Fatal(out)
					}
					value, present := results[0].(map[string]any)["matches_all"]
					if present != tc.broad || (present && value != true) {
						t.Fatalf("%v %v: unexpected matches_all: %s", cmd, output, out)
					}
					var advisories []string = []string{}
					for _, note := range report["notes"].([]any) {
						if strings.HasPrefix(note.(string), "target ") {
							advisories = append(advisories, note.(string))
						}
					}
					notes, err := json.Marshal(advisories)
					if err != nil {
						t.Fatal(err)
					}
					wantNotes := `[]`
					if tc.broad {
						wantNotes = `["target \"linen apron\" has no include or all patterns; it matches every listing not excluded"]`
					}
					if string(notes) != wantNotes {
						t.Fatalf("%v %v: notes = %s, want %s", cmd, output, notes, wantNotes)
					}
				}
			}
		})
	}
	for _, cmd := range [][]string{{"market", "gaps", "--help"}, {"market", "price", "--help"}} {
		out, err := mktRunCLI(t, cmd...)
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{"all:", "AND", "fields", "fold", "JSON"} {
			if !strings.Contains(out, needle) {
				t.Fatal(needle, out)
			}
		}
	}
}

func TestBatchELegacyPriceResultsBytes(t *testing.T) {
	ts, err := mktParseRules("targets:\n - name: linen apron\n   include: apron\n   exclude: toddler")
	if err != nil {
		t.Fatal(err)
	}
	rows := []mktRow{{ListingID: 1000000001, Title: "LINEN APRON", Currency: "USD", Price: sql.NullFloat64{Float64: 24, Valid: true}}, {ListingID: 1000000002, Title: "toddler apron", Currency: "USD", Price: sql.NullFloat64{Float64: 8, Valid: true}}}
	groups := mktPriceGroups(rows, nil, func(r mktRow) []string {
		if ts[0].matchesRow(r) {
			return []string{ts[0].Name}
		}
		return nil
	}, 2)
	raw, _ := json.Marshal(groups)
	want := `[{"group":"linen apron","n":1,"min":24,"median":24,"max":24,"currency":"USD","spearman_rho":null,"rho_pairs":0,"note":"group has 1 listing, fewer than --min-group 2; correlation not computed"}]`
	if string(raw) != want {
		t.Fatalf("got %s; want %s", raw, want)
	}
}

func TestBatchEFoldApplication(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			for _, axis := range []string{"all", "include"} {
				t.Run(axis, func(t *testing.T) {
					for _, fold := range []string{"true", "false"} {
						t.Run(fold, func(t *testing.T) {
							src := `{"targets":[{"name":"x","` + axis + `":["linen apron"],"fold":` + fold + `}]}`
							if format == "yaml" {
								src = "targets:\n - name: x\n   " + axis + ": linen apron\n   fold: " + fold
							}
							ts, err := mktParseRules(src)
							if err != nil {
								t.Fatal(err)
							}
							if got := ts[0].matchesRow(mktRow{Title: "LINEN---APRON"}); got != (fold == "true") {
								t.Fatal(src, got)
							}
						})
					}
				})
			}
		})
	}
}

func TestBatchEFoldDiagnostics(t *testing.T) {
	for _, pattern := range []string{`t-shirts?`, `\bt-shirt\b`, `[.]`, `[a_]`} {
		raw, _ := json.Marshal(map[string]any{"targets": []any{map[string]any{"name": "x", "include": []string{pattern}, "fold": true}}})
		_, err := mktParseRules(string(raw))
		if err == nil {
			t.Fatal(pattern)
		}
		message := err.Error()
		removed := "'-'"
		if pattern == "[.]" {
			removed = "'.'"
		}
		if pattern == "[a_]" {
			removed = "'_'"
		}
		if !strings.Contains(message, `target "x"`) || !strings.Contains(message, removed) || strings.Contains(message, "folded spelling") || strings.Contains(message, `"`+mktFold(pattern)+`"`) {
			t.Fatal(message)
		}
	}
	_, err := mktParseRules(`{"targets":[{"name":"x"}]} {}`)
	if err == nil || !strings.Contains(err.Error(), "single document") {
		t.Fatal(err)
	}
}

func TestBatchELegacyEnvelopeNotes(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400})
	db.Close()
	for _, command := range []string{"gaps", "price"} {
		t.Run(command, func(t *testing.T) {
			for _, broad := range []bool{false, true} {
				patterns := `,"include":["apron"]`
				if broad {
					patterns = ""
				}
				args := []string{"market", command, "--snapshot", fmt.Sprint(snap), "--rules-json", `{"targets":[{"name":"x"` + patterns + `}]}`}
				headline := "no review data collected; price-demand correlation not computed — re-run market snapshot with --with-reviews"
				want := []string{headline, `"x": group has 1 listing, fewer than --min-group 15; correlation not computed`}
				if command == "gaps" {
					args = append(args, "--control-ids", "1000000099")
					headline = "only 0 of 1 control listings were visible in this sample, so a 'no listing found' result may reflect sampling limits rather than a real gap"
					want = []string{headline, "snapshot is partial; absence claims cover only the pages actually fetched"}
				} else {
					args = append(args, "--group-by", "class")
				}
				if broad {
					want = append(want, `target "x" has no include or all patterns; it matches every listing not excluded`)
				}
				out, err := mktRunCLI(t, args...)
				if err != nil {
					t.Fatal(err)
				}
				report := mktDecode(t, out)
				got, _ := json.Marshal(report["notes"])
				expected, _ := json.Marshal(want)
				if string(got) != string(expected) || report["note"] != headline {
					t.Fatalf("got %s", out)
				}
				// Golden legacy envelopes pin metadata and row shape as well as note order.
				golden := `{"excluded_other_currency":0,"group_by":"class","min_group":15,"snapshot_id":` + fmt.Sprint(snap) + `,"results":[{"group":"x","n":1,"min":24,"median":24,"max":24,"currency":"USD","spearman_rho":null,"rho_pairs":0,"note":"group has 1 listing, fewer than --min-group 15; correlation not computed"}]}`
				if command == "gaps" {
					golden = `{"snapshot_id":` + fmt.Sprint(snap) + `,"pages":1,"terms":1,"control_reached":0,"control_ids":1,"control_seen":0,"gap_claims_downgraded":true,"snapshot_note":"snapshot is partial; absence claims cover only the pages actually fetched","results":[{"name":"x","matching_listings":1,"distinct_shops":1,"statement":"1 listing from 1 shop found in 1 page of 1 term"}]}`
				}
				envelope := mktDecode(t, golden)
				envelope["note"] = headline
				envelope["notes"] = want
				if broad {
					envelope["results"].([]any)[0].(map[string]any)["matches_all"] = true
				}
				goldenBytes, _ := json.Marshal(envelope)
				reportBytes, _ := json.Marshal(report)
				if string(goldenBytes) != string(reportBytes) {
					t.Fatalf("full envelope got %s; want %s", reportBytes, goldenBytes)
				}
			}
		})
	}
}

func TestBatchEBroadAdvisoryHasNoHeadline(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400})
	mktFxReviews(t, db.DB(), snap, 1000000001, 1, time.Now())
	db.Close()
	for _, command := range []string{"gaps", "price"} {
		t.Run(command, func(t *testing.T) {
			args := []string{"market", command, "--rules-json", `{"targets":[{"name":"x"}]}`}
			if command == "price" {
				args = append(args, "--group-by", "class")
			}
			out, err := mktRunCLI(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			report := mktDecode(t, out)
			if _, ok := report["note"]; ok {
				t.Fatal(out)
			}
			notes := report["notes"].([]any)
			if notes[len(notes)-1] != `target "x" has no include or all patterns; it matches every listing not excluded` {
				t.Fatal(out)
			}
		})
	}
}

func TestBatchEValidationContext(t *testing.T) {
	for _, src := range []string{
		`{"targets":[{"name":"x","fields":null}]}`,
		`{"targets":[{"name":"x","fold":"true"}]}`,
		"targets:\n - name: x\n   fields: [tags]",
		"targets:\n - name: x\n   fold: yes",
		"targets:\n - fold: yes\n   name: x",
	} {
		_, err := mktParseRules(src)
		if err == nil || !strings.Contains(err.Error(), `target "x":`) {
			t.Fatal(src, err)
		}
		if strings.HasPrefix(src, "targets:") && !strings.Contains(err.Error(), "line ") {
			t.Fatal(err)
		}
	}
}

func TestBatchEFoldSpaceAdvice(t *testing.T) {
	raw := `{"targets":[{"name":"x","include":["linen - apron"],"fold":true}]}`
	_, err := mktParseRules(raw)
	want := `target "x": pattern "linen - apron": fold removes '-'; it and adjacent spaces become one space, so write "linen apron"`
	if err == nil || err.Error() != want {
		t.Fatal(err)
	}
	for _, pattern := range []string{`a(-|x)b`, `a|-`, `[a_]`} {
		_, err := mktCompileTargets([]*mktRawTarget{{name: "x", include: []string{pattern}, fold: true}})
		if err == nil || strings.Contains(err.Error(), "so write") {
			t.Fatal(pattern, err)
		}
	}
}
