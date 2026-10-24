// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// TestNovelKeywordShareHelpWires smoke-tests that the keyword-share command
// resolves at runtime and renders useful --help output.
func TestNovelKeywordShareHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"keyword-share", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keyword-share --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "keyword-share [term...]", "--top", "--app", "--country"} {
		if !strings.Contains(help, want) {
			t.Fatalf("keyword-share --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestKeywordShareDropsLocalCountryFlag(t *testing.T) {
	root := RootCmd()
	cmd, _, err := root.Find([]string{"keyword-share"})
	if err != nil || cmd.Name() != "keyword-share" {
		t.Fatalf("keyword-share not found: %v", err)
	}
	if cmd.LocalNonPersistentFlags().Lookup("country") != nil {
		t.Fatal("keyword-share must not declare a local --country that shadows the root flag")
	}
	if cmd.Annotations["pp:novel-scaffold"] != "" || cmd.Annotations["pp:data-source"] != "local" || cmd.Annotations["mcp:read-only"] != "true" {
		t.Fatalf("annotations = %v", cmd.Annotations)
	}
}

func shareRow(rows []keywordShareRow, id string) *keywordShareRow {
	for i := range rows {
		if rows[i].AppID == id {
			return &rows[i]
		}
	}
	return nil
}

func TestKeywordShareCountsAndTargetGaps(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedKeywordScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "keyword-share", "music streaming", "Podcast Player", "audiobooks", "--top", "5", "--app", s3bSpotify, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v keywordShareView
	decodeS3B(t, out, &v)
	if v.Meta.Storefront != "us" || v.Meta.TermsCounted != 3 || v.Meta.ComparableTerms != 2 || !v.Meta.Unweighted || len(v.Meta.TermsMissing) != 0 {
		t.Fatalf("meta = %+v", v.Meta)
	}
	type want struct {
		held, prev, change int
		avg                float64
		best               int
	}
	expect := map[string]want{
		s3bSpotify:   {2, 2, -1, 2, 2}, // music 1->2, podcast 4->out, audiobooks new at 2 (not comparable)
		"1017492454": {1, 1, 0, 1, 1},  // music 2->1
		"525463029":  {1, 1, 0, 1, 1},  // podcast 1->1
		"379693831":  {2, 0, 1, 3, 1},  // podcast new at 5, audiobooks at 1
		"336353151":  {1, 1, 0, 4, 4},  // music 3->4
	}
	for id, w := range expect {
		r := shareRow(v.Results, id)
		if r == nil {
			t.Fatalf("no row for %s:\n%s", id, out)
		}
		if r.TermsHeld != w.held || r.TermsHeldPrevious != w.prev || r.Change != w.change || r.AvgPosition == nil || *r.AvgPosition != w.avg || *r.BestPosition != w.best {
			t.Fatalf("%s = held %d prev %d change %d avg %s best %s; want %+v", id, r.TermsHeld, r.TermsHeldPrevious, r.Change, s3bOptFloat(r.AvgPosition), s3bOptRank(r.BestPosition), w)
		}
		if r.Target != (id == s3bSpotify) {
			t.Fatalf("%s target flag = %v", id, r.Target)
		}
	}
	if shareRow(v.Results, "2300000001") != nil {
		t.Fatal("rank 6 is outside --top 5 and must not count")
	}
	if shareRow(v.Results, "2300000009") != nil {
		t.Fatal("a gb capture leaked into the us share")
	}
	if v.Results[0].TermsHeld < v.Results[len(v.Results)-1].TermsHeld {
		t.Fatal("rows must be ordered by terms held")
	}

	tg := v.Meta.Target
	if tg == nil || tg.AppID != s3bSpotify || tg.TermsHeld != 2 || len(tg.GapTerms) != 1 {
		t.Fatalf("target = %+v", tg)
	}
	gap := tg.GapTerms[0]
	if gap.Term != "podcast player" || gap.TargetRank == nil || *gap.TargetRank != 9 || strings.Join(gap.Leaders, ",") != "525463029,888422857,414834813" {
		t.Fatalf("gap = %+v", gap)
	}
	if len(tg.HeldTerms) != 2 || tg.HeldTerms[0].Term != "music streaming" || *tg.HeldTerms[0].Position != 2 || *tg.HeldTerms[0].PreviousPosition != 1 {
		t.Fatalf("held terms = %+v", tg.HeldTerms)
	}
}

// Before any capture there is no store; the answer must still name the terms
// that were asked for (all missing) and hint at capturing the first of them.
func TestKeywordShareWithoutAStoreListsTheRequestedTerms(t *testing.T) {
	testenv.Isolate(t)
	absent := filepath.Join(t.TempDir(), "absent.db")
	out, _, err := runS3B(t, absent, "keyword-share", "Podcast  Player", "music streaming", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v keywordShareView
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || strings.Join(v.Meta.TermsMissing, ",") != "podcast player,music streaming" {
		t.Fatalf("no store: terms_missing = %v, results = %d", v.Meta.TermsMissing, len(v.Results))
	}
	if !strings.Contains(v.Meta.Note, `keyword-rank "podcast player" --country us`) {
		t.Fatalf("capture hint should name the requested term: %q", v.Meta.Note)
	}

	v = keywordShareView{}
	out, _, err = runS3B(t, absent, "keyword-share", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Meta.TermsMissing) != 0 || !strings.Contains(v.Meta.Note, `keyword-rank "music streaming" --app 324684580`) {
		t.Fatalf("no terms, no store: %s", out)
	}
}

func TestKeywordShareAllTermsStorefrontAndNegative(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)

	out, _, err := runS3B(t, db, "keyword-share", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v keywordShareView
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "No keyword captures are stored for storefront us") || !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty store = %s", out)
	}

	seedKeywordScenario(t, st, s3bTestNow)
	v = keywordShareView{}
	out, _, err = runS3B(t, db, "keyword-share", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if v.Meta.TermsCounted != 3 || v.Meta.Target != nil || len(v.Results) == 0 {
		t.Fatalf("all tracked us terms = %+v", v.Meta)
	}

	v = keywordShareView{}
	out, _, err = runS3B(t, db, "keyword-share", "--country", "gb", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if v.Meta.Storefront != "gb" || v.Meta.TermsCounted != 1 || len(v.Results) != 2 || shareRow(v.Results, "1017492454") != nil {
		t.Fatalf("gb share = %s", out)
	}

	v = keywordShareView{}
	out, _, err = runS3B(t, db, "keyword-share", "nonexistent term", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || strings.Join(v.Meta.TermsMissing, ",") != "nonexistent term" || !strings.Contains(v.Meta.Note, "None of the requested terms") {
		t.Fatalf("mismatching term must return nothing: %s", out)
	}

	v = keywordShareView{}
	out, _, err = runS3B(t, db, "keyword-share", "music streaming", "--platform", "ipad", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 {
		t.Fatalf("no ipad captures exist: %s", out)
	}

	v = keywordShareView{}
	out, _, err = runS3B(t, db, "keyword-share", "music streaming", "--app", "com.example.nothere", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeS3B(t, out, &v)
	if v.Meta.Target != nil || !strings.Contains(v.Meta.Note, "not in the local store") {
		t.Fatalf("unresolvable target bundle id = %s", out)
	}
	for _, r := range v.Results {
		if r.Target {
			t.Fatalf("no row may be marked target: %+v", r)
		}
	}

	for _, args := range [][]string{
		{"keyword-share", "", "--json"},
		{"keyword-share", "music", "--top", "0"},
		{"keyword-share", "music", "--top", "201"},
		{"keyword-share", "music", "--source", "bogus"},
		{"keyword-share", "music", "--app", "not an app"},
		{"keyword-share", "music", "--platform", "toaster"},
		{"keyword-share", "music", "--limit", "-2"},
		{"keyword-share", "music", "--country", "zz"},
	} {
		if _, _, err := runS3B(t, db, args...); ExitCode(err) != 2 {
			t.Fatalf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}
