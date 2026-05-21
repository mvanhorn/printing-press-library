// PATCH(crawl-stats: SAPISIDHASH unit test — deterministic, no network).

package client

import (
	"strings"
	"testing"
	"time"
)

func TestSAPISIDHashDeterministic(t *testing.T) {
	// Fixed inputs produce a fixed output. The reference value below was
	// computed by hand from the canonical formula (ts=1716300000,
	// sapisid="abcd1234", origin="https://search.google.com"):
	//   payload  = "1716300000 abcd1234 https://search.google.com"
	//   sha1hex  = sha1(payload).hexdigest()
	//   result   = "SAPISIDHASH 1716300000_" + sha1hex
	got := SAPISIDHash("abcd1234", "https://search.google.com", time.Unix(1716300000, 0))
	want := "SAPISIDHASH 1716300000_30892117018aea8395c713cc7094b86ef72e54d7"
	if got != want {
		t.Errorf("SAPISIDHash mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestSAPISIDHashShape(t *testing.T) {
	// Two consecutive calls with the same inputs and different timestamps
	// must differ in the ts prefix. Header format must always carry the
	// "SAPISIDHASH " prefix and an underscore-joined ts/hash pair.
	now1 := time.Unix(1700000000, 0)
	now2 := time.Unix(1700000060, 0)
	h1 := SAPISIDHash("sapisid-val", "https://search.google.com", now1)
	h2 := SAPISIDHash("sapisid-val", "https://search.google.com", now2)
	if h1 == h2 {
		t.Fatal("hashes must differ when ts differs")
	}
	for _, h := range []string{h1, h2} {
		if !strings.HasPrefix(h, "SAPISIDHASH ") {
			t.Errorf("missing SAPISIDHASH prefix: %s", h)
		}
		body := strings.TrimPrefix(h, "SAPISIDHASH ")
		parts := strings.SplitN(body, "_", 2)
		if len(parts) != 2 {
			t.Errorf("missing underscore separator: %s", h)
		}
		if len(parts[1]) != 40 {
			t.Errorf("hash component must be 40-char sha1 hex, got %d: %s", len(parts[1]), parts[1])
		}
	}
}
