// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Destination-alignment tests: the unsubscribe URL host must share the
// sender's registrable domain unless the operator passes
// --allow-third-party ('unsub plan' surfaces the same verdict under
// third_party_hosts).

package cli

import (
	"strings"
	"testing"
)

func TestUnsubHostAligned(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		sender string
		want   bool
	}{
		{"exact host", "https://shop.example/unsub", "news@shop.example", true},
		{"subdomain of sender", "https://mail.shop.example/u/1?id=2", "news@shop.example", true},
		{"sender on subdomain, same org", "https://links.shop.example/x", "no-reply@updates.shop.example", true},
		{"esp third party", "https://u1.esp-mailer.example/x", "news@shop.example", false},
		{"two-level public suffix stays distinct", "https://a.shop.com.ar/u", "n@shop2.com.ar", false},
		{"two-level public suffix same org", "https://links.shop.com.ar/u", "n@shop.com.ar", true},
		{"ip literal", "https://203.0.113.9/u", "news@shop.example", false},
		{"ipv6 literal", "https://[2001:db8::1]/u", "news@shop.example", false},
		{"unparsable url", "https://%zz/u", "news@shop.example", false},
		{"malformed sender", "https://shop.example/u", "not-an-address", false},
		{"omitted two-level suffix stays distinct", "https://victim.co.id/u", "n@attacker.co.id", false},
		{"private-registry suffix stays distinct", "https://victim.github.io/u", "n@attacker.github.io", false},
		{"punycode equals unicode spelling", "https://xn--mnchen-3ya.example/u", "n@münchen.example", true},
		{"trailing dot on host", "https://mail.shop.example./u", "news@shop.example", true},
		{"port ignored", "https://shop.example:8443/u", "news@shop.example", true},
		{"userinfo does not spoof the host", "https://evil.example@shop.example/u", "news@shop.example", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unsubHostAligned(c.url, c.sender); got != c.want {
				t.Fatalf("unsubHostAligned(%q, %q) = %v, want %v", c.url, c.sender, got, c.want)
			}
		})
	}
}

func TestCheckUnsubRunConditions_DestinationAlignment(t *testing.T) {
	mkLive := func(url string) unsubLiveHeaders {
		return unsubLiveHeaders{
			listUnsub:     []string{"<" + url + ">"},
			listUnsubPost: []string{"List-Unsubscribe=One-Click"},
			dkimSigs: []string{"v=1; a=rsa-sha256; d=shop.example; s=sel; " +
				"h=From:To:Subject:List-Unsubscribe:List-Unsubscribe-Post; bh=abc; b=def"},
		}
	}
	stored := "mx.google.com; dkim=pass header.d=shop.example; dmarc=pass (p=REJECT)"
	sender := "news@shop.example"

	aligned := "https://mail.shop.example/u/1"
	if r := checkUnsubRunConditions(aligned, sender, stored, mkLive(aligned), false); r != "" {
		t.Fatalf("aligned host unexpectedly skipped: %q", r)
	}

	third := "https://u1.esp-mailer.example/x"
	r := checkUnsubRunConditions(third, sender, stored, mkLive(third), false)
	if r == "" || !strings.Contains(r, "third-party") {
		t.Fatalf("third-party host without opt-in: want a third-party skip reason, got %q", r)
	}

	if r := checkUnsubRunConditions(third, sender, stored, mkLive(third), true); r != "" {
		t.Fatalf("third-party host with --allow-third-party unexpectedly skipped: %q", r)
	}
}
