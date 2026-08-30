// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseLooseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1.5d", 36 * time.Hour, false},
		{"  7D  ", 7 * 24 * time.Hour, false},
		{"", 0, true},
		{"tomorrow", 0, true},
		{"7x", 0, true},
	}
	for _, tc := range cases {
		got, err := parseLooseDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseLooseDuration(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseLooseDuration(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseLooseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseWindow(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	got, err := parseWindow(now, "7d")
	if err != nil {
		t.Fatalf("parseWindow returned error: %v", err)
	}
	if want := now.AddDate(0, 0, -7); !got.Equal(want) {
		t.Errorf("parseWindow(7d) = %v, want %v", got, want)
	}
	zero, err := parseWindow(now, "")
	if err != nil {
		t.Fatalf("parseWindow(empty) returned error: %v", err)
	}
	if !zero.IsZero() {
		t.Errorf("parseWindow(empty) = %v, want zero time", zero)
	}
	if _, err := parseWindow(now, "nonsense"); err == nil {
		t.Error("parseWindow(nonsense) = nil error, want error")
	}
}

func TestBudgetStatus(t *testing.T) {
	cases := []struct {
		used, cap int
		want      string
	}{
		{0, 100, "ok"},
		{79, 100, "ok"},
		{80, 100, "near-limit"},
		{99, 100, "near-limit"},
		{100, 100, "exhausted"},
		{140, 100, "exhausted"},
	}
	for _, tc := range cases {
		if got := budgetStatus(tc.used, tc.cap); got != tc.want {
			t.Errorf("budgetStatus(%d, %d) = %q, want %q", tc.used, tc.cap, got, tc.want)
		}
	}
}

func TestCounterpartNameResolutionOrder(t *testing.T) {
	attendees := attendeeIndex{
		byProviderID: map[string]string{"prov-1": "Attendee Name"},
		byAttendeeID: map[string]string{},
	}
	relations := map[string]relationRow{"prov-2": {MemberID: "prov-2", Name: "Relation Name"}}

	// A named group chat keeps its own name.
	if got := counterpartName(chatRow{Name: "Group Chat", AttendeeID: "prov-1"}, relations, attendees); got != "Group Chat" {
		t.Errorf("named chat = %q, want %q", got, "Group Chat")
	}
	// One-to-one chats prefer the attendee index.
	if got := counterpartName(chatRow{AttendeeID: "prov-1"}, relations, attendees); got != "Attendee Name" {
		t.Errorf("attendee lookup = %q, want %q", got, "Attendee Name")
	}
	// Then the connection list.
	if got := counterpartName(chatRow{AttendeeID: "prov-2"}, relations, attendees); got != "Relation Name" {
		t.Errorf("relation fallback = %q, want %q", got, "Relation Name")
	}
	// Then the raw id, so a name is never fabricated.
	if got := counterpartName(chatRow{AttendeeID: "prov-3"}, relations, attendees); got != "prov-3" {
		t.Errorf("id fallback = %q, want %q", got, "prov-3")
	}
	if got := counterpartName(chatRow{}, relations, attendees); got != "(unknown)" {
		t.Errorf("empty chat = %q, want %q", got, "(unknown)")
	}
}

func TestRound1(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want float64
	}{{20.34, 20.3}, {20.35, 20.4}, {0, 0}, {120, 120}} {
		if got := round1(tc.in); got != tc.want {
			t.Errorf("round1(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestProviderAliasesCoverSeededVocabulary(t *testing.T) {
	// The learn seeds promise these spellings resolve; accounts alias reads the
	// same table, so a missing entry silently breaks alias resolution.
	for _, provider := range []string{"LINKEDIN", "WHATSAPP", "TELEGRAM", "INSTAGRAM", "MESSENGER", "TWITTER", "GOOGLE", "OUTLOOK", "IMAP", "MAIL", "MOBILE"} {
		if len(providerAliases[provider]) == 0 {
			t.Errorf("providerAliases[%q] is empty", provider)
		}
	}
	if got := providerAliases["GOOGLE"]; len(got) < 2 || got[1] != "gmail" {
		t.Errorf("providerAliases[GOOGLE] = %v, want gmail among aliases", got)
	}
}
