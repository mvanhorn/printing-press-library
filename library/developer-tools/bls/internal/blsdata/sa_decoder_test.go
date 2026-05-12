package blsdata

import "testing"

func TestDecodeAdjustment(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"CUUR0000SA0", "nsa"},
		{"CUSR0000SA0", "seasonal"},
		{"CWUR0000SA0", "nsa"},
		{"CWSR0000SA0", "seasonal"},
		{"LNS14000000", "seasonal"},
		{"LNU14000000", "nsa"},
		{"JTS000000000000000JOL", "seasonal"},
		{"JTU000000000000000JOL", "nsa"},
		{"CES0000000001", "seasonal"},
		{"CEU0000000001", "nsa"},
		{"CIU1010000000000A", "nsa"},
		{"CIS1010000000000I", "seasonal"},
		{"WPSFD4", "seasonal"},
		{"WPUFD4", "nsa"},
		{"APU000074714", "nsa"},
		{"", "unknown"},
		{"X", "unknown"},
		{"XYZ", "unknown"},
	}
	for _, tc := range cases {
		if got := DecodeAdjustment(tc.id); got != tc.want {
			t.Errorf("DecodeAdjustment(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestCompareAdjustmentIDs(t *testing.T) {
	cases := []struct {
		id      string
		wantSA  string
		wantNSA string
	}{
		{"CUUR0000SA0", "CUSR0000SA0", "CUUR0000SA0"},
		{"CUSR0000SA0", "CUSR0000SA0", "CUUR0000SA0"},
		{"LNS14000000", "LNS14000000", "LNU14000000"},
		{"LNU14000000", "LNS14000000", "LNU14000000"},
		{"JTU000000000000000JOL", "JTS000000000000000JOL", "JTU000000000000000JOL"},
		{"APU000074714", "", ""}, // AP is universally NSA — toggle returns empty
		{"", "", ""},
	}
	for _, tc := range cases {
		sa, nsa := CompareAdjustmentIDs(tc.id)
		if sa != tc.wantSA || nsa != tc.wantNSA {
			t.Errorf("CompareAdjustmentIDs(%q) = (%q, %q), want (%q, %q)", tc.id, sa, nsa, tc.wantSA, tc.wantNSA)
		}
	}
}

func TestFootnotesAndCatalog(t *testing.T) {
	if DecodeFootnote("P") == "" {
		t.Error("DecodeFootnote('P') returned empty")
	}
	if DecodeFootnote("XX") != "" {
		t.Error("DecodeFootnote('XX') should return empty for unknown code")
	}
	if FindByID("LNS14000000") == nil {
		t.Error("FindByID('LNS14000000') returned nil; expected entry")
	}
	if FindByID("BOGUS") != nil {
		t.Error("FindByID('BOGUS') should return nil")
	}
	if len(MacroSnapshot()) != 15 {
		t.Errorf("MacroSnapshot() has %d entries; want 15", len(MacroSnapshot()))
	}
}
