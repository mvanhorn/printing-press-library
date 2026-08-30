// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package trivago

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
)

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
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
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
