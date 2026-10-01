// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package trivago

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
)

func TestReadHandshakeBodyRejectsOversizedResponse(t *testing.T) {
	_, err := readHandshakeBody(io.NopCloser(strings.NewReader(strings.Repeat("x", 1<<20+1))))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized handshake error = %v", err)
	}
}

func TestParseMaybeSSE_PlainJSON(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)
	got := parseMaybeSSE(in)
	if string(got) != string(in) {
		t.Fatalf("plain JSON should pass through unchanged; got %q", got)
	}
}

func TestParseMaybeSSE_SingleEvent(t *testing.T) {
	in := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n")
	got := parseMaybeSSE(in)
	want := `{"jsonrpc":"2.0","id":1,"result":{}}`
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseMaybeSSE_LastEventWins(t *testing.T) {
	// Multiple events; the terminal "message" event carries the payload.
	in := []byte("event: ping\ndata: {}\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"k\":\"v\"}}\n\n")
	got := parseMaybeSSE(in)
	want := `{"jsonrpc":"2.0","id":2,"result":{"k":"v"}}`
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWaitForSlot_EnforcesAdaptiveLimiter(t *testing.T) {
	c := &Client{Limiter: cliutil.NewAdaptiveLimiter(20)} // 50ms spacing
	ctx := context.Background()
	if err := c.waitForSlot(ctx); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	start := time.Now()
	if err := c.waitForSlot(ctx); err != nil {
		t.Fatalf("second wait: %v", err)
	}
	elapsed := time.Since(start)
	// Allow a bit of timer slack.
	if elapsed < 40*time.Millisecond {
		t.Fatalf("second wait should have blocked ~50ms; elapsed=%v", elapsed)
	}
}

func TestWaitForSlot_DisabledWhenNil(t *testing.T) {
	c := &Client{Limiter: nil}
	ctx := context.Background()
	start := time.Now()
	_ = c.waitForSlot(ctx)
	_ = c.waitForSlot(ctx)
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("nil Limiter should not block; elapsed=%v", elapsed)
	}
}

func TestWaitForSlot_RespectsContext(t *testing.T) {
	c := &Client{Limiter: cliutil.NewAdaptiveLimiter(2)} // 500ms spacing
	bg := context.Background()
	if err := c.waitForSlot(bg); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	ctx, cancel := context.WithTimeout(bg, 10*time.Millisecond)
	defer cancel()
	err := c.waitForSlot(ctx)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestTruncate(t *testing.T) {
	short := []byte("hello")
	if got := truncate(short); got != "hello" {
		t.Errorf("short input should pass through; got %q", got)
	}
	big := make([]byte, 600)
	for i := range big {
		big[i] = 'a'
	}
	got := truncate(big)
	if len(got) != 512+3 || got[512:] != "..." {
		t.Errorf("big input should be truncated to 512 + '...'; len=%d tail=%q", len(got), got[max(0, len(got)-3):])
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient()
	if c.HTTPClient == nil {
		t.Error("HTTPClient should be set")
	}
	if c.Endpoint != DefaultEndpoint {
		t.Errorf("Endpoint = %q, want %q", c.Endpoint, DefaultEndpoint)
	}
	if c.Limiter == nil {
		t.Error("Limiter should be initialized")
	}
	if got := c.Limiter.Rate(); got != DefaultRatePerSec {
		t.Errorf("Limiter rate = %v, want %v", got, DefaultRatePerSec)
	}
}

// TestCallTool_NilLimiter_NoPanic locks in the contract that callTool's
// limiter signalling paths (OnRateLimit on 429, OnSuccess on a clean
// 2xx) are nil-safe. waitForSlot already exercises the nil path; this
// covers the symmetric assumption for the other two limiter call sites
// so a future refactor that drops a guard panics here, not in prod.
func TestCallTool_NilLimiter_NoPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`))
		case "tools/call":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient: srv.Client(),
		Endpoint:   srv.URL,
		Limiter:    nil,
	}
	if _, err := c.callTool(context.Background(), "noop", map[string]any{}); err != nil {
		t.Fatalf("callTool with nil Limiter returned error: %v", err)
	}
}

func TestCallToolRetriesAfterTransientInitializedNotificationFailure(t *testing.T) {
	var mu sync.Mutex
	var initializeCount, notificationCount, cleanupCount, toolCount int
	cleanupDone := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			cleanupCount++
			mu.Unlock()
			if got := r.Header.Get("Mcp-Session-Id"); got != "session-1" {
				t.Errorf("cleanup session = %q, want session-1", got)
			}
			w.WriteHeader(http.StatusNoContent)
			cleanupDone <- struct{}{}
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch req.Method {
		case "initialize":
			mu.Lock()
			initializeCount++
			attempt := initializeCount
			mu.Unlock()
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("session-%d", attempt))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`, req.ID)
		case "notifications/initialized":
			mu.Lock()
			notificationCount++
			attempt := notificationCount
			mu.Unlock()
			if got, want := r.Header.Get("Mcp-Session-Id"), fmt.Sprintf("session-%d", attempt); got != want {
				t.Errorf("notification session = %q, want %q", got, want)
			}
			if attempt == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("temporary outage"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			mu.Lock()
			toolCount++
			mu.Unlock()
			if got := r.Header.Get("Mcp-Session-Id"); got != "session-2" {
				t.Errorf("tool session = %q, want session-2", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"ok":true}}`, req.ID)
		default:
			t.Errorf("unexpected method %q", req.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
	if _, err := c.callTool(context.Background(), "noop", map[string]any{}); err == nil ||
		!strings.Contains(err.Error(), "initialized notification HTTP 503") {
		t.Fatalf("first call error = %v, want transient notification failure", err)
	}
	if c.sessionID != "" || c.initialized {
		t.Fatalf("failed handshake retained session=%q initialized=%t", c.sessionID, c.initialized)
	}
	if _, err := c.callTool(context.Background(), "noop", map[string]any{}); err != nil {
		t.Fatalf("second call did not recover: %v", err)
	}
	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("failed session was not cleaned up")
	}

	mu.Lock()
	defer mu.Unlock()
	if initializeCount != 2 || notificationCount != 2 || cleanupCount != 1 || toolCount != 1 {
		t.Fatalf("requests initialize=%d notification=%d cleanup=%d tool=%d, want 2/2/1/1",
			initializeCount, notificationCount, cleanupCount, toolCount)
	}
}

func TestCallToolRejectsNotificationRPCErrorAndCleansSession(t *testing.T) {
	var mu sync.Mutex
	var cleanupCount, toolCount int
	cleanupDone := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			cleanupCount++
			mu.Unlock()
			if got := r.Header.Get("Mcp-Session-Id"); got != "rejected-session" {
				t.Errorf("cleanup session = %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
			cleanupDone <- struct{}{}
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "rejected-session")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`, req.ID)
		case "notifications/initialized":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"notification rejected"}}`)
		case "tools/call":
			mu.Lock()
			toolCount++
			mu.Unlock()
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
	_, err := c.callTool(context.Background(), "noop", nil)
	if err == nil || !strings.Contains(err.Error(), "notification rejected") {
		t.Fatalf("callTool error = %v, want notification error", err)
	}
	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("rejected session was not cleaned up")
	}
	mu.Lock()
	defer mu.Unlock()
	if c.initialized || c.sessionID != "" || cleanupCount != 1 || toolCount != 0 {
		t.Fatalf("failed handshake published session or skipped cleanup: initialized=%t cleanup=%d tools=%d", c.initialized, cleanupCount, toolCount)
	}
}

func TestEnsureInitBoundsSlowSessionCleanup(t *testing.T) {
	cleanupStarted := make(chan struct{}, 1)
	releaseCleanup := make(chan struct{})
	var notifications atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			cleanupStarted <- struct{}{}
			<-releaseCleanup
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("session-%d", req.ID))
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`, req.ID)
		case "notifications/initialized":
			if notifications.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"ok":true}}`, req.ID)
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()
	defer close(releaseCleanup)

	c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
	start := time.Now()
	if _, err := c.callTool(context.Background(), "noop", nil); err == nil {
		t.Fatal("first handshake unexpectedly succeeded")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("failed handshake waited %v for cleanup", elapsed)
	}
	select {
	case <-cleanupStarted:
	case <-time.After(time.Second):
		t.Fatal("cleanup request did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.callTool(ctx, "noop", nil); err != nil {
		t.Fatalf("retry blocked behind cleanup: %v", err)
	}
}

func TestEnsureInitWaitingCallerCanCancel(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		switch req.Method {
		case "initialize":
			close(started)
			<-release
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`, req.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
	firstDone := make(chan error, 1)
	go func() { firstDone <- c.ensureInit(context.Background()) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	begin := time.Now()
	err := c.ensureInit(ctx)
	if err != context.DeadlineExceeded || time.Since(begin) > time.Second {
		t.Fatalf("waiting caller error = %v after %v, want prompt cancellation", err, time.Since(begin))
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first handshake: %v", err)
	}
}

func TestEnsureInitBoundsRPCErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32603,"message":%q}}`, req.ID, strings.Repeat("x", 10000))
	}))
	defer srv.Close()
	c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
	err := c.ensureInit(context.Background())
	if err == nil || len(err.Error()) > 600 {
		t.Fatalf("initialize error length = %d, want a bounded message", len(fmt.Sprint(err)))
	}
}

func TestCallToolRejectsInvalidInitializeResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantError  string
	}{
		{
			name:       "non-2xx response",
			statusCode: http.StatusBadGateway,
			body:       "upstream unavailable",
			wantError:  "initialize HTTP 502",
		},
		{
			name:       "malformed JSON",
			statusCode: http.StatusOK,
			body:       `{`,
			wantError:  "decode initialize response",
		},
		{
			name:       "JSON-RPC error",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"initialization failed"}}`,
			wantError:  "initialization failed",
		},
		{
			name:       "missing result",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1}`,
			wantError:  "has no result",
		},
		{
			name:       "scalar result",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1,"result":"garbage"}`,
			wantError:  "invalid initialize result",
		},
		{
			name:       "array result",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1,"result":[]}`,
			wantError:  "invalid initialize result",
		},
		{
			name:       "missing required fields",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1,"result":{}}`,
			wantError:  "missing protocolVersion or serverInfo",
		},
		{
			name:       "non-object capabilities",
			statusCode: http.StatusOK,
			body:       `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":[],"serverInfo":{"name":"test","version":"1"}}}`,
			wantError:  "capabilities must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.Header().Set("Mcp-Session-Id", "invalid-session")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := &Client{HTTPClient: srv.Client(), Endpoint: srv.URL}
			_, err := c.callTool(context.Background(), "noop", map[string]any{})
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("callTool error = %v, want substring %q", err, tt.wantError)
			}
			if requestCount != 1 {
				t.Fatalf("invalid initialize response advanced protocol with %d requests", requestCount)
			}
			if c.sessionID != "" {
				t.Fatalf("invalid initialize response retained session %q", c.sessionID)
			}
		})
	}
}

func TestParseReviewCount(t *testing.T) {
	cases := map[string]int{
		"":          0,
		"42":        42,
		"25,429":    25429,
		"1.563":     1563,
		"8,000":     8000,
		"0":         0,
		"no digits": 0,
		"1,234,567": 1234567,
	}
	for in, want := range cases {
		if got := ParseReviewCount(in); got != want {
			t.Errorf("ParseReviewCount(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestReviewCount_UnmarshalStringAndNumber locks in the tolerant custom
// unmarshaler: Trivago currently returns review_count as a localized
// string ("25,429") but has returned a bare number in the past. Both must
// decode without failing the whole accommodation list.
func TestReviewCount_UnmarshalStringAndNumber(t *testing.T) {
	var byString, byNumber ReviewCount
	if err := json.Unmarshal([]byte(`"25,429"`), &byString); err != nil {
		t.Fatalf("string review_count unmarshal: %v", err)
	}
	if string(byString) != "25,429" {
		t.Fatalf("string got %q", string(byString))
	}
	if err := json.Unmarshal([]byte(`25429`), &byNumber); err != nil {
		t.Fatalf("number review_count unmarshal: %v", err)
	}
	if string(byNumber) != "25429" {
		t.Fatalf("number got %q", string(byNumber))
	}
}

// TestReviewCount_UnmarshalNull treats JSON null as a missing count (empty
// ReviewCount, ParseReviewCount 0). Erroring would fail the whole
// accommodations list when any hotel omits reviews.
func TestReviewCount_UnmarshalNull(t *testing.T) {
	var r ReviewCount
	if err := json.Unmarshal([]byte("null"), &r); err != nil {
		t.Fatalf("null review_count unmarshal: %v", err)
	}
	if r != "" {
		t.Fatalf("null got %q, want empty ReviewCount", r)
	}
	if got := ParseReviewCount(string(r)); got != 0 {
		t.Fatalf("ParseReviewCount of null-decoded value = %d, want 0", got)
	}
}

func TestReviewCount_UnmarshalUnsupportedType(t *testing.T) {
	for _, raw := range []string{"true", "{}", "[]"} {
		var r ReviewCount
		if err := json.Unmarshal([]byte(raw), &r); err == nil {
			t.Errorf("unmarshal %s: expected error, got nil (value %q)", raw, r)
		}
	}
}

// TestDecodeAccommodations_BookingURLFallback confirms that when the API
// omits booking_url (as it does today — only accommodation_url is
// returned), decode backfills BookingURL from URL so booking links keep
// working downstream.
func TestDecodeAccommodations_BookingURLFallback(t *testing.T) {
	raw := json.RawMessage(`{"structuredContent":{"accommodations":[{"accommodation_id":"x1","accommodation_name":"A Hotel","accommodation_url":"https://trivago/u","review_count":"25,429"}]}}`)
	accs, err := decodeAccommodations(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(accs) != 1 {
		t.Fatalf("got %d accommodations", len(accs))
	}
	if accs[0].BookingURL != "https://trivago/u" {
		t.Fatalf("BookingURL = %q, want accommodation_url fallback", accs[0].BookingURL)
	}
	if ParseReviewCount(string(accs[0].ReviewCount)) != 25429 {
		t.Fatalf("review count = %v, want 25429", accs[0].ReviewCount)
	}
}
