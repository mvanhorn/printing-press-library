// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDedupURLs(t *testing.T) {
	got := dedupURLs([]string{"https://a.com", "a.com/", "http://A.com", "https://b.com"})
	if len(got) != 2 {
		t.Fatalf("dedupURLs = %v, want 2 distinct (a.com, b.com)", got)
	}
	if got[0] != "https://a.com" || got[1] != "https://b.com" {
		t.Fatalf("dedupURLs preserved wrong order/first: %v", got)
	}
}
