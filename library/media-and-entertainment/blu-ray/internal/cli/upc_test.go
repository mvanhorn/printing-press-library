package cli

// PATCH: Regression coverage for tolerant UPC import token filtering.

import "testing"

func TestParseUPCListSkipsCSVHeadersAndNonUPCTokens(t *testing.T) {
	t.Parallel()

	got := parseUPCList("upc,title,notes\n012345678901,DVD")
	want := []string{"012345678901"}
	if len(got) != len(want) {
		t.Fatalf("parseUPCList len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseUPCList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
