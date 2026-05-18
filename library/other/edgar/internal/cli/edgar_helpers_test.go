// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestNormalizeCIK(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"320193", "0000320193", false},
		{"0000320193", "0000320193", false},
		{"CIK0000320193", "0000320193", false},
		{"cik320193", "0000320193", false},
		{"abc", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := normalizeCIK(c.in)
		if c.err {
			if err == nil {
				t.Errorf("normalizeCIK(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeCIK(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("normalizeCIK(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeAccession(t *testing.T) {
	wd, nd, err := normalizeAccession("0000320193-22-000049")
	if err != nil {
		t.Fatal(err)
	}
	if wd != "0000320193-22-000049" || nd != "000032019322000049" {
		t.Errorf("got (%q, %q)", wd, nd)
	}
	wd2, nd2, err := normalizeAccession("000032019322000049")
	if err != nil {
		t.Fatal(err)
	}
	if wd2 != wd || nd2 != nd {
		t.Errorf("round-trip failed: (%q, %q)", wd2, nd2)
	}
	if _, _, err := normalizeAccession("short"); err == nil {
		t.Error("expected error on short input")
	}
}

func TestExtractSections_BoundaryUnverifiable_NoHeader(t *testing.T) {
	body := "This filing has no ITEM headers at all, just running text."
	results, anyFailed := extractSections(body, []string{"1A"})
	if !anyFailed {
		t.Fatal("expected anyFailed=true when no headers found")
	}
	if len(results) != 1 || results[0].Error != "boundary_unverifiable" {
		t.Fatalf("expected single boundary_unverifiable, got %+v", results)
	}
}

func TestExtractSections_HappyPath(t *testing.T) {
	body := "PART I\nITEM 1A. Risk Factors\n" + strings.Repeat("Material risks include ...\n", 200) +
		"ITEM 7. Management Discussion\n" + strings.Repeat("MD&A content here.\n", 200)
	results, anyFailed := extractSections(body, []string{"1A"})
	if anyFailed {
		t.Fatalf("expected unambiguous parse; got %+v", results)
	}
	if len(results) != 1 || results[0].Error != "" || results[0].TextLength == 0 {
		t.Fatalf("expected parsed section, got %+v", results)
	}
}

func TestExtractSections_TOCAmbiguity(t *testing.T) {
	// TOC at top + real Item header below. Only one candidate has substantial content.
	body := "TABLE OF CONTENTS\nITEM 1A.   1\nITEM 7.   12\n\nPART I\n\n" +
		"ITEM 1A. Risk Factors\n" + strings.Repeat("Material risks include ...\n", 200)
	results, anyFailed := extractSections(body, []string{"1A"})
	if anyFailed {
		t.Logf("results: %+v", results)
		// With our heuristic, the TOC entry has tiny body (< 2KB) and the real one
		// has substantial content — exactly one substantial candidate — so this should pass.
		t.Fatal("expected TOC entry to be filtered, leaving one substantial candidate")
	}
	if results[0].Error != "" {
		t.Errorf("expected unambiguous, got %+v", results[0])
	}
}

func TestParseEightKItems(t *testing.T) {
	body := "Item 5.02 Departure of Officer. the CEO resigned.\nItem 9.01 Exhibits."
	items := parseEightKItems(body)
	if len(items) != 2 || items[0] != "5.02" || items[1] != "9.01" {
		t.Errorf("got items %+v", items)
	}
}

func TestSeniorOfficerDetection(t *testing.T) {
	cases := map[string]bool{
		"Chief Executive Officer":    true,
		"CEO and Director":           true,
		"chief financial officer":    true,
		"President":                  true,
		"Chairman of the Board":      true,
		"VP of Engineering":          false,
		"Senior Director, Marketing": false,
		"":                           false,
	}
	for title, want := range cases {
		got := seniorOfficerRE.MatchString(title)
		if got != want {
			t.Errorf("seniorOfficerRE(%q) = %v, want %v", title, got, want)
		}
	}
}
