// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared test plumbing for the hand-written uber-jobs commands: one loopback
// fake that serves the careers site and the Oracle fallback from the scrubbed
// corpus, plus helpers to run RootCmd, decode the envelope, and seed the store.
// Nothing here dials a real host; zz_netguard_test.go enforces that.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

const (
	ujcTestdata = "../uberjobs/testdata"
	// ujcAnchor is when the corpus was captured. The fake shifts every true
	// posting date by (now - anchor) so recency windows stay meaningful on
	// any later run; floor dates stay exactly uberjobs.DateFloor.
	ujcAnchor = "2026-10-05T12:00:00Z"
	// ujcOracleDetailID is the id of the real Oracle detail fixture.
	ujcOracleDetailID = "302906"
)

// ujcCorpusRow ids with known shapes, all from search_corpus.json.
const (
	ujcIDNewestNLD  = "303232" // NLD, empty Teams and WorkPattern
	ujcIDFirstRow   = "302016" // first corpus row; UAE + Saudi Arabia
	ujcIDFloorUSA   = "155579" // floor date, USA
	ujcIDOldNLD     = "151342" // NLD, about 82 days before the anchor
	ujcIDMidNLD     = "159438" // NLD, about 21 days before the anchor
	ujcIDGBRIntern  = "303139" // GBR, Intern, newest GBR row
	ujcIDGBRAccount = "300126" // GBR, oldest GBR row
)

var (
	ujcCorpusOnce sync.Once
	ujcCorpusRows []map[string]any
	ujcFacetHTML  []byte
	ujcDetailRaw  []byte
	ujcCorpusErr  error
)

// ujcLoadCorpus reads the fixtures once and shifts the true posting dates.
func ujcLoadCorpus(t *testing.T) []map[string]any {
	t.Helper()
	ujcCorpusOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(ujcTestdata, "search_corpus.json"))
		if err != nil {
			ujcCorpusErr = err
			return
		}
		var env struct {
			Jobs []map[string]any `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			ujcCorpusErr = err
			return
		}
		anchor, _ := time.Parse(time.RFC3339, ujcAnchor)
		shift := time.Now().UTC().Truncate(time.Second).Sub(anchor)
		for _, row := range env.Jobs {
			d := ujcStr(row["DisplayDate"])
			if d == "" || d == uberjobs.DateFloor {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, d); err == nil {
				row["DisplayDate"] = ts.Add(shift).UTC().Format("2006-01-02T15:04:05Z")
			}
		}
		ujcCorpusRows = env.Jobs
		if ujcFacetHTML, err = os.ReadFile(filepath.Join(ujcTestdata, "facets.html")); err != nil {
			ujcCorpusErr = err
			return
		}
		ujcDetailRaw, ujcCorpusErr = os.ReadFile(filepath.Join(ujcTestdata, "oracle_detail.json"))
	})
	if ujcCorpusErr != nil {
		t.Fatalf("loading fixtures: %v", ujcCorpusErr)
	}
	return ujcCorpusRows
}

func ujcStr(v any) string {
	s, _ := v.(string)
	return s
}

// ujcFake is one loopback host. It serves the careers search, facets and
// lookup paths and the two Oracle paths, so a test that needs the Oracle
// fallback starts a second instance: the refusal latch is per host:port.
type ujcFake struct {
	*httptest.Server

	mu         sync.Mutex
	rows       []map[string]any
	drop       map[string]bool
	only       map[string]bool
	totalBump  int
	siteMode   string
	oracleMode string
	counts     map[string]int
	requests   []string
}

// ujcNewFake starts a fake serving every corpus row in "ok" mode.
func ujcNewFake(t *testing.T) *ujcFake {
	t.Helper()
	f := &ujcFake{
		rows:       ujcLoadCorpus(t),
		drop:       map[string]bool{},
		siteMode:   "ok",
		oracleMode: "ok",
		counts:     map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/jobs/search/", f.search)
	mux.HandleFunc("/en/jobs/", f.facets)
	mux.HandleFunc("/api/jobs/recently-viewed/", f.lookup)
	mux.HandleFunc("/hcmRestApi/resources/11.13.18.05/recruitingCEJobRequisitions", f.oracleList)
	mux.HandleFunc("/hcmRestApi/resources/11.13.18.05/recruitingCEJobRequisitionDetails", f.oracleDetail)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.count(r)
		w.WriteHeader(http.StatusNotFound)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

// SetMode switches the careers paths: ok, challenge, 429, 500, or html.
func (f *ujcFake) SetMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.siteMode = mode
}

// SetOracleMode switches the Oracle paths the same way.
func (f *ujcFake) SetOracleMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oracleMode = mode
}

// Drop hides ids from every path, as if the postings closed.
func (f *ujcFake) Drop(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		f.drop[id] = true
	}
}

// Undrop lists ids again, as if the postings reopened.
func (f *ujcFake) Undrop(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.drop, id)
	}
}

// Only restricts the served corpus to ids (nil serves everything).
func (f *ujcFake) Only(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(ids) == 0 {
		f.only = nil
		return
	}
	f.only = map[string]bool{}
	for _, id := range ids {
		f.only[id] = true
	}
}

// SetTotalBump inflates totalJobs so the read looks incomplete.
func (f *ujcFake) SetTotalBump(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.totalBump = n
}

// Count reports how many requests reached path; "" counts every path.
func (f *ujcFake) Count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path == "" {
		return len(f.requests)
	}
	return f.counts[path]
}

// Requests returns every request URL the fake saw, in order.
func (f *ujcFake) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *ujcFake) count(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[r.URL.Path]++
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
}

func (f *ujcFake) state() (rows []map[string]any, bump int, site, oracle string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		id := ujcStr(row["Id"])
		if f.drop[id] || (f.only != nil && !f.only[id]) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, f.totalBump, f.siteMode, f.oracleMode
}

// Postings returns the currently listed rows normalized the way readLive
// normalizes them, for seeding the store.
func (f *ujcFake) Postings(t *testing.T) []uberjobs.Posting {
	t.Helper()
	rows, _, _, _ := f.state()
	return ujcNormalize(t, rows, f.URL)
}

func ujcNormalize(t *testing.T, rows []map[string]any, base string) []uberjobs.Posting {
	t.Helper()
	out := make([]uberjobs.Posting, 0, len(rows))
	for _, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		var raw uberjobs.RawPosting
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, uberjobs.Normalize(raw, base))
	}
	return out
}

// ujcRefuse writes the failure for a non-ok mode and reports whether it did.
func ujcRefuse(w http.ResponseWriter, mode string) bool {
	switch mode {
	case "challenge":
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><script>window._cf_chl_opt={}</script></body></html>")
	case "429":
		w.WriteHeader(http.StatusTooManyRequests)
	case "500":
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"Failed to search jobs","jobs":[],"totalJobs":0}`)
	case "html":
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><head><title>Uber Careers</title></head><body>home</body></html>")
	default:
		return false
	}
	return true
}

func (f *ujcFake) search(w http.ResponseWriter, r *http.Request) {
	f.count(r)
	rows, bump, mode, _ := f.state()
	if ujcRefuse(w, mode) {
		return
	}
	q := r.URL.Query()
	kw := strings.ToLower(q.Get("search"))
	countries := q["countries"]
	team, sub, contract := q.Get("team"), q.Get("subTeam"), q.Get("contractTypes")
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if kw != "" {
			hay := strings.ToLower(ujcStr(row["Title"]) + " " + ujcStr(row["Description"]) + " " + ujcStr(row["AdditionalText"]))
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		if len(countries) > 0 && !ujcRowInCountries(row, countries) {
			continue
		}
		if team != "" {
			teams, _ := row["Teams"].([]any)
			if len(teams) == 0 || ujcStr(teams[0]) != team {
				continue
			}
		}
		if sub != "" && !strings.HasSuffix(ujcStr(row["AdditionalText"]), " "+sub) {
			continue
		}
		if contract != "" && ujcStr(row["ContractType"]) != contract {
			continue
		}
		out = append(out, row)
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("pagesize"))
	if size < 1 {
		size = 10
	}
	total := len(out) + bump
	slice := []map[string]any{}
	if start := (page - 1) * size; start < len(out) {
		end := start + size
		if end > len(out) {
			end = len(out)
		}
		slice = out[start:end]
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Vercel-Cache", "MISS")
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": slice, "totalPages": (total + size - 1) / size, "totalJobs": total, "page": page, "pageSize": size})
}

func ujcRowInCountries(row map[string]any, countries []string) bool {
	locs, _ := row["Locations"].([]any)
	for _, l := range locs {
		lm, _ := l.(map[string]any)
		for _, c := range countries {
			if ujcStr(lm["Country"]) == c {
				return true
			}
		}
	}
	return false
}

func (f *ujcFake) facets(w http.ResponseWriter, r *http.Request) {
	f.count(r)
	_, _, mode, _ := f.state()
	if ujcRefuse(w, mode) {
		return
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write(ujcFacetHTML)
}

func (f *ujcFake) lookup(w http.ResponseWriter, r *http.Request) {
	f.count(r)
	rows, _, mode, _ := f.state()
	if ujcRefuse(w, mode) {
		return
	}
	var body struct {
		JobIDs []string `json:"jobIds"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	want := map[string]bool{}
	for _, id := range body.JobIDs {
		want[id] = true
	}
	jobs := []map[string]any{}
	for _, row := range rows {
		if want[ujcStr(row["Id"])] {
			jobs = append(jobs, map[string]any{"id": row["Id"], "title": row["Title"], "url": "/en/jobs/" + ujcStr(row["Id"]) + "/"})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
}

// ujcISO2 maps the corpus's site country names to the ISO2 codes Oracle sends.
var ujcISO2 = map[string]string{
	"United States": "US", "United Kingdom": "GB", "Germany": "DE", "Netherlands": "NL",
	"India": "IN", "Saudi Arabia": "SA", "United Arab Emirates": "AE", "Malaysia": "MY",
	"Chile": "CL", "Canada": "CA", "Philippines": "PH", "Taiwan": "TW", "Mexico": "MX", "Brazil": "BR",
}

var (
	ujcReLimit   = regexp.MustCompile(`limit=(\d+)`)
	ujcReOffset  = regexp.MustCompile(`offset=(\d+)`)
	ujcReKeyword = regexp.MustCompile(`keyword="((?:[^"\\]|\\.)*)"`)
	ujcReByID    = regexp.MustCompile(`Id="([^"]+)"`)
)

// ujcRawFinder reads finder from the raw query: Oracle keeps ';' literal, and
// net/url's Query() drops any pair containing one.
func ujcRawFinder(r *http.Request) string {
	for _, part := range strings.Split(r.URL.RawQuery, "&") {
		if strings.HasPrefix(part, "finder=") {
			if v, err := url.PathUnescape(strings.TrimPrefix(part, "finder=")); err == nil {
				return v
			}
		}
	}
	return ""
}

func ujcLocation(row map[string]any, i int) (iso2, name string) {
	locs, _ := row["Locations"].([]any)
	if i >= len(locs) {
		return "", ""
	}
	lm, _ := locs[i].(map[string]any)
	country := ujcStr(lm["Country"])
	return ujcISO2[country], strings.Trim(ujcStr(lm["City"])+", "+country, ", ")
}

func (f *ujcFake) oracleList(w http.ResponseWriter, r *http.Request) {
	f.count(r)
	rows, _, _, mode := f.state()
	if ujcRefuse(w, mode) {
		return
	}
	finder := ujcRawFinder(r)
	limit, offset := 25, 0
	if m := ujcReLimit.FindStringSubmatch(finder); m != nil {
		limit, _ = strconv.Atoi(m[1])
	}
	if m := ujcReOffset.FindStringSubmatch(finder); m != nil {
		offset, _ = strconv.Atoi(m[1])
	}
	kw := ""
	if m := ujcReKeyword.FindStringSubmatch(finder); m != nil {
		kw = strings.ToLower(m[1])
	}
	var match []map[string]any
	for _, row := range rows {
		if kw != "" && !strings.Contains(strings.ToLower(ujcStr(row["Title"])), kw) {
			continue
		}
		match = append(match, row)
	}
	sort.SliceStable(match, func(i, j int) bool { return ujcStr(match[i]["DisplayDate"]) > ujcStr(match[j]["DisplayDate"]) })
	list := []map[string]any{}
	for i := offset; i < len(match) && i < offset+limit; i++ {
		row := match[i]
		cc, name := ujcLocation(row, 0)
		secondary := []map[string]any{}
		locs, _ := row["Locations"].([]any)
		for j := 1; j < len(locs); j++ {
			c2, n2 := ujcLocation(row, j)
			secondary = append(secondary, map[string]any{"Name": n2, "CountryCode": c2})
		}
		list = append(list, map[string]any{
			"Id": row["Id"], "Title": row["Title"], "PostedDate": strings.SplitN(ujcStr(row["DisplayDate"]), "T", 2)[0],
			"PrimaryLocation": name, "PrimaryLocationCountry": cc, "secondaryLocations": secondary,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"TotalJobsCount": len(match), "requisitionList": list}}})
}

func (f *ujcFake) oracleDetail(w http.ResponseWriter, r *http.Request) {
	f.count(r)
	rows, _, _, mode := f.state()
	if ujcRefuse(w, mode) {
		return
	}
	id := ""
	if m := ujcReByID.FindStringSubmatch(ujcRawFinder(r)); m != nil {
		id = m[1]
	}
	w.Header().Set("Content-Type", "application/json")
	for _, row := range rows {
		if ujcStr(row["Id"]) != id {
			continue
		}
		if id == ujcOracleDetailID {
			_, _ = w.Write(ujcDetailRaw)
			return
		}
		cc, name := ujcLocation(row, 0)
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
			"Id": row["Id"], "Title": row["Title"], "Category": "Oracle Category",
			"ExternalDescriptionStr": row["Description"], "ExternalPostedStartDate": strings.Replace(ujcStr(row["DisplayDate"]), "Z", "+00:00", 1),
			"PrimaryLocation": name, "PrimaryLocationCountry": cc, "JobSchedule": "Full time",
		}}})
		return
	}
	_, _ = io.WriteString(w, `{"items":[]}`)
}

// ujcClosedURL returns a loopback URL whose port has nothing listening.
func ujcClosedURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	u := srv.URL
	srv.Close()
	return u
}

// ujcClient builds a sibling client for the fake with no gate, no limiter,
// no cache, and no request log, so package-level tests run instantly.
func ujcClient(site *ujcFake, oracle *ujcFake) *uberjobs.Client {
	c := uberjobs.NewClient(site.URL, 5*time.Second, "", "")
	c.Limiter = nil
	c.RequestLog = ""
	c.OracleBase = ""
	if oracle != nil {
		c.OracleBase = oracle.URL
	}
	return c
}

// ujcIsolate sandboxes the user dirs and points both base URLs at loopback.
// oracleURL "" means the Oracle fallback hits a closed loopback port. The
// request log is cleared so no test line reaches the owner's real ledger.
func ujcIsolate(t *testing.T, siteURL, oracleURL string) string {
	t.Helper()
	home := testenv.Isolate(t, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	if siteURL == "" {
		siteURL = ujcClosedURL(t)
	}
	if oracleURL == "" {
		oracleURL = ujcClosedURL(t)
	}
	t.Setenv("UBER_JOBS_BASE_URL", siteURL)
	t.Setenv("UBER_JOBS_ORACLE_BASE_URL", oracleURL)
	t.Setenv("UBER_JOBS_REQUEST_LOG", "")
	t.Setenv("UBER_JOBS_MIN_GAP", "")
	t.Setenv(uberjobs.RefusalCooldownEnv, "")
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	return home
}

// ujcFlags is a rootFlags for package-level calls that build their own client.
func ujcFlags() *rootFlags {
	return &rootFlags{timeout: 20 * time.Second, noCache: true}
}

// ujcRunResult is one RootCmd execution.
type ujcRunResult struct {
	Stdout string
	Stderr string
	Err    error
}

// ujcRun executes RootCmd with args and stdin, capturing stdout separately
// so it decodes as JSON.
func ujcRun(t *testing.T, stdin string, args ...string) ujcRunResult {
	t.Helper()
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return ujcRunResult{Stdout: out.String(), Stderr: errb.String(), Err: err}
}

// ujcCode returns the typed exit code of err: 0 for nil, -1 for an untyped error.
func ujcCode(err error) int {
	if err == nil {
		return 0
	}
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	return -1
}

// ujcWantCode fails unless the run ended with exactly code.
func ujcWantCode(t *testing.T, r ujcRunResult, code int) {
	t.Helper()
	if got := ujcCode(r.Err); got != code {
		t.Fatalf("exit code = %d, want %d (err = %v)\nstdout:\n%s\nstderr:\n%s", got, code, r.Err, ujcTrim(r.Stdout), ujcTrim(r.Stderr))
	}
}

func ujcTrim(s string) string {
	if len(s) > 3000 {
		return s[:3000] + "..."
	}
	return s
}

// ujcEnvelopeKeys are the only top-level keys every command may print.
var ujcEnvelopeKeys = []string{"hits", "meta", "results", "returned", "scan_cap_hit", "scanned"}

// ujcEnvelope decodes stdout as exactly one envelope and checks its shape:
// the six keys, results a JSON array, a known meta.source, and an id on
// every row.
func ujcEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var env map[string]any
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, ujcTrim(stdout))
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout holds more than one JSON value:\n%s", ujcTrim(stdout))
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != strings.Join(ujcEnvelopeKeys, ",") {
		t.Fatalf("envelope keys = %v, want exactly %v", keys, ujcEnvelopeKeys)
	}
	if _, ok := env["results"].([]any); !ok {
		t.Fatalf("results is %T (%v), want a JSON array", env["results"], env["results"])
	}
	meta, ok := env["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is %T, want an object", env["meta"])
	}
	switch src := meta["source"]; src {
	case uberjobs.SourceSite, uberjobs.SourceOracle, uberjobs.SourceLocal, "mixed":
	default:
		t.Fatalf("meta.source = %v, want jobs.uber.com, oracle-ce, local, or mixed", src)
	}
	for i, row := range ujcRows(env) {
		if _, ok := row["id"]; !ok {
			t.Fatalf("results[%d] has no id key: %v", i, row)
		}
	}
	return env
}

// ujcRows returns results as objects.
func ujcRows(env map[string]any) []map[string]any {
	list, _ := env["results"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		m, _ := v.(map[string]any)
		out = append(out, m)
	}
	return out
}

func ujcMeta(env map[string]any) map[string]any {
	m, _ := env["meta"].(map[string]any)
	return m
}

func ujcInt(t *testing.T, v any) int {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("value %v (%T) is not a JSON number", v, v)
	}
	return int(f)
}

func ujcRowIDs(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["id"]))
	}
	return out
}

func ujcContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ujcOpenStore opens (and migrates) a store at path and keeps it open for
// the test. Only package-level calls that take this *sql.DB may read it:
// commands open the store read-only with immutable=1, which by design cannot
// see frames a still-open writer has not checkpointed. Use ujcWithStore
// before running a command.
func ujcOpenStore(t *testing.T, path string) *sql.DB {
	t.Helper()
	s, db, err := openUberStore(context.Background(), path)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return db
}

// ujcWithStore opens the store at path, runs fn, and closes it so the WAL
// is checkpointed before any command reads the file.
func ujcWithStore(t *testing.T, path string, fn func(db *sql.DB)) {
	t.Helper()
	s, db, err := openUberStore(context.Background(), path)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	fn(db)
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}
}

// ujcApply records one read in the store the way sync does, at time at.
func ujcApply(t *testing.T, db *sql.DB, postings []uberjobs.Posting, scope string, complete bool, at time.Time) uberjobs.SyncRun {
	t.Helper()
	run := uberjobs.SyncRun{
		StartedAt: at.UTC().Format(time.RFC3339), Source: uberjobs.SourceSite, Scope: scope,
		Total: len(postings), UniqueCount: len(postings), Complete: complete,
	}
	got, _, err := uberjobs.ApplyRead(context.Background(), db, postings, run, at, uberjobs.ApplyOptions{})
	if err != nil {
		t.Fatalf("ApplyRead: %v", err)
	}
	return got
}

// ujcSeedFullSync writes a complete whole-corpus sync of the fake's current
// rows into the store at path, finished at at.
func ujcSeedFullSync(t *testing.T, path string, f *ujcFake, at time.Time) {
	t.Helper()
	ujcWithStore(t, path, func(db *sql.DB) {
		ujcApply(t, db, f.Postings(t), "all", true, at)
	})
}

// ujcInsertSyncRun inserts a sync_runs row directly, for histories whose
// first sync is older than any real run could be.
func ujcInsertSyncRun(t *testing.T, db *sql.DB, started, finished time.Time, scope string, complete bool) {
	t.Helper()
	c := 0
	if complete {
		c = 1
	}
	if _, err := db.Exec(`INSERT INTO uj_sync_runs (started_at, finished_at, source, scope, total, unique_count, complete, scan_cap_hit, closed_marked, note) VALUES (?, ?, ?, ?, 0, 0, ?, 0, 0, '')`,
		started.UTC().Format(time.RFC3339), finished.UTC().Format(time.RFC3339), uberjobs.SourceSite, scope, c); err != nil {
		t.Fatalf("inserting sync run: %v", err)
	}
}

// ujcDefaultDB is the store path commands use when --db is not given.
func ujcDefaultDB() string { return uberDBPath("") }
