// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// mockChrome stands up an httptest.Server that mimics Chrome's CDP HTTP
// endpoints. tabsBody is returned verbatim from /json — pass invalid JSON
// to exercise the malformed-response path.
func mockChrome(t *testing.T, tabsBody string, versionStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		if versionStatus != http.StatusOK {
			w.WriteHeader(versionStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"Browser":"Chrome/120","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/x"}`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, tabsBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// pinClientToServer makes a CDPClient that talks to srv even when the
// caller passes a fixed port — we override the HTTPClient with a transport
// that rewrites all 127.0.0.1:<port> URLs to point at srv. This lets us
// exercise DiscoverPort/ListTabs without needing a real CDP listener on a
// specific kernel port.
func pinClientToServer(srv *httptest.Server) *CDPClient {
	target, _ := url.Parse(srv.URL)
	hc := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Redirect every dial to the httptest server's host:port.
				var d net.Dialer
				return d.DialContext(ctx, network, target.Host)
			},
		},
	}
	return &CDPClient{HTTPClient: hc}
}

func TestCDPFilterSuperhumanTabs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []Tab
		want int
	}{
		{
			name: "happy path single tab",
			in: []Tab{
				{Type: "page", URL: "https://mail.superhuman.com/user2@example.com/threads"},
			},
			want: 1,
		},
		{
			name: "multiple superhuman tabs",
			in: []Tab{
				{Type: "page", URL: "https://mail.superhuman.com/user2@example.com/threads"},
				{Type: "page", URL: "https://mail.superhuman.com/user@example.com/threads"},
			},
			want: 2,
		},
		{
			name: "no superhuman tab",
			in: []Tab{
				{Type: "page", URL: "https://example.com/"},
				{Type: "page", URL: "https://news.ycombinator.com/"},
			},
			want: 0,
		},
		{
			name: "ignores non-page types",
			in: []Tab{
				{Type: "service_worker", URL: "https://mail.superhuman.com/sw.js"},
				{Type: "background_page", URL: "https://mail.superhuman.com/bg"},
			},
			want: 0,
		},
		{
			name: "empty input",
			in:   nil,
			want: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FilterSuperhumanTabs(tc.in)
			if len(got) != tc.want {
				t.Fatalf("FilterSuperhumanTabs len = %d, want %d (got %#v)", len(got), tc.want, got)
			}
			if got == nil {
				t.Fatalf("FilterSuperhumanTabs returned nil; want non-nil slice")
			}
		})
	}
}

func TestCDPListTabsHappyPath(t *testing.T) {
	t.Parallel()
	body := `[
		{"id":"A","type":"page","url":"https://mail.superhuman.com/user2@example.com/threads","title":"Superhuman","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/A"},
		{"id":"B","type":"page","url":"https://example.com/","title":"Example","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/B"}
	]`
	srv := mockChrome(t, body, http.StatusOK)
	c := pinClientToServer(srv)

	tabs, err := c.ListTabs(context.Background(), 9222)
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	if len(tabs) != 2 {
		t.Fatalf("got %d tabs, want 2", len(tabs))
	}

	sh := FilterSuperhumanTabs(tabs)
	if len(sh) != 1 {
		t.Fatalf("FilterSuperhumanTabs got %d, want 1", len(sh))
	}
	if !strings.Contains(sh[0].URL, "mail.superhuman.com") {
		t.Fatalf("filtered tab URL = %q, want mail.superhuman.com", sh[0].URL)
	}
}

func TestCDPListTabsMultipleSuperhuman(t *testing.T) {
	t.Parallel()
	body := `[
		{"id":"A","type":"page","url":"https://mail.superhuman.com/user2@example.com/threads","webSocketDebuggerUrl":"ws://x"},
		{"id":"B","type":"page","url":"https://mail.superhuman.com/user@example.com/threads","webSocketDebuggerUrl":"ws://x"}
	]`
	srv := mockChrome(t, body, http.StatusOK)
	c := pinClientToServer(srv)

	tabs, err := c.ListTabs(context.Background(), 9222)
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	sh := FilterSuperhumanTabs(tabs)
	if len(sh) != 2 {
		t.Fatalf("FilterSuperhumanTabs got %d, want 2 (tabs: %#v)", len(sh), tabs)
	}
}

func TestCDPListTabsNoSuperhuman(t *testing.T) {
	t.Parallel()
	body := `[{"id":"A","type":"page","url":"https://example.com/","webSocketDebuggerUrl":"ws://x"}]`
	srv := mockChrome(t, body, http.StatusOK)
	c := pinClientToServer(srv)

	tabs, err := c.ListTabs(context.Background(), 9222)
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	sh := FilterSuperhumanTabs(tabs)
	if len(sh) != 0 {
		t.Fatalf("got %d superhuman tabs, want 0", len(sh))
	}
}

func TestCDPListTabsMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := mockChrome(t, `not-a-json-array`, http.StatusOK)
	c := pinClientToServer(srv)

	_, err := c.ListTabs(context.Background(), 9222)
	if err == nil {
		t.Fatalf("ListTabs got nil error, want decode wrap")
	}
	if !strings.HasPrefix(err.Error(), "cdp tab list:") {
		t.Fatalf("error %q does not start with 'cdp tab list:'", err.Error())
	}
}

// TestCDPDiscoverPortNoChrome exercises the no-Chrome path: we point the
// client at a known-dead port (127.0.0.1:1 — reserved, nothing listens)
// and set Port explicitly so we don't scan the user's actual machine.
func TestCDPDiscoverPortNoChrome(t *testing.T) {
	t.Parallel()
	// Use a port we control by binding-then-closing — that guarantees no
	// one else has it during this test on darwin/linux CI.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // Now the port is reliably refused.

	c := &CDPClient{
		Port: port,
		HTTPClient: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = c.DiscoverPort(ctx)
	if err == nil {
		t.Fatalf("DiscoverPort got nil error, want ErrChromeNotRunning")
	}
	if !errors.Is(err, ErrChromeNotRunning) {
		t.Fatalf("DiscoverPort err = %v; want errors.Is(..., ErrChromeNotRunning)", err)
	}
	// The user-facing message must include the relaunch hint.
	if !strings.Contains(err.Error(), "--remote-debugging-port=9222") {
		t.Fatalf("error %q missing actionable relaunch instruction", err.Error())
	}
	if !strings.Contains(err.Error(), "auth login --auto-launch-chrome") {
		t.Fatalf("error %q missing auto-launch hint", err.Error())
	}
}

// TestCDPDiscoverPortFound exercises the happy path on the chosen port.
// We override HTTPClient so the GET goes to our httptest server, which
// stands in for Chrome listening on the candidate port.
func TestCDPDiscoverPortFound(t *testing.T) {
	t.Parallel()
	srv := mockChrome(t, `[]`, http.StatusOK)
	c := pinClientToServer(srv)
	c.Port = 9222

	got, err := c.DiscoverPort(context.Background())
	if err != nil {
		t.Fatalf("DiscoverPort: %v", err)
	}
	if got != 9222 {
		t.Fatalf("DiscoverPort = %d, want 9222", got)
	}
}

// TestCDPEvaluateHandshakeFails confirms the WebSocket wrap-error path.
// We point Evaluate at a httptest server that 404s the upgrade — nhooyr
// returns an error from Dial which we must wrap with the "cdp attach:" prefix.
func TestCDPEvaluateHandshakeFails(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	// Convert http://host:port to ws://host:port/devtools/page/X. The path
	// doesn't matter — the server 404s everything.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/devtools/page/X"

	c := &CDPClient{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.Evaluate(ctx, wsURL, "1+1")
	if err == nil {
		t.Fatalf("Evaluate got nil error, want handshake failure")
	}
	if !strings.HasPrefix(err.Error(), "cdp attach:") {
		t.Fatalf("error %q does not start with 'cdp attach:'", err.Error())
	}
}

// TestCDPEvaluateUnreachable confirms the wrap when the target isn't
// listening at all (no httptest, no anything). This complements the 404
// case and locks in the wrap behavior for the most common failure shape
// the user will hit (Chrome quit between ListTabs and Evaluate).
func TestCDPEvaluateUnreachable(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/devtools/page/X", port)
	c := &CDPClient{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.Evaluate(ctx, wsURL, "1+1")
	if err == nil {
		t.Fatalf("Evaluate got nil error, want unreachable failure")
	}
	if !strings.HasPrefix(err.Error(), "cdp attach:") {
		t.Fatalf("error %q does not start with 'cdp attach:'", err.Error())
	}
}
