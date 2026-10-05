// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// screen (phrase verdicts with evidence, B21) and stats (per-group counts).

package cli

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func ujcStrPtr(s string) *string { return &s }

func ujcDescPosting(id, desc string) uberjobs.Posting {
	p := uberjobs.Posting{ID: id, Title: "posting " + id}
	if desc != "" {
		p.Description = &desc
	}
	return p
}

// TestUJCScreenPostingVerdicts pins the verdict rules and the evidence a
// reader needs to judge a negation: exclude match drops with the sentence,
// a missing require drops with matched false, no description is unscreened.
func TestUJCScreenPostingVerdicts(t *testing.T) {
	desc := "Join our Amsterdam team. Fluent Dutch is required for this role. We offer great benefits."
	p := ujcDescPosting("1", desc)

	row := screenPosting(p, nil, []string{"fluent dutch"})
	if row.Verdict != "drop" || len(row.Evidence) != 1 {
		t.Fatalf("exclude match = %+v, want drop with one evidence entry", row)
	}
	ev := row.Evidence[0]
	if ev.Kind != "exclude" || !ev.Matched || ev.Strength == nil || *ev.Strength != "word" || ev.Sentence == nil || *ev.Sentence != "Fluent Dutch is required for this role." {
		t.Fatalf("exclude evidence = %+v, want a word match on the second sentence", ev)
	}

	row = screenPosting(p, []string{"Python"}, nil)
	if row.Verdict != "drop" || row.Evidence[0].Matched || row.Evidence[0].Strength != nil || row.Evidence[0].Sentence != nil || row.Evidence[0].Kind != "require" {
		t.Fatalf("missing require = %+v, want drop with matched false and no sentence", row)
	}

	row = screenPosting(p, []string{"benefits"}, []string{"german"})
	if row.Verdict != "keep" || len(row.Evidence) != 2 || row.Evidence[0].Matched || !row.Evidence[1].Matched {
		t.Fatalf("require met, exclude absent = %+v, want keep", row)
	}

	row = screenPosting(ujcDescPosting("2", ""), []string{"x"}, []string{"y"})
	if row.Verdict != "unscreened" || len(row.Evidence) != 0 {
		t.Fatalf("no description = %+v, want unscreened with no evidence", row)
	}
	blank := ujcDescPosting("3", "   ")
	if row = screenPosting(blank, nil, []string{"y"}); row.Verdict != "unscreened" {
		t.Fatalf("blank description verdict = %s, want unscreened", row.Verdict)
	}
}

// TestUJCScreenMatchStrength guards B21: a phrase found only inside a longer
// word is labelled substring, and a word-boundary match elsewhere wins.
func TestUJCScreenMatchStrength(t *testing.T) {
	row := screenPosting(ujcDescPosting("1", "We value support for customers."), nil, []string{"port"})
	ev := row.Evidence[0]
	if !ev.Matched || *ev.Strength != "substring" || *ev.Sentence != "We value support for customers." || row.Verdict != "drop" {
		t.Fatalf("phrase inside a word = %+v, want a substring match", ev)
	}
	row = screenPosting(ujcDescPosting("2", "Support is key. The port is busy."), nil, []string{"PORT"})
	ev = row.Evidence[0]
	if *ev.Strength != "word" || *ev.Sentence != "The port is busy." {
		t.Fatalf("word match after a substring = %+v, want the word sentence", ev)
	}
	row = screenPosting(ujcDescPosting("3", "Ports, harbours."), nil, []string{"port"})
	if *row.Evidence[0].Strength != "substring" {
		t.Fatalf("'port' in 'Ports' = %s, want substring", *row.Evidence[0].Strength)
	}
	if got := cleanPhrases([]string{"  fluent \t Dutch ", "", "   "}); len(got) != 1 || got[0] != "fluent Dutch" {
		t.Fatalf("cleanPhrases = %q, want [fluent Dutch]", got)
	}
}

// TestUJCScreenOrderByVerdict keeps the sorted order inside each group.
func TestUJCScreenOrderByVerdict(t *testing.T) {
	in := []screenRow{
		{Posting: uberjobs.Posting{ID: "a"}, Verdict: "drop"},
		{Posting: uberjobs.Posting{ID: "b"}, Verdict: "keep"},
		{Posting: uberjobs.Posting{ID: "c"}, Verdict: "unscreened"},
		{Posting: uberjobs.Posting{ID: "d"}, Verdict: "keep"},
		{Posting: uberjobs.Posting{ID: "e"}, Verdict: "drop"},
	}
	var ids []string
	for _, r := range orderByVerdict(in) {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "b,d,c,a,e" {
		t.Fatalf("orderByVerdict = %v, want b,d,c,a,e", ids)
	}
}

// ujcSeedWithUndescribed seeds the corpus plus one GBR posting that has no
// description (as an Oracle-sourced row would), then closes the store.
func ujcSeedWithUndescribed(t *testing.T, site *ujcFake) {
	t.Helper()
	extra := uberjobs.Posting{
		ID: "999000001", Title: "Synthetic GBR posting with no description", Employer: "uber", Source: uberjobs.SourceOracle,
		CountryCode: ujcStrPtr("GBR"), Country: ujcStrPtr("United Kingdom"),
		Locations:    []uberjobs.Location{{CountryCode: ujcStrPtr("GBR"), Country: ujcStrPtr("United Kingdom")}},
		SalaryRanges: []uberjobs.SalaryRange{},
	}
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		ujcApply(t, db, append(site.Postings(t), extra), "all", true, time.Now())
	})
}

// TestUJCScreenCommand runs screen on the local store: --verdict all
// ordering, meta.extra counts, the default keep view, and usage errors.
func TestUJCScreenCommand(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedWithUndescribed(t, site)

	r := ujcRun(t, "", "screen", "--country", "GBR", "--exclude", "VISA", "--verdict", "all", "--limit", "0", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if ujcMeta(env)["source"] != uberjobs.SourceLocal {
		t.Fatalf("auto screen with a complete sync: source = %v, want local", ujcMeta(env)["source"])
	}
	extra, _ := ujcMeta(env)["extra"].(map[string]any)
	if extra["candidates"] != float64(8) || extra["keep"] != float64(6) || extra["drop"] != float64(1) || extra["unscreened"] != float64(1) || extra["verdict"] != "all" {
		t.Fatalf("meta.extra = %v, want 8 candidates: 6 keep, 1 drop, 1 unscreened", extra)
	}
	rank := map[string]int{"keep": 0, "unscreened": 1, "drop": 2}
	last := -1
	for _, row := range ujcRows(env) {
		v, _ := row["verdict"].(string)
		if rank[v] < last {
			t.Fatalf("--verdict all order broke at %v (%s)", row["id"], v)
		}
		last = rank[v]
		switch v {
		case "drop":
			evs, _ := row["evidence"].([]any)
			ev, _ := evs[0].(map[string]any)
			sentence, _ := ev["sentence"].(string)
			if row["id"] != ujcIDGBRIntern || ev["matched"] != true || !strings.Contains(strings.ToLower(sentence), "visa") {
				t.Fatalf("drop row %v evidence %v, want %s with the visa sentence", row["id"], ev, ujcIDGBRIntern)
			}
		case "unscreened":
			if row["id"] != "999000001" || row["description"] != nil {
				t.Fatalf("unscreened row = %v, want the posting with no description", row["id"])
			}
		}
	}

	r = ujcRun(t, "", "screen", "--country", "GBR", "--exclude", "visa", "--json")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	rows := ujcRows(env)
	if len(rows) != 6 || ujcInt(t, env["hits"]) != 6 {
		t.Fatalf("default verdict returned %d rows, want the 6 kept", len(rows))
	}
	for _, row := range rows {
		if row["verdict"] != "keep" || row["id"] == ujcIDGBRIntern {
			t.Fatalf("default view kept %v (%v)", row["id"], row["verdict"])
		}
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "1 dropped") {
		t.Fatalf("keep view note = %q, want the dropped count", note)
	}

	for _, args := range [][]string{
		{"screen", "--country", "GBR", "--json"},
		{"screen", "--country", "GBR", "--exclude", " ", "--json"},
		{"screen", "--country", "GBR", "--exclude", "visa", "--verdict", "maybe", "--json"},
		{"screen", "extra", "--exclude", "visa", "--json"},
		{"screen", "--country", "Atlantis", "--exclude", "visa", "--json"},
	} {
		ujcWantCode(t, ujcRun(t, "", args...), 2)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("local screen sent %d requests, want 0", n)
	}
}

// ujcCountryCounts counts postings per distinct location country,
// independently of statsGroups.
func ujcCountryCounts(ps []uberjobs.Posting) map[string]int {
	out := map[string]int{}
	for _, p := range ps {
		seen := map[string]bool{}
		for _, l := range p.Locations {
			if l.CountryCode != nil && !seen[*l.CountryCode] {
				seen[*l.CountryCode] = true
				out[*l.CountryCode]++
			}
		}
	}
	return out
}

// TestUJCGroupStatsCountries: a posting in several countries counts once in
// each, one with three US locations counts once, and rows sort by open desc
// then id.
func TestUJCGroupStatsCountries(t *testing.T) {
	site := ujcNewFake(t)
	ps := site.Postings(t)
	inputs := make([]statsInput, 0, len(ps))
	for _, p := range ps {
		inputs = append(inputs, statsInput{posting: p})
	}
	now := time.Now().UTC()
	rows := groupStats(inputs, "country", false, now)
	want := ujcCountryCounts(ps)
	got := map[string]statsRow{}
	for i, r := range rows {
		got[r.ID] = r
		if r.Opened30d != nil || r.Closed30d != nil {
			t.Fatalf("row %s has churn without history: %v %v", r.ID, r.Opened30d, r.Closed30d)
		}
		if i > 0 && (rows[i-1].Open < r.Open || (rows[i-1].Open == r.Open && rows[i-1].ID > r.ID)) {
			t.Fatalf("rows not sorted by open desc then id at %s", r.ID)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("groups = %d, want %d", len(got), len(want))
	}
	for iso, n := range want {
		if got[iso].Open != n {
			t.Errorf("%s open = %d, want %d", iso, got[iso].Open, n)
		}
	}
	if got["ARE"].Open != 1 || got["SAU"].Open != 2 {
		t.Fatalf("ARE=%d SAU=%d, want 1 and 2 (302016 counts in both)", got["ARE"].Open, got["SAU"].Open)
	}
	if got["SAU"].Label == nil || *got["SAU"].Label != "Saudi Arabia" {
		t.Fatalf("SAU label = %v, want Saudi Arabia", got["SAU"].Label)
	}
	if got["USA"].Posted30d > got["USA"].Open || got["USA"].Posted7d > got["USA"].Posted30d {
		t.Fatalf("USA posted_7d=%d posted_30d=%d open=%d, want 7d <= 30d <= open", got["USA"].Posted7d, got["USA"].Posted30d, got["USA"].Open)
	}
	// The two USA floor rows never count as recently posted.
	floors := 0
	for _, p := range ps {
		if p.PostedDateIsFloor && ujcInCountry(p, "USA") {
			floors++
		}
	}
	if floors != 2 || got["USA"].Posted30d > got["USA"].Open-floors {
		t.Fatalf("USA posted_30d=%d counts floor rows (open %d, floors %d)", got["USA"].Posted30d, got["USA"].Open, floors)
	}
	if g := statsGroups(uberjobs.Posting{ID: "x"}, "country"); len(g) != 1 || g[0].id != "(none)" {
		t.Fatalf("posting without locations groups as %v, want (none)", g)
	}
}

// TestUJCGroupStatsHistory: with 30 days of history, opened_30d and
// closed_30d are integers and count first_seen and closed_on in the window.
func TestUJCGroupStatsHistory(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-24 * time.Hour).Format(time.RFC3339)
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	gbr := uberjobs.Posting{ID: "1", Locations: []uberjobs.Location{{CountryCode: ujcStrPtr("GBR")}}}
	inputs := []statsInput{
		{posting: gbr, firstSeen: recent},
		{posting: uberjobs.Posting{ID: "2", Locations: gbr.Locations}, firstSeen: old},
		{posting: uberjobs.Posting{ID: "3", Locations: gbr.Locations}, firstSeen: old, closedOn: &recent},
		{posting: uberjobs.Posting{ID: "4", Locations: []uberjobs.Location{{CountryCode: ujcStrPtr("NLD")}}}, firstSeen: old, closedOn: &old},
	}
	rows := groupStats(inputs, "country", true, now)
	if len(rows) != 1 || rows[0].ID != "GBR" {
		t.Fatalf("rows = %+v, want only GBR (NLD has nothing open and no recent close)", rows)
	}
	r := rows[0]
	if r.Open != 2 || r.Opened30d == nil || *r.Opened30d != 1 || r.Closed30d == nil || *r.Closed30d != 1 {
		t.Fatalf("GBR = open %d opened %v closed %v, want 2 / 1 / 1", r.Open, r.Opened30d, r.Closed30d)
	}
}

// TestUJCStatsCommand runs stats through RootCmd on the local store:
// category equals team, churn stays null until history covers 30 days,
// and bad flags exit 2 before anything runs.
func TestUJCStatsCommand(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now().Add(-time.Hour))

	results := func(args ...string) (map[string]any, string) {
		t.Helper()
		r := ujcRun(t, "", args...)
		ujcWantCode(t, r, 0)
		env := ujcEnvelope(t, r.Stdout)
		b, _ := json.Marshal(env["results"])
		return env, string(b)
	}
	env, team := results("stats", "--by", "team", "--limit", "0", "--json")
	_, category := results("stats", "--by", "category", "--limit", "0", "--json")
	if team != category {
		t.Fatalf("--by category differs from --by team:\n%s\n%s", category, team)
	}
	if ujcMeta(env)["source"] != uberjobs.SourceLocal {
		t.Fatalf("auto stats with a complete sync: source = %v", ujcMeta(env)["source"])
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "less than 30 days ago") {
		t.Fatalf("young history note = %q", note)
	}
	for _, row := range ujcRows(env) {
		if row["opened_30d"] != nil || row["closed_30d"] != nil {
			t.Fatalf("young history row %v has churn %v/%v, want null", row["id"], row["opened_30d"], row["closed_30d"])
		}
	}

	env, _ = results("stats", "--by", "country", "--limit", "2", "--json")
	if ujcInt(t, env["returned"]) != 2 || ujcInt(t, env["hits"]) != len(ujcCountryCounts(site.Postings(t))) {
		t.Fatalf("--limit 2: returned=%v hits=%v", env["returned"], env["hits"])
	}
	if rows := ujcRows(env); rows[0]["id"] != "USA" || ujcInt(t, rows[0]["open"]) < ujcInt(t, rows[1]["open"]) {
		t.Fatalf("top country rows = %v, want USA first by open", ujcRowIDs(rows))
	}

	// Backdate the first complete sync past 30 days and close one GBR
	// posting: churn becomes integers.
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		ujcInsertSyncRun(t, db, time.Now().Add(-40*24*time.Hour), time.Now().Add(-40*24*time.Hour), "all", true)
		site.Drop(ujcIDGBRAccount)
		ujcApply(t, db, site.Postings(t), "all", true, time.Now().Add(-30*time.Minute))
	})
	env, _ = results("stats", "--by", "country", "--limit", "0", "--json")
	if extra, _ := ujcMeta(env)["extra"].(map[string]any); extra["history_covers_30d"] != true || extra["by"] != "country" {
		t.Fatalf("meta.extra = %v, want history_covers_30d true", ujcMeta(env)["extra"])
	}
	var gbr map[string]any
	for _, row := range ujcRows(env) {
		if _, ok := row["opened_30d"].(float64); !ok {
			t.Fatalf("row %v opened_30d = %v, want an integer", row["id"], row["opened_30d"])
		}
		if row["id"] == "GBR" {
			gbr = row
		}
	}
	if gbr == nil || gbr["open"] != float64(6) || gbr["opened_30d"] != float64(7) || gbr["closed_30d"] != float64(1) {
		t.Fatalf("GBR row = %v, want open 6, opened_30d 7, closed_30d 1", gbr)
	}

	for _, args := range [][]string{
		{"stats", "--by", "salary", "--json"},
		{"stats", "extra", "--json"},
		{"stats", "--limit", "-1", "--json"},
		{"stats", "--data-source", "cache", "--json"},
	} {
		ujcWantCode(t, ujcRun(t, "", args...), 2)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("local stats sent %d requests, want 0", n)
	}
}

// TestUJCStatsLiveHasNoChurn: live counts have no history, so churn is null.
func TestUJCStatsLiveHasNoChurn(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	r := ujcRun(t, "", "stats", "--by", "country", "--limit", "0", "--data-source", "live", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if ujcMeta(env)["source"] != uberjobs.SourceSite {
		t.Fatalf("live stats source = %v", ujcMeta(env)["source"])
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "no local history") || !strings.Contains(note, "counts in each") {
		t.Fatalf("live stats note = %q", note)
	}
	want := ujcCountryCounts(site.Postings(t))
	rows := ujcRows(env)
	if len(rows) != len(want) {
		t.Fatalf("live stats groups = %d, want %d", len(rows), len(want))
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row["id"].(string))
		if row["opened_30d"] != nil || row["closed_30d"] != nil {
			t.Fatalf("live row %v has churn %v/%v, want null", row["id"], row["opened_30d"], row["closed_30d"])
		}
		if ujcInt(t, row["open"]) != want[row["id"].(string)] {
			t.Fatalf("live %v open = %v, want %d", row["id"], row["open"], want[row["id"].(string)])
		}
	}
	sort.Strings(ids)
	if !ujcContains(ids, "ARE") || !ujcContains(ids, "SAU") {
		t.Fatalf("live groups %v miss ARE or SAU", ids)
	}
}
