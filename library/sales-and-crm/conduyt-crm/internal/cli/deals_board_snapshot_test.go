package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
)

// Board snapshot (board #116 for app #111, contract frozen at app 5729bc7b4): `deals list --board-snapshot --pipeline <id>
// --board-columns <stageId>[:<limit>],...`. Hand-written path (deals_board_snapshot.go): refusals before any request, a live
// unpaged uncached read, a shape check and the deal count across columns. A reprint must keep it.
const (
	bsPipe = "9a0c6b8e-3f1d-4c2a-8e7b-5d4c3b2a1f00"
	bsS1   = "44444444-4444-4444-8444-444444444441"
	bsS2   = "44444444-4444-4444-8444-444444444442"
	bsSnap = `{"data":{"columns":[{"stageId":"` + bsS1 + `","limit":25,"deals":[{"id":"d1"},{"id":"d2"}]},{"stageId":"` + bsS2 + `","limit":50,"deals":[{"id":"d3"}]}],"meta":{"board_snapshot":true}}}`
)

type bsRun struct {
	queries []url.Values
	out     string
	errOut  string
	err     error
}

// runDealsList runs `deals list` against a local server answering status/body; noCache false keeps the response cache on
// (under a temp CONDUYT_CRM_HOME) so a test can prove the snapshot never reads it. tty puts stdout on /dev/null (a
// character device) so the interactive path runs.
func runDealsList(t *testing.T, status int, body string, noCache, tty bool, args ...string) bsRun {
	t.Helper()
	var r bsRun
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.queries = append(r.queries, req.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		if status == http.StatusServiceUnavailable {
			w.Header().Set("Retry-After", "5")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	if os.Getenv("CONDUYT_CRM_HOME") == "" {
		t.Setenv("CONDUYT_CRM_HOME", t.TempDir())
	}
	var out, errBuf bytes.Buffer
	cmd := newDealsListCmd(&rootFlags{noCache: noCache})
	if tty {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		cmd.SetOut(f)
	} else {
		cmd.SetOut(&out)
	}
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	r.err = cmd.Execute()
	r.out, r.errOut = out.String(), errBuf.String()
	return r
}

func bsArgs(extra ...string) []string {
	return append([]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":25," + bsS2}, extra...)
}

func TestDealsBoardSnapshotSendsTheModeAndNotThePaginatorParams(t *testing.T) {
	r := runDealsList(t, 200, bsSnap, true, false, bsArgs("--status", "open")...)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(r.queries) != 1 {
		t.Fatalf("requests = %d, want 1 (never paged)", len(r.queries))
	}
	q := r.queries[0]
	want := map[string]string{"board_snapshot": "true", "board_columns": bsS1 + ":25," + bsS2, "pipeline": bsPipe, "per_page": "50", "status": "open", "sort": "createdAt", "order": "desc"}
	if len(q) != len(want) {
		t.Errorf("query = %v, want exactly %v", q, want)
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("page") || q.Has("stage") {
		t.Errorf("page/stage sent: %v", q)
	}
}

func TestDealsBoardSnapshotPerPageIsTheDefaultColumnLimit(t *testing.T) {
	r := runDealsList(t, 200, bsSnap, true, false, "--board-snapshot", "--pipeline", bsPipe, "--per-page", "100", "--board-columns", " "+bsS1+" , "+bsS2+":10 ")
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.queries[0].Get("per_page") != "100" || r.queries[0].Get("board_columns") != bsS1+","+bsS2+":10" {
		t.Errorf("query = %v", r.queries[0])
	}
}

func TestDealsBoardSnapshotPrintsTheAnswer(t *testing.T) {
	r := runDealsList(t, 200, bsSnap, true, false, bsArgs()...) // stdout is a buffer (piped): the JSON path
	if r.err != nil {
		t.Fatal(r.err)
	}
	for _, want := range []string{`"columns"`, `"board_snapshot": true`, `"d3"`, `"source": "live"`} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output is missing %s:\n%s", want, r.out)
		}
	}
}

func TestDealsBoardSnapshotRefusalsSendNothing(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"page 1":            {bsArgs("--page", "1"), "cannot be combined with --page"},
		"page 2":            {bsArgs("--page", "2"), "cannot be combined with --page"},
		"stage":             {bsArgs("--stage", bsS2), "cannot be combined with --stage"},
		"all":               {bsArgs("--all"), "cannot be combined with --all"},
		"no pipeline":       {[]string{"--board-snapshot", "--board-columns", bsS1}, "needs --pipeline"},
		"blank pipeline":    {[]string{"--board-snapshot", "--pipeline", " ", "--board-columns", bsS1}, "needs --pipeline"},
		"no columns":        {[]string{"--board-snapshot", "--pipeline", bsPipe}, "needs --board-columns"},
		"columns, no mode":  {[]string{"--pipeline", bsPipe, "--board-columns", bsS1}, "--board-columns needs --board-snapshot"},
		"blank columns":     {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", ""}, "no blank entry"},
		"bad uuid":          {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", "nope"}, "stage UUID"},
		"limit 0":           {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":0"}, "1-1000"},
		"limit 1001":        {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":1001"}, "1-1000"},
		"limit +5":          {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":+5"}, "1-1000"},
		"limit 1e1":         {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":1e1"}, "1-1000"},
		"stage twice":       {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + "," + strings.ToUpper(bsS1)}, "more than once"},
		"per-page 1001":     {[]string{"--board-snapshot", "--pipeline", bsPipe, "--per-page", "1001", "--board-columns", bsS1}, "per-page"},
		"over 5000 rows":    {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", bsS1 + ":1000," + bsS2 + ":1000," + fiftyStages(4, ":1000")}, "6000 rows"},
		"51 columns":        {[]string{"--board-snapshot", "--pipeline", bsPipe, "--board-columns", fiftyStages(51, "")}, "at most 50 columns"},
		"default over 5000": {[]string{"--board-snapshot", "--pipeline", bsPipe, "--per-page", "101", "--board-columns", fiftyStages(50, "")}, "5050 rows"},
		"rows-only flag":    {bsArgs("--rows-only", "true"), "cannot be combined with --rows-only"},
		"summary flag":      {bsArgs("--summary", "true"), "cannot be combined with --summary"},
		"stage-id alias":    {bsArgs("--stage-id", bsS2), "cannot be combined with --stage-id"},
		"health flag":       {bsArgs("--health", "stale"), "cannot be combined with --health"},
		"search filter":     {bsArgs("--search", "acme"), "cannot be combined with --search"},
	}
	for name, tc := range cases {
		r := runDealsList(t, 200, bsSnap, true, false, tc.args...)
		if r.err == nil || !strings.Contains(r.err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, r.err, tc.want)
		}
		if len(r.queries) != 0 {
			t.Errorf("%s: %d requests, want none", name, len(r.queries))
		}
	}
	// the edges pass
	for _, cols := range []string{fiftyStages(50, ""), bsS1 + ":1000," + bsS2 + ":1", bsS1 + ":1000," + bsS2 + ":1000," + fiftyStages(3, ":1000")} {
		if r := runDealsList(t, 200, bsSnap, true, false, "--board-snapshot", "--pipeline", bsPipe, "--board-columns", cols); r.err != nil || len(r.queries) != 1 {
			t.Errorf("edge %d chars: err=%v requests=%d", len(cols), r.err, len(r.queries))
		}
	}
}

func fiftyStages(n int, suffix string) string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("44444444-4444-4444-8444-%012x%s", i, suffix)
	}
	return strings.Join(ids, ",")
}

func TestDealsBoardSnapshotRefusesLocalDataSource(t *testing.T) {
	srvHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { srvHits++ }))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_HOME", t.TempDir())
	cmd := newDealsListCmd(&rootFlags{noCache: true, dataSource: "local"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(bsArgs())
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--data-source local") || srvHits != 0 {
		t.Errorf("err = %v, requests = %d", err, srvHits)
	}
}

func TestDealsBoardSnapshotRefusesAnotherShape(t *testing.T) {
	for name, body := range map[string]string{
		"a list page":     `{"data":[{"id":"d1"}],"meta":{"page":1}}`,
		"no meta flag":    `{"data":{"columns":[],"meta":{}}}`,
		"columns not arr": `{"data":{"columns":{},"meta":{"board_snapshot":true}}}`,
	} {
		r := runDealsList(t, 200, body, true, false, bsArgs()...)
		if r.err == nil || !strings.Contains(r.err.Error(), "board_snapshot: true") {
			t.Errorf("%s: err = %v", name, r.err)
		}
	}
}

func TestDealsBoardSnapshotNeverReadsTheResponseCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bsSnap))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_HOME", t.TempDir())
	run := func(args ...string) {
		cmd := newDealsListCmd(&rootFlags{}) // response cache ON (no --no-cache)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	// control: the plain list IS served from the cache the second time, so the cache is live in this test
	run("--pipeline", bsPipe)
	run("--pipeline", bsPipe)
	if hits != 1 {
		t.Fatalf("control: plain list reached the API %d times, want 1 (cache inactive?)", hits)
	}
	hits = 0
	run(bsArgs()...)
	run(bsArgs()...)
	if hits != 2 {
		t.Fatalf("the snapshot reached the API %d times in two runs, want 2 (never read from the cache)", hits)
	}
}

func TestDealsBoardSnapshotProvenanceCountsDealsAcrossColumns(t *testing.T) {
	r := runDealsList(t, 200, bsSnap, true, true, bsArgs()...)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if !strings.Contains(r.errOut, "3 results (live)") {
		t.Errorf("stderr = %q, want 3 results (live)", r.errOut)
	}
}

func TestDealsList503CodesReachTheCaller(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1") // no client retries, so the test does not sleep through the backoff
	for code, args := range map[string][]string{"board_snapshot_busy": bsArgs(), "gate_busy": {"--pipeline", bsPipe}} {
		r := runDealsList(t, 503, `{"error":"Busy; nothing was changed.","code":"`+code+`","retryAfter":5}`, true, false, args...)
		if r.err == nil || !strings.Contains(r.err.Error(), "503") || !strings.Contains(r.err.Error(), code) || !strings.Contains(r.err.Error(), `"retryAfter":5`) {
			t.Errorf("%s: err = %v", code, r.err)
		}
	}
}

func TestDealsListHelpDocumentsTheSnapshot(t *testing.T) {
	cmd := newDealsListCmd(&rootFlags{})
	for _, want := range []string{"ONE database snapshot", "exactly one column", "board_snapshot_busy", "gate_busy", "--page (even 1", "--stage", "--all", "never the response cache"} {
		if !strings.Contains(cmd.Long, want) {
			t.Errorf("Long is missing %q", want)
		}
	}
	for flag, want := range map[string]string{"board-columns": "<stageId>[:<limit>]", "board-snapshot": "ONE database snapshot"} {
		if f := cmd.Flags().Lookup(flag); f == nil || !strings.Contains(f.Usage, want) {
			t.Errorf("--%s usage = %+v", flag, f)
		}
	}
	raw, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name": "board_columns"`, `"name": "board_snapshot"`, "this executor refuses, before any request, page (even 1)"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("tools-manifest.json is missing %s", want)
		}
	}
}

// Codex r1 P2 (round 2): GET /deals declares per_page up to 1000 (the snapshot's column default accepts 201-1000) and the
// 200 response carries the DealBoardSnapshot branch, in spec.yaml and upstream/openapi.yaml; help and manifest agree.
func TestDealsListSpecDeclaresPerPage1000AndTheSnapshotBranch(t *testing.T) {
	for _, f := range []string{"../../spec.yaml", "../../upstream/openapi.yaml"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		i := strings.Index(s, "\n  /deals:\n    get:")
		if i < 0 {
			t.Fatalf("%s: no GET /deals", f)
		}
		blk := s[i+1:]
		if j := strings.Index(blk[3:], "\n  /"); j > 0 {
			blk = blk[:j+3]
		}
		k := strings.Index(blk, "- name: per_page")
		if k < 0 || !strings.Contains(blk[k:k+400], "maximum: 1000") || strings.Contains(blk[k:k+400], "maximum: 200") {
			t.Errorf("%s: GET /deals per_page is not declared with maximum 1000", f)
		}
		if !strings.Contains(blk, "oneOf:") || !strings.Contains(blk, `$ref: "#/components/schemas/DealBoardSnapshot"`) {
			t.Errorf("%s: GET /deals 200 does not carry the DealBoardSnapshot branch", f)
		}
	}
	if f := newDealsListCmd(&rootFlags{}).Flags().Lookup("per-page"); f == nil || !strings.Contains(f.Usage, "max 1000") || strings.Contains(f.Usage, "max 200") {
		t.Errorf("--per-page usage = %+v", f)
	}
	raw, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Items per page (max 1000); with board_snapshot=true") {
		t.Errorf("tools-manifest.json deals_list per_page is not max 1000")
	}
	// a snapshot column default of 201..1000 is accepted locally
	for _, pp := range []string{"201", "1000"} {
		if r := runDealsList(t, 200, bsSnap, true, false, "--board-snapshot", "--pipeline", bsPipe, "--per-page", pp, "--board-columns", bsS1); r.err != nil || len(r.queries) != 1 || r.queries[0].Get("per_page") != pp {
			t.Errorf("per-page %s: err=%v requests=%d", pp, r.err, len(r.queries))
		}
	}
}
