// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestVersionBelow(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"8.1", "8.2", true},
		{"8.1.30", "8.2", true},
		{"8.2", "8.2", false},
		{"8.10", "8.2", false}, // numeric, not lexicographic
		{"7.4", "8.0", true},
		{"6.4.2", "6.4", false},
		{"6.3", "6.4.1", true},
		{"8.2", "8.2.0", false}, // missing segment == zero, not below
		{"8.2.0", "8.2", false},
		{"8.1", "8.2.0", true},
		{"", "8.2", false}, // unknown never counts as below
		{"8.2", "", false},
	}
	for _, c := range cases {
		if got := versionBelow(c.a, c.b); got != c.want {
			t.Errorf("versionBelow(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCertNameCovers(t *testing.T) {
	cases := []struct {
		cert, domain string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"Example.COM", "example.com", true},
		{"*.example.com", "www.example.com", true},
		{"*.example.com", "a.b.example.com", false}, // wildcard covers one label only
		{"*.example.com", "example.com", false},
		{"other.com", "example.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
	}
	for _, c := range cases {
		if got := certNameCovers(c.cert, c.domain); got != c.want {
			t.Errorf("certNameCovers(%q, %q) = %v, want %v", c.cert, c.domain, got, c.want)
		}
	}
}

func TestDecodeRedirectTargets(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"object entries", `[{"name":"a.com"},{"name":"b.com"}]`, []string{"a.com", "b.com"}},
		{"string entries", `["a.com","b.com"]`, []string{"a.com", "b.com"}},
		{"empty array", `[]`, []string{}},
		{"null", ``, nil},
		{"objects without name", `[{"id":"x"}]`, []string{}},
	}
	for _, c := range cases {
		var raw json.RawMessage
		if c.raw != "" {
			raw = json.RawMessage(c.raw)
		}
		got := decodeRedirectTargets(raw)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: decodeRedirectTargets(%s) = %v, want %v", c.name, c.raw, got, c.want)
		}
	}
}

func TestParseAPITime(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"2026-07-20T12:00:00Z", true},
		{"2026-07-20T12:00:00.123Z", true},
		{"2026-07-20", true},
		{"", false},
		{"not-a-time", false},
	}
	for _, c := range cases {
		if _, ok := parseAPITime(c.in); ok != c.ok {
			t.Errorf("parseAPITime(%q) ok = %v, want %v", c.in, ok, c.ok)
		}
	}
}
