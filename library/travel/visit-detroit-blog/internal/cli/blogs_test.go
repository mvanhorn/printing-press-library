// Copyright 2026 stanrails and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestBlogSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/inside-the-d/donuts/", "donuts"},
		{"/inside-the-d/ultimate-guide/", "ultimate-guide"},
		{"/a/b/c/", "c"},
		{"donuts", "donuts"},
		{"", ""},
	}
	for _, c := range cases {
		if got := blogSlug(c.in); got != c.want {
			t.Errorf("blogSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBlogURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/inside-the-d/donuts/", "https://visitdetroit.com/inside-the-d/donuts/"},
		{"some-slug/x/", "https://visitdetroit.com/some-slug/x/"},
		{"", ""},
	}
	for _, c := range cases {
		if got := blogURL(c.in); got != c.want {
			t.Errorf("blogURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestContainsFold(t *testing.T) {
	list := []string{"Dining", "Culture"}
	if !containsFold(list, "dining") {
		t.Error("containsFold should match case-insensitively")
	}
	if containsFold(list, "Dini") {
		t.Error("containsFold should require an exact (not prefix) match")
	}
	if containsFold(nil, "x") {
		t.Error("containsFold(nil) should be false")
	}
}

func TestRegionMatch(t *testing.T) {
	list := []string{"Downtown Detroit » Greektown", "Wayne County"}
	if !regionMatch(list, "Greektown") {
		t.Error("regionMatch should match a sub-neighborhood as a substring")
	}
	if !regionMatch(list, "wayne county") {
		t.Error("regionMatch should match case-insensitively")
	}
	if regionMatch(list, "Corktown") {
		t.Error("regionMatch should not match an absent region")
	}
}

func TestIntersectIn(t *testing.T) {
	got := intersectIn([]string{"Dining", "Culture"}, lowerSet([]string{"dining", "sports"}))
	if len(got) != 1 || got[0] != "Dining" {
		t.Errorf("intersectIn = %v, want [Dining]", got)
	}
}

func TestParseDateFlag(t *testing.T) {
	if ts, err := parseDateFlag(""); err != nil || ts != 0 {
		t.Errorf("parseDateFlag(empty) = %d,%v, want 0,nil", ts, err)
	}
	if ts, err := parseDateFlag("2024-02-03"); err != nil || ts <= 0 {
		t.Errorf("parseDateFlag(date) = %d,%v, want >0,nil", ts, err)
	}
	if ts, err := parseDateFlag("30d"); err != nil || ts <= 0 {
		t.Errorf("parseDateFlag(30d) = %d,%v, want >0,nil", ts, err)
	}
	if _, err := parseDateFlag("garbage"); err == nil {
		t.Error("parseDateFlag(garbage) should error")
	}
}

func TestFormatFromOutput(t *testing.T) {
	cases := []struct{ in, want string }{
		{"list.json", "json"},
		{"list.csv", "csv"},
		{"list.md", "md"},
		{"noext", "md"},
		{"", "md"},
	}
	for _, c := range cases {
		if got := formatFromOutput(c.in); got != c.want {
			t.Errorf("formatFromOutput(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCSVField(t *testing.T) {
	if got := csvField("plain"); got != "plain" {
		t.Errorf("csvField(plain) = %q", got)
	}
	if got := csvField("a,b"); got != `"a,b"` {
		t.Errorf("csvField(a,b) = %q, want quoted", got)
	}
	if got := csvField(`he said "hi"`); got != `"he said ""hi"""` {
		t.Errorf("csvField with quotes = %q", got)
	}
}

func TestResolveSponsored(t *testing.T) {
	if v, err := resolveSponsored(false, false); err != nil || v != "" {
		t.Errorf("resolveSponsored(f,f) = %q,%v", v, err)
	}
	if v, err := resolveSponsored(true, false); err != nil || v != "no" {
		t.Errorf("resolveSponsored(t,f) = %q,%v", v, err)
	}
	if v, err := resolveSponsored(false, true); err != nil || v != "only" {
		t.Errorf("resolveSponsored(f,t) = %q,%v", v, err)
	}
	if _, err := resolveSponsored(true, true); err == nil {
		t.Error("resolveSponsored(t,t) should error (mutually exclusive)")
	}
}

func TestBlogFilterMatch(t *testing.T) {
	b := blogRecord{
		Categories: []string{"Dining", "Culture"},
		Regions:    []string{"Corktown", "Wayne County"},
		PostDate:   1700000000,
		Sponsored:  false,
	}
	cases := []struct {
		name string
		f    blogFilter
		want bool
	}{
		{"empty matches", blogFilter{}, true},
		{"category match", blogFilter{category: "dining"}, true},
		{"category miss", blogFilter{category: "Sports"}, false},
		{"region match", blogFilter{region: "corktown"}, true},
		{"region miss", blogFilter{region: "Greektown"}, false},
		{"since pass", blogFilter{since: 1600000000}, true},
		{"since fail", blogFilter{since: 1800000000}, false},
		{"until pass", blogFilter{until: 1800000000}, true},
		{"until fail", blogFilter{until: 1600000000}, false},
		{"no-sponsored keeps editorial", blogFilter{sponsored: "no"}, true},
		{"sponsored-only drops editorial", blogFilter{sponsored: "only"}, false},
	}
	for _, c := range cases {
		if got := c.f.match(b); got != c.want {
			t.Errorf("%s: match = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBlogRecordID(t *testing.T) {
	if got := (blogRecord{ObjectID: "123"}).id(); got != "123" {
		t.Errorf("id() with objectID = %q, want 123", got)
	}
	if got := (blogRecord{IDNum: 456}).id(); got != "456" {
		t.Errorf("id() with IDNum = %q, want 456", got)
	}
	if got := (blogRecord{URI: "/inside-the-d/x/"}).id(); got != "x" {
		t.Errorf("id() slug fallback = %q, want x", got)
	}
}
