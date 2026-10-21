// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

// TestNovelPinsAuditHelpWires smoke-tests that the pins audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPinsAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"pins", "audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pins audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pins audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func seedPinterestStore(t *testing.T, boards, pins []string) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	for _, b := range boards {
		if err := db.UpsertBoards(json.RawMessage(b)); err != nil {
			t.Fatalf("seed board: %v", err)
		}
	}
	for _, p := range pins {
		if err := db.UpsertPins(json.RawMessage(p)); err != nil {
			t.Fatalf("seed pin: %v", err)
		}
	}
	return dbPath
}

func runPinterestCmd(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(stdin))
	err := cmd.Execute()
	return out.String(), err
}

const (
	auditBoardA   = `{"id":"b1","name":"Recipes","privacy":"PUBLIC"}`
	auditBoardB   = `{"id":"b2","name":"Baking","privacy":"PUBLIC"}`
	auditCleanPin = `{"id":"p1","board_id":"b1","title":"Rituals","description":"d","alt_text":"candle","link":"https://example.com/a?utm_source=pinterest"}`
	auditNoAltPin = `{"id":"p2","board_id":"b1","title":"No alt","description":"d","link":"https://example.com/b?utm_source=pinterest"}`
	auditNoUTMPin = `{"id":"p3","board_id":"b2","title":"No utm","description":"d","alt_text":"x","link":"https://example.com/c"}`
)

func TestPinsAuditFindsOnlyBrokenPins(t *testing.T) {
	testenv.Isolate(t)
	db := seedPinterestStore(t, []string{auditBoardA, auditBoardB}, []string{auditCleanPin, auditNoAltPin, auditNoUTMPin})
	out, err := runPinterestCmd(t, "", "pins", "audit", "--db", db, "--json")
	if err != nil {
		t.Fatalf("audit: %v\n%s", err, out)
	}
	var rep pinAuditReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if rep.ScannedPins != 3 || rep.PinsWithIssues != 2 {
		t.Fatalf("scanned=%d with_issues=%d, want 3/2", rep.ScannedPins, rep.PinsWithIssues)
	}
	got := map[string][]string{}
	for _, p := range rep.Issues {
		got[p.PinID] = p.Issues
	}
	if _, ok := got["p1"]; ok {
		t.Fatalf("clean pin p1 reported: %v", got)
	}
	if strings.Join(got["p2"], ",") != "missing_alt_text" {
		t.Fatalf("p2 issues = %v", got["p2"])
	}
	if strings.Join(got["p3"], ",") != "link_without_utm" {
		t.Fatalf("p3 issues = %v", got["p3"])
	}
	for _, p := range rep.Issues {
		if p.PinID == "p2" && p.BoardName != "Recipes" {
			t.Fatalf("board name not resolved: %+v", p)
		}
	}
}

func TestPinsAuditGroupByBoardCounts(t *testing.T) {
	testenv.Isolate(t)
	db := seedPinterestStore(t, []string{auditBoardA, auditBoardB}, []string{auditCleanPin, auditNoAltPin, auditNoUTMPin})
	out, err := runPinterestCmd(t, "", "pins", "audit", "--db", db, "--group-by", "board", "--json")
	if err != nil {
		t.Fatalf("audit: %v\n%s", err, out)
	}
	var rows []boardAuditRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	byID := map[string]boardAuditRow{}
	for _, r := range rows {
		byID[r.BoardID] = r
	}
	if r := byID["b1"]; r.Pins != 2 || r.MissingAltText != 1 || r.LinkWithoutUTM != 0 || len(r.PinIDsWithIssues) != 1 {
		t.Fatalf("b1 row = %+v", r)
	}
	if r := byID["b2"]; r.Pins != 1 || r.LinkWithoutUTM != 1 || r.MissingAltText != 0 {
		t.Fatalf("b2 row = %+v", r)
	}
}

func TestPinsAuditIssueFilterAndEmptyStore(t *testing.T) {
	testenv.Isolate(t)
	db := seedPinterestStore(t, []string{auditBoardA, auditBoardB}, []string{auditCleanPin, auditNoAltPin, auditNoUTMPin})
	out, err := runPinterestCmd(t, "", "pins", "audit", "--db", db, "--issue", "link_without_utm", "--json")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	var rep pinAuditReport
	_ = json.Unmarshal([]byte(out), &rep)
	if len(rep.Issues) != 1 || rep.Issues[0].PinID != "p3" {
		t.Fatalf("issue filter returned %+v", rep.Issues)
	}

	empty := seedPinterestStore(t, nil, nil)
	out, err = runPinterestCmd(t, "", "pins", "audit", "--db", empty, "--json")
	if err != nil {
		t.Fatalf("empty audit: %v", err)
	}
	if !strings.Contains(out, `"issues": []`) && !strings.Contains(out, `"issues":[]`) {
		t.Fatalf("empty store should emit issues: [], got %s", out)
	}

	missing := filepath.Join(t.TempDir(), "nope.db")
	out, err = runPinterestCmd(t, "", "pins", "audit", "--db", missing, "--json")
	if err != nil || !strings.Contains(out, `"scanned_pins": 0`) && !strings.Contains(out, `"scanned_pins":0`) {
		t.Fatalf("missing db: err=%v out=%s", err, out)
	}
}

func TestPinsAuditRejectsBadFlags(t *testing.T) {
	testenv.Isolate(t)
	if _, err := runPinterestCmd(t, "", "pins", "audit", "--group-by", "section", "--json"); err == nil {
		t.Fatal("expected usage error for --group-by section")
	}
	if _, err := runPinterestCmd(t, "", "pins", "audit", "--issue", "bogus", "--json"); err == nil {
		t.Fatal("expected usage error for --issue bogus")
	}
}
