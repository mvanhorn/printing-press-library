// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
)

// TestNovelPinsPreflightHelpWires smoke-tests that the pins preflight command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPinsPreflightHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"pins", "preflight", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pins preflight --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "preflight"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pins preflight --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPreflightPinRules(t *testing.T) {
	boards := []localBoard{{ID: "b1", Name: "Recipes"}, {ID: "b2", Name: "Dupe"}, {ID: "b3", Name: "dupe"}}
	sections := []map[string]string{{"id": "s1", "name": "Journals", "board_id": "b1"}}
	good := preflightProposal{Board: "recipes", Section: "Journals", Title: "Rituals", AltText: "candle",
		Link: "https://example.com/a?utm_source=pinterest", ImageURL: "https://example.com/i.jpg"}
	r := preflightPin(good, boards, sections)
	if !r.OK || len(r.Errors) != 0 || len(r.Warnings) != 0 {
		t.Fatalf("good proposal: %+v", r)
	}
	if r.CreateBody["board_id"] != "b1" || r.CreateBody["board_section_id"] != "s1" {
		t.Fatalf("name resolution failed: %+v", r.CreateBody)
	}

	cases := []struct {
		name string
		mut  func(*preflightProposal)
		err  string
		warn string
	}{
		{"title too long", func(p *preflightProposal) { p.Title = strings.Repeat("x", 101) }, "title_too_long", ""},
		{"title at limit ok", func(p *preflightProposal) { p.Title = strings.Repeat("é", 100) }, "", ""},
		{"unknown board", func(p *preflightProposal) { p.Board = "Nope" }, "board_not_found", ""},
		{"ambiguous board", func(p *preflightProposal) { p.Board = "DUPE"; p.Section = "" }, "board_ambiguous", ""},
		{"bad image", func(p *preflightProposal) { p.ImageURL = "ftp://x/y.jpg" }, "image_url_invalid", ""},
		{"no utm", func(p *preflightProposal) { p.Link = "https://example.com/a" }, "", "link_without_utm"},
		{"no alt", func(p *preflightProposal) { p.AltText = "" }, "", "missing_alt_text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mut(&p)
			r := preflightPin(p, boards, sections)
			if tc.err == "" && !r.OK {
				t.Fatalf("unexpected errors %v", r.Errors)
			}
			if tc.err != "" && (r.OK || !containsString(r.Errors, tc.err)) {
				t.Fatalf("want error %s, got %+v", tc.err, r)
			}
			if tc.warn != "" && !containsString(r.Warnings, tc.warn) {
				t.Fatalf("want warning %s, got %v", tc.warn, r.Warnings)
			}
		})
	}

	byID := good
	byID.Board = "1234567890123456789"
	byID.Section = "1234567890123456789"
	unverified := preflightPin(byID, nil, nil)
	if !unverified.OK || unverified.CreateBody["board_id"] != "1234567890123456789" || unverified.CreateBody["board_section_id"] != "1234567890123456789" || !containsString(unverified.Warnings, "board_unverified") || !containsString(unverified.Warnings, "section_unverified") {
		t.Fatalf("empty store with numeric ids should warn, not fail: %+v", unverified)
	}
	namedSection := good
	namedSection.Section = "Missing Section"
	ns := preflightPin(namedSection, boards, sections)
	if ns.OK || !containsString(ns.Errors, "section_unresolved") || ns.CreateBody["board_section_id"] != nil {
		t.Fatalf("an unresolvable section name must not become board_section_id: %+v", ns)
	}
	byName := preflightPin(good, nil, nil)
	if byName.OK || !containsString(byName.Errors, "board_unresolved") || byName.CreateBody["board_id"] != nil {
		t.Fatalf("an unresolvable board name must not become board_id: %+v", byName)
	}
}

func TestPinsPreflightCommandStdinAndRequired(t *testing.T) {
	testenv.Isolate(t)
	db := seedPinterestStore(t, []string{`{"id":"b1","name":"Recipes"}`}, nil)
	in := `{"board":"Recipes","title":"A","alt_text":"x","link":"https://example.com/?utm_source=p","image_url":"https://example.com/i.jpg"}
{"board":"Missing","image_url":"https://example.com/i.jpg"}
`
	out, err := runPinterestCmd(t, in, "pins", "preflight", "--stdin", "--strict", "--db", db, "--json")
	if err == nil || !strings.Contains(err.Error(), "1 of 2 Pin proposals failed") {
		t.Fatalf("a failing proposal must exit non-zero after printing: err=%v", err)
	}
	var res []preflightResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if len(res) != 2 || !res[0].OK || res[0].CreateBody["board_id"] != "b1" || res[1].OK || !containsString(res[1].Errors, "board_not_found") {
		t.Fatalf("stdin results = %+v", res)
	}
	if _, err := runPinterestCmd(t, "", "pins", "preflight", "--title", "x", "--json"); err == nil {
		t.Fatal("expected usage error when --board/--image-url missing")
	}
}
