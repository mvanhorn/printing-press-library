package chrome

import (
	"testing"
	"time"
)

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"github.com": "github.com",
		"100%done":   `100\%done`,
		"a_b":        `a\_b`,
		`back\slash`: `back\\slash`,
		"%_\\":       `\%\_\\`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChromeEpochRoundTrip(t *testing.T) {
	cases := []time.Time{
		time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC),
		time.Date(2010, 6, 15, 12, 30, 0, 0, time.UTC),
	}
	for _, tc := range cases {
		raw := timeToChromeMicros(tc)
		got := chromeMicrosToTime(raw)
		if !got.Equal(tc) {
			t.Fatalf("round-trip mismatch: got %v want %v", got, tc)
		}
	}
}

func TestTransitionLabel(t *testing.T) {
	cases := []struct {
		raw  int64
		want string
	}{
		{1, "typed"},
		{8, "reload"},
		{9, "keyword"},
		{255, "unknown"},
	}
	for _, tc := range cases {
		if got := transitionLabel(tc.raw); got != tc.want {
			t.Fatalf("transitionLabel(%d)=%q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestOriginClassifierConsistency(t *testing.T) {
	guidToDevice := map[string]string{"guid-sync": "device-1"}
	cases := []struct {
		srcVal     int64
		guid       string
		wantOrigin string
		wantID     string
		wantKind   string
	}{
		{srcVal: -1, wantOrigin: "this", wantID: "this", wantKind: "this"},
		{srcVal: 1, wantOrigin: "this", wantID: "this", wantKind: "this"},
		{srcVal: 0, guid: "guid-sync", wantOrigin: "device-1", wantID: "device-1", wantKind: "synced"},
		{srcVal: 0, guid: "unknown", wantOrigin: "synced", wantID: "synced", wantKind: "synced"},
		{srcVal: 2, wantOrigin: "extension", wantID: "extension", wantKind: "extension"},
		{srcVal: 3, wantOrigin: "imported", wantID: "imported", wantKind: "imported"},
		{srcVal: 4, wantOrigin: "imported", wantID: "imported", wantKind: "imported"},
		{srcVal: 5, wantOrigin: "imported", wantID: "imported", wantKind: "imported"},
	}
	for _, tc := range cases {
		if got := visitOrigin(tc.srcVal, tc.guid, guidToDevice); got != tc.wantOrigin {
			t.Fatalf("visitOrigin(%d,%q)=%q want %q", tc.srcVal, tc.guid, got, tc.wantOrigin)
		}
		gotID, gotKind := deviceBucket(tc.srcVal, tc.guid, guidToDevice)
		if gotID != tc.wantID || gotKind != tc.wantKind {
			t.Fatalf("deviceBucket(%d,%q)=(%q,%q) want (%q,%q)", tc.srcVal, tc.guid, gotID, gotKind, tc.wantID, tc.wantKind)
		}
	}
}
