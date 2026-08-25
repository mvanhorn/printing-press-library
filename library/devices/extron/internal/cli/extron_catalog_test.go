// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
)

func TestMatchCategory(t *testing.T) {
	doc := extron.Doc{Title: "MAV Plus Series", Category: "Declaration of Conformity"}
	cases := []struct {
		filter string
		want   bool
	}{
		{"", true},
		{"declaration", true},
		{"Declaration of Conformity", true},
		{"DECLARATION OF CONFORMITY", true},
		{"manual", false},
		{"brochure", false},
	}
	for _, c := range cases {
		if got := matchCategory(doc, c.filter); got != c.want {
			t.Errorf("matchCategory(%q) = %v, want %v", c.filter, got, c.want)
		}
	}
}

func TestMatchLetter(t *testing.T) {
	cases := []struct {
		title, letter string
		want          bool
	}{
		{"MAV Plus Series", "m", true},
		{"MAV Plus Series", "M", true},
		{"MAV Plus Series", "a", false},
		{"12G HD-SDI Setup Guide", "0", true},
		{"12G HD-SDI Setup Guide", "1", false},
		{"Annotator 300", "a", true},
	}
	for _, c := range cases {
		doc := extron.Doc{Title: c.title}
		if got := matchLetter(doc, c.letter); got != c.want {
			t.Errorf("matchLetter(%q, %q) = %v, want %v", c.title, c.letter, got, c.want)
		}
	}
}

func TestParseCatalogDate(t *testing.T) {
	cases := []struct {
		in   string
		want string // yyyy-mm-dd
	}{
		{"Jun. 24, 2002", "2002-06-24"},
		{"Jan. 15, 2021", "2021-01-15"},
		{"garbage", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := parseCatalogDate(c.in)
		if c.want == "" {
			if !got.IsZero() {
				t.Errorf("parseCatalogDate(%q) = %v, want zero", c.in, got)
			}
			continue
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("parseCatalogDate(%q) = %v, want %s", c.in, got, c.want)
		}
	}
}

func TestSortDocsByDateDesc(t *testing.T) {
	docs := []extron.Doc{
		{Title: "Old", Date: "Mar. 1, 2001"},
		{Title: "New", Date: "Jan. 29, 2026"},
		{Title: "Weird", Date: "not a date"},
		{Title: "Mid", Date: "Jun. 24, 2010"},
	}
	sortDocsByDateDesc(docs)
	wantOrder := []string{"New", "Mid", "Old", "Weird"}
	for i, w := range wantOrder {
		if docs[i].Title != w {
			t.Fatalf("sortDocsByDateDesc order[%d] = %q, want %q (got %v)", i, docs[i].Title, w, docs)
		}
	}
}

func TestResolveDocsRanking(t *testing.T) {
	docs := []extron.Doc{
		{Title: "MAV Plus Series", URL: "/download/files/brochure/mav_plus_series_revE.pdf"},
		{Title: "MAV Plus 328 A", URL: "/download/files/declaration/33-1832-01.pdf"},
		{Title: "MGP 641 xi 5K", URL: "/download/files/brochure/MGP_641_xi_5K.pdf"},
	}
	got := resolveDocs(docs, "mav plus", 10)
	if len(got) != 2 {
		t.Fatalf("resolveDocs len = %d, want 2", len(got))
	}
	if got[0].Title != "MAV Plus Series" {
		t.Errorf("resolveDocs[0] = %q, want exact title first", got[0].Title)
	}
	if got[1].Title != "MAV Plus 328 A" {
		t.Errorf("resolveDocs[1] = %q, want substring match second", got[1].Title)
	}
}

func TestParseBOMModel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"MAV 128\n", "MAV 128"},
		{"  DTP2 T 211 4K  \n", "DTP2 T 211 4K"},
		{"Model Number,Description,Qty\n", ""},
		{"SMX 123,Matrix Switcher,1\n", "SMX 123"},
		{"\n", ""},
	}
	for _, c := range cases {
		if got := parseBOMModel(c.in); got != c.want {
			t.Errorf("parseBOMModel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRevsEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"E", "E", true},
		{"Rev E", "E", true},
		{"REV C1", "C1", true},
		{"A", "B", false},
		{"", "", true},
		{"E", "", false},
	}
	for _, c := range cases {
		if got := revsEqual(c.a, c.b); got != c.want {
			t.Errorf("revsEqual(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseDaysAgo(t *testing.T) {
	cutoff, err := parseDaysAgo("30d")
	if err != nil {
		t.Fatalf("parseDaysAgo(30d) error = %v", err)
	}
	if d := time.Since(cutoff); d < 29*24*time.Hour || d > 31*24*time.Hour {
		t.Errorf("parseDaysAgo(30d) off by %v", d)
	}
	if _, err := parseDaysAgo("7"); err != nil {
		t.Errorf("parseDaysAgo(7) error = %v, want bare-day support", err)
	}
	if _, err := parseDaysAgo("nope"); err == nil {
		t.Error("parseDaysAgo(nope) = nil error, want error")
	}
}
