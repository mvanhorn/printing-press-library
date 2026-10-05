// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// One envelope everywhere (B12/B13) and the postings tracker contract.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// TestUJCEnvelopeEveryCommand runs every hand-written command with --json
// against one fake and checks each prints exactly the six-key envelope.
// Commands share the sandbox, so the 2-minute response cache keeps the
// request count (and the 3 s gate waits) low.
func TestUJCEnvelopeEveryCommand(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	run := func(want string, args ...string) map[string]any {
		t.Helper()
		r := ujcRun(t, "", args...)
		ujcWantCode(t, r, 0)
		env := ujcEnvelope(t, r.Stdout)
		if got := ujcMeta(env)["source"]; got != want {
			t.Fatalf("%v: meta.source = %v, want %s", args, got, want)
		}
		if got := ujcMeta(env)["command"]; got != args[0] {
			t.Fatalf("%v: meta.command = %v, want %s", args, got, args[0])
		}
		return env
	}

	env := run(uberjobs.SourceSite, "sync", "--json")
	if rows := ujcRows(env); len(rows) != 1 || rows[0]["unique_count"] != float64(56) || rows[0]["facets_updated"] != true {
		t.Fatalf("sync row = %v, want 56 postings and facets refreshed", rows)
	}

	env = run(uberjobs.SourceSite, "postings", "--country", "GBR", "--json", "--data-source", "live")
	if ujcInt(t, env["hits"]) != 7 || len(ujcRows(env)) != 7 {
		t.Fatalf("postings GBR hits=%v rows=%d, want 7", env["hits"], len(ujcRows(env)))
	}

	env = run(uberjobs.SourceSite, "get", ujcIDNewestNLD, "--json", "--data-source", "live")
	if ids := ujcRowIDs(ujcRows(env)); len(ids) != 1 || ids[0] != ujcIDNewestNLD {
		t.Fatalf("get rows = %v, want [%s]", ids, ujcIDNewestNLD)
	}

	env = run(uberjobs.SourceSite, "facets", "--json", "--data-source", "live")
	if len(ujcRows(env)) == 0 {
		t.Fatalf("facets returned no rows")
	}

	env = run(uberjobs.SourceLocal, "save", "gbr", "", "--country", "GBR", "--json")
	if ids := ujcRowIDs(ujcRows(env)); len(ids) != 1 || ids[0] != "gbr" {
		t.Fatalf("save rows = %v, want [gbr]", ids)
	}

	env = run(uberjobs.SourceLocal, "searches", "--json")
	if ids := ujcRowIDs(ujcRows(env)); len(ids) != 1 || ids[0] != "gbr" {
		t.Fatalf("searches rows = %v, want [gbr]", ids)
	}

	env = run(uberjobs.SourceSite, "new", "gbr", "--json", "--data-source", "live")
	if rows := ujcRows(env); len(rows) != 1 || rows[0]["baseline_established"] != true {
		t.Fatalf("first new = %v, want the baseline established", rows)
	}

	env = run(uberjobs.SourceSite, "new", "--all", "--json", "--data-source", "live")
	if rows := ujcRows(env); len(rows) != 1 || rows[0]["baseline_advanced"] != true || rows[0]["added_count"] != float64(0) {
		t.Fatalf("second new = %v, want an advance with nothing added", rows)
	}

	env = run(uberjobs.SourceLocal, "check", ujcIDNewestNLD, "--json")
	if rows := ujcRows(env); len(rows) != 1 || rows[0]["status"] != "open" {
		t.Fatalf("check rows = %v, want %s open from the fresh sync", rows, ujcIDNewestNLD)
	}

	run(uberjobs.SourceLocal, "screen", "--country", "GBR", "--exclude", "visa", "--json")
	run(uberjobs.SourceLocal, "stats", "--by", "country", "--json")

	env = run(uberjobs.SourceLocal, "searches", "--delete", "gbr", "--json")
	if rows := ujcRows(env); len(rows) != 1 || rows[0]["deleted"] != true {
		t.Fatalf("searches --delete rows = %v, want one deleted row", rows)
	}

	env = run(uberjobs.SourceLocal, "new", "--all", "--json")
	if note, _ := ujcMeta(env)["note"].(string); len(ujcRows(env)) != 0 || !strings.Contains(note, "no saved searches") {
		t.Fatalf("new --all with no searches = %v, want empty results and a note", env)
	}
	// 3 for sync (probe, page, facets) and 2 for the GBR read; everything
	// else came from the response cache or the local store.
	if n := site.Count(""); n != 5 {
		t.Fatalf("fake saw %d requests, want 5: %v", n, site.Requests())
	}
}

// TestUJCEnvelopeColdStore guards the empty paths: a store with no rows, no
// sync, and no snapshot still prints the envelope with results [] (never
// null), and no command dials out.
func TestUJCEnvelopeColdStore(t *testing.T) {
	ujcIsolate(t, "", "")
	cases := [][]string{
		{"searches", "--json"},
		{"new", "--all", "--json"},
		{"facets", "--data-source", "local", "--json"},
		{"postings", "--country", "GBR", "--data-source", "local", "--json"},
		{"stats", "--data-source", "local", "--json"},
		{"screen", "--exclude", "visa", "--data-source", "local", "--json"},
	}
	for _, args := range cases {
		r := ujcRun(t, "", args...)
		ujcWantCode(t, r, 0)
		if !strings.Contains(r.Stdout, `"results": []`) {
			t.Fatalf("%v: results is not an empty array:\n%s", args, r.Stdout)
		}
		env := ujcEnvelope(t, r.Stdout)
		if ujcMeta(env)["source"] != uberjobs.SourceLocal {
			t.Fatalf("%v: meta.source = %v, want local", args, ujcMeta(env)["source"])
		}
	}
	// check answers every id even with no history: unknown, never closed.
	r := ujcRun(t, "", "check", ujcIDNewestNLD, "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	rows := ujcRows(ujcEnvelope(t, r.Stdout))
	if len(rows) != 1 || rows[0]["status"] != "unknown" {
		t.Fatalf("cold check = %v, want one unknown row", rows)
	}
}

// TestUJCEnvelopeStorePrecreatedWithoutTables covers B12's trap: the learn
// loop can create the store file before any uj_ table exists, so read-only
// paths must treat "no such table" as empty, not as an error.
func TestUJCEnvelopeStorePrecreatedWithoutTables(t *testing.T) {
	ujcIsolate(t, "", "")
	s, err := store.OpenWithContext(context.Background(), ujcDefaultDB())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if !storeFileExists("") {
		t.Fatal("store file was not pre-created")
	}
	r := ujcRun(t, "", "check", ujcIDNewestNLD, "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	if rows := ujcRows(ujcEnvelope(t, r.Stdout)); len(rows) != 1 || rows[0]["status"] != "unknown" {
		t.Fatalf("check on a table-less store = %v, want unknown", rows)
	}
	r = ujcRun(t, "", "new", "--all", "--json", "--dry-run")
	ujcWantCode(t, r, 0)
	if !strings.Contains(r.Stdout, "none saved yet") {
		t.Fatalf("new --all --dry-run on a table-less store = %s, want 'none saved yet'", r.Stdout)
	}
}

// ujcAgentRows finds the posting rows in --agent output whether or not the
// generated agent wrapper nested the envelope (see the reported bug).
func ujcAgentRows(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal([]byte(stdout), &top); err != nil {
		t.Fatalf("--agent output is not a JSON object: %v\n%s", err, ujcTrim(stdout))
	}
	res := top["results"]
	if inner, ok := res.(map[string]any); ok {
		res = inner["results"]
	}
	list, ok := res.([]any)
	if !ok {
		t.Fatalf("--agent output has no results array:\n%s", ujcTrim(stdout))
	}
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		m, _ := v.(map[string]any)
		out = append(out, m)
	}
	return out
}

// TestUJCAgentKeepsDescription guards the tracker contract under --agent:
// the generated compaction strips "description" from lists unless the
// command keeps it, and the tracker reads it.
func TestUJCAgentKeepsDescription(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	r := ujcRun(t, "", "postings", "--country", "GBR", "--limit", "3", "--data-source", "local", "--agent")
	ujcWantCode(t, r, 0)
	rows := ujcAgentRows(t, r.Stdout)
	if len(rows) != 3 {
		t.Fatalf("--agent returned %d rows, want 3", len(rows))
	}
	for _, row := range rows {
		if d, _ := row["description"].(string); len(d) < 200 {
			t.Fatalf("row %v lost its description under --agent: %q", row["id"], d)
		}
		for _, k := range uberjobs.ContractFields {
			if _, ok := row[k]; !ok {
				t.Fatalf("row %v lost contract key %q under --agent", row["id"], k)
			}
		}
	}
}

// TestUJCSelectKeepsOnlyRowFields checks --select reaches into results.
func TestUJCSelectKeepsOnlyRowFields(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	r := ujcRun(t, "", "postings", "--country", "GBR", "--limit", "3", "--data-source", "local", "--json", "--select", "results.id,results.title")
	ujcWantCode(t, r, 0)
	var top map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &top); err != nil {
		t.Fatal(err)
	}
	rows := ujcRows(top)
	if len(rows) != 3 {
		t.Fatalf("--select returned %d rows, want 3:\n%s", len(rows), r.Stdout)
	}
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if strings.Join(keys, ",") != "id,title" {
			t.Fatalf("--select row keys = %v, want [id title]", keys)
		}
		if id, _ := row["id"].(string); !validPostingID(id) {
			t.Fatalf("--select row id = %v, want a posting id", row["id"])
		}
	}
}

// TestUJCPostingsCSV checks --csv renders nested locations as text a sheet
// can hold, never Go's map[...] formatting (B13).
func TestUJCPostingsCSV(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	r := ujcRun(t, "", "postings", "--country", "SAU", "--limit", "0", "--data-source", "local", "--csv")
	ujcWantCode(t, r, 0)
	records, err := csv.NewReader(strings.NewReader(r.Stdout)).ReadAll()
	if err != nil {
		t.Fatalf("--csv output does not parse: %v\n%s", err, ujcTrim(r.Stdout))
	}
	if len(records) != 3 {
		t.Fatalf("--csv gave %d records, want a header and the 2 SAU postings", len(records))
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[h] = i
	}
	for _, k := range []string{"id", "title", "locations", "description"} {
		if _, ok := col[k]; !ok {
			t.Fatalf("--csv header %v lacks %q", records[0], k)
		}
	}
	ids := []string{}
	for _, rec := range records[1:] {
		for _, cell := range rec {
			if strings.Contains(cell, "map[") {
				t.Fatalf("--csv cell holds Go map formatting: %q", cell)
			}
		}
		ids = append(ids, rec[col["id"]])
		if !strings.Contains(rec[col["locations"]], "Saudi Arabia") {
			t.Fatalf("locations cell %q does not name Saudi Arabia", rec[col["locations"]])
		}
	}
	if !ujcContains(ids, ujcIDFirstRow) {
		t.Fatalf("--csv ids = %v, want %s (Saudi Arabia is its second location)", ids, ujcIDFirstRow)
	}
}

// ujcByRecent orders postings newest true date first, floor and undated
// last, ties by id descending, computed independently of sortPostings.
func ujcByRecent(ps []uberjobs.Posting) []uberjobs.Posting {
	out := append([]uberjobs.Posting(nil), ps...)
	key := func(p uberjobs.Posting) int64 {
		if p.PostedDateIsFloor || p.PostedRaw == nil {
			return -1 << 62
		}
		ts, err := time.Parse(time.RFC3339, *p.PostedRaw)
		if err != nil {
			return -1 << 62
		}
		return ts.Unix()
	}
	sort.SliceStable(out, func(i, j int) bool {
		if key(out[i]) != key(out[j]) {
			return key(out[i]) > key(out[j])
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func ujcInCountry(p uberjobs.Posting, iso string) bool {
	for _, l := range p.Locations {
		if l.CountryCode != nil && *l.CountryCode == iso {
			return true
		}
	}
	return false
}

func ujcFilterCountry(ps []uberjobs.Posting, iso string) []uberjobs.Posting {
	var out []uberjobs.Posting
	for _, p := range ps {
		if ujcInCountry(p, iso) {
			out = append(out, p)
		}
	}
	return out
}

// TestUJCPostingsTrackerContract pins what the job tracker parses: hits is
// counted before truncation, --offset/--limit page a deterministic newest-
// first order, and every row carries every contract key, nulls included.
func TestUJCPostingsTrackerContract(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	r := ujcRun(t, "", "postings", "--country", "GBR", "--limit", "2", "--offset", "1", "--sort", "recent", "--json", "--data-source", "live")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	gbr := ujcByRecent(ujcFilterCountry(site.Postings(t), "GBR"))
	if len(gbr) != 7 {
		t.Fatalf("fixture has %d GBR postings, want 7", len(gbr))
	}
	if ujcInt(t, env["hits"]) != 7 || ujcInt(t, env["returned"]) != 2 || ujcInt(t, env["scanned"]) != 7 || env["scan_cap_hit"] != false {
		t.Fatalf("hits=%v returned=%v scanned=%v scan_cap_hit=%v, want 7/2/7/false", env["hits"], env["returned"], env["scanned"], env["scan_cap_hit"])
	}
	meta := ujcMeta(env)
	if meta["complete"] != true || meta["fallback"] != false || meta["requests"] != float64(2) {
		t.Fatalf("meta = %v, want a complete two-request read with no fallback", meta)
	}
	rows := ujcRows(env)
	if got, want := ujcRowIDs(rows), []string{gbr[1].ID, gbr[2].ID}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("offset 1 limit 2 ids = %v, want %v", got, want)
	}
	want := append([]string(nil), uberjobs.ContractFields...)
	sort.Strings(want)
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if strings.Join(keys, ",") != strings.Join(want, ",") {
			t.Fatalf("row keys = %v, want exactly the contract fields %v", keys, want)
		}
		if v, ok := row["basic_qualifications"]; !ok || v != nil {
			t.Fatalf("basic_qualifications = %v (present %v), want an explicit null", v, ok)
		}
		if row["employer"] != "uber" || row["source"] != uberjobs.SourceSite || row["country_code"] != "GBR" {
			t.Fatalf("row %v: employer/source/country = %v/%v/%v", row["id"], row["employer"], row["source"], row["country_code"])
		}
		if u, _ := row["url"].(string); !strings.HasPrefix(u, site.URL+"/") || !strings.HasSuffix(u, "/") {
			t.Fatalf("row url = %q, want a jobs URL on the configured base", u)
		}
	}

	// Client-side description filters ride on the cached GBR read: each has
	// a negative and a positive case, matched case-insensitively.
	const phrase = "WORK AUTHORIZATION IN THE UK"
	r = ujcRun(t, "", "postings", "--country", "GBR", "--limit", "0", "--json", "--data-source", "live", "--description-not-contains", phrase)
	ujcWantCode(t, r, 0)
	ids := ujcRowIDs(ujcRows(ujcEnvelope(t, r.Stdout)))
	if len(ids) != 6 || ujcContains(ids, ujcIDGBRIntern) {
		t.Fatalf("--description-not-contains kept %v, want the 6 GBR postings without %s", ids, ujcIDGBRIntern)
	}
	r = ujcRun(t, "", "postings", "--country", "GBR", "--limit", "0", "--json", "--data-source", "live", "--description-contains", strings.ToLower(phrase))
	ujcWantCode(t, r, 0)
	if ids := ujcRowIDs(ujcRows(ujcEnvelope(t, r.Stdout))); len(ids) != 1 || ids[0] != ujcIDGBRIntern {
		t.Fatalf("--description-contains kept %v, want only %s", ids, ujcIDGBRIntern)
	}

	// Bad input fails with a usage error before any request.
	before := site.Count("")
	for _, args := range [][]string{
		{"postings", "--country", "XQZ", "--json", "--data-source", "live"},
		{"postings", "--country", "GBR", "--posted-within", "soon", "--json", "--data-source", "live"},
		{"postings", "--country", "GBR", "--posted-within", "-7d", "--json", "--data-source", "live"},
		{"postings", "--country", "GBR", "--sort", "salary", "--json", "--data-source", "live"},
	} {
		bad := ujcRun(t, "", args...)
		ujcWantCode(t, bad, 2)
		if bad.Stdout != "" {
			t.Fatalf("%v printed output on a usage error: %s", args, bad.Stdout)
		}
	}
	if n := site.Count(""); n != before {
		t.Fatalf("usage errors sent %d requests, want 0", n-before)
	}
}

// TestUJCPostingsLocalOrderAndWindow checks floor-dated rows sort last with
// null dates, and --posted-within drops them and anything older than the
// window, on the local store.
func TestUJCPostingsLocalOrderAndWindow(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	usa := ujcByRecent(ujcFilterCountry(site.Postings(t), "USA"))

	r := ujcRun(t, "", "postings", "--country", "USA", "--limit", "0", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	rows := ujcRows(ujcEnvelope(t, r.Stdout))
	want := make([]string, 0, len(usa))
	for _, p := range usa {
		want = append(want, p.ID)
	}
	if got := ujcRowIDs(rows); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("USA order = %v, want %v", got, want)
	}
	floors := 0
	for i, row := range rows {
		if row["posted_date_is_floor"] != true {
			if floors > 0 {
				t.Fatalf("dated row %v sorts after a floor row", row["id"])
			}
			continue
		}
		floors++
		if row["posted_date"] != nil || row["posted_on"] != nil || row["posted_raw"] != uberjobs.DateFloor {
			t.Fatalf("floor row %d %v: posted_date=%v posted_on=%v posted_raw=%v", i, row["id"], row["posted_date"], row["posted_on"], row["posted_raw"])
		}
	}
	if floors != 2 {
		t.Fatalf("USA has %d floor rows, want 2 (%s and 157568)", floors, ujcIDFloorUSA)
	}

	r = ujcRun(t, "", "postings", "--country", "USA", "--limit", "0", "--posted-within", "7d", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	windowed := ujcRows(ujcEnvelope(t, r.Stdout))
	now := time.Now().UTC()
	if len(windowed) == 0 || windowed[0]["id"] != usa[0].ID {
		t.Fatalf("--posted-within 7d dropped the newest USA posting: %v", ujcRowIDs(windowed))
	}
	for _, row := range windowed {
		if row["posted_date_is_floor"] == true {
			t.Fatalf("--posted-within kept floor row %v", row["id"])
		}
		raw, _ := row["posted_raw"].(string)
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil || now.Sub(ts) > 8*24*time.Hour {
			t.Fatalf("--posted-within 7d kept %v posted %v", row["id"], row["posted_raw"])
		}
	}
	if len(windowed) >= len(rows) {
		t.Fatalf("--posted-within 7d kept %d of %d rows, want old rows dropped", len(windowed), len(rows))
	}

	// The country filter matches any location, not just the primary one:
	// 302016 lists the UAE first and Saudi Arabia second.
	r = ujcRun(t, "", "postings", "--country", "SAU", "--limit", "0", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	sau := ujcRows(ujcEnvelope(t, r.Stdout))
	found := false
	for _, row := range sau {
		if row["id"] == ujcIDFirstRow {
			found = true
			if row["country_code"] != "ARE" {
				t.Fatalf("%s primary country_code = %v, want ARE", ujcIDFirstRow, row["country_code"])
			}
		}
	}
	if !found || len(sau) != 2 {
		t.Fatalf("--country SAU = %v, want 2 rows including %s", ujcRowIDs(sau), ujcIDFirstRow)
	}
}

// TestUJCReadLiveClientFilters exercises the live path's client-side
// filters directly (readLive then applyClientFilters, as gatherPostings
// does), so the floor rule is proven on live rows too.
func TestUJCReadLiveClientFilters(t *testing.T) {
	site := ujcNewFake(t)
	ctx := context.Background()
	f := uberjobs.Filters{Countries: []string{"USA"}, PostedWithin: "7d"}
	read, err := readLive(ctx, ujcClient(site, nil), f)
	if err != nil {
		t.Fatal(err)
	}
	if read.Source != uberjobs.SourceSite || !read.Complete || read.Requests != 2 {
		t.Fatalf("read source=%s complete=%v requests=%d, want a complete two-request site read", read.Source, read.Complete, read.Requests)
	}
	all := len(read.Postings)
	rows, err := applyClientFilters(read.Postings, f, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || len(rows) >= all {
		t.Fatalf("7d window kept %d of %d USA rows, want some but not all", len(rows), all)
	}
	for _, p := range rows {
		if p.PostedDateIsFloor || p.ID == ujcIDFloorUSA {
			t.Fatalf("7d window kept floor row %s", p.ID)
		}
	}
	if _, err := applyClientFilters(read.Postings, uberjobs.Filters{PostedWithin: "0d"}, time.Now()); ujcCode(err) != 2 {
		t.Fatalf("a zero window: err = %v, want a usage error", err)
	}
}

// TestUJCPostingsFilterFlags gives the remaining filters a positive and a
// negative case: server-side keys must reach the site under the exact query
// names, the work pattern applies client-side, and the local store applies
// every filter itself.
func TestUJCPostingsFilterFlags(t *testing.T) {
	site := ujcNewFake(t)
	ctx := context.Background()
	f := uberjobs.Filters{Countries: []string{"GBR"}, Team: "Sales", SubTeam: "Account Management", ContractType: "Full time", Query: "Enterprise"}
	read, err := readLive(ctx, ujcClient(site, nil), f)
	if err != nil {
		t.Fatal(err)
	}
	probe := site.Requests()[0]
	for _, kv := range []string{"search=Enterprise", "team=Sales", "subTeam=Account+Management", "contractTypes=Full+time", "countries=United+Kingdom", "pagesize=1"} {
		if !strings.Contains(probe, kv) {
			t.Fatalf("size probe %q lacks %q", probe, kv)
		}
	}
	if len(read.Postings) == 0 || len(read.Postings) >= 6 {
		t.Fatalf("keyword-narrowed read returned %d rows, want a strict subset of the 6 GBR Account Management postings", len(read.Postings))
	}
	for _, p := range read.Postings {
		if deref(p.JobCategory, "") != "Sales" || deref(p.SubTeam, "") != "Account Management" || !ujcInCountry(p, "GBR") {
			t.Fatalf("row %s escaped the server filters: %v / %v", p.ID, p.JobCategory, p.SubTeam)
		}
	}
	read, err = readLive(ctx, ujcClient(site, nil), uberjobs.Filters{Team: "Nope"})
	if err != nil || len(read.Postings) != 0 || !read.Complete || read.Requests != 1 {
		t.Fatalf("unknown team: rows=%d complete=%v requests=%d err=%v, want an empty complete one-request read", len(read.Postings), read.Complete, read.Requests, err)
	}

	all, err := readLive(ctx, ujcClient(site, nil), uberjobs.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	interns, _ := applyClientFilters(all.Postings, uberjobs.Filters{WorkPattern: "INTERN"}, time.Now())
	ids := []string{}
	for _, p := range interns {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	if strings.Join(ids, ",") != "303092,303139,303140" {
		t.Fatalf("--work-pattern intern = %v, want the 3 Intern postings", ids)
	}
	regular, _ := applyClientFilters(all.Postings, uberjobs.Filters{WorkPattern: "Regular"}, time.Now())
	for _, p := range regular {
		if deref(p.WorkPattern, "") != "Regular" {
			t.Fatalf("--work-pattern Regular kept %s (%v)", p.ID, p.WorkPattern)
		}
	}
	if len(regular) == 0 || len(regular)+len(interns) >= len(all.Postings) {
		t.Fatalf("Regular=%d Intern=%d of %d, want other patterns and blanks excluded from both", len(regular), len(interns), len(all.Postings))
	}

	// The local store applies the same filters offline, case-insensitively.
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	local := func(want int, args ...string) []map[string]any {
		t.Helper()
		r := ujcRun(t, "", append([]string{"postings", "--data-source", "local", "--limit", "0", "--json"}, args...)...)
		ujcWantCode(t, r, 0)
		rows := ujcRows(ujcEnvelope(t, r.Stdout))
		if want >= 0 && len(rows) != want {
			t.Fatalf("local %v returned %d rows, want %d", args, len(rows), want)
		}
		return rows
	}
	for _, row := range local(6, "--country", "gbr", "--team", "sales", "--sub-team", "account management") {
		if row["job_category"] != "Sales" || row["sub_team"] != "Account Management" {
			t.Fatalf("local team filter kept %v", row["id"])
		}
	}
	local(0, "--team", "Nope")
	local(0, "--contract-type", "Part time")
	local(3, "--work-pattern", "intern")
	var want []string
	for _, p := range site.Postings(t) {
		hay := strings.ToLower(p.Title + " " + deref(p.JobCategory, "") + " " + deref(p.SubTeam, "") + " " + deref(p.Description, ""))
		if strings.Contains(hay, "u4b") {
			want = append(want, p.ID)
		}
	}
	got := ujcRowIDs(local(-1, "--base-query", "U4B"))
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") || !ujcContains(got, ujcIDGBRIntern) || len(got) >= 56 {
		t.Fatalf("local --base-query U4B = %v, want %v", got, want)
	}
}

// TestUJCPostingsIncompleteReadIsDisclosed: a short read is still printed,
// but meta.complete is false and the note says why (B6).
func TestUJCPostingsIncompleteReadIsDisclosed(t *testing.T) {
	site := ujcNewFake(t)
	site.SetTotalBump(4)
	read, err := readLive(context.Background(), ujcClient(site, nil), uberjobs.Filters{Countries: []string{"GBR"}})
	if err != nil {
		t.Fatal(err)
	}
	if read.Complete || read.Total != 11 || len(read.Postings) != 7 || !strings.Contains(read.Note, "incomplete") {
		t.Fatalf("short read: complete=%v total=%d rows=%d note=%q, want an incomplete read of 7 of 11", read.Complete, read.Total, len(read.Postings), read.Note)
	}
}

// TestUJCReadLiveOracleFallbackMatchesAnyLocation checks the fallback's own
// country filter, which runs client-side over every Oracle location.
func TestUJCReadLiveOracleFallbackMatchesAnyLocation(t *testing.T) {
	site, oracle := ujcNewFake(t), ujcNewFake(t)
	site.SetMode("challenge")
	read, err := readLive(context.Background(), ujcClient(site, oracle), uberjobs.Filters{Countries: []string{"SAU"}})
	if err != nil {
		t.Fatal(err)
	}
	if read.Source != uberjobs.SourceOracle || !read.Fallback || !strings.Contains(read.FallbackFrom, "refused") {
		t.Fatalf("read source=%s fallback=%v from=%q, want an oracle-ce fallback after the refusal", read.Source, read.Fallback, read.FallbackFrom)
	}
	ids := []string{}
	for _, p := range read.Postings {
		ids = append(ids, p.ID)
		if p.Description != nil || p.JobCategory != nil {
			t.Fatalf("oracle row %s carries description/job_category, want null", p.ID)
		}
	}
	sort.Strings(ids)
	if strings.Join(ids, ",") != "302016,302819" || read.Total != 2 {
		t.Fatalf("SAU fallback ids = %v (total %d), want [302016 302819]", ids, read.Total)
	}
	if n := site.Count("/api/jobs/search/"); n != 1 {
		t.Fatalf("refusing site saw %d requests, want 1", n)
	}

	// Team filters cannot be applied by Oracle, so no fallback is tried.
	site2, oracle2 := ujcNewFake(t), ujcNewFake(t)
	site2.SetMode("challenge")
	_, err = readLive(context.Background(), ujcClient(site2, oracle2), uberjobs.Filters{Team: "Sales"})
	if !uberjobs.IsRefusal(err) || !strings.Contains(err.Error(), "no fallback was attempted") {
		t.Fatalf("team-filtered refusal: err = %v, want the refusal with no fallback", err)
	}
	if n := oracle2.Count(""); n != 0 {
		t.Fatalf("Oracle saw %d requests for a team-filtered read, want 0", n)
	}
}
