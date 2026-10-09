// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetStatuses(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		switch r.URL.Path {
		case "/ok":
			if r.Header.Get("User-Agent") != userAgent {
				t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/flaky":
			if n%2 == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case "/limited":
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/limited-once":
			if n == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(5*time.Second, 0)
	ctx := context.Background()
	cases := []struct {
		path      string
		wantErr   bool
		rateLimit bool
		status    int
	}{
		{"/ok", false, false, 0},
		{"/flaky", false, false, 0},
		{"/limited", true, true, 0},
		{"/limited-once", false, false, 0},
		{"/missing", true, false, 404},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			atomic.StoreInt32(&calls, 0)
			f, err := c.get(ctx, "queue-times", c.std, srv.URL+tc.path, true)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v", err)
			}
			if IsRateLimit(err) != tc.rateLimit {
				t.Fatalf("IsRateLimit = %v for %v", IsRateLimit(err), err)
			}
			if tc.status != 0 && ErrorStatus(err) != tc.status {
				t.Fatalf("want status %d, got %v", tc.status, err)
			}
			if tc.wantErr && ErrorURL(err) != srv.URL+tc.path {
				t.Fatalf("error URL = %q", ErrorURL(err))
			}
			if !tc.wantErr && (len(f.Body) == 0 || f.FetchedAt.IsZero()) {
				t.Fatalf("fetch = %+v", f)
			}
		})
	}
}

func TestTDRMonthURL(t *testing.T) {
	cases := map[[2]string]string{
		{"202611", "ja"}: "https://www.tokyodisneyresort.jp/ticket/index/202611/#search-date",
		{"202612", "en"}: "https://www.tokyodisneyresort.jp/en/ticket/index/202612/#search-date",
	}
	for in, want := range cases {
		if got := TDRMonthURL(in[0], in[1]); got != want {
			t.Errorf("TDRMonthURL(%v) = %s", in, got)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("a\nb"), "a b"},
		{&FetchError{Err: errors.New("GET x: HTTP 404")}, "GET x: HTTP 404"},
	}
	for _, tc := range cases {
		if got := Describe(tc.err); got != tc.want || strings.Contains(got, "\n") {
			t.Errorf("Describe(%v) = %q", tc.err, got)
		}
	}
}

func TestChromeClientRejectsSelfSignedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := New(5*time.Second, 0)
	hc, err := c.chromeClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.get(context.Background(), "tdr", hc, srv.URL+"/", false); err == nil {
		t.Fatal("self-signed certificate was accepted; TLS verification must stay on")
	}
	if _, err := c.get(context.Background(), "themeparks", c.std, srv.URL+"/", true); err == nil {
		t.Fatal("std client accepted a self-signed certificate")
	}
}

func TestRedirectsStayOnHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer other.Close()
	var self *httptest.Server
	self = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, other.URL+"/x", http.StatusFound)
		case "/local":
			http.Redirect(w, r, self.URL+"/final", http.StatusFound)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer self.Close()
	c := New(5*time.Second, 0)
	if _, err := c.get(context.Background(), "queue-times", c.std, self.URL+"/local", true); err != nil {
		t.Fatalf("same-host redirect failed: %v", err)
	}
	_, err := c.get(context.Background(), "queue-times", c.std, self.URL+"/away", true)
	if !errors.Is(err, errCrossHostRedirect) {
		t.Fatalf("cross-host redirect: got %v", err)
	}
}

func TestSleepCtxCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepCtx(ctx, time.Minute); err == nil || time.Since(start) > time.Second {
		t.Fatalf("sleepCtx did not stop on a cancelled context: %v", err)
	}
}
