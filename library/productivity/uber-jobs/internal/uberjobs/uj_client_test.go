// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
)

// ujRefusalCases are the refusal shapes the site or its CDN can send.
// Each must stop the client after exactly one request.
var ujRefusalCases = []struct {
	name      string
	status    int
	header    map[string]string
	body      string
	challenge bool
}{
	{name: "http 403", status: http.StatusForbidden, body: "Forbidden"},
	{name: "http 429", status: http.StatusTooManyRequests, body: "Too Many Requests"},
	{name: "challenge title", status: http.StatusOK, body: "<!DOCTYPE html><html><head><title>Just a moment...</title></head><body></body></html>", challenge: true},
	{name: "cf_chl_opt script", status: http.StatusOK, body: "<html><script>window._cf_chl_opt={cvId:'3'};</script></html>", challenge: true},
	{name: "cf-error-details", status: http.StatusOK, body: `<html><body><div id="cf-error-details">Sorry, you have been blocked</div></body></html>`, challenge: true},
	// A JSON body that would pass the content check still counts as a
	// refusal when the CDN says it mitigated the request.
	{name: "cf-mitigated header", status: http.StatusOK, header: map[string]string{"Cf-Mitigated": "challenge"}, body: `{"jobs":[],"totalJobs":0}`, challenge: true},
	{name: "challenge with 403", status: http.StatusForbidden, body: "<html><head><title>Attention Required! | Cloudflare</title></head></html>", challenge: true},
}

func ujRefusingServer(t *testing.T, status int, header map[string]string, body string) *ujFake {
	t.Helper()
	return ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// TestRefusalStopsAfterOneRequest covers the no-retry rule and both
// latches: the same client sends nothing more to the host, and a new client
// on the same state dir (another process) sends nothing either.
func TestRefusalStopsAfterOneRequest(t *testing.T) {
	// A fixed cooldown keeps the latch active even if the run crosses 00:00 UTC.
	t.Setenv(RefusalCooldownEnv, "1h")
	for _, tc := range ujRefusalCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := ujRefusingServer(t, tc.status, tc.header, tc.body)
			stateDir := t.TempDir()
			c := ujStateClient(t, srv.URL, stateDir)
			ctx := context.Background()

			_, err := c.SearchAll(ctx, Query{})
			var r *RefusalError
			if !errors.As(err, &r) {
				t.Fatalf("err = %v (%T), want *RefusalError", err, err)
			}
			if r.Status != tc.status || r.Challenge != tc.challenge || r.Latched {
				t.Errorf("refusal = status %d challenge %v latched %v, want %d %v false", r.Status, r.Challenge, r.Latched, tc.status, tc.challenge)
			}
			if r.Host != srv.Host() || !strings.Contains(r.URL, searchPath) {
				t.Errorf("refusal host/url = %q %q", r.Host, r.URL)
			}
			var rl *cliutil.RateLimitError
			if !errors.As(err, &rl) {
				t.Errorf("refusal does not unwrap to *cliutil.RateLimitError (exit 7 mapping)")
			}
			if !strings.Contains(err.Error(), "not retrying") {
				t.Errorf("message = %q, want it to say it is not retrying", err.Error())
			}
			if srv.Count() != 1 || c.Requests() != 1 {
				t.Fatalf("server saw %d requests (client counted %d), want exactly 1", srv.Count(), c.Requests())
			}

			// Same client: the per-host latch sends nothing.
			_, err = c.SearchAll(ctx, Query{Search: "different"})
			if !IsRefusal(err) || srv.Count() != 1 {
				t.Errorf("second call: err=%v server requests=%d, want a refusal and still 1", err, srv.Count())
			}
			if c.Refused() == nil || c.RefusedHost(srv.Host()) == nil {
				t.Errorf("client did not record the refusal for %s", srv.Host())
			}

			// New client, same state dir: the latch file sends nothing.
			latch := latchPath(stateDir, srv.Host())
			if _, err := os.Stat(latch); err != nil {
				t.Fatalf("latch file %s not written: %v", latch, err)
			}
			c2 := ujStateClient(t, srv.URL, stateDir)
			_, err = c2.SearchAll(ctx, Query{})
			var r2 *RefusalError
			if !errors.As(err, &r2) {
				t.Fatalf("new client: err = %v, want *RefusalError from the latch", err)
			}
			if !r2.Latched || r2.LatchFile != latch || r2.Status != tc.status {
				t.Errorf("latched refusal = %+v, want Latched, LatchFile %s, status %d", r2, latch, tc.status)
			}
			if !strings.Contains(err.Error(), latch) {
				t.Errorf("latched message %q does not name the latch file", err.Error())
			}
			if !errors.As(err, &rl) {
				t.Errorf("latched refusal does not unwrap to *cliutil.RateLimitError")
			}
			if srv.Count() != 1 || c2.Requests() != 0 {
				t.Errorf("new client reached the server: server=%d client=%d", srv.Count(), c2.Requests())
			}

			log, err := os.ReadFile(filepath.Join(stateDir, "refusals.tsv"))
			if err != nil {
				t.Fatalf("refusals.tsv: %v", err)
			}
			if lines := strings.Count(string(log), "\n"); lines != 1 {
				t.Errorf("refusals.tsv has %d lines, want 1 (latched refusals are not re-logged as new): %q", lines, log)
			}
		})
	}
}

// TestRefusalLatchIsPerHost: a refusal from one host must not block the
// other (the Oracle fallback runs only because the site refused).
func TestRefusalLatchIsPerHost(t *testing.T) {
	// A fixed cooldown keeps the latch active even if the run crosses 00:00 UTC.
	t.Setenv(RefusalCooldownEnv, "1h")
	bad := ujRefusingServer(t, http.StatusForbidden, nil, "Forbidden")
	good := ujSearchServer(t, nil, 0)
	stateDir := t.TempDir()
	c := ujStateClient(t, bad.URL, stateDir)
	if _, err := c.SearchAll(context.Background(), Query{}); !IsRefusal(err) {
		t.Fatalf("err = %v, want refusal", err)
	}
	c.BaseURL = good.URL
	// A filtered query: an empty unfiltered corpus is itself a content error.
	if _, err := c.SearchAll(context.Background(), Query{Team: "Legal"}); err != nil {
		t.Fatalf("other host blocked by the first host's refusal: %v", err)
	}
	if good.Count() != 1 {
		t.Errorf("good host requests = %d, want 1", good.Count())
	}
	if c.RefusedHost(good.Host()) != nil {
		t.Errorf("good host recorded as refused")
	}
	if r := LatchedRefusal(stateDir, good.Host(), time.Now()); r != nil {
		t.Errorf("good host latched: %+v", r)
	}
}

// TestExpiredLatchLetsRequestsThrough: a latch whose Until has passed no
// longer blocks, so a refusal does not silence the CLI forever.
func TestExpiredLatchLetsRequestsThrough(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	stateDir := t.TempDir()
	past := time.Now().UTC().Add(-time.Hour)
	b, _ := json.Marshal(refusalLatch{Host: srv.Host(), URL: srv.URL, Status: 429, At: past.Add(-time.Hour).Format(time.RFC3339), Until: past.Format(time.RFC3339)})
	if err := os.WriteFile(latchPath(stateDir, srv.Host()), b, 0o600); err != nil {
		t.Fatal(err)
	}
	c := ujStateClient(t, srv.URL, stateDir)
	if _, err := c.SearchAll(context.Background(), Query{Team: "Legal"}); err != nil {
		t.Fatalf("expired latch still blocks: %v", err)
	}
	if srv.Count() != 1 {
		t.Errorf("requests = %d, want 1", srv.Count())
	}
}

// TestActiveLatchFromDiskBlocks: a latch written by another process with a
// future Until blocks this client before the gate or the network.
func TestActiveLatchFromDiskBlocks(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	stateDir := t.TempDir()
	until := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	b, _ := json.Marshal(refusalLatch{Host: srv.Host(), URL: srv.URL + searchPath, Status: 403, Challenge: true, At: "2026-10-05T08:00:00Z", Until: until})
	if err := os.WriteFile(latchPath(stateDir, srv.Host()), b, 0o600); err != nil {
		t.Fatal(err)
	}
	c := ujStateClient(t, srv.URL, stateDir)
	_, err := c.SearchAll(context.Background(), Query{})
	var r *RefusalError
	if !errors.As(err, &r) || !r.Latched || !r.Challenge || r.Status != 403 || r.Until != until || r.LatchedAt != "2026-10-05T08:00:00Z" {
		t.Fatalf("err = %v (%+v), want the latched challenge refusal", err, r)
	}
	if srv.Count() != 0 {
		t.Errorf("requests = %d, want 0", srv.Count())
	}
	if !strings.Contains(err.Error(), until) {
		t.Errorf("message %q does not say when the latch ends", err.Error())
	}
}

// TestCorruptLatchFailsClosed: an unreadable latch must block, not open the
// host again; a truncated write is the realistic way to get one.
func TestCorruptLatchFailsClosed(t *testing.T) {
	for _, content := range []string{"not json", `{"host":"x","until":"2026-`, `{"host":"x"}`, ""} {
		srv := ujSearchServer(t, nil, 0)
		stateDir := t.TempDir()
		path := latchPath(stateDir, srv.Host())
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		c := ujStateClient(t, srv.URL, stateDir)
		_, err := c.SearchAll(context.Background(), Query{})
		var r *RefusalError
		if !errors.As(err, &r) || !r.Latched || r.LatchFile != path {
			t.Errorf("latch %q: err = %v, want a latched refusal naming %s", content, err, path)
			continue
		}
		if srv.Count() != 0 {
			t.Errorf("latch %q: requests = %d, want 0", content, srv.Count())
		}
		until, perr := time.Parse(time.RFC3339, r.Until)
		if perr != nil || !until.After(time.Now()) || until.Hour() != 0 || until.Minute() != 0 {
			t.Errorf("latch %q: Until = %q, want the next 00:00 UTC", content, r.Until)
		}
	}
}

// TestTransportErrorClosedPort: a refused TCP connection is a transport
// error (exit 6), never a refusal latch.
func TestTransportErrorClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	stateDir := t.TempDir()
	c := ujStateClient(t, "http://"+addr, stateDir)
	logPath := filepath.Join(t.TempDir(), "ledger.tsv")
	c.RequestLog = logPath
	_, err = c.SearchAll(context.Background(), Query{})
	var te *TransportError
	if !errors.As(err, &te) || !IsTransport(err) {
		t.Fatalf("err = %v (%T), want *TransportError", err, err)
	}
	if te.DNS || IsRefusal(err) {
		t.Errorf("DNS=%v refusal=%v, want a plain connection error", te.DNS, IsRefusal(err))
	}
	if !strings.Contains(err.Error(), "network error reaching") {
		t.Errorf("message = %q", err.Error())
	}
	if _, serr := os.Stat(latchPath(stateDir, addr)); !os.IsNotExist(serr) {
		t.Errorf("a transport failure wrote a refusal latch")
	}
	ledger, _ := os.ReadFile(logPath)
	if !strings.Contains(string(ledger), "\tERR:") {
		t.Errorf("ledger = %q, want an ERR: line for the failed request", ledger)
	}
}

// TestContextDeadlineIsNotRefusal: a caller timeout returns the context
// error so it maps to the timeout exit, and nothing is latched.
func TestContextDeadlineIsNotRefusal(t *testing.T) {
	release := make(chan struct{})
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	stateDir := t.TempDir()
	c := ujStateClient(t, srv.URL, stateDir)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := c.SearchAll(ctx, Query{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v (%T), want context.DeadlineExceeded", err, err)
	}
	if IsRefusal(err) || IsTransport(err) {
		t.Errorf("deadline misclassified: refusal=%v transport=%v", IsRefusal(err), IsTransport(err))
	}
	if c.Refused() != nil || LatchedRefusal(stateDir, srv.Host(), time.Now()) != nil {
		t.Errorf("a timeout latched the host")
	}
}

// TestIsChallengeMarkers: anchored markers only; ordinary pages that merely
// mention Cloudflare or "a moment" are content, not a challenge.
func TestIsChallengeMarkers(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		body   string
		want   bool
	}{
		{"just a moment title", nil, "<TITLE>Just a moment...</TITLE>", true},
		{"cf_chl_opt", nil, "var _cf_chl_opt = {};", true},
		{"cf-error-details", nil, `<div class="cf-error-details">`, true},
		{"attention required", nil, "<title>Attention Required! | Cloudflare</title>", true},
		{"cf-mitigated header", http.Header{"Cf-Mitigated": []string{"challenge"}}, `{"jobs":[]}`, true},
		{"json corpus", nil, `{"jobs":[{"Title":"Just a moment of your time"}],"totalJobs":1}`, false},
		{"mentions cloudflare", nil, "<html><body>We use Cloudflare for speed.</body></html>", false},
		{"empty", nil, "", false},
		{"marker past 16 KiB", nil, strings.Repeat("x", 16400) + "<title>Just a moment", false},
	}
	for _, tc := range cases {
		h := tc.header
		if h == nil {
			h = http.Header{}
		}
		if got := IsChallenge(h, []byte(tc.body)); got != tc.want {
			t.Errorf("%s: IsChallenge = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCacheServesIdenticalGET: an identical GET inside the TTL is served
// from disk with no second request and no ledger line.
func TestCacheServesIdenticalGET(t *testing.T) {
	rows := ujCorpusRows(t)[:3]
	srv := ujSearchServer(t, rows, 3)
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	logPath := filepath.Join(t.TempDir(), "ledger.tsv")
	c.RequestLog = logPath
	ctx := context.Background()

	first, err := c.SearchAll(ctx, Query{})
	if err != nil {
		t.Fatalf("first SearchAll: %v", err)
	}
	second, err := c.SearchAll(ctx, Query{})
	if err != nil {
		t.Fatalf("second SearchAll: %v", err)
	}
	if srv.Count() != 2 {
		t.Errorf("server requests = %d, want 2 (second read fully cached)", srv.Count())
	}
	if first.CacheHit || first.Requests != 2 {
		t.Errorf("first read CacheHit=%v Requests=%d, want false/2", first.CacheHit, first.Requests)
	}
	if !second.CacheHit || second.Requests != 0 || second.Cache["local-cache"] != "hit" {
		t.Errorf("second read CacheHit=%v Requests=%d Cache=%v, want a full local hit", second.CacheHit, second.Requests, second.Cache)
	}
	if len(second.Rows) != 3 || !second.Complete {
		t.Errorf("cached read rows=%d complete=%v, want 3/true", len(second.Rows), second.Complete)
	}
	ledger, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(ledger)), "\n")
	if len(lines) != 2 {
		t.Fatalf("ledger has %d lines, want 2 (cache hits are not network requests): %q", len(lines), ledger)
	}
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			t.Errorf("ledger line has %d fields, want 6: %q", len(f), line)
			continue
		}
		if _, err := time.Parse("2006-01-02T15:04:05Z", f[0]); err != nil {
			t.Errorf("ledger time %q: %v", f[0], err)
		}
		if f[1] != "uber-jobs-pp-cli" || f[2] != "GET" || !strings.HasPrefix(f[3], srv.URL+searchPath) || f[4] != "200" || f[5] == "0" {
			t.Errorf("ledger line = %q", line)
		}
	}
}

// TestCacheMisses: NoCache, a different Accept, a different URL, an expired
// entry, and an error reply all go to the network.
// Probes are filtered: an empty unfiltered corpus is itself a content error.
func TestCacheMisses(t *testing.T) {
	ctx := context.Background()

	t.Run("no cache flag", func(t *testing.T) {
		srv := ujSearchServer(t, nil, 0)
		c := ujClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		c.NoCache = true
		for i := 0; i < 2; i++ {
			if _, _, err := c.ProbeTotal(ctx, Query{Team: "Sales"}); err != nil {
				t.Fatal(err)
			}
		}
		if srv.Count() != 2 {
			t.Errorf("requests = %d, want 2", srv.Count())
		}
		if entries, _ := os.ReadDir(filepath.Join(c.CacheDir, "uberjobs")); len(entries) != 0 {
			t.Errorf("NoCache wrote %d cache files", len(entries))
		}
	})

	t.Run("different accept", func(t *testing.T) {
		srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
		c := ujClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		// get never caches by itself; callers remember a reply only after
		// its content check passes, so this models two validated replies.
		first, err := c.get(ctx, srv.URL+"/x", "application/json", UserAgent)
		if err != nil {
			t.Fatal(err)
		}
		c.remember("application/json", first)
		resp, err := c.get(ctx, srv.URL+"/x", "text/html", UserAgent)
		if err != nil {
			t.Fatal(err)
		}
		c.remember("text/html", resp)
		if resp.CacheHit || srv.Count() != 2 {
			t.Errorf("CacheHit=%v requests=%d, want a miss on a different Accept", resp.CacheHit, srv.Count())
		}
		resp, err = c.get(ctx, srv.URL+"/x", "text/html", UserAgent)
		if err != nil || !resp.CacheHit || string(resp.Body) != "ok" {
			t.Errorf("repeat with same Accept: hit=%v body=%q err=%v, want a cached ok", resp != nil && resp.CacheHit, resp.Body, err)
		}
	})

	t.Run("different url", func(t *testing.T) {
		srv := ujSearchServer(t, nil, 0)
		c := ujClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		if _, _, err := c.ProbeTotal(ctx, Query{Team: "Sales"}); err != nil {
			t.Fatal(err)
		}
		_, resp, err := c.ProbeTotal(ctx, Query{Team: "Legal"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.CacheHit || srv.Count() != 2 {
			t.Errorf("CacheHit=%v requests=%d, want a miss on a different URL", resp.CacheHit, srv.Count())
		}
	})

	t.Run("expired entry", func(t *testing.T) {
		srv := ujSearchServer(t, nil, 0)
		c := ujClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		_, resp, err := c.ProbeTotal(ctx, Query{Team: "Sales"})
		if err != nil {
			t.Fatal(err)
		}
		path := c.cachePath(resp.URL, "application/json")
		var e cacheEntry
		raw, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(raw, &e) != nil {
			t.Fatalf("cache entry at %s unreadable: %v", path, err)
		}
		e.Stored = time.Now().Add(-defaultCacheTTL - time.Second).UnixNano()
		raw, _ = json.Marshal(e)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		_, resp, err = c.ProbeTotal(ctx, Query{Team: "Sales"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.CacheHit || srv.Count() != 2 {
			t.Errorf("CacheHit=%v requests=%d, want an expired entry to miss", resp.CacheHit, srv.Count())
		}
	})

	t.Run("error replies are not cached", func(t *testing.T) {
		first := make(chan struct{}, 1)
		first <- struct{}{}
		srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-first:
				w.WriteHeader(http.StatusBadGateway)
			default:
				_, _ = w.Write(ujSearchJSON(nil, 0))
			}
		})
		c := ujClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		if _, _, err := c.ProbeTotal(ctx, Query{Team: "Sales"}); err == nil {
			t.Fatal("first probe: want a status error")
		}
		total, resp, err := c.ProbeTotal(ctx, Query{Team: "Sales"})
		if err != nil || resp.CacheHit || total != 0 {
			t.Errorf("second probe: total=%d err=%v hit=%v, want a fresh network read", total, err, resp != nil && resp.CacheHit)
		}
		if srv.Count() != 2 {
			t.Errorf("requests = %d, want 2", srv.Count())
		}
	})
}

// TestRequestLogAndOnRequest: every network request (GET and POST) lands in
// the ledger and the OnRequest seam exactly once.
func TestRequestLogAndOnRequest(t *testing.T) {
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"jobs":[]}`))
			return
		}
		_, _ = w.Write(ujSearchJSON(nil, 0))
	})
	c := ujClient(t, srv.URL)
	logPath := filepath.Join(t.TempDir(), "ledger.tsv")
	c.RequestLog = logPath
	var recs []RequestRecord
	c.OnRequest = func(r RequestRecord) { recs = append(recs, r) }
	ctx := context.Background()
	if _, err := c.SearchAll(ctx, Query{Team: "Legal"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Lookup(ctx, []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].Method != "GET" || recs[1].Method != "POST" || recs[0].Status != 200 {
		t.Errorf("OnRequest records = %+v, want GET then POST", recs)
	}
	ledger, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(ledger)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], "\tPOST\t"+srv.URL+lookupPath+"\t200\t") {
		t.Errorf("ledger = %q, want a GET line then a POST line", ledger)
	}
}

// TestNewClientDefaults pins the constructor: honest UA constant, 2-minute
// cache, 1/3 rps limiter, base URL trimmed, and the env request log.
func TestNewClientDefaults(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "ledger.tsv")
	t.Setenv("UBER_JOBS_REQUEST_LOG", ledgerPath)
	t.Setenv("UBER_JOBS_ORACLE_BASE_URL", "http://127.0.0.1:9/")
	c := NewClient("http://127.0.0.1:1/", 0, "", "")
	if c.BaseURL != "http://127.0.0.1:1" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", c.BaseURL)
	}
	if c.OracleBase != "http://127.0.0.1:9" {
		t.Errorf("OracleBase = %q, want the env value trimmed", c.OracleBase)
	}
	if c.HTTP.Timeout != 60*time.Second || c.CacheTTL != 2*time.Minute || c.Limiter == nil {
		t.Errorf("timeout=%v ttl=%v limiter=%v", c.HTTP.Timeout, c.CacheTTL, c.Limiter)
	}
	if c.RequestLog != ledgerPath {
		t.Errorf("RequestLog = %q, want the env value", c.RequestLog)
	}
	if c.HTTP.CheckRedirect == nil || c.HTTP.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Errorf("redirects must not be followed silently")
	}

	t.Setenv("UBER_JOBS_ORACLE_BASE_URL", "")
	if got := NewClient("http://127.0.0.1:1", 0, "", "").OracleBase; got != "" {
		t.Errorf("overridden base without an Oracle override: OracleBase = %q, want disabled", got)
	}
	if got := NewClient("", 0, "", ""); got.BaseURL != DefaultBaseURL || got.OracleBase != OracleDefaultBase {
		t.Errorf("default client bases = %q %q", got.BaseURL, got.OracleBase)
	}
}

// TestRedirectIsNotFollowed: a 3xx is its own request; the client reports
// it as a status error instead of silently fetching the target.
func TestRedirectIsNotFollowed(t *testing.T) {
	target := ujSearchServer(t, nil, 0)
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+searchPath, http.StatusFound)
	})
	c := ujClient(t, srv.URL)
	_, err := c.SearchAll(context.Background(), Query{})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusFound {
		t.Fatalf("err = %v, want *StatusError 302", err)
	}
	if target.Count() != 0 {
		t.Errorf("redirect target received %d requests", target.Count())
	}
}

// TestClientPacesThroughGate: each network request passes the gate; cache
// hits and latched refusals never touch it.
func TestClientPacesThroughGate(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	now := ujT0
	var slept []time.Duration
	c.Gate = &Gate{Dir: t.TempDir(), Gap: MinRequestGap,
		now:   func() time.Time { return now },
		sleep: func(_ context.Context, d time.Duration) error { slept = append(slept, d); now = now.Add(d); return nil },
	}
	ctx := context.Background()
	if _, _, err := c.ProbeTotal(ctx, Query{Team: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ProbeTotal(ctx, Query{Team: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ProbeTotal(ctx, Query{Team: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 1 || slept[0] != MinRequestGap {
		t.Errorf("gate pauses = %v, want one 3s pause (second request; third is a cache hit)", slept)
	}
	if srv.Count() != 2 {
		t.Errorf("requests = %d, want 2", srv.Count())
	}
}
