// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import "testing"

// Label items of an unknown kind are a parse gap, not "no labels".
func TestPrivacyParseGapIsNotNone(t *testing.T) {
	sm := jsonObject{
		"privacyHeader": map[string]any{"items": []any{}},
		"privacyTypes":  map[string]any{"items": []any{map[string]any{"$kind": "PrivacyTypeCard"}}},
	}
	if pv := parsePrivacy(sm); pv.NoneShown || !pv.Unreadable {
		t.Fatalf("unknown label cards: NoneShown=%v Unreadable=%v, want false/true", pv.NoneShown, pv.Unreadable)
	}
	if pv := parsePrivacy(jsonObject{"privacyHeader": map[string]any{"items": []any{}}}); !pv.NoneShown || pv.Unreadable {
		t.Fatalf("an empty section: NoneShown=%v Unreadable=%v, want true/false", pv.NoneShown, pv.Unreadable)
	}
}

// A skipped chart card leaves a gap: later apps keep their page ordinals.
func TestRankedAppsKeepOrdinals(t *testing.T) {
	s := ChartSegment{
		Lockups:      []AppLockup{{AdamID: "a", Position: 1}, {AdamID: "b", Position: 2}, {AdamID: "d", Position: 4}},
		RemainingIDs: []string{"e", "f"},
	}
	want := []RankedApp{{"a", 1}, {"b", 2}, {"d", 4}, {"e", 5}, {"f", 6}}
	got := s.RankedApps()
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
