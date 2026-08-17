// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import "testing"

func TestExtractSessionTokens(t *testing.T) {
	html := `window.WIZ_global_data = {"FdrFJe":"-12345","cfb2h":"boq_test","SNlM0e":"csrf-token-abc"};`
	sid, bl, at, err := extractSessionTokens(html)
	if err != nil {
		t.Fatal(err)
	}
	if sid != "-12345" || bl != "boq_test" || at != "csrf-token-abc" {
		t.Fatalf("got sid=%q bl=%q at=%q", sid, bl, at)
	}
}
