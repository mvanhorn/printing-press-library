// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Regression tests for the Phase 4.95 local code review (2026-10-05). Each
// one failed before its fix.

package uberjobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestQueuedClientSeesLatchUnderGateLock: a second process queued at the
// gate while the first one's request is refused must not send. Before the
// fix it checked the latch before queueing and sent one gap later anyway.
func TestQueuedClientSeesLatchUnderGateLock(t *testing.T) {
	t.Setenv(RefusalCooldownEnv, "1h")
	arrived, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			close(arrived)
			<-release
		})
		w.WriteHeader(http.StatusForbidden)
	})
	stateDir := t.TempDir()
	mk := func() *Client {
		c := NewClient(srv.URL, 5*time.Second, stateDir, "")
		c.Limiter, c.RequestLog = nil, ""
		c.Gate = &Gate{Dir: stateDir, Gap: 50 * time.Millisecond}
		return c
	}
	a, b := mk(), mk()
	errA, errB := make(chan error, 1), make(chan error, 1)
	go func() { _, _, err := a.ProbeTotal(context.Background(), Query{Team: "Legal"}); errA <- err }()
	<-arrived
	go func() { _, _, err := b.ProbeTotal(context.Background(), Query{Team: "Legal"}); errB <- err }()
	time.Sleep(150 * time.Millisecond) // b passes its first latch check and queues at the gate
	close(release)
	if err := <-errA; !IsRefusal(err) {
		t.Fatalf("first process: err = %v, want a refusal", err)
	}
	err := <-errB
	var r *RefusalError
	if !errors.As(err, &r) || !r.Latched {
		t.Fatalf("queued process: err = %v, want the latched refusal", err)
	}
	if n := srv.Count(); n != 1 {
		t.Fatalf("server saw %d requests, want 1: the queued process sent to a host that had just refused", n)
	}
}

// TestGateClampsFutureStamp: a last-request stamp ahead of the clock (the
// clock moved back) costs one gap, not the whole step while holding the lock.
func TestGateClampsFutureStamp(t *testing.T) {
	now := ujT0
	dir := t.TempDir()
	g, slept := ujFakeGate(dir, 3*time.Second, &now)
	if err := os.WriteFile(g.lastPath(), []byte(strconv.FormatInt(now.Add(6*time.Hour).UnixNano(), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if len(*slept) != 1 || (*slept)[0] != 3*time.Second {
		t.Fatalf("pauses = %v, want one 3s pause", *slept)
	}
	if last, ok := g.LastRequest(); !ok || !last.Equal(now) {
		t.Fatalf("stamp = %v (%v), want it rewritten to now %v", last, ok, now)
	}
}

// TestReadCacheFutureStampIsMiss: an entry stamped ahead of the clock is a
// miss, never fresh until the clock catches up.
func TestReadCacheFutureStampIsMiss(t *testing.T) {
	c := ujClient(t, "http://127.0.0.1:9")
	c.CacheDir = t.TempDir()
	const u, accept = "http://127.0.0.1:9/api/jobs/search/?page=1&pagesize=1", "application/json"
	put := func(stored time.Time) {
		raw, _ := json.Marshal(cacheEntry{Stored: stored.UnixNano(), URL: u, Body: []byte(`{}`)})
		p := c.cachePath(u, accept)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put(time.Now())
	if c.readCache(u, accept) == nil {
		t.Fatalf("control: an entry stamped now is not a hit")
	}
	put(time.Now().Add(24 * time.Hour))
	if c.readCache(u, accept) != nil {
		t.Fatalf("an entry stamped 24h ahead was served as fresh")
	}
}

// TestWriteCachePrunesExpiredEntries: expired entries are removed on the
// next write, so keys that move with the corpus size never pile up.
func TestWriteCachePrunesExpiredEntries(t *testing.T) {
	c := ujClient(t, "http://127.0.0.1:9")
	c.CacheDir = t.TempDir()
	const accept = "application/json"
	write := func(u string) string {
		c.writeCache(u, accept, &response{Body: []byte(`{}`), URL: u})
		return c.cachePath(u, accept)
	}
	old := write("http://127.0.0.1:9/a?pagesize=106")
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	fresh := write("http://127.0.0.1:9/b?pagesize=1")
	newest := write("http://127.0.0.1:9/c?pagesize=107")
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("expired entry still on disk (stat err %v)", err)
	}
	for _, p := range []string{fresh, newest} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("live entry %s was pruned: %v", filepath.Base(p), err)
		}
	}
}

// TestSearchAllUnfilteredEmptyIsContentError: the whole site reporting zero
// postings is a bad read; as a complete read it would let sync close every
// stored posting. A filtered query may still be empty and complete.
func TestSearchAllUnfilteredEmptyIsContentError(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	c := ujClient(t, srv.URL)
	var ce *ContentError
	if _, err := c.SearchAll(context.Background(), Query{}); !errors.As(err, &ce) {
		t.Fatalf("unfiltered empty corpus: err = %v, want a content error", err)
	}
	res, err := c.SearchAll(context.Background(), Query{Team: "Legal"})
	if err != nil || !res.Complete || len(res.Rows) != 0 {
		t.Fatalf("filtered empty read: res=%+v err=%v, want complete and empty", res, err)
	}
}

// TestOracleSearchAllTotalDriftIsIncomplete: a TotalJobsCount that moves
// between pages means a posting may have slid past a page boundary, so the
// read is not complete even when the row count reaches the last total.
func TestOracleSearchAllTotalDriftIsIncomplete(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		switch ujOffset(r.URL.RawQuery) {
		case 0:
			_, _ = w.Write(ujOraclePage(base, 1000, 200, 401))
		case 200:
			_, _ = w.Write(ujOraclePage(base, 1200, 200, 400))
		default:
			_, _ = w.Write(ujOraclePage(base, 0, 0, 400))
		}
	})
	c := ujOracleClient(t, srv.URL)
	res, err := c.OracleSearchAll(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 400 || res.Total != 400 {
		t.Fatalf("rows=%d total=%d, want 400 and 400", len(res.Rows), res.Total)
	}
	if res.Complete {
		t.Fatalf("total moved 401 -> 400 mid-walk, yet Complete=true")
	}
}

// ujSentinels are values that must never reach the disk (owner rule A3).
var ujSentinels = map[string]any{
	"HiringManager":               "SENTINEL-HIRING-MANAGER",
	"ExternalContactEmail":        "SENTINEL-CONTACT-EMAIL",
	"ExternalContactName":         "SENTINEL-CONTACT-NAME",
	"InternalResponsibilitiesStr": "SENTINEL-INTERNAL",
}

func ujNoSentinelOnDisk(t *testing.T, dir string) {
	t.Helper()
	n := 0
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		n++
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		// The entry stores the body base64-encoded, so decode it first; a
		// grep of the raw file alone would never see the body.
		var e cacheEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Errorf("cache file %s does not decode: %v", filepath.Base(p), err)
		}
		if strings.Contains(string(raw), "SENTINEL") || strings.Contains(string(e.Body), "SENTINEL") {
			t.Errorf("cache file %s holds a banned field: %.300s", filepath.Base(p), e.Body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("nothing was cached, so the check proves nothing")
	}
}

// TestOracleCacheHoldsOnlyTheProjection: the raw Oracle replies carry
// HiringManager, ExternalContact* and Internal* fields; only the decoded
// projection may be cached, and a cache hit must decode the same way.
func TestOracleCacheHoldsOnlyTheProjection(t *testing.T) {
	t.Run("detail", func(t *testing.T) {
		var env map[string]any
		if err := json.Unmarshal(ujReadTestdata(t, "oracle_detail.json"), &env); err != nil {
			t.Fatal(err)
		}
		item := env["items"].([]any)[0].(map[string]any)
		for k, v := range ujSentinels {
			item[k] = v
		}
		body, _ := json.Marshal(env)
		srv := ujDetailServer(t, body)
		c := ujOracleClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		first, err := c.OracleGet(context.Background(), "302906")
		if err != nil {
			t.Fatal(err)
		}
		ujNoSentinelOnDisk(t, c.CacheDir)
		again, err := c.OracleGet(context.Background(), "302906")
		if err != nil {
			t.Fatal(err)
		}
		if srv.Count() != 1 {
			t.Fatalf("requests = %d, want 1 (the second read is a cache hit)", srv.Count())
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("cache hit decodes differently:\n%+v\n%+v", first, again)
		}
	})
	t.Run("list", func(t *testing.T) {
		base := ujOracleRows(t)[0]
		for k, v := range ujSentinels {
			base[k] = v
		}
		srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
			if ujOffset(r.URL.RawQuery) == 0 {
				_, _ = w.Write(ujOraclePage(base, 1000, 3, 3))
				return
			}
			_, _ = w.Write(ujOraclePage(base, 0, 0, 3))
		})
		c := ujOracleClient(t, srv.URL)
		c.CacheDir = t.TempDir()
		first, err := c.OracleSearchAll(context.Background(), "data")
		if err != nil {
			t.Fatal(err)
		}
		ujNoSentinelOnDisk(t, c.CacheDir)
		again, err := c.OracleSearchAll(context.Background(), "data")
		if err != nil {
			t.Fatal(err)
		}
		if srv.Count() != 2 || again.Requests != 0 {
			t.Fatalf("requests = %d (second read %d), want 2 and 0", srv.Count(), again.Requests)
		}
		if !reflect.DeepEqual(first.Rows, again.Rows) {
			t.Fatalf("cache hit decodes differently")
		}
	})
}

// TestMatchClientSkipsBlankPhrases: a blank phrase matches every description,
// so as an exclusion it dropped every posting; it is now skipped.
func TestMatchClientSkipsBlankPhrases(t *testing.T) {
	p := ujMatchPosting()
	for _, f := range []Filters{
		{DescriptionExcludes: []string{"  "}},
		{DescriptionContains: []string{""}},
	} {
		if !f.MatchClient(p, 0, ujT0) {
			t.Errorf("filters %+v dropped the posting", f)
		}
	}
}

// TestOracleCacheHitDoesNotRefreshStamp: serving an Oracle reply from the
// cache must not re-write it with a new stamp, or a regularly re-read entry
// never expires. (Round 2: rememberProjection dropped the CacheHit flag.)
func TestOracleCacheHitDoesNotRefreshStamp(t *testing.T) {
	srv := ujDetailServer(t, ujReadTestdata(t, "oracle_detail.json"))
	c := ujOracleClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	if _, err := c.OracleGet(context.Background(), "302906"); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(c.CacheDir, "uberjobs", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("cache files = %v (%v), want 1", files, err)
	}
	read := func() int64 {
		raw, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		var e cacheEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		return e.Stored
	}
	before := read()
	time.Sleep(20 * time.Millisecond)
	if _, err := c.OracleGet(context.Background(), "302906"); err != nil {
		t.Fatal(err)
	}
	if srv.Count() != 1 {
		t.Fatalf("requests = %d, want 1 (the second read is a cache hit)", srv.Count())
	}
	if after := read(); after != before {
		t.Fatalf("a cache hit re-stamped the entry (%d -> %d), so it would never expire", before, after)
	}
}

// TestGateExpiredContextDoesNotStamp: a request whose deadline passed while
// it queued for the lock sends nothing and must not move the last-request
// stamp, which would make the next request wait for nothing.
func TestGateExpiredContextDoesNotStamp(t *testing.T) {
	now := ujT0
	g, slept := ujFakeGate(t.TempDir(), 3*time.Second, &now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sent := false
	if err := g.Do(ctx, nil, func() error { sent = true; return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Do with an expired context: err = %v, want context.Canceled", err)
	}
	if sent || len(*slept) != 0 {
		t.Fatalf("sent=%v pauses=%v, want nothing sent and no pause", sent, *slept)
	}
	if _, ok := g.LastRequest(); ok {
		t.Fatalf("an expired request stamped the gate")
	}
}

// TestRefusalWithTruncatedBodyStillLatches: a 403 whose body is cut short
// is still a refusal. Before the fix the read error came first, so the host
// was neither latched nor remembered and the next call sent to it again.
func TestRefusalWithTruncatedBodyStillLatches(t *testing.T) {
	t.Setenv(RefusalCooldownEnv, "1h")
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
		w.(http.Flusher).Flush()    // the status line and partial body reach the client
		panic(http.ErrAbortHandler) // then the connection drops mid-body
	})
	stateDir := t.TempDir()
	c := ujStateClient(t, srv.URL, stateDir)
	if _, _, err := c.ProbeTotal(context.Background(), Query{Team: "Legal"}); !IsRefusal(err) {
		t.Fatalf("403 with a truncated body: err = %v, want a refusal", err)
	}
	if LatchedRefusal(stateDir, srv.Host(), time.Now()) == nil {
		t.Fatalf("no latch file for a host that answered 403")
	}
	if _, _, err := c.ProbeTotal(context.Background(), Query{Team: "Legal"}); !IsRefusal(err) {
		t.Fatalf("second call: err = %v, want the remembered refusal", err)
	}
	if n := srv.Count(); n != 1 {
		t.Fatalf("server saw %d requests, want 1", n)
	}
}

// TestEmptyUnfilteredProbeIsNotCached: the zero-total unfiltered probe is
// rejected before it is cached, so a retry asks the site again instead of
// replaying the bad read for the whole cache TTL.
func TestEmptyUnfilteredProbeIsNotCached(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	var ce *ContentError
	for i := 0; i < 2; i++ {
		if _, err := c.SearchAll(context.Background(), Query{}); !errors.As(err, &ce) {
			t.Fatalf("call %d: err = %v, want a content error", i+1, err)
		}
	}
	if n := srv.Count(); n != 2 {
		t.Fatalf("server saw %d requests, want 2 (the bad probe was served from cache)", n)
	}
}
