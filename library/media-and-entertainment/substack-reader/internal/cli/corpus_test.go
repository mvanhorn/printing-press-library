// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestHostFromURL(t *testing.T) {
	cases := map[string]string{
		"https://creatoreconomy.so/p/some-slug":     "creatoreconomy.so",
		"https://uxmentor.substack.com/p/x":         "uxmentor.substack.com",
		"http://www.userresearchstrategist.com/p/y": "www.userresearchstrategist.com",
		"https://blog.bytebytego.com":               "blog.bytebytego.com",
		"":                                          "",
	}
	for in, want := range cases {
		if got := hostFromURL(in); got != want {
			t.Errorf("hostFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// A custom-domain post carries its real host in canonical_url, not in
// subdomain/custom_domain (verified against peteryang -> creatoreconomy.so).
// This pins that read's canonical-host derivation keys on canonical_url.
func TestParseCorpusPostFallsBackToCanonicalURLHost(t *testing.T) {
	raw := json.RawMessage(`{"title":"t","slug":"s","audience":"only_paid","canonical_url":"https://creatoreconomy.so/p/s"}`)
	p, ok := parseCorpusPost(raw)
	if !ok {
		t.Fatal("parseCorpusPost returned !ok")
	}
	if p.Host != "creatoreconomy.so" {
		t.Errorf("Host = %q, want creatoreconomy.so (from canonical_url fallback)", p.Host)
	}
	if !p.IsPaid() {
		t.Error("expected IsPaid() true for only_paid")
	}
}
