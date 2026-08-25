package client

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/config"
)

func TestSignDNSMadeEasy(t *testing.T) {
	secret := "test-secret-key"
	// Fixed instant so the assertion is deterministic.
	when := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)

	date, sig := signDNSMadeEasy(secret, when)

	wantDate := "Thu, 09 Jul 2026 14:00:00 GMT"
	if date != wantDate {
		t.Fatalf("date = %q, want %q", date, wantDate)
	}

	// The signature MUST be HMAC-SHA1 over the exact date string sent in the
	// header — otherwise the server's recomputation fails.
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(date))
	want := hex.EncodeToString(mac.Sum(nil))
	if sig != want {
		t.Fatalf("sig = %q, want %q", sig, want)
	}
}

func TestSigningTransportSetsAllThreeHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// BaseURL must match the request host or signing is (correctly) skipped.
	cfg := &config.Config{DnsmadeeasyApiKey: "key-123", DnsmadeeasyApiSecret: "secret-abc", BaseURL: srv.URL}
	rt := newSigningTransport(cfg, http.DefaultTransport)

	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if got.Get("x-dnsme-apiKey") != "key-123" {
		t.Errorf("x-dnsme-apiKey = %q, want key-123", got.Get("x-dnsme-apiKey"))
	}
	date := got.Get("x-dnsme-requestDate")
	if date == "" {
		t.Fatal("x-dnsme-requestDate not set")
	}
	// The HMAC header must verify against the date header the server received.
	mac := hmac.New(sha1.New, []byte("secret-abc"))
	_, _ = mac.Write([]byte(date))
	want := hex.EncodeToString(mac.Sum(nil))
	if got.Get("x-dnsme-hmac") != want {
		t.Errorf("x-dnsme-hmac = %q, want %q (HMAC over the sent date)", got.Get("x-dnsme-hmac"), want)
	}
}

func TestSigningTransportSkipsCrossHost(t *testing.T) {
	// A request to a host other than the configured API host must NOT be
	// signed — otherwise a cross-host redirect leaks the API key and a
	// replayable HMAC. The configured API host is a different (unused) host.
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		DnsmadeeasyApiKey:    "key-123",
		DnsmadeeasyApiSecret: "secret-abc",
		BaseURL:              "https://api.dnsmadeeasy.com/V2.0", // different host than srv
	}
	rt := newSigningTransport(cfg, http.DefaultTransport)
	req, _ := http.NewRequest("GET", srv.URL, nil) // request to the OTHER host
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if got.Get("x-dnsme-apiKey") != "" || got.Get("x-dnsme-hmac") != "" || got.Get("x-dnsme-requestDate") != "" {
		t.Errorf("credentials leaked to cross-host request: apiKey=%q hmac=%q date=%q",
			got.Get("x-dnsme-apiKey"), got.Get("x-dnsme-hmac"), got.Get("x-dnsme-requestDate"))
	}
}

func TestSigningTransportPassThroughWithoutCredentials(t *testing.T) {
	var signed bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-dnsme-hmac") != "" {
			signed = true
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &config.Config{} // no credentials
	rt := newSigningTransport(cfg, http.DefaultTransport)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if signed {
		t.Error("request was signed despite missing credentials")
	}
}
