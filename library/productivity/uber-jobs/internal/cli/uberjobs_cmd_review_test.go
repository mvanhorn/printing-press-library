// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Regression tests for the Phase 4.95 local code review (2026-10-05). Each
// one failed before its fix.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// TestUJCNewAllFailureHoldsEveryBaseline: when one saved search fails,
// new --all lists every search, advances no baseline (owner decision: all
// or nothing), and still hands the partial result to --deliver. Before the
// fix the error came back alone while the other searches had already
// advanced, so their additions were lost for every reader; in round 2 the
// rows printed but --deliver and MCP still lost them.
func TestUJCNewAllFailureHoldsEveryBaseline(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	db := ujcDefaultDB()
	site.Drop(ujcIDGBRIntern)
	ujcSeedFullSync(t, db, site, time.Now().Add(-2*time.Hour))
	ujcWantCode(t, ujcRun(t, "", "save", "a-gbr", "", "--country", "GBR", "--json"), 0)
	// A saved recency window the CLI would refuse today; it fails only once
	// the search has a baseline to diff.
	ujcWithStore(t, db, func(sdb *sql.DB) {
		if _, _, err := uberjobs.SaveSearch(context.Background(), sdb, "z-bad", uberjobs.Filters{PostedWithin: "bogus"}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	})
	ujcWantCode(t, ujcRun(t, "", "new", "--all", "--data-source", "local", "--json"), 0)

	site.Undrop(ujcIDGBRIntern)
	ujcSeedFullSync(t, db, site, time.Now().Add(-time.Hour))
	wantHeld := func(what string, env map[string]any) {
		t.Helper()
		rows := ujcRows(env)
		if len(rows) != 2 {
			t.Fatalf("%s: rows = %v, want a-gbr and z-bad", what, rows)
		}
		ok, bad := rows[0], rows[1]
		if note, _ := ok["note"].(string); ok["name"] != "a-gbr" || ok["baseline_advanced"] != false || ok["added_count"] != float64(1) || !strings.Contains(note, "baseline held because saved search z-bad failed") {
			t.Fatalf("%s: a-gbr row = %v, want the addition listed and the baseline held", what, ok)
		}
		if added, _ := ok["added"].([]any); len(added) != 1 || added[0].(map[string]any)["id"] != ujcIDGBRIntern {
			t.Fatalf("%s: a-gbr added = %v, want [%s]", what, ok["added"], ujcIDGBRIntern)
		}
		if note, _ := bad["note"].(string); bad["name"] != "z-bad" || bad["baseline_advanced"] != false || !strings.HasPrefix(note, "failed: ") {
			t.Fatalf("%s: z-bad row = %v, want a failed row", what, bad)
		}
		meta := ujcMeta(env)
		if note, _ := meta["note"].(string); meta["complete"] != false || !strings.Contains(note, "1 of 2 saved searches failed (z-bad), so no baseline advanced") {
			t.Fatalf("%s: meta = %v, want complete=false and the hold note", what, meta)
		}
	}
	r := ujcRun(t, "", "new", "--all", "--data-source", "local", "--json")
	ujcWantCode(t, r, 2)
	wantHeld("stdout", ujcEnvelope(t, r.Stdout))

	// Nothing advanced, so the same run again gives the same result; this
	// time through a --deliver file sink, which Execute skips on an error.
	out := filepath.Join(t.TempDir(), "new.json")
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = devnull // --deliver tees to the real stdout
	r = ujcRun(t, "", "new", "--all", "--data-source", "local", "--json", "--deliver", "file:"+out)
	os.Stdout = stdout
	_ = devnull.Close()
	ujcWantCode(t, r, 2)
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the --deliver sink got nothing: %v", err)
	}
	wantHeld("--deliver file", ujcEnvelope(t, string(raw)))

	// Once the broken search is gone, the held addition shows and advances.
	ujcWantCode(t, ujcRun(t, "", "searches", "--delete", "z-bad", "--json"), 0)
	r = ujcRun(t, "", "new", "--all", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	rows := ujcRows(ujcEnvelope(t, r.Stdout))
	if len(rows) != 1 || rows[0]["added_count"] != float64(1) || rows[0]["baseline_advanced"] != true {
		t.Fatalf("after deleting z-bad: rows = %v, want the held addition listed and the baseline advanced", rows)
	}
}

// TestUJCBlankDescriptionPhraseIsUsageError: a blank phrase (an unset shell
// variable) matched every description, so --description-not-contains ""
// silently emptied the result. It is now a usage error.
func TestUJCBlankDescriptionPhraseIsUsageError(t *testing.T) {
	ujcIsolate(t, "", "")
	for _, args := range [][]string{
		{"postings", "--country", "GBR", "--description-not-contains", "", "--data-source", "local", "--json"},
		{"postings", "--description-contains", "  ", "--data-source", "local", "--json"},
	} {
		r := ujcRun(t, "", args...)
		ujcWantCode(t, r, 2)
		if !strings.Contains(r.Err.Error(), "needs a non-empty phrase") {
			t.Fatalf("%v: err = %v, want the blank-phrase message", args, r.Err)
		}
	}
}

// ujcBlockingRT holds the first call until release closes, then answers
// every call with 403, counting calls.
type ujcBlockingRT struct {
	arrived, release chan struct{}
	once             sync.Once
	mu               sync.Mutex
	calls            int
}

func (rt *ujcBlockingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls++
	rt.mu.Unlock()
	rt.once.Do(func() {
		close(rt.arrived)
		<-rt.release
	})
	return &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("refused")), Request: req}, nil
}

func (rt *ujcBlockingRT) Calls() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.calls
}

// TestUJCGuardQueuedRequestSeesLatchUnderGateLock: a second process queued
// at the gate while the first one's request is refused must not send. Before
// the fix the guard checked the latch before queueing and sent anyway.
func TestUJCGuardQueuedRequestSeesLatchUnderGateLock(t *testing.T) {
	ujcResetGuardRefusal(t)
	t.Setenv(uberjobs.RefusalCooldownEnv, "1h")
	dir := t.TempDir()
	rt := &ujcBlockingRT{arrived: make(chan struct{}), release: make(chan struct{})}
	mk := func() *uberGuard {
		return &uberGuard{base: rt, gate: &uberjobs.Gate{Dir: dir, Gap: 50 * time.Millisecond}, stateDir: dir}
	}
	a, b := mk(), mk()
	const u = "https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1"
	reqA, reqB := ujcGet(t, u), ujcGet(t, u)
	type result struct {
		resp *http.Response
		err  error
	}
	ra, rb := make(chan result, 1), make(chan result, 1)
	go func() { resp, err := a.RoundTrip(reqA); ra <- result{resp, err} }()
	<-rt.arrived
	go func() { resp, err := b.RoundTrip(reqB); rb <- result{resp, err} }()
	time.Sleep(150 * time.Millisecond) // b passes its first latch check and queues at the gate
	close(rt.release)
	x := <-ra
	ujcWantRefusalResponse(t, "first process", x.resp, x.err, http.StatusForbidden, false)
	y := <-rb
	ujcWantRefusalResponse(t, "queued process", y.resp, y.err, http.StatusForbidden, false)
	if n := rt.Calls(); n != 1 {
		t.Fatalf("base saw %d calls, want 1: the queued process sent to a host that had just refused", n)
	}
}

// ujcTruncatedRT answers 200 with a body that fails partway through.
type ujcTruncatedRT struct{}

func (ujcTruncatedRT) RoundTrip(req *http.Request) (*http.Response, error) {
	body := io.MultiReader(strings.NewReader(`{"jobs":[{"Id":"1"`), iotest.ErrReader(io.ErrUnexpectedEOF))
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(body), Request: req}, nil
}

// TestUJCGuardBodyReadErrorIsAnError: a body that fails mid-read is a
// transport failure. Before the fix the guard dropped the read error and
// handed the generated client the truncated bytes as a 200 reply.
func TestUJCGuardBodyReadErrorIsAnError(t *testing.T) {
	ujcResetGuardRefusal(t)
	dir := t.TempDir()
	g := &uberGuard{base: ujcTruncatedRT{}, gate: &uberjobs.Gate{Dir: dir, Gap: time.Millisecond}, stateDir: dir}
	for _, u := range []string{"https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1", "http://127.0.0.1:9/api/jobs/search/"} {
		resp, err := g.RoundTrip(ujcGet(t, u))
		if err == nil {
			t.Fatalf("%s: a truncated body passed as a %d reply", u, resp.StatusCode)
		}
	}
}

// ujcTruncatedRefusalRT answers 403 with a body that fails partway through,
// counting calls.
type ujcTruncatedRefusalRT struct {
	mu    sync.Mutex
	calls int
}

func (rt *ujcTruncatedRefusalRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls++
	rt.mu.Unlock()
	body := io.MultiReader(strings.NewReader("Forbi"), iotest.ErrReader(io.ErrUnexpectedEOF))
	return &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}, Body: io.NopCloser(body), Request: req}, nil
}

// TestUJCGuardTruncatedRefusalIsTerminal: a 403 whose body is cut short is
// still a refusal: the terminal 429 and a latch, never a network error the
// generated client would retry against the refusing host.
func TestUJCGuardTruncatedRefusalIsTerminal(t *testing.T) {
	ujcResetGuardRefusal(t)
	t.Setenv(uberjobs.RefusalCooldownEnv, "1h")
	dir := t.TempDir()
	rt := &ujcTruncatedRefusalRT{}
	g := &uberGuard{base: rt, gate: &uberjobs.Gate{Dir: dir, Gap: time.Millisecond}, stateDir: dir}
	const u = "https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1"
	resp, err := g.RoundTrip(ujcGet(t, u))
	ujcWantRefusalResponse(t, "truncated 403", resp, err, http.StatusForbidden, false)
	resp, err = g.RoundTrip(ujcGet(t, u))
	ujcWantRefusalResponse(t, "second round trip", resp, err, http.StatusForbidden, false)
	if rt.calls != 1 {
		t.Fatalf("base saw %d calls, want 1", rt.calls)
	}
	if uberjobs.LatchedRefusal(dir, "jobs.uber.com", time.Now()) == nil {
		t.Fatalf("no latch file for a host that answered 403")
	}
}

// TestUJCNewAllWriteFailureAdvancesNothing: the baseline writes of one run
// share a transaction, so a store failure on one search advances no search,
// and every row keeps its listing. In round 3 a failed write left the other
// searches advanced while the note said none had.
func TestUJCNewAllWriteFailureAdvancesNothing(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	db := ujcDefaultDB()
	site.Drop(ujcIDGBRIntern)
	ujcSeedFullSync(t, db, site, time.Now().Add(-2*time.Hour))
	for _, name := range []string{"a-gbr", "b-gbr", "c-gbr"} {
		ujcWantCode(t, ujcRun(t, "", "save", name, "", "--country", "GBR", "--json"), 0)
	}
	ujcWantCode(t, ujcRun(t, "", "new", "--all", "--data-source", "local", "--json"), 0)

	site.Undrop(ujcIDGBRIntern)
	ujcSeedFullSync(t, db, site, time.Now().Add(-time.Hour))
	ujcWithStore(t, db, func(sdb *sql.DB) {
		if _, err := sdb.Exec(`CREATE TRIGGER ujc_fail_b BEFORE UPDATE OF last_advanced_at ON uj_saved_searches WHEN NEW.name = 'b-gbr' BEGIN SELECT RAISE(ABORT, 'injected write failure'); END`); err != nil {
			t.Fatal(err)
		}
	})
	r := ujcRun(t, "", "new", "--all", "--data-source", "local", "--json")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "injected write failure") {
		t.Fatalf("err = %v, want the injected write failure", r.Err)
	}
	env := ujcEnvelope(t, r.Stdout)
	rows := ujcRows(env)
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want three", rows)
	}
	for _, row := range rows {
		if note, _ := row["note"].(string); row["baseline_advanced"] != false || row["added_count"] != float64(1) || !strings.Contains(note, "baseline held because the store write failed") {
			t.Fatalf("row %v: want the addition listed and the baseline held", row)
		}
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "the store write failed, so no baseline advanced") {
		t.Fatalf("meta.note = %q, want the write-failure note", note)
	}

	// Nothing advanced: with the fault gone, every search lists the addition again.
	ujcWithStore(t, db, func(sdb *sql.DB) {
		if _, err := sdb.Exec(`DROP TRIGGER ujc_fail_b`); err != nil {
			t.Fatal(err)
		}
	})
	r = ujcRun(t, "", "new", "--all", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	for _, row := range ujcRows(ujcEnvelope(t, r.Stdout)) {
		if row["added_count"] != float64(1) || row["baseline_advanced"] != true {
			t.Fatalf("after the fault: row %v, want the addition listed again and advanced", row)
		}
	}
}

// TestUJCCommitPlansSurvivesExpiredReadDeadline: the writes get their own
// deadline. In round 3 they ran on the command context, so a read timeout
// failed every held search's check write and wiped its listing.
func TestUJCCommitPlansSurvivesExpiredReadDeadline(t *testing.T) {
	ujcIsolate(t, "", "")
	db := ujcOpenStore(t, ujcDefaultDB())
	now := time.Now().UTC()
	if _, _, err := uberjobs.SaveSearch(context.Background(), db, "s", uberjobs.Filters{}, now); err != nil {
		t.Fatal(err)
	}
	plan := newPlan{commit: func(ctx context.Context, tx *sql.Tx) error { return uberjobs.MarkChecked(ctx, tx, "s", now) }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the reads used up the command deadline
	if err := commitPlans(ctx, db, []newPlan{plan}); err != nil {
		t.Fatalf("commitPlans on an expired read context: %v", err)
	}
	sv, err := uberjobs.GetSearch(context.Background(), db, "s")
	if err != nil || sv == nil || sv.LastCheckedAt == nil {
		t.Fatalf("check not recorded: sv=%+v err=%v", sv, err)
	}
}

// TestUJCCareersDryRunJSONNamesTheAction: the generated careers commands'
// --dry-run JSON names the request they would send. The generated read path
// printed a bare {"dry_run": true}, which live dogfood fails as an empty
// dry-run action. Nothing is sent: the base URLs point at a closed port.
func TestUJCCareersDryRunJSONNamesTheAction(t *testing.T) {
	ujcIsolate(t, "", "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"careers", "search", "--countries", "Germany", "--page-size", "100", "--json", "--dry-run"}, "GET /api/jobs/search/"},
		{[]string{"careers", "lookup", "--job-ids", `["302906"]`, "--json", "--dry-run"}, "POST /api/jobs/recently-viewed/"},
	} {
		r := ujcRun(t, "", tc.args...)
		ujcWantCode(t, r, 0)
		var got map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(r.Stdout)), &got); err != nil {
			t.Fatalf("%v: stdout is not one JSON object: %v\n%s", tc.args, err, r.Stdout)
		}
		if got["dry_run"] != true || !strings.HasPrefix(fmt.Sprint(got["action"]), tc.want) {
			t.Fatalf("%v: dry-run JSON = %v, want dry_run true and an action starting %q", tc.args, got, tc.want)
		}
	}
}

// TestUJCWithDryRunActionPassesOtherOutputThrough: only a dry-run object
// without an action changes; other output keeps its bytes.
func TestUJCWithDryRunActionPassesOtherOutputThrough(t *testing.T) {
	for _, raw := range []string{"", "not json", `{"dry_run": false}`, `{"dry_run": true, "action": "kept"}`, `[1,2]`} {
		if got := string(withDryRunAction([]byte(raw), "GET /x")); got != raw {
			t.Errorf("withDryRunAction(%q) = %q, want it unchanged", raw, got)
		}
	}
	if got := string(withDryRunAction([]byte(`{"dry_run":true}`), "GET /x")); got != `{"action":"GET /x (dry run; no request sent)","dry_run":true}`+"\n" {
		t.Errorf("compact input: got %q", got)
	}
}

// TestUJCLocalReadsNeverCreateTheStore: get, facets, postings and stats are
// annotated read-only, so an explicit --data-source local must not create or
// migrate a missing store. Before the fix each opened it through the
// migrating path and left a new store file behind.
func TestUJCLocalReadsNeverCreateTheStore(t *testing.T) {
	ujcIsolate(t, "", "")
	db := ujcDefaultDB()
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"get", "301235", "--data-source", "local", "--json"}, 3},
		{[]string{"facets", "--data-source", "local", "--json"}, 0},
		{[]string{"postings", "--country", "GBR", "--data-source", "local", "--json"}, 0},
		{[]string{"stats", "--by", "country", "--data-source", "local", "--json"}, 0},
	} {
		r := ujcRun(t, "", tc.args...)
		ujcWantCode(t, r, tc.code)
		if tc.code == 0 {
			if note, _ := ujcMeta(ujcEnvelope(t, r.Stdout))["note"].(string); !strings.Contains(note, "sync") {
				t.Fatalf("%v: meta.note = %q, want a pointer to sync", tc.args, note)
			}
		}
		if _, err := os.Stat(db); !os.IsNotExist(err) {
			t.Fatalf("%v created the store at %s (stat err %v)", tc.args, db, err)
		}
	}
}

// TestUJCCareersRefusalExitsSevenPromptly: a refused raw careers request
// exits 7 at once, after one request, rather than waiting out the synthetic
// Retry-After inside the generated retry loop (Greptile, PR #2271).
func TestUJCCareersRefusalExitsSevenPromptly(t *testing.T) {
	ujcResetGuardRefusal(t)
	site := ujcNewFake(t)
	site.SetMode("429")
	ujcIsolate(t, site.URL, "")
	start := time.Now()
	r := ujcRun(t, "", "careers", "search", "--countries", "Germany", "--json")
	ujcWantCode(t, r, 7)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("a refusal took %v to surface, want it at once", took)
	}
	if n := site.Count(""); n != 1 {
		t.Fatalf("the site saw %d requests, want 1", n)
	}
}

// TestUJCNewLocalRefusesAfterPartialSync: a keyword or fallback sync after
// the last complete sync changes the open rows, so a local read would mix
// two snapshots; new must not diff or baseline from it (Greptile, PR #2271).
func TestUJCNewLocalRefusesAfterPartialSync(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	db := ujcDefaultDB()
	ujcSeedFullSync(t, db, site, time.Now().Add(-2*time.Hour))
	ujcWantCode(t, ujcRun(t, "", "save", "gbr", "", "--country", "GBR", "--json"), 0)
	ujcWithStore(t, db, func(sdb *sql.DB) {
		ujcInsertSyncRun(t, sdb, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour), "query", true)
	})
	r := ujcRun(t, "", "new", "gbr", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	rows := ujcRows(ujcEnvelope(t, r.Stdout))
	if note, _ := rows[0]["note"].(string); len(rows) != 1 || rows[0]["baseline_established"] != false || !strings.Contains(note, "after the last complete sync") {
		t.Fatalf("new after a partial sync = %v, want no baseline and the partial-sync reason", rows)
	}
}

// TestUJCNewCommitHoldsWhenSearchChanged: a save that replaces the filters
// between new's read and its commit must not be overwritten by members
// computed with the old filters (Greptile, PR #2271).
func TestUJCNewCommitHoldsWhenSearchChanged(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	path := ujcDefaultDB()
	ujcSeedFullSync(t, path, site, time.Now().Add(-time.Hour))
	db := ujcOpenStore(t, path)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, _, err := uberjobs.SaveSearch(ctx, db, "all-of-it", uberjobs.Filters{}, now); err != nil {
		t.Fatal(err)
	}
	sv, err := uberjobs.GetSearch(ctx, db, "all-of-it")
	if err != nil || sv == nil {
		t.Fatalf("GetSearch: %v %v", sv, err)
	}
	var c *uberjobs.Client
	plan, err := planNewSearch(ctx, ujcFlags(), &c, db, "local", *sv, 50, now)
	if err != nil || !plan.advances {
		t.Fatalf("plan = %+v, %v; want an advancing first baseline", plan.row, err)
	}
	// A concurrent save replaces the filters before new commits.
	if _, _, err := uberjobs.SaveSearch(ctx, db, "all-of-it", uberjobs.Filters{Team: "Legal"}, now); err != nil {
		t.Fatal(err)
	}
	err = commitPlans(ctx, db, []newPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "changed while new was reading") {
		t.Fatalf("commit after a concurrent save: err = %v, want the changed-search error", err)
	}
	members, err := uberjobs.Members(ctx, db, "all-of-it")
	if err != nil || len(members) != 0 {
		t.Fatalf("members = %d (%v), want none: the old filters' members were written", len(members), err)
	}
}

// TestUJCNewCommitHoldsWhenAnotherRunAdvanced: when two new runs read the
// same saved search and the other one advances it first, this run's older
// read must not overwrite the newer baseline (Greptile, PR #2271, round 2).
func TestUJCNewCommitHoldsWhenAnotherRunAdvanced(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	path := ujcDefaultDB()
	ujcSeedFullSync(t, path, site, time.Now().Add(-time.Hour))
	db := ujcOpenStore(t, path)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, _, err := uberjobs.SaveSearch(ctx, db, "all-of-it", uberjobs.Filters{}, now); err != nil {
		t.Fatal(err)
	}
	var c *uberjobs.Client
	sv, _ := uberjobs.GetSearch(ctx, db, "all-of-it")
	if _, _, err := runNewSearch(ctx, ujcFlags(), &c, db, "local", *sv, 50, now); err != nil {
		t.Fatalf("first run (baseline): %v", err)
	}
	sv, _ = uberjobs.GetSearch(ctx, db, "all-of-it")
	plan, err := planNewSearch(ctx, ujcFlags(), &c, db, "local", *sv, 50, now)
	if err != nil || !plan.advances {
		t.Fatalf("plan = %+v, %v; want an advancing plan", plan.row, err)
	}
	// Another run advances the same search (a newer read) before this one commits.
	newer := site.Postings(t)[:3]
	if _, err := uberjobs.AdvanceMembership(ctx, db, "all-of-it", newer, false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	err = commitPlans(ctx, db, []newPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "changed while new was reading") {
		t.Fatalf("commit after another run advanced: err = %v, want the changed-search error", err)
	}
	members, err := uberjobs.Members(ctx, db, "all-of-it")
	present := 0
	for _, m := range members {
		if m.RemovedOn == nil {
			present++
		}
	}
	if err != nil || present != len(newer) {
		t.Fatalf("present members = %d (%v), want the newer run's %d", present, err, len(newer))
	}
}
