// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Table tests for the unsubscribe engine's pure machinery: the one-click
// classification ladder, the registrable-domain approximation and
// alignment, Authentication-Results / DKIM checks, the public-unicast SSRF
// guard, and the pinned-dial transport.

package cli

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
)

// gmailPassAuth is a realistic Gmail Authentication-Results value.
const gmailPassAuth = "mx.google.com; dkim=pass header.i=@letters.example; spf=pass; dmarc=pass (p=REJECT sp=REJECT dis=NONE) header.from=letters.example"

func TestClassifyUnsubSender_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		listUnsub   string
		post        string
		authResults string
		wantClass   string
		wantURL     string
	}{
		{
			name:        "verified one-click",
			listUnsub:   "<https://letters.example/u/1>, <mailto:u@letters.example>",
			post:        "List-Unsubscribe=One-Click",
			authResults: gmailPassAuth,
			wantClass:   classUnsubOneClick,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "wrong List-Unsubscribe-Post value",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "List-Unsubscribe=OneClick",
			authResults: gmailPassAuth,
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "missing post header",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "",
			authResults: gmailPassAuth,
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "http-not-https URL",
			listUnsub:   "<http://letters.example/u/1>",
			post:        "List-Unsubscribe=One-Click",
			authResults: gmailPassAuth,
			wantClass:   classUnsubUnusable,
		},
		{
			name:        "mailto-only",
			listUnsub:   "<mailto:unsubscribe@letters.example?subject=unsub>",
			post:        "",
			authResults: gmailPassAuth,
			wantClass:   classUnsubMailtoOnly,
		},
		{
			name:        "duplicate URL entries downgrade as ambiguity",
			listUnsub:   "<https://letters.example/u/1>, <https://letters.example/u/2>",
			post:        "List-Unsubscribe=One-Click",
			authResults: gmailPassAuth,
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "dmarc missing",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "List-Unsubscribe=One-Click",
			authResults: "mx.google.com; dkim=pass header.i=@letters.example; spf=pass",
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "non-Gmail authserv-id",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "List-Unsubscribe=One-Click",
			authResults: "mx.forwarder.example; dmarc=pass header.from=letters.example",
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "identical duplicate URLs are still ambiguity",
			listUnsub:   "<https://letters.example/u/1>, <https://letters.example/u/1>",
			post:        "List-Unsubscribe=One-Click",
			authResults: gmailPassAuth,
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "case-insensitive trimmed post value still verifies",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "  list-unsubscribe=one-click  ",
			authResults: gmailPassAuth,
			wantClass:   classUnsubOneClick,
			wantURL:     "https://letters.example/u/1",
		},
		{
			name:        "http plus mailto surfaces as mailto-only",
			listUnsub:   "<http://letters.example/u/1>, <mailto:u@letters.example>",
			post:        "List-Unsubscribe=One-Click",
			authResults: gmailPassAuth,
			wantClass:   classUnsubMailtoOnly,
		},
		{
			name:        "empty authentication-results",
			listUnsub:   "<https://letters.example/u/1>",
			post:        "List-Unsubscribe=One-Click",
			authResults: "",
			wantClass:   classUnsubPlainURL,
			wantURL:     "https://letters.example/u/1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyUnsubSender(tc.listUnsub, tc.post, tc.authResults)
			if got.Class != tc.wantClass {
				t.Fatalf("class = %q (reason %q), want %q", got.Class, got.Reason, tc.wantClass)
			}
			if got.URL != tc.wantURL {
				t.Fatalf("url = %q, want %q", got.URL, tc.wantURL)
			}
			if got.Class != classUnsubOneClick && got.Reason == "" {
				t.Fatalf("non-verified class %q must carry a reason", got.Class)
			}
		})
	}
}

func TestRegistrableDomainAndAlignment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		want string
	}{
		{"example.com", "example.com"},
		{"mail.example.com", "example.com"},
		{"a.b.c.example.com", "example.com"},
		{"example.co.uk", "example.co.uk"},
		{"news.example.co.uk", "example.co.uk"},
		{"deep.sub.example.co.uk", "example.co.uk"},
		{"example.com.au", "example.com.au"},
		{"track.example.com.au", "example.com.au"},
		{"EXAMPLE.Com.", "example.com"},
		{"localhost", "localhost"},
		{"co.uk", "co.uk"}, // bare suffix: nothing deeper to take
		{"192.168.0.1", "192.168.0.1"},
	}
	for _, tc := range cases {
		if got := registrableDomain(tc.host); got != tc.want {
			t.Fatalf("registrableDomain(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}

	// Alignment pairs (the audit's aligned bool).
	align := []struct {
		urlHost, fromEmail string
		want               bool
	}{
		{"u.letters.example.com", "news@letters.example.com", true},
		{"unsubscribe.example.co.uk", "deals@example.co.uk", true},
		{"tracker.esp-vendor.com", "news@letters.example.com", false},
		{"example.co.uk", "user@other.co.uk", false},
	}
	for _, tc := range align {
		got := registrableDomain(tc.urlHost) == registrableDomain(emailDomain(tc.fromEmail))
		if got != tc.want {
			t.Fatalf("aligned(%q, %q) = %v, want %v", tc.urlHost, tc.fromEmail, got, tc.want)
		}
	}
}

func TestAuthResultsIsGmailDMARCPass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    string
		want bool
	}{
		{gmailPassAuth, true},
		{"mx.google.com; dmarc=pass", true},
		{"mx.google.com; dkim=pass; spf=pass", false},                  // no dmarc
		{"mx.other.example; dmarc=pass", false},                        // foreign authserv-id
		{"", false},                                                    // absent
		{"mx.google.com; dmarc=fail (p=NONE)", false},                  // explicit fail
		{"mx.google.com; comment dmarc=passing; spf=pass", false},      // dmarc=passing is not a pass token
		{"MX.GOOGLE.COM; DMARC=PASS header.from=example.com", true},    // case-insensitive
		{"mx.google.com 1; spf=pass; dmarc=pass (p=QUARANTINE)", true}, // authserv-id with version
	}
	for _, tc := range cases {
		if got := authResultsIsGmailDMARCPass(tc.v); got != tc.want {
			t.Fatalf("authResultsIsGmailDMARCPass(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

func TestDKIMHelpers(t *testing.T) {
	t.Parallel()
	sig := "v=1; a=rsa-sha256; d=letters.example; s=sel; h=From:Subject:List-Unsubscribe:List-Unsubscribe-Post; bh=abc; b=def"
	if !dkimCoversUnsubHeaders(sig) {
		t.Fatal("h= covering both one-click headers must pass")
	}
	if dkimDomain(sig) != "letters.example" {
		t.Fatalf("dkimDomain = %q", dkimDomain(sig))
	}
	// Missing list-unsubscribe-post in h=.
	sigNoPost := "v=1; d=letters.example; h=From:Subject:List-Unsubscribe; b=x"
	if dkimCoversUnsubHeaders(sigNoPost) {
		t.Fatal("h= without list-unsubscribe-post must fail")
	}
	// Folding whitespace inside h= (unfolded by Gmail into spaces).
	sigFolded := "v=1; d=letters.example; h=From : Subject : List-Unsubscribe : List-Unsubscribe-Post; b=x"
	if !dkimCoversUnsubHeaders(sigFolded) {
		t.Fatal("whitespace inside h= names must not break coverage")
	}
	if parseDKIMTags("")["h"] != "" {
		t.Fatal("empty signature parses to empty tags")
	}
}

func TestIPClassAndVetPublicUnicast(t *testing.T) {
	t.Parallel()
	rejects := map[string]string{
		"10.0.0.1":        "private",
		"172.16.5.9":      "private",
		"192.168.1.1":     "private",
		"127.0.0.1":       "loopback",
		"169.254.10.10":   "link-local",
		"100.64.0.7":      "carrier-grade NAT",
		"0.0.0.0":         "unspecified",
		"224.0.0.5":       "multicast",
		"240.0.0.1":       "reserved",
		"255.255.255.255": "broadcast",
		"::1":             "loopback",
		"fe80::1":         "link-local",
		"fc00::1":         "private",
		"ff02::1":         "multicast",
		"::":              "unspecified",
		"::ffff:10.0.0.1": "private", // IPv4-mapped form must not evade the guard
	}
	for s, wantSub := range rejects {
		ip := net.ParseIP(s)
		class := ipClass(ip)
		if class == "" {
			t.Fatalf("ipClass(%s) = public, want a %q-class rejection", s, wantSub)
		}
		if !strings.Contains(class, wantSub) {
			t.Fatalf("ipClass(%s) = %q, want it to mention %q", s, class, wantSub)
		}
		if err := vetPublicUnicast("host.example", []net.IP{ip}); err == nil {
			t.Fatalf("vetPublicUnicast must reject %s", s)
		}
	}
	accepts := []string{"93.184.216.34", "8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "2001:4860:4860::8888"}
	for _, s := range accepts {
		if class := ipClass(net.ParseIP(s)); class != "" {
			t.Fatalf("ipClass(%s) = %q, want public", s, class)
		}
	}
	if err := vetPublicUnicast("host.example", accepts2IPs(accepts)); err != nil {
		t.Fatalf("all-public answer must pass: %v", err)
	}
	// One private record poisons an otherwise-public answer.
	mixed := append(accepts2IPs(accepts[:1]), net.ParseIP("10.0.0.1"))
	if err := vetPublicUnicast("host.example", mixed); err == nil {
		t.Fatal("mixed public+private answer must be rejected")
	}
	if err := vetPublicUnicast("host.example", nil); err == nil {
		t.Fatal("empty answer must be rejected")
	}
}

func accepts2IPs(ss []string) []net.IP {
	out := make([]net.IP, 0, len(ss))
	for _, s := range ss {
		out = append(out, net.ParseIP(s))
	}
	return out
}

// TestPinnedDialTransport proves the DialContext dials the RESOLVED IP,
// not whatever the URL host would re-resolve to (DNS-rebinding defense),
// while preserving the port.
func TestPinnedDialTransport(t *testing.T) {
	t.Parallel()
	var dialed []string
	fake := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, context.Canceled // never actually connect
	}
	tr := pinnedDialTransport(net.ParseIP("93.184.216.34"), fake)
	if tr.Proxy != nil {
		t.Fatal("pinned transport must disable proxies")
	}
	_, err := tr.DialContext(context.Background(), "tcp", "unsub.example:443")
	if err == nil {
		t.Fatal("fake dialer should error")
	}
	if len(dialed) != 1 || dialed[0] != "93.184.216.34:443" {
		t.Fatalf("dialed %v, want exactly [93.184.216.34:443]", dialed)
	}
	// Non-default port is preserved.
	_, _ = tr.DialContext(context.Background(), "tcp", "unsub.example:8443")
	if dialed[1] != "93.184.216.34:8443" {
		t.Fatalf("port not preserved: %v", dialed)
	}
}

// TestPerformOneClickPost_SchemeAndGuardShortCircuits covers the pre-wire
// refusals that need no server at all.
func TestPerformOneClickPost_SchemeAndGuardShortCircuits(t *testing.T) {
	old := unsubLookupIP
	t.Cleanup(func() { unsubLookupIP = old })

	// http scheme refused before any resolution.
	unsubLookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
		t.Fatal("resolver must not be called for a non-https URL")
		return nil, nil
	}
	if res := performOneClickPost(context.Background(), "http://u.example/x"); res.SkipReason != "url-not-https" {
		t.Fatalf("http URL skip = %+v", res)
	}
	if res := performOneClickPost(context.Background(), "https://user:pw@u.example/x"); res.SkipReason != "url-carries-userinfo" {
		t.Fatalf("userinfo skip = %+v", res)
	}

	// Private resolution refused with ssrf-guard.
	unsubLookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	res := performOneClickPost(context.Background(), "https://u.example/x")
	if res.SkipReason != "ssrf-guard" || res.Err == nil {
		t.Fatalf("loopback resolution = %+v, want ssrf-guard skip", res)
	}
}

// silence the unused warning if http is otherwise unreferenced here.
var _ = http.MethodPost
