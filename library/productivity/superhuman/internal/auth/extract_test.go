// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// TestExtractParseHappyGoogle exercises the happy Google path through
// Parse alone. We don't need a CDP round-trip to verify the parser
// layer — Parse is the contract we own and the IIFE owes us. Using a
// userId of `user_11SzDPi4sKPTbHQRMQ` lets us verify the userPrefix
// rule against the exact example documented in the plan ("chars 7-10 of
// 11SzDPi4sKPTbHQRMQ -> 4sKP").
func TestExtractParseHappyGoogle(t *testing.T) {
	t.Parallel()
	now := time.Now()
	idTokenExpires := now.Add(time.Hour).UnixMilli()
	raw := json.RawMessage(fmt.Sprintf(`{
		"email": "user2@example.com",
		"idToken": "eyJ.id.token",
		"idTokenExpires": %d,
		"refreshToken": "AMf-rt-1",
		"accessToken": "ya29.access-1",
		"accessTokenExpires": %d,
		"userId": "user_11SzDPi4sKPTbHQRMQ",
		"userPrefix": "4sKP",
		"userExternalId": "user_11SzDPi4sKPTbHQRMQ",
		"deviceId": "dev-abc",
		"provider": "google"
	}`, idTokenExpires, idTokenExpires))

	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Email != "user2@example.com" {
		t.Fatalf("Email = %q, want user2@example.com", got.Email)
	}
	if got.IDToken != "eyJ.id.token" {
		t.Fatalf("IDToken = %q", got.IDToken)
	}
	if got.IDTokenExpires != idTokenExpires {
		t.Fatalf("IDTokenExpires = %d, want %d", got.IDTokenExpires, idTokenExpires)
	}
	if got.RefreshToken != "AMf-rt-1" {
		t.Fatalf("RefreshToken = %q", got.RefreshToken)
	}
	if got.AccessToken != "ya29.access-1" {
		t.Fatalf("AccessToken = %q", got.AccessToken)
	}
	if got.UserID != "user_11SzDPi4sKPTbHQRMQ" {
		t.Fatalf("UserID = %q", got.UserID)
	}
	if got.UserExternalID != "user_11SzDPi4sKPTbHQRMQ" {
		t.Fatalf("UserExternalID = %q", got.UserExternalID)
	}
	if got.UserPrefix != "4sKP" {
		t.Fatalf("UserPrefix = %q, want 4sKP", got.UserPrefix)
	}
	if got.DeviceID != "dev-abc" {
		t.Fatalf("DeviceID = %q", got.DeviceID)
	}
	if got.Provider != "google" {
		t.Fatalf("Provider = %q, want google", got.Provider)
	}
}

// TestExtractParseHappyMicrosoft verifies the Microsoft-account branch
// of the parsed envelope. The IIFE returns provider="microsoft" when
// it found window.MicrosoftAccount; the parser must round-trip that
// without alteration.
func TestExtractParseHappyMicrosoft(t *testing.T) {
	t.Parallel()
	now := time.Now()
	raw := json.RawMessage(fmt.Sprintf(`{
		"email": "user@example.com",
		"idToken": "eyJ.id.token.ms",
		"idTokenExpires": %d,
		"refreshToken": "M.R-rt",
		"accessToken": "EwAo-access",
		"accessTokenExpires": %d,
		"userId": "user_MS123abcXYZdefGHI",
		"userPrefix": "abcX",
		"userExternalId": "user_MS123abcXYZdefGHI",
		"deviceId": "dev-ms",
		"provider": "microsoft"
	}`, now.Add(time.Hour).UnixMilli(), now.Add(time.Hour).UnixMilli()))

	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Provider != "microsoft" {
		t.Fatalf("Provider = %q, want microsoft", got.Provider)
	}
	if got.AccessToken == "" {
		t.Fatalf("AccessToken is empty; want populated")
	}
	if got.RefreshToken == "" {
		t.Fatalf("RefreshToken is empty; want populated")
	}
}

// TestExtractParseMalformedTimestamp verifies the defensive default:
// when the page returns a zero or missing idTokenExpires, Parse fills
// it in with now + 1h. The plan's scenario #5 asserts the field is
// non-zero and within ±10s of now+3600s.
func TestExtractParseMalformedTimestamp(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"email": "user2@example.com",
		"idToken": "eyJ.id",
		"idTokenExpires": 0,
		"refreshToken": "rt",
		"accessToken": "at",
		"accessTokenExpires": 0,
		"userId": "user_11SzDPi4sKPTbHQRMQ",
		"userPrefix": "4sKP",
		"userExternalId": "user_11SzDPi4sKPTbHQRMQ",
		"deviceId": "dev",
		"provider": "google"
	}`)
	before := time.Now()
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	after := time.Now()

	expectedMin := before.Add(time.Hour).Add(-10 * time.Second).UnixMilli()
	expectedMax := after.Add(time.Hour).Add(10 * time.Second).UnixMilli()
	if got.IDTokenExpires < expectedMin || got.IDTokenExpires > expectedMax {
		t.Fatalf("IDTokenExpires = %d, want in [%d, %d] (now+1h ±10s)",
			got.IDTokenExpires, expectedMin, expectedMax)
	}
	if got.AccessTokenExpires != got.IDTokenExpires {
		t.Fatalf("AccessTokenExpires = %d, want = IDTokenExpires = %d",
			got.AccessTokenExpires, got.IDTokenExpires)
	}
}

// TestExtractParseMissingIDToken locks in that a load-bearing missing
// field is surfaced as a clear error, not a silent zero value. Without
// idToken there's no way to call Superhuman, so we'd rather fail loudly
// at the extract boundary than discover it via an HTTP 401 later.
func TestExtractParseMissingIDToken(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"email": "user2@example.com",
		"idToken": "",
		"refreshToken": "rt"
	}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatalf("Parse got nil, want error for missing idToken")
	}
	if !strings.Contains(err.Error(), "idToken") {
		t.Fatalf("error %q does not mention idToken", err.Error())
	}
}

// TestExtractParseEmpty locks in the empty-raw guard. An empty
// json.RawMessage upstream (e.g., result.result.value missing because
// the IIFE returned undefined) must produce a parser error, not a
// zero-valued ExtractedTokens.
func TestExtractParseEmpty(t *testing.T) {
	t.Parallel()
	_, err := Parse(nil)
	if err == nil {
		t.Fatalf("Parse(nil) got nil, want error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error %q does not mention empty", err.Error())
	}
}

// TestExtractIIFEEmbed sanity-checks that the IIFE source is non-empty
// and contains the typed error strings the Go side keys on. If a
// future edit drops these strings, this test will catch the silent
// classification regression.
func TestExtractIIFEEmbed(t *testing.T) {
	t.Parallel()
	src := IIFE()
	if src == "" {
		t.Fatalf("IIFE returned empty source")
	}
	mustContain := []string{
		"page not ready",
		"session expired",
		"GoogleAccount",
		"MicrosoftAccount",
		"getIDTokenAsync",
		"userPrefix",
		"_authData",
		"substring(7, 11)",
	}
	for _, s := range mustContain {
		if !strings.Contains(src, s) {
			t.Fatalf("IIFE source missing %q", s)
		}
	}
}

// TestExtractPickTab covers the routing layer in isolation. We mock the
// Tab list to walk through the email-pin matrix: single tab no email,
// multi tab no email (error), multi tab with email match, multi tab
// with email miss.
func TestExtractPickTab(t *testing.T) {
	t.Parallel()
	tabs := []Tab{
		{URL: "https://mail.superhuman.com/user2@example.com/threads", WebSocketDebuggerURL: "ws://x/a"},
		{URL: "https://mail.superhuman.com/user@example.com/threads", WebSocketDebuggerURL: "ws://x/b"},
	}

	// single-tab no email: pick that tab
	got, err := pickTab(tabs[:1], "")
	if err != nil {
		t.Fatalf("single tab no email: %v", err)
	}
	if got.WebSocketDebuggerURL != "ws://x/a" {
		t.Fatalf("got %q want ws://x/a", got.WebSocketDebuggerURL)
	}

	// multi-tab no email: ErrMultipleAccounts
	_, err = pickTab(tabs, "")
	if !errors.Is(err, ErrMultipleAccounts) {
		t.Fatalf("multi tab no email: err = %v, want ErrMultipleAccounts", err)
	}
	for _, want := range []string{"user2@example.com", "user@example.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("multi tab err %q missing %s", err.Error(), want)
		}
	}

	// multi-tab match
	got, err = pickTab(tabs, "user@example.com")
	if err != nil {
		t.Fatalf("multi tab match: %v", err)
	}
	if got.WebSocketDebuggerURL != "ws://x/b" {
		t.Fatalf("got %q want ws://x/b", got.WebSocketDebuggerURL)
	}

	// multi-tab miss
	_, err = pickTab(tabs, "ghost@nowhere.example")
	if err == nil {
		t.Fatalf("multi tab miss: want error")
	}
	if !strings.Contains(err.Error(), "ghost@nowhere.example") {
		t.Fatalf("multi tab miss err %q missing requested email", err.Error())
	}
}

// TestExtractEmailFromURL pins the URL-parsing contract used by the
// error-message helper. The shape is stable enough to test directly.
func TestExtractEmailFromURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://mail.superhuman.com/user2@example.com/threads":              "user2@example.com",
		"https://mail.superhuman.com/user@example.com/threads/inbox":          "user@example.com",
		"https://mail.superhuman.com/user2@example.com?ref=x":                "user2@example.com",
		"https://mail.superhuman.com/user2@example.com":                      "user2@example.com",
		"https://mail.superhuman.com/user2@example.com#frag":                 "user2@example.com",
		"https://example.com/x":                                               "",
		"https://mail.superhuman.com/":                                        "",
	}
	for in, want := range cases {
		got := emailFromURL(in)
		if got != want {
			t.Fatalf("emailFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------
// CDP-round-trip tests. We stand up an in-process WebSocket server that
// speaks just enough CDP to accept the Runtime.evaluate frame and
// return a canned response. The pattern mirrors mockChrome from
// cdp_test.go but for the WS endpoint instead of the HTTP endpoint.
// ---------------------------------------------------------------------

// mockCDPWS returns an httptest.Server whose handler upgrades to a
// WebSocket and replies to the first Runtime.evaluate frame with a
// frame derived from response (passed verbatim into result.result.value
// when isException is false, or used as the exception text when true).
//
// The server is single-shot: it handles one connection and one frame.
// That's all U2 needs — Evaluate sends one request and reads until it
// gets a matching id.
func mockCDPWS(t *testing.T, value json.RawMessage, isException bool, exceptionText string) (wsURL string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("ws accept: %v", err)
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Logf("ws read: %v", err)
			return
		}
		// Decode just enough to get the request id, since Evaluate
		// hard-codes id=1 but a future caller might bump it.
		var req struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &req)
		if req.ID == 0 {
			req.ID = 1
		}

		var respBytes []byte
		if isException {
			// Match the shape U1's Evaluate decodes: result.result with
			// exceptionDetails populated. The Text field carries the
			// error message we want surfaced.
			respObj := map[string]interface{}{
				"id": req.ID,
				"result": map[string]interface{}{
					"exceptionDetails": map[string]interface{}{
						"text":       exceptionText,
						"lineNumber": 1,
					},
				},
			}
			respBytes, _ = json.Marshal(respObj)
		} else {
			respObj := map[string]interface{}{
				"id": req.ID,
				"result": map[string]interface{}{
					"result": map[string]interface{}{
						"type":  "object",
						"value": json.RawMessage(value),
					},
				},
			}
			respBytes, _ = json.Marshal(respObj)
		}

		if err := ws.Write(ctx, websocket.MessageText, respBytes); err != nil {
			t.Logf("ws write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	// Convert http://host:port to ws://host:port — the path is
	// arbitrary; the test handler accepts any path.
	u, _ := url.Parse(srv.URL)
	return "ws://" + u.Host + "/devtools/page/X"
}

// TestExtractFromTabHappyGoogle wires the WS mock into ExtractFromTab
// and verifies the parsed envelope round-trips through the CDP layer.
// This is the integration boundary between U1 and U2.
func TestExtractFromTabHappyGoogle(t *testing.T) {
	t.Parallel()
	exp := time.Now().Add(time.Hour).UnixMilli()
	value := json.RawMessage(fmt.Sprintf(`{
		"email": "user2@example.com",
		"idToken": "eyJ.id",
		"idTokenExpires": %d,
		"refreshToken": "AMf-rt",
		"accessToken": "ya29.at",
		"accessTokenExpires": %d,
		"userId": "user_11SzDPi4sKPTbHQRMQ",
		"userPrefix": "4sKP",
		"userExternalId": "user_11SzDPi4sKPTbHQRMQ",
		"deviceId": "dev-1",
		"provider": "google"
	}`, exp, exp))
	wsURL := mockCDPWS(t, value, false, "")

	c := &CDPClient{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tab := Tab{ID: "X", Type: "page", URL: "https://mail.superhuman.com/user2@example.com/threads", WebSocketDebuggerURL: wsURL}
	got, err := ExtractFromTab(ctx, c, tab)
	if err != nil {
		t.Fatalf("ExtractFromTab: %v", err)
	}
	if got.Email != "user2@example.com" {
		t.Fatalf("Email = %q", got.Email)
	}
	if got.UserPrefix != "4sKP" {
		t.Fatalf("UserPrefix = %q, want 4sKP", got.UserPrefix)
	}
	if got.Provider != "google" {
		t.Fatalf("Provider = %q, want google", got.Provider)
	}
}

// TestExtractFromTabPageNotReady covers scenario #3: the page is
// missing the credential global, so the IIFE throws "page not ready".
// The CDP layer surfaces it as ErrPageNotReady (per cdp.go), and the
// extract wrapper preserves that sentinel through the wrap.
func TestExtractFromTabPageNotReady(t *testing.T) {
	t.Parallel()
	wsURL := mockCDPWS(t, nil, true, "Uncaught Error: extract: page not ready (credential missing)")

	c := &CDPClient{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tab := Tab{WebSocketDebuggerURL: wsURL}
	_, err := ExtractFromTab(ctx, c, tab)
	if err == nil {
		t.Fatalf("ExtractFromTab: want ErrPageNotReady, got nil")
	}
	if !errors.Is(err, ErrPageNotReady) {
		t.Fatalf("err = %v, want errors.Is(..., ErrPageNotReady)", err)
	}
}

// TestExtractFromTabSessionExpired covers scenario #4: getIDTokenAsync
// throws (Firebase session is gone). The IIFE re-throws with
// "session expired" in the message; the wrapper detects that and
// surfaces ErrSessionExpired instead of the generic ErrPageNotReady.
func TestExtractFromTabSessionExpired(t *testing.T) {
	t.Parallel()
	wsURL := mockCDPWS(t, nil, true, "Uncaught Error: extract: session expired (re-log into Superhuman in Chrome)")

	c := &CDPClient{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tab := Tab{WebSocketDebuggerURL: wsURL}
	_, err := ExtractFromTab(ctx, c, tab)
	if err == nil {
		t.Fatalf("ExtractFromTab: want ErrSessionExpired, got nil")
	}
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want errors.Is(..., ErrSessionExpired)", err)
	}
}

// TestExtractFromTabMissingWSURL locks in the cheap guard that prevents
// a no-op WebSocket dial when a caller hands us a half-populated Tab.
func TestExtractFromTabMissingWSURL(t *testing.T) {
	t.Parallel()
	c := &CDPClient{}
	_, err := ExtractFromTab(context.Background(), c, Tab{})
	if err == nil {
		t.Fatalf("ExtractFromTab(empty tab): want error")
	}
	if !strings.Contains(err.Error(), "webSocketDebuggerUrl") {
		t.Fatalf("err %q does not mention webSocketDebuggerUrl", err.Error())
	}
}

// TestExtractNilClient locks in the guard so a future caller error
// (passing a nil *CDPClient) doesn't manifest as a panic deep in the
// WebSocket layer.
func TestExtractNilClient(t *testing.T) {
	t.Parallel()
	_, err := ExtractFromTab(context.Background(), nil, Tab{WebSocketDebuggerURL: "ws://x"})
	if err == nil {
		t.Fatalf("ExtractFromTab(nil client): want error")
	}
	_, err = Extract(context.Background(), nil, "")
	if err == nil {
		t.Fatalf("Extract(nil client): want error")
	}
}

// quietListener is a tiny helper to confirm port-shaped errors don't
// flake. Kept here rather than reused from cdp_test.go because
// duplication is cheaper than coupling for a 4-line helper.
func quietListener(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// TestExtractDiscoverPortFails exercises the top-level Extract wrapper
// when no Chrome is running. We expect the ErrChromeNotRunning sentinel
// from U1 to propagate through Extract's wrap layer.
func TestExtractDiscoverPortFails(t *testing.T) {
	t.Parallel()
	port := quietListener(t)
	c := &CDPClient{
		Port: port,
		HTTPClient: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Extract(ctx, c, "")
	if err == nil {
		t.Fatalf("Extract: want ErrChromeNotRunning, got nil")
	}
	if !errors.Is(err, ErrChromeNotRunning) {
		t.Fatalf("Extract err = %v, want errors.Is(..., ErrChromeNotRunning)", err)
	}
}
