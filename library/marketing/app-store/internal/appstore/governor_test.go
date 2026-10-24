// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// The governor across invocations: every "process" below is a fresh Client
// sharing one state file, the way consecutive CLI runs share
// <state dir>/appstore-governor.json. Before the state file, each new client
// started with an empty limiter and an untripped governor, so it sent at once
// after another process had been throttled (live, 2026-10-23: 2 of 6 page
// fetches throttled at 10 s and 76 s between invocations).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

const candyCrushPageURL = "https://apps.apple.com/us/app/candy-crush-saga/id553834731"

// governorHandler throttles the first product-page request (HTTP 429,
// Retry-After 14) and serves the fixtures afterwards.
func governorHandler(t *testing.T) func(w http.ResponseWriter, r *http.Request, n int) {
	var mu sync.Mutex
	throttled := false
	return func(w http.ResponseWriter, r *http.Request, n int) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/lookup"):
			_, _ = fmt.Fprint(w, lookupOneBody)
		case strings.Contains(r.URL.Path, "/charts/"):
			_, _ = w.Write(readFixture(t, "charts_us_iphone_36.html"))
		case strings.Contains(r.URL.Path, "/app/"):
			mu.Lock()
			first := !throttled
			throttled = true
			mu.Unlock()
			if first {
				w.Header().Set("Retry-After", "14")
				w.Header().Set("Cache-Control", "no-store, private, max-age=0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write(readFixture(t, "product_candycrush_us.html"))
		default:
			_, _ = w.Write(readFixture(t, "hints_insta_us.plist"))
		}
	}
}

func readGovernorFile(t *testing.T, path string) governorState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file: %v", err)
	}
	var st governorState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("state file is not JSON: %v\n%s", err, data)
	}
	return st
}

func TestGovernorRemembersAThrottleAcrossProcesses(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state", GovernorStateFile)
	handler := governorHandler(t)
	newProcess := func() (*Client, *fakeApple, *captureRecorder) {
		return newTestClient(t, handler, func(o *Options) { o.StatePath = state; o.NoCache = true })
	}

	// Process 1: Apple throttles the product page.
	c1, fake1, _ := newProcess()
	_, err := c1.ProductPage(context.Background(), ProductPageQuery{URL: candyCrushPageURL})
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) || fake1.count() != 1 {
		t.Fatalf("process 1: err = %v, requests = %d", err, fake1.count())
	}
	st := readGovernorFile(t, state)
	hold, ok := st.Holds["apps.apple.com"]
	if !ok || hold.Status != http.StatusTooManyRequests || hold.RetryAfter != 14 || hold.URL != candyCrushPageURL {
		t.Fatalf("state file holds = %+v", st.Holds)
	}
	if d := hold.Until.Sub(hold.At); d != ThrottleHold {
		t.Fatalf("hold lasts %v, want max(Retry-After 14s, %v)", d, ThrottleHold)
	}

	// Process 2 (a later invocation): the hold refuses the page request
	// without sending it, and names the original throttle.
	c2, fake2, _ := newProcess()
	_, err = c2.ProductPage(context.Background(), ProductPageQuery{URL: candyCrushPageURL})
	if !errors.As(err, &rl) {
		t.Fatalf("process 2: want *cliutil.RateLimitError, got %T %v", err, err)
	}
	if fake2.count() != 0 || c2.Requests() != 0 {
		t.Fatalf("process 2 sent %d request(s) to a host Apple throttled %v ago", fake2.count(), time.Since(hold.At))
	}
	for _, want := range []string{"Apple answered HTTP 429 for " + candyCrushPageURL, "Retry-After 14s", "holds every request to apps.apple.com until", "nothing was sent"} {
		if !strings.Contains(rl.Body, want) {
			t.Fatalf("refusal %q lacks %q", rl.Body, want)
		}
	}
	if rl.RetryAfter < ThrottleHold-time.Minute || rl.RetryAfter > ThrottleHold {
		t.Fatalf("refusal RetryAfter = %v, want the hold's remaining time", rl.RetryAfter)
	}
	if c2.Tripped() == nil {
		t.Fatal("a refused request must stop the rest of the run too")
	}
	if h, ok := c2.HoldFor("apps.apple.com"); !ok || !h.Until.Equal(hold.Until) {
		t.Fatalf("HoldFor = %+v %v", h, ok)
	}

	// The hold is per host: a later process may still use the Lookup API.
	c3, fake3, _ := newProcess()
	if _, err := c3.Lookup(context.Background(), LookupQuery{IDs: []string{"553834731"}}); err != nil || fake3.count() != 1 {
		t.Fatalf("lookup during an apps.apple.com hold: err = %v, requests = %d", err, fake3.count())
	}

	// Once the hold has passed, pages go out again.
	st = readGovernorFile(t, state)
	h := st.Holds["apps.apple.com"]
	h.Until = time.Now().Add(-time.Second)
	st.Holds["apps.apple.com"] = h
	data, _ := json.Marshal(st)
	if err := os.WriteFile(state, data, 0o600); err != nil {
		t.Fatal(err)
	}
	c4, fake4, _ := newProcess()
	page, err := c4.ProductPage(context.Background(), ProductPageQuery{URL: candyCrushPageURL})
	if err != nil || fake4.count() != 1 || page.AdamID != "553834731" {
		t.Fatalf("after the hold: err = %v, requests = %d", err, fake4.count())
	}
}

func TestGovernorPacesPagesAcrossProcesses(t *testing.T) {
	state := filepath.Join(t.TempDir(), GovernorStateFile)
	sends := &sendTimes{}
	newProcess := func() *Client {
		c, _, _ := newTestClient(t, governorHandler(t), func(o *Options) {
			o.StatePath = state
			o.NoCache = true
			o.Jitter = -1
			o.Rates = map[RateClass]float64{RatePages: 4} // 250 ms apart
			sends.base = o.HTTPClient.Transport
			o.HTTPClient = &http.Client{Transport: sends}
		})
		return c
	}
	for i := 0; i < 3; i++ {
		if _, err := newProcess().ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil {
			t.Fatal(err)
		}
	}
	at := sends.all()
	if len(at) != 3 {
		t.Fatalf("requests = %d, want 3", len(at))
	}
	for i := 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < 240*time.Millisecond {
			t.Fatalf("process %d sent its first page %v after the previous process's page; want >= the 250ms class interval across processes", i+1, gap)
		}
	}
	st := readGovernorFile(t, state)
	if last := st.LastSend[RatePages]; last.Before(at[2].Add(-50 * time.Millisecond)) {
		t.Fatalf("state lastSend[%s] = %v, want the last send slot (~%v)", RatePages, last, at[2])
	}
}

func TestGovernorDefaultPagePacingIsTwentySeconds(t *testing.T) {
	c, err := New(Options{Country: "us", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := c.LimiterRate(RatePages); r != 1.0/20 {
		t.Fatalf("default page rate = %v/s, want one request per 20 s", r)
	}
}

func TestGovernorRefusesASlotPastTheDeadline(t *testing.T) {
	state := filepath.Join(t.TempDir(), GovernorStateFile)
	opts := func(o *Options) {
		o.StatePath = state
		o.NoCache = true
		o.Jitter = -1
		o.Rates = map[RateClass]float64{RatePages: 0.1} // 10 s apart
	}
	c1, fake1, _ := newTestClient(t, governorHandler(t), opts)
	if _, err := c1.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil || fake1.count() != 1 {
		t.Fatalf("first page: %v", err)
	}
	before := readGovernorFile(t, state).LastSend[RatePages]

	c2, fake2, _ := newTestClient(t, governorHandler(t), opts)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, err := c2.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	var pe *PacingError
	if !errors.As(err, &pe) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want *PacingError wrapping DeadlineExceeded, got %T %v", err, err)
	}
	if took := time.Since(start); took > 500*time.Millisecond {
		t.Fatalf("refusal took %v: the client slept toward the deadline instead of failing fast", took)
	}
	if fake2.count() != 0 || pe.Host != "apps.apple.com" || pe.Wait < 8*time.Second {
		t.Fatalf("requests = %d, error = %+v", fake2.count(), pe)
	}
	if !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("message = %q", err)
	}
	if after := readGovernorFile(t, state).LastSend[RatePages]; !after.Equal(before) {
		t.Fatalf("a refused request reserved a slot: lastSend %v -> %v", before, after)
	}
	if c2.Tripped() != nil {
		t.Fatal("a pacing refusal is not a throttle and must not trip the governor")
	}
}

func TestGovernorRefusesASlotPastWaitUntil(t *testing.T) {
	state := filepath.Join(t.TempDir(), GovernorStateFile)
	paced := func(o *Options) {
		o.StatePath = state
		o.NoCache = true
		o.Jitter = -1
		o.Rates = map[RateClass]float64{RatePages: 0.1} // 10 s apart
	}
	c1, fake1, _ := newTestClient(t, governorHandler(t), paced)
	if _, err := c1.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil || fake1.count() != 1 {
		t.Fatalf("first page: %v", err)
	}
	before := readGovernorFile(t, state).LastSend[RatePages]

	// No ctx deadline: WaitUntil alone must refuse the 10 s wait.
	c2, fake2, _ := newTestClient(t, governorHandler(t), func(o *Options) {
		paced(o)
		o.WaitUntil = time.Now().Add(time.Second)
	})
	start := time.Now()
	_, err := c2.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
	var pe *PacingError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PacingError, got %T %v", err, err)
	}
	if took := time.Since(start); took > 500*time.Millisecond {
		t.Fatalf("refusal took %v: the client slept toward WaitUntil instead of failing fast", took)
	}
	if fake2.count() != 0 || pe.Wait < 8*time.Second || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("requests = %d, error = %v", fake2.count(), err)
	}
	if after := readGovernorFile(t, state).LastSend[RatePages]; !after.Equal(before) {
		t.Fatalf("a refused request reserved a slot: lastSend %v -> %v", before, after)
	}

	// A free slot inside WaitUntil is sent as usual.
	c3, fake3, _ := newTestClient(t, governorHandler(t), func(o *Options) {
		o.StatePath = filepath.Join(t.TempDir(), GovernorStateFile)
		o.NoCache = true
		o.WaitUntil = time.Now().Add(time.Minute)
	})
	if _, err := c3.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil || fake3.count() != 1 {
		t.Fatalf("a free slot inside WaitUntil: %v (requests %d)", err, fake3.count())
	}
}

func TestMaxRateSpacesRequestsOfEveryClass(t *testing.T) {
	sends := &sendTimes{}
	c, _, _ := newTestClient(t, governorHandler(t), func(o *Options) {
		o.NoCache = true
		o.Jitter = -1
		o.MaxRate = 5 // 200 ms between any two requests
		sends.base = o.HTTPClient.Transport
		o.HTTPClient = &http.Client{Transport: sends}
	})
	ctx := context.Background()
	if _, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Hints(ctx, "us", "insta"); err != nil {
		t.Fatal(err)
	}
	at := sends.all()
	if len(at) != 3 {
		t.Fatalf("requests = %d", len(at))
	}
	for i := 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < 190*time.Millisecond {
			t.Fatalf("request %d went %v after the previous one (another class); --rate-limit 5 means >= 200ms between any two", i+1, gap)
		}
	}
}

func TestPacingWaitIsAnnounced(t *testing.T) {
	prev := waitNoticeAfter
	waitNoticeAfter = 100 * time.Millisecond
	t.Cleanup(func() { waitNoticeAfter = prev })
	var mu sync.Mutex
	var notes []string
	c, _, _ := newTestClient(t, governorHandler(t), func(o *Options) {
		o.NoCache = true
		o.Jitter = -1
		o.Rates = map[RateClass]float64{RatePages: 4}
		o.Notice = func(s string) { mu.Lock(); notes = append(notes, s); mu.Unlock() }
	})
	for i := 0; i < 2; i++ {
		if _, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notes) != 1 || !strings.Contains(notes[0], "apps.apple.com") || !strings.Contains(notes[0], "across runs") {
		t.Fatalf("notices = %q, want one for the second page's wait", notes)
	}
}

func TestGovernorStateFileProblemsNeverBlockARequest(t *testing.T) {
	dir := t.TempDir()
	t.Run("damaged file is an empty state and is rewritten", func(t *testing.T) {
		state := filepath.Join(dir, "damaged", GovernorStateFile)
		_ = os.MkdirAll(filepath.Dir(state), 0o700)
		if err := os.WriteFile(state, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		c, fake, _ := newTestClient(t, governorHandler(t), func(o *Options) { o.StatePath = state; o.NoCache = true })
		if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"1"}}); err != nil || fake.count() != 1 {
			t.Fatalf("err = %v requests = %d", err, fake.count())
		}
		if st := readGovernorFile(t, state); st.LastSend[RateITunes].IsZero() || st.Version != governorStateVersion {
			t.Fatalf("state not rewritten: %+v", st)
		}
	})
	t.Run("hold from a skewed clock is ignored", func(t *testing.T) {
		state := filepath.Join(dir, "skewed", GovernorStateFile)
		_ = os.MkdirAll(filepath.Dir(state), 0o700)
		now := time.Now()
		st := governorState{Version: governorStateVersion, Holds: map[string]HostHold{
			"itunes.apple.com": {Host: "itunes.apple.com", At: now.Add(72 * time.Hour), Until: now.Add(72*time.Hour + ThrottleHold), Status: 429},
		}}
		data, _ := json.Marshal(st)
		_ = os.WriteFile(state, data, 0o600)
		c, fake, _ := newTestClient(t, governorHandler(t), func(o *Options) { o.StatePath = state; o.NoCache = true })
		if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"1"}}); err != nil || fake.count() != 1 {
			t.Fatalf("a hold 3 days ahead blocked the request: err = %v requests = %d", err, fake.count())
		}
	})
	t.Run("unwritable state directory falls back to memory", func(t *testing.T) {
		blocker := filepath.Join(dir, "a-file")
		_ = os.WriteFile(blocker, []byte("x"), 0o600)
		state := filepath.Join(blocker, GovernorStateFile)
		c, fake, _ := newTestClient(t, governorHandler(t), func(o *Options) { o.StatePath = state; o.NoCache = true })
		for i := 0; i < 2; i++ {
			if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"1"}}); err != nil {
				t.Fatal(err)
			}
		}
		if fake.count() != 2 {
			t.Fatalf("requests = %d", fake.count())
		}
	})
}

func TestHoldKeepsApplesFullRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		v    string
		want time.Duration
	}{
		{"1800", 30 * time.Minute},
		{"14", 14 * time.Second},
		{"999999999999", maxHoldAhead},
		{now.Add(30 * time.Minute).Format(http.TimeFormat), 30 * time.Minute},
		{"soon", 0},
		{"-5", 0},
		{"", 0},
	} {
		h := http.Header{}
		if tc.v != "" {
			h.Set("Retry-After", tc.v)
		}
		if got := HoldRetryAfter(h, now); got != tc.want {
			t.Errorf("Retry-After %q -> %v, want %v", tc.v, got, tc.want)
		}
	}

	// cliutil.RetryAfter caps a retry sleep at a minute; a hold must not.
	state := filepath.Join(t.TempDir(), GovernorStateFile)
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Retry-After", "1800")
		w.WriteHeader(http.StatusTooManyRequests)
	}, func(o *Options) {
		o.StatePath = state
		o.NoCache = true
	})
	_, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
	rl, ok := AsRateLimit(err)
	if !ok || !strings.Contains(rl.Body, "Retry-After 30m0s") {
		t.Fatalf("throttle = %v", err)
	}
	if h, ok := c.HoldFor("apps.apple.com"); !ok || time.Until(h.Until) < 29*time.Minute || h.RetryAfter != 1800 {
		t.Fatalf("hold = %+v %v, want about 30 minutes with Retry-After 1800", h, ok)
	}
}

// A slot reserved before another process was throttled must not be used:
// the hold is re-read after the wait, before anything is sent.
func TestGovernorRechecksHoldsAfterTheWait(t *testing.T) {
	state := filepath.Join(t.TempDir(), GovernorStateFile)
	opts := func(o *Options) {
		o.StatePath = state
		o.NoCache = true
		o.Jitter = -1
		o.Rates = map[RateClass]float64{RatePages: 2} // 500 ms apart
	}
	c1, fake1, _ := newTestClient(t, governorHandler(t), opts)
	if _, err := c1.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil || fake1.count() != 1 {
		t.Fatalf("first page: %v", err)
	}
	c2, fake2, _ := newTestClient(t, governorHandler(t), opts)
	done := make(chan error, 1)
	go func() {
		_, err := c2.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
		done <- err
	}()
	time.Sleep(150 * time.Millisecond)
	held, _ := url.Parse("https://apps.apple.com/us/iphone/charts/36")
	c1.RecordThrottle(held, http.StatusTooManyRequests, 0)
	err := <-done
	if _, ok := AsRateLimit(err); !ok {
		t.Fatalf("a slot reserved before another process's 429 was still used: err = %v", err)
	}
	if fake2.count() != 0 {
		t.Fatalf("process 2 sent %d requests to a held host", fake2.count())
	}
}

// The single 5xx retry that finds no send slot in time reports the 5xx, not
// a pacing refusal (exit 5, not 7).
func TestA5xxRetryWithNoSlotReportsThe5xx(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}, func(o *Options) {
		o.NoCache = true
		o.Jitter = -1
		o.RetryDelay = 10 * time.Millisecond
		o.Rates = map[RateClass]float64{RatePages: 0.2} // 5 s apart
		o.WaitUntil = time.Now().Add(time.Second)
	})
	_, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusServiceUnavailable {
		t.Fatalf("want the 503, got %T %v", err, err)
	}
	if fake.count() != 1 {
		t.Fatalf("requests = %d, want 1 (the retry had no slot)", fake.count())
	}
}

// A slot the run must wait for has to leave sendReserve before the deadline.
func TestGovernorLeavesTimeForTheAnswer(t *testing.T) {
	c, fake, _ := newTestClient(t, governorHandler(t), func(o *Options) {
		o.NoCache = true
		o.Jitter = -1
		o.Rates = map[RateClass]float64{RatePages: 1}
	})
	if _, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	var pe *PacingError
	if !errors.As(err, &pe) || fake.count() != 1 {
		t.Fatalf("got %v with %d requests; want a pacing refusal and no second send", err, fake.count())
	}
}
