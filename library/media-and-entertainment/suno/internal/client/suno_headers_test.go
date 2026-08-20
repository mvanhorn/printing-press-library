// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

// captureRT records the request it receives and returns a canned 200.
type captureRT struct{ got *http.Request }

func (c *captureRT) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
}

func newReq(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestSunoRoundTripperInjectsCookieOnStudioHost(t *testing.T) {
	cap := &captureRT{}
	rt := &sunoRoundTripper{base: cap, deviceID: "dev-1", cookieHeader: "a=1; b=2"}

	// studio host -> Cookie injected
	if _, err := rt.RoundTrip(newReq(t, "https://studio-api-prod.suno.com/api/generate/v2-web/")); err != nil {
		t.Fatal(err)
	}
	if got := cap.got.Header.Get("Cookie"); got != "a=1; b=2" {
		t.Fatalf("studio Cookie = %q, want injected", got)
	}

	// other host -> Cookie NOT injected (Clerk/auth must stay clean)
	if _, err := rt.RoundTrip(newReq(t, "https://auth.suno.com/v1/client")); err != nil {
		t.Fatal(err)
	}
	if got := cap.got.Header.Get("Cookie"); got != "" {
		t.Fatalf("non-studio Cookie = %q, want empty", got)
	}
}

func TestSunoRoundTripperDoesNotClobberExistingCookie(t *testing.T) {
	cap := &captureRT{}
	rt := &sunoRoundTripper{base: cap, deviceID: "dev-1", cookieHeader: "a=1"}
	req := newReq(t, "https://studio-api-prod.suno.com/api/generate/v2-web/")
	req.Header.Set("Cookie", "preset=keep")
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := cap.got.Header.Get("Cookie"); got != "preset=keep" {
		t.Fatalf("Cookie = %q, want preset preserved", got)
	}
}

func TestSunoRoundTripperEmptyCookieHeaderNoOp(t *testing.T) {
	cap := &captureRT{}
	rt := &sunoRoundTripper{base: cap, deviceID: "dev-1", cookieHeader: ""}
	if _, err := rt.RoundTrip(newReq(t, "https://studio-api-prod.suno.com/api/generate/v2-web/")); err != nil {
		t.Fatal(err)
	}
	if got := cap.got.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie = %q, want none when cookieHeader empty", got)
	}
}

func TestSunoRoundTripperSetCookieHeaderRoundtrip(t *testing.T) {
	rt := &sunoRoundTripper{base: http.DefaultTransport, deviceID: "d", cookieHeader: "a=1"}
	rt.setCookieHeader("b=2")
	if got := rt.getCookieHeader(); got != "b=2" {
		t.Fatalf("getCookieHeader = %q, want b=2", got)
	}
}

func TestSunoDynamicHeadersBrowserTokenShape(t *testing.T) {
	h := SunoDynamicHeaders("dev-123")

	if h["Device-Id"] != "dev-123" {
		t.Fatalf("Device-Id = %q, want dev-123", h["Device-Id"])
	}
	if h["Origin"] != "https://suno.com" {
		t.Fatalf("Origin = %q", h["Origin"])
	}
	if h["Referer"] != "https://suno.com/" {
		t.Fatalf("Referer = %q", h["Referer"])
	}

	// Browser-Token must be {"token":"<base64>"} decoding to {"timestamp":<number>}.
	var outer struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(h["Browser-Token"]), &outer); err != nil {
		t.Fatalf("Browser-Token is not JSON: %v (%s)", err, h["Browser-Token"])
	}
	if outer.Token == "" {
		t.Fatal("Browser-Token.token is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(outer.Token)
	if err != nil {
		t.Fatalf("token is not standard base64: %v", err)
	}
	var inner struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal(decoded, &inner); err != nil {
		t.Fatalf("decoded token is not {\"timestamp\":...}: %v (%s)", err, decoded)
	}
	if inner.Timestamp <= 0 {
		t.Fatalf("timestamp = %d, want positive ms-since-epoch", inner.Timestamp)
	}

	// Zero-UUID fallback when deviceID is empty.
	if SunoDynamicHeaders("")["Device-Id"] != "00000000-0000-0000-0000-000000000000" {
		t.Fatal("empty deviceID should fall back to zero UUID")
	}
}
