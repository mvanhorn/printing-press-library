// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelListingsAuditHelpWires smoke-tests that the listings audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelListingsAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"listings", "audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("listings audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("listings audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func shopxListingsAuditFixture(t *testing.T) *shopxFixture {
	f := shopxNewFixture(t)
	for i := int64(1); i <= 7; i++ {
		tags := make([]string, 13)
		for j := range tags {
			tags[j] = "ceramics"
		}
		l := map[string]any{"listing_id": 1000000000 + i, "state": "active", "title": strings.Repeat("陶", 40), "tags": tags, "materials": []string{"linen", "clay", "cotton"}, "gpsr_manufacturer_id": 42, "gpsr_manufacturer_type": "economic_operator", "gpsr_responsible_person_id": 43, "gpsr_responsible_person_type": "economic_operator"}
		photos := 5
		if i == 2 {
			l["tags"] = tags[:7]
			l["materials"] = []string{}
			l["title"] = "Linen apron"
			photos = 3
		}
		if i == 3 {
			l["materials"] = []string{"linen"}
		}
		if i == 4 {
			l["state"] = "inactive"
			l["tags"] = []string{}
		}
		if i == 5 {
			l["gpsr_manufacturer_id"] = 9999
		}
		if i == 7 {
			delete(l, "gpsr_manufacturer_id")
		}
		f.put("listings", l)
		for j := 0; j < photos; j++ {
			im := map[string]any{"listing_image_id": i*100 + int64(j), "parent_id": fmt.Sprint(1000000000 + i), "rank": j + 1, "alt_text": "Juniper Ceramics photo"}
			if i == 2 && j == 1 {
				im["listing_image_id"] = 7701
				im["alt_text"] = "  "
			}
			f.put("listing_images", im)
		}
	}
	f.put("economic_operators", map[string]any{"economic_operator_id": 42, "operator_name": "Juniper Ceramics"}, map[string]any{"economic_operator_id": 43, "operator_name": "Juniper Ceramics EU"})
	return f
}

func shopxListingsAuditDecode(t *testing.T, out string) []shopxAuditRow {
	t.Helper()
	var rows []shopxAuditRow
	results, _ := shopxMustEnvelope(t, out)
	raw, _ := json.Marshal(results)
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		t.Fatalf("invalid rows %q: %v", out, err)
	}
	return rows
}

func TestNovelListingsAuditRules(t *testing.T) {
	for _, mode := range []string{"default", "gpsr", "all"} {
		t.Run(mode, func(t *testing.T) {
			f := shopxListingsAuditFixture(t)
			out, _, err := shopxRun(t, f, "listings", "audit", "--rules", mode)
			if err != nil {
				t.Fatal(err)
			}
			rows := shopxListingsAuditDecode(t, out)
			ids := make([]int64, 0)
			counts := shopxAuditCounts(rows)
			for _, r := range rows {
				ids = append(ids, r.ListingID)
			}
			want := []int64{1000000002}
			if mode == "gpsr" {
				want = []int64{1000000005, 1000000007}
			}
			if mode == "all" {
				want = []int64{1000000002, 1000000005, 1000000007}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("ids=%v want %v", ids, want)
			}
			if mode != "gpsr" {
				rules := []string{}
				for _, f := range rows[0].Findings {
					rules = append(rules, f.Rule)
					if f.Rule == "image_missing_alt" && f.ImageID != 7701 {
						t.Fatalf("image=%d", f.ImageID)
					}
				}
				if !reflect.DeepEqual(rules, []string{"tags_lt_13", "no_materials", "photos_lt_5", "image_missing_alt", "title_lt_40"}) {
					t.Fatalf("rules=%v", rules)
				}
				if counts["tags_lt_13"] != 1 {
					t.Fatal(counts)
				}
			}
			if mode == "default" {
				for rule := range counts {
					if strings.HasPrefix(rule, "gpsr_") {
						t.Fatal(counts)
					}
				}
			}
			if mode != "default" {
				if counts["gpsr_operator_not_synced"] != 1 || counts["gpsr_manufacturer_missing"] != 1 {
					t.Fatal(counts)
				}
				for _, r := range rows {
					if r.ListingID == 1000000005 && !strings.Contains(r.Findings[0].Detail, "manufacturer") {
						t.Fatal(r)
					}
				}
			}
		})
	}
	out, _, err := shopxRun(t, nil, "listings", "audit", "--rules", "banana")
	if err == nil {
		t.Fatalf("invalid rules accepted: %s", out)
	}
}

func TestNovelListingsAuditEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "listings", "audit")
			if err != nil {
				t.Fatal(err)
			}
			if len(shopxMustList(t, out)) != 0 {
				t.Fatal(out)
			}
		})
	}
	if counts := shopxAuditCounts(nil); counts == nil || len(counts) != 0 {
		t.Fatal(counts)
	}
}

func TestNovelListingsAuditUnknownSources(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active", "gpsr_manufacturer_id": 42, "gpsr_manufacturer_type": "economic_operator", "gpsr_responsible_person_id": 43, "gpsr_responsible_person_type": "economic_operator"})
	out, stderr, err := shopxRun(t, f, "listings", "audit", "--rules", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(shopxMustList(t, out)) != 0 {
		t.Fatal(out)
	}
	for _, note := range []string{"hint: listing images not synced; photos_lt_5 and image_missing_alt skipped", "hint: economic_operators not synced; gpsr_operator_not_synced skipped"} {
		if !strings.Contains(stderr, note) {
			t.Fatal(stderr)
		}
	}
}

func TestNovelListingsAuditPlan(t *testing.T) {
	for _, mode := range []string{"default", "gpsr"} {
		t.Run(mode, func(t *testing.T) {
			f := shopxListingsAuditFixture(t)
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
				t.Fatal(err)
			}
			// etsy-ext: existing output requires explicit overwrite consent.
			out, stderr, err := shopxRun(t, f, "listings", "audit", "--rules", mode, "--plan-out", path, "--force")
			if err != nil {
				t.Fatal(err)
			}
			shopxMustList(t, out)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var plan struct {
				Version     int                  `json:"version"`
				GeneratedBy string               `json:"generated_by"`
				Items       []shopxAuditPlanItem `json:"items"`
			}
			if err := json.Unmarshal(b, &plan); err != nil {
				t.Fatal(err)
			}
			if plan.Version != 1 || plan.GeneratedBy != "listings audit" || plan.Items == nil {
				t.Fatalf("plan=%s", b)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatal(info.Mode())
			}
			if !strings.Contains(stderr, "plan written: "+path) {
				t.Fatal(stderr)
			}
			if mode == "gpsr" {
				if len(plan.Items) != 0 {
					t.Fatal(plan)
				}
				return
			}
			if len(plan.Items) != 4 {
				t.Fatalf("items=%s", b)
			}
			got := map[string]string{}
			for _, item := range plan.Items {
				if item.ListingID != 1000000002 {
					t.Fatal(item)
				}
				got[item.Field] = item.Op
				if item.Field == "alt_text" {
					if item.ImageID != 7701 || item.Value != "Linen apron - photo 2" {
						t.Fatal(item)
					}
				} else if item.Value != "" || item.ImageID != 0 {
					t.Fatal(item)
				}
				if item.Field == "tags" && item.Reason != "7 of 13 tags used" {
					t.Fatal(item)
				}
			}
			if !reflect.DeepEqual(got, map[string]string{"tags": "add", "materials": "add", "title": "set", "alt_text": "set"}) {
				t.Fatal(got)
			}
			var raw map[string]any
			if err := json.Unmarshal(b, &raw); err != nil {
				t.Fatal(err)
			}
			for _, v := range raw["items"].([]any) {
				m := v.(map[string]any)
				if m["field"] != "alt_text" {
					if _, ok := m["image_id"]; ok {
						t.Fatal(m)
					}
				}
			}
		})
	}
	t.Run("clean", func(t *testing.T) {
		f := shopxNewFixture(t)
		f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active", "title": strings.Repeat("x", 40), "images": []any{1, 2, 3, 4, 5}})
		path := filepath.Join(t.TempDir(), "plan.json")
		_, _, err := shopxRun(t, f, "listings", "audit", "--plan-out", path, "--force")
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"items":[]`) {
			t.Fatal(string(b))
		}
	})
	t.Run("dry-run", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plan.json")
		_, _, err := shopxRun(t, nil, "listings", "audit", "--dry-run", "--plan-out", path, "--force")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("plan exists or unexpected error: %v", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		_, _, err := shopxRun(t, nil, "listings", "audit", "--plan-out", t.TempDir())
		if err == nil {
			t.Fatal("directory accepted")
		}
	})
}

func TestNovelListingsAuditImageUnionAndCombined(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000002, "state": "active", "title": "Linen apron", "images": []any{map[string]any{"listing_image_id": 1}, 2, 3, 4, 5}})
	f.put("listing_images", map[string]any{"listing_image_id": 1, "listing_id": 1000000002, "alt_text": "apron"})
	out, _, err := shopxRun(t, f, "listings", "audit", "--rules", "all")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxListingsAuditDecode(t, out)
	if len(rows) != 1 {
		t.Fatal(out)
	}
	counts := shopxAuditCounts(rows)
	if counts["photos_lt_5"] != 0 || counts["title_lt_40"] != 1 || counts["gpsr_manufacturer_missing"] != 1 || counts["gpsr_responsible_person_missing"] != 1 {
		t.Fatal(counts)
	}
}
