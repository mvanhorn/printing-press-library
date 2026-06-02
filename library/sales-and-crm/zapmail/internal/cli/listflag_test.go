// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
	"time"
)

func TestParseListFlag(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want any
	}{
		{"empty", "", []string{}},
		{"single", "SMARTLEAD", []string{"SMARTLEAD"}},
		{"csv", "SMARTLEAD,INSTANTLY", []string{"SMARTLEAD", "INSTANTLY"}},
		{"csv with spaces", " a , b ,c", []string{"a", "b", "c"}},
		{"trailing comma", "a,b,", []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseListFlag(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseListFlag(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseListFlagJSON(t *testing.T) {
	got := parseListFlag(`["A","B"]`)
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any from JSON input, got %T", got)
	}
	if len(arr) != 2 || arr[0] != "A" || arr[1] != "B" {
		t.Fatalf("parseListFlag JSON = %#v, want [A B]", arr)
	}
}

func TestParseFlexibleTime(t *testing.T) {
	if !parseFlexibleTime("").IsZero() {
		t.Fatal("empty should be zero time")
	}
	if !parseFlexibleTime("not-a-date").IsZero() {
		t.Fatal("garbage should be zero time")
	}
	got := parseFlexibleTime("2026-03-10T15:00:00.000Z")
	want := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseFlexibleTime = %v, want %v", got, want)
	}
	if d := parseFlexibleTime("2026-03-10"); d.IsZero() {
		t.Fatal("date-only should parse")
	}
}

func TestRound2(t *testing.T) {
	if got := round2(3.14159); got != 3.14 {
		t.Fatalf("round2(3.14159) = %v, want 3.14", got)
	}
}
