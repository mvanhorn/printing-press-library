// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// The traffic guard on the generated client, doctor's content check, and
// the typed exit codes through RootCmd.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// ujcRT is a base RoundTripper that never touches the network: it answers
// each host with a fixed status and records every call.
type ujcRT struct {
	mu     sync.Mutex
	status map[string]int
	calls  []string
}

func (rt *ujcRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls = append(rt.calls, req.URL.String())
	st, ok := rt.status[req.URL.Hostname()]
	rt.mu.Unlock()
	if !ok {
		st = http.StatusOK
	}
	body := `{"jobs":[],"totalJobs":0}`
	if st != http.StatusOK {
		body = "refused"
	}
	return &http.Response{StatusCode: st, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func (rt *ujcRT) Calls() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return len(rt.calls)
}

func ujcGet(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// ujcResetGuardRefusal clears the process-wide last guard refusal for one
// test and restores it after, so guard tests cannot steer doctor tests.
func ujcResetGuardRefusal(t *testing.T) {
	t.Helper()
	lastGuardRefusalMu.Lock()
	prev := lastGuardRefusal
	lastGuardRefusal = nil
	lastGuardRefusalMu.Unlock()
	t.Cleanup(func() {
		lastGuardRefusalMu.Lock()
		lastGuardRefusal = prev
		lastGuardRefusalMu.Unlock()
	})
}

// TestUJCGuardedHost pins which hosts get paced and latched: Uber and the
// Oracle tenant, never look-alikes and never loopback mocks.
func TestUJCGuardedHost(t *testing.T) {
	for host, want := range map[string]bool{
		"jobs.uber.com": true, "JOBS.UBER.COM": true, "jobs.uber.com:443": true, "uber.com": true, "www.uber.com": true,
		"iaziqy.fa.ocs.oraclecloud.com": true, "iaziqy.fa.ocs.oraclecloud.com:443": true,
		"notuber.com": false, "evil-uber.com": false, "uber.com.evil.example": false, "oraclecloud.com.evil.example": false,
		"127.0.0.1:8080": false, "localhost": false, "[::1]:443": false, "": false,
	} {
		if got := guardedHost(host); got != want {
			t.Errorf("guardedHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// TestUJCGuardLatchesRefusedHost: after a 403 the guard sends nothing more
// to that host, and a fresh guard (another process) reads the latch file.
func TestUJCGuardLatchesRefusedHost(t *testing.T) {
	ujcResetGuardRefusal(t)
	t.Setenv(uberjobs.RefusalCooldownEnv, "")
	dir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "requests.tsv")
	rt := &ujcRT{status: map[string]int{"jobs.uber.com": http.StatusForbidden}}
	gate := uberjobs.NewGate(dir)
	g := &uberGuard{base: rt, gate: gate, stateDir: dir, logPath: logPath}
	const u = "https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1"

	resp, err := g.RoundTrip(ujcGet(t, u))
	body := ujcWantRefusalResponse(t, "first round trip", resp, err, http.StatusForbidden, false)
	if msg, _ := body["error"].(string); !strings.Contains(msg, "jobs.uber.com") {
		t.Fatalf("refusal body does not name the host: %v", body)
	}
	if _, ok := gate.LastRequest(); !ok {
		t.Fatalf("a guarded request did not pass through the gate")
	}

	resp, err = g.RoundTrip(ujcGet(t, "https://jobs.uber.com/en/jobs/"))
	ujcWantRefusalResponse(t, "second round trip", resp, err, http.StatusForbidden, false)
	if rt.Calls() != 1 {
		t.Fatalf("base saw %d calls, want 1: nothing is sent after a refusal", rt.Calls())
	}

	latched := uberjobs.LatchedRefusal(dir, "jobs.uber.com", time.Now())
	if latched == nil {
		t.Fatalf("no latch file recorded for the refused host")
	}
	y, m, d := time.Now().UTC().Date()
	if want := time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339); latched.Until != want {
		t.Fatalf("latch until %s, want the next UTC midnight %s", latched.Until, want)
	}

	fresh := &uberGuard{base: rt, gate: uberjobs.NewGate(dir), stateDir: dir}
	resp, err = fresh.RoundTrip(ujcGet(t, u))
	body = ujcWantRefusalResponse(t, "fresh guard", resp, err, http.StatusForbidden, false)
	if msg, _ := body["error"].(string); !strings.Contains(msg, "nothing is sent") {
		t.Fatalf("fresh guard answered without the latch: %v", body)
	}
	if r := guardRefusal(); r == nil || !r.Latched {
		t.Fatalf("guardRefusal() = %v, want the latched refusal", r)
	}
	if rt.Calls() != 1 {
		t.Fatalf("base saw %d calls after the latch, want 1", rt.Calls())
	}
	if got := guardRefusal(); got == nil || got.Host != "jobs.uber.com" {
		t.Fatalf("guardRefusal() = %v, want the jobs.uber.com refusal", got)
	}

	// Another guarded host is unaffected by this host's latch.
	if resp, err := fresh.RoundTrip(ujcGet(t, "https://iaziqy.fa.ocs.oraclecloud.com/x")); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("other host: resp=%v err=%v, want it sent", resp, err)
	}

	lines := ujcLines(t, logPath)
	if len(lines) != 1 || !strings.Contains(lines[0], "\tGET\t"+u+"\t403\t") {
		t.Fatalf("request log = %q, want one line for the one 403 sent", lines)
	}
}

// TestUJCGuardChallengeAt200Latches: a challenge page served with 200 is a
// refusal too (B9), recognized by the body markers.
func TestUJCGuardChallengeAt200Latches(t *testing.T) {
	ujcResetGuardRefusal(t)
	dir := t.TempDir()
	calls := 0
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<html><head><title>Just a moment...</title></head></html>")), Request: req}, nil
	})
	g := &uberGuard{base: base, gate: uberjobs.NewGate(dir), stateDir: dir}
	resp, err := g.RoundTrip(ujcGet(t, "https://jobs.uber.com/en/jobs/"))
	ujcWantRefusalResponse(t, "200 challenge", resp, err, http.StatusOK, true)
	resp, err = g.RoundTrip(ujcGet(t, "https://jobs.uber.com/en/jobs/"))
	ujcWantRefusalResponse(t, "after a 200 challenge", resp, err, http.StatusOK, true)
	if r := guardRefusal(); r == nil || !r.Challenge || calls != 1 {
		t.Fatalf("after a 200 challenge: refusal=%v calls=%d, want a challenge refusal and one call", r, calls)
	}
}

// ujcWantRefusalResponse asserts the guard's answer to a refused request: a
// terminal 429 (Retry-After beyond the generated retry budget) whose JSON
// body names the real refusal.
func ujcWantRefusalResponse(t *testing.T, what string, resp *http.Response, err error, status int, challenge bool) map[string]any {
	t.Helper()
	if err != nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "86400" {
		t.Fatalf("%s: resp=%v err=%v, want the guard's terminal 429", what, resp, err)
	}
	var body map[string]any
	if derr := json.NewDecoder(resp.Body).Decode(&body); derr != nil {
		t.Fatalf("%s: refusal body is not JSON: %v", what, derr)
	}
	if body["refused"] != true || body["status"] != float64(status) || body["challenge"] != challenge {
		t.Fatalf("%s: refusal body = %v, want refused true, status %d, challenge %v", what, body, status, challenge)
	}
	return body
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestUJCGuardUnguardedHostsAreNotPacedOrFileLatched: loopback mocks are
// neither paced, file-latched, nor logged, so verify runs stay fast and
// local. A refusal from them is still typed (exit 7, never auth) and still
// never retried inside one process; another process may try again.
func TestUJCGuardUnguardedHostsAreNotPacedOrFileLatched(t *testing.T) {
	ujcResetGuardRefusal(t)
	dir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "requests.tsv")
	rt := &ujcRT{status: map[string]int{"127.0.0.1": http.StatusForbidden}}
	gate := uberjobs.NewGate(dir)
	g := &uberGuard{base: rt, gate: gate, stateDir: dir, logPath: logPath}
	for i := 0; i < 3; i++ {
		resp, err := g.RoundTrip(ujcGet(t, "http://127.0.0.1:9/api/jobs/search/"))
		ujcWantRefusalResponse(t, "unguarded round trip", resp, err, http.StatusForbidden, false)
	}
	if rt.Calls() != 1 {
		t.Fatalf("base saw %d calls, want 1 (never retried inside one process)", rt.Calls())
	}
	if _, ok := gate.LastRequest(); ok {
		t.Fatalf("an unguarded request was paced through the gate")
	}
	if uberjobs.LatchedRefusal(dir, "127.0.0.1:9", time.Now()) != nil {
		t.Fatalf("an unguarded 403 wrote a cross-process latch")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("unguarded requests were logged (stat err %v)", err)
	}
	fresh := &uberGuard{base: rt, gate: uberjobs.NewGate(dir), stateDir: dir, logPath: logPath}
	resp, err := fresh.RoundTrip(ujcGet(t, "http://127.0.0.1:9/api/jobs/search/"))
	ujcWantRefusalResponse(t, "fresh guard on an unguarded host", resp, err, http.StatusForbidden, false)
	if rt.Calls() != 2 {
		t.Fatalf("base saw %d calls, want 2 (no file latch for an unguarded host)", rt.Calls())
	}
	if resp, err := fresh.RoundTrip(ujcGet(t, "http://localhost:9/ok")); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("another unguarded host: resp=%v err=%v, want it passed through", resp, err)
	}
}

func ujcLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// TestUJCInstallUberGuardLogsEachRealRoundTrip installs the guard the way
// every generated client gets it, with UBER_JOBS_REQUEST_LOG set: one TSV
// line per request that reached the base transport, none for refusals
// answered from the latch or for unguarded hosts. Costs one 3 s gate wait.
func TestUJCInstallUberGuardLogsEachRealRoundTrip(t *testing.T) {
	ujcIsolate(t, "", "")
	ujcResetGuardRefusal(t)
	logPath := filepath.Join(t.TempDir(), "ledger.tsv")
	t.Setenv("UBER_JOBS_REQUEST_LOG", logPath)
	rt := &ujcRT{status: map[string]int{"iaziqy.fa.ocs.oraclecloud.com": http.StatusForbidden}}
	c := &client.Client{HTTPClient: &http.Client{Transport: rt}}
	if err := installUberGuard(c); err != nil {
		t.Fatal(err)
	}
	if err := installUberGuard(c); err != nil {
		t.Fatal(err)
	}
	g, ok := c.HTTPClient.Transport.(*uberGuard)
	if !ok || g.base != rt || g.stateDir != uberStateDir() {
		t.Fatalf("transport = %T (base %v), want one guard over the base in the state dir", c.HTTPClient.Transport, g)
	}

	do := func(u string) (*http.Response, error) {
		resp, err := c.HTTPClient.Do(ujcGet(t, u))
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return resp, err
	}
	if resp, err := do("https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1"); err != nil || resp.StatusCode != 200 {
		t.Fatalf("uber request: %v %v", resp, err)
	}
	start := time.Now()
	if resp, err := do("https://iaziqy.fa.ocs.oraclecloud.com/hcmRestApi/x"); err != nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("oracle request: %v %v, want the guard's terminal 429 for the 403", resp, err)
	}
	if waited := time.Since(start); waited < uberjobs.MinRequestGap-200*time.Millisecond {
		t.Fatalf("second guarded request went out after %v, want the >= %v gap", waited, uberjobs.MinRequestGap)
	}
	if resp, err := do("https://iaziqy.fa.ocs.oraclecloud.com/hcmRestApi/y"); err != nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("latched host: %v %v, want the terminal 429 without a request", resp, err)
	}
	if resp, err := do("http://127.0.0.1:9/passthrough"); err != nil || resp.StatusCode != 200 {
		t.Fatalf("loopback request: %v %v", resp, err)
	}
	if rt.Calls() != 3 {
		t.Fatalf("base saw %d calls, want 3", rt.Calls())
	}
	lines := ujcLines(t, logPath)
	if len(lines) != 2 {
		t.Fatalf("request log has %d lines, want 2 (one per guarded request sent):\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []string{"https://jobs.uber.com/api/jobs/search/?page=1&pagesize=1\t200\t", "https://iaziqy.fa.ocs.oraclecloud.com/hcmRestApi/x\t403\t"} {
		cols := strings.Split(lines[i], "\t")
		if len(cols) != 6 || cols[1] != "uber-jobs-pp-cli(generated client)" || cols[2] != "GET" || !strings.Contains(lines[i], want) {
			t.Fatalf("log line %d = %q, want 6 columns for %q", i, lines[i], want)
		}
		if _, err := time.Parse("2006-01-02T15:04:05Z", cols[0]); err != nil {
			t.Fatalf("log line %d timestamp %q: %v", i, cols[0], err)
		}
	}
}

// TestUJCGuardOnGeneratedClientLatchesChallenge drives a real generated
// client through the guard: a 403 challenge from jobs.uber.com is sent
// once, latched on disk for every later process, recorded for doctor, and
// classified as a refusal (exit 7), never as the auth exit code.
func TestUJCGuardOnGeneratedClientLatchesChallenge(t *testing.T) {
	ujcIsolate(t, "", "")
	ujcResetGuardRefusal(t)
	rt := &ujcRT{status: map[string]int{"jobs.uber.com": http.StatusForbidden}}
	c := client.New(&config.Config{BaseURL: "https://jobs.uber.com"}, 20*time.Second, 0)
	c.NoCache = true
	c.HTTPClient.Transport = rt
	if err := installUberGuard(c); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := c.Get(ctx, "/api/jobs/search/", map[string]string{"page": "1", "pagesize": "1"})
	var limited *platform.RateLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("challenge: err = %T %v, want the generated client's typed rate-limit error", err, err)
	}
	if code := ExitCode(classifyAPIErrorOnly(err)); code != 7 {
		t.Fatalf("challenge classified as exit %d, want 7 (never 4)", code)
	}
	if rt.Calls() != 1 {
		t.Fatalf("base saw %d calls, want exactly 1", rt.Calls())
	}
	if r := guardRefusal(); r == nil || r.Status != http.StatusForbidden || r.Host != "jobs.uber.com" {
		t.Fatalf("guardRefusal() = %v, want the jobs.uber.com 403", r)
	}
	if uberjobs.LatchedRefusal(uberStateDir(), "jobs.uber.com", time.Now()) == nil {
		t.Fatalf("the refusal was not latched in the state dir for later processes")
	}
}

// TestUJCDoctorContentCheck: doctor says "API: reachable" only after a real
// search envelope; a challenge is blocked (7), an HTML 200 is an error (5),
// and a dead port is unreachable (6). Each case gets a fresh sandbox
// because the generated client caches the health response.
func TestUJCDoctorContentCheck(t *testing.T) {
	cases := []struct {
		name, mode, want string
		code             int
	}{
		{"challenge", "challenge", "API: blocked", 7},
		{"html200", "html", "API: error (the health check returned no search envelope)", 5},
		{"ok", "ok", "API: reachable", 0},
		{"deadport", "", "API: unreachable", 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ujcResetGuardRefusal(t)
			siteURL := ""
			var site *ujcFake
			if tc.mode != "" {
				site = ujcNewFake(t)
				site.SetMode(tc.mode)
				siteURL = site.URL
			}
			ujcIsolate(t, siteURL, "")
			r := ujcRun(t, "", "doctor")
			ujcWantCode(t, r, tc.code)
			if !strings.Contains(r.Stdout, tc.want) {
				t.Fatalf("doctor stdout lacks %q:\n%s", tc.want, r.Stdout)
			}
			if tc.code != 0 && strings.Contains(r.Stdout, "reachable") && !strings.Contains(r.Stdout, "unreachable") {
				t.Fatalf("a failed doctor still says reachable:\n%s", r.Stdout)
			}
			if site != nil && site.Count("/api/jobs/search/") < 1 {
				t.Fatalf("doctor sent no health request")
			}
		})
	}

	// A config the client cannot be built from is the generated doctor's to
	// report: the wrapper sends nothing and prints no API verdict of its own.
	t.Run("config-error-defers", func(t *testing.T) {
		ujcResetGuardRefusal(t)
		site := ujcNewFake(t)
		ujcIsolate(t, site.URL, "")
		bad := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(bad, []byte("base_url = [unterminated"), 0o600); err != nil {
			t.Fatal(err)
		}
		r := ujcRun(t, "", "doctor", "--json", "--config", bad)
		ujcWantCode(t, r, 0)
		var got map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &got); err != nil {
			t.Fatalf("doctor --json output: %v\n%s", err, r.Stdout)
		}
		if cfg, _ := got["config"].(string); !strings.HasPrefix(cfg, "error:") {
			t.Fatalf("doctor config = %v, want the generated config error", got["config"])
		}
		if _, ok := got["api"]; ok || got["ok"] == false {
			t.Fatalf("doctor printed an API verdict without a client: %v", got)
		}
		if n := site.Count(""); n != 0 {
			t.Fatalf("doctor with a broken config sent %d requests, want 0", n)
		}
		// Every other command maps the same config error to exit 10.
		ujcWantCode(t, ujcRun(t, "", "postings", "--country", "GBR", "--json", "--data-source", "live", "--config", bad), 10)
	})

	t.Run("challenge-json", func(t *testing.T) {
		ujcResetGuardRefusal(t)
		site := ujcNewFake(t)
		site.SetMode("challenge")
		ujcIsolate(t, site.URL, "")
		r := ujcRun(t, "", "doctor", "--json")
		ujcWantCode(t, r, 7)
		var got map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &got); err != nil {
			t.Fatalf("doctor --json output: %v\n%s", err, r.Stdout)
		}
		if api, _ := got["api"].(string); got["ok"] != false || !strings.HasPrefix(api, "blocked") {
			t.Fatalf("doctor --json = %v, want ok false and api blocked", got)
		}
		if n := site.Count(""); n != 1 {
			t.Fatalf("blocked doctor sent %d requests, want 1 (never retried)", n)
		}
	})
}

// TestUJCExitCodesThroughRootCmd maps each failure class to its typed exit
// code with errors.As on *cliError: 2 usage, 3 not found, 5 HTTP 500 and
// HTML-instead-of-JSON, 6 transport, 7 refusal (403 challenge and 429).
func TestUJCExitCodesThroughRootCmd(t *testing.T) {
	type tc struct {
		name, siteMode, oracleMode string
		deadSite                   bool
		args                       []string
		code                       int
		siteRequests               int
	}
	gbr := []string{"postings", "--country", "GBR", "--json", "--data-source", "live"}
	cases := []tc{
		{name: "usage", siteMode: "ok", args: []string{"postings", "--country", "XQZ", "--json"}, code: 2, siteRequests: 0},
		{name: "notfound", siteMode: "ok", args: []string{"get", "999999999", "--json", "--data-source", "live"}, code: 3, siteRequests: 2},
		{name: "http500", siteMode: "500", args: gbr, code: 5, siteRequests: 1},
		{name: "html200", siteMode: "html", args: gbr, code: 5, siteRequests: 1},
		{name: "transport", deadSite: true, args: gbr, code: 6},
		{name: "challenge", siteMode: "challenge", oracleMode: "challenge", args: gbr, code: 7, siteRequests: 1},
		{name: "429", siteMode: "429", oracleMode: "429", args: gbr, code: 7, siteRequests: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			siteURL, oracleURL := "", ""
			var site *ujcFake
			if !c.deadSite {
				site = ujcNewFake(t)
				site.SetMode(c.siteMode)
				siteURL = site.URL
			}
			if c.oracleMode != "" {
				oracle := ujcNewFake(t)
				oracle.SetOracleMode(c.oracleMode)
				oracleURL = oracle.URL
			}
			ujcIsolate(t, siteURL, oracleURL)
			r := ujcRun(t, "", c.args...)
			var ce *cliError
			if !errors.As(r.Err, &ce) || ce.code != c.code {
				t.Fatalf("err = %v, want *cliError with code %d", r.Err, c.code)
			}
			if r.Stdout != "" {
				t.Fatalf("a failed command printed to stdout: %s", r.Stdout)
			}
			if site != nil && site.Count("") != c.siteRequests {
				t.Fatalf("site saw %d requests, want %d: %v", site.Count(""), c.siteRequests, site.Requests())
			}
			if c.code == 7 && !strings.Contains(r.Err.Error(), "not retried") {
				t.Fatalf("refusal error = %v, want it to say it was not retried", r.Err)
			}
		})
	}
}
