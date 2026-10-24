// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// TestNovelChangesHelpWires smoke-tests that the changes command resolves
// at runtime and renders useful --help output.
func TestNovelChangesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"changes", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("changes --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "changes [flags]", "--since", "--apps", "--kind", "editors_choice"} {
		if !strings.Contains(help, want) {
			t.Fatalf("changes --help missing %q in output:\n%s", want, help)
		}
	}
}

func changeRowsOf(rows []changeRow, kind string) []changeRow {
	out := make([]changeRow, 0)
	for _, r := range rows {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

func TestChangesSingleSnapshotReturnsEmpty(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedSingleCaptureScenario(t, st, s3bTestNow)

	for _, since := range []string{"7d", "all"} {
		out, _, err := runS3B(t, db, "changes", "--since", since, "--json")
		if err != nil {
			t.Fatal(err)
		}
		var v changesView
		decodeS3B(t, out, &v)
		if len(v.Results) != 0 || v.Meta.Comparisons != 0 || v.Meta.TotalChanges != 0 {
			t.Fatalf("--since %s with one snapshot fabricated changes: %s", since, out)
		}
		if !strings.Contains(v.Meta.Note, "No consecutive captures") || !strings.Contains(out, `"results": []`) {
			t.Fatalf("--since %s note/shape = %s", since, out)
		}
	}
}

func TestChangesFeedCoversEveryKind(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedCandyScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "changes", "--since", "7d", "--limit", "0", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	if len(v.Results) != 13 || v.Meta.TotalChanges != 13 {
		t.Fatalf("want 13 changes, got %d:\n%s", len(v.Results), out)
	}
	kinds := map[string]int{}
	for i, r := range v.Results {
		kinds[r.Kind]++
		if r.AppID != s3bCandy {
			t.Fatalf("single-snapshot app %s must not produce changes: %+v", r.AppID, r)
		}
		if i > 0 && r.Date > v.Results[i-1].Date {
			t.Fatalf("feed not newest first at %d: %s after %s", i, r.Date, v.Results[i-1].Date)
		}
		if r.Date == "" || r.PreviousCapturedAt == "" || r.PreviousCapturedAt >= r.Date {
			t.Fatalf("row not bracketed by captures: %+v", r)
		}
	}
	for _, k := range changeKinds {
		if kinds[k] == 0 {
			t.Fatalf("kind %s missing from feed: %v", k, kinds)
		}
	}

	if r := changeRowsOf(v.Results, "version"); len(r) != 1 || r[0].From != "1.300.0" || r[0].To != "1.301.0" || r[0].Date != "2026-10-18T08:00:00Z" ||
		!strings.Contains(r[0].Detail, "released 2026-10-17T10:00:00Z") || !strings.Contains(r[0].Detail, "Halloween levels") {
		t.Fatalf("version rows = %+v", r)
	}
	if r := changeRowsOf(v.Results, "title"); len(r) != 1 || r[0].From != "Candy Crush Saga" || r[0].To != "Candy Crush Saga!" {
		t.Fatalf("title rows = %+v", r)
	}
	if r := changeRowsOf(v.Results, "price"); len(r) != 1 || r[0].From != "Free" || r[0].To != "$0.99" || r[0].Date != "2026-10-22T08:00:00Z" || r[0].PreviousCapturedAt != "2026-10-18T08:00:00Z" {
		t.Fatalf("price rows = %+v", r)
	}
	if v.Results[0].Kind != "price" {
		t.Fatalf("newest change should be the d-2 price change, got %+v", v.Results[0])
	}
	if r := changeRowsOf(v.Results, "subtitle"); len(r) != 1 || r[0].From != "Sweet puzzle" || r[0].To != "Sweetest puzzle" || r[0].Change != "changed" {
		t.Fatalf("subtitle rows = %+v", r)
	}
	if r := changeRowsOf(v.Results, "editors_choice"); len(r) != 1 || r[0].Change != "gained" {
		t.Fatalf("editors_choice rows = %+v", r)
	}
	iap := map[string]changeRow{}
	for _, r := range changeRowsOf(v.Results, "iap") {
		iap[r.Change+":"+r.Field] = r
	}
	if r, ok := iap["changed:Gold Bars"]; !ok || r.From != "$0.99" || r.To != "$1.29" {
		t.Fatalf("iap price change missing: %+v", iap)
	}
	if _, ok := iap["added:Booster Pack"]; !ok {
		t.Fatalf("iap addition missing: %+v", iap)
	}
	if r, ok := iap["removed:Lives"]; !ok || r.From != "$1.99" || len(iap) != 3 {
		t.Fatalf("iap rows = %+v", iap)
	}
	privacy := changeRowsOf(v.Results, "privacy")
	if len(privacy) != 2 {
		t.Fatalf("privacy rows = %+v", privacy)
	}
	for _, r := range privacy {
		switch r.Field {
		case "Data Linked to You":
			if r.Change != "changed" || !strings.Contains(r.Detail, "+Location") || r.To != "Identifiers, Location" {
				t.Fatalf("linked row = %+v", r)
			}
		case "Data Used to Track You":
			if r.Change != "added" || r.To != "Identifiers" {
				t.Fatalf("tracking row = %+v", r)
			}
		default:
			t.Fatalf("unexpected privacy row %+v", r)
		}
	}
	if r := changeRowsOf(v.Results, "event"); len(r) != 1 || r[0].Field != "Halloween Party" || r[0].Change != "new" || !strings.Contains(r[0].Detail, "Challenge") {
		t.Fatalf("event rows = %+v (Summer Fest is baseline, not new)", r)
	}
	featured := changeRowsOf(v.Results, "featured")
	if len(featured) != 2 || featured[0].Change != "lost" || featured[0].Date != "2026-10-21T09:00:00Z" || featured[1].Change != "gained" ||
		featured[1].Date != "2026-10-18T09:00:00Z" || !strings.Contains(featured[1].Detail, "Game of the Day") {
		t.Fatalf("featured rows = %+v", featured)
	}
}

func TestChangesFiltersAndLimits(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedCandyScenario(t, st, s3bTestNow)

	run := func(args ...string) changesView {
		out, _, err := runS3B(t, db, append([]string{"changes"}, append(args, "--json")...)...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var v changesView
		decodeS3B(t, out, &v)
		return v
	}
	if v := run("--since", "7d", "--kind", "iap"); len(v.Results) != 3 || len(changeRowsOf(v.Results, "iap")) != 3 || v.Meta.Comparisons != 1 {
		t.Fatalf("--kind iap = %+v (comparisons %d, want the 1 IAP list pair)", v.Results, v.Meta.Comparisons)
	}
	if v := run("--since", "7d", "--kind", "subtitle,events"); len(v.Results) != 2 || strings.Join(v.Meta.Kinds, ",") != "subtitle,event" {
		t.Fatalf("--kind subtitle,events = %+v %v", v.Results, v.Meta.Kinds)
	}
	if v := run("--since", "30d", "--limit", "0"); len(v.Results) != 14 || len(changeRowsOf(v.Results, "version")) != 2 {
		t.Fatalf("--since 30d should add the 1.299.0 -> 1.300.0 version change: %d rows", len(v.Results))
	}
	if v := run("--since", "7d", "--limit", "2"); len(v.Results) != 2 || !v.Meta.Truncated || v.Meta.TotalChanges != 13 || !strings.Contains(v.Meta.Note, "raise --limit") {
		t.Fatalf("--limit 2 = %+v", v.Meta)
	}
	if v := run("--since", "7d", "--apps", "850417475"); len(v.Results) != 0 || v.Meta.Comparisons != 0 || !strings.Contains(v.Meta.Note, "No consecutive captures") {
		t.Fatalf("single-snapshot app = %+v", v)
	}
	if v := run("--since", "7d", "--apps", "com.king.candycrushsaga", "--kind", "version"); len(v.Results) != 1 || v.Results[0].AppID != s3bCandy {
		t.Fatalf("bundle id --apps = %+v", v.Results)
	}
	if v := run("--since", "7d", "--countries", "gb"); len(v.Results) != 0 || v.Meta.Comparisons != 0 {
		t.Fatalf("--countries gb has no data: %+v", v)
	}
	if v := run("--since", "3d"); len(v.Results) != 1 || v.Results[0].Kind != "price" {
		t.Fatalf("--since 3d = %+v, want only the 2026-10-22 price change", v.Results)
	}
	if v := run("--since", "1d"); len(v.Results) != 0 || v.Meta.Comparisons != 0 || !strings.Contains(v.Meta.Note, "No consecutive captures") {
		t.Fatalf("--since 1d holds no capture pair: %+v", v)
	}

	// --select keeps the envelope and projects rows.
	out, _, err := runS3B(t, db, "changes", "--since", "7d", "--json", "--select", "kind,date")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--select output: %v\n%s", err, out)
	}
	if len(env.Results) != 13 || len(env.Results[0]) != 2 || env.Results[0]["kind"] == nil || env.Meta["total_changes"] == nil {
		t.Fatalf("--select kind,date = %s", out)
	}

	for _, args := range [][]string{
		{"changes", "extra-arg", "--json"},
		{"changes", "--kind", "bogus"},
		{"changes", "--since", "-3d"},
		{"changes", "--since", "soon"},
		{"changes", "--limit", "-1"},
		{"changes", "--apps", "not an app"},
		{"changes", "--countries", "zz"},
	} {
		if _, _, err := runS3B(t, db, args...); ExitCode(err) != 2 {
			t.Fatalf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}

func TestChangesNoStoreAndDiffHelpers(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	out, stderr, err := runS3B(t, filepath.Join(t.TempDir(), "absent.db"), "changes", "--since", "7d", "--json")
	if err != nil {
		t.Fatalf("missing store must exit 0: %v", err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(stderr, "no local mirror") {
		t.Fatalf("missing store = %s / %s", out, stderr)
	}

	same := []appstore.IAPItem{{Position: 1, Name: "A", Price: "$1"}, {Position: 2, Name: "B", Price: "$2"}}
	if d := diffIAPLists(same, same); len(d) != 0 {
		t.Fatalf("identical IAP lists must not differ: %+v", d)
	}
	swapped := []appstore.IAPItem{{Position: 1, Name: "B", Price: "$2"}, {Position: 2, Name: "A", Price: "$1"}}
	if d := diffIAPLists(same, swapped); len(d) != 1 || d[0].change != "reordered" {
		t.Fatalf("reorder = %+v", d)
	}
	p := appstore.Privacy{Types: []appstore.PrivacyType{{Identifier: "T", Title: "Type", Categories: []appstore.PrivacyCategory{{Identifier: "C", Title: "Cat", DataTypes: []string{"x"}}}}}}
	if d := diffPrivacy(p, p); len(d) != 0 {
		t.Fatalf("identical privacy labels must not differ: %+v", d)
	}
	q := appstore.Privacy{Types: []appstore.PrivacyType{{Identifier: "T", Title: "Type", Categories: []appstore.PrivacyCategory{{Identifier: "C", Title: "Cat", DataTypes: []string{"x", "y"}}}}}}
	if d := diffPrivacy(p, q); len(d) != 1 || !strings.Contains(d[0].detail, "data types changed in Cat") {
		t.Fatalf("data type change = %+v", d)
	}
}
