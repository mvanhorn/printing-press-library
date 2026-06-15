// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import (
	"strings"
	"testing"
)

// TestSignedQueryGoldenToken pins the token algorithm against a golden vector.
//
// NOTE ON THE GOLDEN VALUE: the Phase-3 spec quoted a golden of
// e93361f0...0cc9f3, but that value is inconsistent with the spec's OWN signing
// helper code (sha256_hex(hashInput + SALT) over the raw-value key=value join):
// that algorithm produces the value below. This was resolved BEHAVIORALLY
// against the LIVE site: a GET to /queue-results with exactly these params and
// the token below returns {"status":1,...} (accepted, search enqueued) with no
// "Invalid token." error, whereas a wrong token returns status -2. The live
// server is the source of truth, so the golden is pinned to the live-validated
// value. See the report for the raw live-acceptance evidence.
func TestSignedQueryGoldenToken(t *testing.T) {
	params := []KV{
		{"make", "honda"},
		{"model", "civic"},
		{"zip", "33701"},
		{"radius", "200"},
		{"originalradius", "200"},
		{"sort", "best_match"},
		{"sites", "cs"},
		{"deduplicationSites", "te|hem|cs|cv|cm|eb|ot|extended|fbm|st"},
		{"rpp", "50"},
	}
	q := SignedQuery(params)
	// Live-validated token (see note above).
	const want = "69ad96eb9ea6aa0cf3eadb99c70e7f327386069901f06bdfae0598e1d4b97ace"
	idx := strings.Index(q, "&token=")
	if idx < 0 {
		t.Fatalf("no token in query: %q", q)
	}
	got := q[idx+len("&token="):]
	if got != want {
		t.Fatalf("token mismatch:\n got  %s\n want %s\n query %s", got, want, q)
	}
}

func TestSignedQueryEncoding(t *testing.T) {
	// Pipes must be %7C and spaces %20 in the URL, but RAW in the hash input.
	q := SignedQuery([]KV{{"deduplicationSites", "a|b"}, {"k", "x y"}})
	if !strings.Contains(q, "deduplicationSites=a%7Cb") {
		t.Errorf("pipe not encoded in URL: %q", q)
	}
	if !strings.Contains(q, "k=x%20y") {
		t.Errorf("space not encoded as %%20: %q", q)
	}
}

func TestEncodeURIComponent(t *testing.T) {
	cases := map[string]string{
		"a b":   "a%20b",
		"a|b":   "a%7Cb",
		"honda": "honda",
		"[1,2]": "%5B1%2C2%5D",
	}
	for in, want := range cases {
		if got := encodeURIComponent(in); got != want {
			t.Errorf("encodeURIComponent(%q) = %q, want %q", in, got, want)
		}
	}
}
