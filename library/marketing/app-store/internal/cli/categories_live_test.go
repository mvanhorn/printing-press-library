// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func decodeCategoryRows(t *testing.T, out string) []categoryRow {
	t.Helper()
	var rows []categoryRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("categories output is not a JSON row list: %v\n%s", err, out)
	}
	return rows
}

func chartPageHandler(t *testing.T, path, fixture string) func(r *http.Request) fakeResponse {
	body := fixtureBytes(t, fixture)
	return func(r *http.Request) fakeResponse {
		if r.URL.Host == "apps.apple.com" && r.URL.Path == path {
			return okBody(body)
		}
		return notFoundResp()
	}
}

func TestCategoriesListLiveReadsTheChartPage(t *testing.T) {
	env := newS2bEnv(t, false, chartPageHandler(t, "/us/iphone/charts/36", "charts_us_iphone_36.html"))
	out, errOut, err := env.run("categories", "list", "--live", "--json")
	if err != nil {
		t.Fatalf("categories list --live: %v\nstderr: %s", err, errOut)
	}
	rows := decodeCategoryRows(t, out)
	have := map[string]categoryRow{}
	for _, r := range rows {
		if r.Source != categorySourceLive {
			t.Fatalf("row %+v has source %q, want live", r, r.Source)
		}
		if strings.HasPrefix(r.GenreID, "70") {
			t.Fatalf("app category list leaked a game subgenre: %+v", r)
		}
		have[categoryKey(r)] = r
	}
	for _, key := range []string{"36|", "6026|", "6027|", "6018|", "36|10001"} {
		if _, ok := have[key]; !ok {
			t.Fatalf("live app categories lack %s; got %d rows", key, len(rows))
		}
	}
	// The apps chart page never lists Games (its charts live on the games
	// page), so the drift note must not report it as missing.
	if strings.Contains(errOut, "Games (6014)") {
		t.Fatalf("drift note falsely reports Games as missing from the apps page: %q", errOut)
	}
	if kids := have["36|10001"]; kids.ParentID != "36" || !strings.Contains(kids.ChartURL, "ageBandId=10001") {
		t.Fatalf("Kids 6-8 row = %+v, want parent 36 and an ageBandId chart URL", kids)
	}
	if n := env.fake.count("/iphone/charts/36"); n != 1 {
		t.Fatalf("--live sent %d chart page requests, want exactly 1", n)
	}
	if !strings.Contains(errOut, "note:") {
		t.Fatalf("--live should note how the live list compares with the built-in table on stderr; got %q", errOut)
	}
}

func TestCategoriesGamesLiveUsesTheGamesPage(t *testing.T) {
	env := newS2bEnv(t, false, chartPageHandler(t, "/pk/iphone/charts/6014", "charts_pk_iphone_6014.html"))
	out, errOut, err := env.run("categories", "games", "--live", "--country", "pk", "--json")
	if err != nil {
		t.Fatalf("categories games --live: %v\nstderr: %s", err, errOut)
	}
	rows := decodeCategoryRows(t, out)
	have := map[string]bool{}
	for _, r := range rows {
		if r.GenreID != "6014" && !strings.HasPrefix(r.GenreID, "70") {
			t.Fatalf("game category list leaked a non-game row: %+v", r)
		}
		if !strings.Contains(r.ChartURL, "/pk/iphone/charts/") {
			t.Fatalf("row %+v does not point at the pk storefront", r)
		}
		have[r.GenreID] = true
	}
	for _, id := range []string{"6014", "7003", "7012", "7014"} {
		if !have[id] {
			t.Fatalf("live game categories lack %s: %+v", id, rows)
		}
	}
	if n := env.fake.count("/pk/iphone/charts/6014"); n != 1 {
		t.Fatalf("--live sent %d games chart requests, want exactly 1", n)
	}
}

func TestCategoriesBuiltInSendsNoRequest(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("built-in categories sent a request: %s", r.URL)
		return notFoundResp()
	})
	out, _, err := env.run("categories", "list", "--json")
	if err != nil {
		t.Fatalf("categories list: %v", err)
	}
	rows := decodeCategoryRows(t, out)
	if len(rows) == 0 {
		t.Fatal("built-in categories returned no rows")
	}
	for _, r := range rows {
		if r.Source != categorySourceBuiltIn {
			t.Fatalf("row %+v has source %q, want built-in", r, r.Source)
		}
	}
	if urls := env.fake.urls(); len(urls) != 0 {
		t.Fatalf("built-in categories sent %d requests: %v", len(urls), urls)
	}
}

func TestCategoriesLiveThrottleExits7(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
	out, _, err := env.run("categories", "list", "--live", "--json")
	if code := ExitCode(err); code != 7 {
		t.Fatalf("throttled --live exit = %d (%v), want 7", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("throttled --live printed rows: %q", out)
	}
}

func TestCategoryDriftNotes(t *testing.T) {
	table := []categoryRow{{GenreID: "36", Name: "All Apps"}, {GenreID: "6018", Name: "Books"}}
	cases := []struct {
		name string
		live []categoryRow
		want []string
	}{
		{"same", []categoryRow{{GenreID: "36", Name: "All Apps"}, {GenreID: "6018", Name: "Books"}}, []string{"lists the same 2 categories"}},
		{"new genre", []categoryRow{{GenreID: "36", Name: "All Apps"}, {GenreID: "6018", Name: "Books"}, {GenreID: "6099", Name: "Future Genre"}}, []string{"on the live chart page but not in the built-in table: Future Genre (6099)"}},
		{"removed genre", []categoryRow{{GenreID: "36", Name: "All Apps"}}, []string{"in the built-in table but not on the live chart page: Books (6018)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeCategoryDrift(&buf, table, tc.live)
			for _, w := range tc.want {
				if !strings.Contains(buf.String(), w) {
					t.Fatalf("drift note %q lacks %q", buf.String(), w)
				}
			}
		})
	}
}
