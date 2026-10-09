// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	return out.String(), err
}

func fixClock(t *testing.T) {
	t.Helper()
	prev := clockNow
	clockNow = func() time.Time { return time.Date(2026, 10, 9, 11, 0, 0, 0, tickets.JST) }
	t.Cleanup(func() { clockNow = prev })
}

func TestTicketsUsageErrors(t *testing.T) {
	fixClock(t)
	cases := [][]string{
		{"onsale", "--date", "2026-10-01"},                                                           // past date
		{"onsale", "--from", "2026-11-05", "--to", "2026-11-01"},                                     // reversed
		{"onsale", "--from", "2026-11-01", "--to", "2027-02-01"},                                     // too long
		{"onsale", "--date", "2026-11-01", "--from", "2026-11-02"},                                   // mixed
		{"onsale", "--date", "2026-11-01", "--sights", "tokyo-tower"},                                // unknown sight
		{"onsale", "--date", "2026-11-01", "--tz", "Mars/Olympus"},                                   // bad tz
		{"onsale", "--sights", "sky"},                                                                // no dates
		{"availability", "teamlab-planets", "--from", "2026-11-01", "--to", "2026-11-10", "--slots"}, // >7 slot dates
		{"availability", "teamlab-planets", "--date", "2026-11-01", "--party", "pets=2"},
		{"onsale", "--date", "2026-11-01", "--data-source", "local"},
		{"onsale", "--from", "2026-11-01", "--to", "9999-12-31"}, // huge range rejected before it is built
		{"onsale", "--date", "2026-11-01", "--sights", ","},      // empty sight list
		{"availability", ",", "--date", "2026-11-01"},
	}
	for _, args := range cases {
		_, err := runCLI(t, args...)
		if err == nil || ExitCode(err) != 2 {
			t.Errorf("%v: err=%v code=%d, want usage error (2)", args, err, ExitCode(err))
		}
	}
}

func TestTicketsDryRunAndHelp(t *testing.T) {
	for _, args := range [][]string{{"onsale", "--date", "2026-11-01", "--dry-run"}, {"availability", "--dry-run"}, {"sights", "--dry-run"}} {
		out, err := runCLI(t, args...)
		if err != nil || !strings.Contains(out, "dry-run") {
			t.Errorf("%v: %q, %v", args, out, err)
		}
	}
	out, err := runCLI(t, "onsale")
	if err != nil || !strings.Contains(out, "Usage:") {
		t.Errorf("bare onsale must print help: %v", err)
	}
}

func TestSightsOffline(t *testing.T) {
	out, err := runCLI(t, "sights", "sky", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(v.Results) != 1 || v.Results[0]["id"] != "shibuya-sky" || v.Results[0]["name_ja"] != "渋谷スカイ" {
		t.Errorf("results = %v", v.Results)
	}
	if _, err := runCLI(t, "sights", ","); ExitCode(err) != 3 {
		t.Errorf("empty sight list exit = %d, want 3 (no panic)", ExitCode(err))
	}
	if _, err := runCLI(t, "sights", "tokyo-tower"); ExitCode(err) != 3 {
		t.Errorf("unknown sight exit = %d, want 3", ExitCode(err))
	}
}

func TestSourceCommandHidden(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	for _, c := range root.Commands() {
		if c.Name() == "source" && (!c.Hidden || c.Annotations["mcp:hidden"] != "true") {
			t.Error("generated source command must be hidden from help and MCP")
		}
	}
}

func TestBuildAvailabilityStatuses(t *testing.T) {
	now := time.Date(2026, 10, 9, 11, 0, 0, 0, tickets.JST)
	planets, _ := tickets.Lookup("teamlab-planets")
	sky, _ := tickets.Lookup("shibuya-sky")
	pd := map[string]*tickets.SightData{}
	until := tickets.Date{Y: 2026, M: 12, D: 31}
	p := &tickets.SightData{Sight: planets, Days: map[string]tickets.Day{"2026-10-20": {Date: "2026-10-20", Status: tickets.DayFew}}, CalendarUntil: &until}
	two, ten := 2, 10
	p.Slots = []tickets.Slot{{Date: "2026-10-20", Start: "09:00", Stock: &two}, {Date: "2026-10-20", Start: "09:30", Stock: &ten}}
	pd[planets.ID] = p
	pd[sky.ID] = &tickets.SightData{Sight: sky, Days: map[string]tickets.Day{}, RuleConfirmed: true}
	dates := []tickets.Date{{Y: 2026, M: 10, D: 20}, {Y: 2027, M: 1, D: 10}}
	sights := []tickets.Sight{planets, sky}
	onsale := tickets.BuildOnsale(pd, sights, dates[0], dates[0], now, nil)
	party := &tickets.Party{Adults: 2, Children: 2}
	rows := buildAvailability(pd, sights, dates, onsale, true, party, now)
	if len(rows) != 4 {
		t.Fatalf("rows = %d", len(rows))
	}
	r := rows[0]
	if r.Status != tickets.DayFew || r.SlotsFitParty == nil || *r.SlotsFitParty != 1 || len(r.Slots) != 1 || r.Slots[0].Start != "09:30" {
		t.Errorf("planets party fit row = %+v", r)
	}
	if rows[1].Status != tickets.DayNotReleased {
		t.Errorf("planets beyond calendar = %s", rows[1].Status)
	}
	if rows[2].Status != tickets.DayUnknown || !strings.Contains(rows[2].Reason, "waiting room") || !strings.Contains(rows[2].PartyNote, "counter") {
		t.Errorf("sky row = %+v", rows[2])
	}

	// On the visit day, slots that already started are left out and a slot
	// with unknown stock stays without a fit verdict.
	today := tickets.Date{Y: 2026, M: 10, D: 9}
	p.Days[today.String()] = tickets.Day{Date: today.String(), Status: tickets.DayAvailable}
	p.Slots = []tickets.Slot{
		{Date: today.String(), Start: "09:00", StartJST: "2026-10-09T09:00:00+09:00", Stock: &ten},
		{Date: today.String(), Start: "12:00", StartJST: "2026-10-09T12:00:00+09:00"},
		{Date: today.String(), Start: "12:30", StartJST: "2026-10-09T12:30:00+09:00", Stock: &ten},
	}
	rows = buildAvailability(pd, []tickets.Sight{planets}, []tickets.Date{today}, nil, true, party, now)
	r = rows[0]
	if !r.SlotsRead || len(r.Slots) != 2 || r.Slots[0].Start != "12:00" || r.Slots[0].FitsParty != nil || *r.SlotsFitParty != 1 {
		t.Errorf("today row = %+v", r)
	}
}

func TestFailureErrorTypedExit(t *testing.T) {
	rl := []tickets.FetchFailure{{Sight: "teamlab-kyoto", Error: "HTTP 429", RateLimited: true}}
	if err := failureError(rl, 1); ExitCode(err) != 7 {
		t.Errorf("all sights rate limited: exit %d, want 7", ExitCode(err))
	}
	if err := failureError(rl, 2); err != nil {
		t.Errorf("partial failure must not abort: %v", err)
	}
	other := []tickets.FetchFailure{{Sight: "ghibli-museum", Error: "rate limited words in a non-429 error"}}
	if err := failureError(other, 1); ExitCode(err) != 5 {
		t.Errorf("all sights failed: exit %d, want 5", ExitCode(err))
	}
}
