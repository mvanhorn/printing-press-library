// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/config"
)

func TestAllAdvancersHasNoRatio(t *testing.T) {
	p := breadthOf("d", []ipRow{scr("A", "IT", 1, 1, 1, 1, 2, nil), scr("B", "IT", 1, 1, 1, 1, 1, nil)})
	if p.Advancers != 2 || p.AdRatio != nil {
		t.Fatalf("all-advancer day must have no ratio (not 0): %+v", p)
	}
}

func TestExprMissingFieldAndBooleans(t *testing.T) {
	row := map[string]any{"rs": 75.0, "sector": "IT"}
	cases := map[string]bool{
		"not missing":            false, // missing stays unknown, never true
		"not (missing > 5)":      false,
		"not (rs > 80)":          true,
		"(rs > 1) == (rs > 2)":   true,
		"(rs > 1) != (rs > 2)":   false,
		"(rs > 1) == (rs > 100)": false,
		"(rs > 1) != (rs > 100)": true,
		"sector != 5":            false,
		"rs < 1e2":               true,
		"rs > 1E1":               true,
		"rs > 1.5e+1":            true,
	}
	for src, want := range cases {
		e, err := parseIPExpr(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if got := e.matches(row); got != want {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
}

func TestValidSymbol(t *testing.T) {
	for _, ok := range []string{"TCS", "M&M", "BAJAJ-AUTO", "360ONE", "GVT&D"} {
		if !ipValidSymbol(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", ".", "..", "../x", "TCS?x=1", "A B", strings.Repeat("A", 21), "TCS/evil"} {
		if ipValidSymbol(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestSinceCapped(t *testing.T) {
	if _, err := parseSinceDays("9999999999999999999d"); err == nil {
		t.Error("overflowing --since must fail")
	}
	if _, err := parseSinceDays("5000d"); err == nil {
		t.Error("over-cap --since must fail")
	}
	if n, err := parseSinceDays("3650d"); err != nil || n != 3650 {
		t.Errorf("3650d = %d, %v", n, err)
	}
}

func TestValidateSnapshotRejectsGarbage(t *testing.T) {
	good := `[{"symbol":"A","date":"2026-10-16"}]`
	if asof, err := ipValidateSnapshot(ipKindScreener, []byte(good)); err != nil || asof != "2026-10-16" {
		t.Fatalf("good screener: %q %v", asof, err)
	}
	for _, bad := range []string{`{"error":"session expired"}`, `[]`, `[{"symbol":"A"}]`, `null`} {
		if _, err := ipValidateSnapshot(ipKindScreener, []byte(bad)); err == nil {
			t.Errorf("screener %s must be rejected", bad)
		}
	}
	if _, err := ipValidateSnapshot(ipKindLive, []byte(`{"payload":{"rows":[],"sessionDate":"2026-10-16"}}`)); err == nil {
		t.Error("live with no rows must be rejected")
	}
	if _, err := ipValidateSnapshot(ipKindOverview, []byte(`{"error":"x"}`)); err == nil {
		t.Error("overview without asOf must be rejected")
	}
	if asof, err := ipValidateSnapshot(ipKindOverview, []byte(`{"asOf":"2026-10-16","signals":[]}`)); err != nil || asof != "2026-10-16" {
		t.Errorf("good overview: %q %v", asof, err)
	}
}

func TestNormalizeSessionCookieEdgeCases(t *testing.T) {
	cases := map[string]string{
		"abc==":                           "__Secure-authjs.session-token=abc==",
		"COOKIE: a=1; b=2":                "a=1; b=2",
		"line1\nline2":                    "__Secure-authjs.session-token=line1 line2",
		"__Secure-authjs.session-token=z": "__Secure-authjs.session-token=z",
	}
	for in, want := range cases {
		if got := normalizeSessionCookie(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

// The env-var/set-token session must reach the cookie jar even though its name
// contains a dot, and only for an https investorpaisa.com base URL.
func TestSeedSessionCookie(t *testing.T) {
	mk := func(base, cred string) *client.Client {
		jar, _ := cookiejar.New(nil)
		return &client.Client{BaseURL: base, HTTPClient: &http.Client{Jar: jar}, Config: &config.Config{AccessToken: cred}}
	}
	c := mk("https://investorpaisa.com", "__Secure-authjs.session-token=tok123")
	if err := ipSeedSessionCookie(c); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://investorpaisa.com/api/live")
	got := c.HTTPClient.Jar.Cookies(u)
	if len(got) != 1 || got[0].Name != "__Secure-authjs.session-token" || got[0].Value != "tok123" {
		t.Fatalf("cookies = %+v", got)
	}
	// bare value is accepted too
	c2 := mk("https://investorpaisa.com", "bare456")
	_ = ipSeedSessionCookie(c2)
	if g := c2.HTTPClient.Jar.Cookies(u); len(g) != 1 || g[0].Value != "bare456" {
		t.Fatalf("bare cookies = %+v", g)
	}
	// never to another host or plain http
	for _, base := range []string{"http://investorpaisa.com", "https://evil.example.com", "http://127.0.0.1:9"} {
		cc := mk(base, "__Secure-authjs.session-token=tok123")
		_ = ipSeedSessionCookie(cc)
		bu, _ := url.Parse(base)
		if g := cc.HTTPClient.Jar.Cookies(bu); len(g) != 0 {
			t.Errorf("cookie leaked to %s: %+v", base, g)
		}
	}
	// nothing to seed
	if err := ipSeedSessionCookie(mk("https://investorpaisa.com", "")); err != nil {
		t.Fatal(err)
	}
}
